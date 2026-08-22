package benchmark

import (
	"context"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/pipeline"
)

// LLMJudge evaluates agent responses using an LLM as judge.
type LLMJudge struct {
	runner pipeline.Runner
	prompt string // judge prompt template
}

// NewLLMJudge creates an LLM-based judge.
func NewLLMJudge(runner pipeline.Runner) *LLMJudge {
	return &LLMJudge{
		runner: runner,
		prompt: defaultJudgePrompt,
	}
}

// SetPrompt overrides the judge prompt template.
// Use {{.Question}} and {{.Answer}} and {{.Expected}} as placeholders.
func (j *LLMJudge) SetPrompt(prompt string) {
	j.prompt = prompt
}

// Judge evaluates an agent response against the ground truth.
func (j *LLMJudge) Judge(ctx context.Context, q Question, resp AgentResponse) (JudgeScore, error) {
	prompt := strings.NewReplacer(
		"{{.Question}}", q.Text,
		"{{.Answer}}", resp.Answer,
		"{{.Expected}}", q.Answer,
	).Replace(j.prompt)

	req := pipeline.RunRequest{
		Prompt: prompt,
		Options: pipeline.RunOptions{
			MaxTurns: 1,
			Timeout:  30 * time.Second,
		},
	}

	result, err := j.runner.Run(ctx, req)
	if err != nil {
		return JudgeScore{
			QuestionID: q.ID,
			Error:      err.Error(),
		}, err
	}

	correct, score, reasoning := parseJudgeOutput(result.Response)

	return JudgeScore{
		QuestionID: q.ID,
		Score:      score,
		Correct:    correct,
		Reasoning:  reasoning,
	}, nil
}

// parseJudgeOutput extracts a boolean and score from the judge LLM response.
// Expected format: "CORRECT" or "INCORRECT" followed by optional score and reasoning.
func parseJudgeOutput(output string) (correct bool, score float64, reasoning string) {
	upper := strings.ToUpper(output)
	if strings.Contains(upper, "CORRECT") && !strings.Contains(upper, "INCORRECT") {
		correct = true
		score = 1.0
	} else {
		correct = false
		score = 0.0
	}
	reasoning = output
	return
}

const defaultJudgePrompt = `You are an expert judge evaluating an AI agent's answer.

Question: {{.Question}}

Expected Answer: {{.Expected}}

Agent's Answer: {{.Answer}}

Is the agent's answer CORRECT or INCORRECT?
Respond with exactly one word: CORRECT or INCORRECT, followed by a brief explanation.

The answer is considered correct if it contains the key information from the expected answer, even if the wording differs. Minor formatting differences should be ignored.`
