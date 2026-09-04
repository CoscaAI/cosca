package proposal

import (
	"path/filepath"
	"testing"
)

// TestSensitiveRmTarget_RejectsProtectedZones cobre o P2 do endurecimento:
// .opencode/cosca (LIVE) e .opencode devem ser alvos sensíveis, junto de .cosca
// (RUNTIME) e internal/embed/cosca (FROZEN). Fail-closed: nunca "deixar passar".
func TestSensitiveRmTarget_RejectsProtectedZones(t *testing.T) {
	cases := []struct {
		target string
		want   bool
	}{
		// Zonas de autoridade (contrato FROZEN > LIVE > RUNTIME).
		{".opencode/cosca", true},
		{".opencode/cosca/agents", true},
		{".opencode/cosca/skills/SKILLS_CATALOG.md", true},
		{".opencode", true},
		{".opencode/opencode.json", true},
		{".cosca", true},
		{".cosca/knowledge.db", true},
		{".cosca/backups", true},
		{"internal/embed/cosca", true},
		{"internal/embed/cosca/skills", true},
		// Legado (raízes / artefatos críticos) — preservado.
		{".git", true},
		{"/etc/passwd", true},
		{"knowledge.db", true},
		// Alvos livres.
		{"docs/README.md", false},
		{"internal/pkg/util.go", false},
		{"", false},
		// Falso-positivo de prefixo de string: ".cosca-backup" NÃO é ".cosca".
		{".cosca-backup", false},
	}
	for _, c := range cases {
		if got := sensitiveRmTarget(c.target); got != c.want {
			t.Errorf("sensitiveRmTarget(%q) = %v, want %v", c.target, got, c.want)
		}
	}
}

// TestProtectedZoneOf_AssociatesAuthorityZones cobre a API exportada usada
// pela trava P0 (GuardedRemoveAll) e pelo guard do skills sync.
func TestProtectedZoneOf_AssociatesAuthorityZones(t *testing.T) {
	zones := []struct {
		target string
		zone   string
	}{
		{"internal/embed/cosca", "FROZEN"},
		{"internal/embed/cosca/skills/SKILLS_CATALOG.md", "FROZEN"},
		{".opencode/cosca", "LIVE"},
		{".opencode/cosca/agents/x.md", "LIVE"},
		{".opencode", "LIVE"},
		{".cosca", "RUNTIME"},
		{".cosca/knowledge.db", "RUNTIME"},
	}
	for _, z := range zones {
		if got := ProtectedZoneOf(z.target); got != z.zone {
			t.Errorf("ProtectedZoneOf(%q) = %q, want %q", z.target, got, z.zone)
		}
		if !IsProtectedRmTarget(z.target) {
			t.Errorf("IsProtectedRmTarget(%q) = false, want true", z.target)
		}
	}

	free := []string{"docs/README.md", "internal/pkg/util.go", filepath.Join("a", "b", ".opencode-notes")}
	for _, f := range free {
		if ProtectedZoneOf(f) != "" {
			t.Errorf("ProtectedZoneOf(%q) = %q, want \"\"", f, ProtectedZoneOf(f))
		}
		if IsProtectedRmTarget(f) {
			t.Errorf("IsProtectedRmTarget(%q) = true, want false", f)
		}
	}
}

// TestSensitiveRmTarget_AbsolutePathCovered garante que caminhos absolutos
// (como os que o os.RemoveAll/CLI usa) também são reconhecidos como sensíveis.
func TestSensitiveRmTarget_AbsolutePathCovered(t *testing.T) {
	absLive := filepath.Join("C:", "Users", "x", "cosca", ".opencode", "cosca")
	absFrozen := filepath.Join("C:", "Users", "x", "cosca", "internal", "embed", "cosca", "skills")
	absRuntime := filepath.Join("C:", "Users", "x", "cosca", ".cosca", "backups")

	if !sensitiveRmTarget(absLive) {
		t.Errorf("sensitiveRmTarget(%q) = false, want true", absLive)
	}
	if !sensitiveRmTarget(absFrozen) {
		t.Errorf("sensitiveRmTarget(%q) = false, want true", absFrozen)
	}
	if !sensitiveRmTarget(absRuntime) {
		t.Errorf("sensitiveRmTarget(%q) = false, want true", absRuntime)
	}

	absFree := filepath.Join("C:", "Users", "x", "cosca", "docs", "README.md")
	if sensitiveRmTarget(absFree) {
		t.Errorf("sensitiveRmTarget(%q) = true, want false", absFree)
	}
}
