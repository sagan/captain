package http

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/zeptop-dev/captain/internal/domain"
	"github.com/zeptop-dev/captain/internal/http/admin"
	"github.com/zeptop-dev/captain/internal/store"
)

func TestNodeLayoutSharedOrderingAndConflicts(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	req := func(method, path string, body any, want int) []byte {
		t.Helper()
		code, b, _ := r.c.do(method, path, body, nil)
		if code != want {
			t.Fatalf("%s %s: %d %s", method, path, code, b)
		}
		return b
	}
	read := func() store.NodeLayout {
		return mustJSON[store.NodeLayout](t, req("GET", "/api/admin/nodes/layout", nil, 200))
	}
	second := &domain.Node{Name: "Second"}
	third := &domain.Node{Name: "Hidden"}
	for i, n := range []*domain.Node{second, third} {
		code := itoa(int64(i + 100))
		if err := r.st.CreateNode(ctx, n, code, time.Hour); err != nil {
			t.Fatal(err)
		}
		if _, err := r.st.RedeemPairCode(ctx, code, code, "host", "v1.0.0", "linux"); err != nil {
			t.Fatal(err)
		}
	}
	original := read()
	if original.Nodes[0].ID != r.nodeID || original.Nodes[1].ID != second.ID {
		t.Fatal("initial order", original)
	}
	layout := read()
	layout.Groups = []string{"West", "East", "Empty"}
	layout.Nodes = []store.NodePlacement{{ID: r.nodeID, Group: "East"}, {ID: third.ID, Group: "West"}, {ID: second.ID, Group: "West"}}
	req("PUT", "/api/admin/nodes/layout", layout, 200)
	req("PUT", "/api/admin/nodes/layout", original, 409)
	got := read()
	want := []store.NodePlacement{{ID: third.ID, Group: "West"}, {ID: second.ID, Group: "West"}, {ID: r.nodeID, Group: "East"}}
	if !reflect.DeepEqual(got.Nodes, want) {
		t.Fatal(got)
	}
	// Invalid writes must not partially rename groups or drop nodes.
	for _, kind := range []string{"duplicate", "missing", "foreign", "markup", "duplicate-group", "unknown-group"} {
		bad := read()
		switch kind {
		case "duplicate":
			bad.Nodes[1] = bad.Nodes[0]
		case "missing":
			bad.Nodes = bad.Nodes[:1]
		case "foreign":
			bad.Nodes[0].ID = 99999
		case "markup":
			bad.Groups[0] = "<bad>"
		case "duplicate-group":
			bad.Groups = append(bad.Groups, bad.Groups[0])
		case "unknown-group":
			bad.Nodes[0].Group = "unknown"
		}
		req("PUT", "/api/admin/nodes/layout", bad, 400)
		if after := read(); !reflect.DeepEqual(got, after) {
			t.Fatalf("partial write %s", kind)
		}
	}
	listed := mustJSON[[]struct {
		ID    int64  `json:"id"`
		Group string `json:"monitor_group"`
	}](t, req("GET", "/api/admin/nodes", nil, 200))
	if len(listed) != 3 || listed[0].ID != third.ID || listed[1].Group != "West" {
		t.Fatal(listed)
	}
	// Public ordering is the same subset, never exposing hidden groups or nodes.
	if err := r.st.UpdateNodeProbe(ctx, third.ID, true, store.NodeProbeInfo{}, 0, 1, "sum"); err != nil {
		t.Fatal(err)
	}
	req("PUT", "/api/admin/settings/probe", map[string]any{"enabled": true, "path": "/status"}, 200)
	anon := &client{t: t, srv: r.srv}
	code, b, _ := anon.do("GET", "/api/probe", nil, nil)
	if code != 200 {
		t.Fatalf("probe %d %s", code, b)
	}
	pub := mustJSON[struct {
		Nodes []store.NodePlacement `json:"nodes"`
	}](t, b)
	if !reflect.DeepEqual(pub.Nodes, []store.NodePlacement{{ID: second.ID, Group: "West"}, {ID: r.nodeID, Group: "East"}}) {
		t.Fatal(pub)
	}
	// The legacy single-node API creates a catalog group and invalidates stale drafts.
	stale := read()
	req("PUT", "/api/admin/nodes/"+itoa(second.ID)+"/monitor-group", map[string]string{"group": "New"}, 200)
	req("PUT", "/api/admin/nodes/layout", stale, 409)
	got = read()
	if got.Groups[len(got.Groups)-1] != "New" {
		t.Fatal(got)
	}
	// Rename and removal only change placement, not node identity or visibility.
	got.Groups = []string{"Renamed", "New"}
	for i := range got.Nodes {
		switch got.Nodes[i].Group {
		case "West":
			got.Nodes[i].Group = "Renamed"
		case "East":
			got.Nodes[i].Group = ""
		}
	}
	req("PUT", "/api/admin/nodes/layout", got, 200)
	np, err := r.st.NodeProbe(ctx, third.ID)
	if err != nil || !np.Hidden {
		t.Fatal("visibility changed", np, err)
	}
	// Newly created nodes append, and invalidate a previously opened editor.
	stale = read()
	n := &domain.Node{Name: "New node"}
	if err := r.st.CreateNode(ctx, n, "NEW", time.Hour); err != nil {
		t.Fatal(err)
	}
	fresh := read()
	if fresh.Nodes[len(fresh.Nodes)-1].ID != n.ID {
		t.Fatal(fresh)
	}
	req("PUT", "/api/admin/nodes/layout", stale, 409)
}

func TestNodeLayoutPermissions(t *testing.T) {
	r := newRig(t)
	for _, role := range []string{"operator", "support"} {
		u, _ := admin.NewUser(role+"@example.com", "password123", role)
		if err := r.st.CreateStaff(context.Background(), u); err != nil {
			t.Fatal(err)
		}
		c := &client{t: t, srv: r.srv}
		c.do("POST", "/api/admin/login", map[string]string{"Email": u.Email, "Password": "password123"}, nil)
		want := 200
		if role == "support" {
			want = 403
		}
		code, b, _ := c.do("GET", "/api/admin/nodes/layout", nil, nil)
		if code != want {
			t.Fatal(role, code)
		}
		var input any = map[string]any{}
		if role == "operator" {
			input = mustJSON[store.NodeLayout](t, b)
		}
		code, _, _ = c.do("PUT", "/api/admin/nodes/layout", input, nil)
		if code != want {
			t.Fatal(role, code)
		}
	}
	anon := &client{t: t, srv: r.srv}
	for _, method := range []string{"GET", "PUT"} {
		if code, _, _ := anon.do(method, "/api/admin/nodes/layout", nil, nil); code != 401 {
			t.Fatal(code)
		}
	}
}
