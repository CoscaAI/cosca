// Package tool provides the tool registry for managing, discovering, and
// executing tools within the Cosca chat agent system.
package tool

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/execpolicy"
	"github.com/rs/zerolog/log"
)

// ShellTool executes shell commands in a sandboxed environment using the
// Sandbox Gate for OS-level isolation (bwrap on Linux, direct fallback on
// other platforms). The tool enforces workspace rails and supports configurable
// timeouts and working directories.
type ShellTool struct {
	workspace string
	sandbox   chat.Sandbox
	policy    *execpolicy.Policy
	schema    json.RawMessage
}

// ShellOption configures a ShellTool.
type ShellOption func(*ShellTool)

// WithExecPolicy attaches an execution policy to the tool. When set, every
// command is evaluated against the policy before it runs: forbidden commands
// are rejected with the rule's justification, and prompt commands are flagged
// but executed (no approval flow is wired). With no policy, execution keeps
// its existing behavior.
func WithExecPolicy(p *execpolicy.Policy) ShellOption {
	return func(t *ShellTool) {
		t.policy = p
	}
}

// NewShellTool creates a new ShellTool bound to the given workspace and sandbox.
// The sandbox provides the execution isolation layer — on Linux this uses bwrap
// for namespace-based isolation.
func NewShellTool(workspace string, sandbox chat.Sandbox, opts ...ShellOption) *ShellTool {
	t := &ShellTool{
		workspace: workspace,
		sandbox:   sandbox,
		schema: mustMarshalSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "Shell command to execute",
				},
				"timeout": map[string]any{
					"type":        "number",
					"description": "Timeout in milliseconds (default 30000)",
					"default":     30000,
				},
				"workdir": map[string]any{
					"type":        "string",
					"description": "Working directory for the command, relative to the workspace",
				},
			},
			"required": []string{"command"},
		}),
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Name returns the tool identifier.
func (t *ShellTool) Name() string { return "shell" }

// Description returns a human-readable description of the tool.
func (t *ShellTool) Description() string {
	return "Execute shell commands in a sandboxed environment"
}

// Schema returns the JSON Schema describing the tool's parameters.
func (t *ShellTool) Schema() json.RawMessage { return t.schema }

// Execute runs a shell command through the sandbox gate. The command is
// executed via `sh -c <command>` within the workspace sandbox. A timeout
// context is derived from the request context to enforce the deadline.
// Returns a JSON object with stdout, stderr, and exit_code fields.
func (t *ShellTool) Execute(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	var input struct {
		Command string `json:"command"`
		Timeout int    `json:"timeout"`
		Workdir string `json:"workdir"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return errorResult("invalid params: " + err.Error()), nil
	}

	if input.Command == "" {
		return errorResult("command is required"), nil
	}

	if input.Timeout <= 0 {
		input.Timeout = 30000
	}

	// Evaluate the command against the configured exec policy before running.
	// A forbidden command is rejected with the rule's justification; a prompt
	// command is flagged but executed (no approval flow is wired). With no
	// policy configured this is a no-op — existing behavior is unchanged.
	if t.policy != nil {
		switch decision, justification := t.policy.Evaluate(execpolicy.Tokenize(input.Command)); decision {
		case execpolicy.Forbidden:
			msg := "command forbidden by exec policy"
			if justification != "" {
				msg += ": " + justification
			}
			return errorResult(msg), nil
		case execpolicy.Prompt:
			if justification != "" {
				log.Warn().Str("reason", justification).
					Msg("exec policy: command requires approval; executing (no approval flow configured)")
			} else {
				log.Warn().Msg("exec policy: command requires approval; executing (no approval flow configured)")
			}
		}
	}

	// Apply timeout via context deadline. The sandbox gate respects context
	// cancellation and will kill the child process when the deadline expires.
	ctx, cancel := context.WithTimeout(ctx, time.Duration(input.Timeout)*time.Millisecond)
	defer cancel()

	// Build the sandbox command. We run through the appropriate shell for the OS:
	// - Unix: `sh -c <command>` for full shell pipeline support
	// - Windows: `cmd /c <command>` for native Windows command execution
	var shellArgs []string
	if runtime.GOOS == "windows" {
		shellArgs = []string{"cmd", "/c", input.Command}
	} else {
		shellArgs = []string{"sh", "-c", input.Command}
	}
	cmd := chat.Command{
		Args:    shellArgs,
		WorkDir: input.Workdir,
		// The tool's `timeout` becomes the hard runtime cap (MaxRuntime) in the
		// executor. Alongside the context deadline above, this lets the executor
		// report hard_timeout and kill the whole process tree.
		Timeout: time.Duration(input.Timeout) * time.Millisecond,
	}

	// Execute through the sandbox gate with workspace-level isolation.
	// This gives the command read/write access within the workspace but
	// blocks network access and workspace-escape attempts.
	result, err := t.sandbox.Execute(ctx, cmd, chat.SandboxWorkspace)
	if err != nil {
		return errorResult("execution failed: " + err.Error()), nil
	}

	// Return structured output as JSON so the caller can inspect stdout,
	// stderr, and the exit code individually.
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
func (t *ShellTool) Validate(params json.RawMessage) error {
	var input struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return err
	}
	if input.Command == "" {
		return errors.New("command is required")
	}
	return nil
}
