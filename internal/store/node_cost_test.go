package store

import (
	"testing"
	"time"
)

func TestNodeCostValidationAndLegacy(t *testing.T) {
	old := NodeProbeInfo{Price: "legacy price"}
	info := old
	if err := normalizeNodeCost(&info, old); err != nil || info.Price != old.Price {
		t.Fatal(info, err)
	}
	info.Billing = &NodeCost{Amount: "36.36", Currency: "USD", Cycle: "annual"}
	if err := normalizeNodeCost(&info, old); err != nil || info.Price != "USD 36.36 / year" {
		t.Fatal(info, err)
	}
	old = info
	echo := NodeProbeInfo{Price: old.Price}
	if err := normalizeNodeCost(&echo, old); err != nil || echo.Billing == nil {
		t.Fatal("legacy echo lost billing", err)
	}
	edit := NodeProbeInfo{Price: "new legacy price"}
	if err := normalizeNodeCost(&edit, old); err != nil || edit.Billing != nil {
		t.Fatal("legacy edit ignored", err)
	}
	for _, bad := range []NodeCost{{Amount: "-1", Currency: "USD", Cycle: "annual"}, {Amount: "NaN", Currency: "USD", Cycle: "annual"}, {Amount: "1", Currency: "ZZZ", Cycle: "annual"}, {Amount: "1", Currency: "USD", Cycle: "custom", Years: "0"}, {Amount: "1", Currency: "USD", Cycle: "custom", Years: "-1"}} {
		info = NodeProbeInfo{Billing: &bad}
		if normalizeNodeCost(&info, NodeProbeInfo{}) == nil {
			t.Fatal("accepted invalid cost", bad)
		}
	}
	for _, amount := range []string{"", "0", "36.3600"} {
		info = NodeProbeInfo{Billing: &NodeCost{Amount: amount, Currency: "USD", Cycle: "custom", Years: "1.5"}}
		if err := normalizeNodeCost(&info, NodeProbeInfo{}); err != nil {
			t.Fatal(err)
		}
		if amount == "" && info.Price != "" {
			t.Fatal("empty cost visible")
		}
	}
	info = NodeProbeInfo{ExpiresAt: "2027-02-29"}
	if normalizeNodeCost(&info, NodeProbeInfo{}) == nil {
		t.Fatal("accepted invalid date")
	}
	info = NodeProbeInfo{TrafficReset: &NodeTrafficReset{Mode: "expiry"}}
	if normalizeNodeCost(&info, NodeProbeInfo{}) == nil {
		t.Fatal("missing expiry accepted")
	}
}
func TestNodeCostMonthlyBoundaries(t *testing.T) {
	for _, c := range []struct {
		at, expiry, mode, want string
		day                    int
	}{
		{"2027-02-28", "2027-01-31", "expiry", "2027-02-28", 1},
		{"2028-02-29", "2027-01-31", "expiry", "2028-02-29", 1},
		{"2027-02-28", "2027-01-31", "expiry_next", "2027-02-01", 1},
		{"2027-03-01", "2027-01-31", "expiry_next", "2027-03-01", 1},
		{"2027-01-01", "2027-01-31", "expiry_next", "2027-01-01", 1},
		{"2027-06-09", "2027-06-09", "expiry_next", "2027-05-10", 1},
		{"2027-06-10", "2027-06-09", "expiry_next", "2027-06-10", 1},
		{"2027-02-28", "", "manual", "2027-02-28", 31},
		{"2027-03-01", "", "manual", "2027-02-28", 31},
	} {
		t.Run(c.at+c.mode+c.want, func(t *testing.T) {
			at, _ := time.Parse("2006-01-02", c.at)
			p := NodeProbe{ResetDay: c.day, Info: NodeProbeInfo{ExpiresAt: c.expiry, TrafficReset: &NodeTrafficReset{Mode: c.mode}}}
			if got := nodePeriodStart(at, &p).Format("2006-01-02"); got != c.want {
				t.Fatalf("got %s want %s", got, c.want)
			}
		})
	}
}
