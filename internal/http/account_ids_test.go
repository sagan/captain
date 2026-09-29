package http

import (
	"context"
	"testing"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/http/admin"
)

func TestChangeAccountIDsPreservesIdentity(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	id, uuid := r.user("alice@example.com")
	alice, err := r.st.UserByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	portal := &client{t: t, srv: r.srv}
	if code, b, _ := portal.do("POST", "/api/portal/login", map[string]string{"Email": alice.Email, "Password": "password123"}, nil); code != 200 {
		t.Fatalf("login: %d %s", code, b)
	}
	// A paid order and invite relationship must move with the customer.
	code, b, _ := r.c.do("POST", "/api/admin/plans", map[string]any{"Name": "free", "PriceCents": 0, "PeriodDays": 30, "Enabled": true}, nil)
	if code != 200 {
		t.Fatal(string(b))
	}
	plan := mustJSON[map[string]any](t, b)
	if code, b, _ := portal.do("POST", "/api/portal/orders", map[string]any{"plan_id": plan["ID"], "gateway": "balance"}, nil); code != 200 {
		t.Fatalf("order %d %s", code, b)
	}
	inv, err := admin.NewUser("invitee@example.com", "password123", "user")
	if err != nil {
		t.Fatal(err)
	}
	inv.InvitedBy = &id
	if err := r.st.CreateUser(ctx, inv); err != nil {
		t.Fatal(err)
	}
	tok, _, err := r.st.CreateAPIToken(ctx, 1, "test", "full", nil)
	if err != nil {
		t.Fatal(err)
	}
	report := func(seq uint64, amount int64) {
		t.Helper()
		code, b, _ := r.agent.do("POST", "/api/agent/report", agentproto.Report{TrafficSeq: seq, Traffic: []spec.UserTraffic{{UserID: alice.AgentID, Up: amount}}}, nil)
		if code != 200 {
			t.Fatalf("report: %d %s", code, b)
		}
	}
	report(1, 10)
	if code, b, _ := r.c.do("PUT", "/api/admin/users/"+itoa(id)+"/id", map[string]int64{"id": 42}, nil); code != 200 {
		t.Fatalf("renumber: %d %s", code, b)
	}
	moved, err := r.st.UserByID(ctx, 42)
	if err != nil || moved.UUID != uuid || moved.SubToken != alice.SubToken || moved.AgentID != alice.AgentID {
		t.Fatal("customer credentials changed", err)
	}
	if code, b, _ := portal.do("GET", "/api/portal/me", nil, nil); code != 200 || mustJSON[map[string]any](t, b)["id"] != float64(42) {
		t.Fatalf("customer session: %d %s", code, b)
	}
	if got, err := r.st.UserByID(ctx, inv.ID); err != nil || got.InvitedBy == nil || *got.InvitedBy != 42 {
		t.Fatal("invitation not moved", err)
	}
	var orders int
	if err := r.st.DB().QueryRow(`SELECT COUNT(*) FROM orders WHERE user_id = 42`).Scan(&orders); err != nil || orders != 1 {
		t.Fatal("order not moved", err)
	}
	// Reuse the visible ID for somebody else. Buffered traffic still belongs
	// to Alice, because her wire identity never changes.
	otherID, _ := r.user("bob@example.com")
	if code, b, _ := r.c.do("PUT", "/api/admin/users/"+itoa(otherID)+"/id", map[string]int64{"id": id}, nil); code != 200 {
		t.Fatalf("reuse: %d %s", code, b)
	}
	report(2, 7)
	report(2, 7)
	var total int64
	if err := r.st.DB().QueryRow(`SELECT COALESCE(SUM(up_bytes),0) FROM traffic_daily WHERE user_id=42`).Scan(&total); err != nil || total != 17 {
		t.Fatalf("old batch billed wrong customer: %d %v", total, err)
	}
	if err := r.st.DB().QueryRow(`SELECT COALESCE(SUM(up_bytes),0) FROM traffic_daily WHERE user_id=?`, id).Scan(&total); err != nil || total != 0 {
		t.Fatal("ID reuse inherited traffic", total, err)
	}
	if code, _, _ := r.c.do("PUT", "/api/admin/users/42/id", map[string]int64{"id": id}, nil); code != 409 {
		t.Fatalf("collision accepted: %d", code)
	}
	for _, bad := range []int64{0, -1, 9007199254740992} {
		if code, _, _ := r.c.do("PUT", "/api/admin/users/42/id", map[string]int64{"id": bad}, nil); code != 400 {
			t.Fatalf("bad ID %d accepted: %d", bad, code)
		}
	}
	if err := r.st.SetTOTP(ctx, 1, "JBSWY3DPEHPK3PXP", true); err != nil {
		t.Fatal(err)
	}
	if code, b, _ := r.c.do("PUT", "/api/admin/admins/1/id", map[string]int64{"id": 9}, nil); code != 200 {
		t.Fatalf("staff move: %d %s", code, b)
	}
	if code, b, _ := r.c.do("GET", "/api/admin/me", nil, nil); code != 200 || mustJSON[map[string]any](t, b)["id"] != float64(9) {
		t.Fatalf("self session lost: %d %s", code, b)
	}
	if u, _, err := r.st.UserByAPIToken(ctx, tok); err != nil || u.ID != 9 {
		t.Fatal("token lost", err)
	}
	if _, on, err := r.st.TOTP(ctx, 9); err != nil || !on {
		t.Fatal("TOTP lost", err)
	}
	// No operator or customer can renumber either namespace.
	op, _ := admin.NewUser("op@example.com", "password123", "operator")
	if err := r.st.CreateStaff(ctx, op); err != nil {
		t.Fatal(err)
	}
	operator := &client{t: t, srv: r.srv}
	operator.do("POST", "/api/admin/login", map[string]string{"Email": op.Email, "Password": "password123"}, nil)
	for _, c := range []*client{operator, portal} {
		for _, path := range []string{"/api/admin/users/42/id", "/api/admin/admins/9/id"} {
			if code, _, _ := c.do("PUT", path, map[string]int64{"id": 77}, nil); code < 400 {
				t.Fatalf("unauthorized rename: %d", code)
			}
		}
	}
	rows, err := r.st.DB().Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("broken references")
	}
}
