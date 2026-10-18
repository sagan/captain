package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/zeptop-dev/captain/internal/store"
)

// NodeAddresses applies observed addresses independently of report processing.
// Network I/O never holds Accounts or delays billing acknowledgements. Topology
// serializes writes with manual node/ingress edits and whole-site resets.
type NodeAddresses struct {
	Store *store.Store
	DNS   *DNS
	State *AgentState
	Log   *slog.Logger
}

func (s *NodeAddresses) Run(ctx context.Context) {
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
func (s *NodeAddresses) Tick(ctx context.Context, at time.Time) {
	nodes, err := s.Store.ListNodes(ctx)
	if err != nil {
		return
	}
	for _, n := range nodes {
		if ctx.Err() != nil {
			return
		}
		// Snapshot IDs alone are not authority: paired identity and observations are
		// re-read while Topology is held. Reset/re-pair clears all observations.
		func() {
			s.Store.Topology.Lock()
			defer s.Store.Topology.Unlock()
			before, err := s.Store.NodeByID(ctx, n.ID)
			if err != nil || !before.Paired {
				return
			}
			current, pending, err := s.Store.ReconcileNodeAddresses(ctx, n.ID, at)
			if err != nil {
				return
			}
			if (current.PublicAddr != before.PublicAddr || current.V6Addr != before.V6Addr) && s.State != nil {
				s.State.Invalidate()
			}
			if !pending || s.DNS == nil {
				return
			}
			if err = s.Store.BeginAddressDNS(ctx, n.ID, at); err != nil {
				return
			}
			dnsctx, cancel := context.WithTimeout(ctx, 25*time.Second)
			defer cancel()
			results := s.DNS.EnsureMany(WithDNSOrigin(dnsctx, DNSOrigin{NodeID: n.ID, Source: "node_address", Actor: "bosun"}), [2]string{current.Domain, current.PublicAddr}, [2]string{current.Domain, current.V6Addr})
			for _, r := range results {
				if r.Error != "" {
					if s.Log != nil {
						s.Log.Warn("automatic node address DNS update pending", "node", n.ID)
					}
					return
				}
			}
			_ = s.Store.FinishAddressDNS(ctx, n.ID)
		}()
	}
}
