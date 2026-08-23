//
// Tests for `cosca skill evolve` — promoção via PR de skills (internal/cli/
// skill_evolve.go + internal/skilleval/guardrail.go).
//
// Covers:
//   - Registration of `evolve` in the `skill` command tree
//   - `skill evolve <name> --no-llm` em um repo de teste temporário → cria o
//     branch evolve/<skill>-<timestamp>, escreve a skill evoluída (Apply) e faz
//     o commit local, SEM push/merge/auto-deploy (deixa como candidato a PR).
//   - `skill evolve <name> --no-llm --dry-run` → mostra o diff e NÃO cria
//     branch/commit nem altera o disco.
//   - Guardrail de tamanho bloqueando (via --max-growth) → não cria branch.
//
// NOTE: os testes fazem chdir() e NÃO rodam em paralelo. Usam git real num
// repositório temporário. Nenhum LLM é invocado: o caminho é --no-llm
// (StaticMutator + StaticScorer).

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

// chdirSkillEvolveTemp muda para um diretório temporário e restaura o cwd no
// cleanup. Os comandos operam em <tmp> (que também é o repo git).
func chdirSkillEvolveTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	globalFlags = GlobalFlags{}
	return dir
}

// runSkillEvolveCommand executa o RunE de um subcomando com formatter injetado
// em um buffer (padrão de injeção de formatter do CLI).
func runSkillEvolveCommand(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
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

// gitRun executa um comando git no repositório dir.
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := runGitCmd(dir, args...)
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(out)
}

// initEvolveRepo cria um repo git limpo (com um commit inicial) em dir e
// devolve o branch corrente original (main/master).
func initEvolveRepo(t *testing.T, dir string) string {
	t.Helper()
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "config", "user.email", "test@cosca.enterprise")
	gitRun(t, dir, "config", "user.name", "Cosca Test")
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "initial")
	return gitRun(t, dir, "rev-parse", "--abbrev-ref", "HEAD")
}

const evolveSkillFrontmatter = `---
name: demo-skill
description: Skill de demonstração para o survive do GEPA
level: 2
---
`

// evolveSkillBody is the baseline body: long enough that a +1-line mutation
// stays within the default 20% growth cap, and free of the "evolved" token so
// the static scorer grades the candidate arm strictly better.
var evolveSkillBody = strings.Join([]string{
	"# Overview",
	"",
	"This skill helps an agent build reliable REST endpoints.",
	"It covers validation, error handling, idempotency, and observability.",
	"Every request must be validated before the handler is invoked.",
	"",
	"## Steps",
	"1. Validate the incoming payload and reject malformed bodies.",
	"2. Run the business rule and collect the result object.",
	"3. Map domain errors to a stable, documented HTTP status code.",
	"4. Emit a structured log line with request and outcome metadata.",
	"5. Return the payload with the appropriate content-type header.",
	"",
	"The skill never leaks internals and always returns a bounded response.",
}, "\n")

// writeEvolveSkill grava a SKILL.md de uma skill padrão sob .cosca/skills/<name>/.
func writeEvolveSkill(t *testing.T, dir, name string) string {
	t.Helper()
	dirPath := filepath.Join(dir, ".cosca", "skills", name)
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatal(err)
	}
	content := evolveSkillFrontmatter + evolveSkillBody
	if err := os.WriteFile(filepath.Join(dirPath, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dirPath, "SKILL.md")
}

// skipIfNoGit is intentionally absent: git failures surface through gitRun.

// =============================================================================
// Registration
// =============================================================================

func TestSkillEvolve_RegisteredInSkillCommand(t *testing.T) {
	cmd := NewSkillCommand()
	for _, sub := range cmd.Commands() {
		if sub.Name() == "evolve" {
			return
		}
	}
	t.Error("missing skill subcommand: evolve")
}

// =============================================================================
// skill evolve --no-llm — branch local (sem auto-deploy)
// =============================================================================

