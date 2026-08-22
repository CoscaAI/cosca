package benchmark

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/pipeline"
)

// PipelineRunner executes benchmarks using the Cosca pipeline engine.
type PipelineRunner struct {
	engineRunner pipeline.Runner
	judgeRunner  pipeline.Runner // separate runner for judge LLM calls
}

// NewPipelineRunner creates a benchmark runner backed by a pipeline Runner.
func NewPipelineRunner(engineRunner, judgeRunner pipeline.Runner) *PipelineRunner {
	return &PipelineRunner{
		engineRunner: engineRunner,
		judgeRunner:  judgeRunner,
	}
}

// Run executes the benchmark against the given agent.
func (r *PipelineRunner) Run(ctx context.Context, bench Benchmark, agent string, opts RunOptions) (*BenchResult, error) {
	questions := bench.Questions()
	if opts.Limit > 0 && opts.Limit < len(questions) {
		questions = questions[:opts.Limit]
	}

	if opts.Concurrency <= 0 {
		opts.Concurrency = 1
	}

	result := &BenchResult{
		Benchmark: bench.Name(),
		Agent:     agent,
		Model:     opts.Model,
		Total:     len(questions),
		StartedAt: time.Now(),
	}

	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		sem    = make(chan struct{}, opts.Concurrency)
		scores []JudgeScore
		errors []error
	)

	for i, q := range questions {
		wg.Add(1)
		go func(idx int, question Question) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			qCtx := ctx
			if opts.Timeout > 0 {
				var cancel context.CancelFunc
				qCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
				defer cancel()
			}

			// Run agent
			response, err := r.runAgent(qCtx, question, agent, opts.Model)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("q[%d]: %w", idx, err))
				mu.Unlock()
				return
			}

			// Judge response
			score, err := bench.Judge(qCtx, question, response)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("judge q[%d]: %w", idx, err))
				mu.Unlock()
				return
			}

			mu.Lock()
			scores = append(scores, score)
			result.TokensUsed += response.TokensUsed
			mu.Unlock()
		}(i, q)
	}

	wg.Wait()
	result.CompletedAt = time.Now()

	// Aggregate scores
	result.Scores = scores
	for _, s := range scores {
		if s.Correct {
			result.Correct++
		}
		result.AvgScore += s.Score
	}
	if len(scores) > 0 {
		result.Accuracy = float64(result.Correct) / float64(result.Total)
		result.AvgScore = result.AvgScore / float64(len(scores))
	}
	result.DurationMs = result.CompletedAt.Sub(result.StartedAt).Milliseconds()

	if len(errors) > 0 {
		return result, fmt.Errorf("benchmark completed with %d errors: %v", len(errors), errors)
	}

	return result, nil
}

// runAgent executes a single question through the pipeline engine.
func (r *PipelineRunner) runAgent(ctx context.Context, q Question, agent, model string) (AgentResponse, error) {
	start := time.Now()

	req := pipeline.RunRequest{
		Prompt: q.Text,
		Agent:  agent,
		Options: pipeline.RunOptions{
			MaxTurns:    10,
			Timeout:     120 * time.Second,
			EnableBuild: false,
			EnableTest:  false,
		},
	}

	result, err := r.engineRunner.Run(ctx, req)
	if err != nil {
		return AgentResponse{
			QuestionID: q.ID,
			Error:      err.Error(),
		}, err
	}

	return AgentResponse{
		QuestionID: q.ID,
		Answer:     result.Response,
		TokensUsed: result.TokenUsage.Input + result.TokenUsage.Output,
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}

// Ensure interface compliance.
var _ Runner = (*PipelineRunner)(nil)
