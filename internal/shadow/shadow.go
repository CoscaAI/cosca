// Package shadow instrumenta o Cognitive Shadow Mode (ADR-033): o cérebro
// observa o próprio processo de deliberação (ADR-032) SEM ganhar autoridade.
//
// O Shadow reusa o Deliberator.Deliberate (idêntico ao modo ativo) e NUNCA
// aplica o verdict ao fluxo — apenas REGISTRA qual teria sido a decisão do
// Kernel (contrafactual): a percepção (posições), as evidências recuperadas,
// a confiança e o "teria escalado?".
//
// A persistência é um arquivo JSONL append-only em `.cosca/shadow/records.jsonl`
// (runtime, gitignored) — espelho do padrão do `internal/cost` (ADR-015 /
// ADR-031). Cada linha é um ShadowTrace. A leitura tolera versões anteriores
// (campos omitempty/zero-default).
//
// Regra de ouro (a diferença crítica para o modo ativo): o modo ativo DECIDE e
// APLICA; o modo Shadow DECIDE e REGISTRA. O "decide" (o Deliberate()) é
// idêntico; o que muda é o que se faz com o verdict.
package shadow

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/deliberate"
)

// RecordsDir é o subdiretório de runtime (relativo a `.cosca`) que guarda o
// log do Shadow. Gitignored como `cost/` e `logs/` (derivado, regenerável).
const RecordsDir = "shadow"

// RecordsFile é o arquivo JSONL append-only dentro de RecordsDir.
const RecordsFile = "records.jsonl"

// ShadowDecision é o rótulo da decisão contrafactual do Kernel (ADR-033 §3.3).
type ShadowDecision string

const (
	// RETRIEVAL_INSUFFICIENT — len(positions)==0: o Kernel não tem evidência
	// substanciada. Teria escalado (e é o caso mais comum na pré-calibração).
	RETRIEVAL_INSUFFICIENT ShadowDecision = "RETRIEVAL_INSUFFICIENT"
	// EMIT_OK — verdict==EmitOK: o Kernel teria respondido SEM LLM.
	EMIT_OK ShadowDecision = "EMIT_OK"
	// EMIT_WITH_RESERVATIONS — verdict==EmitWithReservations: teria chamado a
	// LLM com mitigação.
	EMIT_WITH_RESERVATIONS ShadowDecision = "EMIT_WITH_RESERVATIONS"
	// ESCALATE — verdict==Escalate: o Kernel reconhece o limite e teria escalado.
	ESCALATE ShadowDecision = "ESCALATE"
)

// ShadowTrace é uma observação do Cognitive Shadow Mode (ADR-033 §3.3): um
// subconjunto orientado à observação do DeliberationTrace, com o rótulo
// legível e a resposta CONTRAFACTUAL (o que o Kernel teria dito/feito).
// Serializável, determinístico e auditável.
type ShadowTrace struct {
	// RequestID é o id da execução observada.
	RequestID string `json:"request_id"`
	// Agent é o agente resolvido (vazio = KERNEL fallback).
	Agent string `json:"agent,omitempty"`
	// Decision é a taxonomia da decisão contrafactual.
	Decision ShadowDecision `json:"decision"`
	// WouldEscalate indica se o Kernel TERIA chamado a LLM (true) ou teria
	// respondido sozinho (false, EMIT_OK).
	WouldEscalate bool `json:"would_escalate"`
	// WouldRespond é a resposta EmitOK contrafactual (só quando Decision==EMIT_OK).
	WouldRespond string `json:"would_respond,omitempty"`
	// Confidence é a confiança final (0..1) do breakdown.
	Confidence float64 `json:"confidence"`
	// Convergence é a convergência (0..1) da deliberação.
	Convergence float64 `json:"convergence"`
	// EvidenceIDs são as evidências que alimentaram a decisão.
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
	// Positions é o nº de posições EFETIVAS (substanciadas) coletadas.
	Positions int `json:"positions"`
	// Reason é o motivo em formato humano-legível ("conflicting evidence",
	// "no substantiated evidence", ...).
	Reason string `json:"reason"`
	// Breakdown é a trilha aritmética da confiança (ConfidenceBreakdown.Breakdown).
	Breakdown string `json:"breakdown,omitempty"`
	// DurationMs é a duração da deliberação (microsegundos → ms).
	DurationMs int64 `json:"duration_ms"`
	// At é quando a observação foi registrada.
	At time.Time `json:"at"`
}

