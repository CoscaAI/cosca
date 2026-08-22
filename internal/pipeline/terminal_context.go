package pipeline

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// TerminalContext wraps SessionContext with terminal-specific tracking.
type TerminalContext struct {
	*SessionContext

	ProjectPath    string
	ProjectType    string
	LastSessionID  string
	ActiveAgent    string
	ActiveModel    string
	AgentRouting   []RoutingDecision
	SessionState   string
	DetectedFiles  []string
	CostEstimate   CostEstimate
	Project        *ProjectInfo
}

// ProjectInfo describes the detected project at the working directory.
type ProjectInfo struct {
	Language       string
	Framework      string
	PackageManager string
	Database       string
	TestFramework  string
	CITool         string
	Containerized  bool
	ConfigFiles    []string
	Dependencies   int
	GitRepo        bool
	Branch         string
	KnownIssues    []string
}

// RoutingDecision records an agent routing event.
type RoutingDecision struct {
	Timestamp time.Time `json:"timestamp"`
	Prompt    string    `json:"prompt"`
	Agent     string    `json:"agent"`
	Reason    string    `json:"reason"`
}

// CostEstimate tracks estimated session costs.
type CostEstimate struct {
	TotalInputTokens  int     `json:"total_input_tokens"`
	TotalOutputTokens int     `json:"total_output_tokens"`
	EstimatedCostUSD  float64 `json:"estimated_cost_usd"`
}

// NewTerminalContext creates a new terminal session context.
func NewTerminalContext(sessionID string) *TerminalContext {
	plan := &Plan{
		ID:        sessionID,
		CreatedAt: time.Now().UTC(),
	}
	return &TerminalContext{
		SessionContext: NewSessionContext(plan),
		SessionState:   "ready",
		AgentRouting:   make([]RoutingDecision, 0),
	}
}

// ─── Config detection ─────────────────────────────────────────────────────

type configEntry struct {
	file     string
	language string
}

var languageConfigs = []struct {
	file     string
	language string
}{
	{"go.mod", "go"},
	{"go.sum", "go"},
	{"Cargo.toml", "rust"},
	{"CMakeLists.txt", "c"},
	{"package.json", "typescript"},
	{"requirements.txt", "python"},
	{"pyproject.toml", "python"},
	{"setup.py", "python"},
	{"Pipfile", "python"},
	{"Gemfile", "ruby"},
	{"pom.xml", "java"},
	{"build.gradle", "java"},
	{"build.gradle.kts", "java"},
	{"Makefile", "c"},
}

var frameworkConfigs = map[string][]string{
	"next.config.ts":       {"typescript", "next.js"},
	"next.config.js":       {"typescript", "next.js"},
	"next.config.mjs":      {"typescript", "next.js"},
	"nuxt.config.ts":       {"typescript", "nuxt"},
	"nuxt.config.js":       {"typescript", "nuxt"},
	"svelte.config.js":     {"typescript", "svelte"},
	"remix.config.js":      {"typescript", "remix"},
	"vite.config.ts":       {"typescript", "vite"},
	"vite.config.js":       {"typescript", "vite"},
	"turbo.json":           {"typescript", "turborepo"},
	"angular.json":         {"typescript", "angular"},
	"tsconfig.json":        {"typescript", ""},
	"manage.py":            {"python", "django"},
	"django":               {"python", "django"},
	"app.py":               {"python", "flask"},
	"alembic.ini":          {"python", "sqlalchemy"},
	"Cargo.lock":           {"rust", "cargo"},
	"go.sum":               {"go", ""},
	"buf.gen.yaml":         {"go", "grpc"},
	"Taskfile.yml":         {"go", "task"},
	"Taskfile.yaml":        {"go", "task"},
	"deno.json":            {"typescript", "deno"},
	"deno.jsonc":           {"typescript", "deno"},
	"Bun.lockb":            {"typescript", "bun"},
}