func TestSkillEvolve_NoLLM_CreatesBranchNoAutoDeploy(t *testing.T) {
	dir := chdirSkillEvolveTemp(t)
	writeEvolveSkill(t, dir, "demo-skill")
	originalBranch := initEvolveRepo(t, dir)

	cmd := NewSkillEvolveCommand()
	_ = cmd.ParseFlags([]string{"--no-llm"})
	out, err := runSkillEvolveCommand(t, cmd, []string{"demo-skill"})
	if err != nil {
		t.Fatalf("skill evolve --no-llm: %v", err)
	}

	// A saída indica candidato a PR e o branch evolve/...
	if !strings.Contains(out, "candidato a PR") {
		t.Errorf("output deve declarar candidato a PR: %q", out)
	}
	if !strings.Contains(out, "evolve/demo-skill-") {
		t.Errorf("output deve citar o branch evolve/demo-skill-*: %q", out)
	}

	// O branch corrente é evolve/demo-skill-<timestamp> (não o branch original).
	current := gitRun(t, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if !strings.HasPrefix(current, "evolve/demo-skill-") {
		t.Fatalf("current branch = %q, want evolve/demo-skill-*", current)
	}
	if current == originalBranch {
		t.Fatalf("branch não foi trocado para o branch de evolução (%q)", current)
	}

	// A skill no disco foi atualizada (Apply): contém o marcador de mutação.
	md, err := os.ReadFile(filepath.Join(dir, ".cosca", "skills", "demo-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("read evolved SKILL.md: %v", err)
	}
	if !strings.Contains(string(md), "evolved") {
		t.Errorf("SKILL.md evoluída deve conter o marcador 'evolved': %q", string(md))
	}
	if !strings.Contains(string(md), "name: demo-skill") {
		t.Errorf("identidade deve ser preservada (name) na SKILL.md: %q", string(md))
	}

	// Um commit local foi criado; o log tem os commits inicial + evolve.
	log := gitRun(t, dir, "log", "--oneline")
	lines := strings.Split(log, "\n")
	if len(lines) < 2 {
		t.Fatalf("esperado >= 2 commits (inicial + evolve), got %d", len(lines))
	}
	if !strings.Contains(lines[0], "evolve(demo-skill)") {
		t.Errorf("commit mais recente deve ser o evolve: %q", lines[0])
	}

	// Working tree limpo: o commit capturou a mudança.
	status := gitRun(t, dir, "status", "--porcelain")
	if status != "" {
		t.Errorf("working tree deve estar limpo após o commit, got: %q", status)
	}

	// SEM auto-deploy: não há remote configurado (nada para push).
	if remote := gitRun(t, dir, "remote"); remote != "" {
		t.Errorf("não deve haver remote configurado (sem push): %q", remote)
	}
}

// =============================================================================
// skill evolve --no-llm --dry-run — apenas o diff, nada persistido
// =============================================================================

func TestSkillEvolve_NoLLM_DryRunOnlyShowsDiff(t *testing.T) {
	dir := chdirSkillEvolveTemp(t)
	originalMd := writeEvolveSkill(t, dir, "demo-skill")
	originalBranch := initEvolveRepo(t, dir)

	beforeLog := gitRun(t, dir, "log", "--oneline")

	cmd := NewSkillEvolveCommand()
	_ = cmd.ParseFlags([]string{"--no-llm", "--dry-run"})
	out, err := runSkillEvolveCommand(t, cmd, []string{"demo-skill"})
	if err != nil {
		t.Fatalf("skill evolve --no-llm --dry-run: %v", err)
	}

	if !strings.Contains(out, "dry-run") {
		t.Errorf("output deve indicar dry-run: %q", out)
	}
	if !strings.Contains(out, "Diff") {
		t.Errorf("output deve mostrar o Diff: %q", out)
	}

	// Nenhum branch/commit criado: branch e log permanecem os mesmos.
	if current := gitRun(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); current != originalBranch {
		t.Fatalf("dry-run não deve trocar de branch: %q vs %q", current, originalBranch)
	}
	if afterLog := gitRun(t, dir, "log", "--oneline"); afterLog != beforeLog {
		t.Fatalf("dry-run não deve criar commit: %q vs %q", afterLog, beforeLog)
	}

	// Disco inalterado.
	after, err := os.ReadFile(originalMd)
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	if string(after) != evolveSkillFrontmatter+evolveSkillBody {
		t.Errorf("dry-run não deve alterar a SKILL.md")
	}
}

// =============================================================================
// skill evolve --no-llm — guardrail de tamanho bloqueia → sem branch
// =============================================================================

func TestSkillEvolve_NoLLM_GrowthGuardrailBlocks(t *testing.T) {
	dir := chdirSkillEvolveTemp(t)
	writeEvolveSkill(t, dir, "demo-skill")
	originalBranch := initEvolveRepo(t, dir)

	cmd := NewSkillEvolveCommand()
	_ = cmd.ParseFlags([]string{"--no-llm", "--max-growth", "0.1"})
	_, err := runSkillEvolveCommand(t, cmd, []string{"demo-skill"})
	if err == nil {
		t.Fatal("esperado erro ao violar o guardrail de crescimento")
	}
	if current := gitRun(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); current != originalBranch {
		t.Fatalf("guardrail que falhou não deve criar branch: %q vs %q", current, originalBranch)
	}
}

// =============================================================================
// skill evolve — LLM (não ligado) degrada sem candidato
// =============================================================================

func TestSkillEvolve_LLMNotWired_NoBranch(t *testing.T) {
	dir := chdirSkillEvolveTemp(t)
	writeEvolveSkill(t, dir, "demo-skill")
	originalBranch := initEvolveRepo(t, dir)

	cmd := NewSkillEvolveCommand()
	out, err := runSkillEvolveCommand(t, cmd, []string{"demo-skill"})
	if err != nil {
		t.Fatalf("skill evolve (default LLM) should degrade gracefully: %v", err)
	}
	if !strings.Contains(out, "use --no-llm") {
		t.Errorf("output deve orientar o uso de --no-llm: %q", out)
	}
	if current := gitRun(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); current != originalBranch {
		t.Fatalf("LLM não ligado não deve criar branch: %q vs %q", current, originalBranch)
	}
}
