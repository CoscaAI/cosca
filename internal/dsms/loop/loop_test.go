package loop

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// preparaTemp cria um dir temporario com os schemas copiados.
func preparaTemp(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "dsms-loop-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	schemaDir := filepath.Join(tmpDir, "schema")
	if err := os.MkdirAll(schemaDir, 0755); err != nil {
		t.Fatalf("create schema dir: %v", err)
	}
	for _, schema := range []string{
		"001_core.sql", "002_knowledge.sql", "003_memory.sql",
		"004_intelligence.sql", "005_operations.sql", "006_cache.sql",
	} {
		src := filepath.Join("..", "schema", schema)
		data, err := os.ReadFile(src)
		if err != nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(schemaDir, schema), data, 0644); err != nil {
			t.Fatalf("copy schema %s: %v", schema, err)
		}
	}

	return tmpDir
}

// TestNewLoop verifica a construcao do loop.
func TestNewLoop(t *testing.T) {
	tmpDir := preparaTemp(t)

	l := NewLoop(tmpDir)
	if l == nil {
		t.Fatal("NewLoop returned nil")
	}
	if l.config == nil {
		t.Error("expected config to be set")
	}
	if l.dsms == nil {
		t.Error("expected dsms to be set")
	}
	if l.scheduler == nil {
		t.Error("expected scheduler to be set")
	}
}

// TestRun_CancelledContext exercita o ciclo completo do loop: Open + scheduler
// + shutdown gracioso via cancelamento de contexto.
func TestRun_CancelledContext(t *testing.T) {
	tmpDir := preparaTemp(t)
	l := NewLoop(tmpDir)

	// Context que cancela apos 300ms => exercita o caminho de shutdown gracioso
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	err := l.Run(ctx)
	// O loop deve parar graciosamente ao cancelar o contexto.
	// Pode retornar context.DeadlineExceeded (ctx.Done) ou nil.
	if err != context.DeadlineExceeded && err != nil {
		t.Fatalf("Run should stop gracefully on cancel, got: %v", err)
	}
}

// TestRun_CancelledFast verifica cancelamento imediato nao trava.
func TestRun_CancelledFast(t *testing.T) {
	tmpDir := preparaTemp(t)
	l := NewLoop(tmpDir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancela imediatamente

	err := l.Run(ctx)
	if err == nil {
		t.Log("Run returned nil on immediate cancel (aceitavel)")
	}
}
