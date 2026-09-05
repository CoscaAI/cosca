package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/procgen"
	"github.com/CoscaAI/cosca/internal/render"
	"github.com/CoscaAI/cosca/internal/scene"
)

// NewNodeGraphDemoSceneCommand — PROVA de funcionamento (P13) do Scene Graph
// (ordem do Professor, L303): a cena é montada com ENTIDADES ESPACIAIS
// (Terrain + Group de procedurais + Camera) e a ponte Compile transforma as
// entidades em nós executáveis do node graph.
//
// Fluxo (a prova do conceito "entidade APONTA para operação"):
//
//	Scene → Compile → Graph → Run
//
// Cena: Terrain(fbm, seed=42, 256×256) + Group "Vegetation" com 2 entidades
// procedurais (perlin e worley) + Camera (espacial pura — não vira nó).
// Compila → grafo → executa pelo Render Engine (seed 42) → PNGs em
// .cosca/scene-demo/.
func NewNodeGraphDemoSceneCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "demo-scene",
		Short: "Scene Graph demo: entities → compiled node graph → PNGs",
		Long: `Renderiza o Scene Graph (fase 2 da ordem do Professor, L303) e prova
(P13) o contrato fechado: entidades espaciais APONTAM para operações do node
graph (NodeRef) e o Compile transforma a cena em um grafo executável.

Cena:
  Terrain(fbm, seed=42, 256×256)
  Group "Vegetation"
    ├─ perlin (128×128, seed=7)
    └─ worley (128×128, seed=11)
  Camera (espacial pura — não vira nó)

Fluxo: Scene → Compile → Graph → Run (Render Engine, seed 42). Saída em
.cosca/scene-demo/: terrain.png, vegetation-perlin.png, vegetation-worley.png
e metrics.jsonl (OpMetric, append-only).`,
		Example: `  cosca ngraph demo-scene`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

			outDir := filepath.Join(".cosca", "scene-demo")
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return fmt.Errorf("scene demo: mkdir: %w", err)
			}

			// 1. Cena: grafo de ENTIDADES ESPACIAIS.
			s := scene.NewScene("scene-demo")
			terrain := scene.NewTerrain("terrain", 256, 256, 1.0, "fbm", 42)
			veg := scene.NewGroup("vegetation")
			perlin := scene.NewProceduralEntity("vegetation_perlin", "perlin", 7, map[string]any{
				"width": 128, "height": 128, "octaves": int64(4),
			})
			worley := scene.NewProceduralEntity("vegetation_worley", "worley", 11, map[string]any{
				"width": 128, "height": 128,
			})
			if err := veg.AddChild(perlin); err != nil {
				return fmt.Errorf("scene demo: %w", err)
			}
			if err := veg.AddChild(worley); err != nil {
				return fmt.Errorf("scene demo: %w", err)
			}
			camera := scene.NewCamera("camera", scene.Vec3{X: 0, Y: 10, Z: 20}, scene.Vec3{X: 0, Y: 0, Z: 0})
			for _, e := range []*scene.Entity{terrain, veg, camera} {
				if err := s.AddRoot(e); err != nil {
					return fmt.Errorf("scene demo: add root %q: %w", e.ID, err)
				}
			}
			if err := s.Validate(); err != nil {
				return fmt.Errorf("scene demo: validate scene: %w", err)
			}

			// 2. Ponte: entidade → nó (Scene → Compile → Graph).
			compiled, err := scene.Compile(s, nil, procgen.Registry())
			if err != nil {
				return fmt.Errorf("scene demo: compile: %w", err)
			}
			compiledCount := countCompiled(s)

			// 3. Executa pelo Render Engine (determinismo §22, seed 42).
			metrics := &procgen.MetricsRecorder{}
			exec := newDispatchExecutor(metrics)
			job, err := render.New(compiled, render.Draft, 42)
			if err != nil {
				return fmt.Errorf("scene demo: %w", err)
			}
			res, err := render.NewRenderer().Render(cmd.Context(), job, exec)
			if err != nil {
				return fmt.Errorf("scene demo: render: %w", err)
			}

			// 4. Prova (P13): PNGs gerados.
			writePNG := func(name string, v any) error {
				img, ok := v.(*procgen.RGBA)
				if !ok {
					return fmt.Errorf("scene demo: node %q result is %T, want *procgen.RGBA", name, v)
				}
				p := filepath.Join(outDir, name+".png")
				if err := img.WritePNG(p); err != nil {
					return fmt.Errorf("scene demo: write %s: %w", p, err)
				}
				return nil
			}
			for pngName, nodeID := range map[string]string{
				"terrain":           "terrain",
				"vegetation-perlin": "vegetation_perlin",
				"vegetation-worley": "vegetation_worley",
			} {
				if err := writePNG(pngName, res.Results[nodeID]); err != nil {
					return err
				}
			}

			metricsPath := filepath.Join(outDir, "metrics.jsonl")
			if err := metrics.Dump(metricsPath); err != nil {
				return fmt.Errorf("scene demo: metrics dump: %w", err)
			}

			formatter.Success(fmt.Sprintf("Scene Graph demo rendered (%d entities compiled, seed=42)", compiledCount))
			formatter.KeyValue("Compiled Nodes", fmt.Sprint(compiled.Len()))
			formatter.KeyValue("Executed", fmt.Sprint(res.Executed))
			formatter.KeyValue("Cache Hits", fmt.Sprint(res.CachedHits))
			formatter.KeyValue("Duration", fmt.Sprintf("%d ms", res.DurationMs))
			formatter.Bullet(filepath.Join(outDir, "terrain.png"))
			formatter.Bullet(filepath.Join(outDir, "vegetation-perlin.png"))
			formatter.Bullet(filepath.Join(outDir, "vegetation-worley.png"))
			formatter.Bullet(metricsPath)
			return nil
		},
	}
	return cmd
}

// countCompiled conta as entidades da cena que apontam para computação
// (NodeRef não vazio) — o número de nós que o Compile deve gerar.
func countCompiled(s *scene.Scene) int {
	var walk func(e *scene.Entity) int
	walk = func(e *scene.Entity) int {
		if e == nil {
			return 0
		}
		n := 0
		if e.NodeRef != "" {
			n++
		}
		for _, c := range e.Children {
			n += walk(c)
		}
		return n
	}
	return walk(s.Root)
}
