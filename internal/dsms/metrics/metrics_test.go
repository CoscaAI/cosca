package metrics

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cosca/internal/dsms"
)

// newTestDSMS cria um DSMS de teste isolado (temp dir + schemas + Open).
func newTestDSMS(t *testing.T) *dsms.DSMS {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "dsms-metrics-*")
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

// TestNewMetricsCache verifica put/get no cache.
func TestNewMetricsCache(t *testing.T) {
	cache := NewMetricsCache()
	if cache == nil {
		t.Fatal("NewMetricsCache returned nil")
	}

	_, ok := cache.Get("db1")
	if ok {
		t.Error("expected miss for missing key")
	}

	m := &DatabaseMetrics{Database: "db1", SizeBytes: 100}
	cache.Set("db1", m)

	got, ok := cache.Get("db1")
	if !ok {
		t.Fatal("expected hit after Set")
	}
	if got.SizeBytes != 100 {
		t.Errorf("expected size 100, got %d", got.SizeBytes)
	}
}

// TestCollectAll verifica a coleta em todos os bancos.
func TestCollectAll(t *testing.T) {
	d := newTestDSMS(t)
	c := NewCollector(d, nil)

	system, err := c.CollectAll(context.Background())
	if err != nil {
		t.Fatalf("CollectAll returned error: %v", err)
	}

	if system.DatabaseCount != len(dsms.AllDatabases()) {
		t.Errorf("expected %d databases, got %d", len(dsms.AllDatabases()), system.DatabaseCount)
	}
	if system.TotalTables == 0 {
		t.Error("expected total tables > 0")
	}
	for dbName, m := range system.Databases {
		if m.SizeBytes == 0 {
			t.Errorf("database %s has size 0", dbName)
		}
	}
}

// TestCollect_SingleDatabase verifica coleta de um banco.
func TestCollect_SingleDatabase(t *testing.T) {
	d := newTestDSMS(t)
	c := NewCollector(d, nil)

	m, err := c.Collect(context.Background(), dsms.DBCore)
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}

	if m.Database != dsms.DBCore {
		t.Errorf("expected %s, got %s", dsms.DBCore, m.Database)
	}
	if m.HealthScore <= 0 || m.HealthScore > 100 {
		t.Errorf("expected health score 0-100, got %.2f", m.HealthScore)
	}
}

// TestCollect_UnknownDatabase verifica erro.
func TestCollect_UnknownDatabase(t *testing.T) {
	d := newTestDSMS(t)
	c := NewCollector(d, nil)

	_, err := c.Collect(context.Background(), "nao-existe.db")
	if err == nil {
		t.Error("expected error for unknown database")
	}
}

// TestCalculateHealthScore verifica a pontuacao de saude.
func TestCalculateHealthScore(t *testing.T) {
	c := NewCollector(newTestDSMS(t), nil)

	// Score maximo
	score := c.calculateHealthScore(&DatabaseMetrics{})
	if score != 100 {
		t.Errorf("expected 100, got %.2f", score)
	}

	// Fragmentacao alta deduz 20
	score = c.calculateHealthScore(&DatabaseMetrics{Fragmentation: 30})
	if score != 80 {
		t.Errorf("expected 80 (frag>20), got %.2f", score)
	}

	// WAL grande deduz 15
	score = c.calculateHealthScore(&DatabaseMetrics{WALSizeBytes: 200 * 1024 * 1024})
	if score != 85 {
		t.Errorf("expected 85 (wal>100MB), got %.2f", score)
	}

	// Nao pode ser negativo
	score = c.calculateHealthScore(&DatabaseMetrics{
		Fragmentation: 30, WALSizeBytes: 200 * 1024 * 1024, SizeBytes: 2 * 1024 * 1024 * 1024,
	})
	if score < 0 {
		t.Errorf("expected non-negative score, got %.2f", score)
	}
}

// TestGetCacheMetrics verifica a coleta de metricas de cache.
func TestGetCacheMetrics(t *testing.T) {
	d := newTestDSMS(t)
	c := NewCollector(d, nil)

	db, _ := d.Cache()
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO search_cache (query_hash, query, results, expires_at) VALUES
		('h1', 'q', '[]', ?),
		('h2', 'q', '[]', ?)
	`, time.Now().UTC().Add(time.Hour).Format("2006-01-02 15:04:05"),
		time.Now().UTC().Add(time.Hour).Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatalf("insert cache: %v", err)
	}

	cache, err := c.GetCacheMetrics(context.Background())
	if err != nil {
		t.Fatalf("GetCacheMetrics returned error: %v", err)
	}

	if cache.TotalEntries != 2 {
		t.Errorf("expected 2 total entries, got %d", cache.TotalEntries)
	}
	if cache.ValidEntries != 2 {
		t.Errorf("expected 2 valid entries, got %d", cache.ValidEntries)
	}
	if cache.ExpiredEntries != 0 {
		t.Errorf("expected 0 expired entries, got %d", cache.ExpiredEntries)
	}
}

// TestReport verifica a geracao do relatorio.
func TestReport(t *testing.T) {
	system := &SystemMetrics{
		DatabaseCount:  2,
		TotalSizeBytes: 1024 * 1024,
		TotalTables:    5,
		TotalIndexes:   3,
		TotalRecords:   100,
		AvgHealthScore: 90,
		Databases: map[string]*DatabaseMetrics{
			"a.db": {Database: "a.db", SizeBytes: 1024, HealthScore: 95},
			"b.db": {Database: "b.db", SizeBytes: 2048, HealthScore: 50},
		},
		CollectedAt: time.Now(),
	}

	report := Report(system)
	if report == "" {
		t.Fatal("Report returned empty string")
	}
	if len(report) < 20 {
		t.Errorf("Report too short: %q", report)
	}
}
