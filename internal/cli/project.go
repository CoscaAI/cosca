package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/embed"
	embedcosca "github.com/CoscaAI/cosca/internal/embed/cosca"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/memoryintegrity"
	"github.com/CoscaAI/cosca/internal/project"
)

const (
	// ProjectsDirEnv overrides the default projects directory.
	ProjectsDirEnv = "COSCA_PROJECTS_DIR"

	// ProjectIdentity is the manifest identity written for client projects.
	// Client projects are deliberately NOT the Cosca self-project, so they
	// never inherit the framework chain (family_chain.dat) or any cosca root.
	ProjectIdentity = "cosca-project"

	// ProjectManifestVersion is the schema version of the client manifest.
	ProjectManifestVersion = "1.0.0"
)

// projectNamePattern restricts project names to lowercase letters, digits and
// hyphens (no spaces, dots, slashes or any path separator).
var projectNamePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// ProjectInfo describes a client project found in the projects directory.
type ProjectInfo struct {
	Name      string    `json:"name" yaml:"name"`
	Path      string    `json:"path" yaml:"path"`
	SizeBytes int64     `json:"size_bytes" yaml:"size_bytes"`
	Size      string    `json:"size" yaml:"size"`
	Modified  time.Time `json:"modified" yaml:"modified"`
}

// NewProjectCommand creates the `cosca project` command group.
func NewProjectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage client projects outside the cosca workspace",
		Long: `Manage client projects created outside the cosca workspace.

Client projects live in the projects directory (default ~/Documents/projects/,
override with $COSCA_PROJECTS_DIR) and have their OWN isolated .cosca — they
do NOT inherit the framework chain or any cosca root state.

When you run "cosca terminal" inside a client project, the jail mounts THAT
project as its root, giving the agent perfect isolation.

Subcommands:
  new <name>     Create a new client project
  list           List existing client projects
  open <name>    Print the cd command (or just the path with --cd)
  remove <name>  Delete a client project (requires --force)`,
		Example: `  cosca project new acme-app     Create ~/Documents/projects/acme-app
  cosca project list              List projects
  cosca project open acme-app     cd ~/Documents/projects/acme-app
  cosca project remove acme-app --force`,
	}

	cmd.AddCommand(
		NewProjectNewCommand(),
		NewProjectListCommand(),
		NewProjectOpenCommand(),
		NewProjectRemoveCommand(),
		NewProjectManifestCommand(),
	)
	return cmd
}

// NewProjectNewCommand creates `cosca project new <name>`.
func NewProjectNewCommand() *cobra.Command {
	var force bool
	var projectType string

	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Create a new client project in the projects directory",
		Long: `Create a new client project in the projects directory
(default ~/Documents/projects/<name>, override with $COSCA_PROJECTS_DIR).

The scaffold contains a README, .gitignore, its own isolated .cosca/
(manifest, constraints, config and state) plus empty src/, docs/, tests/ and
scripts/ directories. A git repository is initialized if git is available.

The command NEVER touches the cosca workspace: the client project has its own
.cosca and does not inherit the framework chain.

Since the Creative/Scientific/Media ecosystem, every project declares a type
(--type) in its Project Manifest (.cosca/project.yaml): editor, image, cinema,
music, game, scientific, 3d, animation, document or lab. The manifest records
models, assets, workflows, dependencies and licenses so the project is
reproducible (§33-§35).`,
		Example: `  cosca project new acme-app
  cosca project new hornfit-cinema --type cinema
  cosca project new acme-app --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			projectsDir, err := resolveProjectsDir(true)
			if err != nil {
				return err
			}

			path, err := createProject(projectsDir, args[0], force, projectType)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, map[string]string{
					"name":   args[0],
					"path":   path,
					"status": "created",
					"type":   projectType,
				})
			}

			formatter.Success(fmt.Sprintf("Project %s created at %s", args[0], path))
			formatter.Println("   cd " + path)
			if projectType != "" {
				formatter.Println("   type: " + projectType + " (Project Manifest .cosca/project.yaml)")
			}
			formatter.Println("   cosca terminal        # start working (jail mounts THIS project as root)")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite an existing project")
	cmd.Flags().StringVar(&projectType, "type", "", "Project type for the Project Manifest: editor, image, cinema, music, game, scientific, 3d, animation, document, lab (default: none)")
	return cmd
}

// NewProjectListCommand creates `cosca project list`.
func NewProjectListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List existing client projects",
		Example: `  cosca project list
  cosca project list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			projectsDir, err := resolveProjectsDir(false)
			if err != nil {
				return err
			}

			projects, err := listProjects(projectsDir)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, projects)
			}

			if len(projects) == 0 {
				formatter.Warning(fmt.Sprintf("No projects found in %s — create one with 'cosca project new <name>'", projectsDir))
				return nil
			}

			formatter.Header(fmt.Sprintf("Client Projects (%d)", len(projects)))
			formatter.KeyValue("Projects Dir", projectsDir)
			formatter.Println("")

			rows := make([][]string, 0, len(projects))
			for _, p := range projects {
				rows = append(rows, []string{
					p.Name,
					p.Size,
					p.Modified.Format("2006-01-02 15:04"),
				})
			}
			formatter.Table([]string{"Name", "Size", "Last Modified"}, rows)
			return nil
		},
	}

	return cmd
}

