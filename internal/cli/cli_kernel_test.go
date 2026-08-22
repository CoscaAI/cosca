//
// Tests for the `cosca kernel` command tree (internal/cli/kernel.go).
//
// Covers:
//   - Registration of `kernel` (and its 4 subcommands) in the root command
//   - `kernel identity` output (text + JSON) — name must be "Cosca Kernel"
//   - `kernel self-test` output (text + JSON) — all checks OK
//   - `kernel memory` / `kernel status` graceful degradation when the
//     knowledge base is absent (WARNING, not a fatal error)
//   - `kernel memory` / `kernel status` success path against a temporary
//     knowledge.db built with t.TempDir() (never the real .cosca/knowledge.db)
//   - resolveKnowledgeDB edge cases (found / not found / empty file)
//
// NOTE: tests that chdir() must NOT run in parallel — they mutate the
// process working directory.

package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	_ "modernc.org/sqlite" // pure-Go SQLite driver for the temp knowledge.db
)

// =============================================================================
// Registration — `kernel` in the root command
// =============================================================================

func TestKernelCommand_RegisteredInRoot(t *testing.T) {
	cmd := NewRootCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "kernel" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("kernel subcommand not registered in root command")
	}
}

func TestKernelCommand_Properties(t *testing.T) {
	cmd := NewKernelCommand()
	if cmd == nil {
		t.Fatal("NewKernelCommand returned nil")
	}
	if cmd.Use != "kernel" {
		t.Errorf("expected Use='kernel', got %q", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
	if cmd.Long == "" {
		t.Error("expected non-empty Long description")
	}

	expected := []string{"identity", "memory", "status", "self-test"}
	if len(cmd.Commands()) != len(expected) {
		t.Errorf("expected %d subcommands, got %d", len(expected), len(cmd.Commands()))
	}
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("missing kernel subcommand: %s", name)
		}
	}
}

func TestKernelCommand_HelpOutput_ContainsSubcommands(t *testing.T) {
	cmd := NewKernelCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	if err := cmd.Help(); err != nil {
		t.Fatalf("Help() returned error: %v", err)
	}
	output := buf.String()
	for _, sub := range []string{"identity", "memory", "status", "self-test"} {
		if !strings.Contains(output, sub) {
			t.Errorf("help output missing subcommand %q; output:\n%s", sub, output)
		}
	}
}

func TestKernelSubcommands_NonNil(t *testing.T) {
	tests := []struct {
		name    string
		factory func() *cobra.Command
		use     string
	}{
		{"identity", NewKernelIdentityCommand, "identity"},
		{"memory", NewKernelMemoryCommand, "memory"},
		{"status", NewKernelStatusCommand, "status"},
		{"self-test", NewKernelSelfTestCommand, "self-test"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.factory()
			if cmd == nil {
				t.Fatalf("factory for %q returned nil", tt.name)
			}
			if cmd.Use != tt.use {
				t.Errorf("expected Use=%q, got %q", tt.use, cmd.Use)
			}
			if cmd.Short == "" {
				t.Error("expected non-empty Short description")
			}
			if cmd.RunE == nil {
				t.Error("expected RunE to be set")
			}
		})
	}
}

// =============================================================================
// `kernel identity`
// =============================================================================

func TestKernelIdentity_RunE_Text(t *testing.T) {
	globalFlags = GlobalFlags{} // avoid cross-test contamination

	cmd := NewKernelIdentityCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true) // noColor
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	output := buf.String()
	for _, want := range []string{
		"Cosca Kernel",       // name
		"Consigliere do Don", // role
		"Cosca v1.5.0",       // project + version
		"auto-detect",        // model
		"Kernel Version",     // version field label
		"L1",                 // first law
		"L6",                 // sixth law
		"P1",                 // first constitutional principle
		"P8",                 // eighth constitutional principle
	} {
		if !strings.Contains(output, want) {
			t.Errorf("identity output missing %q; output:\n%s", want, output)
		}
	}
}

