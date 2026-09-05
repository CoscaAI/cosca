package knowledge

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// ── Helpers ──────────────────────────────────────────────────────────────────

// newTestDB opens an in-memory SQLite database for testing.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err, "open in-memory db should succeed")
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// createTempRepo creates a temporary knowledge repository structure.
// Returns the path to a directory containing category subdirectories.
func createTempRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	return root
}

// writeFile writes a file in the repo and returns its full path.
func writeRepoFile(t *testing.T, repoPath, relPath, content string) string {
	t.Helper()
	fullPath := filepath.Join(repoPath, relPath)
	err := os.MkdirAll(filepath.Dir(fullPath), 0o755)
	require.NoError(t, err, "create directories for test file")
	err = os.WriteFile(fullPath, []byte(content), 0o644)
	require.NoError(t, err, "write test file")
	return fullPath
}

// ── TestNewCompiler ──────────────────────────────────────────────────────────

func TestNewCompiler(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	compiler := NewCompiler(db, "/some/repo/path")

	assert.NotNil(t, compiler, "compiler should not be nil")
	assert.Equal(t, db, compiler.db, "db should match")
	assert.Equal(t, "/some/repo/path", compiler.repoPath, "repoPath should match")
}

func TestNewCompiler_NilDB(t *testing.T) {
	t.Parallel()

	// NewCompiler accepts any *sql.DB — nil is technically possible but would
	// cause panics on use. The constructor itself doesn't validate.
	compiler := NewCompiler(nil, "/repo")
	assert.NotNil(t, compiler)
	assert.Nil(t, compiler.db)
}

// ── TestCompile_EmptyRepo ────────────────────────────────────────────────────

func TestCompile_EmptyRepo(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repoPath := createTempRepo(t)
	compiler := NewCompiler(db, repoPath)

	ctx := context.Background()
	result, err := compiler.Compile(ctx)

	require.NoError(t, err, "compile on empty repo should succeed")
	require.NotNil(t, result)

	assert.Equal(t, 0, result.Total, "no files to compile")
	assert.Equal(t, 0, result.New)
	assert.Equal(t, 0, result.Updated)
	assert.Equal(t, 0, result.Skipped)
	assert.Empty(t, result.Errors)
	assert.GreaterOrEqual(t, result.Duration, int64(0))
}

// ── TestCompileCategory_Heuristics ───────────────────────────────────────────

func TestCompileCategory_Heuristics(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repoPath := createTempRepo(t)

	// Create a heuristics directory with sample YAML files
	heuristicYAML := `id: H-001
title: "Security Without Testing"
domain: testing
type: anti-pattern
severity: critical
description: >
  Security code without test coverage is invisible and unreviewed.
tags: [security, testing, coverage, anti-pattern]
confidence: 0.95
created: 2026-07-29
`
	writeRepoFile(t, repoPath, "heuristics/H-001-security.yaml", heuristicYAML)

	heuristicYAML2 := `id: H-002
title: "CI Threshold Is Truth"
domain: ci-cd
type: pattern
severity: high
description: >
  CI thresholds define reality — push back when they fail.
tags: [ci, quality, gates]
confidence: 0.90
`
	writeRepoFile(t, repoPath, "heuristics/H-002-ci-threshold.yaml", heuristicYAML2)

	compiler := NewCompiler(db, repoPath)

	ctx := context.Background()
	result, err := compiler.CompileCategory(ctx, CategoryHeuristics)

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 2, result.Total, "should compile 2 heuristics")
	assert.Equal(t, 2, result.New, "both should be new")
	assert.Equal(t, 0, result.Updated)
	assert.Equal(t, 0, result.Skipped)
	assert.Empty(t, result.Errors)

	// Verify entries exist in the database
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM knowledge_entries WHERE category = 'heuristics'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Verify entry details
	var title, subCategory string
	var confidence float64
	err = db.QueryRow(
		"SELECT title, sub_category, confidence FROM knowledge_entries WHERE source = ?",
		"heuristics/H-001-security.yaml",
	).Scan(&title, &subCategory, &confidence)
	require.NoError(t, err)
	assert.Equal(t, "Security Without Testing", title)
	assert.Equal(t, "testing", subCategory)
	assert.InDelta(t, 0.95, confidence, 0.01)

	// Verify tags are stored as JSON
	var tagsJSON string
	err = db.QueryRow(
		"SELECT tags FROM knowledge_entries WHERE source = ?",
		"heuristics/H-001-security.yaml",
	).Scan(&tagsJSON)
	require.NoError(t, err)
	assert.Contains(t, tagsJSON, "security")
	assert.Contains(t, tagsJSON, "testing")
	assert.Contains(t, tagsJSON, "coverage")

	// Verify FTS index is populated
	var ftsCount int
	err = db.QueryRow("SELECT COUNT(*) FROM knowledge_fts WHERE category = 'heuristics'").Scan(&ftsCount)
	require.NoError(t, err)
	assert.Equal(t, 2, ftsCount, "FTS should have 2 entries")
}

