// Package windsurf provides the Windsurf (Codeium) editor adapter for Cosca,
// integrating Cosca with Windsurf by managing ~/.windsurf/config.json
// MCP server configuration.
package windsurf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/editors/types"
	embedcosca "github.com/CoscaAI/cosca/internal/embed/cosca"
)

// =============================================================================
// Adapter
// =============================================================================

// Adapter integrates Cosca with Windsurf by managing
// the Windsurf configuration file for MCP server integration.
type Adapter struct {
	types.BaseEditor
	windsurfDir string
}

// NewAdapter creates a new Windsurf editor adapter.
func NewAdapter() *Adapter {
	homeDir, _ := homedir.Dir()
	windsurfDir := filepath.Join(homeDir, ".windsurf")

	return &Adapter{
		BaseEditor: types.BaseEditor{
			NameValue: "windsurf",
			CapabilitiesVal: types.EditorCapabilities{
				SupportsContext:        true,
				SupportsSearch:         true,
				SupportsExecute:        false,
				SupportsWatch:          false,
				SupportsMCP:            true,
				SupportsCustomCommands: false,
				SupportsKeybindings:    false,
			},
		},
		windsurfDir: windsurfDir,
	}
}

// =============================================================================
// Config Types
// =============================================================================

// windsurfConfig represents the Windsurf config.json structure.
type windsurfConfig struct {
	MCPServers map[string]mcpServerConfig `json:"mcpServers,omitempty"`
	Version    string                     `json:"version,omitempty"`
}

// mcpServerConfig represents an MCP server configuration entry.
type mcpServerConfig struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

// WindsurfRulesFileName is the project-level rules file Windsurf reads
// at session start (native .windsurfrules support).
const WindsurfRulesFileName = ".windsurfrules"

// WindsurfRulesContentSelf instructs the Windsurf agent to boot the Cosca
// Kernel from on-disk framework files (Cosca self-project).
const WindsurfRulesContentSelf = `AO INICIAR: carregue .cosca/framework/KERNEL.md.
Cosca Kernel = consigliere do Don. Orquestra, não implementa.
`

// WindsurfRulesContentThirdParty instructs the Windsurf agent to boot the Cosca
// Kernel in a self-contained way for third-party projects (no on-disk framework).
const WindsurfRulesContentThirdParty = `AO INICIAR: carregue o Cosca Kernel.
Cosca Kernel = consigliere do Don. Orquestra, não implementa.
Banco: .cosca/knowledge.db | Memória: .cosca/memory/
`

// =============================================================================
// Version
// =============================================================================

// Version returns the detected Windsurf version.
func (a *Adapter) Version() (string, error) {
	cfgPath := filepath.Join(a.windsurfDir, "config.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return "", fmt.Errorf("read windsurf config: %w", err)
	}

	var cfg windsurfConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parse windsurf config: %w", err)
	}

	if cfg.Version == "" {
		return "1.0.0", nil
	}
	return cfg.Version, nil
}

// =============================================================================
// Detect
// =============================================================================

// Detect checks if Windsurf is installed by looking for ~/.windsurf/
// or a .windsurf/ directory in the project tree.
func (a *Adapter) Detect() (bool, error) {
	// Check global config directory
	if info, err := os.Stat(a.windsurfDir); err == nil && info.IsDir() {
		return true, nil
	}

	// Check for .windsurf/ in project directory
	cwd, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("get working dir: %w", err)
	}

	dir := cwd
	for i := 0; i < 5; i++ {
		projectWindsurf := filepath.Join(dir, ".windsurf")
		if info, err := os.Stat(projectWindsurf); err == nil && info.IsDir() {
			return true, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return false, nil
}

// =============================================================================
// Setup
// =============================================================================

// Setup creates or updates Windsurf config.json to add Cosca MCP server.
func (a *Adapter) Setup(config types.EditorConfig) error {
	cfgPath := filepath.Join(a.windsurfDir, "config.json")

	// Ensure directory exists
	if err := os.MkdirAll(a.windsurfDir, 0o755); err != nil {
		return fmt.Errorf("create windsurf dir: %w", err)
	}

	// Backup existing config
	if config.BackupExisting {
		if _, err := os.Stat(cfgPath); err == nil {
			data, _ := os.ReadFile(cfgPath)
			backupPath := cfgPath + ".bak"
			if err := os.WriteFile(backupPath, data, 0o644); err == nil {
				log.Debug().Str("backup", backupPath).Msg("backed up windsurf config.json")
			}
		}
	}

	// Read existing config or create default
	var cfg windsurfConfig
	if data, err := os.ReadFile(cfgPath); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			log.Warn().Err(err).Msg("existing windsurf config.json is invalid, will overwrite")
			cfg = windsurfConfig{}
		}
	}

	// Initialize MCP servers map if needed
	if cfg.MCPServers == nil {
		cfg.MCPServers = make(map[string]mcpServerConfig)
	}

	// Check if Cosca MCP server already exists
	if _, exists := cfg.MCPServers["cosca"]; exists {
		log.Info().Str("path", cfgPath).Msg("Cosca MCP server already configured in Windsurf")
		return nil
	}

	// Add Cosca MCP server
	coscaBin := config.CoscaBinPath
	if coscaBin == "" {
		coscaBin = "cosca"
	}

	cfg.MCPServers["cosca"] = mcpServerConfig{
		Command: coscaBin,
		Args:    []string{"mcp"},
	}

	// Write config
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal windsurf config: %w", err)
	}

	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		return fmt.Errorf("write windsurf config.json: %w", err)
	}

	log.Info().Str("path", cfgPath).Msg("Windsurf configured with Cosca MCP server")

	// Boot: create the project-level rules file so Windsurf loads the
	// Cosca Kernel at session start.
	if err := setupProjectRules(config); err != nil {
		return err
	}

	return nil
}