var packageManagers = map[string]string{
	"go.mod":               "go modules",
	"go.sum":               "go modules",
	"package.json":         "npm",
	"package-lock.json":    "npm",
	"yarn.lock":            "yarn",
	"pnpm-lock.yaml":       "pnpm",
	"bun.lockb":            "bun",
	"Cargo.toml":           "cargo",
	"Cargo.lock":           "cargo",
	"requirements.txt":     "pip",
	"pyproject.toml":       "pip",
	"Pipfile":              "pipenv",
	"pipenv":               "pipenv",
	"poetry.lock":          "poetry",
	"Gemfile":              "bundler",
	"pom.xml":              "maven",
	"build.gradle":         "gradle",
	"build.gradle.kts":     "gradle",
}

var databaseConfigs = []string{
	"prisma/schema.prisma",
	"drizzle.config.ts",
	"drizzle.config.js",
	"knexfile.js",
	"knexfile.ts",
	"sequelizerc",
	"typeorm.config.ts",
	"ormconfig.json",
	"mikro-orm.config.ts",
	"alembic.ini",
	"sqlx-data.json",
}

var testConfigs = map[string]string{
	"jest.config.ts":      "jest",
	"jest.config.js":      "jest",
	"vitest.config.ts":    "vitest",
	"vitest.config.js":    "vitest",
	"playwright.config.ts": "playwright",
	"cypress.config.ts":   "cypress",
	"cypress.config.js":   "cypress",
	"pytest.ini":          "pytest",
	"pyproject.toml":      "pytest",
	"tox.ini":             "tox",
	"conftest.py":         "pytest",
}

var ciConfigs = map[string]string{
	".github/workflows": "github actions",
	".gitlab-ci.yml":    "gitlab ci",
	"Jenkinsfile":       "jenkins",
	".circleci":         "circleci",
	".travis.yml":       "travis ci",
	"bitbucket-pipelines.yml": "bitbucket pipelines",
	"buildkite":              "buildkite",
}

var containerFiles = []string{
	"Dockerfile",
	"docker-compose.yml",
	"docker-compose.yaml",
	".dockerignore",
	"Containerfile",
}

var lockFilesForDeps = map[string]string{
	"go.sum":              "go",
	"package-lock.json":   "node",
	"yarn.lock":           "node",
	"pnpm-lock.yaml":      "node",
	"bun.lockb":           "node",
	"Cargo.lock":          "rust",
	"Gemfile.lock":        "ruby",
	"poetry.lock":         "python",
	"Pipfile.lock":        "python",
	"requirements.txt":    "python",
}