// NewProjectOpenCommand creates `cosca project open <name>`.
func NewProjectOpenCommand() *cobra.Command {
	var cdOnly bool

	cmd := &cobra.Command{
		Use:   "open <name>",
		Short: "Print the cd command for a project",
		Long: `Print the cd command to enter a client project.

By default the full "cd <path>" command is printed. With --cd only the path is
printed, so it can be consumed by scripts:

    cd "$(cosca project open acme-app --cd)"`,
		Example: `  cosca project open acme-app
  cosca project open acme-app --cd`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectsDir, err := resolveProjectsDir(false)
			if err != nil {
				return err
			}

			path, err := projectPath(projectsDir, args[0])
			if err != nil {
				return err
			}

			if !dirExists(path) {
				return fmt.Errorf("project %q not found at %s", args[0], path)
			}

			if cdOnly {
				fmt.Fprintln(cmd.OutOrStdout(), path)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "cd %s\n", path)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&cdOnly, "cd", false, "Print only the project path (for scripting)")
	return cmd
}

// NewProjectRemoveCommand creates `cosca project remove <name>`.
func NewProjectRemoveCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Delete a client project (requires --force)",
		Long: `Delete a client project from the projects directory.

Deleting is destructive and therefore requires --force. The command refuses to
remove the projects directory itself, the cosca workspace, or any path that
escapes the projects directory (path traversal protection).`,
		Example: `  cosca project remove acme-app --force`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			projectsDir, err := resolveProjectsDir(false)
			if err != nil {
				return err
			}

			path, err := projectPath(projectsDir, args[0])
			if err != nil {
				return err
			}

			if err := removeProject(projectsDir, path, force); err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, map[string]string{
					"name":   args[0],
					"path":   path,
					"status": "removed",
				})
			}

			formatter.Success(fmt.Sprintf("Project %s removed", args[0]))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Required to delete a project")
	return cmd
}

// NewProjectManifestCommand creates `cosca project manifest <name>`.
func NewProjectManifestCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "manifest <name>",
		Short: "Show the Project Manifest of a client project",
		Long: `Show the Project Manifest (.cosca/project.yaml) of a client project.

The Project Manifest declares the product type, models, assets, workflows,
dependencies and licenses — the reproducibility contract of the Creative/
Scientific/Media ecosystem (§33-§35). Projects created without --type have no
Project Manifest (only the framework manifest.yaml); use "cosca project manifest
<name> --set-type cinema" to add one.`,
		Example: `  cosca project manifest acme-app
  cosca project manifest hornfit-cinema --json
  cosca project manifest acme-app --set-type image`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)
			setType := cmd.Flags().Lookup("set-type").Value.String()

			projectsDir, err := resolveProjectsDir(false)
			if err != nil {
				return err
			}
			path, err := projectPath(projectsDir, args[0])
			if err != nil {
				return err
			}
			if !dirExists(path) {
				return fmt.Errorf("project %q not found at %s", args[0], path)
			}

			// --set-type: cria/atualiza o Project Manifest com o tipo dado.
			if setType != "" {
				typ := project.ProjectType(setType)
				if !typ.Valid() {
					return fmt.Errorf("invalid project type %q (valid: %s)", setType, project.TypesList())
				}
				m, err := project.Read(path)
				if err != nil {
					if os.IsNotExist(err) {
						m, err = project.New(args[0], typ)
					}
					if err != nil {
						return err
					}
				}
				m.Type = typ.String()
				if err := m.Write(path); err != nil {
					return fmt.Errorf("write project manifest: %w", err)
				}
				if useJSON {
					return printJSON(cmd, map[string]string{
						"name": args[0],
						"type": typ.String(),
						"path": m.Path(path),
					})
				}
				formatter.Success(fmt.Sprintf("Project %s type set to %s", args[0], typ))
				formatter.Println("   manifest: " + m.Path(path))
				return nil
			}

			m, err := project.Read(path)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("project %q has no Project Manifest (create with 'cosca project new --type' or 'cosca project manifest %s --set-type <type>')", args[0], args[0])
				}
				return err
			}
			if useJSON {
				return printJSON(cmd, m)
			}
			formatter.Header(fmt.Sprintf("Project Manifest — %s", m.Name))
			formatter.KeyValue("Type", m.Type)
			formatter.KeyValue("Version", m.Version)
			formatter.KeyValue("Engine Version", m.EngineVersion)
			formatter.KeyValue("Models", fmt.Sprint(len(m.Models)))
			formatter.KeyValue("Assets", fmt.Sprint(len(m.Assets)))
			formatter.KeyValue("Workflows", fmt.Sprint(len(m.Workflows)))
			formatter.KeyValue("Plugins", fmt.Sprint(len(m.Plugins)))
			formatter.KeyValue("Dependencies", fmt.Sprint(len(m.Dependencies)))
			formatter.KeyValue("Updated", m.Timestamp)
			return nil
		},
	}

	cmd.Flags().String("set-type", "", "Set the project type on the Project Manifest: "+project.TypesList())
	return cmd
}

