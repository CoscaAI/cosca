package orchestration

import (
	"context"
	"time"
)

// ─── Request ─────────────────────────────────────────────────────────────────

// Request represents a user request submitted to the orchestration engine.
type Request struct {
	// ID is the unique request identifier. When empty, the engine generates one.
	ID string `json:"id,omitempty"`

	// Prompt is the natural-language instruction from the user.
	Prompt string `json:"prompt"`

	// Context carries optional key-value metadata passed alongside the prompt
	// (e.g. workspace path, session identifier, target pipeline stage).
	Context map[string]interface{} `json:"context,omitempty"`
}

// ─── Result ──────────────────────────────────────────────────────────────────

// Result represents the final outcome of an orchestrated execution.
type Result struct {
	// ID mirrors the originating request identifier.
	ID string `json:"id"`

	// Response is the final textual answer assembled by the engine.
	Response string `json:"response"`

	// Agent is the name of the agent that handled the request.
	Agent string `json:"agent,omitempty"`

	// SkillsUsed lists the skills invoked during execution.
	SkillsUsed []string `json:"skills_used,omitempty"`

	// MemoryID points to the memory record created when the engine persists
	// the result. May be empty if persistence is disabled.
	MemoryID string `json:"memory_id,omitempty"`

	// Duration is the total wall-clock time spent processing the request.
	Duration time.Duration `json:"duration"`
}

// ─── Pipeline Data ───────────────────────────────────────────────────────────

// PipelineData holds typed pipeline stage outputs instead of map[string]interface{}.
// Each field corresponds to a well-known key previously stored in ContextData.
type PipelineData struct {
	// Knowledge & Memory
	KnowledgeResults  *KnowledgeSearchResults // from knowledge search stage
	KnowledgeCacheHit bool                    // whether knowledge was cached
	MemoryResults     []MemoryRecord          // from ContextBuilder stage
	RetrievedMemories []MemoryRecord          // from MAG/memory search stage
	MemoryContext     string                  // formatted memory summary from MAG

	// Agent & Intent
	ResolvedAgent      string   // from router stage
	AgentRole          string   // agent's role
	AgentDepartment    string   // agent's department
	AgentDescription   string   // agent's description
	AgentCapabilities  []string // agent's capabilities (from capability registry)
	AgentResponsibilities []string // agent's responsibilities (from agent definition)
	SkillsUsed         []string // skills invoked
	RouterMethod       string   // how the agent was selected (explicit/keyword/search/semantic/fallback)

	// Prompt & LLM
	AugmentedPrompt string      // augmented prompt before LLM call
	LLMResponse     string      // raw response from LLM
	LLMModel        string      // model name used
	LLMUsage        interface{} // token usage info (chat.Usage, kept as interface{} to avoid coupling)

	// Tool calls
	ToolCalls   interface{} // tool calls returned by LLM ([]chat.ToolCall)
	ToolResults interface{} // tool call results from executor ([]*ToolCallResult)

	// Execution metadata
	EmbeddingError      string  // embedding error message if any
	SemanticScore       float64 // semantic similarity score (best match)
	SemanticSecondScore float64 // runner-up semantic score (p/ margem)
	SemanticMargin      float64 // gap entre best e second (confiança discriminativa)
	ExecutorFallback    bool    // whether executor used retry/fallback
	ExecutorDeterministic bool // whether executor answered WITHOUT LLM (knowledge-only)
	MemoryID            string  // stored memory ID

	// Metrics
	StageTimings map[string]time.Duration // per-stage timing

	// Extensible: for custom data that doesn't fit the schema yet.
	// Pipeline step outputs, dynamic keys, and user-supplied metadata go here.
	Extra map[string]interface{}
}

// NewPipelineData creates a PipelineData with initialized maps.
func NewPipelineData() PipelineData {
	return PipelineData{
		Extra:        make(map[string]interface{}),
		StageTimings: make(map[string]time.Duration),
	}
}

// Clone returns a shallow copy of PipelineData with deep-copied maps (Extra, StageTimings)
// and deep-copied slices (MemoryResults, RetrievedMemories, SkillsUsed).
func (pd PipelineData) Clone() PipelineData {
	cp := pd

	// Deep-copy maps.
	cp.Extra = copyStringMap(pd.Extra)
	cp.StageTimings = make(map[string]time.Duration, len(pd.StageTimings))
	for k, v := range pd.StageTimings {
		cp.StageTimings[k] = v
	}

	// Deep-copy slices to avoid shared mutation.
	if pd.MemoryResults != nil {
		cp.MemoryResults = make([]MemoryRecord, len(pd.MemoryResults))
		copy(cp.MemoryResults, pd.MemoryResults)
	}
	if pd.RetrievedMemories != nil {
		cp.RetrievedMemories = make([]MemoryRecord, len(pd.RetrievedMemories))
		copy(cp.RetrievedMemories, pd.RetrievedMemories)
	}
	if pd.SkillsUsed != nil {
		cp.SkillsUsed = make([]string, len(pd.SkillsUsed))
		copy(cp.SkillsUsed, pd.SkillsUsed)
	}

	return cp
}

