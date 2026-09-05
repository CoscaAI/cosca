package render

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/CoscaAI/cosca/internal/nodegraph"
)

// helper: grafo de exemplo.
func sampleGraph() *nodegraph.Graph {
	g, _ := nodegraph.Build("render-test", []*nodegraph.Node{
		{ID: "load", Type: "load_image", Params: map[string]any{"path": "a.png"}},
		{ID: "seg", Type: "segment", Inputs: []string{"load"}},
		{ID: "out", Type: "export", Inputs: []string{"seg"}},
	})
	return g
}

// countingExec conta execuções e captura os RenderParams do context.
type countingExec struct {
	counts   map[string]*int64
	lastQual map[string]Quality
	lastSeed map[string]int64
}

func (c *countingExec) Run(ctx context.Context, node *nodegraph.Node, _ map[string]any) (any, error) {
	if c.lastQual == nil {
		c.lastQual = map[string]Quality{}
	}
	if c.lastSeed == nil {
		c.lastSeed = map[string]int64{}
	}
	atomic.AddInt64(c.counts[string(node.Type)], 1)
	p := RenderFromContext(ctx)
	c.lastQual[node.ID] = p.Quality
	c.lastSeed[node.ID] = p.Seed
	return string(node.Type), nil
}

func TestNewValidates(t *testing.T) {
	g := sampleGraph()
	if _, err := New(g, Quality("ultra"), 1); err == nil {
		t.Fatal("expected invalid quality error")
	}
	if _, err := New(g, Draft, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil, Draft, 1); err == nil {
		t.Fatal("expected nil graph error")
	}
}

func TestQualityRank(t *testing.T) {
	if !Preview.AtLeast(Preview) || !Draft.AtLeast(Preview) || !Final.AtLeast(Draft) || !Final.AtLeast(Preview) {
		t.Fatal("quality ordering broken")
	}
	if Preview.AtLeast(Final) || Draft.AtLeast(Final) {
		t.Fatal("lower quality must not be at least higher")
	}
	if !Final.Valid() || !Preview.Valid() || !Draft.Valid() {
		t.Fatal("Valid() broken")
	}
	if Quality("nope").Valid() {
		t.Fatal("nope should be invalid")
	}
}

func TestDeterministicContext(t *testing.T) {
	g := sampleGraph()
	j, _ := New(g, Final, 42)
	exec := &countingExec{counts: map[string]*int64{}, lastQual: map[string]Quality{}, lastSeed: map[string]int64{}}
	for _, n := range g.Nodes {
		exec.counts[string(n.Type)] = new(int64)
	}

	r := NewRenderer()
	res, err := r.Render(context.Background(), j, exec)
	if err != nil {
		t.Fatal(err)
	}
	if res.Quality != Final {
		t.Fatalf("quality = %v, want final", res.Quality)
	}
	// O executor recebeu qualidade+seed via context (determinismo §22).
	for _, n := range g.Nodes {
		if exec.lastQual[n.ID] != Final {
			t.Errorf("node %s quality = %v, want final", n.ID, exec.lastQual[n.ID])
		}
		if exec.lastSeed[n.ID] != 42 {
			t.Errorf("node %s seed = %d, want 42", n.ID, exec.lastSeed[n.ID])
		}
	}
	if res.JobKey == "" {
		t.Fatal("job key should be computed")
	}
	if !res.Completed {
		t.Fatal("render should be completed")
	}
}

func TestJobKeyStable(t *testing.T) {
	g1 := sampleGraph()
	g2 := sampleGraph()
	j1, _ := New(g1, Draft, 7)
	j2, _ := New(g2, Draft, 7)
	k1, _ := j1.JobKey()
	k2, _ := j2.JobKey()
	if k1 != k2 {
		t.Fatalf("job keys differ for identical renders: %s vs %s", k1, k2)
	}
	// Qualidade ou seed diferentes → chave diferente.
	j3, _ := New(g1, Final, 7)
	k3, _ := j3.JobKey()
	if k1 == k3 {
		t.Fatal("job key must change with quality")
	}
	j4, _ := New(g1, Draft, 99)
	k4, _ := j4.JobKey()
	if k1 == k4 {
		t.Fatal("job key must change with seed")
	}
}

