package contextcompile

import (
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/orchestration"
)

// TestCompile_StructuredSections: o contexto compilado tem as seções canônicas
// com o texto esperado.
func TestCompile_StructuredSections(t *testing.T) {
	cc := Compile(CompileInput{
		Prompt: "corrigir endpoint MCP",
		Agent:  "Backend API Specialist",
		Role:   "backend",
		Knowledge: &orchestration.KnowledgeSearchResults{
			Results: []orchestration.KnowledgeSearchResult{
				{ID: "a", Content: "server exposes 11 tools", Score: 0.92, Epistemic: "FACT"},
				{ID: "b", Content: "generic_mcp advertises 6 obsolete tools", Score: 0.61, Epistemic: "INFERRED"},
			},
		},
		Constraints: []string{"não criar contratos duplicados", "preservar API existente"},
		Target:      "internal/mcpserver/registry.go",
	})

	if !strings.Contains(cc.Text, "TASK:") || !strings.Contains(cc.Text, "corrigir endpoint MCP") {
		t.Fatal("seção TASK ausente ou sem conteúdo")
	}
	if !strings.Contains(cc.Text, "STATE:") || !strings.Contains(cc.Text, "Backend API Specialist") {
		t.Fatal("seção STATE ausente ou sem agente")
	}
	if !strings.Contains(cc.Text, "CONSTRAINTS:") || !strings.Contains(cc.Text, "não criar contratos duplicados") {
		t.Fatal("seção CONSTRAINTS ausente ou sem conteúdo")
	}
	if !strings.Contains(cc.Text, "FACTS:") || !strings.Contains(cc.Text, "[FACT]") {
		t.Fatal("seção FACTS ausente ou sem epistemologia")
	}
	if !strings.Contains(cc.Text, "EVIDENCE:") || !strings.Contains(cc.Text, "[INFERRED]") {
		t.Fatal("seção EVIDENCE ausente ou sem epistemologia")
	}
	if !strings.Contains(cc.Text, "TARGET:") || !strings.Contains(cc.Text, "internal/mcpserver/registry.go") {
		t.Fatal("seção TARGET ausente ou sem alvo")
	}
	if !strings.Contains(cc.Text, "OUTPUT_CONTRACT:") || !strings.Contains(cc.Text, "instruction packet") {
		t.Fatal("OUTPUT_CONTRACT ausente ou sem o contrato de resposta")
	}
}

// TestCompile_FactsSeparatedFromEvidence: score alto → FACTS, score baixo →
// EVIDENCE (a fronteira do que é fato vs indício).
func TestCompile_FactsSeparatedFromEvidence(t *testing.T) {
	cc := Compile(CompileInput{
		Prompt: "teste de separação",
		Knowledge: &orchestration.KnowledgeSearchResults{
			Results: []orchestration.KnowledgeSearchResult{
				{ID: "hi", Content: "fato de alta confiança", Score: 0.95},
				{ID: "lo", Content: "indício de média confiança", Score: 0.55},
			},
		},
	})

	// O fato aparece em FACTS, não em EVIDENCE.
	factsIdx := strings.Index(cc.Text, "FACTS:")
	evidIdx := strings.Index(cc.Text, "EVIDENCE:")
	if factsIdx == -1 || evidIdx == -1 {
		t.Fatal("FACTS ou EVIDENCE ausentes")
	}
	if !strings.Contains(cc.Text[factsIdx:evidIdx], "fato de alta confiança") {
		t.Fatal("fato de alta confiança deveria estar em FACTS")
	}
	if strings.Contains(cc.Text[factsIdx:evidIdx], "indício de média confiança") {
		t.Fatal("indício de média confiança NÃO deveria estar em FACTS")
	}
}

// TestCompile_EmptyInputFailOpen: input mínimo → contexto ainda tem TASK e
// OUTPUT_CONTRACT (o contrato de resposta SEMPRE presente).
func TestCompile_EmptyInputFailOpen(t *testing.T) {
	cc := Compile(CompileInput{Prompt: "oi"})
	if !strings.Contains(cc.Text, "TASK:") {
		t.Fatal("TASK ausente com prompt presente")
	}
	if !strings.Contains(cc.Text, "OUTPUT_CONTRACT:") {
		t.Fatal("OUTPUT_CONTRACT deveria estar sempre presente")
	}
}

// TestCompile_AllowedForbiddenSpace: ALLOWED/FORBIDDEN delimitam o espaço de
// ação quando fornecidos.
func TestCompile_AllowedForbiddenSpace(t *testing.T) {
	cc := Compile(CompileInput{
		Prompt:    "editar arquivo",
		Allowed:   []string{"editar arquivos existentes", "rodar testes"},
		Forbidden: []string{"criar implementação paralela"},
	})
	if !strings.Contains(cc.Text, "ALLOWED: editar arquivos existentes, rodar testes") {
		t.Fatal("ALLOWED ausente ou com conteúdo errado")
	}
	if !strings.Contains(cc.Text, "FORBIDDEN: criar implementação paralela") {
		t.Fatal("FORBIDDEN ausente ou com conteúdo errado")
	}
}

// TestCompile_MemorySection: memórias recuperadas entram na seção MEMORY.
func TestCompile_MemorySection(t *testing.T) {
	cc := Compile(CompileInput{
		Prompt: "lembrar contexto",
		Memory: []orchestration.MemoryRecord{
			{ID: "m1", Type: "fact", Layer: "long_term", Content: "decisão anterior sobre registry"},
		},
	})
	if !strings.Contains(cc.Text, "MEMORY:") || !strings.Contains(cc.Text, "decisão anterior sobre registry") {
		t.Fatal("MEMORY ausente ou sem conteúdo")
	}
	if !strings.Contains(cc.Text, "fact/long_term") {
		t.Fatal("MEMORY sem type/layer da record")
	}
}

// TestSections_BudgetSums: o orçamento registrado das seções presentes soma 100
// (base da F3 — Context Budget).
func TestSections_BudgetSums(t *testing.T) {
	cc := Compile(CompileInput{
		Prompt:    "tarefa completa",
		Agent:     "a",
		Role:      "r",
		Knowledge: &orchestration.KnowledgeSearchResults{Results: []orchestration.KnowledgeSearchResult{{ID: "x", Content: "c", Score: 0.9}}},
		Memory:    []orchestration.MemoryRecord{{ID: "m", Content: "mem"}},
		Target:    "t",
	})
	total := 0
	for _, s := range cc.Sections {
		total += s.Budget
	}
	if total != 100 {
		t.Fatalf("orçamento total = %d, esperava 100", total)
	}
}
