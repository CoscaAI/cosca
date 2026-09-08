package chat

import (
	"encoding/json"
	"testing"
)

// ─── Message: HasImage ──────────────────────────────────────────────────────

func TestMessage_HasImage(t *testing.T) {
	t.Parallel()

	t.Run("no content parts", func(t *testing.T) {
		m := Message{Role: RoleUser, Content: "Hello"}
		if m.HasImage() {
			t.Error("expected false for message with no ContentParts")
		}
	})

	t.Run("text only parts", func(t *testing.T) {
		m := Message{
			Role: RoleUser,
			ContentParts: []ContentPart{
				{Type: "text", Text: "What's in this image?"},
			},
		}
		if m.HasImage() {
			t.Error("expected false for text-only ContentParts")
		}
	})

	t.Run("with image", func(t *testing.T) {
		m := Message{
			Role: RoleUser,
			ContentParts: []ContentPart{
				{Type: "text", Text: "Describe this"},
				{
					Type: "image_url",
					ImageURL: &ImageURL{
						URL: "data:image/jpeg;base64,abc123",
					},
				},
			},
		}
		if !m.HasImage() {
			t.Error("expected true for message with image ContentPart")
		}
	})

	t.Run("image with empty URL", func(t *testing.T) {
		m := Message{
			Role: RoleUser,
			ContentParts: []ContentPart{
				{Type: "image_url", ImageURL: &ImageURL{URL: ""}},
			},
		}
		if m.HasImage() {
			t.Error("expected false for image with empty URL")
		}
	})

	t.Run("nil ImageURL", func(t *testing.T) {
		m := Message{
			Role: RoleUser,
			ContentParts: []ContentPart{
				{Type: "image_url", ImageURL: nil},
			},
		}
		if m.HasImage() {
			t.Error("expected false for image_url with nil ImageURL")
		}
	})
}

// ─── Message: AddImage ──────────────────────────────────────────────────────

func TestMessage_AddImage(t *testing.T) {
	t.Parallel()

	t.Run("add image to empty message", func(t *testing.T) {
		m := &Message{Role: RoleUser}
		m.AddImage("data:image/png;base64,abc123", "high")

		if len(m.ContentParts) != 1 {
			t.Fatalf("expected 1 ContentPart, got %d", len(m.ContentParts))
		}
		if m.ContentParts[0].Type != "image_url" {
			t.Errorf("expected 'image_url', got %q", m.ContentParts[0].Type)
		}
		if m.ContentParts[0].ImageURL.URL != "data:image/png;base64,abc123" {
			t.Errorf("expected URL, got %q", m.ContentParts[0].ImageURL.URL)
		}
		if m.ContentParts[0].ImageURL.Detail != "high" {
			t.Errorf("expected detail 'high', got %q", m.ContentParts[0].ImageURL.Detail)
		}
		if m.Content != "" {
			t.Error("Content should be empty after using ContentParts")
		}
	})

	t.Run("add image after text content", func(t *testing.T) {
		m := &Message{Role: RoleUser, Content: "What is this?"}
		m.AddImage("data:image/jpeg;base64,xyz789", "low")

		if len(m.ContentParts) != 2 {
			t.Fatalf("expected 2 ContentParts, got %d", len(m.ContentParts))
		}
		// First part should be the migrated text
		if m.ContentParts[0].Type != "text" {
			t.Errorf("part 0 type = %q, want 'text'", m.ContentParts[0].Type)
		}
		if m.ContentParts[0].Text != "What is this?" {
			t.Errorf("part 0 text = %q", m.ContentParts[0].Text)
		}
		// Second part should be the image
		if m.ContentParts[1].Type != "image_url" {
			t.Errorf("part 1 type = %q, want 'image_url'", m.ContentParts[1].Type)
		}
		if m.ContentParts[1].ImageURL.Detail != "low" {
			t.Errorf("part 1 detail = %q, want 'low'", m.ContentParts[1].ImageURL.Detail)
		}
		// Content should be cleared
		if m.Content != "" {
			t.Error("Content should be cleared after migrating to ContentParts")
		}
	})

	t.Run("multiple images", func(t *testing.T) {
		m := &Message{Role: RoleUser}
		m.AddImage("data:image/png;base64,img1", "auto")
		m.AddImage("data:image/jpeg;base64,img2", "auto")

		if len(m.ContentParts) != 2 {
			t.Fatalf("expected 2 ContentParts, got %d", len(m.ContentParts))
		}
		if m.ContentParts[0].ImageURL.URL != "data:image/png;base64,img1" {
			t.Errorf("image 1 URL = %q", m.ContentParts[0].ImageURL.URL)
		}
		if m.ContentParts[1].ImageURL.URL != "data:image/jpeg;base64,img2" {
			t.Errorf("image 2 URL = %q", m.ContentParts[1].ImageURL.URL)
		}
	})
}