// ── TestCompileCategory_Nonexistent ──────────────────────────────────────────

func TestCompileCategory_Nonexistent(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repoPath := createTempRepo(t)
	compiler := NewCompiler(db, repoPath)

	ctx := context.Background()
	result, err := compiler.CompileCategory(ctx, CategoryArchitecture)

	require.NoError(t, err, "compile on nonexistent category should succeed")
	require.NotNil(t, result)

	assert.Equal(t, 0, result.Total, "no files to compile")
	assert.Equal(t, 0, result.New)
	assert.Equal(t, 0, result.Updated)
	assert.Equal(t, 0, result.Skipped)
}

// ── TestContentHash_NoChange ─────────────────────────────────────────────────

func TestContentHash_NoChange(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repoPath := createTempRepo(t)

	// Create a single pattern file
	patternMD := `---
type: pattern
tags: [architecture, design]
confidence: 0.8
---

# Repository Pattern

Use the repository pattern to abstract data access.
`
	writeRepoFile(t, repoPath, "patterns/repository-pattern.md", patternMD)

	compiler := NewCompiler(db, repoPath)
	ctx := context.Background()

	// First compile — should insert new
	result1, err := compiler.CompileCategory(ctx, CategoryPatterns)
	require.NoError(t, err)
	assert.Equal(t, 1, result1.Total)
	assert.Equal(t, 1, result1.New)
	assert.Equal(t, 0, result1.Skipped)
	assert.Equal(t, 0, result1.Updated)

	// Second compile — file unchanged, should skip
	result2, err := compiler.CompileCategory(ctx, CategoryPatterns)
	require.NoError(t, err)
	assert.Equal(t, 1, result2.Total, "should still count the file")
	assert.Equal(t, 0, result2.New, "nothing new")
	assert.Equal(t, 0, result2.Updated, "nothing updated")
	assert.Equal(t, 1, result2.Skipped, "should be skipped as unchanged")

	// Verify only one entry in DB
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM knowledge_entries WHERE category = 'patterns'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "only one entry should exist")
}

// ── TestContentHash_Changed ──────────────────────────────────────────────────

func TestContentHash_Changed(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repoPath := createTempRepo(t)

	filePath := writeRepoFile(t, repoPath, "patterns/changed.md", `---
type: pattern
---

# Original Title

Original content.
`)

	compiler := NewCompiler(db, repoPath)
	ctx := context.Background()

	// First compile
	result1, err := compiler.CompileCategory(ctx, CategoryPatterns)
	require.NoError(t, err)
	assert.Equal(t, 1, result1.New)

	// Modify the file
	updatedContent := `---
type: pattern
---

# Updated Title

Updated content with more text.
`
	err = os.WriteFile(filePath, []byte(updatedContent), 0o644)
	require.NoError(t, err)

	// Second compile — should detect change and update
	result2, err := compiler.CompileCategory(ctx, CategoryPatterns)
	require.NoError(t, err)
	assert.Equal(t, 1, result2.Total)
	assert.Equal(t, 0, result2.New)
	assert.Equal(t, 1, result2.Updated, "should be detected as updated")
	assert.Equal(t, 0, result2.Skipped)

	// Verify title was updated in DB
	var title string
	err = db.QueryRow(
		"SELECT title FROM knowledge_entries WHERE source = ?", "patterns/changed.md",
	).Scan(&title)
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", title)
}

// ── TestCompileCategory_Patterns ─────────────────────────────────────────────

