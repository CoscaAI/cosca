// Package autonomy implementa a camada de AUTONOMIA COM LIMITES DETERMINÍSTICOS
// (ADAPTER P4, extraído da mineração ADR-017 do prime-agent:
// packages/coding-agent/src/core/autonomous.ts). É a régua I1/I2 levada ao loop
// de continuação: um loop só prossegue sob (a) limites determinísticos de
// continuations/turns/tokens/timeout, (b) um gate de qualidade via shell cujo
// exit-code decide, e (c) verificação sem desperdício (change-detection: um gate
// que já falhou NÃO é re-executado se o workspace não mudou desde a falha).
//
// DETERMINÍSTICO E ZERO-LLM: nenhuma decisão aqui consulta um modelo — o
// `ShouldContinue` é uma função pura sobre o estado + o exit-code dos gates
// (comandos reais como `go build ./...` / `go test ./...`). Fail-closed (I2):
// sem evidência terminal (gate falhou) → NÃO continua; limite atingido → NÃO
// continua. NUNCA endereça um LLM no caminho crítico.
//
// ORÇAMENTO (nuance ADR-031): o teto de tokens de um loop longo conta input +
// output + cacheWrite e EXCLUI cacheRead — recontexto servido do cache não deve
// esgotar o orçamento (prime-agent autonomous.ts:186-194).
//
// O pacote é stdlib-only (nenhuma dependência externa; só stdlib +
// internal/processutil para rodar o gate real) e NÃO acopla ao domínio
// (task/orchestrator/trading): ele lê contadores + exit-codes. A composição com
// o TaskOrchestrator acontece na borda — o Kernel injeta a decisão, o
// orchestrator continua decidindo CONTINUE|COMPLETE|NOOP por objetivo.
package autonomy

import (
	"context"
	"strconv"
	"time"
)

// Limits são os tetos DETERMINÍSTICOS do loop de continuação. Zero usa o default
// (conservador, fail-closed); um valor positivo é o teto. Não há "infinito":
// o loop É parado por um destes limites ou por um gate que falhou.
type Limits struct {
	// MaxContinuations é o teto de CONTINUEs concedidos (escala p/ o Don).
	MaxContinuations int
	// MaxTurns é o teto de rodadas de trabalho (turns).
	MaxTurns int
	// MaxTokens é o teto de tokens de ORÇAMENTO (input+output+cacheWrite,
	// EXCLUI cacheRead — nuance ADR-031). Um loop longo que só recarrega
	// contexto do cache NÃO esgota o orçamento.
	MaxTokens int
	// Timeout é o teto de wall-clock desde o início da autonomia.
	Timeout time.Duration
}

// DefaultLimits espelha os defaults conservadores do prime-agent
// (autonomous.ts:48-55), com MaxContinuations a 3.
var DefaultLimits = Limits{
	MaxContinuations: 3,
	MaxTurns:         12,
	MaxTokens:        80_000,
	Timeout:          30 * time.Minute,
}

// DefaultGateTimeout é o teto de wall-clock de UMA execução de gate.
const DefaultGateTimeout = 5 * time.Minute

// DefaultGateMaxRetries é o número padrão de tentativas de um gate antes de o
// retry esgotar (falha terminal).
const DefaultGateMaxRetries = 3

// GateConfig é UM gate de qualidade via shell (ex.: "go build ./..."). O exit
// code decide: 0 → passa; ≠0 → falha (sem evidência terminal).
type GateConfig struct {
	// Command é o comando determinístico (ex.: "go test ./..."). Rodado via
	// shell (`cmd /c` no Windows, `sh -c` no POSIX).
	Command string
	// MaxRetries é o teto de tentativas do gate (0 → DefaultGateMaxRetries).
	MaxRetries int
	// Timeout é o teto de wall-clock de UMA execução (0 → DefaultGateTimeout).
	Timeout time.Duration
}

// Config parametriza a autonomia.
type Config struct {
	// Enabled liga a camada (false → ShouldContinue responde "disabled").
	Enabled bool
	// Limits são os tetos determinísticos (0 → DefaultLimits).
	Limits Limits
	// Gates são os gates de qualidade (opcional; vazio → sem gate, só bounds).
	Gates []GateConfig
}

