// Package filesystem tests the cross-platform filesystem tools end to end
// against a real temporary workspace, including path-traversal rejection so
// an agent can never escape the project root.
package filesystem

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/chat/tool"
)

func exec(t *testing.T, tool interface {
	Execute(context.Context, json.RawMessage) (*chat.ToolResult, error)
}, params string) *chat.ToolResult {
	t.Helper()
	res, _ := tool.Execute(context.Background(), json.RawMessage(params))
	return res
}

// findToolHelper localiza uma tool pelo nome na lista devolvida por New().
func findToolHelper(t *testing.T, tools []chat.Tool, name string) interface {
	Execute(context.Context, json.RawMessage) (*chat.ToolResult, error)
} {
	t.Helper()
	for _, tl := range tools {
		if tl.Name() == name {
			return tl
		}
	}
	t.Fatalf("tool %q not found", name)
	return nil
}

// writeFixture writes content into a path under the workspace, creating parents.
func writeFixture(t *testing.T, ws, rel, content string) string {
	t.Helper()
	full := filepath.Join(ws, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	return full
}

func run(t *testing.T, name string, fn func(*testing.T)) {
	t.Run(name, func(t *testing.T) { fn(t) })
}

// ─── write_file ───────────────────────────────────────────────────────────────

func TestWriteFileTool(t *testing.T) {
	ws := t.TempDir()

	run(t, "creates new file", func(t *testing.T) {
		tl := NewWriteFileTool(ws)
		res := exec(t, tl, `{"path":"hello.go","content":"package main\n"}`)
		require.Empty(t, res.Error)
		assert.Contains(t, res.Output, "Successfully wrote")
		data, err := os.ReadFile(filepath.Join(ws, "hello.go"))
		require.NoError(t, err)
		assert.Equal(t, "package main\n", string(data))
	})

	run(t, "creates parent directories", func(t *testing.T) {
		tl := NewWriteFileTool(ws)
		res := exec(t, tl, `{"path":"src/deep/nested.txt","content":"nested"}`)
		require.Empty(t, res.Error)
		data, err := os.ReadFile(filepath.Join(ws, "src", "deep", "nested.txt"))
		require.NoError(t, err)
		assert.Equal(t, "nested", string(data))
	})

	run(t, "rejects path traversal ../", func(t *testing.T) {
		tl := NewWriteFileTool(ws)
		res := exec(t, tl, `{"path":"../fora.txt","content":"evil"}`)
		assert.Empty(t, res.Output)
		assert.Contains(t, res.Error, "escapes workspace")
	})

	run(t, "rejects traversal through subdir ../../", func(t *testing.T) {
		tl := NewWriteFileTool(ws)
		res := exec(t, tl, `{"path":"src/../../fora.txt","content":"evil"}`)
		assert.Contains(t, res.Error, "escapes workspace")
	})

	run(t, "rejects absolute path outside", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("absolute-outside path is POSIX-oriented")
		}
		tl := NewWriteFileTool(ws)
		res := exec(t, tl, `{"path":"/etc/evil.txt","content":"evil"}`)
		assert.Contains(t, res.Error, "escapes workspace")
	})

	run(t, "rejects invalid JSON", func(t *testing.T) {
		tl := NewWriteFileTool(ws)
		res := exec(t, tl, `{invalid}`)
		assert.Contains(t, res.Error, "invalid params")
	})
}

// ─── read_file ────────────────────────────────────────────────────────────────

