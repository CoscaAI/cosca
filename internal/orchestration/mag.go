package orchestration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/contenttrust"
	"github.com/rs/zerolog/log"
)

// ─── MAG Configuration ───────────────────────────────────────────────────────

// MAGConfig configures the Memory-Augmented Generation wrapper.
type MAGConfig struct {
	// AutoStore enables automatic storage of results as memories.
	AutoStore bool

	// AutoRetrieve enables pre-execution memory retrieval.
	AutoRetrieve bool

	// MemoryLayer is the default layer for storing memories.
	MemoryLayer string // default: "session"

	// MemoryType is the default memory type for stored results.
	MemoryType string // default: "decision"

	// MaxRetrieveResults is the maximum number of memories to retrieve.
	MaxRetrieveResults int // default: 10

	// StoreTTL is the TTL for stored memories.
	StoreTTL time.Duration // default: 24h

	// MinPriority is the minimum priority for stored memories.
	MinPriority int // default: 5
}

// DefaultMAGConfig returns a MAGConfig populated with sensible defaults.
// Both AutoStore and AutoRetrieve are enabled by default.
func DefaultMAGConfig() MAGConfig {
	return MAGConfig{
		AutoStore:          true,
		AutoRetrieve:       true,
		MemoryLayer:        "session",
		MemoryType:         "decision",
		MaxRetrieveResults: 10,
		StoreTTL:           24 * time.Hour,
		MinPriority:        5,
	}
}

// ─── MAG Engine ──────────────────────────────────────────────────────────────

// MAG wraps the orchestration pipeline with memory capabilities.
// It provides automatic storage and retrieval of pipeline results as
// persistent memories, augmenting context before execution, recording
// intermediate decisions, and storing final results.
type MAG struct {
	retriever MemoryRetriever
	storer    MemoryStorer
	config    MAGConfig
}

// NewMAG creates a new MAG instance backed by the given retriever and storer.
// The config controls whether auto-store and auto-retrieve are enabled, and
// how memories are classified and prioritized.
func NewMAG(retriever MemoryRetriever, storer MemoryStorer, config MAGConfig) *MAG {
	// Apply defaults for any zero-value fields.
	if config.MemoryLayer == "" {
		config.MemoryLayer = "session"
	}
	if config.MemoryType == "" {
		config.MemoryType = "decision"
	}
	if config.MaxRetrieveResults <= 0 {
		config.MaxRetrieveResults = 10
	}
	if config.StoreTTL <= 0 {
		config.StoreTTL = 24 * time.Hour
	}
	if config.MinPriority <= 0 {
		config.MinPriority = 5
	}

	return &MAG{
		retriever: retriever,
		storer:    storer,
		config:    config,
	}
}

// ─── Pre-execution Retrieval ─────────────────────────────────────────────────

// AugmentContext retrieves relevant memories and enriches the pipeline context.
// Called BEFORE the main pipeline executes.
//
// When AutoRetrieve is enabled, it searches the memory engine for records
// relevant to the prompt and stores both a human-readable summary in
// Data.MemoryContext and the raw records in Data.RetrievedMemories.
//
// When AutoRetrieve is disabled, the context is returned unchanged.
func (m *MAG) AugmentContext(ctx context.Context, pc PipelineContext) (PipelineContext, error) {
	if !m.config.AutoRetrieve {
		return pc, nil
	}

	if m.retriever == nil {
		log.Warn().Str("request_id", pc.RequestID).Msg("MAG: no memory retriever configured, skipping context augmentation")
		return pc, nil
	}

	opts := MemorySearchOptions{
		Limit:       m.config.MaxRetrieveResults,
		ReaderAgent: pc.Data.ResolvedAgent,
	}

	records, err := m.retriever.Search(ctx, pc.Prompt, opts)
	if err != nil {
		info := safeError("mag_memory_search_failed", err)
		log.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).
			Str("request_id", pc.RequestID).
			Msg("MAG: memory search failed during context augmentation")
		// Do not fail the pipeline — return the original context.
		return pc, nil
	}

	// Quarantined/blocked records are never made available to context. The raw
	// record shape remains unchanged for compatibility with existing callers.
	records = filterMemoryRecords(records)

	// Build a human-readable summary from retrieved records.
	summary := m.buildMemorySummary(records)

	// Store both the formatted summary and the raw records in the pipeline context.
	pc = pc.WithMemoryContext(summary)
	pc = pc.WithRetrievedMemories(records)

	log.Info().
		Int("record_count", len(records)).
		Str("request_id", pc.RequestID).
		Msg("MAG: context augmented with retrieved memories")

	return pc, nil
}

