package intelligence

import (
	"context"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/CoscaAI/cosca/internal/guardrails"
	"strings"
)

func src(id, topic, content string, ev int, conf float64, rec time.Time) Source {
	return Source{ID: id, Topic: topic, Content: content, Evidence: ev, Confidence: conf, Recency: rec}
}

// ============================================================
// CURRICULUM (determinÃƒÂ­stico)
// ============================================================

func TestPlan_OrdersByPriority(t *testing.T) {
	now := time.Now()
	eng := New(guardrails.DefaultDeps(), func(context.Context) ([]Source, error) {
		return []Source{
			src("a", "api", "rest", 2, 0.5, now.AddDate(0, -2, 0)),        // baixa
			src("b", "api", "grpc", 5, 0.9, now.Add(-2*24*time.Hour)),     // alta (evidÃƒÂªncia 5)
			src("c", "api", "graphql", 4, 0.8, now.Add(-20*24*time.Hour)), // mÃƒÂ©dia-alta
		}, nil
	})

	plan, err := eng.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	if len(plan.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(plan.Items))
	}
	// item "b" (evidÃƒÂªncia 5) deve vir primeiro
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
// CONFLITO (R6) Ã¢â‚¬â€ detecta, nÃƒÂ£o resolve
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
	// mesma conclusÃƒÂ£o (mesmo conteÃƒÂºdo) Ã¢â€ â€™ sem conflito
	incoming := []Source{
		src("new", "api", "use REST", 5, 0.9, now),
	}

	conflicts := eng.DetectConflicts(known, incoming, 0.3)
	if len(conflicts) != 0 {
		t.Fatalf("conclusÃƒÂµes iguais nÃƒÂ£o deveriam gerar conflito, got %d", len(conflicts))
	}
}

func TestDetectConflicts_DifferentTopicNoConflict(t *testing.T) {
	now := time.Now()
	eng := New(guardrails.DefaultDeps(), nil)

	known := []Source{src("old", "api", "use REST", 5, 0.9, now)}
	incoming := []Source{src("new", "banco", "use SQLite", 5, 0.9, now)}

	if c := eng.DetectConflicts(known, incoming, 0.3); len(c) != 0 {
		t.Fatalf("tÃƒÂ³picos diferentes nÃƒÂ£o deveriam conflitar, got %d", len(c))
	}
}

func TestDetectConflicts_DuplicateNotConflict(t *testing.T) {
	now := time.Now()
	eng := New(guardrails.DefaultDeps(), nil)

	// MESMO aprendizado gravado 2x (conteÃƒÂºdo ~idÃƒÂªntico, sÃƒÂ³ o ID muda)
	known := []Source{
		src("a/111", "devops", "Post-Commit Hook Execution #devops #git-hooks", 5, 0.9, now),
	}
	incoming := []Source{
		src("a/222", "devops", "Post-Commit Hook Execution #devops #git-hooks", 5, 0.9, now),
	}

	// Duplicata (sim > 0.85) NÃƒÆ’O ÃƒÂ© conflito
	conflicts := eng.DetectConflicts(known, incoming, 0.4)
	if len(conflicts) != 0 {
		t.Fatalf("duplicata nao deveria virar conflito, got %d", len(conflicts))
	}

	// Mas ÃƒÂ© detectada como DUPLICATA (para condensar R2)
	dups := eng.DetectDuplicates(known, incoming)
	if len(dups) == 0 {
		t.Fatal("esperava duplicata detectada")
	}
}

// ============================================================
// PROMOÃƒâ€¡ÃƒÆ’O (R4) Ã¢â‚¬â€ gateada
// ============================================================

func TestPromoteProposal_GatedWithoutDon(t *testing.T) {
	eng := New(guardrails.DefaultDeps(), nil)
	_, res := eng.PromoteProposal(Proposal{
		ID: "p1", Resource: "memory/agent/kernel/learnings.md",
		EvidenceLevel: 5, ApprovedByDon: false,
	})
	if res.Verdict.Approved {
		t.Fatal("promoÃƒÂ§ÃƒÂ£o sem gate do Don NÃƒÆ’O deveria ser aprovada")
	}
}