func TestCompileCategory_Patterns(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repoPath := createTempRepo(t)

	// Create a pattern with YAML frontmatter
	patternMD := `---
type: pattern
key: pattern-cqrs
tags: [architecture, cqrs, scalability]
category: architecture
confidence: 0.85
---

# CQRS Pattern

Command Query Responsibility Segregation separates reads from writes.

## Intent

Scale read and write operations independently.
`
	writeRepoFile(t, repoPath, "patterns/cqrs.md", patternMD)

	compiler := NewCompiler(db, repoPath)
	ctx := context.Background()

	result, err := compiler.CompileCategory(ctx, CategoryPatterns)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, 1, result.New)

	// Verify frontmatter extraction
	var title, subCategory, tagsJSON string
	var confidence float64
	err = db.QueryRow(
		"SELECT title, sub_category, tags, confidence FROM knowledge_entries WHERE source = ?",
		"patterns/cqrs.md",
	).Scan(&title, &subCategory, &tagsJSON, &confidence)
	require.NoError(t, err)

	assert.Equal(t, "CQRS Pattern", title)
	assert.Equal(t, "architecture", subCategory)
	assert.Contains(t, tagsJSON, "architecture")
	assert.Contains(t, tagsJSON, "cqrs")
	assert.InDelta(t, 0.85, confidence, 0.01)

	// Verify content does NOT include frontmatter
	var content string
	err = db.QueryRow(
		"SELECT content FROM knowledge_entries WHERE source = ?",
		"patterns/cqrs.md",
	).Scan(&content)
	require.NoError(t, err)
	assert.NotContains(t, content, "---", "content should not contain frontmatter delimiters")
	assert.Contains(t, content, "# CQRS Pattern", "content should start with heading")
}

// ── TestCompileCategory_NoFrontmatter ────────────────────────────────────────

func TestCompileCategory_NoFrontmatter(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repoPath := createTempRepo(t)

	// Markdown without frontmatter
	plainMD := `# Simple Pattern

This pattern has no YAML frontmatter.

## Details

Just plain markdown content.
`
	writeRepoFile(t, repoPath, "best-practices/simple.md", plainMD)

	compiler := NewCompiler(db, repoPath)
	ctx := context.Background()

	result, err := compiler.CompileCategory(ctx, CategoryBestPractices)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)

	var title string
	var confidence float64
	err = db.QueryRow(
		"SELECT title, confidence FROM knowledge_entries WHERE source = ?",
		"best-practices/simple.md",
	).Scan(&title, &confidence)
	require.NoError(t, err)

	assert.Equal(t, "Simple Pattern", title, "should extract title from # heading")
	assert.InDelta(t, 0.5, confidence, 0.01, "default confidence should be 0.5")
}

// ── TestCompileCategory_IgnoresNonContent ────────────────────────────────────

func TestCompileCategory_IgnoresNonContent(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repoPath := createTempRepo(t)

	// Write a .gitkeep file and a JSON file
	writeRepoFile(t, repoPath, "heuristics/.gitkeep", "")
	writeRepoFile(t, repoPath, "heuristics/data.json", `{"key": "value"}`)

	compiler := NewCompiler(db, repoPath)
	ctx := context.Background()

	result, err := compiler.CompileCategory(ctx, CategoryHeuristics)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Total, "no supported files should be compiled")

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM knowledge_entries").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// ── TestCompile_MultipleCategories ───────────────────────────────────────────

func TestCompile_MultipleCategories(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repoPath := createTempRepo(t)

	// Create files in multiple categories
	writeRepoFile(t, repoPath, "heuristics/h1.yaml", `id: H1
title: "Heuristic One"
domain: test
tags: [t1]
`)
	writeRepoFile(t, repoPath, "patterns/p1.md", `# Pattern One

Pattern content here.
`)
	writeRepoFile(t, repoPath, "architecture/adr-001.md", `# ADR 001: Use PostgreSQL

Decision to use PostgreSQL for relational data.
`)

	compiler := NewCompiler(db, repoPath)
	ctx := context.Background()

	result, err := compiler.Compile(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 3, result.Total, "should compile files from all categories")
	assert.Equal(t, 3, result.New)
	assert.Equal(t, 0, result.Updated)
	assert.Equal(t, 0, result.Skipped)

	// Verify categories in result
	assert.Contains(t, result.Categories, "heuristics")
	assert.Contains(t, result.Categories, "patterns")
	assert.Contains(t, result.Categories, "architecture")

	// Verify counts per category in DB
	var heuristicsCount, patternsCount, architectureCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM knowledge_entries WHERE category = 'heuristics'").Scan(&heuristicsCount)
	_ = db.QueryRow("SELECT COUNT(*) FROM knowledge_entries WHERE category = 'patterns'").Scan(&patternsCount)
	_ = db.QueryRow("SELECT COUNT(*) FROM knowledge_entries WHERE category = 'architecture'").Scan(&architectureCount)

	assert.Equal(t, 1, heuristicsCount)
	assert.Equal(t, 1, patternsCount)
	assert.Equal(t, 1, architectureCount)
}

