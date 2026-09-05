package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/proposal"
)

// NewProposeCommand cria o comando `cosca propose` — a porta do Kernel
// isolado. O Kernel principal entrega uma PROPOSAL (contrato mínimo) e o
// Kernel isolado devolve o veredicto independente: APPROVE / DENY / REVIEW.
//
// Fluxo (desenhado pelo Don):
//
//	IA externa → UNTRUSTED OUTPUT → Evidence/Provenance → Kernel principal
//	→ PROPOSAL → Kernel isolado → Independent validation
//	→ APPROVE / DENY / REVIEW → Execution
func NewProposeCommand() *cobra.Command {
	var (
		action   string
		target   string
		motive   string
		origin   string
		state    string
		risk     string
		evidence string
		source   string
		timeout  time.Duration
		jsonOut  bool
		semantic bool
		model    string
		baseURL  string
	)

	cmd := &cobra.Command{
		Use:   "propose",
		Short: "Submeter uma proposta ao Kernel isolado (validação independente)",
		Long: `Submete uma proposta (contrato mínimo) ao Kernel isolado para
validação independente. O veredicto é APPROVE, DENY ou REVIEW.

Leis do fluxo:
  - Fail-closed: sem veredicto APPROVE, não existe execução
  - Ações destrutivas/estratégicas exigem o Don (guarda de papel)
  - Proposta incompleta volta em REVIEW com o motivo anexado
  - Evidência sem hash íntegro é rejeitada (proveniência quebrada)

Exemplo:
  cosca propose --action "criar docs/README.md" \
    --target "docs/README.md" --motive "documentar o kernel" \
    --origin "don" --risk normal --evidence "output da IA externa" \
    --source "ia-externa:big-pickle"`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			riskClass, err := proposal.ParseRiskClass(risk)
			if err != nil {
				return err
			}

			p := &proposal.Proposal{
				Action: action,
				Target: target,
				Motive: motive,
				Origin: origin,
				State:  state,
				Risk:   riskClass,
				Evidence: proposal.Provenance{
					Source:       source,
					ReceivedAt:   time.Now(),
					Evidence:     evidence,
					EvidenceHash: proposal.HashEvidence(evidence),
				},
			}

			flow := proposal.NewFlow(proposal.NewValidator(), timeout)
			// L2 semântica: o corpo da jaula (IA local) confirma a verdade
			// do pensamento. Se o Ollama estiver fora do ar, o fluxo nega
			// (fail-closed) — nunca abre exceção.
			if semantic {
				semanticValidator := proposal.NewOllamaValidator(baseURL, model, timeout)
				flow.Validator().SetSemantic(semanticValidator)
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			verdict := flow.Submit(ctx, p)

			if jsonOut {
				out, _ := json.MarshalIndent(verdict, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Kernel isolado → %s\n", verdict.String())
				if verdict.Approved() && verdict.NeedsDon {
					fmt.Fprintln(cmd.OutOrStdout(), "⚠ Recomendação aprovada — execução exige o Don (cosca approve / gate).")
				}
				if verdict.Verdict == proposal.VerdictReview {
					fmt.Fprintln(cmd.OutOrStdout(), "↩ Corrija a proposta apontada e re-submeta (limite de revisões: 2).")
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&action, "action", "", "o que exatamente será feito (obrigatório)")
	cmd.Flags().StringVar(&target, "target", "", "alvo da ação (arquivo, diretório, serviço)")
	cmd.Flags().StringVar(&motive, "motive", "", "porquê — objetivo da família (obrigatório)")
	cmd.Flags().StringVar(&origin, "origin", "", "de onde veio a intenção (obrigatório)")
	cmd.Flags().StringVar(&state, "state", "", "contexto atual relevante (backup/rollback para destrutivas)")
	cmd.Flags().StringVar(&risk, "risk", "normal", "classe de risco: trivial, normal, destructive, strategic")
	cmd.Flags().StringVar(&evidence, "evidence", "", "evidência bruta (UNTRUSTED OUTPUT) da IA externa")
	cmd.Flags().StringVar(&source, "source", "", "origem da evidência (ex.: ia-externa:big-pickle)")
	cmd.Flags().DurationVar(&timeout, "timeout", 3*time.Second, "tempo máximo para o Kernel isolado julgar (fail-closed)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "saída em JSON")
	cmd.Flags().BoolVar(&semantic, "semantic", false, "ativar o corpo da jaula: validação semântica via IA local (Ollama)")
	cmd.Flags().StringVar(&model, "model", "qwen2.5-coder:14b", "modelo local da jaula (validação semântica)")
	cmd.Flags().StringVar(&baseURL, "base-url", "http://127.0.0.1:11434", "servidor Ollama local da jaula")

	return cmd
}
