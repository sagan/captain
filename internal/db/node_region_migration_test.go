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

func TestNodeRegionMigrationPreservesChoices(t *testing.T) {
	ctx := context.Background()
	c, err := db.Open("sqlite", filepath.Join(t.TempDir(), "region.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err = goose.SetDialect("sqlite"); err != nil {
		t.Fatal(err)
	}
	if err = goose.UpToContext(ctx, c, ".", 76); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Exec(`INSERT INTO nodes (id,name,probe_info_json,created_at,updated_at) VALUES (1,'manual','{"region":"JP"}',1,1),(2,'hidden','{}',1,1)`); err != nil {
		t.Fatal(err)
	}
	if err = db.Migrate(ctx, c, "sqlite"); err != nil {
		t.Fatal(err)
	}
	s := store.New(c)
	for _, tc := range []struct {
		id            int64
		mode, country string
	}{{1, "manual", "JP"}, {2, "off", ""}} {
		p, err := s.NodeProbe(ctx, tc.id)
		if err != nil || p.RegionMode != tc.mode || p.Info.Region != tc.country {
			t.Fatal(p, err)
		}
	}
	n := &domain.Node{Name: "new"}
	if err = s.CreateNode(ctx, n, "new", time.Hour); err != nil {
		t.Fatal(err)
	}
	p, err := s.NodeProbe(ctx, n.ID)
	if err != nil || p.RegionMode != "auto" {
		t.Fatal(p, err)
	}
}
