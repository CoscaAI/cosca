//
// `cosca intelligence` — o Intelligence Engine (ADR-047) exposto ao Don.
//
// Roda o motor determinístico sobre o conhecimento REAL da memória curada,
// SEMPRE em shadow-first (G3): SÓ observa, sinaliza, propõe — NUNCA aplica.
// Toda aplicação passaria pelo freio (guardrails G1-G9) e pelo gate do Don.
//
// Subcomandos:
//   plan        Curriculum — o que o engine decidiria estudar (shadow, leitura)
//   conflicts   Detecta conflito entre conhecimento (R6, só sinaliza)
//   gate        Testa uma proposta de edição contra o freio (demo G1-G9)
//
// Regra do Don: o engine valida com CPU (determinístico), não chuta com LLM.
//

package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/guardrails"
	"github.com/CoscaAI/cosca/internal/intelligence"
)

// resolveMemoryDir retorna o diretório da memória curada do projeto.
func resolveMemoryDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	return filepath.Join(dir, ".cosca", "memory"), nil
}

// NewIntelligenceCommand cria a árvore de comandos `cosca intelligence`.
func NewIntelligenceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "intelligence",
		Short: "Intelligence Engine (ADR-047) — motor determinístico do conhecimento, em shadow-first",
		Long: `Intelligence Engine (ADR-047) — o motor que governa o conhecimento da casa.

O motor é DETERMINÍSTICO: ele decide o que estudar (curriculum), detecta
conflito de conhecimento (R6) e propõe substituição/promoção — SEMPRE em
SHADOW-FIRST (G3): só observa e sinaliza, NUNCA aplica. Toda aplicação passaria
pelo freio (guardrails G1-G9) e pelo gate do Don.

Subcomandos:
  plan           Curriculum — o que o engine estudaria (leitura)
  conflicts      Conflitos detectados entre conhecimento (R6, só sinaliza)
  gate           Testa uma proposta contra o freio (demo G1-G9)`,
		Example: `  cosca intelligence plan
  cosca intelligence conflicts
  cosca intelligence gate --resource memory/agent/kernel/learnings.md --evidence 5`,
	}

	cmd.AddCommand(
		NewIntelligencePlanCommand(),
		NewIntelligenceConflictsCommand(),
		NewIntelligenceDuplicatesCommand(),
		NewIntelligenceGateCommand(),
	)
	return cmd
}

// NewIntelligencePlanCommand cria `cosca intelligence plan`.
func NewIntelligencePlanCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "plan",
		Short: "Curriculum — o que o engine decidiria estudar (shadow, leitura)",
		Long: `Lê os learnings.md da memória curada e gera o curriculum: a ordem
que o motor determinístico decidiria estudar, priorizada por evidência +
confiança + recência. Shadow-first: apenas observa, não aplica nada.`,
		Example: `  cosca intelligence plan
  cosca intelligence plan --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			memDir, err := resolveMemoryDir()
			if err != nil {
				return err
			}

			eng := intelligence.New(guardrails.DefaultDeps(), intelligence.MemoryProvider(memDir))
			plan, err := eng.Plan(cmd.Context())
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, plan)
			}

			formatter.Header(fmt.Sprintf("Curriculum — %d item(s) (shadow-first, leitura)", len(plan.Items)))
			if len(plan.Items) == 0 {
				formatter.Warning("Nenhum conhecimento na memória curada para planejar.")
				return nil
			}
			rows := make([][]string, 0, len(plan.Items))
			for _, it := range plan.Items {
				rows = append(rows, []string{
					fmt.Sprintf("%.2f", it.Priority),
					it.SourceID,
					it.Reason,
				})
			}
			formatter.Table([]string{"Prioridade", "Fonte", "Razão (evidência/confiança/recência)"}, rows)
			formatter.KeyValue("Modo", "shadow-first (G3) — nada foi aplicado")
			return nil
		},
	}
}

// NewIntelligenceConflictsCommand cria `cosca intelligence conflicts`.
func NewIntelligenceConflictsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "conflicts",
		Short: "Conflitos detectados entre conhecimento (R6, só sinaliza)",
		Long: `Detecta conhecimento novo que contradiz o antigo (mesmo tópico,
conclusão divergente, evidência melhor). R6 só SINALIZA — quem decide quem
"vence" é o Don (G5), nunca o engine.`,
		Example: `  cosca intelligence conflicts
  cosca intelligence conflicts --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			memDir, err := resolveMemoryDir()
			if err != nil {
				return err
			}

			eng := intelligence.New(guardrails.DefaultDeps(), intelligence.MemoryProvider(memDir))
			srcs, err := eng.Sources(cmd.Context())
			if err != nil {
				return err
			}
			conflicts := eng.DetectConflicts(srcs, srcs, 0.4)

			if useJSON {
				return printJSON(cmd, conflicts)
			}

			formatter.Header(fmt.Sprintf("Conflitos detectados — %d (R6: só sinaliza, não resolve)", len(conflicts)))
			if len(conflicts) == 0 {
				formatter.Success("Nenhum conflito detectado entre o conhecimento curado.")
				return nil
			}
			rows := make([][]string, 0, len(conflicts))
			for _, c := range conflicts {
				rows = append(rows, []string{
					c.OldResource,
					c.NewResource,
					fmt.Sprintf("%.2f", c.Similarity),
					c.Note,
				})
			}
			formatter.Table([]string{"Conhecimento atual", "Novo", "Similaridade", "O que o Don decide"}, rows)
			return nil
		},
	}
}

