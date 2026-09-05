package procgen

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/nodegraph"
)

// TestRNGDeterminism — P1: mesmo seed = mesma sequência (splitmix64, inteiro,
// portátil byte a byte); seeds diferentes → sequências diferentes.
func TestRNGDeterminism(t *testing.T) {
	a := NewRNG(42)
	b := NewRNG(42)
	var av, bv [100]uint64
	for i := 0; i < 100; i++ {
		av[i] = a.Uint64()
		bv[i] = b.Uint64()
	}
	if av != bv {
		t.Fatalf("same seed produced different sequences: %v vs %v", av[:4], bv[:4])
	}

	if c := NewRNG(43).Uint64(); c == av[0] {
		t.Fatal("different seeds produced the same first value")
	}

	r := NewRNG(7)
	for i := 0; i < 1000; i++ {
		if f := r.Float64(); f < 0 || f >= 1 {
			t.Fatalf("Float64 out of [0,1): %v", f)
		}
		if n := r.Intn(10); n < 0 || n >= 10 {
			t.Fatalf("Intn out of [0,10): %v", n)
		}
		if v := r.Range(2, 5); v < 2 || v > 5 {
			t.Fatalf("Range out of [2,5]: %v", v)
		}
	}
	if r.Intn(0) != 0 {
		t.Fatal("Intn(0) should be 0 (fail-closed)")
	}
}

// TestNoiseRange — cada função dentro do range documentado.
func TestNoiseRange(t *testing.T) {
	rng := NewRNG(1)
	for i := 0; i < 2000; i++ {
		x := rng.Range(-50, 50)
		y := rng.Range(-50, 50)
		if p := Perlin2D(rng, x, y); p < -1 || p > 1 {
			t.Fatalf("Perlin2D out of [-1,1]: %v", p)
		}
		if s := Simplex2D(rng, x, y); s < -1 || s > 1 {
			t.Fatalf("Simplex2D out of [-1,1]: %v", s)
		}
		if w := Worley2D(rng, x, y); w < 0 || w > 1.15 {
			t.Fatalf("Worley2D out of [0,~1]: %v", w)
		}
		if v := Voronoi2D(rng, x, y); v < 0 || v > 1.15 {
			t.Fatalf("Voronoi2D out of [0,~1]: %v", v)
		}
	}
}

// TestFBM — 1 octave ≈ base; mais octaves muda o valor.
func TestFBM(t *testing.T) {
	fresh := func(seed int64) *RNG { return NewRNG(seed) }

	base := Perlin2D(fresh(42), 3.7, 1.9)
	fbm1 := FbmPerlin(fresh(42), 3.7, 1.9, 1, 0.5)
	if base != fbm1 {
		t.Fatalf("FbmPerlin with 1 octave should equal Perlin2D: got %v, want %v", fbm1, base)
	}

	changed := false
	for i := 0; i < 20; i++ {
		x := 0.5 + float64(i)*0.37
		y := 0.5 + float64(i)*0.91
		if FbmPerlin(fresh(42), x, y, 4, 0.5) != FbmPerlin(fresh(42), x, y, 1, 0.5) {
			changed = true
			break
		}
	}
	if !changed {
		t.Fatal("more octaves should change the value (FBM)")
	}
}

