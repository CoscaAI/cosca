// Package vscode provides the VS Code editor adapter for Cosca,
// integrating Cosca with Visual Studio Code by creating .vscode/tasks.json
// and related configuration files.
package vscode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/editors/types"
	"github.com/CoscaAI/cosca/internal/safe"
)

// =============================================================================
// Adapter
// =============================================================================

// Adapter integrates Cosca with Visual Studio Code by managing
// .vscode/tasks.json and workspace configuration.
type Adapter struct {
	types.BaseEditor
}

// NewAdapter creates a new VS Code editor adapter.
func NewAdapter() *Adapter {
	return &Adapter{
		BaseEditor: types.BaseEditor{
			NameValue: "vscode",
			CapabilitiesVal: types.EditorCapabilities{
				SupportsContext:        true,
				SupportsSearch:         true,
				SupportsExecute:        true,
				SupportsWatch:          true,
				SupportsMCP:            false,
				SupportsCustomCommands: true,
				SupportsKeybindings:    true,
			},
		},
	}
}

// vscodeTask represents a single VS Code task.
type vscodeTask struct {
	Label          string                 `json:"label"`
	Type           string                 `json:"type"`
	Command        string                 `json:"command,omitempty"`
	Args           []string               `json:"args,omitempty"`
	Presentation   map[string]interface{} `json:"presentation,omitempty"`
	Group          interface{}            `json:"group,omitempty"`
	Detail         string                 `json:"detail,omitempty"`
	ProblemMatcher string                 `json:"problemMatcher,omitempty"`
}

// vscodeTasks represents the VS Code tasks.json structure.
type vscodeTasks struct {
	Version string       `json:"version"`
	Tasks   []vscodeTask `json:"tasks"`
}

// BootKernelTaskLabel is the label of the task that boots the Cosca Kernel.
// It intentionally uses the lowercase "cosca:" prefix to match the
// canonical boot task convention shared across editors.
const BootKernelTaskLabel = "cosca: boot kernel"

// Version returns the detected VS Code version.
func (a *Adapter) Version() (string, error) {
	paths := []string{
		"/usr/share/code",
		"/usr/local/share/code",
		"/Applications/Visual Studio Code.app",
		filepath.Join(homeDir(), ".vscode"),
	}

	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return "1.0", nil
		}
	}

	return "", fmt.Errorf("VS Code not found")
}

// Detect checks if VS Code is available.
func (a *Adapter) Detect() (bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("get working dir: %w", err)
	}

	dir := cwd
	for i := 0; i < 5; i++ {
		vscodeDir := filepath.Join(dir, ".vscode")
		if info, err := os.Stat(vscodeDir); err == nil && info.IsDir() {
			return true, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	commonPaths := []string{
		"/usr/bin/code",
		"/usr/local/bin/code",
		"/snap/bin/code",
		filepath.Join(homeDir(), "bin", "code"),
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return true, nil
		}
	}

	return false, nil
}

