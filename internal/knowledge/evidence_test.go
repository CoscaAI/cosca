package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── Helpers ─────────────────────────────────────────────────────────────────

// newTestEvidence builds a deterministic evidence for tests.
func newTestEvidence(id string, day int) Evidence {
	return Evidence{
		ID:          id,
		Kind:        "test",
		Source:      "test-sandbox",
		Description: "evidência " + id,
		Timestamp:   time.Date(2026, 7, day, 12, 0, 0, 0, time.UTC),
	}
}

// addEvidenceRange adds evidence IDs "ev-<from>".."ev-<to>" through the engine.
func addEvidenceRange(t *testing.T, engine *PromotionEngine, itemID string, from, to int) {
	t.Helper()
	for i := from; i <= to; i++ {
		ev := Evidence{
			ID:          fmt.Sprintf("ev-%d", i),
			Kind:        "test",
			Source:      "test-sandbox",
			Description: fmt.Sprintf("evidência %d", i),
			Timestamp:   time.Date(2026, 7, 1+i%28, 12, 0, 0, 0, time.UTC),
		}
		if _, err := engine.AddEvidence(itemID, ev); err != nil {
			t.Fatalf("AddEvidence(ev-%d): %v", i, err)
		}
	}
}

// ── TestPromote_Thresholds ──────────────────────────────────────────────────

func TestPromote_Thresholds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		item      *KnowledgeItem
		evidence  int
		wantLevel KnowledgeLevel
	}{
		{
			name:      "1 evidence -> observation",
			item:      &KnowledgeItem{ID: "K-1", Title: "T", Confidence: 0.5},
			evidence:  1,
			wantLevel: LevelObservation,
		},
		{
			name:      "3 evidence + confidence 0.70 -> learning",
			item:      &KnowledgeItem{ID: "K-2", Title: "T", Confidence: 0.70},
			evidence:  3,
			wantLevel: LevelLearning,
		},
		{
			name:      "3 evidence + confidence 0.69 stays observation",
			item:      &KnowledgeItem{ID: "K-3", Title: "T", Confidence: 0.69},
			evidence:  3,
			wantLevel: LevelObservation,
		},
		{
			name:      "10 evidence + reproducible -> hypothesis",
			item:      &KnowledgeItem{ID: "K-4", Title: "T", Reproducible: true},
			evidence:  10,
			wantLevel: LevelHypothesis,
		},
		{
			name:      "10 evidence without reproducible -> learning",
			item:      &KnowledgeItem{ID: "K-5", Title: "T", Confidence: 0.95},
			evidence:  10,
			wantLevel: LevelLearning,
		},
		{
			name:      "50 evidence + 5 projects + confidence 0.90 -> theory",
			item:      &KnowledgeItem{ID: "K-6", Title: "T", Confidence: 0.90, Projects: 5},
			evidence:  50,
			wantLevel: LevelTheory,
		},
		{
			name:      "50 evidence + 4 projects stays learning",
			item:      &KnowledgeItem{ID: "K-7", Title: "T", Confidence: 0.95, Projects: 4},
			evidence:  50,
			wantLevel: LevelLearning,
		},
		{
			name:      "200 evidence + 0 rollbacks + confidence 0.99 -> law",
			item:      &KnowledgeItem{ID: "K-8", Title: "T", Confidence: 0.99, Projects: 10, Reproducible: true},
			evidence:  200,
			wantLevel: LevelLaw,
		},
		{
			name:      "200 evidence + 1 rollback stays theory",
			item:      &KnowledgeItem{ID: "K-9", Title: "T", Confidence: 0.99, Projects: 10, Reproducible: true, Rollbacks: 1},
			evidence:  200,
			wantLevel: LevelTheory,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for i := 0; i < tt.evidence; i++ {
				tt.item.AddEvidence(newTestEvidence(fmt.Sprintf("ev-%d", i), 1))
			}
			if got := tt.item.CurrentLevel(); got != tt.wantLevel {
				t.Errorf("CurrentLevel() = %q, want %q (evidence=%d)", got, tt.wantLevel, tt.evidence)
			}
			// Promote must never reach constitution from thresholds.
			if lvl, _ := tt.item.Promote(); lvl == LevelConstitution {
				t.Errorf("Promote() must never return constitution, got %q", lvl)
			}
		})
	}
}

// ── TestAddEvidence_PromotesAutomatically ───────────────────────────────────

