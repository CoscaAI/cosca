//
// Tests for the `cosca hardware` command tree (internal/cli/hardware.go).
//
// Covers:
//   - Registration of `hardware` in the root command
//   - Command properties (Use/Short) and arg constraints
//   - `hardware probe` text output contains CPU/RAM/GPU sections
//   - `hardware` with no subcommand also runs the probe
//   - `hardware probe --json` machine-readable output
//   - `hardware cpu` / `hardware memory` text and JSON output
//   - Unknown subcommand rejected
//
// Uses the newContextWithFormatter + PersistentPreRunE = nil pattern so the
// output goes to a buffer (mirroring cli_capability_test.go).

package cli

import (
	"bytes"
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestHardwareCommand_RegisteredInRoot(t *testing.T) {
	cmd := NewRootCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "hardware" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("hardware subcommand not registered in root command")
	}
}

func TestHardwareCommand_Properties(t *testing.T) {
	cmd := NewHardwareCommand()
	if cmd == nil {
		t.Fatal("NewHardwareCommand returned nil")
	}
	if cmd.Use != "hardware [probe|cpu|memory|topology|environment|gpu|storage]" {
		t.Errorf("expected Use='hardware [probe|cpu|memory|topology|environment|gpu|storage]', got %q", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
	subs := cmd.Commands()
	if len(subs) != 7 {
		t.Fatalf("expected exactly seven subcommands (probe, cpu, memory, topology, environment, gpu, storage), got %d", len(subs))
	}
	names := map[string]bool{}
	for _, sub := range subs {
		names[sub.Name()] = true
	}
	for _, want := range []string{"probe", "cpu", "memory", "topology", "environment", "gpu", "storage"} {
		if !names[want] {
			t.Errorf("expected subcommand %q, got %v", want, names)
		}
	}
}

func TestHardwareCPUCommand_Properties(t *testing.T) {
	cmd := NewHardwareCPUCommand()
	if cmd == nil {
		t.Fatal("NewHardwareCPUCommand returned nil")
	}
	if cmd.Use != "cpu" {
		t.Errorf("expected Use='cpu', got %q", cmd.Use)
	}
	if err := cmd.Args(cmd, []string{"extra"}); err == nil {
		t.Error("cpu with args should fail")
	}
	if err := cmd.Args(cmd, nil); err != nil {
		t.Errorf("cpu with no args should be allowed: %v", err)
	}
}

func TestHardwareProbeCommand_Properties(t *testing.T) {
	cmd := NewHardwareProbeCommand()
	if cmd == nil {
		t.Fatal("NewHardwareProbeCommand returned nil")
	}
	if cmd.Use != "probe" {
		t.Errorf("expected Use='probe', got %q", cmd.Use)
	}
	if err := cmd.Args(cmd, []string{"extra"}); err == nil {
		t.Error("probe with args should fail")
	}
	if err := cmd.Args(cmd, nil); err != nil {
		t.Errorf("probe with no args should be allowed: %v", err)
	}
}

func TestHardwareMemoryCommand_Properties(t *testing.T) {
	cmd := NewHardwareMemoryCommand()
	if cmd == nil {
		t.Fatal("NewHardwareMemoryCommand returned nil")
	}
	if cmd.Use != "memory" {
		t.Errorf("expected Use='memory', got %q", cmd.Use)
	}
	if err := cmd.Args(cmd, []string{"extra"}); err == nil {
		t.Error("memory with args should fail")
	}
	if err := cmd.Args(cmd, nil); err != nil {
		t.Errorf("memory with no args should be allowed: %v", err)
	}
}

func TestHardwareTopologyCommand_Properties(t *testing.T) {
	cmd := NewHardwareTopologyCommand()
	if cmd == nil {
		t.Fatal("NewHardwareTopologyCommand returned nil")
	}
	if cmd.Use != "topology" {
		t.Errorf("expected Use='topology', got %q", cmd.Use)
	}
	if err := cmd.Args(cmd, []string{"extra"}); err == nil {
		t.Error("topology with args should fail")
	}
	if err := cmd.Args(cmd, nil); err != nil {
		t.Errorf("topology with no args should be allowed: %v", err)
	}
}

func TestHardwareStorageCommand_Properties(t *testing.T) {
	cmd := NewHardwareStorageCommand()
	if cmd == nil {
		t.Fatal("NewHardwareStorageCommand returned nil")
	}
	if cmd.Use != "storage" {
		t.Errorf("expected Use='storage', got %q", cmd.Use)
	}
	if err := cmd.Args(cmd, []string{"extra"}); err == nil {
		t.Error("storage with args should fail")
	}
	if err := cmd.Args(cmd, nil); err != nil {
		t.Errorf("storage with no args should be allowed: %v", err)
	}
}

// executeHardware runs a `hardware` subcommand with output to a buffer.
func executeHardware(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := NewRootCommand()
	cmd.PersistentPreRunE = nil
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	formatter := NewOutputFormatter(buf, OutputFormatText, false, false, false)
	cmd.SetContext(newContextWithFormatter(context.Background(), formatter))
	cmd.SetArgs(append([]string{"hardware"}, args...))
	err := cmd.Execute()
	return buf.String(), err
}

func TestHardwareProbeCommand_TextOutputContainsSections(t *testing.T) {
	out, err := executeHardware(t, "probe")
	if err != nil {
		t.Fatalf("hardware probe: %v", err)
	}
	for _, want := range []string{"CPU", "RAM", "GPU"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q section: %q", want, out)
		}
	}
	for _, want := range []string{"Cores", "Total", "Vendor"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q key: %q", want, out)
		}
	}
}

