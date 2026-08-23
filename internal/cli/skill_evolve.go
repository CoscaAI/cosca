//
// `cosca skill evolve <name>` — promoção via PR (guardrails + branch local).
//
// FATIA 7G (bounded) — última da Fase 2 (GEPA). Fecha o ciclo da meta-loop:
//
//   skill evolve <name>   Roda RunGEPA sobre o corpo da skill, aplica os
//                         guardrails (tamanho, preservação semântica,
//                         estrutura, cache "só nova sessão") e, se tudo OK,
//                         cria um BRANCH evolve/<skill>-<timestamp> com a skill
//                         evoluída (Apply) + um commit local. JAMAIS faz
//                         push/merge/auto-deploy — o resultado fica como
//                         "candidato a PR" para revisão humana.
//
// Flags:
//   --no-llm              caminho determinístico: StaticMutator + StaticScorer
//                         (sem LLM; é o que os testes exercem).
//   --dry-run             só mostra o diff do que SERIA produzido; não cria
//                         branch/commit e não altera nada.
//   --iterations <N>      número de iterações do RunGEPA (default 3).
//   --trials <N>          trials por braço no A/B (default = MinTrials).
//   --max-lines <N>       teto de linhas da SKILL.md (default 500).
//   --max-growth <N>      teto de crescimento % vs baseline (default 20).
//
// NOTE: o comando opera no workspace real (os.Getwd()) e faz operações git.
// `skill` já está listado em cmd/cosca/main.go (isAdminCommand e
// integrityGateBypass), então `skill evolve` roda FORA da jaula.

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/sergi/go-diff/diffmatchpatch"

	"github.com/CoscaAI/cosca/internal/skilleval"
	"github.com/CoscaAI/cosca/internal/skills"
)

// defaultEvolveIterations is the default number of GEPA iterations for
// `skill evolve`.
const defaultEvolveIterations = 3

// evolveDefaultRubricToken is the token the deterministic static mutator adds
// to a mutated body AND the default (no-eval.yaml) rubric rewards. It keeps the
// --no-llm path self-consistent: the mutation always introduces this token, so
// the static scorer grades the candidate arm strictly better than the baseline
// arm (which does not contain it), letting GEPA adopt the variant.
const evolveDefaultRubricToken = "evolved"

// skillEvolveOptions carries all tunable inputs of `skill evolve` so the
// command logic is pure and the tests can inject a deterministic clock.
type skillEvolveOptions struct {
	noLLM      bool
	dryRun     bool
	iterations int
	trials     int
	maxLines   int
	maxGrowth  float64
	now        func() time.Time
}

