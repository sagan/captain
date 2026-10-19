package admin

import (
	"errors"
	"github.com/zeptop-dev/captain/internal/store"
	"net/http"
)

func (h *handlers) nodeLayout(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut {
		var in store.NodeLayout
		if !readJSON(w, r, &in) {
			return
		}
		if err := h.Store.SaveNodeLayout(r.Context(), in); err != nil {
			switch {
			case errors.Is(err, store.ErrNodeLayoutConflict):
				fail(w, 409, err.Error())
			case errors.Is(err, store.ErrNodeLayoutInvalid):
				fail(w, 400, err.Error())
			default:
				serverErr(w, err)
			}
			return
		}
		if h.Probe != nil {
			h.Probe.Invalidate()
		}
	}
	out, err := h.Store.NodeLayout(r.Context())
	if err != nil {
		serverErr(w, err)
		return
	}
	ok(w, out)
}
