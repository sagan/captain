package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/captain/internal/store"
)

// Country lookup is panel-side and uses only the observed public IP. It never
// connects to that IP, accepts a user-supplied URL, or sends node credentials.
func lookupCountry(ctx context.Context, ip string) (string, error) {
	client := &http.Client{Timeout: 4 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return lookupCountryWithClient(ctx, ip, client)
}
func lookupCountryWithClient(ctx context.Context, ip string, client *http.Client) (string, error) {
	a, err := netip.ParseAddr(ip)
	if err != nil || !agentproto.ValidPublicAddress(ip, a.Is6()) {
		return "", errors.New("invalid public IP")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.country.is/"+a.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", errors.New("country service unavailable")
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4097))
	if err != nil || len(raw) > 4096 {
		return "", errors.New("invalid country response")
	}
	var out struct {
		IP      string `json:"ip"`
		Country string `json:"country"`
	}
	if json.Unmarshal(raw, &out) != nil {
		return "", errors.New("invalid country response")
	}
	returned, err := netip.ParseAddr(out.IP)
	if err != nil || returned != a {
		return "", errors.New("country response IP mismatch")
	}
	if out.Country == "" {
		return "", nil
	}
	if !store.ValidRegion(out.Country) {
		return "", errors.New("invalid country code")
	}
	return out.Country, nil
}

type NodeRegions struct {
	Store       *store.Store
	Probe       *Probe
	Lookup      func(context.Context, string) (string, error) // nil uses Country.is
	once        sync.Once
	gate        chan struct{}
	nextRequest time.Time // protected by gate
}

func (s *NodeRegions) Run(ctx context.Context) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		s.Tick(ctx, time.Now())
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
func (s *NodeRegions) Tick(ctx context.Context, at time.Time) {
	nodes, err := s.Store.ListNodes(ctx)
	if err != nil {
		return
	}
	for _, n := range nodes {
		if ctx.Err() != nil {
			return
		}
		state, err := s.Store.NodeRegionState(ctx, n.ID)
		if err != nil || state.Mode != "auto" || !state.Fresh(at) {
			continue
		}
		_ = s.Detect(ctx, n.ID, false)
	}
}

// One bounded request stream across scheduled and interactive detection, below
// the provider's 10/s limit. Cached failures retry hourly, successes weekly.
func (s *NodeRegions) Detect(ctx context.Context, id int64, refresh bool) error {
	s.once.Do(func() { s.gate = make(chan struct{}, 1) })
	select {
	case s.gate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-s.gate }()
	state, err := s.Store.NodeRegionState(ctx, id)
	if err != nil {
		return err
	}
	at := time.Now()
	if !state.Fresh(at) || (!refresh && state.Mode != "auto") {
		return nil
	}
	ip4, ip6 := state.IPs()
	d := state.Detection
	changed := false
	detect := func(ip string, result *store.RegionResult) error {
		if ip == "" {
			return nil
		}
		ttl := time.Hour
		if result.Status == "ok" {
			ttl = 7 * 24 * time.Hour
		}
		if refresh {
			ttl = time.Minute
		}
		if result.IP == ip && result.CheckedAt > 0 && at.Sub(time.Unix(result.CheckedAt, 0)) < ttl {
			return nil
		}
		if delay := time.Until(s.nextRequest); delay > 0 {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
			}
		}
		s.nextRequest = time.Now().Add(150 * time.Millisecond)
		lookup := s.Lookup
		if lookup == nil {
			lookup = lookupCountry
		}
		queryCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		country, queryErr := lookup(queryCtx, ip)
		cancel()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		*result = store.RegionResult{IP: ip, CheckedAt: time.Now().Unix(), Status: "unknown"}
		if queryErr != nil {
			result.Status = "failed"
		} else if store.ValidRegion(country) {
			result.Country = country
			result.Status = "ok"
		}
		changed = true
		return nil
	}
	if err = detect(ip4, &d.IPv4); err != nil {
		return err
	}
	if err = detect(ip6, &d.IPv6); err != nil {
		return err
	}
	if !changed {
		return nil
	}
	if err = s.Store.SaveRegionDetection(ctx, state, d); err != nil {
		return err
	}
	if s.Probe != nil {
		s.Probe.Invalidate()
	}
	return nil
}
