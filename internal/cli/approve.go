//
// `cosca approve` — o Don aprova um Plano de Execução.
//
// Fluxo:
//  1. Exibe o plano (formato exato do Don, via plan.String() do estimator).
//  2. Pede confirmação (y/N).
//  3. Se confirmado, roda "go test" nos pacotes afetados (TestPackages) e
//     reporta o resultado.
//  4. Registra a aprovação no audit: best-effort no audit store SQLite
//     (.cosca/audit.db) e sempre no arquivo markdown diário
//     (.cosca/memory/audit/approvals-{data}.md) — o registro canônico.
//
// `--dry-run` mostra exatamente o que seria feito sem executar nada.
//

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/audit"
	"github.com/CoscaAI/cosca/internal/estimator"
	"github.com/CoscaAI/cosca/internal/gate"
	"github.com/CoscaAI/cosca/internal/trace"
)

// approvalAuditDir é o diretório (relativo ao .cosca do projeto) onde as
// aprovações são registradas em markdown.
const approvalAuditDir = "memory/audit"

// approvalAuditFilePrefix é o prefixo do arquivo diário de aprovações.
const approvalAuditFilePrefix = "approvals-"

// approvalAuditFileLayout é o formato da data no nome do arquivo diário.
const approvalAuditFileLayout = "20060102"

