package orchestration

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ─── Step Types ──────────────────────────────────────────────────────────────

// StepType classifies the execution mode of a pipeline step.
type StepType string

const (
	// StepTypeSequential steps run one after another (default).
	StepTypeSequential StepType = "sequential"

	// StepTypeParallel steps run concurrently (fan-out).
	StepTypeParallel StepType = "parallel"

	// StepTypeConditional steps execute only when their Condition evaluates truthy.
	StepTypeConditional StepType = "conditional"
)

// ─── Pipeline Step ───────────────────────────────────────────────────────────

// PipelineStep defines a single step in the skill pipeline.
// Each step has a name, a skill to execute, and optional configuration.
type PipelineStep struct {
	// Name is a human-readable label for this step (used in logs and error messages).
	Name string

	// Skill is the skill name to resolve via the SkillResolver.
	Skill string

	// Input is the key in context data (Data.Extra) to read input from. When empty
	// the pipeline uses pc.Prompt as the step input.
	Input string

	// Output is the key in context data (Data.Extra) where the step result will be stored.
	Output string

	// Timeout is the maximum duration for this step. Zero means no timeout.
	Timeout time.Duration

	// Condition is an optional key in context data (Data.Extra). When set, the step is
	// skipped unless the value is truthy (non-nil, non-empty, non-false).
	Condition string

	// OnError controls step error behaviour: "skip" continues to the next step,
	// "warn" logs a warning and continues, "fail" (default) returns the error.
	OnError string
}

// ─── Parallel Step ───────────────────────────────────────────────────────────

// ParallelStep wraps multiple PipelineSteps that should execute concurrently.
// Results from all steps are merged into a single PipelineContext.
type ParallelStep struct {
	Steps []PipelineStep
}

// ─── Pipeline Definition ─────────────────────────────────────────────────────

// PipelineDefinition describes a complete pipeline of steps.
type PipelineDefinition struct {
	// Name is a human-readable identifier for this pipeline.
	Name string

	// Steps is the ordered list of steps to execute.
	Steps []PipelineStep

	// Parallel, when true, executes all steps concurrently instead of
	// sequentially. Individual parallel groups can also be expressed via
	// ParallelStep wrappers embedded in the step list.
	Parallel bool
}

// ─── Skill Processor ─────────────────────────────────────────────────────────

// SkillProcessor is a function that processes input and returns output.
// The context argument carries deadlines; input is the text to process;
// contextData provides the full pipeline data as a legacy map for
// backward compatibility (use typed field access where possible).
type SkillProcessor func(ctx context.Context, input string, contextData map[string]interface{}) (string, error)

// ─── Pipeline ────────────────────────────────────────────────────────────────

// Pipeline executes a sequence of skill invocations. Skills are resolved via
// the SkillResolver and executed by registered SkillProcessor functions.
type Pipeline struct {
	skills     SkillResolver
	processors map[string]SkillProcessor
	mu         sync.RWMutex
}

// NewPipeline creates a Pipeline backed by the given SkillResolver.
// Built-in default processors are registered automatically:
//   - transform: identity pass-through
//   - filter: basic keyword filter
//   - enrich: adds metadata from context
//   - validate: validates input is non-empty, checks agent and provider
//     are specified when present in context data, and enforces min length
//     constraints.
//   - format: wraps output in markdown code block
func NewPipeline(skills SkillResolver) *Pipeline {
	p := &Pipeline{
		skills:     skills,
		processors: make(map[string]SkillProcessor),
	}

	// Register built-in default processors.
	p.RegisterProcessor("transform", builtinTransform)
	p.RegisterProcessor("filter", builtinFilter)
	p.RegisterProcessor("enrich", builtinEnrich)
	p.RegisterProcessor("validate", builtinValidate)
	p.RegisterProcessor("format", builtinFormat)

	return p
}

// RegisterProcessor registers a skill processor function for the given
// skill name. Existing registrations are overwritten. The processor is
// invoked when a pipeline step references the corresponding skill.
func (p *Pipeline) RegisterProcessor(skillName string, processor SkillProcessor) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.processors[skillName] = processor
}

