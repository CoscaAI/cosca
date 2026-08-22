package orchestration

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
)

// recordingSandbox is a chat.Sandbox fake that records every command handed to
// it, so tests can assert that the executor routes execution through the
// per-command sandbox (the security replacement for the jail).
type recordingSandbox struct {
	commands []chat.Command
	modes    []chat.SandboxMode
	result   *chat.SandboxResult
	err      error
}

func (s *recordingSandbox) Execute(_ context.Context, cmd chat.Command, mode chat.SandboxMode) (*chat.SandboxResult, error) {
	s.commands = append(s.commands, cmd)
	s.modes = append(s.modes, mode)
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

func (s *recordingSandbox) Mode() chat.SandboxMode      { return chat.SandboxWorkspace }
func (s *recordingSandbox) ValidatePath(_ string) error { return nil }

// TestToolExecutor_SandboxedExecuteCommand verifies that an execute_command
// tool call is routed through the configured per-command sandbox with the
// parsed command and workspace isolation mode.
func TestToolExecutor_SandboxedExecuteCommand(t *testing.T) {
	dir := t.TempDir()
	sb := &recordingSandbox{result: &chat.SandboxResult{Stdout: "file1\nfile2\n", ExitCode: 0}}
	ex := NewToolExecutor(ToolExecutorConfig{
		WorkspaceDir: dir,
		Sandbox:      sb,
		Timeout:      5 * time.Second,
	})

	result, err := ex.Execute(context.Background(), makeToolCall("tc-sb-1", "execute_command", map[string]any{
		"command": "ls -la",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}
	if len(sb.commands) != 1 {
		t.Fatalf("sandbox.Execute called %d times, want 1", len(sb.commands))
	}
	got := sb.commands[0]
	if len(got.Args) != 2 || got.Args[0] != "ls" || got.Args[1] != "-la" {
		t.Fatalf("sandbox command = %v, want [ls -la]", got.Args)
	}
	if len(sb.modes) != 1 || sb.modes[0] != chat.SandboxWorkspace {
		t.Fatalf("sandbox mode = %v, want %v", sb.modes, chat.SandboxWorkspace)
	}
	if !strings.Contains(result.Content, "file1") {
		t.Fatalf("expected sandbox stdout in result, got: %q", result.Content)
	}
}

// TestToolExecutor_SandboxedExecuteCommand_Failure verifies that a sandbox
// error (e.g. bwrap failure / non-zero exit) maps to the stable, non-sensitive
// command_failed category — never the raw sandbox error text.
func TestToolExecutor_SandboxedExecuteCommand_Failure(t *testing.T) {
	dir := t.TempDir()
	sb := &recordingSandbox{err: context.DeadlineExceeded}
	ex := NewToolExecutor(ToolExecutorConfig{
		WorkspaceDir: dir,
		Sandbox:      sb,
		Timeout:      5 * time.Second,
	})

	result, err := ex.Execute(context.Background(), makeToolCall("tc-sb-2", "execute_command", map[string]any{
		"command": "pwd",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The stable, non-sensitive category is returned as tool content — never
	// the raw sandbox error text (which could leak paths, env values, or
	// bwrap diagnostics).
	if !strings.Contains(result.Content, "command_failed") {
		t.Fatalf("expected stable command_failed category, got content: %q", result.Content)
	}
	if strings.Contains(result.Content, "deadline") || strings.Contains(result.Content, "sandbox") {
		t.Fatalf("result leaked sandbox internals: %q", result.Content)
	}
}

// TestToolExecutor_WithoutSandboxKeepsDirectExec ensures the jail-scoped
// commands (which run inside the enclosing jail) keep the allowlisted direct
// execution path when no sandbox is configured.
func TestToolExecutor_WithoutSandboxKeepsDirectExec(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("comando POSIX 'pwd' não existe como executável no Windows — exec direto retorna command_failed")
	}
	dir := t.TempDir()
	ex := NewToolExecutor(ToolExecutorConfig{
		WorkspaceDir: dir,
		Timeout:      5 * time.Second,
	})

	result, err := ex.Execute(context.Background(), makeToolCall("tc-direct-1", "execute_command", map[string]any{
		"command": "pwd",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}
	if strings.TrimSpace(result.Content) != dir {
		t.Fatalf("pwd output = %q, want workspace %q", strings.TrimSpace(result.Content), dir)
	}
}

// TestNewEngineWiresSandboxToToolExecutor verifies the full wiring path used by
// the terminal: OrchestratorConfig.Sandbox flows through NewEngine into the
// ToolExecutor, so an LLM-issued execute_command runs inside the sandbox.
func TestNewEngineWiresSandboxToToolExecutor(t *testing.T) {
	dir := t.TempDir()
	sb := &recordingSandbox{result: &chat.SandboxResult{Stdout: dir, ExitCode: 0}}

	provider := newMockChatProvider("test", "test-model")
	calls := 0
	provider.chatFn = func(_ context.Context, _ []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
		calls++
		if calls == 1 {
			return &chat.ChatResponse{
				ID:    "resp-sb",
				Model: "test-model",
				Choices: []chat.Choice{
					{
						Index: 0,
						Message: chat.Message{
							Role:      chat.RoleAssistant,
							Content:   "Running the command.",
							ToolCalls: []chat.ToolCall{makeToolCall("tc-eng-1", "execute_command", map[string]any{"command": "pwd"})},
						},
						FinishReason: chat.FinishReasonToolCalls,
					},
				},
				Usage: chat.Usage{TotalTokens: 20},
			}, nil
		}
		return &chat.ChatResponse{
			ID:    "resp-final",
			Model: "test-model",
			Choices: []chat.Choice{
				{
					Index: 0,
					Message: chat.Message{
						Role:    chat.RoleAssistant,
						Content: "Done.",
					},
					FinishReason: chat.FinishReasonStop,
				},
			},
			Usage: chat.Usage{TotalTokens: 10},
		}, nil
	}

	config := DefaultOrchestratorConfig()
	config.WorkspaceDir = dir
	config.Sandbox = sb

	engine := NewEngine(nil, nil, nil, nil, nil, provider, config, nil)
	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}

	_, err := engine.Execute(context.Background(), &Request{Prompt: "list the files", ID: "req-sb-1"})
	if err != nil {
		t.Fatalf("engine.Execute returned error: %v", err)
	}

	if len(sb.commands) != 1 {
		t.Fatalf("sandbox.Execute called %d times, want 1 (terminal tool execution must be sandboxed)", len(sb.commands))
	}
	if got := sb.commands[0].Args; len(got) != 1 || got[0] != "pwd" {
		t.Fatalf("sandbox command = %v, want [pwd]", got)
	}
	if len(sb.modes) != 1 || sb.modes[0] != chat.SandboxWorkspace {
		t.Fatalf("sandbox mode = %v, want %v", sb.modes, chat.SandboxWorkspace)
	}
}
