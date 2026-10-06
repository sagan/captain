package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/zeptop-dev/captain/internal/db"
	"github.com/zeptop-dev/captain/internal/domain"
	"github.com/zeptop-dev/captain/internal/store"
	"github.com/zeptop-dev/captain/migrations"
)

func TestCustomerIDSequenceMigration(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open("sqlite", filepath.Join(t.TempDir(), "captain.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite"); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpToContext(ctx, conn, ".", 64); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO users (id,email,password_hash,uuid,sub_token,agent_id,created_at,updated_at)
		VALUES (42,'existing@example.com','hash','original-uuid','original-sub',17,1,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`UPDATE account_sequences SET value = 17 WHERE name = 'agent'`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := db.Migrate(ctx, conn, "sqlite"); err != nil {
			t.Fatal(err)
		}
	}
	s := store.New(conn)
	existing, err := s.UserByID(ctx, 42)
	if err != nil || existing.UUID != "original-uuid" || existing.SubToken != "original-sub" || existing.AgentID != 17 {
		t.Fatal("migration changed existing identity", err)
	}
	manual := &domain.User{Email: "manual@example.com", UUID: "manual", SubToken: "manual"}
	if err := s.CreateUserWithID(ctx, manual, 999999); err != nil {
		t.Fatal(err)
	}
	auto := &domain.User{Email: "auto@example.com", UUID: "auto", SubToken: "auto"}
	if err := s.CreateUser(ctx, auto); err != nil || auto.ID != 43 || auto.AgentID != 19 {
		t.Fatal("numbering did not continue from old database", auto.ID, auto.AgentID, err)
	}
}