// ─── Interim Storage ─────────────────────────────────────────────────────────

// RecordDecision stores an intermediate decision during pipeline execution.
// Called DURING pipeline execution for important decision points.
//
// It creates a MemoryRecord of type "decision" in the configured memory layer,
// annotated with pipeline metadata (request ID, agent, stage) and any
// additional caller-supplied metadata.
func (m *MAG) RecordDecision(ctx context.Context, pc PipelineContext, decision string, metadata map[string]string) (*MemoryRecord, error) {
	if m.storer == nil {
		log.Warn().Str("request_id", pc.RequestID).Msg("MAG: no memory storer configured, skipping decision recording")
		return nil, nil
	}

	// Build context summary for the decision record.
	contextSummary := fmt.Sprintf("[Pipeline Stage: %s] [Request: %s] %s", pc.Stage, pc.RequestID, decision)

	// Merge metadata: pipeline-level info plus caller-supplied metadata.
	merged := make(map[string]string, len(metadata)+3)
	merged["request_id"] = pc.RequestID
	merged["stage"] = pc.Stage

	if pc.Data.ResolvedAgent != "" {
		merged["agent"] = pc.Data.ResolvedAgent
	}

	for k, v := range metadata {
		merged[k] = v
	}

	record := MemoryRecord{
		Type:     "decision",
		Layer:    m.config.MemoryLayer,
		Content:  contextSummary,
		Metadata: merged,
		Priority: m.config.MinPriority,
	}

	stored, err := m.storer.Store(ctx, record)
	if err != nil {
		info := safeError("mag_store_decision_failed", err)
		log.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).
			Str("request_id", pc.RequestID).
			Str("layer", m.config.MemoryLayer).
			Msg("MAG: failed to store intermediate decision")
		return nil, safeContextError("mag_store_decision_failed", err)
	}

	log.Debug().
		Str("request_id", pc.RequestID).
		Str("memory_id", stored.ID).
		Str("stage", pc.Stage).
		Msg("MAG: intermediate decision stored as memory")

	return stored, nil
}

// ─── Post-execution Storage ──────────────────────────────────────────────────

// StoreResult persists the final pipeline result as a memory.
// Called AFTER the pipeline completes successfully.
//
// When AutoStore is enabled, it formats the result as structured text and
// stores it in the configured memory layer. The stored memory ID is written
// back to result.MemoryID.
//
// When AutoStore is disabled, it returns nil without storing anything.
func (m *MAG) StoreResult(ctx context.Context, pc PipelineContext, result *Result) (*MemoryRecord, error) {
	if !m.config.AutoStore {
		return nil, nil
	}

	if result == nil {
		log.Warn().Str("request_id", pc.RequestID).Msg("MAG: cannot store result — result is nil")
		return nil, nil
	}

	if m.storer == nil {
		log.Warn().Str("request_id", pc.RequestID).Msg("MAG: no memory storer configured, skipping result storage")
		return nil, nil
	}

	// Build structured content for the memory record.
	content := m.buildResultContent(pc, result)

	// Build metadata from pipeline context and result.
	metadata := make(map[string]string, 8)
	metadata["request_id"] = pc.RequestID

	if result.Agent != "" {
		metadata["agent"] = result.Agent
	} else if pc.Data.ResolvedAgent != "" {
		metadata["agent"] = pc.Data.ResolvedAgent
	}

	if len(result.SkillsUsed) > 0 {
		metadata["skills_used"] = strings.Join(result.SkillsUsed, ", ")
	}

	if result.Duration > 0 {
		metadata["duration"] = result.Duration.String()
	}

	// Extract model from LLMModel (set by executor).
	if pc.Data.LLMModel != "" {
		metadata["model"] = pc.Data.LLMModel
	}

	// Priority increases with the number of skills used.
	priority := m.config.MinPriority
	if len(result.SkillsUsed) > 0 {
		priority += len(result.SkillsUsed)
	}

	record := MemoryRecord{
		Type:     m.config.MemoryType,
		Layer:    m.config.MemoryLayer,
		Content:  content,
		Metadata: metadata,
		Priority: priority,
	}

	stored, err := m.storer.Store(ctx, record)
	if err != nil {
		info := safeError("mag_store_result_failed", err)
		log.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).
			Str("request_id", pc.RequestID).
			Str("layer", m.config.MemoryLayer).
			Str("memory_type", m.config.MemoryType).
			Msg("MAG: failed to store result as memory")
		return nil, safeContextError("mag_store_result_failed", err)
	}

	// Write the memory ID back to the result so callers can reference it.
	result.MemoryID = stored.ID

	log.Info().
		Str("request_id", pc.RequestID).
		Str("memory_id", stored.ID).
		Str("agent", metadata["agent"]).
		Int("priority", priority).
		Msg("MAG: result stored as memory")

	return stored, nil
}

