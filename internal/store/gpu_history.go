package store

import (
	"math"

	"github.com/zeptop-dev/bosun/pkg/spec"
)

// Stored alongside the existing host/interface cursors in probe_counters_json.
// GPU collection is slower than beats; recording every cached copy would bias
// averages and fill gaps with invented measurements after a driver stalls.
type gpuCursor struct {
	Epoch    string `json:"epoch,omitempty"`
	Sequence uint64 `json:"sequence,omitempty"`
}

func freshGPU(r *spec.Resources, old gpuCursor) bool {
	if r == nil || r.GPU == nil {
		return false
	}
	g := r.GPU
	return g.Epoch != "" && len(g.Epoch) <= 128 && g.Sequence > 0 && g.Sequence <= math.MaxInt64 && g.At > 0 && r.At > 0 &&
		g.At <= r.At+5000 && g.At >= r.At-60000 && (g.Epoch != old.Epoch || g.Sequence > old.Sequence)
}
