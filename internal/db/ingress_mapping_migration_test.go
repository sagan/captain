package db_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/zeptop-dev/captain/internal/db"
	"github.com/zeptop-dev/captain/internal/domain"
	"github.com/zeptop-dev/captain/internal/store"
	"github.com/zeptop-dev/captain/migrations"
)

func TestIngressMappingMigration(t *testing.T) {
	ctx := context.Background()
	c, err := db.Open("sqlite", filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite"); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpToContext(ctx, c, ".", 65); err != nil {
		t.Fatal(err)
	}
	s := store.New(c)
	n := &domain.Node{Name: "existing"}
	if err := s.CreateNode(ctx, n, "test-pair-code", time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Exec(`INSERT INTO ingresses (node_id,name,kind,bind_ip,line_ip,entry_host,port_from,port_to,port_offset,reserved_ports,created_at,updated_at) VALUES (?, 'legacy', 'mapped', '10.10.0.2', '198.51.100.20', '203.0.113.30', 20000, 20099, 10000, '20000', 1, 1)`, n.ID); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := db.Migrate(ctx, c, "sqlite"); err != nil {
			t.Fatal(err)
		}
	}
	gs, err := s.IngressesByNode(ctx, n.ID)
	if err != nil || len(gs) != 1 {
		t.Fatal(gs, err)
	}
	g := gs[0]
	if g.RequireIngress || len(g.PortMappings) != 0 || g.Kind != "mapped" || g.AllowsPort(20000) || !g.AllowsPort(20001) || g.EntryPort(20001) != 30001 {
		t.Fatal("migration changed legacy policy", g)
	}
}
