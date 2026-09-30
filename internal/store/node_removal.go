package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

const NodeRemovalKind = "node_remove"

var ErrRemovalPending = errors.New("a node removal is already pending")

type NodeRemovalParams struct {
	Mode      string `json:"mode"`
	KeepData  bool   `json:"keep_data"`
	ExpiresAt int64  `json:"expires_at"`
}

func (s *Store) LatestNodeRemoval(ctx context.Context, nodeID int64) (*NodeJob, error) {
	j, err := scanNodeJob(s.db.QueryRowContext(ctx, `SELECT id,node_id,kind,params_json,result_json,error,created_at,done_at FROM node_jobs WHERE node_id = ? AND kind = ? ORDER BY created_at DESC, rowid DESC LIMIT 1`, nodeID, NodeRemovalKind))
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return &j, nil
}

// QueueNodeRemoval uses a transaction so simultaneous clicks cannot queue
// different destructive operations for the same node.
func (s *Store) QueueNodeRemoval(ctx context.Context, id string, nodeID int64, p NodeRemovalParams) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id,params_json,result_json,done_at FROM node_jobs WHERE node_id = ? AND kind = ? AND (done_at IS NULL OR error = '')`, nodeID, NodeRemovalKind)
	if err != nil {
		return err
	}
	var expired []string
	blocked := false
	for rows.Next() {
		var oldID, params, result string
		var done sql.NullInt64
		if err := rows.Scan(&oldID, &params, &result, &done); err != nil {
			rows.Close()
			return err
		}
		var old NodeRemovalParams
		var progress struct {
			Phase string `json:"phase"`
		}
		_ = json.Unmarshal([]byte(result), &progress)
		if !done.Valid && json.Unmarshal([]byte(params), &old) == nil && old.ExpiresAt <= time.Now().Unix() && progress.Phase != "running" {
			expired = append(expired, oldID)
		} else {
			blocked = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if blocked {
		return ErrRemovalPending
	}
	for _, oldID := range expired {
		if _, err := tx.ExecContext(ctx, `UPDATE node_jobs SET done_at = ?, error = ? WHERE id = ?`, time.Now().Unix(), "removal request expired", oldID); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO node_jobs (id,node_id,kind,params_json,created_at) VALUES (?,?,?,?,?)`, id, nodeID, NodeRemovalKind, string(raw), time.Now().Unix())
	if err != nil {
		return err
	}
	return tx.Commit()
}

// ClaimNodeRemoval refuses expired/finished work before any node-side change.
func (s *Store) ClaimNodeRemoval(ctx context.Context, nodeID int64, id string, at time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	j, err := scanNodeJob(tx.QueryRowContext(ctx, `SELECT id,node_id,kind,params_json,result_json,error,created_at,done_at FROM node_jobs WHERE node_id = ? AND id = ?`, nodeID, id))
	if err != nil {
		return wrapNotFound(err)
	}
	var p NodeRemovalParams
	if j.Kind != NodeRemovalKind || j.DoneAt != nil || json.Unmarshal(j.Params, &p) != nil || at.Unix() >= p.ExpiresAt {
		return errors.New("removal request expired or finished")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE node_jobs SET result_json = ? WHERE node_id = ? AND id = ?`, `{"phase":"running"}`, nodeID, id); err != nil {
		return err
	}
	return tx.Commit()
}
