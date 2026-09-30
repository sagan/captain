package http

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/captain/internal/http/admin"
	"github.com/zeptop-dev/captain/internal/store"
)

func TestNodeRemovalLifecycle(t *testing.T) {
	for _, mode := range []string{"standalone", "uninstall"} {
		t.Run(mode, func(t *testing.T) {
			r := newRig(t)
			path := "/api/admin/nodes/" + itoa(r.nodeID)
			r.agent.do("POST", "/api/agent/report", agentproto.Report{Version: "v0.55.0"}, nil)
			code, b, _ := r.c.do("POST", path+"/removal", map[string]any{"mode": mode, "keep_data": true}, nil)
			if code != 200 {
				t.Fatalf("queue: %d %s", code, b)
			}
			id := mustJSON[map[string]string](t, b)["id"]
			if code, _, _ := r.c.do("POST", path+"/removal", map[string]string{"mode": mode}, nil); code != 409 {
				t.Fatalf("duplicate: %d", code)
			}
			_, b, _ = r.agent.do("GET", "/api/agent/state", nil, nil)
			st := mustJSON[agentproto.State](t, b)
			if len(st.Jobs) != 1 || st.Jobs[0].ID != id {
				t.Fatalf("job not delivered: %+v", st.Jobs)
			}
			if code, _, _ := r.agent.do("POST", "/api/agent/removal", map[string]string{"id": id, "phase": "complete"}, nil); code != 409 {
				t.Fatalf("unclaimed completion: %d", code)
			}
			if code, _, _ := r.c.do("DELETE", path+"?removal_job="+id, nil, nil); code != 409 {
				t.Fatalf("early deletion: %d", code)
			}
			// A normal report cannot claim successful completion, even with a wrong kind.
			r.agent.do("POST", "/api/agent/report", agentproto.Report{Jobs: []agentproto.JobResult{{ID: id, Kind: "other", Result: json.RawMessage(`{"phase":"complete"}`)}}}, nil)
			j, _ := r.st.NodeJob(context.Background(), r.nodeID, id)
			if j.DoneAt != nil {
				t.Fatal("normal report completed removal")
			}
			if code, b, _ := r.agent.do("POST", "/api/agent/removal", map[string]string{"id": id, "phase": "running"}, nil); code != 200 {
				t.Fatalf("claim: %d %s", code, b)
			}
			// A restarted agent may report a launch error while the worker is running.
			r.agent.do("POST", "/api/agent/report", agentproto.Report{Jobs: []agentproto.JobResult{{ID: id, Kind: "node_remove", Error: "late launch error"}}}, nil)
			j, _ = r.st.NodeJob(context.Background(), r.nodeID, id)
			if j.DoneAt != nil {
				t.Fatal("late report overwrote running worker")
			}
			if code, b, _ := r.agent.do("POST", "/api/agent/removal", map[string]any{"id": id, "phase": "complete", "listen": "127.0.0.1:2053", "login_setup": true}, nil); code != 200 {
				t.Fatalf("complete: %d %s", code, b)
			}
			// Delivery can retry, but a finished job cannot be executed again.
			if code, _, _ := r.agent.do("POST", "/api/agent/removal", map[string]string{"id": id, "phase": "complete"}, nil); code != 200 {
				t.Fatalf("retry result: %d", code)
			}
			if code, _, _ := r.agent.do("POST", "/api/agent/removal", map[string]string{"id": id, "phase": "running"}, nil); code != 409 {
				t.Fatalf("replayed work: %d", code)
			}
			_, b, _ = r.c.do("GET", path+"/removal", nil, nil)
			got := mustJSON[store.NodeJob](t, b)
			if got.DoneAt == nil || got.Error != "" {
				t.Fatalf("completion not visible: %+v", got)
			}
			// Successful result remains available even if another ordinary job is queued.
			if err := r.st.CreateNodeJob(context.Background(), "diagnostic", r.nodeID, "doctor", nil); err != nil {
				t.Fatal(err)
			}
			if code, _, _ := r.c.do("DELETE", path+"?removal_job="+id, nil, nil); code != 200 {
				t.Fatalf("delete: %d", code)
			}
			if _, err := r.st.NodeByID(context.Background(), r.nodeID); !errors.Is(err, store.ErrNotFound) {
				t.Fatalf("node remains: %v", err)
			}
			if code, _, _ := r.agent.do("GET", "/api/agent/state", nil, nil); code != 401 {
				t.Fatalf("old node credential: %d", code)
			}
		})
	}
}

