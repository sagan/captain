package http

import (
	"context"
	"testing"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/captain/internal/domain"
)

func TestStaffCustomerNamespaces(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	portal := &client{t: t, srv: r.srv}
	if code, _, _ := portal.do("POST", "/api/portal/login", map[string]string{"email": "admin@test", "password": "password123"}, nil); code != 401 {
		t.Fatalf("console credentials accepted by portal: %d", code)
	}
	// Even a valid console cookie cannot purchase, view or change customer 1.
	for _, req := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/api/portal/me", nil},
		{"GET", "/api/portal/servers", nil},
		{"POST", "/api/portal/orders", map[string]any{"plan_id": 1, "gateway": "balance"}},
		{"PUT", "/api/portal/me/lang", map[string]string{"lang": "ja"}},
		{"POST", "/api/portal/tickets", map[string]string{"Subject": "test", "Body": "test"}},
	} {
		if code, _, _ := r.c.do(req.method, req.path, req.body, nil); code != 401 {
			t.Fatalf("staff reached %s: %d", req.path, code)
		}
	}
	// Same email, different passwords, independent sequences and sessions.
	code, b, _ := r.c.do("POST", "/api/admin/users", map[string]string{"Email": "admin@test", "Password": "customer123"}, nil)
	if code != 200 {
		t.Fatalf("customer: %d %s", code, b)
	}
	u := mustJSON[map[string]any](t, b)
	if u["id"] != float64(1) {
		t.Fatalf("first customer ID: %v", u["id"])
	}
	if code, _, _ := portal.do("POST", "/api/portal/login", map[string]string{"email": "admin@test", "password": "password123"}, nil); code != 401 {
		t.Fatalf("staff password used for customer: %d", code)
	}
	if code, b, _ := portal.do("POST", "/api/portal/login", map[string]string{"email": "admin@test", "password": "customer123"}, nil); code != 200 {
		t.Fatalf("customer login: %d %s", code, b)
	}
	if code, _, _ := portal.do("GET", "/api/admin/me", nil, nil); code != 401 {
		t.Fatalf("customer cookie reached console: %d", code)
	}
	stranger := &client{t: t, srv: r.srv}
	if code, _, _ := stranger.do("POST", "/api/admin/login", map[string]string{"Email": "admin@test", "Password": "customer123"}, nil); code != 401 {
		t.Fatalf("customer password reached console: %d", code)
	}

	_, b, _ = r.c.do("POST", "/api/admin/plans", map[string]any{"Name": "test", "PriceCents": 0, "PeriodDays": 30, "Enabled": true}, nil)
	plan := mustJSON[map[string]any](t, b)
	if code, b, _ := portal.do("POST", "/api/portal/orders", map[string]any{"plan_id": plan["ID"], "gateway": "balance"}, nil); code != 200 {
		t.Fatalf("purchase: %d %s", code, b)
	}
	_, b, _ = r.agent.do("GET", "/api/agent/state", nil, nil)
	state := mustJSON[agentproto.State](t, b)
	if len(state.Users) != 1 || state.Users[0].UUID != u["uuid"] {
		t.Fatal("customer 1 was not provisioned")
	}
	staff, err := r.st.StaffByID(ctx, 1)
	if err != nil || staff.Role != domain.RoleAdmin || staff.UUID != "" || staff.SubToken != "" {
		t.Fatalf("staff has customer credentials: %+v %v", staff, err)
	}

	// Revoking one namespace's sessions must not touch the other.
	if err := r.st.DeleteUserSessions(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := r.c.do("GET", "/api/admin/me", nil, nil); code != 200 {
		t.Fatalf("customer logout revoked staff: %d", code)
	}
	portal.do("POST", "/api/portal/login", map[string]string{"email": "admin@test", "password": "customer123"}, nil)
	if err := r.st.DeleteStaffSessions(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := portal.do("GET", "/api/portal/me", nil, nil); code != 200 {
		t.Fatalf("staff logout revoked customer: %d", code)
	}
	r.c.do("POST", "/api/admin/login", map[string]string{"Email": "admin@test", "Password": "password123"}, nil)
	if err := r.st.DeleteUser(ctx, 1); err == nil {
		t.Fatal("customer with an order should retain referential protection")
	}
	// Changing or banning a customer with the same ID cannot change staff.
	if err := r.st.UpdateUser(ctx, 1, "banned", nil, ""); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := r.c.do("GET", "/api/admin/me", nil, nil); code != 200 {
		t.Fatalf("customer ban affected staff: %d", code)
	}
}
