package admin

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/zeptop-dev/captain/internal/store"
)

func (h *handlers) detectNodeRegion(w http.ResponseWriter, r *http.Request) {
	if h.NodeRegions == nil {
		fail(w, 503, "region detection unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	if err := h.NodeRegions.Detect(ctx, idOf(r), true); err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			fail(w, 404, "node not found")
		case errors.Is(err, store.ErrRegionObservationChanged):
			fail(w, 409, "node address changed; retry detection")
		default:
			fail(w, 503, "region detection unavailable")
		}
		return
	}
	h.getNodeProbe(w, r)
}
