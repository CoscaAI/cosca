package discovery

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"golang.org/x/mod/modfile"
	"gopkg.in/yaml.v3"
)

// ProjectInfo contains all detected project metadata.
type ProjectInfo struct {
	Name             string   `json:"name" yaml:"name"`
	Root             string   `json:"root" yaml:"root"`
	Language         string   `json:"language" yaml:"language"`
	LanguageVersion  string   `json:"language_version,omitempty" yaml:"language_version,omitempty"`
	Framework        string   `json:"framework,omitempty" yaml:"framework,omitempty"`
	FrameworkVersion string   `json:"framework_version,omitempty" yaml:"framework_version,omitempty"`
	Database         string   `json:"database,omitempty" yaml:"database,omitempty"`
	Architecture     string   `json:"architecture,omitempty" yaml:"architecture,omitempty"`
	PackageManager   string   `json:"package_manager,omitempty" yaml:"package_manager,omitempty"`
	BuildSystem      string   `json:"build_system,omitempty" yaml:"build_system,omitempty"`
	TestFramework    string   `json:"test_framework,omitempty" yaml:"test_framework,omitempty"`
	Dockerized       bool     `json:"dockerized" yaml:"dockerized"`
	CiCd             string   `json:"ci_cd,omitempty" yaml:"ci_cd,omitempty"`
	Dependencies     []string `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	ConfigFiles      []string `json:"config_files,omitempty" yaml:"config_files,omitempty"`
}

// DetectProject performs comprehensive project detection.
func DetectProject(ctx context.Context, workDir string, logger zerolog.Logger) (*ProjectInfo, error) {
	info := &ProjectInfo{
		Root: workDir,
	}

	// Get absolute path
	absPath, err := filepath.Abs(workDir)
	if err == nil {
		info.Root = absPath
	}

	// Detect project name from directory
	info.Name = filepath.Base(info.Root)

	// Discover config files
	info.ConfigFiles = findConfigFiles(ctx, workDir)

	// Detect language
	info.Language, info.LanguageVersion = detectLanguage(ctx, workDir, info.ConfigFiles)

	// Detect framework
	info.Framework, info.FrameworkVersion = detectFramework(ctx, workDir, info.Language, info.ConfigFiles)

	// Detect database
	info.Database = detectDatabase(ctx, workDir, info.Language)

	// Detect architecture
	info.Architecture = detectArchitecture(ctx, workDir)

	// Detect package manager
	info.PackageManager = detectPackageManager(ctx, workDir, info.Language, info.ConfigFiles)

	// Detect build system
	info.BuildSystem = detectBuildSystem(ctx, workDir, info.Language, info.ConfigFiles)

	// Detect test framework
	info.TestFramework = detectTestFramework(ctx, workDir, info.Language, info.ConfigFiles)

	// Check for Docker
	info.Dockerized = detectDocker(ctx, workDir)

	// Detect CI/CD
	info.CiCd = detectCICD(ctx, workDir)

	// Detect dependencies
	info.Dependencies = detectDependencies(ctx, workDir, info.Language, info.ConfigFiles)

	logger.Debug().
		Str("name", info.Name).
		Str("language", info.Language).
		Str("framework", info.Framework).
		Str("database", info.Database).
		Str("architecture", info.Architecture).
		Str("package_manager", info.PackageManager).
		Msg("project discovery complete")

	return info, nil
}

// findConfigFiles discovers configuration files in the project.
func findConfigFiles(_ context.Context, workDir string) []string {
	var files []string
	patterns := []string{
		"package.json", "go.mod", "Cargo.toml", "pyproject.toml",
		"build.gradle", "build.gradle.kts", "CMakeLists.txt",
		"composer.json", "Gemfile", "mix.exs", "project.clj",
		"tsconfig.json", "webpack.config.js", "vite.config.ts",
		"next.config.js", "nuxt.config.ts", "astro.config.mjs",
		"Dockerfile", "docker-compose.yml", "Makefile",
		".github/workflows/*.yml", ".gitlab-ci.yml",
		".env", ".env.example",
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(workDir, pattern))
		if err != nil {
			continue
		}
		files = append(files, matches...)
	}

	return files
}

// detectLanguage detects the primary programming language.
func detectLanguage(ctx context.Context, workDir string, configFiles []string) (string, string) {
	for _, f := range configFiles {
		name := filepath.Base(f)
		switch name {
		case "go.mod":
			return detectGoVersion(f)
		case "package.json":
			return detectNodeVersion(f)
		case "Cargo.toml":
			return detectRustVersion(f)
		case "pyproject.toml":
			return detectPythonVersion(f)
		case "build.gradle", "build.gradle.kts":
			return "Java", ""
		case "CMakeLists.txt":
			return "C++", ""
		case "composer.json":
			return "PHP", ""
		case "Gemfile":
			return "Ruby", ""
		case "mix.exs":
			return "Elixir", ""
		case "project.clj":
			return "Clojure", ""
		}
	}

	// Fallback: check file extensions
	return detectLanguageByExtension(ctx, workDir)
}

// detectGoVersion reads the Go version from go.mod.
func detectGoVersion(goModPath string) (string, string) {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "Go", ""
	}
	f, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return "Go", ""
	}
	if f.Go != nil {
		return "Go", f.Go.Version
	}
	return "Go", ""
}

// detectNodeVersion reads name and version from package.json.
func detectNodeVersion(pkgJSONPath string) (string, string) {
	data, err := os.ReadFile(pkgJSONPath)
	if err != nil {
		return "TypeScript", ""
	}
	var pkg struct {
		Name    string `json:"name"`
		Engines struct {
			Node string `json:"node"`
		} `json:"engines"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "TypeScript", ""
	}

	// Detect language based on config files
	dir := filepath.Dir(pkgJSONPath)
	hasTS := false
	hasTSX := false
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".ts") {
			hasTS = true
		}
		if strings.HasSuffix(path, ".tsx") {
			hasTSX = true
		}
		return nil
	})

	lang := "JavaScript"
	if hasTS || hasTSX {
		lang = "TypeScript"
	}
	return lang, pkg.Engines.Node
}