// ─── Full MAG Pipeline ───────────────────────────────────────────────────────

// WrapExecute wraps the full synchronous execute pipeline with MAG.
// This is the main entry point: Augment → Execute → Store.
//
// It follows this flow:
//  1. AugmentContext — retrieve relevant memories and enrich pipeline context
//  2. Call executeFn — the actual pipeline execution
//  3. StoreResult — persist the final result as a memory (if successful)
//
// If any step fails, the error is propagated and the pipeline short-circuits.
func (m *MAG) WrapExecute(
	ctx context.Context,
	pc PipelineContext,
	executeFn func(context.Context, PipelineContext) (PipelineContext, *Result, error),
) (PipelineContext, *Result, error) {
	// 1. Augment context with relevant memories.
	augmentedPc, err := m.AugmentContext(ctx, pc)
	if err != nil {
		info := safeError("mag_context_augmentation_failed", err)
		log.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Str("request_id", pc.RequestID).Msg("MAG: context augmentation failed, continuing with original context")
		augmentedPc = pc
	}

	// 2. Execute the actual pipeline.
	resultPc, result, err := executeFn(ctx, augmentedPc)
	if err != nil {
		// Pipeline failed; do not store the result.
		return resultPc, result, err
	}

	if result == nil {
		return resultPc, result, nil
	}

	// 3. Store the successful result as a memory.
	_, storeErr := m.StoreResult(ctx, resultPc, result)
	if storeErr != nil {
		// Log the storage error but do not fail the pipeline.
		info := safeError("mag_result_storage_failed", storeErr)
		log.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).
			Str("request_id", pc.RequestID).
			Msg("MAG: result storage failed, but pipeline succeeded")
	}

	return resultPc, result, nil
}

// ─── MAG for Streaming ───────────────────────────────────────────────────────

// WrapExecuteStream wraps streaming execution with MAG.
// It augments the context with memories, executes the streaming function,
// and stores the final result after the stream completes.
//
// The flow:
//  1. AugmentContext — retrieve relevant memories
//  2. Emit a StreamEventProgress about memory retrieval
//  3. Call executeStreamFn — the actual streaming pipeline
//  4. StoreResult — persist the final result (if successful)
//  5. Emit a StreamEventProgress about memory storage
func (m *MAG) WrapExecuteStream(
	ctx context.Context,
	pc PipelineContext,
	executeStreamFn func(context.Context, PipelineContext, chan<- StreamEvent) (PipelineContext, *Result, error),
	eventCh chan<- StreamEvent,
) (PipelineContext, *Result, error) {
	// 1. Augment context with relevant memories.
	augmentedPc, err := m.AugmentContext(ctx, pc)
	if err != nil {
		info := safeError("mag_context_augmentation_failed", err)
		log.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Str("request_id", pc.RequestID).Msg("MAG: context augmentation failed, continuing with original context")
		augmentedPc = pc
	}

	// 2. Emit progress event about memory retrieval.
	if m.config.AutoRetrieve {
		recordCount := len(augmentedPc.Data.RetrievedMemories)
		m.emitEvent(eventCh, StreamEvent{
			Type:    StreamEventProgress,
			Content: fmt.Sprintf("Memory context augmented: %d relevant memories retrieved", recordCount),
			Metadata: map[string]interface{}{
				"record_count":  recordCount,
				"auto_retrieve": true,
				"max_results":   m.config.MaxRetrieveResults,
				"stage":         "mag:augment",
			},
		})
	}

	// 3. Execute the streaming pipeline.
	resultPc, result, err := executeStreamFn(ctx, augmentedPc, eventCh)
	if err != nil {
		// Pipeline failed; do not store the result.
		return resultPc, result, err
	}

	if result == nil {
		return resultPc, result, nil
	}

	// 4. Store the successful result as a memory.
	if m.config.AutoStore {
		stored, storeErr := m.StoreResult(ctx, resultPc, result)
		if storeErr != nil {
			info := safeError("mag_result_storage_failed", storeErr)
			log.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).
				Str("request_id", pc.RequestID).
				Msg("MAG: result storage failed, but streaming pipeline succeeded")
		}

		// 5. Emit progress event about memory storage.
		if stored != nil {
			m.emitEvent(eventCh, StreamEvent{
				Type:    StreamEventProgress,
				Content: fmt.Sprintf("Result stored as memory: %s (type: %s, layer: %s)", stored.ID, stored.Type, stored.Layer),
				Metadata: map[string]interface{}{
					"memory_id": stored.ID,
					"type":      stored.Type,
					"layer":     stored.Layer,
					"priority":  stored.Priority,
					"stage":     "mag:store",
				},
			})
		}
	}

	return resultPc, result, nil
}

