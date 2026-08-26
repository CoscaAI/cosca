package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// NewKnowledgeVectorsBackfillCommand cria `cosca knowledge vectors-backfill`.
//
// FASE 5.3 — completo a cobertura vetorial do legado: embede os chunks que ainda
// não têm vetor (derivado do conteúdo, idempotente, aditivo). Sempre rode
// `--dry-run` primeiro (não escreve): audita o gap. A geração usa o provider
// local (Ollama nomic-embed-text, 768) — inferência local, ALMA-compatível.
func NewKnowledgeVectorsBackfillCommand() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "vectors-backfill",
		Short: "Embede os chunks sem vetor (cobertura do significado) — idempotente, aditivo",
		Long: `Embede os chunks que ainda não têm vetor no índice vetorial, derivado do
conteúdo. Idempotente (só os que faltam), aditivo (não toca nos existentes),
não-destrutivo. Fecha o gap de significado que a auditoria 5.3 mediu (~9854 chunks).

Use --dry-run para auditar o gap sem escrever. A geração usa o embedding local
(config: ollama nomic-embed-text, 768-dim) — inferência local, ALMA-compatível.`,
		Example: `  cosca knowledge vectors-backfill --dry-run   # auditar (não escreve)
  cosca knowledge vectors-backfill             # embede os que faltam`,
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
				formatter.Warning("DRY-RUN — nenhuma escrita; apenas auditoria")
			}

			rep, bErr := ke.BackfillVectors(cmd.Context(), dryRun)
			if bErr != nil {
				return fmt.Errorf("backfill de vetores falhou: %w", bErr)
			}

			if useJSON {
				return printJSON(cmd, rep)
			}

			formatter.Header(fmt.Sprintf("Backfill de vetores — %d chunks no total", rep.ChunksTotal))
			formatter.KeyValue("sem vetor (gap)", fmt.Sprintf("%d", rep.Missing))
			formatter.KeyValue("embebidos", fmt.Sprintf("%d", rep.Filled))
			formatter.KeyValue("falharam", fmt.Sprintf("%d", rep.Failed))
			formatter.KeyValue("duracao (s)", fmt.Sprintf("%.1f", rep.DurationSec))
			if len(rep.ByArea) > 0 {
				formatter.Header("Por área")
				for a, n := range rep.ByArea {
					formatter.Bullet(fmt.Sprintf("%-14s %d", a, n))
				}
			}
			if dryRun {
				formatter.Warning("Nada foi escrito. Rode sem --dry-run para embeder.")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Auditar o gap sem escrever nada")

	return cmd
}
