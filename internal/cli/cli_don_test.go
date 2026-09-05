package cli

import (
	"bytes"
	"context"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/kernel"
)

// TestDonStatus_Disarmed verifies status reports DISARMED before arming.
func TestDonStatus_Disarmed(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	globalFlags = GlobalFlags{}

	root := NewRootCommand()
	root.SetArgs([]string{"don", "status", "--json"})
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"armed": false`) {
		t.Fatalf("expected armed:false, got: %s", buf.String())
	}
}

// TestDonPhrase_ArmsAndVerifies runs the full lifecycle:
// arm → status ARMED → verify correct → verify wrong.
func TestDonPhrase_ArmsAndVerifies(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	globalFlags = GlobalFlags{}

	root := NewRootCommand()
	root.SetArgs([]string{"don", "phrase", "sangue e ouro"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Hash file created; phrase never stored in plaintext.
	raw, err := os.ReadFile(".cosca/don.phr")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "sangue e ouro") {
		t.Fatal("phrase must never be stored in plaintext")
	}
	info, _ := os.Stat(".cosca/don.phr")
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("hash file perms must be 0600, got %v", info.Mode().Perm())
	}
	// No Windows o chmod 0600 é mapeado para somente-leitura (0444); a semântica
	// POSIX de permissões não se aplica — o guard acima ignora a checagem lá.

	// Status now ARMED.
	root = NewRootCommand()
	root.SetArgs([]string{"don", "status", "--json"})
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"armed": true`) {
		t.Fatalf("expected armed:true, got: %s", buf.String())
	}

	// Verify correct phrase.
	root = NewRootCommand()
	root.SetArgs([]string{"don", "verify", "sangue e ouro", "--json"})
	buf.Reset()
	root.SetOut(&buf)
	root.SetErr(&buf)
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("correct phrase must verify: %v", err)
	}
	if !strings.Contains(buf.String(), `"ok": true`) {
		t.Fatalf("expected ok:true, got: %s", buf.String())
	}

	// Verify wrong phrase → error.
	root = NewRootCommand()
	root.SetArgs([]string{"don", "verify", "frase errada"})
	buf.Reset()
	root.SetOut(&buf)
	root.SetErr(&buf)
	if err := root.ExecuteContext(context.Background()); err == nil {
		t.Fatal("wrong phrase must fail verification")
	}
}

// TestDonPhrase_EmptyRejected verifies an empty phrase is rejected.
func TestDonPhrase_EmptyRejected(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	globalFlags = GlobalFlags{}

	root := NewRootCommand()
	root.SetArgs([]string{"don", "phrase", "   "})
	if err := root.ExecuteContext(context.Background()); err == nil {
		t.Fatal("empty phrase must be rejected")
	}
}

// TestDonPhrase_CheckFlag verifies --check without re-arming.
func TestDonPhrase_CheckFlag(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	globalFlags = GlobalFlags{}

	root := NewRootCommand()
	root.SetArgs([]string{"don", "phrase", "pai da familia"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}

	root = NewRootCommand()
	root.SetArgs([]string{"don", "phrase", "pai da familia", "--check", "--json"})
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("--check with correct phrase must pass: %v", err)
	}
}

// TestDonVerifyHashMatchesKernel verifies the CLI round-trips the same hash
// the kernel computes, proving integration.
func TestDonVerifyHashMatchesKernel(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	globalFlags = GlobalFlags{}

	hash, err := kernel.HashDonPhrase("segredo de estado")
	if err != nil {
		t.Fatal(err)
	}
	if err := writeDonPhraseFile(hash); err != nil {
		t.Fatal(err)
	}

	root := NewRootCommand()
	root.SetArgs([]string{"don", "verify", "segredo de estado", "--json"})
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("kernel-computed hash must verify via CLI: %v", err)
	}
	if !strings.Contains(buf.String(), `"ok": true`) {
		t.Fatalf("expected ok:true, got: %s", buf.String())
	}
}

// TestDonAttempts verifies the audit trail subcommand runs without error.
func TestDonAttempts(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	globalFlags = GlobalFlags{}

	root := NewRootCommand()
	root.SetArgs([]string{"don", "attempts", "--json"})
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("attempts must run: %v", err)
	}
}

// TestDonLock_PersistsAcrossCLIRuns verifies the brute-force lock survives
// separate CLI process executions: 5 wrong phrases across 5 invocations must
// lock, and the correct phrase must then be refused.
func TestDonLock_PersistsAcrossCLIRuns(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	globalFlags = GlobalFlags{}

	// Arm.
	root := NewRootCommand()
	root.SetArgs([]string{"don", "phrase", "segredo de familia"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}

	// 4 wrong phrases across 4 separate "processes" (fresh roots). Each
	// invocation loads and re-persists the lock state.
	for i := 0; i < 4; i++ {
		root = NewRootCommand()
		root.SetArgs([]string{"don", "verify", "errada"})
		if err := root.ExecuteContext(context.Background()); err == nil {
			t.Fatalf("wrong phrase %d must fail", i+1)
		}
	}

	// State file must now hold 4 failures.
	raw, err := os.ReadFile(".cosca/don.state")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"failures":4`) {
		t.Fatalf("expected 4 persisted failures, got: %s", raw)
	}

	// 5th wrong phrase triggers the lock.
	root = NewRootCommand()
	root.SetArgs([]string{"don", "verify", "errada"})
	if err := root.ExecuteContext(context.Background()); err == nil {
		t.Fatal("5th wrong phrase must fail")
	}

	// Locked: even the correct phrase is refused.
	root = NewRootCommand()
	root.SetArgs([]string{"don", "verify", "segredo de familia"})
	if err := root.ExecuteContext(context.Background()); err == nil {
		t.Fatal("correct phrase must be refused while locked")
	}
}

// TestDonPhrase_ResetsLockState verifies re-arming clears the persisted lock.
func TestDonPhrase_ResetsLockState(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	globalFlags = GlobalFlags{}

	root := NewRootCommand()
	root.SetArgs([]string{"don", "phrase", "primeira frase"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Drive the lock.
	for i := 0; i < 5; i++ {
		root = NewRootCommand()
		root.SetArgs([]string{"don", "verify", "errada"})
		_ = root.ExecuteContext(context.Background())
	}
	if _, err := os.Stat(".cosca/don.state"); err != nil {
		t.Fatalf("lock state must exist after failures: %v", err)
	}

	// Re-arm: lock state must be cleared.
	root = NewRootCommand()
	root.SetArgs([]string{"don", "phrase", "segunda frase"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(".cosca/don.state"); !os.IsNotExist(err) {
		t.Fatalf("lock state must be removed on re-arm, got err=%v", err)
	}

	// New phrase verifies fine (no residual lock).
	root = NewRootCommand()
	root.SetArgs([]string{"don", "verify", "segunda frase", "--json"})
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("new phrase must verify after re-arm: %v", err)
	}
	if !strings.Contains(buf.String(), `"ok": true`) {
		t.Fatalf("expected ok:true, got: %s", buf.String())
	}
}
