package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
)

type testLanguage struct {
	name    string
	command string
	args    []string
}

// TestTool runs tests using the project's test framework. It detects the
// language from workspace files and executes the appropriate test command
// through the sandbox gate.
type TestTool struct {
	workspace string
	sandbox   chat.Sandbox
	schema    json.RawMessage
}

// NewTestTool creates a new TestTool bound to the given workspace and sandbox.
func NewTestTool(workspace string, sandbox chat.Sandbox) *TestTool {
	return &TestTool{
		workspace: workspace,
		sandbox:   sandbox,
		schema: mustMarshalSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"language": map[string]any{
					"type":        "string",
					"enum":        []string{"auto", "go", "node", "rust", "python"},
					"description": "Project language to test. 'auto' detects from workspace files.",
					"default":     "auto",
				},
				"timeout": map[string]any{
					"type":        "number",
					"description": "Timeout in milliseconds (default 300000)",
					"default":     300000,
				},
				"workdir": map[string]any{
					"type":        "string",
					"description": "Working directory for the test command, relative to the workspace",
				},
				"filter": map[string]any{
					"type":        "string",
					"description": "Test filter pattern (e.g. package name, test regex)",
				},
			},
		}),
	}
}

// Name returns the tool identifier.
func (t *TestTool) Name() string { return "test" }

// Description returns a human-readable description of the tool.
func (t *TestTool) Description() string {
	return "Run tests. Detects project language (Go, Node, Rust, Python) from workspace files and runs the appropriate test command."
}

// Schema returns the JSON Schema describing the tool's parameters.
func (t *TestTool) Schema() json.RawMessage { return t.schema }

// Execute detects the project language and runs the test command through the
// sandbox gate. Returns a JSON object with success, language, test_command,
// stdout, stderr, exit_code, pass_count, fail_count, and duration fields.
func (t *TestTool) Execute(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	var input struct {
		Language string `json:"language"`
		Timeout  int    `json:"timeout"`
		Workdir  string `json:"workdir"`
		Filter   string `json:"filter"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return errorResult("invalid params: " + err.Error()), nil
	}

	if input.Language == "" {
		input.Language = "auto"
	}
	if input.Timeout <= 0 {
		input.Timeout = 300000
	}

	lang, err := t.detectLanguage(input.Language)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	args := make([]string, len(lang.args))
	copy(args, lang.args)
	if input.Filter != "" {
		args = append(args, input.Filter)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(input.Timeout)*time.Millisecond)
	defer cancel()

	start := time.Now()
	cmd := chat.Command{
		Args:    args,
		WorkDir: input.Workdir,
	}
	result, err := t.sandbox.Execute(ctx, cmd, chat.SandboxWorkspace)
	duration := time.Since(start)
	if err != nil {
		return errorResult("test failed: " + err.Error()), nil
	}

	success := result.ExitCode == 0
	passCount, failCount := t.parseTestResults(lang.name, result.Stdout, result.Stderr)

	output, err := json.Marshal(map[string]any{
		"success":      success,
		"language":     lang.name,
		"test_command": strings.Join(args, " "),
		"stdout":       result.Stdout,
		"stderr":       result.Stderr,
		"exit_code":    result.ExitCode,
		"pass_count":   passCount,
		"fail_count":   failCount,
		"duration_ms":  duration.Milliseconds(),
	})
	if err != nil {
		return errorResult("failed to marshal result: " + err.Error()), nil
	}

	return &chat.ToolResult{Output: string(output)}, nil
}

// Validate checks whether the given JSON parameters conform to the tool's schema.
func (t *TestTool) Validate(params json.RawMessage) error {
	var input struct {
		Language string `json:"language"`
		Timeout  int    `json:"timeout"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return err
	}
	if input.Language != "" && input.Language != "auto" &&
		input.Language != "go" && input.Language != "node" &&
		input.Language != "rust" && input.Language != "python" {
		return errors.New("language must be one of: auto, go, node, rust, python")
	}
	if input.Timeout < 0 {
		return errors.New("timeout must be non-negative")
	}
	return nil
}

func (t *TestTool) detectLanguage(language string) (*testLanguage, error) {
	if language != "auto" {
		return t.langByName(language)
	}
	return t.detectFromFiles()
}

func (t *TestTool) langByName(name string) (*testLanguage, error) {
	switch name {
	case "go":
		return &testLanguage{name: "go", command: "go", args: []string{"go", "test", "./...", "-race", "-count=1"}}, nil
	case "node":
		return t.resolveNodeTest(), nil
	case "rust":
		return &testLanguage{name: "rust", command: "cargo", args: []string{"cargo", "test"}}, nil
	case "python":
		return &testLanguage{name: "python", command: "python", args: []string{"python", "-m", "pytest"}}, nil
	default:
		return nil, fmt.Errorf("unsupported language: %s", name)
	}
}

