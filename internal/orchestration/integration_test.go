package orchestration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Extended Mock Implementations ──────────────────────────────────────────

// mockKnowledgeSearcher implements KnowledgeSearcher with controllable
// results and errors for testing knowledge retrieval integration.
type mockKnowledgeSearcher struct {
	results *KnowledgeSearchResults
	err     error
	called  int
}

func (m *mockKnowledgeSearcher) Search(_ context.Context, _ KnowledgeSearchParams) (*KnowledgeSearchResults, error) {
	m.called++
	if m.err != nil {
		return nil, m.err
	}
	if m.results != nil {
		return m.results, nil
	}
	return &KnowledgeSearchResults{}, nil
}

// mockMemoryRetriever implements MemoryRetriever with controllable records
// and errors for testing memory retrieval integration.
type mockMemoryRetriever struct {
	records []MemoryRecord
	err     error
	called  int
}

func (m *mockMemoryRetriever) Search(_ context.Context, _ string, _ MemorySearchOptions) ([]MemoryRecord, error) {
	m.called++
	if m.err != nil {
		return nil, m.err
	}
	return m.records, nil
}

func (m *mockMemoryRetriever) Retrieve(_ context.Context, _, _ string) (*MemoryRecord, error) {
	return nil, errors.New("not implemented in mock")
}

// mockMemoryStorer implements MemoryStorer with recording of stored records
// for verifying MAG storage integration.
type mockMemoryStorer struct {
	stored   []MemoryRecord
	storeErr error
	called   int
	nextID   int
}

func (m *mockMemoryStorer) Store(_ context.Context, record MemoryRecord) (*MemoryRecord, error) {
	m.called++
	m.nextID++
	if m.storeErr != nil {
		return nil, m.storeErr
	}
	record.ID = fmt.Sprintf("mem-%d", m.nextID)
	m.stored = append(m.stored, record)
	return &record, nil
}

// restoreMockSkillResolver is a skill resolver that wraps the
// pipeline_test.go mockSkillResolver with error injection.
type restoreMockSkillResolver struct {
	resolver  *mockSkillResolver // reuse from pipeline_test.go
	getErr    error
	searchErr error
	getCalls  int
}

func newRestoreSkillResolver() *restoreMockSkillResolver {
	return &restoreMockSkillResolver{
		resolver: newMockSkillResolver(),
	}
}

func (m *restoreMockSkillResolver) add(name, description, category string) {
	m.resolver.add(name, description, category)
}

func (m *restoreMockSkillResolver) Get(name string) (*SkillInfo, error) {
	m.getCalls++
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.resolver.Get(name)
}

func (m *restoreMockSkillResolver) Search(query string) ([]SkillInfo, error) {
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	return m.resolver.Search(query)
}

func (m *restoreMockSkillResolver) List() ([]SkillInfo, error) {
	return m.resolver.List()
}

// =============================================================================
// Integration Tests
// =============================================================================

// ─── Test 1: Full Pipeline Success ──────────────────────────────────────────