// GateFailure é o registro da ÚLTIMA falha de gate (retenção para o
// change-detection: se o workspace não mudou desde a falha, o gate não é
// re-executado — economia de ciclos, premisa do ADR-031).
type GateFailure struct {
	Command  string
	Attempt  int // tentativa corrente (1-based)
	ExitText string
	Output   string
}

// RuntimeState é o estado MUTÁVEL da autonomia (análogo Go de
// AutonomousRuntimeState do prime-agent). Live mutado por Enable/Disable,
// AddTurn/AddContinuation e por evaluateQualityGates; NUNCA por um LLM.
type RuntimeState struct {
	Enabled         bool
	Limits          Limits
	Gates           []GateConfig
	ContinuationsUsed int
	TurnsUsed       int
	TokensUsed      int
	// StartedAt marca o início da autonomia ativa (zero quando desabilitada).
	StartedAt time.Time

	// GateAttempts conta tentativas por comando de gate (persistência do loop).
	GateAttempts map[string]int
	// LastGateFailure é a última falha observada (nil quando o último gate passou).
	LastGateFailure *GateFailure
	// LastGateFailureSnapshot é o workspace logo APÓS a última falha — a base de
	// comparação do change-detection.
	LastGateFailureSnapshot *WorkspaceSnapshot
}

// BudgetDelta devolve os tokens contados para o ORÇAMENTO de um loop longo:
// input + output + cacheWrite, EXCLUINDO cacheRead (prime autonomous.ts:186-194
// + nuance ADR-031). cache-read é recontexto servido do cache; contá-lo
// cumulativamente esgotaria o orçamento antes de o trabalho não-cacheadado
// atingir o teto.
func BudgetDelta(inputTokens, outputTokens, cacheWriteTokens int) int {
	return inputTokens + outputTokens + cacheWriteTokens
}

// New cria um estado de autonomia com defaults conservadores. Se `cfg.Enabled`
// for true, o estado nasce PROVISIONADO (StartedAt zero) — o borda deve chamar
// Enable(now) para marcar o início real do relógio de wall-clock timeout.
func New(cfg Config) *RuntimeState {
	l := normalizeLimits(cfg.Limits)
	s := &RuntimeState{
		Enabled:      cfg.Enabled,
		Limits:       l,
		Gates:        normalizeGates(cfg.Gates),
		GateAttempts: map[string]int{},
	}
	return s
}

// Enable inicia (ou reinicia) a autonomia ativa em `now`: zera os contadores e o
// historial de gates — exatamente como prime-agent setAutonomousEnabled(true).
func (s *RuntimeState) Enable(now time.Time) {
	s.Enabled = true
	s.StartedAt = now
	s.ContinuationsUsed = 0
	s.TurnsUsed = 0
	s.TokensUsed = 0
	s.GateAttempts = map[string]int{}
	s.LastGateFailure = nil
	s.LastGateFailureSnapshot = nil
}

// Disable desliga a autonomia e limpa o historial (startedAt volta a zero).
func (s *RuntimeState) Disable() {
	s.Enabled = false
	s.StartedAt = time.Time{}
	s.GateAttempts = map[string]int{}
	s.LastGateFailure = nil
	s.LastGateFailureSnapshot = nil
}

// AddTurn acrescenta uma rodada de trabalho: TurnsUsed++ e TokensUsed +=
// BudgetDelta(input, output, cacheWrite) — cacheRead NÃO entra (nuance ADR-031).
// É no-op quando a autonomia está desabilitada.
func (s *RuntimeState) AddTurn(inputTokens, outputTokens, cacheWriteTokens int) {
	if !s.Enabled {
		return
	}
	s.TurnsUsed++
	s.TokensUsed += BudgetDelta(inputTokens, outputTokens, cacheWriteTokens)
}

// AddContinuation registra uma continuação concedida (ContinuationsUsed++).
// No-op quando desabilitada.
func (s *RuntimeState) AddContinuation() {
	if !s.Enabled {
		return
	}
	s.ContinuationsUsed++
}

