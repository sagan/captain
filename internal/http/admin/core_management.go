package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/domain"
	"github.com/zeptop-dev/captain/internal/store"
)

func (h *handlers) queueCoreOperation(w http.ResponseWriter, r *http.Request, n *domain.Node, jobID string, raw json.RawMessage) {
	if !userFrom(r).IsAdmin() {
		fail(w, 403, "admin only")
		return
	}
	var req spec.CoreRequest
	if json.Unmarshal(raw, &req) != nil {
		fail(w, 400, "bad core request")
		return
	}
	if err := req.Validate(); err != nil {
		fail(w, 400, err.Error())
		return
	}
	if !n.Paired || n.LastSeenAt == nil || time.Since(*n.LastSeenAt) >= 3*time.Minute {
		fail(w, 409, "node must be paired and online")
		return
	}
	st, err := h.Store.NodeStatus(r.Context(), n.ID)
	if err != nil {
		serverErr(w, err)
		return
	}
	var inventory *spec.CoreInventory
	if json.Unmarshal(st.CoreInventory, &inventory) != nil || inventory == nil {
		fail(w, 409, "node has not advertised core management support")
		return
	}
	if req.Action == "activate" && req.Revision != inventory.Revision {
		fail(w, 409, "core selection changed; refresh and try again")
		return
	}
	allowed := false
	for _, p := range inventory.Packages {
		if p.Distribution == req.Distribution && p.Version == req.Version && p.Status != "broken" {
			allowed = p.Available && (req.Action == "download" || p.Installed)
		}
	}
	if !allowed {
		fail(w, 409, "core version is unavailable or must be downloaded first")
		return
	}
	for _, instance := range inventory.Instances {
		if req.Action == "activate" && instance.Distribution == req.Distribution && instance.External {
			fail(w, 409, "node config pins an external binary")
			return
		}
	}
	if err := h.Store.QueueCoreOperation(r.Context(), jobID, n.ID, req); err != nil {
		if errors.Is(err, store.ErrCoreOperationPending) {
			fail(w, 409, err.Error())
		} else {
			serverErr(w, err)
		}
		return
	}
	ok(w, map[string]string{"id": jobID})
}
