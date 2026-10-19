package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/zeptop-dev/bosun/pkg/spec"
)

var ErrNodeLayoutConflict = errors.New("node layout changed; reload before saving")
var ErrNodeLayoutInvalid = errors.New("invalid node layout")

type NodePlacement struct {
	ID    int64  `json:"id"`
	Group string `json:"group"`
}
type NodeLayout struct {
	Revision string          `json:"revision"`
	Groups   []string        `json:"groups"`
	Nodes    []NodePlacement `json:"nodes"`
}

// Read in one transaction so the revision covers a consistent membership and order.
func readNodeLayout(ctx context.Context, tx *sql.Tx) (NodeLayout, error) {
	out := NodeLayout{Groups: []string{}, Nodes: []NodePlacement{}}
	rows, err := tx.QueryContext(ctx, `SELECT name FROM node_display_groups ORDER BY position,name`)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			break
		}
		out.Groups = append(out.Groups, name)
	}
	rowErr := rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if rowErr != nil {
		return out, rowErr
	}
	rows, err = tx.QueryContext(ctx, `SELECT id,monitor_group FROM nodes ORDER BY display_order,id`)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var n NodePlacement
		if err = rows.Scan(&n.ID, &n.Group); err != nil {
			break
		}
		out.Nodes = append(out.Nodes, n)
	}
	rowErr = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if rowErr != nil {
		return out, rowErr
	}
	b, _ := json.Marshal(out)
	out.Revision = fmt.Sprintf("%x", sha256.Sum256(b))
	return out, nil
}
func (s *Store) NodeLayout(ctx context.Context) (NodeLayout, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return NodeLayout{}, err
	}
	defer tx.Rollback()
	out, err := readNodeLayout(ctx, tx)
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func validDisplayGroup(name string) bool {
	return name != "" && name == strings.TrimSpace(name) && len(name) <= 128 && spec.PublicLabel(name)
}

// Group order is explicit; the submitted node order is retained within each group.
func writeNodeLayout(ctx context.Context, tx *sql.Tx, in NodeLayout) error {
	ranks := map[string]int{"": len(in.Groups)}
	for i, g := range in.Groups {
		if !validDisplayGroup(g) {
			return ErrNodeLayoutInvalid
		}
		if _, ok := ranks[g]; ok {
			return ErrNodeLayoutInvalid
		}
		ranks[g] = i
	}
	for _, n := range in.Nodes {
		if _, ok := ranks[n.Group]; !ok {
			return ErrNodeLayoutInvalid
		}
	}
	sort.SliceStable(in.Nodes, func(i, j int) bool { return ranks[in.Nodes[i].Group] < ranks[in.Nodes[j].Group] })
	if _, err := tx.ExecContext(ctx, `DELETE FROM node_display_groups`); err != nil {
		return err
	}
	for i, g := range in.Groups {
		if _, err := tx.ExecContext(ctx, `INSERT INTO node_display_groups (name,position) VALUES (?,?)`, g, i); err != nil {
			return err
		}
	}
	for i, n := range in.Nodes {
		if _, err := tx.ExecContext(ctx, `UPDATE nodes SET monitor_group=?,display_order=? WHERE id=?`, n.Group, i+1, n.ID); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) SaveNodeLayout(ctx context.Context, in NodeLayout) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	current, err := readNodeLayout(ctx, tx)
	if err != nil {
		return err
	}
	if in.Revision == "" || current.Revision != in.Revision {
		return ErrNodeLayoutConflict
	}
	if len(in.Nodes) != len(current.Nodes) || len(in.Groups) > 1000 {
		return ErrNodeLayoutInvalid
	}
	ids := map[int64]bool{}
	for _, n := range current.Nodes {
		ids[n.ID] = true
	}
	for _, n := range in.Nodes {
		if !ids[n.ID] {
			return ErrNodeLayoutInvalid
		}
		delete(ids, n.ID)
	}
	if err = writeNodeLayout(ctx, tx, in); err != nil {
		return err
	}
	return tx.Commit()
}

// Keep the existing monitor-group API working, including creation of new groups.
func (s *Store) SetMonitorGroup(ctx context.Context, id int64, group string) error {
	if group != "" && !validDisplayGroup(group) {
		return ErrNodeLayoutInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	layout, err := readNodeLayout(ctx, tx)
	if err != nil {
		return err
	}
	found := false
	for i, n := range layout.Nodes {
		if n.ID == id {
			if n.Group == group {
				return nil
			}
			layout.Nodes = append(append(layout.Nodes[:i:i], layout.Nodes[i+1:]...), NodePlacement{ID: id, Group: group})
			found = true
			break
		}
	}
	if !found {
		return ErrNotFound
	}
	exists := group == ""
	for _, g := range layout.Groups {
		exists = exists || g == group
	}
	if !exists {
		layout.Groups = append(layout.Groups, group)
	}
	if err = writeNodeLayout(ctx, tx, layout); err != nil {
		return err
	}
	return tx.Commit()
}
