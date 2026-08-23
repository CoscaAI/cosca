package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/skills"
)

// NewSkillCommand creates the `cosca skill` command.
func NewSkillCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage Cosca skills",
		Long: `Manage Cosca skills.

Skills define specialized capabilities that agents can use to perform
specific tasks. They encapsulate domain knowledge, instructions, and tools
for particular domains.
`,
		Example: `  cosca skill list
  cosca skill show my-skill
  cosca skill search "database"
  cosca skill install my-skill --source ./skill.yaml
  cosca skill use my-skill
  cosca skill status
  cosca skill validate
  cosca skill migrate --dry-run
  cosca skill curator --dry-run --days 30
  cosca skill curator --archive --days 30
  cosca skill restore my-skill`,
	}

	cmd.AddCommand(
		NewSkillListCommand(),
		NewSkillShowCommand(),
		NewSkillSearchCommand(),
		NewSkillInstallCommand(),
		NewSkillUseCommand(),
		NewSkillStatusCommand(),
		NewSkillCuratorCommand(),
		NewSkillRestoreCommand(),
		NewSkillValidateCommand(),
		NewSkillMigrateCommand(),
		NewSkillBenchmarkCommand(),
		NewSkillHistoryCommand(),
		NewSkillEvalCommand(),
	)

	return cmd
}

// NewSkillListCommand creates the `cosca skill list` subcommand.
func NewSkillListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available skills",
		Long:  `List all available skills in the Cosca system.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := skills.NewManager(coscaDir)
			if mgr == nil {
				return fmt.Errorf("skill manager not available")
			}

			skillList := mgr.List()

			if useJSON {
				return printJSON(cmd, skillList)
			}

			if len(skillList) == 0 {
				formatter.Warning("Nenhuma skill encontrada. Use `cosca skill install <caminho>` para instalar ou `cosca sync` para sincronizar.")
				return nil
			}

			formatter.Header(fmt.Sprintf("Available Skills (%d)", len(skillList)))

			headers := []string{"Name", "Description", "Version"}
			rows := make([][]string, 0, len(skillList))

			for _, s := range skillList {
				rows = append(rows, []string{s.Name, s.Description, s.Version})
			}

			formatter.Table(headers, rows)
			return nil
		},
	}

	return cmd
}

// NewSkillShowCommand creates the `cosca skill show` subcommand.
func NewSkillShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show skill details",
		Long:  `Display detailed information about a specific skill.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := skills.NewManager(coscaDir)
			if mgr == nil {
				return fmt.Errorf("skill manager not available")
			}

			skill, err := mgr.Get(args[0])
			if err != nil {
				return fmt.Errorf("skill %q not found: %w", args[0], err)
			}

			if useJSON {
				return printJSON(cmd, skill)
			}

			formatter.Header(fmt.Sprintf("Skill: %s", skill.Name))
			formatter.KeyValue("Name", skill.Name)
			formatter.KeyValue("Description", skill.Description)
			formatter.KeyValue("Version", skill.Version)
			formatter.KeyValue("Category", skill.Category)
			if skill.License != "" {
				formatter.KeyValue("License", skill.License)
			}
			if skill.Compatibility != "" {
				formatter.KeyValue("Compatibility", skill.Compatibility)
			}
			if skill.AllowedTools != "" {
				formatter.KeyValue("Allowed Tools", skill.AllowedTools)
			}
			if skill.Standard {
				formatter.KeyValue("Format", "standard (skill-name/SKILL.md)")
			} else {
				formatter.KeyValue("Format", "legacy")
			}
			if skill.Dir != "" {
				formatter.KeyValue("Directory", skill.Dir)
			}

			if len(skill.Metadata) > 0 {
				formatter.Println("")
				formatter.Header("Metadata")
				keys := make([]string, 0, len(skill.Metadata))
				for k := range skill.Metadata {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					formatter.KeyValue(k, skill.Metadata[k])
				}
			}

			if len(skill.Resources) > 0 {
				formatter.Println("")
				formatter.Header("Resources")
				for _, r := range skill.Resources {
					formatter.Bullet(fmt.Sprintf("%s (%s)", r.Path, r.Kind))
				}
			}

			if skill.Instructions != "" {
				formatter.Println("")
				formatter.Header("Instructions")
				formatter.Println(skill.Instructions)
			}

			if len(skill.Tools) > 0 {
				formatter.Println("")
				formatter.Header("Tools")
				for _, t := range skill.Tools {
					formatter.Bullet(fmt.Sprintf("%s (%s)", t.Name, t.Description))
				}
			}

			return nil
		},
	}

	return cmd
}

