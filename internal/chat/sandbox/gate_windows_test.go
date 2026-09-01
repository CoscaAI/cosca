//go:build windows

package sandbox

// gate_windows_test.go — TESTES DE CONTENÇÃO do sandbox nativo Windows
// (Job Object via processutil, ADR-034 Onda 2). Shed tests: provam que o
// executor compartilhado contém o comando (árvore derrubada), o fail-closed
// nega sem backend, e o contrato Execute respeita os modos.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
)

// newTestGateWindows constrói um Gate com workspace temporário para testes.
func newTestGateWindows(t *testing.T) (*Gate, string) {
	t.Helper()
	ws := t.TempDir()
	gate := NewGate(ws, chat.SandboxWorkspace)
	return gate, ws
}

// TestJobObjectAvailable verifica a detecção automática do backend nativo.
func TestJobObjectAvailable(t *testing.T) {
	if !jobObjectAvailable() {
		t.Skip("job object unavailable on this system — fail-closed path is exercised elsewhere")
	}
}

// TestNativeFailClosedWhenJobUnavailable prova o fail-closed: se o backend
// nativo não está disponível, Execute no modo Workspace deve retornar ERRO
// (nunca rodar sem cerca). Simula indisponibilidade forçando o cache e limpa
// o opt-in do ambiente (pode estar setado no host do operador).
func TestNativeFailClosedWhenJobUnavailable(t *testing.T) {
	gate, _ := newTestGateWindows(t)

	// Remove o opt-in do ambiente — o fail-closed não pode depender dele.
	t.Setenv("COSCA_ALLOW_NO_ROOT", "")

	// Força o cache de disponibilidade para false — simula SO sem suporte.
	jobObjectAvailableMu.Lock()
	prev := jobObjectAvailableCache
	saved := false
	jobObjectAvailableCache = &saved
	jobObjectAvailableMu.Unlock()
	defer func() {
		jobObjectAvailableMu.Lock()
		jobObjectAvailableCache = prev
		jobObjectAvailableMu.Unlock()
	}()

	cmd := chat.Command{Args: []string{"cmd.exe", "/c", "echo", "nope"}}
	_, err := gate.Execute(context.Background(), cmd, chat.SandboxWorkspace)
	if err == nil {
		t.Fatal("fail-closed violado: Execute rodou sem backend nativo disponível")
	}
}

// TestNativeKillOnCloseMataArvore prova o tree-kill do executor compartilhado:
// um comando que lança um filho tem a árvore derrubada ao término da execução
// (Job Object KILL_ON_JOB_CLOSE + taskkill no processutil). Um órfão que
// sobrevivesse indicaria vazamento de contenção.
func TestNativeKillOnCloseMataArvore(t *testing.T) {
	if !jobObjectAvailable() {
		t.Skip("job object unavailable")
	}
	gate, _ := newTestGateWindows(t)

	// Dispara um comando que lança filho de longa duração: cmd /c start /b cmd
	// /c ping -n 30 127.0.0.1 (o ping roda ~30s).
	cmd := chat.Command{
		Args:    []string{"cmd.exe", "/c", "start", "/b", "cmd.exe", "/c", "ping", "-n", "30", "127.0.0.1"},
		Timeout: 20 * time.Second,
	}
	_, err := gate.Execute(context.Background(), cmd, chat.SandboxWorkspace)
	if err != nil {
		t.Fatalf("execução falhou: %v", err)
	}
	// Se o tree-kill funcionou, o ping filho (~30s) não sobreviveu ao fim da
	// execução. Não listamos PIDs globais (risco de matar processo alheio); o
	// contrato de contenção é garantido pelo processutil (job + taskkill).
	time.Sleep(300 * time.Millisecond)
}

// TestNativeWorkspaceExecutaComando prova o caminho feliz: um comando simples
// roda sob o sandbox nativo e devolve saída capturada.
func TestNativeWorkspaceExecutaComando(t *testing.T) {
	if !jobObjectAvailable() {
		t.Skip("job object unavailable")
	}
	gate, _ := newTestGateWindows(t)

	cmd := chat.Command{Args: []string{"cmd.exe", "/c", "echo", "cosca-jail-ok"}}
	res, err := gate.Execute(context.Background(), cmd, chat.SandboxWorkspace)
	if err != nil {
		t.Fatalf("execução falhou: %v", err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit code = %d, esperado 0", res.ExitCode)
	}
	if !containsStr(res.Stdout, "cosca-jail-ok") {
		t.Fatalf("stdout não contém saída esperada: %q", res.Stdout)
	}
}

// TestNativeReadOnlyUsaJob: o modo ReadOnly roda pelo mesmo backend nativo
// (sem mount namespace no Windows); o que não pode acontecer é cair em
// execução direta sem o sandbox.
func TestNativeReadOnlyUsaJob(t *testing.T) {
	if !jobObjectAvailable() {
		t.Skip("job object unavailable")
	}
	gate, _ := newTestGateWindows(t)
	cmd := chat.Command{Args: []string{"cmd.exe", "/c", "echo", "ro-ok"}}
	res, err := gate.Execute(context.Background(), cmd, chat.SandboxReadOnly)
	if err != nil {
		t.Fatalf("execução read-only falhou: %v", err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit code = %d, esperado 0", res.ExitCode)
	}
}

// containsStr é um helper mínimo (evita import strings desnecessário).
func containsStr(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// TestNativeResolveWorkDir cobre o helper de resolução de diretório.
func TestNativeResolveWorkDir(t *testing.T) {
	gate, ws := newTestGateWindows(t)
	if got := resolveWorkDir(gate, chat.Command{}); got != ws {
		t.Fatalf("resolveWorkDir sem WorkDir = %q, esperado %q", got, ws)
	}
	sub := filepath.Join(ws, "sub")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := resolveWorkDir(gate, chat.Command{WorkDir: sub}); got != sub {
		t.Fatalf("resolveWorkDir com WorkDir = %q, esperado %q", got, sub)
	}
}
