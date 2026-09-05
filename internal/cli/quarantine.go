//
// `cosca quarantine` — Zona de quarentena epistemológica.
//
// Tudo o que a IA inventa (proposals) NÃO entra diretamente em
// knowledge/laws/memory. Passa primeiro pela quarentena:
//
//	proposal → quarantine → validation → evidence → promotion (ou archival)
//
// É o garbage collector epistemológico da Cosca: cada proposal vive em
// .cosca/quarantine/Q-XXXX.json e só é promovida após validação. O que não
// sobrevive é ARQUIVADO em .cosca/quarantine/archive/ — nunca deletado
// (princípio do curador).
//
// Subcomandos:
//   add <--title --content> [--source]   Cria uma proposal Q-XXXX
//   list                                  Tabela das proposals ativas
//   validate <id>                         pending → validating
//   promote <id> --to <alvo>              validating → promoted (+ PromotedTo)
//   discard <id>                          validação falhou → arquivado (nunca deleta)
//
// A promoção registrada aqui NÃO escreve em laws.json: ela é uma etapa
// manual/aprovada separada (o comando apenas registra o alvo PromotedTo).
//

package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/quarantine"
)

// resolveQuarantineStore devolve a zona de quarentena do diretório de
// trabalho atual (<projeto>/.cosca/quarantine).
func resolveQuarantineStore() (*quarantine.Store, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getwd: %w", err)
	}
	return quarantine.NewStore(dir), nil
}

// NewQuarantineCommand cria a árvore de comandos `cosca quarantine`.
func NewQuarantineCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quarantine",
		Short: "Zona de quarentena epistemológica — o que a IA inventa não entra direto no conhecimento",
		Long: `Zona de quarentena epistemológica da Cosca.

Tudo o que a IA inventa (proposals) passa primeiro pela quarentena antes de
entrar em knowledge/laws/memory:

  proposal → quarantine → validation → evidence → promotion (ou archival)

Cada proposal é um arquivo .cosca/quarantine/Q-XXXX.json. A validação
transiciona pending → validating; a promoção registra o alvo (PromotedTo);
o que não sobrevive é ARQUIVADO em .cosca/quarantine/archive/ — nunca
deletado (princípio do curador).

IMPORTANTE: a promoção registrada aqui NÃO grava em laws.json/knowledge —
promover ao conhecimento é uma etapa manual/aprovada separada.

Subcomandos:
  add <--title --content> [--source]   Cria uma proposal Q-XXXX
  list                                  Tabela das proposals ativas
  validate <id>                         pending → validating
  promote <id> --to <alvo>              validating → promoted (+ PromotedTo)
  discard <id>                          arquiva (nunca deleta)`,
		Example: `  cosca quarantine add --title "Cache de embeddings" --content "Propor LRU no indexer" --source agent-x
  cosca quarantine list
  cosca quarantine validate Q-0001
  cosca quarantine promote Q-0001 --to K-06
  cosca quarantine discard Q-0001`,
	}

	cmd.AddCommand(
		NewQuarantineAddCommand(),
		NewQuarantineListCommand(),
		NewQuarantineValidateCommand(),
		NewQuarantinePromoteCommand(),
		NewQuarantineDiscardCommand(),
	)
	return cmd
}

// NewQuarantineAddCommand cria `cosca quarantine add`.
func NewQuarantineAddCommand() *cobra.Command {
	var (
		title   string
		content string
		source  string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Cria uma proposal na zona de quarentena",
		Long: `Registra uma invenção da IA na zona de quarentena: grava
.cosca/quarantine/Q-XXXX.json com status "pending". A proposal NÃO entra em
knowledge/laws/memory — só após validation → promotion.`,
		Example: `  cosca quarantine add --title "Cache LRU" --content "Propor LRU no indexer" --source agent-x`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			store, err := resolveQuarantineStore()
			if err != nil {
				return err
			}
			id, err := store.Add(quarantine.Proposal{
				Title:   title,
				Content: content,
				Source:  source,
			})
			if err != nil {
				return err
			}
			prop, err := store.Get(id)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, prop)
			}
			formatter.Success(fmt.Sprintf("Proposal %s criada na quarentena (status %q)", prop.ID, prop.Status))
			formatter.KeyValue("Título", prop.Title)
			formatter.KeyValue("Fonte", prop.Source)
			formatter.KeyValue("Arquivo", store.Path())
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "título da proposal")
	cmd.Flags().StringVar(&content, "content", "", "texto da proposal da IA")
	cmd.Flags().StringVar(&source, "source", "", "provider/agente que propôs")
	_ = cmd.MarkFlagRequired("title")
	_ = cmd.MarkFlagRequired("content")
	return cmd
}

