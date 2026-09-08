package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

// TestScannerOnRealProject scans the actual Cosca project.
// This is the REAL-WORLD validation - runs on 1900+ Go files.
func TestScannerOnRealProject(t *testing.T) {
	// Get project root - the Cosca repo root is 3 levels up from internal/dsms
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working dir: %v", err)
	}

	// internal/dsms/intelligence/scanner → up to internal/dsms → up to internal → up to root
	projectRoot := filepath.Join(wd, "..", "..", "..", "..")
	projectRoot, err = filepath.Abs(projectRoot)
	if err != nil {
		t.Fatalf("Failed to get abs path: %v", err)
	}

	// Verify it's the Cosca root (has go.mod)
	goMod := filepath.Join(projectRoot, "go.mod")
	if _, err := os.Stat(goMod); err != nil {
		t.Fatalf("Not the project root: %s (no go.mod)", projectRoot)
	}

	t.Logf("Projeto raiz: %s", projectRoot)
	t.Logf("Iniciando scan do projeto real...")

	scanner := NewScanner(projectRoot)
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Print report
	report := Report(result)
	t.Logf("\n%s", report)

	// Assertions
	if result.FilesScanned < 1000 {
		t.Errorf("Expected >= 1000 Go files, got %d", result.FilesScanned)
	}

	if result.FilesAnalyzed == 0 {
		t.Error("No files analyzed")
	}

	// The scan should find issues (real codebase has TODOs, etc)
	if result.TotalFindings == 0 {
		t.Error("Expected findings in real codebase")
	}

	t.Logf("\n✅ Scan real concluído: %d arquivos, %d findings em %v",
		result.FilesAnalyzed, result.TotalFindings, result.Duration)
}
