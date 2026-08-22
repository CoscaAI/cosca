//
// `cosca delegate` — delegação com plano de execução prévio (Gate 0).
//
// Fluxo:
//  1. Gera o Plano de Execução (estimator.Estimate) para o escopo informado.
//  2. MOSTRA o plano ao Don (plan.String() — formato exato do estimator).
//  3. Pergunta "Aprovar e delegar? [y/N]" (exceto com --yes).
//  4. Se aprovado: registra a aprovação no audit
//     (.cosca/memory/audit/approvals-{data}.md, mesmo formato do `cosca
//     approve`) e imprime o CONTRATO DE DELEGAÇÃO (agente, escopo, tarefa e
//     plano resumido).
//  5. Se rejeitado: "plano rejeitado — nada executado".
//
// O comando NÃO executa a tarefa — o Kernel delega via agentes. O output
// final é o contrato de delegação: o plano formalizado com a aprovação do Don.
//

package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/discovery"
	"github.com/CoscaAI/cosca/internal/estimator"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/trace"
)

// delegationPrompt é a pergunta de confirmação do Don (Gate 0).
const delegationPrompt = "\nAprovar e delegar? [y/N]: "

// NewDelegateCommand cria o comando `cosca delegate`.
func NewDelegateCommand() *cobra.Command {
	var (
		target     string
		changeType string
		agent      string
		task       string
		assumeYes  bool
		asJSON     bool
		withTrace  bool
	)

	cmd := &cobra.Command{
		Use:   "delegate --target <glob|lista-de-arquivos> [--task <descrição>]",
		Short: "Delegar uma tarefa com plano de execução prévio",
		Long: `Delega uma tarefa a um agente com o Plano de Execução prévio (Gate 0).

Fluxo:
  1. Gera o Plano de Execução para o escopo informado (mesmo estimador do
     "cosca plan").
  2. MOSTRA o plano ao Don — arquivos afetados, testes previstos, migrações,
     rollback, tempo, risco e confiança.
  3. Pergunta "Aprovar e delegar? [y/N]" (exceto com --yes).
  4. Se aprovado, registra a aprovação em
     .cosca/memory/audit/approvals-{data}.md (como o "cosca approve") e imprime
     o contrato de delegação (agente, escopo, tarefa e plano resumido).
  5. Se rejeitado, nada é executado.

O comando NÃO executa a tarefa: o Kernel delega via agentes. O output final é
o contrato de delegação — o plano formalizado com a aprovação do Don.

Flags:
  --target   glob ou lista de arquivos do escopo (ex: "internal/kernel/*.go"
             ou "internal/kernel/*.go,api/rest/**") — obrigatório
  --type     tipo da alteração: feature, fix, security, refactor, docs, test
  --agent    agente que executará a tarefa (default "cosca-backend")
  --task     descrição da tarefa a ser delegada
  --yes, -y  pular a confirmação do Don (para automação)
  --json     saída do contrato de delegação em JSON`,
		Example: `  cosca delegate --target "internal/estimator/*.go" --task "Integrar o estimator no fluxo"
  cosca delegate --target "internal/kernel/*.go,api/rest/**" --type refactor --agent cosca-backend
  cosca delegate --target "internal/estimator/*.go" --yes --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("falha ao obter o diretório de trabalho: %w", err)
			}

			// Knowledge Readiness Gate — antes de delegar, verifica se o Cosca
			// conhece as ferramentas do projeto. Se houver gaps, avisa e sugere
			// aquisição de conhecimento para evitar alucinação dos agentes.
			if !assumeYes {
				checkReadinessGate(cmd, cwd)
			}

			useJSON := asJSON || IsJSONOutput(cmd)

			// --trace (opt-in): cria o Trace ID universal e abre o ledger
			// append-only (.cosca/trace.db). Best-effort: falha no store só
			// imprime aviso — a delegação nunca é bloqueada pelo trace.
			var tr *delegateTrace
			if withTrace {
				trID := trace.NewID()
				traceOut := cmd.OutOrStdout()
				if useJSON {
					traceOut = cmd.ErrOrStderr()
				}
				if _, err := fmt.Fprintf(traceOut, "Trace: %s\n", trID); err != nil {
					return err
				}
				store, storeErr := trace.NewStore(filepath.Join(cwd, ".cosca", "trace.db"))
				if storeErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "⚠ aviso: trace indisponível (%v) — continuando sem trace.\n", storeErr)
				} else {
					tr = &delegateTrace{id: trID, store: store}
					defer func() { _ = store.Close() }()
				}
			}

			scope := estimator.ExecutionScope{
				ProjectDir:  cwd,
				TargetFiles: splitPlanTarget(target),
				ChangeType:  changeType,
				Agent:       agent,
			}

			// Guarda: o target precisa expandir para pelo menos um arquivo.
			files, err := estimator.ExpandScope(scope)
			if err != nil {
				return err
			}
			if len(files) == 0 {
				return fmt.Errorf("nenhum arquivo encontrado para o target: %s", target)
			}

			plan, err := estimator.Estimate(scope)
			if err != nil {
				return fmt.Errorf("falha ao estimar o plano de execução: %w", err)
			}

			// --trace: o plano foi criado (flight recorder — best-effort).
			if tr != nil {
				tr.append(cmd, "PLAN_CREATED", "running", delegateTraceDetails(target, task, plan))
			}

			// 1. Mostra o plano ao Don (formato exato do estimator).
			if !useJSON {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), plan.String()); err != nil {
					return err
				}
				if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
					return err
				}
				if assumeYes {
					if _, err := fmt.Fprintln(cmd.OutOrStdout(),
						"[--yes] Aprovar e delegar: confirmado automaticamente"); err != nil {
						return err
					}
					if _, err := fmt.Fprintln(cmd.OutOrStdout()); err != nil {
						return err
					}
				}
			}

			// 2. Confirmação do Don (exceto com --yes). Em modo JSON o prompt
			// vai para stderr para manter o stdout com JSON puro.
			if !assumeYes {
				promptWriter := cmd.OutOrStdout()
				if useJSON {
					promptWriter = cmd.ErrOrStderr()
				}
				approved, err := confirmDelegation(cmd, promptWriter)
				if err != nil {
					return err
				}
				if !approved {
					if useJSON {
						tr.append(cmd, "DELEGATED", "rejected", "plano rejeitado pelo Don")
						return printDelegationJSON(cmd, target, agent, task, plan, "rejected", "")
					}
					tr.append(cmd, "DELEGATED", "rejected", "plano rejeitado pelo Don")
					if _, err := fmt.Fprintln(cmd.OutOrStdout(),
						"plano rejeitado — nada executado"); err != nil {
						return err
					}
					return nil
				}
			}

			// 3. Registra a aprovação no audit (mesmo formato do `cosca
			// approve`) + o bloco de delegação.
			coscaDir := filepath.Join(cwd, ".cosca")
			auditFile := filepath.Join(coscaDir, approvalAuditDir,
				approvalAuditFilePrefix+time.Now().Format(approvalAuditFileLayout)+".md")

			// --trace: o Don aprovou o plano (best-effort).
			tr.append(cmd, "APPROVED", "success", auditFile)

			if useJSON {
				// Modo JSON: apenas o markdown canônico (sem audit store e sem
				// avisos no stdout), para manter a saída JSON pura.
				if err := appendApprovalMarkdown(auditFile, plan, "approved"); err != nil {
					return fmt.Errorf("falha ao registrar a aprovação: %w", err)
				}
				if err := appendDelegationSection(auditFile, agent, target, task); err != nil {
					return fmt.Errorf("falha ao registrar a delegação: %w", err)
				}
				tr.append(cmd, "DELEGATED", "success", auditFile)
				return printDelegationJSON(cmd, target, agent, task, plan, "approved", auditFile)
			}

			if err := recordApproval(cmd, coscaDir, auditFile, plan, "approved"); err != nil {
				return fmt.Errorf("falha ao registrar a aprovação: %w", err)
			}
			if err := appendDelegationSection(auditFile, agent, target, task); err != nil {
				return fmt.Errorf("falha ao registrar a delegação: %w", err)
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Aprovação registrada em: %s\n\n", auditFile); err != nil {
				return err
			}

			// 4. Contrato de delegação — o output final.
			printDelegationContract(cmd, plan, target, agent, task)
			tr.append(cmd, "DELEGATED", "success", auditFile)
			return nil
		},
	}

	cmd.Flags().StringVar(&target, "target", "",
		`glob ou lista de arquivos do escopo (ex: "internal/kernel/*.go" ou "internal/kernel/*.go,api/rest/**")`)
	cmd.Flags().StringVar(&changeType, "type", "feature",
		"tipo da alteração (feature, fix, security, refactor, docs, test)")
	cmd.Flags().StringVar(&agent, "agent", "cosca-backend",
		"agente que executará a tarefa")
	cmd.Flags().StringVar(&task, "task", "",
		"descrição da tarefa a ser delegada")
	cmd.Flags().BoolVarP(&assumeYes, "yes", "y", false,
		"pular a confirmação do Don (para automação)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "saída do contrato de delegação em JSON")
	cmd.Flags().BoolVar(&withTrace, "trace", false,
		"registra o Trace ID universal e os eventos (PLAN_CREATED, APPROVED, DELEGATED) em .cosca/trace.db — best-effort")
	_ = cmd.MarkFlagRequired("target")
	return cmd
}

