// Package sandbox provides the sandbox execution layer for the Cosca Chat CLI.
// It implements the chat.Sandbox interface with OS-level isolation via
// bubblewrap. Direct execution is available only through an explicit
// development opt-in.
//
// The Gate supports three isolation modes:
//   - read-only:  direct exec with read-only workspace (advisory without bwrap)
//   - workspace:  bwrap sandbox within workspace bounds (writable workspace)
//   - full:       unrestricted (requires explicit approval)
package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/processutil"
)

// Gate implements chat.Sandbox with OS-level isolation.
// On Linux it uses bubblewrap (bwrap) for namespace-based isolation;
// on other platforms it falls back to direct execution.
type Gate struct {
	workspace    string // absolute path to workspace root
	mode         chat.SandboxMode
	bwrapPath    string // path to bwrap binary (empty if not available)
	workspaceErr error
}

// Command prepares an interactive sandboxed process. It is used by stdio
// transports (MCP), which cannot use Execute's capture-and-wait API.
func (g *Gate) Command(ctx context.Context, cmd chat.Command, mode chat.SandboxMode) (*exec.Cmd, error) {
	if len(cmd.Args) == 0 {
		return nil, fmt.Errorf("sandbox: empty command arguments")
	}
	if err := validateEnv(cmd.Env); err != nil {
		return nil, err
	}
	if err := g.validateWorkspace(); err != nil {
		return nil, err
	}
	if g.bwrapPath == "" {
		if os.Getenv("COSCA_ALLOW_NO_ROOT") != "1" {
			return nil, fmt.Errorf("sandbox unavailable: bubblewrap is required")
		}
		fmt.Fprintln(os.Stderr, "SECURITY WARNING: launching interactive process without sandbox (explicit COSCA_ALLOW_NO_ROOT=1 opt-in)")
		workDir := cmd.WorkDir
		if workDir == "" {
			workDir = g.workspace
		}
		proc := exec.CommandContext(ctx, cmd.Args[0], cmd.Args[1:]...)
		proc.Dir = workDir
		proc.Env = safeEnv(cmd.Env)
		return proc, nil
	}
	return g.bwrapCommand(ctx, cmd, mode)
}

// NewGate creates a sandbox gate for the given workspace.
// The workspace path is resolved to an absolute path if it isn't already.
func NewGate(workspace string, mode chat.SandboxMode) *Gate {
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		absWorkspace = workspace
	}
	g := &Gate{
		workspace: absWorkspace,
		mode:      mode,
		bwrapPath: findBwrap(),
	}
	resolved, err := filepath.EvalSymlinks(absWorkspace)
	if err != nil {
		g.workspaceErr = fmt.Errorf("sandbox: resolve workspace: %w", err)
	} else if resolved == string(filepath.Separator) {
		g.workspaceErr = fmt.Errorf("sandbox: refusing filesystem root as workspace")
	} else if filepath.Clean(resolved) != filepath.Clean(absWorkspace) {
		g.workspaceErr = fmt.Errorf("sandbox: refusing symlinked workspace")
	}
	return g
}

// Mode returns the current sandbox mode.
func (g *Gate) Mode() chat.SandboxMode {
	return g.mode
}

// IsAvailable reports whether the sandbox gate has OS-level isolation
// available (bwrap binary found). When false, the gate still works in
// direct-execution fallback mode but without namespace-level enforcement.
func (g *Gate) IsAvailable() bool {
	return g.bwrapPath != ""
}

// IsVerifiable reports whether a validated, absolute bwrap executable exists.
// IsAvailable is intentionally only a binary-discovery signal; it does not
// promise that the kernel will permit namespaces or that a launch will work.
func (g *Gate) IsVerifiable() bool { return g.bwrapPath != "" && filepath.IsAbs(g.bwrapPath) }

// ValidatePath checks if a path is within the workspace using Rails.
func (g *Gate) ValidatePath(path string) error {
	if err := g.validateWorkspace(); err != nil {
		return err
	}
	return NewRails(g.workspace).Validate(path)
}

func (g *Gate) validateWorkspace() error {
	if g.workspaceErr != nil {
		return g.workspaceErr
	}
	return nil
}

// Execute runs a command with sandbox enforcement.
// The mode parameter can override the gate's default mode for this call.
func (g *Gate) Execute(ctx context.Context, cmd chat.Command, mode chat.SandboxMode) (*chat.SandboxResult, error) {
	switch mode {
	case chat.SandboxReadOnly:
		return g.execReadOnly(ctx, cmd)
	case chat.SandboxWorkspace:
		if g.bwrapPath != "" {
			return g.execBwrap(ctx, cmd, mode)
		}
		return g.execWithoutSandbox(ctx, cmd)
	case chat.SandboxFull:
		return g.execDirect(ctx, cmd)
	default:
		return nil, fmt.Errorf("sandbox: unknown sandbox mode: %v", mode)
	}
}

