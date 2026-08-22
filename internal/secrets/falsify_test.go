package secrets

import (
	"testing"
)

// ── LOOP V5 (BLIND TRANSFER): segunda previsão registrada ANTES ───────
// Hipótese (do histórico): stores com db injetável + acesso direto SEM guard
// → panic com zero-value (audit.Store #30, trace.Store #31).
// PREVISÃO: secrets.Vault.Set/getLocked PANIC com zero-value.

func TestFalsify_VaultZeroValue(t *testing.T) {
	v := &Vault{} // db nil
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		_, _ = v.getLocked("k")
	}()
	if panicked {
		t.Log("CONFIRMAÇÃO: secrets.Vault.getLocked PANIC com zero-value — previsão BLIND CONFIRMADA")
	} else {
		t.Log("CONTRAEXEMPLO: secrets.Vault.getLocked não panicou — previsão refutada")
	}
}
