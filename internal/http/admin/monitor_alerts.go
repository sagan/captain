package admin

import (
	"errors"
	"github.com/zeptop-dev/captain/internal/store"
	"net/http"
	"sort"
	"strconv"
	"time"
)

func monitorQueryID(r *http.Request, key string) (int64, bool) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return 0, true
	}
	id, err := strconv.ParseInt(v, 10, 64)
	return id, err == nil && id > 0
}
func (h *handlers) monitorIncidents(w http.ResponseWriter, r *http.Request) {
	nodeID, valid := monitorQueryID(r, "node_id")
	before, validBefore := monitorQueryID(r, "before")
	state := r.URL.Query().Get("state")
	if !valid || !validBefore || (state != "" && state != "open" && state != "resolved" && state != "acknowledged") {
		fail(w, 400, "invalid incident query")
		return
	}
	rows, err := h.Store.MonitorIncidents(r.Context(), nodeID, before, state)
	if err != nil {
		serverErr(w, err)
		return
	}
	next := int64(0)
	if len(rows) > 50 {
		rows = rows[:50]
		next = rows[49].ID
	}
	ok(w, map[string]any{"items": rows, "next": next})
}
func (h *handlers) acknowledgeIncident(w http.ResponseWriter, r *http.Request) {
	if err := h.Store.AcknowledgeIncident(r.Context(), idOf(r), userFrom(r).Email, time.Now()); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fail(w, 404, "open unacknowledged incident not found")
		} else {
			serverErr(w, err)
		}
		return
	}
	ok(w, map[string]bool{"ok": true})
}
func (h *handlers) monitorWindows(w http.ResponseWriter, r *http.Request) {
	at := time.Now()
	rows, err := h.Store.MonitorWindows(r.Context(), at.Add(-7*24*time.Hour), at.Add(396*24*time.Hour))
	if err != nil {
		serverErr(w, err)
		return
	}
	// Keep every active/scheduled window manageable even when canceled future
	// windows would otherwise fill the display limit.
	sort.SliceStable(rows, func(i, j int) bool {
		activeI := rows[i].CanceledAt == nil && rows[i].EndsAt > at.Unix()
		activeJ := rows[j].CanceledAt == nil && rows[j].EndsAt > at.Unix()
		if activeI != activeJ {
			return activeI
		}
		if activeI {
			return rows[i].StartsAt < rows[j].StartsAt
		}
		return rows[i].StartsAt > rows[j].StartsAt
	})
	truncated := len(rows) > 200
	if truncated {
		rows = rows[:200]
	}
	ok(w, map[string]any{"items": rows, "truncated": truncated, "now": at.Unix()})
}
func (h *handlers) createMonitorWindow(w http.ResponseWriter, r *http.Request) {
	var v store.MonitorWindow
	if !readJSON(w, r, &v) {
		return
	}
	at := time.Now()
	if v.StartsAt == 0 {
		v.StartsAt = at.Unix()
	}
	if err := h.Store.CreateMonitorWindow(r.Context(), &v, at); err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			fail(w, 404, "node not found")
		case errors.Is(err, store.ErrMonitorWindow):
			fail(w, 400, "invalid window: use a future start and a duration up to 30 days")
		case errors.Is(err, store.ErrMonitorWindowLimit):
			fail(w, 409, "too many active or scheduled windows")
		default:
			serverErr(w, err)
		}
		return
	}
	ok(w, v)
}
func (h *handlers) cancelMonitorWindow(w http.ResponseWriter, r *http.Request) {
	if err := h.Store.CancelMonitorWindow(r.Context(), idOf(r), time.Now()); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fail(w, 404, "active or scheduled window not found")
		} else {
			serverErr(w, err)
		}
		return
	}
	ok(w, map[string]bool{"ok": true})
}
func (h *handlers) availability(w http.ResponseWriter, r *http.Request) {
	if _, err := h.Store.NodeByID(r.Context(), idOf(r)); err != nil {
		fail(w, 404, "node not found")
		return
	}
	_, _, span, valid := store.ResourceRange(r.URL.Query().Get("range"))
	if !valid {
		fail(w, 400, "invalid availability range")
		return
	}
	at := time.Now()
	v, err := h.Store.Availability(r.Context(), idOf(r), at.Add(-span), at)
	if err != nil {
		serverErr(w, err)
		return
	}
	ok(w, v)
}
