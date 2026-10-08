package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/zeptop-dev/bosun/pkg/spec"
)

var ErrCoreOperationPending = errors.New("a core operation is already pending")

func (s *Store) SetCoreInventory(ctx context.Context, id int64, inventory *spec.CoreInventory) error {
	raw, err := json.Marshal(inventory)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE nodes SET core_inventory_json = ? WHERE id = ?`, string(raw), id)
	return err
}

func (s *Store) QueueCoreOperation(ctx context.Context, id string, nodeID int64, req spec.CoreRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	if _, err = tx.ExecContext(ctx, `UPDATE node_jobs SET done_at = ?, error = 'core operation expired' WHERE node_id = ? AND kind = ? AND done_at IS NULL AND created_at <= ?`, now, nodeID, spec.CoreManagementKind, now-spec.CoreManagementTTL); err != nil {
		return err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM node_jobs WHERE node_id = ? AND kind = ? AND done_at IS NULL`, nodeID, spec.CoreManagementKind).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return ErrCoreOperationPending
	}
	raw, err := json.Marshal(spec.CoreJobParams{CoreRequest: req, ExpiresAt: now + spec.CoreManagementTTL})
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO node_jobs (id,node_id,kind,params_json,created_at) VALUES (?,?,?,?,?)`, id, nodeID, spec.CoreManagementKind, string(raw), now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) expireCoreOperations(ctx context.Context, nodeID int64) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `UPDATE node_jobs SET done_at = ?, error = 'core operation expired' WHERE node_id = ? AND kind = ? AND done_at IS NULL AND created_at <= ?`, now, nodeID, spec.CoreManagementKind, now-spec.CoreManagementTTL)
	return err
}
