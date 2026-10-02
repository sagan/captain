package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// DashboardNodes uses the node-list's three-minute contact threshold. An
// unpaired node is a setup task, not an offline node. Doctor failures refer to
// the last reported self-check, not a new probe performed by the dashboard.
type DashboardNodes struct {
	Offline    int `json:"offline"`
	Unpaired   int `json:"unpaired"`
	DoctorFail int `json:"doctor_fail"`
}

func (s *Store) DashboardNodes(ctx context.Context, at time.Time) (*DashboardNodes, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT token_hash, last_seen_at, doctor_json FROM nodes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &DashboardNodes{}
	for rows.Next() {
		var token sql.NullString
		var seen sql.NullInt64
		var doctor string
		if err := rows.Scan(&token, &seen, &doctor); err != nil {
			return nil, err
		}
		if !token.Valid || token.String == "" {
			out.Unpaired++
			continue
		}
		if !seen.Valid || at.Sub(time.Unix(seen.Int64, 0)) >= 3*time.Minute {
			out.Offline++
		}
		var report struct {
			Summary struct{ Fail int } `json:"summary"`
		}
		if doctor != "" {
			if err := json.Unmarshal([]byte(doctor), &report); err != nil {
				return nil, err
			}
			if report.Summary.Fail > 0 {
				out.DoctorFail++
			}
		}
	}
	return out, rows.Err()
}

func (s *Store) OpenMonitorIncidents(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM monitor_incidents WHERE ended_at IS NULL`).Scan(&count)
	return count, err
}

// Due assets follow each asset's reminder window, including overdue assets.
// Compare UTC calendar days, exactly as the infrastructure page does.
func (s *Store) DueInfraAssets(ctx context.Context, at time.Time) (int, error) {
	day := at.UTC().Truncate(24 * time.Hour)
	rows, err := s.db.QueryContext(ctx, `SELECT next_due, remind_days FROM infra_assets WHERE active = 1 AND next_due <= ?`, day.AddDate(0, 0, 90).Format("2006-01-02"))
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var due string
		var days int
		if err := rows.Scan(&due, &days); err != nil {
			return 0, err
		}
		date, err := time.Parse("2006-01-02", due)
		if err != nil {
			return 0, err
		}
		if !date.After(day.AddDate(0, 0, days)) {
			count++
		}
	}
	return count, rows.Err()
}