// NewSkillSearchCommand creates the `cosca skill search` subcommand.
func NewSkillSearchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search skills",
		Long:  `Search for skills by name, description, or category.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := skills.NewManager(coscaDir)
			if mgr == nil {
				return fmt.Errorf("skill manager not available")
			}

			results, err := mgr.Search(args[0])
			if err != nil {
				return fmt.Errorf("skill search failed: %w", err)
			}

			if useJSON {
				return printJSON(cmd, results)
			}

			if len(results) == 0 {
				formatter.Warning(fmt.Sprintf("No skills found matching %q", args[0]))
				return nil
			}

			formatter.Header(fmt.Sprintf("Skill Search Results for %q", args[0]))
			for _, r := range results {
				formatter.KeyValue(r.Name, r.Description)
			}
			formatter.KeyValue("Total", fmt.Sprintf("%d", len(results)))

			return nil
		},
	}

	return cmd
}

// NewSkillInstallCommand creates the `cosca skill install` subcommand.
func NewSkillInstallCommand() *cobra.Command {
	var source string

	cmd := &cobra.Command{
		Use:   "install <name>",
		Short: "Install a skill",
		Long:  `Install a skill from a file or registry source.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			spinner := formatter.Spinner(fmt.Sprintf("Installing skill %s", args[0]))
			spinner.Start()

			mgr := skills.NewManager(coscaDir)
			if mgr == nil {
				spinner.Fail("Skill manager not available")
				return fmt.Errorf("skill manager not available")
			}

			skill, err := mgr.Install(args[0], source)
			if err != nil {
				spinner.Fail(fmt.Sprintf("Install failed: %v", err))
				return fmt.Errorf("skill installation failed: %w", err)
			}

			spinner.Stop("Skill installed")

			if useJSON {
				return printJSON(cmd, skill)
			}

			formatter.Success(fmt.Sprintf("Skill %s installed", skill.Name))
			return nil
		},
	}

	cmd.Flags().StringVarP(&source, "source", "s", "", "source file or URL for the skill")
	return cmd
}

// ---------------------------------------------------------------------------
// Telemetria de uso + curador (usage.go)
// ---------------------------------------------------------------------------

// resolveUsageStore abre o sidecar de telemetria de skills do projeto atual
// (<projeto>/.cosca/skills/.usage.json).
func resolveUsageStore() (*skills.UsageStore, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getwd: %w", err)
	}
	return skills.NewUsageStore(filepath.Join(dir, ".cosca"))
}

// NewSkillUseCommand cria `cosca skill use <name>`.
func NewSkillUseCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Registra o uso de uma skill (incrementa use_count e atualiza last_activity)",
		Long: `Registra o uso de uma skill no sidecar de telemetria
(.cosca/skills/.usage.json): incrementa use_count, atualiza last_activity_at
e garante o estado active. É o dado que alimenta o curador (skill curator)
para podar com segurança — sem nunca deletar.`,
		Example: `  cosca skill use my-skill
  cosca skill use my-skill --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getwd: %w", err)
			}
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := skills.NewManager(coscaDir)
			skill, err := mgr.Get(args[0])
			if err != nil {
				return fmt.Errorf("skill %q não encontrada: %w", args[0], err)
			}

			store, err := resolveUsageStore()
			if err != nil {
				return err
			}
			if err := store.RecordUse(skill.Name); err != nil {
				return err
			}
			u, err := store.Get(skill.Name)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, u)
			}
			formatter.Success(fmt.Sprintf("Skill %s usada — use_count=%d", u.SkillName, u.UseCount))
			formatter.KeyValue("Skill", u.SkillName)
			formatter.KeyValue("Uses", fmt.Sprintf("%d", u.UseCount))
			formatter.KeyValue("Last activity", formatUsageTime(u.LastActivityAt))
			formatter.KeyValue("State", u.State)
			return nil
		},
	}
}

// skillStatusRow é a linha da tabela de `cosca skill status`.
type skillStatusRow struct {
	Skill        string `json:"skill"`
	Category     string `json:"category"`
	UseCount     int    `json:"use_count"`
	LastActivity string `json:"last_activity"`
	State        string `json:"state"`
}

// NewSkillStatusCommand cria `cosca skill status`.
func NewSkillStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Tabela de uso e estado das skills (ativo/stale/arquivado)",
		Long: `Tabela de telemetria de uso das skills: skill, categoria, use_count,
