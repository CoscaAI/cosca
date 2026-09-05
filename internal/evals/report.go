package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// CaseStatus is the outcome of a single benchmark case.
type CaseStatus string

const (
	StatusPassed  CaseStatus = "passed"
	StatusFailed  CaseStatus = "failed"
	StatusError   CaseStatus = "error"
	StatusTimeout CaseStatus = "timeout"
)

// CaseResult is the recorded outcome of one case.
type CaseResult struct {
	ID            string         `json:"id"`
	Status        CaseStatus     `json:"status"`
	Duration      string         `json:"duration"`
	Steps         []string       `json:"steps,omitempty"`
	Summary       string         `json:"summary,omitempty"`
	DoD           string         `json:"dod,omitempty"`
	VerifyResults []VerifyResult `json:"verify_results,omitempty"`
	Error         string         `json:"error,omitempty"`
	// Reward is the numeric reward earned by the case: weight on full pass,
	// (passed_verifies/total_verifies)*reward_expectation on partial failure,
	// 0 on error/timeout.
	Reward float64 `json:"reward"`
	// RewardExpectation is the target reward the case should achieve.
	RewardExpectation float64 `json:"reward_expectation"`
	// Weight is the case weight used to scale the reward.
	Weight float64 `json:"weight"`
	// Discovery holds process-of-discovery metrics for oracle-based cases.
	// Nil for static cases that did not use the oracle.
	Discovery *DiscoveryMetrics `json:"discovery,omitempty"`
	// SolutionClass is "known" when the agent's solution matches the reference,
	// "novel" when it differs but passes all secret tests, "" when there is no
	// reference solution to compare against.
	SolutionClass string `json:"solution_class,omitempty"`
}

// EvalMetrics holds the statistical metrics computed for a suite run,
// adapted from OpenAI evals (get_accuracy, get_bootstrap_accuracy_std,
// get_confusion_matrix, compute_matthew_corr, compute_precision,
// compute_recall, compute_f_score). HigherIsBetter defaults to true.
type EvalMetrics struct {
	Accuracy       float64 `json:"accuracy"`
	Precision      float64 `json:"precision"`
	Recall         float64 `json:"recall"`
	F1             float64 `json:"f1"`
	MatthewsCorr   float64 `json:"matthews_corr"`
	BootstrapStd   float64 `json:"bootstrap_std"`
	HigherIsBetter bool    `json:"higher_is_better"`
}

// Report is the full result of a suite run.
type Report struct {
	Suite      string        `json:"suite"`
	Metadata   SuiteMetadata `json:"metadata,omitempty"`
	StartedAt  time.Time     `json:"started_at"`
	FinishedAt time.Time     `json:"finished_at"`
	Total      int           `json:"total"`
	Passed     int           `json:"passed"`
	Failed     int           `json:"failed"`
	Errors     int           `json:"errors"`
	TimedOut   int           `json:"timed_out"`
	// PassRate is passed/total, kept for backward compatibility with the
	// pass/fail summary. Computed across all cases in the run.
	PassRate float64 `json:"pass_rate"`
	// Reward aggregates the per-case rewards (already weighted by case
	// weight): mean/max/min/sum across every case in the run.
	RewardMean float64 `json:"reward_mean"`
	RewardMax  float64 `json:"reward_max"`
	RewardMin  float64 `json:"reward_min"`
	RewardSum  float64 `json:"reward_sum"`
	// Metrics holds the statistical metrics for the run. Old reports without
	// a metrics object unmarshal to zero values.
	Metrics EvalMetrics  `json:"metrics"`
	Cases   []CaseResult `json:"cases"`
}

// Save persists the report to <dir>/<suite>-<timestamp>.json with 0600
// permissions, creating the directory tree as needed. It returns the written
// path.
func (r *Report) Save(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create reports dir: %w", err)
	}
	name := fmt.Sprintf("%s-%s.json", sanitizeFilename(r.Suite), r.StartedAt.UTC().Format("20060102-150405"))
	path := filepath.Join(dir, name)
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal report: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write report: %w", err)
	}
	return path, nil
}

// LatestReport loads the most recent persisted report for a suite from dir.
func LatestReport(dir, suite string) (*Report, error) {
	pattern := sanitizeFilename(suite) + "-*.json"
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return nil, fmt.Errorf("list reports: %w", err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no reports found for suite %q in %s", suite, dir)
	}

	sort.Slice(matches, func(i, j int) bool {
		mi, _ := os.Stat(matches[i])
		mj, _ := os.Stat(matches[j])
		ti := mi.ModTime()
		tj := mj.ModTime()
		if !ti.IsZero() && !tj.IsZero() {
			return ti.After(tj)
		}
		return matches[i] > matches[j]
	})

	data, err := os.ReadFile(matches[0])
	if err != nil {
		return nil, fmt.Errorf("read report %q: %w", matches[0], err)
	}
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse report %q: %w", matches[0], err)
	}
	return &r, nil
}

// sanitizeFilename keeps only characters that are safe in a filename.
func sanitizeFilename(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			out = append(out, r)
		default:
			out = append(out, '-')
		}
	}
	if len(out) == 0 {
		return "suite"
	}
	return string(out)
}
