package evals

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"
)

// fakeRunner is a controllable ExecRunner for harness tests.
type fakeRunner struct {
	outcomes map[string]*CaseOutcome
	errs     map[string]error
	block    map[string]bool
}

func (f *fakeRunner) Execute(ctx context.Context, c Case, qp *QuestionPolicy) (*CaseOutcome, error) {
	if f.block[c.ID] {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if err := f.errs[c.ID]; err != nil {
		return nil, err
	}
	return f.outcomes[c.ID], nil
}

func TestRunSuiteAllPassed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("casos usam Verify via sh -c ('true') — shell POSIX ausente no Windows nativo")
	}
	s := &Suite{Suite: "s", Cases: []Case{{ID: "a", Verify: []string{"true"}}, {ID: "b"}}}
	r := &fakeRunner{
		outcomes: map[string]*CaseOutcome{
			"a": {Status: "passed", Summary: "ok"},
			"b": {Status: "passed", Summary: "ok"},
		},
	}
	rep, err := RunSuite(context.Background(), s, r, RunOptions{})
	if err != nil {
		t.Fatalf("RunSuite: %v", err)
	}
	if rep.Total != 2 || rep.Passed != 2 || rep.Failed != 0 || rep.Errors != 0 {
		t.Errorf("counts: %+v", rep)
	}
	if len(rep.Cases) != 2 || rep.Cases[0].Status != StatusPassed {
		t.Errorf("cases: %+v", rep.Cases)
	}
}

func TestRunSuiteVerifyFailure(t *testing.T) {
	s := &Suite{Suite: "s", Cases: []Case{{ID: "a", Verify: []string{"false"}}}}
	r := &fakeRunner{outcomes: map[string]*CaseOutcome{"a": {Status: "passed"}}}
	rep, err := RunSuite(context.Background(), s, r, RunOptions{})
	if err != nil {
		t.Fatalf("RunSuite: %v", err)
	}
	if rep.Failed != 1 {
		t.Errorf("want 1 failed (verify), got %+v", rep)
	}
	if len(rep.Cases[0].VerifyResults) != 1 || rep.Cases[0].VerifyResults[0].OK {
		t.Errorf("verify results: %+v", rep.Cases[0].VerifyResults)
	}
}

func TestRunSuitePipelineFailed(t *testing.T) {
	s := &Suite{Suite: "s", Cases: []Case{{ID: "a", Verify: []string{"true"}}}}
	r := &fakeRunner{outcomes: map[string]*CaseOutcome{"a": {Status: "failed", Summary: "tasks failed", Failure: "executor failed"}}}
	rep, _ := RunSuite(context.Background(), s, r, RunOptions{})
	if rep.Failed != 1 {
		t.Errorf("want 1 failed (pipeline), got %+v", rep)
	}
	if rep.Cases[0].Status != StatusFailed || rep.Cases[0].Summary != "tasks failed" {
		t.Errorf("case: %+v", rep.Cases[0])
	}
	if rep.Cases[0].Error != "executor failed" {
		t.Errorf("case error should surface pipeline failure: %q", rep.Cases[0].Error)
	}
}

func TestRunSuiteError(t *testing.T) {
	s := &Suite{Suite: "s", Cases: []Case{{ID: "a"}}}
	r := &fakeRunner{errs: map[string]error{"a": fmt.Errorf("provider down")}}
	rep, _ := RunSuite(context.Background(), s, r, RunOptions{})
	if rep.Errors != 1 {
		t.Errorf("want 1 error, got %+v", rep)
	}
	if !strings.Contains(rep.Cases[0].Error, "provider down") {
		t.Errorf("error: %q", rep.Cases[0].Error)
	}
}

func TestRunSuiteTimeout(t *testing.T) {
	s := &Suite{
		Suite:    "s",
		Defaults: Defaults{Timeouts: Timeouts{PlanSeconds: 1, ExecuteSeconds: 1, IdleSeconds: 1}},
		Cases:    []Case{{ID: "a"}},
	}
	r := &fakeRunner{block: map[string]bool{"a": true}}
	start := time.Now()
	rep, err := RunSuite(context.Background(), s, r, RunOptions{})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("RunSuite: %v", err)
	}
	if rep.TimedOut != 1 {
		t.Errorf("want 1 timeout, got %+v", rep)
	}
	if rep.Cases[0].Status != StatusTimeout {
		t.Errorf("case status = %q, want timeout", rep.Cases[0].Status)
	}
	if elapsed > 5*time.Second {
		t.Errorf("timeout case took %v — must not hang", elapsed)
	}
}

