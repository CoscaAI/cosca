package archive

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

	tmpDir, err := os.MkdirTemp("", "dsms-archive-*")
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

func oldTime(days int) time.Time {
	return time.Now().AddDate(0, 0, -days)
}

// TestNewArchiver_DefaultConfig verifica config default.
func TestNewArchiver_DefaultConfig(t *testing.T) {
	d := newTestDSMS(t)
	a := NewArchiver(d, nil)
	if a == nil {
		t.Fatal("NewArchiver returned nil")
	}
	if a.config == nil {
		t.Fatal("expected default config to be populated")
	}
}

// TestArchiveAll_ArchivesOldSessions verifica o arquivamento de sessoes antigas.
func TestArchiveAll_ArchivesOldSessions(t *testing.T) {
	d := newTestDSMS(t)
	// Retencao curta => tudo antigo e arquivado
	a := NewArchiver(d, &Config{
		SessionsHotDays:  0,
		TracesHotDays:    0,
		FailuresHotDays:  0,
		DecisionsHotDays: 0,
		LearningsHotDays: 0,
		CacheTTLHours:    0,
		ArchiveDatabase:  dsms.DBMemory,
	})

	db, _ := d.Memory()
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO sessions (id, agent, ended_at, status) VALUES
		('s-old', 'kernel', ?, 'ended')
	`, oldTime(90).Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	// tambem insere uma sessao RECENTE (futuro) que NAO deve arquivar
	_, err = db.ExecContext(context.Background(), `
		INSERT INTO sessions (id, agent, ended_at, status) VALUES
		('s-new', 'kernel', ?, 'ended')
	`, time.Now().Add(24*time.Hour).Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatalf("insert recent session: %v", err)
	}

	results, err := a.ArchiveAll(context.Background())
	if err != nil {
		t.Fatalf("ArchiveAll returned error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected archive results")
	}

	var remaining int
	db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM sessions").Scan(&remaining)
	if remaining != 1 {
		t.Errorf("expected 1 remaining session (recent), got %d", remaining)
	}
}

// TestArchiveAll_CleansExpiredCache verifica a limpeza de cache expirado.
func TestArchiveAll_CleansExpiredCache(t *testing.T) {
	d := newTestDSMS(t)
	a := NewArchiver(d, &Config{
		CacheTTLHours: 0, // expira tudo
	})

	db, _ := d.Cache()
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO search_cache (query_hash, query, results, expires_at) VALUES
		('h1', 'query', '[]', ?)
	`, oldTime(2).Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatalf("insert cache: %v", err)
	}
	// cache valido (nao expirado) - usar UTC para casar com datetime('now')
	_, err = db.ExecContext(context.Background(), `
		INSERT INTO search_cache (query_hash, query, results, expires_at) VALUES
		('h2', 'query', '[]', ?)
	`, time.Now().UTC().Add(time.Hour).Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatalf("insert valid cache: %v", err)
	}

	_, err = a.ArchiveAll(context.Background())
	if err != nil {
		t.Fatalf("ArchiveAll returned error: %v", err)
	}

	var remaining int
	db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM search_cache").Scan(&remaining)
	if remaining != 1 {
		t.Errorf("expected 1 remaining valid cache entry, got %d", remaining)
	}
}

// TestArchiveAll_DeletesOldFailures verifica a remocao de falhas resolvidas antigas.
func TestArchiveAll_DeletesOldFailures(t *testing.T) {
	d := newTestDSMS(t)
	a := NewArchiver(d, &Config{
		FailuresHotDays: 0,
	})

	db, _ := d.Memory()
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO failures (id, agent, error, resolved_at, created_at) VALUES
		('f1', 'kernel', 'err', ?, ?)
	`, oldTime(10).Format("2006-01-02 15:04:05"), oldTime(10).Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatalf("insert failure: %v", err)
	}
	// falha nao resolvida NAO deve ser apagada
	_, err = db.ExecContext(context.Background(), `
		INSERT INTO failures (id, agent, error, created_at) VALUES
		('f2', 'kernel', 'err', ?)
	`, oldTime(10).Format("2006-01-02 15:04:05"))
	if err != nil {
		t.Fatalf("insert unresolved failure: %v", err)
	}

	_, err = a.ArchiveAll(context.Background())
	if err != nil {
		t.Fatalf("ArchiveAll returned error: %v", err)
	}

	var remaining int
	db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM failures").Scan(&remaining)
	if remaining != 1 {
		t.Errorf("expected 1 remaining unresolved failure, got %d", remaining)
	}
}

// TestReport verifica a geracao do relatorio de archive.
func TestReport(t *testing.T) {
	results := []*ArchiveResult{
		{Table: "sessions", RecordsArchived: 5, RecordsDeleted: 5},
		{Table: "cache", RecordsArchived: 0, RecordsDeleted: 3},
	}

	report := Report(results)
	if report == "" {
		t.Fatal("Report returned empty string")
	}
	if len(report) < 10 {
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