func TestAddEvidence_PromotesAutomatically(t *testing.T) {
	t.Parallel()

	item := &KnowledgeItem{
		ID:           "K-auto",
		Title:        "Item automático",
		Confidence:   0.99, // MinConfidenceLaw — "law" exige confiança >= 0.99
		Projects:     5,
		Reproducible: true,
	}
	engine := NewPromotionEngine()
	if err := engine.Register(item); err != nil {
		t.Fatalf("Register: %v", err)
	}

	stages := []struct {
		total int
		level KnowledgeLevel
	}{
		{total: 1, level: LevelObservation},
		{total: 3, level: LevelLearning},
		{total: 10, level: LevelHypothesis},
		{total: 50, level: LevelTheory},
		{total: 200, level: LevelLaw},
	}

	added := 0
	for _, stage := range stages {
		addEvidenceRange(t, engine, item.ID, added+1, stage.total)
		added = stage.total

		got, ok := engine.Get(item.ID)
		if !ok {
			t.Fatalf("item %q not found after %d evidences", item.ID, stage.total)
		}
		if got.Level != stage.level {
			t.Errorf("after %d evidences Level = %q, want %q", stage.total, got.Level, stage.level)
		}
		if got.PromotedAt.IsZero() {
			t.Errorf("PromotedAt should be set after promotion to %q", stage.level)
		}
	}
}

// ── TestWhyExists ───────────────────────────────────────────────────────────

func TestWhyExists(t *testing.T) {
	t.Parallel()

	item := &KnowledgeItem{
		ID:         "K-why",
		Title:      "Regra de segurança",
		Confidence: 0.8,
		Projects:   2,
	}
	item.AddEvidence(Evidence{
		ID: "ev-1", Kind: "incident", Source: "s1",
		Description: "primeira ocorrência",
		Timestamp:   time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC),
	})
	item.AddEvidence(Evidence{
		ID: "ev-2", Kind: "audit", Source: "s2",
		Description: "achado de auditoria",
		Timestamp:   time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC),
	})

	out := item.WhyExists()

	for _, want := range []string{
		"K-why",
		"Regra de segurança",
		"observation", // 2 evidências não alcançam learning
		"2 evidências",
		"Evidências:",
		"[ev-1] incident: primeira ocorrência [s1, 2026-07-30]",
		"[ev-2] audit: achado de auditoria [s2, 2026-07-31]",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("WhyExists() deve conter %q, output:\n%s", want, out)
		}
	}
}

// ── TestConstitution_RequiresManualApproval ─────────────────────────────────

func TestConstitution_RequiresManualApproval(t *testing.T) {
	t.Parallel()

	item := &KnowledgeItem{
		ID:           "K-const",
		Title:        "Constituição candidata",
		Confidence:   0.99,
		Projects:     100,
		Reproducible: true,
	}
	engine := NewPromotionEngine()
	if err := engine.Register(item); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// 500 evidências: a promoção automática alcança no máximo law.
	addEvidenceRange(t, engine, item.ID, 1, 500)

	got, ok := engine.Get(item.ID)
	if !ok {
		t.Fatal("item não encontrado")
	}
	if got.Level != LevelLaw {
		t.Fatalf("após 500 evidências Level = %q, esperado %q", got.Level, LevelLaw)
	}
	if got.CurrentLevel() == LevelConstitution {
		t.Error("CurrentLevel() nunca deve retornar constitution")
	}
	if lvl, promoted := got.Promote(); lvl == LevelConstitution || promoted {
		t.Errorf("Promote() com 500 evidências: lvl=%q promoted=%v, não deve chegar a constitution", lvl, promoted)
	}

	// Aprovação manual do Don → constitution.
	if err := engine.ApproveConstitution(item.ID); err != nil {
		t.Fatalf("ApproveConstitution: %v", err)
	}
	got, _ = engine.Get(item.ID)
	if got.Level != LevelConstitution {
		t.Fatalf("após aprovação manual Level = %q, esperado %q", got.Level, LevelConstitution)
	}

	// Aprovação em item inexistente deve falhar.
	if err := engine.ApproveConstitution("nao-existe"); err == nil {
		t.Error("ApproveConstitution em item inexistente deve retornar erro")
	}
}

// ── TestPersist_Load ────────────────────────────────────────────────────────