// resolveProjectsDir resolves the projects directory from $COSCA_PROJECTS_DIR
// or defaults to ~/Documents/projects. When create is true the directory is
// created if missing. It refuses the filesystem root and any directory inside
// the cosca workspace.
func resolveProjectsDir(create bool) (string, error) {
	p := os.Getenv(ProjectsDirEnv)
	if p == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("home dir: %w", err)
		}
		p = filepath.Join(home, "Documents", "projects")
	}

	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("resolve projects dir: %w", err)
	}
	abs = filepath.Clean(abs)

	if err := validateProjectsDir(abs); err != nil {
		return "", err
	}

	if create {
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return "", fmt.Errorf("create projects dir %s: %w", abs, err)
		}
	}
	return abs, nil
}

// validateProjectsDir enforces that the projects directory is outside the
// cosca workspace and never the filesystem root.
func validateProjectsDir(dir string) error {
	if filepath.Clean(dir) == string(filepath.Separator) {
		return fmt.Errorf("refusing filesystem root as projects directory")
	}
	if root := findCoscaWorkspaceRoot(); root != "" && pathWithin(root, dir) {
		return fmt.Errorf("projects directory %q must be outside the cosca workspace %q", dir, root)
	}
	return nil
}

// findCoscaWorkspaceRoot walks up from the current directory looking for the
// cosca self-project (go.mod with module github.com/CoscaAI/cosca). Returns ""
// if the workspace cannot be located.
func findCoscaWorkspaceRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if embedcosca.IsSelfProject(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// pathWithin reports whether target is inside root (or equal to it).
func pathWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// validateProjectName restricts project names to lowercase letters, digits and
// hyphens only (no spaces, dots or slashes).
func validateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	if len(name) > 64 {
		return fmt.Errorf("project name too long (max 64 characters)")
	}
	if !projectNamePattern.MatchString(name) {
		return fmt.Errorf("invalid project name %q: use lowercase letters, digits and hyphens only (no spaces, dots or slashes)", name)
	}
	return nil
}

// projectPath safely resolves a project path inside the projects directory,
// rejecting any name that escapes it (path traversal protection).
func projectPath(projectsDir, name string) (string, error) {
	if err := validateProjectName(name); err != nil {
		return "", err
	}
	path := filepath.Join(projectsDir, name)
	if filepath.Clean(path) != path {
		return "", fmt.Errorf("invalid project path %q", path)
	}
	if !pathWithin(projectsDir, path) {
		return "", fmt.Errorf("project path %q escapes projects directory %q", path, projectsDir)
	}
	return path, nil
}