// ─── Execution ───────────────────────────────────────────────────────────────

// Execute runs the pipeline with the given context. When def.Parallel is true
// all steps are executed concurrently; otherwise steps run sequentially in
// order. The returned PipelineContext contains the enriched context data from
// all executed steps.
func (p *Pipeline) Execute(ctx context.Context, pc PipelineContext, def PipelineDefinition) (PipelineContext, error) {
	logger := log.Ctx(ctx).With().
		Str("pipeline", def.Name).
		Str("request_id", pc.RequestID).
		Logger()

	logger.Info().Int("steps", len(def.Steps)).Bool("parallel", def.Parallel).Msg("pipeline execution starting")

	if len(def.Steps) == 0 {
		logger.Debug().Msg("pipeline has no steps, returning context unchanged")
		return pc, nil
	}

	pc = pc.WithStage("pipeline:" + def.Name)

	var err error

	if def.Parallel {
		pc, err = p.ExecuteParallel(ctx, pc, def.Steps)
	} else {
		pc, err = p.executeSequential(ctx, pc, def.Steps)
	}

	if err != nil {
		info := safeError("pipeline_execution_failed", err)
		logger.Error().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("pipeline execution failed")
		return pc, err
	}

	logger.Info().Msg("pipeline execution complete")
	return pc, nil
}

// executeSequential runs steps one after another, respecting conditions and
// error policies.
func (p *Pipeline) executeSequential(ctx context.Context, pc PipelineContext, steps []PipelineStep) (PipelineContext, error) {
	for _, step := range steps {
		// Check context before each step.
		if err := ctx.Err(); err != nil {
			return pc, safePublicError("pipeline", "pipeline_context_cancelled", err)
		}

		var err error
		pc, err = p.ExecuteStep(ctx, pc, step)
		if err != nil {
			// OnError "fail" (default) propagates the error.
			if step.OnError == "" || step.OnError == "fail" {
				return pc, err
			}
			// "skip" and "warn" already handled inside ExecuteStep.
		}
	}

	return pc, nil
}

// ExecuteStep runs a single pipeline step. It:
//  1. Checks the Condition (if set) and skips when falsy.
//  2. Resolves the skill by name via the SkillResolver.
//  3. Reads input from Data.Extra[step.Input], falling back to pc.Prompt.
//  4. Looks up and invokes the registered SkillProcessor.
//  5. Stores the result in Data.Extra[step.Output].
//  6. Handles OnError: "skip" continues, "warn" logs, "fail" returns error.
func (p *Pipeline) ExecuteStep(ctx context.Context, pc PipelineContext, step PipelineStep) (PipelineContext, error) {
	logger := log.Ctx(ctx).With().
		Str("step", step.Name).
		Str("skill", step.Skill).
		Str("request_id", pc.RequestID).
		Logger()

	// 1. Condition check.
	if step.Condition != "" {
		if !isTruthy(pc.Data.Extra, step.Condition) {
			logger.Debug().Str("condition", step.Condition).Msg("step condition not met, skipping")
			return pc, nil
		}
	}

	// 2. Resolve skill for metadata (best-effort; processor drives execution).
	var skillName string
	if p.skills != nil {
		if info, err := p.skills.Get(step.Skill); err == nil && info != nil {
			skillName = info.Name
		}
	}
	if skillName == "" {
		skillName = step.Skill
		logger.Debug().Msg("skill not found in registry, using processor name directly")
	}

	// 3. Read input.
	input := p.readInput(pc, step.Input)
	logger.Debug().Str("input_key", step.Input).Int("input_len", len(input)).Msg("step input resolved")

	// 4. Look up processor.
	processor := p.lookupProcessor(skillName)
	if processor == nil {
		processor = p.lookupProcessor(step.Skill)
	}
	if processor == nil {
		err := fmt.Errorf("no processor registered for skill %q", step.Skill)
		return p.handleStepError(pc, step, err, logger)
	}

	// 5. Execute with optional timeout.
	execCtx := ctx
	var cancel context.CancelFunc
	if step.Timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, step.Timeout)
		defer cancel()
	}

	// Build a legacy map from PipelineData for backward-compatible processor calls.
	legacyMap := pc.Data.ToLegacyMap()

	result, err := processor(execCtx, input, legacyMap)
	if err != nil {
		return p.handleStepError(pc, step, err, logger)
	}

	// 6. Store output.
	outputKey := step.Output
	if outputKey == "" {
		outputKey = step.Skill + "_output"
	}
	pc = pc.WithContextData(outputKey, result)

	logger.Debug().
		Str("output_key", outputKey).
		Int("output_len", len(result)).
		Msg("step executed successfully")

	return pc, nil
}