// Classify mapeia o Emit do deliberate + o nº de posições EFETIVAS para a
// taxonomia ShadowDecision (ADR-033 §3.3).
//
// Regra "zero achismo": sem posição efetiva (substanciada + com EvidenceIDs),
// a decisão é RETRIEVAL_INSUFFICIENT — o Kernel no estado atual não tem o que
// usar para decidir, e teria escalado.
func Classify(verdict deliberate.Emit, effectivePositions int) ShadowDecision {
	if effectivePositions == 0 {
		return RETRIEVAL_INSUFFICIENT
	}
	switch verdict {
	case deliberate.EmitOK:
		return EMIT_OK
	case deliberate.EmitWithReservations:
		return EMIT_WITH_RESERVATIONS
	default:
		return ESCALATE
	}
}

// WillEscalate informa se a decisão teria escalado para a LLM (false == EMIT_OK,
// o Kernel teria respondido sem LLM).
func (d ShadowDecision) WillEscalate() bool {
	return d != EMIT_OK
}

// String devolve a forma legível da decisão.
func (d ShadowDecision) String() string { return string(d) }

// ─── Store (JSONL append-only) ────────────────────────────────────────────────

// Store é o log do Shadow (JSONL append-only) em `.cosca/shadow/`.
// Thread-safe; escrita serializada por mutex (o writer é único, mas a API é
// segura para leitura concorrente).
type Store struct {
	mu  sync.Mutex
	dir string
}

// NewStore cria um Store apontando para o diretório dado (ex.: `.cosca/shadow`).
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// ForCoscaDir cria o Store a partir do diretório `.cosca` do projeto.
func ForCoscaDir(coscaDir string) *Store {
	return NewStore(filepath.Join(coscaDir, RecordsDir))
}

// RecordsPath devolve o caminho do arquivo JSONL.
func (s *Store) RecordsPath() string {
	return filepath.Join(s.dir, RecordsFile)
}

// Append registra uma observação como uma linha JSONL (append-only).
// Best-effort e fail-closed: NUNCA deve derrubar a execução que a registra.
func (s *Store) Append(t ShadowTrace) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(s.RecordsPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	line, err := json.Marshal(t)
	if err != nil {
		return err
	}
	_, err = f.Write(append(line, '\n'))
	return err
}

// Read lê todas as observações registradas (JSONL). Arquivo inexistente →
// lista vazia. Linhas inválidas são ignoradas (a telemetria não pode derrubar
// o relatório por uma linha corrompida).
func (s *Store) Read() ([]ShadowTrace, error) {
	data, err := os.ReadFile(s.RecordsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var traces []ShadowTrace
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var t ShadowTrace
		if err := json.Unmarshal(line, &t); err != nil {
			continue // nunca derrubar o relatório por uma linha corrompida
		}
		traces = append(traces, t)
	}
	return traces, scanner.Err()
}

// List devolve as observações de um request (vazio = todas).
func (s *Store) List(requestID string) ([]ShadowTrace, error) {
	traces, err := s.Read()
	if err != nil {
		return nil, err
	}
	if requestID == "" {
		return traces, nil
	}
	var out []ShadowTrace
	for _, t := range traces {
		if t.RequestID == requestID {
			out = append(out, t)
		}
	}
	return out, nil
}

// ─── Summary / Report ────────────────────────────────────────────────────────

// Summary é a agregação das observações: distribuição de decisões, taxa de
// escalada, taxa de auto-resolução, confiança e convergência médias.
type Summary struct {
	Total           int            `json:"total"`
	Decisions       map[string]int `json:"decisions"`
	EscalationRate  float64        `json:"escalation_rate"`
	SelfResolveRate float64        `json:"self_resolve_rate"`
	AvgConfidence   float64        `json:"avg_confidence"`
	AvgConvergence  float64        `json:"avg_convergence"`
}

