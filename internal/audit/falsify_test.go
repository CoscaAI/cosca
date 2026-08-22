package audit

import (
	"testing"
)

// ── LOOP V5: FALSIFICAÇÃO — domínio stores ─────────────────────────────
//
// O audit.Store acessa s.db (SQLite) diretamente nos métodos. Com zero-value
// (db nil), um método deve PANIC. Previsão: hipótese SOBREVIVE neste domínio.

func TestFalsify_AuditStoreZeroValue(t *testing.T) {
	s := &Store{} // db nil
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		_, _, _ = s.List(10, 0, AuditFilters{})
	}()
	if panicked {
		t.Log("CONFIRMAÇÃO: audit.Store.List PANIC com db nil — hipótese sobrevive no domínio stores")
	} else {
		t.Log("CONTRAEXEMPLO: audit.Store.List não panicou com db nil")
	}
}
