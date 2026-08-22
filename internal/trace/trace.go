// Package trace implements the universal Trace ID + structured event ledger
// for the Cosca (regra do Don: "Eu colocaria um Trace ID universal").
//
// Every important operation gets a universal identifier (TRACE-20260802-7F92A1B3)
// and every event carries the structured metadata needed to reconstruct the
// history: trace_id, parent_event, timestamp, actor, action, input_hash,
// output_hash, code_version, knowledge_version, cognitive_version,
// environment e result.
//
// With a full history it becomes possible to detect the point of divergence —
// "SUSPICIOUS DIVERGENCE — onde esta execução difere das anteriores" — with a
// deterministic heuristic (no LLM): DetectDivergence.
//
// The event ledger lives in SQLite (.cosca/trace.db) and is APPEND-ONLY: no
// UPDATE or DELETE is ever exposed. The flight recorder never rewrites the past.
package trace

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// TraceID is the universal identifier of an operation:
// "TRACE-YYYYMMDD-XXXX", where XXXX is 4 hex chars from crypto/rand.
type TraceID string

// traceIDRe é o padrão canônico de um Trace ID universal.
var traceIDRe = regexp.MustCompile(`^TRACE-\d{8}-[0-9A-Fa-f]{8}$`)

// NewID gera um novo Trace ID universal: "TRACE-20260802-7F92A1B3".
// O sufixo são 8 hex chars (4 bytes) derivados de crypto/rand (~4 bilhões
// de combinações — colisão virtualmente impossível no mesmo dia). Se o RNG
// falhar (extremamente raro), cai para derivação do relógio — nunca bloqueia.
func NewID() TraceID {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		n := uint32(time.Now().UnixNano() & 0xFFFFFFFF)
		b[0] = byte(n >> 24)
		b[1] = byte(n >> 16)
		b[2] = byte(n >> 8)
		b[3] = byte(n)
	}
	return TraceID(fmt.Sprintf("TRACE-%s-%02X%02X%02X%02X",
		time.Now().UTC().Format("20060102"), b[0], b[1], b[2], b[3]))
}

// Parse aceita um Trace ID canônico (case-insensitive) e devolve a forma
// normalizada (TRACE-YYYYMMDD-XXXX maiúsculo). Retorna (id, false) para
// qualquer entrada que não siga o formato exato.
func Parse(s string) (TraceID, bool) {
	up := strings.ToUpper(strings.TrimSpace(s))
	if !traceIDRe.MatchString(up) {
		return "", false
	}
	return TraceID(up), true
}

// String devolve a representação textual do Trace ID.
func (t TraceID) String() string {
	return string(t)
}

// Event é um evento estruturado de um trace — a unidade atômica da
// reconstrução de história. Cada evento carrega o contexto necessário para
// reconstruir o que aconteceu (flight recorder).
type Event struct {
	TraceID          string `json:"trace_id"`                    // TRACE-YYYYMMDD-XXXX
	ParentEvent      string `json:"parent_event,omitempty"`      // id do evento pai (tree structure)
	Timestamp        int64  `json:"timestamp"`                   // unix seconds
	Actor            string `json:"actor"`                       // "kernel", "agent-x", "don"
	Action           string `json:"action"`                      // "PLAN_CREATED", "TASK_STARTED", "TEST_FAILED", ...
	InputHash        string `json:"input_hash,omitempty"`        // SHA-256 do input
	OutputHash       string `json:"output_hash,omitempty"`       // SHA-256 do output
	CodeVersion      string `json:"code_version,omitempty"`      // git describe/short
	KnowledgeVersion string `json:"knowledge_version,omitempty"` // CV-XXXX
	CognitiveVersion string `json:"cognitive_version,omitempty"` // snapshot/versão cognitiva
	Environment      string `json:"environment,omitempty"`       // "linux/amd64"
	Result           string `json:"result,omitempty"`            // "success"|"failed"|"running"|...
	Details          string `json:"details,omitempty"`
}

// Sequence é a sequência ordenada de ações de um trace — a assinatura usada
// para comparar execuções e detectar divergência.
type Sequence struct {
	Actions []string
}

// SequenceFromEvents extrai a sequência ordenada de ações de uma lista de
// eventos já ordenada por timestamp. Eventos sem action são ignorados (não
// contam como passo do trace).
func SequenceFromEvents(events []Event) Sequence {
	s := Sequence{Actions: make([]string, 0, len(events))}
	for _, e := range events {
		if strings.TrimSpace(e.Action) != "" {
			s.Actions = append(s.Actions, e.Action)
		}
	}
	return s
}

// DetectDivergence retorna os índices onde a sequência atual diverge do
// histórico (execuções anteriores). Determinístico, sem LLM.
//
// Heurística documentada:
//   - Para cada posição i da sequência atual que também foi alcançada por ALGUMA
//     execução anterior (i < comprimento máximo dos anteriores), a ação atual é
//     comparada com o que as execuções anteriores fizeram NAQUELA MESMA posição.
//     Se a ação atual não aparece em nenhuma execução anterior na posição i, o
//     índice i é marcado como divergente. Isto cobre tanto "ação diferente do que
//     foi feito antes" quanto "ação inesperada no meio da sequência".
//   - Consequência de uma inserção no meio: além do ponto de inserção, todo índice
//     seguinte cuja ação deixa de bater com a posição histórica (porque a sequência
//     foi deslocada) também é marcado — o "cascade shift". Isto é fiel à definição
//     (a posição difere do que foi feito antes), e o primeiro índice marcado é
//     sempre o ponto real da divergência.
//   - Posições além de TODAS as execuções anteriores (i >= comprimento máximo)
//     são consideradas extensão natural (prefix-extension) e NUNCA são marcadas —
//     uma execução que continua além do histórico é progresso normal, não
//     divergência.
//   - Execuções anteriores vazias ou inexistentes ⇒ nenhuma divergência (não há
//     histórico para comparar).
//   - A sequência atual mais curta que todas as anteriores não é marcada: cada
//     posição existente é válida.
//
// Exemplo: anteriores = [[A B C D], [A B C E]], atual = [A B X C D] ⇒ [2]
// (na posição 2 as anteriores fizeram C, a atual fez X).
func DetectDivergence(previous []Sequence, current Sequence) []int {
	if len(previous) == 0 {
		return nil
	}

	maxLen := 0
	for _, seq := range previous {
		if len(seq.Actions) > maxLen {
			maxLen = len(seq.Actions)
		}
	}

	var divergence []int
	for i, action := range current.Actions {
		if i >= maxLen {
			// Extensão além de todo o histórico — normal (prefix-extension).
			continue
		}
		matched := false
		for _, seq := range previous {
			if i < len(seq.Actions) && seq.Actions[i] == action {
				matched = true
				break
			}
		}
		if !matched {
			divergence = append(divergence, i)
		}
	}
	return divergence
}