func TestHardwareCommand_NoSubcommand_RunsProbe(t *testing.T) {
	out, err := executeHardware(t)
	if err != nil {
		t.Fatalf("hardware with no subcommand: %v", err)
	}
	for _, want := range []string{"CPU", "RAM", "GPU"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q section: %q", want, out)
		}
	}
}

func TestHardwareProbeCommand_JSONOutput(t *testing.T) {
	cmd := NewRootCommand()
	cmd.PersistentPreRunE = nil
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"hardware", "probe", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("hardware probe --json: %v", err)
	}
	for _, want := range []string{`"LogicalCores"`, `"gpu"`, `"vendor"`, `"ProbedAt"`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("JSON output missing %q: %q", want, buf.String())
		}
	}
}

func TestHardwareCommand_UnknownSubcommand_ReturnsError(t *testing.T) {
	out, err := executeHardware(t, "bogus")
	if err == nil {
		t.Error("expected error for unknown subcommand")
	}
	if strings.Contains(out, "CPU") {
		t.Error("probe should not run for unknown subcommand")
	}
}

func TestHardwareCPUCommand_TextOutput(t *testing.T) {
	out, err := executeHardware(t, "cpu")
	if err != nil {
		t.Fatalf("hardware cpu: %v", err)
	}
	for _, want := range []string{"CPU Discovery", "Logical CPUs", "Detected", "Available", "Usable"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %q", want, out)
		}
	}
}

func TestHardwareCPUCommand_JSONOutput(t *testing.T) {
	cmd := NewRootCommand()
	cmd.PersistentPreRunE = nil
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"hardware", "cpu", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("hardware cpu --json: %v", err)
	}
	for _, want := range []string{`"vendor"`, `"arch"`, `"logical_cpus"`, `"detected"`, `"features"`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("JSON output missing %q: %q", want, buf.String())
		}
	}
}

func TestHardwareProbeCommand_JSONIncludesCPU(t *testing.T) {
	cmd := NewRootCommand()
	cmd.PersistentPreRunE = nil
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"hardware", "probe", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("hardware probe --json: %v", err)
	}
	if !strings.Contains(buf.String(), `"cpu"`) {
		t.Errorf("JSON output missing cpu discovery: %q", buf.String())
	}
}

func TestHardwareMemoryCommand_TextOutput(t *testing.T) {
	out, err := executeHardware(t, "memory")
	if err != nil {
		t.Fatalf("hardware memory: %v", err)
	}
	for _, want := range []string{"Memory Discovery", "Physical", "Visible", "Available", "Effective", "Page Size", "Detected"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %q", want, out)
		}
	}
}

