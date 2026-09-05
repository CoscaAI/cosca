//
// Tests for the Hall da Fama do Conhecimento (internal/knowledge/discovery.go).
//
// Covers:
//   - Register + Get (validações: nil, ID/Nome vazios, impacto fora de 1-5,
//     duplicação de ID)
//   - All() ordenado por impacto desc (D-05 impacto 5 vem primeiro)
//   - Top(n) — top N por impacto
//   - Persistência Save/Load em t.TempDir()
//   - Seed real: 5 descobertas (D-01..D-05) com as evidências dos commits
//

package knowledge

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ── Helpers ─────────────────────────────────────────────────────────────────

// testDiscovery monta uma descoberta determinística para testes.
func testDiscovery(id string, impact int, createdAt time.Time) *Discovery {
	return &Discovery{
		ID:                   id,
		Name:                 "Descoberta " + id,
		Description:          "o que mudou com " + id,
		Impact:               impact,
		ProjectsAffected:     1,
		RiskReductionPercent: 50,
		ValidatedSessions:    3,
		Origin:               "test",
		RelatedLaw:           "K-01",
		CreatedAt:            createdAt,
	}
}

// discoveryIDs devolve os IDs na ordem retornada.
func discoveryIDs(items []*Discovery) []string {
	ids := make([]string, 0, len(items))
	for _, d := range items {
		ids = append(ids, d.ID)
	}
	return ids
}

// ── Register + Get ───────────────────────────────────────────────────────────

func TestRegister_Get(t *testing.T) {
	h := NewHallOfFame()

	d := testDiscovery("D-01", 5, time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC))
	if err := h.Register(d); err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, ok := h.Get("D-01")
	if !ok {
		t.Fatal("Get(D-01) not found after Register")
	}
	if got.Name != d.Name || got.Impact != 5 {
		t.Errorf("Get returned %+v, want %+v", got, d)
	}
	if got.RelatedLaw != "K-01" {
		t.Errorf("related_law = %q, want K-01", got.RelatedLaw)
	}

	// Get devolve cópia: mutar a cópia não afeta o registro.
	got.Impact = 1
	again, _ := h.Get("D-01")
	if again.Impact != 5 {
		t.Errorf("Get returned a shared reference: impact mutated to %d", again.Impact)
	}

	if _, ok := h.Get("D-999"); ok {
		t.Error("Get(D-999) should not be found")
	}
}

func TestRegister_Validations(t *testing.T) {
	h := NewHallOfFame()

	if err := h.Register(nil); err == nil {
		t.Error("Register(nil) should error")
	}

	noID := testDiscovery("", 5, time.Now())
	if err := h.Register(noID); err == nil {
		t.Error("Register with empty ID should error")
	}

	noName := testDiscovery("D-01", 5, time.Now())
	noName.Name = "   "
	if err := h.Register(noName); err == nil {
		t.Error("Register with blank name should error")
	}

	// Impacto é escala de estrelas 1-5 — nunca 0, nunca 6.
	for _, bad := range []int{0, 6, -1} {
		d := testDiscovery("D-x", bad, time.Now())
		if err := h.Register(d); err == nil {
			t.Errorf("Register with impact %d should error (fora de 1-5)", bad)
		}
	}

	// Métricas fora da escala são rejeitadas.
	badRisk := testDiscovery("D-r", 5, time.Now())
	badRisk.RiskReductionPercent = 101
	if err := h.Register(badRisk); err == nil {
		t.Error("Register with risk_reduction_percent 101 should error")
	}
	badEconomy := testDiscovery("D-e", 5, time.Now())
	badEconomy.EconomyUSD = -1
	if err := h.Register(badEconomy); err == nil {
		t.Error("Register with negative economy_usd should error")
	}

	// Duplicação de ID é rejeitada.
	if err := h.Register(testDiscovery("D-dup", 5, time.Now())); err != nil {
		t.Fatalf("Register first D-dup: %v", err)
	}
	if err := h.Register(testDiscovery("D-dup", 4, time.Now())); err == nil {
		t.Error("Register duplicate ID should error")
	}
}

func TestRegister_ZeroCreatedAtFilled(t *testing.T) {
	h := NewHallOfFame()

	d := testDiscovery("D-01", 5, time.Time{}) // zero
	if err := h.Register(d); err != nil {
		t.Fatalf("Register: %v", err)
	}
	got, _ := h.Get("D-01")
	if got.CreatedAt.IsZero() {
		t.Error("zero CreatedAt should be filled on Register")
	}
}

// ── All() — ordenado por impacto desc ───────────────────────────────────────

