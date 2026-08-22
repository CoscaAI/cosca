// Package benchmark provides an evaluation framework for Cosca agents.
// It defines benchmarks (question sets with judges), runs agents against them,
// scores the results, and feeds the CMITracker with real performance data.
//
// Architecture adapted from ApodexAI/AgentHarness (Apache 2.0), reimplemented
// in Go for the Cosca ecosystem.
package benchmark

import (
	"context"
	"time"
)

// Question represents a single benchmark question.
type Question struct {
	ID       string            `json:"id"`
	Text     string            `json:"text"`
	Answer   string            `json:"answer"` // ground truth
	Metadata map[string]string `json:"metadata,omitempty"`
}

// AgentResponse is the output of an agent after processing a question.
type AgentResponse struct {
	QuestionID string `json:"question_id"`
	Answer     string `json:"answer"`
	Trace      string `json:"trace,omitempty"`
	TokensUsed int    `json:"tokens_used"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// JudgeScore is the result of judging a single agent response.
type JudgeScore struct {
	QuestionID string  `json:"question_id"`
	Score      float64 `json:"score"` // 0.0 - 1.0
	Correct    bool    `json:"correct"`
	Reasoning  string  `json:"reasoning,omitempty"`
	Error      string  `json:"error,omitempty"`
}

// BenchResult aggregates scores across all questions in a benchmark.
type BenchResult struct {
	Benchmark   string       `json:"benchmark"`
	Agent       string       `json:"agent"`
	Model       string       `json:"model"`
	Total       int          `json:"total"`
	Correct     int          `json:"correct"`
	Accuracy    float64      `json:"accuracy"` // correct / total
	AvgScore    float64      `json:"avg_score"`
	Scores      []JudgeScore `json:"scores"`
	DurationMs  int64        `json:"duration_ms"`
	TokensUsed  int          `json:"tokens_used"`
	StartedAt   time.Time    `json:"started_at"`
	CompletedAt time.Time    `json:"completed_at"`
	CMIUpdate   *CMIDelta    `json:"cmi_update,omitempty"`
}

// CMIDelta captures how benchmark results update the 6 CMI dimensions.
type CMIDelta struct {
	Learning     float64 `json:"learning"`
	Judgment     float64 `json:"judgment"`
	Planning     float64 `json:"planning"`
	SelfCritique float64 `json:"self_critique"`
	Transfer     float64 `json:"transfer"`
	Consistency  float64 `json:"consistency"`
}

// Benchmark is the interface that all benchmarks must implement.
type Benchmark interface {
	// Name returns the benchmark identifier (e.g. "browsecomp").
	Name() string

	// Description returns a human-readable description.
	Description() string

	// Questions returns the set of questions for this benchmark.
	Questions() []Question

	// Judge evaluates an agent's response against the ground truth.
	// Uses LLM-based judging by default; can be overridden for exact-match benchmarks.
	Judge(ctx context.Context, question Question, response AgentResponse) (JudgeScore, error)
}

// Runner executes an agent against a benchmark and collects results.
type Runner interface {
	// Run executes the benchmark against the given agent.
	Run(ctx context.Context, bench Benchmark, agent string, opts RunOptions) (*BenchResult, error)
}

// RunOptions configures a benchmark run.
type RunOptions struct {
	// Agent is the agent name to test (e.g. "cosca-backend").
	Agent string

	// Model is the LLM model to use for the agent.
	Model string

	// Limit caps the number of questions (0 = all).
	Limit int

	// Concurrency is the number of parallel workers (0 = sequential).
	Concurrency int

	// Runs is the number of times to run the benchmark (for statistical significance).
	Runs int

	// Timeout per question (0 = no limit).
	Timeout time.Duration

	// DryRun prints the plan without executing.
	DryRun bool

	// OutputDir is where results are written.
	OutputDir string
}

// Registry holds all available benchmarks.
type Registry struct {
	benchmarks map[string]Benchmark
}

// NewRegistry creates an empty benchmark registry.
func NewRegistry() *Registry {
	return &Registry{
		benchmarks: make(map[string]Benchmark),
	}
}

// Register adds a benchmark to the registry.
func (r *Registry) Register(b Benchmark) {
	r.benchmarks[b.Name()] = b
}

// Get returns a benchmark by name.
func (r *Registry) Get(name string) (Benchmark, bool) {
	b, ok := r.benchmarks[name]
	return b, ok
}

// List returns all registered benchmark names.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.benchmarks))
	for name := range r.benchmarks {
		names = append(names, name)
	}
	return names
}
