package procgen

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/CoscaAI/cosca/internal/nodegraph"
	"github.com/CoscaAI/cosca/internal/render"
)

// Nós do Procedural Kernel System (famílias Noise/Math/Pattern). Todos
// implementam nodegraph.Executor. Cada nó devolve *RGBA (imagem própria do
// engine); nós de Math recebem 1-2 *RGBA via input["a"]/input["b"].
//
// Resolução de parâmetros — DOIS caminhos, ambos suportados e documentados:
//  1. node.Params (grafo JSON) quando presentes: "seed", "width", "height",
//     "octaves", "persistence", "lacunarity", "gain", "frequency", "scale",
//     "base", "min"/"max", "t", "in_min"/"in_max"/"out_min"/"out_max".
//  2. Campos da struct (ex: PerlinNode{Seed, Width, Height}) como defaults.
//
// Para "seed", há ainda um terceiro fallback: o seed do render context (§22)
// injetado pelo Render Engine — assim o determinismo do Job vale para nós que
// não declaram seed no grafo.

// Tipos de nó registrados no Registry().
const (
	NodePerlin      nodegraph.NodeType = "perlin"
	NodeSimplex     nodegraph.NodeType = "simplex"
	NodeWorley      nodegraph.NodeType = "worley"
	NodeVoronoi     nodegraph.NodeType = "voronoi"
	NodeFbm         nodegraph.NodeType = "fbm"
	NodeGradient    nodegraph.NodeType = "gradient"
	NodeChecker     nodegraph.NodeType = "checker"
	NodeStripes     nodegraph.NodeType = "stripes"
	NodeCells       nodegraph.NodeType = "cells"
	NodeRings       nodegraph.NodeType = "rings"
	NodeMathAdd     nodegraph.NodeType = "math_add"
	NodeMathMultiply nodegraph.NodeType = "math_multiply"
	NodeMathRemap   nodegraph.NodeType = "math_remap"
	NodeMathClamp   nodegraph.NodeType = "math_clamp"
	NodeMathLerp    nodegraph.NodeType = "math_lerp"
	NodeMathCurve   nodegraph.NodeType = "math_curve"
)

// Registry mapeia tipo de nó → executor procedural (famílias Noise, Pattern e
// Math). O CLI usa este mapa como registry de executores; o MetricsRecorder é
// opcional — use MetricsSetter para injetar um recorder compartilhado.
func Registry() map[nodegraph.NodeType]nodegraph.Executor {
	return map[nodegraph.NodeType]nodegraph.Executor{
		NodePerlin:       &PerlinNode{},
		NodeSimplex:      &SimplexNode{},
		NodeWorley:       &WorleyNode{},
		NodeVoronoi:      &VoronoiNode{},
		NodeFbm:          &FbmNode{},
		NodeGradient:     &GradientNode{},
		NodeChecker:      &CheckerNode{},
		NodeStripes:      &StripesNode{},
		NodeCells:        &CellsNode{},
		NodeRings:        &RingsNode{},
		NodeMathAdd:      &AddNode{},
		NodeMathMultiply: &MultiplyNode{},
		NodeMathRemap:    &RemapNode{},
		NodeMathClamp:    &ClampNode{},
		NodeMathLerp:     &LerpNode{},
		NodeMathCurve:    &CurveNode{},
	}
}

// MetricsSetter injeta um MetricsRecorder compartilhado nos executores.
type MetricsSetter interface {
	SetMetrics(*MetricsRecorder)
}

// metricHolder carrega o recorder opcional de um nó.
type metricHolder struct {
	Metrics *MetricsRecorder
}

// SetMetrics implementa MetricsSetter.
func (m *metricHolder) SetMetrics(r *MetricsRecorder) { m.Metrics = r }

// =============================================================================
// Helpers de resolução de parâmetros (grafo JSON → valores tipados)
// =============================================================================

// numParam devolve o valor numérico de params[key] (aceita float64 do JSON,
// int/int64 de grafos programáticos e json.Number).
func numParam(params map[string]any, key string) (float64, bool) {
	if params == nil {
		return 0, false
	}
	switch v := params[key].(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f, true
		}
	}
	return 0, false
}

