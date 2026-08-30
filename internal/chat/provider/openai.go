// Package provider implements chat.Provider adapters for various LLM backends.
// Each adapter speaks the provider's native HTTP API — no external SDKs.
package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/CoscaAI/cosca/internal/chat"
)

// ─── Default endpoints ───────────────────────────────────────────────────────

const (
	defaultOpenAIBaseURL   = "https://api.openai.com/v1"
	defaultDeepSeekBaseURL = "https://api.deepseek.com/v1"
)

// ─── OpenAI ──────────────────────────────────────────────────────────────────

// OpenAIProvider implements chat.Provider for the OpenAI API.
// It supports streaming (SSE) and non-streaming chat completions with tool use.
type OpenAIProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewOpenAI creates a new OpenAI provider.
//   - apiKey: OpenAI API key (required).
//   - model:  model name (default "gpt-4o").
//   - baseURL: API base URL (default "https://api.openai.com/v1").
func NewOpenAI(apiKey, model, baseURL string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	if model == "" {
		model = "gpt-4o"
	}
	return &OpenAIProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{},
	}
}

// Name returns "openai".
func (p *OpenAIProvider) Name() string { return "openai" }

// Models returns the list of known OpenAI model identifiers.
func (p *OpenAIProvider) Models() []string {
	return []string{
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-4-turbo",
		"gpt-4",
		"gpt-3.5-turbo",
		"o1",
		"o1-mini",
	}
}

// IsAvailable returns true if an API key is configured.
func (p *OpenAIProvider) IsAvailable() bool {
	return p.apiKey != ""
}

// Chat sends a chat completion request and returns a channel of streaming events.
// When req.Stream is true the channel delivers per-token deltas; otherwise a single
// delta containing the full response is sent, followed by a done event.
func (p *OpenAIProvider) Chat(ctx context.Context, req chat.ChatRequest) (<-chan chat.ChatEvent, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	body, err := p.buildRequestBody(model, req)
	if err != nil {
		return nil, fmt.Errorf("openai: build request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	ch := make(chan chat.ChatEvent, 100)

	go func() {
		defer close(ch)

		resp, err := p.client.Do(httpReq)
		if err != nil {
			ch <- chat.ChatEvent{Type: chat.ChatEventError, Error: fmt.Errorf("openai: request failed: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			errMsg := fmt.Sprintf("openai: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
			ch <- chat.ChatEvent{Type: chat.ChatEventError, Error: fmt.Errorf("%s", errMsg)}
			return
		}

		if req.Stream {
			p.parseStreamResponse(ctx, resp.Body, ch)
		} else {
			p.parseNonStreamResponse(ctx, resp.Body, ch)
		}
	}()

	return ch, nil
}

// ─── Request builder ─────────────────────────────────────────────────────────

// openAIRequest is the JSON body sent to the OpenAI /chat/completions endpoint.
type openAIRequest struct {
	Model         string               `json:"model"`
	Messages      []openAIMessage      `json:"messages"`
	Tools         []openAITool         `json:"tools,omitempty"`
	Stream        bool                 `json:"stream"`
	StreamOptions *openAIStreamOptions `json:"stream_options,omitempty"`
}

type openAIStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type openAIMessage struct {
	Role       string          `json:"role"`
	Content    string          `json:"content,omitempty"`
	ToolCalls  []chat.ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Name       string          `json:"name,omitempty"`
}

type openAITool struct {
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

type openAIFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

func (p *OpenAIProvider) buildRequestBody(model string, req chat.ChatRequest) ([]byte, error) {
	messages := make([]openAIMessage, len(req.Messages))
	for i, m := range req.Messages {
		msg := openAIMessage{
			Role:       string(m.Role),
			Content:    m.Content,
			ToolCalls:  m.ToolCalls,
			ToolCallID: m.ToolCallID,
			Name:       m.Name,
		}
		// For multimodal content, serialize the first text part as Content
		if msg.Content == "" && len(m.ContentParts) > 0 {
			for _, cp := range m.ContentParts {
				if cp.Type == "text" {
					msg.Content = cp.Text
					break
				}
			}
		}
		messages[i] = msg
	}

	r := openAIRequest{
		Model:    model,
		Messages: messages,
		Stream:   req.Stream,
	}

	// Convert tools
	if len(req.Tools) > 0 {
		tools := make([]openAITool, 0, len(req.Tools))
		for _, t := range req.Tools {
			var params map[string]any
			if err := json.Unmarshal(t.Schema(), &params); err != nil {
				params = map[string]any{}
			}
			tools = append(tools, openAITool{
				Type: "function",
				Function: openAIFunction{
					Name:        t.Name(),
					Description: t.Description(),
					Parameters:  params,
				},
			})
		}
		r.Tools = tools
	}

	// Request usage in the final streaming chunk when streaming
	if req.Stream {
		r.StreamOptions = &openAIStreamOptions{IncludeUsage: true}
	}

	return json.Marshal(r)
}

// ─── Streaming (SSE) parser ─────────────────────────────────────────────────

// openAIStreamChunk represents a single SSE data line from the OpenAI streaming API.
type openAIStreamChunk struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Choices []openAIStreamChoice `json:"choices,omitempty"`
	Usage   *chat.Usage          `json:"usage,omitempty"`
}

type openAIStreamChoice struct {
	Index        int               `json:"index"`
	Delta        openAIStreamDelta `json:"delta"`
	FinishReason *string           `json:"finish_reason,omitempty"`
}

type openAIStreamDelta struct {
	Content   string          `json:"content,omitempty"`
	Role      string          `json:"role,omitempty"`
	ToolCalls []chat.ToolCall `json:"tool_calls,omitempty"`
}

func (p *OpenAIProvider) parseStreamResponse(ctx context.Context, body io.Reader, ch chan<- chat.ChatEvent) {
	reader := bufio.NewReader(body)

	for {
		select {
		case <-ctx.Done():
			ch <- chat.ChatEvent{Type: chat.ChatEventError, Error: ctx.Err()}
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// Stream ended normally — send done if we haven't already
				// (usually the final chunk carries usage)
				return
			}
			ch <- chat.ChatEvent{Type: chat.ChatEventError, Error: fmt.Errorf("openai: read stream: %w", err)}
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// End-of-stream marker
		if data == "[DONE]" {
			return
		}

		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			ch <- chat.ChatEvent{Type: chat.ChatEventError, Error: fmt.Errorf("openai: parse chunk: %w", err)}
			return
		}

		// If usage is present, send a done event
		if chunk.Usage != nil {
			ch <- chat.ChatEvent{Type: chat.ChatEventDone, Usage: chunk.Usage}
			return
		}

		// Emit delta for each choice
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				ch <- chat.ChatEvent{
					Type:  chat.ChatEventDelta,
					Delta: choice.Delta.Content,
				}
			}

			// If finish reason is set, this is the final chunk — usage may follow
			if choice.FinishReason != nil && *choice.FinishReason != "" {
				// Don't return yet; a subsequent chunk may carry usage
				// We'll wait for it or for the [DONE] marker
			}
		}
	}
}

// ─── Non-streaming parser ────────────────────────────────────────────────────

// openAIResponse represents a non-streaming chat completion response.
type openAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   *chat.Usage    `json:"usage,omitempty"`
}

