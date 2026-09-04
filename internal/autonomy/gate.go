package autonomy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/processutil"
)

// CommandResult é o resultado DETERMINÍSTICO de um gate (análogo a
// ChildProcessResult do prime-agent): basta o exit-code para decidir.
type CommandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	TimedOut bool
	// Err é um erro de infraestrutura (start/wait), NÃO um exit-code normal.
	Err error
}

// Success é true quando o comando saiu 0 sem timeout nem erro de infra.
func (r CommandResult) Success() bool {
	return r.ExitCode == 0 && !r.TimedOut && r.Err == nil
}

// CommandRunner roda um comando shell determinístico. Injetável para mock nos
// testes (o gate NUNCA abre um LLM). `timeout` <= 0 usa o default do runner.
type CommandRunner interface {
	Run(ctx context.Context, command, dir string, timeout time.Duration) (CommandResult, error)
}

// GateOutcome é o resultado de UMA rodada de avaliação dos gates.
type GateOutcome string

const (
	// GateOutcomePassed: todos os gates passaram (exit 0) → qualidade atingida
	// (Modelo A → STOP, ReasonNotNeeded).
	GateOutcomePassed GateOutcome = "passed"
	// GateOutcomeFailed: ao menos um gate falhou (exit ≠ 0), mas ainda há retry
	// disponível → qualidade NÃO atingida (Modelo A → CONTINUE, ReasonGateFailed).
	GateOutcomeFailed GateOutcome = "failed"
	// GateOutcomeRetryExhausted: os gates falharam além do teto de retries → STOP.
	GateOutcomeRetryExhausted GateOutcome = "retry_exhausted"
	// GateOutcomeError: o gate NÃO pôde ser EXECUTADO por erro de infraestrutura
	// (comando não encontrado, spawn falhou). Diferente de "qualidade não
	// atingida": aqui NÃO SE SABE o estado → fail-closed (STOP).
	GateOutcomeError GateOutcome = "error"
)

// MaxGateOutputChars limita a saída de erro do gate retida em memória.
const MaxGateOutputChars = 6000

// evaluateQualityGates roda os gates de qualidade com change-detection
// (idempotência de verificação — prime autonomous.ts:284-348).
//
// Para cada comando de gate:
//   - Se a ÚLTIMA falha foi DESTE comando e o workspace NÃO mudou desde a falha,
//     o gate NÃO é re-executado (economia de ciclos — ADR-031): incrementa a
//     tentativa e devolve failed/retry_exhausted sem rodar nada.
//   - Senão, executa o comando; exit 0 → zera o contador do comando e continua;
//     ≠0 → registra a falha + o snapshot pós-falha e devolve failed/retry_exhausted.
//
// Mutável: atualiza GateAttempts e LastGateFailure/LastGateFailureSnapshot.
func evaluateQualityGates(ctx context.Context, s *RuntimeState, dir string, snap Snapshotter, runner CommandRunner) (GateOutcome, error) {
	if len(s.Gates) == 0 {
		return GateOutcomePassed, nil
	}
	for _, g := range s.Gates {
		current, err := snapish(ctx, snap, dir)
		if err != nil {
			return GateOutcomeFailed, err
		}
		// Change-detection: gate que JÁ falhou, workspace inalterado → não re-executa.
		if s.LastGateFailure != nil && s.LastGateFailure.Command == g.Command &&
			s.LastGateFailureSnapshot != nil && current.equal(s.LastGateFailureSnapshot) {
			attempt := s.GateAttempts[g.Command] + 1
			s.GateAttempts[g.Command] = attempt
			s.LastGateFailure.Attempt = attempt
			s.LastGateFailure.ExitText = "not rerun: workspace unchanged since previous failed gate"
			s.LastGateFailure.Output = "O gate não foi re-executado porque o workspace não mudou desde a falha. Edite fontes, testes ou um artefato bloqueador antes de tentar terminar de novo."
			if attempt > g.MaxRetries {
				return GateOutcomeRetryExhausted, nil
			}
			return GateOutcomeFailed, nil
		}

		// Executa o gate (determinístico, via shell).
		res, err := runner.Run(ctx, g.Command, dir, g.Timeout)
		if err != nil {
			// Erro de INFRAESTRUTURA (não é "qualidade não atingida"): o comando não
			// pôde ser executado → não se sabe o estado → fail-closed (STOP).
			return GateOutcomeError, err
		}
		// Snapshot pós-run: base de comparação do change-detection.
		post, perr := snapish(ctx, snap, dir)
		if perr != nil {
			post = current
		}

		if res.Success() {
			s.GateAttempts[g.Command] = 0
			if s.LastGateFailure != nil && s.LastGateFailure.Command == g.Command {
				s.LastGateFailure = nil
				s.LastGateFailureSnapshot = nil
			}
			continue
		}
		attempt := s.GateAttempts[g.Command] + 1
		s.GateAttempts[g.Command] = attempt
		s.LastGateFailure = &GateFailure{
			Command:  g.Command,
			Attempt:  attempt,
			ExitText: processExit(res),
			Output:   truncateGateOutput(strings.TrimSpace(strings.Join([]string{res.Stdout, res.Stderr}, "\n"))),
		}
		if s.LastGateFailureSnapshot == nil || !post.equal(s.LastGateFailureSnapshot) {
			s.LastGateFailureSnapshot = &post
		}
		if attempt > g.MaxRetries {
			return GateOutcomeRetryExhausted, nil
		}
		return GateOutcomeFailed, nil
	}
	// Todos passaram: limpa o historial de falha.
	s.LastGateFailure = nil
	s.LastGateFailureSnapshot = nil
	return GateOutcomePassed, nil
}