func (t *TestTool) resolveNodeTest() *testLanguage {
	pkgPath := filepath.Join(t.workspace, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		var pkg struct {
			Scripts struct {
				Test string `json:"test"`
			} `json:"scripts"`
		}
		if json.Unmarshal(data, &pkg) == nil && pkg.Scripts.Test != "" {
			return &testLanguage{name: "node", command: "npm", args: []string{"npm", "test"}}
		}
		if _, err := os.Stat(filepath.Join(t.workspace, "vitest.config.ts")); err == nil {
			return &testLanguage{name: "node", command: "npx", args: []string{"npx", "vitest", "run"}}
		}
		if _, err := os.Stat(filepath.Join(t.workspace, "vitest.config.js")); err == nil {
			return &testLanguage{name: "node", command: "npx", args: []string{"npx", "vitest", "run"}}
		}
	}
	return &testLanguage{name: "node", command: "npm", args: []string{"npm", "test"}}
}

func (t *TestTool) detectFromFiles() (*testLanguage, error) {
	detectors := []struct {
		file string
		fn   func() *testLanguage
	}{
		{"go.mod", func() *testLanguage {
			return &testLanguage{name: "go", command: "go", args: []string{"go", "test", "./...", "-race", "-count=1"}}
		}},
		{"Cargo.toml", func() *testLanguage {
			return &testLanguage{name: "rust", command: "cargo", args: []string{"cargo", "test"}}
		}},
		{"package.json", func() *testLanguage { return t.resolveNodeTest() }},
		{"pyproject.toml", func() *testLanguage {
			return &testLanguage{name: "python", command: "python", args: []string{"python", "-m", "pytest"}}
		}},
		{"setup.py", func() *testLanguage {
			return &testLanguage{name: "python", command: "python", args: []string{"python", "-m", "pytest"}}
		}},
		{"setup.cfg", func() *testLanguage {
			return &testLanguage{name: "python", command: "python", args: []string{"python", "-m", "pytest"}}
		}},
	}
	for _, d := range detectors {
		if _, err := os.Stat(filepath.Join(t.workspace, d.file)); err == nil {
			return d.fn(), nil
		}
	}
	return nil, errors.New("test: could not detect project language — no go.mod, Cargo.toml, package.json, pyproject.toml, setup.py, or setup.cfg found")
}

func (t *TestTool) parseTestResults(language, stdout, stderr string) (passCount, failCount int) {
	combined := stdout + "\n" + stderr
	switch language {
	case "go":
		return parseGoTestResults(combined)
	case "rust":
		return parseRustTestResults(combined)
	case "python":
		return parsePytestResults(combined)
	default:
	}
	if passCount == 0 && failCount == 0 {
		lower := strings.ToLower(combined)
		failCount = strings.Count(lower, "fail")
		passCount = strings.Count(lower, "pass")
	}
	return
}

func parseGoTestResults(output string) (pass, fail int) {
	return parsePassFailLines(output, "ok", "FAIL")
}

func parseRustTestResults(output string) (pass, fail int) {
	return parsePassFailLines(output, "test result: ok", "test result: FAILED")
}

func parsePytestResults(output string) (pass, fail int) {
	for _, line := range strings.Split(output, "\n") {
		// Independent checks, not a switch: pytest's summary line reports both
		// counters together ("4 passed, 1 failed in 1.2s"), so a mutually
		// exclusive switch would swallow the failure count entirely.
		if strings.Contains(line, " passed") {
			if v := extractInt(line, " passed"); v > 0 {
				pass = v
			}
		}
		if strings.Contains(line, " failed") {
			if v := extractInt(line, " failed"); v > 0 {
				fail = v
			}
		}
	}
	return
}

func parsePassFailLines(output, passMarker, failMarker string) (pass, fail int) {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, passMarker) {
			pass++
		} else if strings.HasPrefix(trimmed, failMarker) {
			if !strings.Contains(trimmed, passMarker) {
				fail++
			}
		}
	}
	return
}

func extractInt(line, suffix string) int {
	idx := strings.Index(line, suffix)
	if idx == -1 {
		return 0
	}
	before := strings.TrimSpace(line[:idx])
	if before == "" {
		return 0
	}
	fields := strings.Fields(before)
	if len(fields) == 0 {
		return 0
	}
	val := 0
	fmt.Sscanf(fields[len(fields)-1], "%d", &val)
	return val
}
