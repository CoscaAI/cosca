package cache

import (
	"database/sql"
	"os"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg.MemoryMaxEntries != 10000 {
		t.Errorf("MemoryMaxEntries = %d, want 10000", cfg.MemoryMaxEntries)
	}
	if cfg.MemoryTTL != 5*time.Minute {
		t.Errorf("MemoryTTL = %v, want 5m", cfg.MemoryTTL)
	}
	if cfg.SQLiteTable != "cache" {
		t.Errorf("SQLiteTable = %q", cfg.SQLiteTable)
	}
	if cfg.FileMaxSize != 100*1024*1024 {
		t.Errorf("FileMaxSize = %d", cfg.FileMaxSize)
	}
	if cfg.DefaultTTL != time.Hour {
		t.Errorf("DefaultTTL = %v, want 1h", cfg.DefaultTTL)
	}
	if len(cfg.EnabledLevels) != 2 {
		t.Errorf("EnabledLevels len = %d, want 2", len(cfg.EnabledLevels))
	}
}

func TestLevelConstants(t *testing.T) {
	t.Parallel()
	if LevelMemory != 0 {
		t.Errorf("LevelMemory = %d, want 0", LevelMemory)
	}
	if LevelSQLite != 1 {
		t.Errorf("LevelSQLite = %d, want 1", LevelSQLite)
	}
	if LevelFilesystem != 2 {
		t.Errorf("LevelFilesystem = %d, want 2", LevelFilesystem)
	}
}

func TestEntryDefaults(t *testing.T) {
	t.Parallel()
	entry := Entry{
		Key:   "test-key",
		Value: "test-value",
	}
	if entry.Key != "test-key" {
		t.Errorf("Key = %q", entry.Key)
	}
	if entry.Value != "test-value" {
		t.Errorf("Value = %v", entry.Value)
	}
}

func TestStatsDefaults(t *testing.T) {
	t.Parallel()
	s := Stats{}
	if s.MemoryEntries != 0 {
		t.Errorf("MemoryEntries = %d", s.MemoryEntries)
	}
	if s.Misses != 0 {
		t.Errorf("Misses = %d", s.Misses)
	}
}

func TestNewCacheWithoutDB(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New cache error: %v", err)
	}
	if c == nil {
		t.Fatal("cache is nil")
	}
}

func TestCacheDefaultsOnConfig(t *testing.T) {
	t.Parallel()
	cfg := Config{}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	if c.cfg.MemoryMaxEntries != 10000 {
		t.Errorf("MemoryMaxEntries = %d", c.cfg.MemoryMaxEntries)
	}
	if c.cfg.MemoryTTL != 5*time.Minute {
		t.Errorf("MemoryTTL = %v", c.cfg.MemoryTTL)
	}
	if c.cfg.DefaultTTL != time.Hour {
		t.Errorf("DefaultTTL = %v", c.cfg.DefaultTTL)
	}
}

func TestCacheGetMiss(t *testing.T) {
	t.Parallel()
	c, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	val, ok := c.Get("nonexistent")
	if ok {
		t.Error("Get should return false for nonexistent key")
	}
	if val != nil {
		t.Errorf("val = %v, want nil", val)
	}
}

func TestCacheSetAndGetMemory(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	cfg.EnabledLevels = []Level{LevelMemory}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	err = c.Set("key1", "value1", time.Minute)
	if err != nil {
		t.Fatalf("Set error: %v", err)
	}
	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("Get should return true")
	}
	if val != "value1" {
		t.Errorf("val = %v, want %v", val, "value1")
	}
}

func TestCacheSetDefaultTTL(t *testing.T) {
	t.Parallel()
	c, _ := New(DefaultConfig())
	err := c.Set("key2", 42, 0) // zero TTL should use default
	if err != nil {
		t.Fatalf("Set error: %v", err)
	}
	val, ok := c.Get("key2")
	if !ok {
		t.Fatal("Get should return true")
	}
	if val != 42 {
		t.Errorf("val = %v, want 42", val)
	}
}

