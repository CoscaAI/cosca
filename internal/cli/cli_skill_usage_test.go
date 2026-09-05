//
// Tests for `cosca skill use/status/curator/restore` — telemetria de uso +
// curador (internal/cli/skill.go + internal/skills/usage.go).
//
// Covers:
//   - Registration of use/status/curator/restore in the `skill` command tree
//   - `skill use <name>` → grava .usage.json (incrementa + timestamp)
//   - `skill status` → tabela (skill, category, uses, last activity, state)
//   - `skill curator --dry-run` → lista candidatos e NÃO altera nada
//   - `skill curator --archive` → move locais para .archive/, marca embedded
//   - `skill restore <name>` → archived → active, fonte de volta
//   - Regressão: `skill list` continua funcionando
//
// NOTE: os testes fazem chdir() e NÃO rodam em paralelo. Formatter injetado
// via newContextWithFormatter (padrão do CLI).

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/skills"
)

// chdirSkillUsageTemp muda para um diretório temporário e restaura o cwd no
// cleanup. Os comandos operam em <tmp>/.cosca/skills.
func chdirSkillUsageTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Chdir registra o restore do cwd DEPOIS do RemoveAll do TempDir
	// (cleanups rodam em LIFO) — no Windows não dá para remover o
	// diretório que é o CWD do processo.
	t.Chdir(dir)
	globalFlags = GlobalFlags{}
	return dir
}

// runSkillUsageCommand executa o RunE de um subcomando com formatter injetado
// em um buffer (padrão de injeção de formatter do CLI).
func runSkillUsageCommand(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true) // noColor
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	err := cmd.RunE(cmd, args)
	return buf.String(), err
}

