// Package chat provides the LLM Chat Provider interface, types, and registry
// for the Cosca AI Orchestration Engine. It defines the abstraction for chat
// completion via different LLM backends with fallback support.
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ─── Role ───────────────────────────────────────────────────────────────────

// Role represents the role of a message participant in a chat conversation.
type Role string

// Predefined chat message roles.
const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ─── Finish Reason ──────────────────────────────────────────────────────────

// FinishReason describes why a chat completion ended.
type FinishReason string

// Predefined finish reasons for chat completions.
const (
	FinishReasonStop          FinishReason = "stop"
	FinishReasonToolCalls     FinishReason = "tool_calls"
	FinishReasonLength        FinishReason = "length"
	FinishReasonContentFilter FinishReason = "content_filter"
)

// ─── Content Part (Multi-Modal) ──────────────────────────────────────────────

// ContentPart represents a part of a multimodal message content.
// It supports text and image_url types for vision models.
type ContentPart struct {
	Type     string    `json:"type"`                // "text" or "image_url"
	Text     string    `json:"text,omitempty"`      // for type "text"
	ImageURL *ImageURL `json:"image_url,omitempty"` // for type "image_url"
}

// ImageURL represents an image reference in a multimodal message.
type ImageURL struct {
	URL    string `json:"url"`              // URL or base64 data URI
	Detail string `json:"detail,omitempty"` // "low", "high", "auto" (OpenAI)
}

// ─── Message ────────────────────────────────────────────────────────────────

// Message represents a single message in a chat conversation.
// For multimodal content (text + images), use ContentParts instead of Content.
// ContentParts takes precedence over Content when mapping to provider APIs.
type Message struct {
	Role         Role          `json:"role"`
	Content      string        `json:"content,omitempty"`       // simple text (backward compat)
	ContentParts []ContentPart `json:"content_parts,omitempty"` // multimodal content
	Name         string        `json:"name,omitempty"`
	ToolCalls    []ToolCall    `json:"tool_calls,omitempty"`
	ToolCallID   string        `json:"tool_call_id,omitempty"`
}

// HasImage returns true if the message contains image content in its ContentParts.
func (m *Message) HasImage() bool {
	for _, part := range m.ContentParts {
		if part.Type == "image_url" && part.ImageURL != nil && part.ImageURL.URL != "" {
			return true
		}
	}
	return false
}

// AddImage adds an image URL to the message's ContentParts.
// If ContentParts is nil, it initializes it. If the message has simple Content text,
// it is preserved as the first text part.
func (m *Message) AddImage(url, detail string) {
	// If Content has text but ContentParts hasn't been initialized yet,
	// migrate the text content to ContentParts.
	if m.Content != "" && len(m.ContentParts) == 0 {
		m.ContentParts = append(m.ContentParts, ContentPart{
			Type: "text",
			Text: m.Content,
		})
		m.Content = ""
	}
	m.ContentParts = append(m.ContentParts, ContentPart{
		Type: "image_url",
		ImageURL: &ImageURL{
			URL:    url,
			Detail: detail,
		},
	})
}

// AddText adds a text part to the message's ContentParts.
// If ContentParts is nil, it initializes it. If the message has simple Content text,
// it is preserved as the first text part.
func (m *Message) AddText(text string) {
	if m.Content != "" && len(m.ContentParts) == 0 {
		m.ContentParts = append(m.ContentParts, ContentPart{
			Type: "text",
			Text: m.Content,
		})
		m.Content = ""
	}
	m.ContentParts = append(m.ContentParts, ContentPart{
		Type: "text",
		Text: text,
	})
}

// ─── Tool Call ──────────────────────────────────────────────────────────────

