package sciengine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewValidates(t *testing.T) {
	e, err := New("exp-001", "teste de precisão", ResultCalculated)
	if err != nil {
		t.Fatal(err)
	}
	if e.ID != "exp-001" || e.Kind != ResultCalculated {
		t.Fatalf("experiment: %+v", e)
	}
	if e.Timestamp == "" {
		t.Fatal("timestamp should be set")
	}
	// Validações.
	if _, err := New("", "x", ResultCalculated); err == nil {
		t.Fatal("expected id error")
	}
	if _, err := New("x", "", ResultCalculated); err == nil {
		t.Fatal("expected name error")
	}
	if _, err := New("x", "x", ResultKind("magic")); err == nil {
		t.Fatal("expected kind error")
	}
}

func TestMetrics(t *testing.T) {
	e, _ := New("e1", "m", ResultSimulated)
	e.SetMetric("accuracy", 0.95)
	e.SetMetric("loss", 0.12)
	if e.Metric("accuracy") != 0.95 {
		t.Fatalf("accuracy = %v", e.Metric("accuracy"))
	}
	if e.Metric("missing") != 0 {
		t.Fatal("missing metric should be 0")
	}
}

func TestRegistryPersistence(t *testing.T) {
	root := t.TempDir()
	r, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	e, _ := New("exp-001", "benchmark", ResultCalculated)
	e.SetMetric("score", 0.88)
	e.Parameters["seed"] = 42
	if err := r.Add(e); err != nil {
		t.Fatal(err)
	}

	r2, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Count() != 1 {
		t.Fatalf("Count = %d, want 1", r2.Count())
	}
	got, ok := r2.Get("exp-001")
	if !ok {
		t.Fatal("Get should find exp-001")
	}
	if got.Metric("score") != 0.88 {
		t.Fatalf("score = %v", got.Metric("score"))
	}
	if _, err := os.Stat(filepath.Join(root, DefaultDir, FileName)); err != nil {
		t.Fatalf("experiments file missing: %v", err)
	}
}

func TestAddDuplicate(t *testing.T) {
	r, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e, _ := New("dup", "x", ResultObserved)
	_ = r.Add(e)
	if err := r.Add(e); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestBest(t *testing.T) {
	r, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e1, _ := New("a", "x", ResultSimulated)
	e1.SetMetric("score", 0.7)
	e2, _ := New("b", "x", ResultSimulated)
	e2.SetMetric("score", 0.92)
	_ = r.Add(e1)
	_ = r.Add(e2)

	best, ok := r.Best("score")
	if !ok || best.ID != "b" {
		t.Fatalf("Best = %+v ok=%v", best, ok)
	}
	if _, ok := r.Best("missing"); ok {
		t.Fatal("Best on missing metric should be false")
	}
}

func TestAllKindsValid(t *testing.T) {
	for _, k := range []ResultKind{ResultObserved, ResultCalculated, ResultSimulated, ResultGenerated, ResultHypothesis} {
		if !k.Valid() {
			t.Fatalf("kind %q should be valid", k)
		}
	}
	if ResultKind("bogus").Valid() {
		t.Fatal("bogus should be invalid")
	}
}
