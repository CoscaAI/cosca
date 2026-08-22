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

type buildLanguage struct {
	name    string
	command string
	args    []string
}

// BuildTool compiles code using the project's build system. It detects the
// language from workspace files and runs the appropriate build command through
// the sandbox gate.
type BuildTool struct {
	workspace string
	sandbox   chat.Sandbox
	schema    json.RawMessage
}

// NewBuildTool creates a new BuildTool bound to the given workspace and sandbox.
func NewBuildTool(workspace string, sandbox chat.Sandbox) *BuildTool {
	return &BuildTool{
		workspace: workspace,
		sandbox:   sandbox,
		schema: mustMarshalSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"language": map[string]any{
					"type":        "string",
					"enum":        []string{"auto", "go", "node", "rust", "python"},
					"description": "Project language to build. 'auto' detects from workspace files.",
					"default":     "auto",
				},
				"timeout": map[string]any{
					"type":        "number",
					"description": "Timeout in milliseconds (default 120000)",
					"default":     120000,
				},
				"workdir": map[string]any{
					"type":        "string",
					"description": "Working directory for the build command, relative to the workspace",
				},
			},
		}),
	}
}

// Name returns the tool identifier.
func (t *BuildTool) Name() string { return "build" }

// Description returns a human-readable description of the tool.
func (t *BuildTool) Description() string {
	return "Compile/build source code. Detects project language (Go, Node, Rust, Python) from workspace files and runs the appropriate build command."
}

// Schema returns the JSON Schema describing the tool's parameters.
func (t *BuildTool) Schema() json.RawMessage { return t.schema }

// Execute detects the project language and runs the build command through the
// sandbox gate. Returns a JSON object with success, language, build_command,
// stdout, stderr, exit_code, and duration fields.
func (t *BuildTool) Execute(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	var input struct {
		Language string `json:"language"`
		Timeout  int    `json:"timeout"`
		Workdir  string `json:"workdir"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return errorResult("invalid params: " + err.Error()), nil
	}

	if input.Language == "" {
		input.Language = "auto"
	}
	if input.Timeout <= 0 {
		input.Timeout = 120000
	}

	lang, err := t.detectLanguage(input.Language)
	if err != nil {
		return errorResult(err.Error()), nil
	}

	buildCmd := t.getBuildCommand(lang)

	ctx, cancel := context.WithTimeout(ctx, time.Duration(input.Timeout)*time.Millisecond)
	defer cancel()

	start := time.Now()
	cmd := chat.Command{
		Args:    buildCmd.args,
		WorkDir: input.Workdir,
	}
	result, err := t.sandbox.Execute(ctx, cmd, chat.SandboxWorkspace)
	duration := time.Since(start)
	if err != nil {
		return errorResult("build failed: " + err.Error()), nil
	}

	success := result.ExitCode == 0
	output, err := json.Marshal(map[string]any{
		"success":       success,
		"language":      lang.name,
		"build_command": strings.Join(buildCmd.args, " "),
		"stdout":        result.Stdout,
		"stderr":        result.Stderr,
		"exit_code":     result.ExitCode,
		"duration_ms":   duration.Milliseconds(),
	})
	if err != nil {
		return errorResult("failed to marshal result: " + err.Error()), nil
	}

	return &chat.ToolResult{Output: string(output)}, nil
}

// Validate checks whether the given JSON parameters conform to the tool's schema.
func (t *BuildTool) Validate(params json.RawMessage) error {
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

// detectLanguage determines the project language. If language is "auto",
// it inspects the workspace for language-specific files.
func (t *BuildTool) detectLanguage(language string) (*buildLanguage, error) {
	if language != "auto" {
		return t.langByName(language)
	}
	return t.detectFromFiles()
}

func (t *BuildTool) langByName(name string) (*buildLanguage, error) {
	switch name {
	case "go":
		return &buildLanguage{name: "go", command: "go", args: []string{"go", "build", "./..."}}, nil
	case "node":
		return t.resolveNodeBuild(), nil
	case "rust":
		return &buildLanguage{name: "rust", command: "cargo", args: []string{"cargo", "build"}}, nil
	case "python":
		return &buildLanguage{name: "python", command: "python", args: []string{"python", "-m", "py_compile"}}, nil
	default:
		return nil, fmt.Errorf("unsupported language: %s", name)
	}
}

func (t *BuildTool) resolveNodeBuild() *buildLanguage {
	pkgPath := filepath.Join(t.workspace, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		var pkg struct {
			Scripts struct {
				Build string `json:"build"`
			} `json:"scripts"`
		}
		if json.Unmarshal(data, &pkg) == nil && pkg.Scripts.Build != "" {
			return &buildLanguage{name: "node", command: "npm", args: []string{"npm", "run", "build"}}
		}
		if _, err := os.Stat(filepath.Join(t.workspace, "tsconfig.json")); err == nil {
			return &buildLanguage{name: "node", command: "npx", args: []string{"npx", "tsc", "--noEmit"}}
		}
	}
	return &buildLanguage{name: "node", command: "npm", args: []string{"npm", "run", "build"}}
}

func (t *BuildTool) detectFromFiles() (*buildLanguage, error) {
	detectors := []struct {
		file string
		fn   func() *buildLanguage
	}{
		{"go.mod", func() *buildLanguage {
			return &buildLanguage{name: "go", command: "go", args: []string{"go", "build", "./..."}}
		}},
		{"Cargo.toml", func() *buildLanguage {
			return &buildLanguage{name: "rust", command: "cargo", args: []string{"cargo", "build"}}
		}},
		{"package.json", func() *buildLanguage { return t.resolveNodeBuild() }},
		{"pyproject.toml", func() *buildLanguage {
			return &buildLanguage{name: "python", command: "python", args: []string{"python", "-m", "py_compile"}}
		}},
		{"setup.py", func() *buildLanguage {
			return &buildLanguage{name: "python", command: "python", args: []string{"python", "-m", "py_compile"}}
		}},
		{"setup.cfg", func() *buildLanguage {
			return &buildLanguage{name: "python", command: "python", args: []string{"python", "-m", "py_compile"}}
		}},
	}
	for _, d := range detectors {
		if _, err := os.Stat(filepath.Join(t.workspace, d.file)); err == nil {
			return d.fn(), nil
		}
	}
	return nil, errors.New("build: could not detect project language — no go.mod, Cargo.toml, package.json, pyproject.toml, setup.py, or setup.cfg found")
}

func (t *BuildTool) getBuildCommand(lang *buildLanguage) *buildLanguage {
	return lang
}
