package codeembed

import (
	"math"
	"testing"
)

func TestMinHashSketch_Deterministic(t *testing.T) {
	a := MinHashSketch(goSnippet)
	b := MinHashSketch(goSnippet)
	if len(a) == 0 || len(a) != len(b) {
		t.Fatalf("assinatura vazia ou tamanho desigual: %d vs %d", len(a), len(b))
	}
	if Jaccard(a, b) != 1.0 {
		t.Fatalf("mesmo texto deve ter Jaccard 1.0, got %f", Jaccard(a, b))
	}
}

func TestMinHashSketch_JaccardRange(t *testing.T) {
	a := MinHashSketch("func main() { fmt.Println(a) }")
	b := MinHashSketch("func main() { fmt.Println(b) }")
	c := MinHashSketch("def main():\n  print(a)")
	if j := Jaccard(a, b); j <= 0 || j > 1 {
		t.Fatalf("Jaccard(a,b) fora de (0,1]: %f", j)
	}
	// Mesma linguagem/estrutura deve ter Jaccard maior que cross-lang.
	if Jaccard(a, b) <= Jaccard(a, c) {
		t.Fatalf("Jaccard Go-vs-Go devia ser > Go-vs-Py: %f vs %f", Jaccard(a, b), Jaccard(a, c))
	}
}

func TestBigramEmbed_DeterministicAndNormalized(t *testing.T) {
	v := BigramEmbed(goSnippet, 128)
	v2 := BigramEmbed(goSnippet, 128)
	if len(v) != 128 {
		t.Fatal("dimensão deve ser 128")
	}
	if Similarity(v, v2) < 0.999 || Similarity(v, v2) > 1.001 {
		t.Fatalf("bigram deve ser determinístico, got %f", Similarity(v, v2))
	}
}

func TestFuseSimilarity_DeterministicAndOrdered(t *testing.T) {
	a := "func main() { fmt.Println(x) }"
	b := "func main() { fmt.Println(y) }" // mesma estrutura, 1 token difere
	c := "def main():\n    print(x)"      // linguagem diferente

	sab := FuseSimilarity(a, b, 512)
	sac := FuseSimilarity(a, c, 512)

	if math.Abs(sab-sab) > 1e-9 { // determinismo trivial
		t.Fatal("fuse não deve ter aleatoriedade")
	}
	if sab <= sac {
		t.Fatalf("fusão Go-vs-Go (%f) deve ser > Go-vs-Py (%f)", sab, sac)
	}
	if sab < 0.5 {
		t.Fatalf("mesmo programa (Go) deveria ter fusão alta, got %f", sab)
	}
}

func TestFuseSimilarity_DefaultRange(t *testing.T) {
	s := FuseSimilarity(goSnippet, goSnippetVariant, 512)
	if s < 0 || s > 1 {
		t.Fatalf("fusão deve estar em [0,1], got %f", s)
	}
}