// readInput extracts the step input from the pipeline context. It reads
// Data.Extra[inputKey]; if the value is a string it is returned directly.
// If the key is empty or missing the raw prompt is used as fallback.
func (p *Pipeline) readInput(pc PipelineContext, inputKey string) string {
	if inputKey == "" {
		return pc.Prompt
	}

	raw, ok := pc.Data.Extra[inputKey]
	if !ok {
		return pc.Prompt
	}

	switch v := raw.(type) {
	case string:
		if v == "" {
			return pc.Prompt
		}
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// handleStepError applies the OnError policy for a failing step.
// "skip" returns the context unchanged with no error.
// "warn" logs a warning and returns the context unchanged.
// "fail" (default) returns the error.
func (p *Pipeline) handleStepError(pc PipelineContext, step PipelineStep, err error, _ interface{}) (PipelineContext, error) {
	switch step.OnError {
	case "skip":
		info := safeError("pipeline_step_skipped", err)
		log.Debug().Str("step", step.Name).Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("step skipped due to error")
		return pc, nil
	case "warn":
		info := safeError("pipeline_step_warning", err)
		log.Warn().Str("step", step.Name).Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("step completed with warning")
		return pc, nil
	case "":
		fallthrough
	case "fail":
		fallthrough
	default:
		// A processor error is an untrusted boundary value.  In particular,
		// provider and filesystem errors commonly contain prompts, paths, or
		// response payloads.  Keep the error policy contract (fail returns an
		// error), but never expose the processor's raw text to callers.
		return pc, safePublicError("pipeline step", "pipeline_step_failed", err)
	}
}

// lookupProcessor returns the processor registered for the given skill name.
// Skill names are matched case-insensitively.
func (p *Pipeline) lookupProcessor(skillName string) SkillProcessor {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Exact match first.
	if proc, ok := p.processors[skillName]; ok {
		return proc
	}

	// Case-insensitive match.
	lower := strings.ToLower(skillName)
	for name, proc := range p.processors {
		if strings.ToLower(name) == lower {
			return proc
		}
	}

	return nil
}

// ─── Parallel Execution ──────────────────────────────────────────────────────

// stepResult captures the outcome of a single parallel step execution.
type stepResult struct {
	index int
	pc    PipelineContext
	err   error
}

// ExecuteParallel runs multiple steps concurrently and merges results into a
// single PipelineContext. Each step begins with a copy of the input context;
// results are merged sequentially after all goroutines complete. Context
// cancellation or step deadlines abort outstanding work.
func (p *Pipeline) ExecuteParallel(ctx context.Context, pc PipelineContext, steps []PipelineStep) (PipelineContext, error) {
	logger := log.Ctx(ctx).With().
		Str("request_id", pc.RequestID).
		Int("parallel_steps", len(steps)).
		Logger()

	logger.Debug().Msg("executing parallel steps")

	if len(steps) == 0 {
		return pc, nil
	}

	results := make(chan stepResult, len(steps))
	var wg sync.WaitGroup

	for i, step := range steps {
		// Capture loop variables.
		idx := i
		s := step
		// Deep-copy pc.Data so each goroutine has its own copy and
		// avoids data races when results are merged later.
		goroutinePC := PipelineContext{
			RequestID: pc.RequestID,
			Prompt:    pc.Prompt,
			Data:      pc.Data.Clone(),
			Stage:     pc.Stage,
			Inputs:    pc.Inputs.Clone(),
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			// Each goroutine uses its own isolated copy of the
			// pipeline context.
			stepPC, err := p.ExecuteStep(ctx, goroutinePC, s)

			// Non-blocking send; channel is buffered to size of steps.
			select {
			case results <- stepResult{index: idx, pc: stepPC, err: err}:
			case <-ctx.Done():
			}
		}()
	}

	// Close results channel after all goroutines finish.
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results.
	orderedResults := make([]stepResult, len(steps))
	var firstErr error

	for res := range results {
		orderedResults[res.index] = res

		// Merge context data from successful steps.
		if res.err == nil {
			// Merge extra data.
			for k, v := range res.pc.Data.Extra {
				pc = pc.WithContextData(k, v)
			}
			for k, v := range res.pc.Inputs.Extra {
				pc = pc.WithInput(k, v)
			}
		} else {
			// Collect first error for reporting.
			if firstErr == nil {
				firstErr = res.err
			}
			logger.Warn().
				Str("error_code", safeError("pipeline_step_failed", res.err).Code).
				Str("error_hash", safeError("pipeline_step_failed", res.err).Hash).
				Int("error_length", safeError("pipeline_step_failed", res.err).Length).
				Int("step_index", res.index).
				Str("step_name", steps[res.index].Name).
				Msg("parallel step failed")
		}
	}

	if firstErr != nil {
		return pc, safePublicError("pipeline", "pipeline_parallel_step_failed", firstErr)
	}

	logger.Debug().Int("completed", len(steps)).Msg("all parallel steps completed")
	return pc, nil
}

// ─── Truthiness Helpers ──────────────────────────────────────────────────────

// isTruthy checks whether the value at the given key in contextData is truthy.
// Truthy means: non-nil, non-empty string, non-false boolean, non-zero number,
// non-empty slice/map.
func isTruthy(contextData map[string]interface{}, key string) bool {
	val, ok := contextData[key]
	if !ok {
		return false
	}
	if val == nil {
		return false
	}

	switch v := val.(type) {
	case bool:
		return v
	case string:
		return v != ""
	case int:
		return v != 0
	case int8:
		return v != 0
	case int16:
		return v != 0
	case int32:
		return v != 0
	case int64:
		return v != 0
	case uint:
		return v != 0
	case uint8:
		return v != 0
	case uint16:
		return v != 0
	case uint32:
		return v != 0
	case uint64:
		return v != 0
	case float32:
		return v != 0
	case float64:
		return v != 0
	default:
		// Any other non-nil value (slice, map, struct, etc.) is truthy.
		return true
	}
}

// ─── Built-in Processors ─────────────────────────────────────────────────────

// builtinTransform performs basic text normalization: trims leading/trailing
// whitespace and collapses multiple spaces into one. Register a custom
// transform processor to replace this default behavior.
func builtinTransform(ctx context.Context, input string, _ map[string]interface{}) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	// Collapse whitespace
	words := strings.Fields(input)
	return strings.Join(words, " "), nil
}

// builtinFilter applies a basic keyword filter. It expects contextData to
// contain a "filter_keywords" key with a []string value. Only lines
// containing at least one keyword are retained. When no keywords are
// configured the input passes through unchanged.
func builtinFilter(ctx context.Context, input string, contextData map[string]interface{}) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	keywords := extractStringSlice(contextData, "filter_keywords")
	if len(keywords) == 0 {
		log.Debug().Msg("filter processor: no keywords configured, passing through")
		return input, nil
	}

	var filtered []string
	for _, line := range strings.Split(input, "\n") {
		lineLower := strings.ToLower(line)
		for _, kw := range keywords {
			if strings.Contains(lineLower, strings.ToLower(kw)) {
				filtered = append(filtered, line)
				break
			}
		}
	}

	if len(filtered) == 0 {
		return "", nil
	}

	log.Debug().
		Int("input_lines", len(strings.Split(input, "\n"))).
		Int("filtered_lines", len(filtered)).
		Strs("keywords", keywords).
		Msg("filter processor: filtering complete")

	return strings.Join(filtered, "\n"), nil
}