// NewApproveCommand cria o comando `cosca approve`.
func NewApproveCommand() *cobra.Command {
	var (
		planSource string
		dryRun     bool
		withGate   bool
		withTrace  bool
	)

	cmd := &cobra.Command{
		Use:   "approve --plan <arquivo-ou-json>",
		Short: "Aprovar um plano de execução (roda testes e registra no audit)",
		Long: `Aprova um Plano de Execução gerado por "cosca plan --json".

Fluxo:
  1. Exibe o plano e pede confirmação (y/N).
  2. Se confirmado, roda "go test" nos pacotes afetados (TestPackages).
  3. Registra a aprovação no audit (SQLite em .cosca/audit.db, quando
     disponível) e no arquivo markdown
     .cosca/memory/audit/approvals-{data}.md.

  --dry-run   mostra o que faria (testes e registro) sem executar nada.
  --gate      (opt-in) registra também o gate do plano (motor de transições
              Gate) e o move até "approved" — best-effort, nunca bloqueia a
              aprovação. Sem esta flag, o fluxo é exatamente o de sempre.
  --trace     (opt-in) registra o Trace ID universal e os eventos
              (TESTS_STARTED, TEST_PASSED/TEST_FAILED, APPROVED) em
              .cosca/trace.db — best-effort, nunca bloqueia a aprovação.`,
		Example: `  cosca plan --target "internal/kernel/*.go" --json > plano.json
  cosca approve --plan plano.json
  cosca approve --plan plano.json --dry-run
  cosca approve --plan plano.json --gate
  cosca approve --plan plano.json --trace
  cosca approve --plan '{"TestPackages":["internal/kernel"]}' --dry-run`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("falha ao obter o diretório de trabalho: %w", err)
			}

			// Knowledge Readiness Gate — verifica gaps antes de aprovar execução.
			if !dryRun {
				checkReadinessGate(cmd, cwd)
			}

			plan, err := loadApprovalPlan(planSource)
			if err != nil {
				return err
			}

			// --trace (opt-in): cria o Trace ID universal e abre o ledger
			// append-only (.cosca/trace.db). Best-effort: falha no store só
			// imprime aviso — a aprovação nunca é bloqueada pelo trace.
			var tr *approveTrace
			if withTrace {
				trID := trace.NewID()
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Trace: %s\n", trID); err != nil {
					return err
				}
				store, storeErr := trace.NewStore(filepath.Join(cwd, ".cosca", "trace.db"))
				if storeErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "⚠ aviso: trace indisponível (%v) — continuando sem trace.\n", storeErr)
				} else {
					tr = &approveTrace{id: trID, store: store}
					defer func() { _ = store.Close() }()
				}
			}

			// 1. Exibe o plano (formato exato do Don).
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), plan.String()); err != nil {
				return err
			}

			coscaDir := filepath.Join(cwd, ".cosca")
			auditFile := filepath.Join(coscaDir, approvalAuditDir,
				approvalAuditFilePrefix+time.Now().Format(approvalAuditFileLayout)+".md")
			testArgs := goTestArgs(cwd, plan.TestPackages)

			if dryRun {
				fmt.Fprintln(cmd.OutOrStdout())
				fmt.Fprintln(cmd.OutOrStdout(), "DRY-RUN — nada será executado.")
				if len(testArgs) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "Testes que seriam executados: go %s\n", strings.Join(testArgs, " "))
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "Testes que seriam executados: nenhum pacote no plano")
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Registro de auditoria: %s\n", auditFile)
				return nil
			}

			// 2. Confirmação do Don (y/N).
			confirmed, err := confirmApproval(cmd)
			if err != nil {
				return err
			}
			if !confirmed {
				fmt.Fprintln(cmd.OutOrStdout(), "Aprovação cancelada pelo Don — nada foi executado.")
				return nil
			}

			// 3. Roda os testes dos pacotes afetados e reporta o resultado.
			testResult := "skipped"
			if len(testArgs) > 0 {
				// --trace: os testes começaram (flight recorder — best-effort).
				tr.append(cmd, "TESTS_STARTED", "running", approveTraceDetails(len(plan.TestPackages)))

				fmt.Fprintln(cmd.OutOrStdout())
				fmt.Fprintln(cmd.OutOrStdout(), "Executando testes dos pacotes afetados...")
				output, testErr := runGoTest(cwd, testArgs)
				fmt.Fprint(cmd.OutOrStdout(), output)
				if testErr != nil {
					testResult = "failed"
					tr.append(cmd, "TEST_FAILED", "failed", approveTraceDetails(len(plan.TestPackages)))
					fmt.Fprintln(cmd.OutOrStdout(), "✗ Testes FALHARAM — a aprovação será registrada com status failed.")
				} else {
					testResult = "passed"
					tr.append(cmd, "TEST_PASSED", "passed", approveTraceDetails(len(plan.TestPackages)))
					fmt.Fprintln(cmd.OutOrStdout(), "✓ Testes passaram.")
				}
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Nenhum pacote de teste no plano — pulando go test.")
			}

			// 4. Registra a aprovação no audit.
			if err := recordApproval(cmd, coscaDir, auditFile, plan, testResult); err != nil {
				return err
			}

			// 5. Registra a trilha de decisão (Decision Trace) — best-effort,
			// nunca bloqueia a aprovação (o markdown continua canônico).
			decisionID := recordDecision(cmd, coscaDir, plan, testResult)

			// --trace: a aprovação foi registrada (flight recorder — best-effort).
			tr.append(cmd, "APPROVED", approveTraceResult(testResult),
				approveTraceApprovedDetails(auditFile, decisionID))

			// 6. (Opt-in) Registra o gate do plano e o move até "approved".
			// Só roda com --gate; best-effort, nunca bloqueia a aprovação.
			if withGate {
				recordGateApproval(cmd, coscaDir, planSource)
			}

			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintf(cmd.OutOrStdout(), "Aprovação registrada em: %s\n", auditFile)
			return nil
		},
	}

	cmd.Flags().StringVar(&planSource, "plan", "",
		"arquivo JSON do plano ou JSON inline (gerado por \"cosca plan --json\")")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "mostra o que faria sem executar")
	cmd.Flags().BoolVar(&withGate, "gate", false,
		"registra também o gate do plano (motor de transições Gate) e o move até approved — best-effort")
	cmd.Flags().BoolVar(&withTrace, "trace", false,
		"registra o Trace ID universal e os eventos (TESTS_STARTED, TEST_PASSED/FAILED, APPROVED) em .cosca/trace.db — best-effort")
	_ = cmd.MarkFlagRequired("plan")
	return cmd
}

