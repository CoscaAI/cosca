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

// ─── Defaults ────────────────────────────────────────────────────────────────

const (
	defaultAnthropicBaseURL = "https://api.anthropic.com"
	anthropicVersion        = "2023-06-01"
)

// ─── Anthropic ───────────────────────────────────────────────────────────────

// AnthropicProvider implements chat.Provider for Anthropic Claude.
type AnthropicProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewAnthropic creates a new Anthropic provider.
//   - apiKey: Anthropic API key (required).
//   - model:  model name (default "claude-sonnet-4-20250514").
//   - baseURL: API base URL (default "https://api.anthropic.com").
func NewAnthropic(apiKey, model, baseURL string) *AnthropicProvider {
	if baseURL == "" {
		baseURL = defaultAnthropicBaseURL
	}
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	return &AnthropicProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{},
	}
}

// Name returns "anthropic".
func (p *AnthropicProvider) Name() string { return "anthropic" }

// Models returns known Anthropic Claude model identifiers.
func (p *AnthropicProvider) Models() []string {
	return []string{
		"claude-sonnet-4-20250514",
		"claude-haiku-4-20250514",
		"claude-opus-4-20250514",
		"claude-3-opus-latest",
		"claude-3-sonnet-latest",
		"claude-3-haiku-latest",
	}
}

// IsAvailable returns true if an API key is configured.
func (p *AnthropicProvider) IsAvailable() bool {
	return p.apiKey != ""
}

