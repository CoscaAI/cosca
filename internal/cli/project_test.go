package cli

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/config"
	embedcosca "github.com/CoscaAI/cosca/internal/embed/cosca"
	"github.com/CoscaAI/cosca/internal/memoryintegrity"
	"github.com/CoscaAI/cosca/internal/project"
	_ "modernc.org/sqlite"
)

// =============================================================================
// cosca project new
// =============================================================================

func TestProjectNew_CreatesDirectoryStructure(t *testing.T) {
	projectsDir := t.TempDir()
	t.Setenv(ProjectsDirEnv, projectsDir)

	path, err := createProject(projectsDir, "acme-app", false, "")
	if err != nil {
		t.Fatalf("createProject: %v", err)
	}
	if filepath.Base(path) != "acme-app" {
		t.Errorf("project path base = %q, want acme-app", filepath.Base(path))
	}

	for _, rel := range []string{
		"README.md",
		".gitignore",
		filepath.Join(".cosca", "manifest.yaml"),
		filepath.Join(".cosca", "constraints.yaml"),
		filepath.Join(".cosca", "config.yml"),
		filepath.Join(".cosca", "state.yml"),
		filepath.Join("src", ".gitkeep"),
		filepath.Join("docs", ".gitkeep"),
		filepath.Join("tests", ".gitkeep"),
		filepath.Join("scripts", ".gitkeep"),
	} {
		if _, err := os.Stat(filepath.Join(path, rel)); err != nil {
			t.Errorf("missing scaffold file %s: %v", rel, err)
		}
	}

	// Manifest carries the client identity — NOT the framework chain.
	manifest, err := embedcosca.ReadManifest(filepath.Join(path, ".cosca"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if manifest["identity"] != "cosca-project" {
		t.Errorf("manifest identity = %q, want cosca-project", manifest["identity"])
	}
	if manifest["project_name"] != "acme-app" {
		t.Errorf("manifest project_name = %q, want acme-app", manifest["project_name"])
	}
	if manifest["module"] != "" {
		t.Errorf("manifest module = %q, want empty (no framework module inheritance)", manifest["module"])
	}

	// Constraints default to permissive.
	c, err := config.LoadConstraints(path)
	if err != nil {
		t.Fatalf("load constraints: %v", err)
	}
	if !c.Docs || !c.Network || !c.Test || !c.Build {
		t.Errorf("expected permissive constraints, got %+v", c)
	}
	if c.ReadOnly {
		t.Error("expected read_only to be false")
	}

	// config.yml carries the project name.
	cfgData, err := os.ReadFile(filepath.Join(path, ".cosca", "config.yml"))
	if err != nil {
		t.Fatalf("read config.yml: %v", err)
	}
	if !strings.Contains(string(cfgData), `name: "acme-app"`) {
		t.Errorf("config.yml missing project name, got:\n%s", cfgData)
	}

	// README documents the jail and cosca usage.
	readme, err := os.ReadFile(filepath.Join(path, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	for _, want := range []string{"acme-app", "cosca terminal", "jail"} {
		if !strings.Contains(string(readme), want) {
			t.Errorf("README.md missing %q", want)
		}
	}

	// .gitignore has the standard entries.
	gitignore, err := os.ReadFile(filepath.Join(path, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	for _, want := range []string{"node_modules/", ".venv/", "dist/", ".next/", "__pycache__/", "*.db"} {
		if !strings.Contains(string(gitignore), want) {
			t.Errorf(".gitignore missing %q", want)
		}
	}
}

func TestProjectNew_RefusesInvalidNames(t *testing.T) {
	projectsDir := t.TempDir()
	t.Setenv(ProjectsDirEnv, projectsDir)

	for _, name := range []string{
		"My App",    // space
		"my.app",    // dot
		"my/app",    // slash
		"my\\app",   // backslash
		"../evil",   // traversal
		"my_app",    // underscore
		"UPPER",     // uppercase
		"",          // empty
		"-leading",  // leading hyphen
		"trailing-", // trailing hyphen
		"a--b",      // double hyphen
	} {
		if _, err := createProject(projectsDir, name, false, ""); err == nil {
			t.Errorf("expected error for invalid name %q", name)
		}
	}

	// No stray directories should have been created.
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		t.Fatalf("read projects dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("invalid names must not create directories, got %d entries", len(entries))
	}
}

func TestProjectNew_RefusesOverwriteWithoutForce(t *testing.T) {
	projectsDir := t.TempDir()
	t.Setenv(ProjectsDirEnv, projectsDir)

	if _, err := createProject(projectsDir, "acme", false, ""); err != nil {
		t.Fatalf("first create: %v", err)
	}

	if _, err := createProject(projectsDir, "acme", false, ""); err == nil {
		t.Fatal("expected error overwriting without --force")
	}

	// With --force the project is recreated.
	if _, err := createProject(projectsDir, "acme", true, ""); err != nil {
		t.Fatalf("overwrite with --force: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectsDir, "acme", "README.md")); err != nil {
		t.Fatalf("README.md missing after --force recreate: %v", err)
	}
}

func TestProjectNew_CreatesMemoryIntegrityManifest(t *testing.T) {
	projectsDir := t.TempDir()
	t.Setenv(ProjectsDirEnv, projectsDir)

	path, err := createProject(projectsDir, "demo-cliente", false, "")
	if err != nil {
		t.Fatalf("createProject: %v", err)
	}

	manifestPath := memoryintegrity.DefaultManifestPath(path)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read memory integrity manifest: %v", err)
	}

	var m memoryintegrity.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if m.Version != memoryintegrity.ManifestVersion {
		t.Errorf("manifest version = %q, want %q", m.Version, memoryintegrity.ManifestVersion)
	}
	if m.Algorithm != memoryintegrity.Algorithm {
		t.Errorf("manifest algorithm = %q, want %q", m.Algorithm, memoryintegrity.Algorithm)
	}
	// The manifest lives inside the new project's .cosca tree (its own root),
	// not the framework workspace.
	rel, relErr := filepath.Rel(path, manifestPath)
	if relErr != nil || strings.HasPrefix(rel, "..") {
		t.Errorf("manifest %q is outside the project root %q", manifestPath, path)
	}

	// The manifest must verify cleanly — this is what the runtime integrity
	// gate checks on every startup.
	res, err := memoryintegrity.Verify(path, "")
	if err != nil {
		t.Fatalf("verify manifest: %v", err)
	}
	if len(res.Changed)+len(res.Missing)+len(res.Added) != 0 {
		t.Errorf("manifest mismatch: changed=%d missing=%d added=%d", len(res.Changed), len(res.Missing), len(res.Added))
	}
}

func TestProjectNew_CreatesValidKnowledgeDB(t *testing.T) {
	projectsDir := t.TempDir()
	t.Setenv(ProjectsDirEnv, projectsDir)

	path, err := createProject(projectsDir, "demo-cliente", false, "")
	if err != nil {
		t.Fatalf("createProject: %v", err)
	}

	dbPath := filepath.Join(path, ".cosca", "knowledge.db")
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("knowledge.db missing: %v", err)
	}
	if info.Size() == 0 {
		t.Error("knowledge.db is empty (0 bytes)")
	}

	// Opens as a valid SQLite database with the knowledge schema.
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open knowledge.db: %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table'").Scan(&count); err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	if count == 0 {
		t.Error("knowledge.db has no tables in sqlite_master")
	}

	// Fresh empty DB: no documents indexed (the framework's 566MB DB must
	// never leak into a client project).
	var docs int
	if err := db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&docs); err != nil {
		t.Fatalf("count documents: %v", err)
	}
	if docs != 0 {
		t.Errorf("new project knowledge.db has %d documents, want 0 (must be a fresh DB, not the framework copy)", docs)
	}
}

func TestProjectNew_ForceRecreatesIntegrityAndKnowledge(t *testing.T) {
	projectsDir := t.TempDir()
	t.Setenv(ProjectsDirEnv, projectsDir)

	if _, err := createProject(projectsDir, "acme", false, ""); err != nil {
		t.Fatalf("first create: %v", err)
	}

	// --force overwrite must re-initialize the manifest and knowledge.db.
	if _, err := createProject(projectsDir, "acme", true, ""); err != nil {
		t.Fatalf("overwrite with --force: %v", err)
	}
	if _, err := os.Stat(memoryintegrity.DefaultManifestPath(filepath.Join(projectsDir, "acme"))); err != nil {
		t.Errorf("memory integrity manifest missing after --force recreate: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectsDir, "acme", ".cosca", "knowledge.db")); err != nil {
		t.Errorf("knowledge.db missing after --force recreate: %v", err)
	}
}

// =============================================================================
// cosca project list
// =============================================================================

func TestProjectList_FindsCreatedProjects(t *testing.T) {
	projectsDir := t.TempDir()
	t.Setenv(ProjectsDirEnv, projectsDir)

	for _, name := range []string{"zeta-app", "acme-app", "nebula"} {
		if _, err := createProject(projectsDir, name, false, ""); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}

	projects, err := listProjects(projectsDir)
	if err != nil {
		t.Fatalf("listProjects: %v", err)
	}
	if len(projects) != 3 {
		t.Fatalf("got %d projects, want 3", len(projects))
	}

	// Sorted by name.
	wantOrder := []string{"acme-app", "nebula", "zeta-app"}
	for i, want := range wantOrder {
		if projects[i].Name != want {
			t.Errorf("projects[%d].Name = %q, want %q", i, projects[i].Name, want)
		}
	}

	// Each project reports a size > 0 and a modification time.
	for _, p := range projects {
		if p.SizeBytes <= 0 {
			t.Errorf("project %q size = %d, want > 0", p.Name, p.SizeBytes)
		}
		if p.Size == "" {
			t.Errorf("project %q has empty human-readable size", p.Name)
		}
		if p.Modified.IsZero() {
			t.Errorf("project %q has zero modification time", p.Name)
		}
		if p.Path != filepath.Join(projectsDir, p.Name) {
			t.Errorf("project %q path = %q, want %q", p.Name, p.Path, filepath.Join(projectsDir, p.Name))
		}
	}
}

// =============================================================================
// cosca project remove
// =============================================================================

func TestProjectRemove_RefusesWithoutForce(t *testing.T) {
	projectsDir := t.TempDir()
	t.Setenv(ProjectsDirEnv, projectsDir)

	path, err := createProject(projectsDir, "acme", false, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := removeProject(projectsDir, path, false); err == nil {
		t.Fatal("expected error removing without --force")
	}

	// The project must still exist.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("project was removed without --force: %v", err)
	}
}

func TestProjectRemove_RefusesPathsOutsideProjectsDir(t *testing.T) {
	projectsDir := t.TempDir()
	t.Setenv(ProjectsDirEnv, projectsDir)

	// "../" traversal — parent of the projects dir.
	outside := filepath.Join(projectsDir, "..", "target")
	if err := removeProject(projectsDir, outside, true); err == nil {
		t.Error("expected error for ../ traversal path")
	}

	// Absolute path completely outside the projects dir.
	unrelated := t.TempDir()
	if err := removeProject(projectsDir, unrelated, true); err == nil {
		t.Error("expected error for unrelated absolute path")
	}

	// The projects directory itself must never be removable.
	if err := removeProject(projectsDir, projectsDir, true); err == nil {
		t.Error("expected error removing the projects directory itself")
	}

	// Name-level traversal is rejected by projectPath too.
	if _, err := projectPath(projectsDir, ".."); err == nil {
		t.Error("expected error for projectPath with '..'")
	}
	if _, err := projectPath(projectsDir, "../evil"); err == nil {
		t.Error("expected error for projectPath with '../evil'")
	}
}

func TestProjectRemove_WithForceRemoves(t *testing.T) {
	projectsDir := t.TempDir()
	t.Setenv(ProjectsDirEnv, projectsDir)

	path, err := createProject(projectsDir, "acme", false, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := removeProject(projectsDir, path, true); err != nil {
		t.Fatalf("remove with --force: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("project still exists after remove: %v", err)
	}

	// Removing a missing project fails cleanly.
	if err := removeProject(projectsDir, path, true); err == nil {
		t.Error("expected error removing a missing project")
	}
}

// =============================================================================
// Project Manifest (§34) — .cosca/project.yaml
// =============================================================================

func TestProjectNew_WithTypeWritesProjectManifest(t *testing.T) {
	projectsDir := t.TempDir()
	t.Setenv(ProjectsDirEnv, projectsDir)

	path, err := createProject(projectsDir, "hornfit-cinema", false, "cinema")
	if err != nil {
		t.Fatalf("create with type: %v", err)
	}

	m, err := project.Read(path)
	if err != nil {
		t.Fatalf("read project manifest: %v", err)
	}
	if m.Name != "hornfit-cinema" {
		t.Errorf("manifest name = %q, want hornfit-cinema", m.Name)
	}
	if m.Type != "cinema" {
		t.Errorf("manifest type = %q, want cinema", m.Type)
	}
	if !m.IsType(project.TypeCinema) {
		t.Error("IsType(cinema) should be true")
	}

	// Coexistência: o manifest.yaml da identidade continua intacto.
	ident, err := embedcosca.ReadManifest(filepath.Join(path, ".cosca"))
	if err != nil {
		t.Fatalf("read identity manifest: %v", err)
	}
	if ident["identity"] != ProjectIdentity {
		t.Errorf("identity manifest identity = %q, want %q", ident["identity"], ProjectIdentity)
	}
}

func TestProjectNew_InvalidTypeFails(t *testing.T) {
	projectsDir := t.TempDir()
	t.Setenv(ProjectsDirEnv, projectsDir)

	if _, err := createProject(projectsDir, "bad", false, "holograma"); err == nil {
		t.Fatal("expected error for invalid project type")
	}

	// O projeto não deve ter sido criado parcialmente (o scaffold valida antes).
	if _, err := os.Stat(filepath.Join(projectsDir, "bad")); !os.IsNotExist(err) {
		t.Errorf("invalid-type project should not exist: %v", err)
	}
}

func TestProjectNew_WithoutTypeHasNoProjectManifest(t *testing.T) {
	projectsDir := t.TempDir()
	t.Setenv(ProjectsDirEnv, projectsDir)

	if _, err := createProject(projectsDir, "legado", false, ""); err != nil {
		t.Fatalf("create legacy: %v", err)
	}
	if _, err := project.Read(projectsDir + "/legado"); !os.IsNotExist(err) {
		t.Errorf("legacy project should have no project.yaml, got %v", err)
	}
}

func TestProjectManifest_SetTypeCommand(t *testing.T) {
	projectsDir := t.TempDir()
	t.Setenv(ProjectsDirEnv, projectsDir)

	if _, err := createProject(projectsDir, "acme", false, ""); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Sem --set-type e sem project.yaml → erro (not found).
	if _, err := project.Read(filepath.Join(projectsDir, "acme")); !os.IsNotExist(err) {
		t.Errorf("expected no project manifest initially, got %v", err)
	}

	// --set-type image cria o manifesto com o tipo.
	m, err := project.New("acme", project.TypeImage)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Write(filepath.Join(projectsDir, "acme")); err != nil {
		t.Fatalf("write manifest via set-type: %v", err)
	}

	got, err := project.Read(filepath.Join(projectsDir, "acme"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "image" {
		t.Errorf("set-type result = %q, want image", got.Type)
	}
}
