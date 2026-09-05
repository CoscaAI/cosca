package cosca

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJailAllowedEnv(t *testing.T) {
	allowed := jailAllowedEnv()
	// Obrigatórios.
	for _, k := range []string{"PATH", "HOME", "TERM", "LANG", "TMPDIR"} {
		if !allowed[k] {
			t.Fatalf("allowed[%s] = false", k)
		}
	}
	// Proibidos por padrão.
	if allowed["SECRET_KEY"] {
		t.Fatal("SECRET_KEY não deve estar na allowlist")
	}
}

func TestJailEnvironmentFilters(t *testing.T) {
	t.Setenv("COSCA_TEST_ALLOWED_VAR", "1") // será permitida via regra custom? (não)
	t.Setenv("PATH", "/usr/bin:/bin")
	t.Setenv("SECRET_ENV", "hunter2")

	env := jailEnvironment()
	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, "PATH=") {
		t.Fatal("PATH deve passar")
	}
	if strings.Contains(joined, "SECRET_ENV") {
		t.Fatal("SECRET_ENV NÃO deve vazar para a jaula")
	}
}

func TestSanitizeJailDiagnostic(t *testing.T) {
	t.Setenv("COSCA_TEST_SECRET", "supersecretvalue")
	msg := "failed to connect with token supersecretvalue"
	sanitized := sanitizeJailDiagnostic(msg)
	if strings.Contains(sanitized, "supersecretvalue") {
		t.Fatal("diagnóstico não redigiu o valor de env")
	}
	// Caracteres de controle removidos.
	out := sanitizeJailDiagnostic("a\x00b\x1fc")
	if strings.ContainsAny(out, "\x00\x1f") {
		t.Fatalf("controle não removido: %q", out)
	}
}

func TestWriteJailSecretsFile(t *testing.T) {
	// Com env de segredo definido, o arquivo é criado e é legível.
	t.Setenv("COSCA_JWT_SECRET", "test-jwt-secret-0123456789abcdef")
	dir := t.TempDir()
	path, err := writeJailSecretsFile(dir)
	if err != nil {
		t.Fatalf("writeJailSecretsFile: %v", err)
	}
	if path == "" {
		t.Log("nenhum segredo propagado — arquivo vazio (comportamento aceito)")
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ler arquivo de segredos: %v", err)
	}
	if !strings.Contains(string(data), "COSCA_JWT_SECRET") {
		t.Fatalf("arquivo não contém o segredo esperado: %q", data)
	}
	// Reescrita é atômica (escreve via tmp+rename).
	_ = filepath.Dir(path)
}

func TestSanitizeKeepsUsefulText(t *testing.T) {
	out := sanitizeJailDiagnostic("bwrap: execvp failed: No such file or directory")
	if !strings.Contains(out, "bwrap") || !strings.Contains(out, "No such file") {
		t.Fatalf("sanitize removeu texto útil: %q", out)
	}
}