// builtinEnrich adds metadata from the pipeline context to the output. It
// prepends key-value pairs from contextData as a formatted preamble.
func builtinEnrich(ctx context.Context, input string, contextData map[string]interface{}) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	// Enrichment keys to include in the preamble.
	enrichKeys := []string{
		"resolved_agent",
		"agent_role",
		"agent_department",
		"request_id",
	}

	var preamble strings.Builder
	preamble.WriteString("--- Context Metadata ---\n")
	hasMeta := false

	for _, key := range enrichKeys {
		if val, ok := contextData[key]; ok {
			fmt.Fprintf(&preamble, "%s: %v\n", key, val)
			hasMeta = true
		}
	}

	if !hasMeta {
		preamble.WriteString("(no context metadata available)\n")
	}
	preamble.WriteString("--- End Metadata ---\n\n")

	return preamble.String() + input, nil
}

// builtinValidate performs input validation: rejects empty input, enforces
// a configurable minimum length, and verifies that agent and provider keys
// (if present in context data) are non-empty strings. Register a custom
// validate processor to replace this default behavior with domain-specific
// checks.
func builtinValidate(ctx context.Context, input string, contextData map[string]interface{}) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", fmt.Errorf("validation failed: input is empty")
	}

	// Minimum length check (configurable via contextData).
	minLen := 1
	if v, ok := contextData["min_length"].(int); ok && v > 0 {
		minLen = v
	}
	if len(trimmed) < minLen {
		return "", fmt.Errorf("validation failed: input length %d below minimum %d", len(trimmed), minLen)
	}

	// Verify agent key when present in context data.
	if agentRaw, ok := contextData["agent"]; ok {
		switch v := agentRaw.(type) {
		case string:
			if strings.TrimSpace(v) == "" {
				return "", fmt.Errorf("validation failed: agent key is present but empty")
			}
		case nil:
			return "", fmt.Errorf("validation failed: agent key is nil")
		default:
			if agentRaw == nil {
				return "", fmt.Errorf("validation failed: agent key is nil")
			}
		}
	}

	// Verify provider key when present in context data.
	if providerRaw, ok := contextData["provider"]; ok {
		switch v := providerRaw.(type) {
		case string:
			if strings.TrimSpace(v) == "" {
				return "", fmt.Errorf("validation failed: provider key is present but empty")
			}
		case nil:
			return "", fmt.Errorf("validation failed: provider key is nil")
		default:
			if providerRaw == nil {
				return "", fmt.Errorf("validation failed: provider key is nil")
			}
		}
	}

	log.Debug().Int("input_len", len(input)).Msg("validate processor: validation passed")
	return trimmed, nil
}

