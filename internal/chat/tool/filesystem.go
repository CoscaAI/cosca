// Package tool provides the tool registry for managing, discovering, and
// executing tools within the Cosca chat agent system.
package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/CoscaAI/cosca/internal/chat"
)

// pathValidator abstracts path validation to avoid importing the sandbox
// package. Implementations check that a relative path stays within the
// allowed workspace boundaries.
type pathValidator interface {
	Validate(path string) error
}

// errorResult creates a ToolResult containing only an error message.
// This is the standard way to report expected execution errors (e.g. file
// not found, path validation failure).
func errorResult(msg string) *chat.ToolResult {
	return &chat.ToolResult{Error: msg}
}

// mustMarshalSchema is a helper that marshals a JSON Schema map into
// json.RawMessage. It panics on error because schema definitions are
// static and a marshal failure is a programming error.
func mustMarshalSchema(v map[string]any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic("tool: failed to marshal schema: " + err.Error())
	}
	return data
}

// ─── ReadTool ─────────────────────────────────────────────────────────────────

// ReadTool reads the contents of a file. The path must be relative to the
// workspace and must pass the pathValidator (sandbox rails).
type ReadTool struct {
	workspace string
	rails     pathValidator
	schema    json.RawMessage
}

// NewReadTool creates a new ReadTool bound to the given workspace directory.
// The validator is used to check that requested paths stay within the
// workspace boundary.
func NewReadTool(workspace string, validator pathValidator) *ReadTool {
	return &ReadTool{
		workspace: workspace,
		rails:     validator,
		schema: mustMarshalSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to read, relative to the workspace",
				},
			},
			"required": []string{"path"},
		}),
	}
}

// Name returns the tool identifier.
func (t *ReadTool) Name() string { return "read" }

// Description returns a human-readable description of the tool.
func (t *ReadTool) Description() string { return "Read the contents of a file" }

// Schema returns the JSON Schema describing the tool's parameters.
func (t *ReadTool) Schema() json.RawMessage { return t.schema }

// Execute runs the tool with the given JSON-encoded parameters.
func (t *ReadTool) Execute(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	var input struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return errorResult("invalid params: " + err.Error()), nil
	}

	if t.rails == nil {
		return errorResult("path validation not available"), nil
	}
	if err := t.rails.Validate(input.Path); err != nil {
		return errorResult(err.Error()), nil
	}

	data, err := os.ReadFile(filepath.Join(t.workspace, input.Path))
	if err != nil {
		return errorResult("read failed: " + err.Error()), nil
	}

	return &chat.ToolResult{Output: string(data)}, nil
}

// Validate checks whether the given JSON parameters conform to the tool's
// expected schema.
func (t *ReadTool) Validate(params json.RawMessage) error {
	var input struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}
	if input.Path == "" {
		return errors.New("path is required")
	}
	return nil
}

// ─── WriteTool ────────────────────────────────────────────────────────────────

// WriteTool writes or overwrites a file. If the file already exists, a backup
// is created with a .bak suffix before overwriting. All paths must be relative
// to the workspace and pass the pathValidator.
type WriteTool struct {
	workspace string
	rails     pathValidator
	schema    json.RawMessage
}

// NewWriteTool creates a new WriteTool bound to the given workspace directory.
func NewWriteTool(workspace string, validator pathValidator) *WriteTool {
	return &WriteTool{
		workspace: workspace,
		rails:     validator,
		schema: mustMarshalSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to write, relative to the workspace",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Content to write to the file",
				},
			},
			"required": []string{"path", "content"},
		}),
	}
}

// Name returns the tool identifier.
func (t *WriteTool) Name() string { return "write" }

// Description returns a human-readable description of the tool.
func (t *WriteTool) Description() string {
	return "Write or overwrite a file (creates a .bak backup of the original if it exists)"
}

// Schema returns the JSON Schema describing the tool's parameters.
func (t *WriteTool) Schema() json.RawMessage { return t.schema }