// GetExtraString safely reads a string value from Extra. Returns "" when missing.
func (pd PipelineData) GetExtraString(key string) string {
	v, ok := pd.Extra[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// GetExtraBool safely reads a bool value from Extra. Returns false when missing.
func (pd PipelineData) GetExtraBool(key string) bool {
	v, ok := pd.Extra[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	if !ok {
		return false
	}
	return b
}

// ─── Pipeline Context ────────────────────────────────────────────────────────

// PipelineContext is an immutable value object that flows through every stage
// of the orchestration pipeline. Each stage receives a PipelineContext,
// operates on its data, and returns a new PipelineContext for the next stage.
// This builder-style, copy-on-write approach guarantees that no stage can
// accidentally mutate state used by an earlier or concurrent stage.
type PipelineContext struct {
	// RequestID is the unique identifier of the user request.
	RequestID string

	// Prompt is the original (or augmented) user prompt.
	Prompt string

	// Data carries typed, structured pipeline stage outputs accumulated across
	// pipeline stages (e.g. resolved agent, retrieved knowledge, LLM response).
	Data PipelineData

	// Stage is the name of the pipeline stage currently being processed.
	Stage string

	// Inputs are per-stage named inputs provided by the previous stage.
	Inputs PipelineData
}

// NewPipelineContext creates a fresh PipelineContext primed with the given
// request ID and prompt. All maps are initialized so callers never receive nil.
func NewPipelineContext(requestID, prompt string) PipelineContext {
	return PipelineContext{
		RequestID: requestID,
		Prompt:    prompt,
		Data:      NewPipelineData(),
		Inputs:    NewPipelineData(),
	}
}

// WithStage returns a copy of the context with the Stage field set to name.
func (pc PipelineContext) WithStage(name string) PipelineContext {
	pc.Stage = name
	return pc
}

// WithPrompt returns a copy of the context with the prompt replaced by the
// supplied value. Useful when a stage rewrites or augments the prompt before
// handing it to the LLM.
func (pc PipelineContext) WithPrompt(prompt string) PipelineContext {
	pc.Prompt = prompt
	return pc
}

// ─── Typed Setters ───────────────────────────────────────────────────────────

// WithKnowledgeResults sets the knowledge search results.
func (pc PipelineContext) WithKnowledgeResults(v *KnowledgeSearchResults) PipelineContext {
	pc.Data.KnowledgeResults = v
	return pc
}

// WithKnowledgeCacheHit sets whether the knowledge search was cached.
func (pc PipelineContext) WithKnowledgeCacheHit(v bool) PipelineContext {
	pc.Data.KnowledgeCacheHit = v
	return pc
}

// WithMemoryResults sets the memory search results.
func (pc PipelineContext) WithMemoryResults(v []MemoryRecord) PipelineContext {
	pc.Data.MemoryResults = v
	return pc
}

// WithRetrievedMemories sets the memories retrieved by MAG.
func (pc PipelineContext) WithRetrievedMemories(v []MemoryRecord) PipelineContext {
	pc.Data.RetrievedMemories = v
	return pc
}

// WithMemoryContext sets the formatted memory summary.
func (pc PipelineContext) WithMemoryContext(v string) PipelineContext {
	pc.Data.MemoryContext = v
	return pc
}

// WithAugmentedPrompt sets the augmented prompt.
func (pc PipelineContext) WithAugmentedPrompt(v string) PipelineContext {
	pc.Data.AugmentedPrompt = v
	return pc
}

// WithResolvedAgent sets the resolved agent name.
func (pc PipelineContext) WithResolvedAgent(name string) PipelineContext {
	pc.Data.ResolvedAgent = name
	return pc
}

// WithAgentRole sets the agent's role.
func (pc PipelineContext) WithAgentRole(role string) PipelineContext {
	pc.Data.AgentRole = role
	return pc
}

// WithAgentDepartment sets the agent's department.
func (pc PipelineContext) WithAgentDepartment(dept string) PipelineContext {
	pc.Data.AgentDepartment = dept
	return pc
}

// WithAgentDescription sets the agent's description.
func (pc PipelineContext) WithAgentDescription(desc string) PipelineContext {
	pc.Data.AgentDescription = desc
	return pc
}

// WithAgentCapabilities sets the agent's capabilities list.
func (pc PipelineContext) WithAgentCapabilities(caps []string) PipelineContext {
	pc.Data.AgentCapabilities = caps
	return pc
}

// WithAgentResponsibilities sets the agent's responsibilities list.
func (pc PipelineContext) WithAgentResponsibilities(reps []string) PipelineContext {
	pc.Data.AgentResponsibilities = reps
	return pc
}

// WithSkillsUsed sets the skills used list.
func (pc PipelineContext) WithSkillsUsed(skills []string) PipelineContext {
	pc.Data.SkillsUsed = skills
	return pc
}

// WithRouterMethod sets the router method used.
func (pc PipelineContext) WithRouterMethod(method string) PipelineContext {
	pc.Data.RouterMethod = method
	return pc
}

// WithLLMResponse sets the LLM response text.
func (pc PipelineContext) WithLLMResponse(v string) PipelineContext {
	pc.Data.LLMResponse = v
	return pc
}

// WithLLMModel sets the LLM model name.
func (pc PipelineContext) WithLLMModel(v string) PipelineContext {
	pc.Data.LLMModel = v
	return pc
}

// WithLLMUsage sets the LLM token usage.
func (pc PipelineContext) WithLLMUsage(v interface{}) PipelineContext {
	pc.Data.LLMUsage = v
	return pc
}

// WithToolCalls sets the tool calls from the LLM response.
func (pc PipelineContext) WithToolCalls(v interface{}) PipelineContext {
	pc.Data.ToolCalls = v
	return pc
}

// WithToolResults sets the tool call execution results.
func (pc PipelineContext) WithToolResults(v interface{}) PipelineContext {
	pc.Data.ToolResults = v
	return pc
}

// WithEmbeddingError sets the embedding error message.
func (pc PipelineContext) WithEmbeddingError(v string) PipelineContext {
	pc.Data.EmbeddingError = v
	return pc
}

// WithSemanticScore sets the semantic similarity score.
func (pc PipelineContext) WithSemanticScore(score float64) PipelineContext {
	pc.Data.SemanticScore = score
	return pc
}

// WithSemanticSecondScore sets the runner-up semantic score. O segundo melhor
// score sustenta a margem de confiança (score absoluto ≠ confiança
// discriminativa).
func (pc PipelineContext) WithSemanticSecondScore(score float64) PipelineContext {
	pc.Data.SemanticSecondScore = score
	return pc
}

// WithSemanticMargin sets a margem entre best e second, sinalizando quão
// discriminativo foi o match. Registrada no trace para o Auto-Audit calibrar
// empiricamente o limiar de margem ótimo.
func (pc PipelineContext) WithSemanticMargin(margin float64) PipelineContext {
	pc.Data.SemanticMargin = margin
	return pc
}

// WithExecutorFallback sets whether the executor used a fallback.
func (pc PipelineContext) WithExecutorFallback(v bool) PipelineContext {
	pc.Data.ExecutorFallback = v
	return pc
}

// WithExecutorDeterministic sets whether the executor answered WITHOUT LLM
// (knowledge-only path — the Cosca is a deterministic AI with optional motor).
func (pc PipelineContext) WithExecutorDeterministic(v bool) PipelineContext {
	pc.Data.ExecutorDeterministic = v
	return pc
}

// WithMemoryID sets the stored memory ID.
func (pc PipelineContext) WithMemoryID(v string) PipelineContext {
	pc.Data.MemoryID = v
	return pc
}

// WithStageTiming records a stage's duration.
func (pc PipelineContext) WithStageTiming(stage string, d time.Duration) PipelineContext {
	if pc.Data.StageTimings == nil {
		pc.Data.StageTimings = make(map[string]time.Duration)
	}
	pc.Data.StageTimings[stage] = d
	return pc
}

// WithExtra adds a key-value pair to the Extra map.
func (pc PipelineContext) WithExtra(key string, v interface{}) PipelineContext {
	pc.Data.Extra = copyStringMap(pc.Data.Extra)
	pc.Data.Extra[key] = v
	return pc
}

// ─── Backward-Compatible Generic Setters ─────────────────────────────────────

// WithContextData returns a copy of the context with the given key-value pair
// added to Data.Extra. Existing entries are preserved. This method is retained
// for backward compatibility; prefer typed setters for known keys.
func (pc PipelineContext) WithContextData(key string, value interface{}) PipelineContext {
	pc.Data.Extra = copyStringMap(pc.Data.Extra)
	pc.Data.Extra[key] = value
	return pc
}

// WithInput returns a copy of the context with the given key-value pair added
// to Inputs.Extra. Existing entries are preserved.
func (pc PipelineContext) WithInput(key string, value interface{}) PipelineContext {
	pc.Inputs.Extra = copyStringMap(pc.Inputs.Extra)
	pc.Inputs.Extra[key] = value
	return pc
}

// ─── Legacy Accessors ────────────────────────────────────────────────────────

// GetContextData retrieves a value from Data.Extra by key. Returns nil when the
// key is missing. For typed fields, use the typed accessors on pc.Data directly.
func (pc PipelineContext) GetContextData(key string) interface{} {
	return pc.Data.Extra[key]
}

// ─── Stage Result ────────────────────────────────────────────────────────────

// StageResult captures the outcome of a single pipeline stage. It includes
// timing, success/failure status, and any diagnostic warnings.
type StageResult struct {
	// Stage is the name of the pipeline stage that produced this result.
	Stage string `json:"stage"`

	// Success indicates whether the stage completed without errors.
	Success bool `json:"success"`

	// Error holds the error message when Success is false. It is omitted
	// from JSON when empty so successful stages carry no error baggage.
	Error     string `json:"error,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`

	// Output is the stage's primary output value. Its concrete type
	// depends on the stage (e.g. *KnowledgeSearchResults for the
	// knowledge stage, []AgentInfo for the agent-resolution stage).
	Output interface{} `json:"output,omitempty"`

	// Duration is the wall-clock time consumed by this stage.
	Duration time.Duration `json:"duration"`

	// Warnings are non-fatal diagnostic messages produced by the stage
	// (e.g. "cache miss", "fallback used").
	Warnings []string `json:"warnings,omitempty"`
}

// NewStageResult is a convenience constructor for a successful stage result.
func NewStageResult(stage string, output interface{}, duration time.Duration) StageResult {
	return StageResult{
		Stage:    stage,
		Success:  true,
		Output:   output,
		Duration: duration,
	}
}

// NewStageError is a convenience constructor for a failed stage result.
func NewStageError(stage string, err error, duration time.Duration) StageResult {
	return StageResult{
		Stage:     stage,
		Success:   false,
		Error:     safeErrorMessage("stage_failed"),
		ErrorCode: safeError("stage_failed", err).Code,
		Duration:  duration,
	}
}

// AddWarning appends a warning message and returns the StageResult so calls
// can be chained.
func (sr StageResult) AddWarning(warning string) StageResult {
	sr.Warnings = append(sr.Warnings, warning)
	return sr
}

// ─── Stream Event ────────────────────────────────────────────────────────────

// StreamEventType classifies the kind of stream event emitted during
// streaming execution.
type StreamEventType string

const (
	// StreamEventProgress signals general progress updates (e.g. "resolving agent").
	StreamEventProgress StreamEventType = "progress"

	// StreamEventChunk carries a text chunk from a streaming LLM response.
	StreamEventChunk StreamEventType = "chunk"

	// StreamEventStageTransition is emitted when the pipeline moves from one
	// stage to the next.
	StreamEventStageTransition StreamEventType = "stage_transition"

	// StreamEventError carries a non-fatal error encountered during streaming.
	StreamEventError StreamEventType = "error"
)

// StreamEvent is a lightweight message emitted on the streaming channel
// during ExecuteStream. Consumers can inspect the Type field to decide how
// to handle the Content.
type StreamEvent struct {
	// Type classifies this event so consumers know how to interpret Content.
	Type StreamEventType `json:"type"`

	// Content is the human-readable payload of the event.
	Content string `json:"content"`

	// Metadata carries optional structured data associated with the event
	// (e.g. stage name for stage transitions, token counts for chunks).
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ─── Orchestrator ────────────────────────────────────────────────────────────

// Orchestrator is the central interface of the AI Orchestration Engine.
// Implementations wire together the KnowledgeSearcher, MemoryRetriever,
// MemoryStorer, AgentResolver, SkillResolver, ChatProvider, and Embedder ports
// into a configurable execution pipeline.
type Orchestrator interface {
	// Execute runs the orchestration pipeline synchronously and returns the
	// final result. The context carries deadlines and cancellation signals.
	Execute(ctx context.Context, req *Request) (*Result, error)

	// ExecuteStream runs the orchestration pipeline in streaming mode. It
	// returns a channel that emits StreamEvent values as the pipeline
	// progresses. The channel is closed when execution completes (or the
	// context is cancelled). The caller must consume the channel to avoid
	// deadlocks.
	ExecuteStream(ctx context.Context, req *Request) (<-chan StreamEvent, error)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// copyStringMap returns a shallow copy of the input map. Nil maps produce
// a new empty map so callers never receive nil.
func copyStringMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
