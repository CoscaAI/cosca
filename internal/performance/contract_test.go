package performance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// =============================================================================
// Stats
// =============================================================================

func TestStats_Percentiles(t *testing.T) {
	v := []int64{10, 20, 30, 40, 50}
	if got := percentile(v, 0.50); got != 30 {
		t.Errorf("p50 = %d, want 30", got)
	}
	// pos = 0.95*4 = 3.8 → lo=40 hi=50 frac=0.8 → 40 + round(8) = 48.
	if got := percentile(v, 0.95); got != 48 {
		t.Errorf("p95 = %d, want 48 (interpolação linear arredondada)", got)
	}
	// pos = 0.99*4 = 3.96 → lo=40 hi=50 frac=0.96 → 40 + round(9.6) = 50.
	if got := percentile(v, 0.99); got != 50 {
		t.Errorf("p99 = %d, want 50 (interpolação linear arredondada)", got)
	}
	if got := percentile(nil, 0.5); got != 0 {
		t.Errorf("p50(vazio) = %d, want 0", got)
	}
	if got := percentile([]int64{7}, 0.9); got != 7 {
		t.Errorf("p90(1 elem) = %d, want 7", got)
	}
}

func TestStats_MeanMinMaxStddev(t *testing.T) {
	v := []int64{1, 2, 3, 4, 5}
	if got := mean(v); got != 3 {
		t.Errorf("mean = %d, want 3", got)
	}
	if got := min(v); got != 1 {
		t.Errorf("min = %d, want 1", got)
	}
	if got := max(v); got != 5 {
		t.Errorf("max = %d, want 5", got)
	}
	if got := stddev(v); got < 1.4 || got > 1.5 {
		t.Errorf("stddev = %f, want ~1.414", got)
	}
	if got := stddev(nil); got != 0 {
		t.Errorf("stddev(vazio) = %f, want 0", got)
	}
}

func TestStats_CoV(t *testing.T) {
	// Variação nula → CoV = 0 → confidence high.
	base := Stats{MeanNs: 100, StdDevNs: 1, CoV: 0.01}
	cand := Stats{MeanNs: 100, StdDevNs: 1, CoV: 0.01}
	if got := confidenceFor(base, cand); got != ConfidenceHigh {
		t.Errorf("confidence = %q, want high", got)
	}
	// Variação alta → low.
	base2 := Stats{MeanNs: 100, StdDevNs: 40, CoV: 0.40}
	if got := confidenceFor(base2, cand); got != ConfidenceLow {
		t.Errorf("confidence = %q, want low", got)
	}
}

// =============================================================================
// Runner (o contrato L316)
// =============================================================================

func TestRunner_Keep(t *testing.T) {
	r := NewRunner()
	r.Samples = 3
	r.MinDifferencePct = 2.0

	// Baseline ~100ns, candidato ~80ns → KEEP (candidato mais rápido).
	base := func() (Measurement, error) { return Measurement{LatencyNs: 100, ThroughputPerSec: 10}, nil }
	cand := func() (Measurement, error) { return Measurement{LatencyNs: 80, ThroughputPerSec: 12.5}, nil }

	res, err := r.Run(WorkloadProfile{Name: "vector-search", Dimension: 768}, Experiment{Name: "workers", Value: "8"}, base, cand, "test")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Decision != DecisionKeep {
		t.Errorf("decision = %q, want keep (delta %.1f%%)", res.Decision, res.DeltaPct)
	}
	if res.EpistemicClass != ClassMeasured {
		t.Errorf("epistemic class = %q, want measured (benchmark nunca vira FACT)", res.EpistemicClass)
	}
	if res.Speedup <= 1.0 {
		t.Errorf("speedup = %f, want > 1.0", res.Speedup)
	}
	if res.Provenance.Source != "test" || res.Provenance.Timestamp.IsZero() {
		t.Errorf("provenance incompleta: %+v", res.Provenance)
	}
}

func TestRunner_Reject(t *testing.T) {
	r := NewRunner()
	r.Samples = 3
	r.MinDifferencePct = 2.0

	// Baseline ~100ns, candidato ~120ns → REJECT (candidato mais lento:
	// para LATÊNCIA, menor é melhor — delta positivo = mais lento = reject).
	base := func() (Measurement, error) { return Measurement{LatencyNs: 100, ThroughputPerSec: 10}, nil }
	cand := func() (Measurement, error) { return Measurement{LatencyNs: 120, ThroughputPerSec: 8.3}, nil }

	res, err := r.Run(WorkloadProfile{Name: "vector-search"}, Experiment{Name: "norms", Value: "true", Hypothesis: "pré-calcular normas acelera"}, base, cand, "test")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Decision != DecisionReject {
		t.Errorf("decision = %q, want reject (delta %.1f%%)", res.Decision, res.DeltaPct)
	}
	if res.DeltaPct <= 0 {
		t.Errorf("delta = %.1f%%, want positivo (candidato mais lento)", res.DeltaPct)
	}
}

func TestRunner_InconclusiveWithinNoise(t *testing.T) {
	r := NewRunner()
	r.Samples = 3
	r.MinDifferencePct = 5.0

	// Baseline 100ns, candidato 102ns → 2% de diferença < 5% → INCONCLUSIVE.
	base := func() (Measurement, error) { return Measurement{LatencyNs: 100, ThroughputPerSec: 10}, nil }
	cand := func() (Measurement, error) { return Measurement{LatencyNs: 102, ThroughputPerSec: 9.8}, nil }

	res, err := r.Run(WorkloadProfile{Name: "vector-search"}, Experiment{Name: "unroll4", Value: "true"}, base, cand, "test")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Decision != DecisionInconclusive {
		t.Errorf("decision = %q, want inconclusive (2%% < 5%% mínimo)", res.Decision)
	}
}