// delegateTrace é o estado do --trace no `cosca delegate`: o Trace ID
// universal + o ledger append-only (.cosca/trace.db). Sem --trace é nil e o
// fluxo de delegação não interage com o ledger (zero side effects).
type delegateTrace struct {
	id    trace.TraceID
	store *trace.Store
}

// append registra um evento do trace (actor "kernel") de forma best-effort:
// falha no store imprime aviso no stderr e NUNCA bloqueia a delegação.
func (tr *delegateTrace) append(cmd *cobra.Command, action, result, details string) {
	if tr == nil || tr.store == nil {
		return
	}
	ev := trace.Event{
		TraceID: tr.id.String(),
		Actor:   "kernel",
		Action:  action,
		Result:  result,
		Details: details,
	}
	if err := tr.store.Append(ev); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "⚠ aviso: trace: falha ao registrar %s: %v\n", action, err)
	}
}

// delegateTraceDetails monta o resumo (≤100 chars) dos eventos de trace do
// `cosca delegate`: a tarefa quando informada, senão o escopo + o resumo do
// plano de execução.
func delegateTraceDetails(target, task string, plan *estimator.ExecutionPlan) string {
	d := strings.TrimSpace(task)
	if d == "" {
		d = fmt.Sprintf("escopo %s — %d arquivos · %d testes · %d min · risco %s · confiança %d%%",
			target, plan.FilesAffected, plan.TestsExpected, plan.EstimatedMinutes,
			plan.RiskLevel, plan.ConfidencePercent)
	}
	return truncateTraceDetails(d, 100)
}