// createProject scaffolds a new client project. It refuses to overwrite an
// existing project unless force is set. The project type is validated BEFORE
// any scaffold happens (no partial project on invalid input).
func createProject(projectsDir, name string, force bool, projectType string) (string, error) {
	if projectType != "" {
		typ := project.ProjectType(projectType)
		if !typ.Valid() {
			return "", fmt.Errorf("invalid project type %q (valid: %s)", projectType, project.TypesList())
		}
	}

	path, err := projectPath(projectsDir, name)
	if err != nil {
		return "", err
	}

	exists, err := pathExists(path)
	if err != nil {
		return "", err
	}
	if exists {
		if !force {
			return "", fmt.Errorf("project %q already exists at %s (use --force to overwrite)", name, path)
		}
		if err := os.RemoveAll(path); err != nil {
			return "", fmt.Errorf("remove existing project: %w", err)
		}
	}

	if err := scaffoldProject(path, name, projectType); err != nil {
		return "", err
	}

	// Initialize the project's isolated memory integrity manifest and
	// knowledge database so the project works immediately (the integrity gate
	// refuses to start commands until the manifest exists). On failure the
	// partially-initialized project is rolled back cleanly.
	if err := initProjectWorkspace(path, force); err != nil {
		_ = os.RemoveAll(path)
		return "", err
	}
	return path, nil
}

// initProjectWorkspace initializes the new client project's own memory
// integrity manifest and its empty knowledge database. Both live inside the
// project's isolated .cosca/ and never touch the framework state.
func initProjectWorkspace(path string, force bool) error {
	// 1. Memory integrity manifest (.cosca/audit/memory-integrity-manifest.json).
	// enforceMemoryIntegrityGate blocks every command (except init/version)
	// until this manifest exists and verifies clean. When force is set the
	// project was just recreated, so the manifest cannot exist — WriteWithForce
	// keeps the path idempotent regardless.
	write := memoryintegrity.Write
	if force {
		write = memoryintegrity.WriteWithForce
	}
	if _, err := write(path, ""); err != nil {
		return fmt.Errorf("initialize memory integrity manifest: %w", err)
	}

	// 2. Fresh, empty knowledge database (<project>/.cosca/knowledge.db) — a
	// brand-new SQLite DB owned by THIS project, never a copy of the framework
	// DB (~/.cosca/knowledge.db must remain untouched).
	cfg := knowledge.DefaultConfig()
	cfg.DBPath = filepath.Join(path, ".cosca", "knowledge.db")
	cfg.RootDir = path
	cfg.AutoMigrate = true
	cfg.WatchEnabled = false

	ke, err := knowledge.New(cfg)
	if err != nil {
		return fmt.Errorf("initialize knowledge engine: %w", err)
	}
	if err := ke.Init(); err != nil {
		_ = ke.Close()
		return fmt.Errorf("initialize knowledge database: %w", err)
	}
	if err := ke.Close(); err != nil {
		return fmt.Errorf("close knowledge engine: %w", err)
	}

	// 3. Verify the database file exists and is a valid SQLite database.
	if _, err := os.Stat(cfg.DBPath); err != nil {
		return fmt.Errorf("knowledge database not created at %s: %w", cfg.DBPath, err)
	}
	return nil
}

