// Package tool provides tests for the filesystem tools: ReadTool, WriteTool,
// EditTool, and GlobTool. Tests use a mock path validator and a temporary
// directory as the simulated workspace.
package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Mock Path Validator ─────────────────────────────────────────────────────

// mockValidator implements pathValidator for testing. It validates paths
// against a workspace root, rejecting paths that escape the workspace.
type mockValidator struct {
	workspace string
	// failAll causes Validate to always fail when true.
	failAll bool
}

func (m *mockValidator) Validate(path string) error {
	if m.failAll {
		return fmt.Errorf("validation forced failure")
	}
	clean := filepath.Clean(filepath.Join(m.workspace, path))
	ws := filepath.Clean(m.workspace)
	if !strings.HasPrefix(clean, ws+string(filepath.Separator)) && clean != ws {
		return fmt.Errorf("path %q escapes workspace", path)
	}
	return nil
}

// newWorkspace creates a temp directory and returns its path together with
// a mockValidator bound to it.
func newWorkspace(t *testing.T) (string, *mockValidator) {
	t.Helper()
	ws := t.TempDir()
	return ws, &mockValidator{workspace: ws}
}

// writeTestFile writes content to a file under the given workspace root.
func writeTestFile(t *testing.T, workspace, relPath, content string) string {
	t.Helper()
	fullPath := filepath.Join(workspace, relPath)
	err := os.MkdirAll(filepath.Dir(fullPath), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(fullPath, []byte(content), 0644)
	require.NoError(t, err)
	return fullPath
}

// ─── ReadTool ────────────────────────────────────────────────────────────────

func TestReadTool_Execute(t *testing.T) {
	t.Parallel()

	t.Run("reads existing file", func(t *testing.T) {
		ws, val := newWorkspace(t)
		writeTestFile(t, ws, "hello.txt", "Hello, World!")
		tool := NewReadTool(ws, val)

		result, _ := tool.Execute(context.Background(), json.RawMessage(`{"path":"hello.txt"}`))
		assert.Equal(t, "Hello, World!", result.Output)
		assert.Empty(t, result.Error)
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		ws, val := newWorkspace(t)
		tool := NewReadTool(ws, val)

		result, _ := tool.Execute(context.Background(), json.RawMessage(`{"path":"nonexistent.txt"}`))
		assert.Empty(t, result.Output)
		assert.Contains(t, result.Error, "read failed")
		// A mensagem do OS subjacente é localizada no Windows ("O sistema não
		// pode encontrar o arquivo especificado"), então a asserção do texto
		// em inglês só vale em sistemas POSIX.
		if runtime.GOOS != "windows" {
			assert.Contains(t, result.Error, "no such file")
		}
	})

	t.Run("rejects path outside workspace", func(t *testing.T) {
		ws, val := newWorkspace(t)
		tool := NewReadTool(ws, val)

		result, _ := tool.Execute(context.Background(), json.RawMessage(`{"path":"../../etc/passwd"}`))
		assert.Empty(t, result.Output)
		assert.Contains(t, result.Error, "escapes workspace")
	})

	t.Run("rejects invalid JSON params", func(t *testing.T) {
		ws, val := newWorkspace(t)
		tool := NewReadTool(ws, val)

		result, _ := tool.Execute(context.Background(), json.RawMessage(`{invalid}`))
		assert.Empty(t, result.Output)
		assert.Contains(t, result.Error, "invalid params")
	})

	t.Run("reads file in subdirectory", func(t *testing.T) {
		ws, val := newWorkspace(t)
		writeTestFile(t, ws, "sub/dir/deep.txt", "deep content")
		tool := NewReadTool(ws, val)

		result, _ := tool.Execute(context.Background(), json.RawMessage(`{"path":"sub/dir/deep.txt"}`))
		assert.Equal(t, "deep content", result.Output)
		assert.Empty(t, result.Error)
	})
}

func TestReadTool_Validate(t *testing.T) {
	t.Parallel()

	ws, val := newWorkspace(t)
	tool := NewReadTool(ws, val)

	t.Run("valid params pass", func(t *testing.T) {
		err := tool.Validate(json.RawMessage(`{"path":"file.txt"}`))
		assert.NoError(t, err)
	})

	t.Run("missing path fails", func(t *testing.T) {
		err := tool.Validate(json.RawMessage(`{}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path is required")
	})

	t.Run("empty path fails", func(t *testing.T) {
		err := tool.Validate(json.RawMessage(`{"path":""}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path is required")
	})

	t.Run("invalid JSON fails", func(t *testing.T) {
		err := tool.Validate(json.RawMessage(`{`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid params")
	})
}

func TestReadTool_Metadata(t *testing.T) {
	t.Parallel()

	ws, val := newWorkspace(t)
	tool := NewReadTool(ws, val)

	assert.Equal(t, "read", tool.Name())
	assert.Equal(t, "Read the contents of a file", tool.Description())
	assert.NotEmpty(t, tool.Schema())
}

// ─── WriteTool ───────────────────────────────────────────────────────────────

func TestWriteTool_Execute(t *testing.T) {
	t.Parallel()

	t.Run("creates new file", func(t *testing.T) {
		ws, val := newWorkspace(t)
		tool := NewWriteTool(ws, val)

		result, _ := tool.Execute(context.Background(), json.RawMessage(`{"path":"new.txt","content":"new content"}`))
		assert.Equal(t, "file written successfully", result.Output)
		assert.Empty(t, result.Error)

		data, err := os.ReadFile(filepath.Join(ws, "new.txt"))
		require.NoError(t, err)
		assert.Equal(t, "new content", string(data))
	})

	t.Run("overwrites existing file and creates backup", func(t *testing.T) {
		ws, val := newWorkspace(t)
		writeTestFile(t, ws, "existing.txt", "original content")
		tool := NewWriteTool(ws, val)

		result, _ := tool.Execute(context.Background(), json.RawMessage(`{"path":"existing.txt","content":"updated content"}`))
		assert.Equal(t, "file written successfully", result.Output)

		// Verify content was updated
		data, err := os.ReadFile(filepath.Join(ws, "existing.txt"))
		require.NoError(t, err)
		assert.Equal(t, "updated content", string(data))

		// Verify backup was created
		backupData, err := os.ReadFile(filepath.Join(ws, "existing.txt.bak"))
		require.NoError(t, err)
		assert.Equal(t, "original content", string(backupData))
	})

	t.Run("writes file in subdirectory", func(t *testing.T) {
		ws, val := newWorkspace(t)
		// os.WriteFile does not create parent directories, so create them first.
		err := os.MkdirAll(filepath.Join(ws, "a", "b", "c"), 0o755)
		require.NoError(t, err)
		tool := NewWriteTool(ws, val)

		result, _ := tool.Execute(context.Background(), json.RawMessage(`{"path":"a/b/c/nested.txt","content":"nested"}`))
		assert.Equal(t, "file written successfully", result.Output)

		data, err := os.ReadFile(filepath.Join(ws, "a/b/c/nested.txt"))
		require.NoError(t, err)
		assert.Equal(t, "nested", string(data))
	})

	t.Run("rejects path outside workspace", func(t *testing.T) {
		ws, val := newWorkspace(t)
		tool := NewWriteTool(ws, val)

		result, _ := tool.Execute(context.Background(), json.RawMessage(`{"path":"../../tmp/evil.txt","content":"bad"}`))
		assert.Empty(t, result.Output)
		assert.Contains(t, result.Error, "escapes workspace")
	})

	t.Run("rejects invalid JSON params", func(t *testing.T) {
		ws, val := newWorkspace(t)
		tool := NewWriteTool(ws, val)

		result, _ := tool.Execute(context.Background(), json.RawMessage(`{invalid}`))
		assert.Empty(t, result.Output)
		assert.Contains(t, result.Error, "invalid params")
	})
}

func TestWriteTool_Validate(t *testing.T) {
	t.Parallel()

	ws, val := newWorkspace(t)
	tool := NewWriteTool(ws, val)

	t.Run("valid params pass", func(t *testing.T) {
		err := tool.Validate(json.RawMessage(`{"path":"f.txt","content":"data"}`))
		assert.NoError(t, err)
	})

	t.Run("missing path fails", func(t *testing.T) {
		err := tool.Validate(json.RawMessage(`{"content":"data"}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path is required")
	})

	t.Run("missing content fails", func(t *testing.T) {
		err := tool.Validate(json.RawMessage(`{"path":"f.txt"}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "content is required")
	})

	t.Run("empty content fails", func(t *testing.T) {
		err := tool.Validate(json.RawMessage(`{"path":"f.txt","content":""}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "content is required")
	})

	t.Run("invalid JSON fails", func(t *testing.T) {
		err := tool.Validate(json.RawMessage(`{`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid params")
	})
}

func TestWriteTool_Metadata(t *testing.T) {
	t.Parallel()

	ws, val := newWorkspace(t)
	tool := NewWriteTool(ws, val)

	assert.Equal(t, "write", tool.Name())
	assert.Contains(t, tool.Description(), "Write or overwrite")
	assert.NotEmpty(t, tool.Schema())
}

// ─── EditTool ────────────────────────────────────────────────────────────────

func TestEditTool_Execute(t *testing.T) {
	t.Parallel()

	t.Run("replaces content correctly", func(t *testing.T) {
		ws, val := newWorkspace(t)
		writeTestFile(t, ws, "edit.txt", "Hello, World!")
		tool := NewEditTool(ws, val)

		result, _ := tool.Execute(context.Background(), json.RawMessage(`{"path":"edit.txt","old_string":"World","new_string":"Cosca"}`))
		assert.Equal(t, "file edited successfully", result.Output)
		assert.Empty(t, result.Error)

		data, err := os.ReadFile(filepath.Join(ws, "edit.txt"))
		require.NoError(t, err)
		assert.Equal(t, "Hello, Cosca!", string(data))
	})

	t.Run("returns error if old_string not found", func(t *testing.T) {
		ws, val := newWorkspace(t)
		writeTestFile(t, ws, "greeting.txt", "Hello, World!")
		tool := NewEditTool(ws, val)

		result, _ := tool.Execute(context.Background(), json.RawMessage(`{"path":"greeting.txt","old_string":"Nonexistent","new_string":"replacement"}`))
		assert.Empty(t, result.Output)
		assert.Contains(t, result.Error, "old_string not found")
	})

	t.Run("returns error if old_string appears multiple times", func(t *testing.T) {
		ws, val := newWorkspace(t)
		writeTestFile(t, ws, "repeat.txt", "foo bar foo baz")
		tool := NewEditTool(ws, val)

		result, _ := tool.Execute(context.Background(), json.RawMessage(`{"path":"repeat.txt","old_string":"foo","new_string":"qux"}`))
		assert.Empty(t, result.Output)
		assert.Contains(t, result.Error, "appears 2 times")
	})

	t.Run("rejects path outside workspace", func(t *testing.T) {
		ws, val := newWorkspace(t)
		tool := NewEditTool(ws, val)

		result, _ := tool.Execute(context.Background(), json.RawMessage(`{"path":"../../etc/passwd","old_string":"root","new_string":"user"}`))
		assert.Empty(t, result.Output)
		assert.Contains(t, result.Error, "escapes workspace")
	})

	t.Run("rejects invalid JSON params", func(t *testing.T) {
		ws, val := newWorkspace(t)
		tool := NewEditTool(ws, val)

		result, _ := tool.Execute(context.Background(), json.RawMessage(`{invalid}`))
		assert.Empty(t, result.Output)
		assert.Contains(t, result.Error, "invalid params")
	})

	t.Run("edits file in subdirectory", func(t *testing.T) {
		ws, val := newWorkspace(t)
		writeTestFile(t, ws, "sub/deep/note.txt", "before text")
		tool := NewEditTool(ws, val)

		result, _ := tool.Execute(context.Background(), json.RawMessage(`{"path":"sub/deep/note.txt","old_string":"before","new_string":"after"}`))
		assert.Equal(t, "file edited successfully", result.Output)

		data, err := os.ReadFile(filepath.Join(ws, "sub/deep/note.txt"))
		require.NoError(t, err)
		assert.Equal(t, "after text", string(data))
	})
}

func TestEditTool_Validate(t *testing.T) {
	t.Parallel()

	ws, val := newWorkspace(t)
	tool := NewEditTool(ws, val)

	t.Run("valid params pass", func(t *testing.T) {
		err := tool.Validate(json.RawMessage(`{"path":"f.txt","old_string":"old","new_string":"new"}`))
		assert.NoError(t, err)
	})

	t.Run("missing path fails", func(t *testing.T) {
		err := tool.Validate(json.RawMessage(`{"old_string":"o","new_string":"n"}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path is required")
	})

	t.Run("missing old_string fails", func(t *testing.T) {
		err := tool.Validate(json.RawMessage(`{"path":"f.txt","new_string":"n"}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "old_string is required")
	})

	t.Run("empty old_string fails", func(t *testing.T) {
		err := tool.Validate(json.RawMessage(`{"path":"f.txt","old_string":"","new_string":"n"}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "old_string is required")
	})

	t.Run("invalid JSON fails", func(t *testing.T) {
		err := tool.Validate(json.RawMessage(`{`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid params")
	})
}

func TestEditTool_Metadata(t *testing.T) {
	t.Parallel()

	ws, val := newWorkspace(t)
	tool := NewEditTool(ws, val)

	assert.Equal(t, "edit", tool.Name())
	assert.Contains(t, tool.Description(), "find-and-replace")
	assert.NotEmpty(t, tool.Schema())
}

// ─── GlobTool ────────────────────────────────────────────────────────────────

func TestGlobTool_Execute(t *testing.T) {
	t.Parallel()

	t.Run("finds files by pattern", func(t *testing.T) {
		ws, val := newWorkspace(t)
		writeTestFile(t, ws, "file1.txt", "a")
		writeTestFile(t, ws, "file2.txt", "b")
		writeTestFile(t, ws, "main.go", "c")
		tool := NewGlobTool(ws, val)

		result, _ := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"*.txt"}`))
		assert.Empty(t, result.Error)

		lines := strings.Split(result.Output, "\n")
		assert.ElementsMatch(t, []string{"file1.txt", "file2.txt"}, lines)
	})

	t.Run("returns no matches found", func(t *testing.T) {
		ws, val := newWorkspace(t)
		writeTestFile(t, ws, "main.go", "hello")
		tool := NewGlobTool(ws, val)

		result, _ := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"*.py"}`))
		assert.Empty(t, result.Error)
		assert.Equal(t, "no matches found", result.Output)
	})

	t.Run("finds files in subdirectory", func(t *testing.T) {
		ws, val := newWorkspace(t)
		writeTestFile(t, ws, "sub/a.go", "a")
		writeTestFile(t, ws, "sub/b.go", "b")
		writeTestFile(t, ws, "other.txt", "c")
		tool := NewGlobTool(ws, val)

		result, _ := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"*.go","path":"sub"}`))
		assert.Empty(t, result.Error)

		lines := strings.Split(result.Output, "\n")
		// O glob devolve paths com o separador nativo do OS (filepath.Rel),
		// então o expected usa filepath.Join para casar no Windows (\\) e no
		// Unix (/).
		assert.ElementsMatch(t, []string{filepath.Join("sub", "a.go"), filepath.Join("sub", "b.go")}, lines)
	})

	t.Run("rejects absolute pattern", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("semântica de path absoluto POSIX (pattern \"/etc/*\") — não aplicável no Windows")
		}
		ws, val := newWorkspace(t)
		tool := NewGlobTool(ws, val)

		result, _ := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"/etc/*"}`))
		assert.Empty(t, result.Output)
		assert.Contains(t, result.Error, "pattern must be relative")
	})

	t.Run("rejects invalid JSON params", func(t *testing.T) {
		ws, val := newWorkspace(t)
		tool := NewGlobTool(ws, val)

		result, _ := tool.Execute(context.Background(), json.RawMessage(`{invalid}`))
		assert.Empty(t, result.Output)
		assert.Contains(t, result.Error, "invalid params")
	})

	t.Run("empty pattern returns error", func(t *testing.T) {
		ws, val := newWorkspace(t)
		tool := NewGlobTool(ws, val)

		result, _ := tool.Execute(context.Background(), json.RawMessage(`{"pattern":""}`))
		assert.Contains(t, result.Error, "pattern is required")
	})

	t.Run("pattern with no path defaults to workspace root", func(t *testing.T) {
		ws, val := newWorkspace(t)
		writeTestFile(t, ws, "root.txt", "root")
		writeTestFile(t, ws, "sub/nested.txt", "nested")
		tool := NewGlobTool(ws, val)

		// Only root-level .txt files
		result, _ := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"*.txt"}`))
		assert.Empty(t, result.Error)
		assert.Equal(t, "root.txt", strings.TrimSpace(result.Output))
	})

	t.Run("rejects path outside workspace", func(t *testing.T) {
		ws, val := newWorkspace(t)
		tool := NewGlobTool(ws, val)

		result, _ := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"*.go","path":"../../etc"}`))
		assert.Contains(t, result.Error, "escapes workspace")
	})
}

func TestGlobTool_Validate(t *testing.T) {
	t.Parallel()

	ws, val := newWorkspace(t)
	tool := NewGlobTool(ws, val)

	t.Run("valid params pass", func(t *testing.T) {
		err := tool.Validate(json.RawMessage(`{"pattern":"*.go"}`))
		assert.NoError(t, err)
	})

	t.Run("missing pattern fails", func(t *testing.T) {
		err := tool.Validate(json.RawMessage(`{}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pattern is required")
	})

	t.Run("empty pattern fails", func(t *testing.T) {
		err := tool.Validate(json.RawMessage(`{"pattern":""}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pattern is required")
	})

	t.Run("invalid JSON fails", func(t *testing.T) {
		err := tool.Validate(json.RawMessage(`{`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid params")
	})
}

func TestGlobTool_Metadata(t *testing.T) {
	t.Parallel()

	ws, val := newWorkspace(t)
	tool := NewGlobTool(ws, val)

	assert.Equal(t, "glob", tool.Name())
	assert.Contains(t, tool.Description(), "glob")
	assert.NotEmpty(t, tool.Schema())
}

// ─── Error result helper ─────────────────────────────────────────────────────

func TestErrorResult(t *testing.T) {
	t.Parallel()

	result := errorResult("something went wrong")
	assert.Equal(t, "something went wrong", result.Error)
	assert.Empty(t, result.Output)
	assert.Zero(t, result.Duration)
}

// ─── mustMarshalSchema ───────────────────────────────────────────────────────

func TestMustMarshalSchema(t *testing.T) {
	t.Parallel()

	t.Run("marshals valid schema", func(t *testing.T) {
		schema := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
		}
		result := mustMarshalSchema(schema)
		assert.Contains(t, string(result), `"type":"object"`)
		assert.Contains(t, string(result), `"name"`)
	})

	t.Run("panics on un-marshalable value", func(t *testing.T) {
		assert.Panics(t, func() {
			mustMarshalSchema(map[string]any{
				"fn": func() {}, // not marshalable
			})
		})
	})
}

// ─── Tool interface conformance ──────────────────────────────────────────────

func TestReadTool_ImplementsTool(t *testing.T) {
	t.Parallel()
	ws, val := newWorkspace(t)
	var _ interface{ Name() string } = NewReadTool(ws, val)
}

func TestWriteTool_ImplementsTool(t *testing.T) {
	t.Parallel()
	ws, val := newWorkspace(t)
	var _ interface{ Name() string } = NewWriteTool(ws, val)
}

func TestEditTool_ImplementsTool(t *testing.T) {
	t.Parallel()
	ws, val := newWorkspace(t)
	var _ interface{ Name() string } = NewEditTool(ws, val)
}

func TestGlobTool_ImplementsTool(t *testing.T) {
	t.Parallel()
	ws, val := newWorkspace(t)
	var _ interface{ Name() string } = NewGlobTool(ws, val)
}
