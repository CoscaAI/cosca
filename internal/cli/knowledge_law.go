//
// `cosca knowledge law` — o Cosca Knowledge Lifecycle (CKL) via CLI.
//
// Subcomandos:
//   list          — tabela de todas as leis/regras (ID | Título | Nível |
//                   Evidências | Confiança); na primeira execução semeia as
//                   5 leis reais (seed) quando laws.json não existe
//   show <id>     — lei completa + WhyExists() (as evidências)
//   add-evidence <id> --kind --source --desc [--title] [--confidence] —
//                   adiciona evidência e promove; --title nomeia a lei
//                   criada; --confidence registra a confiança medida (0-1)
//   approve <id>  — aprovação do Don → constituição
//
// O motor de promoção vive em internal/knowledge (PromotionEngine) com
// persistência JSON. O CLI lê/escreve em .cosca/knowledge/laws.json
// (runtime, gitignored). Load no início de cada comando, Save após mutação.
// NUNCA toca em .cosca/framework.
//

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/knowledge"
)

// lawsFileName é o arquivo runtime das leis do CKL (gitignored).
const lawsFileName = "laws.json"

// resolveLawsPath devolve o caminho do arquivo runtime de leis do CKL para o
// diretório de trabalho atual (o Don roda o CLI no projeto):
// <projeto>/.cosca/knowledge/laws.json.
func resolveLawsPath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	return filepath.Join(dir, ".cosca", "knowledge", lawsFileName), nil
}

// loadLawsEngine carrega o PromotionEngine do arquivo runtime. Um arquivo
// inexistente não é erro: resulta em engine vazio (nenhuma lei).
func loadLawsEngine(path string) (*knowledge.PromotionEngine, error) {
	engine := knowledge.NewPromotionEngine()
	if err := engine.Load(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return engine, nil
		}
		return nil, err
	}
	return engine, nil
}

// NewKnowledgeLawCommand cria o comando `cosca knowledge law`.
func NewKnowledgeLawCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "law",
		Short: "Gerencia as leis do conhecimento (CKL)",
		Long: `Gerencia as leis e regras do Cosca Knowledge Lifecycle (CKL).

Cada lei nasce como observação (Princípio 1) e sobe a escada de evidência
(Princípio 3): observation → learning → hypothesis → theory → law.
A aprovação manual do Don eleva a lei à constituição (Princípio 5).

Persistência: .cosca/knowledge/laws.json (runtime, gitignored).
`,
		Example: `  cosca knowledge law list
  cosca knowledge law show K-1
  cosca knowledge law add-evidence K-1 --kind benchmark --source "test/" --desc "reduziu latência em 40%"
  cosca knowledge law approve K-1`,
	}

	cmd.AddCommand(
		NewKnowledgeLawListCommand(),
		NewKnowledgeLawShowCommand(),
		NewKnowledgeLawAddEvidenceCommand(),
		NewKnowledgeLawApproveCommand(),
		NewKnowledgeLawPromoteCommand(),
		NewKnowledgeLawUpgradeCommand(),
	)
	return cmd
}