// Aggregate computa o Summary a partir de uma coleção de observações.
func Aggregate(traces []ShadowTrace) Summary {
	s := Summary{
		Total:     len(traces),
		Decisions: map[string]int{},
	}
	if len(traces) == 0 {
		return s
	}
	var confSum, convSum, escalates float64
	for _, t := range traces {
		s.Decisions[string(t.Decision)]++
		confSum += t.Confidence
		convSum += t.Convergence
		if t.WouldEscalate {
			escalates++
		}
	}
	n := float64(len(traces))
	s.EscalationRate = escalates / n
	s.SelfResolveRate = 1.0 - s.EscalationRate
	s.AvgConfidence = confSum / n
	s.AvgConvergence = convSum / n
	return s
}

// HistogramBin é uma faixa da distribuição de confiança (calibração ADR-031).
type HistogramBin struct {
	Low   float64 `json:"low"`
	High  float64 `json:"high"`
	Count int     `json:"count"`
}

// Report é o relatório-ouro do Don (ADR-033 §3.7): taxa de auto-resolução,
// taxa de escalada, distribuição de decisões e histograma de confiança.
type Report struct {
	Summary
	Histogram []HistogramBin `json:"histogram"`
}

// BuildReport computa o relatório-ouro a partir de uma coleção de observações.
func BuildReport(traces []ShadowTrace) Report {
	rep := Report{Summary: Aggregate(traces)}
	rep.Histogram = confidenceHistogram(traces)
	return rep
}

// confidenceHistogram agrupa a confiança em faixas úteis à calibração de
// emit_threshold (0.70) e reservation_threshold (0.50).
func confidenceHistogram(traces []ShadowTrace) []HistogramBin {
	bins := []HistogramBin{
		{Low: 0.00, High: 0.50},
		{Low: 0.50, High: 0.60},
		{Low: 0.60, High: 0.70},
		{Low: 0.70, High: 0.80},
		{Low: 0.80, High: 0.90},
		{Low: 0.90, High: 1.01}, // 1.01 captura c == 1.0
	}
	for _, t := range traces {
		for i := range bins {
			if t.Confidence >= bins[i].Low && t.Confidence < bins[i].High {
				bins[i].Count++
				break
			}
		}
	}
	return bins
}

// Summary agregada do Store.
func (s *Store) Summary() (Summary, error) {
	traces, err := s.Read()
	if err != nil {
		return Summary{}, err
	}
	return Aggregate(traces), nil
}

// Report do Store (o relatório-ouro).
func (s *Store) Report() (Report, error) {
	traces, err := s.Read()
	if err != nil {
		return Report{}, err
	}
	return BuildReport(traces), nil
}

// ─── Default store + conveniências de pacote ──────────────────────────────────

// resolveCoscaDir devolve o diretório `.cosca` do projeto (mesmo padrão de
// resolução usado pelo cost/activity log em api/rest/server.go).
func resolveCoscaDir() string {
	coscaDir := filepath.Join(".", ".cosca")
	if cwd, err := os.Getwd(); err == nil {
		coscaDir = filepath.Join(cwd, ".cosca")
	}
	return coscaDir
}

// DefaultStore devolve o Store do diretório `.cosca` do projeto (criado lazy;
// Append/Read são nil-safe quanto à não-existência do arquivo).
func DefaultStore() *Store {
	return ForCoscaDir(resolveCoscaDir())
}

// Record registra uma observação no store default, best-effort (ignora erro).
// Fail-closed: nunca deve derrubar a execução.
func Record(t ShadowTrace) {
	_ = DefaultStore().Append(t)
}

// Recorded é a variante de Record que devolve o erro de persistência.
func Recorded(ctx context.Context, t ShadowTrace) error {
	_ = ctx // reservado para log de cancelamento futuro
	return DefaultStore().Append(t)
}


