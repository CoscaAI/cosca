package pipeline

import (
	"context"
	"strings"
	"testing"
)

func TestDefaultPermissionPolicy(t *testing.T) {
	p := DefaultPermissionPolicy()
	ctx := context.Background()

	cases := []struct {
		name string
		pc   *PermissionContext
		want bool
	}{
		{"don can do anything", &PermissionContext{Role: "don", Action: ActionPlanApprove}, true},
		{"admin can do anything", &PermissionContext{Role: "admin", Action: ActionPluginManage}, true},
		{"editor executes", &PermissionContext{Role: "editor", Action: ActionPlanExecute}, true},
		{"editor reads", &PermissionContext{Role: "editor", Action: ActionPlanRead}, true},
		{"editor cannot approve", &PermissionContext{Role: "editor", Action: ActionPlanApprove}, false},
		{"viewer reads", &PermissionContext{Role: "viewer", Action: ActionPlanRead}, true},
		{"viewer cannot execute", &PermissionContext{Role: "viewer", Action: ActionPlanExecute}, false},
		{"unknown role denied", &PermissionContext{Role: "root", Action: ActionPlanRead}, false},
	}

	for _, tc := range cases {
		got, _ := p.Evaluate(ctx, tc.pc)
		if got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestPermissionPolicyNoRules(t *testing.T) {
	p := &PermissionPolicy{Name: "empty"}
	ok, reason := p.Evaluate(context.Background(), &PermissionContext{})
	if ok {
		t.Fatal("policy without rules must deny")
	}
	if !strings.Contains(reason, "has no rules") {
		t.Fatalf("unexpected reason: %q", reason)
	}
}

func TestPermissionEvaluator(t *testing.T) {
	ctx := context.Background()

	// No policies → allow all.
	ev := NewPermissionEvaluator()
	if ok, _ := ev.Authorize(ctx, &PermissionContext{}); !ok {
		t.Fatal("no policies must allow")
	}

	// AND semantics: all policies must pass.
	don := DefaultPermissionPolicy()
	agent := AgentPermissionPolicy()
	ev2 := NewPermissionEvaluator(don, agent)
	pc := &PermissionContext{
		Role:     "don",
		Action:   ActionPlanExecute,
		Metadata: map[string]interface{}{"agent_confidence": 0.9},
	}
	if ok, _ := ev2.Authorize(ctx, pc); !ok {
		t.Fatal("don with high confidence should pass all policies")
	}

	// One failing policy denies.
	pcLow := &PermissionContext{
		Role:     "don",
		Action:   ActionPlanExecute,
		Metadata: map[string]interface{}{"agent_confidence": 0.3},
	}
	if ok, reason := ev2.Authorize(ctx, pcLow); ok {
		t.Fatal("low confidence should be denied by agent policy")
	} else if reason == "" {
		t.Fatal("empty deny reason")
	}

	// AddPolicy appends.
	ev3 := NewPermissionEvaluator()
	ev3.AddPolicy(ResourcePermissionPolicy("db", "api"))
	if ok, _ := ev3.Authorize(ctx, &PermissionContext{Resource: "db"}); !ok {
		t.Fatal("resource policy should allow db")
	}
}

func TestAgentPermissionPolicy(t *testing.T) {
	p := AgentPermissionPolicy()
	ctx := context.Background()

	// Confidence >= 0.70 allows.
	ok, _ := p.Evaluate(ctx, &PermissionContext{Metadata: map[string]interface{}{"agent_confidence": 0.85}})
	if !ok {
		t.Fatal("confidence 0.85 should be allowed")
	}
	// Confidence < 0.70 denied.
	ok, _ = p.Evaluate(ctx, &PermissionContext{Metadata: map[string]interface{}{"agent_confidence": 0.5}})
	if ok {
		t.Fatal("confidence 0.5 should be denied")
	}
	// Missing metadata → denied.
	ok, _ = p.Evaluate(ctx, &PermissionContext{})
	if ok {
		t.Fatal("missing confidence metadata should be denied")
	}
	// Read-only always allowed.
	ok, _ = p.Evaluate(ctx, &PermissionContext{Action: ActionPlanRead})
	if !ok {
		t.Fatal("read-only action should be allowed regardless")
	}
}

func TestResourcePermissionPolicy(t *testing.T) {
	p := ResourcePermissionPolicy("API", "DB")
	ctx := context.Background()

	if ok, _ := p.Evaluate(ctx, &PermissionContext{Resource: "api"}); !ok {
		t.Fatal("case-insensitive resource should be allowed")
	}
	if ok, _ := p.Evaluate(ctx, &PermissionContext{Resource: "DB"}); !ok {
		t.Fatal("DB resource should be allowed")
	}
	if ok, _ := p.Evaluate(ctx, &PermissionContext{Resource: "storage"}); ok {
		t.Fatal("unlisted resource should be denied")
	}
}
