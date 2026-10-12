package admin

import (
	"net/http"

	"github.com/zeptop-dev/bosun/pkg/spec"
)

func (h *handlers) requireProxyNode(w http.ResponseWriter, r *http.Request, id int64) bool {
	if err := h.Store.RequireProxyNode(r.Context(), id); err != nil {
		fail(w, 409, err.Error())
		return false
	}
	return true
}

func (h *handlers) setNodeMode(w http.ResponseWriter, r *http.Request) {
	if !userFrom(r).IsAdmin() {
		fail(w, 403, "admin only")
		return
	}
	h.Store.Topology.Lock()
	defer h.Store.Topology.Unlock()
	var in struct {
		ExpectedRevision uint64        `json:"expected_revision"`
		Mode             spec.NodeMode `json:"mode"`
		Expected         spec.NodeMode `json:"expected"`
	}
	if !decode(r, &in) || in.Mode == "" || in.Mode.Validate() != nil || in.Expected == "" || in.Expected.Validate() != nil {
		fail(w, 400, "valid mode and expected mode are required")
		return
	}
	if err := h.Store.SetNodeMode(r.Context(), idOf(r), in.Mode, in.Expected, in.ExpectedRevision); err != nil {
		fail(w, 409, err.Error())
		return
	}
	ok(w, map[string]bool{"ok": true})
}
