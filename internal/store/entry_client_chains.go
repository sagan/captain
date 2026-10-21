package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/zeptop-dev/captain/internal/domain"
)

const MaxClientChainDepth = 8

var ErrEntryClientChain = errors.New("invalid client proxy chain")
var ErrEntryClientChainInUse = errors.New("subscription node is used by a client proxy chain; remove that reference first")

// ValidateClientChain is called while the topology lock protects HTTP writes.
// Check all roots, including descendants of an edited node, so an edit cannot
// turn an existing short chain into an overlong or cyclic dependency.
func (s *Store) ValidateClientChain(ctx context.Context, proposed *domain.Entry) error {
	if proposed.ClientDialerEntryID != nil && *proposed.ClientDialerEntryID == 0 {
		proposed.ClientDialerEntryID = nil
	}
	if proposed.ClientDialerEntryID == nil {
		return nil
	}
	entries, err := s.ListEntries(ctx)
	if err != nil {
		return err
	}
	byID := make(map[int64]*domain.Entry, len(entries)+1)
	for _, e := range entries {
		byID[e.ID] = e
	}
	id := proposed.ID
	if id == 0 {
		id = -1
	}
	byID[id] = proposed
	for root, e := range byID {
		seen := map[int64]bool{root: true}
		for depth := 1; e.ClientDialerEntryID != nil; depth++ {
			next := *e.ClientDialerEntryID
			if next <= 0 || seen[next] {
				return fmt.Errorf("%w: cycles and self references are not allowed", ErrEntryClientChain)
			}
			if depth >= MaxClientChainDepth {
				return fmt.Errorf("%w: at most %d nodes", ErrEntryClientChain, MaxClientChainDepth)
			}
			seen[next] = true
			e = byID[next]
			if e == nil {
				return fmt.Errorf("%w: front node does not exist", ErrEntryClientChain)
			}
		}
	}
	return nil
}

// CheckClientChainRemoval gives cascade deletions the same understandable
// refusal as deleting a referenced entry. The FK also protects other callers.
func (s *Store) CheckClientChainRemoval(ctx context.Context, kind string, id int64) error {
	return checkClientChainRemoval(ctx, s.db, kind, id)
}

func checkClientChainRemoval(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, kind string, id int64) error {
	where := "e.id = ?"
	switch kind {
	case "inbound":
		where = "e.inbound_id = ?"
	case "node":
		where = `e.inbound_id IN (SELECT id FROM inbounds WHERE node_id = ?)
			OR e.inbound_id IN (SELECT user_inbound_id FROM node_reverse_links WHERE exit_id = ?)
			OR e.inbound_id IN (SELECT receiver_inbound_id FROM node_reverse_links WHERE exit_id = ?)
			OR e.inbound_id IN (SELECT user_inbound_id FROM node_wireguard_links WHERE exit_id = ?)`
	case "entry":
	default:
		return errors.New("invalid client chain resource kind")
	}
	args := []any{id}
	if kind == "node" {
		args = []any{id, id, id, id}
	}
	var count int
	err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM entries e JOIN entries dependent ON dependent.client_dialer_entry_id=e.id WHERE `+"("+where+")", args...).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrEntryClientChainInUse
	}
	return nil
}