func TestCacheExists(t *testing.T) {
	t.Parallel()
	c, _ := New(DefaultConfig())
	_ = c.Set("exists-key", "value", time.Minute)
	if !c.Exists("exists-key") {
		t.Error("Exists should return true for existing key")
	}
	if c.Exists("nonexistent") {
		t.Error("Exists should return false for nonexistent key")
	}
}

func TestCacheDelete(t *testing.T) {
	t.Parallel()
	c, _ := New(DefaultConfig())
	_ = c.Set("del-key", "value", time.Minute)
	_ = c.Delete("del-key")
	if c.Exists("del-key") {
		t.Error("Key should not exist after Delete")
	}
}

func TestCacheClear(t *testing.T) {
	t.Parallel()
	c, _ := New(DefaultConfig())
	_ = c.Set("a", 1, time.Minute)
	_ = c.Set("b", 2, time.Minute)
	_ = c.Clear()
	if c.Exists("a") || c.Exists("b") {
		t.Error("Keys should not exist after Clear")
	}
}

func TestEstimateSize(t *testing.T) {
	t.Parallel()
	size := estimateSize("hello")
	if size <= 0 {
		t.Errorf("estimateSize should be > 0, got %d", size)
	}
}

func TestKeyFunction(t *testing.T) {
	t.Parallel()
	k1 := Key("part1", "part2")
	k2 := Key("part1", "part2")
	k3 := Key("different")
	if k1 != k2 {
		t.Error("Key should be deterministic")
	}
	if k1 == k3 {
		t.Error("Different inputs should produce different keys")
	}
}

func TestMemoryLevelEnabled(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	c, _ := New(cfg)
	if !c.levelEnabled(LevelMemory) {
		t.Error("LevelMemory should be enabled")
	}
	if !c.levelEnabled(LevelSQLite) {
		t.Error("LevelSQLite should be enabled")
	}
}

func TestFilesystemLevelDisabled(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	c, _ := New(cfg)
	if c.levelEnabled(LevelFilesystem) {
		t.Error("LevelFilesystem should be disabled by default")
	}
}

func TestCacheMultipleOperations(t *testing.T) {
	t.Parallel()
	c, _ := New(DefaultConfig())
	// Set multiple, check stats
	_ = c.Set("k1", "v1", time.Minute)
	_ = c.Set("k2", "v2", time.Minute)
	_ = c.Set("k3", "v3", time.Minute)

	_, _ = c.Get("k1")
	_, _ = c.Get("k2")
	_, _ = c.Get("nonexistent")

	stats := c.Stats()
	if stats.MemoryEntries != 3 {
		t.Errorf("MemoryEntries = %d, want 3", stats.MemoryEntries)
	}
}

// ── Filesystem cache level tests ──────────────────────────────────────────

func TestFilesystemCacheSetAndGet(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		EnabledLevels: []Level{LevelFilesystem},
		FileCacheDir:  dir,
		FileMaxSize:   100 * 1024 * 1024,
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.Set("fs-key", "fs-value", time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, ok := c.Get("fs-key")
	if !ok {
		t.Fatal("Get should find fs-key")
	}
	if val != "fs-value" {
		t.Errorf("val = %v, want %q", val, "fs-value")
	}
}

func TestFilesystemCacheExists(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		EnabledLevels: []Level{LevelFilesystem},
		FileCacheDir:  dir,
		FileMaxSize:   100 * 1024 * 1024,
	}
	c, _ := New(cfg)
	_ = c.Set("fs-ex", "v", time.Minute)
	if !c.Exists("fs-ex") {
		t.Error("Exists should be true")
	}
	if c.Exists("fs-no") {
		t.Error("Exists should be false")
	}
}

