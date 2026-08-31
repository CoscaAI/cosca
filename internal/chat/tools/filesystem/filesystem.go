// Package filesystem implements the cross-platform filesystem tools exposed
// to Cosca agents via LLM function calling:
//
//	write_file  — create or overwrite a file
//	read_file   — read a file and return its content
//	edit_file   — find/replace within an existing file
//	list_dir    — list the entries of a directory
//	glob        — find files matching a glob pattern
//
// All tools are pure Go (os/io/filepath) and therefore run on both Linux and
// Windows without the Linux-only bubblewrap sandbox. Paths are always
// validated against the workspace root so the agent can never escape the
// project.
package filesystem

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/chat/tool"
)

// maxReadSize bounds read_file and edit_file so a single tool call cannot
// flood the LLM context with an unbounded file.
const maxReadSize = 2 * 1024 * 1024 // 2 MiB

// pathTracker records the absolute paths that have been successfully read in
// the current session/tool group. It is the deterministic guard that makes
// write_file refuse to blindly overwrite an existing file: a file may only be
// overwritten by write_file if it was first read (read_file / edit_file) in the
// same session. This does NOT depend on the LLM — a rogue or hallucinating
// model cannot destroy code it never read.
//
// It is shared across all filesystem tools created by New() so that a
// read_file in one tool authorizes a write_file in another. It is goroutine
// safe because the executor may run concurrent tool calls.
type pathTracker struct {
	mu    sync.Mutex
	paths map[string]struct{}
}

func newPathTracker() *pathTracker {
	return &pathTracker{paths: make(map[string]struct{})}
}

// markRead records that fullPath was successfully read this session.
func (p *pathTracker) markRead(fullPath string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paths[fullPath] = struct{}{}
}

// wasRead reports whether fullPath was read this session.
func (p *pathTracker) wasRead(fullPath string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.paths[fullPath]
	return ok
}

// errorResult is the standard way to report an expected tool failure
// (validation, missing file, path traversal, ...) as a ToolResult.
func errorResult(msg string) *chat.ToolResult {
	return &chat.ToolResult{Error: msg}
}

// mustMarshalSchema marshals a JSON Schema map into json.RawMessage. It panics
// on error because schemas are static: a marshal failure is a programming bug.
func mustMarshalSchema(v map[string]any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic("filesystem: failed to marshal schema: " + err.Error())
	}
	return data
}

// ─── write_file ───────────────────────────────────────────────────────────────

// WriteFileTool creates or overwrites a file with the given content. Parent
// directories are created automatically. Paths are relative to the workspace.
//
// SECURITY GUARD: write_file NEVER blindly overwrites an existing file. An
// existing file is only overwritten after it has been read (read_file or
// edit_file) in the same session, as tracked by the shared pathTracker. This
// makes destruction of code independent of LLM behaviour: a model that has not
// read a file cannot clobber it.
type WriteFileTool struct {
	validator *Validator
	schema    json.RawMessage
	tracker   *pathTracker
}

// NewWriteFileTool creates a write_file tool bound to the given workspace. It
// uses its own private read tracker; use newWriteFileTool (via New) when you
// want read_file and write_file to share the same session state.
func NewWriteFileTool(workspace string) *WriteFileTool {
	return newWriteFileTool(workspace, newPathTracker())
}

// newWriteFileTool creates a write_file tool sharing the given read tracker.
func newWriteFileTool(workspace string, tracker *pathTracker) *WriteFileTool {
	return &WriteFileTool{
		validator: NewValidator(workspace),
		tracker:   tracker,
		schema: mustMarshalSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File path to create/overwrite, relative to the workspace root",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Full content written to the file",
				},
			},
			"required": []string{"path", "content"},
		}),
	}
}

// Name returns the tool identifier.
func (t *WriteFileTool) Name() string { return "write_file" }

// Description returns a human-readable description of the tool.
func (t *WriteFileTool) Description() string {
	return "Create a NEW file. Parent directories are created automatically. If the file already exists it is NOT overwritten unless it was read first (read_file) in this session; to modify an existing file use edit_file."
}