// NewKnowledgeLawListCommand cria o comando `cosca knowledge law list`.
func NewKnowledgeLawListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Lista todas as leis/regras com nível, confiança e evidências",
		Long: `Lista todas as leis/regras do CKL com nível, confiança e número de
evidências.

Na primeira execução (quando .cosca/knowledge/laws.json ainda não existe),
semeia as 5 leis REAIS que o Cosca já produziu nas ondas de segurança —
cada uma com suas evidências de incidentes, auditorias red team e testes.
O seed é idempotente: nunca sobrescreve um arquivo existente.`,
		Example: `  cosca knowledge law list
  cosca knowledge law list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			path, err := resolveLawsPath()
			if err != nil {
				return err
			}

			// Primeira execução: semear as 5 leis reais (idempotente — um
			// arquivo existente nunca é sobrescrito).
			seeded, err := ensureSeededLaws(path)
			if err != nil {
				return err
			}

			engine, err := loadLawsEngine(path)
			if err != nil {
				return err
			}
			items := engine.All()

			if seeded {
				formatter.Verbose(fmt.Sprintf(
					"Primeira execução: %d leis seed criadas em %s (runtime, gitignored)",
					len(items), path))
			}

			if useJSON {
				return printJSON(cmd, items)
			}

			if len(items) == 0 {
				formatter.Warning("Nenhuma lei registrada ainda — use \"cosca knowledge law add-evidence <id> ...\".")
				return nil
			}

			formatter.Header(fmt.Sprintf("Leis e Regras do Conhecimento (%d)", len(items)))
			rows := make([][]string, 0, len(items))
			for _, it := range items {
				rows = append(rows, []string{
					it.ID,
					it.Title,
					string(it.Level),
					fmt.Sprintf("%d", len(it.Evidence)),
					fmt.Sprintf("%.0f%%", it.Confidence*100),
				})
			}
			formatter.Table([]string{"ID", "Título", "Nível", "Evidências", "Confiança"}, rows)
			return nil
		},
	}
}

// NewKnowledgeLawShowCommand cria o comando `cosca knowledge law show <id>`.
func NewKnowledgeLawShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "show <id>",
		Short:   "Mostra a lei completa + WhyExists() (as evidências)",
		Example: `  cosca knowledge law show K-1`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			path, err := resolveLawsPath()
			if err != nil {
				return err
			}
			engine, err := loadLawsEngine(path)
			if err != nil {
				return err
			}

			item, ok := engine.Get(args[0])
			if !ok {
				return fmt.Errorf("lei %q não encontrada em %s", args[0], path)
			}

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"item": item,
					"why":  item.WhyExists(),
				})
			}

			formatter.Header(fmt.Sprintf("Lei %s — %s", item.ID, item.Title))
			formatter.KeyValue("Nível", string(item.Level))
			formatter.KeyValue("Confiança", fmt.Sprintf("%.0f%%", item.Confidence*100))
			formatter.KeyValue("Evidências", fmt.Sprintf("%d", len(item.Evidence)))
			formatter.KeyValue("Projetos validados", fmt.Sprintf("%d", item.Projects))
			formatter.KeyValue("Rollbacks", fmt.Sprintf("%d", item.Rollbacks))
			reproducible := "não"
			if item.Reproducible {
				reproducible = "sim"
			}
			formatter.KeyValue("Reproduzível", reproducible)
			formatter.KeyValue("Criada em", item.CreatedAt.Format("2006-01-02"))
			formatter.Header("WhyExists — por que esta lei existe?")
			formatter.Println(item.WhyExists())
			return nil
		},
	}
}

// NewKnowledgeLawAddEvidenceCommand cria o comando
// `cosca knowledge law add-evidence <id>`.
func NewKnowledgeLawAddEvidenceCommand() *cobra.Command {
	var (
		kind       string
		source     string
		desc       string
		title      string
		confidence float64
	)

	cmd := &cobra.Command{
		Use:   "add-evidence <id>",
		Short: "Adiciona evidência a uma lei e promove automaticamente",
		Long: `Adiciona uma evidência e promove automaticamente a lei.

A lei nasce como observação na primeira evidência (Princípio 1) e sobe a
escada conforme acumula evidência (Princípio 3): observation → learning →
hypothesis → theory → law. A evidência persiste em
.cosca/knowledge/laws.json — se a lei ainda não existir, ela é criada.

