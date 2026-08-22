package vscode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/editors/types"
)

func readTasks(t *testing.T, tasksFile string) vscodeTasks {
	t.Helper()
	data, err := os.ReadFile(tasksFile)
	if err != nil {
		t.Fatalf("read %s: %v", tasksFile, err)
	}
	var tasks vscodeTasks
	if err := json.Unmarshal(data, &tasks); err != nil {
		t.Fatalf("parse %s: %v", tasksFile, err)
	}
	return tasks
}

func TestSetupCreatesBootTask(t *testing.T) {
	proj := t.TempDir()
	a := NewAdapter()
	cfg := types.DefaultEditorConfig(proj)

	if err := a.Setup(cfg); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	tasks := readTasks(t, filepath.Join(proj, ".vscode", "tasks.json"))

	var boot *vscodeTask
	for i := range tasks.Tasks {
		if tasks.Tasks[i].Label == BootKernelTaskLabel {
			boot = &tasks.Tasks[i]
			break
		}
	}
	if boot == nil {
		t.Fatalf("boot task %q not found in tasks.json: %+v", BootKernelTaskLabel, tasks.Tasks)
	}
	if boot.Command != "cosca" {
		t.Errorf("boot task command = %q, want %q", boot.Command, "cosca")
	}
	if len(boot.Args) != 2 || boot.Args[0] != "kernel" || boot.Args[1] != "identity" {
		t.Errorf("boot task args = %v, want [kernel identity]", boot.Args)
	}
}

func TestSetupUsesConfiguredCoscaBinPath(t *testing.T) {
	proj := t.TempDir()
	a := NewAdapter()
	cfg := types.DefaultEditorConfig(proj)
	cfg.CoscaBinPath = "/custom/bin/cosca"

	if err := a.Setup(cfg); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	tasks := readTasks(t, filepath.Join(proj, ".vscode", "tasks.json"))
	for _, task := range tasks.Tasks {
		if task.Label == BootKernelTaskLabel && task.Command != "/custom/bin/cosca" {
			t.Errorf("boot task command = %q, want %q", task.Command, "/custom/bin/cosca")
		}
	}
}

func TestTeardownRemovesBootTask(t *testing.T) {
	proj := t.TempDir()
	a := NewAdapter()
	cfg := types.DefaultEditorConfig(proj)

	if err := a.Setup(cfg); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	// Add a user-defined task that must survive Teardown.
	tasksFile := filepath.Join(proj, ".vscode", "tasks.json")
	tasks := readTasks(t, tasksFile)
	tasks.Tasks = append(tasks.Tasks, vscodeTask{
		Label:   "build",
		Type:    "shell",
		Command: "go build ./...",
	})
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		t.Fatalf("marshal tasks.json: %v", err)
	}
	if err := os.WriteFile(tasksFile, data, 0o644); err != nil {
		t.Fatalf("write tasks.json: %v", err)
	}

	// Teardown reads .vscode/tasks.json relative to the working directory.
	t.Chdir(proj)
	if err := a.Teardown(); err != nil {
		t.Fatalf("Teardown: %v", err)
	}

	after := readTasks(t, tasksFile)
	for _, task := range after.Tasks {
		if task.Label == BootKernelTaskLabel {
			t.Errorf("boot task %q still present after Teardown: %+v", BootKernelTaskLabel, after.Tasks)
		}
		if task.Label == "Cosca: Search Codebase" || task.Label == "Cosca: Status" {
			t.Errorf("Cosca task %q still present after Teardown: %+v", task.Label, after.Tasks)
		}
	}

	// The user task must be preserved.
	found := false
	for _, task := range after.Tasks {
		if task.Label == "build" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("user task %q removed by Teardown: %+v", "build", after.Tasks)
	}
}

func TestTeardownNoTasksFileIsNoop(t *testing.T) {
	proj := t.TempDir()
	a := NewAdapter()

	t.Chdir(proj)
	if err := a.Teardown(); err != nil {
		t.Fatalf("Teardown on missing tasks.json must return nil, got: %v", err)
	}
}

func TestVersion(t *testing.T) {
	a := NewAdapter()
	v, err := a.Version()
	if err != nil {
		// A detecção depende do VS Code instalado em um dos caminhos
		// conhecidos (POSIX/macOS) ou em ~/.vscode. Em máquinas Windows ou em
		// CI sem o editor instalado a ausência é um resultado legítimo, não uma
		// falha — mesmo comportamento do TestVersion do adapter opencode.
		t.Logf("Version not available (editor not installed): %v", err)
		return
	}
	if v == "" {
		t.Error("Version should not be empty")
	}
}

func TestInfo(t *testing.T) {
	a := NewAdapter()
	info, err := a.Info()
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name == "" {
		t.Error("Info.Name should not be empty")
	}
}

func TestDetect_NotFound(t *testing.T) {
	a := NewAdapter()
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	t.Cleanup(func() { os.Chdir(origDir) })
	found, err := a.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	_ = found
}