// confirmDelegation pede a aprovação do Don ("Aprovar e delegar? [y/N]") e
// devolve true apenas para respostas afirmativas explícitas ("y" ou "yes").
// O prompt é escrito em promptWriter (stdout, ou stderr em modo JSON).
func confirmDelegation(cmd *cobra.Command, promptWriter io.Writer) (bool, error) {
	if _, err := fmt.Fprint(promptWriter, delegationPrompt); err != nil {
		return false, err
	}
	reader := bufio.NewReader(cmd.InOrStdin())
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("falha ao ler a confirmação: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// printDelegationContract imprime o contrato de delegação: agente executor,
// escopo, tarefa e o plano resumido — o output final do `cosca delegate`.
func printDelegationContract(cmd *cobra.Command, plan *estimator.ExecutionPlan, target, agent, task string) {
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, "═══ CONTRATO DE DELEGAÇÃO ═══")
	fmt.Fprintf(w, "Agente executor : %s\n", agent)
	fmt.Fprintf(w, "Escopo          : %s\n", target)
	if task != "" {
		fmt.Fprintf(w, "Tarefa          : %s\n", task)
	}
	fmt.Fprintf(w, "Plano resumido  : %d arquivos · %d testes previstos · %d min · risco %s · confiança %d%%\n",
		plan.FilesAffected, plan.TestsExpected, plan.EstimatedMinutes,
		plan.RiskLevel, plan.ConfidencePercent)
}

// delegationContract é o contrato de delegação emitido por
// `cosca delegate --json`.
type delegationContract struct {
	Status         string                   `json:"status"`
	Agent          string                   `json:"agent"`
	Target         string                   `json:"target"`
	Task           string                   `json:"task,omitempty"`
	Plan           *estimator.ExecutionPlan `json:"plan"`
	ApprovalRecord string                   `json:"approval_record,omitempty"`
}

// printDelegationJSON emite o contrato de delegação em JSON.
func printDelegationJSON(cmd *cobra.Command, target, agent, task string, plan *estimator.ExecutionPlan, status, approvalRecord string) error {
	return printJSON(cmd, delegationContract{
		Status:         status,
		Agent:          agent,
		Target:         target,
		Task:           task,
		Plan:           plan,
		ApprovalRecord: approvalRecord,
	})
}

// appendDelegationSection adiciona o bloco de delegação (agente executor,
// escopo e tarefa) ao arquivo diário de aprovações, logo após o bloco canônico
// do `cosca approve`. Mantém a rastreabilidade de quem executará a tarefa.
func appendDelegationSection(auditFile, agent, target, task string) error {
	f, err := os.OpenFile(auditFile, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("falha ao abrir o arquivo de auditoria %q: %w", auditFile, err)
	}
	defer f.Close()

	var sb strings.Builder
	sb.WriteString("### Delegação\n\n")
	fmt.Fprintf(&sb, "- **Agente executor**: %s\n", agent)
	fmt.Fprintf(&sb, "- **Escopo**: %s\n", target)
	if task != "" {
		fmt.Fprintf(&sb, "- **Tarefa**: %s\n", task)
	}
	sb.WriteString("\n")

	if _, err := f.WriteString(sb.String()); err != nil {
		return fmt.Errorf("falha ao registrar a delegação: %w", err)
	}
	return nil
}

// checkReadinessGate verifica se o Cosca conhece as ferramentas do projeto
// antes de delegar. É um warning não-bloqueante: o Don decide se ignora ou
// resolve os gaps. Usa discovery.DetectProject + knowledge.CheckReadiness.
func checkReadinessGate(cmd *cobra.Command, dir string) {
	info, err := discovery.DetectProject(cmd.Context(), dir, zerolog.Nop())
	if err != nil || len(info.Dependencies) == 0 {
		return // nada a verificar
	}

	localStore := mustPackageStore(dir)
	globalStore := mustGlobalStore()

	report, err := knowledge.CheckReadiness(dir, info.Dependencies, localStore, globalStore)
	if err != nil {
		return
	}

	if report.Sufficient {
		return // tudo pronto, silencioso
	}

	// Gaps encontrados — avisa o Don.
	out := cmd.OutOrStdout()
	fmt.Fprintln(out)
	fmt.Fprintln(out, "┌─ Knowledge Readiness Gate ──────────────────────────────┐")
	fmt.Fprintf(out, "│ ⚠️  %d ferramentas sem conhecimento adequado            │\n", len(report.Gaps))
	fmt.Fprintln(out, "│                                                         │")
	fmt.Fprintln(out, "│ Os agentes podem ALUCINAR ao trabalhar com ferramentas  │")
	fmt.Fprintln(out, "│ desconhecidas. Resolva antes de delegar:                │")
	fmt.Fprintln(out, "│                                                         │")
	for _, g := range report.Gaps {
		fmt.Fprintf(out, "│   %s\n", g)
	}
	fmt.Fprintln(out, "│                                                         │")
	fmt.Fprintln(out, "│ Ou continue assim mesmo (--yes para pular).             │")
	fmt.Fprintln(out, "└─────────────────────────────────────────────────────────┘")
	fmt.Fprintln(out)
}
