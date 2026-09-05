package pipeline

import (
	"strings"
	"testing"
)

func TestNewDefaultPolicy(t *testing.T) {
	p := NewDefaultPolicy()
	if !p.AutoApproveRead || !p.AutoApproveSafeWrite || !p.ConfirmDangerous ||
		!p.ConfirmSystem || !p.BlockIrreversible {
		t.Fatalf("default policy flags wrong: %+v", p)
	}
	if p.AmbiguityThreshold != 0.70 || p.HighRiskThreshold != OpSystem || p.MaxAutonomousAttempts != 3 {
		t.Fatalf("default policy thresholds wrong: %+v", p)
	}
}

func TestInteractionPolicyEvaluate(t *testing.T) {
	p := NewDefaultPolicy()

	// Irreversible → must ask, always.
	d := p.Evaluate(OpIrreversible, 1.0, 0)
	if !d.ShouldAsk || !strings.Contains(d.Reason, "irreversible") {
		t.Fatalf("irreversible decision wrong: %+v", d)
	}
	if len(d.Options) != 2 {
		t.Fatalf("irreversible options = %v", d.Options)
	}

	// System → must ask.
	d = p.Evaluate(OpSystem, 1.0, 0)
	if !d.ShouldAsk || !strings.Contains(d.Reason, "system-level") {
		t.Fatalf("system decision wrong: %+v", d)
	}

	// Dangerous → must ask.
	d = p.Evaluate(OpDangerous, 1.0, 0)
	if !d.ShouldAsk || !strings.Contains(d.Reason, "destructive") {
		t.Fatalf("dangerous decision wrong: %+v", d)
	}

	// High risk + low confidence → ask.
	d = p.Evaluate(OpSystem, 0.3, 0)
	if !d.ShouldAsk {
		t.Fatalf("high-risk low-confidence should ask: %+v", d)
	}

	// Max autonomous attempts exceeded → ask.
	d = p.Evaluate(OpRead, 0.9, 5)
	if !d.ShouldAsk || !strings.Contains(d.Reason, "recovery attempts") {
		t.Fatalf("max attempts decision wrong: %+v", d)
	}

	// Low confidence on modification → ask.
	d = p.Evaluate(OpModify, 0.4, 0)
	if !d.ShouldAsk {
		t.Fatalf("low-confidence modify should ask: %+v", d)
	}

	// Read auto-approves.
	d = p.Evaluate(OpRead, 0.9, 0)
	if d.ShouldAsk || d.AutoChoice != "proceed" {
		t.Fatalf("read should auto-approve: %+v", d)
	}

	// Safe write auto-approves with default policy.
	d = p.Evaluate(OpWrite, 0.9, 0)
	if d.ShouldAsk {
		t.Fatalf("safe write should auto-approve: %+v", d)
	}

	// Without AutoApproveSafeWrite, a plain write still falls through to the
	// policy's final default decision (ShouldAsk=false) — only the explicit
	// read/write fast-path is gated by the flag.
	p2 := NewDefaultPolicy()
	p2.AutoApproveSafeWrite = false
	d = p2.Evaluate(OpWrite, 0.9, 0)
	if d.ShouldAsk {
		t.Fatalf("write without AutoApproveSafeWrite falls back to default proceed: %+v", d)
	}
}

func TestInteractionPolicyPolicyOff(t *testing.T) {
	p := &InteractionPolicy{}
	p.MaxAutonomousAttempts = 100 // zero-value 0 would trigger the "attempts exceeded" gate
	// All confirm flags off → dangerous falls through to final proceed.
	d := p.Evaluate(OpDangerous, 0.5, 0)
	if d.ShouldAsk {
		t.Fatalf("policy off should not ask for dangerous: %+v", d)
	}
}

func TestShouldAskHuman(t *testing.T) {
	p := NewDefaultPolicy()

	// Nil task → proceed.
	if d := p.ShouldAskHuman(nil, nil); d.ShouldAsk {
		t.Fatal("nil task should not ask")
	}

	// Dangerous task description → ask.
	delTask := &TaskNode{ID: "t", Description: "delete all files"}
	if d := p.ShouldAskHuman(delTask, nil); !d.ShouldAsk {
		t.Fatal("delete task should ask")
	}

	// Read task → proceed.
	readTask := &TaskNode{ID: "t", Description: "list files"}
	if d := p.ShouldAskHuman(readTask, nil); d.ShouldAsk {
		t.Fatal("read task should not ask")
	}
}

func TestClassifyTaskOp(t *testing.T) {
	if classifyTaskOp(nil) != OpRead {
		t.Fatal("nil task → OpRead")
	}
	cases := []struct {
		desc string
		want OpLevel
	}{
		{"delete the cache", OpDangerous},
		{"drop database", OpDangerous},
		{"deploy to production", OpSystem},
		{"install dependencies", OpSystem},
		{"create new endpoint", OpWrite},
		{"generate models", OpWrite},
		{"update configuration", OpModify},
		{"refactor the module", OpModify},
		{"read the logs", OpRead},
		{"run tests", OpRead},
		{"analyze coverage", OpRead},
		{"something else", OpModify},
	}
	for _, tc := range cases {
		if got := classifyTaskOp(&TaskNode{Description: tc.desc}); got != tc.want {
			t.Errorf("classifyTaskOp(%q) = %v, want %v", tc.desc, got, tc.want)
		}
	}
}

func TestContainsWord(t *testing.T) {
	if !containsWord("Delete files", "delete") {
		t.Fatal("case-insensitive match failed")
	}
	if containsWord("updated files", "delete") {
		t.Fatal("false positive for non-matching word")
	}
}
