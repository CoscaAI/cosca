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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/CoscaAI/cosca/internal/skilleval"
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

// =============================================================================
// skill evolve --no-llm — FATIA 3.2: split + holdout regression gate
// =============================================================================

// evolveRegressionBaselineBody is a baseline body that carries the "fix:
// holdout" marker (rewarded by the holdout rubric) but is free of the "evolved"
// token. It is long and realistic enough that the guardrails (size, growth,
// semantic preservation, structure) all pass, so the ONLY thing that can block
// the promotion is the holdout regression gate.
const evolveRegressionBaselineBody = `# Overview

This skill guides building reliable retry logic for transient failures.

## Steps
1. Detect transient errors and classify them.
2. fix: holdout
3. Retry with exponential backoff and jitter.
4. Emit a structured log line with request metadata.
5. Return a bounded response with a content-type header.

The skill never leaks internals and always returns a bounded response.`

// runSkillEvolveOpts executes runSkillEvolve directly with a custom
// skillEvolveOptions (test seam), so a test can inject a deterministic
// mutator/scorer without going through the public command flags.
func runSkillEvolveOpts(t *testing.T, opts skillEvolveOptions, args []string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "evolve"}
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true) // noColor
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	err := runSkillEvolve(cmd, args[0], opts)
	return buf.String(), err
}

// writeEvolveSkillCustom grava a SKILL.md de uma skill com um corpo arbitrário.
func writeEvolveSkillCustom(t *testing.T, dir, name, body string) string {
	t.Helper()
	dirPath := filepath.Join(dir, ".cosca", "skills", name)
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatal(err)
	}
	content := evolveSkillFrontmatter + body
	if err := os.WriteFile(filepath.Join(dirPath, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dirPath, "SKILL.md")
}

