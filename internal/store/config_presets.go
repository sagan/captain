package store

import (
	"context"
	"fmt"
	"github.com/zeptop-dev/bosun/pkg/spec"
)

func (s *Store) ConfigPresets(ctx context.Context) ([]spec.ConfigPreset, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,kind,payload_json FROM config_presets ORDER BY name,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []spec.ConfigPreset{}
	for rows.Next() {
		var p spec.ConfigPreset
		var raw string
		if err := rows.Scan(&p.ID, &p.Name, &p.Kind, &raw); err != nil {
			return nil, err
		}
		p.Payload = []byte(raw)
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Store) SaveConfigPreset(ctx context.Context, p *spec.ConfigPreset) error {
	if err := p.Normalize(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if p.ID == 0 {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM config_presets`).Scan(&n); err != nil {
			return err
		}
		if n >= 200 {
			return fmt.Errorf("at most 200 presets")
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO config_presets(name,kind,payload_json) VALUES(?,?,?)`, p.Name, p.Kind, string(p.Payload))
		if err != nil {
			return err
		}
		p.ID, _ = res.LastInsertId()
	} else {
		res, err := tx.ExecContext(ctx, `UPDATE config_presets SET name=?,kind=?,payload_json=? WHERE id=?`, p.Name, p.Kind, string(p.Payload), p.ID)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return ErrNotFound
		}
	}
	return tx.Commit()
}
func (s *Store) DeleteConfigPreset(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM config_presets WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