// detectRustVersion reads version from Cargo.toml.
func detectRustVersion(cargoPath string) (string, string) {
	data, err := os.ReadFile(cargoPath)
	if err != nil {
		return "Rust", ""
	}
	var cfg struct {
		Package struct {
			Name    string `yaml:"name"`
			Edition string `yaml:"edition"`
		} `yaml:"package"`
	}
	if err := yaml.Unmarshal(data, &cfg); err == nil {
		return "Rust", cfg.Package.Edition
	}
	return "Rust", ""
}

// detectPythonVersion reads from pyproject.toml.
func detectPythonVersion(pyprojectPath string) (string, string) {
	data, err := os.ReadFile(pyprojectPath)
	if err != nil {
		return "Python", ""
	}
	var cfg struct {
		Project struct {
			RequiresPython string `yaml:"requires-python"`
		} `yaml:"project"`
	}
	if err := yaml.Unmarshal(data, &cfg); err == nil {
		return "Python", cfg.Project.RequiresPython
	}
	return "Python", ""
}

// detectLanguageByExtension scans source files to guess language.
func detectLanguageByExtension(_ context.Context, workDir string) (string, string) {
	extCount := make(map[string]int)
	_ = filepath.WalkDir(workDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// Skip common non-source directories
		rel, _ := filepath.Rel(workDir, path)
		if strings.HasPrefix(rel, "node_modules") || strings.HasPrefix(rel, ".git") ||
			strings.HasPrefix(rel, "vendor") || strings.HasPrefix(rel, "dist") ||
			strings.HasPrefix(rel, "target") || strings.HasPrefix(rel, "__pycache__") {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != "" {
			extCount[ext]++
		}
		return nil
	})

	// Map extensions to languages
	extLangMap := map[string]string{
		".go":    "Go",
		".ts":    "TypeScript",
		".tsx":   "TypeScript",
		".js":    "JavaScript",
		".jsx":   "JavaScript",
		".py":    "Python",
		".rs":    "Rust",
		".java":  "Java",
		".kt":    "Kotlin",
		".rb":    "Ruby",
		".php":   "PHP",
		".c":     "C",
		".cpp":   "C++",
		".cs":    "C#",
		".swift": "Swift",
	}

	maxCount := 0
	lang := "Unknown"
	for ext, count := range extCount {
		if l, ok := extLangMap[ext]; ok && count > maxCount {
			maxCount = count
			lang = l
		}
	}

	if lang == "Unknown" && extCount[".md"] > 0 {
		return "Markdown", ""
	}

	return lang, ""
}