func TestPersist_Load(t *testing.T) {
	t.Parallel()

	engine := NewPromotionEngine()
	item := &KnowledgeItem{
		ID:           "K-persist",
		Title:        "Lei persistida",
		Confidence:   0.99,
		Projects:     8,
		Reproducible: true,
	}
	if err := engine.Register(item); err != nil {
		t.Fatalf("Register: %v", err)
	}
	addEvidenceRange(t, engine, item.ID, 1, 200) // law

	path := filepath.Join(t.TempDir(), "knowledge", "laws.json")
	if err := engine.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("arquivo não criado em %s: %v", path, err)
	}

	loaded := NewPromotionEngine()
	if err := loaded.Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}

	got, ok := loaded.Get(item.ID)
	if !ok {
		t.Fatal("item não carregado")
	}
	if got.Title != item.Title || got.Level != LevelLaw {
		t.Errorf("carregado: title=%q level=%q, esperado title=%q level=%q",
			got.Title, got.Level, item.Title, LevelLaw)
	}
	if len(got.Evidence) != 200 {
		t.Errorf("len(Evidence) = %d, esperado 200", len(got.Evidence))
	}
	if got.Confidence != 0.99 || got.Projects != 8 || !got.Reproducible {
		t.Errorf("campos não preservados: confidence=%v projects=%d reproducible=%v",
			got.Confidence, got.Projects, got.Reproducible)
	}

	// Load de arquivo inexistente deve retornar erro.
	if err := NewPromotionEngine().Load(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Error("Load de arquivo inexistente deve retornar erro")
	}
}

// ── TestExample_JailRoot ────────────────────────────────────────────────────

func TestExample_JailRoot(t *testing.T) {
	t.Parallel()

	engine := NewPromotionEngine()
	item := &KnowledgeItem{
		ID:           "K-27",
		Title:        "Nunca executar como root automaticamente",
		Confidence:   0.95,
		Projects:     1,
		Reproducible: true,
	}
	if err := engine.Register(item); err != nil {
		t.Fatalf("Register: %v", err)
	}

	evidences := []Evidence{
		{
			ID: "jail-break-1", Kind: "incident", Source: "F001",
			Description: "Fuga da jail: processo executou como root fora do sandbox",
			Timestamp:   time.Date(2026, 7, 30, 9, 15, 0, 0, time.UTC),
		},
		{
			ID: "auditoria-seguranca-2026-07-31", Kind: "audit", Source: "red-team-A1",
			Description: "Red team A1: jail deve recusar root automaticamente",
			Timestamp:   time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC),
		},
		{
			ID: "test-sandbox", Kind: "test", Source: "TestJail_RootUnsafe",
			Description: "Teste confirma: sandbox recusa execução como root",
			Timestamp:   time.Date(2026, 7, 31, 14, 30, 0, 0, time.UTC),
		},
		{
			ID: "onda-1-fix", Kind: "test", Source: "onda-1",
			Description: "Fix Onda 1: jail fail-closed recusa root automaticamente",
			Timestamp:   time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC),
		},
	}
	for _, ev := range evidences {
		if _, err := engine.AddEvidence(item.ID, ev); err != nil {
			t.Fatalf("AddEvidence(%s): %v", ev.ID, err)
		}
	}

	got, ok := engine.Get(item.ID)
	if !ok {
		t.Fatal("item não encontrado")
	}
	if got.Level != LevelLearning {
		t.Fatalf("4 evidências com confiança alta → Level = %q, esperado %q", got.Level, LevelLearning)
	}
	if len(got.Evidence) != 4 {
		t.Fatalf("len(Evidence) = %d, esperado 4", len(got.Evidence))
	}

	out := got.WhyExists()
	for _, want := range []string{
		"K-27",
		"Nunca executar como root automaticamente",
		"learning",
		"4 evidências",
		"jail-break-1",
		"auditoria-seguranca-2026-07-31",
		"test-sandbox",
		"onda-1-fix",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("WhyExists() deve conter %q, output:\n%s", want, out)
		}
	}
}

// ── Registro e validações do PromotionEngine ────────────────────────────────

func TestPromotionEngine_RegisterValidation(t *testing.T) {
	t.Parallel()

	engine := NewPromotionEngine()

	if err := engine.Register(nil); err == nil {
		t.Error("Register(nil) deve retornar erro")
	}
	if err := engine.Register(&KnowledgeItem{}); err == nil {
		t.Error("Register com ID vazio deve retornar erro")
	}

	item := &KnowledgeItem{ID: "K-dup", Title: "Único"}
	if err := engine.Register(item); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := engine.Register(&KnowledgeItem{ID: "K-dup", Title: "Duplicado"}); err == nil {
		t.Error("Register com ID duplicado deve retornar erro")
	}
}