// Execute runs the tool with the given JSON-encoded parameters.
func (t *WriteTool) Execute(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	var input struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return errorResult("invalid params: " + err.Error()), nil
	}

	if t.rails == nil {
		return errorResult("path validation not available"), nil
	}
	if err := t.rails.Validate(input.Path); err != nil {
		return errorResult(err.Error()), nil
	}

	fullPath := filepath.Join(t.workspace, input.Path)

	// Backup the original file if it exists
	if _, err := os.Stat(fullPath); err == nil {
		original, err := os.ReadFile(fullPath)
		if err != nil {
			return errorResult("backup failed: " + err.Error()), nil
		}
		backupPath := fullPath + ".bak"
		if err := os.WriteFile(backupPath, original, 0644); err != nil {
			return errorResult("backup failed: " + err.Error()), nil
		}
	}

	if err := os.WriteFile(fullPath, []byte(input.Content), 0644); err != nil {
		return errorResult("write failed: " + err.Error()), nil
	}

	return &chat.ToolResult{Output: "file written successfully"}, nil
}

// Validate checks whether the given JSON parameters conform to the tool's
// expected schema.
func (t *WriteTool) Validate(params json.RawMessage) error {
	var input struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}
	if input.Path == "" {
		return errors.New("path is required")
	}
	if input.Content == "" {
		return errors.New("content is required")
	}
	return nil
}

// ─── EditTool ─────────────────────────────────────────────────────────────────

// EditTool performs a find-and-replace operation on a file. It reads the file,
// verifies the old string appears exactly once, replaces it with the new string,
// and writes the result back. Paths must be relative to the workspace.
type EditTool struct {
	workspace string
	rails     pathValidator
	schema    json.RawMessage
}

// NewEditTool creates a new EditTool bound to the given workspace directory.
func NewEditTool(workspace string, validator pathValidator) *EditTool {
	return &EditTool{
		workspace: workspace,
		rails:     validator,
		schema: mustMarshalSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to edit, relative to the workspace",
				},
				"old_string": map[string]any{
					"type":        "string",
					"description": "Text to search for (must appear exactly once)",
				},
				"new_string": map[string]any{
					"type":        "string",
					"description": "Text to replace the old string with",
				},
			},
			"required": []string{"path", "old_string", "new_string"},
		}),
	}
}

// Name returns the tool identifier.
func (t *EditTool) Name() string { return "edit" }

// Description returns a human-readable description of the tool.
func (t *EditTool) Description() string {
	return "Perform a find-and-replace edit on a file (old_string must appear exactly once)"
}

// Schema returns the JSON Schema describing the tool's parameters.
func (t *EditTool) Schema() json.RawMessage { return t.schema }

// Execute runs the tool with the given JSON-encoded parameters.
func (t *EditTool) Execute(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	var input struct {
		Path      string `json:"path"`
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return errorResult("invalid params: " + err.Error()), nil
	}

	if t.rails == nil {
		return errorResult("path validation not available"), nil
	}
	if err := t.rails.Validate(input.Path); err != nil {
		return errorResult(err.Error()), nil
	}

	fullPath := filepath.Join(t.workspace, input.Path)

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return errorResult("read failed: " + err.Error()), nil
	}

	content := string(data)

	// Count occurrences of old_string
	count := strings.Count(content, input.OldString)
	switch {
	case count == 0:
		return errorResult("old_string not found in file"), nil
	case count > 1:
		return errorResult(fmt.Sprintf("old_string appears %d times; expected exactly 1", count)), nil
	}

	// Replace (safe because we verified count == 1)
	newContent := strings.Replace(content, input.OldString, input.NewString, 1)

	if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
		return errorResult("write failed: " + err.Error()), nil
	}

	return &chat.ToolResult{Output: "file edited successfully"}, nil
}

