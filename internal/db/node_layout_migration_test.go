package db_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/zeptop-dev/captain/internal/db"
	"github.com/zeptop-dev/captain/internal/domain"
	"github.com/zeptop-dev/captain/internal/store"
	"github.com/zeptop-dev/captain/migrations"
)

func TestNodeLayoutMigrationPreservesInterleavedGroups(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "layout.db")
	c, err := db.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite"); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpToContext(ctx, c, ".", 75); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Exec(`INSERT INTO nodes (id,name,monitor_group,created_at,updated_at) VALUES (1,'first','Z',1,1),(4,'second','A',1,1),(7,'third','Z',1,1),(9,'fourth','',1,1)`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := db.Migrate(ctx, c, "sqlite"); err != nil {
			t.Fatal(err)
		}
	}
	c.Close()
	// Reopen the database, rather than relying on any in-memory ordering.
	c, err = db.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	s := store.New(c)
	layout, err := s.NodeLayout(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []store.NodePlacement{{ID: 1, Group: "Z"}, {ID: 4, Group: "A"}, {ID: 7, Group: "Z"}, {ID: 9, Group: ""}}
	if !reflect.DeepEqual(layout.Nodes, want) || !reflect.DeepEqual(layout.Groups, []string{"Z", "A"}) {
		t.Fatal(layout)
	}
	n := &domain.Node{Name: "appended"}
	if err := s.CreateNode(ctx, n, "new", time.Hour); err != nil {
		t.Fatal(err)
	}
	nodes, err := s.ListNodes(ctx)
	if err != nil || nodes[len(nodes)-1].ID != n.ID {
		t.Fatal(nodes, err)
	}
}