func TestReadFileTool(t *testing.T) {
	ws := t.TempDir()

	run(t, "reads file content", func(t *testing.T) {
		writeFixture(t, ws, "note.txt", "Hello, World!")
		tl := NewReadFileTool(ws)
		res := exec(t, tl, `{"path":"note.txt"}`)
		require.Empty(t, res.Error)
		assert.Equal(t, "Hello, World!", res.Output)
	})

	run(t, "reads nested file", func(t *testing.T) {
		writeFixture(t, ws, "a/b/c.txt", "deep")
		tl := NewReadFileTool(ws)
		res := exec(t, tl, `{"path":"a/b/c.txt"}`)
		require.Empty(t, res.Error)
		assert.Equal(t, "deep", res.Output)
	})

	run(t, "rejects path traversal ../", func(t *testing.T) {
		tl := NewReadFileTool(ws)
		res := exec(t, tl, `{"path":"../fora.txt"}`)
		assert.Empty(t, res.Output)
		assert.Contains(t, res.Error, "escapes workspace")
	})

	run(t, "rejects directory read", func(t *testing.T) {
		require.NoError(t, os.MkdirAll(filepath.Join(ws, "dir"), 0o755))
		tl := NewReadFileTool(ws)
		res := exec(t, tl, `{"path":"dir"}`)
		assert.Contains(t, res.Error, "is a directory")
	})
}

// ─── edit_file ────────────────────────────────────────────────────────────────

func TestEditFileTool(t *testing.T) {
	ws := t.TempDir()

	run(t, "replaces text", func(t *testing.T) {
		writeFixture(t, ws, "edit.txt", "Hello, World!")
		tools := New(ws) // tracker compartilhado (read -> edit)
		exec(t, findToolHelper(t, tools, "read_file"), `{"path":"edit.txt"}`)
		tl := findToolHelper(t, tools, "edit_file")
		res := exec(t, tl, `{"path":"edit.txt","old_string":"World","new_string":"Cosca"}`)
		require.Empty(t, res.Error)
		data, _ := os.ReadFile(filepath.Join(ws, "edit.txt"))
		assert.Equal(t, "Hello, Cosca!", string(data))
	})

	run(t, "rejects old_string not found", func(t *testing.T) {
		writeFixture(t, ws, "a.txt", "abc")
		tools := New(ws)
		exec(t, findToolHelper(t, tools, "read_file"), `{"path":"a.txt"}`)
		tl := findToolHelper(t, tools, "edit_file")
		res := exec(t, tl, `{"path":"a.txt","old_string":"zzz","new_string":"x"}`)
		assert.Contains(t, res.Error, "EDIT_STALE_OR_UNVERIFIED")
	})

	run(t, "rejects multiple occurrences", func(t *testing.T) {
		writeFixture(t, ws, "b.txt", "foo bar foo")
		tools := New(ws)
		exec(t, findToolHelper(t, tools, "read_file"), `{"path":"b.txt"}`)
		tl := findToolHelper(t, tools, "edit_file")
		res := exec(t, tl, `{"path":"b.txt","old_string":"foo","new_string":"qux"}`)
		assert.Contains(t, res.Error, "appears 2 times")
	})

	run(t, "rejects path traversal ../", func(t *testing.T) {
		tl := NewEditFileTool(ws)
		res := exec(t, tl, `{"path":"../fora.txt","old_string":"a","new_string":"b"}`)
		assert.Contains(t, res.Error, "escapes workspace")
	})
}

// ─── list_dir ─────────────────────────────────────────────────────────────────

func TestListDirTool(t *testing.T) {
	ws := t.TempDir()
	writeFixture(t, ws, "file1.go", "a")
	writeFixture(t, ws, "file2.txt", "b")
	require.NoError(t, os.MkdirAll(filepath.Join(ws, "sub"), 0o755))

	run(t, "lists root entries", func(t *testing.T) {
		tl := NewListDirTool(ws)
		res := exec(t, tl, `{}`)
		require.Empty(t, res.Error)
		assert.Contains(t, res.Output, "file1.go")
		assert.Contains(t, res.Output, "file2.txt")
		assert.Contains(t, res.Output, "sub")
		assert.Contains(t, res.Output, "dir")
	})

	run(t, "lists subdirectory", func(t *testing.T) {
		writeFixture(t, ws, "sub/nested.go", "c")
		tl := NewListDirTool(ws)
		res := exec(t, tl, `{"path":"sub"}`)
		require.Empty(t, res.Error)
		assert.Contains(t, res.Output, "nested.go")
	})

	run(t, "rejects path traversal ../", func(t *testing.T) {
		tl := NewListDirTool(ws)
		res := exec(t, tl, `{"path":"../"}`)
		assert.Contains(t, res.Error, "escapes workspace")
	})
}

