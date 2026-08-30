package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// TestCallCLI_PermittedCommand executa um comando PERMITIDO (status) e
// confirma que o CLIExec é chamado e o output retorna.
func TestCallCLI_PermittedCommand(t *testing.T) {
	var called [][]string
	eng := NewEngine(WithKernel(nil), WithCLIExec(func(args []string) (string, error) {
		called = append(called, args)
		return "COSCA OK", nil
	}))

	res, err := eng.Call(context.Background(), ToolCLI, rawArgs(t, `{"args":["status"]}`))
	if err != nil {
		t.Fatalf("Call falhou: %v", err)
	}
	if res.IsError {
		t.Fatalf("esperava sucesso, veio IsError=true: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, "COSCA OK") {
		t.Fatalf("output inesperado: %q", res.Content[0].Text)
	}
	if len(called) != 1 || len(called[0]) == 0 || called[0][0] != "status" {
		t.Fatalf("CLIExec nao chamado com status: %v", called)
	}
}

// TestCallCLI_DeniedCommand bloqueia comando FORA da allowlist (ex: run/exec)
// — default-deny, nunca executa.
func TestCallCLI_DeniedCommand(t *testing.T) {
	var called bool
	eng := NewEngine(WithKernel(nil), WithCLIExec(func(args []string) (string, error) {
		called = true
		return "", nil
	}))

	for _, denied := range []string{"run", "exec", "delegate", "terminal", "rm", "init"} {
		res, err := eng.Call(context.Background(), ToolCLI, rawArgs(t, fmt.Sprintf(`{"args":["%s"]}`, denied)))
		if err != nil {
			t.Fatalf("Call falhou (deveria devolver IsError, nao erro): %v", err)
		}
		if !res.IsError {
			t.Fatalf("%q deveria ser bloqueado (allowlist default-deny), mas executou", denied)
		}
	}
	if called {
		t.Fatal("CLIExec NAO deveria executar comando negado")
	}
}

// TestCallCLI_NoArgs devolve erro claro sem args.
func TestCallCLI_NoArgs(t *testing.T) {
	eng := NewEngine(WithKernel(nil), WithCLIExec(func(args []string) (string, error) {
		t.Fatal("não deveria executar sem args")
		return "", nil
	}))
	res, err := eng.Call(context.Background(), ToolCLI, rawArgs(t, `{}`))
	if err != nil {
		t.Fatalf("deveria devolver IsError, nao erro: %v", err)
	}
	if !res.IsError {
		t.Fatal("sem args deveria ser erro")
	}
}

func rawArgs(t *testing.T, s string) []byte {
	t.Helper()
	return []byte(s)
}
