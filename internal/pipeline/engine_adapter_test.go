package pipeline

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
)

// TestTokenUsageFrom valida o mapeamento puro de chat.Usage → pipeline.TokenUsage
// (ADR-031 Fase 1). É o elo que faz o mapping em cli/run.go (rec.CachedTokens =
// run.TokenUsage.CachedTokens) receber valor real vindo do engine.
func TestTokenUsageFrom(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   chat.Usage
		want TokenUsage
	}{
		{
			name: "mapeia campos base e decomposição",
			in: chat.Usage{
				PromptTokens:     2000,
				CompletionTokens: 500,
				TotalTokens:      2500,
				CachedTokens:     1500,
				ReasoningTokens:  120,
			},
			want: TokenUsage{
				Input:           2000,
				Output:          500,
				CachedTokens:    1500,
				ReasoningTokens: 120,
			},
		},
		{
			name: "zero quando provider não expõe detalhe",
			in: chat.Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
			want: TokenUsage{
				Input:  100,
				Output: 50,
			},
		},
		{
			name: "decomposição sem base",
			in: chat.Usage{
				CachedTokens:    300,
				ReasoningTokens: 10,
			},
			want: TokenUsage{
				CachedTokens:    300,
				ReasoningTokens: 10,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := tokenUsageFrom(tt.in)
			if got != tt.want {
				t.Errorf("tokenUsageFrom() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
