package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// ─── Chain Types ─────────────────────────────────────────────────────────────

// ChainStep defines a single step in a multi-agent chain.
type ChainStep struct {
	// Name is a human-readable name for this step.
	Name string `json:"name"`

	// Agent is the agent to invoke (by name).
	Agent string `json:"agent"`

	// Prompt is the instruction for this agent.
	Prompt string `json:"prompt"`

	// DependsOn lists step names that must complete before this step.
	DependsOn []string `json:"depends_on,omitempty"`

	// Timeout is the maximum duration for this step.
	Timeout time.Duration `json:"timeout,omitempty"`
}

// ChainDefinition defines a complete multi-agent execution chain.
type ChainDefinition struct {
	// Name identifies this chain.
	Name string `json:"name"`

	// Steps defines the agents to invoke and their dependencies.
	Steps []ChainStep `json:"steps"`

	// Parallel enables parallel execution of independent steps.
	Parallel bool `json:"parallel"`
}

// ChainResult holds the result of a multi-agent chain execution.
type ChainResult struct {
	RequestID  string            `json:"request_id"`
	Steps      []ChainStepResult `json:"steps"`
	FinalAgent string            `json:"final_agent"`
	Response   string            `json:"response"`
	Duration   time.Duration     `json:"duration"`
}

// ChainStepResult holds the result of a single chain step.
type ChainStepResult struct {
	Name      string        `json:"name"`
	Agent     string        `json:"agent"`
	Success   bool          `json:"success"`
	Output    string        `json:"output"`
	Error     string        `json:"error,omitempty"`
	ErrorCode string        `json:"error_code,omitempty"`
	Duration  time.Duration `json:"duration"`
}

// ─── Chain Configuration ─────────────────────────────────────────────────────

// ChainConfig configures multi-agent chain execution behaviour.
type ChainConfig struct {
	// Enabled toggles multi-agent chain execution.
	Enabled bool

	// MaxSteps limits the number of steps in dynamic decomposition.
	// Default: 5.
	MaxSteps int

	// MaxParallel limits concurrent step execution.
	// Default: 3.
	MaxParallel int

	// SynthesisAgent is the agent used to merge specialist outputs.
	// Default: "COSCA KERNEL".
	SynthesisAgent string
}

// DefaultChainConfig returns sensible defaults for chain execution.
func DefaultChainConfig() ChainConfig {
	return ChainConfig{
		Enabled:        false,
		MaxSteps:       5,
		MaxParallel:    runtime.NumCPU(),
		SynthesisAgent: "COSCA KERNEL",
	}
}

// ─── Chain Executor ──────────────────────────────────────────────────────────

// ChainExecutor executes multi-agent chains using the orchestration engine.
type ChainExecutor struct {
	engine Orchestrator
	agents AgentResolver
	skills SkillResolver
	config ChainConfig
}

// NewChainExecutor creates a chain executor backed by the given dependencies.
// All dependencies are optional — the executor degrades gracefully when any
// dependency is nil (e.g. skill resolver is only needed for decomposition).
func NewChainExecutor(engine Orchestrator, agents AgentResolver, skills SkillResolver, config ChainConfig) *ChainExecutor {
	if config.MaxSteps <= 0 {
		config.MaxSteps = 5
	}
	if config.MaxParallel <= 0 {
		config.MaxParallel = 3
	}
	if config.SynthesisAgent == "" {
		config.SynthesisAgent = "CEO Agent"
	}

	return &ChainExecutor{
		engine: engine,
		agents: agents,
		skills: skills,
		config: config,
	}
}

// ─── Execute ─────────────────────────────────────────────────────────────────

