package tool

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// WebSearchTool enables agents to search the web via Serper API.
// Requires SERPER_API_KEY environment variable.
type WebSearchTool struct {
	apiKey string
}

// NewWebSearchTool creates a web search tool.
// Uses SERPER_API_KEY from environment if empty string is passed.
func NewWebSearchTool(apiKey string) *WebSearchTool {
	if apiKey == "" {
		apiKey = os.Getenv("SERPER_API_KEY")
	}
	return &WebSearchTool{apiKey: apiKey}
}

// Name returns the tool identifier.
func (t *WebSearchTool) Name() string { return "web_search" }

// Description returns what the tool does.
func (t *WebSearchTool) Description() string {
	return "Search the web for information. Input: search query string. Output: search results with titles and snippets."
}

// IsAvailable returns whether the tool can be used.
func (t *WebSearchTool) IsAvailable() bool {
	return t.apiKey != ""
}

// Execute performs a web search.
func (t *WebSearchTool) Execute(ctx context.Context, input string) (string, error) {
	if !t.IsAvailable() {
		return "", fmt.Errorf("web_search: SERPER_API_KEY not configured")
	}

	// Use net/http for the actual call; for now search() fails closed.
	results, err := t.search(ctx, input)
	if err != nil {
		return "", fmt.Errorf("web_search: %w", err)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Web search results for: %s\n\n", input))
	for i, r := range results {
		b.WriteString(fmt.Sprintf("%d. %s\n   %s\n   %s\n\n", i+1, r.Title, r.Snippet, r.Link))
	}
	return b.String(), nil
}

// SearchResult is a single web search result.
type SearchResult struct {
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	Link    string `json:"link"`
}

// search performs the actual HTTP call to Serper API.
func (t *WebSearchTool) search(ctx context.Context, query string) ([]SearchResult, error) {
	// TODO: Implement HTTP call to Serper API (https://google.serper.dev/search)
	// POST https://google.serper.dev/search
	// Headers: X-API-KEY: $SERPER_API_KEY
	// Body: {"q": query}
	return nil, fmt.Errorf("tool web_search: not implemented (stub) — integrate before use")
}