// DetectProject scans the working directory to detect project type.
func (tc *TerminalContext) DetectProject() *ProjectInfo {
	if tc.ProjectPath == "" {
		return nil
	}

	info := &ProjectInfo{
		Language: "unknown",
	}

	var configFiles []string
	var detectedLang string

	for _, lc := range languageConfigs {
		path := filepath.Join(tc.ProjectPath, lc.file)
		if _, err := os.Stat(path); err == nil {
			configFiles = append(configFiles, lc.file)
			if detectedLang == "" {
				detectedLang = lc.language
				info.Language = lc.language
			}
		}
	}

	for file, frames := range frameworkConfigs {
		path := filepath.Join(tc.ProjectPath, file)
		dirPath := filepath.Join(tc.ProjectPath, file)
		if _, err := os.Stat(path); err == nil {
			configFiles = append(configFiles, file)
			lang := frames[0]
			if info.Language == "" || info.Language == "unknown" {
				info.Language = lang
			}
			if len(frames) > 1 && frames[1] != "" {
				info.Framework = frames[1]
			}
		} else if info2, err2 := os.Stat(dirPath); err2 == nil && info2.IsDir() {
			configFiles = append(configFiles, file)
			if len(frames) > 1 && frames[1] != "" {
				info.Framework = frames[1]
			}
		}
	}

	for _, f := range configFiles {
		if pm, ok := packageManagers[f]; ok {
			info.PackageManager = pm
			break
		}
	}

	for _, dbConfig := range databaseConfigs {
		path := filepath.Join(tc.ProjectPath, dbConfig)
		if _, err := os.Stat(path); err == nil {
			info.Database = detectDatabaseFromConfig(dbConfig)
			configFiles = append(configFiles, dbConfig)
			break
		}
	}

	for cfgFile, framework := range testConfigs {
		path := filepath.Join(tc.ProjectPath, cfgFile)
		if _, err := os.Stat(path); err == nil {
			info.TestFramework = framework
			configFiles = append(configFiles, cfgFile)
			break
		}
	}
	if info.TestFramework == "" {
		if info2, err := os.Stat(filepath.Join(tc.ProjectPath, "tests")); err == nil && info2.IsDir() {
			info.TestFramework = detectTestFromLang(detectedLang)
		} else if info2, err := os.Stat(filepath.Join(tc.ProjectPath, "__tests__")); err == nil && info2.IsDir() {
			info.TestFramework = detectTestFromLang(detectedLang)
		} else if info2, err := os.Stat(filepath.Join(tc.ProjectPath, "spec")); err == nil && info2.IsDir() {
			info.TestFramework = detectTestFromLang(detectedLang)
		}
	}

	for cfgPath, ciName := range ciConfigs {
		path := filepath.Join(tc.ProjectPath, cfgPath)
		if info2, err := os.Stat(path); err == nil && (info2.IsDir() || info2.Mode().IsRegular()) {
			info.CITool = ciName
			configFiles = append(configFiles, cfgPath)
			break
		}
	}

	for _, containerFile := range containerFiles {
		path := filepath.Join(tc.ProjectPath, containerFile)
		if _, err := os.Stat(path); err == nil {
			info.Containerized = true
			configFiles = append(configFiles, containerFile)
			break
		}
	}

	for lockFile := range lockFilesForDeps {
		path := filepath.Join(tc.ProjectPath, lockFile)
		if _, err := os.Stat(path); err == nil {
			info.Dependencies = countDepsFromFile(path, lockFile)
			configFiles = append(configFiles, lockFile)
			break
		}
	}

	info.ConfigFiles = dedupeStrings(configFiles)

	if info2, err := os.Stat(filepath.Join(tc.ProjectPath, ".git")); err == nil && info2.IsDir() {
		info.GitRepo = true
		info.Branch = gitCurrentBranch(tc.ProjectPath)
	}

	tc.Project = info
	tc.ProjectType = info.Language
	tc.DetectedFiles = info.ConfigFiles

	return info
}

func detectDatabaseFromConfig(dbConfig string) string {
	switch {
	case strings.Contains(dbConfig, "prisma") || strings.Contains(dbConfig, "drizzle"):
		return "sqlite/postgres/mysql (orm-managed)"
	case strings.Contains(dbConfig, "alembic"):
		return "postgres"
	case strings.Contains(dbConfig, "sqlx"):
		return "postgres/mysql"
	default:
		return "unknown"
	}
}

func detectTestFromLang(lang string) string {
	switch lang {
	case "go":
		return "go test"
	case "rust":
		return "cargo test"
	case "python":
		return "pytest"
	case "typescript", "javascript", "node":
		return "jest"
	default:
		return ""
	}
}

func dedupeStrings(items []string) []string {
	seen := make(map[string]bool, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

func countDepsFromFile(path, filename string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	content := string(data)
	lines := strings.Split(content, "\n")

	switch filename {
	case "go.sum":
		return len(lines)
	case "package-lock.json", "yarn.lock", "pnpm-lock.yaml":
		return depCountFromLines(lines)
	case "Cargo.lock":
		count := 0
		for _, line := range lines {
			if strings.HasPrefix(line, "[[package]]") {
				count++
			}
		}
		return count
	case "requirements.txt":
		count := 0
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "-") {
				count++
			}
		}
		return count
	case "Gemfile.lock":
		return depCountFromLines(lines)
	default:
		return depCountFromLines(lines)
	}
}