func TestRunSuiteTimeoutOverride(t *testing.T) {
	s := &Suite{
		Suite:    "s",
		Defaults: Defaults{Timeouts: Timeouts{PlanSeconds: 60, ExecuteSeconds: 60, IdleSeconds: 60}},
		Cases:    []Case{{ID: "a"}},
	}
	r := &fakeRunner{block: map[string]bool{"a": true}}
	start := time.Now()
	rep, _ := RunSuite(context.Background(), s, r, RunOptions{Timeout: 200 * time.Millisecond})
	elapsed := time.Since(start)
	if rep.TimedOut != 1 {
		t.Errorf("want 1 timeout, got %+v", rep)
	}
	if elapsed > 5*time.Second {
		t.Errorf("override not honored: took %v", elapsed)
	}
}

func TestRunSuiteOnlyCase(t *testing.T) {
	s := &Suite{Suite: "s", Cases: []Case{{ID: "a"}, {ID: "b"}}}
	r := &fakeRunner{outcomes: map[string]*CaseOutcome{"a": {Status: "passed"}, "b": {Status: "passed"}}}
	rep, _ := RunSuite(context.Background(), s, r, RunOptions{OnlyCaseID: "b"})
	if rep.Total != 1 || rep.Cases[0].ID != "b" {
		t.Errorf("only-case filter: %+v", rep.Cases)
	}
}

func TestResolveSteps(t *testing.T) {
	c := Case{}
	got := ResolveSteps(c)
	if len(got) != 3 || got[0] != "wait_for_plan" || got[1] != "approve_plan" || got[2] != "wait_for_execution" {
		t.Errorf("default steps = %v", got)
	}
	c.Steps = []string{"wait_for_plan"}
	got = ResolveSteps(c)
	if len(got) != 1 || got[0] != "wait_for_plan" {
		t.Errorf("explicit steps = %v", got)
	}
}

func TestReportSummaryLine(t *testing.T) {
	r := &Report{Suite: "s", Total: 3, Passed: 1, Failed: 1, Errors: 1}
	line := r.SummaryLine()
	for _, want := range []string{`suite "s"`, "3 cases", "1 passed", "1 failed", "1 errors"} {
		if !strings.Contains(line, want) {
			t.Errorf("summary %q missing %q", line, want)
		}
	}
}

func TestRunSuiteRewardAggregation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("casos usam Verify via sh -c ('true'/'false') — shell POSIX ausente no Windows nativo")
	}
	s := &Suite{
		Suite: "s",
		Cases: []Case{
			{ID: "a", Verify: []string{"true"}},  // pass, weight 1 → reward 1.0
			{ID: "b", Verify: []string{"true"}},  // pass, weight 1 → reward 1.0
			{ID: "c", Verify: []string{"false"}}, // full fail → reward 0.0
			{ID: "d", Weight: 2.0},               // pass, weight 2 → reward 2.0
			{ID: "e"},                            // error → reward 0.0
		},
	}
	r := &fakeRunner{
		outcomes: map[string]*CaseOutcome{
			"a": {Status: "passed"},
			"b": {Status: "passed"},
			"c": {Status: "passed"},
			"d": {Status: "passed"},
		},
		errs: map[string]error{"e": fmt.Errorf("provider down")},
	}
	rep, err := RunSuite(context.Background(), s, r, RunOptions{})
	if err != nil {
		t.Fatalf("RunSuite: %v", err)
	}
	if rep.RewardSum != 4.0 {
		t.Errorf("reward_sum = %.2f, want 4.00", rep.RewardSum)
	}
	if rep.RewardMean != 0.8 {
		t.Errorf("reward_mean = %.2f, want 0.80", rep.RewardMean)
	}
	if rep.RewardMax != 2.0 || rep.RewardMin != 0.0 {
		t.Errorf("reward max/min = %.2f/%.2f, want 2.00/0.00", rep.RewardMax, rep.RewardMin)
	}
	if rep.PassRate != 0.6 {
		t.Errorf("pass_rate = %.2f, want 0.60", rep.PassRate)
	}
	if rep.Cases[3].Reward != 2.0 {
		t.Errorf("weighted case reward = %.2f, want 2.00", rep.Cases[3].Reward)
	}
	if rep.Cases[3].Weight != 2.0 {
		t.Errorf("weighted case weight = %.2f, want 2.00", rep.Cases[3].Weight)
	}
	if rep.Cases[4].Reward != 0.0 {
		t.Errorf("error case reward = %.2f, want 0.00", rep.Cases[4].Reward)
	}
}