// Validate checks whether the given JSON parameters conform to the tool's
// expected schema.
func (t *EditTool) Validate(params json.RawMessage) error {
	var input struct {
		Path      string `json:"path"`
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}
	if input.Path == "" {
		return errors.New("path is required")
	}
	if input.OldString == "" {
		return errors.New("old_string is required")
	}
	return nil
}

// ─── GlobTool ─────────────────────────────────────────────────────────────────

// GlobTool lists files matching a glob pattern within a directory. The path
// is relative to the workspace and optional (defaults to workspace root).
// Uses path/filepath.Glob under the hood.
type GlobTool struct {
	workspace string
	rails     pathValidator
	schema    json.RawMessage
}

// NewGlobTool creates a new GlobTool bound to the given workspace directory.
func NewGlobTool(workspace string, validator pathValidator) *GlobTool {
	return &GlobTool{
		workspace: workspace,
		rails:     validator,
		schema: mustMarshalSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Glob pattern to match (e.g. *.go, **/*.go). Uses Go's filepath.Glob semantics.",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Directory to search in, relative to workspace (optional, defaults to workspace root)",
				},
			},
			"required": []string{"pattern"},
		}),
	}
}

// Name returns the tool identifier.
func (t *GlobTool) Name() string { return "glob" }

// Description returns a human-readable description of the tool.
func (t *GlobTool) Description() string {
	return "List files matching a glob pattern within the workspace"
}

// Schema returns the JSON Schema describing the tool's parameters.
func (t *GlobTool) Schema() json.RawMessage { return t.schema }

// Execute runs the tool with the given JSON-encoded parameters.
func (t *GlobTool) Execute(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	var input struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return errorResult("invalid params: " + err.Error()), nil
	}

	if input.Pattern == "" {
		return errorResult("pattern is required"), nil
	}

	// Validate the base path if provided
	base := t.workspace
	if input.Path != "" {
		if t.rails == nil {
			return errorResult("path validation not available"), nil
		}
		if err := t.rails.Validate(input.Path); err != nil {
			return errorResult(err.Error()), nil
		}
		base = filepath.Join(t.workspace, input.Path)
	}

	// Prevent absolute patterns from bypassing workspace isolation
	if filepath.IsAbs(input.Pattern) {
		return errorResult("pattern must be relative"), nil
	}

	// Build the full glob pattern
	globPattern := filepath.Join(base, input.Pattern)

	// Reject patterns that escape the base directory. filepath.Join cleans
	// the path, so a pattern like "../../etc/*" resolves OUTSIDE the
	// workspace (e.g. to "/etc/*") while still being "relative". The prefix
	// check catches every such escape before globbing.
	baseClean := filepath.Clean(base)
	if globPattern != baseClean &&
		!strings.HasPrefix(globPattern, baseClean+string(filepath.Separator)) {
		return errorResult("pattern escapes workspace"), nil
	}

	matches, err := filepath.Glob(globPattern)
	if err != nil {
		return errorResult("glob failed: " + err.Error()), nil
	}

	if len(matches) == 0 {
		return &chat.ToolResult{Output: "no matches found"}, nil
	}

	// Make matches relative to workspace, re-validating each one through the
	// workspace rails (defense in depth): this filters out blocked
	// directories (.git, .cosca/data, ...) and symlink escapes that the
	// glob itself cannot see.
	relMatches := make([]string, 0, len(matches))
	for _, m := range matches {
		rel, err := filepath.Rel(t.workspace, m)
		if err != nil {
			continue
		}
		if err := t.rails.Validate(rel); err != nil {
			continue
		}
		relMatches = append(relMatches, rel)
	}

	if len(relMatches) == 0 {
		return &chat.ToolResult{Output: "no matches found"}, nil
	}

	return &chat.ToolResult{Output: strings.Join(relMatches, "\n")}, nil
}

// Validate checks whether the given JSON parameters conform to the tool's
// expected schema.
func (t *GlobTool) Validate(params json.RawMessage) error {
	var input struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}
	if input.Pattern == "" {
		return errors.New("pattern is required")
	}
	return nil
}
