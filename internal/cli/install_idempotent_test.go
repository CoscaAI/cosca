package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runInstallIn runs `cosca install` in dir with the given boolean flags set
// to true, and returns the formatter output. The working directory is
// restored before returning.
func runInstallIn(t *testing.T, dir string, flags ...string) (string, error) {
	t.Helper()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	cmd := NewInstallCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	for _, flag := range flags {
		if err := cmd.Flags().Set(flag, "true"); err != nil {
			t.Fatalf("set flag %s: %v", flag, err)
		}
	}
	err = cmd.RunE(cmd, nil)
	return buf.String(), err
}

// assertArtifact fails unless rel exists in coscaDir with the expected kind.
func assertArtifact(t *testing.T, coscaDir, rel string, wantDir bool) {
	t.Helper()
	path := filepath.Join(coscaDir, rel)
	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("expected artifact %s to exist: %v", path, err)
		return
	}
	if wantDir && !info.IsDir() {
		t.Errorf("expected %s to be a directory, got file", path)
	}
	if !wantDir && info.IsDir() {
		t.Errorf("expected %s to be a file, got directory", path)
	}
}

// verifiableArtifacts are the artifacts the installer can check via os.Stat.
var verifiableArtifacts = []struct {
	rel   string
	isDir bool
}{
	{rel: "plugins", isDir: true},
	{rel: "cache", isDir: true},
	{rel: "index", isDir: true},
	{rel: "runtime", isDir: true},
	{rel: "knowledge.db", isDir: false},
}

func assertInstalledTree(t *testing.T, coscaDir string) {
	t.Helper()
	for _, a := range verifiableArtifacts {
		assertArtifact(t, coscaDir, a.rel, a.isDir)
	}
}

// =============================================================================
// install --status: fresh tree → missing, nothing created
// =============================================================================

func TestInstallStatus_FreshTree_ReportsMissingWithoutCreating(t *testing.T) {
	dir := t.TempDir()
	coscaDir := filepath.Join(dir, ".cosca")

	out, err := runInstallIn(t, dir, "status")
	if err != nil {
		t.Fatalf("install --status returned error: %v", err)
	}
	if strings.Contains(out, "already configured") {
		t.Errorf("expected NOT already configured on fresh tree, got:\n%s", out)
	}
	if !strings.Contains(out, "missing") {
		t.Errorf("expected missing steps reported, got:\n%s", out)
	}
	if !strings.Contains(out, "passo(s) pendente(s)") {
		t.Errorf("expected pending-steps summary, got:\n%s", out)
	}
	if _, err := os.Stat(coscaDir); !os.IsNotExist(err) {
		t.Errorf("--status must NOT create .cosca, stat err = %v", err)
	}
}

func TestInstallCheckAlias_ReportsStatusWithoutCreating(t *testing.T) {
	dir := t.TempDir()
	coscaDir := filepath.Join(dir, ".cosca")

	out, err := runInstallIn(t, dir, "check")
	if err != nil {
		t.Fatalf("install --check returned error: %v", err)
	}
	if !strings.Contains(out, "passo(s) pendente(s)") {
		t.Errorf("expected pending-steps summary for --check, got:\n%s", out)
	}
	if _, err := os.Stat(coscaDir); !os.IsNotExist(err) {
		t.Errorf("--check must NOT create .cosca, stat err = %v", err)
	}
}

// =============================================================================
// install (full) creates artifacts; install --status after → already configured
// =============================================================================

func TestInstall_CreatesArtifacts_ThenStatusAlreadyConfigured(t *testing.T) {
	dir := t.TempDir()
	coscaDir := filepath.Join(dir, ".cosca")

	if _, err := runInstallIn(t, dir); err != nil {
		t.Fatalf("install returned error: %v", err)
	}
	assertInstalledTree(t, coscaDir)

	out, err := runInstallIn(t, dir, "status")
	if err != nil {
		t.Fatalf("install --status returned error: %v", err)
	}
	if !strings.Contains(out, "already configured ✓") {
		t.Errorf("expected 'already configured ✓' after install, got:\n%s", out)
	}
	if strings.Contains(out, "missing") {
		t.Errorf("expected no missing steps after install, got:\n%s", out)
	}
}

