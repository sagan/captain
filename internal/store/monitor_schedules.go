package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode"
)

type MonitorWindow struct {
	ID         int64  `json:"id"`
	NodeID     *int64 `json:"node_id"` // nil applies to all nodes
	Kind       string `json:"kind"`
	StartsAt   int64  `json:"starts_at"`
	EndsAt     int64  `json:"ends_at"`
	Note       string `json:"note"`
	CreatedAt  int64  `json:"created_at"`
	CanceledAt *int64 `json:"canceled_at"`
}

var ErrMonitorWindow = errors.New("invalid monitoring window")
var ErrMonitorWindowLimit = errors.New("too many active or scheduled windows")

func (s *Store) CreateMonitorWindow(ctx context.Context, w *MonitorWindow, at time.Time) error {
	w.Note = strings.TrimSpace(w.Note)
	if (w.Kind != "maintenance" && w.Kind != "silence") || w.StartsAt < at.Unix()-5 || w.StartsAt > at.Add(365*24*time.Hour).Unix() || w.EndsAt <= w.StartsAt || w.EndsAt-w.StartsAt > int64(30*24*time.Hour/time.Second) || len(w.Note) > 512 || strings.IndexFunc(w.Note, unicode.IsControl) >= 0 || (w.NodeID != nil && *w.NodeID <= 0) {
		return ErrMonitorWindow
	}
	// Never permit backdated maintenance to rewrite previously observed downtime.
	w.StartsAt = max(w.StartsAt, at.Unix())
	if w.EndsAt <= w.StartsAt {
		return ErrMonitorWindow
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM monitor_windows WHERE canceled_at IS NULL AND ends_at>?`, at.Unix()).Scan(&count); err != nil {
		return err
	}
	if count >= 200 {
		return ErrMonitorWindowLimit
	}
	if w.NodeID != nil {
		if err = tx.QueryRowContext(ctx, `SELECT 1 FROM nodes WHERE id=?`, *w.NodeID).Scan(&count); err != nil {
			return wrapNotFound(err)
		}
	}
	w.CreatedAt = at.Unix()
	w.CanceledAt = nil
	r, err := tx.ExecContext(ctx, `INSERT INTO monitor_windows(node_id,kind,starts_at,ends_at,note,created_at) VALUES(?,?,?,?,?,?)`, w.NodeID, w.Kind, w.StartsAt, w.EndsAt, w.Note, w.CreatedAt)
	if err != nil {
		return err
	}
	w.ID, _ = r.LastInsertId()
	return tx.Commit()
}
func (s *Store) CancelMonitorWindow(ctx context.Context, id int64, at time.Time) error {
	r, err := s.db.ExecContext(ctx, `UPDATE monitor_windows SET canceled_at=? WHERE id=? AND canceled_at IS NULL AND ends_at>?`, at.Unix(), id, at.Unix())
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Store) MonitorWindows(ctx context.Context, from, to time.Time) ([]MonitorWindow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,node_id,kind,starts_at,ends_at,note,created_at,canceled_at FROM monitor_windows WHERE ends_at>? AND starts_at<? AND (canceled_at IS NULL OR canceled_at>?) ORDER BY starts_at DESC,id DESC`, from.Unix(), to.Unix(), from.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MonitorWindow{}
	for rows.Next() {
		var w MonitorWindow
		if err = rows.Scan(&w.ID, &w.NodeID, &w.Kind, &w.StartsAt, &w.EndsAt, &w.Note, &w.CreatedAt, &w.CanceledAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

type monitorQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func monitorMuted(ctx context.Context, q monitorQuerier, nodeID, at int64) (bool, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM monitor_windows WHERE (node_id IS NULL OR node_id=?) AND starts_at<=? AND ends_at>? AND (canceled_at IS NULL OR canceled_at>?)`, nodeID, at, at, at).Scan(&n)
	return n > 0, err
}
func (s *Store) MonitorMuted(ctx context.Context, nodeID int64, at time.Time) (bool, error) {
	return monitorMuted(ctx, s.db, nodeID, at.Unix())
}
