// Package capability models the Cosca cognitive capability levels (L0–L3).
//
// Os níveis expressam o que a Cosca consegue fazer com base no provider ativo:
//
//	L0 — Determinístico   (sem IA: regras, estado, memória, knowledge, workflow, auditoria, segurança)
//	L1 — Retrieval        (IA opcional: busca de conhecimento, classificação, matching semântico, extração)
//	L2 — Raciocínio       (modelo disponível: análise, planejamento, inferência, código, diagnóstico)
//	L3 — Execução autônoma (modelo + runtime: planeja → propõe → valida → executa → testa → observa → aprende)
//
// A detecção é deliberadamente simples e explícita (uma tabela de providers de
// raciocínio), sem dependência de estado do servidor: basta o nome do provider
// ativo e se o runtime/execução está habilitado.
package capability

import (
	"fmt"
	"sort"
	"strings"
)

// Level representa um nível de capacidade cognitiva.
type Level int

// Níveis de capacidade cognitiva.
const (
	Level0Deterministic Level = iota
	Level1Retrieval
	Level2Reasoning
	Level3Autonomous
)

// reasoningProviders são os providers com modelo de linguagem disponível
// (raciocínio). Providers apenas-embeddings (ex: "local"/tf-idf) ficam de fora,
// permanecendo no mínimo de Retrieval.
var reasoningProviders = map[string]bool{
	"openai":      true,
	"anthropic":   true,
	"google":      true,
	"gemini":      true,
	"azure":       true,
	"deepseek":    true,
	"ollama":      true,
	"groq":        true,
	"mistral":     true,
	"bedrock":     true,
	"aws-bedrock": true,
	"custom":      true,
}

// String retorna o nome pt-BR do nível.
func (l Level) String() string {
	switch l {
	case Level0Deterministic:
		return "L0-determinístico"
	case Level1Retrieval:
		return "L1-retrieval"
	case Level2Reasoning:
		return "L2-raciocínio"
	case Level3Autonomous:
		return "L3-autônomo"
	default:
		return fmt.Sprintf("L%d-desconhecido", int(l))
	}
}

// Description retorna uma descrição curta, em pt-BR, do que o nível habilita.
func (l Level) Description() string {
	switch l {
	case Level0Deterministic:
		return "Modo determinístico: opera sem IA — regras, estado, memória, knowledge, workflow, auditoria, segurança e execução previamente definida."
	case Level1Retrieval:
		return "Retrieval: busca de conhecimento, classificação, matching semântico e extração (IA opcional)."
	case Level2Reasoning:
		return "Raciocínio: análise, planejamento, inferência, geração de código e diagnóstico (modelo disponível)."
	case Level3Autonomous:
		return "Execução autônoma: planeja → propõe → valida → executa → testa → observa → aprende (modelo + runtime)."
	default:
		return "Nível de capacidade desconhecido."
	}
}

// CurrentLevel calcula o nível de capacidade atual a partir do provider ativo
// e da disponibilidade de runtime/execução.
//
// Regras de decisão (simples e explícitas):
//   - provider vazio ou "none"        → L0 (determinístico, sem IA)
//   - provider real (embeddings)      → L1 (retrieval) mínimo
//   - provider de raciocínio (LLM)    → L2 (raciocínio)
//   - provider + runtime habilitado   → L3 (execução autônoma)
//
// "local" (tf-idf) é um provider real porém apenas-embeddings → L1.
func CurrentLevel(activeProvider string, hasRuntime bool) Level {
	provider := strings.ToLower(strings.TrimSpace(activeProvider))
	if provider == "" || provider == "none" {
		return Level0Deterministic
	}

	level := Level1Retrieval
	if reasoningProviders[provider] {
		level = Level2Reasoning
	}
	if hasRuntime {
		level = Level3Autonomous
	}
	return level
}

// baseCapabilities são as capacidades disponíveis em todos os níveis (L0).
func baseCapabilities() []string {
	return []string{
		"knowledge",
		"laws",
		"memory",
		"audit",
		"security",
		"workflow",
		"backup",
	}
}

// allCapabilities é a união de todas as capacidades (níveis L0–L3).
func allCapabilities() []string {
	return []string{
		"knowledge",
		"laws",
		"memory",
		"audit",
		"security",
		"workflow",
		"backup",
		"semantic_search",
		"classification",
		"reasoning",
		"planning",
		"code_gen",
		"autonomous_execution",
	}
}

// Capabilities retorna a lista de capacidades disponíveis no nível informado.
// Níveis superiores são um superconjunto dos inferiores.
func Capabilities(level Level) []string {
	var caps []string
	switch {
	case level >= Level3Autonomous:
		caps = append(caps, "autonomous_execution")
		fallthrough
	case level >= Level2Reasoning:
		caps = append(caps, "reasoning", "planning", "code_gen")
		fallthrough
	case level >= Level1Retrieval:
		caps = append(caps, "semantic_search", "classification")
		fallthrough
	default:
		caps = append(caps, baseCapabilities()...)
	}
	sort.Strings(caps)
	return caps
}

// AllCapabilities retorna a lista completa de capacidades do sistema (L0–L3).
func AllCapabilities() []string {
	return allCapabilities()
}

// UnavailableCapabilities retorna as capacidades NÃO disponíveis no nível
// informado (todas − disponíveis).
func UnavailableCapabilities(level Level) []string {
	available := make(map[string]bool, len(Capabilities(level)))
	for _, c := range Capabilities(level) {
		available[c] = true
	}

	var missing []string
	for _, c := range allCapabilities() {
		if !available[c] {
			missing = append(missing, c)
		}
	}
	sort.Strings(missing)
	return missing
}
