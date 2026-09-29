package store

import (
	"context"
	"errors"

	"github.com/zeptop-dev/captain/internal/domain"
)

const staffCols = "id, email, password_hash, role, status, created_at, updated_at"

// Staff is represented as an authenticated principal by domain.User, but has
// no customer credentials or subscription. Always resolve its ID via staff.
func scanStaff(row interface{ Scan(...any) error }) (*domain.User, error) {
	var u domain.User
	var created, updated int64
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.Status, &created, &updated); err != nil {
		return nil, wrapNotFound(err)
	}
	u.CreatedAt, u.UpdatedAt = unix(created), unix(updated)
	return &u, nil
}

func (s *Store) CreateStaff(ctx context.Context, u *domain.User) error {
	if !domain.ValidStaffRole(u.Role) {
		return errors.New("invalid staff role")
	}
	ts := now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO staff (email, password_hash, role, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`, u.Email, u.PasswordHash, u.Role, u.Status, ts, ts)
	if err != nil {
		return err
	}
	u.ID, err = res.LastInsertId()
	if err != nil {
		return err
	}
	if u.ID > MaxAccountID {
		return ErrAccountID
	}
	u.CreatedAt, u.UpdatedAt = unix(ts), unix(ts)
	u.UUID, u.SubToken, u.InviteCode = "", "", ""
	return tx.Commit()
}

func (s *Store) StaffByID(ctx context.Context, id int64) (*domain.User, error) {
	return scanStaff(s.db.QueryRowContext(ctx, `SELECT `+staffCols+` FROM staff WHERE id = ?`, id))
}
func (s *Store) StaffByEmail(ctx context.Context, email string) (*domain.User, error) {
	return scanStaff(s.db.QueryRowContext(ctx, `SELECT `+staffCols+` FROM staff WHERE email = ?`, email))
}
func (s *Store) UpdateStaffEmail(ctx context.Context, id int64, email string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE staff SET email = ?, updated_at = ? WHERE id = ?`, email, now(), id)
	return err
}
func (s *Store) DeleteStaffSessions(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE staff_id = ?`, id)
	return err
}

// UpdateStaff changes a console account, never the customer with the same ID.
func (s *Store) UpdateStaff(ctx context.Context, id int64, status string, passwordHash string) error {
	if passwordHash != "" {
		if _, err := s.db.ExecContext(ctx, `UPDATE staff SET password_hash = ?, updated_at = ? WHERE id = ?`, passwordHash, now(), id); err != nil {
			return err
		}
	}
	res, err := s.db.ExecContext(ctx, `UPDATE staff SET status = ?, updated_at = ? WHERE id = ?`, status, now(), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListStaff returns console accounts (every role but "user").
func (s *Store) ListStaff(ctx context.Context) ([]*domain.User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+staffCols+` FROM staff WHERE role != 'user' ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.User
	for rows.Next() {
		u, err := scanStaff(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// SetRole changes a staff role; it cannot promote a customer.
func (s *Store) SetRole(ctx context.Context, id int64, role string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE staff SET role = ?, updated_at = ? WHERE id = ?`, role, now(), id)
	return err
}

// CountAdmins returns how many full admins exist (the last one cannot go).
func (s *Store) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM staff WHERE role = 'admin' AND status = 'active'`).Scan(&n)
	return n, err
}

// DeleteStaff removes a console account (never a plain user).
func (s *Store) DeleteStaff(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM staff WHERE id = ? AND role != 'user'`, id)
	return err
}
