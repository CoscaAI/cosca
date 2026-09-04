package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// runUpgradeDirect executa runUpgrade (o corpo real de `cosca upgrade`) com um
// formatter injetado num buffer, sem depender do binding de flag. O Caller deve
// fazer t.Chdir(dir) antes, pois runUpgrade usa os.Getwd().
func runUpgradeDirect(t *testing.T, dryRun bool) (string, error) {
	t.Helper()
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true) // noColor
	cmd.SetContext(newContextWithFormatter(context.Background(), f))
	err := runUpgrade(cmd, dryRun)
	return buf.String(), err
}

// =============================================================================
// (a) upgrade preserva a zona LIVE (.opencode/cosca) e alerta DRIFT
// =============================================================================

func TestUpgrade_PreservesLiveZone_AndDetectsDrift(t *testing.T) {
	dir := t.TempDir()
	liveRoot := filepath.Join(dir, ".opencode", "cosca")
	if err := os.MkdirAll(liveRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	// Um arquivo vindo do LIVE (evolução). Sem FROZEN em disco → só-LIVE → DRIFT.
	if err := os.WriteFile(filepath.Join(liveRoot, "KERNEL.md"), []byte("# LIVE kernel\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	// Com --dry-run: NÃO remove o LIVE e ALERTA DRIFT.
	out, err := runUpgradeDirect(t, true)
	if err != nil {
		t.Fatalf("upgrade --dry-run: %v", err)
	}
	if !strings.Contains(out, "DRIFT") {
		t.Errorf("--dry-run deveria alertar DRIFT (zona LIVE vs FROZEN):\n%s", out)
	}
	if !strings.Contains(out, "Zona LIVE detectada") || !strings.Contains(out, "PROTEGIDO") {
		t.Errorf("--dry-run deveria reconhecer a zona LIVE como protegida:\n%s", out)
	}
	if _, statErr := os.Stat(liveRoot); statErr != nil {
		t.Fatalf("--dry-run NÃO pode remover o LIVE: %v", statErr)
	}

	// Sem --dry-run: TAMBÉM NÃO faz RemoveAll do LIVE (mantém recuperável).
	out2, err := runUpgradeDirect(t, false)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if _, statErr := os.Stat(liveRoot); statErr != nil {
		t.Fatalf("upgrade NÃO pode fazer RemoveAll do LIVE (deve permanecer recuperável): %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(liveRoot, "KERNEL.md")); statErr != nil {
		t.Errorf("conteúdo do LIVE deve permanecer: %v", statErr)
	}
	if !strings.Contains(out2, "Preservando") || !strings.Contains(out2, "PROTEGIDO") {
		t.Errorf("upgrade deveria reportar o LIVE como preservado:\n%s", out2)
	}

	// Snapshot do LIVE gerado em .cosca/backups/pre-upgrade-*/live-snapshot/.
	snapshots, _ := filepath.Glob(filepath.Join(dir, ".cosca", "backups", "pre-upgrade-*", "live-snapshot", "KERNEL.md"))
	if len(snapshots) < 1 {
		t.Errorf("upgrade deveria criar snapshot da zona LIVE, nenhum encontrado em %s", filepath.Join(dir, ".cosca", "backups"))
	}
}

// =============================================================================
// (d) upgrade NÃO RemoveAll .cosca/fallback (alvo ativo do MaterializeFallback)
// =============================================================================

func TestUpgrade_PreservesFallbackTarget(t *testing.T) {
	dir := t.TempDir()
	fallback := filepath.Join(dir, ".cosca", "fallback")
	sub := filepath.Join(fallback, "knowledge")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "X.md"), []byte("conteúdo do fallback\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	out, err := runUpgradeDirect(t, false)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if _, statErr := os.Stat(fallback); statErr != nil {
		t.Fatalf("upgrade NÃO pode remover .cosca/fallback (alvo do MaterializeFallback): %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(sub, "X.md")); statErr != nil {
		t.Errorf("conteúdo de .cosca/fallback deve permanecer: %v", statErr)
	}
	if !strings.Contains(out, "PRESERVADO") || !strings.Contains(out, "MaterializeFallback") {
		t.Errorf("upgrade deveria reportar .cosca/fallback como preservado:\n%s", out)
	}
}

// =============================================================================
// (a) sem zona LIVE presente → sem drift (não-regressão)
// =============================================================================

func TestUpgrade_NoLiveNoDrift(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	out, err := runUpgradeDirect(t, false)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if strings.Contains(out, "DRIFT") {
		t.Errorf("sem zona LIVE não deveria haver alerta de DRIFT:\n%s", out)
	}
}
