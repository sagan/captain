package admin

import (
	"net/http"
	"strings"
	"time"

	"github.com/zeptop-dev/captain/internal/store"
)

type dashboardAttention struct {
	Nodes         *store.DashboardNodes `json:"nodes,omitempty"`
	OpenIncidents *int                  `json:"open_incidents,omitempty"`
	AssetsDue     *int                  `json:"assets_due,omitempty"`
}

// The dashboard's aggregate does not widen access to monitoring or financial
// asset data. Apply the source endpoint's role AND token grant before reading.
func (h *handlers) dashboardAttention(r *http.Request, at time.Time) (*dashboardAttention, error) {
	var scope string
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer"))
	if strings.HasPrefix(token, "cap_") {
		owner, granted, err := h.Store.UserByAPIToken(r.Context(), token)
		if err != nil {
			return nil, err
		}
		if owner == nil {
			return &dashboardAttention{}, nil
		}
		scope = granted
	}
	canRead := func(path string) bool {
		return allowed(userFrom(r).Role, http.MethodGet, path) && (!strings.HasPrefix(token, "cap_") || store.TokenAllowsRequest(scope, http.MethodGet, path, "GET "+path))
	}
	out := &dashboardAttention{}
	var err error
	if canRead("/api/admin/nodes") {
		out.Nodes, err = h.Store.DashboardNodes(r.Context(), at)
		if err != nil {
			return nil, err
		}
	}
	if canRead("/api/admin/monitoring/incidents") {
		n, err := h.Store.OpenMonitorIncidents(r.Context())
		if err != nil {
			return nil, err
		}
		out.OpenIncidents = &n
	}
	if canRead("/api/admin/settings/infrastructure") {
		n, err := h.Store.DueInfraAssets(r.Context(), at)
		if err != nil {
			return nil, err
		}
		out.AssetsDue = &n
	}
	return out, nil
}
