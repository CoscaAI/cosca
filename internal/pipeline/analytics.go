package pipeline

import (
	"fmt"
	"time"
)

// RunAnalytics captures execution metrics for a single pipeline run.
// Designed to answer: "What % of the task was completed without human intervention?"
type RunAnalytics struct {
	PlanID     string `json:"plan_id"`
	Intent     string `json:"intent"`
	IntentType string `json:"intent_type"`

	// Task outcomes
	TasksTotal  int `json:"tasks_total"`
	TasksDone   int `json:"tasks_done"`
	TasksFailed int `json:"tasks_failed"`

	// Auto-recovery
	RecoveriesAttempted int `json:"recoveries_attempted"`
	RecoveriesSucceeded int `json:"recoveries_succeeded"`

	// Code & tests
	TestsRun    int `json:"tests_run"`
	TestsPassed int `json:"tests_passed"`
	TestsFailed int `json:"tests_failed"`

	// Timing
	DurationMs int64 `json:"duration_ms"`

	// Tokens & cost
	LLMCalls      int     `json:"llm_calls"`
	TokensUsed    int64   `json:"tokens_used"`
	EstimatedCost float64 `json:"estimated_cost"`

	// Interventions (incremented externally when Don approves/rejects)
	HumanApprovals   int `json:"human_approvals"`
	HumanRejections  int `json:"human_rejections"`
	HumanCorrections int `json:"human_corrections"`

	// Derived metrics
	AutonomyScore float64 `json:"autonomy_score"` // 0.0–1.0
}

// ComputeDerived calculates AutonomyScore and other derived metrics.
func (a *RunAnalytics) ComputeDerived() {
	// AutonomyScore: tasks completed without human intervention / total tasks.
	// Tasks that succeeded on first attempt count as fully autonomous.
	// Tasks that needed recovery (but no human) count as semi-autonomous (0.7).
	if a.TasksTotal == 0 {
		a.AutonomyScore = 0
		return
	}

	tasksFirstAttempt := a.TasksDone - a.RecoveriesSucceeded
	if tasksFirstAttempt < 0 {
		tasksFirstAttempt = 0
	}

	// Clean tasks: 1.0, recovered tasks: 0.7, failed tasks: 0.0
	score := float64(tasksFirstAttempt)*1.0 + float64(a.RecoveriesSucceeded)*0.7
	score /= float64(a.TasksTotal)

	// Human interventions reduce the score proportionally.
	humanPenalty := float64(a.HumanApprovals+a.HumanRejections+a.HumanCorrections) * 0.1
	score -= humanPenalty
	if score < 0 {
		score = 0
	}
	if score > 1.0 {
		score = 1.0
	}

	a.AutonomyScore = score
}

// Summary returns a human-readable summary.
func (a *RunAnalytics) Summary() string {
	return fmt.Sprintf(
		"Plan %s: %d/%d tasks done, %d auto-recovered, %.0f%% autonomous (%.2f)",
		a.PlanID, a.TasksDone, a.TasksTotal,
		a.RecoveriesSucceeded,
		a.AutonomyScore*100, a.AutonomyScore,
	)
}

// AggregateAnalytics combines analytics from multiple runs.
type AggregateAnalytics struct {
	TotalPlans  int `json:"total_plans"`
	TotalTasks  int `json:"total_tasks"`
	TasksDone   int `json:"tasks_done"`
	TasksFailed int `json:"tasks_failed"`

	RecoveriesAttempted int `json:"recoveries_attempted"`
	RecoveriesSucceeded int `json:"recoveries_succeeded"`

	TestsRun    int `json:"tests_run"`
	TestsPassed int `json:"tests_passed"`
	TestsFailed int `json:"tests_failed"`

	LLMCalls        int     `json:"llm_calls"`
	TokensUsed      int64   `json:"tokens_used"`
	EstimatedCost   float64 `json:"estimated_cost"`
	TotalDurationMs int64   `json:"total_duration_ms"`

	AvgAutonomyScore float64        `json:"avg_autonomy_score"`
	Runs             []RunAnalytics `json:"runs,omitempty"`
}