func depCountFromLines(lines []string) int {
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func gitCurrentBranch(path string) string {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// VerifyDependencies checks for essential tools and returns missing ones.
func VerifyDependencies() []string {
	var missing []string
	tools := []struct {
		name    string
		install string
	}{
		{"git", "sudo apt install git"},
		{"go", "https://go.dev/dl/"},
		{"ollama", "https://ollama.ai"},
		{"bubblewrap", "sudo apt install bubblewrap"},
		{"gcc", "sudo apt install build-essential"},
	}
	for _, tool := range tools {
		if _, err := exec.LookPath(tool.name); err != nil {
			missing = append(missing, tool.name+": "+tool.install)
		}
	}
	return missing
}

func contains(items []string, target string) bool {
	for _, i := range items {
		if i == target {
			return true
		}
	}
	return false
}

// RouteAgent records an agent routing decision.
func (tc *TerminalContext) RouteAgent(prompt, agent, reason string) {
	tc.ActiveAgent = agent
	tc.AgentRouting = append(tc.AgentRouting, RoutingDecision{
		Timestamp: time.Now(),
		Prompt:    truncatePrompt(prompt, 100),
		Agent:     agent,
		Reason:    reason,
	})
}

// UpdateCost tracks token usage and updates cost estimate.
func (tc *TerminalContext) UpdateCost(inputTokens, outputTokens int) {
	tc.AddTokens(inputTokens, outputTokens)
	tc.CostEstimate.TotalInputTokens += inputTokens
	tc.CostEstimate.TotalOutputTokens += outputTokens
	tc.CostEstimate.EstimatedCostUSD = estimateCostUSD(
		tc.CostEstimate.TotalInputTokens,
		tc.CostEstimate.TotalOutputTokens,
		tc.ActiveModel,
	)
}

func estimateCostUSD(input, output int, model string) float64 {
	inputRate := 0.0
	outputRate := 0.0
	switch {
	case strings.Contains(model, "gpt-4"):
		inputRate = 0.00003
		outputRate = 0.00006
	case strings.Contains(model, "claude"):
		inputRate = 0.000015
		outputRate = 0.000075
	default:
		inputRate = 0.000001
		outputRate = 0.000002
	}
	return (float64(input)*inputRate + float64(output)*outputRate) / 1000
}

// Snapshot returns a full copy of the terminal context.
func (tc *TerminalContext) Snapshot() TerminalContext {
	return *tc
}

// Resume restores state from a previous session.
func (tc *TerminalContext) Resume(lastSessionID string) {
	tc.LastSessionID = lastSessionID
}

// ContextSummary returns a formatted summary of the terminal context.
func (tc *TerminalContext) ContextSummary() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Session: %s | State: %s\n", tc.SessionID, tc.SessionState))
	b.WriteString(fmt.Sprintf("Project: %s (%s)\n", tc.ProjectType, tc.ProjectPath))
	if tc.ActiveAgent != "" {
		b.WriteString(fmt.Sprintf("Agent: %s\n", tc.ActiveAgent))
	}
	if tc.ActiveModel != "" {
		b.WriteString(fmt.Sprintf("Model: %s\n", tc.ActiveModel))
	}
	b.WriteString(fmt.Sprintf("Tasks: %d done, %d failed\n",
		tc.Metrics.TasksCompleted, tc.Metrics.TasksFailed))
	b.WriteString(fmt.Sprintf("Tokens: %d in / %d out | Cost: $%.4f\n",
		tc.CostEstimate.TotalInputTokens, tc.CostEstimate.TotalOutputTokens,
		tc.CostEstimate.EstimatedCostUSD))
	b.WriteString(fmt.Sprintf("Routing decisions: %d\n", len(tc.AgentRouting)))
	return b.String()
}

func truncatePrompt(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
