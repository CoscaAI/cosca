package orchestration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
)

// alwaysFailProvider sempre falha com erro transiente (não-stall) — simula um
// provider primário quebrado de verdade.
type alwaysFailProvider struct{ name string }

func (p *alwaysFailProvider) Name() string { return p.name }
func (p *alwaysFailProvider) Model() string { return "fail" }
func (p *alwaysFailProvider) Close() error  { return nil }
func (p *alwaysFailProvider) Chat(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
	return nil, errors.New("rate limit exceeded: provider down")
}
func (p *alwaysFailProvider) ChatStream(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (chat.ChatStream, error) {
	return nil, errors.New("not implemented")
}

// okWorkProvider sempre responde com sucesso — o provider alternativo saudável.
type okWorkProvider struct{ name string }

func (p *okWorkProvider) Name() string { return p.name }
func (p *okWorkProvider) Model() string { return "ok" }
func (p *okWorkProvider) Close() error  { return nil }
func (p *okWorkProvider) Chat(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
	return &chat.ChatResponse{
		Model: p.name,
		Choices: []chat.Choice{{
			Index:        0,
			Message:      chat.Message{Role: chat.RoleAssistant, Content: "fallback worked"},
			FinishReason: chat.FinishReasonStop,
		}},
	}, nil
}
func (p *okWorkProvider) ChatStream(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (chat.ChatStream, error) {
	return nil, errors.New("not implemented")
}

// TestFallbackProvider_EsgotaERecupera prova o gap #6: o primário falha em
// todas as tentativas (transient), esgota os retries, e o fallback alternativo
// recupera com sucesso. Se o fallback não existisse, o run falharia.
func TestFallbackProvider_EsgotaERecupera(t *testing.T) {
	cfg := ExecutorConfig{
		MaxRetries: 2,
		RetryDelay: 10 * time.Millisecond,
		Timeout:    2 * time.Second,
	}
	primary := &alwaysFailProvider{name: "primary-fail"}
	fallback := &okWorkProvider{name: "fallback-ok"}
	ex := NewExecutor(primary, cfg, nil)
	ex.SetFallbackProviders([]chat.ChatProvider{fallback})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	msgs := []chat.Message{{Role: chat.RoleUser, Content: "hello"}}
	resp, err := ex.chatWithRetry(ctx, msgs, chat.ChatOptions{})
	if err != nil {
		t.Fatalf("esperava sucesso via fallback, mas falhou: %v", err)
	}
	if resp == nil || resp.Choices[0].Message.Content != "fallback worked" {
		t.Fatalf("esperava resposta do fallback, obteve: %+v", resp)
	}
	t.Logf("✓ fallback recuperou: response=%q", resp.Choices[0].Message.Content)
}

// TestNoFallback_FalhaAposRetries prova o comportamento LEGACY preservado:
// sem fallback configurado, esgotar os retries = falha (não regride).
func TestNoFallback_FalhaAposRetries(t *testing.T) {
	cfg := ExecutorConfig{MaxRetries: 1, RetryDelay: 10 * time.Millisecond, Timeout: 2 * time.Second}
	primary := &alwaysFailProvider{name: "primary-fail"}
	ex := NewExecutor(primary, cfg, nil) // SEM fallback

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	msgs := []chat.Message{{Role: chat.RoleUser, Content: "hello"}}
	_, err := ex.chatWithRetry(ctx, msgs, chat.ChatOptions{})
	if err == nil {
		t.Fatal("esperava falha (sem fallback), mas obteve sucesso")
	}
	t.Logf("✓ comportamento legacy preservado: falhou após retries (err=%v)", err)
}
