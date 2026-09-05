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
	Model     string             `json:"model"`
	Messages  []ollamaMessage    `json:"messages"`
	Tools     []openAITool       `json:"tools,omitempty"`
	Stream    bool               `json:"stream"`
	KeepAlive string             `json:"keep_alive,omitempty"`
	Options   *ollamaOptions     `json:"options,omitempty"`
}

// ollamaOptions carrega num_ctx (janela de contexto). Sem ele, o Ollama usa o
// default baixo (4096) e trunca prompts grandes da esteira -> o modelo perde a
// instrução de tool-call e responde em prosa (a causa raiz do "pedreiro não
// constrói", 2026-09-03).
type ollamaOptions struct {
	NumCtx int `json:"num_ctx,omitempty"`
}

type ollamaMessage struct {
	Role       string              `json:"role"`
	Content    string              `json:"content,omitempty"`
	ToolCalls  []ollamaReqToolCall `json:"tool_calls,omitempty"`
	ToolCallID string              `json:"tool_call_id,omitempty"`
}

// ollamaReqToolCall marshals a tool call for Ollama's native /api/chat request.
// Unlike the OpenAI JSON wire format (where function.arguments is a JSON
// *string*), Ollama expects arguments as a JSON *object*. We therefore carry
// the already-parsed arguments as json.RawMessage so it round-trips as an
// object (see toOllamaReqToolCalls).
type ollamaReqToolCall struct {
	Index    int               `json:"index,omitempty"`
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Function ollamaReqFunction `json:"function"`
}

type ollamaReqFunction struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
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
	Role      string               `json:"role,omitempty"`
	Content   string               `json:"content,omitempty"`
	ToolCalls []ollamaRespToolCall `json:"tool_calls,omitempty"`
}

// olamaRespToolCall decodes a tool call from Ollama's native /api/chat
// response. Unlike the OpenAI JSON wire format (where function.arguments is a
// JSON *string*), Ollama returns arguments as a JSON *object*. The shared
// chat.ToolCall.Function.Arguments type is a string, so we normalise the
// object to its compact JSON form on decode (see toChatToolCalls).
type ollamaRespToolCall struct {
	Index    int                `json:"index,omitempty"`
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function ollamaRespFunction `json:"function"`
}

type ollamaRespFunction struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
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
			ToolCalls:  toOllamaReqToolCalls(m.ToolCalls),
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

	// Janela de contexto (num_ctx): propaga ChatOptions.NumCtx (default 32768).
	// Sem isto o Ollama trunca prompts grandes da esteira no default 4096.
	if req.NumCtx > 0 {
		r.Options = &ollamaOptions{NumCtx: req.NumCtx}
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
			ch <- chat.ChatEvent{Type: chat.ChatEventToolCall, ToolCalls: toChatToolCalls(chunk.Message.ToolCalls)}
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
		ch <- chat.ChatEvent{Type: chat.ChatEventToolCall, ToolCalls: toChatToolCalls(resp.Message.ToolCalls)}
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

// toChatToolCalls converts Ollama response tool calls (where function.arguments
// is a JSON object) into the shared chat.ToolCall type (where Arguments is a
// JSON string). This bridges Ollama's native /api/chat wire format with the
// OpenAI-compatible function-calling shape used by the rest of the engine and
// by the Tool Executor.
func toChatToolCalls(in []ollamaRespToolCall) []chat.ToolCall {
	if len(in) == 0 {
		return nil
	}
	out := make([]chat.ToolCall, 0, len(in))
	for _, tc := range in {
		out = append(out, chat.ToolCall{
			Index: tc.Index,
			ID:    tc.ID,
			Type:  tc.Type,
			Function: chat.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: normalizeToolArguments(tc.Function.Arguments),
			},
		})
	}
	return out
}

// normalizeToolArguments converts a JSON argument payload into the string form
// expected by chat.FunctionCall.Arguments. Ollama returns function.arguments as
// a JSON object, while its OpenAI-compatible form expects a JSON string:
//   - If the raw payload is already a JSON string (e.g. "{\"path\":\"x\"}"), it
//     is unwrapped to the inner string.
//   - Otherwise (JSON object/array/null) it is kept as its compact JSON form,
//     which is exactly what executor.ToolCall.Input expects after json.Unmarshal.
func normalizeToolArguments(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Already a JSON string literal → unwrap it (deduplicate the quotes).
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Object/array/null → keep the compact JSON form.
	return string(raw)
}

// toOllamaReqToolCalls converts a slice of chat.ToolCall (whose Arguments is a
// JSON string) into the request form Ollama's /api/chat expects, where
// function.arguments is a JSON object. The Arguments string (a valid JSON
// object literal) is carried as json.RawMessage so it is re-emitted verbatim as
// an object instead of being double-encoded as a string.
func toOllamaReqToolCalls(in []chat.ToolCall) []ollamaReqToolCall {
	if len(in) == 0 {
		return nil
	}
	out := make([]ollamaReqToolCall, 0, len(in))
	for _, tc := range in {
		f := ollamaReqFunction{Name: tc.Function.Name}
		if args := strings.TrimSpace(tc.Function.Arguments); args != "" {
			f.Arguments = json.RawMessage(args)
		}
		out = append(out, ollamaReqToolCall{
			Index:    tc.Index,
			ID:       tc.ID,
			Type:     tc.Type,
			Function: f,
		})
	}
	return out
}
