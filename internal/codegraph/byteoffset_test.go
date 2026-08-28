package codegraph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIndex_GetSymbolSource_ByteOffsetO1 verifica o byte-offset O(1) retrieval:
// recupera EXATAMENTE o trecho do símbolo (sem re-parses, sem dump do arquivo).
func TestIndex_GetSymbolSource_ByteOffsetO1(t *testing.T) {
	dir := t.TempDir()
	content := `package demo

// Greet diz ola.
func Greet(name string) string {
	return "hello " + name
}

// Other faz outra coisa.
func Other() int { return 99 }
`
	if err := os.WriteFile(filepath.Join(dir, "demo.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	ix, err := BuildIndex(dir, 256)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	syms := ix.Symbols["demo.go"]
	if len(syms) == 0 {
		t.Fatal("esperava símbolos Go")
	}
	var greetIdx = -1
	for i, s := range syms {
		if s.Name == "Greet" {
			greetIdx = i
		}
	}
	if greetIdx < 0 {
		t.Fatalf("símbolo Greet não encontrado: %+v", syms)
	}

	src, err := ix.GetSymbolSource("demo.go", greetIdx)
	if err != nil {
		t.Fatalf("GetSymbolSource: %v", err)
	}
	if !strings.Contains(src, "func Greet(name string) string") {
		t.Fatalf("source do símbolo deve conter a assinatura: %q", src)
	}
	// O(1): NÃO deve trazer o arquivo inteiro.
	if len(src) >= len(content) {
		t.Fatalf("byte-offset deveria retornar só o símbolo, veio o arquivo (len=%d)", len(src))
	}
}

func TestIndex_GetSymbolSource_OutOfRange(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\nfunc X(){}\n"), 0o644)
	ix, err := BuildIndex(dir, 128)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, err := ix.GetSymbolSource("a.go", 99); err == nil {
		t.Fatal("índice fora do range deve dar erro")
	}
	if _, err := ix.GetSymbolSource("nao-existe.go", 0); err == nil {
		t.Fatal("arquivo inexistente deve dar erro")
	}
}
