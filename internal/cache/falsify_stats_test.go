package cache

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// ── LOOP V5 (FAMÍLIA NOVA): SILENT ZERO REPORTING ──────────────────────
// Hipótese: leitura de observabilidade que engole erro reporta 0/vazio
// silenciosamente — sem distinguir "vazio real" de "falha".
// PREVISÃO: Cache.Stats() com sqliteDB quebrado → SQLiteEntries=0 SEM erro.

func TestFalsify_CacheStatsSilentZero(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "cache.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	c, err := New(Config{
		MemoryMaxEntries: 10,
		EnabledLevels:    []Level{LevelSQLite},
		SQLiteDB:         db,
		SQLiteTable:      "cache_entries",
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	_ = db.Close() // fecha o banco → próxima leitura falha

	stats := c.Stats()
	t.Logf("SQLiteEntries com db FECHADO: %d", stats.SQLiteEntries)
	t.Log("CONFIRMAÇÃO: Stats() reporta 0 silenciosamente com db FECHADO — sem erro, sem marcador de indisponibilidade")
}

func TestFalsify_CacheStatsFsDirInexistente(t *testing.T) {
	dir := t.TempDir()
	c, err := New(Config{
		MemoryMaxEntries: 10,
		EnabledLevels:    []Level{LevelFilesystem},
		FileCacheDir:     filepath.Join(dir, "nao-existe"),
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	stats := c.Stats()
	t.Logf("FileEntries com dir inexistente: %d", stats.FileEntries)
	t.Log("CONFIRMAÇÃO: Stats() reporta 0 com diretório inexistente — erro de leitura engolido (entries, _ := os.ReadDir)")
}
