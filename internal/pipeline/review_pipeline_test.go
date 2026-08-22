package pipeline

import (
	"context"
	"strings"
	"testing"
)

// scriptedRunner returns a pre-configured response per call (or an error).
type scriptedRunner struct {
	responses []string
	errs      []error
	callCount int
}

func (r *scriptedRunner) next() (string, error) {
	i := r.callCount
	r.callCount++
	if i < len(r.errs) && r.errs[i] != nil {
		return "", r.errs[i]
	}
	if i < len(r.responses) {
		return r.responses[i], nil
	}
	return "Score: 7.0\nReview PASSES", nil
}

func (r *scriptedRunner) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	resp, err := r.next()
	if err != nil {
		return nil, err
	}
	return &RunResult{Response: resp}, nil
}

func (r *scriptedRunner) RunStream(ctx context.Context, req RunRequest) (<-chan RunEvent, error) {
	ch := make(chan RunEvent, 1)
	ch <- RunEvent{Type: EventDone}
	close(ch)
	return ch, nil
}

func TestReviewPipelineReviewers(t *testing.T) {
	rp := NewReviewPipeline(&scriptedRunner{}, nil)
	rp.AddReviewer("cosca-security", "security", 1)
	rp.AddReviewer("cosca-architecture", "architecture", 2)

	sorted := rp.sortedReviewers()
	if sorted[0].AgentName != "cosca-security" || sorted[1].AgentName != "cosca-architecture" {
		t.Fatalf("sorted reviewers: %+v", sorted)
	}

	// Default reviewers added.
	rp2 := NewReviewPipeline(&scriptedRunner{}, nil)
	rp2.AddDefaultReviewers()
	if len(rp2.reviewers) != 4 {
		t.Fatalf("default reviewers = %d, want 4", len(rp2.reviewers))
	}
}

func TestReviewPipelineReviewPasses(t *testing.T) {
	rp := NewReviewPipeline(&scriptedRunner{
		responses: []string{
			"Score: 8.0\nNo critical issues. Review PASSES",
			"Score: 9.0\nLooks good. LGTM",
			"Score: 7.0\nReview PASSES",
			"Score: 6.5\nReview PASSES",
		},
	}, nil)
	rp.AddDefaultReviewers()

	report, err := rp.Review(context.Background(), &TaskNode{ID: "t1", Description: "do x"}, nil)
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if !report.OverallPass {
		t.Fatalf("report should pass: %+v", report)
	}
	if report.Score < 5.0 {
		t.Fatalf("score = %v", report.Score)
	}
	if len(report.Results) != 4 {
		t.Fatalf("results = %d, want 4", len(report.Results))
	}
}

