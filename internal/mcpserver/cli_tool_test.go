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

// TestCallSelf_InspectsOrgans valida que cosca.self reporta os órgãos
// injetados e o estado operacional do cérebro.
func TestCallSelf_InspectsOrgans(t *testing.T) {
	eng := NewEngine(WithKernel(nil))
	res, err := eng.Call(context.Background(), ToolSelf, rawArgs(t, `{}`))
	if err != nil {
		t.Fatalf("Call self: %v", err)
	}
	if res.IsError {
		t.Fatalf("esperava sucesso, veio IsError=true: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, "kernel") {
		t.Fatalf("output nao contem organs: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, "operational") || !strings.Contains(res.Content[0].Text, "unavailable") {
		t.Fatalf("output nao tem estados operational/unavailable: %s", res.Content[0].Text)
	}
}

// TestCallWeb_SSRFBlocked valida que cosca.web bloqueia URL apontando para
// rede interna (host que resolve para IP privado) — fail-closed via SSRF guard.
func TestCallWeb_SSRFBlocked(t *testing.T) {
	eng := NewEngine(WithKernel(nil))
	// URL que resolve para loopback (localhost) deve ser bloqueada.
	_, err := eng.Call(context.Background(), ToolWeb, rawArgs(t, `{"url":"http://localhost:3000"}`))
	if err == nil {
		t.Fatal("URL localhost deveria ser bloqueada pelo SSRF guard")
	}
	if !strings.Contains(err.Error(), "não é público") {
		t.Fatalf("erro deveria ser instrutivo (SSRF): %v", err)
	}
}

// TestCallWeb_BadScheme valida que apenas http/https são aceitos.
func TestCallWeb_BadScheme(t *testing.T) {
	eng := NewEngine(WithKernel(nil))
	_, err := eng.Call(context.Background(), ToolWeb, rawArgs(t, `{"url":"file:///etc/passwd"}`))
	if err == nil {
		t.Fatal("file:// deveria ser rejeitado (apenas http/https)")
	}
	if !strings.Contains(err.Error(), "http/https") {
		t.Fatalf("erro deveria indicar apenas http/https: %v", err)
	}
}

// TestCallWeb_ContentIsEnveloped valida o diamante anti prompt-injection: o
// conteúdo de página retornado pelo cosca.web vem ENVELOPADO (marcador
// <cosca-untrusted-data-v1>), para o modelo não seguir instrução da página.
func TestCallWeb_ContentIsEnveloped(t *testing.T) {
	eng := NewEngine(WithKernel(nil))
	res, err := eng.Call(context.Background(), ToolWeb, rawArgs(t, `{"url":"http://example.com","max_len":100}`))
	if err != nil {
		t.Fatalf("Call web: %v", err)
	}
	if res.IsError {
		t.Fatalf("example.com deveria ser público, veio IsError: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, "cosca-untrusted-data-v1") {
		t.Fatalf("conteúdo do web NÃO veio envelopado (risco prompt-injection): %.120s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, "not instructions") {
		t.Fatalf("aviso de não-confiável ausente: %.120s", res.Content[0].Text)
	}
}
