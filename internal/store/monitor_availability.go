package store

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"
)

// ObserveAvailability records only bounded, continuously observed intervals.
// A different process epoch, disabled collection, or a scheduler gap over two
// minutes creates an unknown gap, never inferred uptime or downtime.
func (s *Store) ObserveAvailability(ctx context.Context, nodeID int64, epoch string, at, lastSeen time.Time, grace time.Duration) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var oldEpoch string
	var previous, seen int64
	err = tx.QueryRowContext(ctx, `SELECT epoch,observed_at,last_seen FROM monitor_observations WHERE node_id=?`, nodeID).Scan(&oldEpoch, &previous, &seen)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	end := at.Unix()
	if end <= previous {
		return nil
	}
	last := min(end, lastSeen.Unix())
	if oldEpoch == epoch && end-previous <= 120 {
		last = max(last, seen)
		expiry := seen + int64(grace/time.Second)
		newExpiry := last + int64(grace/time.Second)
		cuts := []int64{previous, end}
		for _, cut := range []int64{expiry, last, newExpiry} {
			if cut > previous && cut < end {
				cuts = append(cuts, cut)
			}
		}
		sort.Slice(cuts, func(i, j int) bool { return cuts[i] < cuts[j] })
		for i := 1; i < len(cuts); i++ {
			a, b := cuts[i-1], cuts[i]
			if b <= a {
				continue
			}
			online := a < expiry || (a >= last && a < newExpiry)
			if err = appendMonitorInterval(ctx, tx, nodeID, a, b, online); err != nil {
				return err
			}
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO monitor_observations(node_id,epoch,observed_at,last_seen) VALUES(?,?,?,?) ON CONFLICT(node_id) DO UPDATE SET epoch=excluded.epoch,observed_at=excluded.observed_at,last_seen=excluded.last_seen`, nodeID, epoch, end, last); err != nil {
		return err
	}
	return tx.Commit()
}
func appendMonitorInterval(ctx context.Context, tx *sql.Tx, nodeID, from, to int64, online bool) error {
	var id, end int64
	var wasOnline bool
	err := tx.QueryRowContext(ctx, `SELECT id,ends_at,online FROM monitor_intervals WHERE node_id=? ORDER BY ends_at DESC,id DESC LIMIT 1`, nodeID).Scan(&id, &end, &wasOnline)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if id != 0 && end == from && wasOnline == online {
		_, err = tx.ExecContext(ctx, `UPDATE monitor_intervals SET ends_at=? WHERE id=?`, to, id)
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO monitor_intervals(node_id,starts_at,ends_at,online) VALUES(?,?,?,?)`, nodeID, from, to, online)
	}
	return err
}

type AvailabilitySummary struct {
	Online      int64    `json:"online_seconds"`
	Offline     int64    `json:"offline_seconds"`
	Unknown     int64    `json:"unknown_seconds"`
	Maintenance int64    `json:"maintenance_seconds"`
	Percent     *float64 `json:"percent"`
	Coverage    *float64 `json:"coverage_percent"`
}
type AvailabilityBucket struct {
	AvailabilitySummary
	From int64 `json:"from"`
	To   int64 `json:"to"`
}
type Availability struct {
	AvailabilitySummary
	From    int64                `json:"from"`
	To      int64                `json:"to"`
	Buckets []AvailabilityBucket `json:"buckets"`
}
type monitorSpan struct {
	from, to int64
	online   bool
}

// Availability excludes the union of maintenance intervals from both numerator
// and denominator. Silence never changes accounting. Unknown time is shown
// separately, so a high availability with low coverage cannot imply an SLA.
func (s *Store) Availability(ctx context.Context, nodeID int64, from, to time.Time) (Availability, error) {
	out := Availability{From: from.Unix(), To: to.Unix(), Buckets: []AvailabilityBucket{}}
	if !to.After(from) || to.Sub(from) > 90*24*time.Hour {
		return out, ErrMonitorWindow
	}
	rows, err := s.db.QueryContext(ctx, `SELECT starts_at,ends_at,online FROM monitor_intervals WHERE node_id=? AND starts_at<? AND ends_at>? ORDER BY starts_at`, nodeID, out.To, out.From)
	if err != nil {
		return out, err
	}
	intervals := []monitorSpan{}
	for rows.Next() {
		var v monitorSpan
		if err = rows.Scan(&v.from, &v.to, &v.online); err != nil {
			rows.Close()
			return out, err
		}
		intervals = append(intervals, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	windows, err := s.MonitorWindows(ctx, from, to)
	if err != nil {
		return out, err
	}
	maintenance := []monitorSpan{}
	for _, w := range windows {
		if w.Kind != "maintenance" || (w.NodeID != nil && *w.NodeID != nodeID) {
			continue
		}
		end := w.EndsAt
		if w.CanceledAt != nil {
			end = min(end, *w.CanceledAt)
		}
		a, b := max(out.From, w.StartsAt), min(out.To, end)
		if b > a {
			maintenance = append(maintenance, monitorSpan{from: a, to: b})
		}
	}
	sort.Slice(maintenance, func(i, j int) bool { return maintenance[i].from < maintenance[j].from })
	merged := []monitorSpan{}
	for _, v := range maintenance {
		if len(merged) > 0 && v.from <= merged[len(merged)-1].to {
			merged[len(merged)-1].to = max(merged[len(merged)-1].to, v.to)
		} else {
			merged = append(merged, v)
		}
	}
	step := int64(86400)
	if to.Sub(from) <= time.Hour {
		step = 60
	} else if to.Sub(from) <= 24*time.Hour {
		step = 3600
	}
	nextInterval := 0
	for a := out.From; a < out.To; {
		b := min(out.To, (a/step+1)*step)
		bucket := AvailabilityBucket{From: a, To: b}
		bucket.Maintenance = spanOverlap(merged, a, b)
		for nextInterval < len(intervals) && intervals[nextInterval].to <= a {
			nextInterval++
		}
		for _, v := range intervals[nextInterval:] {
			if v.from >= b {
				break
			}
			x, y := max(a, v.from), min(b, v.to)
			if y <= x {
				continue
			}
			seconds := y - x - spanOverlap(merged, x, y)
			if v.online {
				bucket.Online += seconds
			} else {
				bucket.Offline += seconds
			}
		}
		bucket.Unknown = max(0, b-a-bucket.Maintenance-bucket.Online-bucket.Offline)
		bucket.calculate()
		out.Online += bucket.Online
		out.Offline += bucket.Offline
		out.Unknown += bucket.Unknown
		out.Maintenance += bucket.Maintenance
		out.Buckets = append(out.Buckets, bucket)
		a = b
	}
	out.calculate()
	return out, nil
}
func spanOverlap(spans []monitorSpan, a, b int64) int64 {
	var n int64
	for _, v := range spans {
		if v.from >= b {
			break
		}
		n += max(0, min(b, v.to)-max(a, v.from))
	}
	return n
}
func (a *AvailabilitySummary) calculate() {
	known := a.Online + a.Offline
	if known > 0 {
		v := 100 * float64(a.Online) / float64(known)
		a.Percent = &v
	}
	if known+a.Unknown > 0 {
		v := 100 * float64(known) / float64(known+a.Unknown)
		a.Coverage = &v
	}
}