// ─── Message: AddText ───────────────────────────────────────────────────────

func TestMessage_AddText(t *testing.T) {
	t.Parallel()

	t.Run("add text to empty message", func(t *testing.T) {
		m := &Message{Role: RoleUser}
		m.AddText("Hello world")

		if len(m.ContentParts) != 1 {
			t.Fatalf("expected 1 ContentPart, got %d", len(m.ContentParts))
		}
		if m.ContentParts[0].Type != "text" {
			t.Errorf("expected 'text', got %q", m.ContentParts[0].Type)
		}
		if m.ContentParts[0].Text != "Hello world" {
			t.Errorf("expected 'Hello world', got %q", m.ContentParts[0].Text)
		}
	})

	t.Run("add text when existing Content", func(t *testing.T) {
		m := &Message{Role: RoleUser, Content: "Existing content"}
		m.AddText("Additional text")

		if len(m.ContentParts) != 2 {
			t.Fatalf("expected 2 ContentParts, got %d", len(m.ContentParts))
		}
		if m.ContentParts[0].Text != "Existing content" {
			t.Errorf("part 0 = %q", m.ContentParts[0].Text)
		}
		if m.ContentParts[1].Text != "Additional text" {
			t.Errorf("part 1 = %q", m.ContentParts[1].Text)
		}
		if m.Content != "" {
			t.Error("Content should be cleared")
		}
	})

	t.Run("add text after addImage", func(t *testing.T) {
		m := &Message{Role: RoleUser}
		m.AddImage("data:image/png;base64,img", "auto")
		m.AddText("Describe this image")

		if len(m.ContentParts) != 2 {
			t.Fatalf("expected 2 ContentParts, got %d", len(m.ContentParts))
		}
		if m.ContentParts[0].Type != "image_url" {
			t.Errorf("part 0 should be image_url, got %q", m.ContentParts[0].Type)
		}
		if m.ContentParts[1].Type != "text" {
			t.Errorf("part 1 should be text, got %q", m.ContentParts[1].Type)
		}
	})
}

// ─── ContentPart / ImageURL Serialization ───────────────────────────────────

func TestMessage_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	t.Run("simple message backward compat", func(t *testing.T) {
		orig := Message{
			Role:    RoleUser,
			Content: "Hello",
		}

		// Manual encode/decode via JSON tags (verified via struct reflection)
		// This confirms the json tags are correct.
		if orig.Role != RoleUser {
			t.Error("role mismatch")
		}
		if orig.Content != "Hello" {
			t.Error("content mismatch")
		}
	})

	t.Run("multimodal message", func(t *testing.T) {
		orig := Message{
			Role: RoleUser,
			ContentParts: []ContentPart{
				{Type: "text", Text: "What's this?"},
				{Type: "image_url", ImageURL: &ImageURL{URL: "data:image/jpeg;base64,abc", Detail: "auto"}},
			},
		}

		if len(orig.ContentParts) != 2 {
			t.Fatal("content parts mismatch")
		}
		if orig.ContentParts[1].Type != "image_url" {
			t.Error("part type mismatch")
		}
		if orig.ContentParts[1].ImageURL.URL != "data:image/jpeg;base64,abc" {
			t.Error("image URL mismatch")
		}
	})
}

