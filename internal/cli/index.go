package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/knowledge"
)

// NewIndexCommand creates the `cosca index` command and its subcommands.
func NewIndexCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Manage the Cosca index",
		Long: `Manage the Cosca file index.

The index tracks all files in your project for fast search and knowledge retrieval.
It supports incremental updates and full rebuilds.

Subcommands:
  rebuild    Rebuild the entire index from scratch
  update     Incremental update of the index
  status     Show index status
  stats      Show index statistics
  verify     Verify index integrity
`,
		Example: `  cosca index rebuild        Rebuild the index
  cosca index update         Incremental update
  cosca index status         Index status overview
  cosca index stats          Detailed index statistics
  cosca index verify         Verify index integrity`,
	}

	cmd.AddCommand(
		NewIndexRebuildCommand(),
		NewIndexUpdateCommand(),
		NewIndexStatusCommand(),
		NewIndexStatsCommand(),
		NewIndexVerifyCommand(),
	)

	return cmd
}

// NewIndexRebuildCommand creates the `cosca index rebuild` subcommand.
func NewIndexRebuildCommand() *cobra.Command {
	var rebuildVectors bool
	var watch bool
	var watchInterval time.Duration

	cmd := &cobra.Command{
		Use:   "rebuild",
		Short: "Rebuild the entire index (file index [+ vetores + módulos do split])",
		Long: `Rebuild the entire index from scratch.

Padrão: reconstrói o índice de arquivos (scan + re-index).
Com --vectors: também refaz o índice VETORIAL (chunks sem vetor são
re-embebidos — idempotente, aditivo) e reconstrói os módulos físicos do
split (cosca db build, ADR-013 Fase C).

Com --watch: o rebuild roda e o processo CONTINUA verificando a cobertura
vetorial em loop (o "deixa rodando" do Don) — se a cobertura cair, o
backfill é disparado automaticamente.`,
		Example: `  cosca index rebuild                # file index only
  cosca index rebuild --vectors     # + vetores + módulos do split
  cosca index rebuild --vectors --watch --watch-interval 5m`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			spinner := formatter.Spinner("Rebuilding index from scratch")
			spinner.Start()

			idx := NewIndexer(dir)
			startTime := time.Now()

			if _, err := idx.Rebuild(); err != nil {
				spinner.Fail(fmt.Sprintf("Rebuild failed: %v", err))
				return fmt.Errorf("index rebuild failed: %w", err)
			}

			if rebuildVectors {
				formatter.Print("Reconstruindo índice vetorial (backfill)…")
				if err := rebuildVectorsCmd(dir, coscaDir); err != nil {
					spinner.Fail(fmt.Sprintf("Vector rebuild failed: %v", err))
					return fmt.Errorf("vector rebuild failed: %w", err)
				}
			}
			elapsed := time.Since(startTime)

			spinner.Stop("Index rebuilt")

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"status":     "ok",
					"duration":   elapsed.Round(time.Millisecond).String(),
					"vectors":    rebuildVectors,
					"watch_mode": watch,
				})
			}

			formatter.Success(fmt.Sprintf("Index rebuilt in %s", elapsed.Round(time.Millisecond)))

			if watch {
				formatter.Print("Modo contínuo: monitorando cobertura vetorial (Ctrl+C para parar)")
				return watchVectorCoverageLoop(cmd, coscaDir, watchInterval)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&rebuildVectors, "vectors", false, "reconstruir também o índice vetorial (backfill) + módulos do split")
	cmd.Flags().BoolVar(&watch, "watch", false, "modo contínuo: monitora a cobertura vetorial e dispara backfill se cair")
	cmd.Flags().DurationVar(&watchInterval, "watch-interval", 5*time.Minute, "intervalo do watchdog de cobertura")
	return cmd
}

// rebuildVectorsCmd refaz o índice vetorial (backfill idempotente) e
// reconstrói os módulos físicos do split (ADR-013 Fase C).
func rebuildVectorsCmd(dir, coscaDir string) error {
	// 1. Backfill: embebe chunks sem vetor (idempotente, aditivo).
	kb := filepath.Join(coscaDir, "knowledge.db")
	if _, err := os.Stat(kb); err == nil {
		eng, err := newKnowledgeEngineForIndex(dir, coscaDir)
		if err != nil {
			return fmt.Errorf("open knowledge engine: %w", err)
		}
		if eng != nil {
			if _, err := eng.BackfillVectors(context.Background(), false); err != nil {
				_ = eng.Close()
				return fmt.Errorf("vector backfill: %w", err)
			}
			_ = eng.Close()
		}
	}

	// 2. Módulos físicos do split (ADR-013 Fase C) — aditivo, reconstruível.
	kb = filepath.Join(coscaDir, "knowledge.db")
	if _, err := os.Stat(kb); err == nil {
		if _, err := dbBuildModules(nil, kb, coscaDir); err != nil {
			return fmt.Errorf("db build (split): %w", err)
		}
	}
	return nil
}

