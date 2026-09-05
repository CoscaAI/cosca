//
// `cosca skill benchmark | history | eval list` — A/B da meta-loop de skills.
//
// FATIA 3 (bounded) da auto-evolução ADR-8101. Estes subcomandos dão o lado
// de execução/inspeção do harness de avaliação de skills:
//
//   skill benchmark <name>   Roda o A/B (RunAB) de uma .eval.yaml e persiste
//                            o resultado em .cosca/evals/skills/<name>.benchmark.json
//                            + append imutável em <name>.history.json.
//   skill history <name>     Imprime o histórico append-only da skill.
//   skill eval list          Lista as .eval.yaml definidas.
//
// A Runner usada aqui é uma FIXTURE estática (staticRunner) que devolve o
// corpo da skill como output — NÃO depende de LLM. Isso permite que o comando
// rode offline/em teste sem um provedor de chat. A Runner real (que invoca o
// agente/skill) e o gate real (auditoria de catálogo + regressão) entram em
// incrementos posteriores da meta-loop; aqui ficam plugáveis e documentados.
//
// NOTE: os comandos operam no workspace real (os.Getwd()), então rodam FORA da
// jaula (ver cmd/cosca/main.go isAdminCommand/ integrityGateBypass).

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/CoscaAI/cosca/internal/skilleval"
)

// ─── skill eval (definition) ───────────────────────────────────────────────

// skillEvalDef is the on-disk shape of a `<name>.eval.yaml` under
// .cosca/evals/skills/. It is the input contract for `skill benchmark`.
type skillEvalDef struct {
	Name        string          `yaml:"name"`
	Description string          `yaml:"description"`
	Version     string          `yaml:"version"`
	With        string          `yaml:"with"`    // corpo candidato (o que evolui)
	Without     string          `yaml:"without"` // corpo baseline (opcional; "" = degradado)
	Scorer      string          `yaml:"scorer"`  // "static" (default) | "llm"
	Cases       []skillEvalCase `yaml:"cases"`
	Gate        *skillEvalGate  `yaml:"gate"`
}

// skillEvalCase is one evaluation case in a .eval.yaml.
type skillEvalCase struct {
	ID     string   `yaml:"id"`
	Task   string   `yaml:"task"`
	Rubric []string `yaml:"rubric"`
	Weight float64  `yaml:"weight"`
}

// skillEvalGate is the (deterministic, LLM-free) regression gate config. The
// real gate — auditoria de catálogo + regressão — entra em incremento
// posterior; aqui ele apenas reflete a config do YAML como um veredito estático.
type skillEvalGate struct {
	CatalogAudit bool     `yaml:"catalog_audit"`
	RegTests     bool     `yaml:"reg_tests"`
	RegDelta     bool     `yaml:"reg_delta"`
	Details      []string `yaml:"details"`
}

// skillCases converts the YAML cases into the harness SkillCase list.
func (d *skillEvalDef) skillCases() []skilleval.SkillCase {
	out := make([]skilleval.SkillCase, 0, len(d.Cases))
	for _, c := range d.Cases {
		out = append(out, skilleval.SkillCase{
			ID: c.ID, Task: c.Task, Rubric: c.Rubric, Weight: c.Weight,
		})
	}
	return out
}

// scorer resolves the grading backend. For "llm" it returns the LLM judge
// contract (which today degrades a grading to the neutral score — no model
// invocation), otherwise the deterministic StaticScorer.
func (d *skillEvalDef) scorer() (skilleval.Scorer, error) {
	switch strings.ToLower(d.Scorer) {
	case "llm":
		return skilleval.LLMJudgeScorer{}, nil
	default:
		return skilleval.StaticScorer{}, nil
	}
}

// scorerLabel is the human-readable scorer description for the dry-run outline.
func (d *skillEvalDef) scorerLabel() string {
	switch strings.ToLower(d.Scorer) {
	case "llm":
		return "llm (fixture: neutral-seed, sem invocar modelo)"
	default:
		return "static (fixture determinística, sem LLM)"
	}
}

// gate builds the regression gate function. It returns a pass verdict carrying
// the YAML config as details. When the def has no gate block, the gate passes
// by default (it never blocks) — a later increment wires the real audit.
func (d *skillEvalDef) gate() func(context.Context) *skilleval.GateResult {
	g := &skilleval.GateResult{Passed: true, CatalogAudit: true, RegTests: true, RegDelta: true}
	if d.Gate != nil {
		g.CatalogAudit = d.Gate.CatalogAudit
		g.RegTests = d.Gate.RegTests
		g.RegDelta = d.Gate.RegDelta
		g.Details = d.Gate.Details
		g.Passed = d.Gate.CatalogAudit && d.Gate.RegTests && d.Gate.RegDelta
	}
	return func(context.Context) *skilleval.GateResult { return g }
}