--title nomeia a lei quando ela é criada (default: o ID). --confidence
registra a confiança medida da evidência (0-1): com evidência real medida,
a lei pode ser promovida para learning (confiança >= 0.70) em vez de ficar
na observação.`,
		Example: `  cosca knowledge law add-evidence K-1 --kind benchmark --source "test/" --desc "reduziu latência em 40%"
  cosca knowledge law add-evidence K-27 --title "Nunca executar como root automaticamente" --kind audit --source "red-team-A1" --desc "jail deve recusar root" --confidence 0.95
  cosca knowledge law add-evidence K-2 --kind auditoria --source "audit/2026-07.md" --desc "sem violações"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			// Confiança é uma escala 0-1 (Princípio 3: conhecimento sem
			// evidência é opinião — fora da escala não é medição).
			if confidence < 0 || confidence > 1 {
				return fmt.Errorf("--confidence deve estar entre 0 e 1 (recebido %.2f)", confidence)
			}

			path, err := resolveLawsPath()
			if err != nil {
				return err
			}
			engine, err := loadLawsEngine(path)
			if err != nil {
				return err
			}

			// Evidência sem tipo, fonte ou descrição não é evidência
			// (Princípio 3: conhecimento sem evidência é opinião).
			if strings.TrimSpace(kind) == "" || strings.TrimSpace(source) == "" || strings.TrimSpace(desc) == "" {
				return fmt.Errorf("--kind, --source e --desc são obrigatórios para adicionar evidência")
			}

			// A lei nasce da primeira evidência (Princípio 1): se o ID ainda
			// não existe, registra uma nova observação antes de adicionar.
			// O título vem de --title (default: o ID).
			if _, ok := engine.Get(args[0]); !ok {
				conf := 0.10 // observação: confiança baixa sem medição
				if confidence > 0 {
					conf = confidence
				}
				lawTitle := args[0]
				if strings.TrimSpace(title) != "" {
					lawTitle = title
				}
				if err := engine.Register(&knowledge.KnowledgeItem{
					ID:         args[0],
					Title:      lawTitle,
					Confidence: conf,
				}); err != nil {
					return err
				}
			} else if confidence > 0 {
				// Lei já existe: registra a confiança medida da evidência
				// para permitir a promoção (learning exige >= 0.70).
				if err := engine.SetConfidence(args[0], confidence); err != nil {
					return err
				}
			}

			// ID único por evidência. `time.Now().UnixNano()` colide em
			// Windows (relógio de resolução grossa — duas chamadas rápidas
			// retornam o mesmo valor), o que faz o AddEvidence SUBSTITUIR
			// a evidência anterior em vez de acumular. Usa crypto/rand:
			// unicidade garantida independente do relógio.
			evID, evIDErr := randomHexSecret(8)
			if evIDErr != nil {
				return fmt.Errorf("gerar ID de evidência: %w", evIDErr)
			}
			ev := knowledge.Evidence{
				ID:          fmt.Sprintf("%s-%s", args[0], evID),
				Kind:        kind,
				Source:      source,
				Description: desc,
				Timestamp:   time.Now(),
			}
			item, err := engine.AddEvidence(args[0], ev)
			if err != nil {
				return err
			}
			if err := engine.Save(path); err != nil {
				return fmt.Errorf("salvar %s: %w", path, err)
			}

			if useJSON {
				return printJSON(cmd, item)
			}

			formatter.Success(fmt.Sprintf(
				"Evidência adicionada a %s — nível %q, %d evidência(s), confiança %.0f%%",
				item.ID, item.Level, len(item.Evidence), item.Confidence*100))
			return nil
		},
	}

	cmd.Flags().StringVar(&kind, "kind", "", "tipo da evidência (incidente, auditoria, teste, benchmark, projeto, sessão)")
	cmd.Flags().StringVar(&source, "source", "", "fonte da evidência (arquivo, commit, URL, benchmark...)")
	cmd.Flags().StringVar(&desc, "desc", "", "descrição da evidência")
	cmd.Flags().StringVar(&title, "title", "", "título da lei (usado ao criar)")
	cmd.Flags().Float64Var(&confidence, "confidence", 0.0, "confiança 0-1 (medida por evidência real; default 0.10)")
	_ = cmd.MarkFlagRequired("kind")
	_ = cmd.MarkFlagRequired("source")
	_ = cmd.MarkFlagRequired("desc")
	return cmd
}

// NewKnowledgeLawApproveCommand cria o comando `cosca knowledge law approve <id>`.
func NewKnowledgeLawApproveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "approve <id>",
		Short: "Aprovação do Don — promove a lei a princípio constitucional",
		Long: `Registra a aprovação manual do Don: a lei é elevada a constituição
(nível "constitution"). Nenhuma promoção automática chega lá — somente a
palavra do Don (Princípio 5).`,
		Example: `  cosca knowledge law approve K-1`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			path, err := resolveLawsPath()
			if err != nil {
				return err
			}
			engine, err := loadLawsEngine(path)
			if err != nil {
				return err
			}

			if err := engine.ApproveConstitution(args[0]); err != nil {
				return err
			}
			if err := engine.Save(path); err != nil {
				return fmt.Errorf("salvar %s: %w", path, err)
			}

			item, ok := engine.Get(args[0])
			if !ok {
				return fmt.Errorf("lei %q não encontrada após mutação", args[0])
			}

			if useJSON {
				return printJSON(cmd, item)
			}

			formatter.Success(fmt.Sprintf(
				"Lei %s aprovada pelo Don → Constituição (nível %q)",
				item.ID, item.Level))
			return nil
		},
	}
}

// NewKnowledgeLawPromoteCommand cria o comando `cosca knowledge law promote <id>`.
func NewKnowledgeLawPromoteCommand() *cobra.Command {
	var verify bool

	cmd := &cobra.Command{
		Use:   "promote <id>",
		Short: "Promoção manual do Don — eleva a lei ao nível law (status KNOWN)",
		Long: `Registra a promoção manual do Don: a lei é elevada ao nível "law"
com status "KNOWN". Esta é a rota curta para itens que não podem
realisticamente acumular 200 evidências, mas foram validados pelo Don.

Com --verify, também atualiza last_verified e verification_count após a
promoção — útil quando o Don está conferindo a acurácia da lei pessoalmente.`,
		Example: `  cosca knowledge law promote K-01
  cosca knowledge law promote K-01 --verify`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			path, err := resolveLawsPath()
			if err != nil {
				return err
			}
			engine, err := loadLawsEngine(path)
			if err != nil {
				return err
			}

			if err := engine.ApproveLaw(args[0]); err != nil {
				return err
			}
			if verify {
				if err := engine.VerifyItem(args[0]); err != nil {
					return err
				}
			}
			if err := engine.Save(path); err != nil {
				return fmt.Errorf("salvar %s: %w", path, err)
			}

			item, ok := engine.Get(args[0])
			if !ok {
				return fmt.Errorf("lei %q não encontrada após mutação", args[0])
			}

			if useJSON {
				return printJSON(cmd, item)
			}

			msg := fmt.Sprintf(
				"Lei %s promovida pelo Don → Law (nível %q, status %q)",
				item.ID, item.Level, item.Status)
			if verify {
				msg += " — verificado"
			}
			formatter.Success(msg)
			return nil
		},
	}

	cmd.Flags().BoolVar(&verify, "verify", false, "também atualiza last_verified e verification_count após a promoção")
	return cmd
}

