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
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
)

// ─── Defaults ────────────────────────────────────────────────────────────────

const defaultOllamaBaseURL = "http://localhost:11434"

// defaultOllamaTimeout é generoso de propósito. O Ollama carrega o modelo na
// PRIMEIRA chamada após o servidor iniciar (ex.: qwen2.5:7b, ~5GB) e gera
// texto em CPU no nosso caso — um timeout curto (5s) matava a primeira
// resposta em silêncio (Tokens: 0) e podia cortar gerações longas no meio.
const defaultOllamaTimeout = 5 * time.Minute

// defaultOllamaKeepAlive mantém o modelo carregado no servidor por 30min de
// inatividade, evitando o recarregamento frio (e o risco de timeout) entre
// turnos da voz. O padrão do Ollama é 5min — curto demais para uso contínuo.
const defaultOllamaKeepAlive = "30m"

// ─── Ollama ──────────────────────────────────────────────────────────────────

// OllamaProvider implements chat.Provider for Ollama (local LLM server).
// It uses Ollama's native API at /api/chat.
type OllamaProvider struct {
	model   string
	baseURL string
	client  *http.Client
}

// NewOllama creates a new Ollama provider.
//   - model:   model name (default "llama3").
//   - baseURL: Ollama server URL (default "http://localhost:11434").
func NewOllama(model, baseURL string) *OllamaProvider {
	if baseURL == "" {
		baseURL = defaultOllamaBaseURL
	}
	if model == "" {
		model = "llama3"
	}
	return &OllamaProvider{
		model:   model,
		baseURL: strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/v1"),
		client: &http.Client{
			Timeout: defaultOllamaTimeout,
		},
	}
}

// Name returns "ollama".
func (p *OllamaProvider) Name() string { return "ollama" }

// Models returns a list of common Ollama models.
func (p *OllamaProvider) Models() []string {
	return []string{
		"llama3",
		"llama3:70b",
		"mistral",
		"mixtral",
		"codellama",
		"phi3",
		"qwen2",
		"gemma2",
	}
}

