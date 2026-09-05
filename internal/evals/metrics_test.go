package evals

import (
	"context"
	"math"
	"runtime"
	"testing"
)

const tol = 1e-9

func TestComputeConfusionMatrixMixedResults(t *testing.T) {
	results := []CaseResult{
		{ID: "a", Status: StatusPassed, Reward: 1.0},
		{ID: "b", Status: StatusFailed, Reward: 0.0},
		{ID: "c", Status: StatusError, Reward: 0.0},
		{ID: "d", Status: StatusFailed, Reward: 0.6}, // partial: 2/3 verifies × 0.9
	}
	cm := ComputeConfusionMatrix(results, 0.5)
	if cm.TP != 2 {
		t.Errorf("TP = %d, want 2 (passed + partial-reward)", cm.TP)
	}
	if cm.FN != 2 {
		t.Errorf("FN = %d, want 2 (failed + error)", cm.FN)
	}
	if cm.FP != 0 || cm.TN != 0 {
		t.Errorf("FP/TN = %d/%d, want 0/0", cm.FP, cm.TN)
	}
}

func TestComputeConfusionMatrixThreshold(t *testing.T) {
	results := []CaseResult{
		{ID: "a", Status: StatusFailed, Reward: 0.6},
		{ID: "b", Status: StatusFailed, Reward: 0.4},
	}
	// At the default threshold 0.5 only the 0.6 case is positive.
	cm := ComputeConfusionMatrix(results, 0.5)
	if cm.TP != 1 || cm.FN != 1 {
		t.Errorf("default threshold: TP/FN = %d/%d, want 1/1", cm.TP, cm.FN)
	}
	// A stricter threshold moves the 0.6 case to negative.
	cm = ComputeConfusionMatrix(results, 0.8)
	if cm.TP != 0 || cm.FN != 2 {
		t.Errorf("threshold 0.8: TP/FN = %d/%d, want 0/2", cm.TP, cm.FN)
	}
}

func TestAccuracy(t *testing.T) {
	cases := []struct {
		tp, tn, total int
		want          float64
	}{
		{2, 0, 3, 2.0 / 3.0},
		{3, 2, 5, 1.0},
		{0, 0, 0, 0.0},
		{0, 0, 4, 0.0},
	}
	for _, c := range cases {
		if got := Accuracy(c.tp, c.tn, c.total); math.Abs(got-c.want) > tol {
			t.Errorf("Accuracy(%d,%d,%d) = %v, want %v", c.tp, c.tn, c.total, got, c.want)
		}
	}
}

func TestPrecisionRecallEdgeCases(t *testing.T) {
	if got := Precision(0, 0); got != 0 {
		t.Errorf("Precision(0,0) = %v, want 0", got)
	}
	if got := Precision(5, 0); got != 1 {
		t.Errorf("Precision(5,0) = %v, want 1", got)
	}
	if got := Precision(5, 5); math.Abs(got-0.5) > tol {
		t.Errorf("Precision(5,5) = %v, want 0.5", got)
	}
	if got := Recall(0, 0); got != 0 {
		t.Errorf("Recall(0,0) = %v, want 0", got)
	}
	if got := Recall(5, 0); got != 1 {
		t.Errorf("Recall(5,0) = %v, want 1", got)
	}
	if got := Recall(5, 3); math.Abs(got-5.0/8.0) > tol {
		t.Errorf("Recall(5,3) = %v, want %v", got, 5.0/8.0)
	}
}

func TestFScoreBetaVariants(t *testing.T) {
	// 2 passed, 1 failed → precision 1.0, recall 2/3 → F1 = 0.8.
	wantF1 := 0.8
	if got := FScore(2, 0, 1, 1); math.Abs(got-wantF1) > tol {
		t.Errorf("FScore(2,0,1,beta=1) = %v, want F1 %v", got, wantF1)
	}
	if got := FScore(0, 0, 0, 1); got != 0 {
		t.Errorf("FScore(0,0,0,1) = %v, want 0", got)
	}
	gotBeta2 := FScore(2, 0, 1, 2)
	if math.Abs(gotBeta2-wantF1) < tol {
		t.Errorf("FScore(2,0,1,beta=2) = %v, must differ from F1 %v", gotBeta2, wantF1)
	}
	// beta=2 biases recall (FN penalized harder) → lower score here.
	wantBeta2 := (1 + 4) * 1.0 * (2.0 / 3.0) / (4*1.0 + 2.0/3.0) // = 5/7
	if math.Abs(gotBeta2-wantBeta2) > tol {
		t.Errorf("FScore(2,0,1,beta=2) = %v, want %v", gotBeta2, wantBeta2)
	}
}