func TestAll_SortedByImpact(t *testing.T) {
	h := NewHallOfFame()

	// Registra fora de ordem de propósito.
	mustRegister(t, h, testDiscovery("D-01", 5, time.Date(2026, 7, 31, 11, 36, 0, 0, time.UTC)))
	mustRegister(t, h, testDiscovery("D-05", 5, time.Date(2026, 7, 31, 12, 51, 0, 0, time.UTC)))
	mustRegister(t, h, testDiscovery("D-04", 4, time.Date(2026, 7, 31, 11, 58, 0, 0, time.UTC)))
	mustRegister(t, h, testDiscovery("D-03", 4, time.Date(2026, 7, 31, 11, 36, 0, 0, time.UTC)))
	mustRegister(t, h, testDiscovery("D-02", 3, time.Date(2026, 7, 31, 11, 36, 0, 0, time.UTC)))

	items := h.All()
	if len(items) != 5 {
		t.Fatalf("All() returned %d items, want 5", len(items))
	}

	// D-05 (impacto 5, mais recente) vem primeiro; D-02 (impacto 3) por último.
	want := []string{"D-05", "D-01", "D-04", "D-03", "D-02"}
	got := discoveryIDs(items)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("All() order = %v, want %v", got, want)
	}

	// Todos os impactos desc: 5, 5, 4, 4, 3.
	for i, d := range items {
		if i > 0 && items[i-1].Impact < d.Impact {
			t.Errorf("All() not sorted by impact desc at %d: %d < %d", i, items[i-1].Impact, d.Impact)
		}
	}

	// All devolve cópias: mutar o resultado não afeta o registro.
	items[0].Impact = 1
	again, _ := h.Get("D-05")
	if again.Impact != 5 {
		t.Error("All() returned a shared reference: impact mutated")
	}
}

func TestAll_Empty(t *testing.T) {
	h := NewHallOfFame()
	if items := h.All(); len(items) != 0 {
		t.Errorf("All() on empty hall = %d items, want 0", len(items))
	}
}

// ── Top(n) ──────────────────────────────────────────────────────────────────

func TestTop(t *testing.T) {
	h := NewHallOfFame()
	mustRegister(t, h, testDiscovery("D-01", 3, time.Date(2026, 7, 31, 11, 0, 0, 0, time.UTC)))
	mustRegister(t, h, testDiscovery("D-02", 5, time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)))
	mustRegister(t, h, testDiscovery("D-03", 4, time.Date(2026, 7, 31, 11, 30, 0, 0, time.UTC)))
	mustRegister(t, h, testDiscovery("D-04", 2, time.Date(2026, 7, 31, 11, 15, 0, 0, time.UTC)))

	tests := []struct {
		n    int
		want []string
	}{
		{1, []string{"D-02"}},
		{2, []string{"D-02", "D-03"}},
		{3, []string{"D-02", "D-03", "D-01"}},
		{99, []string{"D-02", "D-03", "D-01", "D-04"}}, // n >= total → todos
		{0, []string{}},
		{-3, []string{}},
	}
	for _, tt := range tests {
		got := discoveryIDs(h.Top(tt.n))
		if strings.Join(got, ",") != strings.Join(tt.want, ",") {
			t.Errorf("Top(%d) = %v, want %v", tt.n, got, tt.want)
		}
	}
}

// ── Persistência ────────────────────────────────────────────────────────────

// TestHallOfFame_Persist_Load verifies the Hall of Fame Save/Load round trip
// (renamed from TestPersist_Load to avoid colliding with the CKL
// PromotionEngine persistence test in evidence_test.go).
func TestHallOfFame_Persist_Load(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".cosca", "knowledge", "hall-of-fame.json")

	h := NewHallOfFame()
	mustRegister(t, h, testDiscovery("D-01", 5, time.Date(2026, 7, 31, 11, 36, 0, 0, time.UTC)))
	mustRegister(t, h, testDiscovery("D-02", 4, time.Date(2026, 7, 31, 11, 58, 0, 0, time.UTC)))
	if err := h.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// O arquivo nasce com permissões restritas (convenção .cosca).
	// 0600 é bit POSIX — chmod é no-op no Windows, então o assert só vale em unix.
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat saved file: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0600 {
			t.Errorf("saved file perm = %o, want 600", perm)
		}
	}

	loaded := NewHallOfFame()
	if err := loaded.Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if items := loaded.All(); len(items) != 2 {
		t.Fatalf("loaded %d discoveries, want 2", len(items))
	}
	got, ok := loaded.Get("D-01")
	if !ok {
		t.Fatal("loaded hall missing D-01")
	}
	if got.Impact != 5 || got.RiskReductionPercent != 50 || got.RelatedLaw != "K-01" {
		t.Errorf("loaded D-01 = %+v, want impact 5 / risk 50 / law K-01", got)
	}
	if got.CreatedAt.IsZero() {
		t.Error("loaded D-01 CreatedAt is zero")
	}

	// Save é determinístico: o mesmo conteúdo gera o mesmo arquivo.
	h2 := NewHallOfFame()
	mustRegister(t, h2, testDiscovery("D-02", 4, time.Date(2026, 7, 31, 11, 58, 0, 0, time.UTC)))
	mustRegister(t, h2, testDiscovery("D-01", 5, time.Date(2026, 7, 31, 11, 36, 0, 0, time.UTC)))
	if err := h2.Save(path); err != nil {
		t.Fatalf("Save h2: %v", err)
	}
	reloaded := NewHallOfFame()
	if err := reloaded.Load(path); err != nil {
		t.Fatalf("reload: %v", err)
	}
	gotOrder := discoveryIDs(reloaded.All())
	if strings.Join(gotOrder, ",") != "D-01,D-02" {
		t.Errorf("reloaded order = %v, want D-01,D-02 (impacto desc)", gotOrder)
	}
}

