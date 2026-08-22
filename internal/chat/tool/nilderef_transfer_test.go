package tool

import (
	"context"
	"encoding/json"
	"testing"
)

// ── LOOP V4 L336: transferência da família para os TOOLS do chat ───────
//
// A heurística "deps injetáveis sem guard no método = risco" deve se
// transferir: os tools (Read/Write/Edit/Glob/Search) acessam t.rails.Validate
// sem guard — com rails nil, um Execute deve PANIC. Previsão: sim.

func scanToolPanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("TRANSFERÊNCIA CONFIRMADA: %s PANIC com dep nil (bug da família!)", name)
		}
	}()
	fn()
}

func TestTransfer_ReadToolNilRails(t *testing.T) {
	rt := &ReadTool{} // workspace/rails nil
	scanToolPanic(t, "ReadTool", func() {
		_, _ = rt.Execute(context.Background(), json.RawMessage(`{"path":"x"}`))
	})
}

func TestTransfer_WriteToolNilRails(t *testing.T) {
	wt := &WriteTool{}
	scanToolPanic(t, "WriteTool", func() {
		_, _ = wt.Execute(context.Background(), json.RawMessage(`{"path":"x","content":"y"}`))
	})
}

func TestTransfer_EditToolNilRails(t *testing.T) {
	et := &EditTool{}
	scanToolPanic(t, "EditTool", func() {
		_, _ = et.Execute(context.Background(), json.RawMessage(`{"path":"x","old_string":"a","new_string":"b"}`))
	})
}

func TestTransfer_GlobToolNilRails(t *testing.T) {
	gt := &GlobTool{}
	scanToolPanic(t, "GlobTool", func() {
		_, _ = gt.Execute(context.Background(), json.RawMessage(`{"pattern":"*.go","path":"sub"}`))
	})
}