func TestRunner_ErrorPropagation(t *testing.T) {
	r := NewRunner()
	base := func() (Measurement, error) { return Measurement{LatencyNs: 100}, nil }
	cand := func() (Measurement, error) { return Measurement{}, os.ErrNotExist }

	if _, err := r.Run(WorkloadProfile{}, Experiment{}, base, cand, "test"); err == nil {
		t.Error("expected error from candidate fn to propagate")
	}
}

// =============================================================================
// Store (memória experimental L316)
// =============================================================================

func TestStore_RecordAndLookup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "experiments.jsonl")
	s := NewStore(path)

	res := &ExperimentResult{
		Workload:   WorkloadProfile{Name: "vector-search", Dimension: 768, DatasetSize: 100000},
		Experiment: Experiment{Name: "norms", Value: "true", Hypothesis: "pré-calcular normas acelera"},
		BaselineStats: Stats{MeanNs: 100, P50Ns: 100},
		ResultStats:   Stats{MeanNs: 120, P50Ns: 120},
		DeltaPct:      -14.5,
		Decision:      DecisionReject,
		Confidence:    ConfidenceHigh,
		Reason:        "overhead de acesso supera benefício",
		EpistemicClass: ClassMeasured,
		Provenance:    Provenance{Source: "test", Timestamp: time.Now().UTC(), Confidence: ConfidenceHigh},
	}
	if err := s.Record(res); err != nil {
		t.Fatalf("record: %v", err)
	}

	// Lookup encontra o rejeitado — o futuro agente "já testamos isso".
	got, ok := s.Lookup("vector-search", "norms")
	if !ok {
		t.Fatal("lookup should find the rejected experiment")
	}
	if got.Decision != DecisionReject {
		t.Errorf("decision = %q, want reject", got.Decision)
	}
	if got.Reason == "" {
		t.Error("reason should be preserved")
	}

	// Lookup de algo nunca testado → não encontra.
	if _, ok := s.Lookup("vector-search", "quantization"); ok {
		t.Error("lookup of untested experiment should fail")
	}
}

func TestStore_AppendOnlyMultiple(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "experiments.jsonl")
	s := NewStore(path)

	// Mesmo experimento, 2 rodadas — a mais recente vence no lookup.
	_ = s.Record(&ExperimentResult{
		Workload: WorkloadProfile{Name: "w1"}, Experiment: Experiment{Name: "x"},
		Decision: DecisionReject, EpistemicClass: ClassMeasured,
		Provenance: Provenance{Source: "run-1", Timestamp: time.Now().UTC(), Confidence: ConfidenceMedium},
	})
	_ = s.Record(&ExperimentResult{
		Workload: WorkloadProfile{Name: "w1"}, Experiment: Experiment{Name: "x"},
		Decision: DecisionKeep, EpistemicClass: ClassMeasured,
		Provenance: Provenance{Source: "run-2", Timestamp: time.Now().UTC(), Confidence: ConfidenceHigh},
	})

	got, _ := s.Lookup("w1", "x")
	if got.Decision != DecisionKeep || got.Provenance.Source != "run-2" {
		t.Errorf("latest should win: decision=%q source=%q", got.Decision, got.Provenance.Source)
	}

	all, err := s.LoadAll()
	if err != nil {
		t.Fatalf("load all: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("load all = %d, want 2 (append-only preserva o histórico)", len(all))
	}
}

func TestStore_EmptyAndMissing(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "nope", "missing.jsonl"))
	if _, err := s.LoadAll(); err != nil {
		t.Errorf("load missing = %v, want nil (histórico vazio é válido)", err)
	}
	if _, ok := s.Lookup("x", "y"); ok {
		t.Error("lookup on empty store should fail")
	}
	// Volátil (sem path) não falha.
	if err := NewStore("").Record(&ExperimentResult{EpistemicClass: ClassMeasured}); err != nil {
		t.Errorf("volatile record = %v, want nil", err)
	}
}

// =============================================================================
// JSON roundtrip do contrato
// =============================================================================

func TestExperimentResult_JSONRoundtrip(t *testing.T) {
	res := &ExperimentResult{
		Workload:       WorkloadProfile{Name: "vector-search", Dimension: 768, DatasetSize: 100000, Limit: 10, Precision: "float32"},
		Experiment:     Experiment{Name: "workers", Value: "12", Hypothesis: "mais workers melhora", Description: "teste"},
		BaselineStats:  Stats{MeanNs: 8050000, P50Ns: 8000000, Samples: 5},
		ResultStats:    Stats{MeanNs: 6500000, P50Ns: 6500000, Samples: 5},
		Speedup:        1.23,
		DeltaPct:       18.75,
		Decision:       DecisionKeep,
		Confidence:     ConfidenceHigh,
		Reason:         "candidato mais rápido",
		EpistemicClass: ClassMeasured,
		Provenance:     Provenance{Source: "autopsy-2", Timestamp: time.Now().UTC(), Evidence: "commit abc", Confidence: ConfidenceHigh},
	}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back ExperimentResult
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Workload.Name != "vector-search" || back.Decision != DecisionKeep {
		t.Errorf("roundtrip perdeu campos: %+v", back)
	}
	if back.Provenance.Source != "autopsy-2" {
		t.Errorf("provenance perdida: %+v", back.Provenance)
	}
}
