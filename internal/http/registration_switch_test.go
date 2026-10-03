package http

import (
	"context"
	"fmt"
	"testing"

	"github.com/zeptop-dev/captain/internal/http/admin"
	"github.com/zeptop-dev/captain/internal/mail"
	"github.com/zeptop-dev/captain/internal/store"
)

func TestRegistrationRuntimeSwitch(t *testing.T) {
	for _, initial := range []bool{false, true} {
		t.Run(fmt.Sprint(initial), func(t *testing.T) {
			p := newPortalRig(t, initial)
			c := &client{t: t, srv: p.srv}
			put := func(v map[string]any) {
				t.Helper()
				if code, b, _ := p.admin.do("PUT", "/api/admin/settings/registration", v, nil); code != 200 {
					t.Fatalf("save: %d %s", code, b)
				}
			}
			check := func(want bool) {
				t.Helper()
				for _, endpoint := range []struct{ path, field string }{{"/api/portal/register/policy", "open"}, {"/api/site", "registration"}} {
					code, b, _ := c.do("GET", endpoint.path, nil, nil)
					if code != 200 || mustJSON[map[string]any](t, b)[endpoint.field] != want {
						t.Fatalf("%s want %v: %d %s", endpoint.path, want, code, b)
					}
				}
			}
			check(initial)
			code, b, _ := p.admin.do("GET", "/api/admin/settings/registration", nil, nil)
			if code != 200 || mustJSON[map[string]any](t, b)["enabled"] != initial {
				t.Fatalf("initial admin status: %d %s", code, b)
			}
			// Saving old fields alone preserves deployment defaults.
			put(map[string]any{"invite_only": false})
			check(initial)
			put(map[string]any{"enabled": true})
			check(true)
			if code, b, _ := c.do("POST", "/api/portal/register", map[string]string{"Email": "new@example.com", "Password": "password123"}, nil); code != 200 {
				t.Fatalf("runtime opening: %d %s", code, b)
			}
			put(map[string]any{"enabled": false})
			check(false)
			// Old clients and explicit null must not restore the open default.
			put(map[string]any{"invite_only": false})
			put(map[string]any{"enabled": nil})
			check(false)
			for _, req := range []struct {
				path string
				body map[string]string
			}{
				{"/api/portal/register", map[string]string{"Email": "blocked@example.com", "Password": "password123"}},
				{"/api/portal/verify/send", map[string]string{"Email": "blocked@example.com", "Purpose": "register"}},
			} {
				if code, b, _ := c.do("POST", req.path, req.body, nil); code != 403 {
					t.Fatalf("closed %s: %d %s", req.path, code, b)
				}
			}
			if _, err := p.st.UserByEmail(context.Background(), "blocked@example.com"); err == nil {
				t.Fatal("closed registration created a user")
			}
			if code, b, _ := c.do("POST", "/api/portal/login", map[string]string{"Email": "new@example.com", "Password": "password123"}, nil); code != 200 {
				t.Fatalf("existing login blocked: %d %s", code, b)
			}
			if code, b, _ := p.admin.do("POST", "/api/admin/users", map[string]string{"Email": "manual@example.com", "Password": "password123"}, nil); code != 200 {
				t.Fatalf("manual creation blocked: %d %s", code, b)
			}
			put(map[string]any{"enabled": true})
			check(true)
			put(map[string]any{"enabled": false})
			// Read errors must not silently discard the closed override.
			if _, err := p.db.Exec(`UPDATE settings SET value_json = 'broken' WHERE key = ?`, store.SettingRegistration); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{"/api/portal/register/policy", "/api/site", "/api/admin/settings/registration"} {
				if code, _, _ := p.admin.do("GET", path, nil, nil); code != 500 {
					t.Fatalf("masked read failure %s: %d", path, code)
				}
			}
			if code, _, _ := c.do("POST", "/api/portal/register", map[string]string{"Email": "error@example.com", "Password": "password123"}, nil); code != 500 {
				t.Fatal("registration failed open", code)
			}
			if code, _, _ := p.admin.do("PUT", "/api/admin/settings/registration", map[string]any{"invite_only": false}, nil); code != 500 {
				t.Fatal("overwrote unreadable setting", code)
			}
		})
	}
}

func TestRegistrationClosedResetAndPermissions(t *testing.T) {
	ctx := context.Background()
	ms := mail.Settings{Provider: "smtp", FromAddress: "noreply@example.com"}
	ms.SMTP.Host = "smtp.example.com"
	closed := false
	p := newPortalRig(t, true, func(st *store.Store) {
		if err := st.SetSetting(ctx, mail.SettingKey, ms); err != nil {
			t.Fatal(err)
		}
		if err := st.SetSetting(ctx, store.SettingRegistration, store.RegistrationSettings{Enabled: &closed}); err != nil {
			t.Fatal(err)
		}
	})
	p.customer("existing@example.com")
	var sent []mail.Message
	original := mail.SendFunc
	mail.SendFunc = func(_ context.Context, _ mail.Settings, m mail.Message) error { sent = append(sent, m); return nil }
	defer func() { mail.SendFunc = original }()
	c := &client{t: t, srv: p.srv}
	if code, b, _ := c.do("POST", "/api/portal/verify/send", map[string]string{"Email": "existing@example.com", "Purpose": "reset"}, nil); code != 200 || len(sent) != 1 {
		t.Fatalf("reset blocked: %d %s mails=%d", code, b, len(sent))
	}
	if code, _, _ := c.do("POST", "/api/portal/verify/send", map[string]string{"Email": "new@example.com", "Purpose": "register"}, nil); code != 403 || len(sent) != 1 {
		t.Fatalf("signup mail bypass: %d mails=%d", code, len(sent))
	}
	for _, role := range []string{"operator", "support"} {
		u, err := admin.NewUser(role+"@example.com", "password123", role)
		if err != nil {
			t.Fatal(err)
		}
		if err := p.st.CreateStaff(ctx, u); err != nil {
			t.Fatal(err)
		}
		staff := &client{t: t, srv: p.srv}
		if code, _, _ := staff.do("POST", "/api/admin/login", map[string]string{"Email": u.Email, "Password": "password123"}, nil); code != 200 {
			t.Fatal("staff login", code)
		}
		for _, method := range []string{"GET", "PUT"} {
			if code, _, _ := staff.do(method, "/api/admin/settings/registration", map[string]any{"enabled": true}, nil); code != 403 {
				t.Fatalf("%s %s registration policy: %d", role, method, code)
			}
		}
	}
}

func TestRegistrationLegacyExternalStatus(t *testing.T) {
	p := newPortalRig(t, false, func(st *store.Store) {
		if err := st.SetSetting(context.Background(), store.SettingOIDC, store.OIDCSettings{Providers: []store.OIDCProvider{{ID: "example", Name: "Example", Issuer: "https://login.example.com", ClientID: "client", ClientSecret: "private-test-secret", AutoRegister: true}}}); err != nil {
			t.Fatal(err)
		}
	})
	for _, method := range []string{"GET", "PUT"} {
		code, b, _ := p.admin.do(method, "/api/admin/settings/registration", map[string]any{"ip_limit": 2}, nil)
		if code != 200 {
			t.Fatalf("%s: %d %s", method, code, b)
		}
		data := mustJSON[map[string]any](t, b)
		if data["enabled"] != true || data["password_open"] != false || data["oidc_open"] != true {
			t.Fatalf("hidden legacy exception: %s", b)
		}
		if _, exists := data["settings"].(map[string]any)["enabled"]; exists {
			t.Fatal("legacy save unexpectedly overrides signup policy")
		}
	}
}
