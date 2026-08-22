package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/nodegraph"
	"github.com/CoscaAI/cosca/internal/render"
)

// NewRenderCommand creates the `cosca render` command group — Render Engine
// (§22 do manifesto Creative/Scientific/Media), Fase 1 etapa 1.7.
func NewRenderCommand() *cobra.Command {
	var quality string
	var seed int64
	var checkpoint string
	var useCache bool

	cmd := &cobra.Command{
		Use:   "render <workflow.json>",
		Short: "Render Engine — deterministic, cacheable, resumable (§22)",
		Long: `Render Engine (§22) — executa um node graph com:
  - QUALIDADE: preview (rápido) → draft → final (completo)
  - DETERMINISMO: mesmo seed + mesmo grafo = mesmo resultado
  - CACHE por assinatura (§23): não re-renderiza o inalterado
  - RESUMÍVEL: checkpoint em disco — crash recovery (§40)

O executor roda nós de MÍDIA (load, probe, extract_audio, transcode,
extract_frame, convert_audio, validate) via ffmpeg/ffprobe; outros tipos de nó
passam direto — executores de tasks/models/gpu plugam nas próximas etapas.
Use --cache para ver o cache por assinatura (§23) e --checkpoint para o
resume (§40) em ação.`,
		Example: `  cosca render workflow.json
  cosca render workflow.json --quality final --seed 42
  cosca render workflow.json --cache
  cosca render workflow.json --checkpoint .cosca/checkpoints/render.json
  cosca render workflow.json --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			g, err := loadNodeGraphFile(args[0])
			if err != nil {
				return err
			}

			job, err := render.New(g, render.Quality(quality), seed)
			if err != nil {
				return err
			}
			job.CheckpointPath = checkpoint
			if useCache {
				job.Cache = nodegraph.NewCache()
			}

			exec := newDispatchExecutor(nil)

			r := render.NewRenderer()
			res, err := r.Render(context.Background(), job, exec)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, res)
			}

			formatter.Header(fmt.Sprintf("Render %s — %s", res.Quality, g.Name))
			formatter.KeyValue("Job Key", res.JobKey[:16]+"...")
			formatter.KeyValue("Quality", string(res.Quality))
			formatter.KeyValue("Executed", fmt.Sprint(res.Executed))
			formatter.KeyValue("Cache Hits", fmt.Sprint(res.CachedHits))
			formatter.KeyValue("Resumed", fmt.Sprint(res.Resumed))
			formatter.KeyValue("Duration", fmt.Sprintf("%d ms", res.DurationMs))
			formatter.Println("")
			formatter.Println("  Results:")
			for _, n := range g.Nodes {
				formatter.Bullet(fmt.Sprintf("%s → %v", n.ID, res.Results[n.ID]))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&quality, "quality", "draft", "Render quality: "+render.QualitiesList())
	cmd.Flags().Int64Var(&seed, "seed", 0, "Determinism seed (§22)")
	cmd.Flags().StringVar(&checkpoint, "checkpoint", "", "Checkpoint file path (resumible §40)")
	cmd.Flags().BoolVar(&useCache, "cache", false, "Enable signature cache (§23)")
	return cmd
}