// NewSkillEvolveCommand creates the `cosca skill evolve <name>` subcommand.
func NewSkillEvolveCommand() *cobra.Command {
	opts := skillEvolveOptions{
		iterations: 3,
		trials:     skilleval.MinTrials,
		maxLines:   skilleval.DefaultMaxSkillLines,
		maxGrowth:  skilleval.DefaultMaxGrowthPct,
		now:        func() time.Time { return time.Now().UTC() },
	}

	cmd := &cobra.Command{
		Use:   "evolve <name>",
		Short: "Roda o GEPA e promove via PR (branch local) uma skill candidata",
		Long: `Promoção via PR da meta-loop de skills (Fase 2 — GEPA).

Roda RunGEPA sobre o corpo da skill, aplica os guardrails e, se tudo OK, cria
um branch evolve/<skill>-<timestamp> com a skill evoluída + um commit local.
O comando JAMAIS faz push, merge ou auto-deploy: o branch fica como candidato
a PR para revisão humana.

Sem --no-llm, o caminho usa o mutator/juiz LLM (que ainda não está ligado e
degrada: nenhuma mutação é adotada). Use --no-llm para o caminho determinístico
(StaticMutator + StaticScorer), que é o que roda offline e nos testes.`,
		Example: `  cosca skill evolve my-skill --no-llm
  cosca skill evolve my-skill --no-llm --dry-run
  cosca skill evolve my-skill --no-llm --iterations 5 --max-growth 10
  cosca skill evolve my-skill --no-llm --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSkillEvolve(cmd, args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.noLLM, "no-llm", false, "caminho determinístico: StaticMutator + StaticScorer (sem LLM)")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "só mostra o diff do que seria produzido (não cria branch/commit)")
	cmd.Flags().IntVar(&opts.iterations, "iterations", defaultEvolveIterations, "número de iterações do GEPA")
	cmd.Flags().IntVar(&opts.trials, "trials", skilleval.MinTrials, "trials por braço no A/B")
	cmd.Flags().IntVar(&opts.maxLines, "max-lines", skilleval.DefaultMaxSkillLines, "teto de linhas da SKILL.md")
	cmd.Flags().Float64Var(&opts.maxGrowth, "max-growth", skilleval.DefaultMaxGrowthPct, "teto de crescimento % vs baseline")
	return cmd
}

// runSkillEvolve orchestrates the promotion-via-PR flow for a single skill.
func runSkillEvolve(cmd *cobra.Command, name string, opts skillEvolveOptions) error {
	formatter := GetFormatter(cmd)
	useJSON := IsJSONOutput(cmd)
	ctx := cmd.Context()

	if opts.iterations <= 0 {
		opts.iterations = 3
	}
	if opts.trials <= 0 {
		opts.trials = skilleval.MinTrials
	}
	if opts.now == nil {
		opts.now = func() time.Time { return time.Now().UTC() }
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}
	coscaDir := filepath.Join(cwd, ".cosca")

	mgr := skills.NewManager(coscaDir)
	skill, err := mgr.Get(name)
	if err != nil {
		return fmt.Errorf("skill %q não encontrada: %w", name, err)
	}

	mdPath, err := resolveSkillMarkdownPath(coscaDir, skill)
	if err != nil {
		return err
	}

	base, err := skilleval.ParseSkill(mdPath)
	if err != nil {
		return fmt.Errorf("parse skill %q: %w", name, err)
	}
	if base.Identity.Name == "" {
		base.Identity.Name = name
	}

	// Evaluation configuration: reuse a `.eval.yaml` when present for its cases
	// and gate, otherwise fall back to the deterministic default case set.
	cases, gate := resolveEvolveEvaluation(coscaDir, name)

	scorer, mutator := resolveEvolveEngine(opts.noLLM)

	evaluate := func(ctx context.Context, body string) (*skilleval.SkillBenchmark, error) {
		return skilleval.RunAB(ctx, scorer, staticRunner(body), staticRunner(base.Body), cases, opts.trials, gate)
	}

	// Baseline score: grade the baseline body once so GEPA knows the bar to beat.
	var baselineScore float64
	if bench, berr := evaluate(ctx, base.Body); berr == nil && bench != nil {
		baselineScore = bench.Without.MedianScore
	}

	result, gerr := skilleval.RunGEPA(ctx, base, mutator, baselineScore, evaluate, opts.iterations)
	if gerr != nil && gerr != context.Canceled {
		return fmt.Errorf("run GEPA for %q: %w", name, gerr)
	}
	best := result.Best
	if best == nil {
		best = base
	}

	if useJSON {
		return printJSON(cmd, map[string]interface{}{
			"skill":      name,
			"iterations": result.Iterations,
			"score":      result.Score,
			"candidate":  result.Candidate,
			"body":       best.Body,
			"dry_run":    opts.dryRun,
		})
	}

	timestamp := opts.now().Format("20060102T150405Z")
	sessionName := fmt.Sprintf("evolve/%s-%s", sanitizeBranchName(name), timestamp)

	// Apply the four guardrails to the GEPA best (the variant we would promote).
	sizeReport := skilleval.CheckSizeLimit(best, opts.maxLines, opts.maxGrowth, base)
	similarity, semOK := skilleval.CheckSemanticPreservation(base.Body, best.Body)
	structReport := skilleval.CheckStructural(best)
	cacheReport := skilleval.CheckCaching(sessionName)

	guardrailsOK := sizeReport.OK && semOK && structReport.OK && cacheReport.OK

	if opts.dryRun {
		printEvolveDryRun(formatter, name, result, best, base,
			sizeReport, similarity, semOK, structReport, cacheReport)
		return nil
	}

	if !guardrailsOK {
		var violations []string
		violations = append(violations, sizeReport.Violations...)
		violations = append(violations, structReport.Violations...)
		violations = append(violations, cacheReport.Violations...)
		if !semOK {
			violations = append(violations, fmt.Sprintf("semantic preservation below %.2f (similarity %.3f)", skilleval.DefaultMinSimilarity, similarity))
		}
		if len(violations) == 0 {
			violations = append(violations, "guardrails blocked the promotion")
		}
		formatter.Error(fmt.Sprintf("skill evolve bloqueado por guardrails (%s)", strings.Join(violations, "; ")))
		return fmt.Errorf("skill evolve %q: guardrails failed: %s", name, strings.Join(violations, "; "))
	}

	if best.Body == base.Body {
		formatter.Warning("evolve: GEPA não adotou nenhuma mudança (sem candidato). Nenhum branch/PR criado.")
		if !opts.noLLM {
			formatter.Warning("O caminho LLM ainda não está ligado — use --no-llm para o caminho determinístico.")
		}
		return nil
	}

	// Promoção via PR: nunca auto-deploy. Só cria o branch e o commit local.
	newContent := best.Apply()
	if err := promoteViaPR(cwd, mdPath, newContent, sessionName, name, result); err != nil {
		return err
	}

	formatter.Success(fmt.Sprintf("skill %s evoluída — candidato a PR no branch %s.", name, sessionName))
	formatter.KeyValue("Branch", sessionName)
	formatter.KeyValue("Iterations", fmt.Sprintf("%d", result.Iterations))
	formatter.KeyValue("Score", fmt.Sprintf("%.4f", result.Score))
	formatter.KeyValue("Similarity", fmt.Sprintf("%.3f", similarity))
	formatter.Warning("JAMAIS auto-deploy: revise o diff e a PR antes de merge/push.")
	return nil
}

// resolveEvolveEngine picks the scorer + mutator. With --no-llm it is the
// deterministic static pair; otherwise the LLM pair (which today degrades to a
// no-op, since the provider is not wired — see MutatorLLM / LLMJudgeScorer).
func resolveEvolveEngine(noLLM bool) (scorer skilleval.Scorer, mutator skilleval.Mutator) {
	if noLLM {
		scorer = skilleval.StaticScorer{}
		mutator = skilleval.MutatorStatic(evolveStaticMutate)
		return scorer, mutator
	}
	scorer = skilleval.LLMJudgeScorer{}
	mutator = skilleval.MutatorLLM(nil)
	return scorer, mutator
}

// evolveStaticMutate is the deterministic --no-llm mutation rule: it appends a
// marker line recording the number of A/B feedback points. The token "evolved"
// is what the default rubric rewards, so the candidate arm beats the baseline
// arm and GEPA adopts the variant.
func evolveStaticMutate(body string, feedback []string) string {
	return strings.TrimRight(body, "\n") + "\n<!-- evolved: " +
		fmt.Sprintf("%d", len(feedback)) + " feedback points -->\n"
}

// resolveEvolveEvaluation returns the SkillCase set and the gate function used
// by the GEPA evaluator. It prefers an existing `.eval.yaml` for the skill
// (reusing its cases and gate), otherwise it returns the deterministic default
// case set (rubric = [evolveDefaultRubricToken]) with a nil gate.
func resolveEvolveEvaluation(coscaDir, name string) ([]skilleval.SkillCase, func(context.Context) *skilleval.GateResult) {
	evalDir := filepath.Join(coscaDir, "evals", skilleval.EvalDirName)
	def, err := loadSkillEval(evalDir, name)
	if err != nil {
		return defaultEvolveCases(), nil
	}
	return def.skillCases(), def.gate()
}

// defaultEvolveCases is the fallback evaluation used when the skill has no
// `.eval.yaml`: a single case whose rubric rewards the marker token that the
// deterministic static mutator always introduces.
func defaultEvolveCases() []skilleval.SkillCase {
	return []skilleval.SkillCase{{
		ID:     "evolve",
		Task:   "a skill evoluída preserva o sinal de evolução",
		Rubric: []string{evolveDefaultRubricToken},
	}}
}

// resolveSkillMarkdownPath resolves the on-disk SKILL.md (or legacy .md) for a
// locally installed skill. Embedded framework skills are refused: they live in
// the compiled binary and are read-only.
func resolveSkillMarkdownPath(coscaDir string, skill *skills.Skill) (string, error) {
	if skill == nil {
		return "", fmt.Errorf("skill is nil")
	}
	if skill.Embedded {
		return "", fmt.Errorf("skill %q é embutida no framework (read-only); evolua uma skill instalada localmente", skill.Name)
	}
	skillsRoot := filepath.Join(coscaDir, "skills")
	if skill.Standard {
		if skill.Dir == "" {
			return "", fmt.Errorf("standard skill %q has no Dir", skill.Name)
		}
		return filepath.Join(skillsRoot, filepath.FromSlash(skill.Dir), "SKILL.md"), nil
	}
	// Legacy layout: <name>.md under the skills dir (or its subdir).
	return filepath.Join(skillsRoot, filepath.FromSlash(skill.Dir), skill.Name+".md"), nil
}

// promoteViaPR creates the evolve branch, writes the evolved SKILL.md, and
// makes a local commit. It NEVER pushes, merges, or auto-deploys: the branch is
// left as a human-reviewable PR candidate. relPath is committed by path so any
// unrelated pre-existing index changes are not swept into the commit.
func promoteViaPR(repoDir, mdPath, newContent, branch, name string, result *skilleval.GEPAResult) error {
	if _, err := runGitCmd(repoDir, "rev-parse", "--git-dir"); err != nil {
		return fmt.Errorf("promote via PR: %w", err)
	}
	if _, err := runGitCmd(repoDir, "checkout", "-b", branch); err != nil {
		return fmt.Errorf("create branch %q: %w", branch, err)
	}
	if err := os.WriteFile(mdPath, []byte(newContent), 0o644); err != nil {
		return fmt.Errorf("write evolved skill %q: %w", mdPath, err)
	}
	rel, err := filepath.Rel(repoDir, mdPath)
	if err != nil {
		return fmt.Errorf("resolve git path: %w", err)
	}
	rel = filepath.ToSlash(rel)
	if _, err := runGitCmd(repoDir, "add", rel); err != nil {
		return fmt.Errorf("stage %q: %w", rel, err)
	}
	msg := fmt.Sprintf("evolve(%s): skill candidate via GEPA (iterations=%d, score=%.4f)", name, result.Iterations, result.Score)
	if _, err := runGitCmd(repoDir, "commit", "-m", msg, "--", rel); err != nil {
		return fmt.Errorf("commit %q: %w", rel, err)
	}
	return nil
}

// printEvolveDryRun renders the dry-run preview: GEPA summary, the four
// guardrail verdicts, and the diff (via go-diff) of what the mutation produces.
// Nothing is written to disk and no branch/commit is created.
func printEvolveDryRun(formatter *OutputFormatter, name string, result *skilleval.GEPAResult,
	best, base *skilleval.Genome, sizeReport skilleval.GuardrailReport,
	similarity float64, semOK bool, structReport, cacheReport skilleval.GuardrailReport) {

	formatter.Header(fmt.Sprintf("skill evolve (dry-run) — %s", name))
	formatter.KeyValue("Iterations", fmt.Sprintf("%d", result.Iterations))
	formatter.KeyValue("Score", fmt.Sprintf("%.4f", result.Score))
	formatter.KeyValue("Candidate", yesNo(result.Candidate))
	formatter.KeyValue("Size guardrail", yesNo(sizeReport.OK))
	formatter.KeyValue("Semantic guardrail", fmt.Sprintf("%s (%.3f)", yesNo(semOK), similarity))
	formatter.KeyValue("Structural guardrail", yesNo(structReport.OK))
	formatter.KeyValue("Cache guardrail", yesNo(cacheReport.OK))

	if best.Body != base.Body {
		formatter.Println("")
		formatter.Header("Diff (proposta)")
		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(base.Source, best.Apply(), true)
		formatter.Println(strings.TrimSpace(dmp.DiffPrettyText(diffs)))
	} else {
		formatter.Warning("Nenhuma mudança de corpo foi adotada; sem diff.")
	}
	formatter.Warning("dry-run: nenhum branch/commit criado; nada foi alterado no disco.")
}

// sanitizeBranchName ensures a skill name can be embedded in a git branch name
// (it only allows [a-zA-Z0-9_-], replacing everything else with '-').
func sanitizeBranchName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return b.String()
}