// staticRunner is the documented StaticRunner FIXTURE: it executes the skill
// body by returning it verbatim as the produced output. It never touches an
// LLM, so `skill benchmark` works offline and in tests. A real agent runner
// (invoking the skill/agent) lands in a later increment and plugs in here.
func staticRunner(body string) skilleval.Runner {
	return func(context.Context) (string, error) { return body, nil }
}

// evalSkillsDir resolves `.cosca/evals/skills/` below the working directory.
func evalSkillsDirFromCWD() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	return filepath.Join(dir, ".cosca", "evals", skilleval.EvalDirName), nil
}

// loadSkillEval reads and parses <dir>/<name>.eval.yaml.
func loadSkillEval(dir, name string) (*skillEvalDef, error) {
	path := filepath.Join(dir, name+".eval.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("skill eval %q não encontrado em %s (esperava %s): %w", name, dir, path, err)
	}
	var def skillEvalDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parse skill eval %q: %w", name, err)
	}
	if def.Name == "" {
		def.Name = name
	}
	return &def, nil
}

// listSkillEvals returns the sorted names of the `.eval.yaml` files in dir.
func listSkillEvals(dir string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.eval.yaml"))
	if err != nil {
		return nil, fmt.Errorf("list skill evals: %w", err)
	}
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		base := filepath.Base(m)
		names = append(names, strings.TrimSuffix(base, ".eval.yaml"))
	}
	sort.Strings(names)
	return names, nil
}

// ─── skill command registration (added to the `cosca skill` tree) ──────────

// NewSkillEvalCommand creates the `cosca skill eval` group.
func NewSkillEvalCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "eval",
		Short: "Inspeciona as definições de avaliação de skill (.eval.yaml)",
		Long: `Inspeciona as definições de avaliação de skill (meta-loop A/B).
As definições vivem em .cosca/evals/skills/<name>.eval.yaml e descrevem o
corpo candidato (with), o baseline (without) e os casos com rubrica.

Subcomandos:
  list    Lista as .eval.yaml definidas em .cosca/evals/skills/.`,
		Example: `  cosca skill eval list`,
		Args:    cobra.NoArgs,
	}
	cmd.AddCommand(NewSkillEvalListCommand())
	return cmd
}

// NewSkillEvalListCommand creates `cosca skill eval list`.
func NewSkillEvalListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Lista as .eval.yaml definidas em .cosca/evals/skills/",
		Long:  `Lista os nomes das avaliações de skill (.eval.yaml) definidas no projeto.`,
		Example: `  cosca skill eval list
  cosca skill eval list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			skillsDir, err := evalSkillsDirFromCWD()
			if err != nil {
				return err
			}
			names, err := listSkillEvals(skillsDir)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, names)
			}
			if len(names) == 0 {
				formatter.Warning(fmt.Sprintf("Nenhuma .eval.yaml em %s — crie uma ou rode \"cosca skill benchmark <name>\" com o arquivo.", skillsDir))
				return nil
			}

			formatter.Header(fmt.Sprintf("Skill evals — %d definida(s)", len(names)))
			rows := make([][]string, 0, len(names))
			for _, n := range names {
				rows = append(rows, []string{n})
			}
			formatter.Table([]string{"Name"}, rows)
			return nil
		},
	}
}

// NewSkillBenchmarkCommand creates `cosca skill benchmark <name>`.
func NewSkillBenchmarkCommand() *cobra.Command {
	var (
		trials int
		dryRun bool
		all    bool
		noGate bool
	)

	cmd := &cobra.Command{
		Use:   "benchmark <name>",
		Short: "Roda o A/B (RunAB) de uma skill e persiste benchmark + histórico",
		Long: `Roda o A/B de uma skill a partir da sua .eval.yaml e persiste o resultado.

A avaliação lê .cosca/evals/skills/<name>.eval.yaml, executa o braço candidato
(with) contra o baseline (without) via RunAB e grava:
  <name>.benchmark.json — snapshot do resultado (0600)
  <name>.history.json  — histórico append-only (0600)

A Runner de execução é uma FIXTURE estática (staticRunner) que devolve o corpo
da skill como output — NÃO depende de LLM, então roda offline e em testes. A
Runner real (invocando o agente/skill) e o gate real (catálogo + regressão)
entram em incrementos posteriores.