// Chat sends a chat completion request and returns a channel of streaming events.
func (p *AnthropicProvider) Chat(ctx context.Context, req chat.ChatRequest) (<-chan chat.ChatEvent, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	body, err := p.buildRequestBody(model, req)
	if err != nil {
		return nil, fmt.Errorf("anthropic: build request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	ch := make(chan chat.ChatEvent, 100)

	go func() {
		defer close(ch)

		resp, err := p.client.Do(httpReq)
		if err != nil {
			ch <- chat.ChatEvent{Type: chat.ChatEventError, Error: fmt.Errorf("anthropic: request failed: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			errMsg := fmt.Sprintf("anthropic: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
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

// ─── Anthropic request types ─────────────────────────────────────────────────

// anthropicRequest is the JSON body for the Anthropic /v1/messages endpoint.
type anthropicRequest struct {
	Model     string             `json:"model"`
	Messages  []anthropicMessage `json:"messages"`
	System    string             `json:"system,omitempty"`
	MaxTokens int                `json:"max_tokens,omitempty"`
	Stream    bool               `json:"stream"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
}

type anthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"` // string or array of content blocks
}

// anthropicContentBlock is a single content block inside a message.
type anthropicContentBlock struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	ID    string `json:"id,omitempty"`    // for tool_use
	Name  string `json:"name,omitempty"`  // for tool_use
	Input any    `json:"input,omitempty"` // for tool_use
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
}

func (p *AnthropicProvider) buildRequestBody(model string, req chat.ChatRequest) ([]byte, error) {
	// Extract system message and convert remaining messages
	var systemText string
	messages := make([]anthropicMessage, 0, len(req.Messages))

	for _, m := range req.Messages {
		switch m.Role {
		case chat.RoleSystem:
			if systemText == "" {
				systemText = m.Content
			} else {
				systemText += "\n" + m.Content
			}
			continue

		case chat.RoleTool:
			// Anthropic expects tool results as user messages with content blocks
			content := fmt.Sprintf(`[{"type": "tool_result", "tool_use_id": %q, "content": %q}]`, m.ToolCallID, m.Content)
			messages = append(messages, anthropicMessage{
				Role:    "user",
				Content: json.RawMessage(content),
			})

		case chat.RoleAssistant:
			// Assistant messages may contain tool calls
			if len(m.ToolCalls) > 0 {
				blocks := make([]anthropicContentBlock, 0)
				if m.Content != "" {
					blocks = append(blocks, anthropicContentBlock{Type: "text", Text: m.Content})
				}
				for _, tc := range m.ToolCalls {
					var input any
					_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
					blocks = append(blocks, anthropicContentBlock{
						Type:  "tool_use",
						ID:    tc.ID,
						Name:  tc.Function.Name,
						Input: input,
					})
				}
				data, _ := json.Marshal(blocks)
				messages = append(messages, anthropicMessage{
					Role:    "assistant",
					Content: data,
				})
			} else {
				messages = append(messages, anthropicMessage{
					Role:    "assistant",
					Content: mustMarshalString(m.Content),
				})
			}

		default: // user
			messages = append(messages, anthropicMessage{
				Role:    "user",
				Content: mustMarshalString(m.Content),
			})
		}
	}

	r := anthropicRequest{
		Model:     model,
		Messages:  messages,
		System:    systemText,
		MaxTokens: 4096,
		Stream:    req.Stream,
	}

	// Convert tools
	if len(req.Tools) > 0 {
		tools := make([]anthropicTool, 0, len(req.Tools))
		for _, t := range req.Tools {
			var inputSchema map[string]any
			if err := json.Unmarshal(t.Schema(), &inputSchema); err != nil {
				inputSchema = map[string]any{}
			}
			tools = append(tools, anthropicTool{
				Name:        t.Name(),
				Description: t.Description(),
				InputSchema: inputSchema,
			})
		}
		r.Tools = tools
	}

	return json.Marshal(r)
}

func mustMarshalString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

// ─── Anthropic streaming (SSE) parser ────────────────────────────────────────

// anthropicStreamEvent is a single SSE event from the Anthropic streaming API.
type anthropicStreamEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"-"` // not used; we parse the event body directly
}

// anthropicMessageStart is the first event in a stream.
type anthropicMessageStart struct {
	Type    string            `json:"type"`
	Message anthropicStartMsg `json:"message"`
}

type anthropicStartMsg struct {
	ID    string      `json:"id"`
	Model string      `json:"model"`
	Usage *chat.Usage `json:"usage,omitempty"`
}

// anthropicContentBlockDelta carries text delta.
type anthropicContentBlockDelta struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
}

// anthropicMessageDelta carries usage at the end.
type anthropicMessageDeltaEvent struct {
	Type  string `json:"type"`
	Delta struct {
		StopReason   string `json:"stop_reason"`
		StopSequence string `json:"stop_sequence"`
	} `json:"delta"`
	Usage *chat.Usage `json:"usage,omitempty"`
}

func (p *AnthropicProvider) parseStreamResponse(ctx context.Context, body io.Reader, ch chan<- chat.ChatEvent) {
	reader := bufio.NewReader(body)
	var eventName string

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
			ch <- chat.ChatEvent{Type: chat.ChatEventError, Error: fmt.Errorf("anthropic: read stream: %w", err)}
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "event: ") {
			eventName = strings.TrimPrefix(line, "event: ")
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			switch eventName {
			case "content_block_delta":
				var evt anthropicContentBlockDelta
				if err := json.Unmarshal([]byte(data), &evt); err != nil {
					continue
				}
				if evt.Delta.Text != "" {
					ch <- chat.ChatEvent{Type: chat.ChatEventDelta, Delta: evt.Delta.Text}
				}

			case "message_delta":
				var evt anthropicMessageDeltaEvent
				if err := json.Unmarshal([]byte(data), &evt); err != nil {
					continue
				}
				if evt.Usage != nil {
					ch <- chat.ChatEvent{Type: chat.ChatEventDone, Usage: evt.Usage}
					return
				}

			case "message_stop":
				// Stream complete; if no usage was sent, send a done without usage
				ch <- chat.ChatEvent{Type: chat.ChatEventDone, Usage: &chat.Usage{}}
				return

			case "error":
				var errEvt struct {
					Type  string `json:"type"`
					Error struct {
						Type    string `json:"type"`
						Message string `json:"message"`
					} `json:"error"`
				}
				if err := json.Unmarshal([]byte(data), &errEvt); err == nil {
					ch <- chat.ChatEvent{Type: chat.ChatEventError, Error: fmt.Errorf("anthropic: %s: %s", errEvt.Error.Type, errEvt.Error.Message)}
				}
				return
			}
		}
	}
}

// ─── Non-streaming parser ────────────────────────────────────────────────────

// anthropicNonStreamResponse is the response shape for non-streaming requests.
type anthropicNonStreamResponse struct {
	ID      string                  `json:"id"`
	Type    string                  `json:"type"`
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
	Model   string                  `json:"model"`
	Usage   *chat.Usage             `json:"usage,omitempty"`
}

func (p *AnthropicProvider) parseNonStreamResponse(ctx context.Context, body io.Reader, ch chan<- chat.ChatEvent) {
	var resp anthropicNonStreamResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		ch <- chat.ChatEvent{Type: chat.ChatEventError, Error: fmt.Errorf("anthropic: decode response: %w", err)}
		return
	}

	// Concatenate text content blocks
	var fullText strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" {
			fullText.WriteString(block.Text)
		}
	}

	if fullText.Len() > 0 {
		ch <- chat.ChatEvent{Type: chat.ChatEventDelta, Delta: fullText.String()}
	}

	usage := resp.Usage
	if usage == nil {
		usage = &chat.Usage{}
	}
	ch <- chat.ChatEvent{Type: chat.ChatEventDone, Usage: usage}
}
