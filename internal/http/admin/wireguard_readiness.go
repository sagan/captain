package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/auth"
	"github.com/zeptop-dev/captain/internal/store"
)

func (h *handlers) wireGuardReadiness(w http.ResponseWriter, r *http.Request) {
	id, valid := pathID(r)
	if !valid {
		fail(w, 400, "bad id")
		return
	}
	state, err := h.Store.WireGuardReadiness(r.Context(), id, r.URL.Query().Get("transit") == "true")
	if err != nil {
		fail(w, 404, "node not found")
		return
	}
	job, err := h.Store.LatestWireGuardSetupJob(r.Context(), id)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		serverErr(w, err)
		return
	}
	state.Job = job
	ok(w, state)
}
func (h *handlers) wireGuardSetup(w http.ResponseWriter, r *http.Request) {
	id, valid := pathID(r)
	if !valid {
		fail(w, 400, "bad id")
		return
	}
	var in struct {
		Action string `json:"action"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if in.Action != "check" && in.Action != "install" {
		fail(w, 400, "invalid WireGuard setup action")
		return
	}
	h.Store.Topology.Lock()
	defer h.Store.Topology.Unlock()
	state, err := h.Store.WireGuardReadiness(r.Context(), id, false)
	if err != nil {
		fail(w, 404, "node not found")
		return
	}
	if !h.requireProxyNode(w, r, id) {
		return
	}
	if !state.Supported || !state.Fresh {
		fail(w, 409, "upgrade bosun and wait for a fresh WireGuard setup report")
		return
	}
	if in.Action == "install" && !state.Installable {
		fail(w, 409, "missing tools cannot be installed automatically in this environment")
		return
	}
	old, err := h.Store.LatestWireGuardSetupJob(r.Context(), id)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		serverErr(w, err)
		return
	}
	if old != nil && old.DoneAt == nil {
		ok(w, map[string]string{"id": old.ID})
		return
	}
	p := spec.WireGuardSetupRequest{Action: in.Action, ExpiresAt: time.Now().Unix() + spec.WireGuardSetupTTL}
	raw, _ := json.Marshal(p)
	jobID := auth.Token(12)
	if err = h.Store.CreateNodeJob(r.Context(), jobID, id, spec.WireGuardSetupKind, raw); err != nil {
		serverErr(w, err)
		return
	}
	h.State.Invalidate()
	ok(w, map[string]string{"id": jobID})
}
