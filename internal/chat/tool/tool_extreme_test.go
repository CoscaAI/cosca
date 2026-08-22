package tool

import (
	"context"
	"encoding/json"
	"testing"
)

// ── P1-3: Tool output extremo ───────────────────────────────────────────

func mustParams(t *testing.T, s string) json.RawMessage {
	t.Helper()
	return json.RawMessage(s)
}

func TestTool_ReadHugeFileDoesNotBreak(t *testing.T) {
	ws := t.TempDir()
	big := make([]byte, 5*1024*1024) // 5MB
	for i := range big {
		big[i] = byte('a' + i%26)
	}
	writeTestFile(t, ws, "big.txt", string(big))

	rt := NewReadTool(ws, &mockValidator{workspace: ws})
	res, err := rt.Execute(context.Background(), mustParams(t, `{"path":"big.txt"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Error != "" {
		t.Fatalf("huge file read failed: %q", res.Error)
	}
	if len(res.Output) != len(big) {
		t.Fatalf("huge file output truncated: got %d, want %d", len(res.Output), len(big))
	}
}

func TestTool_WriteEmptyAndHugeContent(t *testing.T) {
	ws := t.TempDir()
	wt := NewWriteTool(ws, &mockValidator{workspace: ws})

	// Conteúdo vazio: o Validate recusa (content obrigatório).
	if err := wt.Validate(mustParams(t, `{"path":"a.txt","content":""}`)); err == nil {
		t.Fatal("empty content must fail validation")
	}
	// Conteúdo gigante.
	huge := make([]byte, 2*1024*1024)
	for i := range huge {
		huge[i] = 'x'
	}
	res, err := wt.Execute(context.Background(), mustParams(t, `{"path":"big-out.txt","content":"`+string(huge)+`"}`))
	if err != nil {
		t.Fatalf("Execute huge write: %v", err)
	}
	if res.Error != "" {
		t.Fatalf("huge write failed: %q", res.Error)
	}
}

func TestTool_EditOutputGarbage(t *testing.T) {
	ws := t.TempDir()
	writeTestFile(t, ws, "f.txt", "abcabc")
	et := NewEditTool(ws, &mockValidator{workspace: ws})

	// old_string com múltiplas ocorrências → erro claro (não corrompe).
	res, err := et.Execute(context.Background(), mustParams(t, `{"path":"f.txt","old_string":"abc","new_string":"x"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Error == "" {
		t.Fatal("edit with ambiguous old_string must fail (appears 2x)")
	}
	// old_string inexistente → erro claro.
	res, _ = et.Execute(context.Background(), mustParams(t, `{"path":"f.txt","old_string":"zzz","new_string":"x"}`))
	if res.Error == "" {
		t.Fatal("edit with missing old_string must fail")
	}
	// Params inválidos (lixo) → erro, não panic.
	res, _ = et.Execute(context.Background(), mustParams(t, `{not json`))
	if res.Error == "" {
		t.Fatal("garbage params must return an error result")
	}
}
