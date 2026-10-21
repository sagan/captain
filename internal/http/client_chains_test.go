package http

import (
	"context"
	"strings"
	"testing"

	"github.com/zeptop-dev/captain/internal/domain"
	"github.com/zeptop-dev/captain/internal/service"
)

func TestClientChainAPIAndAuthorizedSubscription(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	uid, path, _ := profileSetup(t, r)
	entries, err := r.st.ListEntries(ctx)
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
	front := entries[0]
	payload := map[string]any{"Name": "Chained", "InboundID": front.InboundID, "DisplayHost": "exit.example.com", "DisplayPort": 443, "ClientDialerEntryID": front.ID}
	visitor := &client{t: t, srv: r.srv}
	if code, _, _ := visitor.do("POST", "/api/admin/entries", payload, nil); code != 401 {
		t.Fatalf("unauthenticated creation returned %d", code)
	}
	code, body, _ := r.c.do("POST", "/api/admin/entries", payload, nil)
	if code != 200 {
		t.Fatal(code, string(body))
	}
	chain := mustJSON[domain.Entry](t, body)
	if chain.ClientDialerEntryID == nil || *chain.ClientDialerEntryID != front.ID {
		t.Fatal("reference not returned")
	}
	code, body, _ = r.c.do("GET", path+"?client=clash", nil, nil)
	if code != 200 || !strings.Contains(string(body), "dialer-proxy: Example") {
		t.Fatal(code, string(body))
	}
	u, _ := r.st.UserByID(ctx, uid)
	links, err := (&service.Subscription{Store: r.st}).EntryLinks(ctx, u)
	if err != nil {
		t.Fatal(err)
	}
	for _, link := range links {
		if link.EntryID == chain.ID && (link.URI != "" || !link.ClientChain) {
			t.Fatal("chain exposed a direct share URI")
		}
	}
	// Old clients that omit the new field must not turn the chain into direct.
	delete(payload, "ClientDialerEntryID")
	payload["Enabled"] = true
	code, body, _ = r.c.do("PATCH", "/api/admin/entries/"+itoa(chain.ID), payload, nil)
	if code != 200 || mustJSON[domain.Entry](t, body).ClientDialerEntryID == nil {
		t.Fatal(code, string(body))
	}
	code, body, _ = r.c.do("DELETE", "/api/admin/entries/"+itoa(front.ID), nil, nil)
	if code != 409 {
		t.Fatal("deleted shared front", code, string(body))
	}
	// A blocked front removes the dependent chain from the full subscription.
	if err := r.st.SetUserEntryBlocks(ctx, uid, []int64{front.ID}); err != nil {
		t.Fatal(err)
	}
	code, body, _ = r.c.do("GET", path+"?client=clash", nil, nil)
	if code != 200 || strings.Contains(string(body), "Chained") {
		t.Fatal("dependent leaked", code, string(body))
	}
	if err := r.st.SetUserEntryBlocks(ctx, uid, nil); err != nil {
		t.Fatal(err)
	}
	code, body, _ = r.c.do("GET", path+"?client=surge", nil, nil)
	if code != 200 || strings.Contains(string(body), "Chained") {
		t.Fatal("unsupported format fell back direct", code, string(body))
	}
	// Explicit null is how a new client removes the relationship.
	payload["ClientDialerEntryID"] = nil
	code, body, _ = r.c.do("PATCH", "/api/admin/entries/"+itoa(chain.ID), payload, nil)
	if code != 200 || mustJSON[domain.Entry](t, body).ClientDialerEntryID != nil {
		t.Fatal(code, string(body))
	}
}
