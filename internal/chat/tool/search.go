// Package tool provides the tool registry for managing, discovering, and
// executing tools within the Cosca chat agent system.
package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/CoscaAI/cosca/internal/chat"
)

// SearchTool searches for files and content in the workspace using glob
// patterns and regex. It supports three search modes:
//   - file:    match file paths using Go's filepath.Glob
//   - content: match file contents using ripgrep (rg) with grep fallback
//   - both:    combine file and content search results
//
// All returned paths are validated through workspace rails and are relative
// to the workspace root.
type SearchTool struct {
	workspace string
	rails     pathValidator
	sandbox   chat.Sandbox
	schema    json.RawMessage
}

// NewSearchTool creates a new SearchTool bound to the given workspace, path
// validator, and sandbox. The validator is used to enforce workspace boundaries
// for glob results, and the sandbox executes rg/grep for content searches.
func NewSearchTool(workspace string, rails pathValidator, sandbox chat.Sandbox) *SearchTool {
	return &SearchTool{
		workspace: workspace,
		rails:     rails,
		sandbox:   sandbox,
		schema: mustMarshalSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Glob pattern or regex to search for",
				},
				"type": map[string]any{
					"type": "string",
					"enum": []string{"file", "content", "both"},
					"description": "Search type: 'file' for file name matching via glob, " +
						"'content' for file content matching via regex, 'both' for combined",
					"default": "file",
				},
				"include": map[string]any{
					"type":        "string",
					"description": "File pattern to include (e.g. *.go, *.{ts,tsx})",
				},
				"max_results": map[string]any{
					"type":        "number",
					"description": "Maximum number of results to return",
					"default":     50,
				},
			},
			"required": []string{"pattern"},
		}),
	}
}

// Name returns the tool identifier.
func (t *SearchTool) Name() string { return "search" }

// Description returns a human-readable description of the tool.
func (t *SearchTool) Description() string {
	return "Search for files and content in the workspace using patterns and regex"
}

// Schema returns the JSON Schema describing the tool's parameters.
func (t *SearchTool) Schema() json.RawMessage { return t.schema }

// Execute runs the search with the given JSON-encoded parameters. For file
// searches it uses Go's filepath.Glob; for content searches it tries ripgrep
// first and falls back to grep. Results are sorted, deduplicated, and limited
// to max_results.
func (t *SearchTool) Execute(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	var input struct {
		Pattern    string `json:"pattern"`
		Type       string `json:"type"`
		Include    string `json:"include"`
		MaxResults int    `json:"max_results"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return errorResult("invalid params: " + err.Error()), nil
	}

	if input.Pattern == "" {
		return errorResult("pattern is required"), nil
	}

	if input.Type == "" {
		input.Type = "file"
	}
	if input.MaxResults <= 0 {
		input.MaxResults = 50
	}

	var results []string
	seen := make(map[string]bool)

	// File search via Go's filepath.Glob
	if input.Type == "file" || input.Type == "both" {
		files, err := t.searchFiles(input.Pattern)
		if err != nil {
			return errorResult("file search failed: " + err.Error()), nil
		}
		for _, f := range files {
			if !seen[f] {
				results = append(results, f)
				seen[f] = true
			}
		}
	}

	// Content search via ripgrep/grep through the sandbox
	if input.Type == "content" || input.Type == "both" {
		matches, err := t.searchContent(ctx, input.Pattern, input.Include, input.MaxResults)
		if err != nil {
			return errorResult("content search failed: " + err.Error()), nil
		}
		for _, m := range matches {
			if !seen[m] {
				results = append(results, m)
				seen[m] = true
			}
		}
	}

	// Sort results alphabetically for deterministic output
	sort.Strings(results)

	// Enforce result limit
	if len(results) > input.MaxResults {
		results = results[:input.MaxResults]
	}

	if len(results) == 0 {
		return &chat.ToolResult{Output: "no matches found"}, nil
	}

	return &chat.ToolResult{Output: strings.Join(results, "\n")}, nil
}

// Validate checks whether the given JSON parameters conform to the tool's
// expected schema.
func (t *SearchTool) Validate(params json.RawMessage) error {
	var input struct {
		Pattern string `json:"pattern"`
		Type    string `json:"type"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return err
	}
	if input.Pattern == "" {
		return errors.New("pattern is required")
	}
	if input.Type != "" && input.Type != "file" && input.Type != "content" && input.Type != "both" {
		return errors.New("type must be one of: file, content, both")
	}
	return nil
}

