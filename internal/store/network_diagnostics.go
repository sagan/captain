package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/zeptop-dev/bosun/pkg/spec"
)

var ErrDiagnosticPending = errors.New("a network diagnostic is already pending")

// QueueNetworkDiagnostic serializes the admission check with insertion. The
// node enforces the same one-at-a-time limit independently of this database.
func (s *Store) QueueNetworkDiagnostic(ctx context.Context, id string, nodeID int64, req spec.DiagnosticRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	if _, err = tx.ExecContext(ctx, `UPDATE node_jobs SET done_at = ?, error = 'diagnostic request expired' WHERE node_id = ? AND kind = ? AND done_at IS NULL AND created_at <= ?`, now, nodeID, spec.NetworkDiagnosticKind, now-spec.DiagnosticTTLSeconds); err != nil {
		return err
	}
	var pending int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM node_jobs WHERE node_id = ? AND kind = ? AND done_at IS NULL`, nodeID, spec.NetworkDiagnosticKind).Scan(&pending); err != nil {
		return err
	}
	if pending > 0 {
		return ErrDiagnosticPending
	}
	raw, err := json.Marshal(spec.DiagnosticParams{DiagnosticRequest: req, ExpiresAt: now + spec.DiagnosticTTLSeconds})
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO node_jobs (id,node_id,kind,params_json,created_at) VALUES (?,?,?,?,?)`, id, nodeID, spec.NetworkDiagnosticKind, string(raw), now); err != nil {
		return err
	}
	// Keep at most 20 checks per node, including the current one. Destructive
	// lifecycle jobs have separate retention and must never be removed here.
	if _, err = tx.ExecContext(ctx, `DELETE FROM node_jobs WHERE node_id = ? AND kind = ? AND id NOT IN (SELECT id FROM node_jobs WHERE node_id = ? AND kind = ? ORDER BY created_at DESC,rowid DESC LIMIT 20)`, nodeID, spec.NetworkDiagnosticKind, nodeID, spec.NetworkDiagnosticKind); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) expireNetworkDiagnostics(ctx context.Context, nodeID int64) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `UPDATE node_jobs SET done_at = ?, error = 'diagnostic request expired' WHERE node_id = ? AND kind = ? AND done_at IS NULL AND created_at <= ?`, now, nodeID, spec.NetworkDiagnosticKind, now-spec.DiagnosticTTLSeconds)
	return err
}

func (s *Store) NetworkDiagnostics(ctx context.Context, nodeID int64) ([]NodeJob, error) {
	if err := s.expireNetworkDiagnostics(ctx, nodeID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,node_id,kind,params_json,result_json,error,created_at,done_at FROM node_jobs WHERE node_id = ? AND kind = ? AND created_at >= ? ORDER BY created_at DESC,rowid DESC LIMIT 20`, nodeID, spec.NetworkDiagnosticKind, time.Now().Add(-24*time.Hour).Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []NodeJob{}
	for rows.Next() {
		j, err := scanNodeJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
