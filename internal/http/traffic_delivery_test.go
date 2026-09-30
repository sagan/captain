package http

import (
	"context"
	"testing"
	"time"

	"github.com/zeptop-dev/bosun/pkg/agentproto"
	"github.com/zeptop-dev/bosun/pkg/spec"
)

func TestTrafficReceiptRollsBackWithAccounting(t *testing.T) {
	r := newRig(t)
	uid, _ := r.user("traffic@example.com")
	rep := agentproto.Report{TrafficSeq: 31, Traffic: []spec.UserTraffic{{UserID: uid, Up: 900}}}
	if _, err := r.st.DB().Exec(`CREATE TRIGGER reject_traffic BEFORE INSERT ON traffic_daily BEGIN SELECT RAISE(FAIL, 'injected write failure'); END`); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := r.agent.do("POST", "/api/agent/report", rep, nil); code != 500 {
		t.Fatal("failed accounting acknowledged", code)
	}
	if _, err := r.st.DB().Exec(`DROP TRIGGER reject_traffic`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if code, b, _ := r.agent.do("POST", "/api/agent/report", rep, nil); code != 200 {
			t.Fatalf("retry: %d %s", code, b)
		}
	}
	sub, err := r.st.ActiveSubscription(context.Background(), uid)
	if err != nil || sub.UsedUpBytes != 900 {
		t.Fatalf("failed write consumed receipt: %+v %v", sub, err)
	}
}

func TestTrafficRetryDoesNotDuplicateNodeCounters(t *testing.T) {
	r := newRig(t)
	ibs, _ := r.st.InboundsByNode(context.Background(), r.nodeID)
	rep := agentproto.Report{TrafficSeq: 32, Inbounds: map[string]spec.Traffic{ibs[0].Tag: {Up: 123}}, Outbounds: map[string]spec.Traffic{"direct": {Down: 456}}}
	for range 2 {
		r.agent.do("POST", "/api/agent/report", rep, nil)
	}
	in, _ := r.st.InboundTrafficByNode(context.Background(), r.nodeID, time.Now())
	out, _ := r.st.OutboundTrafficByNode(context.Background(), r.nodeID, time.Now())
	if in[ibs[0].ID].Total != 123 || out["direct"].Total != 456 {
		t.Fatalf("recounted: %v %v", in, out)
	}
}

func TestTrafficEpochReplayAndCounterFailure(t *testing.T) {
	r := newRig(t)
	uid, _ := r.user("epoch@example.com")
	a := agentproto.Report{TrafficEpoch: "0123456789abcdef0123456789abcdef", TrafficSeq: 1, Traffic: []spec.UserTraffic{{UserID: uid, Up: 100}}, Outbounds: map[string]spec.Traffic{"direct": {Up: 100}}}
	if _, err := r.st.DB().Exec(`CREATE TRIGGER reject_outbound BEFORE INSERT ON outbound_traffic_daily BEGIN SELECT RAISE(FAIL, 'injected counter failure'); END`); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := r.agent.do("POST", "/api/agent/report", a, nil); code != 500 {
		t.Fatal("counter error acknowledged", code)
	}
	sub, _ := r.st.ActiveSubscription(context.Background(), uid)
	if sub.UsedUpBytes != 0 {
		t.Fatal("charge committed before counters", sub.UsedUpBytes)
	}
	if _, err := r.st.DB().Exec(`DROP TRIGGER reject_outbound`); err != nil {
		t.Fatal(err)
	}
	b := a
	b.TrafficEpoch = "abcdef0123456789abcdef0123456789"
	for _, rep := range []agentproto.Report{a, b, a, b} {
		if code, body, _ := r.agent.do("POST", "/api/agent/report", rep, nil); code != 200 {
			t.Fatalf("epoch: %d %s", code, body)
		}
	}
	sub, _ = r.st.ActiveSubscription(context.Background(), uid)
	if sub.UsedUpBytes != 200 {
		t.Fatal("epoch replay charged", sub.UsedUpBytes)
	}
	a.TrafficSeq = ^uint64(0)
	if code, _, _ := r.agent.do("POST", "/api/agent/report", a, nil); code != 400 {
		t.Fatal("overflow receipt accepted", code)
	}
}
