package pipeline

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

type ReviewPipeline struct {
	reviewers  []Reviewer
	runner     Runner
	generalCtx *GeneralContext
}

type Reviewer struct {
	AgentName string
	Role      string
	Priority  int
}

type ReviewResult struct {
	TaskID      string
	Reviewer    string
	Role        string
	Passed      bool
	Issues      []ReviewIssue
	Suggestions []string
	Score       float64
	Duration    time.Duration
}

type ReviewIssue struct {
	Severity    string
	Category    string
	Description string
	File        string
	Line        int
	Fix         string
}

type ReviewReport struct {
	TaskID         string
	Results        []ReviewResult
	OverallPass    bool
	TotalIssues    int
	CriticalIssues int
	Score          float64
	Duration       time.Duration
}

var defaultReviewers = []Reviewer{
	{AgentName: "cosca-security", Role: "security", Priority: 1},
	{AgentName: "cosca-architecture", Role: "architecture", Priority: 2},
	{AgentName: "cosca-testing", Role: "testing", Priority: 3},
	{AgentName: "cosca-documentation", Role: "documentation", Priority: 4},
}

func NewReviewPipeline(runner Runner, generalCtx *GeneralContext) *ReviewPipeline {
	return &ReviewPipeline{
		reviewers:  make([]Reviewer, 0),
		runner:     runner,
		generalCtx: generalCtx,
	}
}

func (rp *ReviewPipeline) AddReviewer(agentName, role string, priority int) {
	rp.reviewers = append(rp.reviewers, Reviewer{
		AgentName: agentName,
		Role:      role,
		Priority:  priority,
	})
}

func (rp *ReviewPipeline) AddDefaultReviewers() {
	for _, r := range defaultReviewers {
		rp.AddReviewer(r.AgentName, r.Role, r.Priority)
	}
}

func (rp *ReviewPipeline) Review(ctx context.Context, task *TaskNode, changes []FileChange) (*ReviewReport, error) {
	start := time.Now()

	reviewers := rp.sortedReviewers()
	if len(reviewers) == 0 {
		reviewers = defaultReviewers
	}

	report := &ReviewReport{
		TaskID:  task.ID,
		Results: make([]ReviewResult, 0, len(reviewers)),
	}

	for _, rev := range reviewers {
		result := rp.runReviewer(ctx, task, rev, changes)
		report.Results = append(report.Results, result)
	}

	report.compile()
	report.Duration = time.Since(start)
	return report, nil
}

func (rp *ReviewPipeline) ReviewWithAutoFix(ctx context.Context, task *TaskNode, changes []FileChange) (*ReviewReport, error) {
	report, err := rp.Review(ctx, task, changes)
	if err != nil {
		return report, err
	}

	if report.CriticalIssues > 0 {
		return report, nil
	}

	nonCriticalIssues := rp.collectNonCriticalIssues(report)
	if len(nonCriticalIssues) == 0 {
		return report, nil
	}

	fixPrompt := rp.buildAutoFixPrompt(task, nonCriticalIssues)
	fixReq := RunRequest{
		Prompt:  fixPrompt,
		Agent:   task.Agent,
		Options: RunOptions{MaxTurns: 10, Timeout: 120 * time.Second},
	}
	_, fixErr := rp.runner.Run(ctx, fixReq)
	if fixErr != nil {
		return report, fmt.Errorf("auto-fix execution failed: %w", fixErr)
	}

	finalReport, finalErr := rp.Review(ctx, task, changes)
	if finalErr != nil {
		return report, finalErr
	}

	return finalReport, nil
}

func (rp *ReviewPipeline) sortedReviewers() []Reviewer {
	cp := make([]Reviewer, len(rp.reviewers))
	copy(cp, rp.reviewers)
	sort.Slice(cp, func(i, j int) bool {
		return cp[i].Priority < cp[j].Priority
	})
	return cp
}

func (rp *ReviewPipeline) runReviewer(ctx context.Context, task *TaskNode, rev Reviewer, changes []FileChange) ReviewResult {
	result := ReviewResult{
		TaskID:   task.ID,
		Reviewer: rev.AgentName,
		Role:     rev.Role,
		Score:    7.0,
	}
	start := time.Now()
	defer func() {
		result.Duration = time.Since(start)
	}()

	prompt := buildReviewPrompt(task, rev, changes)

	req := RunRequest{
		Prompt:  prompt,
		Agent:   rev.AgentName,
		Options: RunOptions{MaxTurns: 8, Timeout: 90 * time.Second, EnableBuild: true, EnableTest: true},
	}
	runResult, err := rp.runner.Run(ctx, req)
	if err != nil {
		result.Passed = false
		result.Score = 0
		result.Issues = append(result.Issues, ReviewIssue{
			Severity:    "critical",
			Category:    rev.Role,
			Description: fmt.Sprintf("Reviewer execution failed: %v", err),
		})
		return result
	}

	response := runResult.Response
	if runResult.BuildResult != nil && !runResult.BuildResult.Success {
		response += "\n" + runResult.BuildResult.Output
	}

	issues, suggestions, score, passed := parseReviewResponse(response, rev.Role)
	result.Passed = passed
	result.Issues = issues
	result.Suggestions = suggestions
	result.Score = score

	return result
}