última atividade e estado (✓ ativo / ⚠ stale / 🗄 arquivado). Junta o catálogo
do Manager com o sidecar .cosca/skills/.usage.json. Somente leitura.`,
		Example: `  cosca skill status
  cosca skill status --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getwd: %w", err)
			}
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := skills.NewManager(coscaDir)
			store, err := resolveUsageStore()
			if err != nil {
				return err
			}
			snap, err := store.Snapshot()
			if err != nil {
				return err
			}

			var rows []skillStatusRow
			seen := make(map[string]bool, len(snap))

			names := make([]string, 0, len(snap))
			for name := range snap {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				u := snap[name]
				rows = append(rows, skillStatusRow{
					Skill:        u.SkillName,
					Category:     usageCategory(mgr, name),
					UseCount:     u.UseCount,
					LastActivity: formatUsageTime(u.LastActivityAt),
					State:        usageStateLabel(u.State),
				})
				seen[name] = true
			}
			for _, s := range mgr.List() {
				if seen[s.Name] {
					continue
				}
				rows = append(rows, skillStatusRow{
					Skill:        s.Name,
					Category:     s.Category,
					UseCount:     0,
					LastActivity: "nunca usada",
					State:        usageStateLabel(skills.StateActive),
				})
			}

			if useJSON {
				return printJSON(cmd, rows)
			}
			if len(rows) == 0 {
				formatter.Warning("Nenhuma skill — rode \"cosca skill list\" para ver o catálogo.")
				return nil
			}

			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{r.Skill, r.Category, fmt.Sprintf("%d", r.UseCount), r.LastActivity, r.State})
			}
			formatter.Header(fmt.Sprintf("Skills — %d skill(s)", len(rows)))
			formatter.Table([]string{"Skill", "Category", "Uses", "Last activity", "State"}, table)
			return nil
		},
	}
}

