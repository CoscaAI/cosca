package csnparser

import (
	"context"
	"testing"
	"time"

	"cosca/internal/dsms/intelligence/rules"
)

// TestParseCodeSearchNet parses the downloaded CodeSearchNet parquet files.
func TestParseCodeSearchNet(t *testing.T) {
	parser := NewParser(`E:\trainer`)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	t.Log("Lendo arquivos parquet do CodeSearchNet em E:\\trainer...")
	result, err := parser.Parse(ctx, 12)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	t.Logf("\n════════════════════════════════════════════════════════")
	t.Logf("  CODESEARCHNET — DADOS REAIS")
	t.Logf("════════════════════════════════════════════════════════")
	t.Logf("  Arquivos lidos: %d", result.FilesRead)
	t.Logf("  Linhas totais: %d", result.RowsRead)
	t.Logf("  Com código: %d", result.RowsWithCode)
	t.Logf("  Com docstring: %d", result.RowsWithDoc)
	t.Logf("  Tempo: %v", result.Duration)
	t.Logf("")
	t.Logf("  ── Por linguagem ──")
	for lang, count := range result.ByLanguage {
		t.Logf("    %s: %d", lang, count)
	}
	t.Logf("")
	t.Logf("  ── Amostra de pares (docstring → código) ──")
	for i, pair := range result.SamplePairs {
		if i >= 5 {
			break
		}
		doc := pair.Docstring
		if len(doc) > 80 {
			doc = doc[:80] + "..."
		}
		code := pair.Code
		if len(code) > 80 {
			code = code[:80] + "..."
		}
		t.Logf("    %d. [%s] %s.%s", i+1, pair.Language, pair.Repo, pair.FuncName)
		t.Logf("       DOC: %s", doc)
		t.Logf("       CODE: %s", code)
	}
	t.Logf("════════════════════════════════════════════════════════")

	if result.RowsRead == 0 {
		t.Fatal("No rows read - parquet schema may differ")
	}

	if result.RowsWithCode == 0 {
		t.Error("No rows with code")
	}

	t.Logf("\n✅ CodeSearchNet lido: %d linhas em %v", result.RowsRead, result.Duration)
}

// TestExtractRulesFromCSN extracts rules from CodeSearchNet pairs.
func TestExtractRulesFromCSN(t *testing.T) {
	parser := NewParser(`E:\trainer`)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	result, err := parser.Parse(ctx, 12)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Extract rules from sample pairs
	extracted := ExtractRules(result.SamplePairs)

	t.Logf("\n════════════════════════════════════════════════════════")
	t.Logf("  EXTRAÇÃO DE REGRAS DO CODESEARCHNET")
	t.Logf("════════════════════════════════════════════════════════")
	t.Logf("  Pares analisados: %d", len(result.SamplePairs))
	t.Logf("  Regras extraídas: %d", len(extracted))

	byDomain := make(map[string]int)
	for _, r := range extracted {
		byDomain[r.Domain]++
	}
	for domain, count := range byDomain {
		t.Logf("    %s: %d", domain, count)
	}
	t.Logf("════════════════════════════════════════════════════════")

	// Register into engine
	engine := rules.NewEngine()
	for _, r := range extracted {
		engine.Register(r)
	}

	t.Logf("\n✅ %d regras do CodeSearchNet registradas", len(extracted))
}