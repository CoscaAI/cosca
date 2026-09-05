package codeembed

import (
	"math"
	"strings"
	"testing"
)

const goSnippet = `package main

import "fmt"

func main() {
    name := "cosca"
    fmt.Println("hello", name)
    for i := 0; i < 10; i++ {
        fmt.Println(i)
    }
}`

const goSnippetVariant = `package main

import "fmt"

func main() {
    title := "cosca"
    fmt.Println("hi", title)
    for i := 0; i < 10; i++ {
        fmt.Println(i)
    }
}`

const pythonSnippet = `def main():
    name = "cosca"
    print("hello", name)
    for i in range(10):
        print(i)`

func TestTokenize_SplitsCamelAndSnake(t *testing.T) {
	got := strings.Join(Tokenize("parseConfig_worker"), " ")
	want := "parse config worker"
	if got != want {
		t.Fatalf("tokenize = %q, want %q", got, want)
	}
}

func TestTokenize_StripsOperators(t *testing.T) {
	got := Tokenize("a := b != c; d == e")
	for _, tok := range got {
		if tok == "=" || tok == ";" || tok == "!" {
			t.Fatalf("operador de um char não deve virar token: %q", got)
		}
	}
}

func TestEmbed_Deterministic(t *testing.T) {
	// I1: mesmo input → mesmo vetor, sempre.
	a := Embed(goSnippet, 512)
	b := Embed(goSnippet, 512)
	if sim := Similarity(a, b); math.Abs(sim-1.0) > 1e-9 {
		t.Fatalf("vetores idênticos devem ter similaridade ~1.0, got %f", sim)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("embed não é determinístico (posição %d)", i)
		}
	}
}

func TestEmbed_Normalized(t *testing.T) {
	v := Embed(goSnippet, 512)
	var norm float64
	for _, x := range v {
		norm += x * x
	}
	if norm < 0.999 || norm > 1.001 {
		t.Fatalf("vetor deve ser normalizado (L2), norm=%f", norm)
	}
}

func TestSimilarity_CodeSemantics(t *testing.T) {
	goVec := Embed(goSnippet, 512)
	goVarVec := Embed(goSnippetVariant, 512)
	pyVec := Embed(pythonSnippet, 512)

	goVsGoVar := Similarity(goVec, goVarVec)
	goVsPy := Similarity(goVec, pyVec)

	// Mesma linguagem e estrutura → mais similar que linguagem diferente.
	if goVsGoVar <= goVsPy {
		t.Fatalf("esperava similaridade Go-vs-Go > Go-vs-Python: go=%.3f py=%.3f", goVsGoVar, goVsPy)
	}
	// Variação do mesmo programa deve ser bem similar (>0.5).
	if goVsGoVar < 0.5 {
		t.Fatalf("Go-vs-GoVariant deveria ser alta, got %.3f", goVsGoVar)
	}
}

func TestEmbed_Dimension(t *testing.T) {
	if len(Embed(goSnippet, 256)) != 256 {
		t.Fatal("dimensão deve ser respeitada")
	}
	if len(Embed(goSnippet, 0)) != DefaultDim {
		t.Fatal("dimensão default deve ser DefaultDim")
	}
}

func TestQuantizeInt8_Range(t *testing.T) {
	v := Embed(goSnippet, 64)
	q := QuantizeInt8(v)
	if len(q) != len(v) {
		t.Fatalf("int8 deve ter a mesma dimensão: %d vs %d", len(q), len(v))
	}
	for _, x := range q {
		if x < -127 || x > 127 {
			t.Fatalf("int8 fora do range: %d", x)
		}
	}
}