func TestPersist_LoadMissingFile(t *testing.T) {
	h := NewHallOfFame()
	err := h.Load(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Fatal("Load of missing file should error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Load error = %v, want os.ErrNotExist", err)
	}
}

// ── Seed real ───────────────────────────────────────────────────────────────

func TestSeed(t *testing.T) {
	h := NewHallOfFame()
	if err := SeedHallOfFame(h); err != nil {
		t.Fatalf("SeedHallOfFame: %v", err)
	}

	items := h.All()
	if len(items) != 5 {
		t.Fatalf("seed has %d discoveries, want 5", len(items))
	}

	want := map[string]struct {
		name          string
		impact        int
		projects      int
		riskReduction float64
		sessions      int
		origin        string
		relatedLaw    string
	}{
		"D-01": {"Jail fail-closed — recusa executar como root", 5, 1, 87, 4, "red-team-onda-1 (auditoria A1+C3)", "K-01"},
		"D-02": {"Registro público desabilitado por padrão", 4, 1, 70, 3, "red-team-onda-1 (auditoria A1)", "K-02"},
		"D-03": {"Path traversal bloqueado por containment", 4, 1, 65, 5, "red-team-onda-1 (C1+C2)", "K-03"},
		"D-04": {"Refresh token com rotação e revogação", 4, 1, 60, 4, "red-team-onda-2 (A4)", "K-04"},
		"D-05": {"Cosca Knowledge Lifecycle (CKL)", 5, 1, 0, 1, "conversa do Don — conhecimento como código", "K-05"},
	}

	for _, d := range items {
		exp, ok := want[d.ID]
		if !ok {
			t.Errorf("unexpected seed discovery ID %q", d.ID)
			continue
		}
		if d.Name != exp.name {
			t.Errorf("seed %s name = %q, want %q", d.ID, d.Name, exp.name)
		}
		if d.Impact != exp.impact {
			t.Errorf("seed %s impact = %d, want %d", d.ID, d.Impact, exp.impact)
		}
		if d.ProjectsAffected != exp.projects {
			t.Errorf("seed %s projects_affected = %d, want %d", d.ID, d.ProjectsAffected, exp.projects)
		}
		if d.RiskReductionPercent != exp.riskReduction {
			t.Errorf("seed %s risk_reduction_percent = %.1f, want %.1f", d.ID, d.RiskReductionPercent, exp.riskReduction)
		}
		if d.ValidatedSessions != exp.sessions {
			t.Errorf("seed %s validated_sessions = %d, want %d", d.ID, d.ValidatedSessions, exp.sessions)
		}
		if d.Origin != exp.origin {
			t.Errorf("seed %s origin = %q, want %q", d.ID, d.Origin, exp.origin)
		}
		if d.RelatedLaw != exp.relatedLaw {
			t.Errorf("seed %s related_law = %q, want %q", d.ID, d.RelatedLaw, exp.relatedLaw)
		}
		if d.Description == "" {
			t.Errorf("seed %s has empty description", d.ID)
		}
		if d.CreatedAt.IsZero() {
			t.Errorf("seed %s has zero CreatedAt", d.ID)
		}
	}

	// Impacto em estrelas 1-5: o seed nunca confunde com confidence do CKL.
	// D-05 (impacto 5) deve renderizar 5 estrelas preenchidas.
	if got, _ := h.Get("D-05"); got.Stars() != "★★★★★" {
		t.Errorf("D-05.Stars() = %q, want ★★★★★", got.Stars())
	}
	if got, _ := h.Get("D-04"); got.Stars() != "★★★★☆" {
		t.Errorf("D-04.Stars() = %q, want ★★★★☆", got.Stars())
	}
}

// ── Helpers de teste ────────────────────────────────────────────────────────

func mustRegister(t *testing.T, h *HallOfFame, d *Discovery) {
	t.Helper()
	if err := h.Register(d); err != nil {
		t.Fatalf("Register(%s): %v", d.ID, err)
	}
}