// NewKnowledgeLawUpgradeCommand cria o comando `cosca knowledge law upgrade [id]`.
// Atualiza metadados (reproducible + provenance) de leis existentes a partir das
// definições canônicas do seed.
func NewKnowledgeLawUpgradeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade [id]",
		Short: "Atualiza metadados (reproducible + provenance) de leis existentes",
		Long: `Atualiza metadados de leis existentes a partir das definições canônicas
do seed (SeedDefinitions). Útil quando metadados mudaram — por exemplo,
reproducible: false → true ou provenance P0 → P5 — e o seed não vai
reescrever o arquivo (ele só roda na primeira execução).

Sem argumentos, atualiza TODAS as leis conhecidas. Com um ID, atualiza
apenas aquela lei específica.`,
		Example: `  cosca knowledge law upgrade
  cosca knowledge law upgrade K-01`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			path, err := resolveLawsPath()
			if err != nil {
				return err
			}

			// Carrega os dados brutos do JSON para modificar in-place
			// e depois salvar de volta.
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("ler %s: %w (use 'cosca knowledge law list' primeiro para criar o arquivo)", path, err)
			}

			var store lawStoreJSON
			if err := json.Unmarshal(data, &store); err != nil {
				return fmt.Errorf("parse %s: %w", path, err)
			}

			// Constrói mapa de definições canônicas do seed.
			defs := SeedDefinitions()
			byID := make(map[string]SeedMetadata, len(defs))
			for _, d := range defs {
				byID[d.ID] = d
			}

			// Determina escopo.
			scopeSet := make(map[string]bool)
			if len(args) == 1 {
				if _, ok := byID[args[0]]; !ok {
					return fmt.Errorf("lei %q não tem definição canônica no seed", args[0])
				}
				scopeSet[args[0]] = true
			} else {
				for _, d := range defs {
					scopeSet[d.ID] = true
				}
			}

			updated := 0
			for i := range store.Items {
				item := &store.Items[i]
				if !scopeSet[item.ID] {
					continue
				}
				def, ok := byID[item.ID]
				if !ok {
					continue
				}

				changed := false

				// Atualiza reproducible.
				if item.Reproducible != def.Reproducible {
					item.Reproducible = def.Reproducible
					changed = true
				}

				// Atualiza provenance das evidências que batem por ID.
				for j := range item.Evidence {
					ev := &item.Evidence[j]
					for _, seedEv := range def.Evidences {
						if ev.ID != seedEv.ID {
							continue
						}
						// Só atualiza se a procedência atual é mais baixa
						// ou se os campos de source ref estão vazios.
						if ev.Provenance.Rank() < seedEv.Provenance.Rank() ||
							ev.Repository == "" || ev.Path == "" {
							ev.Provenance = seedEv.Provenance
							ev.Repository = seedEv.Repository
							ev.Commit = seedEv.Commit
							if seedEv.Path != "" && ev.Path == "" {
								ev.Path = seedEv.Path
							}
							if seedEv.SHA256 != "" && ev.SHA256 == "" {
								ev.SHA256 = seedEv.SHA256
							}
							changed = true
						}
						break // evidência encontrada, próximo item.Evidence
					}
				}

				if changed {
					updated++
				}
			}

			if updated > 0 {
				data, err := json.MarshalIndent(store, "", "  ")
				if err != nil {
					return fmt.Errorf("encode: %w", err)
				}
				if err := os.WriteFile(path, data, 0600); err != nil {
					return fmt.Errorf("salvar %s: %w", path, err)
				}
			}

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"updated": updated,
					"scoped":  len(scopeSet),
				})
			}

			if updated == 0 {
				formatter.Verbose("Nenhuma lei precisava de atualização — metadados já estão corretos.")
			} else {
				formatter.Success(fmt.Sprintf(
					"%d lei(s) atualizada(s) com metadados canônicos do seed (reproducible + provenance).",
					updated))
			}
			return nil
		},
	}
}

// lawStoreJSON espelha o layout JSON on-disk do PromotionEngine.
type lawStoreJSON struct {
	Version int                       `json:"version"`
	Items   []knowledge.KnowledgeItem `json:"items"`
}
