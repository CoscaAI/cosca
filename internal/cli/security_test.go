//
// Tests for `cosca security scan` — dependency vulnerability scanning
// backed by Google's osv-scanner / OSV.dev.
//
// Network-dependent tests are gated behind a connectivity check and skipped
// when offline so the suite still passes in CI sandboxes without egress.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/security"
)

// networkAvailable reports whether api.osv.dev is reachable.
func networkAvailable() bool {
	conn, err := net.DialTimeout("tcp", "api.osv.dev:443", 3*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// buildSecurityRootCommand builds the `security` command wired to a buffer.
func buildSecurityRootCommand(t *testing.T) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	root := NewRootCommand()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetContext(context.Background())
	return root, buf
}

// runSecurityScan executes `security scan <args...>` against the root command.
func runSecurityScan(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root, buf := buildSecurityRootCommand(t)
	root.SetArgs(append([]string{"security", "scan"}, args...))
	err := root.Execute()
	return buf.String(), err
}

// =============================================================================
// Command registration
// =============================================================================

func TestSecurityCommand_RegisteredInRoot(t *testing.T) {
	root := NewRootCommand()
	found := false
	for _, sub := range root.Commands() {
		if sub.Name() == "security" {
			found = true
			hasScan := false
			for _, s := range sub.Commands() {
				if s.Name() == "scan" {
					hasScan = true
				}
			}
			if !hasScan {
				t.Fatal("security command missing scan subcommand")
			}
		}
	}
	if !found {
		t.Fatal("security subcommand not registered in root command")
	}
}

func TestSecurityScanCommand_Properties(t *testing.T) {
	cmd := NewSecurityScanCommand()
	if cmd.Use != "scan [dir]" {
		t.Errorf("expected Use='scan [dir]', got %q", cmd.Use)
	}
	for _, flag := range []string{"severity", "recursive", "exit-zero"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("missing flag --%s", flag)
		}
	}
}

// =============================================================================
// Exit code logic
// =============================================================================

func TestExitCodeForScan(t *testing.T) {
	clean := &security.ScanResult{SeverityCounts: map[string]int{}}
	if got := exitCodeForScan(clean, false); got != 0 {
		t.Errorf("clean scan: expected exit 0, got %d", got)
	}

	low := &security.ScanResult{SeverityCounts: map[string]int{security.SeverityLow: 3}}
	if got := exitCodeForScan(low, false); got != 0 {
		t.Errorf("low-only scan: expected exit 0, got %d", got)
	}

	high := &security.ScanResult{SeverityCounts: map[string]int{security.SeverityHigh: 1, security.SeverityLow: 2}}
	if got := exitCodeForScan(high, false); got != 1 {
		t.Errorf("high scan: expected exit 1, got %d", got)
	}

	crit := &security.ScanResult{SeverityCounts: map[string]int{security.SeverityCritical: 1}}
	if got := exitCodeForScan(crit, false); got != 1 {
		t.Errorf("critical scan: expected exit 1, got %d", got)
	}

	if got := exitCodeForScan(crit, true); got != 0 {
		t.Errorf("critical scan with --exit-zero: expected exit 0, got %d", got)
	}
}

// =============================================================================
// Scan with no dependency files (offline-safe)
// =============================================================================

