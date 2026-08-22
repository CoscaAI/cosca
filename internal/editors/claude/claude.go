// Package claude provides the Claude Code editor adapter for Cosca,
// integrating Cosca with Claude Code by creating and managing
// CLAUDE.md configuration files.
package claude

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/editors/types"
	embedcosca "github.com/CoscaAI/cosca/internal/embed/cosca"
)

// =============================================================================
// Adapter
// =============================================================================

// Adapter integrates Cosca with Claude Code by managing
// CLAUDE.md configuration files in the project directory.
type Adapter struct {
	types.BaseEditor
}

// NewAdapter creates a new Claude Code editor adapter.
func NewAdapter() *Adapter {
	return &Adapter{
		BaseEditor: types.BaseEditor{
			NameValue: "claude",
			CapabilitiesVal: types.EditorCapabilities{
				SupportsContext:        true,
				SupportsSearch:         true,
				SupportsExecute:        true,
				SupportsWatch:          false,
				SupportsMCP:            true,
				SupportsCustomCommands: false,
				SupportsKeybindings:    false,
			},
		},
	}
}

// CoscaBootMarker is the CLAUDE.md heading that marks the automatic Kernel boot
// section. It is used for idempotency checks.
const CoscaBootMarker = "## Cosca Kernel — Boot Automático"

// coscaIntegrationSectionSelf is the CLAUDE.md section for Cosca integration
// when the project IS the Cosca project itself (framework files on disk at .cosca/framework/).
var coscaIntegrationSectionSelf = `## Cosca Kernel — Boot Automático

AO INICIAR, carregue o Cosca Kernel:
- Manual: .cosca/framework/KERNEL.md (identidade, leis, operações)
- Estado cognitivo: .cosca/framework/knowledge/cognitive/cognitive-state.md
- O Kernel é o consigliere do Don. Orquestra, não implementa.
- Comandos: cosca kernel identity | cosca kernel memory | cosca run <prompt>

## Cosca Integration

Cosca (Agent Operating System) CLI is available as a development tool for codebase
analysis, semantic search, and context management.

### Available Commands

- cosca search <query> - Semantic codebase search
- cosca index - Index the codebase for search
- cosca context <file> - Get context for a specific file
- cosca status - Check Cosca system status

### Usage Guidelines

1. Before searching the codebase, use cosca search for semantic understanding
2. Use cosca context to gather relevant context before making changes
3. Keep the index up to date after significant changes with cosca index

### Configuration

Cosca is configured via ~/.config/cosca/config.yaml. Project-specific
configuration can be placed in .cosca/config.yaml.
`

// coscaIntegrationSectionThirdParty is the self-contained CLAUDE.md section
// for third-party projects (no framework files on disk).
var coscaIntegrationSectionThirdParty = `## Cosca Kernel — Boot Automático

AO INICIAR, carregue o Cosca Kernel:
- Manual: .cosca/knowledge.db (banco de evolução do projeto)
- Estado cognitivo: .cosca/memory/ (aprendizados dos agentes)
- O Kernel é o consigliere do Don. Orquestra, não implementa.
- Comandos: cosca kernel identity | cosca kernel memory | cosca run <prompt>

## Cosca Integration

Cosca (Agent Operating System) CLI is available as a development tool for codebase
analysis, semantic search, and context management.

### Available Commands

- cosca search <query> - Semantic codebase search
- cosca index - Index the codebase for search
- cosca context <file> - Get context for a specific file
- cosca status - Check Cosca system status

### Usage Guidelines

1. Before searching the codebase, use cosca search for semantic understanding
2. Use cosca context to gather relevant context before making changes
3. Keep the index up to date after significant changes with cosca index

### Configuration

Cosca is configured via ~/.config/cosca/config.yaml. Project-specific
configuration can be placed in .cosca/config.yaml.
`

// Version returns "1.0" as Claude Code doesn't have a formal version API.
func (a *Adapter) Version() (string, error) {
	return "1.0", nil
}

