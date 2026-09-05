//
// `cosca cofre` — a fronteira de validação de dados do Cosca (zona Cofre).
//
// O Cofre (ADR-012 & ORACLE_PROTOCOL) é a zona air-gap que VALIDA a ENTRADA de
// dados: o Cosca externo (Kernel, com internet) explora e monta um
// `SemanticPackage`; o Oráculo (determinístico, dentro do Cofre) valida e
// devolve uma DECISÃO (ACCEPT / ACCEPT_WITH_CAVEAT / REJECT / INCONCLUSIVE)
// + gaps/perguntas. A IA local pode COMPREENDER, mas NUNCA decide.
//
// Este comando é um guard puramente local — NUNCA abre a rede. Ele existe para
// que o Kernel possa submeter um pacote à validação E para que a mesma
// validação rode DENTRO do Cofre. A decisão é 100% `Gate.Evaluate()`.
//
// Subcomandos:
//   validate <file|json>   Recebe um SemanticPackage e roda o Gate → DECISÃO
//   gate                   Mostra as regras do Gate (condições → decisão) p/ auditoria
//   health                 Diagnóstico do Cofre (bwrap, jail, air-gap)
//

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/oracle"
	"github.com/CoscaAI/cosca/pkg/cosca"
)

// NewCofreCommand cria a árvore de comandos `cosca cofre`.
func NewCofreCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cofre",
		Short: "Fronteira de validação de dados do Cosca — o Oráculo determinístico da zona Cofre (air-gap)",
		Long: `O Cofre é a fronteira semântica de ENTRADA de dados do Cosca (ADR-012).

O Cosca externo (zona Kernel, com internet) explora e monta um pacote
semântico; o Oráculo (determinístico, fail-closed) valida o SIGNIFICADO — nunca
só a forma — e devolve uma DECISÃO:

  ACCEPT | ACCEPT_WITH_CAVEAT | REJECT | INCONCLUSIVE

Regras inegociáveis (ORACLE_PROTOCOL §25-§29):
  • Nenhuma informação externa ganha autoridade só por entrar no Cofre.
  • Sem evidência suficiente → INCONCLUSIVE (nunca inventar conclusão).
  • Fail-closed: sem prova, não passa.
  • A IA local pode COMPREENDER, mas NUNCA decide. A decisão é 100% determinística.

Nenhum subcomando abre a rede — o Cofre é um guard puramente local.

Subcomandos:
  validate <file|json>   Recebe um SemanticPackage e roda o Gate → DECISÃO
  gate                   Mostra as regras do Gate (condições → decisão) p/ auditoria
  health                 Diagnóstico do Cofre (bwrap, jail, air-gap)`,
		Example: `  cosca cofre validate ./packet.json
  cosca cofre validate '{"intent":"...","result":"...","confidence":"HIGH"}'
  cat packet.json | cosca cofre validate
  cosca cofre validate --json
  cosca cofre gate
  cosca cofre health`,
	}

	cmd.AddCommand(
		NewCofreValidateCommand(),
		NewCofreGateCommand(),
		NewCofreHealthCommand(),
	)
	return cmd
}

// ─── validate ─────────────────────────────────────────────────────────────────

// cofreValidateResult é a saída em JSON do `cofre validate`: o pacote semântico
// recebido + o veredito do Oráculo.
type cofreValidateResult struct {
	Package oracle.SemanticPackage `json:"package"`
	Verdict oracle.Verdict         `json:"verdict"`
}

