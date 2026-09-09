package intelligence

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/guardrails"
)

// ============================================================
// MemoryProvider — conecta o motor ao conhecimento REAL
// ============================================================

func TestMemoryProvider_ReadsRealLearnings(t *testing.T) {
	// USAR os learnings.md REAIS da casa (não mock)
	memAgentDir := filepath.Join("..", "..", ".cosca", "memory")
	if _, err := os.Stat(filepath.Join(memAgentDir, "agent")); err != nil {
		t.Skipf("memória real não disponível: %v", err)
	}

	p := MemoryProvider(memAgentDir)
	srcs, err := p(context.Background())
	if err != nil {
		t.Fatalf("MemoryProvider error: %v", err)
	}
	if len(srcs) == 0 {
		t.Fatal("esperava ler learnings reais da casa")
	}

	// valida que cada source tem conteúdo e tópico
	seen := map[string]bool{}
	for _, s := range srcs {
		if s.Content == "" {
			t.Errorf("source %s sem conteúdo", s.ID)
		}
		if s.Topic == "" {
			t.Errorf("source %s sem tópico", s.ID)
		}
		if s.ID == "" {
			t.Errorf("source sem ID")
		}
		seen[s.ID] = true
	}
	t.Logf("lid %d learnings reais da casa", len(srcs))
}

func TestMemoryProvider_PlansOverRealKnowledge(t *testing.T) {
	memAgentDir := filepath.Join("..", "..", ".cosca", "memory")
	if _, err := os.Stat(filepath.Join(memAgentDir, "agent")); err != nil {
		t.Skipf("memória real não disponível: %v", err)
	}

	eng := New(DefaultGuardDeps(), MemoryProvider(memAgentDir))
	plan, err := eng.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	if len(plan.Items) == 0 {
		t.Fatal("esperava curriculum a partir da memória real")
	}
	// itens devem estar ordenados por prioridade descendente
	for i := 1; i < len(plan.Items); i++ {
		if plan.Items[i].Priority > plan.Items[i-1].Priority {
			t.Errorf("curriculum não ordenado em %d", i)
		}
	}
	t.Logf("curriculum com %d itens; top: %s (%.2f)", len(plan.Items), plan.Items[0].SourceID, plan.Items[0].Priority)
}

// ============================================================
// parse de uma linha de índice (determinístico)
// ============================================================

func TestLineToSource_ParsesRealIndexLine(t *testing.T) {
	line := "## 2026-07-30 | 2026-07-30 | 2026-07-30 — Cognitive Compression Engine — Spec Completa (Fase 3) | Level 4 | L | #cognitive-compression #fase-3 #principles | d78199d289c517f0"
	s := lineToSource("cosca-ai", line)

	if s.Topic != "cognitive-compression/fase-3" {
		t.Errorf("expected topic 'cognitive-compression/fase-3' (subtag), got %q", s.Topic)
	}
	if s.Evidence != 5 {
		t.Errorf("expected evidence 5 (hash de provenance), got %d", s.Evidence)
	}
	if s.Confidence != 0.8 {
		t.Errorf("expected confidence 0.8 (Level 4), got %.2f", s.Confidence)
	}
	if s.ID != "cosca-ai/d78199d289c517f0" {
		t.Errorf("expected ID com hash, got %q", s.ID)
	}
}

func TestLineToSource_NoHashLowerEvidence(t *testing.T) {
	line := "## 2026-07-28 | L | #graph #code-imports | 1234"
	s := lineToSource("cosca-ai", line)
	if s.Evidence != 3 {
		t.Errorf("expected evidence 3 (sem hash completo), got %d", s.Evidence)
	}
	// só 1 tag de domínio + agente não detectável -> topic = primeira palavra
	if s.Topic != "graph/code-imports" {
		t.Errorf("expected topic 'graph/code-imports' (subtag), got %q", s.Topic)
	}
}

// DefaultGuardDeps: reusa a mesma Deps do guardrails (para os testes do engine
// sobre dados reais).
func DefaultGuardDeps() guardrails.Deps {
	return guardrails.DefaultDeps()
}
