//
// `cosca knowledge evidence` — procedência das evidências do CKL (P0-P5).
//
// Subcomandos:
//   add --item <id> --id <evid> --kind --source --desc
//       [--provenance P0..P5] [--repo --commit --path --sha256] —
//       adiciona evidência com procedência e referência de fonte auditável
//       (git commit é ouro: repository, commit, path, sha256, retrieved)
//   list <item> — tabela das evidências com procedência + commit + sha256
//   repro <item> — mostra quais evidências são totalmente reproduzíveis
//                  (P4/P5 com repository + commit + sha256)
//
// Procedência (da conversa do Don): P0—desconhecida, P1—fonte externa não
// verificada, P2—repositório identificável, P3—fonte oficial,
// P4—código/teste reproduzível, P5—múltiplas fontes independentes. Para P4
// e P5, --commit e --sha256 são obrigatórios: sem eles é impossível
// reproduzir "foi desse código, exatamente nessa versão, que essa evidência
// veio". O item deve existir (use `cosca knowledge law add-evidence` para
// criar). Persistência: .cosca/knowledge/laws.json (runtime, gitignored).
//
// Reutiliza o padrão do `law`: resolveLawsPath + loadLawsEngine + o seed
// idempotente da primeira execução (ensureSeededLaws).
//

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/knowledge"
)

// NewKnowledgeEvidenceCommand cria o comando `cosca knowledge evidence`.
func NewKnowledgeEvidenceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "evidence",
		Short: "Procedência das evidências (P0-P5) e reprodutibilidade",
		Long: `Gerencia a procedência e a reprodutibilidade das evidências do
Cosca Knowledge Lifecycle (CKL).

Cada evidência tem uma procedência (P0-P5), o nível de confiança na origem:
P0—desconhecida, P1—fonte externa não verificada, P2—repositório
identificável, P3—fonte oficial, P4—código/teste reproduzível e
P5—múltiplas fontes independentes.

Git commit é ouro: além da procedência, uma evidência pode carregar a
referência de fonte auditável — repository, commit, path, sha256 e
retrieved — permitindo reproduzir "foi desse código, exatamente nessa
versão, que essa evidência veio".

Persistência: .cosca/knowledge/laws.json (runtime, gitignored). Evidências
antigas sem procedência migram automaticamente para P0 no load.
`,
		Example: `  cosca knowledge evidence add --item K-01 --id E-401 --kind test --source "github.com/CoscaAI/cosca" --desc "teste reproduzível" --provenance P4 --repo CoscaAI/cosca --commit 7a91c2 --path internal/runtime/foo.go --sha256 abc123...
  cosca knowledge evidence list K-01
  cosca knowledge evidence repro K-01`,
	}

	cmd.AddCommand(
		NewKnowledgeEvidenceAddCommand(),
		NewKnowledgeEvidenceListCommand(),
		NewKnowledgeEvidenceReproCommand(),
	)
	return cmd
}

