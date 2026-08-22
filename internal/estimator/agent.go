package estimator

import (
	"os"
	"strconv"
	"strings"
)

const (
	// DefaultAgentConfidence é a confiança usada quando o agente não é
	// encontrado no Trust Registry (calibração conservadora).
	DefaultAgentConfidence = 0.85

	// DefaultTrustRegistryPath é o caminho relativo (a partir do ProjectDir)
	// do Trust Registry do Cosca.
	DefaultTrustRegistryPath = ".cosca/memory/trust/TRUST_REGISTRY.md"

	// TrustRegistryEnvVar permite sobrescrever o caminho do Trust Registry
	// via variável de ambiente (útil para testes e ambientes alternativos).
	TrustRegistryEnvVar = "COSCA_TRUST_REGISTRY"
)

// AgentConfidence retorna a confiança histórica do agente extraída do Trust
// Registry (campo reliability_score, 0.00-1.00). Quando o arquivo não existe,
// o agente não é encontrado ou o score é inválido, retorna DefaultAgentConfidence.
func AgentConfidence(agentName, trustRegistryPath string) float64 {
	score, found := lookupAgentConfidence(agentName, trustRegistryPath)
	if !found {
		return DefaultAgentConfidence
	}
	return score
}

// lookupAgentConfidence localiza o agente dentro do bloco "reliability_score:"
// do TRUST_REGISTRY.md e extrai o campo "score". O segundo retorno indica se a
// confiança foi encontrada (fonte: histórico real) ou não (heurística).
func lookupAgentConfidence(agentName, trustRegistryPath string) (float64, bool) {
	if trustRegistryPath == "" || agentName == "" {
		return 0, false
	}
	data, err := os.ReadFile(trustRegistryPath)
	if err != nil {
		return 0, false
	}

	lines := strings.Split(string(data), "\n")
	// Varre TODAS as ocorrências de "reliability_score:" — o arquivo contém um
	// placeholder documental (seção 2.4) antes do bloco real com os scores.
	for i, l := range lines {
		if !strings.HasPrefix(strings.TrimSpace(l), "reliability_score:") {
			continue
		}
		if score, ok := parseReliabilityBlock(lines, i, agentName); ok {
			return score, true
		}
	}
	return 0, false
}

// parseReliabilityBlock procura o agente dentro do bloco de reliability_score
// que começa na linha start (índice em lines) e devolve o score encontrado.
func parseReliabilityBlock(lines []string, start int, agentName string) (float64, bool) {
	relIndent := indentOf(lines[start])
	for i := start + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if indentOf(line) <= relIndent {
			return 0, false // saiu do bloco sem encontrar o agente
		}
		if !strings.HasSuffix(trimmed, ":") {
			continue
		}
		if !strings.EqualFold(strings.TrimSuffix(trimmed, ":"), agentName) {
			continue
		}

		agentIndent := indentOf(line)
		for j := i + 1; j < len(lines); j++ {
			inner := lines[j]
			innerTrim := strings.TrimSpace(inner)
			if innerTrim == "" || strings.HasPrefix(innerTrim, "#") {
				continue
			}
			if indentOf(inner) <= agentIndent {
				break
			}
			if strings.HasPrefix(innerTrim, "score:") {
				raw := strings.TrimSpace(strings.TrimPrefix(innerTrim, "score:"))
				if v, err := strconv.ParseFloat(raw, 64); err == nil && v >= 0 && v <= 1 {
					return v, true
				}
			}
		}
		return 0, false
	}
	return 0, false
}

// indentOf conta os espaços/tabs à esquerda da linha (indentação YAML).
func indentOf(line string) int {
	n := 0
	for n < len(line) && (line[n] == ' ' || line[n] == '\t') {
		n++
	}
	return n
}

// trustRegistryPath resolve o caminho do Trust Registry: usa a env
// COSCA_TRUST_REGISTRY quando definida, senão o padrão relativo ao ProjectDir.
func trustRegistryPath(projectDir string) string {
	if p := os.Getenv(TrustRegistryEnvVar); p != "" {
		return p
	}
	return projectDir + string(os.PathSeparator) + DefaultTrustRegistryPath
}
