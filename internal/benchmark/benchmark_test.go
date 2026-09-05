package benchmark

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/pipeline"
)

// stubRunner implements pipeline.Runner with a canned response.
type stubRunner struct {
	response string
	err      error
}

func (r *stubRunner) Run(ctx context.Context, req pipeline.RunRequest) (*pipeline.RunResult, error) {
	if r.err != nil {
		return nil, r.err
	}
	return &pipeline.RunResult{Response: r.response}, nil
}

func (r *stubRunner) RunStream(ctx context.Context, req pipeline.RunRequest) (<-chan pipeline.RunEvent, error) {
	ch := make(chan pipeline.RunEvent, 1)
	ch <- pipeline.RunEvent{Type: pipeline.EventDone}
	close(ch)
	return ch, nil
}

func TestRegistry(t *testing.T) {
	reg := NewRegistry()
	if len(reg.List()) != 0 {
		t.Fatal("empty registry must list nothing")
	}

	b := NewCoscaSelfEval(NewLLMJudge(&stubRunner{response: "CORRECT"}))
	reg.Register(b)

	if _, ok := reg.Get("cosca-self"); !ok {
		t.Fatal("registered benchmark not found")
	}
	if _, ok := reg.Get("ghost"); ok {
		t.Fatal("ghost benchmark should not exist")
	}
	if got := reg.List(); len(got) != 1 || got[0] != "cosca-self" {
		t.Fatalf("list = %v", got)
	}
}

func TestCoscaSelfEvalMetadata(t *testing.T) {
	b := NewCoscaSelfEval(NewLLMJudge(&stubRunner{response: "CORRECT"}))
	if b.Name() != "cosca-self" {
		t.Fatalf("name = %q", b.Name())
	}
	if b.Description() == "" {
		t.Fatal("empty description")
	}
	qs := b.Questions()
	if len(qs) != 5 {
		t.Fatalf("questions = %d, want 5", len(qs))
	}
	for i, q := range qs {
		if q.ID == "" || q.Text == "" || q.Answer == "" {
			t.Fatalf("question %d incomplete: %+v", i, q)
		}
	}
}

func TestParseJudgeOutput(t *testing.T) {
	cases := []struct {
		out     string
		correct bool
		score   float64
	}{
		{"CORRECT", true, 1.0},
		{"The answer is CORRECT.", true, 1.0},
		{"correct with explanation", true, 1.0},
		{"INCORRECT", false, 0.0},
		{"The answer is INCORRECT because...", false, 0.0},
		{"CORRECT... wait INCORRECT", false, 0.0}, // INCORRECT wins
		{"no verdict given", false, 0.0},
	}
	for _, tc := range cases {
		correct, score, reason := parseJudgeOutput(tc.out)
		if correct != tc.correct || score != tc.score {
			t.Errorf("parseJudgeOutput(%q) = %v,%v want %v,%v", tc.out, correct, score, tc.correct, tc.score)
		}
		if reason == "" {
			t.Errorf("parseJudgeOutput(%q): empty reasoning", tc.out)
		}
	}
}

func TestLLMJudge(t *testing.T) {
	j := NewLLMJudge(&stubRunner{response: "CORRECT — the answer matches"})
	score, err := j.Judge(context.Background(), Question{ID: "q1", Text: "Q?", Answer: "A"}, AgentResponse{Answer: "A"})
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if !score.Correct || score.Score != 1.0 || score.QuestionID != "q1" {
		t.Fatalf("score: %+v", score)
	}
	if !strings.Contains(score.Reasoning, "CORRECT") {
		t.Fatalf("reasoning: %q", score.Reasoning)
	}

	// Runner error propagates.
	j2 := NewLLMJudge(&stubRunner{err: errBench})
	_, err = j2.Judge(context.Background(), Question{}, AgentResponse{})
	if err == nil {
		t.Fatal("runner error must propagate")
	}
}

func TestLLMJudgeSetPrompt(t *testing.T) {
	j := NewLLMJudge(&stubRunner{response: "CORRECT"})
	j.SetPrompt("Custom: {{.Question}} | {{.Answer}} | {{.Expected}}")
	if j.prompt != "Custom: {{.Question}} | {{.Answer}} | {{.Expected}}" {
		t.Fatalf("prompt = %q", j.prompt)
	}
}

func TestScorerSaveLoadList(t *testing.T) {
	dir := t.TempDir()
	s := NewScorer(dir)

	result := &BenchResult{
		Benchmark: "cosca-self", Agent: "cosca-backend", Model: "gpt-4o",
		Total: 5, Correct: 4, Accuracy: 0.8, AvgScore: 0.9,
		DurationMs: 1000, TokensUsed: 500,
		CompletedAt: time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC),
	}
	if err := s.Save(result); err != nil {
		t.Fatalf("Save: %v", err)
	}

	files, err := s.List()
	if err != nil || len(files) != 1 {
		t.Fatalf("List = %v, %v", files, err)
	}

	loaded, err := s.Load(files[0])
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Benchmark != "cosca-self" || loaded.Accuracy != 0.8 || loaded.Correct != 4 {
		t.Fatalf("roundtrip: %+v", loaded)
	}
}

func TestScorerFormatReport(t *testing.T) {
	s := NewScorer(t.TempDir())
	out := s.FormatReport(&BenchResult{
		Benchmark: "b", Agent: "a", Model: "m", Total: 10, Correct: 7,
		Accuracy: 0.7, AvgScore: 0.75, DurationMs: 200, TokensUsed: 100,
	})
	for _, want := range []string{"Benchmark: b", "Agent:     a", "Accuracy:  70.0%", "Tokens:    100"} {
		if !strings.Contains(out, want) {
			t.Fatalf("report missing %q:\n%s", want, out)
		}
	}
}

func TestScorerCompare(t *testing.T) {
	s := NewScorer(t.TempDir())
	a := &BenchResult{Agent: "a", Accuracy: 0.8}
	b := &BenchResult{Agent: "a", Accuracy: 0.5}
	out := s.Compare(a, b)
	if !strings.Contains(out, "↑") || !strings.Contains(out, "80.0%") || !strings.Contains(out, "50.0%") {
		t.Fatalf("compare up: %q", out)
	}
	out = s.Compare(b, a)
	if !strings.Contains(out, "↓") {
		t.Fatalf("compare down: %q", out)
	}
}

var errBench = &benchErr{"benchmark failure"}

type benchErr struct{ msg string }

func (e *benchErr) Error() string { return e.msg }

func TestScorerLoadMissing(t *testing.T) {
	s := NewScorer(t.TempDir())
	if _, err := s.Load(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("Load(missing) expected error")
	}
	_ = os.Getwd
}
