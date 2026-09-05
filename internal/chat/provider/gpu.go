package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/compute"
)

type GPUChatProvider struct {
	executor compute.GPUExecutor
	model    string
}

func NewGPUChatProvider(executor compute.GPUExecutor, model string) *GPUChatProvider {
	if model == "" {
		model = "llama3.1:8b"
	}
	return &GPUChatProvider{
		executor: executor,
		model:    model,
	}
}

func (p *GPUChatProvider) Name() string { return "cosca-gpu" }

func (p *GPUChatProvider) Models() []string {
	return []string{
		"llama3.1:8b",
		"llama3:8b",
		"qwen2.5:7b",
		"mistral:7b",
		"codellama:7b",
	}
}

func (p *GPUChatProvider) IsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return p.executor.Available(ctx)
}

func (p *GPUChatProvider) Chat(ctx context.Context, req chat.ChatRequest) (<-chan chat.ChatEvent, error) {
	prompt := p.formatMessagesAsPrompt(req.Messages)
	model := req.Model
	if model == "" {
		model = p.model
	}

	ch := make(chan chat.ChatEvent, 1)
	go func() {
		defer close(ch)

		gpuReq := compute.GPURequest{
			Model:  model,
			Prompt: prompt,
		}
		result, err := p.executor.Execute(ctx, gpuReq)
		if err != nil {
			ch <- chat.ChatEvent{Type: chat.ChatEventError, Error: fmt.Errorf("gpu: %w", err)}
			return
		}

		usage := &chat.Usage{
			CompletionTokens: result.TotalTokens,
			TotalTokens:      result.TotalTokens,
		}
		ch <- chat.ChatEvent{
			Type:  chat.ChatEventDelta,
			Delta: result.Output,
		}

		ch <- chat.ChatEvent{Type: chat.ChatEventDone, Usage: usage}
	}()
	return ch, nil
}

func (p *GPUChatProvider) formatMessagesAsPrompt(messages []chat.Message) string {
	var b strings.Builder
	for _, msg := range messages {
		switch msg.Role {
		case chat.RoleSystem:
			fmt.Fprintf(&b, "<|system|>\n%s\n</|system|>\n\n", msg.Content)
		case chat.RoleUser:
			fmt.Fprintf(&b, "<|user|>\n%s\n</|user|>\n\n", msg.Content)
		case chat.RoleAssistant:
			fmt.Fprintf(&b, "<|assistant|>\n%s\n</|assistant|>\n\n", msg.Content)
		default:
			b.WriteString(msg.Content)
			b.WriteString("\n\n")
		}
	}
	b.WriteString("<|assistant|>\n")
	return b.String()
}