// NewSkillCuratorCommand cria `cosca skill curator [--dry-run|--archive] [--days N]`.
func NewSkillCuratorCommand() *cobra.Command {
	var (
		days    int
		dryRun  bool
		archive bool
	)

	cmd := &cobra.Command{
		Use:   "curator",
		Short: "Curador de skills: lista (--dry-run) ou arquiva (--archive) skills stale",
		Long: `Curador de skills (princípio do curador: NUNCA deleta, só arquiva).

O curador é EXPLÍCITO: nunca roda sozinho e nunca apaga nada. Ele lista as
skills active sem uso há <days> dias (padrão 30) e, com --archive, arquiva:
a fonte local é movida para .cosca/skills/.archive/<name>/ e a skill marcada
state=archived. Skills embutidas (sem fonte local) são apenas marcadas como
archived — nunca movidas.

  --dry-run   lista as skills que seriam arquivadas; NÃO altera nada.
  --archive   arquiva as skills stale; nunca deleta.`,
		Example: `  cosca skill curator --dry-run
  cosca skill curator --dry-run --days 60
  cosca skill curator --archive --days 30`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			if !dryRun && !archive {
				return fmt.Errorf("especifique --dry-run ou --archive (o curador nunca roda sozinho)")
			}

			store, err := resolveUsageStore()
			if err != nil {
				return err
			}
			stale, err := store.StaleSince(days)
			if err != nil {
				return err
			}

			if dryRun {
				if len(stale) == 0 {
					formatter.Success(fmt.Sprintf("Nenhuma skill stale há %d+ dias — nada a arquivar.", days))
					return nil
				}
				rows := make([][]string, 0, len(stale))
				for _, u := range stale {
					rows = append(rows, []string{u.SkillName, fmt.Sprintf("%d", u.UseCount), formatUsageTime(u.LastActivityAt)})
				}
				formatter.Header(fmt.Sprintf("Candidatos a arquivamento (sem uso há %d+ dias) — %d skill(s)", days, len(stale)))
				formatter.Table([]string{"Skill", "Uses", "Last activity"}, rows)
				formatter.Warning("dry-run: NADA foi alterado. Rode \"cosca skill curator --archive\" para arquivar.")
				return nil
			}

			// --archive
			if len(stale) == 0 {
				formatter.Success(fmt.Sprintf("Nenhuma skill stale há %d+ dias — nada a arquivar.", days))
				return nil
			}

			type archiveResult struct {
				Skill  string `json:"skill"`
				Action string `json:"action"`
				State  string `json:"state"`
				Notes  string `json:"notes,omitempty"`
			}
			var results []archiveResult
			var failed []string
			for _, u := range stale {
				if err := store.Archive(u.SkillName); err != nil {
					failed = append(failed, fmt.Sprintf("%s: %v", u.SkillName, err))
					continue
				}
				after, err := store.Get(u.SkillName)
				if err != nil {
					failed = append(failed, fmt.Sprintf("%s: %v", u.SkillName, err))
					continue
				}
				action := "movida para .archive/"
				if after.Notes != "" {
					action = "apenas marcada (embedded)"
				}
				results = append(results, archiveResult{
					Skill:  u.SkillName,
					Action: action,
					State:  after.State,
					Notes:  after.Notes,
				})
			}

			if useJSON {
				return printJSON(cmd, results)
			}
			formatter.Header(fmt.Sprintf("Curador — %d skill(s) arquivada(s)", len(results)))
			for _, r := range results {
				formatter.Success(fmt.Sprintf("%s → %s (%s)", r.Skill, r.State, r.Action))
			}
			for _, f := range failed {
				formatter.Error(f)
			}
			formatter.Warning("O curador NUNCA deleta: skills locais foram movidas para .cosca/skills/.archive/; skills embutidas apenas marcadas.")
			return nil
		},
	}

	cmd.Flags().IntVar(&days, "days", 30, "limite em dias sem uso para considerar stale")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "apenas lista o que seria arquivado (não altera nada)")
	cmd.Flags().BoolVar(&archive, "archive", false, "arquiva as skills stale (nunca deleta)")
	return cmd
}

// NewSkillRestoreCommand cria `cosca skill restore <name>`.
func NewSkillRestoreCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "restore <name>",
		Short: "Restaura uma skill arquivada (archived → active, move a fonte de volta)",
		Long: `Restaura uma skill arquivada: marca state=active e move a fonte
de volta de .cosca/skills/.archive/<name>/ para .cosca/skills/. Skills
embutidas (apenas marcadas) são reativadas sem movimento de arquivos.`,
		Example: `  cosca skill restore my-skill`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			store, err := resolveUsageStore()
			if err != nil {
				return err
			}
			if err := store.Restore(args[0]); err != nil {
				return err
			}
			u, err := store.Get(args[0])
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, u)
			}
			formatter.Success(fmt.Sprintf("Skill %s restaurada — state=%s", u.SkillName, u.State))
			return nil
		},
	}
}