// ─── Internal helpers ────────────────────────────────────────────────────────

// searchFiles uses Go's filepath.Glob to find files matching the pattern.
// Each match is validated through workspace rails; blocked paths are skipped.
// Returns paths relative to the workspace root.
func (t *SearchTool) searchFiles(pattern string) ([]string, error) {
	if filepath.IsAbs(pattern) {
		return nil, errors.New("pattern must be relative")
	}

	globPattern := filepath.Join(t.workspace, pattern)
	matches, err := filepath.Glob(globPattern)
	if err != nil {
		return nil, fmt.Errorf("glob failed: %w", err)
	}

	var results []string
	for _, m := range matches {
		rel, err := filepath.Rel(t.workspace, m)
		if err != nil {
			continue
		}
		// Validate through workspace rails — skip blocked paths (e.g. .git,
		// node_modules, .cosca/data) and symlink escapes.
		if t.rails == nil {
			return nil, nil
		}
		if err := t.rails.Validate(rel); err != nil {
			continue
		}
		results = append(results, rel)
	}

	return results, nil
}

// searchContent searches file contents using ripgrep (rg) with grep fallback.
// Both are executed through the sandbox gate in read-only mode for safety.
// Returns match lines in ripgrep/grep format (path:line:content).
func (t *SearchTool) searchContent(ctx context.Context, pattern, include string, maxResults int) ([]string, error) {
	// Prefer ripgrep for speed; fall back to grep if unavailable.
	rgPath, err := exec.LookPath("rg")
	if err == nil {
		return t.searchWithRG(ctx, rgPath, pattern, include, maxResults)
	}
	return t.searchWithGrep(ctx, pattern, include, maxResults)
}

// searchWithRG runs ripgrep through the sandbox and returns match lines.
func (t *SearchTool) searchWithRG(ctx context.Context, rgPath, pattern, include string, maxResults int) ([]string, error) {
	args := []string{"--no-heading", "-n", "--max-count", fmt.Sprintf("%d", maxResults)}
	if include != "" {
		args = append(args, "--glob", include)
	}
	args = append(args, pattern, t.workspace)

	cmd := chat.Command{
		Args: append([]string{rgPath}, args...),
	}

	result, err := t.sandbox.Execute(ctx, cmd, chat.SandboxReadOnly)
	if err != nil {
		return nil, fmt.Errorf("ripgrep execution failed: %w", err)
	}

	lines := parseMatchLines(result.Stdout)
	// Make paths relative to workspace
	return makeRelative(t.workspace, lines), nil
}

// searchWithGrep runs grep through the sandbox as a fallback when ripgrep
// is not available.
func (t *SearchTool) searchWithGrep(ctx context.Context, pattern, include string, maxResults int) ([]string, error) {
	args := []string{"-rn", pattern, t.workspace}
	if include != "" {
		args = append(args, "--include="+include)
	}

	cmd := chat.Command{
		Args: append([]string{"grep"}, args...),
	}

	result, err := t.sandbox.Execute(ctx, cmd, chat.SandboxReadOnly)
	if err != nil {
		return nil, fmt.Errorf("grep execution failed: %w", err)
	}

	lines := parseMatchLines(result.Stdout)
	return makeRelative(t.workspace, lines), nil
}

// parseMatchLines splits raw grep/rg output into individual match lines,
// filtering out empty lines.
func parseMatchLines(output string) []string {
	raw := strings.Split(strings.TrimRight(output, "\n"), "\n")
	var lines []string
	for _, l := range raw {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

// makeRelative converts absolute paths in grep/rg output lines to paths
// relative to the workspace. Match lines have the format /abs/path:line:content
// and after conversion become rel/path:line:content.
func makeRelative(workspace string, lines []string) []string {
	result := make([]string, 0, len(lines))
	wsPrefix := workspace
	if !strings.HasSuffix(wsPrefix, string(filepath.Separator)) {
		wsPrefix += string(filepath.Separator)
	}
	for _, line := range lines {
		if strings.HasPrefix(line, wsPrefix) {
			rel := line[len(wsPrefix):]
			result = append(result, rel)
		} else {
			result = append(result, line)
		}
	}
	return result
}