// Schema returns the JSON Schema describing the tool's parameters.
func (t *WriteFileTool) Schema() json.RawMessage { return t.schema }

// Execute writes the file after validating the path stays inside the workspace
// (lexically AND after resolving symlinks) and after enforcing the overwrite
// guard: an existing file may only be overwritten if it was read this session.
func (t *WriteFileTool) Execute(_ context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	var input struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return errorResult("write_file: invalid params: " + err.Error()), nil
	}
	if input.Path == "" {
		return errorResult("write_file: path is required"), nil
	}

	fullPath, err := t.validator.ResolveVerified(input.Path)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	// ── CRITICAL GUARD: refuse to overwrite an existing file that was not read
	// in this session. A model (regardless of size/quality) must never silently
	// replace code it has not seen. This is enforced deterministically, not by
	// prompting.
	if info, statErr := os.Stat(fullPath); statErr == nil {
		if info.IsDir() {
			return errorResult(fmt.Sprintf("write_file: %q is a directory, not a file", input.Path)), nil
		}
		if !t.tracker.wasRead(fullPath) {
			return errorResult("write_file: file already exists. Read it first (read_file) or use edit_file to modify it, or delete it to recreate."), nil
		}
	} else if !os.IsNotExist(statErr) {
		return errorResult("write_file: " + statErr.Error()), nil
	}

	// Create parent directories automatically (MkdirAll is a no-op if present).
	// The symlink verification in ResolveVerified already ran before this, so
	// MkdirAll cannot escape the workspace through an ancestor symlink.
	if dir := filepath.Dir(fullPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return errorResult("write_file: failed to create parent directory: " + err.Error()), nil
		}
	}

	// ── EXISTENCE-before-WRITE (item 2): creating a brand-new file, but if the
	// parent directory already holds a file with the same stem or extension,
	// surface it as a hint so the agent does not silently introduce a
	// near-duplicate. Informational and non-blocking: creation still proceeds.
	hint := similarSiblingHint(input.Path, fullPath)

	if err := os.WriteFile(fullPath, []byte(input.Content), 0o644); err != nil {
		return errorResult("write_file: write failed: " + err.Error()), nil
	}

	// A fresh write is a form of "having seen" the path — record it so a
	// subsequent write_file to the same file (e.g. to fix a mistake) is allowed
	// within the same session rather than being spuriously rejected.
	t.tracker.markRead(fullPath)

	output := fmt.Sprintf("Successfully wrote %d bytes to %s", len(input.Content), input.Path)
	if hint != "" {
		output += "\n" + hint
	}
	return &chat.ToolResult{Output: output}, nil
}

// similarSiblingHint returns a non-empty advisory when the parent directory of
// the target already contains a file with the same basename stem or the same
// file extension (case-insensitive), warning that a brand-new write_file may be
// about to create a near-duplicate of an existing file. It is purely
// informational — creation is never blocked by this hint.
func similarSiblingHint(inputPath, fullPath string) string {
	dir := filepath.Dir(fullPath)
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return ""
	}
	target := filepath.Base(inputPath)
	targetStem := strings.TrimSuffix(target, filepath.Ext(target))
	targetExt := strings.ToLower(filepath.Ext(target))

	var similar []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == target {
			continue
		}
		stem := strings.TrimSuffix(name, filepath.Ext(name))
		ext := strings.ToLower(filepath.Ext(name))
		if strings.EqualFold(stem, targetStem) || (targetExt != "" && strings.EqualFold(ext, targetExt)) {
			similar = append(similar, name)
		}
	}
	if len(similar) == 0 {
		return ""
	}
	sort.Strings(similar)
	const maxShown = 5
	shown := similar
	if len(shown) > maxShown {
		shown = shown[:maxShown]
		shown = append(shown, "...")
	}
	return fmt.Sprintf("(advisory: similar file(s) already exist in this directory: %s — if one matches your intent, use read_file/edit_file on it instead of creating a new file)", strings.Join(shown, ", "))
}

