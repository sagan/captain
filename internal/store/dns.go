package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/zeptop-dev/captain/internal/dns"
)

// DNSChange contains only record data, never provider credentials or raw errors.
// Unknown includes in-flight requests, crashes and ambiguous transport failures.
type DNSChange struct {
	ID         int64       `json:"id"`
	NodeID     *int64      `json:"node_id"`
	Source     string      `json:"source"`
	Actor      string      `json:"actor"`
	Zone       string      `json:"zone"`
	Name       string      `json:"name"`
	Action     string      `json:"action"`
	Before     *dns.Record `json:"before"`
	Wanted     dns.Record  `json:"wanted"`
	After      *dns.Record `json:"after"`
	Outcome    string      `json:"outcome"`
	CreatedAt  int64       `json:"created_at"`
	FinishedAt int64       `json:"finished_at"`
}

func (s *Store) BeginDNSChange(ctx context.Context, c *DNSChange) (err error) {
	before, err := json.Marshal(c.Before)
	if err != nil {
		return err
	}
	wanted, err := json.Marshal(c.Wanted)
	if err != nil {
		return err
	}
	c.CreatedAt, c.Outcome = now(), "unknown"
	// Unlike an ordinary local edit, the next step mutates an external service.
	// WAL NORMAL may lose a committed row after power loss. Reserve the sole
	// connection and sync this intent with FULL before authorizing that step,
	// then restore the caller's connection policy even after cancellation.
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var previous int
	if err = conn.QueryRowContext(ctx, `PRAGMA synchronous`).Scan(&previous); err != nil {
		return err
	}
	if _, err = conn.ExecContext(ctx, `PRAGMA synchronous=FULL`); err != nil {
		return err
	}
	defer func() {
		restoreCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		policies := []string{"PRAGMA synchronous=OFF", "PRAGMA synchronous=NORMAL", "PRAGMA synchronous=FULL", "PRAGMA synchronous=EXTRA"}
		if previous < 0 || previous >= len(policies) {
			previous = 2
		}
		_, restoreErr := conn.ExecContext(restoreCtx, policies[previous])
		if err == nil {
			err = restoreErr
		}
	}()
	r, err := conn.ExecContext(ctx, `INSERT INTO dns_changes(node_id,source,actor,zone,name,action,before_json,wanted_json,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, nullInt64(c.NodeID), c.Source, c.Actor, c.Zone, c.Name, c.Action, string(before), string(wanted), c.CreatedAt)
	if err == nil {
		c.ID, err = r.LastInsertId()
	}
	return err
}

func (s *Store) FinishDNSChange(ctx context.Context, id int64, outcome string, after *dns.Record) error {
	b, err := json.Marshal(after)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE dns_changes SET outcome=?,after_json=?,finished_at=? WHERE id=?`, outcome, string(b), now(), id)
	return err
}

func (s *Store) DNSChanges(ctx context.Context, nodeID, before int64) ([]DNSChange, error) {
	query := `SELECT id,node_id,source,actor,zone,name,action,before_json,wanted_json,after_json,outcome,created_at,finished_at FROM dns_changes WHERE 1=1`
	args := []any{}
	if nodeID > 0 {
		query += ` AND node_id=?`
		args = append(args, nodeID)
	}
	if before > 0 {
		query += ` AND id<?`
		args = append(args, before)
	}
	rows, err := s.db.QueryContext(ctx, query+` ORDER BY id DESC LIMIT 51`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DNSChange{}
	for rows.Next() {
		var c DNSChange
		var old, wanted, after string
		if err := rows.Scan(&c.ID, &c.NodeID, &c.Source, &c.Actor, &c.Zone, &c.Name, &c.Action, &old, &wanted, &after, &c.Outcome, &c.CreatedAt, &c.FinishedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(old), &c.Before); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(wanted), &c.Wanted); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(after), &c.After); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) PruneDNSChanges(ctx context.Context, at time.Time) error {
	// Keep incomplete/unknown outcomes for investigation until normal retention.
	_, err := s.db.ExecContext(ctx, `DELETE FROM dns_changes WHERE created_at<?`, at.AddDate(0, 0, -180).Unix())
	return err
}

type DNSAnswer struct {
	Source    string   `json:"source"`
	Addresses []string `json:"addresses"`
	Error     string   `json:"error,omitempty"` // not_found | timeout | unavailable
}

type DNSTarget struct {
	Key      string      `json:"key"`
	Host     string      `json:"host"`
	Label    string      `json:"label"`
	Mode     string      `json:"mode"` // direct | shared | resolve_only | conflict
	Expected []string    `json:"expected"`
	Status   string      `json:"status"` // ok | mismatch | missing | inconsistent | unknown | conflict | invalid
	Answers  []DNSAnswer `json:"answers"`
}

type DNSHealth struct {
	NodeID        int64       `json:"node_id"`
	Node          string      `json:"node"`
	Fingerprint   string      `json:"-"`
	CheckedAt     int64       `json:"checked_at"`
	Stale         bool        `json:"stale"`
	Targets       []DNSTarget `json:"targets"`
	FailureKey    string      `json:"-"`
	FailureSince  int64       `json:"failure_since"`
	FailureCount  int         `json:"failure_count"`
	LastFailureAt int64       `json:"-"`
}

// Internal fields are persisted separately from the public JSON representation.
type dnsHealthRow struct {
	Health        DNSHealth
	Fingerprint   string
	FailureKey    string
	LastFailureAt int64
}

func (s *Store) DNSHealth(ctx context.Context, id int64) (*DNSHealth, error) {
	var raw string
	if err := s.db.QueryRowContext(ctx, `SELECT snapshot_json FROM node_dns_health WHERE node_id=?`, id).Scan(&raw); err != nil {
		return nil, wrapNotFound(err)
	}
	var r dnsHealthRow
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return nil, err
	}
	r.Health.Fingerprint, r.Health.FailureKey, r.Health.LastFailureAt = r.Fingerprint, r.FailureKey, r.LastFailureAt
	return &r.Health, nil
}
func (s *Store) SaveDNSHealth(ctx context.Context, h *DNSHealth) error {
	raw, err := json.Marshal(dnsHealthRow{*h, h.Fingerprint, h.FailureKey, h.LastFailureAt})
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO node_dns_health(node_id,snapshot_json) VALUES(?,?) ON CONFLICT(node_id) DO UPDATE SET snapshot_json=excluded.snapshot_json`, h.NodeID, string(raw))
	return err
}
