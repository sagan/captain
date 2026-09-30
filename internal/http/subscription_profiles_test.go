package http

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/zeptop-dev/captain/internal/store"
)

func profileSetup(t *testing.T, r *rig) (int64, string, store.SubscriptionProfile) {
	t.Helper()
	id, _ := r.user("profile@example.com")
	u, err := r.st.UserByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	ibs, err := r.st.InboundsByNode(context.Background(), r.nodeID)
	if err != nil {
		t.Fatal(err)
	}
	if code, b, _ := r.c.do("POST", "/api/admin/entries", map[string]any{"Name": "Example", "InboundID": ibs[0].ID, "DisplayHost": "node.example.com", "DisplayPort": 443}, nil); code != 200 {
		t.Fatal(code, string(b))
	}
	tpl := store.NamedSubTemplate{Name: "Named Clash", Format: "clash", Body: "mixed-port: 17891\nproxies: []\n"}
	code, b, _ := r.c.do("POST", "/api/admin/settings/sub-library/templates", tpl, nil)
	if code != 200 {
		t.Fatal(code, string(b))
	}
	tpl = mustJSON[store.NamedSubTemplate](t, b)
	profile := store.SubscriptionProfile{Name: "Assigned profile", Settings: store.SubProfileSettings{Title: "Custom title", RemarkPrefix: "VIP ", Headers: map[string]string{"Profile-Update-Interval": "3"}}, Templates: map[string]int64{"clash": tpl.ID}}
	code, b, _ = r.c.do("POST", "/api/admin/settings/sub-profiles", profile, nil)
	if code != 200 {
		t.Fatal(code, string(b))
	}
	profile = mustJSON[store.SubscriptionProfile](t, b)
	return id, "/sub/" + u.SubToken, profile
}
func TestSubscriptionProfileAssignmentTemplateAndRenumber(t *testing.T) {
	r := newRig(t)
	id, path, p := profileSetup(t, r)
	global := store.SubscriptionProfile{Name: "Default", Default: true, Settings: store.SubProfileSettings{Title: "Global profile"}}
	if err := r.st.SaveSubscriptionProfile(context.Background(), &global); err != nil {
		t.Fatal(err)
	}
	code, _, headers := r.c.do("GET", path+"?client=clash", nil, nil)
	if code != 200 || headers.Get("Profile-Title") != "base64:"+base64.StdEncoding.EncodeToString([]byte("Global profile")) {
		t.Fatal("default profile not used", code, headers)
	}
	if code, b, _ := r.c.do("PUT", "/api/admin/users/"+itoa(id)+"/subscription-profile", map[string]int64{"profile_id": p.ID}, nil); code != 200 {
		t.Fatal(code, string(b))
	}
	code, b, headers := r.c.do("GET", path+"?client=clash", nil, nil)
	if code != 200 || !strings.Contains(string(b), "mixed-port: 17891") || !strings.Contains(string(b), "VIP Example") || headers.Get("Profile-Update-Interval") != "3" {
		t.Fatal(code, string(b), headers)
	}
	if code, _, _ := r.c.do("DELETE", "/api/admin/settings/sub-profiles/"+itoa(p.ID), nil, nil); code != 409 {
		t.Fatal("deleted assigned profile", code)
	}
	if code, _, _ := r.c.do("DELETE", "/api/admin/settings/sub-library/templates/"+itoa(p.Templates["clash"]), nil, nil); code != 409 {
		t.Fatal("deleted referenced template", code)
	}
	if code, b, _ := r.c.do("PUT", "/api/admin/users/"+itoa(id)+"/id", map[string]int64{"id": 55}, nil); code != 200 {
		t.Fatal(code, string(b))
	}
	moved, err := r.st.AssignedSubscriptionProfile(context.Background(), 55)
	if err != nil || moved == nil || *moved != p.ID {
		t.Fatal("profile not moved", moved, err)
	}
	if code, b, _ := r.c.do("POST", "/api/admin/settings/sub-profiles/preview", map[string]any{"profile": p, "user_id": 55, "format": "clash"}, nil); code != 200 || !strings.Contains(string(b), "VIP Example") {
		t.Fatal(code, string(b))
	}
}
func TestSubscriptionProfileDoesNotBypassAccessOrResponseRules(t *testing.T) {
	for _, action := range []string{"block", "serve"} {
		t.Run(action, func(t *testing.T) {
			r := newRig(t)
			id, path, p := profileSetup(t, r)
			if err := r.st.AssignSubscriptionProfile(context.Background(), id, &p.ID); err != nil {
				t.Fatal(err)
			}
			rule := map[string]any{"name": "priority", "enabled": true, "action": action, "format": "clash", "template": "mixed-port: 27891\n", "headers": map[string]string{"Profile-Update-Interval": "6"}}
			if code, b, _ := r.c.do("PUT", "/api/admin/settings/response-rules", []any{rule}, nil); code != 200 {
				t.Fatal(code, string(b))
			}
			code, b, headers := r.c.do("GET", path+"?client=clash", nil, nil)
			if action == "block" {
				if code != 403 {
					t.Fatal("profile bypassed rule", code)
				}
				return
			}
			if code != 200 || !strings.Contains(string(b), "mixed-port: 27891") || headers.Get("Profile-Update-Interval") != "6" {
				t.Fatal("rule lost precedence", code, string(b), headers)
			}
			if _, err := r.st.DB().Exec(`UPDATE subscriptions SET expires_at=1 WHERE user_id=?`, id); err != nil {
				t.Fatal(err)
			}
			_, b, _ = r.c.do("GET", path+"?client=clash", nil, nil)
			if strings.Contains(string(b), "node.example.com") {
				t.Fatal("profile granted expired user a node")
			}
		})
	}
}
func TestSubscriptionProfileHWIDAndEscapedPage(t *testing.T) {
	r := newRig(t)
	id, path, p := profileSetup(t, r)
	yes := true
	one := 1
	p.Settings.HWID = &store.ProfileHWID{Enabled: &yes, Require: &yes, DeviceLimit: &one}
	p.Settings.Page = &store.ProfilePage{Enabled: true, Title: "<script>alert(1)</script>", Description: "<img src=x onerror=alert(1)>", Accent: "#112233"}
	if err := r.st.SaveSubscriptionProfile(context.Background(), &p); err != nil {
		t.Fatal(err)
	}
	if err := r.st.AssignSubscriptionProfile(context.Background(), id, &p.ID); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := r.c.do("GET", path+"?client=page", nil, nil); code != 404 {
		t.Fatal("page bypassed required HWID", code)
	}
	code, b, head := r.c.do("GET", path+"?client=page", nil, map[string]string{"x-hwid": "device-aaaa"})
	if code != 200 || !strings.Contains(head.Get("Content-Type"), "text/html") || strings.Contains(string(b), "<script>") || strings.Contains(string(b), "<img ") || !strings.Contains(string(b), "&lt;script&gt;") || head.Get("Referrer-Policy") != "no-referrer" {
		t.Fatal(code, string(b), head)
	}
	_, _, head = r.c.do("GET", path+"?client=clash", nil, map[string]string{"x-hwid": "device-bbbb"})
	if head.Get("x-hwid-max-devices-reached") != "true" {
		t.Fatal("profile limit not applied", head)
	}
	if _, err := r.st.DB().Exec(`UPDATE users SET hwid_limit=0 WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	_, _, head = r.c.do("GET", path+"?client=clash", nil, map[string]string{"x-hwid": "device-bbbb"})
	if head.Get("x-hwid-max-devices-reached") != "" {
		t.Fatal("user override lost priority", head)
	}
}
func TestSubscriptionProfileRejectsUnsafeOverrides(t *testing.T) {
	r := newRig(t)
	for _, headers := range []map[string]string{{"Set-Cookie": "stolen=true"}, {"Content-Type": "text/html"}, {"Subscription-Userinfo": "total=999"}, {"Support-Url": "javascript:alert(1)"}, {"Announce": "hello\r\nSet-Cookie:x"}, {"Profile-Update-Interval": "0"}} {
		if code, _, _ := r.c.do("POST", "/api/admin/settings/sub-profiles", store.SubscriptionProfile{Name: "Unsafe", Settings: store.SubProfileSettings{Headers: headers}}, nil); code != 400 {
			t.Fatal(headers, code)
		}
	}
	for _, tpl := range []store.NamedSubTemplate{{Name: "Bad", Format: "clash", Body: "[unclosed"}, {Name: "Bad", Format: "singbox", Body: "{}"}} {
		if code, _, _ := r.c.do("POST", "/api/admin/settings/sub-library/templates", tpl, nil); code != 400 {
			t.Fatal(tpl, code)
		}
	}
	_, path, p := profileSetup(t, r)
	// Another customer keeps the default, never inheriting the explicit assignment.
	other, _ := r.user("other-profile@example.com")
	p.Settings.Title = "Isolated"
	p.Default = false
	if err := r.st.SaveSubscriptionProfile(context.Background(), &p); err != nil {
		t.Fatal(err)
	}
	if err := r.st.AssignSubscriptionProfile(context.Background(), other, &p.ID); err != nil {
		t.Fatal(err)
	}
	_, _, head := r.c.do("GET", path, nil, nil)
	if head.Get("Profile-Title") == "base64:"+base64.StdEncoding.EncodeToString([]byte("Isolated")) {
		t.Fatal("cross-user profile leak")
	}
}
