package authority

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/proposal"
)

func TestGuard_WriteFrozenWithoutAuth_FailClosed(t *testing.T) {
	// T13: tentativa de sobrescrever o FROZEN sem autorização → FAIL-CLOSED.
	frozen := t.TempDir()
	live := t.TempDir()
	withRoots := GuardOptions{FrozenRoot: frozen, LiveRoot: live, AllowFrozenWrite: false}
	sanctioned := GuardOptions{FrozenRoot: frozen, LiveRoot: live, AllowFrozenWrite: true}

	// Escrever em qualquer lugar do FROZEN sem promoção → bloqueado.
	target := filepath.Join(frozen, "a.md")
	err := GuardWrite(target, withRoots)
	var ge *GuardError
	if !errors.As(err, &ge) {
		t.Fatalf("esperava GuardError, obteve %T: %v", err, err)
	}
	if ge.Zone != ZoneFrozen || ge.Op != OpWrite {
		t.Errorf("GuardError incorreto: %+v", ge)
	}

	// Criação de diretório no FROZEN também bloqueada.
	if err := GuardMkdir(filepath.Join(frozen, "novo", "dir"), withRoots); err == nil {
		t.Errorf("GuardMkdir deveria bloquear dentro do FROZEN")
	}

	// O caminho SANCTIONED (promoção explícita) é permitido.
	if err := GuardWrite(target, sanctioned); err != nil {
		t.Errorf("write sancionado (promoção) deveria passar: %v", err)
	}
}

func TestGuard_DeleteProtectedZone_FailClosed(t *testing.T) {
	// G2/G5: deletar em qualquer zona protegida (FROZEN/LIVE/RUNTIME) é FAIL-CLOSED.
	frozen := t.TempDir()
	live := t.TempDir()
	runtimeRoot := filepath.Join(t.TempDir(), ".cosca")
	opts := GuardOptions{FrozenRoot: frozen, LiveRoot: live, RuntimeRoot: runtimeRoot}

	for _, target := range []string{
		filepath.Join(frozen, "blocks", "agent1.md"),
		filepath.Join(live, "KERNEL.md"),
		filepath.Join(runtimeRoot, "x.md"),
	} {
		if err := GuardDelete(target, opts); err == nil {
			t.Errorf("GuardDelete(%q) deveria ser FAIL-CLOSED", target)
		} else {
			var ge *GuardError
			if !errors.As(err, &ge) || ge.Op != OpDelete {
				t.Errorf("GuardDelete(%q) retorno inesperado: %v", target, err)
			}
		}
	}

	// Alvo livre (fora de qualquer zona) não é bloqueado.
	free := filepath.Join(t.TempDir(), "scratch", "file.txt")
	if err := GuardDelete(free, opts); err != nil {
		t.Errorf("GuardDelete de alvo livre não devia bloquear: %v", err)
	}
}

func TestGuard_ZoneOfRepoLayout(t *testing.T) {
	// Reusa proposal.ProtectedZoneOf: o layout real do repo mapeia corretamente.
	cases := map[string]string{
		"internal/embed/cosca":         string(ZoneFrozen),
		"internal/embed/cosca/KERNEL.md": string(ZoneFrozen),
		".opencode/cosca":              string(ZoneLive),
		".opencode/cosca/shared/X.md":  string(ZoneLive),
		".opencode/opencode.json":      string(ZoneLive),
		".cosca":                       string(ZoneRuntime),
		".cosca/knowledge.db":          string(ZoneRuntime),
		"cmd/main.go":                  "",
		"internal/authority/x.go":      "",
	}
	for target, want := range cases {
		got := proposal.ProtectedZoneOf(target)
		if got != want {
			t.Errorf("ProtectedZoneOf(%q) = %q, esperado %q", target, got, want)
		}
	}
}
