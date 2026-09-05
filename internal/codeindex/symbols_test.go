package codeindex

import "testing"

func TestExtractFile(t *testing.T) {
	syms, err := ExtractFile("symbols.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) == 0 {
		t.Fatal("esperava símbolos extraídos")
	}
	found := false
	for _, s := range syms {
		if s.Name == "ExtractFile" && s.Kind == "func" {
			found = true
		}
	}
	if !found {
		t.Errorf("esperava achar ExtractFile (func), achei %d símbolos", len(syms))
	}
}

func TestWalkDir(t *testing.T) {
	syms, err := WalkDir(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) == 0 {
		t.Fatal("esperava símbolos no diretório")
	}
}