func intParam(params map[string]any, key string, def int) int {
	if v, ok := numParam(params, key); ok {
		return int(v)
	}
	return def
}

func int64Param(params map[string]any, key string, def int64) int64 {
	if v, ok := numParam(params, key); ok {
		return int64(v)
	}
	return def
}

func floatParam(params map[string]any, key string, def float64) float64 {
	if v, ok := numParam(params, key); ok {
		return v
	}
	return def
}

func strParam(params map[string]any, key, def string) string {
	if params == nil {
		return def
	}
	if s, ok := params[key].(string); ok {
		return s
	}
	return def
}

// positive devolve v quando > 0, senão def (default seguro).
func positive(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

func positiveF(v, def float64) float64 {
	if v <= 0 {
		return def
	}
	return v
}

// seedFor resolve o seed na ordem: params["seed"] > struct field > render ctx.
func seedFor(ctx context.Context, params map[string]any, field int64) int64 {
	if v, ok := numParam(params, "seed"); ok {
		return int64(v)
	}
	if rp := render.RenderFromContext(ctx); rp.Seed != 0 {
		return rp.Seed
	}
	return field
}

// recordMetric registra a OpMetric de uma operação (embrião do Performance
// Memory). CacheHit fica false aqui: o cache por assinatura (§23) é decisão
// do grafo — o Stats de graph/render carrega os CachedHits reais.
func recordMetric(m *MetricsRecorder, operation string, w, h int, start time.Time) {
	if m == nil {
		return
	}
	m.Record(OpMetric{
		Operation: operation,
		Width:     w,
		Height:    h,
		Backend:   "cpu-go",
		LatencyMs: float64(time.Since(start).Microseconds()) / 1000.0,
		MemoryKB:  int64(w*h*4) / 1024,
		CacheHit:  false,
		Quality:   1.0,
	})
}

// noiseImage aplica uma função de noise por pixel (com frequência freq sobre
// as coords normalizadas) e devolve a imagem. Se remap, [-1,1]→[0,1].
func noiseImage(rng *RNG, w, h int, freq float64, remap bool, f func(rng *RNG, x, y float64) float64) *RGBA {
	img := NewRGBA(w, h)
	img.Map(func(x, y int, nx, ny float64) [3]float64 {
		v := f(rng, nx*freq, ny*freq)
		if remap {
			v = (v + 1) * 0.5
		}
		v = Clamp(v, 0, 1)
		return [3]float64{v, v, v}
	})
	return img
}

// =============================================================================
// Família NOISE
// =============================================================================

// PerlinNode gera uma imagem de Perlin noise (ou FBM-Perlin quando
// octaves > 1). Params: seed, width, height, frequency, octaves, persistence.
type PerlinNode struct {
	metricHolder
	Seed        int64
	Width       int
	Height      int
	Octaves     int
	Persistence float64
	Frequency   float64
}

// Run implementa nodegraph.Executor.
func (n *PerlinNode) Run(ctx context.Context, node *nodegraph.Node, _ map[string]any) (any, error) {
	start := time.Now()
	params := node.Params
	seed := seedFor(ctx, params, n.Seed)
	w := positive(intParam(params, "width", n.Width), 256)
	h := positive(intParam(params, "height", n.Height), 256)
	freq := positiveF(floatParam(params, "frequency", n.Frequency), 4)
	octaves := positive(intParam(params, "octaves", n.Octaves), 1)
	persistence := positiveF(floatParam(params, "persistence", n.Persistence), 0.5)
	rng := NewRNG(seed)
	var img *RGBA
	if octaves > 1 {
		img = noiseImage(rng, w, h, freq, true, func(r *RNG, x, y float64) float64 {
			return FbmPerlin(r, x, y, octaves, persistence)
		})
	} else {
		img = noiseImage(rng, w, h, freq, true, Perlin2D)
	}
	recordMetric(n.Metrics, "perlin", w, h, start)
	return img, nil
}

// SimplexNode gera uma imagem de Simplex noise (ou FBM-Simplex quando
// octaves > 1). Params: seed, width, height, frequency, octaves, persistence.
type SimplexNode struct {
	metricHolder
	Seed        int64
	Width       int
	Height      int
	Octaves     int
	Persistence float64
	Frequency   float64
}

// Run implementa nodegraph.Executor.
func (n *SimplexNode) Run(ctx context.Context, node *nodegraph.Node, _ map[string]any) (any, error) {
	start := time.Now()
	params := node.Params
	seed := seedFor(ctx, params, n.Seed)
	w := positive(intParam(params, "width", n.Width), 256)
	h := positive(intParam(params, "height", n.Height), 256)
	freq := positiveF(floatParam(params, "frequency", n.Frequency), 4)
	octaves := positive(intParam(params, "octaves", n.Octaves), 1)
	persistence := positiveF(floatParam(params, "persistence", n.Persistence), 0.5)
	rng := NewRNG(seed)
	var img *RGBA
	if octaves > 1 {
		img = noiseImage(rng, w, h, freq, true, func(r *RNG, x, y float64) float64 {
			return FbmSimplex(r, x, y, octaves, persistence)
		})
	} else {
		img = noiseImage(rng, w, h, freq, true, Simplex2D)
	}
	recordMetric(n.Metrics, "simplex", w, h, start)
	return img, nil
}

// WorleyNode gera uma imagem de Worley noise (F1). Params: seed, width,
// height, frequency.
type WorleyNode struct {
	metricHolder
	Seed      int64
	Width     int
	Height    int
	Frequency float64
}

// Run implementa nodegraph.Executor.
func (n *WorleyNode) Run(ctx context.Context, node *nodegraph.Node, _ map[string]any) (any, error) {
	start := time.Now()
	params := node.Params
	seed := seedFor(ctx, params, n.Seed)
	w := positive(intParam(params, "width", n.Width), 256)
	h := positive(intParam(params, "height", n.Height), 256)
	freq := positiveF(floatParam(params, "frequency", n.Frequency), 4)
	rng := NewRNG(seed)
	img := noiseImage(rng, w, h, freq, false, Worley2D)
	recordMetric(n.Metrics, "worley", w, h, start)
	return img, nil
}

// VoronoiNode gera uma imagem de Voronoi cell borders (F2-F1). Params: seed,
// width, height, frequency.
type VoronoiNode struct {
	metricHolder
	Seed      int64
	Width     int
	Height    int
	Frequency float64
}

// Run implementa nodegraph.Executor.
func (n *VoronoiNode) Run(ctx context.Context, node *nodegraph.Node, _ map[string]any) (any, error) {
	start := time.Now()
	params := node.Params
	seed := seedFor(ctx, params, n.Seed)
	w := positive(intParam(params, "width", n.Width), 256)
	h := positive(intParam(params, "height", n.Height), 256)
	freq := positiveF(floatParam(params, "frequency", n.Frequency), 4)
	rng := NewRNG(seed)
	img := noiseImage(rng, w, h, freq, false, Voronoi2D)
	recordMetric(n.Metrics, "voronoi", w, h, start)
	return img, nil
}

// FbmNode gera uma imagem de fractal brownian motion sobre Perlin ou Simplex
// (params["base"] = "perlin"|"simplex"). Params: seed, width, height,
// frequency, octaves, persistence, lacunarity, gain, base.
type FbmNode struct {
	metricHolder
	Seed        int64
	Width       int
	Height      int
	Octaves     int
	Persistence float64
	Lacunarity  float64
	Gain        float64
	Frequency   float64
	Base        string
}

// Run implementa nodegraph.Executor.
func (n *FbmNode) Run(ctx context.Context, node *nodegraph.Node, _ map[string]any) (any, error) {
	start := time.Now()
	params := node.Params
	seed := seedFor(ctx, params, n.Seed)
	w := positive(intParam(params, "width", n.Width), 256)
	h := positive(intParam(params, "height", n.Height), 256)
	freq := positiveF(floatParam(params, "frequency", n.Frequency), 4)
	octaves := positive(intParam(params, "octaves", n.Octaves), 5)
	persistence := positiveF(floatParam(params, "persistence", n.Persistence), 0.5)
	lacunarity := positiveF(floatParam(params, "lacunarity", n.Lacunarity), 2.0)
	gain := positiveF(floatParam(params, "gain", n.Gain), persistence)
	base := strParam(params, "base", n.Base)
	rng := NewRNG(seed)
	img := noiseImage(rng, w, h, freq, true, func(r *RNG, x, y float64) float64 {
		switch base {
		case "simplex":
			return FBM2D(r, x, y, octaves, lacunarity, gain, persistence, Simplex2D)
		default:
			return FBM2D(r, x, y, octaves, lacunarity, gain, persistence, Perlin2D)
		}
	})
	recordMetric(n.Metrics, "fbm", w, h, start)
	return img, nil
}

// =============================================================================
// Família PATTERN
// =============================================================================

// GradientNode — gradiente diagonal (x+y)/2. Params: width, height.
type GradientNode struct {
	metricHolder
	Width  int
	Height int
}

// Run implementa nodegraph.Executor.
func (n *GradientNode) Run(_ context.Context, node *nodegraph.Node, _ map[string]any) (any, error) {
	start := time.Now()
	params := node.Params
	w := positive(intParam(params, "width", n.Width), 256)
	h := positive(intParam(params, "height", n.Height), 256)
	img := NewRGBA(w, h)
	img.Map(func(x, y int, nx, ny float64) [3]float64 {
		v := Gradient(nx, ny)
		return [3]float64{v, v, v}
	})
	recordMetric(n.Metrics, "gradient", w, h, start)
	return img, nil
}

// CheckerNode — xadrez. Params: width, height, scale.
type CheckerNode struct {
	metricHolder
	Width  int
	Height int
	Scale  int
}

// Run implementa nodegraph.Executor.
func (n *CheckerNode) Run(_ context.Context, node *nodegraph.Node, _ map[string]any) (any, error) {
	start := time.Now()
	params := node.Params
	w := positive(intParam(params, "width", n.Width), 256)
	h := positive(intParam(params, "height", n.Height), 256)
	scale := positive(intParam(params, "scale", n.Scale), 8)
	img := NewRGBA(w, h)
	img.Map(func(x, y int, nx, ny float64) [3]float64 {
		v := Checker(nx, ny, scale)
		return [3]float64{v, v, v}
	})
	recordMetric(n.Metrics, "checker", w, h, start)
	return img, nil
}

// StripesNode — listras verticais. Params: width, height, scale.
type StripesNode struct {
	metricHolder
	Width  int
	Height int
	Scale  int
}

// Run implementa nodegraph.Executor.
func (n *StripesNode) Run(_ context.Context, node *nodegraph.Node, _ map[string]any) (any, error) {
	start := time.Now()
	params := node.Params
	w := positive(intParam(params, "width", n.Width), 256)
	h := positive(intParam(params, "height", n.Height), 256)
	scale := positive(intParam(params, "scale", n.Scale), 12)
	img := NewRGBA(w, h)
	img.Map(func(x, y int, nx, ny float64) [3]float64 {
		v := Stripes(nx, ny, scale)
		return [3]float64{v, v, v}
	})
	recordMetric(n.Metrics, "stripes", w, h, start)
	return img, nil
}

// CellsNode — células Voronoi com valor aleatório por célula. Params: seed,
// width, height, scale.
type CellsNode struct {
	metricHolder
	Seed   int64
	Width  int
	Height int
	Scale  int
}

// Run implementa nodegraph.Executor.
func (n *CellsNode) Run(ctx context.Context, node *nodegraph.Node, _ map[string]any) (any, error) {
	start := time.Now()
	params := node.Params
	seed := seedFor(ctx, params, n.Seed)
	w := positive(intParam(params, "width", n.Width), 256)
	h := positive(intParam(params, "height", n.Height), 256)
	scale := positive(intParam(params, "scale", n.Scale), 6)
	rng := NewRNG(seed)
	img := NewRGBA(w, h)
	img.Map(func(x, y int, nx, ny float64) [3]float64 {
		v := Cells(nx, ny, rng, scale)
		return [3]float64{v, v, v}
	})
	recordMetric(n.Metrics, "cells", w, h, start)
	return img, nil
}

// RingsNode — anéis concêntricos. Params: width, height, scale.
type RingsNode struct {
	metricHolder
	Width  int
	Height int
	Scale  float64
}

// Run implementa nodegraph.Executor.
func (n *RingsNode) Run(_ context.Context, node *nodegraph.Node, _ map[string]any) (any, error) {
	start := time.Now()
	params := node.Params
	w := positive(intParam(params, "width", n.Width), 256)
	h := positive(intParam(params, "height", n.Height), 256)
	scale := positiveF(floatParam(params, "scale", n.Scale), 24)
	img := NewRGBA(w, h)
	img.Map(func(x, y int, nx, ny float64) [3]float64 {
		v := Rings(nx, ny, scale)
		return [3]float64{v, v, v}
	})
	recordMetric(n.Metrics, "rings", w, h, start)
	return img, nil
}

// =============================================================================
// Família MATH (como nós de imagem)
// =============================================================================

// resolveInput devolve o *RGBA de um input de nó de Math. DOIS caminhos
// (documentado): primeiro a chave literal "a"/"b" (contrato programático do
// executor, como no spec do kernel); se ausente, a posição no grafo
// (node.Inputs[0] → a, node.Inputs[1] → b — o nodegraph.Run chaveia o mapa de
// inputs por ID do nó). Fail-closed: erro explícito se ausente ou tipo errado.
func resolveInput(node *nodegraph.Node, input map[string]any, key string, pos int) (*RGBA, error) {
	if v, ok := input[key]; ok {
		return asRGBA(node, key, v)
	}
	if len(node.Inputs) > pos {
		id := node.Inputs[pos]
		v, ok := input[id]
		if !ok {
			return nil, fmt.Errorf("procgen: node %q (%s): input %q (position %d) has no output", node.ID, node.Type, id, pos)
		}
		return asRGBA(node, id, v)
	}
	return nil, fmt.Errorf("procgen: node %q (%s): missing input %q (position %d)", node.ID, node.Type, key, pos)
}

// asRGBA valida o tipo de um valor de input.
func asRGBA(node *nodegraph.Node, key string, v any) (*RGBA, error) {
	img, ok := v.(*RGBA)
	if !ok {
		return nil, fmt.Errorf("procgen: node %q (%s): input %q is %T, want *procgen.RGBA", node.ID, node.Type, key, v)
	}
	return img, nil
}

// requirePair valida que a e b têm as mesmas dimensões (fail-closed).
func requirePair(a, b *RGBA) error {
	if a.W != b.W || a.H != b.H {
		return fmt.Errorf("procgen: input images differ in size (%dx%d vs %dx%d)", a.W, a.H, b.W, b.H)
	}
	return nil
}

// grayOf devolve o valor normalizado [0,1] do canal R do pixel idx.
func grayOf(im *RGBA, idx int) float64 {
	return float64(im.Pix[idx]) / 255.0
}

// setGray grava o valor normalizado [0,1] como cinza (R=G=B, A=255).
func setGray(im *RGBA, idx int, v float64) {
	c := uint8(Clamp(v, 0, 1) * 255)
	im.Pix[idx+0] = c
	im.Pix[idx+1] = c
	im.Pix[idx+2] = c
	im.Pix[idx+3] = 255
}

// apply1 aplica op a cada pixel de uma imagem.
func apply1(a *RGBA, op func(v float64) float64) *RGBA {
	out := NewRGBA(a.W, a.H)
	for i := 0; i < len(a.Pix); i += 4 {
		setGray(out, i, op(grayOf(a, i)))
	}
	return out
}

// apply2 aplica op aos pixels de duas imagens (mesmas dimensões garantidas
// pelo chamador).
func apply2(a, b *RGBA, op func(x, y float64) float64) *RGBA {
	out := NewRGBA(a.W, a.H)
	for i := 0; i < len(a.Pix); i += 4 {
		setGray(out, i, op(grayOf(a, i), grayOf(b, i)))
	}
	return out
}

// AddNode — soma duas imagens (a + b, clamp [0,1]). Inputs: a, b.
type AddNode struct {
	metricHolder
}

// Run implementa nodegraph.Executor.
func (n *AddNode) Run(_ context.Context, node *nodegraph.Node, input map[string]any) (any, error) {
	start := time.Now()
	a, err := resolveInput(node, input, "a", 0)
	if err != nil {
		return nil, err
	}
	b, err := resolveInput(node, input, "b", 1)
	if err != nil {
		return nil, err
	}
	if err := requirePair(a, b); err != nil {
		return nil, err
	}
	out := apply2(a, b, Add)
	recordMetric(n.Metrics, "math_add", a.W, a.H, start)
	return out, nil
}

// MultiplyNode — multiplica duas imagens (a * b, clamp [0,1]). Inputs: a, b.
type MultiplyNode struct {
	metricHolder
}

// Run implementa nodegraph.Executor.
func (n *MultiplyNode) Run(_ context.Context, node *nodegraph.Node, input map[string]any) (any, error) {
	start := time.Now()
	a, err := resolveInput(node, input, "a", 0)
	if err != nil {
		return nil, err
	}
	b, err := resolveInput(node, input, "b", 1)
	if err != nil {
		return nil, err
	}
	if err := requirePair(a, b); err != nil {
		return nil, err
	}
	out := apply2(a, b, Multiply)
	recordMetric(n.Metrics, "math_multiply", a.W, a.H, start)
	return out, nil
}

// RemapNode — re-mapeia os valores de a. Params: in_min, in_max, out_min,
// out_max. Input: a.
type RemapNode struct {
	metricHolder
}

// Run implementa nodegraph.Executor.
func (n *RemapNode) Run(_ context.Context, node *nodegraph.Node, input map[string]any) (any, error) {
	start := time.Now()
	a, err := resolveInput(node, input, "a", 0)
	if err != nil {
		return nil, err
	}
	params := node.Params
	inMin := floatParam(params, "in_min", 0)
	inMax := floatParam(params, "in_max", 1)
	outMin := floatParam(params, "out_min", 0)
	outMax := floatParam(params, "out_max", 1)
	out := apply1(a, func(v float64) float64 {
		return Remap(v, inMin, inMax, outMin, outMax)
	})
	recordMetric(n.Metrics, "math_remap", a.W, a.H, start)
	return out, nil
}

// ClampNode — restringe os valores de a a [min,max]. Params: min, max.
// Input: a.
type ClampNode struct {
	metricHolder
}

// Run implementa nodegraph.Executor.
func (n *ClampNode) Run(_ context.Context, node *nodegraph.Node, input map[string]any) (any, error) {
	start := time.Now()
	a, err := resolveInput(node, input, "a", 0)
	if err != nil {
		return nil, err
	}
	params := node.Params
	min := floatParam(params, "min", 0)
	max := floatParam(params, "max", 1)
	out := apply1(a, func(v float64) float64 {
		return Clamp(v, min, max)
	})
	recordMetric(n.Metrics, "math_clamp", a.W, a.H, start)
	return out, nil
}

// LerpNode — mixa duas imagens com t. Inputs: a, b. Params: t (default 0.5).
type LerpNode struct {
	metricHolder
}

// Run implementa nodegraph.Executor.
func (n *LerpNode) Run(_ context.Context, node *nodegraph.Node, input map[string]any) (any, error) {
	start := time.Now()
	a, err := resolveInput(node, input, "a", 0)
	if err != nil {
		return nil, err
	}
	b, err := resolveInput(node, input, "b", 1)
	if err != nil {
		return nil, err
	}
	if err := requirePair(a, b); err != nil {
		return nil, err
	}
	t := floatParam(node.Params, "t", 0.5)
	out := apply2(a, b, func(x, y float64) float64 {
		return Lerp(x, y, t)
	})
	recordMetric(n.Metrics, "math_lerp", a.W, a.H, start)
	return out, nil
}

// CurveNode — aplica smoothstep (Curve) aos valores de a. Input: a.
type CurveNode struct {
	metricHolder
}

// Run implementa nodegraph.Executor.
func (n *CurveNode) Run(_ context.Context, node *nodegraph.Node, input map[string]any) (any, error) {
	start := time.Now()
	a, err := resolveInput(node, input, "a", 0)
	if err != nil {
		return nil, err
	}
	out := apply1(a, func(v float64) float64 {
		return Curve(Clamp(v, 0, 1))
	})
	recordMetric(n.Metrics, "math_curve", a.W, a.H, start)
	return out, nil
}