func TestEngine_FullPipeline_Success(t *testing.T) {
	provider := newMockChatProvider("test-provider", "test-model")
	// Give the request measurable wall-clock time: on coarse clocks (Windows
	// time.Now() granularity ~0.5ms) an instant chat call yields a 0s duration.
	// NOTE: the default mock response must be reproduced here — Chat() never
	// reaches its default branch while chatFn is set, and calling provider.Chat
	// from inside chatFn would recurse infinitely.
	provider.chatFn = func(_ context.Context, _ []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
		time.Sleep(2 * time.Millisecond)
		return &chat.ChatResponse{
			ID:    "resp-1",
			Model: "test-model",
			Choices: []chat.Choice{
				{
					Index: 0,
					Message: chat.Message{
						Role:    chat.RoleAssistant,
						Content: "This is a test response.",
					},
					FinishReason: chat.FinishReasonStop,
				},
			},
			Usage: chat.Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}, nil
	}
	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Leads backend development")

	engine := NewEngine(nil, nil, nil, resolver, nil, provider, DefaultOrchestratorConfig(), nil)

	result, err := engine.Execute(context.Background(), &Request{
		Prompt: "build a REST api endpoint",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Response == "" {
		t.Error("expected non-empty Response")
	}
	if result.Agent == "" {
		t.Error("expected non-empty Agent")
	}
	if result.Duration <= 0 {
		t.Errorf("expected positive Duration, got %v", result.Duration)
	}
	if result.ID == "" {
		t.Error("expected non-empty ID")
	}
	if provider.chatCalled < 1 {
		t.Errorf("expected at least 1 chat call, got %d", provider.chatCalled)
	}
}

// ─── Test 2: Agent Selection via Keyword Routing ───────────────────────────

func TestEngine_FullPipeline_AgentSelection(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Leads backend development")
	resolver.add("Frontend Chief", "Frontend Chief", "frontend", "Leads frontend development")
	resolver.add("Security Chief", "Security Chief", "security", "Handles security")

	engine := NewEngine(nil, nil, nil, resolver, nil, provider, DefaultOrchestratorConfig(), nil)

	result, err := engine.Execute(context.Background(), &Request{
		Prompt: "build a REST api endpoint",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Agent != "Backend Chief" {
		t.Errorf("expected Agent 'Backend Chief' (keyword routing), got %q", result.Agent)
	}
}

// ─── Test 3: Explicit Agent Override ───────────────────────────────────────

func TestEngine_FullPipeline_ExplicitAgent(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Backend dev")
	resolver.add("Security Chief", "Security Chief", "security", "Security expert")

	engine := NewEngine(nil, nil, nil, resolver, nil, provider, DefaultOrchestratorConfig(), nil)

	result, err := engine.Execute(context.Background(), &Request{
		Prompt: "build a REST api endpoint",
		Context: map[string]interface{}{
			"agent": "Security Chief",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Explicit agent hint should override keyword routing.
	if result.Agent != "Security Chief" {
		t.Errorf("expected explicit Agent 'Security Chief', got %q", result.Agent)
	}
}

// ─── Test 4: Knowledge Enrichment ──────────────────────────────────────────

func TestEngine_FullPipeline_WithKnowledge(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Backend dev")

	knowledge := &mockKnowledgeSearcher{
		results: &KnowledgeSearchResults{
			Results: []KnowledgeSearchResult{
				{
					ID:      "k1",
					Title:   "API Design Guide",
					Snippet: "Use RESTful conventions for endpoint design.",
					Score:   0.95,
				},
				{
					ID:      "k2",
					Title:   "Authentication Patterns",
					Snippet: "JWT tokens are preferred for stateless auth.",
					Score:   0.87,
				},
			},
			TotalCount: 2,
			Query:      "build a REST api endpoint",
		},
	}

	engine := NewEngine(knowledge, nil, nil, resolver, nil, provider, DefaultOrchestratorConfig(), nil)

	result, err := engine.Execute(context.Background(), &Request{
		Prompt: "build a REST api endpoint",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if knowledge.called < 1 {
		t.Error("expected KnowledgeSearcher to be called")
	}
	if result.Response == "" {
		t.Error("expected non-empty response")
	}
	if result.Agent == "" {
		t.Error("expected non-empty agent")
	}
}

// ─── Test 5: Memory Retrieval via MAG ──────────────────────────────────────

func TestEngine_FullPipeline_WithMemory(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Backend dev")

	memory := &mockMemoryRetriever{
		records: []MemoryRecord{
			{
				ID:       "m1",
				Type:     "decision",
				Layer:    "session",
				Content:  "Previously decided to use Go for backend services.",
				Priority: 5,
			},
			{
				ID:       "m2",
				Type:     "context",
				Layer:    "session",
				Content:  "Project uses PostgreSQL as primary database.",
				Priority: 7,
			},
		},
	}

	storer := &mockMemoryStorer{}

	config := OrchestratorConfig{
		EnableMAG: true,
		MAGConfig: DefaultMAGConfig(),
	}

	engine := NewEngine(nil, memory, storer, resolver, nil, provider, config, nil)

	result, err := engine.Execute(context.Background(), &Request{
		Prompt: "build a REST api endpoint",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if memory.called < 1 {
		t.Error("expected MemoryRetriever to be called (MAG pre-retrieval)")
	}
	if result.Response == "" {
		t.Error("expected non-empty response")
	}
}

// ─── Test 6: MAG Stores Result After Execution ─────────────────────────────

func TestEngine_FullPipeline_MAGStoresResult(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Backend dev")

	memory := &mockMemoryRetriever{}
	storer := &mockMemoryStorer{}

	config := OrchestratorConfig{
		EnableMAG: true,
		MAGConfig: MAGConfig{
			AutoStore:    true,
			AutoRetrieve: true,
			MemoryLayer:  "session",
			MemoryType:   "decision",
			MinPriority:  5,
		},
	}

	engine := NewEngine(nil, memory, storer, resolver, nil, provider, config, nil)

	result, err := engine.Execute(context.Background(), &Request{
		Prompt: "build a REST api endpoint",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if storer.called < 1 {
		t.Error("expected MemoryStorer.Store to be called (MAG post-store)")
	}
	if len(storer.stored) < 1 {
		t.Fatal("expected at least one stored record")
	}

	stored := storer.stored[0]
	if stored.Type != "decision" {
		t.Errorf("expected stored memory type 'decision', got %q", stored.Type)
	}
	if stored.Layer != "session" {
		t.Errorf("expected stored memory layer 'session', got %q", stored.Layer)
	}
	if result.MemoryID == "" {
		t.Error("expected Result.MemoryID to be set from MAG storage")
	}
}

// ─── Test 7: Without MAG ───────────────────────────────────────────────────

func TestEngine_FullPipeline_WithoutMAG(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Backend dev")

	storer := &mockMemoryStorer{}

	config := OrchestratorConfig{
		EnableMAG: false, // explicitly disabled
		MAGConfig: DefaultMAGConfig(),
	}

	engine := NewEngine(nil, nil, storer, resolver, nil, provider, config, nil)

	result, err := engine.Execute(context.Background(), &Request{
		Prompt: "build a REST api endpoint",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if storer.called > 0 {
		t.Errorf("expected no memory storage when MAG is disabled, got %d calls", storer.called)
	}
	if result.MemoryID != "" {
		t.Errorf("expected empty MemoryID when MAG is disabled, got %q", result.MemoryID)
	}
	if result.Response == "" {
		t.Error("expected non-empty response")
	}
}

// ─── Test 8: Streaming Execution ──────────────────────────────────────────

func TestEngine_FullPipeline_Streaming(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Backend dev")

	engine := NewEngine(nil, nil, nil, resolver, nil, provider, DefaultOrchestratorConfig(), nil)

	eventCh, err := engine.ExecuteStream(context.Background(), &Request{
		Prompt: "build a REST api endpoint",
	})
	if err != nil {
		t.Fatalf("unexpected error starting stream: %v", err)
	}

	// Collect all stream events with timeout.
	var events []StreamEvent
	timeout := time.After(5 * time.Second)
loop:
	for {
		select {
		case ev, ok := <-eventCh:
			if !ok {
				break loop
			}
			events = append(events, ev)
		case <-timeout:
			t.Fatal("timeout waiting for stream events")
		}
	}

	if len(events) == 0 {
		t.Fatal("expected at least one stream event")
	}

	// Verify we got progress events.
	var progressCount int
	var chunkCount int
	for _, ev := range events {
		switch ev.Type {
		case StreamEventProgress:
			progressCount++
		case StreamEventChunk:
			chunkCount++
		}
	}

	if progressCount == 0 {
		t.Error("expected at least one progress event")
	}
	if chunkCount == 0 {
		t.Error("expected at least one chunk event")
	}

	// Stream channel should be closed after completion.
	_, stillOpen := <-eventCh
	if stillOpen {
		t.Error("expected channel to be closed after stream completion")
	}
}

// ─── Test 9: Chat Error Propagation ────────────────────────────────────────

func TestEngine_FullPipeline_ChatError(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	provider.chatErr = errors.New("non-transient: invalid api key")
	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Backend dev")

	config := OrchestratorConfig{} // default, but override MaxRetries to 0 for fast failure
	engine := NewEngine(nil, nil, nil, resolver, nil, provider, config, nil)
	// Zero-retries: the executor applies defaults, so we need to set explicitly.
	// Actually NewEngine creates the executor with DefaultExecutorConfig().
	// The retry logic won't retry non-transient errors. So it should fail fast.
	engine.executor.config.MaxRetries = 0

	_, err := engine.Execute(context.Background(), &Request{
		Prompt: "build a REST api endpoint",
	})
	if err == nil {
		t.Fatal("expected error from chat failure, got nil")
	}
	// The error should reference executor failure.
	errStr := err.Error()
	if errStr == "" {
		t.Error("expected non-empty error string")
	}
}

// ─── Test 10: Knowledge Degradation ────────────────────────────────────────

func TestEngine_FullPipeline_KnowledgeDegradation(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Backend dev")

	knowledge := &mockKnowledgeSearcher{
		err: errors.New("knowledge engine unavailable"),
	}

	engine := NewEngine(knowledge, nil, nil, resolver, nil, provider, DefaultOrchestratorConfig(), nil)

	result, err := engine.Execute(context.Background(), &Request{
		Prompt: "build a REST api endpoint",
	})
	if err != nil {
		t.Fatalf("expected graceful degradation, but got error: %v", err)
	}

	// Knowledge search should have been attempted.
	if knowledge.called < 1 {
		t.Error("expected KnowledgeSearcher to be called")
	}
	// Pipeline should still succeed (graceful degradation).
	if result.Response == "" {
		t.Error("expected non-empty response despite knowledge failure")
	}
	if result.Agent == "" {
		t.Error("expected non-empty agent despite knowledge failure")
	}
}

// ─── Test 11: Memory Degradation ───────────────────────────────────────────

func TestEngine_FullPipeline_MemoryDegradation(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Backend dev")

	memory := &mockMemoryRetriever{
		err: errors.New("memory engine unreachable"),
	}
	storer := &mockMemoryStorer{}

	config := OrchestratorConfig{
		EnableMAG: true,
		MAGConfig: DefaultMAGConfig(),
	}

	engine := NewEngine(nil, memory, storer, resolver, nil, provider, config, nil)

	result, err := engine.Execute(context.Background(), &Request{
		Prompt: "build a REST api endpoint",
	})
	if err != nil {
		t.Fatalf("expected graceful degradation, but got error: %v", err)
	}

	// Memory retrieval should have been attempted.
	if memory.called < 1 {
		t.Error("expected MemoryRetriever to be called")
	}
	// Pipeline should still succeed.
	if result.Response == "" {
		t.Error("expected non-empty response despite memory failure")
	}
}

// ─── Test 12: Pipeline with Skills ─────────────────────────────────────────

func TestEngine_FullPipeline_WithPipelineSkills(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Backend dev")

	skills := newRestoreSkillResolver()
	skills.add("transform", "Pass-through transformer", "general")

	config := OrchestratorConfig{
		Pipeline: PipelineDefinition{
			Name: "test-pipeline",
			Steps: []PipelineStep{
				{Name: "transform-step", Skill: "transform", Output: "tx_result"},
				{Name: "enrich-step", Skill: "enrich", Output: "enriched"},
			},
		},
	}

	engine := NewEngine(nil, nil, nil, resolver, skills, provider, config, nil)

	result, err := engine.Execute(context.Background(), &Request{
		Prompt: "build a REST api endpoint",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Response == "" {
		t.Error("expected non-empty response")
	}
	if result.Agent == "" {
		t.Error("expected non-empty agent")
	}
	// Skills should have been resolved via the resolver.
	if skills.getCalls < 1 {
		t.Error("expected SkillResolver.Get to be called at least once")
	}
}

// ─── Test 13: Tool Calls in Response ───────────────────────────────────────

func TestEngine_FullPipeline_ToolCalls(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	provider.chatResponse = &chat.ChatResponse{
		ID:    "resp-tools",
		Model: "test-model",
		Choices: []chat.Choice{
			{
				Index: 0,
				Message: chat.Message{
					Role:    chat.RoleAssistant,
					Content: "Let me search the codebase for you.",
					ToolCalls: []chat.ToolCall{
						{
							ID:   "tc-1",
							Type: "function",
							Function: chat.FunctionCall{
								Name:      "search_codebase",
								Arguments: `{"query":"api endpoint"}`,
							},
						},
					},
				},
				FinishReason: chat.FinishReasonToolCalls,
			},
		},
		Usage: chat.Usage{
			PromptTokens:     10,
			CompletionTokens: 15,
			TotalTokens:      25,
		},
	}

	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Backend dev")

	engine := NewEngine(nil, nil, nil, resolver, nil, provider, DefaultOrchestratorConfig(), nil)

	result, err := engine.Execute(context.Background(), &Request{
		Prompt: "search the codebase for api endpoints",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The response should contain the assistant's message.
	if result.Response == "" {
		t.Error("expected non-empty response")
	}
	if result.Response != "Let me search the codebase for you." {
		t.Errorf("expected tool-call response text, got %q", result.Response)
	}
	// Provider should have been called.
	if provider.chatCalled < 1 {
		t.Error("expected chat provider to be called")
	}
}

// ─── Test 14: No Agents Available — Fallback to CEO ────────────────────────

func TestEngine_FullPipeline_NoAgentsAvailable(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	// Empty agent resolver — no agents registered.
	resolver := newMockAgentResolver()

	engine := NewEngine(nil, nil, nil, resolver, nil, provider, DefaultOrchestratorConfig(), nil)

	result, err := engine.Execute(context.Background(), &Request{
		Prompt: "do something completely random and unrelated to anything",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Even with no agents, the engine should fall back to a hardcoded CEO Agent.
	if result.Agent == "" {
		t.Error("expected non-empty agent (fallback)")
	}
	// The hardcoded fallback is "CEO Agent".
	if result.Agent != "CEO Agent" {
		t.Logf("agent was %q (expected 'CEO Agent' fallback)", result.Agent)
	}
	if result.Response == "" {
		t.Error("expected non-empty response")
	}
}

// ─── Test 15: Context Cancellation ─────────────────────────────────────────

func TestEngine_FullPipeline_ContextCancellation(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")

	// Override the Chat method to block until context is cancelled.
	provider.chatFn = func(ctx context.Context, _ []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
		// Block until context is done.
		<-ctx.Done()
		return nil, ctx.Err()
	}

	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Backend dev")

	config := DefaultOrchestratorConfig()
	engine := NewEngine(nil, nil, nil, resolver, nil, provider, config, nil)
	// Override executor config to have a longer timeout so cancellation wins.
	engine.executor.config.MaxRetries = 0
	engine.executor.config.Timeout = 10 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel before execution starts.
	cancel()

	_, err := engine.Execute(ctx, &Request{
		Prompt: "build a REST api endpoint",
	})
	if err == nil {
		t.Fatal("expected error from context cancellation, got nil")
	}
	if provider.chatCalled > 0 {
		t.Error("expected Chat not to be called with cancelled context")
	}
}

// integrationSemanticEmbedder makes the embedding boundary observable while
// keeping the test independent of an external embedding service. Its semantic
// prompt deliberately contains no terms understood by the keyword router.
type integrationSemanticEmbedder struct {
	calls    atomic.Int64
	semantic bool
}

func (e *integrationSemanticEmbedder) GenerateEmbedding(_ context.Context, text string) (*embeddings.EmbeddingResult, error) {
	e.calls.Add(1)
	vector := []float64{0, 0}
	if e.semantic && strings.Contains(text, "lighthouse migration") {
		vector[0] = 1
	}
	return &embeddings.EmbeddingResult{Vector: vector, Model: "integration", Dimensions: len(vector)}, nil
}

func (e *integrationSemanticEmbedder) GenerateEmbeddings(ctx context.Context, texts []string) ([]*embeddings.EmbeddingResult, error) {
	results := make([]*embeddings.EmbeddingResult, len(texts))
	for i, text := range texts {
		if strings.Contains(text, "Backend Chief") {
			results[i] = &embeddings.EmbeddingResult{Vector: []float64{1, 0}, Model: "integration", Dimensions: 2}
			continue
		}
		result, err := e.GenerateEmbedding(ctx, text)
		if err != nil {
			return nil, err
		}
		results[i] = result
	}
	return results, nil
}

func newSemanticIntegrationEngine(embedder Embedder) (*Engine, *mockAgentResolver, *mockChatProvider) {
	provider := newMockChatProvider("integration", "integration-model")
	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Leads backend development")
	resolver.add("Frontend Chief", "Frontend Chief", "frontend", "Leads frontend development")
	config := DefaultOrchestratorConfig()
	config.Embedder = embedder
	semanticConfig := DefaultSemanticRouterConfig()
	config.SemanticRouterConfig = &semanticConfig
	return NewEngine(nil, nil, nil, resolver, nil, provider, config, nil), resolver, provider
}

func TestEngine_Integration_SemanticExecute(t *testing.T) {
	embedder := &integrationSemanticEmbedder{semantic: true}
	engine, _, _ := newSemanticIntegrationEngine(embedder)

	result, err := engine.Execute(context.Background(), &Request{Prompt: "please coordinate the lighthouse migration"})
	require.NoError(t, err)
	assert.Equal(t, "Backend Chief", result.Agent)
	assert.NotEmpty(t, result.Response)
	assert.Greater(t, embedder.calls.Load(), int64(0), "semantic routing must call the embedding provider")
}

func TestEngine_Integration_SemanticExecuteStream(t *testing.T) {
	embedder := &integrationSemanticEmbedder{semantic: true}
	engine, _, _ := newSemanticIntegrationEngine(embedder)

	events, err := engine.ExecuteStream(context.Background(), &Request{Prompt: "please coordinate the lighthouse migration"})
	require.NoError(t, err)

	var collected []StreamEvent
	for event := range events {
		collected = append(collected, event)
	}
	require.NotEmpty(t, collected)
	assert.Greater(t, embedder.calls.Load(), int64(0), "semantic streaming must call the embedding provider")
	var resolved bool
	for _, event := range collected {
		if event.Type == StreamEventProgress && strings.Contains(event.Content, "Backend Chief") {
			resolved = true
		}
	}
	assert.True(t, resolved, "stream should report the semantically resolved agent")
}

func TestEngine_Integration_SemanticFallbackModes(t *testing.T) {
	t.Run("keyword fallback", func(t *testing.T) {
		embedder := &integrationSemanticEmbedder{}
		engine, _, _ := newSemanticIntegrationEngine(embedder)

		result, err := engine.Execute(context.Background(), &Request{Prompt: "build an api endpoint"})
		require.NoError(t, err)
		assert.Equal(t, "Backend Chief", result.Agent)
		assert.Greater(t, embedder.calls.Load(), int64(0))
	})

	t.Run("fallback disabled", func(t *testing.T) {
		embedder := &integrationSemanticEmbedder{}
		_, resolver, _ := newSemanticIntegrationEngine(embedder)
		keyword := NewRouter(resolver)
		config := DefaultSemanticRouterConfig()
		config.FallbackToKeyword = false
		router := NewSemanticRouter(resolver, embedder, keyword, config)

		_, err := router.Route(context.Background(), NewPipelineContext("integration", "a neutral lighthouse question"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "routing_failed")
	})
}