// Validate checks that the JSON parameters conform to the tool's schema.
func (t *WriteFileTool) Validate(params json.RawMessage) error {
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

// ─── read_file ────────────────────────────────────────────────────────────────

// ReadFileTool reads a file and returns its content. Paths are relative to
// the workspace.
type ReadFileTool struct {
	validator *Validator
	schema    json.RawMessage
	tracker   *pathTracker
}

// NewReadFileTool creates a read_file tool bound to the given workspace. It
// uses its own private read tracker; use newReadFileTool (via New) when you
// want read_file and write_file to share the same session state.
func NewReadFileTool(workspace string) *ReadFileTool {
	return newReadFileTool(workspace, newPathTracker())
}

// newReadFileTool creates a read_file tool sharing the given read tracker.
func newReadFileTool(workspace string, tracker *pathTracker) *ReadFileTool {
	return &ReadFileTool{
		validator: NewValidator(workspace),
		tracker:   tracker,
		schema: mustMarshalSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File path to read, relative to the workspace root",
				},
			},
			"required": []string{"path"},
		}),
	}
}

// Name returns the tool identifier.
func (t *ReadFileTool) Name() string { return "read_file" }

// Description returns a human-readable description of the tool.
func (t *ReadFileTool) Description() string {
	return "Read a file and return its content. Path is relative to the workspace root."
}

// Schema returns the JSON Schema describing the tool's parameters.
func (t *ReadFileTool) Schema() json.RawMessage { return t.schema }

// Execute reads the file after validating the path stays inside the workspace.
func (t *ReadFileTool) Execute(_ context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	var input struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return errorResult("read_file: invalid params: " + err.Error()), nil
	}
	if input.Path == "" {
		return errorResult("read_file: path is required"), nil
	}

	fullPath, err := t.validator.ResolveVerified(input.Path)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return errorResult("read_file: " + err.Error()), nil
	}
	if info.IsDir() {
		return errorResult(fmt.Sprintf("read_file: %q is a directory, not a file", input.Path)), nil
	}
	if info.Size() > maxReadSize {
		return errorResult(fmt.Sprintf("read_file: file size %d exceeds maximum %d bytes", info.Size(), maxReadSize)), nil
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return errorResult("read_file: read failed: " + err.Error()), nil
	}

	// A successful read authorizes a subsequent write_file to overwrite this
	// exact path in the same session (the read → edit/write flow).
	t.tracker.markRead(fullPath)

	return &chat.ToolResult{Output: string(data)}, nil
}

// Validate checks that the JSON parameters conform to the tool's schema.
func (t *ReadFileTool) Validate(params json.RawMessage) error {
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

// ─── edit_file ────────────────────────────────────────────────────────────────

// EditFileTool performs a find-and-replace edit on an existing file. The
// old_string must appear exactly once; otherwise the edit is rejected to avoid
// ambiguous rewrites. Paths are relative to the workspace.
type EditFileTool struct {
	validator *Validator
	schema    json.RawMessage
	tracker   *pathTracker
}

// NewEditFileTool creates an edit_file tool bound to the given workspace.
func NewEditFileTool(workspace string) *EditFileTool {
	return newEditFileTool(workspace, newPathTracker())
}

// newEditFileTool creates an edit_file tool sharing the given read tracker.
func newEditFileTool(workspace string, tracker *pathTracker) *EditFileTool {
	return &EditFileTool{
		validator: NewValidator(workspace),
		tracker:   tracker,
		schema: mustMarshalSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File path to edit, relative to the workspace root",
				},
				"old_string": map[string]any{
					"type":        "string",
					"description": "Exact text to find (must appear exactly once)",
				},
				"new_string": map[string]any{
					"type":        "string",
					"description": "Replacement text for old_string",
				},
			},
			"required": []string{"path", "old_string", "new_string"},
		}),
	}
}

// Name returns the tool identifier.
func (t *EditFileTool) Name() string { return "edit_file" }

// Description returns a human-readable description of the tool.
func (t *EditFileTool) Description() string {
	return "Find and replace text in an existing file. old_string must appear exactly once. Returns a confirmation."
}

// Schema returns the JSON Schema describing the tool's parameters.
func (t *EditFileTool) Schema() json.RawMessage { return t.schema }