// IsAvailable attempts a lightweight reachability check against the Ollama server.
// It sends a GET to the server root with a short timeout.
func (p *OllamaProvider) IsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/", nil)
	if err != nil {
		return false
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Chat sends a chat completion request and returns a channel of streaming events.
// It uses Ollama's native /api/chat endpoint.
func (p *OllamaProvider) Chat(ctx context.Context, req chat.ChatRequest) (<-chan chat.ChatEvent, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	body, err := p.buildRequestBody(model, req)
	if err != nil {
		return nil, fmt.Errorf("ollama: build request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimSuffix(p.baseURL, "/v1")+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	ch := make(chan chat.ChatEvent, 100)

	go func() {
		defer close(ch)

		resp, err := p.client.Do(httpReq)
		if err != nil {
			ch <- chat.ChatEvent{Type: chat.ChatEventError, Error: fmt.Errorf("ollama: request failed: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			errMsg := fmt.Sprintf("ollama: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
			ch <- chat.ChatEvent{Type: chat.ChatEventError, Error: fmt.Errorf("%s", errMsg)}
			return
		}

		if req.Stream {
			p.parseStreamResponse(ctx, resp.Body, ch, model)
		} else {
			p.parseNonStreamResponse(ctx, resp.Body, ch)
		}
	}()

	return ch, nil
}

// ─── Request/response types ──────────────────────────────────────────────────

type ollamaRequest struct {
	Model     string          `json:"model"`
	Messages  []ollamaMessage `json:"messages"`
	Tools     []openAITool    `json:"tools,omitempty"`
	Stream    bool            `json:"stream"`
	KeepAlive string          `json:"keep_alive,omitempty"`
}

type ollamaMessage struct {
	Role       string          `json:"role"`
	Content    string          `json:"content,omitempty"`
	ToolCalls  []chat.ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

type ollamaStreamChunk struct {
	Model           string              `json:"model"`
	CreatedAt       string              `json:"created_at"`
	Message         ollamaStreamMessage `json:"message"`
	Done            bool                `json:"done"`
	Usage           *chat.Usage         `json:"usage,omitempty"`
	PromptEvalCount int                 `json:"prompt_eval_count"`
	EvalCount       int                 `json:"eval_count"`
}

type ollamaStreamMessage struct {
	Role      string          `json:"role,omitempty"`
	Content   string          `json:"content,omitempty"`
	ToolCalls []chat.ToolCall `json:"tool_calls,omitempty"`
}

type ollamaResponse struct {
	Model           string              `json:"model"`
	CreatedAt       string              `json:"created_at"`
	Message         ollamaStreamMessage `json:"message"`
	Done            bool                `json:"done"`
	Usage           *chat.Usage         `json:"usage,omitempty"`
	PromptEvalCount int                 `json:"prompt_eval_count"`
	EvalCount       int                 `json:"eval_count"`
}

func (p *OllamaProvider) buildRequestBody(model string, req chat.ChatRequest) ([]byte, error) {
	messages := make([]ollamaMessage, len(req.Messages))
	for i, m := range req.Messages {
		msg := ollamaMessage{
			Role:       string(m.Role),
			Content:    m.Content,
			ToolCalls:  m.ToolCalls,
			ToolCallID: m.ToolCallID,
		}
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

	r := ollamaRequest{
		Model:     model,
		Messages:  messages,
		Stream:    req.Stream,
		KeepAlive: defaultOllamaKeepAlive,
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

	return json.Marshal(r)
}

// ─── Streaming parser ────────────────────────────────────────────────────────

func (p *OllamaProvider) parseStreamResponse(ctx context.Context, body io.Reader, ch chan<- chat.ChatEvent, model string) {
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
				return
			}
			ch <- chat.ChatEvent{Type: chat.ChatEventError, Error: fmt.Errorf("ollama: read stream: %w", err)}
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var chunk ollamaStreamChunk
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			// Ollama may return data: prefixed lines or raw JSON — try both
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					return
				}
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					continue
				}
			} else {
				continue
			}
		}

		if len(chunk.Message.ToolCalls) > 0 {
			ch <- chat.ChatEvent{Type: chat.ChatEventToolCall, ToolCalls: chunk.Message.ToolCalls}
		}

		if chunk.Message.Content != "" {
			ch <- chat.ChatEvent{Type: chat.ChatEventDelta, Delta: chunk.Message.Content}
		}

		if chunk.Done {
			usage := buildUsage(chunk.Usage, chunk.PromptEvalCount, chunk.EvalCount)
			ch <- chat.ChatEvent{Type: chat.ChatEventDone, Usage: usage}
			return
		}
	}
}

// ─── Non-streaming parser ────────────────────────────────────────────────────

func (p *OllamaProvider) parseNonStreamResponse(ctx context.Context, body io.Reader, ch chan<- chat.ChatEvent) {
	var resp ollamaResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		ch <- chat.ChatEvent{Type: chat.ChatEventError, Error: fmt.Errorf("ollama: decode response: %w", err)}
		return
	}

	// Surface tool calls as first-class events (never discard them).
	if len(resp.Message.ToolCalls) > 0 {
		ch <- chat.ChatEvent{Type: chat.ChatEventToolCall, ToolCalls: resp.Message.ToolCalls}
	}
	if resp.Message.Content != "" {
		ch <- chat.ChatEvent{Type: chat.ChatEventDelta, Delta: resp.Message.Content}
	}

	usage := buildUsage(resp.Usage, resp.PromptEvalCount, resp.EvalCount)
	ch <- chat.ChatEvent{Type: chat.ChatEventDone, Usage: usage}
}

// buildUsage merges a parsed usage object (if present) with Ollama's native
// top-level prompt_eval_count/eval_count fields into a chat.Usage.
func buildUsage(u *chat.Usage, promptEval, eval int) *chat.Usage {
	if u == nil {
		u = &chat.Usage{}
	}
	if promptEval > 0 {
		u.PromptTokens = promptEval
	}
	if eval > 0 {
		u.CompletionTokens = eval
	}
	if u.TotalTokens == 0 && (u.PromptTokens > 0 || u.CompletionTokens > 0) {
		u.TotalTokens = u.PromptTokens + u.CompletionTokens
	}
	return u
}
