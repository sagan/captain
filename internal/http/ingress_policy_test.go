package http

import (
	"context"
	"strings"
	"testing"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/domain"
	"github.com/zeptop-dev/captain/internal/store"
)

func TestNATIngressPolicyAPI(t *testing.T) {
	r := newRig(t)
	call := func(method, path string, body any, want int) []byte {
		t.Helper()
		code, b, _ := r.c.do(method, path, body, nil)
		if code != want {
			t.Fatalf("%s %s: want %d got %d %s", method, path, want, code, b)
		}
		return b
	}
	path := "/api/admin/nodes/" + itoa(r.nodeID)
	in := map[string]any{"Name": "NAT", "Kind": "nat", "EntryHost": "203.0.113.30", "BindIP": "10.10.0.2", "RequireIngress": true, "PortMappings": []spec.PortMapping{{LocalFrom: 20000, LocalTo: 20009, PublicFrom: 30000}, {LocalFrom: 40000, LocalTo: 40000, PublicFrom: 443}}, "ReservedPorts": []int{20000}}
	// Enabling node-wide restrictions must not invalidate the existing direct inbound.
	call("POST", path+"/ingresses", in, 409)
	gs, err := r.st.IngressesByNode(context.Background(), r.nodeID)
	if err != nil || len(gs) != 0 {
		t.Fatal("failed create persisted", gs, err)
	}
	ibs, _ := r.st.AllInboundsByNode(context.Background(), r.nodeID)
	call("DELETE", "/api/admin/inbounds/"+itoa(ibs[0].ID), nil, 200)
	b := call("POST", path+"/ingresses", in, 200)
	g := mustJSON[struct {
		Ingress store.Ingress `json:"ingress"`
	}](t, b).Ingress
	gpath := "/api/admin/ingresses/" + itoa(g.ID)
	postIB := func(port int, id any, listen string, want int) []byte {
		t.Helper()
		return call("POST", path+"/inbounds", map[string]any{"Tag": "nat-in", "Protocol": "mieru", "Port": port, "IngressID": id, "Listen": listen}, want)
	}
	postIB(20001, nil, "", 400)
	postIB(20000, g.ID, "", 400)
	postIB(20010, g.ID, "", 400)
	postIB(20001, g.ID, "0.0.0.0", 400)
	ib := mustJSON[domain.Inbound](t, postIB(20001, g.ID, "", 200))
	call("PATCH", "/api/admin/inbounds/"+itoa(ib.ID), map[string]any{"Port": 20010}, 400)
	entry := mustJSON[domain.Entry](t, call("POST", "/api/admin/entries", map[string]any{"Name": "NAT entry", "InboundID": ib.ID}, 200))
	if entry.DisplayHost != "203.0.113.30" || entry.DisplayPort != 30001 {
		t.Fatal("unmapped entry", entry)
	}
	f := store.NodeForward{Forward: spec.Forward{Tag: "relay", Port: 20002, Protocol: "both", Target: "198.51.100.20:443", IngressID: itoa(g.ID)}}
	putF := func(f store.NodeForward, want int) {
		t.Helper()
		call("PUT", path+"/forwards", map[string]any{"Forwards": []store.NodeForward{f}}, want)
	}
	for _, backend := range []string{"", "nft", "realm"} {
		f.Backend = backend
		putF(f, 200)
	}
	for _, port := range []int{20000, 20010} {
		bad := f
		bad.Port = port
		putF(bad, 400)
	}
	bad := f
	bad.IngressID = ""
	putF(bad, 400)
	bad = f
	bad.IngressID = "999999"
	putF(bad, 400)
	bad = f
	bad.Listen = "0.0.0.0"
	putF(bad, 400)
	bad = f
	bad.Port = ib.Port
	bad.Protocol = "tcp"
	putF(bad, 400)
	// A different node cannot borrow this ingress reference.
	n := mustJSON[map[string]any](t, call("POST", "/api/admin/nodes", map[string]string{"Name": "other"}, 200))
	call("PUT", "/api/admin/nodes/"+itoa(int64(n["id"].(float64)))+"/forwards", map[string]any{"Forwards": []store.NodeForward{f}}, 400)
	// Old clients omit new fields. Omission and null must retain restrictions.
	old := map[string]any{"Name": "Renamed NAT", "EntryHost": "203.0.113.30", "BindIP": "10.10.0.2", "ReservedPorts": []int{20000}}
	for range 2 {
		call("PATCH", gpath, old, 200)
		old["PortMappings"] = nil
		old["RequireIngress"] = nil
	}
	got, err := r.st.IngressByID(context.Background(), g.ID)
	if err != nil || !got.RequireIngress || got.Kind != "nat" || got.EntryPort(40000) != 443 {
		t.Fatal("old client erased policy", got, err)
	}
	in["PortMappings"] = []spec.PortMapping{{LocalFrom: 40000, LocalTo: 40000, PublicFrom: 443}}
	call("PATCH", gpath, in, 409)
	in["PortMappings"] = g.PortMappings
	in["ReservedPorts"] = []int{20002}
	call("PATCH", gpath, in, 409)
	call("DELETE", gpath, nil, 409)
	// State is fully resolved before it reaches older nodes.
	code, b, _ := r.agent.do("GET", "/api/agent/state", nil, nil)
	if code != 200 {
		t.Fatalf("state: %d %s", code, b)
	}
	state := mustJSON[agentproto.State](t, b)
	if len(state.Forwards) != 1 || state.Forwards[0].Listen != "10.10.0.2" || state.Forwards[0].Port != 20002 || state.Forwards[0].IngressID != "" {
		t.Fatal("forward state", state.Forwards)
	}
	if len(state.Node.Inbounds) != 1 || state.Node.Inbounds[0].Listen != "10.10.0.2" {
		t.Fatal("inbound state", state.Node.Inbounds)
	}
	// The second listener of mieru BOTH must not take the reserved next port.
	in["ReservedPorts"] = []int{20000, 20004}
	call("PATCH", gpath, in, 200)
	call("POST", path+"/inbounds", map[string]any{"Tag": "both", "Protocol": "mieru", "Port": 20003, "IngressID": g.ID, "Settings": map[string]any{"mieru_transport": "BOTH"}}, 400)
	// Invalid updates leave the saved forwarding list intact.
	fs, err := r.st.NodeForwards(context.Background(), r.nodeID)
	if err != nil || len(fs) != 1 || fs[0].Port != 20002 || fs[0].IngressID != itoa(g.ID) {
		t.Fatal("failed update changed forwards", fs, err)
	}
	call("DELETE", "/api/admin/inbounds/"+itoa(ib.ID), nil, 200)
	call("DELETE", gpath, nil, 409) // still referenced by the forward
	call("PUT", path+"/forwards", map[string]any{"Forwards": []any{}}, 200)
	call("DELETE", gpath, nil, 200)
	anon := &client{t: t, srv: r.srv}
	if code, _, _ := anon.do("POST", path+"/ingresses", in, nil); code != 401 {
		t.Fatal("unauthenticated ingress edit", code)
	}
}

func TestIngressInvalidMappingsAPI(t *testing.T) {
	r := newRig(t)
	for _, tc := range []struct {
		maps   []spec.PortMapping
		offset int
	}{
		{[]spec.PortMapping{{LocalFrom: 1, LocalTo: 2, PublicFrom: 65535}}, 0},
		{[]spec.PortMapping{{LocalFrom: 1, LocalTo: 2, PublicFrom: 3}, {LocalFrom: 2, LocalTo: 3, PublicFrom: 5}}, 0},
		{[]spec.PortMapping{{LocalFrom: 1, LocalTo: 2, PublicFrom: 3}, {LocalFrom: 5, LocalTo: 6, PublicFrom: 4}}, 0},
		{nil, 65535},
	} {
		code, b, _ := r.c.do("POST", "/api/admin/nodes/"+itoa(r.nodeID)+"/ingresses", map[string]any{"Name": "invalid", "EntryHost": "203.0.113.30", "PortMappings": tc.maps, "PortOffset": tc.offset}, nil)
		if code != 400 || strings.Contains(string(b), "internal error") {
			t.Fatalf("validation: %d %s", code, b)
		}
	}
}
