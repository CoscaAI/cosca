package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/diagnostics"
)

type RecoveryLoop struct {
	Classifier *diagnostics.Classifier
	MaxRetries int           // default 3
	RetryDelay time.Duration // default 2s
	AutoFix    bool
	workDir    string
}

type RecoveryResult struct {
	OriginalError   string
	ErrorCategory   diagnostics.ErrorCategory
	KnowledgeGap    bool
	FixAttempted    bool
	FixDescription  string
	RetrySuccessful bool
	AttemptsUsed    int
}

func NewRecoveryLoop(classifier *diagnostics.Classifier) *RecoveryLoop {
	return &RecoveryLoop{
		Classifier: classifier,
		MaxRetries: 3,
		RetryDelay: 2 * time.Second,
	}
}

// SetWorkDir sets the working directory used to resolve relative file paths.
func (r *RecoveryLoop) SetWorkDir(dir string) {
	r.workDir = dir
}

// retryBackoff returns an exponential backoff delay for a 1-based attempt:
// RetryDelay * 2^(attempt-1), capped at 30s. Replaces the previous fixed sleep
// so a worker waiting on a transient dependency backs off instead of hammering
// the same retry delay.
func (r *RecoveryLoop) retryBackoff(attempt int) time.Duration {
	d := r.RetryDelay
	for i := 1; i < attempt; i++ {
		d *= 2
		if d >= 30*time.Second {
			return 30 * time.Second
		}
	}
	return d
}

// Recover attempts to fix a build/test failure and retry the task.
func (r *RecoveryLoop) Recover(ctx context.Context, task *TaskNode, failure string, runner Runner) (*RecoveryResult, error) {
	result := &RecoveryResult{OriginalError: failure}

	for attempt := 1; attempt <= r.MaxRetries; attempt++ {
		result.AttemptsUsed = attempt

		// 1. Classify the error (nil-safe: use generic if no classifier)
		var classified diagnostics.ClassifiedError
		if r.Classifier != nil {
			classified = r.Classifier.Classify(failure)
		} else {
			classified = diagnostics.ClassifiedError{
				Category:   diagnostics.CatUnknown,
				RawMessage: failure,
			}
		}
		result.ErrorCategory = classified.Category

		// 2. Build fix prompt based on category
		var fixPrompt string
		switch classified.Category {
		case diagnostics.CatCompilation:
			fixPrompt = fmt.Sprintf("Fix compilation error in %s:%d: %s", classified.File, classified.Line, classified.RawMessage)
		case diagnostics.CatDependency:
			fixPrompt = fmt.Sprintf("Install missing dependency: %s. Error: %s", classified.Symbol, classified.RawMessage)
			result.KnowledgeGap = true
		case diagnostics.CatTest:
			fixPrompt = fmt.Sprintf("Fix failing test: %s. Error: %s", classified.Symbol, classified.RawMessage)
		case diagnostics.CatRuntime:
			fixPrompt = fmt.Sprintf("Fix runtime error at %s:%d: %s", classified.File, classified.Line, classified.RawMessage)
		case diagnostics.CatPermission, diagnostics.CatConfiguration:
			fixPrompt = fmt.Sprintf("Fix configuration/permission issue: %s. Suggestion: %s", classified.RawMessage, classified.Suggestion)
		default:
			fixPrompt = fmt.Sprintf("Fix the following error: %s", failure)
		}

		result.FixDescription = fixPrompt
		result.FixAttempted = true

		if classified.Category == diagnostics.CatCompilation && classified.File != "" && classified.Line > 0 {
			if autoFixed := r.tryDeterministicFix(classified); autoFixed {
				result.RetrySuccessful = true
				return result, nil
			}
		}

		// 3. Run fix task via LLM
		fixReq := RunRequest{
			Prompt:  fixPrompt,
			Agent:   task.Agent,
			Options: RunOptions{MaxTurns: 10, Timeout: 120 * time.Second},
		}
		fixResult, err := runner.Run(ctx, fixReq)
		if err != nil {
			failure = err.Error()
			time.Sleep(r.retryBackoff(attempt))
			continue
		}
		if !fixRunSucceeded(fixResult) {
			failure = fixFailureText(fixResult)
			time.Sleep(r.retryBackoff(attempt))
			continue
		}
		result.RetrySuccessful = true
		return result, nil
	}

	return result, nil
}

