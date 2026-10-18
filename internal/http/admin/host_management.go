package admin

import (
	"encoding/json"
	"errors"
	"github.com/zeptop-dev/bosun/pkg/selfupdate"
	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/auth"
	"github.com/zeptop-dev/captain/internal/store"
	"net/http"
	"time"
)

func (h *handlers) hostManagement(w http.ResponseWriter, r *http.Request) {
	if !userFrom(r).IsAdmin() {
		fail(w, 403, "admin only")
		return
	}
	id, valid := pathID(r)
	if !valid {
		fail(w, 400, "bad id")
		return
	}
	n, e := h.Store.NodeByID(r.Context(), id)
	if e != nil {
		fail(w, 404, "node not found")
		return
	}
	supported := n.Version == spec.HostManagementMinimumVersion || selfupdate.Newer(n.Version, spec.HostManagementMinimumVersion)
	online := n.Paired && n.LastSeenAt != nil && time.Since(*n.LastSeenAt) < 3*time.Minute
	if r.Method == http.MethodPost {
		var req spec.HostRequest
		if !decode(r, &req) {
			return
		}
		if e = req.Validate(); e != nil {
			fail(w, 400, e.Error())
			return
		}
		if !supported || !online {
			fail(w, 409, "host management requires an online bosun "+spec.HostManagementMinimumVersion+" or newer")
			return
		}
		job := auth.Token(12)
		if e = h.Store.QueueHostOperation(r.Context(), job, id, req); e != nil {
			if errors.Is(e, store.ErrHostPending) {
				fail(w, 409, e.Error())
			} else {
				serverErr(w, e)
			}
			return
		}
		ok(w, map[string]string{"id": job})
		return
	}
	jobs, e := h.Store.HostOperations(r.Context(), id)
	if e != nil {
		serverErr(w, e)
		return
	}
	var snapshot *spec.HostSnapshot
	for _, job := range jobs {
		var result spec.HostResult
		if json.Unmarshal(job.Result, &result) == nil && result.State != nil {
			snapshot = result.State
			break
		}
	}
	// Public keys are not secrets, but keep historical key material out of list
	// responses. Current observations expose public-key fingerprints instead.
	for i := range jobs {
		var p spec.HostJobParams
		if json.Unmarshal(jobs[i].Params, &p) == nil {
			delete(p.Values, "public_key")
			jobs[i].Params, _ = json.Marshal(p)
		}
	}
	ok(w, map[string]any{"supported": supported, "online": online, "minimum": spec.HostManagementMinimumVersion, "state": snapshot, "history": jobs})
}
