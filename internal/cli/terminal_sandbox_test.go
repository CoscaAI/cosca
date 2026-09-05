package cli

import (
	"runtime"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
)

// TestTerminalWiringBuildsSandboxedToolExecution verifies that the terminal
// wiring (OpenCode mode: runs outside the jail) creates the per-command
// sandbox gate and configures it for workspace-isolated execution. This is the
// security replacement for the jail: LLM-issued commands execute inside this
// gate (bwrap), never as the terminal process itself.
func TestTerminalWiringBuildsSandboxedToolExecution(t *testing.T) {
	dir := t.TempDir()

	gate := buildTerminalSandboxGate(dir)
	if gate == nil {
		t.Fatal("buildTerminalSandboxGate returned nil — the terminal tool sandbox is not wired")
	}

	if got := gate.Mode(); got != chat.SandboxWorkspace {
		t.Fatalf("gate.Mode() = %v, want %v (workspace-confined, writable)", got, chat.SandboxWorkspace)
	}

	// The gate must accept paths inside the workspace…
	if err := gate.ValidatePath(dir); err != nil {
		t.Fatalf("gate.ValidatePath(workspace) returned error: %v", err)
	}
	// …and reject escapes outside it. Semântica POSIX: "/etc" é um caminho
	// absoluto fora do workspace. No Windows "/etc" é RELATIVO (sem volume;
	// filepath.IsAbs("/etc") == false) e resolve para <workspace>\etc — não é
	// um escape, logo a asserção de rejeição não se aplica nesta plataforma.
	if runtime.GOOS != "windows" {
		if err := gate.ValidatePath("/etc"); err == nil {
			t.Error("gate.ValidatePath(/etc) = nil, want escape rejected")
		}
	}
}
