package evals

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadSuiteFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "suite.yml")
	data := `suite: test-suite
defaults:
  timeouts:
    plan_seconds: 60
    execute_seconds: 120
    idle_seconds: 30
cases:
  - id: case-a
    prompt: "Crie um CRUD de clientes"
    workflow: feature-development
    steps:
      - wait_for_plan
      - approve_plan
      - wait_for_execution
    verify:
      - "go build ./..."
    question_strategy: fail
    question_answers:
      - "sim"
    timeouts:
      execute_seconds: 90
  - id: case-b
    prompt: "Corrija o bug"
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := LoadSuite(path)
	if err != nil {
		t.Fatalf("LoadSuite: %v", err)
	}

	if s.Suite != "test-suite" {
		t.Errorf("suite name = %q, want %q", s.Suite, "test-suite")
	}
	if got := s.Defaults.Timeouts.PlanSeconds; got != 60 {
		t.Errorf("plan_seconds = %d, want 60", got)
	}
	if len(s.Cases) != 2 {
		t.Fatalf("cases = %d, want 2", len(s.Cases))
	}

	a := s.Cases[0]
	if a.ID != "case-a" || a.Workflow != "feature-development" {
		t.Errorf("case-a = %+v", a)
	}
	if len(a.Steps) != 3 || a.Steps[0] != "wait_for_plan" {
		t.Errorf("case-a steps = %v", a.Steps)
	}
	if len(a.Verify) != 1 || a.Verify[0] != "go build ./..." {
		t.Errorf("case-a verify = %v", a.Verify)
	}
	if ParseQuestionStrategy(a.QuestionStrategy) != QuestionStrategyFail {
		t.Errorf("case-a strategy = %q, want fail", a.QuestionStrategy)
	}
	if len(a.QuestionAnswers) != 1 || a.QuestionAnswers[0] != "sim" {
		t.Errorf("case-a answers = %v", a.QuestionAnswers)
	}

	// Per-case timeouts override defaults.
	rt := s.ResolveTimeouts(a)
	if rt.ExecuteSeconds != 90 {
		t.Errorf("case-a execute = %d, want 90 (override)", rt.ExecuteSeconds)
	}
	if rt.PlanSeconds != 60 {
		t.Errorf("case-a plan = %d, want 60 (from defaults)", rt.PlanSeconds)
	}
	if rt.Total() != time.Duration(60+90+30)*time.Second {
		t.Errorf("case-a total = %v", rt.Total())
	}
}

func TestLoadSuiteValidation(t *testing.T) {
	cases := []string{
		"",                                // empty
		"suite: x\ncases: []",             // empty case list is valid
		"cases:\n  - prompt: only-prompt", // missing suite name
		"suite: x\ncases:\n  - id: a\n  - id: a\n  - id: b", // duplicate
		"suite: x\ncases:\n  - id: a\n",                     // missing prompt and workflow
	}
	wantErr := []bool{true, false, true, true, true}

	for i, yml := range cases {
		dir := t.TempDir()
		path := filepath.Join(dir, "s.yml")
		if yml != "" {
			if err := os.WriteFile(path, []byte(yml), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := LoadSuite(path)
			if (err != nil) != wantErr[i] {
				t.Errorf("case %d: err = %v, wantErr %v", i, err, wantErr[i])
			}
		} else {
			_, err := LoadSuite(path)
			if err == nil {
				t.Errorf("case %d: expected error for empty file", i)
			}
		}
	}
}

func TestListCases(t *testing.T) {
	s := &Suite{
		Suite: "s",
		Cases: []Case{{ID: "b"}, {ID: "a"}, {ID: "c"}},
	}
	got := s.ListCases()
	want := []string{"b", "a", "c"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ListCases[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if s.Case("a") == nil || s.Case("zz") != nil {
		t.Error("Case lookup failed")
	}
}

func TestLoadSuiteMetadataAndRewards(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "suite.yml")
	data := `suite: meta-suite
metadata:
  author: cosca-kernel
  description: "benchmark"
  difficulty: easy
  category: pipeline
  tags: [smoke, esteira]
  created: "2026-08-11"
cases:
  - id: a
    prompt: "hello"
    reward_expectation: 0.8
    weight: 2.0
    difficulty: hard
    category: bugfix
  - id: b
    prompt: "world"
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := LoadSuite(path)
	if err != nil {
		t.Fatalf("LoadSuite: %v", err)
	}

	m := s.Metadata
	if m.Author != "cosca-kernel" || m.Description != "benchmark" {
		t.Errorf("metadata author/desc = %q/%q", m.Author, m.Description)
	}
	if m.Difficulty != "easy" || m.Category != "pipeline" {
		t.Errorf("metadata difficulty/category = %q/%q", m.Difficulty, m.Category)
	}
	if len(m.Tags) != 2 || m.Tags[0] != "smoke" || m.Tags[1] != "esteira" {
		t.Errorf("metadata tags = %v", m.Tags)
	}
	if m.Created != "2026-08-11" {
		t.Errorf("metadata created = %q", m.Created)
	}

	a := s.Cases[0]
	if a.RewardExpectation != 0.8 || a.Weight != 2.0 {
		t.Errorf("case a reward/weight = %v/%v", a.RewardExpectation, a.Weight)
	}
	if a.EffectiveRewardExpectation() != 0.8 || a.EffectiveWeight() != 2.0 {
		t.Errorf("case a effective reward/weight = %v/%v", a.EffectiveRewardExpectation(), a.EffectiveWeight())
	}

	// Per-case metadata overrides the suite metadata.
	if s.CaseDifficulty(a) != "hard" || s.CaseCategory(a) != "bugfix" {
		t.Errorf("case a override difficulty/category = %q/%q", s.CaseDifficulty(a), s.CaseCategory(a))
	}
	b := s.Cases[1]
	if s.CaseDifficulty(b) != "easy" || s.CaseCategory(b) != "pipeline" {
		t.Errorf("case b fallback difficulty/category = %q/%q", s.CaseDifficulty(b), s.CaseCategory(b))
	}

	// Unset reward/weight default to 1.0.
	if b.EffectiveRewardExpectation() != 1.0 || b.EffectiveWeight() != 1.0 {
		t.Errorf("case b effective reward/weight = %v/%v", b.EffectiveRewardExpectation(), b.EffectiveWeight())
	}
}

func TestLoadSuiteRewardValidation(t *testing.T) {
	cases := []string{
		"suite: x\ncases:\n  - id: a\n    prompt: p\n    reward_expectation: 1.5",
		"suite: x\ncases:\n  - id: a\n    prompt: p\n    reward_expectation: -0.2",
		"suite: x\ncases:\n  - id: a\n    prompt: p\n    weight: -1",
	}
	for i, yml := range cases {
		dir := t.TempDir()
		path := filepath.Join(dir, "s.yml")
		if err := os.WriteFile(path, []byte(yml), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadSuite(path); err == nil {
			t.Errorf("case %d: expected validation error", i)
		}
	}
}