// detectFramework detects the web or application framework.
func detectFramework(_ context.Context, workDir string, _ string, _ []string) (string, string) {
	// Ordered list of framework indicators
	type frameworkIndicator struct {
		name     string
		file     string
		contains string
	}

	indicators := []frameworkIndicator{
		{name: "Next.js", file: "next.config.js"},
		{name: "Next.js", file: "next.config.ts"},
		{name: "Nuxt.js", file: "nuxt.config.ts"},
		{name: "Astro", file: "astro.config.mjs"},
		{name: "Remix", file: "remix.config.js"},
		{name: "Gatsby", file: "gatsby-config.js"},
		{name: "Vue", file: "vue.config.js"},
		{name: "SvelteKit", file: "svelte.config.js"},
		{name: "NestJS", file: "nest-cli.json"},
		{name: "FastAPI", file: "pyproject.toml", contains: "fastapi"},
		{name: "Django", file: "manage.py"},
	}

	for _, ind := range indicators {
		path := filepath.Join(workDir, ind.file)
		if data, err := os.ReadFile(path); err == nil {
			if ind.contains == "" || strings.Contains(strings.ToLower(string(data)), ind.contains) {
				return ind.name, ""
			}
		}
	}

	// Check package manager files for framework dependencies
	depFiles := map[string]string{
		"package.json":   "package.json",
		"go.mod":         "go.mod",
		"Cargo.toml":     "Cargo.toml",
		"pyproject.toml": "pyproject.toml",
	}

	frameworkDeps := map[string][]string{
		"Express": {"express"},
		"Fastify": {"fastify"},
		"Hono":    {"hono"},
		"Elysia":  {"elysia"},
		"Gin":     {"gin"},
		"Echo":    {"echo"},
		"Fiber":   {"fiber"},
		"Actix":   {"actix-web"},
		"Rocket":  {"rocket"},
		"Flask":   {"flask"},
		"Django":  {"django"},
		"FastAPI": {"fastapi"},
		"Spring":  {"spring-boot", "spring-framework"},
		"Laravel": {"laravel"},
		"Rails":   {"rails"},
	}

	for _, depFile := range depFiles {
		path := filepath.Join(workDir, depFile)
		if data, err := os.ReadFile(path); err == nil {
			content := strings.ToLower(string(data))
			for framework, deps := range frameworkDeps {
				for _, dep := range deps {
					if strings.Contains(content, dep) {
						return framework, ""
					}
				}
			}
		}
	}

	return "", ""
}

// detectDatabase detects the database system used by the project.
func detectDatabase(_ context.Context, workDir string, _ string) string {
	dbIndicators := []struct {
		name   string
		search []string
	}{
		{name: "PostgreSQL", search: []string{"postgres", "pg", "psql", "postgresql"}},
		{name: "MySQL", search: []string{"mysql", "mySQL"}},
		{name: "SQLite", search: []string{"sqlite", "sqlite3"}},
		{name: "MongoDB", search: []string{"mongodb", "mongo"}},
		{name: "Redis", search: []string{"redis"}},
		{name: "DynamoDB", search: []string{"dynamodb", "dynamo"}},
		{name: "Cassandra", search: []string{"cassandra"}},
		{name: "Elasticsearch", search: []string{"elasticsearch", "elastic"}},
		{name: "Firebase", search: []string{"firebase", "firestore"}},
		{name: "Supabase", search: []string{"supabase"}},
		{name: "Neo4j", search: []string{"neo4j"}},
		{name: "CockroachDB", search: []string{"cockroach"}},
		{name: "PlanetScale", search: []string{"planetscale"}},
	}

	// Check package manager files for dependencies
	depFiles := []string{"package.json", "go.mod", "Cargo.toml", "pyproject.toml", "requirements.txt", "Gemfile"}
	for _, depFile := range depFiles {
		path := filepath.Join(workDir, depFile)
		if data, err := os.ReadFile(path); err == nil {
			content := strings.ToLower(string(data))
			for _, db := range dbIndicators {
				for _, search := range db.search {
					if strings.Contains(content, search) {
						return db.name
					}
				}
			}
		}
	}

	// Check for docker-compose with database services
	dockerPath := filepath.Join(workDir, "docker-compose.yml")
	if data, err := os.ReadFile(dockerPath); err == nil {
		content := strings.ToLower(string(data))
		for _, db := range dbIndicators {
			for _, search := range db.search {
				if strings.Contains(content, search) {
					return db.name
				}
			}
		}
	}

	// Check env files
	envPath := filepath.Join(workDir, ".env")
	if data, err := os.ReadFile(envPath); err == nil {
		content := strings.ToLower(string(data))
		for _, db := range dbIndicators {
			for _, search := range db.search {
				if strings.Contains(content, search) {
					return db.name
				}
			}
		}
	}

	return ""
}