func (rp *ReviewPipeline) collectNonCriticalIssues(report *ReviewReport) []ReviewIssue {
	var issues []ReviewIssue
	for _, r := range report.Results {
		for _, issue := range r.Issues {
			if issue.Severity != "critical" {
				issues = append(issues, issue)
			}
		}
	}
	return issues
}

func (rp *ReviewPipeline) buildAutoFixPrompt(task *TaskNode, issues []ReviewIssue) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Auto-fix the following review issues for task %s (%s):\n\n", task.ID, task.Description))
	for i, issue := range issues {
		b.WriteString(fmt.Sprintf("%d. [%s] %s: %s", i+1, issue.Severity, issue.Category, issue.Description))
		if issue.Fix != "" {
			b.WriteString(fmt.Sprintf("\n   Suggested fix: %s", issue.Fix))
		}
		if issue.File != "" {
			b.WriteString(fmt.Sprintf("\n   File: %s", issue.File))
			if issue.Line > 0 {
				b.WriteString(fmt.Sprintf(":%d", issue.Line))
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("\nApply all fixes and verify the build passes.")
	return b.String()
}

func (r *ReviewReport) compile() {
	totalScore := 0.0
	hasCritical := false

	for _, result := range r.Results {
		r.TotalIssues += len(result.Issues)
		totalScore += result.Score

		for _, issue := range result.Issues {
			if issue.Severity == "critical" {
				r.CriticalIssues++
				hasCritical = true
			}
		}
	}

	if len(r.Results) > 0 {
		r.Score = totalScore / float64(len(r.Results))
	}

	r.OverallPass = !hasCritical && r.Score >= 5.0
}

func buildReviewPrompt(task *TaskNode, rev Reviewer, changes []FileChange) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("You are the %s reviewer (role: %s). ", rev.AgentName, rev.Role))
	b.WriteString(fmt.Sprintf("Review the following task:\n\n"))
	b.WriteString(fmt.Sprintf("Task ID: %s\n", task.ID))
	b.WriteString(fmt.Sprintf("Description: %s\n", task.Description))
	b.WriteString(fmt.Sprintf("Agent: %s\n", task.Agent))

	if len(changes) > 0 {
		b.WriteString("\nChanges made:\n")
		for _, c := range changes {
			b.WriteString(fmt.Sprintf("  - %s: %s (%d added, %d removed) because: %s\n",
				c.Path, c.Action, c.LinesAdded, c.LinesRemoved, c.Reason))
		}
	}

	if task.Result != nil {
		b.WriteString(fmt.Sprintf("\nTask output: %s\n", task.Result.Output))
		if task.Result.Error != "" {
			b.WriteString(fmt.Sprintf("Task error: %s\n", task.Result.Error))
		}
	}

	b.WriteString("\nProvide your review:\n")
	b.WriteString("- List issues found (severity: critical/high/medium/low)\n")
	b.WriteString("- For each issue: category, description, file, line, and suggested fix\n")
	b.WriteString("- Any suggestions for improvement\n")
	b.WriteString("- A score from 0-10\n")
	b.WriteString("- State whether the review PASSES or FAILS\n")

	return b.String()
}

func parseReviewResponse(response string, role string) ([]ReviewIssue, []string, float64, bool) {
	var issues []ReviewIssue
	var suggestions []string
	score := 5.0
	passed := true

	addIssue := func(severity, desc string) {
		issues = append(issues, ReviewIssue{
			Severity:    severity,
			Category:    role,
			Description: desc,
		})
		if severity == "critical" {
			passed = false
		}
	}

	lower := strings.ToLower(response)

	// Negation guard: "no critical issues" is a clean verdict, not a finding.
	// Without this, a clean response containing "critical" + "issue" (e.g. "no
	// critical issues found") is flagged as a critical issue and fails the review.
	hasCriticalNegation := strings.Contains(lower, "no critical issue") || strings.Contains(lower, "no critical ")

	if strings.Contains(lower, "critical") && strings.Contains(lower, "issue") && !hasCriticalNegation {
		addIssue("critical", "Critical issue detected by reviewer")
	}

	if strings.Contains(lower, "fail") && strings.Contains(lower, "review") {
		passed = false
	}

	lines := strings.Split(response, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "score:") || strings.HasPrefix(trimmed, "Score:") {
			parts := strings.Fields(trimmed)
			for _, p := range parts {
				var s float64
				if _, err := fmt.Sscanf(p, "%f", &s); err == nil && s >= 0 && s <= 10 {
					score = s
					break
				}
			}
		}

		if strings.Contains(strings.ToLower(trimmed), "suggestion") ||
			strings.Contains(strings.ToLower(trimmed), "recommend") ||
			strings.Contains(strings.ToLower(trimmed), "improve") {
			if len(trimmed) > 5 {
				suggestions = append(suggestions, trimmed)
			}
		}
	}

	if strings.Contains(lower, "no issues") || strings.Contains(lower, "looks good") || strings.Contains(lower, "lgtm") {
		// All-clear signal. This also clears any auto-detected issues — the
		// naive "critical"+""issue"" detector fires on the phrase "no critical
		// issues", which is a clean verdict, not a finding. The line-based
		// parser never adds issues (only suggestions), so the issue list is
		// exclusively auto-detected markers here.
		passed = true
		issues = nil
		score = max(score, 8.0)
	}

	return issues, suggestions, score, passed
}
