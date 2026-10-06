package http

import (
	"context"
	"errors"
	"testing"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/domain"
	"github.com/zeptop-dev/captain/internal/http/admin"
	"github.com/zeptop-dev/captain/internal/store"
)

func TestManualCustomerIDLifecycle(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	create := func(email string, id int64) *domain.User {
		t.Helper()
		code, b, _ := r.c.do("POST", "/api/admin/users", map[string]any{"Email": email, "Password": "password123", "id": id}, nil)
		if code != 200 || mustJSON[map[string]any](t, b)["id"] != float64(id) {
			t.Fatalf("manual create: %d %s", code, b)
		}
		u, err := r.st.UserByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	u := create("tester@example.com", 999999)
	// Normal creation stays at 1 even after creating the large testing ID.
	if id, _ := r.user("ordinary@example.com"); id != 1 {
		t.Fatal("large ID advanced automatic numbering", id)
	}
	plan := &domain.Plan{Name: "manual testing", PeriodDays: 1, QuotaBytes: 10000, Enabled: true}
	if err := r.st.CreatePlan(ctx, plan); err != nil {
		t.Fatal(err)
	}
	if code, b, _ := r.c.do("POST", "/api/admin/users/999999/grant", map[string]any{"PlanID": plan.ID}, nil); code != 200 {
		t.Fatalf("grant: %d %s", code, b)
	}
	portal := &client{t: t, srv: r.srv}
	if code, b, _ := portal.do("POST", "/api/portal/login", map[string]string{"Email": u.Email, "Password": "password123"}, nil); code != 200 {
		t.Fatalf("login: %d %s", code, b)
	}
	report := func(seq uint64, up int64) {
		t.Helper()
		code, b, _ := r.agent.do("POST", "/api/agent/report", agentproto.Report{TrafficSeq: seq, Traffic: []spec.UserTraffic{{UserID: u.AgentID, Up: up}}}, nil)
		if code != 200 {
			t.Fatalf("report: %d %s", code, b)
		}
	}
	report(1, 20)
	if code, b, _ := r.c.do("PUT", "/api/admin/users/999999/id", map[string]int64{"id": 888888}, nil); code != 200 {
		t.Fatalf("change ID: %d %s", code, b)
	}
	moved, err := r.st.UserByID(ctx, 888888)
	if err != nil || moved.UUID != u.UUID || moved.SubToken != u.SubToken || moved.AgentID != u.AgentID {
		t.Fatal("renumbering changed identity", err)
	}
	if code, b, _ := portal.do("GET", "/api/portal/me", nil, nil); code != 200 || mustJSON[map[string]any](t, b)["id"] != float64(888888) {
		t.Fatalf("renumbered session: %d %s", code, b)
	}
	report(2, 30)
	sub, err := r.st.ActiveSubscription(ctx, 888888)
	if err != nil || sub.UsedUpBytes != 50 {
		t.Fatal("manual account traffic not billed correctly", sub, err)
	}
	if code, b, _ := r.c.do("DELETE", "/api/admin/users/888888", nil, nil); code != 200 {
		t.Fatalf("delete test account after grant and use: %d %s", code, b)
	}
	if _, err := r.st.UserByID(ctx, 888888); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("deleted account remains", err)
	}
	if code, _, _ := portal.do("GET", "/api/portal/me", nil, nil); code != 401 {
		t.Fatalf("deleted account session survived: %d", code)
	}
	if code, _, _ := portal.do("GET", "/sub/"+u.SubToken, nil, nil); code != 404 {
		t.Fatalf("deleted account subscription survived: %d", code)
	}
	replacement := create("replacement@example.com", 888888)
	if replacement.AgentID == u.AgentID {
		t.Fatal("deleted accounting identity reused")
	}
	if code, b, _ := r.c.do("POST", "/api/admin/users/888888/grant", map[string]any{"PlanID": plan.ID}, nil); code != 200 {
		t.Fatalf("replacement grant: %d %s", code, b)
	}
	// A late batch from the deleted identity must not reach the replacement.
	report(3, 100)
	sub, err = r.st.ActiveSubscription(ctx, 888888)
	if err != nil || sub.UsedUpBytes != 0 {
		t.Fatal("replacement inherited old usage", sub, err)
	}
	var records int
	if err := r.st.DB().QueryRow(`SELECT COUNT(*) FROM traffic_daily WHERE user_id = 888888`).Scan(&records); err != nil || records != 0 {
		t.Fatal("replacement inherited history", records, err)
	}
	if id, _ := r.user("next@example.com"); id != 2 {
		t.Fatal("renumbering or deletion advanced automatic numbering", id)
	}
}

func TestManualCustomerIDValidationAndAccess(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	post := func(c *client, id any, email string, want int) {
		t.Helper()
		code, b, _ := c.do("POST", "/api/admin/users", map[string]any{"id": id, "Email": email, "Password": "password123"}, nil)
		if code != want {
			t.Fatalf("create ID %v: want %d, got %d %s", id, want, code, b)
		}
	}
	for _, id := range []any{0, -1, 1.5, "999999", store.MaxAccountID + 1} {
		post(r.c, id, "invalid@example.com", 400)
	}
	post(r.c, 999999, "test@example.com", 200)
	post(r.c, 999999, "collision@example.com", 409)
	post(r.c, 888888, "test@example.com", 409)
	if _, err := r.st.UserByID(ctx, 888888); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("failed creation left an account", err)
	}
	// A historical owner without a users row is not available for assignment.
	if _, err := r.st.DB().Exec(`INSERT INTO dyn_limits (user_id,mbps,since,until) VALUES (777777,2,1,9999999999)`); err != nil {
		t.Fatal(err)
	}
	post(r.c, 777777, "history@example.com", 409)
	for _, role := range []string{"operator", "support"} {
		u, err := admin.NewUser(role+"@example.com", "password123", role)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.st.CreateStaff(ctx, u); err != nil {
			t.Fatal(err)
		}
		c := &client{t: t, srv: r.srv}
		if code, b, _ := c.do("POST", "/api/admin/login", map[string]string{"Email": u.Email, "Password": "password123"}, nil); code != 200 {
			t.Fatalf("staff login: %d %s", code, b)
		}
		post(c, 555555, role+"-manual@example.com", 403)
		want := 403
		if role == "operator" {
			want = 200
		}
		post(c, nil, role+"-auto@example.com", want)
	}
	portal := &client{t: t, srv: r.srv}
	if code, b, _ := portal.do("POST", "/api/portal/login", map[string]string{"Email": "test@example.com", "Password": "password123"}, nil); code != 200 {
		t.Fatalf("customer login: %d %s", code, b)
	}
	post(portal, 555555, "customer-manual@example.com", 401)
	anonymous := &client{t: t, srv: r.srv}
	post(anonymous, 555555, "anonymous-manual@example.com", 401)
	u, err := r.st.UserByEmail(ctx, "operator-auto@example.com")
	if err != nil || u.ID != 1 {
		t.Fatal("operator automatic creation regressed", u, err)
	}
}
