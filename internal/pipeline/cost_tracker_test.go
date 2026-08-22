package pipeline

import (
	"strings"
	"testing"
	"time"
)

func TestCostTrackerTrackTask(t *testing.T) {
	c := NewCostTracker()
	c.TrackTask("s1", "t1", "gpt-4o", 1_000_000, 0, 10*time.Millisecond)

	tokens, cost, ok := c.SessionTotal("s1")
	if !ok {
		t.Fatal("session not found")
	}
	if tokens != 1_000_000 {
		t.Fatalf("tokens = %d", tokens)
	}
	// 1M prompt tokens @ $2.50/M = $2.50.
	if cost != 2.50 {
		t.Fatalf("cost = %v, want 2.50", cost)
	}

	// Second task on same session accumulates.
	c.TrackTask("s1", "t2", "gpt-4o", 0, 100_000, 5*time.Millisecond)
	tokens, cost, _ = c.SessionTotal("s1")
	if tokens != 1_100_000 {
		t.Fatalf("tokens = %d", tokens)
	}
	// +100K output @ $10/M = $1.00 → total $3.50.
	if cost != 3.50 {
		t.Fatalf("cost = %v, want 3.50", cost)
	}
}

func TestCostTrackerUnknownModelDefault(t *testing.T) {
	c := NewCostTracker()
	// Unknown model falls back to deepseek-v4 pricing (0.14 / 0.28).
	c.TrackTask("s", "t", "some-new-model", 1_000_000, 0, time.Millisecond)
	_, cost, _ := c.SessionTotal("s")
	if cost != 0.14 {
		t.Fatalf("fallback cost = %v, want 0.14", cost)
	}
}

func TestCostTrackerSessionTotalMissing(t *testing.T) {
	c := NewCostTracker()
	if _, _, ok := c.SessionTotal("ghost"); ok {
		t.Fatal("missing session must return ok=false")
	}
}

func TestCostTrackerFormatReport(t *testing.T) {
	c := NewCostTracker()
	out := c.FormatReport("ghost")
	if !strings.Contains(out, "No cost data") {
		t.Fatalf("missing session report: %q", out)
	}

	c.TrackTask("s2", "t1", "ollama-local", 5000, 2000, 3*time.Millisecond)
	c.TrackTask("s2", "t2", "deepseek-v4", 1000, 500, 1*time.Millisecond)

	out = c.FormatReport("s2")
	if !strings.Contains(out, "COST REPORT: s2") {
		t.Fatalf("header: %q", out)
	}
	if !strings.Contains(out, "By Model") || !strings.Contains(out, "Task Details") {
		t.Fatalf("sections: %q", out)
	}
	if !strings.Contains(out, "ollama-local") || !strings.Contains(out, "deepseek-v4") {
		t.Fatalf("models: %q", out)
	}
	if !strings.Contains(out, "t1") || !strings.Contains(out, "t2") {
		t.Fatalf("tasks: %q", out)
	}
}

func TestCostTrackerEstimateRemaining(t *testing.T) {
	c := NewCostTracker()
	if tok, cost := c.EstimateRemaining("ghost", 5); tok != 0 || cost != 0 {
		t.Fatalf("missing session estimate = %d, %v", tok, cost)
	}

	c.TrackTask("s3", "t1", "gpt-4o-mini", 1_000_000, 0, time.Millisecond) // $0.15
	tok, cost := c.EstimateRemaining("s3", 2)
	if tok != 2_000_000 {
		t.Fatalf("est tokens = %d, want 2M", tok)
	}
	if cost != 0.30 {
		t.Fatalf("est cost = %v, want 0.30", cost)
	}
}

func TestRound6(t *testing.T) {
	if round6(1.23456789) != 1.234568 {
		t.Fatalf("round6 = %v", round6(1.23456789))
	}
	if round6(0) != 0 {
		t.Fatal("round6(0) != 0")
	}
}

func TestModelPricingHasLocal(t *testing.T) {
	if p, ok := ModelPricing["ollama-local"]; !ok || p.Input != 0 || p.Output != 0 {
		t.Fatal("ollama-local must be free")
	}
}
