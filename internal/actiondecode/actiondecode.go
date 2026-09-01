// Package actiondecode — ACTION DECODER (ADR-035, F1).
//
// Transforma a resposta do LLM em um instruction packet estruturado
// (DECISION/TARGET/ACTION/ARGUMENTS/CONFIDENCE/VERIFICATION) que o runtime
// EXECUTA em vez de interpretar prosa. O LLM vira um coprocessador cognitivo:
// o COSCA manda contexto compilado e recebe de volta uma decisão acionável.
//
// FAIL-OPEN OBRIGATÓRIO (ADR-035 §3): se o LLM não devolver JSON estruturado
// válido, o decoder NUNCA quebra — devolve a resposta como texto (Status=text),
// preservando o comportamento atual. O packet é um ENRIQUECIMENTO, não um
// requisito. Decisão de design aprovada na conversa com o professor
// (2026-09-01): "e se a decisão for trivial/determinística, nem chamar LLM".
package actiondecode

import (
	"encoding/json"
	"strings"
)

// Decision é a ação estruturada que o runtime pode executar.
type Decision struct {
	// Decision é o verbo de alto nível (EDIT, CREATE, DELETE, RUN, SEARCH,
	// ANSWER, ESCALATE...). Empty = resposta textual (fail-open).
	Decision string `json:"decision,omitempty"`
	// Target é o alvo da ação (arquivo, endpoint, pacote...).
	Target string `json:"target,omitempty"`
	// Action é a operação concreta (align_registry, add_test...).
	Action string `json:"action,omitempty"`
	// Arguments são os parâmetros da ação (opcional).
	Arguments map[string]any `json:"arguments,omitempty"`
	// Confidence é a confiança do modelo na decisão (0.0–1.0).
	Confidence float64 `json:"confidence,omitempty"`
	// Reason é a justificativa em uma frase (para o runtime/log).
	Reason string `json:"reason,omitempty"`
	// Verification são os comandos que provam a decisão (ex.: "go test ./...").
	Verification []string `json:"verification,omitempty"`
}

// Status categoriza o resultado do decode.
type Status string

const (
	// StatusStructured indica que o LLM devolveu um instruction packet válido.
	StatusStructured Status = "structured"
	// StatusText indica que o LLM devolveu prosa — fail-open, resposta textual.
	StatusText Status = "text"
)

// Packet é o resultado do Action Decoder: ou um instruction packet estruturado
// (Status=structured) ou a resposta textual crua (Status=text, fail-open).
type Packet struct {
	Status Status
	// Structured é preenchido quando Status=structured.
	Structured *Decision
	// Text é a resposta crua quando Status=text (e também o reason/prosa
	// quando o LLM mistura prosa com JSON).
	Text string
}

// minimumConfidence é o piso de confiança para aceitar uma decisão estruturada.
// Abaixo dele, o decoder NÃO trata como decisão confiável — devolve o packet
// como text (o runtime não executa decisão de baixa confiança sem contexto).
const minimumConfidence = 0.5

// Decode interpreta a resposta do LLM como instruction packet. Estratégia:
//  1. Tenta extrair o bloco JSON (entre chaves, ou { "decision": ... }).
//  2. Se o JSON tem decision/action e confidence ≥ piso → StatusStructured.
//  3. Qualquer falha → StatusText (fail-open — resposta crua preservada).
func Decode(raw string) Packet {
	if strings.TrimSpace(raw) == "" {
		return Packet{Status: StatusText, Text: raw}
	}

	block := extractJSON(raw)
	if block == "" {
		return Packet{Status: StatusText, Text: raw}
	}

	var d Decision
	if err := json.Unmarshal([]byte(block), &d); err != nil {
		return Packet{Status: StatusText, Text: raw}
	}

	// Requer pelo menos decision OU action (um packet vazio não é decisão).
	if strings.TrimSpace(d.Decision) == "" && strings.TrimSpace(d.Action) == "" {
		return Packet{Status: StatusText, Text: raw}
	}

	// Confiança abaixo do piso → não executa como decisão (fail-open).
	if d.Confidence > 0 && d.Confidence < minimumConfidence {
		return Packet{Status: StatusText, Text: raw}
	}

	return Packet{Status: StatusStructured, Structured: &d, Text: raw}
}

// extractJSON localiza o primeiro bloco JSON balanceado na resposta. O LLM
// pode embrulhar o JSON em markdown (```json ... ```) ou prosa; extraímos o
// primeiro '{' e casamos chaves balanceadas. Retorna "" se não achar.
func extractJSON(raw string) string {
	start := strings.Index(raw, "{")
	if start == -1 {
		return ""
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(raw); i++ {
		c := raw[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return raw[start : i+1]
			}
		}
	}
	return ""
}
