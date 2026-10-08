package store

import (
	"context"
	"encoding/json"
	"github.com/zeptop-dev/bosun/pkg/spec"
)

func (s *Store) NodeEgressUpstreams(ctx context.Context, id int64) ([]spec.EgressUpstream, error) {
	var raw string
	if err := s.db.QueryRowContext(ctx, `SELECT egress_upstreams_json FROM nodes WHERE id = ?`, id).Scan(&raw); err != nil {
		return nil, wrapNotFound(err)
	}
	list := []spec.EgressUpstream{}
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (s *Store) SetNodeEgressUpstreams(ctx context.Context, id int64, list []spec.EgressUpstream) error {
	if err := spec.ValidateEgressUpstreams(list); err != nil {
		return err
	}
	if list == nil {
		list = []spec.EgressUpstream{}
	}
	raw, err := json.Marshal(list)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE nodes SET egress_upstreams_json = ?, updated_at = ? WHERE id = ?`, string(raw), now(), id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return ErrNotFound
	}
	return err
}
