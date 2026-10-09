package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/zeptop-dev/captain/internal/domain"
)

type NodeDomainUse struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Shared     bool   `json:"shared"`
	PublicAddr string `json:"-"`
	V6Addr     string `json:"-"`
}

type NodeDomainConflict struct{ Uses []NodeDomainUse }

func (e *NodeDomainConflict) Error() string {
	names := make([]string, 0, len(e.Uses))
	for _, n := range e.Uses {
		names = append(names, fmt.Sprintf("%s (#%d)", n.Name, n.ID))
	}
	return "domain is already used by " + strings.Join(names, ", ") + "; choose another domain or enable shared / external DNS"
}

type nodeDomainReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func nodeDomainUses(ctx context.Context, q nodeDomainReader, name string, excludeID int64) ([]NodeDomainUse, error) {
	out := []NodeDomainUse{}
	key := domain.NodeDomainKey(name)
	if key == "" {
		return out, nil
	}
	rows, err := q.QueryContext(ctx, `SELECT id, name, domain, domain_shared, public_addr, v6_addr FROM nodes WHERE id <> ? ORDER BY id`, excludeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var u NodeDomainUse
		var host string
		var shared int
		if err := rows.Scan(&u.ID, &u.Name, &host, &shared, &u.PublicAddr, &u.V6Addr); err != nil {
			return nil, err
		}
		u.Shared = shared != 0
		if domain.NodeDomainKey(host) == key {
			out = append(out, u)
		}
	}
	return out, rows.Err()
}

func (s *Store) NodeDomainUses(ctx context.Context, name string, excludeID int64) ([]NodeDomainUse, error) {
	return nodeDomainUses(ctx, s.db, name, excludeID)
}

// Validation and the write share one transaction: concurrent requests cannot
// both claim an exclusive name. Unchanged legacy duplicates remain editable.
func validateNodeDomain(ctx context.Context, tx *sql.Tx, n *domain.Node, old *domain.Node) error {
	name, err := domain.NormalizeNodeDomain(n.Domain)
	if err != nil {
		return err
	}
	n.Domain = name
	if n.DomainShared || name == "" {
		return nil
	}
	if old != nil && !old.DomainShared && domain.NodeDomainKey(old.Domain) == name {
		return nil
	}
	excludeID := int64(0)
	if old != nil {
		excludeID = old.ID
	}
	uses, err := nodeDomainUses(ctx, tx, name, excludeID)
	if err != nil {
		return err
	}
	if len(uses) != 0 {
		return &NodeDomainConflict{Uses: uses}
	}
	return nil
}