// Execute runs a pre-defined chain of agents. Steps are ordered by dependency
// (topological sort) and executed level by level. When ChainDefinition.Parallel
// is true, steps at the same dependency level run concurrently.
func (ce *ChainExecutor) Execute(ctx context.Context, def ChainDefinition) (*ChainResult, error) {
	startTime := time.Now()
	requestID := uuid.New().String()

	logger := log.Ctx(ctx).With().
		Str("chain", def.Name).
		Str("request_id", requestID).
		Logger()

	logger.Info().
		Int("steps", len(def.Steps)).
		Bool("parallel", def.Parallel).
		Msg("chain execution starting")

	if len(def.Steps) == 0 {
		return &ChainResult{
			RequestID: requestID,
			Response:  "No steps defined in chain",
			Duration:  time.Since(startTime),
		}, nil
	}

	if ce.engine == nil {
		return nil, fmt.Errorf("chain: no orchestrator configured, cannot execute steps")
	}

	// 1. Topological sort.
	levels, err := resolveDependencies(def.Steps)
	if err != nil {
		info := safeError("chain_dependency_resolution_failed", err)
		logger.Error().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("dependency resolution failed")
		return nil, safeContextError("chain_dependency_resolution_failed", err)
	}

	logger.Debug().Int("levels", len(levels)).Msg("dependencies resolved")

	// 2. Execute level by level.
	var allResults []ChainStepResult

	for levelIdx, level := range levels {
		if err := ctx.Err(); err != nil {
			info := safeError("chain_cancelled", err)
			logger.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Int("level", levelIdx).Msg("chain cancelled by context")
			// Propagate cancellation through remaining steps as errors.
			for _, step := range level {
				allResults = append(allResults, ChainStepResult{
					Name:    step.Name,
					Agent:   step.Agent,
					Success: false,
					Error:   safeErrorMessage("chain_cancelled"), ErrorCode: "chain_cancelled",
					Duration: 0,
				})
			}
			continue
		}

		if def.Parallel && len(level) > 1 {
			// Limit parallelism.
			maxParallel := ce.config.MaxParallel
			if maxParallel <= 0 {
				maxParallel = len(level)
			}

			levelResults := ce.executeLevelParallel(ctx, level, allResults, requestID, maxParallel)
			allResults = append(allResults, levelResults...)
		} else {
			for _, step := range level {
				if err := ctx.Err(); err != nil {
					allResults = append(allResults, ChainStepResult{
						Name:    step.Name,
						Agent:   step.Agent,
						Success: false,
						Error:   safeErrorMessage("chain_cancelled"), ErrorCode: "chain_cancelled",
						Duration: 0,
					})
					continue
				}
				result := ce.executeStep(ctx, step, allResults, requestID)
				allResults = append(allResults, result)
			}
		}
	}

	// 3. Build final response from the last successful step.
	response := ""
	finalAgent := ""
	for i := len(allResults) - 1; i >= 0; i-- {
		if allResults[i].Success {
			response = allResults[i].Output
			finalAgent = allResults[i].Agent
			break
		}
	}

	if response == "" {
		response = "Chain execution completed but all steps failed."
	}

	cr := &ChainResult{
		RequestID:  requestID,
		Steps:      allResults,
		FinalAgent: finalAgent,
		Response:   response,
		Duration:   time.Since(startTime),
	}

	logger.Info().
		Str("final_agent", finalAgent).
		Int("completed_steps", len(allResults)).
		Dur("duration", cr.Duration).
		Msg("chain execution complete")

	return cr, nil
}

// ─── Execute Dynamic ─────────────────────────────────────────────────────────

// ExecuteDynamic decomposes a complex request and runs a dynamically
// determined chain of agents. The CEO agent analyzes the request and
// determines which specialists to invoke.
func (ce *ChainExecutor) ExecuteDynamic(ctx context.Context, prompt string) (*ChainResult, error) {
	logger := log.Ctx(ctx).With().Str("chain_mode", "dynamic").Logger()

	if ce.engine == nil {
		return nil, fmt.Errorf("chain: no orchestrator configured for dynamic execution")
	}

	// 1. Build and send decomposition prompt.
	decompPrompt := ce.decomposePrompt(prompt)

	req := &Request{
		ID:      uuid.New().String(),
		Prompt:  decompPrompt,
		Context: map[string]interface{}{"agent": "CEO Agent"},
	}

	logger.Info().Msg("requesting task decomposition")

	result, err := ce.engine.Execute(ctx, req)
	if err != nil {
		return nil, safeContextError("chain_decomposition_failed", err)
	}

	// 2. Parse the decomposition response.
	steps, err := ce.parseDecomposition(result.Response)
	if err != nil {
		info := safeError("decomposition_parse_failed", err)
		logger.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("failed to parse decomposition, falling back to single-agent execution")
		// Fallback: treat as single step with CEO agent.
		return ce.fallbackSingleAgent(ctx, prompt, req.ID)
	}

	if len(steps) == 0 {
		logger.Warn().Msg("decomposition produced no steps, falling back")
		return ce.fallbackSingleAgent(ctx, prompt, req.ID)
	}

	// 3. Enforce MaxSteps limit.
	if ce.config.MaxSteps > 0 && len(steps) > ce.config.MaxSteps {
		logger.Warn().
			Int("decomposed", len(steps)).
			Int("max_steps", ce.config.MaxSteps).
			Msg("too many decomposed steps, truncating")
		steps = steps[:ce.config.MaxSteps]
	}

	logger.Info().Int("steps", len(steps)).Msg("decomposition complete, executing chain")

	// 4. Execute the discovered chain.
	def := ChainDefinition{
		Name:     "dynamic-" + req.ID,
		Steps:    steps,
		Parallel: true,
	}

	chainResult, err := ce.Execute(ctx, def)
	if err != nil {
		return nil, err
	}

	// 5. Synthesize the final response via the synthesis agent.
	synthesis, err := ce.synthesizeResults(ctx, prompt, chainResult.Steps)
	if err != nil {
		info := safeError("synthesis_failed", err)
		logger.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("synthesis failed, returning merged results")
		// Fall back without placing the provider error in the user/model output.
		chainResult.Response = safeErrorMessage("synthesis_failed") + "\n\n--- Results ---\n\n" +
			ce.formatStepResults(chainResult.Steps)
		return chainResult, nil
	}

	chainResult.Response = synthesis
	chainResult.FinalAgent = ce.config.SynthesisAgent

	return chainResult, nil
}