func TestPromotionEngine_GetAndAddEvidenceUnknown(t *testing.T) {
	t.Parallel()

	engine := NewPromotionEngine()
	if _, ok := engine.Get("nao-existe"); ok {
		t.Error("Get de item inexistente deve retornar ok=false")
	}
	if _, err := engine.AddEvidence("nao-existe", newTestEvidence("e1", 1)); err == nil {
		t.Error("AddEvidence em item inexistente deve retornar erro")
	}
}

func TestPromotionEngine_AllSorted(t *testing.T) {
	t.Parallel()

	engine := NewPromotionEngine()
	for _, id := range []string{"K-2", "K-1", "K-3"} {
		if err := engine.Register(&KnowledgeItem{ID: id, Title: id}); err != nil {
			t.Fatalf("Register(%s): %v", id, err)
		}
	}

	all := engine.All()
	if len(all) != 3 {
		t.Fatalf("len(All()) = %d, esperado 3", len(all))
	}
	for i, want := range []string{"K-1", "K-2", "K-3"} {
		if all[i].ID != want {
			t.Errorf("All()[%d] = %q, esperado %q", i, all[i].ID, want)
		}
	}
}

func TestAddEvidence_DeduplicatesByID(t *testing.T) {
	t.Parallel()

	item := &KnowledgeItem{ID: "K-dedup", Title: "Dedup"}
	ev := newTestEvidence("e-1", 1)
	item.AddEvidence(ev)
	item.AddEvidence(ev) // mesmo ID: substitui, não duplica

	if len(item.Evidence) != 1 {
		t.Fatalf("len(Evidence) = %d, esperado 1 (deduplicação por ID)", len(item.Evidence))
	}
}

// ── SetConfidence ────────────────────────────────────────────────────────────

