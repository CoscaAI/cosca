// Package evals implements the Cosca benchmark harness (`cosca eval`).
// It loads YAML benchmark suites, drives every case through the REAL
// pipeline wiring (buildPipelineWiring), applies scripted steps and
// verification commands, and persists machine-readable reports.
package evals

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/CoscaAI/cosca/internal/embed"
)

// Timeouts controls the pacing of a single case. The plan/execute/idle
// phases map onto the pipeline lifecycle; the harness uses their sum as the
// default per-case wall-clock deadline.
type Timeouts struct {
	PlanSeconds    int `yaml:"plan_seconds"`
	ExecuteSeconds int `yaml:"execute_seconds"`
	IdleSeconds    int `yaml:"idle_seconds"`
}

// Total returns the default per-case deadline derived from the phases.
func (t Timeouts) Total() time.Duration {
	secs := t.PlanSeconds + t.ExecuteSeconds + t.IdleSeconds
	if secs <= 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}

// Defaults holds suite-wide defaults applied to every case.
type Defaults struct {
	Timeouts Timeouts `yaml:"timeouts"`
}

// SuiteMetadata is the optional top-level `metadata` block of a suite. It
// describes the benchmark authorship, difficulty, category and tags; per-case
// difficulty/category override the suite-level values.
type SuiteMetadata struct {
	Author      string   `yaml:"author" json:"author,omitempty"`
	Description string   `yaml:"description" json:"description,omitempty"`
	Difficulty  string   `yaml:"difficulty" json:"difficulty,omitempty"` // easy | medium | hard
	Category    string   `yaml:"category" json:"category,omitempty"`
	Tags        []string `yaml:"tags" json:"tags,omitempty"`
	Created     string   `yaml:"created" json:"created,omitempty"`
}

// Case is a single benchmark case.
type Case struct {
	ID               string    `yaml:"id"`
	Prompt           string    `yaml:"prompt"`
	Workflow         string    `yaml:"workflow"`
	Steps            []string  `yaml:"steps"`
	Verify           []string  `yaml:"verify"`
	QuestionStrategy string    `yaml:"question_strategy"`
	QuestionAnswers  []string  `yaml:"question_answers"`
	Timeouts         *Timeouts `yaml:"timeouts"`
	// RewardExpectation is the target reward the case SHOULD achieve, in
	// [0,1]. Optional, default 1.0 = full credit expected. It scales the
	// reward of partially-failed cases (some verifies passed).
	RewardExpectation float64 `yaml:"reward_expectation"`
	// Weight lets a suite weigh harder cases more. Optional, default 1.0.
	// A case that passes all verifies earns reward = weight; the report
	// aggregation (mean/max/min/sum) is computed over these per-case rewards.
	Weight float64 `yaml:"weight"`
	// Difficulty and Category are per-case metadata overrides for the suite
	// metadata block (both optional).
	Difficulty string `yaml:"difficulty"`
	Category   string `yaml:"category"`
	// ProblemClass classifies what the case measures (reproduction, discovery,
	// open, existence). Empty = reproduction.
	ProblemClass string `yaml:"problem_class"`
	// Oracle configures oracle-based evaluation. When nil, the case runs in
	// static mode (verify at the end only).
	Oracle *Oracle `yaml:"oracle"`
}

// EffectiveWeight returns the case weight, defaulting to 1.0.
func (c Case) EffectiveWeight() float64 {
	if c.Weight <= 0 {
		return 1.0
	}
	return c.Weight
}

// EffectiveRewardExpectation returns the case reward expectation clamped to
// [0,1], defaulting to 1.0 when unset.
func (c Case) EffectiveRewardExpectation() float64 {
	if c.RewardExpectation <= 0 || c.RewardExpectation > 1 {
		return 1.0
	}
	return c.RewardExpectation
}

// CaseDifficulty resolves a case's effective difficulty, falling back to the
// suite metadata when the case does not override it.
func (s *Suite) CaseDifficulty(c Case) string {
	if c.Difficulty != "" {
		return c.Difficulty
	}
	return s.Metadata.Difficulty
}

