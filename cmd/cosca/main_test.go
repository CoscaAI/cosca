package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/memoryintegrity"
)

func TestIsAdminCommandIncludesTerminal(t *testing.T) {
	admin := []string{"init", "config", "version", "hook", "project", "models", "security", "cofre", "terminal",
		"status", "doctor", "health", "capability", "fabric"}
	jailed := []string{"serve", "runtime", "chat", "workflow", "run", "pipeline", "install"}

	// Subcomandos do runtime: leitura/controle do daemon rodam FORA da jaula
	// (precisam ver o PID real do host e o workspace real — a jaula remapeia
	// PIDs e monta o workspace em "/"); start/restart sobem o daemon (executa
	// código de agente) e permanecem na jaula.
	runtimeAdmin := [][]string{{"runtime", "status"}, {"runtime", "stop"}, {"runtime", "logs"}, {"runtime", "info"}}
	runtimeJailed := [][]string{{"runtime", "start"}, {"runtime", "restart"}}

	original := os.Args
	defer func() { os.Args = original }()

	for _, name := range admin {
		os.Args = []string{"cosca", name}
		if !isAdminCommand() {
			t.Errorf("isAdminCommand() = false for %q, want true (must run outside the jail)", name)
		}
	}
	for _, name := range jailed {
		os.Args = []string{"cosca", name}
		if isAdminCommand() {
			t.Errorf("isAdminCommand() = true for %q, want false (must stay jailed)", name)
		}
	}
	for _, args := range runtimeAdmin {
		os.Args = append([]string{"cosca"}, args...)
		if !isAdminCommand() {
			t.Errorf("isAdminCommand() = false for %v, want true (must run outside the jail)", args)
		}
	}
	for _, args := range runtimeJailed {
		os.Args = append([]string{"cosca"}, args...)
		if isAdminCommand() {
			t.Errorf("isAdminCommand() = true for %v, want false (must stay jailed)", args)
		}
	}

	// register precisa ler a chave privada do kernel (fora da jaula); integrity
	// audita o manifesto. Ambos rodam fora da jaula. Os demais subcomandos de
	// memory permanecem na jaula.
	memoryAdmin := [][]string{{"memory", "integrity"}, {"memory", "register"}}
	memoryJailed := [][]string{{"memory", "list"}, {"memory", "search"}, {"memory", "stats"}}
	for _, args := range memoryAdmin {
		os.Args = append([]string{"cosca"}, args...)
		if !isAdminCommand() {
			t.Errorf("isAdminCommand() = false for %v, want true (must run outside the jail)", args)
		}
	}
	for _, args := range memoryJailed {
		os.Args = append([]string{"cosca"}, args...)
		if isAdminCommand() {
			t.Errorf("isAdminCommand() = true for %v, want false (must stay jailed)", args)
		}
	}
}

func TestIntegrityGateBypassIsExplicit(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{[]string{"init"}, true},
		{[]string{"version"}, true},
		{[]string{"memory", "integrity", "init"}, true},
		{[]string{"memory", "integrity", "verify"}, true},
		{[]string{"serve"}, false},
		{[]string{"chat"}, false},
		{[]string{"workflow", "run"}, false},
		{[]string{"serve", "--integrity-bypass"}, false},
	} {
		if got := integrityGateBypass(tc.args); got != tc.want {
			t.Errorf("%v: got %v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestIntegrityGateTempProjectFailClosedAndPasses(t *testing.T) {
	root := t.TempDir()
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	for _, name := range []string{".cosca/framework/KERNEL.md", ".cosca/framework/CONSTITUTION.md", ".cosca/framework/MEMORY_MODEL.md", ".cosca/framework/shared/AUTO_EVOLUTION_PROTOCOL.md", ".cosca/memory/LEARNING_PROTOCOL.md"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// FIRST-BOOT (regra do Don): manifest ausente em máquina nova NÃO recusa —
	// o gate estabelece a linha de base automaticamente (equivalente seguro do
	// `cosca memory integrity init`) e segue. Fail-closed continua valendo para
	// MISMATCH (alteração posterior), testado abaixo.
	if err := enforceMemoryIntegrityGate([]string{"serve"}); err != nil {
		t.Fatalf("first boot without manifest should auto-init and pass: %v", err)
	}
	if _, statErr := os.Stat(memoryintegrity.DefaultManifestPath(root)); os.IsNotExist(statErr) {
		t.Fatalf("first boot should have created the integrity manifest: %v", statErr)
	}
	if err := enforceMemoryIntegrityGate([]string{"serve"}); err != nil {
		t.Fatalf("verified manifest should pass: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".cosca/framework/KERNEL.md"), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := enforceMemoryIntegrityGate([]string{"serve"}); err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("mismatch should fail closed: %v", err)
	}
	// The gate resolves the real cwd, so verify root detection independently and
	// ensure the test fixture itself remains isolated from the repository.
	if detected, err := memoryintegrity.ProjectRoot(root); err != nil || detected != root {
		t.Fatalf("root detection: %q, %v", detected, err)
	}
}
