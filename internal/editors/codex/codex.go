// Package codex provides the Codex CLI editor adapter for Cosca,
// integrating Cosca with Codex CLI by creating .codex configuration files.
package codex

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

// Adapter integrates Cosca with Codex CLI by managing
// .codex configuration files.
type Adapter struct {
	types.BaseEditor
}

// NewAdapter creates a new Codex CLI editor adapter.
func NewAdapter() *Adapter {
	return &Adapter{
		BaseEditor: types.BaseEditor{
			NameValue: "codex",
			CapabilitiesVal: types.EditorCapabilities{
				SupportsContext:        true,
				SupportsSearch:         true,
				SupportsExecute:        true,
				SupportsWatch:          false,
				SupportsMCP:            true,
				SupportsCustomCommands: true,
				SupportsKeybindings:    false,
			},
		},
	}
}

// =============================================================================
// Cosca Kernel boot constants
// =============================================================================

const (
	// bootInstructionSelf is written into .codex/config.json for the Cosca
	// self-project (framework files on disk).
	bootInstructionSelf = "Carregue .cosca/framework/KERNEL.md ao iniciar. O Cosca Kernel é o consigliere do Don. Orquestra, nunca implementa."

	// bootInstructionThirdParty is written into .codex/config.json for
	// third-party projects (framework in binary embed, no on-disk files).
	bootInstructionThirdParty = "Carregue o Cosca Kernel ao iniciar. O Cosca Kernel é o consigliere do Don. Orquestra, nunca implementa. Banco: .cosca/knowledge.db — Memória: .cosca/memory/"

	// agentsBootBlockSelf is the AGENTS.md section that boots the Cosca Kernel
	// from on-disk framework files (Cosca self-project).
	agentsBootBlockSelf = `## Cosca Kernel — Boot Automático

AO INICIAR, carregue o Cosca Kernel:
- Manual: .cosca/framework/KERNEL.md (identidade, leis, operações)
- Estado cognitivo: .cosca/framework/knowledge/cognitive/cognitive-state.md
- O Kernel é o consigliere do Don. Orquestra, não implementa.
- Comandos: cosca kernel identity | cosca kernel memory | cosca run <prompt>
`

	// agentsBootBlockThirdParty is the AGENTS.md boot section for third-party
	// projects (framework in binary embed, no on-disk framework).
	agentsBootBlockThirdParty = `## Cosca Kernel — Boot Automático

AO INICIAR, carregue o Cosca Kernel:
- Manual: .cosca/knowledge.db (banco de evolução do projeto)
- Estado cognitivo: .cosca/memory/ (aprendizados dos agentes)
- O Kernel é o consigliere do Don. Orquestra, não implementa.
- Comandos: cosca kernel identity | cosca kernel memory | cosca run <prompt>
`

	// agentsBootMarker marks the AGENTS.md boot section (idempotency check).
	agentsBootMarker = "## Cosca Kernel — Boot Automático"
)

// codexTool represents a tool configuration in Codex CLI.
type codexTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Command     string `json:"command"`
	Args        string `json:"args,omitempty"`
}

// codexConfig represents the Codex CLI configuration structure.
type codexConfig struct {
	Instructions string      `json:"instructions,omitempty"`
	Tools        []codexTool `json:"tools,omitempty"`
}

// Version returns the detected Codex CLI version.
func (a *Adapter) Version() (string, error) {
	homeDir, _ := homedir.Dir()
	codexDir := filepath.Join(homeDir, ".codex")

	if info, err := os.Stat(codexDir); err == nil && info.IsDir() {
		return "1.0", nil
	}

	return "", fmt.Errorf("codex CLI not found")
}

// Detect checks if Codex CLI is installed.
func (a *Adapter) Detect() (bool, error) {
	// Check several possible Codex installation paths
	homeDir, _ := homedir.Dir()
	paths := []string{
		filepath.Join(homeDir, ".codex"),
		filepath.Join(homeDir, ".config", "codex"),
		"/usr/local/bin/codex",
		"/usr/bin/codex",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return true, nil
		}
	}

	// Check if codex binary is in PATH
	if path, err := execLookPath("codex"); err == nil && path != "" {
		return true, nil
	}

	return false, nil
}