type openAIChoice struct {
	Index        int               `json:"index"`
	Message      openAIRespMessage `json:"message"`
	FinishReason *string           `json:"finish_reason,omitempty"`
}

type openAIRespMessage struct {
	Role      string          `json:"role"`
	Content   string          `json:"content,omitempty"`
	ToolCalls []chat.ToolCall `json:"tool_calls,omitempty"`
}

func (p *OpenAIProvider) parseNonStreamResponse(ctx context.Context, body io.Reader, ch chan<- chat.ChatEvent) {
	var resp openAIResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		ch <- chat.ChatEvent{Type: chat.ChatEventError, Error: fmt.Errorf("openai: decode response: %w", err)}
		return
	}

	// Send full content as a single delta
	if len(resp.Choices) > 0 && resp.Choices[0].Message.Content != "" {
		ch <- chat.ChatEvent{
			Type:  chat.ChatEventDelta,
			Delta: resp.Choices[0].Message.Content,
		}
	}

	// Send usage in a done event
	usage := resp.Usage
	if usage == nil {
		usage = &chat.Usage{}
	}
	ch <- chat.ChatEvent{Type: chat.ChatEventDone, Usage: usage}
}

// ─── DeepSeek (OpenAI-compatible) ────────────────────────────────────────────

// DeepSeekProvider implements chat.Provider for DeepSeek.
// DeepSeek exposes an OpenAI-compatible API, so it embeds OpenAIProvider.
type DeepSeekProvider struct {
	*OpenAIProvider
}

// NewDeepSeek creates a new DeepSeek provider.
//   - apiKey: DeepSeek API key.
//   - model:  model name (default "deepseek-v4-flash").
func NewDeepSeek(apiKey, model string) *DeepSeekProvider {
	return NewDeepSeekWithBaseURL(apiKey, model, defaultDeepSeekBaseURL)
}

// NewDeepSeekWithBaseURL creates a new DeepSeek provider with an explicit base
// URL. DeepSeek exposes an OpenAI-compatible API, so it reuses the OpenAI
// transport. An empty baseURL falls back to the built-in DeepSeek endpoint.
// This is what the chat registry factory uses so a configured override
// (config base_url / DEEPSEEK_BASE_URL) is honoured at selection time.
func NewDeepSeekWithBaseURL(apiKey, model, baseURL string) *DeepSeekProvider {
	if model == "" {
		model = "deepseek-v4-flash"
	}
	if baseURL == "" {
		baseURL = defaultDeepSeekBaseURL
	}
	return &DeepSeekProvider{
		OpenAIProvider: NewOpenAI(apiKey, model, baseURL),
	}
}

// Name returns "deepseek".
func (p *DeepSeekProvider) Name() string { return "deepseek" }

// Models returns known DeepSeek model identifiers.
func (p *DeepSeekProvider) Models() []string {
	return []string{
		"deepseek-v4-flash",
		"deepseek-chat",
		"deepseek-reasoner",
	}
}
