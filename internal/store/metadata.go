package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode"
)

var ErrMetadata = errors.New("metadata must have at most 32 fields, 80-character keys and 2048-character values (16 KiB total)")

func ValidateMetadata(m map[string]string) error {
	if len(m) > 32 {
		return ErrMetadata
	}
	for k, v := range m {
		if strings.TrimSpace(k) == "" || len(k) > 80 || len(v) > 2048 {
			return ErrMetadata
		}
		for _, r := range k {
			if unicode.IsControl(r) {
				return ErrMetadata
			}
		}
	}
	b, _ := json.Marshal(m)
	if len(b) > 16384 {
		return ErrMetadata
	}
	return nil
}
func metadataTable(kind string) (string, error) {
	switch kind {
	case "users", "nodes":
		return kind, nil
	}
	return "", ErrMetadata
}
func (s *Store) Metadata(ctx context.Context, kind string, id int64) (map[string]string, error) {
	table, err := metadataTable(kind)
	if err != nil {
		return nil, err
	}
	var b string
	if err = s.db.QueryRowContext(ctx, `SELECT metadata_json FROM `+table+` WHERE id=?`, id).Scan(&b); err != nil {
		return nil, wrapNotFound(err)
	}
	m := map[string]string{}
	if err = json.Unmarshal([]byte(b), &m); err != nil {
		return nil, err
	}
	return m, nil
}
func (s *Store) SetMetadata(ctx context.Context, kind string, id int64, m map[string]string) error {
	table, err := metadataTable(kind)
	if err != nil {
		return err
	}
	if err = ValidateMetadata(m); err != nil {
		return err
	}
	if m == nil {
		m = map[string]string{}
	}
	b, _ := json.Marshal(m)
	res, err := s.db.ExecContext(ctx, `UPDATE `+table+` SET metadata_json=? WHERE id=?`, string(b), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
