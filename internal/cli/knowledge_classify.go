package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
)

// NewKnowledgeClassifyCommand cria `cosca knowledge classify`.
//
// FASE 5 — Backfill epistêmico do legado. O knowledge.db existente foi indexado
// ANTES da Fase 2/4: os documentos não têm `metadata_json.epistemic`
// (verificado na 4.2: 0/2387 docs). Este comando percorre TODOS os documentos
// existentes e povoa a classe epistêmica DETERMINÍSTICA de cada um, de forma
// ADITIVA + IDEMPOTENTE + FAIL-CLOSED — sem re-indexar, sem tocar
// chunks/vectors/grafo, sem migração destrutiva.
//
// Sempre rode `--dry-run` primeiro (não escreve nada) para auditar os números;
// depois aplique de verdade. Em caso de qualquer dúvida, o backfill é reversível
// (só remove as chaves epistemic/kind/scope adicionadas deste metadata_json).
func NewKnowledgeClassifyCommand() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "classify",
		Short: "Backfill epistêmico: povoa metadata_json.epistemic do legado (aditivo e idempotente)",
		Long: `Povoa a classe epistêmica (FACT/MEASURED/EVIDENCE/INFERRED/RULE/DECISION/PROFILE)
de todos os documentos já indexados no knowledge base, usando o classificador
determinístico da Fase 2 + o mapeamento epistemológico da Fase 4.

Aditivo: mescla no metadata_json, nunca remove chave existente.
Idempotente: documento que já tem epistemic é pulado.
Fail-closed: transiente/baixa confiança não é classificado (fica como está).
Reversível: é só metadata — remover as chaves volta ao estado anterior.
Sem re-indexar: não toca em chunks, vetores nem grafo.

Use --dry-run para auditar os números sem escrever nada.`,
		Example: `  cosca knowledge classify --dry-run   # auditar (não escreve)
  cosca knowledge classify             # aplicar de verdade
  cosca knowledge classify --json      # saída estruturada`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			kbDir := filepath.Join(dir, ".cosca")

			ke, kErr := openKnowledgeEngine(kbDir, false, dir)
			if kErr != nil {
				return fmt.Errorf("knowledge engine not available: %w", kErr)
			}
			if iErr := ke.Init(); iErr != nil {
				return fmt.Errorf("knowledge engine init failed: %w", iErr)
			}
			defer ke.Close()

			if dryRun {
				formatter.Warning("DRY-RUN — nenhuma escrita; apenas prognóstico")
			}

			rep, bErr := ke.BackfillEpistemic(cmd.Context(), dryRun)
			if bErr != nil {
				return fmt.Errorf("backfill epistémico falhou: %w", bErr)
			}

			if useJSON {
				return printJSON(cmd, rep)
			}

			formatter.Header(fmt.Sprintf("Backfill epistêmico — %d documentos", rep.Scanned))
			formatter.KeyValue("atualizados", fmt.Sprintf("%d", rep.Updated))
			formatter.KeyValue("já rotulados", fmt.Sprintf("%d", rep.AlreadyLabeled))
			formatter.KeyValue("arquivo ausente", fmt.Sprintf("%d", rep.MissingFile))
			formatter.KeyValue("falha ao classificar (fail-closed)", fmt.Sprintf("%d", rep.NotPersistent))
			formatter.KeyValue("sem kind classificável", fmt.Sprintf("%d", rep.NoKind))
			if len(rep.ByEpistemic) > 0 {
				keys := make([]string, 0, len(rep.ByEpistemic))
				for k := range rep.ByEpistemic {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				formatter.Header("Distribuição por classe epistêmica")
				for _, k := range keys {
					formatter.Bullet(fmt.Sprintf("%-9s %d", k, rep.ByEpistemic[k]))
				}
			}
			if dryRun {
				formatter.Warning("Nada foi escrito. Rode sem --dry-run para aplicar.")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Calcular e reportar sem escrever nada (auditoria)")

	return cmd
}