// ToolCall represents a request from the model to invoke a specific tool.
type ToolCall struct {
	// Index identifies the tool call position within a streaming delta array.
	// This is used by the ResponseParser to correlate partial tool call deltas
	// emitted across multiple stream chunks from the LLM provider.
	Index    int          `json:"index,omitempty"`
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall describes the function name and its JSON-encoded arguments.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ─── Tool Definition ────────────────────────────────────────────────────────

// ToolDefinition describes a tool available for the model to call.
type ToolDefinition struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

// FunctionDef defines the signature of a callable function.
type FunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// ─── Chat Options ───────────────────────────────────────────────────────────

// ChatOptions holds optional parameters for a chat completion request.
//
//nolint:revive // Stutter name preserved for API compatibility — used as chat.ChatOptions externally.
type ChatOptions struct {
	// Temperature controls randomness (0.0–2.0). Higher values produce more
	// creative outputs. Default is 0.7.
	Temperature float64 `json:"temperature,omitempty"`

	// MaxTokens is the maximum number of tokens to generate. 0 means the
	// model default limit is used.
	MaxTokens int `json:"max_tokens,omitempty"`

	// NumCtx é o tamanho da janela de contexto (num_ctx) do modelo. 0 = o
	// provider usa o default. Quando > 0, o provider envia options.num_ctx no
	// request (Ollama). Sem isto, o Ollama roda os modelos com context_length
	// baixo (4096) e TRUNCA prompts grandes da esteira — fazendo o modelo
	// perder a instrução de tool-call e responder em prosa. Fix do "pedreiro
	// não constrói" (2026-09-03). Padrão: 32768 (ver DefaultChatOptions).
	NumCtx int `json:"num_ctx,omitempty"`

	// TopP is the nucleus sampling probability threshold (0.0–1.0).
	TopP float64 `json:"top_p,omitempty"`

	// Stop is a list of sequences where the model will stop generating.
	Stop []string `json:"stop,omitempty"`

	// Tools defines the set of tools available during the completion.
	Tools []ToolDefinition `json:"tools,omitempty"`

	// ToolChoice forces a specific tool call behavior.
	ToolChoice string `json:"tool_choice,omitempty"`

	// Stream enables server-sent events streaming.
	Stream bool `json:"stream,omitempty"`

	// Seed sets a deterministic seed for reproducible outputs when supported.
	Seed *int `json:"seed,omitempty"`

	// Extra carries arbitrary provider-specific options.
	Extra map[string]any `json:"extra,omitempty"`
}

// DefaultChatOptions returns sensible default chat options.
func DefaultChatOptions() ChatOptions {
	return ChatOptions{
		Temperature: 0.7,
		MaxTokens:   0, // model default
		TopP:        1.0,
		// Janela de contexto do Ollama. O contexto real da esteira
		// (conhecimento+memoria+instrucoes+tools+multiplos tasks) passa de 100K
		// tokens (medido em execucao). Sem janela adequada, o Ollama trunca e
		// o modelo perde a secao de tools -> responde em prosa. 131072 (128K)
		// da folga; Qwen2.5/Qwen3-coder suportam >128K nativamente.
		NumCtx: 131072,
	}
}

// ─── Chat Response ──────────────────────────────────────────────────────────

// ChatResponse represents the full response from a chat completion.
//
//nolint:revive // Stutter name preserved for API compatibility — used as chat.ChatResponse externally.
type ChatResponse struct {
	ID        string    `json:"id"`
	Model     string    `json:"model"`
	Choices   []Choice  `json:"choices"`
	Usage     Usage     `json:"usage,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// Choice represents a single completion choice returned by the model.
type Choice struct {
	Index        int          `json:"index"`
	Message      Message      `json:"message"`
	FinishReason FinishReason `json:"finish_reason,omitempty"`
}

// Usage holds token usage statistics for a completion.
//
// ADR-031 Fase 0.1: além do total, decompomos o uso para medir a eficiência
// real — tokens de CACHE (reuso de prompt) e de REASONING (pensamento) são
// o que distingue o "útil" do "gasto". Preenchidos quando o provider expõe o
// detalhe (OpenAI-compat: prompt_tokens_details.cached_tokens e
// completion_tokens_details.reasoning_tokens).
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`

	// Decomposição (Fase 0.1) — 0 quando o provider não expõe o detalhe.
	CachedTokens    int `json:"cached_tokens,omitempty"`    // reuso de prompt (cache)
	ReasoningTokens int `json:"reasoning_tokens,omitempty"` // tokens de pensamento (thinking)
}

// RawUsage captura os detalhes de decomposição do payload OpenAI-compatível:
// prompt_tokens_details.cached_tokens e completion_tokens_details.reasoning_tokens
// (OpenAI nested) e prompt_cache_hit_tokens/prompt_cache_miss_tokens (DeepSeek
// flat). Embed da Usage para preservar os campos base na desserialização.
type RawUsage struct {
	Usage
	PromptTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`

	// DeepSeek (OpenAI-compat): context caching AUTOMÁTICO por prefixo no servidor.
	// prompt_tokens == prompt_cache_hit_tokens + prompt_cache_miss_tokens.
	// Ausente em OpenAI/Anthropic/Ollama → 0 (decode seguro).
	PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"`
}

// UnmarshalJSON promove os detalhes de decomposição do payload bruto para a
// Usage embutida (ADR-031 Fase 1). Ordem de precedência documentada — NUNCA soma
// (evita dupla contagem quando o provider envia mais de uma fonte):
//
//	CachedTokens = prompt_tokens_details.cached_tokens (OpenAI nested, 1ª)
//	            → prompt_cache_hit_tokens (DeepSeek flat, 2ª)
//	            → cached_tokens (flat legado, 3ª — já decodificado no embutido)
//	            → 0 (nenhum detalhe presente)
//
// O valor promovido é clampado a PromptTokens (defensivo: hit > prompt é
// anomalia do provider, nunca cache maior que o prompt). ReasoningTokens segue
// a mesma regra: completion_tokens_details.reasoning_tokens (nested) tem
// precedência; quando ausente, mantém o flat reasoning_tokens decodificado.
// prompt_cache_miss_tokens fica APENAS no campo cru — não entra em chat.Usage
// (a miss é o complemento informativo de CachedAndEffective, não um total novo).
func (u *RawUsage) UnmarshalJSON(data []byte) error {
	// Alias sem métodos: o tipo derivado não herda UnmarshalJSON, evitando a
	// recursão infinita de decodificar RawUsage dentro de RawUsage.
	type rawUsageAlias RawUsage
	var alias rawUsageAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}

	// Promoção do cache: primeira fonte não-zero na ordem documentada.
	cached := alias.PromptTokensDetails.CachedTokens
	if cached == 0 && alias.PromptCacheHitTokens > 0 {
		cached = alias.PromptCacheHitTokens
	}
	if cached == 0 {
		cached = alias.CachedTokens // flat legado já decodificado
	}
	if cached > alias.PromptTokens {
		cached = alias.PromptTokens
	}
	alias.CachedTokens = cached

	// Promoção do reasoning: nested primeiro; senão mantém o flat decodificado.
	if rt := alias.CompletionTokensDetails.ReasoningTokens; rt > 0 {
		alias.ReasoningTokens = rt
	}

	*u = RawUsage(alias)
	return nil
}

