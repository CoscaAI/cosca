package compact

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"cosca/internal/dsms"
)

// newTestDSMS cria um DSMS de teste isolado (temp dir + schemas + Open).
func newTestDSMS(t *testing.T) *dsms.DSMS {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "dsms-compact-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	schemaDir := filepath.Join(tmpDir, "schema")
	if err := os.MkdirAll(schemaDir, 0755); err != nil {
		t.Fatalf("create schema dir: %v", err)
	}
	for _, schema := range []string{
		"001_core.sql", "002_knowledge.sql", "003_memory.sql",
		"004_intelligence.sql", "005_operations.sql", "006_cache.sql",
	} {
		src := filepath.Join("..", "schema", schema)
		data, err := os.ReadFile(src)
		if err != nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(schemaDir, schema), data, 0644); err != nil {
			t.Fatalf("copy schema %s: %v", schema, err)
		}
	}

	d := dsms.New(dsms.DefaultConfig(tmpDir))
	if err := d.Open(context.Background()); err != nil {
		t.Fatalf("open dsms: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	return d
}

// TestNewCompactor_DefaultConfig verifica o uso de config default.
func TestNewCompactor_DefaultConfig(t *testing.T) {
	d := newTestDSMS(t)
	c := NewCompactor(d, nil)
	if c == nil {
		t.Fatal("NewCompactor returned nil")
	}
	if c.config == nil {
		t.Fatal("expected default config to be populated")
	}
}

// TestCompactAll verifica a compacao em todos os bancos.
func TestCompactAll(t *testing.T) {
	d := newTestDSMS(t)
	c := NewCompactor(d, nil)

	results, err := c.CompactAll(context.Background())
	if err != nil {
		t.Fatalf("CompactAll returned error: %v", err)
	}

	if len(results) != len(dsms.AllDatabases()) {
		t.Fatalf("expected %d results, got %d", len(dsms.AllDatabases()), len(results))
	}

	for _, r := range results {
		if r.Database == "" {
			t.Error("result has empty database name")
		}
		if r.SizeBefore == 0 {
			t.Errorf("database %s has size_before 0 (schema not applied?)", r.Database)
		}
	}
}

// TestCompact_SingleDatabase verifica a compacao de um banco.
func TestCompact_SingleDatabase(t *testing.T) {
	d := newTestDSMS(t)
	c := NewCompactor(d, nil)

	result, err := c.Compact(context.Background(), dsms.DBCore)
	if err != nil {
		t.Fatalf("Compact returned error: %v", err)
	}

	if result.Database != dsms.DBCore {
		t.Errorf("expected %s, got %s", dsms.DBCore, result.Database)
	}
	if result.Duration == 0 {
		t.Error("expected non-zero duration")
	}
}

// TestCompact_UnknownDatabase verifica erro em banco desconhecido.
func TestCompact_UnknownDatabase(t *testing.T) {
	d := newTestDSMS(t)
	c := NewCompactor(d, nil)

	_, err := c.Compact(context.Background(), "nao-existe.db")
	if err == nil {
		t.Error("expected error for unknown database")
	}
}

// TestCompact_HighVacuumThreshold nao deve fazer vacuum (bancos pequenos).
func TestCompact_HighVacuumThreshold(t *testing.T) {
	d := newTestDSMS(t)
	// Threshold muito alto => nunca vacuuma
	c := NewCompactor(d, &Config{
		FragmentationThreshold: 1000.0,
		VacuumThreshold:        1 << 40, // 1TB
		ReindexThreshold:       1000.0,
		MaxWALSize:             1 << 40,
	})

	results, err := c.CompactAll(context.Background())
	if err != nil {
		t.Fatalf("CompactAll returned error: %v", err)
	}

	for _, r := range results {
		if r.Vacuumed {
			t.Errorf("database %s should not have vacuumed with high threshold", r.Database)
		}
	}
}

// TestReport verifica a geracao do relatorio de compactacao.
func TestReport(t *testing.T) {
	results := []*CompactResult{
		{Database: "a.db", SizeBefore: 1000, SizeAfter: 800, SizeReduction: 200, Vacuumed: true},
		{Database: "b.db", SizeBefore: 5000, SizeAfter: 3000, SizeReduction: 2000, Reindexed: true},
	}

	report := Report(results)
	if report == "" {
		t.Fatal("Report returned empty string")
	}
	if len(report) < 20 {
		t.Errorf("Report too short: %q", report)
	}
}

// TestReport_Empty verifica relatorio vazio.
func TestReport_Empty(t *testing.T) {
	report := Report(nil)
	if report == "" {
		t.Fatal("Report returned empty string")
	}
}