// TestImageRoundtrip — Set/Get, PPM header, PNG válido re-lido com image/png.
func TestImageRoundtrip(t *testing.T) {
	im := NewRGBA(8, 4)
	if len(im.Pix) != 8*4*4 {
		t.Fatalf("pixel buffer: got %d, want %d", len(im.Pix), 8*4*4)
	}
	im.Set(1, 2, 10, 20, 30, 40)
	if r, g, b, a := im.Get(1, 2); r != 10 || g != 20 || b != 30 || a != 40 {
		t.Fatalf("Set/Get roundtrip failed: %d %d %d %d", r, g, b, a)
	}
	im.Set(-1, 0, 255, 0, 0, 255) // out of bounds: no-op
	if r, _, _, _ := im.Get(-1, 0); r != 0 {
		t.Fatal("out-of-bounds Get should return 0")
	}

	// Map preenche com valores normalizados.
	m := NewRGBA(2, 2)
	m.Map(func(x, y int, nx, ny float64) [3]float64 { return [3]float64{nx, ny, 1} })
	if r, g, b, a := m.Get(0, 0); r != 0 || g != 0 || b != 255 || a != 255 {
		t.Fatalf("Map(0,0) = %d %d %d %d", r, g, b, a)
	}
	if r, g, b, _ := m.Get(1, 1); r != 255 || g != 255 || b != 255 {
		t.Fatalf("Map(1,1) = %d %d %d", r, g, b)
	}

	// Blit com alpha blending (src over dst).
	dst := NewRGBA(2, 2)
	dst.Set(0, 0, 255, 0, 0, 255)
	src := NewRGBA(1, 1)
	src.Set(0, 0, 0, 255, 0, 128) // alpha ~0.5
	dst.Blit(dst, src, 0, 0)
	if r, _, _, _ := dst.Get(0, 0); r != 127 {
		t.Fatalf("Blit alpha blend: got %d, want 127", r)
	}

	dir := t.TempDir()

	ppmPath := filepath.Join(dir, "out.ppm")
	if err := im.WritePPM(ppmPath); err != nil {
		t.Fatalf("WritePPM: %v", err)
	}
	data, err := os.ReadFile(ppmPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("P6\n")) {
		t.Fatalf("PPM must start with \"P6\\n\": %q", data[:4])
	}

	pngPath := filepath.Join(dir, "out.png")
	if err := im.WritePNG(pngPath); err != nil {
		t.Fatalf("WritePNG: %v", err)
	}
	f, err := os.Open(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	decoded, err := png.Decode(f)
	if err != nil {
		t.Fatalf("re-read PNG: %v", err)
	}
	if decoded.Bounds() != image.Rect(0, 0, 8, 4) {
		t.Fatalf("PNG bounds: got %v", decoded.Bounds())
	}
	c, ok := decoded.At(1, 2).(color.NRGBA)
	if !ok {
		t.Fatalf("PNG pixel is %T, want color.NRGBA", decoded.At(1, 2))
	}
	if c.R != 10 || c.G != 20 || c.B != 30 || c.A != 40 {
		t.Fatalf("PNG pixel(1,2) = %d %d %d %d", c.R, c.G, c.B, c.A)
	}
}

// TestNodeGraphIntegration — grafo real perlin → math_clamp com
// procgen.Registry() + cache por assinatura: 2ª run = CachedHits.
func TestNodeGraphIntegration(t *testing.T) {
	g := nodegraph.New("procgen-test")
	mustAdd := func(n *nodegraph.Node) {
		t.Helper()
		if err := g.AddNode(n); err != nil {
			t.Fatalf("AddNode %q: %v", n.ID, err)
		}
	}
	mustAdd(&nodegraph.Node{
		ID: "perlin", Type: NodePerlin,
		Params: map[string]any{"seed": int64(7), "width": int64(32), "height": int64(32)},
	})
	mustAdd(&nodegraph.Node{
		ID: "clamp", Type: NodeMathClamp,
		Params: map[string]any{"min": 0.2, "max": 0.8},
		Inputs: []string{"perlin"},
	})

	metrics := &MetricsRecorder{}
	registry := Registry()
	for _, e := range registry {
		if ms, ok := e.(MetricsSetter); ok {
			ms.SetMetrics(metrics)
		}
	}
	exec := registry[NodePerlin]
	exec2 := registry[NodeMathClamp]
	// Executor composto manualmente: perlin e clamp vêm do Registry.
	combined := nodegraph.ExecutorFunc(func(ctx context.Context, node *nodegraph.Node, input map[string]any) (any, error) {
		switch node.Type {
		case NodePerlin:
			return exec.Run(ctx, node, input)
		case NodeMathClamp:
			return exec2.Run(ctx, node, input)
		}
		return nil, nil
	})

	cache := nodegraph.NewCache()
	stats1, err := g.Run(context.Background(), combined, &nodegraph.RunOptions{Cache: cache})
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}
	if stats1.Executed != 2 {
		t.Fatalf("run 1: executed = %d, want 2", stats1.Executed)
	}
	out, ok := stats1.Results["clamp"].(*RGBA)
	if !ok {
		t.Fatalf("clamp result is %T, want *RGBA", stats1.Results["clamp"])
	}
	if out.W != 32 || out.H != 32 {
		t.Fatalf("clamp image size: %dx%d", out.W, out.H)
	}
	// Clamp [0.2,0.8]: nenhum pixel fora da faixa.
	for i := 0; i < len(out.Pix); i += 4 {
		v := out.Pix[i]
		if v < 51 || v > 204 { // 0.2*255=51, 0.8*255=204
			t.Fatalf("clamped pixel out of [0.2,0.8]: %d", v)
		}
	}

	stats2, err := g.Run(context.Background(), combined, &nodegraph.RunOptions{Cache: cache})
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}
	if stats2.CachedHits != 2 {
		t.Fatalf("run 2: cached hits = %d, want 2 (§23)", stats2.CachedHits)
	}
	if stats2.Executed != 0 {
		t.Fatalf("run 2: executed = %d, want 0", stats2.Executed)
	}
	if metrics.Len() != 2 {
		t.Fatalf("metrics recorded = %d, want 2", metrics.Len())
	}
}

// TestMetrics — MetricsRecorder grava e Dump produz JSONL válido.
func TestMetrics(t *testing.T) {
	m := &MetricsRecorder{}
	m.Record(OpMetric{Operation: "fbm", Width: 64, Height: 64, Backend: "cpu-go", LatencyMs: 1.25, MemoryKB: 16})
	m.Record(OpMetric{Operation: "math_clamp", Width: 64, Height: 64})
	if m.Len() != 2 {
		t.Fatalf("Len = %d, want 2", m.Len())
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.jsonl")
	if err := m.Dump(path); err != nil {
		t.Fatalf("Dump: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("JSONL lines = %d, want 2", len(lines))
	}
	for _, l := range lines {
		var om OpMetric
		if err := json.Unmarshal([]byte(l), &om); err != nil {
			t.Fatalf("invalid JSONL line %q: %v", l, err)
		}
		if om.Backend == "" {
			t.Fatal("backend default not applied")
		}
		if om.Timestamp == "" {
			t.Fatal("timestamp not stamped")
		}
	}
}
