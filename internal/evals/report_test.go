package evals

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLatestReportOldFormatMissingFields verifies backward compatibility:
// reports persisted before the reward/metadata fields existed (no
// reward_mean/reward_max/reward_min/reward_sum/pass_rate/metadata) still load
// and default the new fields to zero.
func TestLatestReportOldFormatMissingFields(t *testing.T) {
	dir := t.TempDir()
	old := `{
  "suite": "legacy-suite",
  "started_at": "2026-01-01T00:00:00Z",
  "finished_at": "2026-01-01T00:01:00Z",
  "total": 2,
  "passed": 1,
  "failed": 1,
  "errors": 0,
  "timed_out": 0,
  "cases": [
    {"id": "a", "status": "passed", "duration": "1s", "summary": "ok"},
    {"id": "b", "status": "failed", "duration": "2s", "summary": "nope"}
  ]
}`
	path := filepath.Join(dir, "legacy-suite-20260101-000000.json")
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}

	rep, err := LatestReport(dir, "legacy-suite")
	if err != nil {
		t.Fatalf("LatestReport: %v", err)
	}
	if rep.RewardMean != 0 || rep.RewardMax != 0 || rep.RewardMin != 0 || rep.RewardSum != 0 {
		t.Errorf("old report reward fields should default to 0, got %+v", rep)
	}
	if rep.PassRate != 0 {
		t.Errorf("old report pass_rate should default to 0, got %v", rep.PassRate)
	}
	if rep.Cases[0].Reward != 0 || rep.Cases[0].Weight != 0 {
		t.Errorf("old report case reward/weight should default to 0, got %+v", rep.Cases[0])
	}
	if rep.Metadata.Author != "" {
		t.Errorf("old report metadata should be empty, got %+v", rep.Metadata)
	}
	if rep.Metrics.Accuracy != 0 || rep.Metrics.Precision != 0 || rep.Metrics.Recall != 0 ||
		rep.Metrics.F1 != 0 || rep.Metrics.MatthewsCorr != 0 || rep.Metrics.BootstrapStd != 0 ||
		rep.Metrics.HigherIsBetter {
		t.Errorf("old report metrics should default to zero values, got %+v", rep.Metrics)
	}
}

// TestReportRoundTripNewFields ensures a report written with the new fields
// survives Save → LatestReport with the values intact.
func TestReportRoundTripNewFields(t *testing.T) {
	dir := t.TempDir()
	rep := &Report{
		Suite:     "rt-suite",
		Metadata:  SuiteMetadata{Author: "cosca-kernel", Difficulty: "easy"},
		Total:     1,
		Passed:    1,
		PassRate:  1.0,
		RewardSum: 2.0, RewardMean: 2.0, RewardMax: 2.0, RewardMin: 2.0,
		Metrics: EvalMetrics{
			Accuracy: 1.0, Precision: 1.0, Recall: 1.0, F1: 1.0,
			MatthewsCorr: 1.0, BootstrapStd: 0.0, HigherIsBetter: true,
		},
		Cases: []CaseResult{{
			ID: "a", Status: StatusPassed, Reward: 2.0, RewardExpectation: 0.8, Weight: 2.0,
		}},
	}
	path, err := rep.Save(dir)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !strings.Contains(filepath.Base(path), "rt-suite") {
		t.Errorf("saved file = %q", path)
	}

	got, err := LatestReport(dir, "rt-suite")
	if err != nil {
		t.Fatalf("LatestReport: %v", err)
	}
	if got.RewardSum != 2.0 || got.RewardMean != 2.0 || got.RewardMax != 2.0 || got.RewardMin != 2.0 {
		t.Errorf("round-trip reward agg = %+v", got)
	}
	if got.PassRate != 1.0 {
		t.Errorf("round-trip pass_rate = %v", got.PassRate)
	}
	if got.Metadata.Author != "cosca-kernel" {
		t.Errorf("round-trip metadata = %+v", got.Metadata)
	}
	if c := got.Cases[0]; c.Reward != 2.0 || c.RewardExpectation != 0.8 || c.Weight != 2.0 {
		t.Errorf("round-trip case reward fields = %+v", c)
	}
	m := got.Metrics
	if m.Accuracy != 1.0 || m.Precision != 1.0 || m.Recall != 1.0 || m.F1 != 1.0 ||
		m.MatthewsCorr != 1.0 || m.BootstrapStd != 0.0 || !m.HigherIsBetter {
		t.Errorf("round-trip metrics = %+v", m)
	}
}