// ── TestCompile_FTSIndexing ──────────────────────────────────────────────────

func TestCompile_FTSIndexing(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repoPath := createTempRepo(t)

	writeRepoFile(t, repoPath, "best-practices/error-handling.md", `# Error Handling Best Practices

Always wrap errors with context. Never ignore errors silently.

Use `+"`fmt.Errorf`"+` with `+"`%w`"+` verb for error wrapping.
`)
	writeRepoFile(t, repoPath, "best-practices/logging.md", `# Logging Best Practices

Use structured logging with zerolog. Include trace IDs in all log entries.
`)

	compiler := NewCompiler(db, repoPath)
	ctx := context.Background()

	_, err := compiler.CompileCategory(ctx, CategoryBestPractices)
	require.NoError(t, err)

	// Search FTS index
	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM knowledge_fts WHERE knowledge_fts MATCH ?",
		"error",
	).Scan(&count)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 1, "should find 'error' in FTS")

	// Search for content in another file
	err = db.QueryRow(
		"SELECT COUNT(*) FROM knowledge_fts WHERE knowledge_fts MATCH ?",
		"logging",
	).Scan(&count)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 1, "should find 'logging' in FTS")

	// Search by category — FTS5 requires hyphens to be quoted
	err = db.QueryRow(
		"SELECT COUNT(*) FROM knowledge_fts WHERE category MATCH ?",
		`"best-practices"`,
	).Scan(&count)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 1, "should match category")
}

// ── TestCompile_ContextCancellation ──────────────────────────────────────────

func TestCompile_ContextCancellation(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repoPath := createTempRepo(t)

	// Create many files to give the walk time to notice cancellation
	for i := 0; i < 50; i++ {
		writeRepoFile(t, repoPath,
			filepath.Join("patterns", "p"+string(rune('a'+i%26))+".md"),
			"# Pattern\n\nContent.",
		)
	}

	compiler := NewCompiler(db, repoPath)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := compiler.Compile(ctx)
	assert.Error(t, err, "should error with cancelled context")
	assert.Contains(t, err.Error(), "cancel", "error should mention cancellation")
}

// ── TestSchemaIdempotent ─────────────────────────────────────────────────────

func TestSchemaIdempotent(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	compiler := NewCompiler(db, "/nonexistent")

	// initSchema should succeed multiple times
	err := compiler.initSchema()
	require.NoError(t, err)

	err = compiler.initSchema()
	require.NoError(t, err, "initSchema should be idempotent")

	// Verify tables exist
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'knowledge_entries'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'knowledge_fts'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify triggers exist
	var triggerCount int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name LIKE 'knowledge_entries_%'",
	).Scan(&triggerCount)
	require.NoError(t, err)
	assert.Equal(t, 3, triggerCount, "should have ai, ad, au triggers")
}

// ── TestComputeContentHash ───────────────────────────────────────────────────

func TestComputeContentHash(t *testing.T) {
	t.Parallel()

	h1 := computeContentHash("hello world")
	h2 := computeContentHash("hello world")
	h3 := computeContentHash("different content")

	assert.Equal(t, h1, h2, "same content should produce same hash")
	assert.NotEqual(t, h1, h3, "different content should produce different hash")
	assert.Len(t, h1, 64, "SHA-256 produces 64 hex characters")
}

// ── TestExtractHeadingTitle ──────────────────────────────────────────────────

func TestExtractHeadingTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "standard h1",
			content:  "# My Title\n\nContent here.",
			expected: "My Title",
		},
		{
			name:     "h1 with spaces",
			content:  "#   Extra Spaces  \n",
			expected: "Extra Spaces",
		},
		{
			name:     "h1 after content",
			content:  "Some text\n\n# The Real Title\nMore content.",
			expected: "The Real Title",
		},
		{
			name:     "no heading",
			content:  "Just some text without a heading.",
			expected: "",
		},
		{
			name:     "h2 before h1",
			content:  "## Subtitle\n\n# Main Title\n\nContent.",
			expected: "Main Title",
		},
		{
			name:     "empty content",
			content:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractHeadingTitle(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ── TestExtractTags ──────────────────────────────────────────────────────────

func TestExtractTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    map[string]interface{}
		expected []string
	}{
		{
			name:     "no tags key",
			input:    map[string]interface{}{"title": "test"},
			expected: nil,
		},
		{
			name: "yaml list of strings",
			input: map[string]interface{}{
				"tags": []interface{}{"a", "b", "c"},
			},
			expected: []string{"a", "b", "c"},
		},
		{
			name: "string slice (already parsed)",
			input: map[string]interface{}{
				"tags": []string{"x", "y"},
			},
			expected: []string{"x", "y"},
		},
		{
			name: "comma-separated string",
			input: map[string]interface{}{
				"tags": "one, two, three",
			},
			expected: []string{"one", " two", " three"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTags(tt.input)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// ── TestAllCategories ────────────────────────────────────────────────────────

func TestAllCategories(t *testing.T) {
	t.Parallel()

	cats := AllCategories()
	assert.Len(t, cats, 7)
	assert.Contains(t, cats, CategoryPatterns)
	assert.Contains(t, cats, CategoryHeuristics)
	assert.Contains(t, cats, CategoryArchitecture)
	assert.Contains(t, cats, CategoryFailures)
	assert.Contains(t, cats, CategoryBestPractices)
	assert.Contains(t, cats, CategoryCognitive)
}

// ── TestKnowledgeEntry_Defaults ─────────────────────────────────────────────────────

func TestKnowledgeEntry_Defaults(t *testing.T) {
	t.Parallel()

	entry := &KnowledgeEntry{}
	assert.Equal(t, KnowledgeCategory(""), entry.Category)
	assert.Equal(t, "", entry.Title)
	assert.Equal(t, float64(0), entry.Confidence)
	assert.Nil(t, entry.Tags)
	assert.True(t, entry.UpdatedAt.IsZero())
}

func TestKnowledgeEntry_WithValues(t *testing.T) {
	t.Parallel()

	now := time.Now()
	entry := &KnowledgeEntry{
		Category:    CategoryHeuristics,
		SubCategory: "testing",
		Title:       "Test Entry",
		Content:     "Some content",
		Tags:        []string{"go", "testing"},
		Confidence:  0.9,
		Source:      "heuristics/h-001.yaml",
		UpdatedAt:   now,
	}

	assert.Equal(t, CategoryHeuristics, entry.Category)
	assert.Equal(t, "testing", entry.SubCategory)
	assert.Equal(t, "Test Entry", entry.Title)
	assert.Equal(t, []string{"go", "testing"}, entry.Tags)
	assert.Equal(t, 0.9, entry.Confidence)
	assert.Equal(t, "heuristics/h-001.yaml", entry.Source)
	assert.Equal(t, now, entry.UpdatedAt)
}

// ── TestCompileResult_Defaults ─────────────────────────────────────────────────────────────────────

func TestCompileResult_Defaults(t *testing.T) {
	t.Parallel()

	r := &CompileResult{}
	assert.Equal(t, 0, r.Total)
	assert.Equal(t, 0, r.New)
	assert.Equal(t, 0, r.Updated)
	assert.Equal(t, 0, r.Skipped)
	assert.Nil(t, r.Categories)
	assert.Nil(t, r.Errors)
}

func TestCompileResult_WithValues(t *testing.T) {
	t.Parallel()

	r := &CompileResult{
		Total:      10,
		New:        5,
		Updated:    3,
		Skipped:    2,
		Categories: []string{"heuristics", "patterns"},
		Errors:     []string{"parse error: file.md"},
		Duration:   150,
	}

	assert.Equal(t, 10, r.Total)
	assert.Equal(t, 5, r.New)
	assert.Equal(t, 3, r.Updated)
	assert.Equal(t, 2, r.Skipped)
	assert.Len(t, r.Categories, 2)
	assert.Len(t, r.Errors, 1)
	assert.Equal(t, int64(150), r.Duration)
}

// ── TestCompileCategory_YAMLWithoutTitle ─────────────────────────────────────

func TestCompileCategory_YAMLWithoutTitle(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repoPath := createTempRepo(t)

	// YAML with only an ID, no title field
	yamlContent := `id: H-099
domain: unknown
type: unknown
description: No explicit title.
tags: [test]
`
	writeRepoFile(t, repoPath, "heuristics/no-title.yaml", yamlContent)

	compiler := NewCompiler(db, repoPath)
	ctx := context.Background()

	result, err := compiler.CompileCategory(ctx, CategoryHeuristics)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)

	var title string
	err = db.QueryRow(
		"SELECT title FROM knowledge_entries WHERE source = ?",
		"heuristics/no-title.yaml",
	).Scan(&title)
	require.NoError(t, err)
	assert.Equal(t, "H-099", title, "should fall back to ID when title is missing")
}

// ── TestCompileCategory_MarkdownWithoutHeading ───────────────────────────────

func TestCompileCategory_MarkdownWithoutHeading(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repoPath := createTempRepo(t)

	// Markdown file without a # heading — should use filename as title
	mdContent := `This markdown file has no heading at all.

Just some plain text content.
`
	writeRepoFile(t, repoPath, "patterns/no-heading.md", mdContent)

	compiler := NewCompiler(db, repoPath)
	ctx := context.Background()

	result, err := compiler.CompileCategory(ctx, CategoryPatterns)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)

	var title string
	err = db.QueryRow(
		"SELECT title FROM knowledge_entries WHERE source = ?",
		"patterns/no-heading.md",
	).Scan(&title)
	require.NoError(t, err)
	assert.Equal(t, "no-heading", title, "should use filename (without extension) as title")
}

// ── TestCompileCategory_SkipsDirectories ─────────────────────────────────────

func TestCompileCategory_SkipsDirectories(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repoPath := createTempRepo(t)

	// Create a subdirectory with a file — the subdirectory itself is skipped
	writeRepoFile(t, repoPath, "patterns/subdir/nested.md", "# Nested Pattern\n\nNested content.")

	compiler := NewCompiler(db, repoPath)
	ctx := context.Background()

	result, err := compiler.CompileCategory(ctx, CategoryPatterns)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total, "file in subdirectory should be found")

	var content string
	err = db.QueryRow(
		"SELECT content FROM knowledge_entries WHERE source = ?",
		"patterns/subdir/nested.md",
	).Scan(&content)
	require.NoError(t, err)
	assert.Contains(t, content, "Nested content")
}

// ── TestCompileCategory_EmptyFile ────────────────────────────────────────────

func TestCompileCategory_EmptyFile(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repoPath := createTempRepo(t)

	writeRepoFile(t, repoPath, "patterns/empty.md", "")

	compiler := NewCompiler(db, repoPath)
	ctx := context.Background()

	result, err := compiler.CompileCategory(ctx, CategoryPatterns)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total, "empty file should still be processed")

	var title string
	err = db.QueryRow(
		"SELECT title FROM knowledge_entries WHERE source = ?",
		"patterns/empty.md",
	).Scan(&title)
	require.NoError(t, err)
	assert.Equal(t, "empty", title, "should use filename for empty markdown")

	// Verify the entry has EOL content_hash
	var content string
	var hash string
	err = db.QueryRow(
		"SELECT content, content_hash FROM knowledge_entries WHERE source = ?",
		"patterns/empty.md",
	).Scan(&content, &hash)
	require.NoError(t, err)
	// verify hash for empty content is deterministic
	assert.NotEmpty(t, hash)
	assert.Len(t, hash, 64)
}

