// Package zed provides the Zed editor adapter for Cosca,
// integrating Cosca with Zed by managing ~/.config/zed/settings.json
// MCP server configuration.
package zed

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/editors/types"
)

// =============================================================================
// Adapter
// =============================================================================

// Adapter integrates Cosca with Zed by managing
// the Zed settings.json file for MCP server integration.
type Adapter struct {
	types.BaseEditor
	zedDir string
}

// NewAdapter creates a new Zed editor adapter.
func NewAdapter() *Adapter {
	homeDir, _ := homedir.Dir()
	zedDir := filepath.Join(homeDir, ".config", "zed")

	return &Adapter{
		BaseEditor: types.BaseEditor{
			NameValue: "zed",
			CapabilitiesVal: types.EditorCapabilities{
				SupportsContext:        true,
				SupportsSearch:         true,
				SupportsExecute:        false,
				SupportsWatch:          false,
				SupportsMCP:            true,
				SupportsCustomCommands: false,
				SupportsKeybindings:    true,
			},
		},
		zedDir: zedDir,
	}
}

// =============================================================================
// Config Types
// =============================================================================

// =============================================================================
// Project Boot Config
// =============================================================================

// zedProjectSettingsPath returns the project-level .zed/settings.json path.
func zedProjectSettingsPath(projectDir string) string {
	return filepath.Join(projectDir, ".zed", "settings.json")
}

// ZedContextServerSettings is the project-level .zed/settings.json content.
// Zed natively supports per-project context_servers entries, which makes any
// MCP-compatible editor read the Cosca kernel as a tool on session start.
const ZedContextServerSettings = `{
  "context_servers": {
    "cosca": { "command": "cosca", "args": ["mcp"] }
  }
}
`

// setupProjectSettings writes .zed/settings.json in the project directory.
// It is idempotent: if the cosca context server is already registered, the
// file is left untouched.
func setupProjectSettings(projectDir string) error {
	settingsPath := zedProjectSettingsPath(projectDir)

	if existing, err := os.ReadFile(settingsPath); err == nil {
		var existingSettings map[string]interface{}
		if json.Unmarshal(existing, &existingSettings) == nil {
			if servers, ok := existingSettings["context_servers"].(map[string]interface{}); ok {
				if _, exists := servers["cosca"]; exists {
					log.Info().Str("path", settingsPath).Msg("Cosca context server already in .zed/settings.json")
					return nil
				}
			}
		}
	}

	zedDir := filepath.Dir(settingsPath)
	if err := os.MkdirAll(zedDir, 0o755); err != nil {
		return fmt.Errorf("create .zed directory: %w", err)
	}

	if err := os.WriteFile(settingsPath, []byte(ZedContextServerSettings), 0o644); err != nil {
		return fmt.Errorf("write .zed/settings.json: %w", err)
	}

	log.Info().Str("path", settingsPath).Msg(".zed/settings.json created with Cosca context server")
	return nil
}

// =============================================================================
// Version
// =============================================================================

// Version returns the detected Zed version.
func (a *Adapter) Version() (string, error) {
	// Zed stores version info in channel-specific files like stable.json, nightly.json
	channels := []string{"stable.json", "nightly.json", "preview.json"}
	for _, ch := range channels {
		chPath := filepath.Join(a.zedDir, ch)
		data, err := os.ReadFile(chPath)
		if err != nil {
			continue
		}
		var chInfo struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(data, &chInfo); err == nil && chInfo.Version != "" {
			return chInfo.Version, nil
		}
	}

	// Fallback: check if binary is installed
	paths := []string{
		"/usr/bin/zed",
		"/usr/local/bin/zed",
		"/opt/homebrew/bin/zed",
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return "1.0.0", nil
		}
	}

	return "", fmt.Errorf("zed version not found")
}

// =============================================================================
// Detect
// =============================================================================

// Detect checks if Zed is installed by looking for ~/.config/zed/ directory.
func (a *Adapter) Detect() (bool, error) {
	if info, err := os.Stat(a.zedDir); err == nil && info.IsDir() {
		return true, nil
	}

	// Also check for Zed binary as fallback
	paths := []string{
		"/usr/bin/zed",
		"/usr/local/bin/zed",
		"/opt/homebrew/bin/zed",
		filepath.Join(homeDir(), ".cargo", "bin", "zed"),
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return true, nil
		}
	}

	return false, nil
}

// =============================================================================
// Setup
// =============================================================================