// NewCofreValidateCommand cria `cosca cofre validate <file|json>`.
//
// Fluxo (entrada → decisão):
//   entrada (arquivo JSON | JSON inline | stdin)
//     → parse de SemanticPackage
//     → Gate.Evaluate (determinístico, fail-closed, NUNCA abre a rede)
//     → DECISÃO + gaps/perguntas (texto ou JSON --json)
//     → exit code: 0 para ACCEPT/ACCEPT_WITH_CAVEAT, 1 REJECT, 2 INCONCLUSIVE
func NewCofreValidateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [file|json]",
		Short: "Valida um SemanticPackage e produz a DECISÃO do Oráculo (+ gaps/perguntas)",
		Long: `Recebe um pacote semântico (SemanticPackage) e roda o Gate.Evaluate() — o
bloqueio determinístico da zona Cofre. NUNCA abre a rede: é um guard puramente
local.

A entrada pode vir de:
  • arquivo JSON:      cosca cofre validate ./pacote.json
  • JSON inline:       cosca cofre validate '{"intent":...}'
  • stdin:             cat pacote.json | cosca cofre validate

A saída é a DECISÃO (ACCEPT | ACCEPT_WITH_CAVEAT | REJECT | INCONCLUSIVE) e o
feedback estruturado (contract gaps, semantic gaps, required, questions, reason)
— nunca um simples "invalid" (§20). Use --json para saída máquina-legível.

Exit codes:
  0  ACCEPT / ACCEPT_WITH_CAVEAT  (o pacote passou pelo guard)
  1  REJECT                       (contrato/semântica incompatível)
  2  INCONCLUSIVE                 ("ainda não sabemos" — não passou, não inventado)`,
		Example: `  cosca cofre validate ./pacote.json
  cosca cofre validate '{"intent":"diagnosticar","result":"achei X","evidence":"bench 29s→5.4s","provenance":"commit abc","confidence":"HIGH"}'
  cat pacote.json | cosca cofre validate --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			pkg, err := readSemanticPackage(cmd, args)
			if err != nil {
				return err
			}

			// O Gate é o único que decide (fail-closed). A IA local (se algum dia
			// conectada) entra só como COMPREENSOR, nunca veredito.
			verdict := oracle.NewGate().Evaluate(pkg)

			if useJSON {
				return printJSON(cmd, cofreValidateResult{Package: pkg, Verdict: verdict})
			}

			printCofreVerdict(formatter, pkg, verdict)

			if code := cofreExitCode(verdict.Decision); code != 0 {
				return ExitCodeError{Code: code}
			}
			return nil
		},
	}
	return cmd
}

// readSemanticPackage resolve a entrada do pacote semântico: arquivo, JSON
// inline ou stdin (é preciso fornecer um dos três).
func readSemanticPackage(cmd *cobra.Command, args []string) (oracle.SemanticPackage, error) {
	var raw []byte
	var err error

	switch {
	case len(args) == 1:
		// Se o argumento é um arquivo existente, lê o arquivo; senão interpreta
		// como JSON inline (o Kernel pode passar o pacote como string).
		if info, statErr := os.Stat(args[0]); statErr == nil && !info.IsDir() {
			raw, err = os.ReadFile(args[0])
		} else {
			raw = []byte(args[0])
		}
	default:
		raw, err = io.ReadAll(cmd.InOrStdin())
	}
	if err != nil {
		return oracle.SemanticPackage{}, fmt.Errorf("falha ao ler o pacote semântico: %w", err)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return oracle.SemanticPackage{}, fmt.Errorf("nenhum pacote semântico fornecido — passe um arquivo JSON, JSON inline ou stdin")
	}

	var pkg oracle.SemanticPackage
	if err := json.Unmarshal(raw, &pkg); err != nil {
		return oracle.SemanticPackage{}, fmt.Errorf("JSON de pacote semântico inválido: %w", err)
	}
	return pkg, nil
}

// printCofreVerdict imprime a decisão do Oráculo em texto.
func printCofreVerdict(f *OutputFormatter, pkg oracle.SemanticPackage, v oracle.Verdict) {
	f.Header("Cofre — Verdict do Oráculo")
	f.KeyValue("Decisão", string(v.Decision))
	f.KeyValue("Intenção", emptyDash(pkg.Intent))
	f.KeyValue("Resultado", emptyDash(pkg.Result))
	f.KeyValue("Fonte", emptyDash(pkg.Source))
	f.KeyValue("Proveniência", emptyDash(pkg.Provenance))
	f.KeyValue("Confiança", stringConfidence(pkg.Confidence))
	f.Println("")

	if len(v.ContractGaps) > 0 {
		f.Header("Contract Gaps")
		for _, g := range v.ContractGaps {
			f.Bullet(g)
		}
	}
	if len(v.SemanticGaps) > 0 {
		f.Header("Semantic Gaps")
		for _, g := range v.SemanticGaps {
			f.Bullet(g)
		}
	}
	if len(v.Required) > 0 {
		f.Header("Required")
		for _, r := range v.Required {
			f.Bullet(r)
		}
	}
	if len(v.Questions) > 0 {
		f.Header("Perguntas do Oráculo")
		for _, q := range v.Questions {
			f.Bullet(q)
		}
	}

	f.Println("")
	if v.Reason != "" {
		f.KeyValue("Motivo", v.Reason)
	}
}

func emptyDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func stringConfidence(c oracle.Confidence) string {
	if c == "" {
		return "—"
	}
	return string(c)
}

// cofreExitCode mapeia a decisão para o exit code do processo (guarda útel p/ scripts).
func cofreExitCode(d oracle.Decision) int {
	switch d {
	case oracle.Accept, oracle.AcceptWithCaveat:
		return 0
	case oracle.Inconclusive:
		return 2
	default: // Reject
		return 1
	}
}

// ─── gate ─────────────────────────────────────────────────────────────────────

// cofreGateRule é uma linha das regras determinísticas do Gate (auditoria).
type cofreGateRule struct {
	Condition string
	Decision  string
}

// cofreGateRules reflete (não substitui) as regras implementadas em
// Gate.Evaluate (internal/oracle/oracle.go..Evaluate). Fonte de auditoria —
// a verdade normativa é o código, este é o espelho legível.
var cofreGateRules = []cofreGateRule{
	{"Pacote vazio", "REJECT"},
	{"Sem intenção", "REJECT"},
	{"Sem resultado", "REJECT"},
	{"Resultado sem evidência/proveniência/confiança", "INCONCLUSIVE + perguntas"},
	{"Confidence HIGH sem evidência", "REJECT"},
	{"Confidence HIGH sem proveniência", "REJECT"},
	{"Afirmação causal sem evidência causal", "ACCEPT_WITH_CAVEAT (correlação ≠ causa)"},
	{"Contradição com a memória assinada", "ACCEPT_WITH_CAVEAT + perguntas"},
	{"Confiança baixa ou não declarada / sem evidência", "ACCEPT_WITH_CAVEAT (exploração)"},
	{"Contrato completo + evidência + proveniência", "ACCEPT"},
}

// NewCofreGateCommand cria `cosca cofre gate` — a auditoria das regras do guard.
func NewCofreGateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "gate",
		Short: "Mostra as regras determinísticas do Gate (condições → decisão) para auditoria",
		Long: `Imprime as regras do bloqueio determinístico do Cofre. Este é o espelho
