package evals

import (
	"context"
	"strconv"
	"testing"
	"time"
)

// secretNumberOracle validates exactly one hidden candidate.
type secretNumberOracle struct {
	secret string
}

func (o secretNumberOracle) Verify(_ context.Context, candidate string) (bool, string, error) {
	return candidate == o.secret, "", nil
}

// scanSolver scans 1..100 in order, skipping candidates it has already tried
// (it uses the history as memory). With an EMPTY history it always returns "1"
// — it cannot remember that it already tried 1, so it loops forever.
func scanSolver(_ context.Context, history []Attempt) (string, string, error) {
	tried := map[string]bool{}
	for _, a := range history {
		tried[a.Candidate] = true
	}
	for i := 1; i <= 100; i++ {
		c := strconv.Itoa(i)
		if !tried[c] {
			return c, "", nil
		}
	}
	return "", "", nil // exhausted
}

// pacedScanSolver behaves like scanSolver but adds a short pause per attempt
// so the elapsed time is measurable even on platforms with coarse clock
// resolution (e.g. Windows ~15ms). The pause keeps the "time is measured"
// assertion meaningful without depending on the environment's clock.
func pacedScanSolver(_ context.Context, history []Attempt) (string, string, error) {
	time.Sleep(20 * time.Millisecond)
	return scanSolver(context.Background(), history)
}

func TestRunTrial(t *testing.T) {
	oracle := secretNumberOracle{secret: "3"}
	solver := SolverFunc(pacedScanSolver)

	tr := RunTrial(context.Background(), oracle, solver, 10)
	if !tr.Solved {
		t.Fatalf("expected solved, got %+v", tr)
	}
	if tr.AttemptCount != 3 || tr.Discarded != 2 {
		t.Fatalf("expected 3 attempts / 2 discarded, got %+v", tr)
	}
	if tr.TimeToSolve <= 0 {
		t.Fatalf("expected positive time-to-solve, got %v", tr.TimeToSolve)
	}
}

func TestRunTrialBudgetExhausted(t *testing.T) {
	oracle := secretNumberOracle{secret: "1000"} // never found by scan of 1..100
	solver := SolverFunc(scanSolver)

	tr := RunTrial(context.Background(), oracle, solver, 5)
	if tr.Solved {
		t.Fatalf("expected unsolved (budget exhausted), got %+v", tr)
	}
	if tr.AttemptCount != 5 {
		t.Fatalf("expected 5 attempts (budget), got %d", tr.AttemptCount)
	}
}

func TestRunConditionStats(t *testing.T) {
	oracle := secretNumberOracle{secret: "3"}
	solver := SolverFunc(scanSolver)

	res := RunCondition(context.Background(), oracle, solver, "managed", 5, 10)
	if res.SolvedRate != 1.0 {
		t.Fatalf("expected 100%% solved, got %v", res.SolvedRate)
	}
	if res.MedianAttempts != 3 {
		t.Fatalf("expected median attempts 3, got %d", res.MedianAttempts)
	}
	if res.IQRAttempts != [2]int{3, 3} {
		t.Fatalf("expected IQR [3,3], got %v", res.IQRAttempts)
	}
}

func TestRunAblationManagedWins(t *testing.T) {
	oracle := secretNumberOracle{secret: "42"}
	solver := SolverFunc(scanSolver)

	cmp := RunAblation(context.Background(), oracle, solver, 3, 200)

	// Managed: scans 1..42 using memory → solves in 42 attempts.
	if cmp.Managed.SolvedRate != 1.0 || cmp.Managed.MedianAttempts != 42 {
		t.Fatalf("managed should solve in 42 attempts, got %+v", cmp.Managed)
	}
	// Alone (empty history): always returns "1" → never solves within budget.
	if cmp.Alone.SolvedRate != 0.0 {
		t.Fatalf("alone should never solve, got %+v", cmp.Alone)
	}
	if !cmp.ManagedWins() {
		t.Fatalf("expected ManagedWins=true, got false")
	}
}

func TestMedianHelpers(t *testing.T) {
	if got := medianInt([]int{1, 2, 3}); got != 2 {
		t.Fatalf("median odd: got %d", got)
	}
	if got := medianInt([]int{1, 2, 3, 4}); got != 2 {
		t.Fatalf("median even (floor): got %d", got)
	}
	if got := quantileInt([]int{1, 2, 3, 4}, 0.75); got != 3 {
		t.Fatalf("q75: got %d", got)
	}
	if got := medianDuration([]time.Duration{time.Second, 3 * time.Second, 2 * time.Second}); got != 2*time.Second {
		t.Fatalf("median duration: got %v", got)
	}
}
