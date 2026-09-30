package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

var ErrPasskeyLimit = errors.New("passkey or challenge limit reached")

type Passkey struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	CreatedAt  time.Time           `json:"created_at"`
	LastUsedAt *time.Time          `json:"last_used_at"`
	Credential webauthn.Credential `json:"-"`
	Stored     string              `json:"-"`
}

func (s *Store) PasskeyHandle(ctx context.Context, staffID int64, rp string) ([]byte, error) {
	handle := make([]byte, 32)
	if _, err := rand.Read(handle); err != nil {
		return nil, err
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO staff_passkey_users(staff_id,rp_id,handle) VALUES(?,?,?) ON CONFLICT(staff_id,rp_id) DO NOTHING`, staffID, rp, handle); err != nil {
		return nil, err
	}
	err := s.db.QueryRowContext(ctx, `SELECT handle FROM staff_passkey_users WHERE staff_id=? AND rp_id=?`, staffID, rp).Scan(&handle)
	return handle, err
}
func (s *Store) Passkeys(ctx context.Context, staffID int64, rp string) ([]Passkey, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT credential_id,name,credential_json,created_at,last_used_at FROM staff_passkeys WHERE staff_id=? AND rp_id=? ORDER BY created_at`, staffID, rp)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Passkey{}
	for rows.Next() {
		var p Passkey
		var created int64
		var used sql.NullInt64
		if err = rows.Scan(&p.ID, &p.Name, &p.Stored, &created, &used); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(p.Stored), &p.Credential); err != nil {
			return nil, err
		}
		p.CreatedAt = unix(created)
		p.LastUsedAt = unixPtr(used)
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Store) PasskeyOwner(ctx context.Context, id []byte, handle []byte, rp string) (int64, error) {
	var owner int64
	err := s.db.QueryRowContext(ctx, `SELECT p.staff_id FROM staff_passkeys p JOIN staff_passkey_users u ON u.staff_id=p.staff_id AND u.rp_id=p.rp_id WHERE p.credential_id=? AND p.rp_id=? AND u.handle=?`, base64.RawURLEncoding.EncodeToString(id), rp, handle).Scan(&owner)
	return owner, wrapNotFound(err)
}
func (s *Store) AddPasskey(ctx context.Context, staffID int64, rp, name string, c *webauthn.Credential) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM staff_passkeys WHERE staff_id=?`, staffID).Scan(&count); err != nil {
		return err
	}
	if count >= 10 {
		return ErrPasskeyLimit
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO staff_passkeys(credential_id,staff_id,rp_id,name,credential_json,created_at) VALUES(?,?,?,?,?,?)`, base64.RawURLEncoding.EncodeToString(c.ID), staffID, rp, name, string(b), now())
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) UpdatePasskey(ctx context.Context, staffID int64, rp string, c *webauthn.Credential, old string) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE staff_passkeys SET credential_json=?,last_used_at=? WHERE credential_id=? AND staff_id=? AND rp_id=? AND credential_json=?`, string(b), now(), base64.RawURLEncoding.EncodeToString(c.ID), staffID, rp, old)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return ErrNotFound
	}
	return nil
}
func (s *Store) DeletePasskey(ctx context.Context, staffID int64, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM staff_passkeys WHERE credential_id=? AND staff_id=?`, id, staffID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return ErrNotFound
	}
	return nil
}
func (s *Store) SavePasskeyChallenge(ctx context.Context, id, kind, binding, name string, staffID *int64, session *webauthn.SessionData) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM passkey_challenges WHERE expires_at<=?`, now()); err != nil {
		return err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM passkey_challenges`).Scan(&count); err != nil {
		return err
	}
	if count >= 2048 {
		return ErrPasskeyLimit
	}
	b, err := json.Marshal(session)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO passkey_challenges(id,staff_id,kind,binding,session_json,name,expires_at) VALUES(?,?,?,?,?,?,?)`, id, nullInt64(staffID), kind, binding, string(b), name, time.Now().Add(5*time.Minute).Unix())
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Consume deletes before cryptographic validation. A failed ceremony also uses
// its challenge; a replay cannot reuse it, including across process restarts.
func (s *Store) ConsumePasskeyChallenge(ctx context.Context, id, kind, binding string, staffID int64) (*webauthn.SessionData, string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback()
	var b, name string
	err = tx.QueryRowContext(ctx, `SELECT session_json,name FROM passkey_challenges WHERE id=? AND kind=? AND binding=? AND COALESCE(staff_id,0)=? AND expires_at>?`, id, kind, binding, staffID, now()).Scan(&b, &name)
	if err != nil {
		return nil, "", wrapNotFound(err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM passkey_challenges WHERE id=?`, id); err != nil {
		return nil, "", err
	}
	var session webauthn.SessionData
	if err = json.Unmarshal([]byte(b), &session); err != nil {
		return nil, "", err
	}
	if err = tx.Commit(); err != nil {
		return nil, "", err
	}
	return &session, name, nil
}