// fallbackSingleAgent runs the prompt through the CEO agent as a fallback
// when decomposition fails or produces no steps.
func (ce *ChainExecutor) fallbackSingleAgent(ctx context.Context, prompt, requestID string) (*ChainResult, error) {
	req := &Request{
		ID:      requestID + "-fallback",
		Prompt:  prompt,
		Context: map[string]interface{}{"agent": "CEO Agent"},
	}

	result, err := ce.engine.Execute(ctx, req)
	if err != nil {
		return nil, safeContextError("chain_fallback_failed", err)
	}

	return &ChainResult{
		RequestID:  requestID,
		Steps:      []ChainStepResult{},
		FinalAgent: "CEO Agent",
		Response:   result.Response,
		Duration:   result.Duration,
	}, nil
}

// ─── Execute Parallel ────────────────────────────────────────────────────────

// ExecuteParallel runs multiple agents on the same prompt concurrently
// and merges their results.
func (ce *ChainExecutor) ExecuteParallel(ctx context.Context, prompt string, agentNames []string) (*ChainResult, error) {
	startTime := time.Now()
	requestID := uuid.New().String()

	logger := log.Ctx(ctx).With().
		Str("chain_mode", "parallel").
		Str("request_id", requestID).
		Logger()

	logger.Info().Int("agents", len(agentNames)).Msg("parallel chain execution starting")

	if len(agentNames) == 0 {
		return &ChainResult{
			RequestID: requestID,
			Response:  "No agents specified for parallel execution",
			Duration:  time.Since(startTime),
		}, nil
	}

	if ce.engine == nil {
		return nil, fmt.Errorf("chain: no orchestrator configured for parallel execution")
	}

	// Build steps.
	steps := make([]ChainStep, len(agentNames))
	for i, name := range agentNames {
		steps[i] = ChainStep{
			Name:   fmt.Sprintf("parallel-%d", i+1),
			Agent:  name,
			Prompt: prompt,
		}
	}

	// Execute all in parallel (no dependencies).
	maxParallel := ce.config.MaxParallel
	if maxParallel <= 0 {
		maxParallel = len(steps)
	}

	results := make([]ChainStepResult, len(steps))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxParallel)

	for i, step := range steps {
		idx := i
		s := step
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Acquire semaphore to limit concurrency.
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[idx] = ChainStepResult{
					Name:    s.Name,
					Agent:   s.Agent,
					Success: false,
					Error:   safeErrorMessage("chain_cancelled"), ErrorCode: "chain_cancelled",
				}
				return
			}

			results[idx] = ce.executeStep(ctx, s, nil, requestID)
		}()
	}

	wg.Wait()

	// Merge all successful outputs.
	var outputs []string
	for _, r := range results {
		if r.Success && r.Output != "" {
			outputs = append(outputs, fmt.Sprintf("[%s]: %s", r.Agent, r.Output))
		}
	}

	response := strings.Join(outputs, "\n\n---\n\n")
	if response == "" {
		response = "All parallel agents failed to produce output."
	}

	cr := &ChainResult{
		RequestID: requestID,
		Steps:     results,
		Response:  response,
		Duration:  time.Since(startTime),
	}

	logger.Info().
		Int("completed", len(results)).
		Dur("duration", cr.Duration).
		Msg("parallel chain execution complete")

	return cr, nil
}

