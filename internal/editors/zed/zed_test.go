package zed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/editors/types"
)

func TestSetupCreatesProjectContextServer(t *testing.T) {
	t.Parallel()
	proj := t.TempDir()
	zedHome := t.TempDir()

	a := NewAdapter()
	a.zedDir = zedHome // do not touch the real ~/.config/zed

	cfg := types.DefaultEditorConfig(proj)
	if err := a.Setup(cfg); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	settingsPath := zedProjectSettingsPath(proj)
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf(".zed/settings.json not created: %v", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf(".zed/settings.json is not valid JSON: %v", err)
	}

	servers, ok := settings["context_servers"].(map[string]interface{})
	if !ok {
		t.Fatalf("context_servers missing in .zed/settings.json: %s", data)
	}
	cosca, ok := servers["cosca"].(map[string]interface{})
	if !ok {
		t.Fatalf("cosca context server missing: %s", data)
	}
	if cosca["command"] != "cosca" {
		t.Errorf("cosca command = %v, want cosca", cosca["command"])
	}

	content := string(data)
	if !strings.Contains(content, "cosca") {
		t.Errorf("content must contain cosca, got: %s", content)
	}

	// The global MCP settings must still be written (to the overridden dir).
	globalPath := filepath.Join(zedHome, "settings.json")
	if _, err := os.Stat(globalPath); err != nil {
		t.Errorf("global zed settings.json not created: %v", err)
	}
}

func TestSetupProjectContextServerIdempotent(t *testing.T) {
	t.Parallel()
	proj := t.TempDir()
	a := NewAdapter()
	a.zedDir = t.TempDir()

	cfg := types.DefaultEditorConfig(proj)
	if err := a.Setup(cfg); err != nil {
		t.Fatalf("first Setup: %v", err)
	}
	if err := a.Setup(cfg); err != nil {
		t.Fatalf("second Setup: %v", err)
	}

	data, err := os.ReadFile(zedProjectSettingsPath(proj))
	if err != nil {
		t.Fatalf(".zed/settings.json missing after second Setup: %v", err)
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf(".zed/settings.json is not valid JSON: %v", err)
	}
	servers, ok := settings["context_servers"].(map[string]interface{})
	if !ok {
		t.Fatalf("context_servers missing after second Setup: %s", data)
	}
	if len(servers) != 1 {
		t.Errorf("context_servers must have exactly 1 entry, got %d: %s", len(servers), data)
	}
}

func TestVersion(t *testing.T) {
	a := NewAdapter()
	v, err := a.Version()
	if err != nil {
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
