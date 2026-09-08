package orchestration

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/CoscaAI/cosca/internal/contenttrust"
	"github.com/rs/zerolog/log"
)

// ContextBuilder enriches pipeline context with knowledge and memory search results.
// It implements the Context-Building stage of the orchestration pipeline, fetching
// relevant information from the Knowledge Engine and Memory Engine to augment the
// user's prompt with contextual data before it reaches the agent.
type ContextBuilder struct {
	knowledge  KnowledgeSearcher
	memory     MemoryRetriever
	embedCache *EmbedCache
}

// NewContextBuilder creates a new ContextBuilder backed by the given knowledge
// searcher and memory retriever implementations. The embedCache parameter is
// optional — pass nil to disable the knowledge search cache.
func NewContextBuilder(knowledge KnowledgeSearcher, memory MemoryRetriever, embedCache *EmbedCache) *ContextBuilder {
	return &ContextBuilder{
		knowledge:  knowledge,
		memory:     memory,
		embedCache: embedCache,
	}
}

// Build performs both a knowledge search and a memory search, then retrieves
// recent session memories. It enriches the PipelineContext with all results and
// builds an augmented prompt combining the found context with the original prompt.
// Errors from individual search operations are logged as warnings but do not fail
// the pipeline — the original prompt is preserved.
func (cb *ContextBuilder) Build(ctx context.Context, pc PipelineContext) (PipelineContext, error) {
	pc = pc.WithStage("context_builder")

	// 1. Knowledge search (up to 10 results)
	pc, _ = cb.BuildKnowledge(ctx, pc, 10)

	// 2. Memory search (session layer, recent memories)
	pc, _ = cb.BuildMemory(ctx, pc)

	// 3. Build augmented prompt from collected context
	pc = cb.augmentPrompt(pc)

	return pc, nil
}

// BuildKnowledge searches only the knowledge engine for information relevant
// to the user's prompt. Results are stored in Data.KnowledgeResults.
// When an EmbedCache is configured, previously-seen prompts are served from
// cache, avoiding redundant knowledge searches. Errors are logged but do not
// fail the pipeline — the original context is returned unchanged on error.
func (cb *ContextBuilder) BuildKnowledge(ctx context.Context, pc PipelineContext, maxResults int) (PipelineContext, error) {
	pc = pc.WithStage("context_builder:knowledge")

	// 1. Check cache before calling the knowledge engine.
	if cb.embedCache != nil {
		if cached, ok := cb.embedCache.Get(pc.Prompt); ok {
			cached.Results = filterKnowledgeResults(cached.Results)
			pc = pc.WithKnowledgeResults(cached)
			pc = pc.WithKnowledgeCacheHit(true)
			log.Debug().
				Int("count", len(cached.Results)).
				Msg("context builder: knowledge search served from embed cache")
			return pc, nil
		}
		// Cache miss — mark for metrics tracking.
		pc = pc.WithKnowledgeCacheHit(false)
	}

	if cb.knowledge == nil {
		log.Warn().Msg("context builder: no knowledge searcher configured, skipping knowledge enrichment")
		return pc, nil
	}

	params := KnowledgeSearchParams{
		Query: pc.Prompt,
		Limit: maxResults,
	}

	// ── ADR-045 F4: Task-Aware Search ─────────────────────────────
	// Quando o contextpipeline ajustou os SearchParams pela fase detectada
	// (DESIGN-001 §5.4), o estágio de busca os consome aqui. Aditivo:
	// SearchParams zero-value → comportamento atual (retrocompatível).
	if sp := pc.Data.SearchParams; sp.Limit > 0 || sp.MinScore > 0 || sp.Path != "" || len(sp.Types) > 0 {
		if sp.Limit > 0 {
			params.Limit = sp.Limit
		}
		params.MinScore = sp.MinScore
		if sp.Path != "" {
			params.Path = sp.Path
		}
		if len(sp.Types) > 0 {
			params.Types = sp.Types
		}
	}

	// ── Awakening query ──────────────────────────────────────────────
	// When the prompt is a greeting (the family's "oi"), the oracle is
	// waking up. Search for IDENTITY + STATE, not the raw greeting —
	// this is how the Cosca Kernel awakens (SEMANTIC_AWAKENING_PROTOCOL
	// Pilar 1: "quem sou eu").
	if isGreetingPrompt(pc.Prompt) {
		params.Query = "cosca kernel consigliere braço direito do Don identidade quem sou estado sistema"
		log.Debug().Str("query", params.Query).Msg("context builder: awakening query (greeting detected)")
	}

	results, err := cb.knowledge.Search(ctx, params)
	if err != nil {
		info := safeError("knowledge_search_failed", err)
		log.Warn().Str("request_id", pc.RequestID).Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("context builder: knowledge search failed")
		return pc, err
	}

	if results == nil {
		log.Warn().Msg("context builder: knowledge search returned nil results")
		return pc, nil
	}
	results.Results = filterKnowledgeResults(results.Results)

	pc = pc.WithKnowledgeResults(results)

	// 3. Store in cache for future reuse.
	if cb.embedCache != nil {
		cb.embedCache.Set(pc.Prompt, results)
	}

	log.Debug().
		Int("count", len(results.Results)).
		Msg("context builder: knowledge search completed")

	return pc, nil
}

