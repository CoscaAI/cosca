// Package contextcompile — CONTEXT COMPILER (ADR-035, F2).
//
// Monta o contexto entregue ao LLM como um ESTADO OPERACIONAL compilado
// (TASK/STATE/CONSTRAINTS/FACTS/EVIDENCE/TARGET/OUTPUT_CONTRACT) em vez de
// documentos empilhados. É a evolução do BuildCleanContext (ADR-032): o mesmo
// princípio (bloco determinístico estruturado), com o template orientado à
// tarefa que o professor desenhou (2026-09-01):
//
//	"não mandar documentos inteiros; mandar um estado de execução."
//
// PURE e testável sem LLM. ADITIVO: não substitui o BuildCleanContext — é a
// camada que os motores PODEM usar (F2 conecta ao loop LLM).
package contextcompile

import (
	"fmt"
	"strings"

	"github.com/CoscaAI/cosca/internal/orchestration"
)

// Section é uma seção do contexto compilado, com o percentual de orçamento
// que ela representa (Context Budget — F3 usa isto para alocar tokens).
type Section struct {
	// Name é o identificador da seção (TASK, STATE, ...).
	Name string
	// Content é o texto compilado da seção.
	Content string
	// Budget é o percentual ideal do contexto para esta seção (soma = 100).
	Budget int
}

// CompileInput é o que o compilador precisa da tarefa em execução.
type CompileInput struct {
	// Prompt é a intenção do usuário (a TASK).
	Prompt string
	// Agent é o agente resolvido (STATE).
	Agent string
	// Role é a função do agente (STATE).
	Role string
	// Knowledge são os resultados de busca (FACTS/EVIDENCE) — pode ser nil.
	Knowledge *orchestration.KnowledgeSearchResults
	// Memory são os resultados de memória (MEMORY) — pode ser nil.
	Memory []orchestration.MemoryRecord
	// Constraints são as restrições da tarefa (CONSTRAINTS).
	Constraints []string
	// Allowed/Forbidden delimitam o espaço de ação do agente.
	Allowed   []string
	Forbidden []string
	// Target é o alvo provável da ação (preenchido por heurística simples).
	Target string
}

// CompiledContext é o resultado do compilador: as seções + o texto montado.
type CompiledContext struct {
	// Sections são as seções com seus orçamentos.
	Sections []Section
	// Text é o contexto compilado pronto para o LLM (seções concatenadas).
	Text string
}

// sectionOrder define a ordem canônica e o orçamento de cada seção
// (ADR-035 §2.2 — Context Budget de exemplo: TASK 5, STATE 15, ...).
var sectionOrder = []struct {
	name   string
	budget int
}{
	{"TASK", 5},
	{"STATE", 15},
	{"CONSTRAINTS", 10},
	{"FACTS", 25},
	{"EVIDENCE", 25},
	{"MEMORY", 10},
	{"TARGET", 5},
	{"OUTPUT_CONTRACT", 5},
}

