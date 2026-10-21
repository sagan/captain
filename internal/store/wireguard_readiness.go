package store

import (
	"context"
	"time"

	"github.com/zeptop-dev/bosun/pkg/spec"
)

type WireGuardReadiness struct {
	NodeID      int64                     `json:"node_id"`
	Name        string                    `json:"name"`
	Ready       bool                      `json:"ready"`
	Supported   bool                      `json:"supported"`
	Fresh       bool                      `json:"fresh"`
	ReportedAt  int64                     `json:"reported_at"`
	Checks      []spec.PrivateAccessCheck `json:"checks"`
	Installable bool                      `json:"installable"`
	Packages    []string                  `json:"packages"`
	Job         *NodeJob                  `json:"job,omitempty"`
}

func (s *Store) WireGuardReadiness(ctx context.Context, id int64, transit bool) (*WireGuardReadiness, error) {
	n, err := s.NodeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	out := &WireGuardReadiness{NodeID: id, Name: n.Name, Packages: []string{}, Checks: []spec.PrivateAccessCheck{}}
	add := func(id string, ok bool) { out.Checks = append(out.Checks, spec.PrivateAccessCheck{ID: id, OK: ok}) }
	add("paired", n.Paired)
	add("proxy", NodeProxyAllowed(n))
	rep, at, err := s.WireGuardReport(ctx, id)
	if err != nil {
		return nil, err
	}
	if !at.IsZero() {
		out.ReportedAt = at.Unix()
	}
	out.Fresh = rep != nil && time.Since(at) >= 0 && time.Since(at) <= 3*time.Minute
	add("fresh", out.Fresh)
	out.Supported = rep != nil && rep.Setup != nil
	if out.Fresh {
		if rep.Setup != nil {
			out.Checks = append(out.Checks, rep.Setup.Checks...)
			out.Packages = rep.Setup.Packages
			out.Installable = rep.Setup.Installable && NodeProxyAllowed(n) && n.Paired
		}
		add("runtime", rep.Available)
	}
	out.Ready = s.WireGuardCapable(ctx, id, transit)
	if transit && out.Fresh && rep.Available {
		add("xray", out.Ready)
	}
	return out, nil
}
func (s *Store) LatestWireGuardSetupJob(ctx context.Context, id int64) (*NodeJob, error) {
	if err := s.expireWireGuardSetup(ctx, id); err != nil {
		return nil, err
	}
	row := s.db.QueryRowContext(ctx, `SELECT id,node_id,kind,params_json,result_json,error,created_at,done_at FROM node_jobs WHERE node_id=? AND kind=? ORDER BY (done_at IS NULL) DESC,created_at DESC,done_at DESC,id DESC LIMIT 1`, id, spec.WireGuardSetupKind)
	j, err := scanNodeJob(row)
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &j, nil
}

func (s *Store) expireWireGuardSetup(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE node_jobs SET error='WireGuard setup request expired',done_at=? WHERE node_id=? AND kind=? AND done_at IS NULL AND created_at<?`, now(), id, spec.WireGuardSetupKind, time.Now().Add(-spec.WireGuardSetupTTL*time.Second).Unix())
	return err
}
