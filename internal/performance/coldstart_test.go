package performance

import (
	"path/filepath"
	"testing"
	"time"
)

// sampla gera uma amostra com ElapsedMs fixo (nome constante).
func mkSample(ms float64, name string) ColdStartSample {
	s := NewColdStartSample(name, time.Duration(ms)*time.Millisecond, "test")
	s.ElapsedMs = ms // NewColdStartSample passa por Duration; garantimos o valor exato
	return s
}

func TestEvaluateColdStart_NoEvidenceFailsClosed(t *testing.T) {
	// I2: sem evidência = falha (nunca "passa" sem medir).
	empty := ColdStartSummary{Samples: 0}
	if got := EvaluateColdStart(empty, 500); got != ColdStartFail {
		t.Fatalf("sem amostras deve ser fail (fail-closed), got %s", got)
	}
}

func TestEvaluateColdStart_Thresholds(t *testing.T) {
	cases := []struct {
		name    string
		median  float64
		budget  float64
		want    ColdStartDecision
	}{
		{name: "dentro do orçamento", median: 80, budget: 500, want: ColdStartPass},
		{name: "igual ao orçamento", median: 500, budget: 500, want: ColdStartPass},
		{name: "acima mas < 2x", median: 700, budget: 500, want: ColdStartWarn},
		{name: "exatamente 2x", median: 1000, budget: 500, want: ColdStartWarn},
		{name: "acima de 2x (regressão)", median: 1200, budget: 500, want: ColdStartFail},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sum := ColdStartSummary{Samples: 3, MedianMs: c.median}
			if got := EvaluateColdStart(sum, c.budget); got != c.want {
				t.Fatalf("median=%v budget=%v: got %s, want %s", c.median, c.budget, got, c.want)
			}
		})
	}
}

func TestSummarizeColdStart_Empty(t *testing.T) {
	sum := SummarizeColdStart(nil)
	if sum.Samples != 0 {
		t.Fatalf("empty deve ter Samples=0, got %d", sum.Samples)
	}
}

func TestSummarizeColdStart_Aggregates(t *testing.T) {
	samples := []ColdStartSample{
		mkSample(80, "cosca.cts"),
		mkSample(120, "cosca.cts"),
		mkSample(100, "cosca.cts"),
		mkSample(90, "cosca.cts"),
		mkSample(110, "cosca.cts"),
	}
	sum := SummarizeColdStart(samples)
	if sum.Samples != 5 {
		t.Fatalf("Samples=%d, want 5", sum.Samples)
	}
	if sum.MinMs != 80 || sum.MaxMs != 120 {
		t.Fatalf("min/max = %v/%v, want 80/120", sum.MinMs, sum.MaxMs)
	}
	if sum.MedianMs != 100 {
		t.Fatalf("median=%v, want 100", sum.MedianMs)
	}
	if sum.Epistemic != ClassMeasured {
		t.Fatalf("epistemic=%s, want %s", sum.Epistemic, ClassMeasured)
	}
}

func TestColdStartStore_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "coldstart.jsonl")
	store := NewColdStartStore(path)

	if err := store.Record(mkSample(90, "cosca.cts")); err != nil {
		t.Fatalf("record 1: %v", err)
	}
	if err := store.Record(mkSample(110, "cosca.cts")); err != nil {
		t.Fatalf("record 2: %v", err)
	}

	loaded, err := store.LoadAll()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded %d, want 2", len(loaded))
	}
	if loaded[0].ElapsedMs != 90 || loaded[1].ElapsedMs != 110 {
		t.Fatalf("round-trip divergiu: %v", loaded)
	}
}

func TestColdStartStore_VolatileNoPath(t *testing.T) {
	// Sem path = sem persistência (não deve errar).
	store := NewColdStartStore("")
	if err := store.Record(mkSample(88, "cosca.cts")); err != nil {
		t.Fatalf("volatile record: %v", err)
	}
	got, err := store.LoadAll()
	if err != nil {
		t.Fatalf("volatile load: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("volatile deve retornar vazio, got %d", len(got))
	}
}
