package evals

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// CaseOutcome is the result of executing one case through an ExecRunner. The
// ExecRunner (in the cli package, backed by buildPipelineWiring) is the REAL
// pipeline; the harness only orchestrates timeouts, scripted steps and
// verification around it.
type CaseOutcome struct {
	Status      string // "passed" | "failed"
	Summary     string
	Failure     string // underlying pipeline failure reason (failed cases)
	Steps       []string
	PlanTasks   int
	TasksDone   int
	TasksFailed int
	DoD         string
}

// ExecRunner executes a single case through the real pipeline wiring. The
// cli package provides the concrete implementation on top of
// buildPipelineWiring; tests substitute a fake.
type ExecRunner interface {
	Execute(ctx context.Context, c Case, qp *QuestionPolicy) (*CaseOutcome, error)
}

// RunOptions controls a suite run.
type RunOptions struct {
	// ProjectDir is the working directory for verify commands. When empty
	// the process working directory is used.
	ProjectDir string
	// Timeout overrides the per-case deadline. Zero means "derive from the
	// suite defaults".
	Timeout time.Duration
	// VerifyTimeout is the per-command verify timeout (default 30s).
	VerifyTimeout time.Duration
	// OnlyCaseID limits the run to a single case ID.
	OnlyCaseID string
	// StripCanary controls whether canary marker lines are stripped from a
	// case prompt before it reaches the pipeline. The CLI defaults it to
	// true; tests that need the raw prompt (canary intact) set it to false.
	StripCanary bool
	// Progress is an optional per-case progress callback.
	Progress func(format string, args ...any)
}

// RunSuite runs every (selected) case in the suite through runner and
// assembles a Report. It never blocks past a case's deadline: when the
// pipeline exceeds it, the case is recorded as timeout and execution moves
// on.
func RunSuite(ctx context.Context, suite *Suite, runner ExecRunner, opts RunOptions) (*Report, error) {
	if suite == nil {
		return nil, fmt.Errorf("suite is nil")
	}
	if runner == nil {
		return nil, fmt.Errorf("runner is nil")
	}

	verifyTimeout := opts.VerifyTimeout
	if verifyTimeout <= 0 {
		verifyTimeout = 30 * time.Second
	}
	if opts.ProjectDir == "" {
		opts.ProjectDir = "."
	}

	report := &Report{Suite: suite.Suite, Metadata: suite.Metadata, StartedAt: time.Now().UTC()}

	for i := range suite.Cases {
		c := suite.Cases[i]
		if opts.OnlyCaseID != "" && c.ID != opts.OnlyCaseID {
			continue
		}

		cr := runOneCase(ctx, suite, runner, c, opts, verifyTimeout)
		switch cr.Status {
		case StatusPassed:
			report.Passed++
		case StatusFailed:
			report.Failed++
		case StatusError:
			report.Errors++
		case StatusTimeout:
			report.TimedOut++
		}
		report.Total++
		report.Cases = append(report.Cases, cr)

		if opts.Progress != nil {
			opts.Progress("case %s: %s (%s)", c.ID, cr.Status, cr.Duration)
		}
	}

	report.FinishedAt = time.Now().UTC()
	aggregateRewards(report)
	computeMetrics(report)
	return report, nil
}

// aggregateRewards computes the reward aggregation (mean/max/min/sum over the
// per-case rewards, which already carry the case weight) and the pass rate.
// It is applied after every selected case has run.
func aggregateRewards(report *Report) {
	if report.Total > 0 {
		report.PassRate = float64(report.Passed) / float64(report.Total)
	}
	n := len(report.Cases)
	if n == 0 {
		return
	}
	sum := 0.0
	max := report.Cases[0].Reward
	min := report.Cases[0].Reward
	for _, cr := range report.Cases {
		sum += cr.Reward
		if cr.Reward > max {
			max = cr.Reward
		}
		if cr.Reward < min {
			min = cr.Reward
		}
	}
	report.RewardSum = sum
	report.RewardMean = sum / float64(n)
	report.RewardMax = max
	report.RewardMin = min
}

