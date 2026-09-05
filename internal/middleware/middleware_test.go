package middleware

import (
	"context"
	"errors"
	"testing"
)

// TestChain_ForwardsToTerminal valida que os middlewares chamam o terminal.
func TestChain_ForwardsToTerminal(t *testing.T) {
	var order []string
	chain := NewChain(
		func(ctx context.Context, m *Context, next Handler) (string, error) {
			order = append(order, "mw1-before")
			r, err := next(ctx, m)
			order = append(order, "mw1-after")
			return r, err
		},
		func(ctx context.Context, m *Context, next Handler) (string, error) {
			order = append(order, "mw2-before")
			r, err := next(ctx, m)
			order = append(order, "mw2-after")
			return r, err
		},
	)

	result, err := chain.Run(context.Background(), &Context{}, func(ctx context.Context, m *Context) (string, error) {
		order = append(order, "terminal")
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result != "ok" {
		t.Fatalf("result = %q, esperava ok", result)
	}
	// mw1 (mais externa) envolve mw2 que envolve o terminal.
	want := []string{"mw1-before", "mw2-before", "terminal", "mw2-after", "mw1-after"}
	if len(order) != len(want) {
		t.Fatalf("ordem = %v, esperava %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("ordem[%d] = %q, esperava %q (full %v)", i, order[i], want[i], order)
		}
	}
}

// TestChain_AbortMiddleware valida que um middleware pode abortar (curto-
// circuito) SEM chamar o terminal — o padrão wrap_model_call do LangChain.
func TestChain_AbortMiddleware(t *testing.T) {
	var terminalCalled bool
	chain := NewChain(
		func(ctx context.Context, m *Context, next Handler) (string, error) {
			_ = next // não chama o terminal
			return "", errors.New("budget exceeded")
		},
	)

	_, err := chain.Run(context.Background(), &Context{}, func(ctx context.Context, m *Context) (string, error) {
		terminalCalled = true
		return "ok", nil
	})
	if err == nil {
		t.Fatal("esperava erro do middleware abortado")
	}
	if terminalCalled {
		t.Fatal("terminal nao deveria ser chamado apos abort")
	}
}