// writeLocalSkill cria uma skill local .cosca/skills/<name>/<name>.md para o
// manager enxergá-la (e o curador poder mover a fonte).
func writeLocalSkill(t *testing.T, dir, name string) {
	t.Helper()
	dirPath := filepath.Join(dir, ".cosca", "skills", name)
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "# " + name + "\n\n## Description\n" + name + " skill local de teste\n"
	if err := os.WriteFile(filepath.Join(dirPath, name+".md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// usageFilePath retorna o caminho do sidecar de telemetria do projeto.
func usageFilePath(dir string) string {
	return filepath.Join(dir, ".cosca", "skills", ".usage.json")
}

// backdateSkillUsage grava a skill como usada há <days> dias (para o curador
// considerá-la stale).
func backdateSkillUsage(t *testing.T, dir, name string, days int) {
	t.Helper()
	store, err := skills.NewUsageStore(filepath.Join(dir, ".cosca"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordUse(name); err != nil {
		t.Fatal(err)
	}
	snap, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	u := snap[name]
	u.LastActivityAt = time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)
	snap[name] = u
	if err := store.Save(snap); err != nil {
		t.Fatal(err)
	}
}

// =============================================================================
// Registration
// =============================================================================

func TestSkillUsage_RegisteredInSkillCommand(t *testing.T) {
	cmd := NewSkillCommand()
	registered := map[string]bool{}
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, want := range []string{"use", "status", "curator", "restore"} {
		if !registered[want] {
			t.Errorf("missing skill subcommand: %s", want)
		}
	}
	// os subcomandos pré-existentes continuam presentes (sem regressão)
	for _, want := range []string{"list", "show", "search", "install"} {
		if !registered[want] {
			t.Errorf("missing pre-existing skill subcommand: %s", want)
		}
	}
}

func TestSkillUsage_ArgConstraints(t *testing.T) {
	use := NewSkillUseCommand()
	if err := use.Args(use, nil); err == nil {
		t.Error("skill use should require a name")
	}
	restore := NewSkillRestoreCommand()
	if err := restore.Args(restore, nil); err == nil {
		t.Error("skill restore should require a name")
	}
	curator := NewSkillCuratorCommand()
	if err := curator.Args(curator, []string{"extra"}); err == nil {
		t.Error("skill curator should accept no args")
	}
}

// =============================================================================
// skill use
// =============================================================================

func TestSkillUseCommand_RecordsUse(t *testing.T) {
	dir := chdirSkillUsageTemp(t)
	writeLocalSkill(t, dir, "alpha-skill")

	out, err := runSkillUsageCommand(t, NewSkillUseCommand(), []string{"alpha-skill"})
	if err != nil {
		t.Fatalf("skill use: %v", err)
	}
	if !strings.Contains(out, "alpha-skill") {
		t.Errorf("use output missing skill name: %q", out)
	}
	if !strings.Contains(out, "use_count=1") {
		t.Errorf("use output missing use_count: %q", out)
	}

	store, err := skills.NewUsageStore(filepath.Join(dir, ".cosca"))
	if err != nil {
		t.Fatal(err)
	}
	u, err := store.Get("alpha-skill")
	if err != nil {
		t.Fatal(err)
	}
	if u.UseCount != 1 || u.LastActivityAt == "" || u.State != skills.StateActive {
		t.Errorf("unexpected usage: %+v", u)
	}
	if _, err := os.Stat(usageFilePath(dir)); err != nil {
		t.Errorf("usage file must exist: %v", err)
	}
}

func TestSkillUseCommand_UnknownSkillFails(t *testing.T) {
	chdirSkillUsageTemp(t)
	if _, err := runSkillUsageCommand(t, NewSkillUseCommand(), []string{"definitely-not-a-skill-xyz"}); err == nil {
		t.Error("expected error for unknown skill")
	}
}

// =============================================================================
// skill status
// =============================================================================

func TestSkillStatusCommand_Table(t *testing.T) {
	dir := chdirSkillUsageTemp(t)
	writeLocalSkill(t, dir, "alpha-skill")

	if _, err := runSkillUsageCommand(t, NewSkillUseCommand(), []string{"alpha-skill"}); err != nil {
		t.Fatal(err)
	}

	out, err := runSkillUsageCommand(t, NewSkillStatusCommand(), nil)
	if err != nil {
		t.Fatalf("skill status: %v", err)
	}
	if !strings.Contains(out, "alpha-skill") {
		t.Errorf("status missing skill: %q", out)
	}
	if !strings.Contains(out, "ativo") {
		t.Errorf("status missing active state label: %q", out)
	}
	if !strings.Contains(out, "Uses") {
		t.Errorf("status missing column header: %q", out)
	}
	if !strings.Contains(out, "Last activity") {
		t.Errorf("status missing activity column: %q", out)
	}
}

// =============================================================================
// skill curator --dry-run
// =============================================================================

func TestSkillCuratorDryRun_ListsAndChangesNothing(t *testing.T) {
	dir := chdirSkillUsageTemp(t)
	writeLocalSkill(t, dir, "old-skill")
	writeLocalSkill(t, dir, "new-skill")

	// new-skill usada hoje; old-skill usada há 40 dias
	if _, err := runSkillUsageCommand(t, NewSkillUseCommand(), []string{"new-skill"}); err != nil {
		t.Fatal(err)
	}
	backdateSkillUsage(t, dir, "old-skill", 40)

	cur := NewSkillCuratorCommand()
	_ = cur.ParseFlags([]string{"--dry-run", "--days", "30"})
	out, err := runSkillUsageCommand(t, cur, nil)
	if err != nil {
		t.Fatalf("curator dry-run: %v", err)
	}
	if !strings.Contains(out, "old-skill") {
		t.Errorf("dry-run should list old-skill: %q", out)
	}
	if strings.Contains(out, "new-skill") {
		t.Errorf("dry-run should NOT list fresh skill: %q", out)
	}
	if !strings.Contains(out, "NADA foi alterado") {
		t.Errorf("dry-run should state nothing changed: %q", out)
	}

	// NADA mudou: estado permanece active e a fonte não foi movida
	store, err := skills.NewUsageStore(filepath.Join(dir, ".cosca"))
	if err != nil {
		t.Fatal(err)
	}
	snap, _ := store.Snapshot()
	if snap["old-skill"].State != skills.StateActive {
		t.Errorf("dry-run must not change state: %+v", snap["old-skill"])
	}
	if _, err := os.Stat(filepath.Join(dir, ".cosca", "skills", "old-skill")); err != nil {
		t.Errorf("dry-run must not move files: %v", err)
	}
}

// =============================================================================
// skill curator --archive
// =============================================================================

func TestSkillCuratorArchive_ArchivesAndNeverDeletes(t *testing.T) {
	dir := chdirSkillUsageTemp(t)
	writeLocalSkill(t, dir, "stale-a")
	writeLocalSkill(t, dir, "stale-b")

	backdateSkillUsage(t, dir, "stale-a", 45)
	backdateSkillUsage(t, dir, "stale-b", 45)

	// skill embutida (sem fonte local): apenas marcada, nunca movida
	store, err := skills.NewUsageStore(filepath.Join(dir, ".cosca"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordUse("embedded-stale"); err != nil {
		t.Fatal(err)
	}
	snap, _ := store.Snapshot()
	u := snap["embedded-stale"]
	u.LastActivityAt = time.Now().UTC().AddDate(0, 0, -60).Format(time.RFC3339)
	snap["embedded-stale"] = u
	if err := store.Save(snap); err != nil {
		t.Fatal(err)
	}

	cur := NewSkillCuratorCommand()
	_ = cur.ParseFlags([]string{"--archive", "--days", "30"})
	out, err := runSkillUsageCommand(t, cur, nil)
	if err != nil {
		t.Fatalf("curator archive: %v", err)
	}

	// locais movidos para .archive/ e marcados archived (nunca deletados)
	for _, n := range []string{"stale-a", "stale-b"} {
		archivedDir := filepath.Join(dir, ".cosca", "skills", ".archive", n)
		if _, err := os.Stat(filepath.Join(archivedDir, n+".md")); err != nil {
			t.Errorf("archived content must exist (never deleted): %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, ".cosca", "skills", n)); !os.IsNotExist(err) {
			t.Errorf("local dir should have moved for %s (err=%v)", n, err)
		}
		got, _ := store.Get(n)
		if got.State != skills.StateArchived {
			t.Errorf("%s state = %q, want archived", n, got.State)
		}
	}

	// embutida: apenas marcada, sem movimento
	eu, _ := store.Get("embedded-stale")
	if eu.State != skills.StateArchived {
		t.Errorf("embedded-stale state = %q, want archived", eu.State)
	}
	if !strings.Contains(eu.Notes, "embedded") {
		t.Errorf("embedded-stale notes should mention embedded: %q", eu.Notes)
	}
	if _, err := os.Stat(filepath.Join(dir, ".cosca", "skills", ".archive", "embedded-stale")); !os.IsNotExist(err) {
		t.Errorf("embedded skill must NOT be moved (err=%v)", err)
	}

	if !strings.Contains(out, "NUNCA deleta") {
		t.Errorf("output should document never-delete policy: %q", out)
	}
}

// =============================================================================
// skill restore
// =============================================================================

func TestSkillRestoreCommand_MovesBack(t *testing.T) {
	dir := chdirSkillUsageTemp(t)
	writeLocalSkill(t, dir, "revive-skill")

	backdateSkillUsage(t, dir, "revive-skill", 50)

	cur := NewSkillCuratorCommand()
	_ = cur.ParseFlags([]string{"--archive", "--days", "30"})
	if _, err := runSkillUsageCommand(t, cur, nil); err != nil {
		t.Fatalf("curator archive: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".cosca", "skills", ".archive", "revive-skill")); err != nil {
		t.Fatalf("precondition: skill should be archived: %v", err)
	}

	out, err := runSkillUsageCommand(t, NewSkillRestoreCommand(), []string{"revive-skill"})
	if err != nil {
		t.Fatalf("skill restore: %v", err)
	}
	if !strings.Contains(out, "restaurada") {
		t.Errorf("restore output missing confirmation: %q", out)
	}
	if _, err := os.Stat(filepath.Join(dir, ".cosca", "skills", "revive-skill", "revive-skill.md")); err != nil {
		t.Errorf("restored source must be back: %v", err)
	}
	store, err := skills.NewUsageStore(filepath.Join(dir, ".cosca"))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := store.Get("revive-skill")
	if u.State != skills.StateActive {
		t.Errorf("state = %q, want active", u.State)
	}
}

// =============================================================================
// Curador sem flag → erro (nunca roda sozinho)
// =============================================================================

func TestSkillCurator_RequiresDryRunOrArchive(t *testing.T) {
	chdirSkillUsageTemp(t)
	if _, err := runSkillUsageCommand(t, NewSkillCuratorCommand(), nil); err == nil {
		t.Error("curator with neither --dry-run nor --archive should fail")
	}
}

// =============================================================================
// Regressão: skill list continua funcionando
// =============================================================================

func TestSkillList_StillWorksAfterTelemetry(t *testing.T) {
	dir := chdirSkillUsageTemp(t)
	writeLocalSkill(t, dir, "listable-skill")

	out, err := runSkillUsageCommand(t, NewSkillListCommand(), nil)
	if err != nil {
		t.Fatalf("skill list: %v", err)
	}
	if !strings.Contains(out, "listable-skill") {
		t.Errorf("skill list should still list local skill: %q", out)
	}
}

// compile-time guard: os novos constructors seguem o contrato do cobra.
var (
	_ *cobra.Command = NewSkillUseCommand()
	_ *cobra.Command = NewSkillStatusCommand()
	_ *cobra.Command = NewSkillCuratorCommand()
	_ *cobra.Command = NewSkillRestoreCommand()
)
