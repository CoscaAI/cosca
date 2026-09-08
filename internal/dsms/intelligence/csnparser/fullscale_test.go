package csnparser

import (
	"context"
	"testing"
	"time"

	"cosca/internal/dsms/intelligence/rules"
)

// TestExtractRulesFullScale extracts rules from ALL CodeSearchNet pairs.
func TestExtractRulesFullScale(t *testing.T) {
	parser := NewParser(`E:\trainer`)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	t.Log("Lendo TODOS os pares do CodeSearchNet...")
	result, err := parser.Parse(ctx, 0) // 0 = todos os arquivos
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	t.Logf("Pares totais: %d", result.RowsRead)

	// Build full pairs list from all rows
	// (Parse only keeps 10 samples, so we re-read with full collection)
	allPairs := collectAllPairs(parser, ctx)

	t.Logf("Pares coletados: %d", len(allPairs))

	// Extract rules from ALL pairs
	start := time.Now()
	extracted := ExtractRules(allPairs)
	elapsed := time.Since(start)

	t.Logf("\n════════════════════════════════════════════════════════")
	t.Logf("  EXTRAÇÃO EM ESCALA — CODESEARCHNET")
	t.Logf("════════════════════════════════════════════════════════")
	t.Logf("  Pares analisados: %d", len(allPairs))
	t.Logf("  Regras extraídas: %d", len(extracted))
	t.Logf("  Tempo: %v", elapsed)

	byDomain := make(map[string]int)
	for _, r := range extracted {
		byDomain[r.Domain]++
	}
	t.Logf("")
	t.Logf("  ── Por domínio ──")
	for domain, count := range byDomain {
		t.Logf("    %s: %d", domain, count)
	}
	t.Logf("════════════════════════════════════════════════════════")

	// Register into engine
	engine := rules.NewEngine()
	registered := 0
	for _, r := range extracted {
		if err := engine.Register(r); err == nil {
			registered++
		}
	}

	t.Logf("\n✅ %d regras registradas no engine", registered)

	if len(extracted) < 10 {
		t.Errorf("Expected >= 10 rules, got %d", len(extracted))
	}
}

// collectAllPairs reads all pairs from all files.
func collectAllPairs(p *Parser, ctx context.Context) []*CodePair {
	var all []*CodePair

	files, _ := filepathGlob(p.Dir)
	for _, file := range files {
		pairs := readFilePairs(file, ctx)
		all = append(all, pairs...)
	}

	return all
}