package admin

import (
	"net/http"

	"github.com/zeptop-dev/bosun/pkg/selfupdate"
	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/store"
)

func (h *handlers) returnRoutes(w http.ResponseWriter, r *http.Request) {
	if !userFrom(r).IsAdmin() {
		fail(w, 403, "admin only")
		return
	}
	id, valid := pathID(r)
	if !valid {
		fail(w, 400, "bad id")
		return
	}
	if _, err := h.Store.NodeByID(r.Context(), id); err != nil {
		fail(w, 404, "node not found")
		return
	}
	p, err := h.Store.RouteSchedule(r.Context(), id)
	if err != nil {
		serverErr(w, err)
		return
	}
	history, err := h.Store.ReturnRoutes(r.Context(), id)
	if err != nil {
		serverErr(w, err)
		return
	}
	ok(w, map[string]any{"schedule": p, "history": history})
}
func (h *handlers) setReturnRoutes(w http.ResponseWriter, r *http.Request) {
	if !userFrom(r).IsAdmin() {
		fail(w, 403, "admin only")
		return
	}
	h.Store.Topology.Lock()
	defer h.Store.Topology.Unlock()
	id, valid := pathID(r)
	if !valid {
		fail(w, 400, "bad id")
		return
	}
	var p store.RouteSchedule
	if !decode(r, &p) || p.Validate() != nil {
		fail(w, 400, "invalid route schedule")
		return
	}
	n, err := h.Store.NodeByID(r.Context(), id)
	if err != nil {
		fail(w, 404, "node not found")
		return
	}
	if p.Enabled && (n.Version != spec.ReturnRouteMinimumVersion && !selfupdate.Newer(n.Version, spec.ReturnRouteMinimumVersion)) {
		fail(w, 409, "return routes require bosun "+spec.ReturnRouteMinimumVersion)
		return
	}
	if err := h.Store.SetRouteSchedule(r.Context(), id, p); err != nil {
		serverErr(w, err)
		return
	}
	ok(w, p)
}
