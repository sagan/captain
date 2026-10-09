package store

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/zeptop-dev/captain/internal/db"
	"github.com/zeptop-dev/captain/internal/domain"
)

func TestNodeDomainConcurrentClaims(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open("sqlite", filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn, "sqlite"); err != nil {
		t.Fatal(err)
	}
	s := New(conn)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- s.CreateNode(ctx, &domain.Node{Name: fmt.Sprint(i), Domain: " SAME.Example.com. "}, fmt.Sprint(i), time.Hour)
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		var conflict *NodeDomainConflict
		if err == nil {
			successes++
		} else if !errors.As(err, &conflict) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("exclusive claims succeeded: %d", successes)
	}
	// IDNA aliases are also the same name.
	if err := s.CreateNode(ctx, &domain.Node{Name: "unicode", Domain: "bücher.example.com"}, "unicode", time.Hour); err != nil {
		t.Fatal(err)
	}
	var conflict *NodeDomainConflict
	if err := s.CreateNode(ctx, &domain.Node{Name: "ascii", Domain: "xn--bcher-kva.example.com."}, "ascii", time.Hour); !errors.As(err, &conflict) {
		t.Fatalf("IDNA duplicate: %v", err)
	}
}
