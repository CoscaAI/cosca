//
// `cosca conflict` — Contradição como entidade primeira classe (CONFLICT-XXX).
//
// Quando duas fontes divergem sobre o mesmo item ("A: X funciona dessa
// maneira." / "B: X não funciona dessa maneira."), o Cosca NÃO escolhe uma:
// ele registra o conflito e se recusa a promover o item para lei enquanto o
// conflito estiver aberto.
//
//	Fonte A ──┐
//	          ├── CONFLICT ──┘
//	Fonte B ──┘
//
// Cada conflito vive em .cosca/conflict.db (SQLite, base dedicada — não toca
// no schema de conhecimento). A convenção das alegações é "item:evidence"
// (ex.: "K-27:E-101").
//
// Subcomandos:
//   new --item <id> --claim-a <ref> --claim-b <ref> [--desc <texto>]
//       Registra CONFLICT-XXX em aberto para o item
//   list [--open|--resolved]   Tabela dos conflitos (por padrão: abertos)
//   show <id>                  Detalhe completo de um conflito
//   resolve <id>               Marca o conflito como resolvido
//
// NOTE: a elevação do estado epistemológico para CONFLICTING é uma integração
// OPT-IN via StatusWithConflicts — o CLI registra/consulta aqui; os chamadores
// (runtime/servidor) decidem quando aplicar o status.
//

package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/knowledge"
)

// resolveConflictStore abre a base de conflitos do projeto atual
// (<projeto>/.cosca/conflict.db).
func resolveConflictStore() (*knowledge.ConflictStore, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getwd: %w", err)
	}
	return knowledge.NewConflictStore(filepath.Join(dir, ".cosca", "conflict.db"))
}

// NewConflictCommand cria a árvore de comandos `cosca conflict`.
func NewConflictCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conflict",
		Short: "Contradição como entidade primeira classe — registro de conflitos entre fontes (CONFLICT-XXX)",
		Long: `Contradição como entidade primeira classe.

Quando duas fontes divergem sobre o mesmo item de conhecimento, o Cosca NÃO
escolhe uma das fontes (A + B → IA escolhe uma). Ele registra o conflito e se
recusa a promover o item para lei enquanto o conflito estiver aberto:

  Fonte A ──┐
            ├── CONFLICT ──┘
  Fonte B ──┘

Assim o Cosca pode responder honestamente: "Existem duas fontes conflitantes.
Não vou promover isso para uma lei."

Cada conflito vive em .cosca/conflict.db (SQLite, base dedicada). A convenção
das alegações é "item:evidence" (ex.: "K-27:E-101").

Subcomandos:
  new --item <id> --claim-a <ref> --claim-b <ref> [--desc <texto>]
                      Registra CONFLICT-XXX em aberto
  list [--open|--resolved]
                      Lista os conflitos (por padrão: apenas os abertos)
  show <id>           Detalhe completo de um conflito
  resolve <id>        Marca o conflito como resolvido`,
		Example: `  cosca conflict new --item K-27 --claim-a "K-27:E-101" --claim-b "K-27:E-203" --desc "fonte A diz X, fonte B diz o contrário"
  cosca conflict list
  cosca conflict list --resolved
  cosca conflict show CONFLICT-001
  cosca conflict resolve CONFLICT-001`,
	}

	cmd.AddCommand(
		NewConflictNewCommand(),
		NewConflictListCommand(),
		NewConflictShowCommand(),
		NewConflictResolveCommand(),
	)
	return cmd
}

// NewConflictNewCommand cria `cosca conflict new --item <id> --claim-a <ref>
// --claim-b <ref> [--desc <texto>]`.
func NewConflictNewCommand() *cobra.Command {
	var (
		itemID string
		claimA string
		claimB string
		desc   string
	)

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Registra um conflito CONFLICT-XXX em aberto entre duas alegações",
		Long: `Registra uma contradição entre duas fontes sobre o mesmo item de
conhecimento: atribui um ID CONFLICT-XXX sequencial e o persiste em
.cosc/conflict.db no estado "open". As alegações seguem a convenção
"item:evidence" (ex.: "K-27:E-101").`,
		Example: `  cosca conflict new --item K-27 --claim-a "K-27:E-101" --claim-b "K-27:E-203" --desc "fonte A diz X, fonte B diz o contrário"`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			store, err := resolveConflictStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			id, err := store.Add(knowledge.ConflictRecord{
				ItemID:      itemID,
				ClaimA:      claimA,
				ClaimB:      claimB,
				Description: desc,
			})
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
			formatter.Success(fmt.Sprintf("Conflito %s registrado — %s (aberto)", rec.ID, rec.ItemID))
			formatter.KeyValue("ID", rec.ID)
			formatter.KeyValue("Item afetado", rec.ItemID)
			formatter.KeyValue("Fonte A", rec.ClaimA)
			formatter.KeyValue("Fonte B", rec.ClaimB)
			formatter.KeyValue("Descrição", rec.Description)
			formatter.KeyValue("Detectado em", rec.DetectedAt.Format("2006-01-02 15:04:05"))
			return nil
		},
	}

	cmd.Flags().StringVar(&itemID, "item", "", "ID do item de conhecimento afetado (ex.: K-27)")
	cmd.Flags().StringVar(&claimA, "claim-a", "", "alegação da fonte A, no formato item:evidence (ex.: K-27:E-101)")
	cmd.Flags().StringVar(&claimB, "claim-b", "", "alegação da fonte B, no formato item:evidence (ex.: K-27:E-203)")
	cmd.Flags().StringVar(&desc, "desc", "", "descrição da contradição (ex.: fonte A diz X, fonte B diz o contrário)")
	_ = cmd.MarkFlagRequired("item")
	_ = cmd.MarkFlagRequired("claim-a")
	_ = cmd.MarkFlagRequired("claim-b")
	return cmd
}

