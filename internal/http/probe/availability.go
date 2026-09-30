package probe

import (
	"encoding/json"
	"github.com/zeptop-dev/captain/internal/store"
	"net/http"
	"time"
)

func (h *handlers) availability(w http.ResponseWriter, r *http.Request) {
	s := h.Probe.Settings(r.Context())
	if !s.PublicShows("history") || !s.PublicShows("availability") {
		http.NotFound(w, r)
		return
	}
	id, ok := h.nodeID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	n, err := h.Store.NodeByID(r.Context(), id)
	if err != nil || !n.Paired {
		http.NotFound(w, r)
		return
	}
	_, _, span, valid := store.ResourceRange(r.URL.Query().Get("range"))
	if !valid {
		http.Error(w, "invalid availability range", 400)
		return
	}
	at := time.Now()
	v, err := h.Store.Availability(r.Context(), id, at.Add(-span), at)
	if err != nil {
		http.Error(w, "internal error", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}
