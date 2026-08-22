//
// `cosca knowledge claim` — classificação de afirmações ("How to think").
//
// Subcomandos:
//   add      --statement "..." [--kind auto|FACT|...] [--evidence N]
//            [--validated] [--derived] [--tested] [--assumed]
//            [--confidence 0-1] — classifica (ou recebe o tipo explícito),
//            persiste e devolve CL-XXXX + a razão da classificação
//   classify --statement "..." [flags] — mostra a classificação SEM gravar
//            (dry-run)
//   list     — tabela das afirmações (ID | Tipo | Afirmação | Confiança |
//            Confiável)
//   status <id> — detalhe de uma afirmação + veredito IsTrustworthy
//
// A classificação é DETERMINÍSTICA (sem LLM): FACT > EVIDENCE > INFERENCE >
// HYPOTHESIS > ASSUMPTION > UNKNOWN, com a assunção explícita (--assumed)
// sobrepondo tudo. O veredito "Confiável" (IsTrustworthy) diz se o General
// Context pode usar a afirmação como está ou se precisa sinalizá-la como
// suposição/desconhecida.
//
// Persistência: .cosca/claims.db (runtime, gitignored) — base SQLite
// dedicada, sem tocar em laws.json nem em knowledge.db.
//

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/knowledge"
)

// resolveClaimsPath devolve o caminho da base SQLite de claims para o
// diretório de trabalho atual: <projeto>/.cosca/claims.db.
func resolveClaimsPath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	return filepath.Join(dir, ".cosca", "claims.db"), nil
}

// NewKnowledgeClaimCommand cria o comando `cosca knowledge claim`.
func NewKnowledgeClaimCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claim",
		Short: "Classifica afirmações (FACT/EVIDENCE/INFERENCE/ASSUMPTION/HYPOTHESIS/UNKNOWN)",
		Long: `Classifica afirmações durante o raciocínio — o "How to think" do
General Context (Root Cause Analysis, Five Whys, Decision Trees, Evidence
Evaluation, Uncertainty).

Cada afirmação recebe um tipo determinístico (sem LLM): FACT, EVIDENCE,
INFERENCE, ASSUMPTION, HYPOTHESIS ou UNKNOWN — em vez de simplesmente
produzir uma resposta plausível.

Precedência: FACT > EVIDENCE > INFERENCE > HYPOTHESIS > ASSUMPTION > UNKNOWN.
A bandeira --assumed (assunção explícita) sobrepõe tudo.

Persistência: .cosca/claims.db (runtime, gitignored) — base SQLite dedicada.
`,
		Example: `  cosca knowledge claim add --statement "A API X rejeita tokens JWT" --evidence 2 --validated
  cosca knowledge claim classify --statement "A API X é lenta" --evidence 1
  cosca knowledge claim list
  cosca knowledge claim status CL-0001`,
	}

	cmd.AddCommand(
		NewKnowledgeClaimAddCommand(),
		NewKnowledgeClaimClassifyCommand(),
		NewKnowledgeClaimListCommand(),
		NewKnowledgeClaimStatusCommand(),
	)
	return cmd
}

// classifyClaim resolve o tipo de uma afirmação a partir dos sinais: --kind
// explícito (FACT/EVIDENCE/...) ou a classificação determinística (auto). Em
// ambos os casos devolve também a razão pt-BR da decisão.
func classifyClaim(kindFlag, statement string, evidence int, validated, derived, tested, assumed bool) (knowledge.ClaimKind, string) {
	if strings.TrimSpace(kindFlag) != "" && !strings.EqualFold(kindFlag, "auto") {
		k := knowledge.ClaimKind(strings.ToUpper(strings.TrimSpace(kindFlag)))
		return k, k.Description()
	}
	classifier := &knowledge.Classifier{ExplicitlyAssumed: assumed}
	return classifier.ClassifyWithReason(statement, evidence, validated, derived, tested)
}