// watchVectorCoverageLoop monitora a cobertura vetorial e dispara backfill
// quando cai abaixo do limiar — o "deixa rodando" do Don.
func watchVectorCoverageLoop(cmd *cobra.Command, coscaDir string, interval time.Duration) error {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	formatter := GetFormatter(cmd)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	ctx := cmd.Context()
	for {
		select {
		case <-ctx.Done():
			formatter.Print("Watchdog de cobertura parado.")
			return nil
		case <-ticker.C:
			kb := filepath.Join(coscaDir, "knowledge.db")
			if _, err := os.Stat(kb); err != nil {
				continue
			}
			eng, err := newKnowledgeEngineForIndex(".", coscaDir)
			if err != nil || eng == nil {
				continue
			}
			chunks, vectors := eng.VectorCoverageCounts()
			var coverage float64
			if chunks > 0 {
				coverage = float64(vectors) / float64(chunks)
			}
			switch {
			case chunks == 0:
				formatter.Print("Cobertura vetorial: base vazia (aguardando indexação)")
			case coverage >= 0.8:
				formatter.Print(fmt.Sprintf("Cobertura vetorial: %.1f%% (%d/%d) — saudável", coverage*100, vectors, chunks))
			default:
				formatter.Warning(fmt.Sprintf("Cobertura vetorial: %.1f%% (%d/%d) — DEGRADADA, disparando backfill", coverage*100, vectors, chunks))
				if _, err := eng.BackfillVectors(context.Background(), false); err != nil {
					formatter.Error(fmt.Sprintf("Backfill falhou: %v", err))
				} else {
					formatter.Success("Backfill de vetores concluído")
				}
			}
			_ = eng.Close()
		}
	}
}

// newKnowledgeEngineForIndex abre o knowledge engine para operações de índice
// (backfill, cobertura). Segue o mesmo padrão do CLI knowledge.go com os envs
// de embedding do projeto.
func newKnowledgeEngineForIndex(dir, coscaDir string) (*knowledge.Engine, error) {
	var (
		embeddingProvider   string
		embeddingBaseURL    string
		embeddingModel      string
		embeddingDigest     string
		embeddingAPIKey     string
		embeddingDimensions int
	)
	if c, loadErr := config.Load(); loadErr == nil {
		embeddingProvider = c.Embedding.Provider
		embeddingBaseURL = c.Embedding.BaseURL
		embeddingModel = c.Embedding.Model
		embeddingDigest = c.Embedding.Digest
		embeddingAPIKey = c.Embedding.APIKey
		embeddingDimensions = c.Embedding.Dimensions
	}

	ke, err := knowledge.New(knowledge.Config{
		DBPath:              filepath.Join(coscaDir, "knowledge.db"),
		RootDir:             dir,
		AutoMigrate:         true,
		EmbeddingProvider:   embeddingProvider,
		EmbeddingBaseURL:    embeddingBaseURL,
		EmbeddingModel:      embeddingModel,
		EmbeddingDigest:     embeddingDigest,
		EmbeddingAPIKey:     embeddingAPIKey,
		EmbeddingDimensions: embeddingDimensions,
	})
	if err != nil {
		return nil, fmt.Errorf("knowledge engine not available: %w", err)
	}
	if err := ke.Init(); err != nil {
		_ = ke.Close()
		return nil, fmt.Errorf("init knowledge engine: %w", err)
	}
	return ke, nil
}

// NewIndexUpdateCommand creates the `cosca index update` subcommand.
func NewIndexUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Incrementally update the index",
		Long:  `Scan for changes since the last index update and apply them incrementally.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()

			spinner := formatter.Spinner("Updating index incrementally")
			spinner.Start()

			idx := NewIndexer(dir)
			_, err := idx.Update()
			if err != nil {
				spinner.Fail(fmt.Sprintf("Update failed: %v", err))
				return fmt.Errorf("index update failed: %w", err)
			}

			spinner.Stop("Index updated")

			if useJSON {
				return printJSON(cmd, map[string]string{"status": "ok"})
			}

			formatter.Success("Index updated successfully")
			return nil
		},
	}

	return cmd
}

// NewIndexStatusCommand creates the `cosca index status` subcommand.
func NewIndexStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show index status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()

			idx := NewIndexer(dir)
			status := idx.Status()

			if useJSON {
				return printJSON(cmd, status)
			}

			formatter.Header("Index Status")
			formatter.KeyValue("Files Indexed", fmt.Sprintf("%d", status.FilesIndexed))
			formatter.KeyValue("Last Sync", status.LastSync.Format("2006-01-02 15:04:05"))

			return nil
		},
	}

	return cmd
}

// NewIndexStatsCommand creates the `cosca index stats` subcommand.
func NewIndexStatsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show index statistics",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()

			idx := NewIndexer(dir)
			stats := idx.Stats()

			if useJSON {
				return printJSON(cmd, stats)
			}

			formatter.Header("Index Statistics")
			formatter.KeyValue("Total Files", fmt.Sprintf("%d", stats.TotalFiles))
			formatter.KeyValue("Indexed Files", fmt.Sprintf("%d", stats.IndexedFiles))

			return nil
		},
	}

	return cmd
}

// NewIndexVerifyCommand creates the `cosca index verify` subcommand.
func NewIndexVerifyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify index integrity",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()

			spinner := formatter.Spinner("Verifying index integrity")
			spinner.Start()

			idx := NewIndexer(dir)
			result, err := idx.Verify()
			if err != nil {
				spinner.Fail(fmt.Sprintf("Verification failed: %v", err))
				return fmt.Errorf("index verification failed: %w", err)
			}

			spinner.Stop(fmt.Sprintf("Verification complete: %d issues found", len(result.Issues)))

			if useJSON {
				return printJSON(cmd, result)
			}

			if len(result.Issues) == 0 {
				formatter.Success("Index integrity verified — no issues found")
			} else {
				formatter.Warning(fmt.Sprintf("Index verification found %d issues", len(result.Issues)))
			}

			return nil
		},
	}

	return cmd
}