// LimitReason devolve o limite que foi atingido e true; ou ("", false) quando
// nenhum teto foi alcançado. A ordem é a do prime-agent autonomousLimitReason.
func (s *RuntimeState) LimitReason(now time.Time) (LimitReason, bool) {
	if s.Limits.MaxContinuations > 0 && s.ContinuationsUsed >= s.Limits.MaxContinuations {
		return LimitContinuations, true
	}
	if s.Limits.MaxTurns > 0 && s.TurnsUsed >= s.Limits.MaxTurns {
		return LimitTurns, true
	}
	if s.Limits.MaxTokens > 0 && s.TokensUsed >= s.Limits.MaxTokens {
		return LimitTokens, true
	}
	if s.Limits.Timeout > 0 && !s.StartedAt.IsZero() && now.Sub(s.StartedAt) >= s.Limits.Timeout {
		return LimitTimeout, true
	}
	return "", false
}

// LimitReason identifica QUAL teto determinístico parou o loop.
type LimitReason string

const (
	LimitContinuations LimitReason = "max_continuations"
	LimitTurns         LimitReason = "max_turns"
	LimitTokens        LimitReason = "max_tokens"
	LimitTimeout       LimitReason = "timeout"
)

// Reason é o motivo determinístico do veredito ShouldContinue. Semântica MODELO
// A (prime-agent autonomous.ts:227-252): gate_failed → CONTINUE (qualidade não
// atingida ainda — segue trabalhando pra consertar); gate_passed → STOP (meta
// alcançada); gate_error/infra → STOP (não sei o estado, fail-closed); limites /
// retry esgotado → STOP.
type Reason string

const (
	// ReasonLimitReached — algum teto (continuations/turns/tokens/timeout) OU o
	// retry do gate esgotou: o loop para. `Decision.Limit` identifica o teto.
	ReasonLimitReached Reason = "limit_reached"
	// ReasonNotNeeded — todos os gates passaram (qualidade/objetivo atingido):
	// não há mais trabalho a fazer — STOP. Ou um sinal terminal veio da borda.
	ReasonNotNeeded Reason = "not_needed"
	// ReasonGateFailed — um gate de qualidade falhou (ex.: go build ≠0): a
	// qualidade ainda NÃO foi atingida. Modelo A → CONTINUE (trabalhar pra
	// consertar); a evidência terminal ainda não existe.
	ReasonGateFailed Reason = "gate_failed"
	// ReasonGateError — o gate não pôde ser EXECUTADO por erro de infraestrutura
	// (ex.: comando não encontrado, spawn falhou). Diferente de "qualidade não
	// atingida": aqui NÃO SE SABE o estado. Fail-closed → STOP (não continua).
	ReasonGateError Reason = "gate_error"
	// ReasonMissingEvidence — sem gates configurados e sob os limites (loop com
	// trabalho por fazer): continua. O "primeiro" caso do loop "não provou que
	// terminou, continua até limite/evidência".
	ReasonMissingEvidence Reason = "missing_terminal_evidence"
	// ReasonDisabled — a camada de autonomia está desligada; nada a decidir.
	ReasonDisabled Reason = "disabled"
)

// Decision é o veredito determinístico de ShouldContinue.
type Decision struct {
	// Continue é true sse o loop deve receber OUTRA continuação.
	Continue bool
	// Reason é o motivo determinístico do veredito.
	Reason Reason
	// Limit identifica o teto atingido quando Reason == ReasonLimitReached.
	Limit LimitReason
	// Detailed é a saída textual do gate/limite para observabilidade.
	Detailed string
}

