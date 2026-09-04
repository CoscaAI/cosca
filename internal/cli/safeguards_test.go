package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGuardedRemoveAll_BlocksProtectedZones é a prova da trava P0 (teste f):
// GuardedRemoveAll deve RECUSAR (FAIL-CLOSED, nada executado) qualquer alvo de
// zona protegida (FROZEN / LIVE / RUNTIME) sem importar o caminho.
func TestGuardedRemoveAll_BlocksProtectedZones(t *testing.T) {
	// HERMÉTICO: cria sub-caminhos com os segmentos das zonas protegidas sob
	// t.TempDir() e prova que GuardedRemoveAll RECUSA (FAIL-CLOSED) e NÃO remove.
	dir := t.TempDir()
	for _, seg := range []string{
		filepath.Join("internal", "embed", "cosca"), // FROZEN
		filepath.Join(".opencode", "cosca"),          // LIVE
		filepath.Join(".opencode"),                   // LIVE (pai)
		".cosca",                                     // RUNTIME
	} {
		z := filepath.Join(dir, seg)
		if err := os.MkdirAll(z, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := GuardedRemoveAll(z); err == nil || !strings.Contains(err.Error(), "FAIL-CLOSED") {
			t.Errorf("GuardedRemoveAll(%q) deveria bloquear FAIL-CLOSED, got %v", z, err)
		}
		if _, statErr := os.Stat(z); statErr != nil {
			t.Errorf("zona protegida %q não pode ter sido removida: %v", z, statErr)
		}
	}
}

// TestGuardedRemoveAll_AllowsFreeTarget garante que alvos fora das zonas
// protegidas continuam removíveis (a trava não bloqueia tudo).
func TestGuardedRemoveAll_AllowsFreeTarget(t *testing.T) {
	dir := t.TempDir()
	free := filepath.Join(dir, "free-dir")
	if err := os.MkdirAll(free, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(free, "x.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := GuardedRemoveAll(free); err != nil {
		t.Fatalf("GuardedRemoveAll em alvo livre deveria funcionar: %v", err)
	}
	if _, statErr := os.Stat(free); !os.IsNotExist(statErr) {
		t.Error("alvo livre deveria ter sido removido")
	}
}

// TestGuardedRemoveAll_ReturnsRealErrDelega para um alvo livre inexistente —
// os.RemoveAll não falha para caminho inexistente; apenas confirma a delegação.
func TestGuardedRemoveAll_NoPathIsNonFatal(t *testing.T) {
	dir := t.TempDir()
	if err := GuardedRemoveAll(filepath.Join(dir, "nao-existe")); err != nil {
		t.Errorf("remove de caminho inexistente (alvo livre) não deveria falhar: %v", err)
	}
}