// Execute performs the find-and-replace after validating the path.
func (t *EditFileTool) Execute(_ context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	var input struct {
		Path      string `json:"path"`
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return errorResult("edit_file: invalid params: " + err.Error()), nil
	}
	if input.Path == "" {
		return errorResult("edit_file: path is required"), nil
	}
	if input.OldString == "" {
		return errorResult("edit_file: old_string is required"), nil
	}

	fullPath, err := t.validator.ResolveVerified(input.Path)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return errorResult("edit_file: file not found: " + input.Path), nil
		}
		return errorResult("edit_file: read failed: " + err.Error()), nil
	}
	if len(data) > maxReadSize {
		return errorResult(fmt.Sprintf("edit_file: file size %d exceeds maximum %d bytes", len(data), maxReadSize)), nil
	}

	content := string(data)
	count := strings.Count(content, input.OldString)
	switch {
	case count == 0:
		return errorResult("edit_file: old_string not found in file"), nil
	case count > 1:
		return errorResult(fmt.Sprintf("edit_file: old_string appears %d times; expected exactly 1", count)), nil
	}

	newContent := strings.Replace(content, input.OldString, input.NewString, 1)

	if err := os.WriteFile(fullPath, []byte(newContent), 0o644); err != nil {
		return errorResult("edit_file: write failed: " + err.Error()), nil
	}

	// edit_file reads the file to perform the edit, so the path is "seen" and a
	// later write_file overwrite of the same file is permitted this session.
	t.tracker.markRead(fullPath)

	return &chat.ToolResult{Output: "file edited successfully"}, nil
}

// Validate checks that the JSON parameters conform to the tool's schema.
func (t *EditFileTool) Validate(params json.RawMessage) error {
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

// ─── list_dir ─────────────────────────────────────────────────────────────────

// ListDirTool lists the entries of a directory. The path is optional and
// defaults to the workspace root. Paths are relative to the workspace.
type ListDirTool struct {
	validator *Validator
	schema    json.RawMessage
}

// NewListDirTool creates a list_dir tool bound to the given workspace.
func NewListDirTool(workspace string) *ListDirTool {
	return &ListDirTool{
		validator: NewValidator(workspace),
		schema: mustMarshalSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Directory to list, relative to the workspace root (optional; defaults to root)",
				},
				"directory": map[string]any{
					"type":        "string",
					"description": "Alias for path",
				},
			},
		}),
	}
}

// Name returns the tool identifier.
func (t *ListDirTool) Name() string { return "list_dir" }

// Description returns a human-readable description of the tool.
func (t *ListDirTool) Description() string {
	return "List the entries of a directory. Returns each entry's name and whether it is a file or directory."
}

// Schema returns the JSON Schema describing the tool's parameters.
func (t *ListDirTool) Schema() json.RawMessage { return t.schema }

// Execute lists the directory after validating the path.
func (t *ListDirTool) Execute(_ context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	var input struct {
		Path      string `json:"path"`
		Directory string `json:"directory"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return errorResult("list_dir: invalid params: " + err.Error()), nil
	}

	target := input.Path
	if target == "" {
		target = input.Directory
	}
	if target == "" {
		target = "."
	}

	fullPath, err := t.validator.ResolveVerified(target)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return errorResult("list_dir: " + err.Error()), nil
	}

	// Sort by name for deterministic output. os.ReadDir returns entries in
	// directory order, which is not guaranteed to be alphabetical.
	names := make([]string, 0, len(entries))
	type entryLine struct{ name, marker string }
	lines := make([]entryLine, 0, len(entries))
	for _, e := range entries {
		marker := "file"
		if e.IsDir() {
			marker = "dir "
		}
		lines = append(lines, entryLine{name: e.Name(), marker: marker})
		names = append(names, e.Name())
	}
	sort.Strings(names)
	byName := make(map[string]entryLine, len(lines))
	for _, l := range lines {
		byName[l.name] = l
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Contents of %s (%d entries):\n", target, len(entries))
	for _, n := range names {
		fmt.Fprintf(&sb, "  %s  %s\n", byName[n].marker, n)
	}

	return &chat.ToolResult{Output: sb.String()}, nil
}

// Validate checks that the JSON parameters conform to the tool's schema.
func (t *ListDirTool) Validate(params json.RawMessage) error {
	var input struct {
		Path      string `json:"path"`
		Directory string `json:"directory"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}
	return nil
}

