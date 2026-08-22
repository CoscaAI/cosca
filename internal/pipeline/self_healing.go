package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/diagnostics"
	"github.com/CoscaAI/cosca/internal/orchestration"
)

type SelfHealer struct {
	classifier   *diagnostics.Classifier
	recoveryLoop *RecoveryLoop
	runner       Runner
	knowledge    orchestration.KnowledgeSearcher
	maxAttempts  int
	healHistory  []HealAttempt
	mu           sync.RWMutex
}

type HealAttempt struct {
	AttemptID string
	Error     string
	Category  string
	Diagnosis string
	Action    string
	Success   bool
	Duration  time.Duration
	Timestamp time.Time
}

type HealReport struct {
	TotalAttempts int
	Successful    int
	Failed        int
	Attempts      []HealAttempt
	SuccessRate   float64
}

func NewSelfHealer(
	classifier *diagnostics.Classifier,
	recovery *RecoveryLoop,
	runner Runner,
	knowledge orchestration.KnowledgeSearcher,
) *SelfHealer {
	if classifier == nil {
		classifier = diagnostics.NewClassifier()
	}
	if recovery == nil {
		recovery = NewRecoveryLoop(classifier)
	}
	return &SelfHealer{
		classifier:   classifier,
		recoveryLoop: recovery,
		runner:       runner,
		knowledge:    knowledge,
		maxAttempts:  5,
		healHistory:  make([]HealAttempt, 0),
	}
}

func (s *SelfHealer) Heal(ctx context.Context, task *TaskNode, err error) (*HealAttempt, error) {
	attempt := HealAttempt{
		AttemptID: fmt.Sprintf("HA-%d", time.Now().UnixNano()),
		Error:     err.Error(),
		Timestamp: time.Now().UTC(),
	}
	start := time.Now()

	defer func() {
		attempt.Duration = time.Since(start)
		s.mu.Lock()
		s.healHistory = append(s.healHistory, attempt)
		s.mu.Unlock()
	}()

	classified := s.classifier.Classify(err.Error())
	attempt.Category = string(classified.Category)

	diagnosis := s.buildDiagnosis(classified)
	attempt.Diagnosis = diagnosis

	if s.knowledge != nil {
		results, searchErr := s.knowledge.Search(ctx, orchestration.KnowledgeSearchParams{
			Query:    fmt.Sprintf("%s: %s", classified.Category, err.Error()),
			Limit:    5,
			MinScore: 0.3,
		})
		if searchErr == nil && len(results.Results) > 0 {
			diagnosis += "\nKnowledge matches found:"
			for _, r := range results.Results {
				diagnosis += fmt.Sprintf("\n  - %s (score %.2f): %s", r.Title, r.Score, r.Snippet)
			}
			attempt.Diagnosis = diagnosis
		}
	}

	fixPrompt := s.buildFixPrompt(task, classified)
	attempt.Action = fixPrompt

	recoveryResult, recoverErr := s.recoveryLoop.Recover(ctx, task, err.Error(), s.runner)
	if recoverErr != nil {
		return &attempt, fmt.Errorf("self-healing recovery failed: %w", recoverErr)
	}

	attempt.Success = recoveryResult.RetrySuccessful

	if !attempt.Success {
		attempt.Diagnosis += fmt.Sprintf("\nRecovery failed after %d attempts", recoveryResult.AttemptsUsed)
	}

	return &attempt, nil
}

func (s *SelfHealer) HealWithRetry(ctx context.Context, task *TaskNode, err error) error {
	for i := 0; i < s.maxAttempts; i++ {
		attempt, healErr := s.Heal(ctx, task, err)
		if healErr != nil {
			if i == s.maxAttempts-1 {
				return fmt.Errorf("self-healing exhausted %d attempts: %w", s.maxAttempts, healErr)
			}
			time.Sleep(s.recoveryLoop.RetryDelay)
			continue
		}
		if attempt.Success {
			return nil
		}
		if i < s.maxAttempts-1 {
			err = fmt.Errorf("heal attempt %d failed: %s", i+1, attempt.Diagnosis)
			time.Sleep(s.recoveryLoop.RetryDelay)
		}
	}
	return fmt.Errorf("self-healing failed after %d attempts", s.maxAttempts)
}

func (s *SelfHealer) Report() *HealReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var successful, failed int
	attempts := make([]HealAttempt, len(s.healHistory))
	copy(attempts, s.healHistory)

	for _, a := range s.healHistory {
		if a.Success {
			successful++
		} else {
			failed++
		}
	}

	var rate float64
	if len(s.healHistory) > 0 {
		rate = float64(successful) / float64(len(s.healHistory))
	}

	return &HealReport{
		TotalAttempts: len(s.healHistory),
		Successful:    successful,
		Failed:        failed,
		Attempts:      attempts,
		SuccessRate:   rate,
	}
}

func (s *SelfHealer) CanAutoFix(err error) bool {
	if err == nil {
		return false
	}
	classified := s.classifier.Classify(err.Error())
	switch classified.Category {
	case diagnostics.CatCompilation:
		return true
	case diagnostics.CatDependency:
		return true
	case diagnostics.CatTest:
		return true
	case diagnostics.CatRuntime:
		return classified.Confidence > 0.8
	case diagnostics.CatConfiguration:
		return true
	default:
		return false
	}
}

func (s *SelfHealer) buildDiagnosis(classified diagnostics.ClassifiedError) string {
	diag := fmt.Sprintf("Category: %s | Language: %s | Confidence: %.2f",
		classified.Category, classified.Language, classified.Confidence)

	if classified.File != "" {
		diag += fmt.Sprintf("\nFile: %s", classified.File)
		if classified.Line > 0 {
			diag += fmt.Sprintf(":%d", classified.Line)
		}
	}
	if classified.Symbol != "" {
		diag += fmt.Sprintf("\nSymbol: %s", classified.Symbol)
	}
	if classified.Suggestion != "" {
		diag += fmt.Sprintf("\nSuggestion: %s", classified.Suggestion)
	}
	return diag
}

func (s *SelfHealer) buildFixPrompt(task *TaskNode, classified diagnostics.ClassifiedError) string {
	taskCtx := ""
	if task != nil {
		taskCtx = fmt.Sprintf("Task %s (%s): ", task.ID, task.Description)
	}

	switch classified.Category {
	case diagnostics.CatCompilation:
		return fmt.Sprintf("%sFix compilation error in %s:%d: %s. Ensure the code compiles successfully.",
			taskCtx, classified.File, classified.Line, classified.RawMessage)
	case diagnostics.CatDependency:
		return fmt.Sprintf("%sResolve dependency error: %s. Install missing packages or fix version mismatches.",
			taskCtx, classified.RawMessage)
	case diagnostics.CatTest:
		return fmt.Sprintf("%sFix failing test: %s. Review and correct the test logic or assertions.",
			taskCtx, classified.RawMessage)
	case diagnostics.CatRuntime:
		return fmt.Sprintf("%sFix runtime error in %s:%d: %s. Review and patch the code.",
			taskCtx, classified.File, classified.Line, classified.RawMessage)
	case diagnostics.CatPermission, diagnostics.CatConfiguration:
		return fmt.Sprintf("%sFix configuration/permission issue: %s. Suggestion: %s",
			taskCtx, classified.RawMessage, classified.Suggestion)
	default:
		return fmt.Sprintf("%sFix the following error: %s", taskCtx, classified.RawMessage)
	}
}
