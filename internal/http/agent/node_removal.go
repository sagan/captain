package agent

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/zeptop-dev/captain/internal/store"
)

// A worker outside the bosun service claims a request before stopping that
// service, then reports the actual outcome with the existing node credential.
func (h *handlers) nodeRemoval(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID         string `json:"id"`
		Phase      string `json:"phase"`
		Error      string `json:"error"`
		Listen     string `json:"listen"`
		LoginSetup bool   `json:"login_setup"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&in); err != nil || (in.Phase != "running" && in.Phase != "complete") {
		fail(w, 400, "bad removal result")
		return
	}
	n := nodeFrom(r)
	j, err := h.Store.NodeJob(r.Context(), n.ID, in.ID)
	if err != nil || j.Kind != store.NodeRemovalKind {
		fail(w, 404, "removal job not found")
		return
	}
	var p store.NodeRemovalParams
	if json.Unmarshal(j.Params, &p) != nil {
		fail(w, 500, "internal error")
		return
	}
	if j.DoneAt != nil {
		if in.Phase == "running" {
			fail(w, 409, "removal request already finished")
			return
		}
		ok(w, map[string]bool{"ok": true})
		return
	}
	if in.Phase == "running" {
		if err := h.Store.ClaimNodeRemoval(r.Context(), n.ID, j.ID, time.Now()); err != nil {
			fail(w, 409, "removal request expired or finished")
			return
		}
	} else {
		var prior struct {
			Phase string `json:"phase"`
		}
		if json.Unmarshal(j.Result, &prior) != nil || prior.Phase != "running" {
			fail(w, 409, "removal was not claimed")
			return
		}
		result, _ := json.Marshal(map[string]any{"phase": "complete", "mode": p.Mode, "listen": in.Listen, "login_setup": in.LoginSetup})
		if err := h.Store.CompleteNodeJob(r.Context(), n.ID, j.ID, result, in.Error); err != nil {
			fail(w, 500, "internal error")
			return
		}
	}
	if h.State != nil {
		h.State.Invalidate()
	}
	ok(w, map[string]bool{"ok": true})
}
