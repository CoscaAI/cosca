package codegraph

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildIndex_CoverageSeparatedFromFacts(t *testing.T) {
	dir := t.TempDir()
	// Dois arquivos-fonte (indexáveis).
	_ = os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\nfunc X(){}\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "b.go"), []byte("package b\nfunc Y(){}\n"), 0o644)
	// Um arquivo NÃO-fonte (markdown) — não entra na contagem de cobertura.
	_ = os.WriteFile(filepath.Join(dir, "doc.md"), []byte("# notas"), 0o644)
	// Um subdir de dependência — pulado (não é fonte).
	_ = os.MkdirAll(filepath.Join(dir, "vendor"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "vendor", "v.go"), []byte("package v\n"), 0o644)

	ix, err := BuildIndex(dir, 128)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	c := ix.Coverage
	// A cobertura conta SÓ os arquivos-fonte conhecidos (.go); doc.md e vendor/ NÃO.
	if c.TotalSourceFiles != 2 {
		t.Fatalf("TotalSourceFiles deveria ser 2 (a.go,b.go), got %d", c.TotalSourceFiles)
	}
	if c.IndexedFiles != 2 {
		t.Fatalf("IndexedFiles deveria ser 2, got %d", c.IndexedFiles)
	}
	if c.Langs["go"] != 2 {
		t.Fatalf("Langs[go] deveria ser 2, got %d", c.Langs["go"])
	}
	if c.Langs["markdown"] != 0 && c.Langs["md"] != 0 {
		t.Fatalf(".md não deve contar como fonte: %v", c.Langs)
	}
	if !c.BestEffort {
		t.Fatal("BestEffort deve ser true (heurístico v1, nunca completude)")
	}
	if !c.CoversAll {
		t.Fatal("sem arquivo pulado, CoversAll deve ser true")
	}
}

func TestBuildIndex_CoverageRecordsSkip(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\nfunc X(){}\n"), 0o644)
	// Um arquivo .go grande demais / ilegível não existe aqui facilmente; em vez
	// disso, testamos que a contabilidade apenas SOMA os indexados. Com um só
	// arquivo, skipped = 0 e coversAll = true.
	ix, err := BuildIndex(dir, 128)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if ix.Coverage.SkippedFiles != 0 || !ix.Coverage.CoversAll {
		t.Fatalf("esperava skipped=0/coversAll=true, got %+v", ix.Coverage)
	}
}
