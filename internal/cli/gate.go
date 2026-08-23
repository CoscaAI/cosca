//
// `cosca gate` — Motor de transições Gate com guarda de papel.
//
// Aprovação de planos (Gate 0-4) como uma máquina de estados real: só o
// papel certo move um plano entre estados. A guarda de papel garante que só
// o approver (don/admin) aprova um plano e que o executor NÃO executa sem
// aprovação do Don.
//
//	plan → approving → approved → executed
//	          └→ rejected        └→ rolled-back
//
// Cada gate vive em .cosca/gate.db (SQLite): estado atual + histórico
// imutável (append-only) de transições.
//
// Subcomandos:
//   new --plan <ref>     Cria G-XXXX no estado plan
//   list                 Tabela dos gates existentes
//   status [id]          Estado atual (e última transição) do gate
//   move <id> --to <estado> --as <papel>   Valida a guarda e transiciona
//   ledger <id>          Histórico imutável completo de transições
//

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/gate"
)

// resolveGateStore abre a base do Gate do projeto atual
// (<projeto>/.cosca/gate.db).
func resolveGateStore() (*gate.GateStore, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getwd: %w", err)
	}
	return gate.NewGateStore(filepath.Join(dir, ".cosca", "gate.db"))
}

// NewGateCommand cria a árvore de comandos `cosca gate`.
func NewGateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gate",
		Short: "Motor de transições Gate com guarda de papel — aprovação de planos como máquina de estados",
		Long: `Motor de transições Gate com guarda de papel.

O fluxo de aprovação (Gate 0-4) é uma máquina de estados real: apenas o
papel certo move um plano entre estados.

  plan → approving → approved → executed
            └→ rejected        └→ rolled-back

Transições permitidas (TransitionTable):
  plan      → approving   : editor, admin, don
  approving → approved    : SOMENTE don/admin (o approver)
  approving → rejected    : don/admin
  approved  → executed    : SOMENTE don/admin (approver ≠ executor)
  executed  → rolled-back : don/admin

Cada gate vive em .cosca/gate.db (SQLite), com o estado atual e o histórico
imutável (append-only) de transições.

Subcomandos:
  new --plan <ref>            Cria G-XXXX no estado plan
  list                        Tabela dos gates existentes
  status [id]                 Estado atual (e última transição) do gate
  move <id> --to <estado> --as <papel>
                              Valida a guarda de papel e transiciona
  ledger <id>                 Histórico imutável completo de transições`,
		Example: `  cosca gate new --plan plano-deploy-v2
  cosca gate list
  cosca gate status G-0001
  cosca gate move G-0001 --to approving --as editor
  cosca gate move G-0001 --to approved --as don
  cosca gate ledger G-0001`,
	}

	cmd.AddCommand(
		NewGateNewCommand(),
		NewGateListCommand(),
		NewGateStatusCommand(),
		NewGateMoveCommand(),
		NewGateLedgerCommand(),
		NewGateCatalogCommand(),
	)
	return cmd
}

// NewGateNewCommand cria `cosca gate new --plan <ref>`.
func NewGateNewCommand() *cobra.Command {
	var planRef string

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Cria um gate G-XXXX para um plano (estado inicial: plan)",
		Long: `Cria um novo gate para o plano indicado em --plan: atribui um ID
G-XXXX sequencial e o registra no estado "plan" (Gate 0) em .cosca/gate.db.`,
		Example: `  cosca gate new --plan plano-deploy-v2`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			store, err := resolveGateStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			id, err := store.Create(planRef, gate.RoleDon)
			if err != nil {
				return err
			}
			rec, err := store.Get(id)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, rec)
			}
			formatter.Success(fmt.Sprintf("Gate %s criado — estado %q para o plano %q",
				rec.ID, rec.State, rec.PlanRef))
			formatter.KeyValue("ID", rec.ID)
			formatter.KeyValue("Plano", rec.PlanRef)
			formatter.KeyValue("Estado", string(rec.State))
			formatter.KeyValue("Criado em", rec.CreatedAt.Format(time.RFC3339))
			return nil
		},
	}

	cmd.Flags().StringVar(&planRef, "plan", "", "referência do plano (ex.: plano-deploy-v2)")
	_ = cmd.MarkFlagRequired("plan")
	return cmd
}

// NewGateListCommand cria `cosca gate list`.
func NewGateListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Lista os gates existentes (id, plano, estado, atualizado em)",
		Long: `Lista os gates registrados em .cosca/gate.db em uma tabela
(id, plano, estado, atualizado em). Apenas leitura.`,
		Example: `  cosca gate list
  cosca gate list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			store, err := resolveGateStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			records, err := store.List(1000)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, records)
			}
			if len(records) == 0 {
				formatter.Warning("Nenhum gate criado ainda — rode \"cosca gate new --plan <ref>\".")
				return nil
			}

			rows := make([][]string, 0, len(records))
			for _, r := range records {
				rows = append(rows, []string{
					r.ID,
					r.PlanRef,
					string(r.State),
					r.UpdatedAt.Format("2006-01-02 15:04:05"),
				})
			}
			formatter.Header(fmt.Sprintf("Gates — %d registro(s)", len(records)))
			formatter.Table([]string{"ID", "Plano", "Estado", "Atualizado em"}, rows)
			return nil
		},
	}
}

// NewGateStatusCommand cria `cosca gate status [id]`.
func NewGateStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status [id]",
		Short: "Mostra o estado atual (e última transição) de um gate",
		Long: `Mostra o estado atual e a última transição de um gate. Sem o
