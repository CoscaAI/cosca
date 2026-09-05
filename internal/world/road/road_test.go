package road

import (
	"testing"
)

func TestGenerate_Deterministic(t *testing.T) {
	opts := Options{Seed: 12345}
	a, err := Generate(opts)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	b, _ := Generate(opts)
	if len(a) != len(b) {
		t.Fatalf("mesma seed deve dar mesma rede: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("segmento %d divergiu (determinismo I1): %+v vs %+v", i, a[i], b[i])
		}
	}
}

func TestGenerate_ProducesNetwork(t *testing.T) {
	segs, err := Generate(Options{Seed: 42, Steps: 30})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(segs) < 5 {
		t.Fatalf("esperava rede com varios segmentos, got %d", len(segs))
	}
	// Interseção/snap: ao menos alguns segmentos devem terminar perto de outros
	// (conectividade), não só pontas soltas da origem.
	connected := false
	for i := 0; i < len(segs); i++ {
		for j := i + 1; j < len(segs); j++ {
			if samePoint(segs[i].B, segs[j].A) || samePoint(segs[i].B, segs[j].B) {
				connected = true
			}
		}
	}
	if !connected {
		t.Fatal("rede deveria ter snap/merge (nós compartilhados via interseção)")
	}
}

func TestGenerate_FailClosed(t *testing.T) {
	if _, err := Generate(Options{Seed: 1, BranchChance: 1.5}); err == nil {
		t.Fatal("BranchChance > 1 deve dar erro (fail-closed)")
	}
}

func TestGenerate_DifferentSeeds_Different(t *testing.T) {
	a, _ := Generate(Options{Seed: 1})
	b, _ := Generate(Options{Seed: 2})
	// Seeds diferentes podem coincidir no comprimento, mas devem divergir no
	// conteúdo com alta probabilidade. Checamos ao menos uma diferença.
	same := len(a) == len(b) && SegmentsEqual(a, b)
	if same {
		t.Fatal("seeds diferentes deveriam produzir redes distintas (com probabilidade altíssima)")
	}
}

func samePoint(x, y Point) bool { return x.X == y.X && x.Z == y.Z }

func SegmentsEqual(a, b []Segment) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