func runOneCase(ctx context.Context, suite *Suite, runner ExecRunner, c Case, opts RunOptions, verifyTimeout time.Duration) CaseResult {
	start := time.Now()
	cr := CaseResult{ID: c.ID, Steps: append([]string(nil), c.Steps...)}
	finalize := func() {
		cr.Duration = time.Since(start).Round(time.Millisecond).String()
		cr.RewardExpectation = c.EffectiveRewardExpectation()
		cr.Weight = c.EffectiveWeight()
		cr.Reward = computeReward(c, cr)
	}

	// The case's raw YAML prompt keeps any canary markers; only the copy
	// handed to the pipeline is stripped, so benchmark authors can embed
	// `# canary: <hash>` lines to detect training-data contamination.
	if opts.StripCanary {
		c.Prompt = StripCanary(c.Prompt)
	}

	qp := ResolveQuestionPolicy(c)
	timeout := suite.ResolveTimeouts(c).Total()
	if opts.Timeout > 0 {
		timeout = opts.Timeout
	}
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	caseCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan struct{})
	var outcome *CaseOutcome
	var execErr error
	go func() {
		outcome, execErr = runner.Execute(caseCtx, c, qp)
		close(done)
	}()

	select {
	case <-caseCtx.Done():
		if caseCtx.Err() == context.DeadlineExceeded {
			cr.Status = StatusTimeout
			cr.Error = fmt.Sprintf("case exceeded deadline (%s)", timeout)
		} else {
			cr.Status = StatusError
			cr.Error = caseCtx.Err().Error()
		}
		cancel()
		finalize()
		return cr
	case <-done:
	}

	if execErr != nil {
		cr.Status = StatusError
		cr.Error = execErr.Error()
		finalize()
		return cr
	}

	if outcome == nil {
		cr.Status = StatusError
		cr.Error = "runner returned no outcome"
		finalize()
		return cr
	}

	cr.Summary = outcome.Summary
	cr.DoD = outcome.DoD
	if len(outcome.Steps) > 0 {
		cr.Steps = outcome.Steps
	}

	if outcome.Status == "failed" {
		cr.Status = StatusFailed
		if outcome.Failure != "" {
			cr.Error = outcome.Failure
		}
		finalize()
		return cr
	}

	// Pipeline mechanism passed → run the scripted verification commands.
	if len(c.Verify) > 0 {
		results := RunVerifyCommands(ctx, opts.ProjectDir, c.Verify, verifyTimeout)
		cr.VerifyResults = results
		allOK := true
		for _, vr := range results {
			if !vr.OK {
				allOK = false
				break
			}
		}
		if !allOK {
			cr.Status = StatusFailed
		} else {
			cr.Status = StatusPassed
		}
	} else {
		cr.Status = StatusPassed
	}

	finalize()
	return cr
}

// computeReward derives a case's numeric reward from its outcome:
//   - passed (all verifies OK, or no verifies) → 1.0 × weight
//   - failed but partial (some verifies passed, some failed) →
//     (passed_verifies/total_verifies) × reward_expectation
//   - failed with no passed verifies, error or timeout → 0.0
func computeReward(c Case, cr CaseResult) float64 {
	switch cr.Status {
	case StatusPassed:
		return 1.0 * c.EffectiveWeight()
	case StatusFailed:
		if n := len(cr.VerifyResults); n > 0 {
			passed := 0
			for _, vr := range cr.VerifyResults {
				if vr.OK {
					passed++
				}
			}
			if passed > 0 {
				return (float64(passed) / float64(n)) * c.EffectiveRewardExpectation()
			}
		}
		return 0.0
	default: // error, timeout
		return 0.0
	}
}

// defaultSteps is the scripted step list applied when a case declares none.
var defaultSteps = []string{"wait_for_plan", "approve_plan", "wait_for_execution"}

// ResolveSteps returns a case's effective scripted steps.
func ResolveSteps(c Case) []string {
	if len(c.Steps) == 0 {
		return append([]string(nil), defaultSteps...)
	}
	return c.Steps
}

// SummaryLine renders a one-line report summary used by the CLI.
func (r *Report) SummaryLine() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("suite %q: %d cases", r.Suite, r.Total))
	if r.Passed > 0 {
		b.WriteString(fmt.Sprintf(", %d passed", r.Passed))
	}
	if r.Failed > 0 {
		b.WriteString(fmt.Sprintf(", %d failed", r.Failed))
	}
	if r.Errors > 0 {
		b.WriteString(fmt.Sprintf(", %d errors", r.Errors))
	}
	if r.TimedOut > 0 {
		b.WriteString(fmt.Sprintf(", %d timed out", r.TimedOut))
	}
	return b.String()
}