func TestMatthewCorr(t *testing.T) {
	cases := []struct {
		name           string
		tp, fp, tn, fn int
		want           float64
	}{
		{"perfect", 5, 0, 5, 0, 1.0},
		{"random", 5, 5, 5, 5, 0.0},
		{"inverted", 0, 5, 0, 5, -1.0},
		{"degenerate", 0, 0, 0, 0, 0.0},
		{"one-sided", 2, 0, 0, 1, 0.0}, // TN=FP=0 → no denominator
	}
	for _, c := range cases {
		got := MatthewCorr(c.tp, c.fp, c.tn, c.fn)
		if math.Abs(got-c.want) > tol {
			t.Errorf("%s: MatthewCorr(%d,%d,%d,%d) = %v, want %v",
				c.name, c.tp, c.fp, c.tn, c.fn, got, c.want)
		}
	}
}

func TestBootstrapStdIdenticalRewards(t *testing.T) {
	rewards := []float64{0.5, 0.5, 0.5, 0.5}
	if got := BootstrapStd(rewards, 1000); got != 0 {
		t.Errorf("BootstrapStd(identical) = %v, want 0", got)
	}
}

func TestBootstrapStdVariedRewards(t *testing.T) {
	rewards := []float64{0, 1, 1, 0, 1, 1, 0, 1, 0.5, 0.5}
	if got := BootstrapStd(rewards, 1000); got <= 0 {
		t.Errorf("BootstrapStd(varied) = %v, want > 0", got)
	}
	if got := BootstrapStd(nil, 1000); got != 0 {
		t.Errorf("BootstrapStd(empty) = %v, want 0", got)
	}
}

func TestBootstrapStdDeterministicWithSameSeed(t *testing.T) {
	rewards := []float64{0, 1, 1, 0, 1, 1, 0, 1, 0.5, 0.5, 0.2, 0.9}
	a := BootstrapStdSeed(rewards, 500, 42)
	b := BootstrapStdSeed(rewards, 500, 42)
	if a != b {
		t.Errorf("same seed must be deterministic: %v vs %v", a, b)
	}
	// The default-seeded call must also be reproducible.
	c := BootstrapStd(rewards, 500)
	d := BootstrapStd(rewards, 500)
	if c != d {
		t.Errorf("default seed must be deterministic: %v vs %v", c, d)
	}
}

func TestRunSuiteComputesMetrics(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("casos usam Verify via sh -c ('true'/'false') — shell POSIX ausente no Windows nativo")
	}
	s := &Suite{
		Suite: "s",
		Cases: []Case{
			{ID: "a", Verify: []string{"true"}},  // pass → reward 1.0
			{ID: "b", Verify: []string{"true"}},  // pass → reward 1.0
			{ID: "c", Verify: []string{"false"}}, // fail → reward 0.0
		},
	}
	r := &fakeRunner{outcomes: map[string]*CaseOutcome{
		"a": {Status: "passed"},
		"b": {Status: "passed"},
		"c": {Status: "passed"},
	}}
	rep, err := RunSuite(context.Background(), s, r, RunOptions{})
	if err != nil {
		t.Fatalf("RunSuite: %v", err)
	}

	m := rep.Metrics
	if math.Abs(m.Accuracy-2.0/3.0) > tol {
		t.Errorf("accuracy = %v, want 0.667", m.Accuracy)
	}
	if math.Abs(m.Precision-1.0) > tol {
		t.Errorf("precision = %v, want 1.0", m.Precision)
	}
	if math.Abs(m.Recall-2.0/3.0) > tol {
		t.Errorf("recall = %v, want 0.667", m.Recall)
	}
	if math.Abs(m.F1-0.8) > tol {
		t.Errorf("f1 = %v, want 0.8", m.F1)
	}
	if m.MatthewsCorr != 0 {
		t.Errorf("matthews_corr = %v, want 0 (TN=FP=0)", m.MatthewsCorr)
	}
	if m.BootstrapStd <= 0 {
		t.Errorf("bootstrap_std = %v, want > 0 for varied rewards", m.BootstrapStd)
	}
	if !m.HigherIsBetter {
		t.Errorf("higher_is_better = false, want true")
	}
}
