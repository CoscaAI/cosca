package evals

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestRunVerifyCommandsPass(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("RunVerifyCommands executa via sh -c (shell POSIX) — sh ausente no Windows nativo")
	}
	res := RunVerifyCommands(context.Background(), t.TempDir(), []string{"echo hello"}, 10*time.Second)
	if len(res) != 1 {
		t.Fatalf("results = %d, want 1", len(res))
	}
	if !res[0].OK {
		t.Errorf("echo should pass: %+v", res[0])
	}
	if res[0].OutputTail != "hello\n" {
		t.Errorf("output tail = %q, want %q", res[0].OutputTail, "hello\n")
	}
	if res[0].Error != "" {
		t.Errorf("unexpected error %q", res[0].Error)
	}
}

func TestRunVerifyCommandsFail(t *testing.T) {
	res := RunVerifyCommands(context.Background(), t.TempDir(), []string{"exit 3"}, 10*time.Second)
	if len(res) != 1 {
		t.Fatalf("results = %d, want 1", len(res))
	}
	if res[0].OK {
		t.Error("exit 3 should fail")
	}
	if res[0].Error == "" {
		t.Error("expected an error string")
	}
}

func TestRunVerifyCommandsMultiple(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("RunVerifyCommands executa via sh -c (shell POSIX) — sh ausente no Windows nativo")
	}
	res := RunVerifyCommands(context.Background(), t.TempDir(),
		[]string{"echo one", "false", "echo three"}, 10*time.Second)
	if len(res) != 3 {
		t.Fatalf("results = %d, want 3", len(res))
	}
	if !res[0].OK || res[1].OK || !res[2].OK {
		t.Errorf("expected [true false true], got [%v %v %v]", res[0].OK, res[1].OK, res[2].OK)
	}
}

func TestRunVerifyCommandsTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep semantics differ on windows")
	}
	// sh -c "sleep 5" with a 300ms timeout must be killed.
	start := time.Now()
	res := RunVerifyCommands(context.Background(), t.TempDir(), []string{"sleep 5"}, 300*time.Millisecond)
	elapsed := time.Since(start)
	if len(res) != 1 {
		t.Fatalf("results = %d", len(res))
	}
	if res[0].OK {
		t.Error("sleep 5 with 300ms timeout must fail")
	}
	if res[0].Error != "verify command exceeded timeout" {
		t.Errorf("error = %q, want timeout", res[0].Error)
	}
	if elapsed > 3*time.Second {
		t.Errorf("verify took %v — command was not killed", elapsed)
	}
}

func TestRunVerifyCommandsContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := RunVerifyCommands(ctx, t.TempDir(), []string{"echo hi"}, 10*time.Second)
	if len(res) != 1 {
		t.Fatalf("results = %d", len(res))
	}
	if res[0].OK {
		t.Error("canceled context must fail the command")
	}
	if res[0].Error != "verify command canceled" {
		t.Errorf("error = %q, want canceled", res[0].Error)
	}
}