func TestFilesystemCacheDelete(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		EnabledLevels: []Level{LevelFilesystem},
		FileCacheDir:  dir,
		FileMaxSize:   100 * 1024 * 1024,
	}
	c, _ := New(cfg)
	_ = c.Set("fs-del", "v", time.Minute)
	_ = c.Delete("fs-del")
	if c.Exists("fs-del") {
		t.Error("Key should be gone after delete")
	}
}

func TestFilesystemCacheClear(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		EnabledLevels: []Level{LevelFilesystem},
		FileCacheDir:  dir,
		FileMaxSize:   100 * 1024 * 1024,
	}
	c, _ := New(cfg)
	_ = c.Set("fs-a", 1, time.Minute)
	_ = c.Set("fs-b", 2, time.Minute)
	_ = c.Clear()
	if c.Exists("fs-a") || c.Exists("fs-b") {
		t.Error("Keys should be gone after clear")
	}
}

func TestFilesystemCacheStats(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		EnabledLevels: []Level{LevelFilesystem},
		FileCacheDir:  dir,
		FileMaxSize:   100 * 1024 * 1024,
	}
	c, _ := New(cfg)
	_ = c.Set("fs-s", "stats-test", time.Minute)
	stats := c.Stats()
	if stats.FileEntries != 1 {
		t.Errorf("FileEntries = %d, want 1", stats.FileEntries)
	}
}

// ── SQLite cache level tests ──────────────────────────────────────────────

func testSQLiteDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_journal_mode=WAL")
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSQLiteCacheSetAndGet(t *testing.T) {
	db := testSQLiteDB(t)
	cfg := Config{
		EnabledLevels: []Level{LevelSQLite},
		SQLiteDB:      db,
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.Set("sql-key", map[string]string{"a": "b"}, time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, ok := c.Get("sql-key")
	if !ok {
		t.Fatal("Get should find sql-key")
	}
	m, ok2 := val.(map[string]interface{})
	if !ok2 {
		t.Fatalf("expected map, got %T", val)
	}
	if m["a"] != "b" {
		t.Errorf("m[a] = %v", m["a"])
	}
}

func TestSQLiteCacheExists(t *testing.T) {
	db := testSQLiteDB(t)
	cfg := Config{
		EnabledLevels: []Level{LevelSQLite},
		SQLiteDB:      db,
	}
	c, _ := New(cfg)
	_ = c.Set("sqle", "v", time.Minute)
	if !c.Exists("sqle") {
		t.Error("Exists should be true")
	}
	if c.Exists("sqle-no") {
		t.Error("Exists should be false")
	}
}

func TestSQLiteCacheDelete(t *testing.T) {
	db := testSQLiteDB(t)
	cfg := Config{
		EnabledLevels: []Level{LevelSQLite},
		SQLiteDB:      db,
	}
	c, _ := New(cfg)
	_ = c.Set("sqldel", "v", time.Minute)
	_ = c.Delete("sqldel")
	if c.Exists("sqldel") {
		t.Error("Key should be gone after delete")
	}
}

func TestSQLiteCacheClear(t *testing.T) {
	db := testSQLiteDB(t)
	cfg := Config{
		EnabledLevels: []Level{LevelSQLite},
		SQLiteDB:      db,
	}
	c, _ := New(cfg)
	_ = c.Set("sql-a", 1, time.Minute)
	_ = c.Set("sql-b", 2, time.Minute)
	_ = c.Clear()
	if c.Exists("sql-a") || c.Exists("sql-b") {
		t.Error("Keys should be gone after clear")
	}
}

func TestSQLiteCacheStats(t *testing.T) {
	db := testSQLiteDB(t)
	cfg := Config{
		EnabledLevels: []Level{LevelSQLite},
		SQLiteDB:      db,
	}
	c, _ := New(cfg)
	_ = c.Set("sqls", "stats", time.Minute)
	stats := c.Stats()
	if stats.SQLiteEntries != 1 {
		t.Errorf("SQLiteEntries = %d, want 1", stats.SQLiteEntries)
	}
}

// ── LRU eviction ──────────────────────────────────────────────────────────

func TestMemoryLRUEviction(t *testing.T) {
	cfg := Config{
		EnabledLevels:    []Level{LevelMemory},
		MemoryMaxEntries: 3,
		MemoryTTL:        time.Hour,
	}
	c, _ := New(cfg)
	_ = c.Set("a", 1, time.Hour)
	_ = c.Set("b", 2, time.Hour)
	_ = c.Set("c", 3, time.Hour)
	// Access a to make it recently used; b becomes LRU
	_, _ = c.Get("a")
	// Insert 4th — should evict b (LRU)
	_ = c.Set("d", 4, time.Hour)
	if c.Exists("b") {
		t.Error("b should have been evicted (LRU)")
	}
	if !c.Exists("a") {
		t.Error("a should still exist (recently used)")
	}
	if !c.Exists("c") {
		t.Error("c should still exist")
	}
	if !c.Exists("d") {
		t.Error("d should exist (new)")
	}
}

// ── TTL expiration ────────────────────────────────────────────────────────

func TestMemoryTTLExpiration(t *testing.T) {
	cfg := Config{
		EnabledLevels: []Level{LevelMemory},
		MemoryTTL:     10 * time.Millisecond,
	}
	c, _ := New(cfg)
	_ = c.Set("exp", "x", 10*time.Millisecond)
	// Should exist before TTL
	if !c.Exists("exp") {
		t.Fatal("should exist before TTL")
	}
	time.Sleep(15 * time.Millisecond)
	// Should be gone after TTL
	if c.Exists("exp") {
		t.Error("should be expired after TTL")
	}
	val, ok := c.Get("exp")
	if ok || val != nil {
		t.Error("expired key should not be returned")
	}
}

// ── Delete non-existing key ───────────────────────────────────────────────

func TestDeleteNonExistingKey(t *testing.T) {
	c, _ := New(DefaultConfig())
	// Should not panic or error
	if err := c.Delete("never-existed"); err != nil {
		t.Errorf("Delete should succeed for non-existing key: %v", err)
	}
}

// ── New with filesystem dir creation ──────────────────────────────────────

func TestNewWithFilesystemDir(t *testing.T) {
	dir := t.TempDir()
	fsDir := dir + "/cache-files"
	cfg := Config{
		EnabledLevels: []Level{LevelFilesystem},
		FileCacheDir:  fsDir,
		FileMaxSize:   100 * 1024 * 1024,
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Directory should be created
	if _, err := os.Stat(fsDir); os.IsNotExist(err) {
		t.Error("filesystem cache dir was not created")
	}
	_ = c
}

// ── EstimateSize with non-JSON-marshalable ────────────────────────────────

func TestEstimateSizeNonJSON(t *testing.T) {
	// channel is not JSON-marshalable
	ch := make(chan int)
	size := estimateSize(ch)
	if size != 1024 {
		t.Errorf("estimateSize for channel = %d, want 1024", size)
	}
}

// ── Get with filesystem JSON decode ───────────────────────────────────────

func TestFilesystemCacheGetJSON(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		EnabledLevels: []Level{LevelFilesystem},
		FileCacheDir:  dir,
		FileMaxSize:   100 * 1024 * 1024,
	}
	c, _ := New(cfg)
	type obj struct {
		Name string
		Age  int
	}
	_ = c.Set("json-key", obj{Name: "Don", Age: 42}, time.Minute)
	val, ok := c.Get("json-key")
	if !ok {
		t.Fatal("Get should find json-key")
	}
	m, ok2 := val.(map[string]interface{})
	if !ok2 {
		t.Fatalf("expected map, got %T", val)
	}
	if m["Name"] != "Don" {
		t.Errorf("Name = %v", m["Name"])
	}
}
