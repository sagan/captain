package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// StaffByIdentity finds the staff account linked to a provider subject.
func (s *Store) StaffByIdentity(ctx context.Context, provider, subject string) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT user_id FROM staff_identities WHERE provider = ? AND subject = ?`, provider, subject).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

// LinkStaffIdentity attaches a provider subject to a staff account (replacing an older
// link of the same provider for that user).
func (s *Store) LinkStaffIdentity(ctx context.Context, userID int64, provider, subject, email string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM staff_identities WHERE user_id = ? AND provider = ?`, userID, provider); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO staff_identities (provider, subject, user_id, email, created_at) VALUES (?, ?, ?, ?, ?)`, provider, subject, userID, email, now())
	return err
}

// UnlinkStaffIdentity removes a provider link from a user.
func (s *Store) UnlinkStaffIdentity(ctx context.Context, userID int64, provider string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM staff_identities WHERE user_id = ? AND provider = ?`, userID, provider)
	return err
}

// IdentitiesByStaff lists a staff account's linked logins.
func (s *Store) IdentitiesByStaff(ctx context.Context, userID int64) ([]Identity, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT provider, subject, user_id, email, created_at FROM staff_identities WHERE user_id = ? ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Identity{}
	for rows.Next() {
		var i Identity
		var created int64
		if err := rows.Scan(&i.Provider, &i.Subject, &i.UserID, &i.Email, &created); err != nil {
			return nil, err
		}
		i.CreatedAt = time.Unix(created, 0)
		out = append(out, i)
	}
	return out, rows.Err()
}
