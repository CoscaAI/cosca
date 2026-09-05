package plugins

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestWasmPluginRun_ExecutesEphemeralUnit fecha o gap de modelo (LLRT): o
// wasmPlugin agora roda como unidade efêmera (via Run → chama a entrada e
// captura o stdout), não só como hook de ciclo de vida.
func TestWasmPluginRun_ExecutesEphemeralUnit(t *testing.T) {
	dir := t.TempDir()
	// O fixture escreve "hello from wasm\n" no stdout via WASI fd_write.
	wasmPath := filepath.Join(dir, "echo.wasm")
	copyFile(t, filepath.Join("testdata", "echo.wasm"), wasmPath)

	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}

	l := NewLoader(LoaderConfig{})
	p, err := l.loadWASMPlugin(&PluginManifest{
		ID:      "echo",
		Name:    "echo",
		Version: "1.0",
		Runtime: RuntimeWASM,
	}, wasmPath)
	if err != nil {
		t.Fatalf("loadWASMPlugin: %v", err)
	}

	if err := p.Init(&PluginContext{DataDir: dataDir}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer p.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := p.Run(ctx, "parametro")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "hello from wasm" {
		t.Fatalf("Run output = %q, want %q", out, "hello from wasm")
	}
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}
