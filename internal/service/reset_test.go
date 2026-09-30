package service

import (
	"testing"
	"time"

	"github.com/zeptop-dev/captain/internal/store"
)

func TestResetDiscardsOldAccountAndNodeState(t *testing.T) {
	at := time.Now()
	state := &AgentState{}
	if !state.withhold(map[int64]bool{1: true}, at)[1] {
		t.Fatal("hold not set")
	}
	state.Reset()
	if state.withhold(map[int64]bool{}, at)[1] {
		t.Fatal("new customer 1 inherited the old device hold")
	}
	limit := &DynLimit{recent: map[int64][]sample{1: {{bytes: 1000, win: time.Minute, end: at}}}, settings: store.DynLimitSettings{Enabled: true}, fetched: at}
	limit.Reset()
	if len(limit.recent[1]) != 0 {
		t.Fatal("new customer 1 inherited the old traffic window")
	}
	probe := &Probe{live: map[int64]*Live{1: {At: at}}, pending: []probeNotice{{Text: "old node alert", NodeID: 1}}, started: at}
	gen := probe.Gen()
	probe.Reset()
	if len(probe.live) != 0 || len(probe.pending) != 0 || probe.Gen() == gen {
		t.Fatal("new node 1 inherited monitoring data")
	}
}
