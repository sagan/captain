package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/db"
	"github.com/zeptop-dev/captain/internal/domain"
)

func TestClientChainStoreReferencesAndDepth(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open("sqlite", filepath.Join(t.TempDir(), "captain.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn, "sqlite"); err != nil {
		t.Fatal(err)
	}
	s := New(conn)
	n := &domain.Node{Name: "server"}
	if err := s.CreateNode(ctx, n, "pair", time.Hour); err != nil {
		t.Fatal(err)
	}
	ib := &domain.Inbound{NodeID: n.ID, Tag: "vless", Protocol: spec.VLESS, Port: 443, Enabled: true}
	if err := s.CreateInbound(ctx, ib); err != nil {
		t.Fatal(err)
	}
	var entries []*domain.Entry
	for i := range MaxClientChainDepth {
		e := &domain.Entry{Name: "node", InboundID: ib.ID, DisplayHost: "example.com", DisplayPort: 443, Enabled: true}
		if i > 0 {
			e.ClientDialerEntryID = &entries[i-1].ID
		}
		if err := s.CreateEntry(ctx, e); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, e)
	}
	got, err := s.EntryByID(ctx, entries[1].ID)
	if err != nil || got.ClientDialerEntryID == nil || *got.ClientDialerEntryID != entries[0].ID {
		t.Fatalf("reference lost: %+v %v", got, err)
	}
	tooDeep := &domain.Entry{Name: "too-deep", InboundID: ib.ID, DisplayHost: "example.com", DisplayPort: 443, ClientDialerEntryID: &entries[7].ID}
	if err := s.CreateEntry(ctx, tooDeep); !errors.Is(err, ErrEntryClientChain) {
		t.Fatalf("depth accepted: %v", err)
	}
	entries[0].ClientDialerEntryID = &entries[7].ID
	if err := s.UpdateEntry(ctx, entries[0]); !errors.Is(err, ErrEntryClientChain) {
		t.Fatalf("cycle accepted: %v", err)
	}
	entries[0].ClientDialerEntryID = nil
	for _, kind := range []string{"entry", "inbound", "node"} {
		id := entries[0].ID
		if kind == "inbound" {
			id = ib.ID
		}
		if kind == "node" {
			id = n.ID
		}
		if err := s.CheckClientChainRemoval(ctx, kind, id); !errors.Is(err, ErrEntryClientChainInUse) {
			t.Fatalf("%s reference not protected: %v", kind, err)
		}
	}
	if err := s.QueueNodeRemoval(ctx, "blocked-removal", n.ID, NodeRemovalParams{Mode: "uninstall", ExpiresAt: time.Now().Add(time.Minute).Unix()}); !errors.Is(err, ErrEntryClientChainInUse) {
		t.Fatalf("removal queued for referenced front: %v", err)
	}
	if err := s.DeleteEntry(ctx, entries[0].ID); !errors.Is(err, ErrEntryClientChainInUse) {
		t.Fatal(err)
	}
	// Only the leaf can be removed; removing it never deletes the shared front.
	if err := s.DeleteEntry(ctx, entries[7].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EntryByID(ctx, entries[0].ID); err != nil {
		t.Fatal(err)
	}
	standalone := &domain.Entry{Name: "standalone", InboundID: ib.ID, DisplayHost: "example.com", DisplayPort: 443}
	if err := s.CreateEntry(ctx, standalone); err != nil {
		t.Fatal(err)
	}
	entries[0].ClientDialerEntryID = &standalone.ID
	if err := s.UpdateEntry(ctx, entries[0]); err != nil {
		t.Fatal(err)
	}
	// Extending the front must validate all its descendants, not just itself.
	other := &domain.Entry{Name: "other", InboundID: ib.ID, DisplayHost: "example.com", DisplayPort: 443}
	if err := s.CreateEntry(ctx, other); err != nil {
		t.Fatal(err)
	}
	standalone.ClientDialerEntryID = &other.ID
	if err := s.UpdateEntry(ctx, standalone); !errors.Is(err, ErrEntryClientChain) {
		t.Fatalf("descendant depth accepted: %v", err)
	}
	admin := &domain.User{Email: "admin@example.com", PasswordHash: "test-hash", Role: domain.RoleAdmin, Status: "active"}
	if err := s.CreateStaff(ctx, admin); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ResetSite(ctx, admin.ID); err != nil {
		t.Fatalf("site reset with referenced client chains: %v", err)
	}
}

func TestClientChainProtectsReverseResources(t *testing.T) {
	for _, kind := range []string{"wireguard", "vless"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			conn, err := db.Open("sqlite", filepath.Join(t.TempDir(), "captain.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err = db.Migrate(ctx, conn, "sqlite"); err != nil {
				t.Fatal(err)
			}
			s := New(conn)
			a, b := &domain.Node{Name: "transit"}, &domain.Node{Name: "exit"}
			for _, n := range []*domain.Node{a, b} {
				if err = s.CreateNode(ctx, n, "pair-"+n.Name, time.Hour); err != nil {
					t.Fatal(err)
				}
			}
			ib := domain.Inbound{NodeID: a.ID, Tag: "reverse-user", Protocol: spec.VLESS, Port: 18443, Core: "xray", Enabled: true}
			entry := domain.Entry{Name: "reverse node", DisplayHost: "example.com", DisplayPort: 18443, Enabled: true}
			var frontID, inboundID int64
			var remove func() error
			if kind == "wireguard" {
				w := WireGuardWrite{Link: WireGuardLink{ID: "wg", TransitID: a.ID, ExitID: b.ID, Endpoint: "192.0.2.10:51820", ListenPort: 51820, MTU: 1380}, User: ib, Entry: entry}
				if err = s.SaveWireGuardLink(ctx, &w); err != nil {
					t.Fatal(err)
				}
				frontID, inboundID = w.Entry.ID, w.User.ID
				remove = func() error { return s.DeleteWireGuardLink(ctx, b.ID, w.Link.ID, w.Link.Version) }
			} else {
				receiver := ib
				receiver.Tag, receiver.Port = "reverse-receiver", 19443
				writes := []ReverseWrite{{Link: ReverseLink{ID: "reverse", ExitID: b.ID, TransitID: a.ID}, User: ib, Receiver: receiver, Entry: entry}}
				if err = s.SaveReverseLinks(ctx, b.ID, map[string]int64{}, writes); err != nil {
					t.Fatal(err)
				}
				frontID, inboundID = writes[0].Entry.ID, writes[0].User.ID
				remove = func() error { return s.SaveReverseLinks(ctx, b.ID, map[string]int64{"reverse": 1}, nil) }
			}
			dependent := &domain.Entry{Name: "chain", InboundID: inboundID, DisplayHost: "example.com", DisplayPort: 18443, ClientDialerEntryID: &frontID, Enabled: true}
			if err = s.QueueNodeRemoval(ctx, "remove-exit", b.ID, NodeRemovalParams{Mode: "uninstall", ExpiresAt: time.Now().Add(time.Minute).Unix()}); err != nil {
				t.Fatal(err)
			}
			if err = s.CreateEntry(ctx, dependent); !errors.Is(err, ErrEntryClientChain) {
				t.Fatalf("chain through removing reverse exit: %v", err)
			}
			if _, err = conn.Exec(`DELETE FROM node_jobs WHERE id='remove-exit'`); err != nil {
				t.Fatal(err)
			}
			if err = s.CreateEntry(ctx, dependent); err != nil {
				t.Fatal(err)
			}
			if err = remove(); !errors.Is(err, ErrEntryClientChainInUse) {
				t.Fatalf("reverse remove: %v", err)
			}
			// Deleting exit B also deletes owned listeners on A. Check both sides.
			if err = s.DeleteNode(ctx, b.ID); !errors.Is(err, ErrEntryClientChainInUse) {
				t.Fatalf("exit remove: %v", err)
			}
			if _, err = s.EntryByID(ctx, frontID); err != nil {
				t.Fatal("reference damaged", err)
			}
			if err = s.DeleteEntry(ctx, dependent.ID); err != nil {
				t.Fatal(err)
			}
			if err = remove(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestClientChainCannotMoveFrontToRemovingServer(t *testing.T) {
	s, ns, ib, _ := lineFixture(t)
	ctx := context.Background()
	front := &domain.Entry{Name: "front", InboundID: ib.ID, DisplayHost: "example.com", DisplayPort: 443}
	if err := s.CreateEntry(ctx, front); err != nil {
		t.Fatal(err)
	}
	child := &domain.Entry{Name: "child", InboundID: ib.ID, DisplayHost: "example.com", DisplayPort: 443, ClientDialerEntryID: &front.ID}
	if err := s.CreateEntry(ctx, child); err != nil {
		t.Fatal(err)
	}
	other := &domain.Inbound{NodeID: ns[1].ID, Tag: "other", Protocol: spec.VLESS, Port: 443, Enabled: true}
	if err := s.CreateInbound(ctx, other); err != nil {
		t.Fatal(err)
	}
	if err := s.QueueNodeRemoval(ctx, "remove", ns[1].ID, NodeRemovalParams{Mode: "uninstall", ExpiresAt: time.Now().Add(time.Minute).Unix()}); err != nil {
		t.Fatal(err)
	}
	front.InboundID = other.ID
	if err := s.UpdateEntry(ctx, front); !errors.Is(err, ErrEntryClientChain) {
		t.Fatalf("moved referenced ordinary front: %v", err)
	}
	if _, err := s.db.Exec(`UPDATE node_jobs SET done_at=?,error='' WHERE id='remove'`, now()); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateEntry(ctx, front); !errors.Is(err, ErrEntryClientChain) {
		t.Fatalf("successful removal allowed new path: %v", err)
	}
	if _, err := s.db.Exec(`UPDATE node_jobs SET error='failed' WHERE id='remove'`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateEntry(ctx, front); err != nil {
		t.Fatalf("failed removal still blocked path: %v", err)
	}
}
