package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/zeptop-dev/bosun/pkg/selfupdate"
	"github.com/zeptop-dev/bosun/pkg/spec"

	"github.com/zeptop-dev/captain/internal/auth"
	"github.com/zeptop-dev/captain/internal/store"
)

// Node jobs: one-off tasks (REALITY target scans) the panel hands a node
// through its state. The node answers in its next report; the UI polls.

var jobKinds = map[string]bool{"reality_scan": true, "warp_register": true, "rollback": true, spec.NetworkDiagnosticKind: true}

func (h *handlers) createNodeJob(w http.ResponseWriter, r *http.Request) {
	id, okID := pathID(r)
	if !okID {
		fail(w, http.StatusBadRequest, "bad id")
		return
	}
	var in struct {
		Kind   string          `json:"kind"`
		Params json.RawMessage `json:"params"`
	}
	if !decode(r, &in) || !jobKinds[in.Kind] {
		fail(w, http.StatusBadRequest, "unknown job kind")
		return
	}
	n, err := h.Store.NodeByID(r.Context(), id)
	if err != nil {
		fail(w, http.StatusNotFound, "node not found")
		return
	}
	jobID := auth.Token(12)
	if in.Kind == spec.NetworkDiagnosticKind {
		if !userFrom(r).IsAdmin() {
			fail(w, 403, "admin only")
			return
		}
		var p spec.DiagnosticRequest
		if json.Unmarshal(in.Params, &p) != nil {
			fail(w, 400, "bad diagnostic request")
			return
		}
		if err := p.Validate(); err != nil {
			fail(w, 400, err.Error())
			return
		}
		if !n.Paired || n.LastSeenAt == nil || time.Since(*n.LastSeenAt) >= 3*time.Minute {
			fail(w, 409, "node must be paired and online")
			return
		}
		minimum := "v0.56.0"
		if p.Type == "exit" {
			minimum = "v0.57.0"
		}
		if n.Version != minimum && !selfupdate.Newer(n.Version, minimum) {
			fail(w, 409, "this diagnostic requires bosun "+minimum+" or newer")
			return
		}
		if err := h.Store.QueueNetworkDiagnostic(r.Context(), jobID, id, p); err != nil {
			if errors.Is(err, store.ErrDiagnosticPending) {
				fail(w, 409, err.Error())
			} else {
				serverErr(w, err)
			}
			return
		}
		ok(w, map[string]string{"id": jobID})
		return
	}
	if err := h.Store.CreateNodeJob(r.Context(), jobID, id, in.Kind, in.Params); err != nil {
		serverErr(w, err)
		return
	}
	ok(w, map[string]string{"id": jobID})
}

func (h *handlers) getNodeJob(w http.ResponseWriter, r *http.Request) {
	id, okID := pathID(r)
	if !okID {
		fail(w, http.StatusBadRequest, "bad id")
		return
	}
	j, err := h.Store.NodeJob(r.Context(), id, r.PathValue("job"))
	if errors.Is(err, store.ErrNotFound) {
		fail(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		serverErr(w, err)
		return
	}
	if j.Kind == spec.NetworkDiagnosticKind && !userFrom(r).IsAdmin() {
		fail(w, 403, "admin only")
		return
	}
	ok(w, j)
}

func (h *handlers) networkDiagnostics(w http.ResponseWriter, r *http.Request) {
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
	list, err := h.Store.NetworkDiagnostics(r.Context(), id)
	if err != nil {
		serverErr(w, err)
		return
	}
	ok(w, list)
}
