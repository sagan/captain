package http

import (
	"context"
	"testing"
	"time"

	"github.com/zeptop-dev/captain/internal/http/admin"
	"github.com/zeptop-dev/captain/internal/store"
)

const infraBase = "/api/admin/settings/infrastructure"

func infraFixture(t *testing.T, r *rig) (store.InfraSupplier, store.InfraAsset) {
	t.Helper()
	code, b, _ := r.c.do("POST", infraBase+"/suppliers", store.InfraSupplier{Name: "Vendor", URL: "https://example.com"}, nil)
	if code != 200 {
		t.Fatal(code, string(b))
	}
	s := mustJSON[store.InfraSupplier](t, b)
	code, b, _ = r.c.do("POST", infraBase+"/assets", store.InfraAsset{SupplierID: s.ID, NodeID: &r.nodeID, Name: "Compute", Currency: "USD", AmountMinor: 500, PeriodMonths: 1, AnchorDay: 31, NextDue: "2026-01-31", RemindDays: 7, Active: true}, nil)
	if code != 200 {
		t.Fatal(code, string(b))
	}
	return s, mustJSON[store.InfraAsset](t, b)
}
func TestInfraPaymentAtomicRetryCalendarAndHistory(t *testing.T) {
	r := newRig(t)
	s, a := infraFixture(t, r)
	ctx := context.Background()
	input := store.InfraPaymentInput{RequestKey: "payment_request_0001", Revision: a.Revision, AmountMinor: 550, PaidDate: "2026-01-30", Reference: "invoice-1"}
	endpoint := infraBase + "/assets/" + itoa(a.ID) + "/payments"
	if _, e := r.st.DB().Exec(`CREATE TRIGGER fail_asset BEFORE UPDATE ON infra_assets BEGIN SELECT RAISE(FAIL,'injected'); END`); e != nil {
		t.Fatal(e)
	}
	if code, _, _ := r.c.do("POST", endpoint, input, nil); code != 500 {
		t.Fatal(code)
	}
	_, total, e := r.st.InfraPayments(ctx, a.ID, 0)
	if e != nil || total != 0 {
		t.Fatal("partial payment", total, e)
	}
	if _, e = r.st.DB().Exec(`DROP TRIGGER fail_asset`); e != nil {
		t.Fatal(e)
	}
	var first store.InfraPayment
	for range 2 {
		code, b, _ := r.c.do("POST", endpoint, input, nil)
		if code != 200 {
			t.Fatal(code, string(b))
		}
		p := mustJSON[store.InfraPayment](t, b)
		if first.ID != 0 && p.ID != first.ID {
			t.Fatal("duplicate payment")
		}
		first = p
	}
	assets, _ := r.st.InfraAssets(ctx)
	if len(assets) != 1 || assets[0].NextDue != "2026-02-28" || assets[0].Revision != 2 {
		t.Fatal(assets)
	}
	input.AmountMinor = 600
	if code, _, _ := r.c.do("POST", endpoint, input, nil); code != 409 {
		t.Fatal("changed replay", code)
	}
	input.RequestKey = "payment_request_0002"
	if code, _, _ := r.c.do("POST", endpoint, input, nil); code != 409 {
		t.Fatal("stale revision accepted", code)
	}
	input.Revision = 2
	input.PaidDate = "2026-02-28"
	code, b, _ := r.c.do("POST", endpoint, input, nil)
	if code != 200 || mustJSON[store.InfraPayment](t, b).NextDue != "2026-03-31" {
		t.Fatal(code, string(b))
	}
	if code, _, _ := r.c.do("DELETE", infraBase+"/assets/"+itoa(a.ID)+"?revision=3", nil, nil); code != 409 {
		t.Fatal("history removed", code)
	}
	if code, _, _ := r.c.do("DELETE", infraBase+"/suppliers/"+itoa(s.ID), nil, nil); code != 409 {
		t.Fatal("supplier removed", code)
	}
	// Names are snapshots; node removal detaches the asset without deleting costs.
	s.Name = "Renamed"
	if e := r.st.SaveInfraSupplier(ctx, &s); e != nil {
		t.Fatal(e)
	}
	if code, b, _ := r.c.do("DELETE", "/api/admin/nodes/"+itoa(r.nodeID), nil, nil); code != 200 {
		t.Fatal(code, string(b))
	}
	assets, _ = r.st.InfraAssets(ctx)
	if assets[0].NodeID != nil {
		t.Fatal("node not detached")
	}
	rows, total, _ := r.st.InfraPayments(ctx, a.ID, 0)
	if total != 2 || rows[0].SupplierName != "Vendor" {
		t.Fatal(rows)
	}
	costs, _ := r.st.InfraCosts(ctx)
	if len(costs) != 1 || costs[0].AmountMinor != 1150 {
		t.Fatal(costs)
	}
	// Staff renumbering follows references while payment data stays fixed.
	if code, b, _ := r.c.do("PUT", "/api/admin/admins/1/id", map[string]int64{"id": 42}, nil); code != 200 {
		t.Fatal(code, string(b))
	}
	rows, _, _ = r.st.InfraPayments(ctx, a.ID, 0)
	if rows[0].StaffID == nil || *rows[0].StaffID != 42 {
		t.Fatal("staff reference not moved")
	}
}