// Setup creates .codex configuration with Cosca tool integration and
// writes the Cosca Kernel boot block into AGENTS.md at the project root.
// Both operations are idempotent.
func (a *Adapter) Setup(config types.EditorConfig) error {
	projectDir := config.ProjectDir
	if projectDir == "" {
		projectDir = "."
	}

	codexDir := filepath.Join(projectDir, ".codex")
	codexFile := filepath.Join(codexDir, "config.json")

	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		return fmt.Errorf("create .codex directory: %w", err)
	}

	// Read existing config or create default
	cfg := codexConfig{}
	fileExists := false
	if data, err := os.ReadFile(codexFile); err == nil {
		fileExists = true
		if err := json.Unmarshal(data, &cfg); err != nil {
			log.Warn().Err(err).Msg("invalid .codex/config.json, creating new")
			cfg = codexConfig{}
		}
	}

	// Idempotency: if the Kernel tools and instruction are already present,
	// leave config.json untouched.
	bootInstr := bootInstructionThirdParty
	if embedcosca.IsSelfProject(projectDir) {
		bootInstr = bootInstructionSelf
	}

	alreadyConfigured := (cfg.Instructions == bootInstructionSelf || cfg.Instructions == bootInstructionThirdParty) &&
		hasTool(cfg.Tools, "cosca-kernel") &&
		hasTool(cfg.Tools, "cosca-run")

	if alreadyConfigured {
		log.Info().Str("path", codexFile).Msg("Cosca Kernel already configured in .codex/config.json")
	} else {
		// Backup existing config before modifying it.
		if config.BackupExisting && fileExists {
			if err := backupFile(codexFile); err != nil {
				log.Warn().Err(err).Str("path", codexFile).Msg("failed to back up codex config.json")
			}
		}

		cfg.Instructions = bootInstr

		// Merge Cosca tools (avoid duplicates).
		existingNames := make(map[string]bool)
		for _, t := range cfg.Tools {
			existingNames[t.Name] = true
		}

		for _, t := range coscaTools() {
			if !existingNames[t.Name] {
				cfg.Tools = append(cfg.Tools, t)
			}
		}

		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal codex config: %w", err)
		}

		if err := os.WriteFile(codexFile, data, 0o644); err != nil {
			return fmt.Errorf("write codex config: %w", err)
		}
	}

	// AGENTS.md: Codex reads this file at the project root on startup.
	if err := a.updateAgentsMD(projectDir, config.BackupExisting); err != nil {
		return fmt.Errorf("update AGENTS.md: %w", err)
	}

	log.Info().Str("path", codexFile).Msg("Codex CLI configured with Cosca Kernel boot")
	return nil
}

// Validate checks that the .codex configuration contains Cosca tools.
func (a *Adapter) Validate() error {
	codexFile := filepath.Join(".", ".codex", "config.json")

	data, err := os.ReadFile(codexFile)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(".codex/config.json not found: %w", types.ErrEditorNotConfigured)
		}
		return fmt.Errorf("read .codex/config.json: %w", err)
	}

	var cfg codexConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse .codex/config.json: %w", err)
	}

	for _, tool := range cfg.Tools {
		if tool.Name == "cosca-kernel" || tool.Name == "cosca-search" || tool.Name == "cosca-index" {
			return nil
		}
	}

	return fmt.Errorf("Cosca tools not found in .codex configuration: %w",
		types.ErrEditorNotConfigured)
}

// Teardown removes Cosca tools from the Codex configuration.
func (a *Adapter) Teardown() error {
	codexFile := filepath.Join(".", ".codex", "config.json")

	data, err := os.ReadFile(codexFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read .codex/config.json: %w", err)
	}

	var cfg codexConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse .codex/config.json: %w", err)
	}

	// Filter out all Cosca tools (search, index, context, kernel, run).
	filtered := make([]codexTool, 0, len(cfg.Tools))
	for _, tool := range cfg.Tools {
		if strings.HasPrefix(tool.Name, "cosca-") {
			continue
		}
		filtered = append(filtered, tool)
	}
	cfg.Tools = filtered
	cfg.Instructions = ""

	outData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal codex config: %w", err)
	}

	if err := os.WriteFile(codexFile, outData, 0o644); err != nil {
		return fmt.Errorf("write codex config: %w", err)
	}

	log.Info().Str("path", codexFile).Msg("Cosca tools removed from Codex CLI configuration")
	return nil
}