func TestHardwareMemoryCommand_JSONOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("probes de memória dependem de /proc/meminfo e RLIMIT_AS (Linux) — no Windows o JSON é honesto com campos omitidos + limitations")
	}
	cmd := NewRootCommand()
	cmd.PersistentPreRunE = nil
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"hardware", "memory", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("hardware memory --json: %v", err)
	}
	for _, want := range []string{`"physical_bytes"`, `"visible_bytes"`, `"available_bytes"`,
		`"effective_bytes"`, `"page_size"`, `"detected"`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("JSON output missing %q: %q", want, buf.String())
		}
	}
}

func TestHardwareProbeCommand_JSONIncludesMemory(t *testing.T) {
	cmd := NewRootCommand()
	cmd.PersistentPreRunE = nil
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"hardware", "probe", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("hardware probe --json: %v", err)
	}
	if !strings.Contains(buf.String(), `"memory"`) {
		t.Errorf("JSON output missing memory discovery: %q", buf.String())
	}
}

func TestHardwareTopologyCommand_TextOutput(t *testing.T) {
	out, err := executeHardware(t, "topology")
	if err != nil {
		t.Fatalf("hardware topology: %v", err)
	}
	for _, want := range []string{"Topology Discovery", "NUMA Nodes", "Packages", "Detected", "Per-CPU Topology"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %q", want, out)
		}
	}
}

func TestHardwareTopologyCommand_JSONOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("topologia depende de /proc/cpuinfo e sysfs (Linux) — no Windows o resultado é synthetic com limitations")
	}
	cmd := NewRootCommand()
	cmd.PersistentPreRunE = nil
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"hardware", "topology", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("hardware topology --json: %v", err)
	}
	for _, want := range []string{`"nodes"`, `"cpu_to_node"`, `"packages"`, `"detected"`, `"cpu_topology"`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("JSON output missing %q: %q", want, buf.String())
		}
	}
}

func TestHardwareProbeCommand_JSONIncludesTopology(t *testing.T) {
	cmd := NewRootCommand()
	cmd.PersistentPreRunE = nil
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"hardware", "probe", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("hardware probe --json: %v", err)
	}
	if !strings.Contains(buf.String(), `"topology"`) {
		t.Errorf("JSON output missing topology discovery: %q", buf.String())
	}
}

func TestHardwareStorageCommand_TextOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("storage depende de /proc/mounts, statfs e /sys/block (Linux) — no Windows o output é honesto com limitations")
	}
	out, err := executeHardware(t, "storage")
	if err != nil {
		t.Fatalf("hardware storage: %v", err)
	}
	for _, want := range []string{"Storage Discovery", "Detected", "Capabilities", "Reported", "Measured", "Focos do Cosca", "Knowledge.db", "Filesystems Montados", "Limitations"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %q", want, out)
		}
	}
}

func TestHardwareStorageCommand_JSONOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("storage depende de /proc/mounts, statfs e /sys/block (Linux) — no Windows o JSON é honesto com limitations")
	}
	cmd := NewRootCommand()
	cmd.PersistentPreRunE = nil
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"hardware", "storage", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("hardware storage --json: %v", err)
	}
	for _, want := range []string{`"mounts"`, `"capabilities"`, `"reported"`, `"measured"`,
		`"knowledge_db_mount"`, `"workspace_mount"`, `"tmp_mount"`, `"detected"`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("JSON output missing %q: %q", want, buf.String())
		}
	}
}

func TestHardwareProbeCommand_JSONIncludesStorage(t *testing.T) {
	cmd := NewRootCommand()
	cmd.PersistentPreRunE = nil
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"hardware", "probe", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("hardware probe --json: %v", err)
	}
	if !strings.Contains(buf.String(), `"storage"`) {
		t.Errorf("JSON output missing storage discovery: %q", buf.String())
	}
}

// compile-time guard: the command constructors must match the cobra contract.
var (
	_ *cobra.Command = NewHardwareCommand()
	_ *cobra.Command = NewHardwareProbeCommand()
	_ *cobra.Command = NewHardwareCPUCommand()
	_ *cobra.Command = NewHardwareMemoryCommand()
	_ *cobra.Command = NewHardwareTopologyCommand()
	_ *cobra.Command = NewHardwareStorageCommand()
)
