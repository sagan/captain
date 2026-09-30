package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/zeptop-dev/bosun/pkg/selfupdate"
	"github.com/zeptop-dev/captain/internal/auth"
	"github.com/zeptop-dev/captain/internal/store"
)

func (h *handlers) startNodeRemoval(w http.ResponseWriter, r *http.Request) {
	if !userFrom(r).IsAdmin() {
		fail(w, 403, "admin only")
		return
	}
	var in struct {
		Mode     string `json:"mode"`
		KeepData bool   `json:"keep_data"`
	}
	if !decode(r, &in) || (in.Mode != "standalone" && in.Mode != "uninstall") {
		fail(w, 400, "choose standalone or uninstall")
		return
	}
	id, valid := pathID(r)
	if !valid {
		fail(w, 400, "bad id")
		return
	}
	n, err := h.Store.NodeByID(r.Context(), id)
	if err != nil {
		serverErr(w, err)
		return
	}
	if !n.Paired || n.LastSeenAt == nil || time.Since(*n.LastSeenAt) >= 3*time.Minute {
		fail(w, 409, "node must be paired and online")
		return
	}
	if n.Version != "v0.55.0" && !selfupdate.Newer(n.Version, "v0.55.0") {
		fail(w, 409, "remote removal requires bosun v0.55.0 or newer")
		return
	}
	jobID := auth.Token(12)
	p := store.NodeRemovalParams{Mode: in.Mode, KeepData: in.KeepData, ExpiresAt: time.Now().Add(5 * time.Minute).Unix()}
	if err := h.Store.QueueNodeRemoval(r.Context(), jobID, id, p); err != nil {
		if errors.Is(err, store.ErrRemovalPending) {
			fail(w, 409, err.Error())
		} else {
			serverErr(w, err)
		}
		return
	}
	ok(w, map[string]string{"id": jobID})
}

func (h *handlers) getNodeRemoval(w http.ResponseWriter, r *http.Request) {
	id, valid := pathID(r)
	if !valid {
		fail(w, 400, "bad id")
		return
	}
	j, err := h.Store.LatestNodeRemoval(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		ok(w, nil)
		return
	}
	if err != nil {
		serverErr(w, err)
		return
	}
	// An unclaimed request expires without being sent to a returning node.
	var p store.NodeRemovalParams
	var progress struct {
		Phase string `json:"phase"`
	}
	_ = json.Unmarshal(j.Params, &p)
	_ = json.Unmarshal(j.Result, &progress)
	if j.DoneAt == nil && progress.Phase != "running" && time.Now().Unix() >= p.ExpiresAt {
		at := time.Now()
		j.DoneAt = &at
		j.Error = "removal request expired; reconnect the node and retry"
	}
	ok(w, j)
}