// ── TestCompileCategory_UnsupportedExtension ────────────────────────────────

func TestCompileCategory_UnsupportedExtension(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repoPath := createTempRepo(t)

	writeRepoFile(t, repoPath, "patterns/readme.txt", "This is a text file.")
	writeRepoFile(t, repoPath, "patterns/data.xml", "<root>xml</root>")

	compiler := NewCompiler(db, repoPath)
	ctx := context.Background()

	result, err := compiler.CompileCategory(ctx, CategoryPatterns)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Total, "unsupported extensions should be ignored")

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM knowledge_entries").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// ── TestCompileCategory_UpdatedAt ────────────────────────────────────────────

func TestCompileCategory_UpdatedAt(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repoPath := createTempRepo(t)

	writeRepoFile(t, repoPath, "heuristics/timestamp.yaml", `id: TS-1
title: "Timestamp Test"
domain: test
description: Check updated_at.
tags: []
`)

	compiler := NewCompiler(db, repoPath)
	ctx := context.Background()

	before := time.Now().Truncate(time.Second)
	_, err := compiler.CompileCategory(ctx, CategoryHeuristics)
	require.NoError(t, err)

	var updatedAtStr string
	err = db.QueryRow(
		"SELECT updated_at FROM knowledge_entries WHERE source = ?",
		"heuristics/timestamp.yaml",
	).Scan(&updatedAtStr)
	require.NoError(t, err)

	updatedAt, err := time.Parse(time.RFC3339, updatedAtStr)
	require.NoError(t, err)
	assert.False(t, updatedAt.Before(before),
		"updated_at should not be before compile start time")
}