// claimTrustworthy aplica a mesma regra de ClaimRecord.IsTrustworthy a um par
// (tipo, confiança) ainda não persistido.
func claimTrustworthy(kind knowledge.ClaimKind, confidence float64) bool {
	return (knowledge.ClaimRecord{Kind: kind, Confidence: confidence}).IsTrustworthy()
}

// trustYesNo formata o veredito de confiabilidade para a tabela/saída.
func trustYesNo(kind knowledge.ClaimKind, confidence float64) string {
	if claimTrustworthy(kind, confidence) {
		return "sim"
	}
	return "não"
}

// formatConfidence formata a confiança para exibição.
func formatConfidence(c float64) string {
	return fmt.Sprintf("%.2f", c)
}

// truncateStatement encurta a afirmação para a tabela (máx. 48 runas).
func truncateStatement(s string) string {
	const max = 48
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max]) + "…"
}

// NewKnowledgeClaimAddCommand cria o comando `cosca knowledge claim add`.
func NewKnowledgeClaimAddCommand() *cobra.Command {
	var (
		statement  string
		kindFlag   string
		evidence   int
		validated  bool
		derived    bool
		tested     bool
		assumed    bool
		confidence float64
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Registra uma afirmação classificada (CL-XXXX)",
		Long: `Registra uma afirmação com seu tipo de classificação e persiste em
.cosca/claims.db (CL-XXXX).

Sem --kind (default "auto"), a classificação é determinística (sem LLM) a
partir dos sinais:
  --evidence N     quantidade de evidências observadas
  --validated      as evidências foram validadas
  --derived        a afirmação foi construída a partir de outra afirmação
  --tested         foi testada, mas ainda não confirmada
  --assumed        assumida explicitamente (sobrepõe qualquer evidência)

Com --kind explícito (FACT/EVIDENCE/INFERENCE/ASSUMPTION/HYPOTHESIS/UNKNOWN)
os sinais são ignorados — o tipo é o informado.

Precedência automática: FACT > EVIDENCE > INFERENCE > HYPOTHESIS > ASSUMPTION
> UNKNOWN. Devolve o ID (CL-XXXX) + a razão da classificação.`,
		Example: `  cosca knowledge claim add --statement "O timeout padrão é 30s" --evidence 2 --validated
  cosca knowledge claim add --statement "A latência aumenta com o load" --evidence 3 --tested
  cosca knowledge claim add --statement "Presumo que o serviço está UP" --assumed
  cosca knowledge claim add --statement "X deriva de Y" --kind INFERENCE`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			if strings.TrimSpace(statement) == "" {
				return fmt.Errorf("--statement é obrigatório")
			}

			kind, reason := classifyClaim(kindFlag, statement, evidence, validated, derived, tested, assumed)
			if !kind.Valid() {
				return fmt.Errorf("--kind inválido: %q (use auto ou FACT/EVIDENCE/INFERENCE/ASSUMPTION/HYPOTHESIS/UNKNOWN)", kindFlag)
			}
			if confidence < 0 || confidence > 1 {
				return fmt.Errorf("--confidence %v fora do intervalo [0,1]", confidence)
			}

			path, err := resolveClaimsPath()
			if err != nil {
				return err
			}
			store, err := knowledge.NewClaimStore(path)
			if err != nil {
				return err
			}
			defer store.Close()

			id, err := store.Add(knowledge.ClaimRecord{
				Kind:       kind,
				Statement:  statement,
				Confidence: confidence,
			})
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"id":          id,
					"kind":        string(kind),
					"reason":      reason,
					"statement":   statement,
					"confidence":  confidence,
					"trustworthy": claimTrustworthy(kind, confidence),
				})
			}

			formatter.Success(fmt.Sprintf("Afirmação %s registrada como %s", id, kind))
			formatter.KeyValue("Classificação", string(kind))
			formatter.KeyValue("Razão", reason)
			formatter.KeyValue("Confiança", formatConfidence(confidence))
			formatter.KeyValue("Confiável", trustYesNo(kind, confidence))
			return nil
		},
	}

	cmd.Flags().StringVar(&statement, "statement", "", "a afirmação a classificar")
	cmd.Flags().StringVar(&kindFlag, "kind", "auto", "auto (determinístico) ou FACT/EVIDENCE/INFERENCE/ASSUMPTION/HYPOTHESIS/UNKNOWN")
	cmd.Flags().IntVar(&evidence, "evidence", 0, "quantidade de evidências observadas")
	cmd.Flags().BoolVar(&validated, "validated", false, "as evidências foram validadas")
	cmd.Flags().BoolVar(&derived, "derived", false, "afirmação derivada de outra afirmação")
	cmd.Flags().BoolVar(&tested, "tested", false, "afirmação testada, mas não confirmada")
	cmd.Flags().BoolVar(&assumed, "assumed", false, "assunção explícita (sobrepõe qualquer evidência)")
	cmd.Flags().Float64Var(&confidence, "confidence", 0, "confiança 0-1 (default 0)")
	_ = cmd.MarkFlagRequired("statement")
	return cmd
}