// Detect checks if Claude Code is available by looking for CLAUDE.md
// in the project root or checking environment variables.
func (a *Adapter) Detect() (bool, error) {
	// Check for CLAUDE.md in current directory or parent
	cwd, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("get working dir: %w", err)
	}

	// Look for CLAUDE.md in current or parent directories
	dir := cwd
	for i := 0; i < 5; i++ {
		claudePath := filepath.Join(dir, "CLAUDE.md")
		if _, err := os.Stat(claudePath); err == nil {
			return true, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Check environment variables
	if os.Getenv("CLAUDE_CODE") != "" || os.Getenv("ANTHROPIC_API_KEY") != "" {
		return true, nil
	}

	return false, nil
}

// Setup creates or updates CLAUDE.md with Cosca integration instructions.
// The Cosca Kernel boot section is appended at the top of the integration
// block so Claude Code loads it automatically on startup.
func (a *Adapter) Setup(config types.EditorConfig) error {
	claudePath := filepath.Join(config.ProjectDir, "CLAUDE.md")

	// Read existing content or create new.
	content, err := os.ReadFile(claudePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read CLAUDE.md: %w", err)
	}

	if err == nil {
		existing := string(content)

		// Idempotency: if the Kernel boot section is already present, do nothing.
		if strings.Contains(existing, CoscaBootMarker) {
			log.Info().Str("path", claudePath).Msg("Cosca Kernel boot already present in CLAUDE.md")
			return nil
		}

		// Backup existing CLAUDE.md before modifying it.
		if config.BackupExisting {
			if err := backupFile(claudePath); err != nil {
				log.Warn().Err(err).Str("path", claudePath).Msg("failed to back up CLAUDE.md")
			}
		}

		// Upgrade path: drop any previous "## Cosca ..." section (old format
		// without the boot block) and append the new one.
		existing = removeCoscaSections(existing)
		existing = strings.TrimRight(existing, "\n")

		// Select boot content: self (framework on disk) or third-party (self-contained).
		integSection := coscaIntegrationSectionThirdParty
		if embedcosca.IsSelfProject(config.ProjectDir) {
			integSection = coscaIntegrationSectionSelf
		}
		content = []byte(existing + "\n" + integSection)
	} else {
		integSection := coscaIntegrationSectionThirdParty
		if embedcosca.IsSelfProject(config.ProjectDir) {
			integSection = coscaIntegrationSectionSelf
		}
		content = []byte("# Claude Code Configuration\n" + integSection)
	}

	if err := os.WriteFile(claudePath, content, 0o644); err != nil {
		return fmt.Errorf("write CLAUDE.md: %w", err)
	}

	log.Info().Str("path", claudePath).Msg("CLAUDE.md updated with Cosca Kernel boot")
	return nil
}

// Validate checks that CLAUDE.md contains the Cosca integration section.
func (a *Adapter) Validate() error {
	claudePath := filepath.Join(".", "CLAUDE.md")

	data, err := os.ReadFile(claudePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("CLAUDE.md not found: %w", types.ErrEditorNotConfigured)
		}
		return fmt.Errorf("read CLAUDE.md: %w", err)
	}

	content := string(data)
	if !strings.Contains(content, CoscaBootMarker) {
		return fmt.Errorf("Cosca Kernel boot section not found in CLAUDE.md: %w",
			types.ErrEditorNotConfigured)
	}

	return nil
}

// Teardown removes the Cosca integration section from CLAUDE.md.
func (a *Adapter) Teardown() error {
	claudePath := filepath.Join(".", "CLAUDE.md")

	data, err := os.ReadFile(claudePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read CLAUDE.md: %w", err)
	}

	content := string(data)

	// Remove the Cosca integration section (boot block or legacy section).
	startMarker := "## Cosca"
	startIdx := strings.Index(content, startMarker)
	if startIdx == -1 {
		return nil // section not found, nothing to remove
	}

	// Find the next section or end of file
	remaining := content[startIdx+len(startMarker):]
	endIdx := strings.Index(remaining, "\n## ")
	if endIdx == -1 {
		content = content[:startIdx]
	} else {
		content = content[:startIdx] + remaining[endIdx:]
	}

	// Clean up extra whitespace
	content = strings.TrimSpace(content) + "\n"

	if err := os.WriteFile(claudePath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write CLAUDE.md: %w", err)
	}

	log.Info().Str("path", claudePath).Msg("Cosca integration removed from CLAUDE.md")
	return nil
}

// Info returns detailed information about the Claude Code integration.
func (a *Adapter) Info() (types.EditorInfo, error) {
	detected := false
	claudePath := filepath.Join(".", "CLAUDE.md")
	if _, err := os.Stat(claudePath); err == nil {
		detected = true
	}

	return types.EditorInfo{
		Name:         "claude",
		Version:      "1.0",
		Path:         claudePath,
		Capabilities: a.Capabilities(),
		Detected:     detected,
	}, nil
}

// =============================================================================
// Helpers
// =============================================================================

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

// removeCoscaSections strips every "## Cosca ..." section (boot block and
// legacy "## Cosca Integration") from content, keeping everything else.
func removeCoscaSections(content string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	skipping := false

	for _, line := range lines {
		// A new top-level heading ends the current skip region.
		if skipping && strings.HasPrefix(line, "## ") {
			skipping = false
		}
		// Any "## Cosca ..." heading starts a skip region.
		if strings.HasPrefix(line, "## Cosca") {
			skipping = true
			continue
		}
		if skipping {
			continue
		}
		out = append(out, line)
	}

	return strings.Join(out, "\n")
}
