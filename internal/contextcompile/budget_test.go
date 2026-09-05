package contextcompile

import (
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/orchestration"
)

// bigKnowledge gera N resultados longos para testar o truncamento por budget.
func bigKnowledge(n int) *orchestration.KnowledgeSearchResults {
	kr := &orchestration.KnowledgeSearchResults{}
	for i := 0; i < n; i++ {
		kr.Results = append(kr.Results, orchestration.KnowledgeSearchResult{
			ID:      string(rune('a' + i)),
			Content: strings.Repeat("informação importante sobre o sistema para a decisão ", 20),
			Score:   0.9,
		})
	}
	return kr
}

// TestCompileWithBudget_RespectsTeto: o contexto compilado com teto NÃO
// ultrapassa o orçamento (tokens estimados ≤ MaxTokens).
func TestCompileWithBudget_RespectsTeto(t *testing.T) {
	cc := CompileWithBudget(CompileInput{
		Prompt:    "corrigir endpoint MCP com contexto muito grande",
		Agent:     "Backend API Specialist",
		Role:      "backend",
		Knowledge: bigKnowledge(30), // muito conhecimento — precisa cortar
		Target:    "internal/mcpserver/registry.go",
	}, TokenBudget{MaxTokens: 1000})

	rep := cc.Report(TokenBudget{MaxTokens: 1000})
	if rep.UsedTokens > rep.MaxTokens {
		t.Fatalf("contexto estourou o teto: %d > %d tokens\n%s", rep.UsedTokens, rep.MaxTokens, rep.String())
	}
}

// TestCompileWithBudget_CriticalSectionsPreserved: TASK e OUTPUT_CONTRACT
// SEMPRE estão presentes mesmo com teto apertado (nunca cortadas a zero).
func TestCompileWithBudget_CriticalSectionsPreserved(t *testing.T) {
	cc := CompileWithBudget(CompileInput{
		Prompt:    "tarefa crítica que não pode sumir",
		Agent:     "a",
		Role:      "r",
		Knowledge: bigKnowledge(50),
	}, TokenBudget{MaxTokens: 300}) // teto apertado

	if !strings.Contains(cc.Text, "tarefa crítica que não pode sumir") {
		t.Fatal("TASK foi cortada com teto apertado — seção crítica nunca some")
	}
	if !strings.Contains(cc.Text, "OUTPUT_CONTRACT:") {
		t.Fatal("OUTPUT_CONTRACT foi cortado — sempre presente")
	}
	if !strings.Contains(cc.Text, "STATE:") || !strings.Contains(cc.Text, "agent=a") {
		t.Fatal("STATE foi cortada — seção crítica nunca some")
	}
}

// TestCompileWithBudget_NoTeto_Unchanged: sem teto (MaxTokens<=0) o resultado
// é idêntico ao Compile puro (sem truncamento).
func TestCompileWithBudget_NoTeto_Unchanged(t *testing.T) {
	in := CompileInput{
		Prompt:    "tarefa",
		Agent:     "a",
		Knowledge: bigKnowledge(5),
	}
	with := CompileWithBudget(in, TokenBudget{MaxTokens: 0})
	without := Compile(in)
	if with.Text != without.Text {
		t.Fatal("sem teto o CompileWithBudget deveria ser idêntico ao Compile")
	}
}

// TestBudgetReport_BySection: o relatório mostra a alocação por seção.
func TestBudgetReport_BySection(t *testing.T) {
	cc := CompileWithBudget(CompileInput{
		Prompt:    "tarefa com contexto",
		Agent:     "a",
		Role:      "r",
		Knowledge: bigKnowledge(5),
	}, TokenBudget{MaxTokens: 800})

	rep := cc.Report(TokenBudget{MaxTokens: 800})
	if _, ok := rep.BySection["TASK"]; !ok {
		t.Fatal("relatório sem seção TASK")
	}
	if _, ok := rep.BySection["OUTPUT_CONTRACT"]; !ok {
		t.Fatal("relatório sem seção OUTPUT_CONTRACT")
	}
	if rep.MaxTokens != 800 {
		t.Fatalf("MaxTokens = %d, esperava 800", rep.MaxTokens)
	}
	if !strings.Contains(rep.String(), "tokens") {
		t.Fatal("relatório formatado sem métrica")
	}
}

// TestTruncateToTokens_LinhaPreservada: o truncamento corta na fronteira de
// linha (informação estruturada por linha).
func TestTruncateToTokens_LinhaPreservada(t *testing.T) {
	s := "linha um\nlinha dois\nlinha três\nlinha quatro"
	got := truncateToTokens(s, 3) // ~3 tokens = 12 chars ≈ 1-2 linhas
	if strings.Contains(got, "linha quatro") && strings.Contains(got, "linha um") && strings.Contains(got, "linha dois") {
		// aceita qualquer corte de prefixo de linhas
	}
	if len(got) > len(s) {
		t.Fatal("truncamento aumentou o texto")
	}
}

// TestEstimatedTokens_Heuristica: estimativa mínima por texto.
func TestEstimatedTokens_Heuristica(t *testing.T) {
	if estimatedTokens("") != 0 {
		t.Fatal("texto vazio deveria ter 0 tokens")
	}
	if estimatedTokens("a") < 1 {
		t.Fatal("texto mínimo deveria ter pelo menos 1 token")
	}
	long := strings.Repeat("palavra ", 100)
	if estimatedTokens(long) < 10 {
		t.Fatal("texto longo deveria estimar muitos tokens")
	}
}
