package nodegraph

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
)

// countingExecutor conta execuções por tipo de nó.
type countingExecutor struct {
	counts map[string]*int64
}

func (c *countingExecutor) Run(_ context.Context, node *Node, _ map[string]any) (any, error) {
	atomic.AddInt64(c.counts[string(node.Type)], 1)
	return string(node.Type) + ":" + node.ID, nil
}

func newCounting(counts map[string]*int64) *countingExecutor {
	return &countingExecutor{counts: counts}
}

func TestRunExecutesAll(t *testing.T) {
	g, _ := Build("p", []*Node{
		np("load", "load_image", map[string]any{"path": "a.png"}),
		n("seg", "segment", "load"),
		n("out", "export", "seg"),
	})
	counts := map[string]*int64{"load_image": new(int64), "segment": new(int64), "export": new(int64)}
	stats, err := g.Run(context.Background(), newCounting(counts), nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Executed != 3 {
		t.Fatalf("Executed = %d, want 3", stats.Executed)
	}
	if stats.CachedHits != 0 {
		t.Fatalf("CachedHits = %d, want 0 (no cache)", stats.CachedHits)
	}
	// Saídas encadeadas corretamente.
	if stats.Results["seg"] != "segment:seg" {
		t.Fatalf("seg result = %v", stats.Results["seg"])
	}
}

func TestRunWithCache_SecondRunAllHits(t *testing.T) {
	g, _ := Build("p", []*Node{
		np("load", "load_image", map[string]any{"path": "a.png"}),
		n("seg", "segment", "load"),
		n("out", "export", "seg"),
	})
	cache := NewCache()
	counts := map[string]*int64{"load_image": new(int64), "segment": new(int64), "export": new(int64)}
	exec := newCounting(counts)

	// Primeira execução: 3 nós executados.
	s1, err := g.Run(context.Background(), exec, &RunOptions{Cache: cache})
	if err != nil {
		t.Fatal(err)
	}
	if s1.Executed != 3 || s1.CachedHits != 0 {
		t.Fatalf("first run: executed=%d hits=%d", s1.Executed, s1.CachedHits)
	}

	// Segunda execução com o mesmo grafo: TUDO do cache (0 executados).
	s2, err := g.Run(context.Background(), exec, &RunOptions{Cache: cache})
	if err != nil {
		t.Fatal(err)
	}
	if s2.Executed != 0 || s2.CachedHits != 3 {
		t.Fatalf("second run: executed=%d hits=%d (want 0/3)", s2.Executed, s2.CachedHits)
	}
	if *counts["segment"] != 1 {
		t.Fatalf("segment ran %d times, want 1 (cache must prevent re-run)", *counts["segment"])
	}
}

func TestRunCache_ParamChangeReexecutesAffectedOnly(t *testing.T) {
	// Cenário do P1/§23: mudar path do load invalida load+seg+out (todos os
	// descendentes) mas a segunda execução de "load" com o MESMO path pode ser
	// hit. O teste prova que o nó dependente re-executa quando o ancestral muda.
	mkGraph := func(path string) *Graph {
		g, _ := Build("p", []*Node{
			np("load", "load_image", map[string]any{"path": path}),
			n("seg", "segment", "load"),
			n("out", "export", "seg"),
		})
		return g
	}

	cache := NewCache()
	counts := map[string]*int64{"load_image": new(int64), "segment": new(int64), "export": new(int64)}
	exec := newCounting(counts)

	g1 := mkGraph("a.png")
	s1, err := g1.Run(context.Background(), exec, &RunOptions{Cache: cache})
	if err != nil {
		t.Fatal(err)
	}
	if s1.Executed != 3 {
		t.Fatalf("run1 executed = %d, want 3", s1.Executed)
	}

	// Mesmo grafo (mesma assinatura) → 100% cache hit.
	g1b := mkGraph("a.png")
	s1b, err := g1b.Run(context.Background(), exec, &RunOptions{Cache: cache})
	if err != nil {
		t.Fatal(err)
	}
	if s1b.Executed != 0 || s1b.CachedHits != 3 {
		t.Fatalf("run1b executed=%d hits=%d, want 0/3", s1b.Executed, s1b.CachedHits)
	}

	// Muda o param do ancestral → descendentes re-executam (assinatura mudou).
	g2 := mkGraph("b.png")
	s2, err := g2.Run(context.Background(), exec, &RunOptions{Cache: cache})
	if err != nil {
		t.Fatal(err)
	}
	if s2.Executed != 3 {
		t.Fatalf("run2 executed = %d, want 3 (ancestor changed)", s2.Executed)
	}
	if s2.CachedHits != 0 {
		t.Fatalf("run2 hits = %d, want 0", s2.CachedHits)
	}
}

func TestRunCache_IndependentNodeHits(t *testing.T) {
	// Dois subgrafos independentes: mudar um NÃO afeta o outro (cache por
	// assinatura é local ao nó — descendência determina a invalidação).
	g1, _ := Build("p", []*Node{
		np("a", "node_a", map[string]any{"v": 1}),
		np("b", "node_b", map[string]any{"v": 1}),
	})
	g2, _ := Build("p", []*Node{
		np("a", "node_a", map[string]any{"v": 2}),
		np("b", "node_b", map[string]any{"v": 1}),
	})
	cache := NewCache()
	counts := map[string]*int64{"node_a": new(int64), "node_b": new(int64)}
	exec := newCounting(counts)

	if _, err := g1.Run(context.Background(), exec, &RunOptions{Cache: cache}); err != nil {
		t.Fatal(err)
	}
	if *counts["node_a"] != 1 || *counts["node_b"] != 1 {
		t.Fatalf("after run1: a=%d b=%d, want 1/1", *counts["node_a"], *counts["node_b"])
	}

	s2, err := g2.Run(context.Background(), exec, &RunOptions{Cache: cache})
	if err != nil {
		t.Fatal(err)
	}
	// node_a mudou → re-executa (1 hit: o "b" com v=1), node_b → hit.
	if s2.Executed != 1 || s2.CachedHits != 1 {
		t.Fatalf("run2 executed=%d hits=%d, want 1/1", s2.Executed, s2.CachedHits)
	}
	if *counts["node_a"] != 2 || *counts["node_b"] != 1 {
		t.Fatalf("after run2: a=%d b=%d, want 2/1", *counts["node_a"], *counts["node_b"])
	}
}

func TestRunExecutorError(t *testing.T) {
	g, _ := Build("p", []*Node{
		n("a", "fail"),
	})
	exec := ExecutorFunc(func(ctx context.Context, node *Node, in map[string]any) (any, error) {
		return nil, fmt.Errorf("boom")
	})
	if _, err := g.Run(context.Background(), exec, nil); err == nil {
		t.Fatal("expected executor error to propagate")
	}
}

func TestRunNilExecutor(t *testing.T) {
	g, _ := Build("p", []*Node{n("a", "x")})
	if _, err := g.Run(context.Background(), nil, nil); err == nil {
		t.Fatal("expected nil executor error")
	}
}