// builtinFormat wraps the input in a markdown code block. When contextData
// contains a "format_language" key its value is used as the language hint
// for the fenced code block.
func builtinFormat(ctx context.Context, input string, contextData map[string]interface{}) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	lang := extractString(contextData, "format_language")
	if lang == "" {
		lang = "text"
	}

	formatted := fmt.Sprintf("```%s\n%s\n```", lang, input)

	log.Debug().
		Str("language", lang).
		Int("input_len", len(input)).
		Int("output_len", len(formatted)).
		Msg("format processor: output wrapped in code block")

	return formatted, nil
}

// ─── Service ─────────────────────────────────────────────────────────────────

// Service provides a convenience wrapper for creating and executing pipelines.
// It holds a reference to the SkillResolver and manages the set of registered
// skill processors.
type Service struct {
	pipeline *Pipeline
}

// NewService creates a new pipeline Service backed by the given SkillResolver.
func NewService(skills SkillResolver) *Service {
	return &Service{
		pipeline: NewPipeline(skills),
	}
}

// RegisterProcessor delegates to the underlying Pipeline.
func (s *Service) RegisterProcessor(skillName string, processor SkillProcessor) {
	s.pipeline.RegisterProcessor(skillName, processor)
}

// Run executes a pipeline definition against the given context.
func (s *Service) Run(ctx context.Context, pc PipelineContext, def PipelineDefinition) (PipelineContext, error) {
	return s.pipeline.Execute(ctx, pc, def)
}