// NewQuarantineListCommand cria `cosca quarantine list`.
func NewQuarantineListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Lista as proposals ativas na quarentena",
		Long: `Lista as proposals ativas (fora do archive) em uma tabela
(id, status, título, criada em). Proposals arquivadas não aparecem — use
"cosca quarantine show" (futuro) para consultar o archive.`,
		Example: `  cosca quarantine list
  cosca quarantine list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			store, err := resolveQuarantineStore()
			if err != nil {
				return err
			}
			props, err := store.List()
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, props)
			}
			if len(props) == 0 {
				formatter.Warning("Quarentena vazia — nada inventado ainda.")
				return nil
			}

			formatter.Header(fmt.Sprintf("Zona de quarentena — %d proposal(s) ativa(s)", len(props)))
			rows := make([][]string, 0, len(props))
			for _, p := range props {
				rows = append(rows, []string{
					p.ID,
					p.Status,
					p.Title,
					p.CreatedAt.Format("2006-01-02 15:04:05"),
				})
			}
			formatter.Table([]string{"ID", "Status", "Título", "Criada em"}, rows)
			return nil
		},
	}
}

// NewQuarantineValidateCommand cria `cosca quarantine validate <id>`.
func NewQuarantineValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <id>",
		Short: "Transiciona a proposal para validating (pending → validating)",
		Long: `Marca a proposal como em validação (status "validating").
A validação é o passo anterior obrigatório à promoção.`,
		Example: `  cosca quarantine validate Q-0001`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			store, err := resolveQuarantineStore()
			if err != nil {
				return err
			}
			norm, err := quarantine.NormalizeID(args[0])
			if err != nil {
				return err
			}
			if err := store.SetStatus(norm, quarantine.StatusValidating); err != nil {
				return err
			}
			prop, err := store.Get(norm)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, prop)
			}
			formatter.Success(fmt.Sprintf("Proposal %s em validação — status %q", prop.ID, prop.Status))
			return nil
		},
	}
}

// NewQuarantinePromoteCommand cria `cosca quarantine promote <id> --to <alvo>`.
func NewQuarantinePromoteCommand() *cobra.Command {
	var to string

	cmd := &cobra.Command{
		Use:   "promote <id>",
		Short: "Promove a proposal validada, registrando o alvo (validating → promoted)",
		Long: `Promove a proposal (status "promoted") e registra o alvo da promoção
em PromotedTo (ex.: --to K-06). A proposal deve estar em validating.

IMPORTANTE: isto NÃO escreve em laws.json/knowledge — a promoção ao
conhecimento é uma etapa manual/aprovada separada. Este comando apenas
registra a intenção no arquivo da quarentena.`,
		Example: `  cosca quarantine promote Q-0001 --to K-06`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			store, err := resolveQuarantineStore()
			if err != nil {
				return err
			}
			norm, err := quarantine.NormalizeID(args[0])
			if err != nil {
				return err
			}
			if err := store.Promote(norm, to); err != nil {
				return err
			}
			prop, err := store.Get(norm)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, prop)
			}
			formatter.Success(fmt.Sprintf(
				"Proposal %s promovida → %q (status %q). Promoção ao conhecimento é etapa manual/aprovada separada.",
				prop.ID, prop.PromotedTo, prop.Status))
			return nil
		},
	}

	cmd.Flags().StringVar(&to, "to", "", "alvo da promoção (ex.: K-06 ou learning:xyz)")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

// NewQuarantineDiscardCommand cria `cosca quarantine discard <id>`.
func NewQuarantineDiscardCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "discard <id>",
		Short: "Arquiva a proposal que não sobreviveu à validação (nunca deleta)",
		Long: `Arquiva a proposal: o status vira "discarded" e o arquivo é movido
para .cosca/quarantine/archive/. A quarentena NUNCA deleta — só arquiva
(princípio do curador: "nunca deleta, só arquiva").`,
		Example: `  cosca quarantine discard Q-0001`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			store, err := resolveQuarantineStore()
			if err != nil {
				return err
			}
			norm, err := quarantine.NormalizeID(args[0])
			if err != nil {
				return err
			}
			if err := store.Discard(norm); err != nil {
				return err
			}
			prop, err := store.Get(norm)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, prop)
			}
			formatter.Success(fmt.Sprintf(
				"Proposal %s descartada e arquivada em %s (status %q)",
				prop.ID, store.ArchivePath(), prop.Status))
			return nil
		},
	}
}