Flags:
  --trials <N>    número de trials por braço (default 5, mínimo de evidência)
  --dry-run       estima a corrida sem rodar (sem grading, sem persistir)
  --all           roda TODAS as .eval.yaml definidas em .cosca/evals/skills/
  --no-gate       não exige o gate de regressão (gate = nil)`,
		Example: `  cosca skill benchmark my-skill
  cosca skill benchmark my-skill --trials 10
  cosca skill benchmark my-skill --dry-run
  cosca skill benchmark --all
  cosca skill benchmark my-skill --no-gate`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			return runSkillBenchmark(cmd, name, trials, dryRun, all, noGate)
		},
	}

	cmd.Flags().IntVar(&trials, "trials", skilleval.MinTrials, "número de trials por braço (default é MinTrials)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "estima a corrida sem rodar (sem grading/persistência)")
	cmd.Flags().BoolVar(&all, "all", false, "roda todas as evals definidas em .cosca/evals/skills/")
	cmd.Flags().BoolVar(&noGate, "no-gate", false, "não exige o gate de regressão (gate = nil)")
	return cmd
}

// NewSkillHistoryCommand creates `cosca skill history <name>`.
func NewSkillHistoryCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "history <name>",
		Short: "Imprime o histórico append-only de benchmarks de uma skill",
		Long: `Imprime o histórico imutável de benchmarks da skill, lido de
.cosca/evals/skills/<name>.history.json. Cada linha é uma corrida: versão,
parent (versão da qual foi derivada), data, taxa de aprovação e a marcação de
melhor corrida ("✓"). Somente leitura.`,
		Example: `  cosca skill history my-skill
  cosca skill history my-skill --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getwd: %w", err)
			}
			entries, err := skilleval.LoadHistory(dir, args[0])
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, entries)
			}
			if len(entries) == 0 {
				formatter.Warning(fmt.Sprintf("Sem histórico para a skill %q — rode \"cosca skill benchmark %s\" primeiro.", args[0], args[0]))
				return nil
			}

			formatter.Header(fmt.Sprintf("Skill %s — histórico (%d registro(s))", args[0], len(entries)))
			rows := make([][]string, 0, len(entries))
			for _, e := range entries {
				best := ""
				if e.IsCurrentBest {
					best = "✓"
				}
				parent := e.Parent
				if parent == "" {
					parent = "—"
				}
				rows = append(rows, []string{e.Version, parent, e.Date, fmt.Sprintf("%.3f", e.PassRate), e.Note, best})
			}
			formatter.Table([]string{"Version", "Parent", "Date", "PassRate", "Note", "Best"}, rows)
			return nil
		},
	}
}

// ─── benchmark runner ───────────────────────────────────────────────────────

// runSkillBenchmark orchestrates a single or multi benchmark run.
func runSkillBenchmark(cmd *cobra.Command, name string, trials int, dryRun, all, noGate bool) error {
	formatter := GetFormatter(cmd)
	useJSON := IsJSONOutput(cmd)

	skillsDir, err := evalSkillsDirFromCWD()
	if err != nil {
		return err
	}

	if all {
		names, err := listSkillEvals(skillsDir)
		if err != nil {
			return err
		}
		if len(names) == 0 {
			return fmt.Errorf("nenhuma .eval.yaml em %s", skillsDir)
		}
		defs := make([]*skillEvalDef, 0, len(names))
		for _, n := range names {
			def, err := loadSkillEval(skillsDir, n)
			if err != nil {
				return err
			}
			defs = append(defs, def)
		}

		if dryRun {
			for _, def := range defs {
				printSkillEvalEstimate(formatter, def, trials)
			}
			return nil
		}

		results := make([]*skilleval.SkillBenchmark, 0, len(defs))
		for _, def := range defs {
			bench, err := runOneSkillBenchmark(cmd, def, trials, noGate)
			if err != nil {
				return err
			}
			if err := persistBenchmark(def, bench); err != nil {
				return err
			}
			results = append(results, bench)
		}

		if useJSON {
			return printJSON(cmd, results)
		}
		for _, b := range results {
			printBenchmark(formatter, b)
		}
		return nil
	}

	if name == "" {
		return fmt.Errorf("skill benchmark exige um nome (ou use --all)")
	}

	def, err := loadSkillEval(skillsDir, name)
	if err != nil {
		return err
	}

	if dryRun {
		printSkillEvalEstimate(formatter, def, trials)
		return nil
	}

	bench, err := runOneSkillBenchmark(cmd, def, trials, noGate)
	if err != nil {
		return err
	}
	if err := persistBenchmark(def, bench); err != nil {
		return err
	}

	if useJSON {
		return printJSON(cmd, bench)
	}
	printBenchmark(formatter, bench)
	return nil
}