// loadApprovalPlan carrega o plano a partir do --plan: um caminho de arquivo
// existente ou JSON inline.
func loadApprovalPlan(source string) (*estimator.ExecutionPlan, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, fmt.Errorf("--plan é obrigatório (arquivo JSON ou JSON inline)")
	}

	data := []byte(source)
	if info, err := os.Stat(source); err == nil && !info.IsDir() {
		data, err = os.ReadFile(source)
		if err != nil {
			return nil, fmt.Errorf("falha ao ler o arquivo de plano %q: %w", source, err)
		}
	}

	var plan estimator.ExecutionPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("plano inválido — esperava o JSON de \"cosca plan --json\": %w", err)
	}
	return &plan, nil
}

// confirmApproval pede a confirmação do Don (y/N) e devolve true apenas para
// respostas afirmativas explícitas ("y" ou "yes", ignorando maiúsculas).
func confirmApproval(cmd *cobra.Command) (bool, error) {
	if _, err := fmt.Fprint(cmd.OutOrStdout(), "\nConfirmar aprovação? [y/N]: "); err != nil {
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

// goTestArgs converte os TestPackages do plano (caminhos absolutos ou
// relativos) em argumentos de "go test" ("./<pkg>/...") relativos ao projeto.
// Pacotes fora do projeto são ignorados.
func goTestArgs(projectDir string, packages []string) []string {
	var args []string
	for _, pkg := range packages {
		pkg = strings.TrimSpace(pkg)
		if pkg == "" {
			continue
		}
		// filepath.IsAbs é específico do SO: no Windows, "/proj/x" sem drive
		// não é absoluto. Tratar também o prefixo "/" (estilo POSIX) como
		// absoluto mantém o comportamento idêntico no Linux e aceita specs
		// cross-platform no Windows (ex.: planos gravados em outro SO).
		if filepath.IsAbs(pkg) || strings.HasPrefix(pkg, "/") {
			rel, err := filepath.Rel(projectDir, pkg)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				continue // pacote fora do projeto — ignora
			}
			pkg = rel
		}
		pkg = filepath.ToSlash(strings.TrimPrefix(pkg, "."+string(filepath.Separator)))
		args = append(args, "./"+pkg+"/...")
	}
	return args
}

// runGoTest executa "go test" com os argumentos dados no diretório do projeto
// e devolve a saída combinada e o erro (se algum pacote falhar).
func runGoTest(projectDir string, args []string) (string, error) {
	cmd := exec.Command("go", append([]string{"test"}, args...)...)
	cmd.Dir = projectDir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// recordApproval registra a decisão do Don: best-effort no audit store SQLite
// (.cosca/audit.db) e sempre no arquivo markdown diário
// (.cosca/memory/audit/approvals-{data}.md), com timestamp, plano e decisão.
func recordApproval(cmd *cobra.Command, coscaDir, auditFile string, plan *estimator.ExecutionPlan, testResult string) error {
	// 1) Audit store SQLite — best-effort (não bloqueia a aprovação).
	store, storeErr := audit.NewStore(filepath.Join(coscaDir, "audit.db"))
	if storeErr == nil {
		entry := &audit.AuditEntry{
			UserID:   "don",
			Action:   "plan.approve",
			Resource: strings.Join(plan.TestPackages, ","),
			Details:  audit.DetailsJSON(approvalAuditDetails(plan, testResult)),
			Status:   approvalStatus(testResult),
		}
		if err := store.Record(entry); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "⚠ aviso: falha ao registrar no audit store: %v\n", err)
		}
		_ = store.Close()
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "⚠ aviso: audit store indisponível (%v) — registrando apenas em markdown.\n", storeErr)
	}

	// 2) Arquivo markdown — registro canônico das aprovações do Don.
	return appendApprovalMarkdown(auditFile, plan, testResult)
}

