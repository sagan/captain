package store

import (
	"errors"
	"strings"
	"time"
)

// UserFilter applies to customer accounts only. Subscription filters match any
// active row, while the list still displays the primary subscription summary.
type UserFilter struct {
	Query, Status, Access, Sort string
	GroupID                     *int64
	PlanID                      int64
	ExpiresFrom, ExpiresTo      int64
	Desc                        bool
}

func (f UserFilter) Validate() error {
	if len(f.Query) > 320 || (f.Status != "" && f.Status != "active" && f.Status != "banned") || (f.Access != "" && f.Access != "usable" && f.Access != "expired" && f.Access != "exhausted" && f.Access != "none") || f.PlanID < 0 || (f.GroupID != nil && *f.GroupID < 0) || f.ExpiresFrom < 0 || f.ExpiresTo < 0 || (f.ExpiresTo > 0 && f.ExpiresFrom > f.ExpiresTo) {
		return errors.New("invalid user filter")
	}
	switch f.Sort {
	case "", "id", "email", "balance", "created", "expires", "usage":
	default:
		return errors.New("invalid user sort")
	}
	return nil
}

func (f UserFilter) query(at time.Time) (string, []any, string) {
	parts := []string{"u.role = 'user'"}
	args := []any{}
	add := func(q string, a ...any) { parts = append(parts, q); args = append(args, a...) }
	if f.Query != "" {
		add("u.email LIKE ?", "%"+f.Query+"%")
	}
	if f.Status != "" {
		add("u.status = ?", f.Status)
	}
	if f.GroupID != nil {
		if *f.GroupID == 0 {
			add("u.group_id IS NULL")
		} else {
			add("u.group_id = ?", *f.GroupID)
		}
	}
	base := "SELECT 1 FROM subscriptions f WHERE f.user_id = u.id AND f.status = 'active'"
	if f.PlanID > 0 {
		add("EXISTS ("+base+" AND f.plan_id = ?)", f.PlanID)
	}
	fresh := " AND (f.expires_at IS NULL OR f.expires_at > ?)"
	usable := fresh + " AND (f.quota_bytes = 0 OR f.used_up_bytes + f.used_down_bytes < f.quota_bytes)"
	switch f.Access {
	case "usable":
		add("EXISTS ("+base+usable+")", at.Unix())
	case "expired":
		add("EXISTS ("+base+") AND NOT EXISTS ("+base+fresh+")", at.Unix())
	case "exhausted":
		add("EXISTS ("+base+fresh+") AND NOT EXISTS ("+base+usable+")", at.Unix(), at.Unix())
	case "none":
		add("NOT EXISTS (" + base + ")")
	}
	if f.ExpiresFrom > 0 || f.ExpiresTo > 0 {
		q := base + " AND f.expires_at IS NOT NULL"
		if f.ExpiresFrom > 0 {
			q += " AND f.expires_at >= ?"
			args = append(args, f.ExpiresFrom)
		}
		if f.ExpiresTo > 0 {
			q += " AND f.expires_at <= ?"
			args = append(args, f.ExpiresTo)
		}
		parts = append(parts, "EXISTS ("+q+")")
	}
	sort := map[string]string{"": "u.id", "id": "u.id", "email": "u.email", "balance": "u.balance_cents", "created": "u.created_at", "expires": "sub.expires_at", "usage": "sub.used_up_bytes + sub.used_down_bytes"}[f.Sort]
	direction := " ASC"
	if f.Desc {
		direction = " DESC"
	}
	return "WHERE " + strings.Join(parts, " AND "), args, sort + direction + ", u.id" + direction
}
