package evals

import (
	"fmt"
	"strings"
	"sync"
)

// QuestionStrategy controls how the harness reacts to pipeline ask_user
// events. The real pipeline currently has no user-interaction hook; the
// policy is still resolved for every case so scripted answers are available
// to any runner that surfaces a question, and the strategy is exercised and
// unit-tested here.
type QuestionStrategy string

const (
	// QuestionStrategySkip auto-dismisses every question (no answer
	// injected, the pipeline proceeds with its default choice).
	QuestionStrategySkip QuestionStrategy = "skip"
	// QuestionStrategyAnswersThenSkip injects the scripted answers in
	// order and auto-dismisses any remaining questions.
	QuestionStrategyAnswersThenSkip QuestionStrategy = "answers_then_skip"
	// QuestionStrategyFail fails the case when a question is asked and no
	// scripted answer is available.
	QuestionStrategyFail QuestionStrategy = "fail"
)

// ParseQuestionStrategy normalizes a raw YAML value into a strategy.
// Empty and unknown values default to skip.
func ParseQuestionStrategy(s string) QuestionStrategy {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "answers_then_skip":
		return QuestionStrategyAnswersThenSkip
	case "fail":
		return QuestionStrategyFail
	default:
		return QuestionStrategySkip
	}
}

// ValidQuestionStrategy reports whether s is one of the accepted strategy
// names.
func ValidQuestionStrategy(s string) bool {
	switch ParseQuestionStrategy(s) {
	case QuestionStrategySkip, QuestionStrategyAnswersThenSkip, QuestionStrategyFail:
		return true
	}
	return false
}

// QuestionPolicy resolves a case's question configuration into an injectable
// policy. It is safe for concurrent use.
type QuestionPolicy struct {
	Strategy QuestionStrategy
	Answers  []string
	mu       sync.Mutex
}

// ResolveQuestionPolicy builds a QuestionPolicy from a case.
func ResolveQuestionPolicy(c Case) *QuestionPolicy {
	return &QuestionPolicy{
		Strategy: ParseQuestionStrategy(c.QuestionStrategy),
		Answers:  append([]string(nil), c.QuestionAnswers...),
	}
}

// Ask answers a single pipeline ask_user event. It returns the scripted
// answer to inject, whether the question was handled at all (handled=false
// means the question is auto-dismissed), and an error when the strategy
// forbids proceeding without a scripted answer.
func (p *QuestionPolicy) Ask(question string) (answer string, handled bool, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch p.Strategy {
	case QuestionStrategyFail:
		if len(p.Answers) > 0 {
			a := p.Answers[0]
			p.Answers = p.Answers[1:]
			return a, true, nil
		}
		return "", false, fmt.Errorf("question %q asked but no scripted answer (strategy %q)", question, QuestionStrategyFail)

	case QuestionStrategyAnswersThenSkip:
		if len(p.Answers) > 0 {
			a := p.Answers[0]
			p.Answers = p.Answers[1:]
			return a, true, nil
		}
		return "", true, nil

	default: // skip
		return "", true, nil
	}
}

// WouldFail reports whether the policy has no answer to inject and would
// error out on the first ask_user event.
func (p *QuestionPolicy) WouldFail() bool {
	return p.Strategy == QuestionStrategyFail && len(p.Answers) == 0
}

// Remaining returns the number of scripted answers left to inject.
func (p *QuestionPolicy) Remaining() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.Answers)
}
