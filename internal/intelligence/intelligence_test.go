package intelligence

import (
	"context"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/guardrails"
)

func src(id, topic, content string, ev int, conf float64, rec time.Time) Source {
	return Source{ID: id, Topic: topic, Content: content, Evidence: ev, Confidence: conf, Recency: rec}
}

// ============================================================
// CURRICULUM (determinístico)
// ============================================================

func TestPlan_OrdersByPriority(t *testing.T) {
	now := time.Now()
	eng := New(guardrails.DefaultDeps(), func(context.Context) ([]Source, error) {
		return []Source{
			src("a", "api", "rest", 2, 0.5, now.AddDate(0, -2, 0)),        // baixa
			src("b", "api", "grpc", 5, 0.9, now.Add(-2*24*time.Hour)),     // alta (evidência 5)
			src("c", "api", "graphql", 4, 0.8, now.Add(-20*24*time.Hour)), // média-alta
		}, nil
	})

	plan, err := eng.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	if len(plan.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(plan.Items))
	}
	// item "b" (evidência 5) deve vir primeiro
	if plan.Items[0].SourceID != "b" {
		t.Errorf("expected 'b' first (highest evidence), got %s", plan.Items[0].SourceID)
	}
	if plan.Items[len(plan.Items)-1].Priority > plan.Items[0].Priority {
		t.Error("priority should be descending")
	}
}

func TestPlan_NoSources(t *testing.T) {
	eng := New(guardrails.DefaultDeps(), func(context.Context) ([]Source, error) {
		return nil, nil
	})
	plan, err := eng.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	if len(plan.Items) != 0 {
		t.Errorf("expected 0 items when no sources, got %d", len(plan.Items))
	}
}

// ============================================================
// CONFLITO (R6) — detecta, não resolve
// ============================================================

func TestDetectConflicts_SignalsConflict(t *testing.T) {
	now := time.Now()
	eng := New(guardrails.DefaultDeps(), nil)

	known := []Source{
		src("old", "transporte", "use REST para servicos", 2, 0.5, now),
	}
	incoming := []Source{
		src("new", "transporte", "use gRPC para servicos internos", 5, 0.9, now),
	}

	conflicts := eng.DetectConflicts(known, incoming, 0.3)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].OldResource != "old" || conflicts[0].NewResource != "new" {
		t.Errorf("unexpected conflict pairing: %+v", conflicts[0])
	}
}

func TestDetectConflicts_SameConclusionNoConflict(t *testing.T) {
	now := time.Now()
	eng := New(guardrails.DefaultDeps(), nil)

	known := []Source{
		src("old", "api", "use REST", 2, 0.5, now),
	}
	// mesma conclusão (mesmo conteúdo) → sem conflito
	incoming := []Source{
		src("new", "api", "use REST", 5, 0.9, now),
	}

	conflicts := eng.DetectConflicts(known, incoming, 0.3)
	if len(conflicts) != 0 {
		t.Fatalf("conclusões iguais não deveriam gerar conflito, got %d", len(conflicts))
	}
}

func TestDetectConflicts_DifferentTopicNoConflict(t *testing.T) {
	now := time.Now()
	eng := New(guardrails.DefaultDeps(), nil)

	known := []Source{src("old", "api", "use REST", 5, 0.9, now)}
	incoming := []Source{src("new", "banco", "use SQLite", 5, 0.9, now)}

	if c := eng.DetectConflicts(known, incoming, 0.3); len(c) != 0 {
		t.Fatalf("tópicos diferentes não deveriam conflitar, got %d", len(c))
	}
}

// ============================================================
// PROMOÇÃO (R4) — gateada
// ============================================================

func TestPromoteProposal_GatedWithoutDon(t *testing.T) {
	eng := New(guardrails.DefaultDeps(), nil)
	_, res := eng.PromoteProposal(Proposal{
		ID: "p1", Resource: "memory/agent/kernel/learnings.md",
		EvidenceLevel: 5, ApprovedByDon: false,
	})
	if res.Verdict.Approved {
		t.Fatal("promoção sem gate do Don NÃO deveria ser aprovada")
	}
}

func TestPromoteProposal_ApprovedWithEvidenceAndSnapshot(t *testing.T) {
	eng := New(guardrails.DefaultDeps(), nil)
	prop, res := eng.PromoteProposal(Proposal{
		ID: "p2", Resource: "memory/agent/kernel/learnings.md",
		EvidenceLevel: 5, ApprovedByDon: true, HasSnapshot: true,
	})
	if !res.Verdict.Approved {
		t.Fatalf("promoção legítima deveria passar, mas: %v", res.Verdict.Reasons)
	}
	if prop.Role != guardrails.RoleProposer {
		t.Errorf("expected proposer role, got %s", prop.Role)
	}
}

// ============================================================
// HELPERS determinísticos
// ============================================================

func TestSimilarityApprox(t *testing.T) {
	if similarityApprox("use REST para servicos", "use GRPC para servicos") <= 0 {
		t.Error("similaridade de textos relacionados deveria ser > 0")
	}
	if similarityApprox("abc", "xyz") != 0 {
		t.Error("textos sem overlap deveriam ter similaridade 0")
	}
}

func TestRecencyScore(t *testing.T) {
	now := time.Now()
	if recencyScore(now) != 1.0 {
		t.Error("recém-atualizado deveria pontuar 1.0")
	}
	if recencyScore(now.AddDate(0, -4, 0)) != 0.1 {
		t.Error("muito antigo (>90 dias) deveria pontuar 0.1")
	}
}