// ─── Synthesis ───────────────────────────────────────────────────────────────

// synthesizeResults takes outputs from multiple agents and produces
// a unified response by asking the synthesis agent to merge them.
func (ce *ChainExecutor) synthesizeResults(ctx context.Context, originalPrompt string, stepResults []ChainStepResult) (string, error) {
	formatted := ce.formatStepResults(stepResults)

	synthesisPrompt := fmt.Sprintf(
		"You are the %s. You delegated parts of a task to specialists.\n"+
			"Here are their results:\n\n%s\n\n"+
			"Original request: %s\n\n"+
			"Synthesize a unified, comprehensive response that combines all the specialist outputs. "+
			"Do not simply list the results — integrate them into a coherent final answer.",
		ce.config.SynthesisAgent,
		formatted,
		originalPrompt,
	)

	req := &Request{
		ID:      uuid.New().String(),
		Prompt:  synthesisPrompt,
		Context: map[string]interface{}{"agent": ce.config.SynthesisAgent},
	}

	result, err := ce.engine.Execute(ctx, req)
	if err != nil {
		return "", safeContextError("synthesis_execution_failed", err)
	}

	return result.Response, nil
}

// ─── Step Execution ──────────────────────────────────────────────────────────

// executeStep runs a single chain step through the orchestration engine.
// It builds the context from completed dependencies and records the outcome.
func (ce *ChainExecutor) executeStep(ctx context.Context, step ChainStep, completed []ChainStepResult, requestID string) ChainStepResult {
	startTime := time.Now()

	logger := log.Ctx(ctx).With().
		Str("chain_step", step.Name).
		Str("chain_agent", step.Agent).
		Str("request_id", requestID).
		Logger()

	logger.Info().Msg("executing chain step")

	// Build context for this step — inject outputs from dependency steps.
	stepCtx := ce.buildStepContext(step, completed)

	req := &Request{
		ID:      requestID + "-" + step.Name,
		Prompt:  step.Prompt,
		Context: stepCtx,
	}

	// Apply step-level timeout if configured.
	execCtx := ctx
	var cancel context.CancelFunc
	if step.Timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, step.Timeout)
		defer cancel()
	}

	// Check context before executing.
	if err := execCtx.Err(); err != nil {
		return ChainStepResult{
			Name:    step.Name,
			Agent:   step.Agent,
			Success: false,
			Error:   safeErrorMessage("chain_cancelled"), ErrorCode: "chain_cancelled",
			Duration: time.Since(startTime),
		}
	}

	result, err := ce.engine.Execute(execCtx, req)

	if err != nil {
		info := safeError("chain_step_failed", err)
		logger.Error().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("chain step failed")
		return ChainStepResult{
			Name:      step.Name,
			Agent:     step.Agent,
			Success:   false,
			Error:     safeErrorMessage("chain_step_failed"),
			ErrorCode: "chain_step_failed",
			Duration:  time.Since(startTime),
		}
	}

	logger.Debug().
		Str("agent_used", result.Agent).
		Dur("duration", result.Duration).
		Msg("chain step completed")

	return ChainStepResult{
		Name:     step.Name,
		Agent:    step.Agent,
		Success:  true,
		Output:   result.Response,
		Duration: time.Since(startTime),
	}
}

// executeLevelParallel runs a group of independent steps concurrently,
// respecting the configured parallelism limit.
func (ce *ChainExecutor) executeLevelParallel(ctx context.Context, steps []ChainStep, completed []ChainStepResult, requestID string, maxParallel int) []ChainStepResult {
	results := make([]ChainStepResult, len(steps))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxParallel)

	for i, step := range steps {
		idx := i
		s := step
		wg.Add(1)
		go func() {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[idx] = ChainStepResult{
					Name:    s.Name,
					Agent:   s.Agent,
					Success: false,
					Error:   safeErrorMessage("chain_cancelled"), ErrorCode: "chain_cancelled",
				}
				return
			}

			results[idx] = ce.executeStep(ctx, s, completed, requestID)
		}()
	}

	wg.Wait()
	return results
}

// ─── Prompt Building ─────────────────────────────────────────────────────────

