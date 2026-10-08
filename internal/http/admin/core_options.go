package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/domain"
)

// Only a recent capability report can rule out a core. Old/offline nodes keep
// the existing save behavior and validate again when they apply the inbound.
func (h *handlers) nodeCoreCandidates(ctx context.Context, n *domain.Node) []spec.CoreCandidate {
	if n.LastSeenAt == nil || time.Since(*n.LastSeenAt) >= 3*time.Minute {
		return nil
	}
	status, err := h.Store.NodeStatus(ctx, n.ID)
	if err != nil {
		return nil
	}
	var cores map[string]agentproto.CoreStatus
	if json.Unmarshal(status.Cores, &cores) != nil {
		return nil
	}
	return agentproto.CoreCandidates(cores)
}

func (h *handlers) coreOptions(w http.ResponseWriter, r *http.Request) {
	id, valid := pathID(r)
	if !valid {
		fail(w, http.StatusBadRequest, "bad id")
		return
	}
	n, err := h.Store.NodeByID(r.Context(), id)
	if err != nil {
		fail(w, http.StatusNotFound, "node not found")
		return
	}
	ok(w, spec.PreviewCores(spec.CoreProbe(r.URL.Query()), h.nodeCoreCandidates(r.Context(), n)))
}

func (h *handlers) checkInboundCore(ctx context.Context, ib *domain.Inbound) string {
	if !ib.Enabled && !ib.Settings.PrivateAccess.Enabled() {
		return ""
	}
	n, err := h.Store.NodeByID(ctx, ib.NodeID)
	if err != nil {
		return "node not found"
	}
	candidates := h.nodeCoreCandidates(ctx, n)
	if ib.Settings.PrivateAccess.Enabled() && candidates == nil {
		return "private access requires a recent supported node capability report"
	}
	if candidates != nil {
		if _, err := spec.SelectCore(ib.Spec(), candidates); err != nil {
			return err.Error()
		}
	}
	return ""
}
