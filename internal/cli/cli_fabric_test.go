//
// Tests for the `cosca fabric status` command tree (internal/cli/fabric.go).
//
// Covers:
//   - Registration of `fabric` in the root command
//   - Command properties (Use/Short) and the `status` subcommand
//   - `fabric status` prints the COMPUTE FABRIC STATUS report with pool names
//
// Uses the PersistentPreRunE = nil pattern so the output goes to a buffer
// (mirroring cli_hardware_test.go). The fabric's Start reads /proc directly;
// on systems without /proc the probe returns zero values and the pools clamp
// to their minimums, so the command still produces a valid report.

package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestFabricCommand_RegisteredInRoot(t *testing.T) {
	cmd := NewRootCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "fabric" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("fabric subcommand not registered in root command")
	}
}

func TestFabricCommand_Properties(t *testing.T) {
	cmd := NewFabricCommand()
	if cmd == nil {
		t.Fatal("NewFabricCommand returned nil")
	}
	if cmd.Name() != "fabric" {
		t.Errorf("expected name 'fabric', got %q", cmd.Name())
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
	if cmd.RunE == nil {
		t.Error("expected RunE to be set on the fabric command")
	}
	if len(cmd.Commands()) != 1 || cmd.Commands()[0].Name() != "status" {
		t.Errorf("expected exactly one subcommand 'status', got %d", len(cmd.Commands()))
	}
}

func TestFabricStatusCommand_Properties(t *testing.T) {
	cmd := NewFabricStatusCommand()
	if cmd == nil {
		t.Fatal("NewFabricStatusCommand returned nil")
	}
	if cmd.Use != "status" {
		t.Errorf("expected Use='status', got %q", cmd.Use)
	}
	if err := cmd.Args(cmd, []string{"extra"}); err == nil {
		t.Error("status with args should fail")
	}
	if err := cmd.Args(cmd, nil); err != nil {
		t.Errorf("status with no args should be allowed: %v", err)
	}
}

// executeFabricStatus runs `fabric status` with output to a buffer.
func executeFabricStatus(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := NewRootCommand()
	cmd.PersistentPreRunE = nil
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(context.Background())
	cmd.SetArgs(append([]string{"fabric"}, args...))
	err := cmd.Execute()
	return buf.String(), err
}

func TestFabricStatusCommand_RunE(t *testing.T) {
	out, err := executeFabricStatus(t, "status")
	if err != nil {
		t.Fatalf("fabric status: %v", err)
	}
	for _, want := range []string{"COMPUTE FABRIC STATUS", "agent", "tool", "Hardware", "Worker Pools", "Backpressure"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q; output:\n%s", want, out)
		}
	}
}

func TestFabricCommand_NoSubcommand_RunsStatus(t *testing.T) {
	out, err := executeFabricStatus(t)
	if err != nil {
		t.Fatalf("fabric with no subcommand: %v", err)
	}
	for _, want := range []string{"COMPUTE FABRIC STATUS", "agent", "index", "sandbox"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q; output:\n%s", want, out)
		}
	}
}