// detectArchitecture detects the project architecture pattern.
func detectArchitecture(_ context.Context, workDir string) string {
	// Check for monorepo indicators
	if _, err := os.Stat(filepath.Join(workDir, "packages")); err == nil {
		return "monorepo"
	}
	if _, err := os.Stat(filepath.Join(workDir, "apps")); err == nil {
		return "monorepo"
	}

	// Look for docker-compose (likely microservices)
	if _, err := os.Stat(filepath.Join(workDir, "docker-compose.yml")); err == nil {
		data, _ := os.ReadFile(filepath.Join(workDir, "docker-compose.yml"))
		serviceCount := strings.Count(string(data), "image:")
		if serviceCount > 2 {
			return "microservices"
		}
	}

	// Check for service directories
	entries, _ := os.ReadDir(workDir)
	serviceDirs := 0
	for _, entry := range entries {
		if entry.IsDir() && strings.Contains(entry.Name(), "service") {
			serviceDirs++
		}
	}
	if serviceDirs > 1 {
		return "microservices"
	}

	return "monolith"
}

// detectPackageManager detects the package manager.
func detectPackageManager(_ context.Context, workDir string, language string, _ []string) string {
	switch language {
	case "Go":
		return "go mod"
	case "Rust":
		return "cargo"
	case "Python":
		if _, err := os.Stat(filepath.Join(workDir, "poetry.lock")); err == nil {
			return "poetry"
		}
		if _, err := os.Stat(filepath.Join(workDir, "Pipfile")); err == nil {
			return "pipenv"
		}
		if _, err := os.Stat(filepath.Join(workDir, "uv.lock")); err == nil {
			return "uv"
		}
		return "pip"
	case "Java":
		if _, err := os.Stat(filepath.Join(workDir, "gradlew")); err == nil {
			return "gradle"
		}
		return "maven"
	case "Ruby":
		return "bundler"
	case "PHP":
		return "composer"
	case "JavaScript", "TypeScript":
		return detectNodePackageManager(workDir)
	}
	return ""
}

// detectNodePackageManager detects the Node.js package manager.
func detectNodePackageManager(workDir string) string {
	if _, err := os.Stat(filepath.Join(workDir, "pnpm-lock.yaml")); err == nil {
		return "pnpm"
	}
	if _, err := os.Stat(filepath.Join(workDir, "yarn.lock")); err == nil {
		return "yarn"
	}
	if _, err := os.Stat(filepath.Join(workDir, "bun.lockb")); err == nil {
		return "bun"
	}
	if _, err := os.Stat(filepath.Join(workDir, "package-lock.json")); err == nil {
		return "npm"
	}
	return "npm"
}

// detectBuildSystem detects the build system.
func detectBuildSystem(_ context.Context, workDir string, _ string, configFiles []string) string {
	for _, f := range configFiles {
		name := filepath.Base(f)
		switch name {
		case "Makefile":
			return "Make"
		case "webpack.config.js", "webpack.config.ts":
			return "Webpack"
		case "vite.config.ts", "vite.config.js":
			return "Vite"
		case "next.config.js", "next.config.ts":
			return "Next.js (Turbopack)"
		case "tsconfig.json":
			pkgJSON := filepath.Join(workDir, "package.json")
			if data, err := os.ReadFile(pkgJSON); err == nil {
				content := string(data)
				if strings.Contains(content, "tsup") {
					return "tsup"
				}
				if strings.Contains(content, "esbuild") {
					return "esbuild"
				}
				if strings.Contains(content, "rollup") {
					return "Rollup"
				}
				if strings.Contains(content, "parcel") {
					return "Parcel"
				}
			}
			return "tsc"
		case "Cargo.toml":
			return "Cargo"
		case "pyproject.toml":
			return "setuptools"
		case "build.gradle", "build.gradle.kts":
			return "Gradle"
		case "pom.xml":
			return "Maven"
		}
	}
	return ""
}

// detectTestFramework detects the test framework.
func detectTestFramework(_ context.Context, workDir string, language string, _ []string) string {
	switch language {
	case "Go":
		return "go test"
	case "Rust":
		return "cargo test"
	case "Python":
		if _, err := os.Stat(filepath.Join(workDir, "pytest.ini")); err == nil {
			return "pytest"
		}
		if _, err := os.Stat(filepath.Join(workDir, "setup.cfg")); err == nil {
			if data, _ := os.ReadFile(filepath.Join(workDir, "setup.cfg")); strings.Contains(string(data), "pytest") {
				return "pytest"
			}
		}
		return "unittest"
	case "JavaScript", "TypeScript":
		return detectNodeTestFramework(workDir)
	case "Java":
		return "JUnit"
	case "Ruby":
		return "RSpec"
	}
	return ""
}