// runOneSkillBenchmark executes a single A/B for a def (no persistence).
func runOneSkillBenchmark(cmd *cobra.Command, def *skillEvalDef, trials int, noGate bool) (*skilleval.SkillBenchmark, error) {
	scorer, err := def.scorer()
	if err != nil {
		return nil, err
	}
	cases := def.skillCases()
	with := staticRunner(def.With)
	without := staticRunner(def.Without)

	var gate func(context.Context) *skilleval.GateResult
	if !noGate {
		gate = def.gate()
	}

	bench, err := skilleval.RunAB(cmd.Context(), scorer, with, without, cases, trials, gate)
	if err != nil {
		return nil, fmt.Errorf("run A/B for %q: %w", def.Name, err)
	}
	bench.Skill = def.Name
	bench.Date = time.Now().UTC().Format(time.RFC3339)
	return bench, nil
}

// persistBenchmark saves the snapshot and appends to the immutable history.
func persistBenchmark(def *skillEvalDef, bench *skilleval.SkillBenchmark) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}

	if err := skilleval.SaveBenchmark(root, bench); err != nil {
		return err
	}

	isBest := true
	if hist, herr := skilleval.LoadHistory(root, def.Name); herr == nil && len(hist) > 0 {
		var best float64
		for _, h := range hist {
			if h.PassRate > best {
				best = h.PassRate
			}
		}
		isBest = bench.With.MedianScore > best
	}

	version := def.Version
	if version == "" {
		version = "dev"
	}
	note := fmt.Sprintf("benchmark: candidate=%v delta=%.4f (arm with).", bench.IsCandidate, bench.Delta.Score)
	if _, err := skilleval.AppendHistory(root, def.Name, version, bench.With.MedianScore, note, isBest); err != nil {
		return err
	}
	return nil
}

// printSkillEvalEstimate renders the dry-run outline of a def.
func printSkillEvalEstimate(formatter *OutputFormatter, def *skillEvalDef, trials int) {
	formatter.Header(fmt.Sprintf("Skill eval (dry-run) — %s", def.Name))
	formatter.KeyValue("Description", def.Description)
	formatter.KeyValue("Version", def.Version)
	formatter.KeyValue("Cases", fmt.Sprintf("%d", len(def.Cases)))
	formatter.KeyValue("Trials/arm", fmt.Sprintf("%d", trials))
	formatter.KeyValue("Scorer", def.scorerLabel())
	formatter.KeyValue("With body", fmt.Sprintf("%d char(s)", len(def.With)))
	formatter.KeyValue("Without body", fmt.Sprintf("%d char(s)", len(def.Without)))
	formatter.Warning("dry-run: nenhum benchmark foi de fato executado; nada foi persistido.")
}

// printBenchmark renders a SkillBenchmark as key/values + a summary row.
func printBenchmark(formatter *OutputFormatter, b *skilleval.SkillBenchmark) {
	formatter.Header(fmt.Sprintf("Skill benchmark — %s", b.Skill))
	formatter.KeyValue("Date", b.Date)
	formatter.KeyValue("Candidate", yesNo(b.IsCandidate))
	formatter.KeyValue("Delta score", fmt.Sprintf("%.4f", b.Delta.Score))
	formatter.KeyValue("Delta std", fmt.Sprintf("%.4f", b.Delta.ScoreStd))
	formatter.KeyValue("With (median/IQR)", fmt.Sprintf("%.3f [%.3f-%.3f]", b.With.MedianScore, b.With.IQRS[0], b.With.IQRS[1]))
	formatter.KeyValue("Without (median/IQR)", fmt.Sprintf("%.3f [%.3f-%.3f]", b.Without.MedianScore, b.Without.IQRS[0], b.Without.IQRS[1]))
	formatter.KeyValue("With solve rate", fmt.Sprintf("%.3f", b.With.SolveRate))
	formatter.KeyValue("Without solve rate", fmt.Sprintf("%.3f", b.Without.SolveRate))
	if b.Gate != nil {
		formatter.KeyValue("Gate", fmt.Sprintf("passed=%v (catalog=%v reg_tests=%v reg_delta=%v)", b.Gate.Passed, b.Gate.CatalogAudit, b.Gate.RegTests, b.Gate.RegDelta))
	} else {
		formatter.KeyValue("Gate", "não exigido (--no-gate)")
	}
}
