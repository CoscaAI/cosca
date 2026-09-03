// Package filesystem — security regression tests.
//
// These tests verify the deterministic safety guard that makes the filesystem
// tools safe regardless of the LLM model: write_file must NEVER blindly
// overwrite existing code, and no tool may follow a symlink out of the
// workspace. These are the exact failure modes that allowed a model to delete
// production code.
package filesystem

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CoscaAI/cosca/internal/chat/tool"
)

// ─── Tool-level overwrite guard ───────────────────────────────────────────────

// simulates the disaster scenario: an existing file with real content, and a
// write_file of new content WITHOUT a prior read_file. It must FAIL and must
// PRESERVE the original content byte-for-byte.
func TestWriteFileGuard_DisasterScenario(t *testing.T) {
	ws := t.TempDir()
	tracker := newPathTracker()
	wf := newWriteFileTool(ws, tracker)
	rf := newReadFileTool(ws, tracker)

	const original = "package main\n\nfunc main() { println(\"solicitek-production\") }\n"
	writeFixture(t, ws, "solicitek.go", original)

	run(t, "refuses overwrite of existing file without prior read", func(t *testing.T) {
		res := exec(t, wf, `{"path":"solicitek.go","content":"DESTROYED"}`)

		// Must reject and NOT overwrite.
		assert.NotEmpty(t, res.Error)
		assert.Contains(t, res.Error, "already exists")

		// Original content preserved.
		data, err := os.ReadFile(filepath.Join(ws, "solicitek.go"))
		require.NoError(t, err)
		assert.Equal(t, original, string(data))
	})

	run(t, "read then write is allowed", func(t *testing.T) {
		// Explicit read_file of the same path (same session/tracker).
		r := exec(t, rf, `{"path":"solicitek.go"}`)
		require.Empty(t, r.Error, "read_file should succeed")
		assert.Equal(t, original, r.Output)

		res := exec(t, wf, `{"path":"solicitek.go","content":"REWRITTEN"}`)
		require.Empty(t, res.Error, "write_file after read should be allowed")
		data, err := os.ReadFile(filepath.Join(ws, "solicitek.go"))
		require.NoError(t, err)
		assert.Equal(t, "REWRITTEN", string(data))
	})
}

func TestWriteFileGuard_NewFileAllowed(t *testing.T) {
	ws := t.TempDir()
	tracker := newPathTracker()
	wf := newWriteFileTool(ws, tracker)

	run(t, "creating a brand new file is always allowed", func(t *testing.T) {
		res := exec(t, wf, `{"path":"brand/new/hello.txt","content":"fresh"}`)
		require.Empty(t, res.Error)
		data, err := os.ReadFile(filepath.Join(ws, "brand", "new", "hello.txt"))
		require.NoError(t, err)
		assert.Equal(t, "fresh", string(data))
	})
}

func TestWriteFileGuard_NewFileSuggestsSibling(t *testing.T) {
	// Item 2 (existence-before-write): creating a brand-new file must still be
	// allowed, but when a similar file already exists in the parent directory the
	// agent should be advised to read/edit it instead of creating a near-duplicate.
	ws := t.TempDir()
	tracker := newPathTracker()
	wf := newWriteFileTool(ws, tracker)

	writeFixture(t, ws, "cmd/user.go", "package cmd\n")

	run(t, "creation allowed but sibling is surfaced", func(t *testing.T) {
		res := exec(t, wf, `{"path":"cmd/main.go","content":"package cmd\n"}`)
		require.Empty(t, res.Error, "creating a new file must not be blocked")
		assert.Contains(t, res.Output, "Successfully wrote")
		assert.Contains(t, res.Output, "user.go", "existing sibling should be surfaced as a hint")
		data, err := os.ReadFile(filepath.Join(ws, "cmd", "main.go"))
		require.NoError(t, err)
		assert.Equal(t, "package cmd\n", string(data))
	})
}

func TestWriteFileGuard_ReadThenWriteAcrossNewTools(t *testing.T) {
	// The scenario the executor actually uses: all tools are built by New() and
	// share one read tracker, so read_file + write_file interoperate.
	ws := t.TempDir()
	tools := New(ws)

	var wf *WriteFileTool
	var rf *ReadFileTool
	for _, tl := range tools {
		switch tt := tl.(type) {
		case *WriteFileTool:
			wf = tt
		case *ReadFileTool:
			rf = tt
		}
	}
	require.NotNil(t, wf)
	require.NotNil(t, rf)

	writeFixture(t, ws, "notes.txt", "hello")
	require.Empty(t, exec(t, rf, `{"path":"notes.txt"}`).Error)

	res := exec(t, wf, `{"path":"notes.txt","content":"world"}`)
	require.Empty(t, res.Error)
	data, _ := os.ReadFile(filepath.Join(ws, "notes.txt"))
	assert.Equal(t, "world", string(data))
}