// appendApprovalMarkdown registra a aprovação do Don no arquivo markdown
// diário (approvals-{data}.md) — o registro canônico. Cria o diretório e o
// cabeçalho do arquivo quando necessário e faz append do bloco de aprovação.
func appendApprovalMarkdown(auditFile string, plan *estimator.ExecutionPlan, testResult string) error {
	if err := os.MkdirAll(filepath.Dir(auditFile), 0o755); err != nil {
		return fmt.Errorf("falha ao criar o diretório de auditoria: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("## Aprovação do Don\n\n")
	fmt.Fprintf(&sb, "- **Data/Hora**: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(&sb, "- **Decisão**: %s\n", decisionText(testResult))
	fmt.Fprintf(&sb, "- **Resultado dos testes**: %s\n", testResult)
	if len(plan.TestPackages) > 0 {
		fmt.Fprintf(&sb, "- **Pacotes testados**: %s\n", strings.Join(plan.TestPackages, ", "))
	}
	if len(plan.Migrations) > 0 {
		fmt.Fprintf(&sb, "- **Migrações**: %s\n", strings.Join(plan.Migrations, ", "))
	}
	fmt.Fprintf(&sb, "- **Método de confiança**: %s\n\n", plan.Method)

	sb.WriteString("### Plano aprovado\n\n```text\n")
	sb.WriteString(plan.String())
	sb.WriteString("\n```\n\n")

	f, err := os.OpenFile(auditFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("falha ao abrir o arquivo de auditoria %q: %w", auditFile, err)
	}
	defer f.Close()

	// Cabeçalho quando o arquivo diário é recém-criado.
	if info, err := f.Stat(); err == nil && info.Size() == 0 {
		if _, err := f.WriteString("# Aprovações de Planos de Execução\n\n"); err != nil {
			return fmt.Errorf("falha ao escrever o cabeçalho de auditoria: %w", err)
		}
	}

	if _, err := f.WriteString(sb.String()); err != nil {
		return fmt.Errorf("falha ao registrar a aprovação: %w", err)
	}
	return nil
}

// approvalAuditDetails monta o payload JSON armazenado no audit store.
func approvalAuditDetails(plan *estimator.ExecutionPlan, testResult string) map[string]interface{} {
	details := map[string]interface{}{
		"files_affected":     len(plan.Files),
		"tests_expected":     plan.TestsExpected,
		"test_result":        testResult,
		"estimated_minutes":  plan.EstimatedMinutes,
		"risk":               plan.RiskLevel,
		"confidence_percent": plan.ConfidencePercent,
		"method":             plan.Method,
	}
	if len(plan.Files) > 0 {
		details["files"] = plan.Files
	}
	if len(plan.TestPackages) > 0 {
		details["test_packages"] = plan.TestPackages
	}
	if len(plan.Migrations) > 0 {
		details["migrations"] = plan.Migrations
	}
	return details
}

// approvalStatus mapeia o resultado dos testes para o Status do audit store.
func approvalStatus(testResult string) string {
	switch testResult {
	case "passed":
		return "success"
	case "failed":
		return "failed"
	default:
		return "approved"
	}
}

// decisionText formata a decisão para o arquivo markdown.
func decisionText(testResult string) string {
	switch testResult {
	case "passed":
		return "approved (testes passaram)"
	case "failed":
		return "approved com testes falhando (exigir correção)"
	default:
		return "approved (sem testes no plano)"
	}
}

// recordDecision registra a trilha de decisão (Decision Trace) para uma
// aprovação do Don — best-effort: se o store de decisões falhar, a aprovação
// segue registrada (o markdown do recordApproval continua canônico). Nunca
// retorna erro para o fluxo de aprovação. Devolve o ID da decisão (D-XXXX)
// registrada, ou "" quando indisponível/falhou.
func recordDecision(cmd *cobra.Command, coscaDir string, plan *estimator.ExecutionPlan, testResult string) string {
	store, err := audit.NewDecisionStore(filepath.Join(coscaDir, "audit.db"))
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "⚠ aviso: trilha de decisão indisponível (%v) — a aprovação segue registrada no audit.\n", err)
		return ""
	}
	defer func() { _ = store.Close() }()

	rec := audit.DecisionRecord{
		Input:       plan.String(),
		KnowledgeUsed: []string{},
		LawsApplied:   []string{},
		Evidence:      []string{},
		Provider:      "", // aprovação é determinística (não-IA)
		Model:         "",
		Approval:      "Don / Gate 0",
		Result:        testResult,
		Rollback:      plan.RollbackDetail,
		Status:        "approved",
		// Fase 2A (ADR-029 §2.4): amarra a decisão ao snapshot de conhecimento
		// vigente — "qual conhecimento o cérebro tinha quando o Don aprovou?".
		// Best-effort: se o lock não existir, fica vazio e a decisão ainda grava.
		KnowledgeSnapshot: audit.ResolveKnowledgeSnapshot(coscaDir),
	}

	id, err := store.Record(rec)
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "⚠ aviso: falha ao registrar a trilha de decisão: %v\n", err)
		return ""
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Trilha de decisão: %s\n", id)
	return id
}