auditável do Gate.Evaluate() (internal/oracle/oracle.go) — a verdade normativa
é o código; a tabela abaixo documenta as transições condição → decisão.

Nenhum subcomando abre a rede. O Gate é fail-closed: na dúvida, não passa.`,
		Example: `  cosca cofre gate
  cosca cofre gate --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			if useJSON {
				return printJSON(cmd, cofreGateRules)
			}

			f.Header("Cofre — Regras do Gate (fail-closed)")
			f.Println("Condição → Decisão. Na dúvida, não passa.")
			rows := make([][]string, 0, len(cofreGateRules))
			for _, r := range cofreGateRules {
				rows = append(rows, []string{r.Decision, r.Condition})
			}
			f.Table([]string{"Decisão", "Condição"}, rows)
			return nil
		},
	}
}

// ─── health ───────────────────────────────────────────────────────────────────

// cofreHealth é o diagnóstico do estado do Cofre no ambiente atual.
type cofreHealth struct {
	BwrapAvailable   bool   `json:"bwrap_available"`
	InsideJail       bool   `json:"inside_jail"`
	NetworkReachable bool   `json:"network_reachable"`
	AirGapActive     bool   `json:"airgap_active"`
	Message          string `json:"message"`
	Warning          string `json:"warning,omitempty"`
}

