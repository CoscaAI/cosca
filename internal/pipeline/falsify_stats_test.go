package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

// ── LOOP V5 (FAMÍLIA NOVA): SILENT ZERO REPORTING (caso pipeline) ─────
// Hipótese: leitura de observabilidade/decisão que engole erro reporta
// "unknown"/fallback silenciosamente.
// PREVISÃO: detectProjectType com workDir inacessível → "unknown" SEM erro
// (mascarando um diretório Go real); goPackages idem.

func TestFalsify_DetectProjectTypeWorkDirInexistente(t *testing.T) {
	dir := t.TempDir()
	v := NewVerificationRunner(filepath.Join(dir, "nao-existe"))

	proj, err := v.detectProjectType()
	if err != nil {
		t.Logf("CONTRAEXEMPLO: detectProjectType com dir inexistente agora retorna ERRO: %v", err)
		return
	}
	// Para ser honesto: o dir não existe — "unknown" poderia ser aceitável,
	// mas o ponto é que NÃO há distinção entre "dir vazio" e "leitura falhou".
	t.Logf("detectProjectType com dir INEXISTENTE: %q (silencioso)", proj)
	if proj == "unknown" {
		t.Log("CONFIRMAÇÃO: reporta 'unknown' silenciosamente — erro de leitura mascarado")
	}
}

func TestFalsify_GoPackagesSubdirIlegivel(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignora permissões")
	}
	dir := t.TempDir()
	// subdir com um .go, depois torna ilegível
	sub := filepath.Join(dir, "pkg")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.go"), []byte("package pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })

	v := NewVerificationRunner(dir)
	pkgs := v.goPackages()
	t.Logf("goPackages com subdir ILEGÍVEL: %v", pkgs)
	t.Log("CONFIRMAÇÃO: goPackages engole o erro do ReadDir — retorna fallback silenciosamente")
}