// TestSetConfidence_Promotes verifies that recording measured confidence on
// an existing item re-evaluates its level: 3 evidences at the observation
// floor (confiança 0.10) become learning once the measured confidence
// reaches >= 0.70.
func TestSetConfidence_Promotes(t *testing.T) {
	t.Parallel()

	engine := NewPromotionEngine()
	if err := engine.Register(&KnowledgeItem{ID: "K-conf", Title: "Conf", Confidence: 0.10}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	addEvidenceRange(t, engine, "K-conf", 1, 3)

	got, _ := engine.Get("K-conf")
	if got.Level != LevelObservation {
		t.Fatalf("3 evidências com confiança 0.10 → %q, esperado %q", got.Level, LevelObservation)
	}

	if err := engine.SetConfidence("K-conf", 0.8); err != nil {
		t.Fatalf("SetConfidence: %v", err)
	}

	got, _ = engine.Get("K-conf")
	if got.Confidence != 0.8 {
		t.Errorf("Confidence = %.2f, esperado 0.8", got.Confidence)
	}
	if got.Level != LevelLearning {
		t.Errorf("3 evidências + confiança 0.8 → %q, esperado %q", got.Level, LevelLearning)
	}
}

func TestSetConfidence_Validation(t *testing.T) {
	t.Parallel()

	engine := NewPromotionEngine()
	if err := engine.Register(&KnowledgeItem{ID: "K-conf", Title: "Conf"}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	for _, bad := range []float64{-0.1, 1.5} {
		if err := engine.SetConfidence("K-conf", bad); err == nil {
			t.Errorf("SetConfidence(%v) deve retornar erro (fora de 0-1)", bad)
		}
	}
	if err := engine.SetConfidence("nao-existe", 0.8); err == nil {
		t.Error("SetConfidence em item inexistente deve retornar erro")
	}
}

// ── Reevaluate ───────────────────────────────────────────────────────────────

// TestReevaluate_PromotesStaleItems verifies that Reevaluate re-derives
// levels from the current thresholds: an item persisted at the observation
// floor with 3 evidences + confidence 0.95 (as if confidence had changed
// since the last evaluation) is promoted to learning.
func TestReevaluate_PromotesStaleItems(t *testing.T) {
	t.Parallel()

	engine := NewPromotionEngine()
	stale := &KnowledgeItem{
		ID:         "K-stale",
		Title:      "Item defasado",
		Level:      LevelObservation, // nível congelado, thresholds já dizem learning
		Confidence: 0.95,
		Evidence: []Evidence{
			{ID: "e1", Kind: "test", Source: "s", Description: "1", Timestamp: time.Now()},
			{ID: "e2", Kind: "test", Source: "s", Description: "2", Timestamp: time.Now()},
			{ID: "e3", Kind: "test", Source: "s", Description: "3", Timestamp: time.Now()},
		},
	}
	if err := engine.Register(stale); err != nil {
		t.Fatalf("Register: %v", err)
	}

	promoted, newLaws := engine.Reevaluate()
	if promoted != 1 {
		t.Errorf("promoted = %d, want 1", promoted)
	}
	if len(newLaws) != 0 {
		t.Errorf("newLaws = %v, want none", newLaws)
	}

	got, ok := engine.Get("K-stale")
	if !ok {
		t.Fatal("item não encontrado após Reevaluate")
	}
	if got.Level != LevelLearning {
		t.Errorf("level = %q, want %q", got.Level, LevelLearning)
	}
}

// TestReevaluate_ReportsNewLaws verifies that an item crossing the law
// threshold (>= 200 evidences, 0 rollbacks, confidence >= 0.99) is reported
// in newLaws and promoted to law.
func TestReevaluate_ReportsNewLaws(t *testing.T) {
	t.Parallel()

	engine := NewPromotionEngine()
	candidate := &KnowledgeItem{
		ID:           "K-law",
		Title:        "Lei candidata",
		Level:        LevelTheory, // defasado: thresholds já dizem law
		Confidence:   0.99,
		Projects:     10,
		Reproducible: true,
	}
	for i := 0; i < ThresholdLawEvidence; i++ {
		candidate.Evidence = append(candidate.Evidence, Evidence{
			ID: fmt.Sprintf("e-%d", i), Kind: "test", Source: "s",
			Description: fmt.Sprintf("ev %d", i), Timestamp: time.Now(),
		})
	}
	if err := engine.Register(candidate); err != nil {
		t.Fatalf("Register: %v", err)
	}

	promoted, newLaws := engine.Reevaluate()
	if promoted != 1 {
		t.Errorf("promoted = %d, want 1", promoted)
	}
	if len(newLaws) != 1 || newLaws[0] != "K-law" {
		t.Errorf("newLaws = %v, want [K-law]", newLaws)
	}

	got, _ := engine.Get("K-law")
	if got.Level != LevelLaw {
		t.Errorf("level = %q, want %q", got.Level, LevelLaw)
	}
}

// TestReevaluate_PreservesConstitution verifies that a Don-approved
// constitution is never demoted by a re-evaluation, even when the thresholds
// would place the item lower.
func TestReevaluate_PreservesConstitution(t *testing.T) {
	t.Parallel()

	engine := NewPromotionEngine()
	item := &KnowledgeItem{
		ID:    "K-const",
		Title: "Princípio constitucional",
		Level: LevelConstitution, // aprovação manual do Don
	}
	if err := engine.Register(item); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if _, err := engine.AddEvidence(item.ID, newTestEvidence("e1", 1)); err != nil {
		t.Fatalf("AddEvidence: %v", err)
	}

	promoted, newLaws := engine.Reevaluate()
	if promoted != 0 {
		t.Errorf("promoted = %d, want 0 (constituição nunca é re-promovida)", promoted)
	}
	if len(newLaws) != 0 {
		t.Errorf("newLaws = %v, want none", newLaws)
	}

	got, _ := engine.Get(item.ID)
	if got.Level != LevelConstitution {
		t.Errorf("level = %q, want %q (constituição preservada)", got.Level, LevelConstitution)
	}
}

// ── ApproveLaw ────────────────────────────────────────────────────────────────

// TestApproveLaw_PromotesAndSetsKnown verifies that the Don's manual
// promotion to law sets LevelLaw and StatusKnown, and fails for unknown items.
func TestApproveLaw_PromotesAndSetsKnown(t *testing.T) {
	t.Parallel()

	engine := NewPromotionEngine()
	item := &KnowledgeItem{
		ID:         "K-law-promo",
		Title:      "Lei promovida manualmente",
		Level:      LevelLearning,
		Confidence: 0.95,
		Status:     StatusSupported,
	}
	if err := engine.Register(item); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Antes da promoção: nível learning, status SUPPORTED.
	got, _ := engine.Get(item.ID)
	if got.Level != LevelLearning {
		t.Fatalf("antes da promoção Level = %q, esperado %q", got.Level, LevelLearning)
	}
	if got.Status != StatusSupported {
		t.Fatalf("antes da promoção Status = %q, esperado %q", got.Status, StatusSupported)
	}

	// Promoção manual do Don.
	if err := engine.ApproveLaw(item.ID); err != nil {
		t.Fatalf("ApproveLaw: %v", err)
	}

	got, _ = engine.Get(item.ID)
	if got.Level != LevelLaw {
		t.Errorf("após ApproveLaw Level = %q, esperado %q", got.Level, LevelLaw)
	}
	if got.Status != StatusKnown {
		t.Errorf("após ApproveLaw Status = %q, esperado %q", got.Status, StatusKnown)
	}
	if got.PromotedAt.IsZero() {
		t.Error("PromotedAt deve ser registrado após ApproveLaw")
	}

	// ApproveLaw em item inexistente deve falhar.
	if err := engine.ApproveLaw("nao-existe"); err == nil {
		t.Error("ApproveLaw em item inexistente deve retornar erro")
	}
}

// TestPromote_SetsKnownOnLaw verifies that automatic promotion to law via
// Promote() also sets StatusKnown (future-proof for items reaching 200
// evidences).
func TestPromote_SetsKnownOnLaw(t *testing.T) {
	t.Parallel()

	item := &KnowledgeItem{
		ID:           "K-auto-law",
		Title:        "Lei automática",
		Confidence:   0.99,
		Projects:     10,
		Reproducible: true,
		Status:       StatusSupported,
	}
	// Simula 200 evidências para atingir law.
	for i := 0; i < ThresholdLawEvidence; i++ {
		item.AddEvidence(newTestEvidence(fmt.Sprintf("ev-%d", i), 1))
	}

	if item.Level != LevelLaw {
		t.Fatalf("Level = %q, esperado %q", item.Level, LevelLaw)
	}
	if item.Status != StatusKnown {
		t.Errorf("Status = %q, esperado %q (deve ser KNOWN ao atingir law)",
			item.Status, StatusKnown)
	}
}

// ExamplePromotionEngine demonstra o ciclo de vida do conhecimento com o caso
// real do Cosca: a lei "Nunca executar como root automaticamente", sustentada
// por 4 evidências (incidente F001, auditoria de segurança, teste do sandbox
// e o fix da Onda 1) — o que promove o item a learning.
func ExamplePromotionEngine() {
	engine := NewPromotionEngine()
	item := &KnowledgeItem{
		ID:           "K-27",
		Title:        "Nunca executar como root automaticamente",
		Confidence:   0.95,
		Projects:     1,
		Reproducible: true,
	}
	_ = engine.Register(item)

	evidences := []Evidence{
		{
			ID: "jail-break-1", Kind: "incident", Source: "F001",
			Description: "Fuga da jail: processo executou como root fora do sandbox",
			Timestamp:   time.Date(2026, 7, 30, 9, 15, 0, 0, time.UTC),
		},
		{
			ID: "auditoria-seguranca-2026-07-31", Kind: "audit", Source: "red-team-A1",
			Description: "Red team A1: jail deve recusar root automaticamente",
			Timestamp:   time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC),
		},
		{
			ID: "test-sandbox", Kind: "test", Source: "TestJail_RootUnsafe",
			Description: "Teste confirma: sandbox recusa execução como root",
			Timestamp:   time.Date(2026, 7, 31, 14, 30, 0, 0, time.UTC),
		},
		{
			ID: "onda-1-fix", Kind: "test", Source: "onda-1",
			Description: "Fix Onda 1: jail fail-closed recusa root automaticamente",
			Timestamp:   time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC),
		},
	}
	for _, ev := range evidences {
		if _, err := engine.AddEvidence(item.ID, ev); err != nil {
			panic(err)
		}
	}

	got, _ := engine.Get(item.ID)
	fmt.Println("Level:", got.Level)
	fmt.Println(got.WhyExists())

	// Output:
	// Level: learning
	// K-27 "Nunca executar como root automaticamente" [learning]
	// Por que eu existo? 4 evidências, 1 projeto, confiança 0.95, 0 rollbacks.
	// Reproduzível: sim
	// Evidências:
	//   - [jail-break-1] incident: Fuga da jail: processo executou como root fora do sandbox [F001, 2026-07-30]
	//   - [auditoria-seguranca-2026-07-31] audit: Red team A1: jail deve recusar root automaticamente [red-team-A1, 2026-07-31]
	//   - [test-sandbox] test: Teste confirma: sandbox recusa execução como root [TestJail_RootUnsafe, 2026-07-31]
	//   - [onda-1-fix] test: Fix Onda 1: jail fail-closed recusa root automaticamente [onda-1, 2026-08-01]
}