// Setup adds the Cosca MCP server to Zed's settings.json.
func (a *Adapter) Setup(config types.EditorConfig) error {
	settingsPath := filepath.Join(a.zedDir, "settings.json")

	// Ensure directory exists
	if err := os.MkdirAll(a.zedDir, 0o755); err != nil {
		return fmt.Errorf("create zed config dir: %w", err)
	}

	// Backup existing settings
	if config.BackupExisting {
		if _, err := os.Stat(settingsPath); err == nil {
			data, _ := os.ReadFile(settingsPath)
			backupPath := settingsPath + ".bak"
			if err := os.WriteFile(backupPath, data, 0o644); err == nil {
				log.Debug().Str("backup", backupPath).Msg("backed up zed settings.json")
			}
		}
	}

	// Read existing settings or create default
	var settings map[string]interface{}
	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			log.Warn().Err(err).Msg("existing zed settings.json is invalid, will overwrite")
			settings = make(map[string]interface{})
		}
	} else {
		settings = make(map[string]interface{})
	}

	// Get or create MCP servers section
	mcpServersRaw, exists := settings["mcp_servers"]
	var mcpServers map[string]interface{}
	if exists {
		switch v := mcpServersRaw.(type) {
		case map[string]interface{}:
			mcpServers = v
		default:
			mcpServers = make(map[string]interface{})
		}
	} else {
		mcpServers = make(map[string]interface{})
	}

	// Check if Cosca MCP server already exists
	if _, exists := mcpServers["cosca"]; exists {
		log.Info().Str("path", settingsPath).Msg("Cosca MCP server already configured in Zed")
		return nil
	}

	// Add Cosca MCP server
	coscaBin := config.CoscaBinPath
	if coscaBin == "" {
		coscaBin = "cosca"
	}

	mcpServers["cosca"] = map[string]interface{}{
		"command": coscaBin,
		"args":    []interface{}{"mcp"},
	}

	settings["mcp_servers"] = mcpServers

	// Write settings
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal zed settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		return fmt.Errorf("write zed settings.json: %w", err)
	}

	log.Info().Str("path", settingsPath).Msg("Zed configured with Cosca MCP server")

	// Boot: create the project-level .zed/settings.json so the Cosca Kernel
	// is loaded as a context server on session start.
	if err := setupProjectSettings(config.ProjectDir); err != nil {
		return err
	}

	return nil
}

// =============================================================================
// Validate
// =============================================================================

// Validate checks that Zed's settings.json contains the Cosca MCP server entry.
func (a *Adapter) Validate() error {
	settingsPath := filepath.Join(a.zedDir, "settings.json")

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("zed settings.json not found: %w", types.ErrEditorNotConfigured)
		}
		return fmt.Errorf("read zed settings.json: %w", err)
	}

	if !json.Valid(data) {
		return fmt.Errorf("zed settings.json is not valid JSON")
	}

	var settings struct {
		MCPServers map[string]interface{} `json:"mcp_servers"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("parse zed settings.json: %w", err)
	}

	if settings.MCPServers == nil {
		return fmt.Errorf("no MCP servers configured in Zed: %w", types.ErrEditorNotConfigured)
	}

	if _, exists := settings.MCPServers["cosca"]; !exists {
		return fmt.Errorf("Cosca MCP server not found in Zed settings: %w", types.ErrEditorNotConfigured)
	}

	return nil
}

// =============================================================================
// Teardown
// =============================================================================

// Teardown removes the Cosca MCP server entry from Zed's settings.json.
func (a *Adapter) Teardown() error {
	settingsPath := filepath.Join(a.zedDir, "settings.json")

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing to tear down
		}
		return fmt.Errorf("read zed settings.json: %w", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("parse zed settings.json: %w", err)
	}

	// Remove Cosca MCP server
	if mcpServersRaw, exists := settings["mcp_servers"]; exists {
		if mcpServers, ok := mcpServersRaw.(map[string]interface{}); ok {
			delete(mcpServers, "cosca")

			// If no more MCP servers, remove the field entirely
			if len(mcpServers) == 0 {
				delete(settings, "mcp_servers")
			} else {
				settings["mcp_servers"] = mcpServers
			}
		}
	}

	outData, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal zed settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, outData, 0o644); err != nil {
		return fmt.Errorf("write zed settings.json: %w", err)
	}

	log.Info().Str("path", settingsPath).Msg("Cosca MCP server removed from Zed settings")
	return nil
}

// =============================================================================
// Info
// =============================================================================

// Info returns detailed information about the Zed integration.
func (a *Adapter) Info() (types.EditorInfo, error) {
	detected := false
	version := ""

	if info, err := os.Stat(a.zedDir); err == nil && info.IsDir() {
		detected = true
		if v, err := a.Version(); err == nil {
			version = v
		}
	}

	return types.EditorInfo{
		Name:         "zed",
		Version:      version,
		Path:         a.zedDir,
		Capabilities: a.Capabilities(),
		Detected:     detected,
	}, nil
}

// homeDir returns the user's home directory.
func homeDir() string {
	if dir := os.Getenv("HOME"); dir != "" {
		return dir
	}
	dir, _ := homedir.Dir()
	return dir
}
