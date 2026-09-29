package db_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/zeptop-dev/captain/internal/auth"
	"github.com/zeptop-dev/captain/internal/db"
	"github.com/zeptop-dev/captain/internal/domain"
	"github.com/zeptop-dev/captain/internal/store"
	"github.com/zeptop-dev/captain/migrations"
)

func TestStaffMigration(t *testing.T) {
	for _, history := range []bool{false, true} {
		name := "console_only"
		if history {
			name = "legacy_customer_history"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			conn, err := db.Open("sqlite", filepath.Join(t.TempDir(), "c.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			goose.SetBaseFS(migrations.FS)
			goose.SetLogger(goose.NopLogger())
			if err := goose.SetDialect("sqlite"); err != nil {
				t.Fatal(err)
			}
			if err := goose.UpToContext(ctx, conn, ".", 51); err != nil {
				t.Fatal(err)
			}
			exec := func(q string, args ...any) {
				t.Helper()
				if _, err := conn.ExecContext(ctx, q, args...); err != nil {
					t.Fatal(err)
				}
			}
			expires := time.Now().Add(time.Hour).Unix()
			exec(`INSERT INTO users (id,email,password_hash,role,uuid,sub_token,status,created_at,updated_at,totp_secret,totp_enabled)
                VALUES (1,'admin@example.com','old-hash','admin','old-uuid','old-sub','active',1,1,'totp-secret',1)`)
			exec(`INSERT INTO sessions (id,user_id,expires_at,created_at,admin) VALUES ('console',1,?,1,1),('old-portal',1,?,1,0)`, expires, expires)
			exec(`INSERT INTO api_tokens (user_id,name,token_hash,created_at,scope,expires_at) VALUES (1,'read',?,1,'read',?)`, auth.SHA256Hex("cap_test"), expires)
			exec(`INSERT INTO identities VALUES ('idp','subject',1,'admin@example.com',1)`)
			exec(`INSERT INTO admin_log (at,user_id,email,role,method,path) VALUES (1,1,'admin@example.com','admin','POST','/api/admin/groups')`)
			if history {
				exec(`INSERT INTO plans (id,name,price_cents,period_days,quota_bytes,enabled,created_at,updated_at) VALUES (1,'test',0,30,0,1,1,1)`)
				exec(`INSERT INTO orders (id,no,user_id,plan_id,amount_cents,gateway,status,created_at) VALUES (1,'order',1,1,0,'balance','paid',1)`)
				exec(`INSERT INTO subscriptions (user_id,plan_id,starts_at,quota_bytes,created_at,updated_at) VALUES (1,1,1,0,1,1)`)
				// Existing customer IDs and credentials must survive unchanged.
				exec(`INSERT INTO users (id,email,password_hash,role,uuid,sub_token,status,created_at,updated_at) VALUES (4,'user@example.com','customer-hash','user','customer-uuid','customer-sub','active',1,1)`)
				exec(`INSERT INTO sessions (id,user_id,expires_at,created_at,admin) VALUES ('customer',4,?,1,0)`, expires)
				exec(`INSERT INTO identities VALUES ('idp','customer-subject',4,'user@example.com',1)`)
			}
			if err := db.Migrate(ctx, conn, "sqlite"); err != nil {
				t.Fatal(err)
			}
			if err := db.Migrate(ctx, conn, "sqlite"); err != nil {
				t.Fatal("second startup:", err)
			}
			st := store.New(conn)
			staff, err := st.StaffByID(ctx, 1)
			if err != nil || staff.PasswordHash != "old-hash" || staff.SubToken != "" || staff.UUID != "" {
				t.Fatalf("staff: %+v %v", staff, err)
			}
			if secret, on, err := st.TOTP(ctx, 1); err != nil || !on || secret != "totp-secret" {
				t.Fatalf("totp lost: %v", err)
			}
			if u, admin, err := st.SessionUser(ctx, "console", time.Now()); err != nil || !admin || !u.IsAdmin() {
				t.Fatalf("console session: %v", err)
			}
			if _, _, err := st.SessionUser(ctx, "old-portal", time.Now()); !errors.Is(err, store.ErrNotFound) {
				t.Fatalf("staff portal session survived: %v", err)
			}
			if u, scope, err := st.UserByAPIToken(ctx, "cap_test"); err != nil || !u.IsAdmin() || scope != "read" {
				t.Fatalf("token: %v", err)
			}
			if id, err := st.StaffByIdentity(ctx, "idp", "subject"); err != nil || id != 1 {
				t.Fatalf("staff identity: %d %v", id, err)
			}
			if _, err := st.UserByIdentity(ctx, "idp", "subject"); !errors.Is(err, store.ErrNotFound) {
				t.Fatal("staff identity still in customer namespace")
			}
			if history {
				u, err := st.UserByID(ctx, 1)
				if err != nil || u.Status != "banned" || u.PasswordHash != "" || u.Role != "user" {
					t.Fatalf("legacy history login active: %+v %v", u, err)
				}
				var count int
				if err := conn.QueryRow(`SELECT COUNT(*) FROM orders WHERE user_id=1`).Scan(&count); err != nil || count != 1 {
					t.Fatal("lost order", err)
				}
				u, err = st.UserByID(ctx, 4)
				if err != nil || u.UUID != "customer-uuid" || u.SubToken != "customer-sub" {
					t.Fatal("customer changed", err)
				}
				if u, staff, err := st.SessionUser(ctx, "customer", time.Now()); err != nil || staff || u.ID != 4 {
					t.Fatal("customer session lost", err)
				}
			} else {
				if _, err := st.UserByID(ctx, 1); !errors.Is(err, store.ErrNotFound) {
					t.Fatal("console still occupies customer ID")
				}
				u := &domain.User{Email: "user@example.com", UUID: "customer-uuid", SubToken: "customer-sub", Status: "active"}
				if err := st.CreateUser(ctx, u); err != nil || u.ID != 1 {
					t.Fatalf("first customer: %d %v", u.ID, err)
				}
			}
			rows, err := conn.Query(`PRAGMA foreign_key_check`)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			if rows.Next() {
				t.Fatal("broken foreign key after migration")
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			var id sql.NullInt64
			if err := conn.QueryRow(`SELECT user_id FROM admin_log LIMIT 1`).Scan(&id); err != nil || id.Int64 != 1 {
				t.Fatal("audit actor changed", err)
			}
		})
	}
}