func fixRunSucceeded(res *RunResult) bool {
	if res.BuildResult != nil && !res.BuildResult.Success {
		return false
	}
	if res.TestResult != nil && !res.TestResult.Success {
		return false
	}
	return true
}

func (r *RecoveryLoop) tryDeterministicFix(classified diagnostics.ClassifiedError) bool {
	// Consent gate: never mutate source files unless AutoFix was explicitly
	// enabled (Config.Pipeline.AutoFix, default false).
	if !r.AutoFix {
		return false
	}

	exeDir := r.workDir
	if exeDir == "" {
		var err error
		exeDir, err = os.Getwd()
		if err != nil {
			return false
		}
	}
	exeDir = filepath.Clean(exeDir)

	filePath := classified.File
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(exeDir, filePath)
	}
	filePath = filepath.Clean(filePath)

	// Never touch files outside the working directory.
	rel, err := filepath.Rel(exeDir, filePath)
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	content := string(data)
	original := content

	switch {
	case classified.Symbol != "" && strings.Contains(content, classified.Symbol):
		content = r.fixUndefinedSymbol(content, classified)

	case strings.Contains(content, "undefined:"):
		re := regexp.MustCompile(`undefined:\s*([\w.]+)`)
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 {
			classified.Symbol = matches[1]
			content = r.fixUndefinedSymbol(content, classified)
		}
	}

	if content == original {
		return false
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return false
	}

	return true
}

func (r *RecoveryLoop) fixUndefinedSymbol(content string, classified diagnostics.ClassifiedError) string {
	lines := strings.Split(content, "\n")
	targetLine := classified.Line - 1
	if targetLine < 0 || targetLine >= len(lines) {
		return content
	}

	line := lines[targetLine]

	if strings.Contains(line, ", "+classified.Symbol) {
		line = strings.ReplaceAll(line, ", "+classified.Symbol, "")
	} else if strings.Contains(line, classified.Symbol+",") {
		line = strings.ReplaceAll(line, classified.Symbol+",", "")
	} else if strings.Contains(line, classified.Symbol+")") {
		line = strings.ReplaceAll(line, classified.Symbol+")", ")")
	} else {
		line = strings.ReplaceAll(line, classified.Symbol, "")
	}

	lines[targetLine] = line
	return strings.Join(lines, "\n")
}

func fixFailureText(res *RunResult) string {
	if res.BuildResult != nil && !res.BuildResult.Success {
		return res.BuildResult.Output
	}
	if res.TestResult != nil && !res.TestResult.Success {
		return res.TestResult.Output
	}
	return res.Response
}

// RecoverFromHistory attempts to resume a plan by replaying its event history.
// Returns the reconstructed plan and whether recovery is needed.
func RecoverFromHistory(planID string, history *WorkflowHistory, checkpointStore *CheckpointStore) (*Plan, bool, error) {
	events, err := history.Load(planID)
	if err != nil {
		return nil, false, fmt.Errorf("recover: load history: %w", err)
	}

	if len(events) == 0 {
		return nil, false, nil
	}

	var checkpoint *Checkpoint
	if checkpointStore != nil {
		cp, cpErr := checkpointStore.Load(planID)
		if cpErr == nil {
			checkpoint = cp
		}
	}

	plan := &Plan{ID: planID}
	replayed := ReplayPlan(plan, events)

	var lastSeq int64
	if len(events) > 0 {
		lastSeq = events[len(events)-1].Sequence
	}

	needsRecovery := false
	for _, task := range replayed.Tasks {
		if task.Status == TaskPending || task.Status == TaskRunning {
			needsRecovery = true
			break
		}
	}

	if checkpoint != nil && checkpoint.LastSequence >= lastSeq {
		needsRecovery = false
		for _, task := range replayed.Tasks {
			if cpStatus, ok := checkpoint.TaskStatuses[task.ID]; ok {
				task.Status = TaskStatus(cpStatus)
			}
		}
	}

	if checkpointStore != nil && needsRecovery {
		cp := Checkpoint{
			PlanID:       planID,
			LastSequence: lastSeq,
			TaskStatuses: make(map[string]string),
		}
		for _, task := range replayed.Tasks {
			cp.TaskStatuses[task.ID] = string(task.Status)
		}
		_ = checkpointStore.Save(cp)
	}

	return replayed, needsRecovery, nil
}
