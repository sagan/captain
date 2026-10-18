package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/zeptop-dev/bosun/pkg/spec"
	"time"
)

var ErrHostPending = errors.New("a host management task is already pending")

func (s *Store) QueueHostOperation(ctx context.Context, id string, nodeID int64, r spec.HostRequest) error {
	if e := r.Validate(); e != nil {
		return e
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	if _, e = tx.ExecContext(ctx, `UPDATE node_jobs SET done_at=?,error='host task expired; inspect actual state before retry' WHERE node_id=? AND kind=? AND done_at IS NULL AND created_at<=?`, now, nodeID, spec.HostManagementKind, now-spec.HostManagementTTL); e != nil {
		return e
	}
	var count int
	if e = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM node_jobs WHERE node_id=? AND kind=? AND done_at IS NULL`, nodeID, spec.HostManagementKind).Scan(&count); e != nil {
		return e
	}
	if count > 0 {
		return ErrHostPending
	}
	raw, _ := json.Marshal(spec.HostJobParams{HostRequest: r, ExpiresAt: now + spec.HostManagementTTL})
	if _, e = tx.ExecContext(ctx, `INSERT INTO node_jobs (id,node_id,kind,params_json,created_at) VALUES (?,?,?,?,?)`, id, nodeID, spec.HostManagementKind, string(raw), now); e != nil {
		return e
	}
	// Keep only the newest completed observation. Checks must not evict
	// administrator operations from the 180-day / 200-operation history.
	if _, e = tx.ExecContext(ctx, `DELETE FROM node_jobs WHERE node_id=? AND kind=? AND done_at IS NOT NULL AND params_json LIKE '{"action":"inspect"%' AND id NOT IN (SELECT id FROM node_jobs WHERE node_id=? AND kind=? AND done_at IS NOT NULL AND params_json LIKE '{"action":"inspect"%' ORDER BY created_at DESC,rowid DESC LIMIT 1)`, nodeID, spec.HostManagementKind, nodeID, spec.HostManagementKind); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `DELETE FROM node_jobs WHERE node_id=? AND kind=? AND done_at IS NOT NULL AND params_json NOT LIKE '{"action":"inspect"%' AND (created_at<? OR id NOT IN (SELECT id FROM node_jobs WHERE node_id=? AND kind=? AND params_json NOT LIKE '{"action":"inspect"%' ORDER BY created_at DESC,rowid DESC LIMIT 200))`, nodeID, spec.HostManagementKind, now-180*86400, nodeID, spec.HostManagementKind); e != nil {
		return e
	}

	return tx.Commit()
}
func (s *Store) expireHostOperations(ctx context.Context, id int64) error {
	now := time.Now().Unix()
	_, e := s.db.ExecContext(ctx, `UPDATE node_jobs SET done_at=?,error='host task expired; inspect actual state before retry' WHERE node_id=? AND kind=? AND done_at IS NULL AND created_at<=?`, now, id, spec.HostManagementKind, now-spec.HostManagementTTL)
	return e
}
func (s *Store) HostOperations(ctx context.Context, id int64) ([]NodeJob, error) {
	if e := s.expireHostOperations(ctx, id); e != nil {
		return nil, e
	}
	if _, e := s.db.ExecContext(ctx, `DELETE FROM node_jobs WHERE node_id=? AND kind=? AND done_at IS NOT NULL AND params_json NOT LIKE '{"action":"inspect"%' AND created_at<?`, id, spec.HostManagementKind, time.Now().Add(-180*24*time.Hour).Unix()); e != nil {
		return nil, e
	}
	rows, e := s.db.QueryContext(ctx, `SELECT id,node_id,kind,params_json,result_json,error,created_at,done_at FROM node_jobs WHERE node_id=? AND kind=? ORDER BY created_at DESC,rowid DESC LIMIT 203`, id, spec.HostManagementKind)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []NodeJob{}
	for rows.Next() {
		j, e := scanNodeJob(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
