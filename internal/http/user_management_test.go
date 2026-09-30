package http

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/zeptop-dev/captain/internal/http/admin"
	"github.com/zeptop-dev/captain/internal/store"
)

func bulkPreview(t *testing.T, r *rig, req store.UserBulkRequest) store.UserBulkJob {
	t.Helper()
	code, b, _ := r.c.do("POST", "/api/admin/users/bulk/preview", req, nil)
	if code != 200 {
		t.Fatalf("preview %d %s", code, b)
	}
	return mustJSON[store.UserBulkJob](t, b)
}
func bulkExecute(t *testing.T, r *rig, job store.UserBulkJob) store.UserBulkJob {
	t.Helper()
	code, b, _ := r.c.do("POST", "/api/admin/users/bulk/jobs/"+job.ID, map[string]bool{"confirm": true}, nil)
	if code != 200 {
		t.Fatalf("execute %d %s", code, b)
	}
	return mustJSON[store.UserBulkJob](t, b)
}
func TestUserBulkRetryAndTransactionRollback(t *testing.T) {
	r := newRig(t)
	a, _ := r.user("bulk-a@example.com")
	b, _ := r.user("bulk-b@example.com")
	job := bulkPreview(t, r, store.UserBulkRequest{IDs: []int64{a, b}, Action: "ban"})
	if _, err := r.st.DB().Exec(`CREATE TRIGGER fail_bulk BEFORE UPDATE OF status ON users WHEN OLD.id=` + itoa(b) + ` BEGIN SELECT RAISE(FAIL,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	code, _, _ := r.c.do("POST", "/api/admin/users/bulk/jobs/"+job.ID, map[string]bool{"confirm": true}, nil)
	if code != 500 || r.status(a) != "active" {
		t.Fatal("partial batch committed", code, r.status(a))
	}
	if _, err := r.st.DB().Exec(`DROP TRIGGER fail_bulk`); err != nil {
		t.Fatal(err)
	}
	result := bulkExecute(t, r, job)
	if !result.Done || result.Rows[0].Outcome != "applied" || r.status(b) != "banned" {
		t.Fatal(result)
	}
	ctx := context.Background()
	sub, _ := r.st.ActiveSubscription(ctx, a)
	before := sub.ExpiresAt.Unix()
	extend := bulkPreview(t, r, store.UserBulkRequest{IDs: []int64{a}, Action: "extend", PlanID: sub.PlanID, Days: 10})
	for range 2 {
		bulkExecute(t, r, extend)
	}
	after, _ := r.st.ActiveSubscription(ctx, a)
	if after.ExpiresAt.Unix() != before+10*86400 {
		t.Fatal("extension applied twice", after)
	}
}
func TestUserBulkRejectsStaleIDsAndPreservesHistory(t *testing.T) {
	r := newRig(t)
	a, _ := r.user("bulk-old@example.com")
	b, _ := r.user("bulk-new@example.com")
	job := bulkPreview(t, r, store.UserBulkRequest{IDs: []int64{a, b}, Action: "ban"})
	for _, pair := range [][2]int64{{a, 42}, {b, a}} {
		code, body, _ := r.c.do("PUT", "/api/admin/users/"+itoa(pair[0])+"/id", map[string]int64{"id": pair[1]}, nil)
		if code != 200 {
			t.Fatal(code, string(body))
		}
	}
	result := bulkExecute(t, r, job)
	if result.Rows[0].Outcome != "conflict" || r.status(a) != "active" || r.status(42) != "active" {
		t.Fatal("stale identity accepted", result)
	}
	// Invited accounts protect their inviter even without any orders.
	u, err := admin.NewUser("bulk-invite@example.com", "password123", "user")
	if err != nil {
		t.Fatal(err)
	}
	u.InvitedBy = &a
	if err = r.st.CreateUser(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	job = bulkPreview(t, r, store.UserBulkRequest{IDs: []int64{a, u.ID}, Action: "delete"})
	result = bulkExecute(t, r, job)
	if result.Rows[0].Outcome != "history" || result.Rows[1].Outcome != "applied" {
		t.Fatal(result)
	}
	if _, err = r.st.UserByID(context.Background(), a); err != nil {
		t.Fatal("history deleted", err)
	}
}
func TestUserBulkOwnerExpiryRolesAndMetadata(t *testing.T) {
	r := newRig(t)
	id, _ := r.user("bulk-auth@example.com")
	ctx := context.Background()
	job := bulkPreview(t, r, store.UserBulkRequest{IDs: []int64{id}, Action: "ban"})
	operator, err := admin.NewUser("operator@example.com", "password123", "operator")
	if err != nil {
		t.Fatal(err)
	}
	if err = r.st.CreateStaff(ctx, operator); err != nil {
		t.Fatal(err)
	}
	other := &client{t: t, srv: r.srv}
	other.do("POST", "/api/admin/login", map[string]string{"Email": operator.Email, "Password": "password123"}, nil)
	for _, method := range []string{"GET", "POST"} {
		if code, _, _ := other.do(method, "/api/admin/users/bulk/jobs/"+job.ID, map[string]bool{"confirm": true}, nil); code != 404 {
			t.Fatal("other staff accessed operation", code)
		}
	}
	if _, err = r.st.ExecuteUserBulk(ctx, 1, job.ID, time.Now().Add(time.Hour)); err != store.ErrBulkRequest {
		t.Fatal("expired preview", err)
	}
	// Staff renumbering must migrate ownership, without exposing customer metadata.
	if code, b, _ := r.c.do("PUT", "/api/admin/admins/1/id", map[string]int64{"id": 31}, nil); code != 200 {
		t.Fatal(code, string(b))
	}
	bulkExecute(t, r, job)
	path := "/api/admin/users/" + itoa(id) + "/metadata"
	if code, b, _ := r.c.do("PUT", path, map[string]string{"contact": "internal-note"}, nil); code != 200 {
		t.Fatal(code, string(b))
	}
	_, body, _ := r.c.do("GET", path, nil, nil)
	if !strings.Contains(string(body), "internal-note") {
		t.Fatal(string(body))
	}
	if code, _, _ := r.c.do("PUT", path, map[string]string{"bad": strings.Repeat("x", 2049)}, nil); code != 400 {
		t.Fatal(code)
	}
	_, body, _ = r.c.do("GET", "/api/admin/users", nil, nil)
	if strings.Contains(string(body), "internal-note") {
		t.Fatal("metadata leaked into user projection")
	}
	support, err := admin.NewUser("support@example.com", "password123", "support")
	if err != nil {
		t.Fatal(err)
	}
	if err = r.st.CreateStaff(ctx, support); err != nil {
		t.Fatal(err)
	}
	c := &client{t: t, srv: r.srv}
	c.do("POST", "/api/admin/login", map[string]string{"Email": support.Email, "Password": "password123"}, nil)
	if code, _, _ := c.do("POST", "/api/admin/users/bulk/preview", store.UserBulkRequest{IDs: []int64{id}, Action: "unban"}, nil); code != 403 {
		t.Fatal("support could mutate", code)
	}
}
func TestUserFiltersValidateAndMatchSecondaryPlans(t *testing.T) {
	r := newRig(t)
	a, _ := r.user("z-filter@example.com")
	b, _ := r.user("a-filter@example.com")
	ctx := context.Background()
	sub, _ := r.st.ActiveSubscription(ctx, b)
	r.c.do("POST", "/api/admin/users/"+itoa(a)+"/grant", map[string]int64{"PlanID": sub.PlanID}, nil)
	code, body, _ := r.c.do("GET", "/api/admin/users?plan_id="+itoa(sub.PlanID)+"&sort=email&direction=asc&per_page=25", nil, nil)
	var page struct {
		Items []struct {
			ID       int64 `json:"id"`
			SubCount int   `json:"sub_count"`
		}
		Total   int
		PerPage int `json:"per_page"`
	}
	page = mustJSON[struct {
		Items []struct {
			ID       int64 `json:"id"`
			SubCount int   `json:"sub_count"`
		}
		Total   int
		PerPage int `json:"per_page"`
	}](t, body)
	if code != 200 || page.Total != 2 || page.Items[0].ID != b || page.Items[1].SubCount != 2 || page.PerPage != 25 {
		t.Fatal(code, string(body))
	}
	for _, filter := range []string{"sort=bogus", "status=staff", "per_page=1000000", "group_id=-1", "expires_from=x", "direction=bogus"} {
		if code, _, _ := r.c.do("GET", "/api/admin/users?"+filter, nil, nil); code != 400 {
			t.Fatal(filter, code)
		}
	}
}