// snapish captura o workspace se o snapshotter estiver presente; senão devolve
// um snapshot vazio (sem change-detection útil — o gate sempre re-executa). Isso
// mantém o caminho crítico determinístico mesmo sem um snapshotter real.
func snapish(ctx context.Context, snap Snapshotter, dir string) (WorkspaceSnapshot, error) {
	if snap == nil {
		return WorkspaceSnapshot{}, nil
	}
	return snap.Capture(ctx, dir)
}

// processExit formata a causa da não-execução do gate (análogo a
// formatProcessExit do prime-agent).
func processExit(r CommandResult) string {
	switch {
	case r.Err != nil:
		return r.Err.Error()
	case r.TimedOut:
		return "timed out"
	default:
		return fmt.Sprintf("exited %d", r.ExitCode)
	}
}

// truncateGateOutput limita a saída retida a MaxGateOutputChars.
func truncateGateOutput(output string) string {
	if len(output) <= MaxGateOutputChars {
		return output
	}
	return output[:MaxGateOutputChars] + "\n... [truncated]"
}

// shellCommand devolve o binário + args do shell nativo: `cmd /C` no Windows,
// `sh -c` no POSIX (mesmo contrato de internal/chat/tool/shell.go).
func shellCommand(command string) (string, []string) {
	if os.PathSeparator == '\\' {
		return "cmd", []string{"/c", command}
	}
	return "sh", []string{"-c", command}
}

// shellRunner é a implementação REAL do CommandRunner via processutil (stdlib
// only, tree-kill, timeout independente de idle). Zero-LLM.
type shellRunner struct {
	defaultTimeout time.Duration
}

// NewShellRunner cria um CommandRunner real que executa comandos no shell do
// sistema com um teto de wall-clock por execução.
func NewShellRunner() CommandRunner {
	return &shellRunner{defaultTimeout: DefaultGateTimeout}
}

// Run executa o comando no shell nativo, com o workdir `dir`. `timeout` <= 0 usa
// o default. O exit-code decide (0 = passou); timeout/erro de infra = NÃO passou.
func (r *shellRunner) Run(ctx context.Context, command, dir string, timeout time.Duration) (CommandResult, error) {
	if timeout <= 0 {
		timeout = r.defaultTimeout
	}
	if timeout <= 0 {
		timeout = DefaultGateTimeout
	}
	name, args := shellCommand(command)
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	res, waitErr := processutil.Run(ctx, cmd, processutil.Config{MaxRuntime: timeout})
	if waitErr != nil {
		return CommandResult{Err: waitErr, TimedOut: timedOutStatus(res)}, waitErr
	}
	return CommandResult{
		ExitCode: res.ExitCode,
		Stdout:   res.Stdout,
		Stderr:   res.Stderr,
		TimedOut: timedOutStatus(res),
		Err:      res.Err,
	}, nil
}

// timedOutStatus reporta se o processo terminou por timeout/idle (para o gate
// NÃO interpretar um exit-code como falha de comando quando foi timeout).
func timedOutStatus(r *processutil.Result) bool {
	if r == nil {
		return false
	}
	switch r.Status {
	case processutil.StatusHardTimeout, processutil.StatusIdleTimeout, processutil.StatusCancelled:
		return true
	}
	return false
}
