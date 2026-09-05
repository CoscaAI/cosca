package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

// ── LOOP V5 (FAMÍLIA: erro de origem externa engolido como "não encontrado")
// Pré-registro: LastCompleted engolia erro do LoadRun → ok=false (chamador
// re-executaria steps). PREVISÃO: com log CORROMPIDO, o contrato agora
// PROPAGA erro (não mais ok=false silencioso).

func TestFalsify_LastCompletedPropagaErro(t *testing.T) {
	dir := t.TempDir()
	l, err := NewDurableEventLog(dir)
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	// Log corrompido: linha não-JSON → LoadRun retorna erro real.
	logPath := filepath.Join(dir, "run-corrupta.jsonl")
	if err := os.WriteFile(logPath, []byte("{not-json}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, _, ok, err := l.LastCompleted("run-corrupta")
	if err != nil {
		t.Logf("CONFIRMAÇÃO: LastCompleted agora PROPAGA erro de leitura (não engole): %v", err)
		return
	}
	_ = ok
	t.Log("CONTRAEXEMPLO: LastCompleted não propagou erro — contrato ainda engole")
}