// writeEvolveEval grava uma .eval.yaml sob .cosca/evals/skills/<name>.eval.yaml
// com o conjunto de casos dado (mesma forma que o CLI lê).
func writeEvolveEval(t *testing.T, dir, name string, cases []skilleval.SkillCase) {
	t.Helper()
	evalDir := filepath.Join(dir, ".cosca", "evals", skilleval.EvalDirName)
	if err := os.MkdirAll(evalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString("name: " + name + "\n")
	b.WriteString("description: demo\n")
	b.WriteString("version: dev\n")
	b.WriteString("scorer: static\n")
	b.WriteString("cases:\n")
	for _, c := range cases {
		b.WriteString("  - id: " + c.ID + "\n")
		b.WriteString("    task: " + c.Task + "\n")
		b.WriteString("    rubric:\n")
		for _, r := range c.Rubric {
			// Quote each rubric value so a token containing a colon (e.g.
			// "fix: holdout") is parsed as a scalar string, not as a YAML map.
			b.WriteString("      - " + strconv.Quote(r) + "\n")
		}
	}
	if err := os.WriteFile(filepath.Join(evalDir, name+".eval.yaml"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// cliEvolveSplitHoldout descobre, deterministicamente (mesmo skill + seed do
// CLI: DefaultPromotionSeed), quais IDs caem no holdout de um dataset de n
// casos. O CLI usa esse mesmo split no gate de regressão, então o teste pode
// rotular os casos do holdout como o marcador que o candidato DERRUBA.
func cliEvolveSplitHoldout(t *testing.T, skill string, n int) map[string]bool {
	t.Helper()
	cases := make([]skilleval.SkillCase, 0, n)
	for i := 0; i < n; i++ {
		cases = append(cases, skilleval.SkillCase{ID: "p" + strconv.Itoa(i), Rubric: []string{"x"}})
	}
	split, err := skilleval.SplitEval(skill, cases, skilleval.DefaultSplitRatio, skilleval.DefaultPromotionSeed)
	require.NoError(t, err)
	hold := make(map[string]bool, len(split.Holdout))
	for _, c := range split.Holdout {
		hold[c.ID] = true
	}
	return hold
}

// cliEvolveCases monta n casos; os IDs que caem no holdout ganham o marcador
// "fix: holdout" (que o candidato perde), os demais ganham "evolved" (que o
// candidato ganha). Assim o candidato melhora no treino/validação mas regride
// no holdout — o overfit que o gate deve barrar.
func cliEvolveCases(n int, holdoutIDs map[string]bool) []skilleval.SkillCase {
	cases := make([]skilleval.SkillCase, 0, n)
	for i := 0; i < n; i++ {
		id := "p" + strconv.Itoa(i)
		rubric := []string{"evolved"}
		if holdoutIDs[id] {
			rubric = []string{"fix: holdout"}
		}
		cases = append(cases, skilleval.SkillCase{ID: id, Task: "case " + strconv.Itoa(i), Rubric: rubric})
	}
	return cases
}

// cliEvolveCasesAllEvolved monta n casos que todos recompensam o token "evolved"
// que o mutator estático introduz — o candidato melhora em TODOS (treino e
// holdout), então o gate de regressão passa e a promoção acontece.
func cliEvolveCasesAllEvolved(n int) []skilleval.SkillCase {
	cases := make([]skilleval.SkillCase, 0, n)
	for i := 0; i < n; i++ {
		cases = append(cases, skilleval.SkillCase{
			ID: "p" + strconv.Itoa(i), Task: "case " + strconv.Itoa(i), Rubric: []string{"evolved"},
		})
	}
	return cases
}

func TestSkillEvolve_NoLLM_RegressionGateBlocksNoBranch(t *testing.T) {
	dir := chdirSkillEvolveTemp(t)
	writeEvolveSkillCustom(t, dir, "demo-skill", evolveRegressionBaselineBody)

	// Descobre o holdout determinístico (mesmo seed do CLI) e rotula os casos:
	// os do holdout recompensam "fix: holdout"; os demais recompensam "evolved".
	holdoutIDs := cliEvolveSplitHoldout(t, "demo-skill", 12)
	cases := cliEvolveCases(12, holdoutIDs)
	writeEvolveEval(t, dir, "demo-skill", cases)

	originalBranch := initEvolveRepo(t, dir)

	// Injeta um mutator determinístico que SUBSTITUI o marcador do holdout pelo
	// "evolved": o candidato melhora no treino/validação, mas perde o marcador
	// no holdout — o overfit que o gate de regressão deve barrar.
	mutator := skilleval.MutatorStatic(func(body string, _ []string) string {
		return strings.Replace(body, "fix: holdout", "evolved", 1)
	})
	opts := skillEvolveOptions{
		noLLM:      true,
		iterations: 3,
		trials:     skilleval.MinTrials,
		maxLines:   skilleval.DefaultMaxSkillLines,
		maxGrowth:  skilleval.DefaultMaxGrowthPct,
		now:        func() time.Time { return time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC) },
		mutator:    mutator,
		scorer:     skilleval.StaticScorer{},
	}
	out, err := runSkillEvolveOpts(t, opts, []string{"demo-skill"})
	require.Error(t, err, "a regressão no holdout deve bloquear a promoção")
	require.Contains(t, out, "gate de regressão")
	require.Contains(t, out, "Nenhum branch/PR criado")

	// O gate de regressão falhou => NENHUM branch é criado.
	if current := gitRun(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); current != originalBranch {
		t.Fatalf("regressão no holdout não deve criar branch: %q vs %q", current, originalBranch)
	}
	// A skill no disco permanece a baseline (sem Apply evoluído).
	md, err := os.ReadFile(filepath.Join(dir, ".cosca", "skills", "demo-skill", "SKILL.md"))
	require.NoError(t, err)
	require.Contains(t, string(md), "fix: holdout", "o corpo da skill não deve ser evoluído quando o gate falha")
}

func TestSkillEvolve_NoLLM_ImprovesHoldoutCreatesBranch(t *testing.T) {
	dir := chdirSkillEvolveTemp(t)
	writeEvolveSkill(t, dir, "demo-skill")
	initEvolveRepo(t, dir)
	// Todo o dataset recompensa "evolved": o candidato melhora no treino/validação
	// E no holdout, então o gate de regressão passa e a promoção acontece.
	writeEvolveEval(t, dir, "demo-skill", cliEvolveCasesAllEvolved(12))

	cmd := NewSkillEvolveCommand()
	_ = cmd.ParseFlags([]string{"--no-llm"})
	out, err := runSkillEvolveCommand(t, cmd, []string{"demo-skill"})
	require.NoError(t, err)
	require.Contains(t, out, "candidato a PR")
	require.Contains(t, out, "Holdout regression")
	require.Contains(t, out, "passed=true", "o gate de regressão no holdout deve passar para promover")

	current := gitRun(t, dir, "rev-parse", "--abbrev-ref", "HEAD")
	require.True(t, strings.HasPrefix(current, "evolve/demo-skill-"), "uma melhoria no holdout deve criar o branch evolve")
}