// detectNodeTestFramework detects the Node.js test framework.
func detectNodeTestFramework(workDir string) string {
	pkgJSONPath := filepath.Join(workDir, "package.json")
	data, err := os.ReadFile(pkgJSONPath)
	if err != nil {
		return ""
	}

	var pkg struct {
		DevDependencies map[string]string `json:"devDependencies"`
		Dependencies    map[string]string `json:"dependencies"`
		Scripts         map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}

	allDeps := make(map[string]string)
	for k, v := range pkg.Dependencies {
		allDeps[k] = v
	}
	for k, v := range pkg.DevDependencies {
		allDeps[k] = v
	}

	if _, ok := allDeps["vitest"]; ok {
		return "Vitest"
	}
	if _, ok := allDeps["jest"]; ok {
		return "Jest"
	}
	if _, ok := allDeps["mocha"]; ok {
		return "Mocha"
	}
	if _, ok := allDeps["ava"]; ok {
		return "AVA"
	}
	if _, ok := allDeps["tape"]; ok {
		return "Tape"
	}
	if _, ok := allDeps["web-test-runner"]; ok {
		return "@web/test-runner"
	}

	// Check scripts
	for _, script := range pkg.Scripts {
		if strings.Contains(script, "vitest") {
			return "Vitest"
		}
		if strings.Contains(script, "jest") {
			return "Jest"
		}
		if strings.Contains(script, "mocha") {
			return "Mocha"
		}
	}

	return ""
}

// detectDocker checks if the project uses Docker.
func detectDocker(_ context.Context, workDir string) bool {
	if _, err := os.Stat(filepath.Join(workDir, "Dockerfile")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(workDir, "docker-compose.yml")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(workDir, "docker-compose.yaml")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(workDir, ".dockerignore")); err == nil {
		return true
	}
	return false
}

// detectCICD detects the CI/CD system.
func detectCICD(_ context.Context, workDir string) string {
	if _, err := os.Stat(filepath.Join(workDir, ".github", "workflows")); err == nil {
		return "GitHub Actions"
	}
	if _, err := os.Stat(filepath.Join(workDir, ".gitlab-ci.yml")); err == nil {
		return "GitLab CI"
	}
	if _, err := os.Stat(filepath.Join(workDir, ".circleci", "config.yml")); err == nil {
		return "CircleCI"
	}
	if _, err := os.Stat(filepath.Join(workDir, "Jenkinsfile")); err == nil {
		return "Jenkins"
	}
	if _, err := os.Stat(filepath.Join(workDir, ".buildkite", "pipeline.yml")); err == nil {
		return "Buildkite"
	}
	if _, err := os.Stat(filepath.Join(workDir, "bitbucket-pipelines.yml")); err == nil {
		return "Bitbucket Pipelines"
	}
	if _, err := os.Stat(filepath.Join(workDir, ".travis.yml")); err == nil {
		return "Travis CI"
	}
	return ""
}

// detectDependencies reads the dependency list from package manager files.
func detectDependencies(_ context.Context, workDir string, language string, _ []string) []string {
	var deps []string

	switch language {
	case "Go":
		data, err := os.ReadFile(filepath.Join(workDir, "go.mod"))
		if err != nil {
			return nil
		}
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		inRequire := false
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "require (") {
				inRequire = true
				continue
			}
			if inRequire && line == ")" {
				break
			}
			if inRequire && line != "" {
				parts := strings.Fields(line)
				if len(parts) >= 1 {
					deps = append(deps, parts[0])
				}
			}
		}
	case "JavaScript", "TypeScript":
		data, err := os.ReadFile(filepath.Join(workDir, "package.json"))
		if err != nil {
			return nil
		}
		var pkg struct {
			Dependencies    map[string]string `json:"dependencies"`
			DevDependencies map[string]string `json:"devDependencies"`
		}
		if err := json.Unmarshal(data, &pkg); err == nil {
			for k := range pkg.Dependencies {
				deps = append(deps, k)
			}
			for k := range pkg.DevDependencies {
				deps = append(deps, k)
			}
		}
	}

	return deps
}