// ─── Helper Methods ──────────────────────────────────────────────────────────

// buildMemorySummary formats a list of memory records into a human-readable
// text summary suitable for injection into the LLM context.
//
// Each record is rendered as a paragraph with its type, content snippet,
// and priority. Returns an empty string when records is nil or empty.
func (m *MAG) buildMemorySummary(records []MemoryRecord) string {
	if len(records) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Relevant Past Memories\n\n")

	for i, rec := range records {
		// Truncate long content for readability.
		snippet := truncateString(rec.Content, 500)
		item := contenttrust.FromMetadata(contenttrust.Default(contenttrust.OriginMemory, snippet, rec.ID), rec.Metadata)

		fmt.Fprintf(&sb, "### Memory %d (%s, priority %d)\n", i+1, rec.Type, rec.Priority)
		sb.WriteString(contenttrust.Envelope(item))

		// Include relevant metadata if present.
		if rec.Metadata != nil {
			if agent, ok := rec.Metadata["agent"]; ok && agent != "" {
				fmt.Fprintf(&sb, "\n*Agent: %s*", agent)
			}
			if stage, ok := rec.Metadata["stage"]; ok && stage != "" {
				fmt.Fprintf(&sb, " | *Stage: %s*", stage)
			}
		}

		sb.WriteString("\n\n")
	}

	return sb.String()
}

// buildResultContent formats a pipeline result into structured memory content
// suitable for persistent storage.
//
// The output is a Markdown-formatted document containing the original request,
// the agent that handled it, the full response, skills used, and total duration.
func (m *MAG) buildResultContent(pc PipelineContext, result *Result) string {
	var sb strings.Builder

	// --- Request ---
	sb.WriteString("## Request\n")
	sb.WriteString(pc.Prompt)
	sb.WriteString("\n\n")

	// --- Agent ---
	agentName := result.Agent
	if agentName == "" {
		agentName = pc.Data.ResolvedAgent
	}
	agentRole := pc.Data.AgentRole

	sb.WriteString("## Agent\n")
	if agentRole != "" && agentRole != agentName {
		fmt.Fprintf(&sb, "%s (%s)", agentName, agentRole)
	} else {
		sb.WriteString(agentName)
	}
	sb.WriteString("\n\n")

	// --- Response ---
	sb.WriteString("## Response\n")
	if result.Response != "" {
		sb.WriteString(result.Response)
	} else {
		sb.WriteString("*(empty response)*")
	}
	sb.WriteString("\n\n")

	// --- Skills Used ---
	sb.WriteString("## Skills Used\n")
	if len(result.SkillsUsed) > 0 {
		sb.WriteString(strings.Join(result.SkillsUsed, ", "))
	} else {
		sb.WriteString("*(none)*")
	}
	sb.WriteString("\n\n")

	// --- Duration ---
	sb.WriteString("## Duration\n")
	if result.Duration > 0 {
		sb.WriteString(result.Duration.String())
	} else {
		sb.WriteString("*(unknown)*")
	}
	sb.WriteString("\n")

	return sb.String()
}

// emitEvent sends a StreamEvent to the eventCh channel with best-effort
// delivery. If the channel is full or the consumer has stopped reading,
// the event is silently dropped after a short timeout.
func (m *MAG) emitEvent(eventCh chan<- StreamEvent, ev StreamEvent) {
	if eventCh == nil {
		return
	}

	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()

	select {
	case eventCh <- ev:
	case <-timer.C:
		// Consumer is too slow or has stopped reading; drop the event.
	}
}