// execDirect runs a command directly via os/exec without sandbox isolation.
func (g *Gate) execDirect(ctx context.Context, cmd chat.Command) (*chat.SandboxResult, error) {
	if len(cmd.Args) == 0 {
		return nil, fmt.Errorf("sandbox: empty command arguments")
	}
	if os.Getenv("COSCA_ALLOW_NO_ROOT") != "1" {
		return nil, fmt.Errorf("sandbox: direct execution requires explicit COSCA_ALLOW_NO_ROOT=1")
	}
	if err := validateEnv(cmd.Env); err != nil {
		return nil, err
	}
	if err := g.validateWorkspace(); err != nil {
		return nil, err
	}

	execCmd := exec.CommandContext(ctx, cmd.Args[0], cmd.Args[1:]...)

	// Set environment variables (merged with current process env).
	// Always set Env, including when no overrides were supplied. A nil Env
	// inherits the caller's (potentially credential-bearing) environment.
	execCmd.Env = safeEnv(cmd.Env)

	// Set working directory (defaults to workspace root).
	workDir := cmd.WorkDir
	if workDir == "" {
		workDir = g.workspace
	}
	execCmd.Dir = workDir

	// Run through the shared executor: it streams stdout/stderr, enforces an
	// independent idle timeout and hard runtime cap, and terminates the whole
	// process tree (including grandchildren that hold the output pipe open)
	// so a command that daemonises cannot hang the caller or leak processes.
	res, err := processutil.Run(ctx, execCmd, processutil.Config{
		IdleTimeout: cmd.IdleTimeout,
		MaxRuntime:  cmd.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("sandbox: command execution failed: %w", err)
	}

	return &chat.SandboxResult{
		Stdout:   res.Stdout,
		Stderr:   res.Stderr,
		ExitCode: res.ExitCode,
		Duration: res.Duration,
		Status:   string(res.Status),
		IdleFor:  res.IdleFor,
	}, nil
}

var allowedEnv = map[string]struct{}{
	"PATH": {}, "HOME": {}, "LANG": {}, "LC_ALL": {}, "TERM": {}, "TMPDIR": {}, "TZ": {},
	"OPENAI_API_KEY": {}, "ANTHROPIC_API_KEY": {}, "DEEPSEEK_API_KEY": {}, "GITHUB_TOKEN": {},
	"AWS_ACCESS_KEY_ID": {}, "AWS_SECRET_ACCESS_KEY": {}, "AWS_SESSION_TOKEN": {}, "AWS_REGION": {},
	"MCP_TEST_HELPER": {}, "MCP_HANG": {}, "MCP_SHOULD_FAIL": {}, "MCP_TOOLS_LIST": {}, "MCP_TEST_VAR": {},
}

func validateEnv(env map[string]string) error {
	for key := range env {
		if _, ok := allowedEnv[key]; !ok {
			return fmt.Errorf("sandbox: environment variable %q is not allowed", key)
		}
	}
	return nil
}

// ValidateEnvironment applies the same allowlist used at every process
// construction boundary, including callers that provide their own launcher.
func ValidateEnvironment(env map[string]string) error { return validateEnv(env) }

func safeEnv(extra map[string]string) []string {
	keys := make([]string, 0, len(extra))
	for key := range extra {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	// Keep this non-nil even for an empty allowlist: exec.Cmd treats nil as
	// "inherit the parent environment".
	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+extra[key])
	}
	return env
}

func (g *Gate) execWithoutSandbox(ctx context.Context, cmd chat.Command) (*chat.SandboxResult, error) {
	if os.Getenv("COSCA_ALLOW_NO_ROOT") != "1" {
		return nil, fmt.Errorf("sandbox unavailable: bubblewrap is required; set COSCA_ALLOW_NO_ROOT=1 only for explicit development use")
	}
	// Keep the opt-in visible and do not include command/environment values.
	fmt.Fprintln(os.Stderr, "SECURITY WARNING: executing without sandbox (explicit COSCA_ALLOW_NO_ROOT=1 opt-in)")
	return g.execDirect(ctx, cmd)
}

// execReadOnly runs a command with the workspace mounted read-only.
// If bwrap is available, it delegates to execBwrap with SandboxReadOnly mode.
// Without bwrap, it falls back to direct execution (advisory only).
func (g *Gate) execReadOnly(ctx context.Context, cmd chat.Command) (*chat.SandboxResult, error) {
	if g.bwrapPath != "" {
		return g.execBwrap(ctx, cmd, chat.SandboxReadOnly)
	}
	return g.execWithoutSandbox(ctx, cmd)
}
