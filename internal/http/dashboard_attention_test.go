package http

import (
	"context"
	"testing"
	"time"

	"github.com/zeptop-dev/captain/internal/http/admin"
	"github.com/zeptop-dev/captain/internal/store"
)

func TestDashboardAttentionCountsAndPermissions(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	at := time.Now().UTC().Truncate(time.Second)
	if _, err := r.st.DB().Exec(`UPDATE nodes SET last_seen_at = ? WHERE id = ?`, at.Unix(), r.nodeID); err != nil {
		t.Fatal(err)
	}
	for _, n := range []struct {
		name   string
		token  any
		seen   any
		doctor string
	}{
		{"offline", "test-offline", at.Add(-3 * time.Minute).Unix(), `{"summary":{"fail":1}}`},
		{"never reported", "test-never", nil, ""},
		{"unpaired", nil, nil, `{"summary":{"fail":1}}`},
	} {
		if _, err := r.st.DB().Exec(`INSERT INTO nodes(name,token_hash,last_seen_at,doctor_json,created_at,updated_at) VALUES(?,?,?,?,?,?)`, n.name, n.token, n.seen, n.doctor, at.Unix(), at.Unix()); err != nil {
			t.Fatal(err)
		}
	}
	counts, err := r.st.DashboardNodes(ctx, at)
	if err != nil || *counts != (store.DashboardNodes{Offline: 2, Unpaired: 1, DoctorFail: 1}) {
		t.Fatalf("node counts: %+v %v", counts, err)
	}
	_, asset := infraFixture(t, r)
	today := at.Truncate(24 * time.Hour)
	for _, tc := range []struct {
		offset, days int
		active       bool
		want         int
	}{{0, 0, true, 1}, {-1, 0, true, 1}, {7, 7, true, 1}, {8, 7, true, 0}, {-1, 7, false, 0}} {
		if _, err := r.st.DB().Exec(`UPDATE infra_assets SET next_due=?,remind_days=?,active=? WHERE id=?`, today.AddDate(0, 0, tc.offset).Format("2006-01-02"), tc.days, tc.active, asset.ID); err != nil {
			t.Fatal(err)
		}
		got, err := r.st.DueInfraAssets(ctx, at.In(time.FixedZone("test", -7*3600)))
		if err != nil || got != tc.want {
			t.Fatalf("due %+v: %d %v", tc, got, err)
		}
	}
	if _, err := r.st.DB().Exec(`UPDATE infra_assets SET active=1,next_due=? WHERE id=?`, today.Format("2006-01-02"), asset.ID); err != nil {
		t.Fatal(err)
	}
	notice, err := r.st.CheckIncident(ctx, r.nodeID, store.IncidentCheck{Kind: "cpu", Active: true}, at)
	if err != nil || notice == nil {
		t.Fatal(err)
	}
	// Acknowledgement is not recovery: the dashboard must still count it.
	if code, _, _ := r.c.do("POST", "/api/admin/monitoring/incidents/"+itoa(notice.ID)+"/ack", nil, nil); code != 200 {
		t.Fatal(code)
	}
	get := func(c *client, headers map[string]string) map[string]any {
		t.Helper()
		code, b, _ := c.do("GET", "/api/admin/dashboard", nil, headers)
		if code != 200 {
			t.Fatalf("dashboard %d %s", code, b)
		}
		d := mustJSON[map[string]any](t, b)
		for _, key := range []string{"stats", "traffic", "open_tickets"} {
			if _, ok := d[key]; !ok {
				t.Fatal("removed existing field", key)
			}
		}
		return d["attention"].(map[string]any)
	}
	a := get(r.c, nil)
	if a["assets_due"] != float64(1) || a["open_incidents"] != float64(1) {
		t.Fatalf("attention %+v", a)
	}
	for _, role := range []string{"operator", "support"} {
		u, _ := admin.NewUser(role+"@example.com", "password123", role)
		if err := r.st.CreateStaff(ctx, u); err != nil {
			t.Fatal(err)
		}
		c := &client{t: t, srv: r.srv}
		c.do("POST", "/api/admin/login", map[string]string{"Email": u.Email, "Password": "password123"}, nil)
		a := get(c, nil)
		if _, ok := a["assets_due"]; ok {
			t.Fatalf("asset count exposed to %s", role)
		}
		if role == "support" && len(a) != 0 {
			t.Fatalf("support operational data %+v", a)
		}
		if role == "operator" && (a["nodes"] == nil || a["open_incidents"] != float64(1)) {
			t.Fatalf("operator data %+v", a)
		}
	}
	c := &client{t: t, srv: r.srv}
	for _, tc := range []struct {
		grants []string
		fields int
	}{
		{[]string{"dashboard:read"}, 0},
		{[]string{"dashboard:read", "nodes:read"}, 1},
		{[]string{"dashboard:read", "monitoring:read"}, 1},
		{[]string{"dashboard:read", "GET /api/admin/settings/infrastructure"}, 1},
	} {
		token, _, err := r.st.CreateAPIToken(ctx, 1, "dashboard", "read", nil, tc.grants)
		if err != nil {
			t.Fatal(err)
		}
		a := get(c, map[string]string{"Authorization": "Bearer " + token})
		if len(a) != tc.fields {
			t.Fatalf("token grant %+v: %+v", tc.grants, a)
		}
	}
	// Storage failure must not turn into a healthy zero on the dashboard.
	if _, err := r.st.DB().Exec(`DROP TABLE monitor_incidents`); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := r.c.do("GET", "/api/admin/dashboard", nil, nil); code != 500 {
		t.Fatal("masked summary failure", code)
	}
}