// NewKnowledgeClaimClassifyCommand cria o comando
// `cosca knowledge claim classify` (dry-run — nada é gravado).
func NewKnowledgeClaimClassifyCommand() *cobra.Command {
	var (
		statement  string
		kindFlag   string
		evidence   int
		validated  bool
		derived    bool
		tested     bool
		assumed    bool
		confidence float64
	)

	cmd := &cobra.Command{
		Use:   "classify",
		Short: "Mostra a classificação de uma afirmação SEM gravar (dry-run)",
		Long: `Classifica uma afirmação com as regras determinísticas (sem LLM) e
mostra o tipo + a razão, SEM persistir nada — um dry-run do "add".

Use os mesmos sinais do "add" (--evidence, --validated, --derived, --tested,
--assumed) ou um --kind explícito. Nada é gravado em disco.`,
		Example: `  cosca knowledge claim classify --statement "A API X é lenta" --evidence 1
  cosca knowledge claim classify --statement "X deriva de Y" --derived
  cosca knowledge claim classify --statement "Fato" --evidence 2 --validated`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			if strings.TrimSpace(statement) == "" {
				return fmt.Errorf("--statement é obrigatório")
			}

			kind, reason := classifyClaim(kindFlag, statement, evidence, validated, derived, tested, assumed)
			if !kind.Valid() {
				return fmt.Errorf("--kind inválido: %q (use auto ou FACT/EVIDENCE/INFERENCE/ASSUMPTION/HYPOTHESIS/UNKNOWN)", kindFlag)
			}

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"kind":        string(kind),
					"reason":      reason,
					"description": kind.Description(),
					"statement":   statement,
					"trustworthy": claimTrustworthy(kind, confidence),
					"dry_run":     true,
				})
			}

			formatter.Header("Classificação (dry-run — nada foi gravado)")
			formatter.KeyValue("Afirmação", statement)
			formatter.KeyValue("Classificação", string(kind))
			formatter.KeyValue("Razão", reason)
			formatter.KeyValue("Confiável", trustYesNo(kind, confidence))
			return nil
		},
	}

	cmd.Flags().StringVar(&statement, "statement", "", "a afirmação a classificar")
	cmd.Flags().StringVar(&kindFlag, "kind", "auto", "auto (determinístico) ou FACT/EVIDENCE/INFERENCE/ASSUMPTION/HYPOTHESIS/UNKNOWN")
	cmd.Flags().IntVar(&evidence, "evidence", 0, "quantidade de evidências observadas")
	cmd.Flags().BoolVar(&validated, "validated", false, "as evidências foram validadas")
	cmd.Flags().BoolVar(&derived, "derived", false, "afirmação derivada de outra afirmação")
	cmd.Flags().BoolVar(&tested, "tested", false, "afirmação testada, mas não confirmada")
	cmd.Flags().BoolVar(&assumed, "assumed", false, "assunção explícita (sobrepõe qualquer evidência)")
	cmd.Flags().Float64Var(&confidence, "confidence", 0, "confiança 0-1 (apenas para o veredito de confiabilidade)")
	_ = cmd.MarkFlagRequired("statement")
	return cmd
}

