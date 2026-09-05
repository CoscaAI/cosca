// Package tool provides the tool registry for managing, discovering, and
// executing tools within the Cosca chat agent system.
package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/CoscaAI/cosca/internal/chat"
)

// allowedGitCommands is the whitelist of read-only git subcommands that the
// GitTool will execute. Any command not in this set is rejected before ever
// reaching the sandbox.
var allowedGitCommands = map[string]bool{
	"status":    true,
	"diff":      true,
	"log":       true,
	"show":      true,
	"blame":     true,
	"branch":    true,
	"ls-files":  true,
	"rev-parse": true,
	"describe":  true,
	"stash":     true,
}

// stashAllowedSubcommands restricts the `stash` command to read-only
// operations. Write operations (push, pop, apply, drop, clear, create, etc.)
// are blocked.
var stashAllowedSubcommands = map[string]bool{
	"list": true,
	"show": true,
}

// GitTool executes read-only git commands against the workspace repository.
// All commands are validated against a whitelist of safe subcommands and
// executed through the sandbox gate in read-only mode to prevent any
// accidental repository mutation.
type GitTool struct {
	workspace string
	sandbox   chat.Sandbox
	schema    json.RawMessage
}

// NewGitTool creates a new GitTool bound to the given workspace and sandbox.
// The tool enforces a strict read-only policy: only whitelisted git
// subcommands are allowed, and execution runs through the sandbox in
// read-only mode.
func NewGitTool(workspace string, sandbox chat.Sandbox) *GitTool {
	return &GitTool{
		workspace: workspace,
		sandbox:   sandbox,
		schema: mustMarshalSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "Git subcommand (e.g. status, diff, log, show, blame, branch, ls-files)",
				},
				"args": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Additional arguments to pass to the git subcommand",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Path within the repo to restrict the command to",
				},
			},
			"required": []string{"command"},
		}),
	}
}

// Name returns the tool identifier.
func (t *GitTool) Name() string { return "git" }

// Description returns a human-readable description of the tool.
func (t *GitTool) Description() string {
	return "Execute git commands (read-only) against the workspace repository"
}

// Schema returns the JSON Schema describing the tool's parameters.
func (t *GitTool) Schema() json.RawMessage { return t.schema }

// Execute runs a read-only git command through the sandbox gate. The command
// is first validated against the allowed subcommands whitelist, then executed
// as `git -C <workspace> <command> <args...>` with optional path restriction.
// Returns a JSON object with stdout, stderr, and exit_code fields.
func (t *GitTool) Execute(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	var input struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
		Path    string   `json:"path"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return errorResult("invalid params: " + err.Error()), nil
	}

	if input.Command == "" {
		return errorResult("command is required"), nil
	}

	// ── Whitelist check ──────────────────────────────────────────────────
	if !allowedGitCommands[input.Command] {
		return errorResult(fmt.Sprintf(
			"git command %q is not allowed. Allowed commands: status, diff, log, show, "+
				"blame, branch, ls-files, rev-parse, describe, stash (list/show only)",
			input.Command,
		)), nil
	}

	// ── Stash subcommand restriction ─────────────────────────────────────
	if input.Command == "stash" {
		subcmd := ""
		if len(input.Args) > 0 {
			subcmd = input.Args[0]
		}
		if !stashAllowedSubcommands[subcmd] {
			return errorResult("stash: only 'list' and 'show' subcommands are allowed"), nil
		}
	}

	// ── Build git arguments ──────────────────────────────────────────────
	gitArgs := []string{"-C", t.workspace, input.Command}
	gitArgs = append(gitArgs, input.Args...)

	// Append path restriction using the `-- <path>` convention supported by
	// most git commands (status, diff, log, show, blame, ls-files).
	if input.Path != "" {
		gitArgs = append(gitArgs, "--", input.Path)
	}

	cmd := chat.Command{
		Args: append([]string{"git"}, gitArgs...),
	}

	// Execute through the sandbox in read-only mode. This provides defence
	// in depth: even if a write command somehow bypasses the whitelist, the
	// sandbox filesystem is mounted read-only.
	result, err := t.sandbox.Execute(ctx, cmd, chat.SandboxReadOnly)
	if err != nil {
		return errorResult("git execution failed: " + err.Error()), nil
	}

	// Return structured output as JSON.
	output, err := json.Marshal(map[string]any{
		"stdout":    result.Stdout,
		"stderr":    result.Stderr,
		"exit_code": result.ExitCode,
	})
	if err != nil {
		return errorResult("failed to marshal result: " + err.Error()), nil
	}

	return &chat.ToolResult{Output: string(output)}, nil
}

// Validate checks whether the given JSON parameters conform to the tool's
// expected schema.
func (t *GitTool) Validate(params json.RawMessage) error {
	var input struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return err
	}
	if input.Command == "" {
		return errors.New("command is required")
	}
	if !allowedGitCommands[input.Command] {
		return fmt.Errorf("git command %q is not in the allowed list", input.Command)
	}
	return nil
}
