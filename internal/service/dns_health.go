package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zeptop-dev/captain/internal/domain"
	"github.com/zeptop-dev/captain/internal/notify"
	"github.com/zeptop-dev/captain/internal/store"
)

const dnsInterval = 5 * time.Minute

var ErrDNSBusy = errors.New("DNS check already running or capacity reached")
var ErrDNSConfigChanged = errors.New("DNS configuration changed during check")

// DNSHealth uses only configured node/entry names. It never changes DNS. The
// resolver is injectable so tests don't depend on public network availability.
type DNSHealth struct {
	Store  *store.Store
	Probe  *Probe
	Log    *slog.Logger
	Lookup func(context.Context, string, string) ([]string, error)
	Now    func() time.Time
	mu     sync.Mutex
	active map[int64]bool
}

func (d *DNSHealth) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

func dnsFingerprint(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func dnsIPs(values ...string) []string {
	out := []string{}
	for _, v := range values {
		if ip := net.ParseIP(strings.TrimSpace(v)); ip != nil {
			out = append(out, ip.String())
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// targets is called under Topology so names and their ownership form one view.
func (d *DNSHealth) targets(ctx context.Context, id int64) (*domain.Node, []store.DNSTarget, error) {
	n, err := d.Store.NodeByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	targets := []store.DNSTarget{}
	add := func(key, host, label string, expected []string, shared bool) error {
		if strings.TrimSpace(host) == "" {
			return nil
		}
		t := store.DNSTarget{Key: key, Host: host, Label: label, Expected: expected, Mode: "direct", Status: "unknown", Answers: []store.DNSAnswer{}}
		normalized, e := domain.NormalizeNodeDomain(host)
		if e != nil {
			t.Status = "invalid"
			targets = append(targets, t)
			return nil
		}
		t.Host = normalized
		uses, e := d.Store.NodeDomainUses(ctx, normalized, 0)
		if e != nil {
			return e
		}
		for _, u := range uses {
			shared = shared || u.Shared
		}
		switch {
		case shared:
			t.Mode = "shared"
			t.Expected = []string{}
		case len(uses) > 1:
			t.Mode = "conflict"
		case len(expected) == 0:
			t.Mode = "resolve_only"
		}
		targets = append(targets, t)
		return nil
	}
	if err := add("node", n.Domain, n.Name, dnsIPs(n.PublicAddr, n.V6Addr), n.DomainShared); err != nil {
		return nil, nil, err
	}
	gs, err := d.Store.IngressesByNode(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	for _, g := range gs {
		// Never compare a public NAT/IPLC name with BindIP or LineIP.
		host := g.EntryDomain
		if host == "" && net.ParseIP(g.EntryHost) == nil {
			host = g.EntryHost
		}
		if err := add("ingress:"+strconv.FormatInt(g.ID, 10), host, g.Name, dnsIPs(g.EntryHost), false); err != nil {
			return nil, nil, err
		}
	}
	return n, targets, nil
}

func (d *DNSHealth) Snapshot(ctx context.Context, id int64) (*store.DNSHealth, error) {
	d.Store.Topology.Lock()
	defer d.Store.Topology.Unlock()
	n, targets, err := d.targets(ctx, id)
	if err != nil {
		return nil, err
	}
	fingerprint := dnsFingerprint(targets)
	h, err := d.Store.DNSHealth(ctx, id)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	if h == nil || h.Fingerprint != fingerprint {
		return &store.DNSHealth{NodeID: id, Node: n.Name, Targets: targets, Stale: true}, nil
	}
	h.Node = n.Name
	age := d.now().Sub(time.Unix(h.CheckedAt, 0))
	h.Stale = age < 0 || age > 3*dnsInterval
	return h, nil
}
func (d *DNSHealth) Snapshots(ctx context.Context) ([]store.DNSHealth, error) {
	nodes, err := d.Store.ListNodes(ctx)
	if err != nil {
		return nil, err
	}
	out := []store.DNSHealth{}
	for _, n := range nodes {
		h, e := d.Snapshot(ctx, n.ID)
		if errors.Is(e, store.ErrNotFound) {
			continue
		}
		if e != nil {
			return nil, e
		}
		if len(h.Targets) > 0 {
			out = append(out, *h)
		}
	}
	return out, nil
}

func lookupDNS(ctx context.Context, source, host string) ([]string, error) {
	r := &net.Resolver{PreferGo: true, StrictErrors: true}
	if source == "cloudflare" {
		r.Dial = func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, "1.1.1.1:53")
		}
	}
	ips, err := r.LookupIP(ctx, "ip", host+".")
	values := []string{}
	for _, ip := range ips {
		values = append(values, ip.String())
	}
	return dnsIPs(values...), err
}
func (d *DNSHealth) resolve(ctx context.Context, host string) []store.DNSAnswer {
	lookup := d.Lookup
	if lookup == nil {
		lookup = lookupDNS
	}
	answers := make([]store.DNSAnswer, 2)
	var wg sync.WaitGroup
	for i, source := range []string{"panel", "cloudflare"} {
		wg.Add(1)
		go func(i int, source string) {
			defer wg.Done()
			limited, cancel := context.WithTimeout(ctx, 4*time.Second)
			defer cancel()
			ips, err := lookup(limited, source, host)
			a := store.DNSAnswer{Source: source, Addresses: dnsIPs(ips...)}
			if err != nil {
				a.Addresses = []string{}
				a.Error = "unavailable"
				var de *net.DNSError
				if errors.Is(err, context.DeadlineExceeded) {
					a.Error = "timeout"
				}
				if errors.As(err, &de) {
					if de.IsNotFound {
						a.Error = "not_found"
					} else if de.IsTimeout {
						a.Error = "timeout"
					}
				}
			} else if len(a.Addresses) == 0 {
				a.Error = "not_found"
			}
			answers[i] = a
		}(i, source)
	}
	wg.Wait()
	return answers
}
func dnsTargetStatus(t store.DNSTarget) string {
	if t.Status == "invalid" {
		return "invalid"
	}
	if t.Mode == "conflict" {
		return "conflict"
	}
	for _, a := range t.Answers {
		if a.Error != "" && a.Error != "not_found" {
			return "unknown"
		}
	}
	a, b := t.Answers[0], t.Answers[1]
	// CDN answers may deliberately differ by resolver location. Only direct
	// address checks require identical answers from these two viewpoints.
	if t.Mode != "direct" {
		if a.Error == "not_found" && b.Error == "not_found" {
			return "missing"
		}
		if a.Error != b.Error {
			return "inconsistent"
		}
		return "ok"
	}
	if a.Error != b.Error || !slices.Equal(a.Addresses, b.Addresses) {
		return "inconsistent"
	}
	if a.Error == "not_found" {
		return "missing"
	}
	if !slices.Equal(a.Addresses, t.Expected) {
		return "mismatch"
	}
	return "ok"
}
func dnsFailure(status string) bool {
	return status == "mismatch" || status == "missing" || status == "conflict" || status == "invalid"
}

// Check assumes the caller holds Accounts.RLock, like other admin requests.
// A 30-second cooldown and global/per-node limit bound manual and scheduled work.
func (d *DNSHealth) Check(ctx context.Context, id int64) (*store.DNSHealth, error) {
	d.mu.Lock()
	if d.active == nil {
		d.active = map[int64]bool{}
	}
	if d.active[id] || len(d.active) >= 4 {
		d.mu.Unlock()
		return nil, ErrDNSBusy
	}
	d.active[id] = true
	d.mu.Unlock()
	defer func() { d.mu.Lock(); delete(d.active, id); d.mu.Unlock() }()
	callerCtx := ctx
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	d.Store.Topology.Lock()
	n, targets, err := d.targets(ctx, id)
	old, oldErr := d.Store.DNSHealth(ctx, id)
	d.Store.Topology.Unlock()
	if err != nil {
		return nil, err
	}
	if oldErr != nil && !errors.Is(oldErr, store.ErrNotFound) {
		return nil, oldErr
	}
	fingerprint := dnsFingerprint(targets)
	if old != nil && old.Fingerprint == fingerprint && d.now().Unix() >= old.CheckedAt && d.now().Sub(time.Unix(old.CheckedAt, 0)) < 30*time.Second {
		old.Node = n.Name
		return old, nil
	}
	for i := range targets {
		if targets[i].Status == "invalid" {
			continue
		}
		if ctx.Err() != nil {
			targets[i].Answers = []store.DNSAnswer{{Source: "panel", Addresses: []string{}, Error: "timeout"}, {Source: "cloudflare", Addresses: []string{}, Error: "timeout"}}
			targets[i].Status = "unknown"
			continue
		}
		targets[i].Answers = d.resolve(ctx, targets[i].Host)
		targets[i].Status = dnsTargetStatus(targets[i])
	}
	if err := callerCtx.Err(); err != nil {
		return nil, err
	}
	// The query budget may expire on a node with many slow names. Preserve
	// completed observations and explicitly unknown targets instead of losing
	// the whole snapshot. Persistence has its own bounded budget.
	ctx, finish := context.WithTimeout(callerCtx, 5*time.Second)
	defer finish()
	now := d.now()
	h := &store.DNSHealth{NodeID: id, Node: n.Name, Targets: targets, CheckedAt: now.Unix(), Fingerprint: fingerprint}
	failures := []string{}
	allOK := true
	for _, t := range targets {
		if dnsFailure(t.Status) {
			failures = append(failures, t.Key+":"+t.Status)
		}
		if t.Status != "ok" {
			allOK = false
		}
	}
	// A different fault or edited target starts a fresh sustained-failure window.
	if len(failures) > 0 {
		h.FailureKey = dnsFingerprint(failures)
		h.FailureCount = 1
		h.FailureSince = now.Unix()
		h.LastFailureAt = now.Unix()
		if old != nil && old.Fingerprint == fingerprint && old.FailureKey == h.FailureKey && now.Unix() >= old.CheckedAt && now.Unix()-old.CheckedAt <= int64(3*dnsInterval/time.Second) {
			h.FailureCount, h.FailureSince, h.LastFailureAt = old.FailureCount, old.FailureSince, old.LastFailureAt
			if now.Unix()-old.LastFailureAt >= int64(dnsInterval/time.Second) {
				h.FailureCount++
				h.LastFailureAt = now.Unix()
			}
		}
	}
	// Revalidate after network I/O; an old result cannot overwrite a rename,
	// shared-mode change, ingress edit, deletion, or newer target configuration.
	d.Store.Topology.Lock()
	defer d.Store.Topology.Unlock()
	_, current, err := d.targets(ctx, id)
	if err != nil {
		return nil, err
	}
	if dnsFingerprint(current) != fingerprint {
		return nil, ErrDNSConfigChanged
	}
	if err := d.Store.SaveDNSHealth(ctx, h); err != nil {
		return nil, err
	}
	if d.Probe != nil {
		d.Probe.monitorMu.Lock()
		defer d.Probe.monitorMu.Unlock()
		enabled := d.Probe.Settings(ctx).Enabled
		check := store.IncidentCheck{Kind: "dns", Value: float64(h.FailureCount), Threshold: 3}
		switch {
		case !enabled || len(targets) == 0 || old != nil && old.Fingerprint != fingerprint:
			check.Resolution = "disabled"
			d.Probe.incident(ctx, n, check, now, "")
		case h.FailureCount >= 3 && now.Unix()-h.FailureSince >= int64(2*dnsInterval/time.Second):
			check.Active = true
			d.Probe.incident(ctx, n, check, now, fmt.Sprintf("⚠️ %s: DNS mismatch or missing records persisted across three checks. Review node DNS health.", notify.Escape(n.Name)))
		case allOK:
			d.Probe.incident(ctx, n, check, now, "")
			// Unknown/inconsistent results are not recovery evidence.
		}
	}
	return h, nil
}

// Run is independent of the billing/job ticker. At most four checks run at once;
// a slow resolver cannot block housekeeping or other nodes indefinitely.
func (d *DNSHealth) Run(ctx context.Context) {
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		d.Store.Accounts.RLock()
		nodes, err := d.Store.ListNodes(ctx)
		d.Store.Accounts.RUnlock()
		if err == nil {
			jobs := make(chan int64)
			var wg sync.WaitGroup
			for range 4 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for id := range jobs {
						d.Store.Accounts.RLock()
						h, e := d.Snapshot(ctx, id)
						if e == nil && (h.Stale || h.CheckedAt == 0 || d.now().Sub(time.Unix(h.CheckedAt, 0)) >= dnsInterval) {
							_, e = d.Check(ctx, id)
						}
						d.Store.Accounts.RUnlock()
						if e != nil && !errors.Is(e, ErrDNSBusy) && !errors.Is(e, ErrDNSConfigChanged) && !errors.Is(e, store.ErrNotFound) && ctx.Err() == nil && d.Log != nil {
							d.Log.Warn("DNS health check", "node_id", id, "err", e)
						}
					}
				}()
			}
			for _, n := range nodes {
				if ctx.Err() != nil {
					break
				}
				jobs <- n.ID
			}
			close(jobs)
			wg.Wait()
		} else if d.Log != nil {
			d.Log.Warn("DNS health inventory", "err", err)
		}
		d.Store.Accounts.RLock()
		if e := d.Store.PruneDNSChanges(ctx, time.Now()); e != nil && d.Log != nil {
			d.Log.Warn("DNS history retention", "err", e)
		}
		d.Store.Accounts.RUnlock()
		timer.Reset(time.Minute)
	}
}