// approveTrace é o estado do --trace no `cosca approve`: o Trace ID
// universal + o ledger append-only (.cosca/trace.db). Sem --trace é nil e o
// fluxo de aprovação não interage com o ledger (zero side effects).
type approveTrace struct {
	id    trace.TraceID
	store *trace.Store
}

// append registra um evento do trace (actor "kernel") de forma best-effort:
// falha no store imprime aviso no stderr e NUNCA bloqueia a aprovação.
func (tr *approveTrace) append(cmd *cobra.Command, action, result, details string) {
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

// approveTraceDetails monta o detalhe (≤100 chars) dos eventos de teste do
// trace do `cosca approve`: a contagem de pacotes do plano.
func approveTraceDetails(packages int) string {
	return fmt.Sprintf("%d pacote(s)", packages)
}

// approveTraceResult mapeia o resultado dos testes para o Result do evento
// APPROVED do trace: "approved" em qualquer caso que não "failed".
func approveTraceResult(testResult string) string {
	if testResult == "failed" {
		return "failed"
	}
	return "approved"
}

// approveTraceApprovedDetails monta o detalhe do evento APPROVED: o caminho
// do arquivo de auditoria + o ID da trilha de decisão, quando registrado.
func approveTraceApprovedDetails(auditFile, decisionID string) string {
	d := auditFile
	if decisionID != "" {
		d += fmt.Sprintf(" · decisão %s", decisionID)
	}
	return truncateTraceDetails(d, 100)
}

// recordGateApproval é o hook OPT-IN do motor de transições Gate em
// `cosca approve` (flag --gate). Após a aprovação (audit + decisão), cria o
// gate do plano (G-XXXX, estado plan) e o move até "approved" — tudo como o
// papel "don" (o approver). É 100% best-effort: qualquer falha imprime um
// aviso e NUNCA bloqueia a aprovação (o fluxo sem --gate permanece idêntico).
func recordGateApproval(cmd *cobra.Command, coscaDir, planRef string) {
	store, err := gate.NewGateStore(filepath.Join(coscaDir, "gate.db"))
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "⚠ aviso: gate indisponível (%v) — a aprovação segue registrada no audit.\n", err)
		return
	}
	defer func() { _ = store.Close() }()

	id, err := store.Create(planRef, gate.RoleDon)
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "⚠ aviso: falha ao criar o gate do plano: %v\n", err)
		return
	}
	// O Don submete o plano à análise e o aprova: plan → approving → approved.
	if err := store.Move(id, gate.StateApproving, gate.RoleDon, gate.RoleDon); err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "⚠ aviso: falha ao mover o gate %s para approving: %v\n", id, err)
		return
	}
	if err := store.Move(id, gate.StateApproved, gate.RoleDon, gate.RoleDon); err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "⚠ aviso: falha ao mover o gate %s para approved: %v\n", id, err)
		return
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Gate %s: plano aprovado (%s) — motor de transições Gate registrado.\n", id, string(gate.StateApproved))
}