func TestNodeRemovalGuardsAndFailure(t *testing.T) {
	r := newRig(t)
	path := "/api/admin/nodes/" + itoa(r.nodeID) + "/removal"
	body := map[string]string{"mode": "uninstall"}
	op, _ := admin.NewUser("operator@example.com", "password123", "operator")
	if err := r.st.CreateStaff(context.Background(), op); err != nil {
		t.Fatal(err)
	}
	operator := &client{t: t, srv: r.srv}
	operator.do("POST", "/api/admin/login", map[string]string{"Email": op.Email, "Password": "password123"}, nil)
	if code, _, _ := operator.do("POST", path, body, nil); code != 403 {
		t.Fatalf("operator may uninstall: %d", code)
	}
	if code, _, _ := r.c.do("POST", path, body, map[string]string{"Origin": "https://evil.example.com"}); code != 403 {
		t.Fatalf("cross-origin removal: %d", code)
	}
	for _, version := range []string{"v0.54.0", "dev"} {
		r.agent.do("POST", "/api/agent/report", agentproto.Report{Version: version}, nil)
		if code, _, _ := r.c.do("POST", path, body, nil); code != 409 {
			t.Fatalf("old version %s: %d", version, code)
		}
	}
	r.agent.do("POST", "/api/agent/report", agentproto.Report{Version: "v0.55.0"}, nil)
	if _, err := r.st.DB().Exec(`UPDATE nodes SET last_seen_at = ? WHERE id = ?`, time.Now().Add(-4*time.Minute).Unix(), r.nodeID); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := r.c.do("POST", path, body, nil); code != 409 {
		t.Fatalf("offline removal: %d", code)
	}
	r.agent.do("POST", "/api/agent/report", agentproto.Report{Version: "v0.55.0"}, nil)
	if code, _, _ := r.c.do("POST", path, map[string]string{"mode": "shell"}, nil); code != 400 {
		t.Fatalf("bad mode: %d", code)
	}
	_, b, _ := r.c.do("POST", path, body, nil)
	id := mustJSON[map[string]string](t, b)["id"]
	// The owning node cannot finish another node's job.
	_, b, _ = r.c.do("POST", "/api/admin/nodes", map[string]string{"Name": "other"}, nil)
	other := int64(mustJSON[map[string]any](t, b)["id"].(float64))
	if err := r.st.QueueNodeRemoval(context.Background(), "other-job", other, store.NodeRemovalParams{Mode: "uninstall", ExpiresAt: time.Now().Add(time.Minute).Unix()}); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := r.agent.do("POST", "/api/agent/removal", map[string]string{"id": "other-job", "phase": "running"}, nil); code != 404 {
		t.Fatalf("cross-node claim: %d", code)
	}
	r.agent.do("POST", "/api/agent/report", agentproto.Report{Jobs: []agentproto.JobResult{{ID: id, Kind: "node_remove", Error: "unsupported installation"}}}, nil)
	j, _ := r.st.NodeJob(context.Background(), r.nodeID, id)
	if j.DoneAt == nil || j.Error == "" {
		t.Fatal("launch failure not persisted")
	}
	if code, _, _ := r.c.do("DELETE", "/api/admin/nodes/"+itoa(r.nodeID)+"?removal_job="+id, nil, nil); code != 409 {
		t.Fatalf("failed operation deletion: %d", code)
	}
	if _, err := r.st.NodeByID(context.Background(), r.nodeID); err != nil {
		t.Fatal("failure deleted node")
	}
	_, b, _ = r.c.do("POST", path, body, nil)
	id = mustJSON[map[string]string](t, b)["id"]
	r.agent.do("POST", "/api/agent/removal", map[string]string{"id": id, "phase": "running"}, nil)
	r.agent.do("POST", "/api/agent/removal", map[string]string{"id": id, "phase": "complete", "error": "service stop failed"}, nil)
	if code, _, _ := r.c.do("DELETE", "/api/admin/nodes/"+itoa(r.nodeID)+"?removal_job="+id, nil, nil); code != 409 {
		t.Fatalf("worker failure deletion: %d", code)
	}
	// Legacy record-only removal stays available for offline/unsupported nodes.
	if code, _, _ := r.c.do("DELETE", "/api/admin/nodes/"+itoa(r.nodeID), nil, nil); code != 200 {
		t.Fatalf("record-only compatibility: %d", code)
	}
}

func TestNodeRemovalExpiry(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	path := "/api/admin/nodes/" + itoa(r.nodeID) + "/removal"
	past := store.NodeRemovalParams{Mode: "uninstall", ExpiresAt: time.Now().Add(-time.Minute).Unix()}
	if err := r.st.QueueNodeRemoval(ctx, "expired", r.nodeID, past); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := r.agent.do("POST", "/api/agent/removal", map[string]string{"id": "expired", "phase": "running"}, nil); code != 409 {
		t.Fatalf("expired claim: %d", code)
	}
	_, b, _ := r.c.do("GET", path, nil, nil)
	j := mustJSON[store.NodeJob](t, b)
	if j.DoneAt == nil || j.Error == "" {
		t.Fatal("expiry not visible")
	}
	// GET is read-only. The next queue transaction retires the stale job.
	old, _ := r.st.NodeJob(ctx, r.nodeID, "expired")
	if old.DoneAt != nil {
		t.Fatal("GET modified state")
	}
	future := past
	future.ExpiresAt = time.Now().Add(time.Minute).Unix()
	if err := r.st.QueueNodeRemoval(ctx, "fresh", r.nodeID, future); err != nil {
		t.Fatal(err)
	}
	old, _ = r.st.NodeJob(ctx, r.nodeID, "expired")
	if old.DoneAt == nil {
		t.Fatal("stale job not retired")
	}
	if err := r.st.ClaimNodeRemoval(ctx, r.nodeID, "fresh", time.Now()); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(past)
	if _, err := r.st.DB().Exec(`UPDATE node_jobs SET params_json = ? WHERE id = ?`, string(raw), "fresh"); err != nil {
		t.Fatal(err)
	}
	if err := r.st.QueueNodeRemoval(ctx, "unsafe", r.nodeID, future); !errors.Is(err, store.ErrRemovalPending) {
		t.Fatalf("running job replaced after expiry: %v", err)
	}
}
