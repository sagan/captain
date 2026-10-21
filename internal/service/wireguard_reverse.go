package service

import (
	"context"

	"github.com/zeptop-dev/bosun/pkg/spec"
)

// Both endpoints must have fresh enforcement capability. Missing support
// removes the entire managed subscriber inbound, not merely its WG routing.
func (a *AgentState) wireGuardState(ctx context.Context, nodeID int64) ([]spec.WireGuardLink, map[int64]bool, error) {
	links, err := a.Store.WireGuardLinks(ctx, nodeID)
	if err != nil {
		return nil, nil, err
	}
	configs := []spec.WireGuardLink{}
	permitted := map[int64]bool{}
	for _, link := range links {
		ib, err := a.Store.InboundByID(ctx, link.UserInboundID)
		if err != nil {
			return nil, nil, err
		}
		if !ib.Enabled {
			continue
		}
		if !a.Store.WireGuardCapable(ctx, link.TransitID, true) || !a.Store.WireGuardCapable(ctx, link.ExitID, false) {
			continue
		}
		if link.DomainPreference != "" && !a.Store.WireGuardDomainPreferenceCapable(ctx, link.TransitID) {
			continue
		}
		if link.AutoForwarding && !a.Store.WireGuardForwardingCapable(ctx, link.ExitID) {
			continue
		}
		config, err := link.Config(nodeID == link.ExitID, ib.Settings.PrivateAccess)
		if err != nil {
			return nil, nil, err
		}
		configs = append(configs, config)
		if nodeID == link.TransitID {
			permitted[ib.ID] = true
		}
	}
	return configs, permitted, nil
}