func TestKernelIdentity_RunE_JSON(t *testing.T) {
	globalFlags = GlobalFlags{}

	cmd := NewKernelIdentityCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	cmd.PersistentFlags().BoolP("json", "j", false, "json output")
	if err := cmd.PersistentFlags().Set("json", "true"); err != nil {
		t.Fatalf("set json flag: %v", err)
	}

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	var id map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &id); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if id["name"] != "Cosca Kernel" {
		t.Errorf("json name = %v, want %q", id["name"], "Cosca Kernel")
	}
	if id["id"] != "cosca-kernel" {
		t.Errorf("json id = %v, want %q", id["id"], "cosca-kernel")
	}
	if id["version"] != "1.0.0" {
		t.Errorf("json version = %v, want %q", id["version"], "1.0.0")
	}
	if id["project"] != "Cosca" {
		t.Errorf("json project = %v, want %q", id["project"], "Cosca")
	}
	if _, ok := id["expertise"].([]interface{}); !ok {
		t.Errorf("expected expertise to be a JSON array, got %T", id["expertise"])
	}
}

// =============================================================================
// `kernel self-test`
// =============================================================================

func TestKernelSelfTest_RunE_AllOK(t *testing.T) {
	globalFlags = GlobalFlags{}

	cmd := NewKernelSelfTestCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	output := buf.String()
	for _, want := range []string{
		"Kernel Self-Test",
		"identity",
		"laws",
		"constitution",
		"go-runtime",
		"Kernel íntegro",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("self-test output missing %q; output:\n%s", want, output)
		}
	}
	if strings.Contains(output, "❌") {
		t.Errorf("self-test reported a failure:\n%s", output)
	}
}