// NewKnowledgeClaimListCommand cria o comando `cosca knowledge claim list`.
func NewKnowledgeClaimListCommand() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lista as afirmações classificadas (tabela)",
		Long: `Lista as afirmações registradas numa tabela:
ID | Tipo | Afirmação | Confiança | Confiável.

A coluna "Confiável" usa IsTrustworthy: FACT é sempre confiável; EVIDENCE é
confiável com confiança >= 0.70; INFERENCE/ASSUMPTION/HYPOTHESIS/UNKNOWN
não são — o General Context deve sinalizá-las em vez de usá-las como fato.`,
		Example: `  cosca knowledge claim list
  cosca knowledge claim list --limit 50
  cosca knowledge claim list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			path, err := resolveClaimsPath()
			if err != nil {
				return err
			}
			store, err := knowledge.NewClaimStore(path)
			if err != nil {
				return err
			}
			defer store.Close()

			records, err := store.List(limit)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, records)
			}

			if len(records) == 0 {
				formatter.Warning("Nenhuma afirmação registrada ainda — use \"cosca knowledge claim add --statement ...\".")
				return nil
			}

			formatter.Header(fmt.Sprintf("Afirmações classificadas (%d)", len(records)))
			rows := make([][]string, 0, len(records))
			for _, c := range records {
				rows = append(rows, []string{
					c.ID,
					string(c.Kind),
					truncateStatement(c.Statement),
					formatConfidence(c.Confidence),
					trustYesNo(c.Kind, c.Confidence),
				})
			}
			formatter.Table([]string{"ID", "Tipo", "Afirmação", "Confiança", "Confiável"}, rows)
			return nil
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "l", 20, "máximo de afirmações a listar")
	return cmd
}

// NewKnowledgeClaimStatusCommand cria o comando
// `cosca knowledge claim status <id>`.
func NewKnowledgeClaimStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status <id>",
		Short: "Detalhe de uma afirmação + veredito de confiabilidade",
		Long: `Mostra o detalhe de uma afirmação (CL-XXXX): tipo, descrição,
afirmação, suportes, contradições, confiança, data e o veredito IsTrustworthy
— se o General Context pode usar a afirmação como está ou se precisa
sinalizá-la como suposição/desconhecida.`,
		Example: `  cosca knowledge claim status CL-0001
  cosca knowledge claim status cl-0001 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			path, err := resolveClaimsPath()
			if err != nil {
				return err
			}
			store, err := knowledge.NewClaimStore(path)
			if err != nil {
				return err
			}
			defer store.Close()

			rec, err := store.Get(args[0])
			if err != nil {
				return err
			}
			if rec == nil {
				return fmt.Errorf("afirmação %q não encontrada em %s", args[0], path)
			}

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"id":           rec.ID,
					"kind":         string(rec.Kind),
					"description":  rec.Kind.Description(),
					"statement":    rec.Statement,
					"supports":     rec.Supports,
					"contradicted": rec.ContradictedBy,
					"confidence":   rec.Confidence,
					"created_at":   rec.CreatedAt,
					"trustworthy":  rec.IsTrustworthy(),
				})
			}

			formatter.Header(fmt.Sprintf("Afirmação %s — %s", rec.ID, rec.Kind))
			formatter.KeyValue("Afirmação", rec.Statement)
			formatter.KeyValue("Tipo", string(rec.Kind))
			formatter.KeyValue("Descrição", rec.Kind.Description())
			if len(rec.Supports) > 0 {
				formatter.KeyValue("Suporta", strings.Join(rec.Supports, ", "))
			}
			if len(rec.ContradictedBy) > 0 {
				formatter.KeyValue("Contradita por", strings.Join(rec.ContradictedBy, ", "))
			}
			formatter.KeyValue("Confiança", formatConfidence(rec.Confidence))
			formatter.KeyValue("Criada em", rec.CreatedAt.Format("2006-01-02 15:04:05"))
			formatter.KeyValue("Confiável", trustYesNo(rec.Kind, rec.Confidence))
			return nil
		},
	}
}