// Compile monta o contexto compilado a partir do input. TODAS as seções
// canônicas são registradas com seu orçamento fixo (soma = 100, base da F3 —
// Context Budget); o TEXTO só inclui as seções com conteúdo (vazias omitidas
// para não poluir o LLM).
func Compile(in CompileInput) *CompiledContext {
	// Registra todas as seções canônicas com orçamento fixo (ADR-035 §2.2).
	compiled := &CompiledContext{}
	for _, s := range sectionOrder {
		compiled.Sections = append(compiled.Sections, Section{Name: s.name, Budget: s.budget})
	}
	sectionBy := func(name string) *Section {
		for i := range compiled.Sections {
			if compiled.Sections[i].Name == name {
				return &compiled.Sections[i]
			}
		}
		return nil
	}

	var sb strings.Builder

	// TASK — a intenção do usuário.
	task := strings.TrimSpace(in.Prompt)
	if task != "" {
		sectionBy("TASK").Content = task
		sb.WriteString("TASK:\n  " + task + "\n")
	}

	// STATE — o agente resolvido e seu papel.
	var stateParts []string
	if in.Agent != "" {
		stateParts = append(stateParts, "agent="+in.Agent)
	}
	if in.Role != "" {
		stateParts = append(stateParts, "role="+in.Role)
	}
	if len(stateParts) > 0 {
		content := "  " + strings.Join(stateParts, " · ")
		sectionBy("STATE").Content = content
		sb.WriteString("STATE:\n" + content + "\n")
	}

	// CONSTRAINTS — restrições explícitas.
	if len(in.Constraints) > 0 {
		content := formatBullets(in.Constraints)
		sectionBy("CONSTRAINTS").Content = content
		sb.WriteString("CONSTRAINTS:\n" + content + "\n")
	}

	// FACTS — conhecimento de alta confiança (score alto) com epistemologia.
	if in.Knowledge != nil && len(in.Knowledge.Results) > 0 {
		content := compileFacts(in.Knowledge)
		sectionBy("FACTS").Content = content
		sb.WriteString("FACTS:\n" + content + "\n")
	}

	// EVIDENCE — conhecimento adicional (baixa confiança vira evidência, não fato).
	if in.Knowledge != nil && len(in.Knowledge.Results) > 1 {
		content := compileEvidence(in.Knowledge)
		sectionBy("EVIDENCE").Content = content
		sb.WriteString("EVIDENCE:\n" + content + "\n")
	}

	// MEMORY — memórias relevantes recuperadas.
	if len(in.Memory) > 0 {
		content := compileMemory(in.Memory)
		sectionBy("MEMORY").Content = content
		sb.WriteString("MEMORY:\n" + content + "\n")
	}

	// TARGET — alvo provável (heurística simples).
	if in.Target != "" {
		content := "  " + in.Target
		sectionBy("TARGET").Content = content
		sb.WriteString("TARGET:\n" + content + "\n")
	}

	// ALLOWED/FORBIDDEN (espaço de ação) — dentro de CONSTRAINTS quando presentes.
	var space []string
	if len(in.Allowed) > 0 {
		space = append(space, "ALLOWED: "+strings.Join(in.Allowed, ", "))
	}
	if len(in.Forbidden) > 0 {
		space = append(space, "FORBIDDEN: "+strings.Join(in.Forbidden, ", "))
	}
	if len(space) > 0 {
		sb.WriteString("\n" + strings.Join(space, "\n") + "\n")
	}

	// OUTPUT_CONTRACT — o formato obrigatório da resposta (conecta com o
	// ActionDecoder, ADR-035 F1). Sempre presente: o LLM sabe como responder.
	outputContract := "  Responda com um instruction packet JSON: {\"decision\":\"EDIT|CREATE|DELETE|RUN|SEARCH|ANSWER|ESCALATE\",\"target\":\"...\",\"action\":\"...\",\"confidence\":0.0-1.0,\"reason\":\"...\",\"verification\":[\"...\"]}\n" +
		"  Se não puder decidir, responda em prosa normal (texto) — nunca invente um packet."
	sectionBy("OUTPUT_CONTRACT").Content = outputContract
	sb.WriteString("\nOUTPUT_CONTRACT:\n" + outputContract + "\n")

	compiled.Text = sb.String()
	return compiled
}

// compileFacts monta os FATOS: resultados de score alto (>= 0.7) com a classe
// epistêmica prefixada ([F:FACT], [M:MEASURED]...).
func compileFacts(k *orchestration.KnowledgeSearchResults) string {
	var parts []string
	for _, r := range k.Results {
		if r.Score >= 0.7 {
			parts = append(parts, fmt.Sprintf("  [%s] %s (score %.2f)", epistemicTag(r.Epistemic), truncate(r.Content, 200), r.Score))
		}
	}
	if len(parts) == 0 {
		return "  (nenhum fato de alta confiança)"
	}
	return strings.Join(parts, "\n")
}

// compileEvidence monta as EVIDÊNCIAS: resultados de score médio (< 0.7) que
// apoiam a decisão sem serem fatos.
func compileEvidence(k *orchestration.KnowledgeSearchResults) string {
	var parts []string
	for _, r := range k.Results {
		if r.Score < 0.7 {
			parts = append(parts, fmt.Sprintf("  [%s] %s (score %.2f)", epistemicTag(r.Epistemic), truncate(r.Content, 200), r.Score))
		}
	}
	if len(parts) == 0 {
		return "  (nenhuma evidência adicional)"
	}
	return strings.Join(parts, "\n")
}

// compileMemory monta a seção de memórias relevantes recuperadas.
func compileMemory(records []orchestration.MemoryRecord) string {
	var parts []string
	for _, m := range records {
		parts = append(parts, fmt.Sprintf("  [%s/%s] %s", m.Type, m.Layer, truncate(m.Content, 150)))
	}
	return strings.Join(parts, "\n")
}

// formatBullets formata uma lista como bullets indentados.
func formatBullets(items []string) string {
	var parts []string
	for _, it := range items {
		parts = append(parts, "  - "+strings.TrimSpace(it))
	}
	return strings.Join(parts, "\n")
}

// epistemicTag devolve a tag epistêmica legível, ou "?" quando ausente.
func epistemicTag(epistemic string) string {
	if epistemic == "" {
		return "?"
	}
	return epistemic
}

// truncate limita o texto a maxLen bytes preservando o final da frase.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}