// NewKnowledgeEvidenceAddCommand cria o comando
// `cosca knowledge evidence add`.
func NewKnowledgeEvidenceAddCommand() *cobra.Command {
	var (
		item         string
		evID         string
		kind         string
		source       string
		desc         string
		provenance   string
		repo         string
		commit       string
		evidencePath string
		sha256       string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Adiciona evidência com procedência (P0-P5) e referência de fonte",
		Long: `Adiciona uma evidência a um item existente do CKL com procedência
(P0-P5) e, quando aplicável, a referência de fonte auditável.

A procedência diz de onde a evidência veio (P0—desconhecida a
P5—múltiplas fontes independentes). Para P4 (código/teste reproduzível) e
P5, --commit e --sha256 são obrigatórios: sem eles não é possível
reproduzir a evidência daquela versão exata do código.

Sem --provenance, a evidência nasce como P0 (desconhecida). O item deve
existir — use "cosca knowledge law add-evidence <id>" para criá-lo.`,
		Example: `  cosca knowledge evidence add --item K-01 --id E-401 --kind test --source "github.com/CoscaAI/cosca" --desc "teste reproduzível" --provenance P4 --repo CoscaAI/cosca --commit 7a91c2 --path internal/runtime/foo.go --sha256 abc123...
  cosca knowledge evidence add --item K-02 --id E-500 --kind audit --source "red-team-A1" --desc "achado de auditoria" --provenance P2`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			// Procedência: default P0 quando não informada; inválida → erro.
			prov := knowledge.ProvenanceUnknown
			if strings.TrimSpace(provenance) != "" {
				var ok bool
				prov, ok = knowledge.ParseProvenance(provenance)
				if !ok {
					return fmt.Errorf("--provenance inválido: %q (use P0 a P5)", provenance)
				}
			}

			// P4/P5 exigem commit + sha256: sem eles não há reprodutibilidade.
			if prov == knowledge.ProvenanceReproducible || prov == knowledge.ProvenanceIndependent {
				if strings.TrimSpace(commit) == "" || strings.TrimSpace(sha256) == "" {
					return fmt.Errorf("--commit e --sha256 são obrigatórios para procedência P4/P5 (código/teste reproduzível)")
				}
			}

			lawsPath, err := resolveLawsPath()
			if err != nil {
				return err
			}

			// Primeira execução: semear as 5 leis reais (idempotente).
			if _, err := ensureSeededLaws(lawsPath); err != nil {
				return err
			}

			engine, err := loadLawsEngine(lawsPath)
			if err != nil {
				return err
			}

			// O item deve existir (ao contrário do law add-evidence, este
			// comando não cria itens — só anexa evidência com procedência).
			if _, ok := engine.Get(item); !ok {
				return fmt.Errorf("item %q não encontrado em %s (use \"cosca knowledge law list\" para ver os IDs)", item, lawsPath)
			}

			if strings.TrimSpace(evID) == "" {
				return fmt.Errorf("--id é obrigatório para adicionar evidência")
			}
			if strings.TrimSpace(kind) == "" || strings.TrimSpace(source) == "" || strings.TrimSpace(desc) == "" {
				return fmt.Errorf("--kind, --source e --desc são obrigatórios para adicionar evidência")
			}

			now := time.Now()
			ev := knowledge.Evidence{
				ID:          evID,
				Kind:        kind,
				Source:      source,
				Description: desc,
				Timestamp:   now,
				Provenance:  prov,
				Repository:  repo,
				Commit:      commit,
				Path:        evidencePath,
				SHA256:      sha256,
				Retrieved:   now,
			}
			itemOut, err := engine.AddEvidence(item, ev)
			if err != nil {
				return err
			}
			if err := engine.Save(lawsPath); err != nil {
				return fmt.Errorf("salvar %s: %w", lawsPath, err)
			}

			if useJSON {
				return printJSON(cmd, itemOut)
			}

			formatter.Success(fmt.Sprintf(
				"Evidência %s adicionada ao item %s — procedência %s (%s), nível %q, %d evidência(s)",
				ev.ID, itemOut.ID, ev.Provenance, ev.Provenance.Description(), itemOut.Level, len(itemOut.Evidence)))
			return nil
		},
	}

	cmd.Flags().StringVar(&item, "item", "", "ID do item de conhecimento (deve existir)")
	cmd.Flags().StringVar(&evID, "id", "", "ID da evidência (ex.: E-401)")
	cmd.Flags().StringVar(&kind, "kind", "", "tipo da evidência (incidente, auditoria, teste, benchmark, projeto, sessão)")
	cmd.Flags().StringVar(&source, "source", "", "fonte da evidência (arquivo, commit, URL, benchmark...)")
	cmd.Flags().StringVar(&desc, "desc", "", "descrição da evidência")
	cmd.Flags().StringVar(&provenance, "provenance", "", "procedência P0-P5 (default P0: desconhecida)")
	cmd.Flags().StringVar(&repo, "repo", "", "repositório da fonte (ex.: CoscaAI/cosca)")
	cmd.Flags().StringVar(&commit, "commit", "", "commit da fonte — obrigatório para P4/P5")
	cmd.Flags().StringVar(&evidencePath, "path", "", "caminho no repositório (ex.: internal/runtime/foo.go)")
	cmd.Flags().StringVar(&sha256, "sha256", "", "hash SHA-256 do artefato — obrigatório para P4/P5")
	_ = cmd.MarkFlagRequired("item")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("kind")
	_ = cmd.MarkFlagRequired("source")
	_ = cmd.MarkFlagRequired("desc")
	return cmd
}