func TestPromoteProposal_ApprovedWithEvidenceAndSnapshot(t *testing.T) {
	eng := New(guardrails.DefaultDeps(), nil)
	prop, res := eng.PromoteProposal(Proposal{
		ID: "p2", Resource: "memory/agent/kernel/learnings.md",
		EvidenceLevel: 5, ApprovedByDon: true, HasSnapshot: true,
	})
	if !res.Verdict.Approved {
		t.Fatalf("promoÃƒÂ§ÃƒÂ£o legÃƒÂ­tima deveria passar, mas: %v", res.Verdict.Reasons)
	}
	if prop.Role != guardrails.RoleProposer {
		t.Errorf("expected proposer role, got %s", prop.Role)
	}
}

// ============================================================
// HELPERS determinÃƒÂ­sticos
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
		t.Error("recÃƒÂ©m-atualizado deveria pontuar 1.0")
	}
	if recencyScore(now.AddDate(0, -4, 0)) != 0.1 {
		t.Error("muito antigo (>90 dias) deveria pontuar 0.1")
	}
}

// ============================================================
// EMBEDDING â€” conflito por SIGNIFICADO (mais preciso que texto)
// ============================================================

// fakeEmbedder gera um vetor determinÃ­stico a partir de "conceitos" do texto.
// Representa significado: textos sobre o mesmo assunto tendem a vetores
// prÃ³ximos mesmo com palavras diferentes (sinÃ´nimo/parÃ¡frase).
func fakeEmbedder(text string) ([]float64, error) {
	v := make([]float64, 8)
	// conceito semÃ¢ntico "rest/api"
	if containsWord(text, "rest") || containsWord(text, "http") || containsWord(text, "endpoint") || containsWord(text, "api") {
		v[0] = 1
	}
	// conceito "grpc"
	if containsWord(text, "grpc") || containsWord(text, "protobuf") || containsWord(text, "stream") {
		v[1] = 1
	}
	// conceito "banco/sql"
	if containsWord(text, "sql") || containsWord(text, "query") || containsWord(text, "schema") {
		v[2] = 1
	}
	// conceito "cache"
	if containsWord(text, "cache") || containsWord(text, "redis") || containsWord(text, "ttl") {
		v[3] = 1
	}
	return v, nil
}

func containsWord(text, w string) bool {
	for _, tok := range tokenize(strings.ToLower(text)) {
		if tok == w {
			return true
		}
	}
	return false
}

// TestEmbedding_DetectsConflictByMeaning: dois textos com PALAVRAS diferentes
// (sinÃ´nimos/parafrase) mas MESMO SIGNIFICADO â€” Jaccard diz baixo, embedding
// (cosine) diz alto â†’ detecta o conflito que o texto nÃ£o pega.
func TestEmbedding_DetectsConflictByMeaning(t *testing.T) {
	now := time.Now()
	eng := New(guardrails.DefaultDeps(), nil)
	_ = eng.SetEmbedder(fakeEmbedder)

	now2 := time.Now()
	// "REST" vs "endpoint http" â€” palavras DIFERENTES, mesmo conceito (rest/api)
	known := []Source{src("a/111", "comunicacao", "use REST para servicos", 5, 0.9, now)}
	incoming := []Source{src("a/222", "comunicacao", "exposta por endpoint http", 5, 0.9, now2)}

	jac := similarityApprox(known[0].Content, incoming[0].Content)
	embSim := eng.similarity(known[0].Content, incoming[0].Content)
	t.Logf("Jaccard=%.2f | embedding=%.2f", jac, embSim)

	if jac >= 0.6 {
		t.Skip("Jaccard ja detecta (nao demonstra o valor do embedding)")
	}
	if embSim <= jac {
		t.Errorf("embedding deveria dar similaridade maior que Jaccard para mesmo conceito (jac=%.2f, emb=%.2f)", jac, embSim)
	}
}

// TestEmbedding_DifferentConceptLowSimilarity: conceitos DISTINTOS â†’ cosine baixo.
func TestEmbedding_DifferentConceptLowSimilarity(t *testing.T) {
	now := time.Now()
	eng := New(guardrails.DefaultDeps(), nil)
	_ = eng.SetEmbedder(fakeEmbedder)

	a := src("a/111", "x", "use REST para servicos http", 5, 0.9, now)
	b := src("a/222", "x", "otimizacao de cache redis ttl", 5, 0.9, now)

	// mesmo que compartilhem a palavra "para", conceitos diferentes -> cosine baixo
	if sim := eng.similarity(a.Content, b.Content); sim >= 0.5 {
		t.Errorf("conceitos distintos deveriam ter cosine baixo, got %.2f", sim)
	}
}

// ensure embeddings import Ã© usado (referÃªncia indireta)
var _ = embeddings.CosineSimilarity
