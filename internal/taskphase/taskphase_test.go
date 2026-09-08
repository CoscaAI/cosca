package taskphase

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/search"
)

// ─── DetectPhase tests ───────────────────────────────────────────────────────

func TestDetectPhase_Exploration(t *testing.T) {
	t.Parallel()

	signals := &PhaseSignals{
		HasOpenFiles:            false,
		HasRecentModifications:  false,
		OpenFileCount:           0,
		TargetIsSpecific:        false,
		HasTestTerms:            false,
		HasImplementationTerms:  false,
		HasExplorationTerms:     true,
		SearchHistoryCount:      0,
		TargetHasTests:          false,
	}

	det := DetectPhase(signals)

	if det.Phase != PhaseExploration {
		t.Errorf("expected PhaseExploration, got %v", det.Phase)
	}
	if det.Confidence <= 0.5 {
		t.Errorf("expected confidence > 0.5, got %f", det.Confidence)
	}
	if det.Reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestDetectPhase_Implementation(t *testing.T) {
	t.Parallel()

	signals := &PhaseSignals{
		HasOpenFiles:            true,
		HasRecentModifications:  true,
		OpenFileCount:           3,
		TargetIsSpecific:        true,
		HasTestTerms:            false,
		HasImplementationTerms:  true,
		HasExplorationTerms:     false,
		SearchHistoryCount:      1,
		TargetHasTests:          false,
	}

	det := DetectPhase(signals)

	if det.Phase != PhaseImplementation {
		t.Errorf("expected PhaseImplementation, got %v", det.Phase)
	}
	if det.Confidence <= 0.5 {
		t.Errorf("expected confidence > 0.5, got %f", det.Confidence)
	}
	if det.Reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestDetectPhase_Verification(t *testing.T) {
	t.Parallel()

	signals := &PhaseSignals{
		HasOpenFiles:            true,
		HasRecentModifications:  true,
		OpenFileCount:           1,
		TargetIsSpecific:        true,
		HasTestTerms:            true,
		HasImplementationTerms:  false,
		HasExplorationTerms:     false,
		SearchHistoryCount:      4,
		TargetHasTests:          true,
	}

	det := DetectPhase(signals)

	if det.Phase != PhaseVerification {
		t.Errorf("expected PhaseVerification, got %v", det.Phase)
	}
	if det.Confidence <= 0.5 {
		t.Errorf("expected confidence > 0.5, got %f", det.Confidence)
	}
	if det.Reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestDetectPhase_Ambiguous(t *testing.T) {
	t.Parallel()

	// All signals present → must not panic and must return a valid phase.
	signals := &PhaseSignals{
		HasOpenFiles:            true,
		HasRecentModifications:  true,
		OpenFileCount:           2,
		TargetIsSpecific:        true,
		HasTestTerms:            true,
		HasImplementationTerms:  true,
		HasExplorationTerms:     true,
		SearchHistoryCount:      2,
		TargetHasTests:          true,
	}

	det := DetectPhase(signals)

	switch det.Phase {
	case PhaseExploration, PhaseImplementation, PhaseVerification:
		// valid
	default:
		t.Errorf("unexpected phase %v", det.Phase)
	}

	if det.Confidence < 0 || det.Confidence > 1.0 {
		t.Errorf("confidence out of range: %f", det.Confidence)
	}

	if det.Reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestDetectPhase_NilSignals(t *testing.T) {
	t.Parallel()

	det := DetectPhase(nil)

	if det.Phase != PhaseExploration {
		t.Errorf("expected PhaseExploration for nil signals, got %v", det.Phase)
	}
	if det.Confidence != 0 {
		t.Errorf("expected confidence 0 for nil signals, got %f", det.Confidence)
	}
	if det.Reason != "sem sinais" {
		t.Errorf("expected reason 'sem sinais', got %q", det.Reason)
	}
}

func TestDetectPhase_Determinism(t *testing.T) {
	t.Parallel()

	signals := &PhaseSignals{
		HasOpenFiles:            true,
		HasRecentModifications:  false,
		OpenFileCount:           2,
		TargetIsSpecific:        true,
		HasTestTerms:            false,
		HasImplementationTerms:  true,
		HasExplorationTerms:     false,
		SearchHistoryCount:      1,
		TargetHasTests:          false,
	}

	det1 := DetectPhase(signals)
	det2 := DetectPhase(signals)

	if det1.Phase != det2.Phase {
		t.Errorf("non-deterministic: phase %v != %v", det1.Phase, det2.Phase)
	}
	if det1.Confidence != det2.Confidence {
		t.Errorf("non-deterministic: confidence %f != %f", det1.Confidence, det2.Confidence)
	}
	if det1.Reason != det2.Reason {
		t.Errorf("non-deterministic: reason %q != %q", det1.Reason, det2.Reason)
	}
}

// ─── PhaseSearchParams tests ─────────────────────────────────────────────────

func TestPhaseSearchParams_Exploration(t *testing.T) {
	t.Parallel()

	detection := PhaseDetection{Phase: PhaseExploration, Confidence: 0.70, Reason: "teste"}
	params := search.SearchParams{Query: "como funciona o search"}

	result := PhaseSearchParams(params, detection)

	if result.Limit != 30 {
		t.Errorf("exploration: expected Limit=30, got %d", result.Limit)
	}
	if !result.EnableFTS {
		t.Error("exploration: expected EnableFTS=true")
	}
	if !result.EnableVector {
		t.Error("exploration: expected EnableVector=true")
	}
	if result.EnableGraph {
		t.Error("exploration: expected EnableGraph=false")
	}
	if !result.EnableFacets {
		t.Error("exploration: expected EnableFacets=true")
	}
	if result.MinScore != 0 {
		t.Errorf("exploration: expected MinScore=0, got %f", result.MinScore)
	}
	// Query must be preserved.
	if result.Query != "como funciona o search" {
		t.Errorf("exploration: query was modified: %q", result.Query)
	}
}

func TestPhaseSearchParams_Implementation(t *testing.T) {
	t.Parallel()

	detection := PhaseDetection{Phase: PhaseImplementation, Confidence: 0.80, Reason: "teste"}
	params := search.SearchParams{Query: "implementar endpoint REST"}

	result := PhaseSearchParams(params, detection)

	if result.Limit != 20 {
		t.Errorf("implementation: expected Limit=20, got %d", result.Limit)
	}
	if !result.EnableFTS {
		t.Error("implementation: expected EnableFTS=true")
	}
	if !result.EnableVector {
		t.Error("implementation: expected EnableVector=true")
	}
	if !result.EnableGraph {
		t.Error("implementation: expected EnableGraph=true")
	}
	if result.EnableFacets {
		t.Error("implementation: expected EnableFacets=false")
	}
	if result.MinScore != 0.15 {
		t.Errorf("implementation: expected MinScore=0.15, got %f", result.MinScore)
	}
	if result.Query != "implementar endpoint REST" {
		t.Errorf("implementation: query was modified: %q", result.Query)
	}
}

func TestPhaseSearchParams_Verification(t *testing.T) {
	t.Parallel()

	detection := PhaseDetection{Phase: PhaseVerification, Confidence: 0.90, Reason: "teste"}
	params := search.SearchParams{Query: "testar handler de autenticação"}

	result := PhaseSearchParams(params, detection)

	if result.Limit != 10 {
		t.Errorf("verification: expected Limit=10, got %d", result.Limit)
	}
	if !result.EnableFTS {
		t.Error("verification: expected EnableFTS=true")
	}
	if !result.EnableVector {
		t.Error("verification: expected EnableVector=true")
	}
	if result.EnableGraph {
		t.Error("verification: expected EnableGraph=false")
	}
	if result.EnableFacets {
		t.Error("verification: expected EnableFacets=false")
	}
	if result.MinScore != 0.30 {
		t.Errorf("verification: expected MinScore=0.30, got %f", result.MinScore)
	}
	if result.Query != "testar handler de autenticação" {
		t.Errorf("verification: query was modified: %q", result.Query)
	}
}

// ─── Phase.String() ──────────────────────────────────────────────────────────

func TestPhaseString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		phase Phase
		want  string
	}{
		{PhaseExploration, "exploration"},
		{PhaseImplementation, "implementation"},
		{PhaseVerification, "verification"},
		{Phase(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.phase.String(); got != tt.want {
			t.Errorf("Phase(%d).String() = %q, want %q", int(tt.phase), got, tt.want)
		}
	}
}

// ─── ContainsTerm helper ─────────────────────────────────────────────────────

func TestContainsTerm(t *testing.T) {
	t.Parallel()

	if !ContainsTerm("implementar um endpoint", ImplementationTerms) {
		t.Error("expected to find 'implementar' in ImplementationTerms")
	}
	if ContainsTerm("implementar um endpoint", ExplorationTerms) {
		t.Error("did not expect 'implementar' in ExplorationTerms")
	}
	if !ContainsTerm("testar o handler", VerificationTerms) {
		t.Error("expected to find 'testar' in VerificationTerms")
	}
	if ContainsTerm("hello world", nil) {
		t.Error("nil terms map should return false")
	}
}

// ─── Term lists exported and non-empty ───────────────────────────────────────

func TestTermListsExported(t *testing.T) {
	t.Parallel()

	if len(ExplorationTerms) == 0 {
		t.Error("ExplorationTerms is empty")
	}
	if len(ImplementationTerms) == 0 {
		t.Error("ImplementationTerms is empty")
	}
	if len(VerificationTerms) == 0 {
		t.Error("VerificationTerms is empty")
	}
}