// NewCofreHealthCommand cria `cosca cofre health` — diagnóstica o air-gap.
func NewCofreHealthCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Diagnóstico do Cofre: bwrap disponível, dentro da jaula, air-gap ativo",
		Long: `Diagnostica se a zona Cofre está na configuração esperada:
  • bwrap disponível?      (isolamento de namespace/FS — Linux)
  • dentro da jaula?       (cosca.InsideJail)
  • modo air-gap?          (sem alcance de rede — a premissa do Oracle puro)

AVISO: se a rede estiver acessível OU o bwrap/jaula estiver ausente, o Cofre
NÃO está air-gap de verdade (ADR-012 §3.7 / threat-model-cobre-windows §0/V9).
Este comando é diagnóstico best-effort — nunca decide por si.`,
		Example: `  cosca cofre health
  cosca cofre health --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			_, bwrapErr := exec.LookPath("bwrap")
			insideJail := cosca.InsideJail()
			netReach := networkReachable(400 * time.Millisecond)

			airGap := bwrapErr == nil && insideJail && !netReach

			h := cofreHealth{
				BwrapAvailable:   bwrapErr == nil,
				InsideJail:       insideJail,
				NetworkReachable: netReach,
				AirGapActive:     airGap,
				Message:          airGapMessage(bwrapErr == nil, insideJail, netReach),
			}
			if !airGap {
				h.Warning = "O Cofre NÃO está air-gap de verdade — não trate este processo como zona isolada."
			}

			if useJSON {
				return printJSON(cmd, h)
			}

			f.Header("Cofre — Health (diagnóstico best-effort)")
			printHealthItem(f, "bwrap disponível", h.BwrapAvailable)
			printHealthItem(f, "Dentro da jaula", h.InsideJail)
			printHealthItem(f, "Sem alcance de rede", !h.NetworkReachable)
			f.Println("")
			f.KeyValue("Air-gap ativo", cofreYesNo(h.AirGapActive))
			f.KeyValue("Mensagem", h.Message)
			if h.Warning != "" {
				f.Warning(h.Warning)
			}
			return nil
		},
	}
}

// airGapMessage descreve o estado do air-gap em linguagem natural.
func airGapMessage(bwrap bool, jail bool, netReach bool) string {
	if bwrap && jail && !netReach {
		return "Cofre em modo air-gap: sem egresso, guard ativo."
	}
	var parts []string
	if !bwrap {
		parts = append(parts, "bwrap indisponível (sem isolamento de namespace)")
	}
	if !jail {
		parts = append(parts, "processo fora da jaula")
	}
	if netReach {
		parts = append(parts, "rede acessível (air-gap rompido)")
	}
	if len(parts) == 0 {
		return "estado indeterminado"
	}
	return "air-gap NÃO garantido: " + strings.Join(parts, "; ")
}

// networkReachable faz um probe TCP best-effort, com timeout, para detectar se
// há rota de saída. Nunca envia payload — só o handshake de conexão. Puramente
// diagnóstico.
func networkReachable(timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", "1.1.1.1:53", timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func cofreYesNo(b bool) string {
	if b {
		return "sim"
	}
	return "não"
}

// compile-time guard: os comandos do cofre são construídos no padrão cobra.
var (
	_ *cobra.Command = NewCofreCommand()
	_ *cobra.Command = NewCofreValidateCommand()
	_ *cobra.Command = NewCofreGateCommand()
	_ *cobra.Command = NewCofreHealthCommand()
)