// CachedAndEffective devolve (cached, effective) onde effective = prompt - cached
// (os tokens efetivamente processados, fora do reuso de cache).
//
//nolint:unparam // semantic clarity: caller uses both dimensions.
func (u Usage) CachedAndEffective() (cached, effective int) {
	cached = u.CachedTokens
	if cached > u.PromptTokens {
		cached = u.PromptTokens
	}
	effective = u.PromptTokens - cached
	return cached, effective
}

// ReasoningTokens devolve os tokens de pensamento (0 se não informado).
func (u Usage) ReasoningTokensCount() int {
	return u.ReasoningTokens
}

// ─── Chat Stream ────────────────────────────────────────────────────────────

// ChatStream is an iterator over streaming chat completion chunks.
//
//nolint:revive // Stutter name preserved for API compatibility — used as chat.ChatStream externally.
type ChatStream interface {
	// Recv returns the next chunk from the stream. When the stream ends,
	// Recv returns a chunk with a non-empty FinishReason and no error.
	Recv() (*ChatStreamChunk, error)

	// Close terminates the stream and releases resources.
	Close() error
}

// ChatStreamChunk represents a single delta chunk in a streaming response.
//
//nolint:revive // Stutter name preserved for API compatibility — used as chat.ChatStreamChunk externally.
type ChatStreamChunk struct {
	ID      string         `json:"id"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
	Usage   *Usage         `json:"usage,omitempty"`
}

// StreamChoice represents a single delta chunk choice.
type StreamChoice struct {
	Index        int          `json:"index"`
	Delta        Message      `json:"delta"` // Content, ToolCalls, Role — partial
	FinishReason FinishReason `json:"finish_reason,omitempty"`
}

// ─── Chat Provider Interface ────────────────────────────────────────────────

// ChatProvider defines the interface for LLM chat completion providers.
// Implementations back the AI Orchestration Engine with support for both
// synchronous and streaming completions.
//
//nolint:revive // Stutter name preserved for API compatibility — used as chat.ChatProvider externally.
type ChatProvider interface {
	// Chat sends a synchronous chat completion request and returns the full response.
	Chat(ctx context.Context, messages []Message, opts ChatOptions) (*ChatResponse, error)

	// ChatStream initiates a streaming chat completion request.
	ChatStream(ctx context.Context, messages []Message, opts ChatOptions) (ChatStream, error)

	// Model returns the name of the model backing this provider.
	Model() string

	// Name returns the provider's logical name (e.g. "openai", "anthropic").
	Name() string

	// Close releases any resources held by the provider.
	Close() error
}

// ─── Provider Error ─────────────────────────────────────────────────────────

// ProviderError wraps an error with the provider name for better diagnostics.
type ProviderError struct {
	Provider string
	Err      error
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
	return fmt.Sprintf("provider %q: %s", e.Provider, e.Err.Error())
}

// Unwrap allows errors.Is / errors.As to inspect the underlying error.
func (e *ProviderError) Unwrap() error {
	return e.Err
}

// NewProviderError creates a new ProviderError.
func NewProviderError(provider string, err error) error {
	return &ProviderError{Provider: provider, Err: err}
}
