package router

import (
	"errors"
	"math"
	"testing"
)

func TestWeightsForMode(t *testing.T) {
	tests := []struct {
		name      string
		mode      Mode
		wantSum   float64
		dominant  func(Weights) float64
		dominantN string
	}{
		{"balanced", ModeBalanced, 1.0, func(w Weights) float64 { return w.Health }, "health"},
		{"fast", ModeFast, 1.0, func(w Weights) float64 { return w.Latency }, "latency"},
		{"cheap", ModeCheap, 1.0, func(w Weights) float64 { return w.Cost }, "cost"},
		{"quality", ModeQuality, 1.0, func(w Weights) float64 { return w.TaskFit }, "taskFit"},
		{"offline", ModeOffline, 1.0, func(w Weights) float64 { return w.Health }, "health"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := WeightsForMode(tt.mode)
			sum := w.Health + w.Cost + w.Latency + w.TaskFit
			if math.Abs(sum-tt.wantSum) > 1e-9 {
				t.Fatalf("weights sum = %v, want %v", sum, tt.wantSum)
			}

			dom := tt.dominant(w)
			for _, v := range []float64{w.Health, w.Cost, w.Latency, w.TaskFit} {
				if v > dom {
					t.Fatalf("dominant metric %s=%v is not the maximum", tt.dominantN, dom)
				}
			}
		})
	}
}

func TestWeightsDominantMetric(t *testing.T) {
	tests := []struct {
		mode Mode
		key  string
	}{
		{ModeFast, "latency"},
		{ModeCheap, "cost"},
		{ModeQuality, "taskFit"},
	}

	for _, tt := range tests {
		w := WeightsForMode(tt.mode)
		got := map[string]float64{
			"health":  w.Health,
			"cost":    w.Cost,
			"latency": w.Latency,
			"taskFit": w.TaskFit,
		}[tt.key]
		for k, v := range map[string]float64{
			"health":  w.Health,
			"cost":    w.Cost,
			"latency": w.Latency,
			"taskFit": w.TaskFit,
		} {
			if k == tt.key {
				continue
			}
			if v >= got {
				t.Fatalf("mode %s: %s=%v should dominate but %s=%v is not lower", tt.mode, tt.key, got, k, v)
			}
		}
	}
}