// ── TestCompileCategory_FilePathPreservation ─────────────────────────────────

func TestCompileCategory_FilePathPreservation(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repoPath := createTempRepo(t)

	// File paths should be relative to repo root, using forward slashes
	writeRepoFile(t, repoPath, filepath.Join("patterns", "deep", "nested", "file.md"),
		"# Deep Nested\n\nContent.",
	)

	compiler := NewCompiler(db, repoPath)
	ctx := context.Background()

	_, err := compiler.CompileCategory(ctx, CategoryPatterns)
	require.NoError(t, err)

	var source string
	err = db.QueryRow(
		"SELECT source FROM knowledge_entries WHERE category = 'patterns'",
	).Scan(&source)
	require.NoError(t, err)

	// On Linux, filepath.Rel produces paths with forward separators
	// but on Windows it's backslashes. Just check the essential parts.
	assert.True(t,
		strings.Contains(source, "deep") && strings.Contains(source, "nested") && strings.Contains(source, "file.md"),
		"source path should contain the nested path elements, got: %s", source,
	)
}

// ── TestCompileResult_SuccessAllCategories ───────────────────────────────────

func TestCompileResult_SuccessAllCategories(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	repoPath := createTempRepo(t)

	// Create files in ALL six categories
	writeRepoFile(t, repoPath, "patterns/p1.md", "# Pattern 1\n\nContent.")
	writeRepoFile(t, repoPath, "heuristics/h1.yaml", `id: H1
title: "H1"
domain: test
`)
	writeRepoFile(t, repoPath, "architecture/adr.md", "# ADR 1\n\nDecision.")
	writeRepoFile(t, repoPath, "failures/postmortem.md", "# Incident Report\n\nWhat happened.")
	writeRepoFile(t, repoPath, "best-practices/guide.md", "# Best Practice\n\nGuidance.")
	writeRepoFile(t, repoPath, "cognitive/framework.md", "# Cognitive Framework\n\nMental model.")

	compiler := NewCompiler(db, repoPath)
	ctx := context.Background()

	result, err := compiler.Compile(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 6, result.Total)
	assert.Equal(t, 6, result.New)
	assert.Equal(t, 0, result.Updated)
	assert.Equal(t, 0, result.Skipped)
	assert.Empty(t, result.Errors)

	// All seven categories should be in the result
	assert.Len(t, result.Categories, 7)

	// Verify DB counts per category
	categories := map[string]int{
		"patterns":       1,
		"heuristics":     1,
		"architecture":   1,
		"failures":       1,
		"best-practices": 1,
		"cognitive":      1,
		"acquired":       0,
	}
	for cat, expected := range categories {
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM knowledge_entries WHERE category = ?", cat).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, expected, count, "category %s should have %d entry", cat, expected)
	}
}

// ── Benchmark ────────────────────────────────────────────────────────────────

func BenchmarkCompiler_CompileCategory(b *testing.B) {
	for i := 0; b.Loop(); i++ {
		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			b.Fatal(err)
		}

		repoPath := b.TempDir()
		os.MkdirAll(filepath.Join(repoPath, "heuristics"), 0o755)

		for j := 0; j < 10; j++ {
			content := fmt.Sprintf(`id: H-%03d
title: "Heuristic %d"
domain: benchmark
description: Just a benchmark file.
tags: [bench, test]
`, j, j)
			os.WriteFile(
				filepath.Join(repoPath, "heuristics", fmt.Sprintf("h-%03d.yaml", j)),
				[]byte(content), 0o644,
			)
		}

		compiler := NewCompiler(db, repoPath)
		_, _ = compiler.CompileCategory(context.Background(), CategoryHeuristics)
		_ = db.Close()
	}
}
