package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat/tool"
)

// TestBuildEngineModelNamePropagatesToProvider verifica a correção do
// `cosca exec --model X`: quando o --model é o NOME DO MODELO (ex.
// "qwen2.5-coder:14b") e não um provider registrado, o buildEngine deve
// propagá-lo para a env var do provider local (COSCA_OLLAMA_MODEL) ANTES da
// seleção — caso contrário o provider cai no default "llama3" e toda
// execução falha silenciosamente (llama3 not found).
func TestBuildEngineModelNamePropagatesToProvider(t *testing.T) {
	os.Unsetenv("COSCA_OLLAMA_MODEL")

	// Um nome de modelo real (não provider) — deve ser propagado.
	modelName := "qwen2.5-coder:14b"
	buildEngineWithMode(modelName, true)

	if got := os.Getenv("COSCA_OLLAMA_MODEL"); got != modelName {
		t.Fatalf("COSCA_OLLAMA_MODEL = %q, want %q (modelo do --model propagado ao provider)", got, modelName)
	}
}

// TestBuildEngineProviderNameNotPropagated verifica que um nome de PROVIDER
// (ex. "none") NÃO é propagado como modelo — ele é resolvido via registry.
func TestBuildEngineProviderNameNotPropagated(t *testing.T) {
	os.Unsetenv("COSCA_OLLAMA_MODEL")

	buildEngineWithMode("none", true)

	if got := os.Getenv("COSCA_OLLAMA_MODEL"); got != "" {
		t.Fatalf("COSCA_OLLAMA_MODEL = %q, want \"\" (provider 'none' não deve ser propagado como modelo)", got)
	}
}

// TestEngineFilesystemToolsUseReadBeforeWriteGuard é o teste de regressão do
// gap de segurança do engine_builder (ADR-034): o conjunto de filesystem tools
// registrado no caminho do engine/chat (registerFilesystemTools) deve expor o
// write_file SEGURO — que recusa sobrescrever arquivo existente sem leitura
// prévia na mesma sessão — e NUNCA o conjunto LEGACY (write/read/edit) que
// sobrescrevia cegamente.
func TestEngineFilesystemToolsUseReadBeforeWriteGuard(t *testing.T) {
	ws := t.TempDir()
	reg := tool.NewRegistry()

	// Mesma chamada usada pelo buildEngineWithMode (caminho engine/chat).
	registerFilesystemTools(reg, ws)

	// O conjunto novo expõe write_file/read_file/edit_file/list_dir/glob.
	for _, name := range []string{"read_file", "write_file", "edit_file", "list_dir", "glob"} {
		if reg.Get(name) == nil {
			t.Fatalf("engine filesystem set missing tool %q", name)
		}
	}
	// ...e NUNCA o conjunto LEGACY, que sobrescrevia sem guard.
	for _, legacyName := range []string{"write", "read", "edit"} {
		if reg.Get(legacyName) != nil {
			t.Fatalf("engine filesystem set must NOT expose legacy tool %q", legacyName)
		}
	}

	// Guard determinístico: um write_file em arquivo existente SEM leitura
	// prévia na sessão deve ser RECUSADO.
	existing := filepath.Join(ws, "main.go")
	if err := os.WriteFile(existing, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	res := reg.Execute(context.Background(), "write_file",
		json.RawMessage(`{"path":"main.go","content":"package main\n\n// clobbered\n"}`))
	if res.Error == "" || !strings.Contains(res.Error, "already exists") {
		t.Fatalf("write_file must refuse to overwrite an unread existing file (guard), got error=%q output=%q", res.Error, res.Output)
	}

	// Prova de que NÃO sobrescreveu: o conteúdo original permanece intacto.
	data, err := os.ReadFile(existing)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(data) != "package main\n" {
		t.Fatalf("guard failed: existing file was overwritten despite no prior read, content=%q", string(data))
	}

	// Após read_file na mesma sessão, o write_file pode sobrescrever.
	reg.Execute(context.Background(), "read_file", json.RawMessage(`{"path":"main.go"}`))
	res = reg.Execute(context.Background(), "write_file",
		json.RawMessage(`{"path":"main.go","content":"package main\n\n// now authorized\n"}`))
	if res.Error != "" {
		t.Fatalf("write_file after read_file should be allowed, got error=%q", res.Error)
	}
}