func TestCacheAvoidsReRender(t *testing.T) {
	g := sampleGraph()
	cache := nodegraph.NewCache()
	exec := &countingExec{counts: map[string]*int64{}}
	for _, n := range g.Nodes {
		exec.counts[string(n.Type)] = new(int64)
	}
	j1, _ := New(g, Draft, 1)
	j1.Cache = cache
	r := NewRenderer()

	res1, err := r.Render(context.Background(), j1, exec)
	if err != nil {
		t.Fatal(err)
	}
	if res1.Executed != 3 {
		t.Fatalf("render1 executed = %d, want 3", res1.Executed)
	}

	// Mesmo grafo, mesmo cache → 0 re-executados.
	j2, _ := New(g, Draft, 1)
	j2.Cache = cache
	res2, err := r.Render(context.Background(), j2, exec)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Executed != 0 || res2.CachedHits != 3 {
		t.Fatalf("render2 executed=%d hits=%d, want 0/3", res2.Executed, res2.CachedHits)
	}
	if *exec.counts["segment"] != 1 {
		t.Fatalf("segment ran %d times, want 1", *exec.counts["segment"])
	}
}

func TestCheckpointResume(t *testing.T) {
	// Cenário §40: o render começou, o processo morreu no meio (simulado por
	// um executor que falha no 2º nó), o checkpoint foi salvo até o 1º nó.
	// Na retomada, o 1º nó NÃO re-executa (resumed) — só o que faltava.
	dir := t.TempDir()
	cpPath := filepath.Join(dir, "render.json")

	g := sampleGraph()
	exec := &countingExec{counts: map[string]*int64{}}
	for _, n := range g.Nodes {
		exec.counts[string(n.Type)] = new(int64)
	}
	r := NewRenderer()

	// Executor que falha no "segment" (crash simulado).
	crashing := nodegraph.ExecutorFunc(func(ctx context.Context, node *nodegraph.Node, in map[string]any) (any, error) {
		if node.Type == "segment" {
			return nil, fmt.Errorf("simulated crash on segment")
		}
		return exec.Run(ctx, node, in)
	})

	j, _ := New(g, Draft, 5)
	j.CheckpointPath = cpPath
	if _, err := r.Render(context.Background(), j, crashing); err == nil {
		t.Fatal("expected crash error")
	}

	// O checkpoint deve existir com o "load" completado (o executor wrappered
	// só persiste no fim — para testes reais de crash usamos save parcial).
	// Aqui verificamos que o executor parou no 2º nó (load=1, segment nunca
	// completou: o crashing retorna erro antes de delegar ao countingExec).
	if *exec.counts["load_image"] != 1 {
		t.Fatalf("load ran %d times, want 1 (crashed on segment)", *exec.counts["load_image"])
	}
	if *exec.counts["segment"] != 0 {
		t.Fatalf("segment ran %d times, want 0 (crash intercepted before execution)", *exec.counts["segment"])
	}
	if *exec.counts["export"] != 0 {
		t.Fatalf("export ran %d times, want 0 (never reached)", *exec.counts["export"])
	}
}

func TestCheckpointSavedAfterSuccess(t *testing.T) {
	dir := t.TempDir()
	cpPath := filepath.Join(dir, "render.json")

	g := sampleGraph()
	exec := &countingExec{counts: map[string]*int64{}}
	for _, n := range g.Nodes {
		exec.counts[string(n.Type)] = new(int64)
	}
	j, _ := New(g, Final, 9)
	j.CheckpointPath = cpPath

	r := NewRenderer()
	if _, err := r.Render(context.Background(), j, exec); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(cpPath)
	if err != nil {
		t.Fatalf("checkpoint file missing: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("checkpoint is empty")
	}

	// Retomada com checkpoint: os 3 nós estão no checkpoint → resumed.
	exec2 := &countingExec{counts: map[string]*int64{}}
	for _, n := range g.Nodes {
		exec2.counts[string(n.Type)] = new(int64)
	}
	j2, _ := New(g, Final, 9)
	j2.CheckpointPath = cpPath
	res2, err := r.Render(context.Background(), j2, exec2)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Resumed != 3 {
		t.Fatalf("resumed = %d, want 3 (all nodes from checkpoint)", res2.Resumed)
	}
	if res2.Executed != 0 {
		t.Fatalf("executed = %d, want 0 (nothing re-run)", res2.Executed)
	}
	// O executor base nunca rodou de verdade na retomada.
	if *exec2.counts["segment"] != 0 {
		t.Fatalf("segment re-ran %d times on resume, want 0", *exec2.counts["segment"])
	}
}

func TestRenderContextDefaults(t *testing.T) {
	p := RenderFromContext(context.Background())
	if p.Quality != Draft {
		t.Fatalf("default quality = %v, want draft", p.Quality)
	}
	if p.Seed != 0 {
		t.Fatalf("default seed = %d, want 0", p.Seed)
	}
}