// ─── glob ─────────────────────────────────────────────────────────────────────

func TestGlobTool(t *testing.T) {
	ws := t.TempDir()
	writeFixture(t, ws, "file1.go", "a")
	writeFixture(t, ws, "file2.go", "b")
	writeFixture(t, ws, "main.txt", "c")
	writeFixture(t, ws, "sub/nested.go", "d")

	run(t, "finds by pattern at root", func(t *testing.T) {
		tl := NewGlobTool(ws)
		res := exec(t, tl, `{"pattern":"*.go"}`)
		require.Empty(t, res.Error)
		lines := strings.Split(res.Output, "\n")
		assert.ElementsMatch(t, []string{"file1.go", "file2.go"}, lines)
	})

	run(t, "finds by pattern in subdir", func(t *testing.T) {
		tl := NewGlobTool(ws)
		res := exec(t, tl, `{"pattern":"*.go","path":"sub"}`)
		require.Empty(t, res.Error)
		assert.Equal(t, filepath.Join("sub", "nested.go"), strings.TrimSpace(res.Output))
	})

	run(t, "returns no matches", func(t *testing.T) {
		tl := NewGlobTool(ws)
		res := exec(t, tl, `{"pattern":"*.py"}`)
		require.Empty(t, res.Error)
		assert.Equal(t, "no matches found", res.Output)
	})

	run(t, "rejects path traversal in base ../", func(t *testing.T) {
		tl := NewGlobTool(ws)
		res := exec(t, tl, `{"pattern":"*.go","path":"../../etc"}`)
		assert.Contains(t, res.Error, "escapes workspace")
	})

	run(t, "rejects path traversal in pattern ../../", func(t *testing.T) {
		tl := NewGlobTool(ws)
		res := exec(t, tl, `{"pattern":"../../evil/*.go"}`)
		assert.Contains(t, res.Error, "escapes workspace")
	})
}

// ─── Validator ────────────────────────────────────────────────────────────────

func TestValidator(t *testing.T) {
	ws := t.TempDir()

	run(t, "accepts in-workspace relative path", func(t *testing.T) {
		v := NewValidator(ws)
		resolved, err := v.Resolve("src/hello.go")
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(resolved, filepath.Clean(ws)+string(filepath.Separator)))
	})

	run(t, "rejects ../ traversal", func(t *testing.T) {
		v := NewValidator(ws)
		_, err := v.Resolve("../fora.txt")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "escapes workspace")
	})

	run(t, "rejects nested ../../ traversal", func(t *testing.T) {
		v := NewValidator(ws)
		_, err := v.Resolve("src/../../fora.txt")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "escapes workspace")
	})

	run(t, "rejects null byte", func(t *testing.T) {
		v := NewValidator(ws)
		_, err := v.Resolve("a\x00b.txt")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "null byte")
	})

	run(t, "rejects absolute path outside", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("absolute-outside path is POSIX-oriented")
		}
		v := NewValidator(ws)
		_, err := v.Resolve("/tmp/evil")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "escapes workspace")
	})
}

// ─── Registration & interface conformance ─────────────────────────────────────

func TestRegisterAndConformance(t *testing.T) {
	ws := t.TempDir()
	reg := tool.NewRegistry()
	tools := Register(reg, ws)

	require.Len(t, tools, 5)

	names := reg.List()
	assert.ElementsMatch(t, []string{"write_file", "read_file", "edit_file", "list_dir", "glob"}, names)

	assert.Len(t, reg.Definitions(), 5)
	// Every registered tool is executable through the registry.
	res := reg.Execute(context.Background(), "write_file", json.RawMessage(`{"path":"x.go","content":"code"}`))
	require.Empty(t, res.Error)
}
