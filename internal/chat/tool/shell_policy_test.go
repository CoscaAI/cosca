package tool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/execpolicy"
)

// fakeSandbox records the commands executed and never actually runs them.
type fakeSandbox struct {
	commands []chat.Command
}

func (f *fakeSandbox) Execute(_ context.Context, cmd chat.Command, _ chat.SandboxMode) (*chat.SandboxResult, error) {
	f.commands = append(f.commands, cmd)
	return &chat.SandboxResult{Stdout: "ok", ExitCode: 0}, nil
}
func (f *fakeSandbox) Mode() chat.SandboxMode    { return chat.SandboxWorkspace }
func (f *fakeSandbox) ValidatePath(string) error { return nil }

func newTestShell(policy *execpolicy.Policy) (*ShellTool, *fakeSandbox) {
	sb := &fakeSandbox{}
	return NewShellTool("/workspace", sb, WithExecPolicy(policy)), sb
}

func TestShellToolForbiddenCommand(t *testing.T) {
	policy, err := execpolicy.LoadYAML([]byte(`
rules:
  - pattern: ["git", "reset", "--hard"]
    decision: forbidden
    justification: "operação destrutiva"
    match: [["git", "reset", "--hard"]]
  - pattern: ["ls"]
    decision: allow
    match: [["ls", "-l"]]
`))
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	tool, sb := newTestShell(policy)

	res, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"git reset --hard"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Error == "" {
		t.Fatal("expected policy error for forbidden command")
	}
	if !strings.Contains(res.Error, "operação destrutiva") {
		t.Errorf("error does not include justification: %q", res.Error)
	}
	if len(sb.commands) != 0 {
		t.Errorf("forbidden command was executed: %v", sb.commands)
	}
}

func TestShellToolAllowedCommand(t *testing.T) {
	policy, err := execpolicy.LoadYAML([]byte(`
rules:
  - pattern: ["ls"]
    decision: allow
    match: [["ls", "-l"]]
`))
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	tool, sb := newTestShell(policy)

	res, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"ls -l"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Error != "" {
		t.Errorf("unexpected error: %q", res.Error)
	}
	if len(sb.commands) != 1 || sb.commands[0].Args[0] != "sh" {
		t.Errorf("expected one sandboxed sh -c execution, got %v", sb.commands)
	}
}

func TestShellToolPromptAllowsWithLog(t *testing.T) {
	policy, err := execpolicy.LoadYAML([]byte(`
rules:
  - pattern: ["cp"]
    decision: prompt
    match: [["cp", "foo", "bar"]]
`))
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	tool, sb := newTestShell(policy)

	res, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"cp foo bar"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Error != "" {
		t.Errorf("prompt command should execute (approval flow not wired): %q", res.Error)
	}
	if len(sb.commands) != 1 {
		t.Errorf("prompt command was not executed: %v", sb.commands)
	}
}

func TestShellToolNoPolicyUnchanged(t *testing.T) {
	tool, sb := newTestShell(nil)

	res, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"git reset --hard"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Error != "" {
		t.Errorf("no policy must keep current behavior: %q", res.Error)
	}
	if len(sb.commands) != 1 {
		t.Errorf("expected command to run without policy, got %v", sb.commands)
	}
}