// ─── Symlink escape (tool level) ──────────────────────────────────────────────

func TestSymlinkEscapeRejected(t *testing.T) {
	ws := t.TempDir()
	outside := t.TempDir()
	writeFixture(t, outside, "secret.txt", "TOP SECRET")

	link := filepath.Join(ws, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink not permitted on this platform: %v", err)
	}

	tracker := newPathTracker()
	wf := newWriteFileTool(ws, tracker)
	rf := newReadFileTool(ws, tracker)
	ef := newEditFileTool(ws, tracker)

	run(t, "read_file rejects symlink escaping the workspace", func(t *testing.T) {
		res := exec(t, rf, `{"path":"escape/secret.txt"}`)
		assert.NotEmpty(t, res.Error)
		assert.Contains(t, res.Error, "symlink")
		// The outside secret must NOT be returned.
		assert.NotContains(t, res.Output, "TOP SECRET")
	})

	run(t, "write_file rejects symlink escaping the workspace", func(t *testing.T) {
		res := exec(t, wf, `{"path":"escape/new.txt","content":"evil"}`)
		assert.NotEmpty(t, res.Error)
		// Nothing must be written outside.
		_, err := os.Stat(filepath.Join(outside, "new.txt"))
		assert.Error(t, err, "no file should be created outside the workspace")
	})

	run(t, "edit_file rejects symlink escaping the workspace", func(t *testing.T) {
		res := exec(t, ef, `{"path":"escape/secret.txt","old_string":"SECRET","new_string":"LEAK"}`)
		assert.NotEmpty(t, res.Error)
	})

	run(t, "in-workspace symlink is still allowed", func(t *testing.T) {
		// A symlink that stays inside the workspace must not be blocked.
		realDir := filepath.Join(ws, "real")
		require.NoError(t, os.MkdirAll(realDir, 0o755))
		writeFixture(t, ws, "real/inner.txt", "inside")
		alias := filepath.Join(ws, "alias")
		if err := os.Symlink(realDir, alias); err != nil {
			return
		}
		res := exec(t, rf, `{"path":"alias/inner.txt"}`)
		require.Empty(t, res.Error)
		assert.Equal(t, "inside", res.Output)
	})
}

// ─── Edit-then-write also authorizes overwrite ────────────────────────────────

func TestEditFileAuthorizesOverwrite(t *testing.T) {
	ws := t.TempDir()
	tracker := newPathTracker()
	wf := newWriteFileTool(ws, tracker)
	ef := newEditFileTool(ws, tracker)
	rf := newReadFileTool(ws, tracker)

	writeFixture(t, ws, "doc.txt", "alpha")
	// EVIDENCE GATE: edit_file exige read_file antes (a invariante).
	// read_file registra o snapshot; depois edit_file valida old_string contra ele.
	require.Empty(t, exec(t, rf, `{"path":"doc.txt"}`).Error)
	require.Empty(t, exec(t, ef, `{"path":"doc.txt","old_string":"alpha","new_string":"beta"}`).Error)

	// After a successful edit the path is "seen" — a write_file overwrite is ok.
	res := exec(t, wf, `{"path":"doc.txt","content":"gamma"}`)
	require.Empty(t, res.Error)
	data, _ := os.ReadFile(filepath.Join(ws, "doc.txt"))
	assert.Equal(t, "gamma", string(data))
}

// ─── Disaster simulation via the registry wiring the executor uses ────────────

func TestWriteFileGuard_RegistryDoesNotDestroy(t *testing.T) {
	ws := t.TempDir()
	reg := tool.NewRegistry()
	Register(reg, ws)

	const originalCode = "func Handle() int { return 7 }"
	writeFixture(t, ws, "api/service.go", originalCode)

	// The model calls write_file directly on an existing file without reading.
	res := reg.Execute(context.Background(), "write_file",
		json.RawMessage(`{"path":"api/service.go","content":"func Handle() int { return 1 }"}`))
	assert.NotEmpty(t, res.Error)
	assert.Contains(t, res.Error, "already exists")

	// Original code must be intact.
	data, err := os.ReadFile(filepath.Join(ws, "api/service.go"))
	require.NoError(t, err)
	assert.Equal(t, originalCode, string(data))
}
