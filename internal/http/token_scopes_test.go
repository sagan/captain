package http

import (
	"context"
	"encoding/json"
	"github.com/zeptop-dev/captain/internal/http/admin"
	"slices"
	"strings"
	"testing"
)

func TestTokenScopeCatalogUsesAdminAuthorization(t *testing.T) {
	r := newRig(t)
	c := &client{t: t, srv: r.srv}
	const path = "/api/admin/tokens/scopes"
	if code, _, _ := c.do("GET", path, nil, nil); code != 401 {
		t.Fatalf("anonymous catalog: %d", code)
	}
	code, b, _ := r.c.do("GET", path, nil, nil)
	if code != 200 {
		t.Fatalf("staff catalog: %d %s", code, b)
	}
	resources := mustJSON[[]string](t, b)
	for _, resource := range []string{"users", "nodes", "config-presets"} {
		if !slices.Contains(resources, resource) {
			t.Fatalf("catalog missing %q", resource)
		}
	}
	for _, grant := range []string{"users:read", "tokens:read"} {
		token, _, err := r.st.CreateAPIToken(context.Background(), 1, "catalog", "read", nil, []string{grant})
		if err != nil {
			t.Fatal(err)
		}
		want := 403
		if grant == "tokens:read" {
			want = 200
		}
		if code, _, _ := c.do("GET", path, nil, map[string]string{"Authorization": "Bearer " + token}); code != want {
			t.Fatalf("catalog grant %s: got %d want %d", grant, code, want)
		}
	}
}

func TestResourceTokenScopesApplyToRESTAndMCP(t *testing.T) {
	r := newRig(t)
	_, b, _ := r.c.do("POST", "/api/admin/tokens", map[string]any{"Name": "users automation", "Scopes": []string{"users:read", "users:write"}}, nil)
	token := mustJSON[map[string]any](t, b)["token"].(string)
	c := &client{t: t, srv: r.srv}
	h := map[string]string{"Authorization": "Bearer " + token}
	if code, _, _ := c.do("GET", "/api/admin/users", nil, h); code != 200 {
		t.Fatal(code)
	}
	if code, _, _ := c.do("POST", "/api/admin/users", map[string]string{"Email": "scope@example.com", "Password": "password123"}, h); code != 200 {
		t.Fatal(code)
	}
	for _, path := range []string{"/api/admin/nodes", "/api/admin/settings/site", "/api/admin/system/selfcheck"} {
		if code, _, _ := c.do("GET", path, nil, h); code != 403 {
			t.Fatal("scope escaped", path, code)
		}
	}
	code, b, _ := c.do("POST", "/mcp", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}, h)
	if code != 200 || !strings.Contains(string(b), "user_create") || strings.Contains(string(b), "node_list") {
		t.Fatalf("MCP visibility %d %s", code, b)
	}
	_, b, _ = c.do("POST", "/mcp", map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": map[string]any{"name": "node_list", "arguments": map[string]any{}}}, h)
	if !strings.Contains(string(b), `"isError":true`) {
		t.Fatal("MCP invoked hidden tool", string(b))
	}
}

func TestEndpointScopesAndEmptyRestrictionsFailClosed(t *testing.T) {
	r := newRig(t)
	c := &client{t: t, srv: r.srv}
	for _, grants := range [][]string{{"GET /api/admin/users"}, {}, {"tokens:*"}} {
		_, b, _ := r.c.do("POST", "/api/admin/tokens", map[string]any{"Name": "narrow", "Scopes": grants}, nil)
		h := map[string]string{"Authorization": "Bearer " + mustJSON[map[string]any](t, b)["token"].(string)}
		want := 403
		if len(grants) > 0 && grants[0] == "GET /api/admin/users" {
			want = 200
		}
		if code, _, _ := c.do("GET", "/api/admin/users", nil, h); code != want {
			t.Fatal(grants, code)
		}
		if code, _, _ := c.do("POST", "/api/admin/tokens", map[string]string{"Name": "escalation"}, h); code != 403 {
			t.Fatal("minted unrestricted successor", code)
		}
		_, b, _ = c.do("POST", "/mcp", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}, h)
		var reply struct {
			Result struct {
				Tools []struct {
					Name string `json:"name"`
				} `json:"tools"`
			} `json:"result"`
		}
		if err := json.Unmarshal(b, &reply); err != nil {
			t.Fatal(err)
		}
		if want == 200 {
			if len(reply.Result.Tools) != 1 || reply.Result.Tools[0].Name != "user_list" {
				t.Fatal(string(b))
			}
		} else if len(reply.Result.Tools) != 0 {
			t.Fatal(string(b))
		}
	}
	if code, _, _ := r.c.do("POST", "/api/admin/tokens", map[string]any{"Name": "invalid", "Scopes": []string{"users:typo"}}, nil); code != 400 {
		t.Fatal("bad grant accepted", code)
	}
}

func TestTokenScopeCannotExpandBaseOrStaffRole(t *testing.T) {
	r := newRig(t)
	c := &client{t: t, srv: r.srv}
	token, _, err := r.st.CreateAPIToken(context.Background(), 1, "read token", "read", nil, []string{"*"})
	if err != nil {
		t.Fatal(err)
	}
	h := map[string]string{"Authorization": "Bearer " + token}
	if code, _, _ := c.do("POST", "/api/admin/users", map[string]string{"Email": "no@example.com"}, h); code != 403 {
		t.Fatal("read token gained write", code)
	}
	support, _ := admin.NewUser("support-scope@example.com", "password123", "support")
	if err := r.st.CreateStaff(context.Background(), support); err != nil {
		t.Fatal(err)
	}
	token, _, err = r.st.CreateAPIToken(context.Background(), support.ID, "support token", "full", nil, []string{"*"})
	if err != nil {
		t.Fatal(err)
	}
	h["Authorization"] = "Bearer " + token
	if code, _, _ := c.do("GET", "/api/admin/nodes", nil, h); code != 403 {
		t.Fatal("scope expanded role", code)
	}
	_, b, _ := c.do("POST", "/mcp", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}, h)
	if strings.Contains(string(b), "node_list") || strings.Contains(string(b), "user_create") {
		t.Fatal("MCP expanded role", string(b))
	}
}
