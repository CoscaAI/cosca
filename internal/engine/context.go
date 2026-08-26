package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/contenttrust"
	"github.com/CoscaAI/cosca/internal/modlink"
)

// ─── Default Context Limits ───────────────────────────────────────────────────

const (
	// DefaultMaxTokens is the fallback context window size when none is provided.
	DefaultMaxTokens = 128000

	// MaxConversationTurns is the maximum number of message pairs kept in context.
	MaxConversationTurns = 50
)

// ─── Context Builder ───────────────────────────────────────────────────────────

// ContextBuilder assembles the context for LLM calls following the Context Pipeline
// defined in next-gen-cli-design.md §9:
//
//  1. System Identity — agent's system prompt defining identity and behavior
//  2. Project Context — AGENTS.md chain of inheritance (stub, reserved for future)
//  3. MAG Memory — relevant semantic memories from previous sessions
//  4. Knowledge — relevant documentation from the knowledge base
//  5. Skills — progressive disclosure (names only, full content on demand)
//  6. Tools — available tool definitions and schemas
//  7. Conversation History — last N turns of the conversation
//  8. User Request — the current prompt from the user
type ContextBuilder struct {
	agentRegistry *AgentRegistry
	maxTokens     int
	skills        []string // available skill names
	projectDir    string   // project root; AGENTS.md chain is loaded from here
}

// NewContextBuilder creates a ContextBuilder with a reference to the agent registry
// and the model's maximum context window size in tokens. If maxTokens is <= 0,
// DefaultMaxTokens (128K) is used.
func NewContextBuilder(registry *AgentRegistry, maxTokens int) *ContextBuilder {
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	return &ContextBuilder{
		agentRegistry: registry,
		maxTokens:     maxTokens,
	}
}

// SetSkills sets the list of available skill names for inclusion in the
// system prompt. Only skill names are disclosed; full skill content is
// loaded on demand by the agent.
func (b *ContextBuilder) SetSkills(skills []string) {
	b.skills = skills
}

// WithProjectDir sets the project root directory from which the AGENTS.md
// inheritance chain is loaded during Build. When empty (or when this is never
// called), the current working directory is used as the starting point.
func (b *ContextBuilder) WithProjectDir(dir string) *ContextBuilder {
	b.projectDir = dir
	return b
}

// Build assembles the full context for a single LLM call. It merges the agent's
// system prompt with project context, memories, knowledge, tools, conversation
// history, and the user's current request into a BuiltContext ready for provider
// dispatch.
//
// Parameters:
//   - ctx: Context for cancellation and deadlines.
//   - agentName: The registered agent name (e.g. "cosca-architecture").
//   - messages: The full conversation history, including the latest user message.
//   - tools: The set of tools available for this turn.
//   - memories: Pre-retrieved relevant memory strings (from MAG engine).
//   - knowledge: Pre-retrieved relevant knowledge strings (from Knowledge engine).
func (b *ContextBuilder) Build(
	ctx context.Context,
	agentName string,
	messages []chat.Message,
	tools []chat.Tool,
	memories []string,
	knowledge []string,
) *BuiltContext {
	// ── Step 1-5: Assemble system prompt sections ──────────────────────────

	var sections []string

	// 1. System Identity — agent's system prompt.
	agent := b.agentRegistry.Get(agentName)
	if agent != nil && agent.SystemPrompt != "" {
		sections = append(sections, agent.SystemPrompt)
	}

	// 2. Project Context — AGENTS.md chain of inheritance (root → subdir).
	// Each entry is wrapped in a content trust envelope to prevent semantic
	// injection: untrusted workspace content must never appear as raw system
	// prompt text.
	startDir := b.projectDir
	if startDir == "" {
		if cwd, err := os.Getwd(); err == nil {
			startDir = cwd
		}
	}
	if chain := loadAgentsMDChain(startDir); len(chain) > 0 {
		chainSections := make([]string, 0, len(chain))
		for _, entry := range chain {
			wrapped := contenttrust.Envelope(contenttrust.Default(
				contenttrust.OriginKnowledge, entry.Content, "agents.md:"+entry.Path))
			chainSections = append(chainSections, wrapped)
		}
		sections = append(sections, strings.Join(chainSections, "\n\n"))
	}

	// 3. MAG Memory — relevant semantic memories.
	if len(memories) > 0 {
		sections = append(sections, "=== RELEVANT MEMORIES ===")
		fragments := make([]string, 0, len(memories))
		for i, memory := range memories {
			fragments = append(fragments, contenttrust.Envelope(contenttrust.Default(contenttrust.OriginMemory, memory, fmt.Sprintf("memory:%d", i))))
		}
		sections = append(sections, strings.Join(fragments, "\n"))
	}

	// 4. Knowledge — relevant documentation.
	if len(knowledge) > 0 {
		sections = append(sections, "=== RELEVANT KNOWLEDGE ===")
		fragments := make([]string, 0, len(knowledge))
		for i, document := range knowledge {
			fragments = append(fragments, contenttrust.Envelope(contenttrust.Default(contenttrust.OriginKnowledge, document, fmt.Sprintf("knowledge:%d", i))))
		}
		sections = append(sections, strings.Join(fragments, "\n"))
	}

	// 5. Skills — progressive disclosure (names only, full content on demand).
	if len(b.skills) > 0 {
		sections = append(sections, "=== AVAILABLE SKILLS ===")
		sections = append(sections, strings.Join(b.skills, "\n"))
	}

	fullSystemPrompt := strings.Join(sections, "\n\n")

	// ── Step 6: Convert chat.Tool instances to ToolDefinitions ─────────────

	toolDefs := make([]chat.ToolDefinition, 0, len(tools))
	for _, tool := range tools {
		var params map[string]any
		if schema := tool.Schema(); len(schema) > 0 {
			// Silently ignore schema parse errors; parameters will be nil.
			_ = json.Unmarshal(schema, &params)
		}
		toolDefs = append(toolDefs, chat.ToolDefinition{
			Type: "function",
			Function: chat.FunctionDef{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  params,
			},
		})
	}

	// ── Token estimate ─────────────────────────────────────────────────────

	totalText := fullSystemPrompt
	for _, msg := range messages {
		totalText += msg.Content
		for _, part := range msg.ContentParts {
			totalText += part.Text
		}
	}
	tokenEstimate := b.EstimateTokens(totalText)

	// ── Assemble result ────────────────────────────────────────────────────

	return &BuiltContext{
		SystemPrompt:    fullSystemPrompt,
		Messages:        messages,
		Tools:           tools,
		ToolDefinitions: toolDefs,
		TokenEstimate:   tokenEstimate,
		ContextLimit:    b.maxTokens,
		UsagePct:        float64(tokenEstimate) / float64(b.maxTokens) * 100,
	}
}