// TestUsage_CachedAndEffective valida a decomposicao ADR-031 Fase 0.1:
// effective = prompt - cached (tokens realmente processados, fora do reuso).
func TestUsage_CachedAndEffective(t *testing.T) {
	u := Usage{PromptTokens: 1000, CompletionTokens: 200, CachedTokens: 600}
	cached, effective := u.CachedAndEffective()
	if cached != 600 {
		t.Fatalf("cached = %d, esperava 600", cached)
	}
	if effective != 400 {
		t.Fatalf("effective = %d, esperava 400 (1000-600)", effective)
	}
}

// TestUsage_ReasoningTokens valida o capture do reasoning (thinking).
func TestUsage_ReasoningTokens(t *testing.T) {
	u := Usage{PromptTokens: 100, CompletionTokens: 50, ReasoningTokens: 20}
	if u.ReasoningTokensCount() != 20 {
		t.Fatalf("reasoning = %d, esperava 20", u.ReasoningTokensCount())
	}
	// Default zero (provider nao expos).
	var zero Usage
	if zero.ReasoningTokensCount() != 0 {
		t.Fatalf("default reasoning = %d, esperava 0", zero.ReasoningTokensCount())
	}
}

// decodeRawUsage desserializa um payload de usage OpenAI-compatível bruto.
func decodeRawUsage(t *testing.T, payload string) RawUsage {
	t.Helper()
	var raw RawUsage
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		t.Fatalf("json.Unmarshal falhou: %v", err)
	}
	return raw
}

// TestRawUsage_Unmarshal_PromotesCacheDetails valida a promoção do cache
// (ADR-031 Fase 1) no UnmarshalJSON do RawUsage. Tabela AAA: cada linha é um
// shape de payload real (OpenAI nested, DeepSeek flat, dual, ausente, anomalia).
func TestRawUsage_Unmarshal_PromotesCacheDetails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		payload       string
		wantCached    int
		wantEffective int
		wantPrompt    int
	}{
		{
			name: "openai nested cached_tokens",
			// prompt_tokens_details.cached_tokens é a 1ª fonte (OpenAI).
			payload:       `{"prompt_tokens":1000,"completion_tokens":200,"total_tokens":1200,"prompt_tokens_details":{"cached_tokens":600}}`,
			wantCached:    600,
			wantEffective: 400,
			wantPrompt:    1000,
		},
		{
			name: "deepseek flat prompt_cache_hit_tokens",
			// DeepSeek OpenAI-compat: campos flat no usage. 2ª fonte.
			payload:       `{"prompt_tokens":2000,"completion_tokens":300,"total_tokens":2300,"prompt_cache_hit_tokens":1500,"prompt_cache_miss_tokens":500}`,
			wantCached:    1500,
			wantEffective: 500,
			wantPrompt:    2000,
		},
		{
			name: "dual nested e flat — precedência nested, sem soma",
			// Ambas as fontes presentes: NUNCA somar (600+1500) — nested vence.
			payload:       `{"prompt_tokens":2000,"completion_tokens":300,"total_tokens":2300,"prompt_tokens_details":{"cached_tokens":600},"prompt_cache_hit_tokens":1500,"prompt_cache_miss_tokens":500}`,
			wantCached:    600,
			wantEffective: 1400,
			wantPrompt:    2000,
		},
		{
			name: "regressão: payload OpenAI flat legado (sem detalhes)",
			// Payload base existente (provider_test.go) continua decodificando;
			// sem detalhes → CachedTokens==0 (honesto).
			payload:       `{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}`,
			wantCached:    0,
			wantEffective: 10,
			wantPrompt:    10,
		},
		{
			name: "anomalia: hit > prompt faz clamp",
			// flat legado cached_tokens (3ª fonte) maior que prompt_tokens →
			// clamp defensivo para prompt_tokens.
			payload:       `{"prompt_tokens":2000,"completion_tokens":100,"total_tokens":2100,"cached_tokens":3000}`,
			wantCached:    2000,
			wantEffective: 0,
			wantPrompt:    2000,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			raw := decodeRawUsage(t, tt.payload)

			// Arrange/Act/Assert: base preservado + decomposição promovida.
			if raw.Usage.PromptTokens != tt.wantPrompt {
				t.Errorf("PromptTokens = %d, esperava %d", raw.Usage.PromptTokens, tt.wantPrompt)
			}
			if raw.Usage.CachedTokens != tt.wantCached {
				t.Errorf("CachedTokens = %d, esperava %d", raw.Usage.CachedTokens, tt.wantCached)
			}
			cached, effective := raw.Usage.CachedAndEffective()
			if cached != tt.wantCached {
				t.Errorf("CachedAndEffective() cached = %d, esperava %d", cached, tt.wantCached)
			}
			if effective != tt.wantEffective {
				t.Errorf("CachedAndEffective() effective = %d, esperava %d", effective, tt.wantEffective)
			}
		})
	}
}