func TestModeFromString(t *testing.T) {
	tests := []struct {
		in      string
		want    Mode
		wantErr bool
	}{
		{"balanced", ModeBalanced, false},
		{"fast", ModeFast, false},
		{"cheap", ModeCheap, false},
		{"quality", ModeQuality, false},
		{"offline", ModeOffline, false},
		{"FAST", ModeFast, false},
		{" Cheap ", ModeCheap, false},
		{"", "", true},
		{"unknown", "", true},
		{"qualityy", "", true},
	}

	for _, tt := range tests {
		got, err := ModeFromString(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("ModeFromString(%q): expected error, got %q", tt.in, got)
			}
			if !errors.Is(err, ErrUnknownMode) {
				t.Fatalf("ModeFromString(%q): error %v does not wrap ErrUnknownMode", tt.in, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ModeFromString(%q): unexpected error %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("ModeFromString(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestScore(t *testing.T) {
	// All metrics at 1.0 with balanced weights yields exactly 1.0.
	c := Candidate{Health: 1, Cost: 1, Latency: 1, TaskFit: 1}
	if got := Score(c, WeightsForMode(ModeBalanced)); math.Abs(got-1.0) > 1e-9 {
		t.Fatalf("Score(all 1.0, balanced) = %v, want 1.0", got)
	}

	// Zero metrics yield zero.
	z := Candidate{}
	if got := Score(z, WeightsForMode(ModeBalanced)); got != 0 {
		t.Fatalf("Score(all 0) = %v, want 0", got)
	}

	// Single-metric mode: fast weights all on latency.
	c2 := Candidate{Health: 0, Cost: 0, Latency: 1, TaskFit: 0}
	if got := Score(c2, WeightsForMode(ModeFast)); math.Abs(got-0.55) > 1e-9 {
		t.Fatalf("Score(latency=1, fast) = %v, want 0.55", got)
	}
}

func TestRankOrderingAndExclusion(t *testing.T) {
	candidates := []Candidate{
		{Name: "openai", Health: 1, Cost: 0.2, Latency: 0.5, TaskFit: 0.9, Available: true},
		{Name: "anthropic", Health: 1, Cost: 0.1, Latency: 0.6, TaskFit: 0.95, Available: true},
		{Name: "dead-open", Health: 0, Cost: 1, Latency: 1, TaskFit: 1, Available: true},    // open circuit
		{Name: "unavailable", Health: 1, Cost: 1, Latency: 1, TaskFit: 1, Available: false}, // not available
		{Name: "deepseek", Health: 1, Cost: 0.9, Latency: 0.4, TaskFit: 0.8, Available: true},
	}

	got, err := Rank(candidates, ModeBalanced)
	if err != nil {
		t.Fatalf("Rank: unexpected error %v", err)
	}

	for _, c := range got {
		if c.Name == "dead-open" || c.Name == "unavailable" {
			t.Fatalf("Rank returned excluded candidate %q", c.Name)
		}
	}

	if len(got) != 3 {
		t.Fatalf("Rank returned %d candidates, want 3", len(got))
	}

	for i := 1; i < len(got); i++ {
		if Score(got[i-1], WeightsForMode(ModeBalanced)) < Score(got[i], WeightsForMode(ModeBalanced)) {
			t.Fatalf("Rank not sorted descending at index %d: %v < %v", i, got[i-1].Name, got[i].Name)
		}
	}
}

func TestRankDeterministicNameTieBreak(t *testing.T) {
	// Identical metrics across candidates forces the Name tie-break.
	candidates := []Candidate{
		{Name: "zeta", Health: 1, Cost: 1, Latency: 1, TaskFit: 1, Available: true},
		{Name: "alpha", Health: 1, Cost: 1, Latency: 1, TaskFit: 1, Available: true},
		{Name: "mike", Health: 1, Cost: 1, Latency: 1, TaskFit: 1, Available: true},
	}

	got, err := Rank(candidates, ModeBalanced)
	if err != nil {
		t.Fatalf("Rank: unexpected error %v", err)
	}

	want := []string{"alpha", "mike", "zeta"}
	for i, name := range want {
		if got[i].Name != name {
			t.Fatalf("Rank[%d] = %q, want %q (deterministic name tie-break)", i, got[i].Name, name)
		}
	}
}

func TestRankFailOpen(t *testing.T) {
	candidates := []Candidate{
		{Name: "openai", Health: 1, Cost: 0.2, Latency: 0.5, TaskFit: 0.9, Available: false},
		{Name: "anthropic", Health: 0, Cost: 0.1, Latency: 0.6, TaskFit: 0.95, Available: true},
	}

	got, err := Rank(candidates, ModeBalanced)
	if err != nil {
		t.Fatalf("Rank fail-open: unexpected error %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Rank fail-open returned %d candidates, want 2 (all original)", len(got))
	}
	if got[0].Name != "openai" {
		t.Fatalf("Rank fail-open best = %q, want openai", got[0].Name)
	}
}

func TestRankOffline(t *testing.T) {
	candidates := []Candidate{
		{Name: "openai", Health: 1, Cost: 1, Latency: 1, TaskFit: 1, Available: true, Offline: false},
		{Name: "ollama", Health: 1, Cost: 1, Latency: 1, TaskFit: 1, Available: true, Offline: true},
		{Name: "local", Health: 1, Cost: 1, Latency: 1, TaskFit: 1, Available: true, Offline: true},
	}

	got, err := Rank(candidates, ModeOffline)
	if err != nil {
		t.Fatalf("Rank offline: unexpected error %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Rank offline returned %d candidates, want 2", len(got))
	}
	for _, c := range got {
		if !c.Offline {
			t.Fatalf("Rank offline returned non-offline candidate %q", c.Name)
		}
	}
}

func TestRankOfflineNone(t *testing.T) {
	candidates := []Candidate{
		{Name: "openai", Health: 1, Cost: 1, Latency: 1, TaskFit: 1, Available: true, Offline: false},
	}

	_, err := Rank(candidates, ModeOffline)
	if !errors.Is(err, ErrNoOfflineProvider) {
		t.Fatalf("Rank offline (none): err = %v, want ErrNoOfflineProvider", err)
	}
}

func TestRankEmpty(t *testing.T) {
	_, err := Rank(nil, ModeBalanced)
	if !errors.Is(err, ErrNoCandidates) {
		t.Fatalf("Rank(nil): err = %v, want ErrNoCandidates", err)
	}
}

func TestRankUnknownMode(t *testing.T) {
	_, err := Rank([]Candidate{{Name: "openai", Available: true, Health: 1}}, Mode("bogus"))
	if !errors.Is(err, ErrUnknownMode) {
		t.Fatalf("Rank(unknown): err = %v, want ErrUnknownMode", err)
	}
}

func TestDecide(t *testing.T) {
	candidates := []Candidate{
		{Name: "openai", Health: 1, Cost: 0.5, Latency: 0.5, TaskFit: 0.5, Available: true},
	}

	t.Run("allowed above threshold", func(t *testing.T) {
		v := Decide(candidates, ModeBalanced, 0.4)
		if !v.Allowed {
			t.Fatalf("Decide: expected allowed, got reason %q", v.Reason)
		}
		if len(v.Ordered) != 1 {
			t.Fatalf("Decide: Ordered len = %d, want 1", len(v.Ordered))
		}
	})

	t.Run("rejected below threshold", func(t *testing.T) {
		v := Decide(candidates, ModeBalanced, 0.9)
		if v.Allowed {
			t.Fatalf("Decide: expected not allowed")
		}
		if v.Reason == "" {
			t.Fatalf("Decide: expected non-empty reason")
		}
	})

	t.Run("empty input", func(t *testing.T) {
		v := Decide(nil, ModeBalanced, 0.5)
		if v.Allowed {
			t.Fatalf("Decide(nil): expected not allowed")
		}
		if v.Reason != ErrNoCandidates.Error() {
			t.Fatalf("Decide(nil): reason %q does not match ErrNoCandidates", v.Reason)
		}
	})
}

func TestWeightsValidate(t *testing.T) {
	tests := []struct {
		name    string
		w       Weights
		wantErr bool
	}{
		{"valid balanced", WeightsForMode(ModeBalanced), false},
		{"negative health", Weights{Health: -0.1, Cost: 0.4, Latency: 0.4, TaskFit: 0.3}, true},
		{"negative cost", Weights{Health: 0.4, Cost: -0.1, Latency: 0.4, TaskFit: 0.3}, true},
		{"sum too low", Weights{Health: 0.2, Cost: 0.2, Latency: 0.2, TaskFit: 0.2}, true},
		{"sum too high", Weights{Health: 0.5, Cost: 0.5, Latency: 0.5, TaskFit: 0.5}, true},
		{"all zero", Weights{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.w.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("Validate(%+v): expected error, got nil", tt.w)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate(%+v): unexpected error %v", tt.w, err)
			}
		})
	}
}
