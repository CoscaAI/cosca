package contextcompile

// budget.go — CONTEXT BUDGET (ADR-035, F3).
//
// O compilador aloca tokens por seção conforme o orçamento percentual
// (TASK 5%, STATE 15%...) e preenche o orçamento com INFORMAÇÃO, não com
// texto (professor, 2026-09-01). Quando o teto estoura, as seções são
// truncadas por prioridade — as críticas (TASK, STATE, CONSTRAINTS,
// OUTPUT_CONTRACT) nunca são cortadas; FACTS/EVIDENCE/MEMORY/TARGET são.

import (
	"fmt"
	"strings"
)

// TokenBudget configura o teto de tokens do contexto compilado.
type TokenBudget struct {
	// MaxTokens é o teto total do contexto (ex.: 2048). <= 0 = sem teto
	// (Compile puro, sem truncamento).
	MaxTokens int
}

// estimatedTokens é uma heurística conservadora de tokens por texto
// (~4 chars/token, piso 1 por palavra significativa). Suficiente para
// alocação por seção — a medição fina é do cost (ADR-031).
func estimatedTokens(s string) int {
	if s == "" {
		return 0
	}
	n := len(s) / 4
	if n < 1 {
		n = 1
	}
	return n
}

// truncateToTokens limita o texto a um teto de tokens estimados, cortando na
// fronteira de linha quando possível.
func truncateToTokens(s string, maxTokens int) string {
	if maxTokens <= 0 || estimatedTokens(s) <= maxTokens {
		return s
	}
	// Corte por linhas primeiro (informação estruturada por linha).
	lines := strings.Split(s, "\n")
	var out []string
	used := 0
	for _, ln := range lines {
		t := estimatedTokens(ln)
		if used+t > maxTokens {
			break
		}
		out = append(out, ln)
		used += t
	}
	if len(out) == 0 {
		// Nenhuma linha cabe inteira — corta por caractere.
		maxChars := maxTokens * 4
		if len(s) > maxChars {
			return s[:maxChars] + "…"
		}
		return s
	}
	return strings.Join(out, "\n")
}

// CompileWithBudget compila o contexto respeitando o teto de tokens: cada
// seção recebe maxTokens * budget/100 tokens estimados. Seções críticas
// (TASK, STATE, CONSTRAINTS, OUTPUT_CONTRACT) têm prioridade: o teto delas é
// garantido antes das demais; se o total das críticas estourar o teto, as
// demais (FACTS/EVIDENCE/MEMORY/TARGET) são encolhidas.
func CompileWithBudget(in CompileInput, budget TokenBudget) *CompiledContext {
	cc := Compile(in)
	if budget.MaxTokens <= 0 {
		return cc
	}

	// Seções críticas que nunca são cortadas abaixo do mínimo.
	critical := map[string]bool{
		"TASK": true, "STATE": true, "CONSTRAINTS": true, "OUTPUT_CONTRACT": true,
	}
	// Seções flexíveis (cortadas quando o teto estoura).
	flexibleOrder := []string{"FACTS", "EVIDENCE", "MEMORY", "TARGET"}

	// 1. Reserva o orçamento das seções críticas.
	criticalTokens := 0
	for i := range cc.Sections {
		s := &cc.Sections[i]
		if critical[s.Name] {
			alloc := budget.MaxTokens * s.Budget / 100
			if alloc < minCriticalTokens {
				alloc = minCriticalTokens
			}
			s.Content = truncateToTokens(s.Content, alloc)
			criticalTokens += estimatedTokens(s.Content)
		}
	}

	// 2. O restante do teto vai para as seções flexíveis, na ordem de
	//    prioridade (FACTS > EVIDENCE > MEMORY > TARGET), com o orçamento
	//    proporcional como guia.
	remaining := budget.MaxTokens - criticalTokens
	if remaining < 0 {
		remaining = 0
	}
	for _, name := range flexibleOrder {
		for i := range cc.Sections {
			s := &cc.Sections[i]
			if s.Name != name {
				continue
			}
			// Alocação proporcional ao orçamento da seção no total flexível.
			alloc := remaining * s.Budget / flexibleBudgetSum
			s.Content = truncateToTokens(s.Content, alloc)
		}
	}

	// 3. Remonta o texto a partir das seções (com conteúdo).
	cc.Text = renderText(cc.Sections)
	return cc
}

// flexibleBudgetSum é a soma dos orçamentos das seções flexíveis
// (FACTS 25 + EVIDENCE 25 + MEMORY 10 + TARGET 5 = 65).
const flexibleBudgetSum = 65

// minCriticalTokens é o teto mínimo garantido para cada seção crítica (nunca
// zero — o LLM sempre sabe a TASK e o OUTPUT_CONTRACT).
const minCriticalTokens = 64

// renderText remonta o texto do contexto a partir das seções com conteúdo.
func renderText(sections []Section) string {
	var sb strings.Builder
	for _, s := range sections {
		if s.Content == "" {
			continue
		}
		sb.WriteString(s.Name + ":\n" + s.Content + "\n")
	}
	return sb.String()
}

// BudgetReport descreve a alocação real de tokens por seção — usado pela F5
// (medição) e pelo cost para auditabilidade.
type BudgetReport struct {
	// MaxTokens é o teto configurado.
	MaxTokens int
	// UsedTokens é o total estimado usado.
	UsedTokens int
	// BySection mapeia seção -> tokens alocados.
	BySection map[string]int
}

// Report gera o relatório de alocação do contexto compilado.
func (cc *CompiledContext) Report(budget TokenBudget) BudgetReport {
	rep := BudgetReport{MaxTokens: budget.MaxTokens, BySection: make(map[string]int)}
	for _, s := range cc.Sections {
		t := estimatedTokens(s.Content)
		rep.BySection[s.Name] = t
		rep.UsedTokens += t
	}
	return rep
}

// String formata o relatório como tabela legível (para logs/dashboard).
func (r BudgetReport) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "context budget: %d/%d tokens\n", r.UsedTokens, r.MaxTokens)
	for _, name := range []string{"TASK", "STATE", "CONSTRAINTS", "FACTS", "EVIDENCE", "MEMORY", "TARGET", "OUTPUT_CONTRACT"} {
		if t, ok := r.BySection[name]; ok && t > 0 {
			fmt.Fprintf(&sb, "  %-16s %4d tokens\n", name, t)
		}
	}
	return sb.String()
}
