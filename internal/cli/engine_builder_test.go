package cli

import (
	"os"
	"testing"
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
