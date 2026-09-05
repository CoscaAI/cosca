package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoaderInitAndLoadAll(t *testing.T) {
	// Diretório de plugins inexistente → LoadAll retorna erro claro.
	l := NewLoader(LoaderConfig{PluginsDir: filepath.Join(t.TempDir(), "missing")})
	if _, err := l.LoadAll(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("LoadAll(missing dir) must error")
	}

	// Diretório vazio → lista vazia, sem erro.
	empty := t.TempDir()
	plugins, err := l.LoadAll(empty)
	if err != nil || len(plugins) != 0 {
		t.Fatalf("LoadAll(empty) = %d, %v", len(plugins), err)
	}

	// Diretório com arquivo não-plugin (markdown) → erro por entrada,
	// mas LoadAll continua (retorna 0 plugins, sem falha total).
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "not-plugin.md"), []byte("# x"), 0o644)
	plugins, err = l.LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll with bad file: %v", err)
	}
	if len(plugins) != 0 {
		t.Fatalf("plugins = %d, want 0 (arquivo md não é plugin)", len(plugins))
	}
}