func TestKernelSelfTest_RunE_JSON(t *testing.T) {
	globalFlags = GlobalFlags{}

	cmd := NewKernelSelfTestCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	cmd.PersistentFlags().BoolP("json", "j", false, "json output")
	if err := cmd.PersistentFlags().Set("json", "true"); err != nil {
		t.Fatalf("set json flag: %v", err)
	}

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	var checks []struct {
		Name   string `json:"name"`
		OK     bool   `json:"ok"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(buf.Bytes(), &checks); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if len(checks) < 4 {
		t.Errorf("expected at least 4 checks, got %d", len(checks))
	}
	for _, c := range checks {
		if !c.OK {
			t.Errorf("check %q not OK: %s", c.Name, c.Detail)
		}
	}
}

// =============================================================================
// End-to-end via root ExecuteContext
// =============================================================================

func TestKernelCommand_ExecuteContext_IdentityJSON(t *testing.T) {
	globalFlags = GlobalFlags{}

	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"kernel", "identity", "--json"})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, `"name": "Cosca Kernel"`) {
		t.Errorf("expected JSON identity output, got:\n%s", output)
	}
}

func TestKernelCommand_ExecuteContext_SelfTestJSON(t *testing.T) {
	globalFlags = GlobalFlags{}

	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"kernel", "self-test", "--json"})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, `"ok": true`) {
		t.Errorf("expected all checks ok:true, got:\n%s", output)
	}
}

func TestKernelCommand_ExecuteContext_SelfTest_RunsWithoutError(t *testing.T) {
	globalFlags = GlobalFlags{}

	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	// Text output goes through the formatter (os.Stdout); we assert the
	// command resolves and runs without error.
	root.SetArgs([]string{"kernel", "self-test"})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}
}

// =============================================================================
// Graceful degradation — knowledge.db absent (WARNING, not fatal)
// =============================================================================

func TestKernelMemory_RunE_MissingDB_Graceful(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	globalFlags = GlobalFlags{}
	cmd := NewKernelMemoryCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("expected graceful degradation (warning, nil error), got error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Knowledge base unavailable") {
		t.Errorf("expected WARNING about missing knowledge base, got output: %q", output)
	}
}

func TestKernelStatus_RunE_MissingDB_Graceful(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	globalFlags = GlobalFlags{}
	cmd := NewKernelStatusCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("expected graceful degradation (warning, nil error), got error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Knowledge base unavailable") {
		t.Errorf("expected WARNING about missing knowledge base, got output: %q", output)
	}
}

// TestKernelStatus_RunE_CorruptDB_NoFatalError verifies that a corrupt
// .cosca/knowledge.db (exists but is not a valid SQLite database) degrades
// gracefully: resolveKnowledgeDB() accepts it (size > 0) and OpenMemory()'s
// Ping succeeds, so the failure surfaces from Stats() and must be reported
// as a WARNING with a nil error, never a fatal error.
func TestKernelStatus_RunE_CorruptDB_NoFatalError(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	coscaDir := filepath.Join(tmpDir, ".cosca")
	if err := os.MkdirAll(coscaDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(coscaDir, "knowledge.db"), []byte("this is not a sqlite database"), 0o644); err != nil {
		t.Fatalf("write corrupt db: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	globalFlags = GlobalFlags{}
	cmd := NewKernelStatusCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("expected graceful degradation (warning, nil error), got error: %v", err)
	}
	// The corrupt DB must degrade to a warning, never a fatal error. Newer
	// modernc.org/sqlite validates the file header at Ping() (so the failure
	// surfaces as "Cannot open kernel memory") while older versions deferred
	// it to Stats() ("Knowledge base corrupt"). Both are graceful.
	if !strings.Contains(buf.String(), "Knowledge base") &&
		!strings.Contains(buf.String(), "Cannot open kernel memory") {
		t.Errorf("expected a graceful warning about the knowledge base, got output: %q", buf.String())
	}
}

// =============================================================================
// Success path — temp knowledge.db built with t.TempDir()
// =============================================================================

func TestKernelStatus_RunE_WithTempDB(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".cosca", "knowledge.db")
	createKernelTestDB(t, dbPath)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	globalFlags = GlobalFlags{}
	cmd := NewKernelStatusCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	output := buf.String()
	for _, want := range []string{"Memory Status", "operational", "documents", "chunks", "knowledge_entries"} {
		if !strings.Contains(output, want) {
			t.Errorf("status output missing %q; output:\n%s", want, output)
		}
	}
}

func TestKernelStatus_RunE_WithTempDB_JSON(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".cosca", "knowledge.db")
	createKernelTestDB(t, dbPath)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	globalFlags = GlobalFlags{}
	cmd := NewKernelStatusCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	cmd.PersistentFlags().BoolP("json", "j", false, "json output")
	if err := cmd.PersistentFlags().Set("json", "true"); err != nil {
		t.Fatalf("set json flag: %v", err)
	}

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	var stats map[string]int
	if err := json.Unmarshal(buf.Bytes(), &stats); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if stats["documents"] < 1 {
		t.Errorf("expected documents >= 1, got %d", stats["documents"])
	}
	if stats["knowledge_entries"] < 1 {
		t.Errorf("expected knowledge_entries >= 1, got %d", stats["knowledge_entries"])
	}
}

func TestKernelMemory_RunE_WithTempDB(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".cosca", "knowledge.db")
	createKernelTestDB(t, dbPath)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	globalFlags = GlobalFlags{}
	cmd := NewKernelMemoryCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	output := buf.String()
	for _, want := range []string{
		"Kernel Memory",
		"Documents",
		"Chunks",
		"Amostra de Learnings", // seeded learning document
	} {
		if !strings.Contains(output, want) {
			t.Errorf("memory output missing %q; output:\n%s", want, output)
		}
	}
}

// =============================================================================
// resolveKnowledgeDB
// =============================================================================

func TestResolveKnowledgeDB_Found(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".cosca", "knowledge.db")
	createKernelTestDB(t, dbPath)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	got, err := resolveKnowledgeDB()
	if err != nil {
		t.Fatalf("resolveKnowledgeDB returned error: %v", err)
	}
	if got != dbPath {
		t.Errorf("resolveKnowledgeDB = %q, want %q", got, dbPath)
	}
}

func TestResolveKnowledgeDB_NotFound(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	_, err := resolveKnowledgeDB()
	if err == nil {
		t.Fatal("expected error when knowledge.db is absent")
	}
	if !strings.Contains(err.Error(), "knowledge.db not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestResolveKnowledgeDB_EmptyFileIgnored(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	coscaDir := filepath.Join(tmpDir, ".cosca")
	if err := os.MkdirAll(coscaDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Empty file (0 bytes) must be ignored (resolveKnowledgeDB requires size > 0).
	if err := os.WriteFile(filepath.Join(coscaDir, "knowledge.db"), nil, 0o644); err != nil {
		t.Fatalf("write empty db: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	if _, err := resolveKnowledgeDB(); err == nil {
		t.Fatal("expected error for empty knowledge.db file")
	}
}

func TestResolveKnowledgeDB_EnvOverride(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "custom-knowledge.db")
	createKernelTestDB(t, envPath)

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	t.Setenv("COSCA_KNOWLEDGE_DB", envPath)
	got, err := resolveKnowledgeDB()
	if err != nil {
		t.Fatalf("resolveKnowledgeDB with env: %v", err)
	}
	if got != envPath {
		t.Errorf("resolveKnowledgeDB = %q, want env path %q", got, envPath)
	}
}

func TestResolveKnowledgeDB_EnvMissing(t *testing.T) {
	tmpDir := t.TempDir()

	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	t.Setenv("COSCA_KNOWLEDGE_DB", filepath.Join(tmpDir, "missing.db"))
	_, err := resolveKnowledgeDB()
	if err == nil {
		t.Fatal("expected error when COSCA_KNOWLEDGE_DB points to a missing file")
	}
	if !strings.Contains(err.Error(), "COSCA_KNOWLEDGE_DB") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// =============================================================================
// Helpers
// =============================================================================

// createKernelTestDB creates a minimal SQLite knowledge base at dbPath with
// the tables the kernel memory layer expects (documents, chunks, vectors,
// knowledge_entries), seeded with one learning document and one knowledge
// entry. It does NOT depend on the real .cosca/knowledge.db.
func createKernelTestDB(t *testing.T, dbPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()

	// Mirror the real knowledge base schema (plus the vectors table that
	// kernel.Memory.Stats depends on).
	schema := []string{
		`CREATE TABLE IF NOT EXISTS documents (
			id TEXT PRIMARY KEY, path TEXT, hash TEXT, title TEXT,
			doc_type TEXT, metadata_json TEXT, frontmatter_json TEXT,
			size INTEGER, token_count INTEGER, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE IF NOT EXISTS chunks (
			id TEXT PRIMARY KEY, document_id TEXT, content TEXT, heading TEXT,
			section_type TEXT, position INTEGER, hash TEXT, token_count INTEGER,
			metadata_json TEXT, embedding BLOB)`,
		`CREATE TABLE IF NOT EXISTS vectors (
			id TEXT PRIMARY KEY, vector BLOB NOT NULL,
			metadata TEXT NOT NULL DEFAULT '{}',
			document_id TEXT NOT NULL DEFAULT '',
			chunk_id TEXT NOT NULL DEFAULT '',
			entity_id TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now')))`,
		`CREATE TABLE IF NOT EXISTS knowledge_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT, category TEXT NOT NULL,
			sub_category TEXT NOT NULL DEFAULT '', title TEXT NOT NULL,
			content TEXT NOT NULL, tags TEXT NOT NULL DEFAULT '[]',
			confidence REAL NOT NULL DEFAULT 0.5, source TEXT NOT NULL,
			updated_at TEXT NOT NULL, content_hash TEXT NOT NULL)`,
	}
	for _, s := range schema {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}

	seed := []string{
		`INSERT INTO documents (id, path, title, doc_type) VALUES
			('doc-learn-1', '/proj/.cosca/memory/agent/cosca-kernel/learnings.md',
			 'L41 — Restauração de Memória', 'markdown')`,
		`INSERT INTO chunks (id, document_id, content, heading) VALUES
			('chunk-1', 'doc-learn-1', 'Backup recovery com diff direcional', 'L41')`,
		`INSERT INTO knowledge_entries (category, title, content, tags, confidence, source, updated_at, content_hash) VALUES
			('heuristics', 'H-001', 'Segurança sem teste é fachada', '["security"]', 0.95, 'test', '2026-07-31', 'h1')`,
	}
	for _, s := range seed {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("seed data: %v", err)
		}
	}
}