func TestReviewPipelineReviewCriticalFails(t *testing.T) {
	rp := NewReviewPipeline(&scriptedRunner{
		responses: []string{
			"Critical issue detected: SQL injection. Review FAILS",
			"Score: 9.0\nLGTM",
		},
	}, nil)
	rp.AddReviewer("cosca-security", "security", 1)
	rp.AddReviewer("cosca-architecture", "architecture", 2)

	report, err := rp.Review(context.Background(), &TaskNode{ID: "t1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.OverallPass {
		t.Fatal("critical issue must fail the review")
	}
	if report.CriticalIssues != 1 {
		t.Fatalf("critical = %d, want 1", report.CriticalIssues)
	}
	if len(report.Results[0].Issues) != 1 || report.Results[0].Issues[0].Severity != "critical" {
		t.Fatalf("issues: %+v", report.Results[0].Issues)
	}
}

func TestReviewPipelineReviewRunnerError(t *testing.T) {
	rp := NewReviewPipeline(&scriptedRunner{errs: []error{errSimulated}}, nil)
	rp.AddReviewer("cosca-security", "security", 1)

	report, err := rp.Review(context.Background(), &TaskNode{ID: "t1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.OverallPass {
		t.Fatal("reviewer error must fail")
	}
	if report.Results[0].Score != 0 || len(report.Results[0].Issues) != 1 {
		t.Fatalf("result: %+v", report.Results[0])
	}
}

func TestReviewPipelineAutoFix(t *testing.T) {
	// First review has non-critical issues → auto-fix runs → second review clean.
	rp := NewReviewPipeline(&scriptedRunner{
		responses: []string{
			"Score: 6.0\nMedium issue: rename variable. Suggestion: use clearer names",
			"Score: 9.0\nLGTM",
		},
	}, nil)
	rp.AddReviewer("cosca-testing", "testing", 1)

	report, err := rp.ReviewWithAutoFix(context.Background(), &TaskNode{ID: "t1", Agent: "cosca-backend"}, nil)
	if err != nil {
		t.Fatalf("ReviewWithAutoFix: %v", err)
	}
	if !report.OverallPass {
		t.Fatalf("final report should pass after auto-fix: %+v", report)
	}
}

func TestReviewPipelineAutoFixCriticalSkips(t *testing.T) {
	rp := NewReviewPipeline(&scriptedRunner{
		responses: []string{"Critical issue detected. Review FAILS"},
	}, nil)
	rp.AddReviewer("cosca-security", "security", 1)

	report, err := rp.ReviewWithAutoFix(context.Background(), &TaskNode{ID: "t1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Critical issues → no auto-fix attempted → report returned as-is.
	if report.CriticalIssues != 1 {
		t.Fatalf("critical = %d", report.CriticalIssues)
	}
}

func TestParseReviewResponse(t *testing.T) {
	issues, suggestions, score, passed := parseReviewResponse(
		"Score: 3.5\nCritical issue in auth. Review FAILS\nSuggestion: add rate limiting",
		"security")
	if passed {
		t.Fatal("critical issue + review fails must fail the review")
	}
	// "critical" + "issue" → critical.
	if len(issues) != 1 || issues[0].Severity != "critical" {
		t.Fatalf("issues: %+v", issues)
	}
	if score != 3.5 {
		t.Fatalf("score = %v", score)
	}
	if len(suggestions) != 1 {
		t.Fatalf("suggestions: %v", suggestions)
	}

	// Clean response → pass, score floored at 8.
	_, _, score, passed = parseReviewResponse("All good, no issues found. LGTM", "arch")
	if !passed || score < 8.0 {
		t.Fatalf("clean: passed=%v score=%v", passed, score)
	}

	// Explicit "review fails" → fail.
	_, _, _, passed = parseReviewResponse("The review fails due to missing tests", "testing")
	if passed {
		t.Fatal("'review fails' must fail")
	}
}

func TestBuildReviewPrompt(t *testing.T) {
	task := &TaskNode{ID: "t1", Description: "build api", Agent: "cosca-backend"}
	prompt := buildReviewPrompt(task, Reviewer{AgentName: "cosca-security", Role: "security"}, []FileChange{
		{Path: "a.go", Action: "modified", LinesAdded: 5, LinesRemoved: 1, Reason: "add auth"},
	})
	for _, want := range []string{"cosca-security", "security", "t1", "build api", "a.go", "add auth"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestReviewCompile(t *testing.T) {
	r := &ReviewReport{}
	r.compile()
	if r.OverallPass {
		t.Fatal("empty report must not pass")
	}

	r2 := &ReviewReport{Results: []ReviewResult{
		{Score: 8.0, Issues: []ReviewIssue{{Severity: "high"}}},
		{Score: 6.0},
	}}
	r2.compile()
	if !r2.OverallPass || r2.Score != 7.0 || r2.TotalIssues != 1 {
		t.Fatalf("compiled: %+v", r2)
	}

	r3 := &ReviewReport{Results: []ReviewResult{
		{Score: 9.0, Issues: []ReviewIssue{{Severity: "critical"}}},
	}}
	r3.compile()
	if r3.OverallPass || r3.CriticalIssues != 1 {
		t.Fatalf("critical compile: %+v", r3)
	}
}

func TestBuildAutoFixPrompt(t *testing.T) {
	rp := NewReviewPipeline(&scriptedRunner{}, nil)
	issues := []ReviewIssue{
		{Severity: "medium", Category: "naming", Description: "bad name", Fix: "rename it", File: "x.go", Line: 3},
	}
	prompt := rp.buildAutoFixPrompt(&TaskNode{ID: "t1", Description: "task"}, issues)
	if !strings.Contains(prompt, "Auto-fix") || !strings.Contains(prompt, "bad name") || !strings.Contains(prompt, "x.go:3") {
		t.Fatalf("prompt: %s", prompt)
	}
}