// NewKnowledgeEvidenceListCommand cria o comando
// `cosca knowledge evidence list <item>`.
func NewKnowledgeEvidenceListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list <item>",
		Short: "Lista as evidências de um item com procedência, commit e sha256",
		Long: `Lista as evidências de um item do CKL numa tabela com procedência
(P0-P5), commit e SHA-256 abreviado, e se cada evidência é totalmente
reproduzível.`,
		Example: `  cosca knowledge evidence list K-01
  cosca knowledge evidence list K-01 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			lawsPath, err := resolveLawsPath()
			if err != nil {
				return err
			}
			if _, err := ensureSeededLaws(lawsPath); err != nil {
				return err
			}
			engine, err := loadLawsEngine(lawsPath)
			if err != nil {
				return err
			}

			item, ok := engine.Get(args[0])
			if !ok {
				return fmt.Errorf("item %q não encontrado em %s", args[0], lawsPath)
			}

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"item":     item,
					"evidence": item.Evidence,
				})
			}

			if len(item.Evidence) == 0 {
				formatter.Warning(fmt.Sprintf(
					"Item %s não possui evidências ainda — use \"cosca knowledge evidence add --item %s ...\".",
					item.ID, item.ID))
				return nil
			}

			formatter.Header(fmt.Sprintf("Evidências de %s — %s (%d)", item.ID, item.Title, len(item.Evidence)))
			rows := make([][]string, 0, len(item.Evidence))
			for _, ev := range item.Evidence {
				rows = append(rows, []string{
					ev.ID,
					ev.Kind,
					string(ev.Provenance),
					ev.Commit,
					abbreviateSHA(ev.SHA256),
					reproYesNo(ev.IsReproducible()),
				})
			}
			formatter.Table([]string{"ID", "Tipo", "Proveniência", "Commit", "SHA256", "Reproduzível"}, rows)
			return nil
		},
	}
}

// NewKnowledgeEvidenceReproCommand cria o comando
// `cosca knowledge evidence repro <item>`.
func NewKnowledgeEvidenceReproCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "repro <item>",
		Short: "Mostra quais evidências são totalmente reproduzíveis (P4/P5)",
		Long: `Mostra quais evidências de um item são totalmente reproduzíveis:
procedência >= P4 (código/teste reproduzível ou múltiplas fontes
independentes) E com repository + commit + sha256 — o trio que permite
re-buscar o código exato daquela versão.

É a resposta do Don: "Foi desse código, exatamente nessa versão, que essa
evidência veio."`,
		Example: `  cosca knowledge evidence repro K-01
  cosca knowledge evidence repro K-01 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			lawsPath, err := resolveLawsPath()
			if err != nil {
				return err
			}
			if _, err := ensureSeededLaws(lawsPath); err != nil {
				return err
			}
			engine, err := loadLawsEngine(lawsPath)
			if err != nil {
				return err
			}

			item, ok := engine.Get(args[0])
			if !ok {
				return fmt.Errorf("item %q não encontrado em %s", args[0], lawsPath)
			}

			repro := item.ReproducibleEvidence()

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"item":         item,
					"reproducible": repro,
				})
			}

			formatter.Header(fmt.Sprintf("Evidências reproduzíveis — %s (%s)", item.ID, item.Title))
			formatter.KeyValue("Total de evidências", fmt.Sprintf("%d", len(item.Evidence)))
			formatter.KeyValue("Reproduzíveis", fmt.Sprintf("%d", len(repro)))
			for _, ev := range repro {
				formatter.Bullet(fmt.Sprintf("%s [%s] %s — %s@%s",
					ev.ID, ev.Kind, ev.Description, ev.Repository, ev.Commit))
			}
			if len(repro) == 0 {
				formatter.Warning("Nenhuma evidência é totalmente reproduzível — requer P4/P5 com repository + commit + sha256.")
			}
			return nil
		},
	}
}

// abbreviateSHA encurta o hash SHA-256 para exibição em tabela: os 12
// primeiros caracteres seguidos de "…" quando há mais.
func abbreviateSHA(sha string) string {
	if len(sha) <= 12 {
		return sha
	}
	return sha[:12] + "…"
}

// reproYesNo formata o resultado de IsReproducible para a tabela.
func reproYesNo(reproducible bool) string {
	if reproducible {
		return "sim"
	}
	return "não"
}