// decomposePrompt builds the task decomposition prompt including the
// user's request, the list of available agents, and the catalogue of
// available skills (auto-routing by description).
func (ce *ChainExecutor) decomposePrompt(prompt string) string {
	var agentList strings.Builder

	if ce.agents != nil {
		// Try listing all agents for completeness.
		agents, err := ce.agents.Search("")
		if err == nil && len(agents) > 0 {
			for _, a := range agents {
				fmt.Fprintf(&agentList, "- **%s** (%s): %s\n", a.Name, a.Role, a.Description)
			}
		} else {
			agentList.WriteString("(No agent list available — use known Cosca agent names)\n")
		}
	} else {
		agentList.WriteString("(No agent resolver configured — use known Cosca agent names)\n")
	}

	// Skill catalogue for auto-routing: names plus one-line descriptions so the
	// LLM can self-discover which skill matches the task. The skill body is
	// deliberately omitted (progressive disclosure — loaded on demand via
	// GetInstructions). Fail-open: an empty or failing resolver simply skips
	// the section.
	var skillList strings.Builder
	if ce.skills != nil {
		if skills, err := ce.skills.List(); err == nil && len(skills) > 0 {
			skillList.WriteString("\n=== AVAILABLE SKILLS ===\n")
			for _, s := range skills {
				fmt.Fprintf(&skillList, "- %s: %s\n", s.Name, skillDescriptionLine(s.Description, 140))
			}
		}
	}

	return fmt.Sprintf(
		`You are a task decomposition specialist. Analyze the following request and determine which Cosca agents should handle it.
Break it down into steps, specifying the exact agent name and a detailed sub-task instruction for each step.
Order steps logically — later steps may depend on earlier ones.

Request: %s

Available agents:
%s%s

Respond with ONLY a JSON array of step objects. Each object must have "agent" (exact name from the list above) and "prompt" (the sub-task for that agent).
Do NOT include any explanatory text, markdown fences, or code blocks. Example:
[{"agent": "Backend Chief", "prompt": "design the API endpoints"}, {"agent": "Database Chief", "prompt": "design the database schema"}]`,
		prompt,
		agentList.String(),
		skillList.String(),
	)
}

// skillDescriptionLine flattens a skill description to a single line and
// truncates it to at most maxLen characters, appending "..." when truncated.
// It keeps the catalogue bounded so prompts stay small.
func skillDescriptionLine(description string, maxLen int) string {
	description = strings.ReplaceAll(description, "\r\n", " ")
	description = strings.ReplaceAll(description, "\n", " ")
	description = strings.TrimSpace(description)
	return truncateString(description, maxLen)
}

// ─── Decomposition Parsing ───────────────────────────────────────────────────

// parseDecomposition extracts a JSON array of chain steps from an LLM response.
// It handles responses with markdown fences, extra text, and minor formatting quirks.
func (ce *ChainExecutor) parseDecomposition(response string) ([]ChainStep, error) {
	response = strings.TrimSpace(response)
	if response == "" {
		return nil, fmt.Errorf("empty decomposition response")
	}

	// Try to find a JSON array in the response — strip markdown fences or
	// surrounding text.
	jsonStr := extractJSONArray(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON array found in decomposition response")
	}

	// Parse into an intermediate raw structure.
	type rawStep struct {
		Agent  string `json:"agent"`
		Prompt string `json:"prompt"`
	}

	var rawSteps []rawStep
	if err := json.Unmarshal([]byte(jsonStr), &rawSteps); err != nil {
		return nil, fmt.Errorf("failed to parse decomposition JSON: %w", err)
	}

	var steps []ChainStep
	for i, raw := range rawSteps {
		if strings.TrimSpace(raw.Agent) == "" || strings.TrimSpace(raw.Prompt) == "" {
			log.Warn().
				Int("index", i).
				Msg("skipping empty step in decomposition")
			continue
		}
		steps = append(steps, ChainStep{
			Name:   fmt.Sprintf("step-%d", i+1),
			Agent:  strings.TrimSpace(raw.Agent),
			Prompt: strings.TrimSpace(raw.Prompt),
		})
	}

	return steps, nil
}