// Setup creates .vscode/tasks.json with Cosca commands.
func (a *Adapter) Setup(config types.EditorConfig) error {
	vscodeDir := filepath.Join(config.ProjectDir, ".vscode")
	tasksFile := filepath.Join(vscodeDir, "tasks.json")

	if err := os.MkdirAll(vscodeDir, 0o755); err != nil {
		return fmt.Errorf("create .vscode directory: %w", err)
	}

	tasks := vscodeTasks{
		Version: "2.0.0",
	}
	if data, err := os.ReadFile(tasksFile); err == nil {
		if err := json.Unmarshal(data, &tasks); err != nil {
			log.Warn().Err(err).Msg("invalid tasks.json, creating new")
			tasks = vscodeTasks{Version: "2.0.0"}
		}
	}

	coscaTasks := []vscodeTask{
		{
			Label: "cosca: boot kernel",
			Type:  "shell",
			Command: func() string {
				if config.CoscaBinPath != "" {
					return config.CoscaBinPath
				}
				return "cosca"
			}(),
			Args:   []string{"kernel", "identity"},
			Group:  "none",
			Detail: "Boot the Cosca Kernel (consigliere do Don) — identity",
			Presentation: map[string]interface{}{
				"echo":   false,
				"reveal": "silent",
				"focus":  false,
				"panel":  "shared",
			},
		},
		{
			Label:   "Cosca: Search Codebase",
			Type:    "shell",
			Command: "cosca search \"${input:query}\"",
			Group:   "none",
			Detail:  "Search the codebase using Cosca semantic search",
			Presentation: map[string]interface{}{
				"echo":   true,
				"reveal": "always",
				"focus":  false,
				"panel":  "shared",
			},
		},
		{
			Label:   "Cosca: Index Codebase",
			Type:    "shell",
			Command: "cosca index",
			Group:   "none",
			Detail:  "Index the codebase with Cosca for semantic search",
			Presentation: map[string]interface{}{
				"echo":   true,
				"reveal": "always",
				"focus":  true,
				"panel":  "dedicated",
			},
		},
		{
			Label:   "Cosca: Get Context",
			Type:    "shell",
			Command: "cosca context ${file}",
			Group:   "none",
			Detail:  "Get Cosca context for the current file",
			Presentation: map[string]interface{}{
				"echo":   true,
				"reveal": "always",
				"focus":  false,
				"panel":  "shared",
			},
		},
		{
			Label:   "Cosca: Status",
			Type:    "shell",
			Command: "cosca status",
			Group:   "none",
			Detail:  "Check Cosca system status",
		},
	}

	existingLabels := make(map[string]bool)
	for _, t := range tasks.Tasks {
		existingLabels[t.Label] = true
	}

	for _, t := range coscaTasks {
		if !existingLabels[t.Label] {
			tasks.Tasks = append(tasks.Tasks, t)
		}
	}

	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal tasks.json: %w", err)
	}

	if err := os.WriteFile(tasksFile, data, 0o644); err != nil {
		return fmt.Errorf("write tasks.json: %w", err)
	}

	extFile := filepath.Join(vscodeDir, "extensions.json")
	if _, err := os.Stat(extFile); os.IsNotExist(err) {
		extData, _ := json.MarshalIndent(map[string]interface{}{
			"recommendations": []string{},
		}, "", "  ")
		safe.WriteFile(extFile, extData, 0o644)
	}

	log.Info().Str("path", tasksFile).Int("tasks", len(coscaTasks)).
		Msg(".vscode/tasks.json configured with Cosca commands")
	return nil
}

// Validate checks that .vscode/tasks.json contains Cosca tasks.
func (a *Adapter) Validate() error {
	tasksFile := filepath.Join(".vscode", "tasks.json")

	data, err := os.ReadFile(tasksFile)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(".vscode/tasks.json not found: %w", types.ErrEditorNotConfigured)
		}
		return fmt.Errorf("read tasks.json: %w", err)
	}

	var tasks vscodeTasks
	if err := json.Unmarshal(data, &tasks); err != nil {
		return fmt.Errorf("parse tasks.json: %w", err)
	}

	for _, t := range tasks.Tasks {
		if strings.HasPrefix(t.Label, "Cosca:") {
			return nil
		}
	}

	return fmt.Errorf("Cosca tasks not found in .vscode/tasks.json: %w",
		types.ErrEditorNotConfigured)
}

// Teardown removes Cosca tasks from .vscode/tasks.json.
func (a *Adapter) Teardown() error {
	tasksFile := filepath.Join(".vscode", "tasks.json")

	data, err := os.ReadFile(tasksFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read tasks.json: %w", err)
	}

	var tasks vscodeTasks
	if err := json.Unmarshal(data, &tasks); err != nil {
		return fmt.Errorf("parse tasks.json: %w", err)
	}

	filtered := make([]vscodeTask, 0)
	for _, t := range tasks.Tasks {
		// Remove every "Cosca:" task plus the canonical boot task. The boot
		// task uses the lowercase "cosca: boot kernel" label (BootKernelTaskLabel),
		// so a plain "Cosca:" prefix filter would leave it behind.
		if t.Label == BootKernelTaskLabel || strings.HasPrefix(t.Label, "Cosca:") {
			continue
		}
		filtered = append(filtered, t)
	}
	tasks.Tasks = filtered

	outData, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal tasks.json: %w", err)
	}

	if err := os.WriteFile(tasksFile, outData, 0o644); err != nil {
		return fmt.Errorf("write tasks.json: %w", err)
	}

	log.Info().Str("path", tasksFile).Msg("Cosca tasks removed from .vscode/tasks.json")
	return nil
}

// Info returns information about the VS Code integration.
func (a *Adapter) Info() (types.EditorInfo, error) {
	detected := false
	version := ""

	if v, err := a.Version(); err == nil {
		version = v
	}

	vscodeDir := filepath.Join(".", ".vscode")
	if info, err := os.Stat(vscodeDir); err == nil && info.IsDir() {
		detected = true
	}

	return types.EditorInfo{
		Name:         "vscode",
		Version:      version,
		Path:         vscodeDir,
		Capabilities: a.Capabilities(),
		Detected:     detected,
	}, nil
}

// homeDir returns the user's home directory.
func homeDir() string {
	if dir := os.Getenv("HOME"); dir != "" {
		return dir
	}
	return "~"
}