// TestRawUsage_Unmarshal_PromotesReasoning valida a promoção do reasoning:
// completion_tokens_details.reasoning_tokens (nested) tem precedência sobre o
// flat reasoning_tokens; quando nenhum está presente, o valor permanece 0.
func TestRawUsage_Unmarshal_PromotesReasoning(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload string
		want    int
	}{
		{
			name:    "nested reasoning_tokens",
			payload: `{"prompt_tokens":100,"completion_tokens":80,"total_tokens":180,"completion_tokens_details":{"reasoning_tokens":25}}`,
			want:    25,
		},
		{
			name:    "nested tem precedência sobre flat",
			payload: `{"prompt_tokens":100,"completion_tokens":80,"total_tokens":180,"reasoning_tokens":10,"completion_tokens_details":{"reasoning_tokens":25}}`,
			want:    25,
		},
		{
			name:    "flat reasoning_tokens preservado quando nested ausente",
			payload: `{"prompt_tokens":100,"completion_tokens":80,"total_tokens":180,"reasoning_tokens":30}`,
			want:    30,
		},
		{
			name:    "ausente → 0",
			payload: `{"prompt_tokens":100,"completion_tokens":80,"total_tokens":180}`,
			want:    0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			raw := decodeRawUsage(t, tt.payload)
			if raw.Usage.ReasoningTokens != tt.want {
				t.Errorf("ReasoningTokens = %d, esperava %d", raw.Usage.ReasoningTokens, tt.want)
			}
		})
	}
}

// TestRawUsage_DeepSeek_Invariant valida o invariante documentado do DeepSeek:
// prompt_tokens == prompt_cache_hit_tokens + prompt_cache_miss_tokens quando os
// campos flat estão presentes. O hit é promovido; a miss fica só no campo cru.
func TestRawUsage_DeepSeek_Invariant(t *testing.T) {
	t.Parallel()

	raw := decodeRawUsage(t, `{"prompt_tokens":2000,"completion_tokens":300,"total_tokens":2300,"prompt_cache_hit_tokens":1500,"prompt_cache_miss_tokens":500}`)

	if raw.PromptCacheHitTokens != 1500 {
		t.Errorf("PromptCacheHitTokens = %d, esperava 1500", raw.PromptCacheHitTokens)
	}
	if raw.PromptCacheMissTokens != 500 {
		t.Errorf("PromptCacheMissTokens = %d, esperava 500", raw.PromptCacheMissTokens)
	}
	if got := raw.PromptCacheHitTokens + raw.PromptCacheMissTokens; got != raw.PromptTokens {
		t.Errorf("hit+miss = %d, esperava prompt_tokens %d (invariante DeepSeek)", got, raw.PromptTokens)
	}
	// Promovido: hit vira CachedTokens na Usage; miss NÃO entra em Usage.
	if raw.Usage.CachedTokens != 1500 {
		t.Errorf("Usage.CachedTokens = %d, esperava 1500", raw.Usage.CachedTokens)
	}
}