// NewIntelligenceDuplicatesCommand cria `cosca intelligence duplicates` — o que
// condensar (R2): aprendees duplicados (mesmo aprendizado gravado N×) que
// deveriam virar 1. Shadow-first: só lista, nunca edita.
func NewIntelligenceDuplicatesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "duplicates",
		Short: "Duplicatas para condensar (R2) — mesmo aprendizado gravado N×",
		Long: `Detecta aprendizados duplicados (mesmo aprendizado gravado várias vezes
com hashes diferentes — ex.: 'Post-Commit Hook Execution' 34× no cosca-devops).
São candidatos à condensação (R2): virar 1 entrada. Shadow-first: só lista,
nunca edita — a aplicação da condensação é decisão sua (gate do Don).`,
		Example: `  cosca intelligence duplicates
  cosca intelligence duplicates --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			memDir, err := resolveMemoryDir()
			if err != nil {
				return err
			}
			eng := intelligence.New(guardrails.DefaultDeps(), intelligence.MemoryProvider(memDir))
			srcs, err := eng.Sources(cmd.Context())
			if err != nil {
				return err
			}
			dups := eng.DetectDuplicates(srcs, srcs)

			if useJSON {
				return printJSON(cmd, dups)
			}

			formatter.Header(fmt.Sprintf("Duplicatas para condensar (R2) — %d par(es)", len(dups)))
			if len(dups) == 0 {
				formatter.Success("Nenhuma duplicata detectada.")
				return nil
			}
			rows := make([][]string, 0, len(dups))
			for _, d := range dups {
				rows = append(rows, []string{
					fmt.Sprintf("%.2f", d.Similarity),
					d.SourceA,
					d.SourceB,
				})
			}
			formatter.Table([]string{"Similaridade", "Fonte A", "Fonte B"}, rows)
			formatter.KeyValue("Modo", "shadow-first (G3) — nada foi condensado/alterado")
			return nil
		},
	}
}

// NewIntelligenceGateCommand cria `cosca intelligence gate` — demo do freio.
func NewIntelligenceGateCommand() *cobra.Command {
	var (
		resource    string
		evidence    int
		donApproved bool
		shadow      bool
		content     string
	)

	cmd := &cobra.Command{
		Use:   "gate",
		Short: "Testa uma proposta de edição contra o freio (demo G1-G9)",
		Long: `Submete uma proposta de edição de conhecimento ao Contrato de
Salvaguarda (G1-G9) e devolve o veredito do freio. É a demonstração de que o
engine NÃO edita sozinho: sem aprovação do Don (G2), sem evidência (G6), sem
snapshot (G4) ou sobre recurso imutável (G1) → o freio NEGA.`,
		Example: `  cosca intelligence gate --resource memory/agent/kernel/learnings.md --don --evidence 5 --shadow
   cosca intelligence gate --resource ads/ADR-001 ... --evidence 5`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			proposal := guardrails.Proposal{
				ID:            "cli-" + resource,
				Resource:      resource,
				Role:          guardrails.RoleProposer,
				NewContent:    content,
				EvidenceLevel: evidence,
				ApprovedByDon: donApproved,
				ShadowMode:    shadow,
				HasSnapshot:   true, // o gate demo sempre assume snapshot prévio
			}
			res := guardrails.Evaluate(proposal, guardrails.DefaultDeps())

			if useJSON {
				return printJSON(cmd, res)
			}
			if res.Verdict.Approved {
				formatter.Success("PROPOSTA APROVADA pelo freio G1-G9 — pode aplicar (com snapshot).")
			} else {
				formatter.Warning("PROPOSTA NEGADA pelo freio G1-G9:")
			}
			for _, r := range res.Verdict.Reasons {
				formatter.KeyValue("Motivo", r)
			}
			formatter.KeyValue("Regras avaliadas", fmt.Sprintf("%d (G1-G9)", len(res.RulesChecked)))
			return nil
		},
	}

	cmd.Flags().StringVar(&resource, "resource", "", "recurso a editar (ex: memory/agent/kernel/learnings.md)")
	cmd.Flags().IntVar(&evidence, "evidence", 5, "nível de evidência (0-5; regra: só aplica >= 4)")
	cmd.Flags().BoolVar(&donApproved, "don", false, "proposta aprovada pelo Don (G2)")
	cmd.Flags().BoolVar(&shadow, "shadow", true, "rodar em shadow-first (G3: não aplica)")
	cmd.Flags().StringVar(&content, "content", "", "novo conteúdo da proposta (para validação do guard)")
	return cmd
}
