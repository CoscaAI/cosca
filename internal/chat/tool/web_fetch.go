package tool

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// WebFetchTool enables agents to fetch and extract text from web pages.
// Uses Jina Reader API. Requires JINA_API_KEY environment variable.
type WebFetchTool struct {
	apiKey string
}

// NewWebFetchTool creates a web fetch tool.
func NewWebFetchTool(apiKey string) *WebFetchTool {
	if apiKey == "" {
		apiKey = os.Getenv("JINA_API_KEY")
	}
	return &WebFetchTool{apiKey: apiKey}
}

func (t *WebFetchTool) Name() string        { return "web_fetch" }
func (t *WebFetchTool) Description() string { return "Fetch and extract text content from a URL." }
func (t *WebFetchTool) IsAvailable() bool   { return t.apiKey != "" }

// Execute fetches content from a URL.
func (t *WebFetchTool) Execute(ctx context.Context, input string) (string, error) {
	if !t.IsAvailable() {
		return "", fmt.Errorf("web_fetch: JINA_API_KEY not configured")
	}

	content, err := t.fetch(ctx, input)
	if err != nil {
		return "", fmt.Errorf("web_fetch: %w", err)
	}

	if len(content) > 8000 {
		content = content[:8000] + "\n... (truncated)"
	}

	return fmt.Sprintf("Content from %s:\n\n%s", input, content), nil
}

func (t *WebFetchTool) fetch(ctx context.Context, url string) (string, error) {
	// TODO: Implement HTTP call to Jina Reader API
	// GET https://r.jina.ai/{url}
	// Headers: Authorization: Bearer $JINA_API_KEY
	// Returns: Markdown extracted from the page
	return "", fmt.Errorf("tool web_fetch: not implemented (stub) — integrate before use")
}

// sanitizeURL performs basic URL validation.
func sanitizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "https://" + raw
	}
	return raw
}