func TestInfraAuthorizationValidationAndReminders(t *testing.T) {
	r := newRig(t)
	_, a := infraFixture(t, r)
	ctx := context.Background()
	for _, role := range []string{"operator", "support"} {
		u, _ := admin.NewUser(role+"@example.com", "password123", role)
		if e := r.st.CreateStaff(ctx, u); e != nil {
			t.Fatal(e)
		}
		c := &client{t: t, srv: r.srv}
		c.do("POST", "/api/admin/login", map[string]string{"Email": u.Email, "Password": "password123"}, nil)
		for _, method := range []string{"GET", "POST"} {
			path := infraBase
			if method == "POST" {
				path += "/suppliers"
			}
			if code, _, _ := c.do(method, path, nil, nil); code != 403 {
				t.Fatal(role, method, code)
			}
		}
	}
	for _, bad := range []store.InfraAsset{func() store.InfraAsset { x := a; x.AmountMinor = -1; return x }(), func() store.InfraAsset { x := a; x.Currency = "usd"; return x }(), func() store.InfraAsset { x := a; x.NextDue = "2026-02-30"; return x }(), func() store.InfraAsset { x := a; x.AnchorDay = 32; return x }()} {
		if e := bad.Validate(); e == nil {
			t.Fatal("invalid asset accepted", bad)
		}
	}
	if e := (store.InfraSupplier{Name: "x", URL: "javascript:alert(1)"}).Validate(); e == nil {
		t.Fatal("unsafe URL")
	}
	before, _ := time.Parse(time.DateOnly, "2026-01-23")
	at := before.AddDate(0, 0, 1)
	if claimed, e := r.st.ClaimInfraReminder(ctx, a, before); e != nil || claimed {
		t.Fatal(claimed, e)
	}
	if claimed, e := r.st.ClaimInfraReminder(ctx, a, at); e != nil || !claimed {
		t.Fatal(claimed, e)
	}
	if claimed, _ := r.st.ClaimInfraReminder(ctx, a, at); claimed {
		t.Fatal("duplicate lease")
	}
	if e := r.st.FinishInfraReminder(ctx, a, at, false); e != nil {
		t.Fatal(e)
	}
	if claimed, _ := r.st.ClaimInfraReminder(ctx, a, at.Add(time.Minute)); !claimed {
		t.Fatal("failed send cannot retry")
	}
	if e := r.st.FinishInfraReminder(ctx, a, at, true); e != nil {
		t.Fatal(e)
	}
	if claimed, _ := r.st.ClaimInfraReminder(ctx, a, at.Add(time.Hour)); claimed {
		t.Fatal("sent twice")
	}
	if claimed, _ := r.st.ClaimInfraReminder(ctx, a, at.AddDate(0, 0, 1)); !claimed {
		t.Fatal("next daily reminder absent")
	}
	a.Active = false
	if e := r.st.SaveInfraAsset(ctx, &a); e != nil {
		t.Fatal(e)
	}
	if claimed, _ := r.st.ClaimInfraReminder(ctx, a, at.AddDate(0, 0, 2)); claimed {
		t.Fatal("archived reminder")
	}
	for _, pair := range [][3]string{{"2024-01-31", "1", "2024-02-29"}, {"2024-02-29", "1", "2024-03-31"}, {"2026-12-31", "1", "2027-01-31"}} {
		next, e := store.NextInfraDue(pair[0], 1, 31)
		if e != nil || next != pair[2] {
			t.Fatal(pair, next, e)
		}
	}
}

func TestConfigPresetPreviewDoesNotMutateNode(t *testing.T) {
	r := newRig(t)
	code, b, _ := r.c.do("POST", "/api/admin/config-presets", map[string]any{"name": "Block example", "kind": "routes", "payload": []map[string]any{{"match": []string{"domain:example.com"}, "action": "block"}}}, nil)
	if code != 200 {
		t.Fatal(code, string(b))
	}
	p := mustJSON[map[string]any](t, b)
	code, b, _ = r.c.do("POST", "/api/admin/config-presets/preview", map[string]any{"id": p["id"], "current": map[string]any{"outbounds": []any{}, "routes": []any{}}}, nil)
	if code != 200 {
		t.Fatal(code, string(b))
	}
	nr, e := r.st.NodeRouting(context.Background(), r.nodeID)
	if e != nil || len(nr.Routes) != 0 {
		t.Fatal("preview changed node", nr, e)
	}
	_, b, _ = r.c.do("POST", "/api/admin/tokens", map[string]any{"Name": "preset-reader", "Scopes": []string{"config-presets:read"}}, nil)
	token := mustJSON[map[string]any](t, b)["token"].(string)
	c := &client{t: t, srv: r.srv}
	headers := map[string]string{"Authorization": "Bearer " + token}
	if code, _, _ := c.do("GET", "/api/admin/config-presets", nil, headers); code != 200 {
		t.Fatal(code)
	}
	if code, _, _ := c.do("POST", "/api/admin/config-presets/preview", nil, headers); code != 403 {
		t.Fatal(code)
	}
}