// setupProjectRules writes .windsurfrules in the project root.
// It is idempotent: if the file already contains the Cosca Kernel boot
// marker, it is left untouched.
func setupProjectRules(config types.EditorConfig) error {
	rulesPath := filepath.Join(config.ProjectDir, WindsurfRulesFileName)

	// Select boot content: self (framework on disk) or third-party (self-contained).
	rulesContent := WindsurfRulesContentThirdParty
	bootMarker := "consigliere do Don"
	if embedcosca.IsSelfProject(config.ProjectDir) {
		rulesContent = WindsurfRulesContentSelf
		bootMarker = "KERNEL.md"
	}

	if existing, err := os.ReadFile(rulesPath); err == nil {
		if strings.Contains(string(existing), bootMarker) {
			log.Info().Str("path", rulesPath).Msg("Cosca Kernel rules already in .windsurfrules")
			return nil
		}
	}

	if err := os.WriteFile(rulesPath, []byte(rulesContent), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", WindsurfRulesFileName, err)
	}

	log.Info().Str("path", rulesPath).Msg(".windsurfrules created with Cosca Kernel boot")
	return nil
}

// =============================================================================
// Validate
// =============================================================================

// Validate checks that Windsurf config.json exists and contains the Cosca MCP server entry.
func (a *Adapter) Validate() error {
	cfgPath := filepath.Join(a.windsurfDir, "config.json")

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("windsurf config.json not found: %w", types.ErrEditorNotConfigured)
		}
		return fmt.Errorf("read windsurf config.json: %w", err)
	}

	if !json.Valid(data) {
		return fmt.Errorf("windsurf config.json is not valid JSON")
	}

	var cfg windsurfConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse windsurf config.json: %w", err)
	}

	if cfg.MCPServers == nil {
		return fmt.Errorf("no MCP servers configured in Windsurf: %w", types.ErrEditorNotConfigured)
	}

	if _, exists := cfg.MCPServers["cosca"]; !exists {
		return fmt.Errorf("Cosca MCP server not found in Windsurf config: %w", types.ErrEditorNotConfigured)
	}

	return nil
}

// =============================================================================
// Teardown
// =============================================================================

// Teardown removes the Cosca MCP server entry from Windsurf config.json.
func (a *Adapter) Teardown() error {
	cfgPath := filepath.Join(a.windsurfDir, "config.json")

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing to tear down
		}
		return fmt.Errorf("read windsurf config.json: %w", err)
	}

	var cfg windsurfConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse windsurf config.json: %w", err)
	}

	// Remove Cosca MCP server
	if cfg.MCPServers != nil {
		delete(cfg.MCPServers, "cosca")
	}

	// If no more MCP servers, remove the field entirely
	if len(cfg.MCPServers) == 0 {
		cfg.MCPServers = nil
	}

	outData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal windsurf config: %w", err)
	}

	if err := os.WriteFile(cfgPath, outData, 0o644); err != nil {
		return fmt.Errorf("write windsurf config.json: %w", err)
	}

	log.Info().Str("path", cfgPath).Msg("Cosca MCP server removed from Windsurf config")
	return nil
}

// =============================================================================
// Info
// =============================================================================

// Info returns detailed information about the Windsurf integration.
func (a *Adapter) Info() (types.EditorInfo, error) {
	detected := false
	version := ""

	if info, err := os.Stat(a.windsurfDir); err == nil && info.IsDir() {
		detected = true
		if v, err := a.Version(); err == nil {
			version = v
		}
	}

	return types.EditorInfo{
		Name:         "windsurf",
		Version:      version,
		Path:         a.windsurfDir,
		Capabilities: a.Capabilities(),
		Detected:     detected,
	}, nil
}