// CaseCategory resolves a case's effective category, falling back to the
// suite metadata when the case does not override it.
func (s *Suite) CaseCategory(c Case) string {
	if c.Category != "" {
		return c.Category
	}
	return s.Metadata.Category
}

// Suite is a parsed benchmark suite.
type Suite struct {
	Suite    string        `yaml:"suite"`
	Metadata SuiteMetadata `yaml:"metadata"`
	Defaults Defaults      `yaml:"defaults"`
	Cases    []Case        `yaml:"cases"`
}

// ResolveTimeouts returns the effective timeouts for a case, layering any
// per-case overrides on top of the suite defaults.
func (s *Suite) ResolveTimeouts(c Case) Timeouts {
	t := s.Defaults.Timeouts
	if c.Timeouts != nil {
		if c.Timeouts.PlanSeconds > 0 {
			t.PlanSeconds = c.Timeouts.PlanSeconds
		}
		if c.Timeouts.ExecuteSeconds > 0 {
			t.ExecuteSeconds = c.Timeouts.ExecuteSeconds
		}
		if c.Timeouts.IdleSeconds > 0 {
			t.IdleSeconds = c.Timeouts.IdleSeconds
		}
	}
	return t
}

// ListCases returns the case IDs in suite order.
func (s *Suite) ListCases() []string {
	ids := make([]string, 0, len(s.Cases))
	for _, c := range s.Cases {
		ids = append(ids, c.ID)
	}
	return ids
}

// Case returns the case with the given ID, or nil.
func (s *Suite) Case(id string) *Case {
	for i := range s.Cases {
		if s.Cases[i].ID == id {
			return &s.Cases[i]
		}
	}
	return nil
}

// LoadSuite loads a suite from an explicit file path (relative or absolute)
// or from the embedded evals/suites tree. Bare names ("smoke", "smoke.yml")
// and embed-relative paths ("evals/suites/smoke.yml") both resolve against
// the embedded assets.
func LoadSuite(path string) (*Suite, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("suite path is empty")
	}

	data, err := readSuiteData(path)
	if err != nil {
		return nil, err
	}

	var s Suite
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse suite %q: %w", path, err)
	}
	if err := s.Validate(); err != nil {
		return nil, fmt.Errorf("suite %q: %w", path, err)
	}
	return &s, nil
}

// Validate checks structural invariants shared by file and embedded suites.
func (s *Suite) Validate() error {
	if s.Suite == "" {
		return fmt.Errorf("missing suite name")
	}
	seen := make(map[string]struct{}, len(s.Cases))
	for _, c := range s.Cases {
		if c.ID == "" {
			return fmt.Errorf("case with empty id")
		}
		if _, dup := seen[c.ID]; dup {
			return fmt.Errorf("duplicate case id %q", c.ID)
		}
		seen[c.ID] = struct{}{}
		if c.Prompt == "" && c.Workflow == "" {
			return fmt.Errorf("case %q: prompt or workflow is required", c.ID)
		}
		if c.RewardExpectation < 0 || c.RewardExpectation > 1 {
			return fmt.Errorf("case %q: reward_expectation %v out of range [0,1]", c.ID, c.RewardExpectation)
		}
		if c.Weight < 0 {
			return fmt.Errorf("case %q: weight %v must be non-negative", c.ID, c.Weight)
		}
	}
	return nil
}

// readSuiteData resolves a suite reference to raw YAML bytes, preferring an
// on-disk file and falling back to the embedded evals/suites tree.
func readSuiteData(path string) ([]byte, error) {
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read suite file %q: %w", path, err)
		}
		return data, nil
	}

	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".yml")
	base = strings.TrimSuffix(base, ".yaml")

	candidates := []string{
		"evals/suites/" + base + ".yml",
		"evals/suites/" + base + ".yaml",
	}
	if strings.HasPrefix(path, "evals/suites/") {
		candidates = append(candidates, path)
	}
	for _, candidate := range candidates {
		if data, err := embed.ReadFile(candidate); err == nil {
			return data, nil
		}
	}

	return nil, fmt.Errorf("suite %q not found (looked on disk and in embedded evals/suites)", path)
}
