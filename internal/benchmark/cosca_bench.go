package benchmark

import "context"

// CoscaSelfEval is a built-in benchmark that evaluates Cosca agents against
// tasks they should be able to perform. It serves as a smoke test and
// regression detector for the agent framework itself.
type CoscaSelfEval struct {
	judge *LLMJudge
}

// NewCoscaSelfEval creates the Cosca self-evaluation benchmark.
func NewCoscaSelfEval(judge *LLMJudge) *CoscaSelfEval {
	return &CoscaSelfEval{judge: judge}
}

func (b *CoscaSelfEval) Name() string        { return "cosca-self" }
func (b *CoscaSelfEval) Description() string { return "Cosca agent self-evaluation benchmark" }

func (b *CoscaSelfEval) Questions() []Question {
	return []Question{
		{
			ID:     "cosca-001",
			Text:   "What is the purpose of the Cosca pipeline? Describe its main components in one sentence.",
			Answer: "The Cosca pipeline decomposes prompts into task DAGs, executes them with build/test validation, auto-recovers from failures, and validates with Definition of Done.",
		},
		{
			ID:     "cosca-002",
			Text:   "How does the Single Owner Model work in Cosca? Answer in one sentence.",
			Answer: "The runtime daemon owns Knowledge, Memory, and Compute engines; the serve API Gateway delegates to the runtime via gRPC.",
		},
		{
			ID:     "cosca-003",
			Text:   "What are the 6 Definition of Done checks in Cosca?",
			Answer: "code_exists, builds, tests_pass, no_regressions, security_ok, documented",
		},
		{
			ID:     "cosca-004",
			Text:   "What command should a Cosca agent run before calling the LLM?",
			Answer: "cosca knowledge search",
		},
		{
			ID:     "cosca-005",
			Text:   "What are the 6 CMI dimensions tracked by the CMITracker?",
			Answer: "Learning, Judgment, Planning, SelfCritique, Transfer, Consistency",
		},
	}
}

func (b *CoscaSelfEval) Judge(ctx context.Context, q Question, resp AgentResponse) (JudgeScore, error) {
	return b.judge.Judge(ctx, q, resp)
}