// knowledgeNoRouteLine é a linha curta injetada no SystemPrompt quando a busca
// de conhecimento está em estado NO_ROUTE (modo modular). Ela diz ao agente,
// com honestidade, que não há espaço semântico confiável — e nunca fabrica um.
var knowledgeNoRouteLine = "=== KNOWLEDGE SCOPE === NO_ROUTE (sem espaço semântico confiável para esta consulta)"

// applyKnowledgeNoRoute marcay o BuiltContext com o estado NO_ROUTE e anexa a
// linha de escopo ao SystemPrompt. É idempotente: chamar de novo não duplica a
// linha. Quando `noRoute` é false, o contexto permanece intacto (legacy).
func applyKnowledgeNoRoute(built *BuiltContext, noRoute bool, scope *modlink.SearchScope) *BuiltContext {
	if built == nil || !noRoute {
		return built
	}
	built.KnowledgeNoRoute = true
	if scope != nil {
		built.ScopeInfo = scope
	}
	if !strings.Contains(built.SystemPrompt, knowledgeNoRouteLine) {
		if strings.TrimSpace(built.SystemPrompt) != "" {
			built.SystemPrompt += "\n\n"
		}
		built.SystemPrompt += knowledgeNoRouteLine
	}
	return built
}

// EstimateTokens returns a rough token estimate for the given text.
// Uses the rule-of-thumb of ~4 characters per token for English text, plus a
// small overhead factor for message formatting and special tokens.
//
// This is intentionally simplistic — accurate tokenization requires provider-
// specific tokenizers (tiktoken, claude tokenizer, etc.) that may be swapped
// in later as optional enhancements.
func (b *ContextBuilder) EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	// Base estimate: 4 chars per token (standard heuristic).
	tokens := len(text) / 4
	// Overhead: ~1% for message format wrappers and special tokens.
	tokens += len(text) / 100
	if tokens < 1 {
		tokens = 1
	}
	return tokens
}

// agentsMDEntry pairs an AGENTS.md file path with its raw contents.
type agentsMDEntry struct {
	Path    string
	Content string
}

// loadAgentsMDChain walks from startDir up to the filesystem root, collecting
// every AGENTS.md file found along the way. The returned entries are ordered
// from the outermost (root) AGENTS.md down to the innermost (startDir) one,
// forming the inheritance chain. Directories without an AGENTS.md are skipped
// silently; a missing startDir yields an empty chain.
func loadAgentsMDChain(startDir string) []agentsMDEntry {
	var chain []agentsMDEntry
	dir := startDir
	for dir != "" {
		path := filepath.Join(dir, "AGENTS.md")
		if data, err := os.ReadFile(path); err == nil {
			chain = append(chain, agentsMDEntry{Path: path, Content: string(data)})
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Reverse so the root AGENTS.md comes first in the inheritance chain.
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}