// NewConflictListCommand cria `cosca conflict list [--open|--resolved]`.
func NewConflictListCommand() *cobra.Command {
	var onlyResolved bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lista os conflitos registrados (por padrão: apenas os abertos)",
		Long: `Lista os conflitos em uma tabela (ID, item, alegações, status,
detectado em). Por padrão mostra apenas os conflitos abertos (a regra do Don:
conflito aberto = não promover). Use --resolved para ver os resolvidos ou
informe ambos para ver tudo.`,
		Example: `  cosca conflict list
  cosca conflict list --resolved
  cosca conflict list --open
  cosca conflict list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			store, err := resolveConflictStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			var status []knowledge.ConflictStatus
			switch {
			case onlyResolved:
				status = []knowledge.ConflictStatus{knowledge.ConflictResolved}
			default:
				// Padrão honesto: só os abertos bloqueiam promoção.
				status = []knowledge.ConflictStatus{knowledge.ConflictOpen}
			}

			records, err := store.List(status...)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, records)
			}
			if len(records) == 0 {
				formatter.Warning("Nenhum conflito encontrado — rode \"cosca conflict new --item K-01 --claim-a ... --claim-b ...\".")
				return nil
			}

			rows := make([][]string, 0, len(records))
			for _, r := range records {
				rows = append(rows, []string{
					r.ID,
					r.ItemID,
					r.ClaimA,
					r.ClaimB,
					string(r.Status),
					r.DetectedAt.Format("2006-01-02 15:04:05"),
				})
			}
			label := "abertos"
			if onlyResolved {
				label = "resolvidos"
			}
			formatter.Header(fmt.Sprintf("Conflitos %s — %d registro(s)", label, len(records)))
			formatter.Table([]string{"ID", "Item", "Fonte A", "Fonte B", "Status", "Detectado em"}, rows)
			return nil
		},
	}

	cmd.Flags().BoolVar(&onlyResolved, "resolved", false, "listar apenas conflitos resolvidos")
	cmd.Flags().Bool("open", false, "listar apenas conflitos abertos (padrão)")
	return cmd
}

// NewConflictShowCommand cria `cosca conflict show <id>`.
func NewConflictShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Mostra o detalhe completo de um conflito",
		Long: `Imprime o detalhe completo de um conflito CONFLICT-XXX: alegações
das duas fontes, item afetado, descrição, quando foi detectado e o status
(open/resolved). Apenas leitura.`,
		Example: `  cosca conflict show CONFLICT-001
  cosca conflict show CONFLICT-001 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			store, err := resolveConflictStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			norm, err := knowledge.NormalizeConflictID(args[0])
			if err != nil {
				return err
			}
			rec, err := store.Get(norm)
			if err != nil {
				return err
			}
			if rec == nil {
				return fmt.Errorf("conflito %s não encontrado em .cosca/conflict.db", norm)
			}

			if useJSON {
				return printJSON(cmd, rec)
			}

			formatter.Header(fmt.Sprintf("CONFLICT %s — %s", rec.ID, rec.Status))
			formatter.KeyValue("Item afetado", rec.ItemID)
			formatter.KeyValue("Fonte A", rec.ClaimA)
			formatter.KeyValue("Fonte B", rec.ClaimB)
			formatter.KeyValue("Descrição", rec.Description)
			formatter.KeyValue("Status", string(rec.Status))
			formatter.KeyValue("Detectado em", rec.DetectedAt.Format("2006-01-02 15:04:05"))
			if !rec.ResolvedAt.IsZero() {
				formatter.KeyValue("Resolvido em", rec.ResolvedAt.Format("2006-01-02 15:04:05"))
			} else {
				formatter.KeyValue("Resolvido em", "—")
			}
			return nil
		},
	}
}

// NewConflictResolveCommand cria `cosca conflict resolve <id>`.
func NewConflictResolveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "resolve <id>",
		Short: "Marca um conflito como resolvido (status=resolved + ResolvedAt)",
		Long: `Resolve um conflito CONFLICT-XXX: define o status como resolved e
grava ResolvedAt. Com o conflito resolvido, o item deixa de ser bloqueado
para promoção por este conflito. Um conflito já resolvido é um no-op.`,
		Example: `  cosca conflict resolve CONFLICT-001`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			store, err := resolveConflictStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			norm, err := knowledge.NormalizeConflictID(args[0])
			if err != nil {
				return err
			}
			before, err := store.Get(norm)
			if err != nil {
				return err
			}
			if before == nil {
				return fmt.Errorf("conflito %s não encontrado em .cosca/conflict.db", norm)
			}

			if err := store.Resolve(norm); err != nil {
				return err
			}
			rec, err := store.Get(norm)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, rec)
			}
			if before.Status == knowledge.ConflictResolved {
				formatter.Warning(fmt.Sprintf("Conflito %s já estava resolvido — nenhuma alteração.", norm))
				return nil
			}
			formatter.Success(fmt.Sprintf("Conflito %s resolvido — o item %s não está mais bloqueado por ele.", norm, rec.ItemID))
			formatter.KeyValue("ID", norm)
			formatter.KeyValue("Status", string(rec.Status))
			formatter.KeyValue("Resolvido em", rec.ResolvedAt.Format("2006-01-02 15:04:05"))
			return nil
		},
	}
}
