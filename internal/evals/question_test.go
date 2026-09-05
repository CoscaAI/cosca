package evals

import (
	"strings"
	"testing"
)

func TestParseQuestionStrategy(t *testing.T) {
	table := map[string]QuestionStrategy{
		"":                  QuestionStrategySkip,
		"skip":              QuestionStrategySkip,
		"SKIP":              QuestionStrategySkip,
		"answers_then_skip": QuestionStrategyAnswersThenSkip,
		"Answers_Then_Skip": QuestionStrategyAnswersThenSkip,
		"fail":              QuestionStrategyFail,
		"bogus":             QuestionStrategySkip,
	}
	for in, want := range table {
		if got := ParseQuestionStrategy(in); got != want {
			t.Errorf("ParseQuestionStrategy(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQuestionPolicySkip(t *testing.T) {
	p := ResolveQuestionPolicy(Case{QuestionStrategy: "skip"})
	if p.WouldFail() {
		t.Error("skip strategy should never fail")
	}
	answer, handled, err := p.Ask("any question")
	if err != nil {
		t.Errorf("skip: unexpected error %v", err)
	}
	if !handled {
		t.Error("skip: question should be auto-handled")
	}
	if answer != "" {
		t.Errorf("skip: answer = %q, want empty", answer)
	}
}

func TestQuestionPolicyAnswersThenSkip(t *testing.T) {
	c := Case{
		QuestionStrategy: "answers_then_skip",
		QuestionAnswers:  []string{"first", "second"},
	}
	p := ResolveQuestionPolicy(c)
	if p.Remaining() != 2 {
		t.Fatalf("remaining = %d, want 2", p.Remaining())
	}

	a, handled, err := p.Ask("q1")
	if err != nil || !handled || a != "first" {
		t.Errorf("q1: answer=%q handled=%v err=%v", a, handled, err)
	}
	a, handled, err = p.Ask("q2")
	if err != nil || !handled || a != "second" {
		t.Errorf("q2: answer=%q handled=%v err=%v", a, handled, err)
	}
	// Answers exhausted → remaining questions auto-skipped, no error.
	a, handled, err = p.Ask("q3")
	if err != nil {
		t.Errorf("q3: unexpected error %v", err)
	}
	if !handled {
		t.Error("q3: should be auto-handled after answers exhausted")
	}
	if a != "" {
		t.Errorf("q3: answer = %q, want empty", a)
	}
}

func TestQuestionPolicyFail(t *testing.T) {
	// Without scripted answers, an ask_user event must fail the case.
	p := ResolveQuestionPolicy(Case{QuestionStrategy: "fail"})
	if !p.WouldFail() {
		t.Error("fail strategy with no answers should report WouldFail")
	}
	_, handled, err := p.Ask("which schema?")
	if handled {
		t.Error("fail strategy with no answers must not auto-handle")
	}
	if err == nil || !strings.Contains(err.Error(), "no scripted answer") {
		t.Errorf("fail: err = %v, want 'no scripted answer'", err)
	}

	// With scripted answers, the fail strategy injects them until exhausted,
	// then errors.
	p2 := ResolveQuestionPolicy(Case{
		QuestionStrategy: "fail",
		QuestionAnswers:  []string{"schema-b"},
	})
	if p2.WouldFail() {
		t.Error("fail strategy with answers should not report WouldFail")
	}
	a, handled, err := p2.Ask("which schema?")
	if err != nil || !handled || a != "schema-b" {
		t.Errorf("q1: answer=%q handled=%v err=%v", a, handled, err)
	}
	_, _, err = p2.Ask("which schema?")
	if err == nil {
		t.Error("fail strategy must error once answers are exhausted")
	}
}

func TestResolveQuestionPolicy(t *testing.T) {
	c := Case{QuestionAnswers: []string{"a"}}
	p := ResolveQuestionPolicy(c)
	if p.Strategy != QuestionStrategySkip {
		t.Errorf("default strategy = %q, want skip", p.Strategy)
	}
	if len(p.Answers) != 1 || p.Answers[0] != "a" {
		t.Errorf("answers = %v", p.Answers)
	}
}
