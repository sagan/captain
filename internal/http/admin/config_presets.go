package admin

import (
	"encoding/json"
	"github.com/zeptop-dev/bosun/pkg/spec"
	"net/http"
)

func (h *handlers) registerConfigPresets(m *http.ServeMux) {
	m.HandleFunc("GET /api/admin/config-presets", h.requireAdmin(h.listConfigPresets))
	m.HandleFunc("POST /api/admin/config-presets", h.requireAdmin(h.saveConfigPreset))
	m.HandleFunc("PUT /api/admin/config-presets/{id}", h.requireAdmin(h.saveConfigPreset))
	m.HandleFunc("DELETE /api/admin/config-presets/{id}", h.requireAdmin(h.deleteConfigPreset))
	m.HandleFunc("POST /api/admin/config-presets/preview", h.requireAdmin(h.previewConfigPreset))
}
func (h *handlers) listConfigPresets(w http.ResponseWriter, r *http.Request) {
	v, e := h.Store.ConfigPresets(r.Context())
	if e != nil {
		serverErr(w, e)
		return
	}
	ok(w, v)
}
func (h *handlers) saveConfigPreset(w http.ResponseWriter, r *http.Request) {
	var p spec.ConfigPreset
	if !decode(r, &p) {
		fail(w, 400, "invalid preset")
		return
	}
	p.ID = 0
	if r.Method == "PUT" {
		id, v := pathID(r)
		if !v {
			fail(w, 400, "invalid id")
			return
		}
		p.ID = id
	}
	if err := p.Normalize(); err != nil {
		fail(w, 400, err.Error())
		return
	}
	if err := h.Store.SaveConfigPreset(r.Context(), &p); err != nil {
		serverErr(w, err)
		return
	}
	ok(w, p)
}
func (h *handlers) deleteConfigPreset(w http.ResponseWriter, r *http.Request) {
	if err := h.Store.DeleteConfigPreset(r.Context(), idOf(r)); err != nil {
		fail(w, 404, "preset not found")
		return
	}
	ok(w, map[string]bool{"ok": true})
}
func (h *handlers) previewConfigPreset(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID      int64           `json:"id"`
		Current json.RawMessage `json:"current"`
	}
	if !decode(r, &in) {
		fail(w, 400, "invalid preview")
		return
	}
	list, err := h.Store.ConfigPresets(r.Context())
	if err != nil {
		serverErr(w, err)
		return
	}
	for _, p := range list {
		if p.ID == in.ID {
			v, e := p.Preview(in.Current)
			if e != nil {
				fail(w, 400, e.Error())
				return
			}
			ok(w, v)
			return
		}
	}
	fail(w, 404, "preset not found")
}
