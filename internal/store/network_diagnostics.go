package store

import (
	"context"
	"database/sql"
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
	if err := queueNetworkDiagnostic(ctx, tx, id, nodeID, req, time.Now().Unix()); err != nil {
		return err
	}
	return tx.Commit()
}

func queueNetworkDiagnostic(ctx context.Context, tx *sql.Tx, id string, nodeID int64, req spec.DiagnosticRequest, now int64) error {
	var err error
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
	if _, err = tx.ExecContext(ctx, `INSERT INTO node_jobs (id,node_id,kind,params_json,created_at,route_check) VALUES (?,?,?,?,?,?)`, id, nodeID, spec.NetworkDiagnosticKind, string(raw), now, boolInt(req.Type == "return_route")); err != nil {
		return err
	}
	limit, route := 20, boolInt(req.Type == "return_route")
	if route == 1 {
		limit = 120
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM node_jobs WHERE node_id = ? AND kind = ? AND route_check = ? AND done_at IS NOT NULL AND (created_at < ? OR id NOT IN (SELECT id FROM node_jobs WHERE node_id = ? AND kind = ? AND route_check = ? ORDER BY created_at DESC,rowid DESC LIMIT ?))`, nodeID, spec.NetworkDiagnosticKind, route, now-30*86400, nodeID, spec.NetworkDiagnosticKind, route, limit); err != nil {
		return err
	}
	return nil
}

func (s *Store) expireNetworkDiagnostics(ctx context.Context, nodeID int64) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `UPDATE node_jobs SET done_at = ?, error = 'diagnostic request expired' WHERE node_id = ? AND kind = ? AND done_at IS NULL AND created_at <= ?`, now, nodeID, spec.NetworkDiagnosticKind, now-spec.DiagnosticTTLSeconds)
	return err
}

// NetworkDiagnostics preserves the ordinary diagnostic history for older UIs.
func (s *Store) NetworkDiagnostics(ctx context.Context, nodeID int64) ([]NodeJob, error) {
	jobs, err := s.NetworkDiagnosticsWithTools(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	out := []NodeJob{}
	for _, j := range jobs {
		var p spec.DiagnosticRequest
		if json.Unmarshal(j.Params, &p) == nil && (p.Type == "route_tool_check" || p.Type == "route_tool_install") {
			continue
		}
		out = append(out, j)
	}
	return out, nil
}

func (s *Store) NetworkDiagnosticsWithTools(ctx context.Context, nodeID int64) ([]NodeJob, error) {
	if err := s.expireNetworkDiagnostics(ctx, nodeID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,node_id,kind,params_json,result_json,error,created_at,done_at FROM node_jobs WHERE node_id = ? AND kind = ? AND route_check = 0 AND created_at >= ? ORDER BY created_at DESC,rowid DESC LIMIT 20`, nodeID, spec.NetworkDiagnosticKind, time.Now().Add(-24*time.Hour).Unix())
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