// Add incorporates a single run into the aggregate.
func (agg *AggregateAnalytics) Add(run *RunAnalytics) {
	agg.TotalPlans++
	agg.TotalTasks += run.TasksTotal
	agg.TasksDone += run.TasksDone
	agg.TasksFailed += run.TasksFailed
	agg.RecoveriesAttempted += run.RecoveriesAttempted
	agg.RecoveriesSucceeded += run.RecoveriesSucceeded
	agg.TestsRun += run.TestsRun
	agg.TestsPassed += run.TestsPassed
	agg.TestsFailed += run.TestsFailed
	agg.LLMCalls += run.LLMCalls
	agg.TokensUsed += run.TokensUsed
	agg.EstimatedCost += run.EstimatedCost
	agg.TotalDurationMs += run.DurationMs

	// Weighted average of autonomy scores
	if agg.TotalPlans > 1 {
		prevWeight := float64(agg.TotalPlans-1) / float64(agg.TotalPlans)
		newWeight := 1.0 / float64(agg.TotalPlans)
		agg.AvgAutonomyScore = agg.AvgAutonomyScore*prevWeight + run.AutonomyScore*newWeight
	} else {
		agg.AvgAutonomyScore = run.AutonomyScore
	}

	agg.Runs = append(agg.Runs, *run)
}

// AnalyticsStore persists RunAnalytics to a JSONL file per plan.
type AnalyticsStore struct {
	history *WorkflowHistory
}

// NewAnalyticsStore creates a store backed by the event history.
func NewAnalyticsStore(history *WorkflowHistory) *AnalyticsStore {
	return &AnalyticsStore{history: history}
}

// Save persists a RunAnalytics record as an event in the history.
func (s *AnalyticsStore) Save(analytics *RunAnalytics) error {
	analytics.ComputeDerived()

	event := StepEvent{
		Type:      "analytics_snapshot",
		PlanID:    analytics.PlanID,
		Input:     analytics.Intent,
		Output:    analytics.Summary(),
		Duration:  analytics.DurationMs,
		Timestamp: time.Now().UTC(),
	}
	return s.history.Append(analytics.PlanID, event)
}

// Load builds RunAnalytics from a plan's event history.
func (s *AnalyticsStore) Load(planID string) (*RunAnalytics, error) {
	events, err := s.history.Load(planID)
	if err != nil {
		return nil, err
	}

	a := &RunAnalytics{PlanID: planID}

	// Track first-attempt successes vs recovery successes
	taskFirstSuccess := make(map[string]bool)
	taskRecovered := make(map[string]bool)
	taskCompleted := make(map[string]bool)

	for _, ev := range events {
		switch ev.Type {
		case PlanCreated:
			a.Intent = ev.Input

		case StepEventStarted:
			a.LLMCalls++

		case StepEventCompleted:
			a.TasksDone++
			taskCompleted[ev.TaskID] = true
			if !taskRecovered[ev.TaskID] {
				taskFirstSuccess[ev.TaskID] = true
			}

		case StepEventFailed:
			a.TasksFailed++

		case StepEventRetrying:
			a.RecoveriesAttempted++
			taskRecovered[ev.TaskID] = true

		case PlanCompleted:
			// Last PlanCompleted event overwrites duration/total
			if a.TasksTotal == 0 {
				a.TasksTotal = a.TasksDone
			}

		case PlanFailed:
			if a.TasksTotal == 0 {
				a.TasksTotal = a.TasksDone + a.TasksFailed
			}
		}

		a.DurationMs += ev.Duration
	}

	// Recoveries that succeeded = tasks that were retried AND eventually
	// completed. (A completed-after-retry task is excluded from
	// taskFirstSuccess by design, so the count comes from the completion map.)
	for taskID := range taskRecovered {
		if taskCompleted[taskID] {
			a.RecoveriesSucceeded++
		}
	}

	if a.TasksTotal == 0 {
		a.TasksTotal = a.TasksDone + a.TasksFailed
	}

	a.ComputeDerived()
	return a, nil
}