// extractJSONArray extracts the first JSON array from a string that may
// contain markdown fences or surrounding text.
func extractJSONArray(s string) string {
	// Strip markdown code fences: ```json ... ```
	if strings.Contains(s, "```") {
		// Extract content between fences.
		start := strings.Index(s, "```")
		if start >= 0 {
			start += 3
			// Skip optional language tag.
			if nl := strings.Index(s[start:], "\n"); nl >= 0 {
				start += nl + 1
			}
			end := strings.Index(s[start:], "```")
			if end >= 0 {
				s = strings.TrimSpace(s[start : start+end])
			}
		}
	}

	// Find the first '[' and the matching closing ']'.
	jsonStart := strings.Index(s, "[")
	if jsonStart < 0 {
		return ""
	}

	// Simple bracket matching to find the right closing bracket.
	depth := 0
	for i := jsonStart; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return s[jsonStart : i+1]
			}
		}
	}

	return ""
}

// ─── Context Building ────────────────────────────────────────────────────────

// buildStepContext injects previous step outputs into a step's context.
// Dependency outputs are stored as "{stepName}_output" in the context map.
// The agent name is set as ContextData["agent"] so the router picks it up.
func (ce *ChainExecutor) buildStepContext(step ChainStep, completed []ChainStepResult) map[string]interface{} {
	ctxMap := make(map[string]interface{})

	// Set the target agent so the router resolves it directly.
	ctxMap["agent"] = step.Agent

	// Index completed steps by name for O(1) lookup.
	completedByName := make(map[string]ChainStepResult, len(completed))
	for _, r := range completed {
		completedByName[r.Name] = r
	}

	// Inject outputs of step dependencies.
	for _, dep := range step.DependsOn {
		if result, ok := completedByName[dep]; ok && result.Success {
			ctxMap[dep+"_output"] = result.Output
		}
	}

	return ctxMap
}

// ─── Formatting ──────────────────────────────────────────────────────────────

// formatStepResults formats step results for synthesis.
func (ce *ChainExecutor) formatStepResults(results []ChainStepResult) string {
	var sb strings.Builder

	for i, r := range results {
		fmt.Fprintf(&sb, "## Step %d: %s (Agent: %s)\n", i+1, r.Name, r.Agent)
		if r.Success {
			sb.WriteString(r.Output)
		} else {
			fmt.Fprintf(&sb, "[ERROR] %s", safeErrorMessage(r.ErrorCode))
		}
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// ─── Dependency Resolution ───────────────────────────────────────────────────

// resolveDependencies performs a topological sort on step list, grouping
// steps into execution levels. Steps within the same level are independent
// and can run in parallel. Returns an error when a cycle or unknown
// dependency is detected.
func resolveDependencies(steps []ChainStep) ([][]ChainStep, error) {
	if len(steps) == 0 {
		return nil, nil
	}

	// Build name → index mapping.
	nameToIdx := make(map[string]int, len(steps))
	for i, step := range steps {
		if step.Name == "" {
			return nil, fmt.Errorf("chain: step at index %d has an empty name", i)
		}
		if _, exists := nameToIdx[step.Name]; exists {
			return nil, fmt.Errorf("chain: duplicate step name %q", step.Name)
		}
		nameToIdx[step.Name] = i
	}

	// Build adjacency list and in-degree counts.
	inDegree := make([]int, len(steps))
	adj := make([][]int, len(steps))

	for i, step := range steps {
		for _, dep := range step.DependsOn {
			depIdx, ok := nameToIdx[dep]
			if !ok {
				return nil, fmt.Errorf("chain: step %q depends on unknown step %q", step.Name, dep)
			}
			// Prevent self-dependency.
			if depIdx == i {
				return nil, fmt.Errorf("chain: step %q cannot depend on itself", step.Name)
			}
			adj[depIdx] = append(adj[depIdx], i)
			inDegree[i]++
		}
	}

	// Kahn's algorithm — BFS by level.
	var levels [][]ChainStep
	queue := make([]int, 0)

	for i, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, i)
		}
	}

	for len(queue) > 0 {
		var nextQueue []int
		var level []ChainStep

		for _, idx := range queue {
			level = append(level, steps[idx])
			for _, neighbor := range adj[idx] {
				inDegree[neighbor]--
				if inDegree[neighbor] == 0 {
					nextQueue = append(nextQueue, neighbor)
				}
			}
		}

		levels = append(levels, level)
		queue = nextQueue
	}

	// If not all steps were processed, there is a cycle.
	totalProcessed := 0
	for _, level := range levels {
		totalProcessed += len(level)
	}
	if totalProcessed < len(steps) {
		return nil, fmt.Errorf("chain: circular dependency detected among steps")
	}

	return levels, nil
}