argumento id, lista todos os gates (como "cosca gate list").`,
		Example: `  cosca gate status
  cosca gate status G-0001
  cosca gate status G-0001 --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			store, err := resolveGateStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			if len(args) == 0 {
				records, err := store.List(1000)
				if err != nil {
					return err
				}
				if useJSON {
					return printJSON(cmd, records)
				}
				if len(records) == 0 {
					formatter.Warning("Nenhum gate criado ainda — rode \"cosca gate new --plan <ref>\".")
					return nil
				}
				rows := make([][]string, 0, len(records))
				for _, r := range records {
					rows = append(rows, []string{
						r.ID,
						r.PlanRef,
						string(r.State),
						r.UpdatedAt.Format("2006-01-02 15:04:05"),
					})
				}
				formatter.Header(fmt.Sprintf("Gates — %d registro(s)", len(records)))
				formatter.Table([]string{"ID", "Plano", "Estado", "Atualizado em"}, rows)
				return nil
			}

			norm, err := gate.NormalizeID(args[0])
			if err != nil {
				return err
			}
			rec, err := store.Get(norm)
			if err != nil {
				return err
			}
			if rec == nil {
				return fmt.Errorf("gate %s não encontrado em .cosca/gate.db", norm)
			}

			if useJSON {
				return printJSON(cmd, rec)
			}

			formatter.Header(fmt.Sprintf("GATE %s — %s", rec.ID, rec.State))
			formatter.KeyValue("Plano", rec.PlanRef)
			formatter.KeyValue("Estado", string(rec.State))
			formatter.KeyValue("Criado em", rec.CreatedAt.Format(time.RFC3339))
			formatter.KeyValue("Atualizado em", rec.UpdatedAt.Format(time.RFC3339))
			if n := len(rec.Transitions); n > 0 {
				last := rec.Transitions[n-1]
				formatter.KeyValue("Última transição",
					fmt.Sprintf("%s → %s (por %s)", last.From, last.To, last.By))
			} else {
				formatter.KeyValue("Última transição", "nenhuma ainda")
			}
			return nil
		},
	}
}

// NewGateMoveCommand cria `cosca gate move <id> --to <estado> --as <papel>`.
func NewGateMoveCommand() *cobra.Command {
	var (
		to string
		as string
	)

	cmd := &cobra.Command{
		Use:   "move <id>",
		Short: "Transiciona o gate validando a guarda de papel (--to <estado> --as <papel>)",
		Long: `Move um gate para o estado alvo validando a guarda de papel: a
transição só acontece se (a) existir na TransitionTable e (b) o papel
informado em --as estiver autorizado. Se negado, imprime o erro claro e o
estado permanece inalterado.

Transições permitidas:
  plan      → approving   : editor, admin, don
  approving → approved    : SOMENTE don/admin (o approver)
  approving → rejected    : don/admin
  approved  → executed    : SOMENTE don/admin (approver ≠ executor)
  executed  → rolled-back : don/admin`,
		Example: `  cosca gate move G-0001 --to approving --as editor
  cosca gate move G-0001 --to approved --as don`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			target, err := gate.ParseState(to)
			if err != nil {
				return err
			}

			store, err := resolveGateStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			norm, err := gate.NormalizeID(args[0])
			if err != nil {
				return err
			}
			before, err := store.Get(norm)
			if err != nil {
				return err
			}
			if before == nil {
				return fmt.Errorf("gate %s não encontrado em .cosca/gate.db", norm)
			}

			if err := store.Move(norm, target, as, as); err != nil {
				return err
			}
			rec, err := store.Get(norm)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, rec)
			}
			if before.State == target {
				formatter.Warning(fmt.Sprintf("Gate %s já está em %q — nenhuma transição registrada.", norm, target))
				return nil
			}
			formatter.Success(fmt.Sprintf("Gate %s movido %s → %s (por %s)",
				norm, before.State, rec.State, as))
			formatter.KeyValue("ID", norm)
			formatter.KeyValue("Estado", string(rec.State))
			formatter.KeyValue("Transições registradas", fmt.Sprintf("%d", len(rec.Transitions)))
			return nil
		},
	}

	cmd.Flags().StringVar(&to, "to", "", "estado alvo (plan, approving, approved, executed, rejected, rolled-back)")
	cmd.Flags().StringVar(&as, "as", "", "papel que executa a transição (ex.: don, admin, editor, specialist)")
	_ = cmd.MarkFlagRequired("to")
	_ = cmd.MarkFlagRequired("as")
	return cmd
}

// NewGateLedgerCommand cria `cosca gate ledger <id>`.
func NewGateLedgerCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "ledger <id>",
		Short: "Histórico imutável completo de transições do gate",
		Long: `Imprime o livro-razão (ledger) do gate: TODAS as transições em
ordem cronológica (append-only — nada aqui edita/apaga). Cada linha registra
de/para, quem executou, quando e a duração desde a transição anterior.`,
		Example: `  cosca gate ledger G-0001
  cosca gate ledger G-0001 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			store, err := resolveGateStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			norm, err := gate.NormalizeID(args[0])
			if err != nil {
				return err
			}
			ledger, err := store.Ledger(norm)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, ledger)
			}

			formatter.Header(fmt.Sprintf("LEDGER %s — %d transição(ões)", norm, len(ledger)))
			if len(ledger) == 0 {
				formatter.Warning("Nenhuma transição ainda — o gate nasceu em plan.")
				return nil
			}
			rows := make([][]string, 0, len(ledger))
			for _, t := range ledger {
				rows = append(rows, []string{
					string(t.From),
					string(t.To),
					t.By,
					t.At.Format("2006-01-02 15:04:05"),
					fmt.Sprintf("%ds", t.DurationSec),
				})
			}
			formatter.Table([]string{"De", "Para", "Por", "Quando", "Duração"}, rows)
			return nil
		},
	}
}