// BuildMemory searches only the memory engine for records relevant to the
// user's prompt across the session layer (limit 5). Results are stored in
// Data.MemoryResults.
//
// To avoid redundant searches, if MAG (Memory-Augmented Generation) has already
// retrieved memories during pre-execution context augmentation and stored them
// in Data.RetrievedMemories, those cached results are reused directly
// instead of querying the memory engine again.
//
// Errors are logged but do not fail the pipeline.
func (cb *ContextBuilder) BuildMemory(ctx context.Context, pc PipelineContext) (PipelineContext, error) {
	pc = pc.WithStage("context_builder:memory")

	// Reuse MAG's cached memory results when available (avoids redundant search).
	if len(pc.Data.RetrievedMemories) > 0 {
		pc = pc.WithMemoryResults(filterMemoryRecords(pc.Data.RetrievedMemories))
		log.Debug().
			Int("count", len(pc.Data.RetrievedMemories)).
			Str("request_id", pc.RequestID).
			Str("query_hash", promptHash(pc.Prompt)).
			Int("query_length", len(pc.Prompt)).
			Msg("context builder: memory results reused from MAG cache (skipping redundant search)")
		return pc, nil
	}

	if cb.memory == nil {
		log.Warn().Msg("context builder: no memory retriever configured, skipping memory enrichment")
		return pc, nil
	}

	opts := MemorySearchOptions{
		Layers:      []string{"session"},
		Limit:       5,
		ReaderAgent: pc.Data.ResolvedAgent,
	}

	results, err := cb.memory.Search(ctx, pc.Prompt, opts)
	if err != nil {
		info := safeError("memory_search_failed", err)
		log.Warn().Str("request_id", pc.RequestID).Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("context builder: memory search failed")
		return pc, err
	}

	pc = pc.WithMemoryResults(filterMemoryRecords(results))

	log.Debug().
		Int("count", len(results)).
		Msg("context builder: memory search completed")

	return pc, nil
}

func promptHash(prompt string) string {
	h := sha256.Sum256([]byte(prompt))
	return fmt.Sprintf("%x", h[:16])
}

// augmentPrompt builds an augmented prompt that combines the retrieved knowledge
// snippets and memory records as contextual preamble before the original prompt.
// The augmented prompt is stored in Data.AugmentedPrompt.
func (cb *ContextBuilder) augmentPrompt(pc PipelineContext) PipelineContext {
	var parts []string

	// Collect knowledge results
	if kr := pc.Data.KnowledgeResults; kr != nil && len(kr.Results) > 0 {
		parts = append(parts, "## Relevant Knowledge")
		for i, r := range kr.Results {
			snippet := r.Snippet
			if snippet == "" {
				snippet = truncateString(r.Content, 300)
			}
			title := r.Title
			if title == "" {
				title = r.DocumentPath
			}
			if title == "" {
				title = fmt.Sprintf("Result %d", i+1)
			}
			parts = append(parts, contenttrust.Envelope(contenttrust.Default(contenttrust.OriginKnowledge, fmt.Sprintf("%s: %s", title, snippet), r.DocumentPath)))
		}
	}

	// Collect memory results
	if memResults := pc.Data.MemoryResults; len(memResults) > 0 {
		parts = append(parts, "## Recent Session Memory")
		for _, m := range memResults {
			snippet := truncateString(m.Content, 300)
			item := contenttrust.FromMetadata(contenttrust.Default(contenttrust.OriginMemory, fmt.Sprintf("[%s] %s", m.Type, snippet), m.ID), m.Metadata)
			if !contenttrust.IsExcluded(item) {
				parts = append(parts, contenttrust.Envelope(item))
			}
		}
	}

	// If we have context, build the augmented prompt
	if len(parts) > 0 {
		parts = append(parts, "", "## User Request", pc.Prompt)
		augmented := strings.Join(parts, "\n")
		pc = pc.WithAugmentedPrompt(augmented)
	} else {
		// No context found; augmented prompt is the original
		pc = pc.WithAugmentedPrompt(pc.Prompt)
	}

	return pc
}

// isGreetingPrompt reports whether the prompt is a greeting — the signal
// that the oracle is waking up (SEMANTIC_AWAKENING_PROTOCOL).
func isGreetingPrompt(prompt string) bool {
	p := strings.ToLower(strings.TrimSpace(prompt))
	if p == "" {
		return false
	}
	greetings := []string{"oi", "olá", "ola", "bom dia", "boa tarde", "boa noite", "hello", "hi", "hey", "e aí", "e ai", "tudo bem", "como vai"}
	for _, g := range greetings {
		if p == g || strings.HasPrefix(p, g+" ") || strings.HasPrefix(p, g+",") || strings.HasPrefix(p, g+"!") || strings.HasPrefix(p, g+"?") {
			return true
		}
	}
	return false
}

func filterMemoryRecords(records []MemoryRecord) []MemoryRecord {
	filtered := make([]MemoryRecord, 0, len(records))
	for _, record := range records {
		item := contenttrust.FromMetadata(contenttrust.Default(contenttrust.OriginMemory, record.Content, record.ID), record.Metadata)
		if !contenttrust.IsExcluded(item) {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func filterKnowledgeResults(results []KnowledgeSearchResult) []KnowledgeSearchResult {
	filtered := make([]KnowledgeSearchResult, 0, len(results))
	for _, result := range results {
		item := contenttrust.Default(contenttrust.OriginKnowledge, result.Content, result.DocumentPath)
		item.PolicyState = contenttrust.PolicyState(strings.ToLower(strings.TrimSpace(result.PolicyState)))
		if item.PolicyState == "" {
			item.PolicyState = contenttrust.StateAllowed
		}
		if !contenttrust.IsExcluded(item) {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

// truncateString safely truncates a string to maxLen, appending "..." if truncated.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
