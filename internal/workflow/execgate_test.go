package workflow

import "testing"

func TestExecGate_LowRiskApprove(t *testing.T) {
	g := NewExecGate()
	v, reason := g.Evaluate(Proposal{Intent: "read file", Args: map[string]any{"path": "/etc/hosts"}, Ref: "node:1"})
	if v != VerdictApprove {
		t.Fatalf("leitura deveria ser Approve, got %s (%s)", v, reason)
	}
}

func TestExecGate_MediumNeedsApproval(t *testing.T) {
	g := NewExecGate()
	v, _ := g.Evaluate(Proposal{Intent: "update world state", Args: map[string]any{"x": 1}})
	if v != VerdictNeedsApproval {
		t.Fatalf("mutação de estado deveria ser NeedsApproval (gate do Don), got %s", v)
	}
}

func TestExecGate_HighRiskDenyFailClosed(t *testing.T) {
	g := NewExecGate() // DenyHighRisk=true por default
	for _, intent := range []string{"delete resource", "run shell", "upload to server", "create file"} {
		v, reason := g.Evaluate(Proposal{Intent: intent})
		if v != VerdictDeny {
			t.Fatalf("%q deveria ser Deny (fail-closed I2), got %s (%s)", intent, v, reason)
		}
	}
}

func TestExecGate_HighRiskCanBeEscalated(t *testing.T) {
	g := NewExecGate()
	g.DenyHighRisk = false // opt-out explícito: high-risk vira NeedsApproval
	v, _ := g.Evaluate(Proposal{Intent: "delete resource"})
	if v != VerdictNeedsApproval {
		t.Fatalf("DenyHighRisk=false deveria escalar para NeedsApproval, got %s", v)
	}
}

func TestExecGate_Deterministic(t *testing.T) {
	g := NewExecGate()
	a, _ := g.Evaluate(Proposal{Intent: "run shell"})
	b, _ := g.Evaluate(Proposal{Intent: "run shell"})
	if a != b {
		t.Fatal("gate deve ser determinístico (I1)")
	}
}
