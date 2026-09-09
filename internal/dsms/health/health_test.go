package health

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"cosca/internal/dsms"
)

// newTestDSMS cria um DSMS de teste isolado (temp dir + schemas copiados + Open).
func newTestDSMS(t *testing.T) *dsms.DSMS {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "dsms-health-*")
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

// TestNewHealthChecker verifica a construcao do checker.
func TestNewHealthChecker(t *testing.T) {
	d := newTestDSMS(t)
	h := NewHealthChecker(d)
	if h == nil {
		t.Fatal("NewHealthChecker returned nil")
	}
}

// TestCheckAll_AllDatabasesHealthy verifica o health check em todos os bancos.
func TestCheckAll_AllDatabasesHealthy(t *testing.T) {
	d := newTestDSMS(t)
	h := NewHealthChecker(d)

	statuses, err := h.CheckAll(context.Background())
	if err != nil {
		t.Fatalf("CheckAll returned error: %v", err)
	}

	if len(statuses) != len(dsms.AllDatabases()) {
		t.Fatalf("expected %d statuses, got %d", len(dsms.AllDatabases()), len(statuses))
	}

	for _, s := range statuses {
		if s.Database == "" {
			t.Errorf("status has empty database name")
		}
		if !s.Healthy {
			t.Errorf("database %s should be healthy, got false", s.Database)
		}
		if s.TableCount == 0 {
			t.Errorf("database %s has 0 tables (schema not applied?)", s.Database)
		}
	}
}

// TestCheck_SingleDatabase verifica o health check de um unico banco.
func TestCheck_SingleDatabase(t *testing.T) {
	d := newTestDSMS(t)
	h := NewHealthChecker(d)

	status, err := h.Check(context.Background(), dsms.DBCore)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if status.Database != dsms.DBCore {
		t.Errorf("expected database %s, got %s", dsms.DBCore, status.Database)
	}
	if !status.IntegrityOK {
		t.Error("expected integrity to be ok")
	}
	if status.SizeBytes == 0 {
		t.Error("expected size bytes to be > 0")
	}
}

// TestCheck_UnknownDatabase verifica erro em banco desconhecido.
func TestCheck_UnknownDatabase(t *testing.T) {
	d := newTestDSMS(t)
	h := NewHealthChecker(d)

	_, err := h.Check(context.Background(), "nao-existe.db")
	if err == nil {
		t.Error("expected error for unknown database")
	}
}

// TestCheckAlerts verifica a geracao de alertas.
func TestCheckAlerts(t *testing.T) {
	h := NewHealthChecker(newTestDSMS(t))

	healthyStatuses := []*HealthStatus{
		{Database: "a.db", Healthy: true, IntegrityOK: true, WALHealthy: true, FragmentationOK: true},
	}
	alerts := h.CheckAlerts(healthyStatuses)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts for healthy status, got %d", len(alerts))
	}

	badStatuses := []*HealthStatus{
		{Database: "b.db", Healthy: false, IntegrityOK: false, WALHealthy: true, FragmentationOK: true},
		{Database: "c.db", Healthy: false, IntegrityOK: true, WALHealthy: false, FragmentationOK: true},
		{Database: "d.db", Healthy: false, IntegrityOK: true, WALHealthy: true, FragmentationOK: false},
	}
	alerts = h.CheckAlerts(badStatuses)
	// Cada status com 1 falha explicita gera 1 alerta => 3 alertas no total
	if len(alerts) != 3 {
		t.Errorf("expected 3 alerts for bad statuses, got %d", len(alerts))
	}

	// Verifica severidade critica
	hasCritical := false
	for _, a := range alerts {
		if a.Severity == "critical" {
			hasCritical = true
		}
	}
	if !hasCritical {
		t.Error("expected at least one critical alert")
	}

	// Um status com TODAS as falhas gera 3 alertas (integrity + wal + frag)
	fullBad := []*HealthStatus{
		{Database: "e.db", Healthy: false, IntegrityOK: false, WALHealthy: false, FragmentationOK: false},
	}
	alerts = h.CheckAlerts(fullBad)
	if len(alerts) != 3 {
		t.Errorf("expected 3 alerts for all-failed status, got %d", len(alerts))
	}
}

// TestReport verifica a geracao do relatorio de saude.
func TestReport(t *testing.T) {
	h := NewHealthChecker(newTestDSMS(t))
	statuses := []*HealthStatus{
		{Database: "a.db", Healthy: true, IntegrityOK: true, SizeBytes: 100, TableCount: 3, IndexCount: 2},
		{Database: "b.db", Healthy: false, IntegrityOK: false, SizeBytes: 200, TableCount: 1, IndexCount: 0},
	}

	report := h.Report(statuses)
	if report == "" {
		t.Fatal("Report returned empty string")
	}
	if len(report) < 20 {
		t.Errorf("Report too short: %q", report)
	}
}