// Info returns information about the Codex CLI integration.
func (a *Adapter) Info() (types.EditorInfo, error) {
	detected := false
	codexDir := filepath.Join(".", ".codex")
	if info, err := os.Stat(codexDir); err == nil && info.IsDir() {
		detected = true
	}

	return types.EditorInfo{
		Name:         "codex",
		Version:      "1.0",
		Path:         codexDir,
		Capabilities: a.Capabilities(),
		Detected:     detected,
	}, nil
}

// =============================================================================
// Helper functions
// =============================================================================

// coscaTools returns the full set of Cosca tools for Codex, including the
// Cosca Kernel boot tool and the prompt runner.
func coscaTools() []codexTool {
	return []codexTool{
		{
			Name:        "cosca-search",
			Description: "Semantic codebase search using Cosca",
			Command:     "cosca",
			Args:        "search \"$QUERY\"",
		},
		{
			Name:        "cosca-index",
			Description: "Index the codebase with Cosca",
			Command:     "cosca",
			Args:        "index",
		},
		{
			Name:        "cosca-context",
			Description: "Get Cosca context for code understanding",
			Command:     "cosca",
			Args:        "context \"$FILE\"",
		},
		{
			Name:        "cosca-kernel",
			Description: "Carregar o Cosca Kernel (KERNEL.md + cognitive-state)",
			Command:     "cosca",
			Args:        "kernel identity",
		},
		{
			Name:        "cosca-run",
			Description: "Executar prompt pelo Kernel",
			Command:     "cosca",
			Args:        "run \"$QUERY\"",
		},
	}
}

// hasTool reports whether a tool with the given name already exists.
func hasTool(tools []codexTool, name string) bool {
	for _, t := range tools {
		if t.Name == name {
			return true
		}
	}
	return false
}

// backupFile copies path to path+".bak" when the file exists.
func backupFile(path string) error {
	if _, err := os.Stat(path); err != nil {
		return nil // nothing to back up
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path+".bak", data, 0o644)
}

// updateAgentsMD creates or updates AGENTS.md at the project root with the
// Cosca Kernel boot block. Codex reads AGENTS.md on startup, which makes the
// Kernel load automatically. Idempotent.
func (a *Adapter) updateAgentsMD(projectDir string, backup bool) error {
	agentsPath := filepath.Join(projectDir, "AGENTS.md")

	// Select boot block: self (framework on disk) or third-party (self-contained).
	bootBlock := agentsBootBlockThirdParty
	if embedcosca.IsSelfProject(projectDir) {
		bootBlock = agentsBootBlockSelf
	}

	data, err := os.ReadFile(agentsPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read AGENTS.md: %w", err)
	}

	if err == nil {
		content := string(data)

		// Idempotency: boot block already present.
		if strings.Contains(content, agentsBootMarker) {
			log.Info().Str("path", agentsPath).Msg("Cosca Kernel boot already present in AGENTS.md")
			return nil
		}

		// Backup existing AGENTS.md before modifying it.
		if backup {
			if err := backupFile(agentsPath); err != nil {
				log.Warn().Err(err).Str("path", agentsPath).Msg("failed to back up AGENTS.md")
			}
		}

		content = strings.TrimRight(content, "\n") + "\n\n" + bootBlock
		if err := os.WriteFile(agentsPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write AGENTS.md: %w", err)
		}
		log.Info().Str("path", agentsPath).Msg("AGENTS.md updated with Cosca Kernel boot")
		return nil
	}

	// Fresh file.
	content := "# Project Agent Instructions\n\n" + bootBlock
	if err := os.WriteFile(agentsPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write AGENTS.md: %w", err)
	}
	log.Info().Str("path", agentsPath).Msg("AGENTS.md created with Cosca Kernel boot")
	return nil
}

// execLookPath checks if a binary is in PATH (simplified, avoids os/exec import).
func execLookPath(name string) (string, error) {
	pathEnv := os.Getenv("PATH")
	dirs := filepath.SplitList(pathEnv)
	for _, dir := range dirs {
		fullPath := filepath.Join(dir, name)
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return fullPath, nil
		}
	}
	return "", fmt.Errorf("not found in PATH")
}
