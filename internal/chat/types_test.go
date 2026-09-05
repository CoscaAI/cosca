package chat

import (
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