// ShouldContinue devolve o veredito DETERMINÍSTICO (zero-LLM) de autonomia.
//
// SEMÂNTICA — MODELO A (prime-agent shouldAutonomouslyContinue):
//
//	disabled            → STOP (nada a decidir; ReasonDisabled)
//	limite atingido     → STOP (ReasonLimitReached + Limit)
//	gates:
//	  todos PASSARAM    → STOP (ReasonNotNeeded — qualidade/objetivo atingido)
//	  algum FALHOU      → CONTINUE (ReasonGateFailed — consertar, sem evidência)
//	  erro de INFRA     → STOP (ReasonGateError — não se sabe o estado, fail-closed)
//	  retry esgotado    → STOP (ReasonLimitReached)
//	sem gates, sob limite → CONTINUE (ReasonMissingEvidence — trabalho a fazer)
//
// Ordem de precedência (fail-closed, sem desperdício de ciclos):
// disabled → limite → gates (change-detection) → resto. `snap` e `runner` são
// injetáveis (mock nos testes); `dir` é o worktree onde os gates rodam.
func ShouldContinue(ctx context.Context, s *RuntimeState, dir string, snap Snapshotter, runner CommandRunner, now time.Time) (Decision, error) {
	if !s.Enabled {
		return Decision{Continue: false, Reason: ReasonDisabled}, nil
	}
	if reason, ok := s.LimitReason(now); ok {
		return Decision{Continue: false, Reason: ReasonLimitReached, Limit: reason,
			Detailed: "limite de autonomia atingido: " + string(reason)}, nil
	}
	if len(s.Gates) > 0 {
		outcome, err := evaluateQualityGates(ctx, s, dir, snap, runner)
		if err != nil {
			// Erro de infra no gate: fail-closed — NÃO continua (não se sabe o estado).
			return Decision{Continue: false, Reason: ReasonGateError, Detailed: err.Error()}, err
		}
		switch outcome {
		case GateOutcomePassed:
			// Qualidade atingida → não há mais trabalho a fazer.
			return Decision{Continue: false, Reason: ReasonNotNeeded,
				Detailed: "gate de qualidade passou (meta atingida)"}, nil
		case GateOutcomeFailed:
			// Qualidade ainda não atingida → CONTINUE (consertar), sem evidência
			// terminal. Modelo A.
			return Decision{Continue: true, Reason: ReasonGateFailed,
				Detailed: gateDetails(s)}, nil
		case GateOutcomeRetryExhausted:
			return Decision{Continue: false, Reason: ReasonLimitReached,
				Detailed: "gate de qualidade esgotou as tentativas"}, nil
		default: // GateOutcomeError — caller não deveria chegar aqui (err != nil já curto-circuita).
			return Decision{Continue: false, Reason: ReasonGateError,
				Detailed: "erro de infra no gate"}, err
		}
	}
	return Decision{Continue: true, Reason: ReasonMissingEvidence,
		Detailed: "em progresso, sob limites, sem evidência terminal ainda"}, nil
}

// gateDetails monta a descrição da última falha de gate (observabilidade).
func gateDetails(s *RuntimeState) string {
	if s.LastGateFailure == nil {
		return "gate de qualidade falhou"
	}
	d := "gate de qualidade falhou: " + s.LastGateFailure.Command + " " +
		"(tentativa " + strconv.Itoa(s.LastGateFailure.Attempt) + ") " + s.LastGateFailure.ExitText
	if s.LastGateFailure.Output != "" {
		d += "\n" + s.LastGateFailure.Output
	}
	return d
}

// ── defaults construtivos ───────────────────────────────────────────────────

func normalizeLimits(l Limits) Limits {
	if l.MaxContinuations <= 0 {
		l.MaxContinuations = DefaultLimits.MaxContinuations
	}
	if l.MaxTurns <= 0 {
		l.MaxTurns = DefaultLimits.MaxTurns
	}
	if l.MaxTokens <= 0 {
		l.MaxTokens = DefaultLimits.MaxTokens
	}
	if l.Timeout <= 0 {
		l.Timeout = DefaultLimits.Timeout
	}
	return l
}

func normalizeGates(gates []GateConfig) []GateConfig {
	if len(gates) == 0 {
		return nil
	}
	out := make([]GateConfig, len(gates))
	for i, g := range gates {
		out[i] = g
		if out[i].MaxRetries <= 0 {
			out[i].MaxRetries = DefaultGateMaxRetries
		}
		if out[i].Timeout <= 0 {
			out[i].Timeout = DefaultGateTimeout
		}
	}
	return out
}
