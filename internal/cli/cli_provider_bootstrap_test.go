package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/providers/bootstrap"
)

// TestProviderBootstrapCommand_Registered ensures the bootstrap subcommand is
// wired into the provider command tree.
func TestProviderBootstrapCommand_Registered(t *testing.T) {
	cmd := NewProviderCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "bootstrap" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("provider command must register a 'bootstrap' subcommand")
	}
}

// TestProviderBootstrapCommand_Help exercises --help and checks the documented
// flags are present.
func TestProviderBootstrapCommand_Help(t *testing.T) {
	cmd := NewProviderBootstrapCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"bootstrap", "--install", "--pull", "--model", "--url", "--json"} {
		if !strings.Contains(out, want) {
			t.Errorf("help output missing %q", want)
		}
	}
}

// TestProviderBootstrapCommand_JSON runs the command in detection mode (no
// --install, no --pull) with a controlled empty PATH so it can never install,
// start or pull anything. On this machine ollama is not running, so the report
// must be JSON with provider=ollama and detection steps, OK=false, and the
// install step must be skipped.
func TestProviderBootstrapCommand_JSON(t *testing.T) {
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)

	cmd := NewProviderBootstrapCommand()
	out := new(bytes.Buffer)
	warn := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(warn)
	f := NewOutputFormatter(warn, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--json"})

	err := cmd.Execute()
	// On a machine without a functional ollama the command returns an error
	// ("bootstrap incomplete") AFTER emitting the JSON report; that is the
	// expected non-zero exit signal, not a test failure.
	if err != nil {
		t.Logf("bootstrap command returned error (expected when ollama not functional): %v", err)
	}

	raw := out.String()
	if strings.TrimSpace(raw) == "" {
		t.Fatalf("expected JSON report on stdout, got empty output (stderr: %q)", warn.String())
	}
	var report bootstrap.Report
	if jerr := json.Unmarshal([]byte(raw), &report); jerr != nil {
		t.Fatalf("stdout is not a valid bootstrap Report: %v\nstdout: %q\nstderr: %q", jerr, raw, warn.String())
	}
	if report.Provider != "ollama" {
		t.Errorf("report.provider = %q, want ollama", report.Provider)
	}
	if len(report.Steps) == 0 {
		t.Error("report.steps must be non-empty")
	}
	for _, s := range report.Steps {
		if s.Name == "install" && s.Status == "ok" {
			t.Error("install must never run without --install")
		}
	}
}