func TestRunSuitePartialReward(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("caso usa Verify via sh -c ('true'/'false') — shell POSIX ausente no Windows nativo")
	}
	s := &Suite{
		Suite: "s",
		Cases: []Case{{
			ID:                "a",
			Verify:            []string{"true", "false", "true"},
			RewardExpectation: 0.5,
			Weight:            3.0,
		}},
	}
	r := &fakeRunner{outcomes: map[string]*CaseOutcome{"a": {Status: "passed"}}}
	rep, err := RunSuite(context.Background(), s, r, RunOptions{})
	if err != nil {
		t.Fatalf("RunSuite: %v", err)
	}
	// 2/3 verifies passed × expectation 0.5 = 0.333...
	if got := rep.Cases[0].Reward; got != 2.0/3.0*0.5 {
		t.Errorf("partial reward = %.3f, want %.3f", got, 2.0/3.0*0.5)
	}
	if rep.Cases[0].Status != StatusFailed {
		t.Errorf("status = %q, want failed (partial verify)", rep.Cases[0].Status)
	}
	if rep.Cases[0].RewardExpectation != 0.5 {
		t.Errorf("reward_expectation = %.2f, want 0.50", rep.Cases[0].RewardExpectation)
	}
}

func TestRunSuiteDefaultRewardFields(t *testing.T) {
	s := &Suite{Suite: "s", Cases: []Case{{ID: "a"}}}
	r := &fakeRunner{outcomes: map[string]*CaseOutcome{"a": {Status: "passed"}}}
	rep, _ := RunSuite(context.Background(), s, r, RunOptions{})
	if rep.Cases[0].Reward != 1.0 {
		t.Errorf("default reward = %.2f, want 1.00", rep.Cases[0].Reward)
	}
	if rep.Cases[0].RewardExpectation != 1.0 {
		t.Errorf("default reward_expectation = %.2f, want 1.00", rep.Cases[0].RewardExpectation)
	}
	if rep.Cases[0].Weight != 1.0 {
		t.Errorf("default weight = %.2f, want 1.00", rep.Cases[0].Weight)
	}
}

func TestRunSuiteStripsCanaryWhenEnabled(t *testing.T) {
	s := &Suite{Suite: "s", Cases: []Case{{
		ID:     "a",
		Prompt: "# canary: 1234\nBuild the thing.",
	}}}
	got := make(chan string, 1)
	r := &promptCaptureRunner{prompts: got}
	_, err := RunSuite(context.Background(), s, r, RunOptions{StripCanary: true})
	if err != nil {
		t.Fatalf("RunSuite: %v", err)
	}
	if p := <-got; p != "Build the thing." {
		t.Errorf("runner received prompt %q, want stripped %q", p, "Build the thing.")
	}
}

func TestRunSuiteKeepsCanaryWhenDisabled(t *testing.T) {
	s := &Suite{Suite: "s", Cases: []Case{{
		ID:     "a",
		Prompt: "# canary: 1234\nBuild the thing.",
	}}}
	got := make(chan string, 1)
	r := &promptCaptureRunner{prompts: got}
	_, err := RunSuite(context.Background(), s, r, RunOptions{StripCanary: false})
	if err != nil {
		t.Fatalf("RunSuite: %v", err)
	}
	if p := <-got; p != "# canary: 1234\nBuild the thing." {
		t.Errorf("runner received prompt %q, want raw %q", p, "# canary: 1234\nBuild the thing.")
	}
}

func TestRunSuiteMetadataPropagates(t *testing.T) {
	s := &Suite{
		Suite: "s",
		Metadata: SuiteMetadata{
			Author:      "cosca-kernel",
			Description: "desc",
			Difficulty:  "easy",
			Category:    "pipeline",
			Tags:        []string{"smoke"},
			Created:     "2026-08-11",
		},
		Cases: []Case{
			{ID: "a"},
			{ID: "b", Difficulty: "hard", Category: "bugfix"},
		},
	}
	r := &fakeRunner{outcomes: map[string]*CaseOutcome{"a": {Status: "passed"}, "b": {Status: "passed"}}}
	rep, _ := RunSuite(context.Background(), s, r, RunOptions{})
	if rep.Metadata.Author != "cosca-kernel" || rep.Metadata.Difficulty != "easy" {
		t.Errorf("report metadata = %+v", rep.Metadata)
	}
	if s.CaseDifficulty(s.Cases[0]) != "easy" || s.CaseCategory(s.Cases[0]) != "pipeline" {
		t.Errorf("case a fallback difficulty/category = %q/%q", s.CaseDifficulty(s.Cases[0]), s.CaseCategory(s.Cases[0]))
	}
	if s.CaseDifficulty(s.Cases[1]) != "hard" || s.CaseCategory(s.Cases[1]) != "bugfix" {
		t.Errorf("case b override difficulty/category = %q/%q", s.CaseDifficulty(s.Cases[1]), s.CaseCategory(s.Cases[1]))
	}
}

// promptCaptureRunner records the prompt it receives instead of executing.
type promptCaptureRunner struct {
	prompts chan string
}

func (p *promptCaptureRunner) Execute(_ context.Context, c Case, _ *QuestionPolicy) (*CaseOutcome, error) {
	p.prompts <- c.Prompt
	return &CaseOutcome{Status: "passed"}, nil
}
