package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/nodegraph"
	"github.com/CoscaAI/cosca/internal/procgen"
	"github.com/CoscaAI/cosca/internal/render"
)

// NewNodeGraphDemoProceduralCommand — PROVA de funcionamento (P13) do
// Procedural Kernel System: monta um grafo real de nós procedurais e o
// executa pelo Render Engine com seed fixa 42.
//
// Grafo:  fbm(terreno) → math_clamp → math_lerp ← gradient
//
// Gera PNGs em `.cosca/procgen-demo/` (terreno + mix) e as métricas de
// performance (OpMetric — embrião do Performance Memory) em JSONL.
func NewNodeGraphDemoProceduralCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "demo-procedural",
		Short: "Render the Procedural Kernel System demo (noise+math graph → PNG)",
		Long: `Renderiza o Procedural Kernel System (ordem do Professor, L303) e prova
(P13) que o contrato fecha: grafo serializável + procgen.Registry() +
Render Engine (§22) + métricas de performance.

Grafo:
  fbm(terreno, seed=42, octaves=5) → math_clamp [0,1] → math_lerp(t=0.5)
                                                       ↑ gradient

Saída em .cosca/procgen-demo/: terrain.png, procedural-mix.png e
metrics.jsonl (JSONL de OpMetric, append-only).`,
		Example: `  cosca ngraph demo-procedural`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

			outDir := filepath.Join(".cosca", "procgen-demo")
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return fmt.Errorf("procgen demo: mkdir: %w", err)
			}

			const size = 256
			g := nodegraph.New("procgen-demo")
			terrain := &nodegraph.Node{
				ID: "terrain", Type: procgen.NodeFbm,
				Params: map[string]any{
					"seed": int64(42), "octaves": int64(5), "persistence": 0.5,
					"frequency": 3.0, "width": size, "height": size,
				},
			}
			clamp := &nodegraph.Node{
				ID: "clamp", Type: procgen.NodeMathClamp,
				Params: map[string]any{"min": 0.0, "max": 1.0},
				Inputs: []string{"terrain"},
			}
			gradient := &nodegraph.Node{
				ID: "gradient", Type: procgen.NodeGradient,
				Params: map[string]any{"width": size, "height": size},
			}
			mix := &nodegraph.Node{
				ID: "mix", Type: procgen.NodeMathLerp,
				Params: map[string]any{"t": 0.5},
				Inputs: []string{"clamp", "gradient"},
			}
			for _, n := range []*nodegraph.Node{terrain, clamp, gradient, mix} {
				if err := g.AddNode(n); err != nil {
					return fmt.Errorf("procgen demo: add node %q: %w", n.ID, err)
				}
			}

			metrics := &procgen.MetricsRecorder{}
			exec := newDispatchExecutor(metrics)

			job, err := render.New(g, render.Draft, 42)
			if err != nil {
				return fmt.Errorf("procgen demo: %w", err)
			}

			res, err := render.NewRenderer().Render(cmd.Context(), job, exec)
			if err != nil {
				return fmt.Errorf("procgen demo: render: %w", err)
			}

			writePNG := func(name string, v any) error {
				img, ok := v.(*procgen.RGBA)
				if !ok {
					return fmt.Errorf("procgen demo: node %q result is %T, want *procgen.RGBA", name, v)
				}
				p := filepath.Join(outDir, name+".png")
				if err := img.WritePNG(p); err != nil {
					return fmt.Errorf("procgen demo: write %s: %w", p, err)
				}
				return nil
			}
			if err := writePNG("terrain", res.Results["terrain"]); err != nil {
				return err
			}
			if err := writePNG("procedural-mix", res.Results["mix"]); err != nil {
				return err
			}

			metricsPath := filepath.Join(outDir, "metrics.jsonl")
			if err := metrics.Dump(metricsPath); err != nil {
				return fmt.Errorf("procgen demo: metrics dump: %w", err)
			}

			formatter.Success(fmt.Sprintf("Procedural Kernel demo rendered (%d nodes, seed=42)", g.Len()))
			formatter.KeyValue("Executed", fmt.Sprint(res.Executed))
			formatter.KeyValue("Cache Hits", fmt.Sprint(res.CachedHits))
			formatter.KeyValue("Duration", fmt.Sprintf("%d ms", res.DurationMs))
			formatter.Bullet(filepath.Join(outDir, "terrain.png"))
			formatter.Bullet(filepath.Join(outDir, "procedural-mix.png"))
			formatter.Bullet(metricsPath)
			return nil
		},
	}
	return cmd
}