func TestSecurityScan_NoDependencyFiles(t *testing.T) {
	dir := t.TempDir()
	out, err := runSecurityScan(t, dir)
	if err != nil {
		t.Fatalf("scan returned error: %v", err)
	}
	if !strings.Contains(out, "No dependency files found") {
		t.Errorf("expected 'No dependency files found' in output, got: %q", out)
	}
	// Exit code must be 0 (no critical/high vulns).
	code := ExitCode(err)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

// =============================================================================
// JSON output
// =============================================================================

func TestSecurityScan_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	root, buf := buildSecurityRootCommand(t)
	root.SetArgs([]string{"security", "scan", "--json", dir})
	if err := root.Execute(); err != nil {
		t.Fatalf("scan returned error: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if out == "" {
		t.Fatal("expected non-empty JSON output")
	}
	var parsed security.ScanResult
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if parsed.Dir != dir {
		t.Errorf("expected Dir=%q, got %q", dir, parsed.Dir)
	}
	if !parsed.NoDependencyFiles {
		t.Error("expected NoDependencyFiles=true for empty dir")
	}
}

// =============================================================================
// Severity filter (unit test on the filter logic, offline-safe)
// =============================================================================

func TestSeverityFilter(t *testing.T) {
	high := &security.ScanResult{
		SeverityCounts: map[string]int{
			security.SeverityCritical: 1,
			security.SeverityHigh:     2,
			security.SeverityMedium:   3,
			security.SeverityLow:      4,
		},
	}
	if !high.HasCriticalHigh() {
		t.Error("expected HasCriticalHigh()=true")
	}
	if high.CriticalHigh() != 3 {
		t.Errorf("expected CriticalHigh()=3, got %d", high.CriticalHigh())
	}
}

func TestSeverityFilter_UnknownKept(t *testing.T) {
	unknown := &security.ScanResult{
		SeverityCounts: map[string]int{security.SeverityUnknown: 1},
	}
	if unknown.HasCriticalHigh() {
		t.Error("unknown severity must not count as critical/high")
	}
	if got := exitCodeForScan(unknown, false); got != 0 {
		t.Errorf("unknown-only scan: expected exit 0, got %d", got)
	}
}

// =============================================================================
// Real integration scan against the cosca repository (network-gated)
// =============================================================================

func TestSecurityScan_IntegrationOnCoscaRepo(t *testing.T) {
	if !networkAvailable() {
		t.Skip("network unavailable — skipping integration scan")
	}

	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// The cosca repo itself contains a go.mod to scan.
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		t.Skip("go.mod not found in current dir — skipping")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	result, err := security.Scan(ctx, security.Options{
		Dir:         dir,
		Recursive:   true,
		MinSeverity: security.SeverityLow,
	})
	if err != nil {
		t.Fatalf("integration scan failed: %v", err)
	}
	if result.NoDependencyFiles {
		t.Fatal("expected dependency files to be found in the cosca repo")
	}
	if result.PackagesScanned == 0 {
		t.Error("expected at least one package scanned")
	}
	if result.Sources == 0 {
		t.Error("expected at least one dependency source")
	}
}

// =============================================================================
// Exit code via error propagation (offline-safe path)
// =============================================================================

func TestExitCodeError_ErrorString(t *testing.T) {
	e := ExitCodeError{Code: 2}
	if e.Error() == "" {
		t.Error("expected non-empty error string")
	}
}

func TestExitCode_FromError(t *testing.T) {
	if got := ExitCode(ExitCodeError{Code: 2}); got != 2 {
		t.Errorf("expected ExitCode 2, got %d", got)
	}
	if got := ExitCode(errors.New("plain")); got != 0 {
		t.Errorf("expected ExitCode 0 for plain error, got %d", got)
	}
}

// =============================================================================
// Scan error → exit code 2
// =============================================================================

func TestSecurityScan_InvalidSeverity_ExitCode2(t *testing.T) {
	root, buf := buildSecurityRootCommand(t)
	root.SetArgs([]string{"security", "scan", "--severity", "banana", t.TempDir()})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected an error for an invalid severity value")
	}
	if got := ExitCode(err); got != 2 {
		t.Errorf("expected exit code 2 for scan error, got %d", got)
	}
	_ = buf
}

func TestSecurityScan_InvalidSeverity_JSON(t *testing.T) {
	root, buf := buildSecurityRootCommand(t)
	root.SetArgs([]string{"security", "scan", "--json", "--severity", "banana", t.TempDir()})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected an error for an invalid severity value")
	}
	if got := ExitCode(err); got != 2 {
		t.Errorf("expected exit code 2 for JSON scan error, got %d", got)
	}
	out := strings.TrimSpace(buf.String())
	var parsed map[string]interface{}
	if uerr := json.Unmarshal([]byte(out), &parsed); uerr != nil {
		t.Fatalf("JSON error output is not valid JSON: %v\n%s", uerr, out)
	}
	if parsed["error"] == nil {
		t.Error("expected 'error' key in JSON error output")
	}
}
