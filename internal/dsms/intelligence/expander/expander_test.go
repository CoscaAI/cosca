package expander

import (
	"context"
	"testing"
	"time"

	"cosca/internal/dsms/intelligence/rules"
)

// TestScanCoscaTmp scans E:\cosca-tmp and E:\cosca-tmp-miner.
func TestScanCoscaTmp(t *testing.T) {
	expander := NewExpander()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	t.Log("Escaneando E:\\cosca-tmp e E:\\cosca-tmp-miner...")
	result, err := expander.Scan(ctx, 20000)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	t.Logf("\n════════════════════════════════════════════════════════")
	t.Logf("  SCAN DE CONHECIMENTO EXTERNO (E:\\)")
	t.Logf("════════════════════════════════════════════════════════")
	t.Logf("  Raízes escaneadas: %d", result.RootsScanned)
	t.Logf("  Arquivos de conhecimento: %d", result.FilesFound)
	t.Logf("  Docs lidos: %d", result.DocsFound)
	t.Logf("  Total: %.1f MB", float64(result.TotalBytes)/1024/1024)
	t.Logf("  Tempo: %v", result.Duration)
	t.Logf("")
	t.Logf("  ── Por extensão ──")
	for ext, count := range result.ByExtension {
		t.Logf("    %s: %d", ext, count)
	}
	t.Logf("")
	t.Logf("  ── Amostra de docs ──")
	for i, doc := range result.SampleDocs {
		if i >= 5 {
			break
		}
		preview := doc.Content
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		t.Logf("    %d. [%s] %s", i+1, doc.Language, doc.Path)
	}
	t.Logf("════════════════════════════════════════════════════════")

	if result.FilesFound < 1000 {
		t.Errorf("Expected >= 1000 files, got %d", result.FilesFound)
	}

	t.Logf("\n✅ Scan externo concluído: %d arquivos em %v", result.FilesFound, result.Duration)
}

// TestExtractRulesFromDocs extracts rules from scanned docs.
func TestExtractRulesFromDocs(t *testing.T) {
	expander := NewExpander()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := expander.Scan(ctx, 500)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Extract rules from docs
	extracted := ExtractRules(result.SampleDocs)

	t.Logf("\n════════════════════════════════════════════════════════")
	t.Logf("  EXTRAÇÃO DE REGRAS DO CONHECIMENTO EXTERNO")
	t.Logf("════════════════════════════════════════════════════════")
	t.Logf("  Docs analisados: %d", len(result.SampleDocs))
	t.Logf("  Regras extraídas: %d", len(extracted))
	t.Logf("")

	// Group by domain
	byDomain := make(map[string]int)
	for _, r := range extracted {
		byDomain[r.Domain]++
	}
	for domain, count := range byDomain {
		t.Logf("    %s: %d", domain, count)
	}

	t.Logf("════════════════════════════════════════════════════════")

	// Register into rules engine
	engine := rules.NewEngine()
	for _, r := range extracted {
		engine.Register(r)
	}

	t.Logf("\n✅ %d regras registradas no engine", len(extracted))
}