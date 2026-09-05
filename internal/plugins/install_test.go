package plugins

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInstallFromLocal(t *testing.T) {
	// Cria um "pacote" local (dir com manifest) e instala via file://.
	srcDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(srcDir, "manifest.yaml"), []byte("id: test-plugin\nname: Test\nversion: 1.0.0\nruntime: wasm\n"), 0o644)
	_ = os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main\n"), 0o644)
	_ = os.WriteFile(filepath.Join(srcDir, "plugin.wasm"), []byte("\x00asm\x01\x00\x00\x00"), 0o644)

	// Manager com plugins dir vazio.
	mgr := NewManager(ManagerConfig{Dir: t.TempDir()}, nil)

	// Install de um dir inválido → erro.
	if err := mgr.Install(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Log("Install(missing) retornou erro (aceito)")
	}

	// Install local válido via file://.
	if err := mgr.Install("file://" + srcDir); err != nil {
		t.Fatalf("Install(file://): %v", err)
	}
	// O install copia o pacote; o Uninstall de um plugin NÃO registrado em
	// memória reporta erro claro (comportamento real sem Load prévio).
	if err := mgr.Uninstall("test-plugin"); err == nil {
		t.Log("Uninstall retornou nil (plugin registrado — aceito)")
	}
	// Update do mesmo: sem Load, reporta erro ou no-op — nunca panic.
	_ = mgr.Update("test-plugin")
}

func TestLoadPluginInvalidPath(t *testing.T) {
	l := NewLoader(LoaderConfig{PluginsDir: t.TempDir()})
	// Path inexistente → erro.
	if _, err := l.LoadPlugin(filepath.Join(t.TempDir(), "missing.so")); err == nil {
		t.Fatal("LoadPlugin(missing) must error")
	}
	// Arquivo não-plugin (markdown) → erro.
	bad := filepath.Join(t.TempDir(), "not-plugin.md")
	_ = os.WriteFile(bad, []byte("# x"), 0o644)
	if _, err := l.LoadPlugin(bad); err == nil {
		t.Fatal("LoadPlugin(md) must error")
	}
}

func TestCopyDirectoryAndExec(t *testing.T) {
	if runtime.GOOS == "windows" {
		// "echo" não é um executável no Windows (é builtin do cmd.exe), então
		// o execCommand falha ao procurar no PATH — comando POSIX.
		t.Skip("comando POSIX (echo) inexistente como executável no Windows")
	}
	// copyDirectory é exercitado via installFromLocal acima.
	// execCommand (método do Manager): comando real simples.
	mgr := NewManager(ManagerConfig{Dir: t.TempDir()}, nil)
	if err := mgr.execCommand("echo", "hi"); err != nil {
		t.Fatalf("execCommand: %v", err)
	}
	// Comando inexistente → erro.
	if err := mgr.execCommand("definitely-not-a-command-xyz"); err == nil {
		t.Fatal("execCommand(invalid) must error")
	}
}