// =============================================================================
// install a second time → no re-creation, summary "nada a repetir"
// =============================================================================

func TestInstall_SecondRun_DoesNotRecreate(t *testing.T) {
	dir := t.TempDir()
	coscaDir := filepath.Join(dir, ".cosca")

	if _, err := runInstallIn(t, dir); err != nil {
		t.Fatalf("first install: %v", err)
	}
	assertInstalledTree(t, coscaDir)

	// Marker files prove guarded artifacts are not wiped/rebuilt on re-run.
	markers := []string{
		filepath.Join(coscaDir, "index", "marker.txt"),
		filepath.Join(coscaDir, "cache", "marker.txt"),
	}
	for _, m := range markers {
		if err := os.WriteFile(m, []byte("keep"), 0644); err != nil {
			t.Fatalf("write marker %s: %v", m, err)
		}
	}

	out, err := runInstallIn(t, dir)
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if !strings.Contains(out, "nenhuma ação repetida") {
		t.Errorf("expected 'nenhuma ação repetida' summary on second run, got:\n%s", out)
	}
	if strings.Contains(out, "reparos aplicados") {
		t.Errorf("expected no repairs on clean second run, got:\n%s", out)
	}
	if !strings.Contains(out, "already configured ✓") {
		t.Errorf("expected guarded steps to skip with 'already configured ✓', got:\n%s", out)
	}
	if strings.Contains(out, "Knowledge rebuild warning") {
		t.Errorf("second run must skip the knowledge rebuild, got:\n%s", out)
	}
	for _, m := range markers {
		if _, err := os.Stat(m); err != nil {
			t.Errorf("marker %s was removed by second install: %v", m, err)
		}
	}
}

// =============================================================================
// simulated drift (delete knowledge.db) → install repairs it and reports
// =============================================================================

func TestInstall_DriftKnowledgeDB_Repairs(t *testing.T) {
	dir := t.TempDir()
	coscaDir := filepath.Join(dir, ".cosca")

	if _, err := runInstallIn(t, dir); err != nil {
		t.Fatalf("first install: %v", err)
	}
	assertInstalledTree(t, coscaDir)

	if err := os.Remove(filepath.Join(coscaDir, "knowledge.db")); err != nil {
		t.Fatalf("remove knowledge.db: %v", err)
	}

	out, err := runInstallIn(t, dir)
	if err != nil {
		t.Fatalf("install after drift: %v", err)
	}
	if !strings.Contains(out, "detected drift") {
		t.Errorf("expected drift detection on repair run, got:\n%s", out)
	}
	if !strings.Contains(out, "reparos aplicados: 1") {
		t.Errorf("expected 'reparos aplicados: 1', got:\n%s", out)
	}
	assertArtifact(t, coscaDir, "knowledge.db", false)
}

// =============================================================================
// --status without a prior install after partial tree → drift summary
// =============================================================================

func TestInstallStatus_PartialTree_ReportsDrift(t *testing.T) {
	dir := t.TempDir()
	coscaDir := filepath.Join(dir, ".cosca")
	if err := os.MkdirAll(filepath.Join(coscaDir, "plugins"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	out, err := runInstallIn(t, dir, "status")
	if err != nil {
		t.Fatalf("install --status returned error: %v", err)
	}
	if !strings.Contains(out, "drift detected — rode cosca install") {
		t.Errorf("expected drift summary for partial tree, got:\n%s", out)
	}
	if strings.Contains(out, "already configured") {
		t.Errorf("expected NOT already configured for partial tree, got:\n%s", out)
	}
}
