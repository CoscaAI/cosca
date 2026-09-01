package orchestration

import (
	"context"
	"encoding/json"
	"runtime"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/chat/executor"
	"github.com/CoscaAI/cosca/internal/chat/sandbox"
	"github.com/CoscaAI/cosca/internal/chat/tool"
)

// dualSandbox implementa TANTO chat.Sandbox (o que a ShellTool espera) QUANTO
// executor.SandboxGate (o que o executor canônico espera), registrando cada
// comando roteado — prova que a execução passa pelo sandbox.
type dualSandbox struct {
	commands []chat.Command
	modes    []chat.SandboxMode
	result   *chat.SandboxResult
	err      error
}

func (s *dualSandbox) Execute(_ context.Context, cmd chat.Command, mode chat.SandboxMode) (*chat.SandboxResult, error) {
	s.commands = append(s.commands, cmd)
	s.modes = append(s.modes, mode)
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

func (s *dualSandbox) IsAvailable() bool { return true }
func (s *dualSandbox) Mode() chat.SandboxMode {
	return chat.SandboxWorkspace
}
func (s *dualSandbox) ValidatePath(_ string) error { return nil }

// newTestToolRunner monta o executor canônico (o mesmo que internal/toolrun
// monta em produção) sobre um sandbox fake, e devolve o ToolRunner — o caminho
// real de execução de ferramentas do orchestration.
func newTestToolRunner(t *testing.T, workspace string, sb *dualSandbox) ToolRunner {
	t.Helper()

	reg := tool.NewRegistry()
	reg.Register(tool.NewShellTool(workspace, sb))
	reg.Register(tool.NewReadTool(workspace, sandbox.NewRails(workspace)))

	ex := executor.New(reg, sb, workspace)
	return NewToolRunner(ex)
}

// TestToolRunner_ExecuteCommand_Sandboxed verifica que um execute_command do
// LLM é roteado pelo executor canônico ATRAVÉS do sandbox (o substituto de
// segurança do jail) — o contrato que o antigo ToolExecutor garantia e que o
// executor canônico agora cumpre com todos os gates.
func TestToolRunner_ExecuteCommand_Sandboxed(t *testing.T) {
	dir := t.TempDir()
	sb := &dualSandbox{result: &chat.SandboxResult{Stdout: "file1\nfile2\n", ExitCode: 0}}
	runner := newTestToolRunner(t, dir, sb)

	// O executor canônico usa o nome REAL da tool de comando: "shell".
	args, _ := json.Marshal(map[string]any{"command": "ls -la"})
	results, err := runner.ExecuteAll(context.Background(), []chat.ToolCall{
		{ID: "tc-sb-1", Type: "function", Function: chat.FunctionCall{Name: "shell", Arguments: string(args)}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error != "" {
		t.Fatalf("unexpected tool error: %s", results[0].Error)
	}
	if len(sb.commands) != 1 {
		t.Fatalf("sandbox.Execute called %d times, want 1", len(sb.commands))
	}
	got := sb.commands[0]
	// No Windows o shell tool roteia via `cmd /c <comando>` — aceita o prefixo.
	if runtime.GOOS == "windows" {
		if len(got.Args) != 3 || got.Args[0] != "cmd" || got.Args[1] != "/c" || got.Args[2] != "ls -la" {
			t.Fatalf("sandbox command args (windows) = %v, want [cmd /c ls -la]", got.Args)
		}
	} else {
		if len(got.Args) != 2 || got.Args[0] != "ls" || got.Args[1] != "-la" {
			t.Fatalf("sandbox command args = %v, want [ls -la]", got.Args)
		}
	}
	if len(sb.modes) != 1 || sb.modes[0] != chat.SandboxWorkspace {
		t.Fatalf("sandbox mode = %v, want %v", sb.modes, chat.SandboxWorkspace)
	}
}

// TestToolRunner_ExecuteCommand_SandboxFailure: quando o sandbox falha, o
// resultado carrega o erro mascarado (sem crash e sem expor texto sensível).
func TestToolRunner_ExecuteCommand_SandboxFailure(t *testing.T) {
	dir := t.TempDir()
	sb := &dualSandbox{err: context.DeadlineExceeded}
	runner := newTestToolRunner(t, dir, sb)

	args, _ := json.Marshal(map[string]any{"command": "ls"})
	results, err := runner.ExecuteAll(context.Background(), []chat.ToolCall{
		{ID: "tc-sb-2", Type: "function", Function: chat.FunctionCall{Name: "shell", Arguments: string(args)}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error == "" {
		t.Fatal("expected error surfaced in result on sandbox failure")
	}
}

// TestToolRunner_UnknownTool: uma tool que não existe no registry canônico
// retorna erro no RESULT (não derruba a execução).
func TestToolRunner_UnknownTool(t *testing.T) {
	dir := t.TempDir()
	sb := &dualSandbox{result: &chat.SandboxResult{Stdout: "ok", ExitCode: 0}}
	runner := newTestToolRunner(t, dir, sb)

	results, err := runner.ExecuteAll(context.Background(), []chat.ToolCall{
		{ID: "tc-unk", Type: "function", Function: chat.FunctionCall{Name: "execute_sql", Arguments: `{"query":"select 1"}`}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error == "" {
		t.Fatal("expected error for unknown tool (the old deriveTools announced tools that did not exist)")
	}
}