// ─── glob ─────────────────────────────────────────────────────────────────────

// GlobTool lists files matching a glob pattern. The pattern is relative to
// the workspace (or an optional subdirectory). Uses path/filepath.Glob.
type GlobTool struct {
	validator *Validator
	schema    json.RawMessage
}

// NewGlobTool creates a glob tool bound to the given workspace.
func NewGlobTool(workspace string) *GlobTool {
	return &GlobTool{
		validator: NewValidator(workspace),
		schema: mustMarshalSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Glob pattern to match (e.g. *.go, **/*.go). Relative to the workspace.",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Base directory to search in, relative to the workspace (optional; defaults to root)",
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
	return "Find files matching a glob pattern within the workspace. Returns relative paths, one per line."
}

// Schema returns the JSON Schema describing the tool's parameters.
func (t *GlobTool) Schema() json.RawMessage { return t.schema }

// Execute finds files matching the pattern after validating the base path.
func (t *GlobTool) Execute(_ context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	var input struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return errorResult("glob: invalid params: " + err.Error()), nil
	}
	if input.Pattern == "" {
		return errorResult("glob: pattern is required"), nil
	}
	if filepath.IsAbs(input.Pattern) {
		return errorResult("glob: pattern must be relative"), nil
	}

	base := t.validator.Workspace()
	if input.Path != "" {
		resolved, err := t.validator.ResolveVerified(input.Path)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		base = resolved
	}

	globalPattern := filepath.Join(base, input.Pattern)
	globalPattern = filepath.Clean(globalPattern)

	// Reject patterns that escape the base directory. filepath.Clean resolves
	// `..` segments, so "../../etc/*" becomes a path OUTSIDE the base. The
	// prefix check (case-insensitive on Windows) catches every such escape
	// before globbing.
	if !withinWorkspace(base, globalPattern) {
		return errorResult("glob: pattern escapes workspace"), nil
	}

	matches, err := filepath.Glob(globalPattern)
	if err != nil {
		return errorResult("glob: glob failed: " + err.Error()), nil
	}
	if len(matches) == 0 {
		return &chat.ToolResult{Output: "no matches found"}, nil
	}

	// Make matches relative to the workspace and re-validate each through the
	// symlink-aware workspace validator (defense in depth — filters blocked
	// directories, residual escapes, and symlinks that resolve outside).
	var relMatches []string
	for _, m := range matches {
		rel, err := filepath.Rel(t.validator.Workspace(), m)
		if err != nil {
			continue
		}
		if _, err := t.validator.ResolveVerified(rel); err != nil {
			continue
		}
		relMatches = append(relMatches, rel)
	}
	sort.Strings(relMatches)

	if len(relMatches) == 0 {
		return &chat.ToolResult{Output: "no matches found"}, nil
	}

	return &chat.ToolResult{Output: strings.Join(relMatches, "\n")}, nil
}

// Validate checks that the JSON parameters conform to the tool's schema.
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

// ─── Registration ─────────────────────────────────────────────────────────────

// New returns all five filesystem tools bound to the given workspace, sharing a
// single session read-tracker so that a read_file authorizes a subsequent
// write_file overwrite of the same path. This is the deterministic overwrite
// guard: an agent cannot destroy code it has not read, regardless of model.
func New(workspace string) []chat.Tool {
	tracker := newPathTracker()
	return []chat.Tool{
		newWriteFileTool(workspace, tracker),
		newReadFileTool(workspace, tracker),
		newEditFileTool(workspace, tracker),
		NewListDirTool(workspace),
		NewGlobTool(workspace),
	}
}

// Register registers all filesystem tools into a concrete tool.Registry bound
// to the given workspace. It returns the registered tools so the caller can
// report the tool count. It is a convenience wrapper around New + Register to
// keep the executor wiring in one place.
func Register(reg *tool.Registry, workspace string) []chat.Tool {
	tools := New(workspace)
	for _, t := range tools {
		reg.Register(t)
	}
	return tools
}