// RunStep executes a single pipeline step against the given context.
func (s *Service) RunStep(ctx context.Context, pc PipelineContext, step PipelineStep) (PipelineContext, error) {
	return s.pipeline.ExecuteStep(ctx, pc, step)
}

// RunParallel executes multiple steps concurrently.
func (s *Service) RunParallel(ctx context.Context, pc PipelineContext, steps []PipelineStep) (PipelineContext, error) {
	return s.pipeline.ExecuteParallel(ctx, pc, steps)
}

// ─── Context Data Helpers ────────────────────────────────────────────────────

// extractString reads a string value from a context-data map. Returns ""
// when the key is missing or the value is not a string.
func extractString(data map[string]interface{}, key string) string {
	v, ok := data[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// extractStringSlice reads a []string value from a context-data map.
// Returns nil when the key is missing or the value is not a []string.
func extractStringSlice(data map[string]interface{}, key string) []string {
	v, ok := data[key]
	if !ok {
		return nil
	}
	// Try direct []string first.
	if slice, ok := v.([]string); ok {
		return slice
	}
	// Try []interface{} conversion.
	if ifaceSlice, ok := v.([]interface{}); ok {
		out := make([]string, 0, len(ifaceSlice))
		for _, item := range ifaceSlice {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// ─── PipelineData Legacy Adapter ─────────────────────────────────────────────

// ToLegacyMap builds a flat map[string]interface{} from PipelineData,
// combining typed fields and the Extra map for backward compatibility
// with SkillProcessor functions and other legacy code.
func (pd PipelineData) ToLegacyMap() map[string]interface{} {
	m := copyStringMap(pd.Extra)

	// Knowledge & Memory
	if pd.KnowledgeResults != nil {
		m["knowledge_results"] = pd.KnowledgeResults
	}
	m["knowledge_cache_hit"] = pd.KnowledgeCacheHit
	if len(pd.MemoryResults) > 0 {
		m["memory_results"] = pd.MemoryResults
	}
	if len(pd.RetrievedMemories) > 0 {
		m["retrieved_memories"] = pd.RetrievedMemories
	}
	if pd.MemoryContext != "" {
		m["memory_context"] = pd.MemoryContext
	}

	// Agent & Intent
	if pd.ResolvedAgent != "" {
		m["resolved_agent"] = pd.ResolvedAgent
	}
	if pd.AgentRole != "" {
		m["agent_role"] = pd.AgentRole
	}
	if pd.AgentDepartment != "" {
		m["agent_department"] = pd.AgentDepartment
	}
	if pd.AgentDescription != "" {
		m["agent_description"] = pd.AgentDescription
	}
	if len(pd.SkillsUsed) > 0 {
		m["skills_used"] = pd.SkillsUsed
	}
	if pd.RouterMethod != "" {
		m["router_method"] = pd.RouterMethod
	}

	// Prompt & LLM
	if pd.AugmentedPrompt != "" {
		m["augmented_prompt"] = pd.AugmentedPrompt
	}
	if pd.LLMResponse != "" {
		m["llm_response"] = pd.LLMResponse
	}
	if pd.LLMModel != "" {
		m["llm_model"] = pd.LLMModel
	}
	if pd.LLMUsage != nil {
		m["llm_usage"] = pd.LLMUsage
	}

	// Tool calls
	if pd.ToolCalls != nil {
		m["tool_calls"] = pd.ToolCalls
	}
	if pd.ToolResults != nil {
		m["tool_results"] = pd.ToolResults
	}

	// Execution metadata
	if pd.EmbeddingError != "" {
		m["embedding_error"] = pd.EmbeddingError
	}
	if pd.SemanticScore != 0 {
		m["semantic_score"] = pd.SemanticScore
	}
	if pd.SemanticSecondScore != 0 {
		m["semantic_second_score"] = pd.SemanticSecondScore
	}
	if pd.SemanticMargin != 0 {
		m["semantic_margin"] = pd.SemanticMargin
	}
	m["executor_fallback"] = pd.ExecutorFallback
	if pd.MemoryID != "" {
		m["memory_id"] = pd.MemoryID
	}

	return m
}
