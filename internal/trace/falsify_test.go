package trace

import (
	"testing"
)

// ── LOOP V5 (BLIND TRANSFER): previsão registrada ANTES ───────────────
// Hipótese (do histórico): stores com db injetável + acesso direto SEM guard
// → panic com zero-value (como audit.Store #30).
// PREVISÃO: trace.Store.Get/Append PANIC com zero-value.

func TestFalsify_TraceStoreZeroValue(t *testing.T) {
	s := &Store{} // db nil
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		if _, ok := Parse("TRACE-20260814-ABCDEF01"); !ok {
			t.Log("Parse rejeitou o ID — teste inválido")
			return
		}
		_, _ = s.Get("TRACE-20260814-ABCDEF01")
	}()
	if panicked {
		t.Log("CONFIRMAÇÃO: trace.Store.Get PANIC com zero-value — previsão BLIND CONFIRMADA")
	} else {
		t.Log("CONTRAEXEMPLO: trace.Store.Get não panicou — previsão refutada")
	}
}
