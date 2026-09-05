package filesystem

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/CoscaAI/cosca/internal/chat"
)

// execTool executa uma tool pelo nome extraído da lista New().
func execTool(t *testing.T, tools []chat.Tool, name, params string) *chat.ToolResult {
	t.Helper()
	for _, tl := range tools {
		if tl.Name() == name {
			res, _ := tl.Execute(context.Background(), json.RawMessage(params))
			return res
		}
	}
	t.Fatalf("tool %q not found", name)
	return nil
}

// O EVIDENCE GATE do edit_file (invariante de segurança, ADR): o runtime nunca
// deixa o modelo transformar conteúdo inventado em mutação. Testa os 6 casos
// obrigatórios do professor.
func TestEditFileEvidenceGate(t *testing.T) {
	t.Run("sem read previo -> REJECT (EDIT_REQUIRES_READ)", func(t *testing.T) {
		ws := t.TempDir()
		writeFixture(t, ws, "a.txt", "line one\nline two\n")
		tools := New(ws)
		res := execTool(t, tools, "edit_file", `{"path":"a.txt","old_string":"line one","new_string":"line UNO"}`)
		require.NotNil(t, res)
		require.Contains(t, res.Error, "EDIT_REQUIRES_READ", "edit sem read deve ser rejeitado, got: %q", res.Error)
	})

	t.Run("read + old_string inexistente -> REJECT (EDIT_STALE_OR_UNVERIFIED)", func(t *testing.T) {
		ws := t.TempDir()
		writeFixture(t, ws, "b.txt", "alpha beta gamma\n")
		tools := New(ws)
		execTool(t, tools, "read_file", `{"path":"b.txt"}`)
		res := execTool(t, tools, "edit_file", `{"path":"b.txt","old_string":"delta","new_string":"x"}`)
		require.NotNil(t, res)
		require.Contains(t, res.Error, "EDIT_STALE_OR_UNVERIFIED", "old_string fora do snapshot deve ser rejeitado, got: %q", res.Error)
		data, _ := os.ReadFile(filepath.Join(ws, "b.txt"))
		require.Equal(t, "alpha beta gamma\n", string(data), "arquivo nao deve mudar")
	})

	t.Run("read + old_string valido -> ALLOW e muta", func(t *testing.T) {
		ws := t.TempDir()
		writeFixture(t, ws, "c.txt", "foo bar baz\n")
		tools := New(ws)
		execTool(t, tools, "read_file", `{"path":"c.txt"}`)
		res := execTool(t, tools, "edit_file", `{"path":"c.txt","old_string":"bar","new_string":"BAR"}`)
		require.NotNil(t, res)
		require.Empty(t, res.Error, "edit valido deve passar, got: %q", res.Error)
		data, _ := os.ReadFile(filepath.Join(ws, "c.txt"))
		require.Equal(t, "foo BAR baz\n", string(data))
	})

	t.Run("arquivo alterado depois do read -> REJECT (stale)", func(t *testing.T) {
		ws := t.TempDir()
		writeFixture(t, ws, "d.txt", "original content\n")
		tools := New(ws)
		execTool(t, tools, "read_file", `{"path":"d.txt"}`)
		// fonte de verdade diverge do snapshot observado
		writeFixture(t, ws, "d.txt", "changed content\n")
		res := execTool(t, tools, "edit_file", `{"path":"d.txt","old_string":"original","new_string":"x"}`)
		require.NotNil(t, res)
		require.Contains(t, res.Error, "EDIT_STALE_OR_UNVERIFIED", "arquivo alterado apos read deve ser rejeitado, got: %q", res.Error)
	})

	t.Run("erro de enforcement volta com codigo estruturado", func(t *testing.T) {
		ws := t.TempDir()
		writeFixture(t, ws, "e.txt", "content X\n")
		tools := New(ws)
		res := execTool(t, tools, "edit_file", `{"path":"e.txt","old_string":"X","new_string":"Y"}`)
		require.NotNil(t, res)
		require.Contains(t, res.Error, "EDIT_REQUIRES_READ")
	})

	t.Run("modelo consegue read->edit apos erro", func(t *testing.T) {
		ws := t.TempDir()
		writeFixture(t, ws, "f.txt", "hello world\n")
		tools := New(ws)
		// 1. edit sem read -> REJECT
		r1 := execTool(t, tools, "edit_file", `{"path":"f.txt","old_string":"hello","new_string":"HELLO"}`)
		require.Contains(t, r1.Error, "EDIT_REQUIRES_READ")
		// 2. read -> snapshot
		r2 := execTool(t, tools, "read_file", `{"path":"f.txt"}`)
		require.Empty(t, r2.Error)
		// 3. edit com old_string real -> SUCCESS
		r3 := execTool(t, tools, "edit_file", `{"path":"f.txt","old_string":"hello","new_string":"HELLO"}`)
		require.Empty(t, r3.Error, "apos read, edit valido deve funcionar, got: %q", r3.Error)
		data, _ := os.ReadFile(filepath.Join(ws, "f.txt"))
		require.Equal(t, "HELLO world\n", string(data))
	})
}