// scaffoldProject creates the full project structure for a client project.
// It never touches the cosca workspace: everything is written inside path.
func scaffoldProject(path, name string, projectType string) error {
	coscaDir := filepath.Join(path, ".cosca")

	dirs := []string{
		path,
		coscaDir,
		filepath.Join(path, "src"),
		filepath.Join(path, "docs"),
		filepath.Join(path, "tests"),
		filepath.Join(path, "scripts"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", d, err)
		}
	}

	created := time.Now().UTC().Format("2006-01-02")

	readme := fmt.Sprintf(`# %s

Created: %s via "cosca project new"

## Getting Started

    cd %s
    cosca terminal

## The Jail

Cosca runs itself inside a sandbox ("jail"). The jail mounts THIS project
directory as the workspace root — the agent can only see and modify files
inside this project. The Cosca framework is compiled directly into the "cosca"
binary, so no framework files are needed inside this project. This project has
its own isolated .cosca/ and does not inherit the framework chain.

## Layout

    src/      source code
    docs/     documentation
    tests/    tests
    scripts/  helper scripts
`, name, created, path)

	if err := os.WriteFile(filepath.Join(path, "README.md"), []byte(readme), 0o644); err != nil {
		return fmt.Errorf("write README.md: %w", err)
	}

	const gitignore = "node_modules/\n.venv/\ndist/\n.next/\n__pycache__/\n*.db\n"
	if err := os.WriteFile(filepath.Join(path, ".gitignore"), []byte(gitignore), 0o644); err != nil {
		return fmt.Errorf("write .gitignore: %w", err)
	}

	if err := writeProjectManifest(coscaDir, name); err != nil {
		return err
	}

	// Project Manifest do ecossistema Creative/Scientific/Media (§34). Quando
	// o tipo é fornecido, o manifest de projeto é gravado; sem tipo, o projeto
	// continua sendo um cliente genérico (sem project.yaml — compatível).
	// O tipo já foi validado em createProject (antes do scaffold).
	if projectType != "" {
		m, err := project.New(name, project.ProjectType(projectType))
		if err != nil {
			return fmt.Errorf("create project manifest: %w", err)
		}
		if err := m.Write(path); err != nil {
			return fmt.Errorf("write project manifest: %w", err)
		}
		// Kit de início (§31): artefato de exemplo por tipo (scene/workflow/obj).
		if err := writeProjectExamples(path, projectType); err != nil {
			return fmt.Errorf("write project examples: %w", err)
		}
	}

	// Permissive defaults: docs, network, test and build all allowed.
	if err := config.SaveConstraints(path, config.DefaultConstraints()); err != nil {
		return fmt.Errorf("write constraints: %w", err)
	}

	cfgTemplate, err := embed.ReadFile("templates/scaffold/config.yml")
	if err != nil {
		return fmt.Errorf("read scaffold config template: %w", err)
	}
	cfgContent := strings.Replace(string(cfgTemplate), `name: ""`, fmt.Sprintf(`name: %q`, name), 1)
	if err := os.WriteFile(filepath.Join(coscaDir, "config.yml"), []byte(cfgContent), 0o644); err != nil {
		return fmt.Errorf("write config.yml: %w", err)
	}

	stateTemplate, err := embed.ReadFile("templates/scaffold/state.yml")
	if err != nil {
		return fmt.Errorf("read scaffold state template: %w", err)
	}
	if err := os.WriteFile(filepath.Join(coscaDir, "state.yml"), stateTemplate, 0o644); err != nil {
		return fmt.Errorf("write state.yml: %w", err)
	}

	for _, d := range []string{"src", "docs", "tests", "scripts"} {
		if err := os.WriteFile(filepath.Join(path, d, ".gitkeep"), []byte{}, 0o644); err != nil {
			return fmt.Errorf("write %s/.gitkeep: %w", d, err)
		}
	}

	gitInit(path)
	return nil
}

// writeProjectManifest writes .cosca/manifest.yaml with the client project
// identity (cosca-project). The client project does NOT inherit the framework
// chain — it has its own isolated manifest.
func writeProjectManifest(coscaDir, name string) error {
	manifest := map[string]string{
		"project_name": name,
		"identity":     ProjectIdentity,
		"module":       "",
		"version":      ProjectManifestVersion,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
	}
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.MkdirAll(coscaDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", coscaDir, err)
	}
	if err := os.WriteFile(filepath.Join(coscaDir, embedcosca.ManifestFileName), data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", embedcosca.ManifestFileName, err)
	}
	return nil
}

// gitInit initializes a git repository in dir. Best-effort: if git is
// unavailable the project is still created (no initial commit is made).
func gitInit(dir string) {
	git, err := exec.LookPath("git")
	if err != nil {
		return
	}
	cmd := exec.Command(git, "init")
	cmd.Dir = dir
	_ = cmd.Run()
}

// listProjects returns the client projects found in the projects directory,
// sorted by name, including size and last modification time.
func listProjects(projectsDir string) ([]ProjectInfo, error) {
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read projects dir %s: %w", projectsDir, err)
	}

	var projects []ProjectInfo
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		path := filepath.Join(projectsDir, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}
		size := dirSize(path)
		projects = append(projects, ProjectInfo{
			Name:      e.Name(),
			Path:      path,
			SizeBytes: size,
			Size:      formatSize(size),
			Modified:  info.ModTime(),
		})
	}

	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
	return projects, nil
}

// removeProject deletes a project directory. It requires force and refuses to
// remove the projects directory itself, the cosca workspace, or any path that
// escapes the projects directory.
func removeProject(projectsDir, path string, force bool) error {
	if !pathWithin(projectsDir, path) {
		return fmt.Errorf("refusing to remove %q: path is outside the projects directory %q", path, projectsDir)
	}
	if filepath.Clean(path) == filepath.Clean(projectsDir) {
		return fmt.Errorf("refusing to remove the projects directory itself")
	}
	if root := findCoscaWorkspaceRoot(); root != "" && filepath.Clean(path) == filepath.Clean(root) {
		return fmt.Errorf("refusing to remove the cosca workspace")
	}

	exists, err := pathExists(path)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("project %q not found at %s", filepath.Base(path), path)
	}
	if !force {
		return fmt.Errorf("refusing to remove %s without --force", path)
	}

	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove project %s: %w", path, err)
	}
	return nil
}

// pathExists reports whether the path exists (file or directory).
func pathExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