// NewSkillValidateCommand creates `cosca skill validate`.
//
// Scans all loaded skills (embedded + local) and validates each against the
// Agent Skills standard naming/frontmatter rules. Legacy Cosca skills produce
// warnings, not errors, unless --strict is given.
func NewSkillValidateCommand() *cobra.Command {
	var strict bool

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate skills against the Agent Skills standard",
		Long: `Validate all loaded skills (embedded + local) against the Agent Skills
standard (agentskills.io/specification): naming rules, required frontmatter
fields, and size limits.

Legacy Cosca skills (flat .md files with descriptive names) commonly produce
warnings — these are reported but not fatal unless --strict is given, in which
case the command exits non-zero when any skill fails validation.`,
		Example: `  cosca skill validate
  cosca skill validate --strict
  cosca skill validate --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := skills.NewManager(coscaDir)
			results := mgr.ValidateAll()

			if useJSON {
				return printJSON(cmd, results)
			}

			valid, invalid := 0, 0
			rows := make([][]string, 0, len(results))
			for _, r := range results {
				format := "legacy"
				if r.Standard {
					format = "standard"
				}
				if len(r.Violations) > 0 {
					invalid++
					rows = append(rows, []string{r.Name, format, "✗", strings.Join(r.Violations, "; ")})
				} else {
					valid++
					rows = append(rows, []string{r.Name, format, "✓", ""})
				}
			}

			formatter.Header(fmt.Sprintf("Skill Validation (%d total)", len(results)))
			if len(rows) > 0 {
				formatter.Table([]string{"Name", "Format", "Status", "Violations"}, rows)
			}
			formatter.KeyValue("Conformant", fmt.Sprintf("%d", valid))
			formatter.KeyValue("With violations", fmt.Sprintf("%d", invalid))

			if invalid > 0 {
				formatter.Warning("Legacy Cosca skills are reported as warnings; run `cosca skill migrate` to generate spec-conformant frontmatter.")
			}
			if strict && invalid > 0 {
				return fmt.Errorf("%d skill(s) failed Agent Skills validation", invalid)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&strict, "strict", false, "exit non-zero if any skill fails validation")
	return cmd
}

// NewSkillMigrateCommand creates `cosca skill migrate`.
//
// Prints the frontmatter block that would make each legacy Cosca skill
// conformant with the Agent Skills standard. With --write, the frontmatter is
// prepended to local skill files (embedded framework skills stay read-only).
func NewSkillMigrateCommand() *cobra.Command {
	var (
		write  bool
		dryRun bool
	)

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Generate Agent Skills frontmatter for legacy Cosca skills",
		Long: `Migrate legacy Cosca skills to the Agent Skills standard.

The Agent Skills standard (agentskills.io/specification) requires every skill
to live in a skill-name/SKILL.md directory with YAML frontmatter declaring at
least name and description. Legacy Cosca skills are flat .md files with
descriptive names and no frontmatter — they still parse and work, but do not
validate as spec-conformant.

'migrate' prints the frontmatter block that would make each legacy skill
conformant. With --write it prepends that frontmatter to local files.
Embedded framework skills are always read-only (they live in the compiled
binary) — they are shown for reference but never written.`,
		Example: `  cosca skill migrate
  cosca skill migrate --dry-run
  cosca skill migrate --write`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := skills.NewManager(coscaDir)
			results, err := mgr.MigrateLegacy(write && !dryRun)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, results)
			}

			if len(results) == 0 {
				formatter.Success("No legacy skills to migrate.")
				return nil
			}

			added, skipped := 0, 0
			for _, r := range results {
				switch r.Action {
				case "skip":
					skipped++
					continue
				case "written":
					added++
					formatter.Success(fmt.Sprintf("%s (%s) → frontmatter written", r.Name, r.Source))
					continue
				case "embedded-readonly":
					added++
					formatter.Header(fmt.Sprintf("%s (embedded, read-only)", r.Name))
				case "would-add":
					added++
					formatter.Header(r.Name)
				}
				formatter.Println(r.Frontmatter)
				formatter.Println("")
			}
			formatter.KeyValue("Migratable", fmt.Sprintf("%d", added))
			formatter.KeyValue("Already conformant", fmt.Sprintf("%d", skipped))
			return nil
		},
	}

	cmd.Flags().BoolVar(&write, "write", false, "prepend generated frontmatter to local skill files (embedded skills stay read-only)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "only print the frontmatter that would be added (does not modify files)")
	return cmd
}

// usageCategory devolve a categoria da skill no Manager ("" se não existir).
func usageCategory(mgr *skills.Manager, name string) string {
	if s, err := mgr.Get(name); err == nil {
		return s.Category
	}
	return ""
}

// usageStateLabel traduz o estado para o rótulo da tabela de status.
func usageStateLabel(state string) string {
	switch state {
	case skills.StateActive:
		return "✓ ativo"
	case skills.StateStale:
		return "⚠ stale"
	case skills.StateArchived:
		return "🗄 arquivado"
	default:
		return state
	}
}

// formatUsageTime imprime um timestamp RFC3339 no formato amigável da casa
// ("" para skills nunca usadas).
func formatUsageTime(ts string) string {
	if ts == "" {
		return "nunca usada"
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	return t.Local().Format("2006-01-02 15:04")
}
