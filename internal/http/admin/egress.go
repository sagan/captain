package admin

import (
	"github.com/zeptop-dev/bosun/pkg/selfupdate"
	"github.com/zeptop-dev/bosun/pkg/spec"
	"net/http"
)

const egressMinimumVersion = "v0.62.0"

func egressCapable(version string) bool {
	return version == egressMinimumVersion || selfupdate.Newer(version, egressMinimumVersion)
}

func (h *handlers) getNodeEgress(w http.ResponseWriter, r *http.Request) {
	n, err := h.Store.NodeByID(r.Context(), idOf(r))
	if err != nil {
		fail(w, 404, "node not found")
		return
	}
	list, err := h.Store.NodeEgressUpstreams(r.Context(), n.ID)
	if err != nil {
		serverErr(w, err)
		return
	}
	ok(w, map[string]any{"upstreams": list, "supported": egressCapable(n.Version), "minimum_version": egressMinimumVersion})
}
func (h *handlers) putNodeEgress(w http.ResponseWriter, r *http.Request) {
	h.Store.Topology.Lock()
	defer h.Store.Topology.Unlock()
	var in struct {
		Upstreams *[]spec.EgressUpstream `json:"upstreams"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if in.Upstreams == nil {
		fail(w, 400, "upstreams is required (use [] to clear)")
		return
	}
	n, err := h.Store.NodeByID(r.Context(), idOf(r))
	if err != nil {
		fail(w, 404, "node not found")
		return
	}
	if len(*in.Upstreams) > 0 && !egressCapable(n.Version) {
		fail(w, 409, "upstream exceptions require bosun "+egressMinimumVersion+" or newer")
		return
	}
	if err := spec.ValidateEgressUpstreams(*in.Upstreams); err != nil {
		fail(w, 400, err.Error())
		return
	}
	if err := h.Store.SetNodeEgressUpstreams(r.Context(), n.ID, *in.Upstreams); err != nil {
		serverErr(w, err)
		return
	}
	h.getNodeEgress(w, r)
}
