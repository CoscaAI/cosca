//
// Tests for `cosca skill benchmark | history | eval list` — meta-loop A/B de
// skills (internal/cli/skill_eval.go + internal/skilleval/report.go).
//
// Covers:
//   - Registration of benchmark/history/eval in the `skill` command tree
//   - `skill benchmark <name>` com a StaticRunner fixture → persiste
//     <name>.benchmark.json + append imutável em <name>.history.json
//   - `skill benchmark --dry-run` → estima SEM persistir nada
//   - `skill history <name>` → imprime o histórico append-only
//   - `skill eval list` → lista as .eval.yaml definidas
//
// NOTE: os testes fazem chdir() e NÃO rodam em paralelo. Formatter injetado
// via newContextWithFormatter (padrão do CLI). Nenhum LLM é invocado: a runner
// é a StaticRunner de fixture e o Scorer é o StaticScorer.

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/skilleval"
)

// chdirSkillEvalTemp muda para um diretório temporário e restaura o cwd no
// cleanup. Os comandos operam em <tmp>/.cosca/evals/skills.
func chdirSkillEvalTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	globalFlags = GlobalFlags{}
	return dir
}

// runSkillEvalCommand executa o RunE de um subcomando com formatter injetado em
// um buffer (padrão de injeção de formatter do CLI).
func runSkillEvalCommand(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
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

// writeSkillEvalFile grava uma .eval.yaml em .cosca/evals/skills/ do projeto.
func writeSkillEvalFile(t *testing.T, dir, name, content string) {
	t.Helper()
	dirPath := filepath.Join(dir, ".cosca", "evals", skilleval.EvalDirName)
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirPath, name+".eval.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// skillPackPath retorna o caminho de um artefato persistido pela skill.
func skillPackPath(dir, file string) string {
	return filepath.Join(dir, ".cosca", "evals", skilleval.EvalDirName, file)
}

// demoSkillEval é a definição usada nos testes: with contém as condições da
// rubrica (o StaticScorer marca pass), without é vazio (degradado → 0).
const demoSkillEval = `name: demo-skill
description: Skill de demonstração para o A/B
version: v1
scorer: static
with: |
  return 200
  log error details
without: ""
cases:
  - id: c1
    task: handle request
    rubric:
      - return 200
      - log error
  - id: c2
    task: handle retry
    rubric:
      - retry once
`

// =============================================================================
// Registration
// =============================================================================

func TestSkillEval_RegisteredInSkillCommand(t *testing.T) {
	cmd := NewSkillCommand()
	registered := map[string]bool{}
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, want := range []string{"benchmark", "history", "eval"} {
		if !registered[want] {
			t.Errorf("missing skill subcommand: %s", want)
		}
	}
	// eval é um grupo: expõe `list`.
	evalCmd := NewSkillEvalCommand()
	evalNames := map[string]bool{}
	for _, sub := range evalCmd.Commands() {
		evalNames[sub.Name()] = true
	}
	if !evalNames["list"] {
		t.Error("missing skill eval subcommand: list")
	}
}

// =============================================================================
// skill benchmark (StaticRunner) — persistência
// =============================================================================

func TestSkillBenchmark_StaticRunner_PersistsBenchmarkAndHistory(t *testing.T) {
	dir := chdirSkillEvalTemp(t)
	writeSkillEvalFile(t, dir, "demo-skill", demoSkillEval)

	out, err := runSkillEvalCommand(t, NewSkillBenchmarkCommand(), []string{"demo-skill"})
	if err != nil {
		t.Fatalf("skill benchmark: %v", err)
	}
	if !strings.Contains(out, "demo-skill") {
		t.Errorf("benchmark output deve citar a skill: %q", out)
	}

	// Snapshot JSON + histórico append-only foram persistidos.
	if _, err := os.Stat(skillPackPath(dir, "demo-skill.benchmark.json")); err != nil {
		t.Errorf("benchmark file não criado: %v", err)
	}
	if _, err := os.Stat(skillPackPath(dir, "demo-skill.history.json")); err != nil {
		t.Errorf("history file não criado: %v", err)
	}

	// O benchmark carregado de volta confirma o resultado A/B candidato.
	bench, err := skilleval.LoadLatestBenchmark(dir, "demo-skill")
	if err != nil {
		t.Fatalf("LoadLatestBenchmark: %v", err)
	}
	if !bench.IsCandidate {
		t.Errorf("IsCandidate = %v, want true (with melhor que without)", bench.IsCandidate)
	}
	if bench.With.MedianScore <= bench.Without.MedianScore {
		t.Errorf("with (%.3f) deve superar without (%.3f)", bench.With.MedianScore, bench.Without.MedianScore)
	}

	// Histórico com uma única entrada: v1, best, parent vazio.
	hist, err := skilleval.LoadHistory(dir, "demo-skill")
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(hist) != 1 {
		t.Fatalf("history = %d entrada(s), want 1", len(hist))
	}
	if hist[0].Version != "v1" || !hist[0].IsCurrentBest || hist[0].Parent != "" {
		t.Errorf("history entry inesperada: %+v", hist[0])
	}
}

func TestSkillBenchmark_All_RunsEveryEval(t *testing.T) {
	dir := chdirSkillEvalTemp(t)
	writeSkillEvalFile(t, dir, "demo-skill", demoSkillEval)
	writeSkillEvalFile(t, dir, "other-skill", `name: other-skill
version: v2
scorer: static
with: |
  return 200
without: ""
cases:
  - id: x
    task: t
    rubric:
      - return 200
`)

	cmd := NewSkillBenchmarkCommand()
	_ = cmd.ParseFlags([]string{"--all"})
	out, err := runSkillEvalCommand(t, cmd, nil)
	if err != nil {
		t.Fatalf("skill benchmark --all: %v", err)
	}
	if !strings.Contains(out, "demo-skill") || !strings.Contains(out, "other-skill") {
		t.Errorf("--all deve rodar as duas evals: %q", out)
	}
	for _, n := range []string{"demo-skill", "other-skill"} {
		if _, err := os.Stat(skillPackPath(dir, n+".benchmark.json")); err != nil {
			t.Errorf("--all deve persistir benchmark de %s: %v", n, err)
		}
	}
}

// =============================================================================
// skill benchmark --dry-run — não persiste nada
// =============================================================================

func TestSkillBenchmark_DryRun_DoesNotPersist(t *testing.T) {
	dir := chdirSkillEvalTemp(t)
	writeSkillEvalFile(t, dir, "demo-skill", demoSkillEval)

	cmd := NewSkillBenchmarkCommand()
	_ = cmd.ParseFlags([]string{"--dry-run"})
	out, err := runSkillEvalCommand(t, cmd, []string{"demo-skill"})
	if err != nil {
		t.Fatalf("skill benchmark --dry-run: %v", err)
	}
	if !strings.Contains(out, "dry-run") {
		t.Errorf("dry-run deve indicar que nada foi executado: %q", out)
	}

	// Sem artifacts persistidos.
	if _, err := os.Stat(skillPackPath(dir, "demo-skill.benchmark.json")); !os.IsNotExist(err) {
		t.Errorf("dry-run não deve criar benchmark (err=%v)", err)
	}
	if _, err := os.Stat(skillPackPath(dir, "demo-skill.history.json")); !os.IsNotExist(err) {
		t.Errorf("dry-run não deve criar history (err=%v)", err)
	}
}

func TestSkillBenchmark_NoNameFails(t *testing.T) {
	chdirSkillEvalTemp(t)
	if _, err := runSkillEvalCommand(t, NewSkillBenchmarkCommand(), nil); err == nil {
		t.Error("skill benchmark sem nome e sem --all deve falhar")
	}
}

func TestSkillBenchmark_UnknownEvalFails(t *testing.T) {
	chdirSkillEvalTemp(t)
	if _, err := runSkillEvalCommand(t, NewSkillBenchmarkCommand(), []string{"never-defined"}); err == nil {
		t.Error("skill benchmark de eval inexistente deve falhar")
	}
}

// =============================================================================
// skill history
// =============================================================================

func TestSkillHistory_PrintsTableAfterBenchmark(t *testing.T) {
	dir := chdirSkillEvalTemp(t)
	writeSkillEvalFile(t, dir, "demo-skill", demoSkillEval)

	if _, err := runSkillEvalCommand(t, NewSkillBenchmarkCommand(), []string{"demo-skill"}); err != nil {
		t.Fatal(err)
	}

	out, err := runSkillEvalCommand(t, NewSkillHistoryCommand(), []string{"demo-skill"})
	if err != nil {
		t.Fatalf("skill history: %v", err)
	}
	if !strings.Contains(out, "demo-skill") {
		t.Errorf("history output deve citar a skill: %q", out)
	}
	if !strings.Contains(out, "v1") {
		t.Errorf("history output deve conter a versão: %q", out)
	}
	if !strings.Contains(out, "Version") || !strings.Contains(out, "PassRate") || !strings.Contains(out, "Parent") {
		t.Errorf("history output deve ter os cabeçalhos da tabela: %q", out)
	}
}

func TestSkillHistory_EmptyShowsWarning(t *testing.T) {
	chdirSkillEvalTemp(t)
	out, err := runSkillEvalCommand(t, NewSkillHistoryCommand(), []string{"never-run-skill"})
	if err != nil {
		t.Fatalf("skill history vazia: %v", err)
	}
	if !strings.Contains(out, "Sem histórico") {
		t.Errorf("history vazia deve avisar: %q", out)
	}
}

// =============================================================================
// skill eval list
// =============================================================================

func TestSkillEvalList_ListsDefined(t *testing.T) {
	dir := chdirSkillEvalTemp(t)
	writeSkillEvalFile(t, dir, "demo-skill", demoSkillEval)
	writeSkillEvalFile(t, dir, "other-skill", demoSkillEval)

	out, err := runSkillEvalCommand(t, NewSkillEvalListCommand(), nil)
	if err != nil {
		t.Fatalf("skill eval list: %v", err)
	}
	if !strings.Contains(out, "demo-skill") || !strings.Contains(out, "other-skill") {
		t.Errorf("eval list deve listar as evals: %q", out)
	}
}

func TestSkillEvalList_EmptyShowsWarning(t *testing.T) {
	chdirSkillEvalTemp(t)
	out, err := runSkillEvalCommand(t, NewSkillEvalListCommand(), nil)
	if err != nil {
		t.Fatalf("skill eval list vazia: %v", err)
	}
	if !strings.Contains(out, "Nenhuma .eval.yaml") {
		t.Errorf("eval list vazia deve avisar: %q", out)
	}
}
