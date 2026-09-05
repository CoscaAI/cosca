// Package cursor provides the Cursor editor adapter for Cosca,
// integrating Cosca with Cursor by managing .cursorrules configuration.
package cursor

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

// Adapter integrates Cosca with Cursor by managing
// .cursorrules configuration files.
type Adapter struct {
	types.BaseEditor
}

// NewAdapter creates a new Cursor editor adapter.
func NewAdapter() *Adapter {
	return &Adapter{
		BaseEditor: types.BaseEditor{
			NameValue: "cursor",
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

// CoscaRulesContent contains the Cursor rules for Cosca integration.
const CoscaRulesContent = `
# Cosca Integration Rules

You have access to the Cosca (Agent Operating System) for codebase
analysis, semantic search, and context management.

## Available Tools

- Use "cosca search <query>" for semantic codebase search before making changes
- Use "cosca context <file>" to gather context about specific files
- Use "cosca index" after significant changes to update the search index

## Workflow Guidelines

1. **Before editing**: Search the codebase with "cosca search" to understand existing patterns
2. **During development**: Use "cosca context" to reference relevant code
3. **After changes**: Run "cosca index" to keep the index up to date
`

// CoscaKernelRuleFileName is the Cursor rules file that boots the Cosca Kernel.
const CoscaKernelRuleFileName = "cosca-kernel.mdc"

// CoscaKernelRuleContentSelf is the Cursor rules (.mdc) content that instructs
// the agent to load the Cosca Kernel from on-disk framework files (Cosca self-project).
const CoscaKernelRuleContentSelf = `---
description: Cosca Kernel boot
globs: **/*
---
AO INICIAR a sessão, carregue .cosca/framework/KERNEL.md.
O Cosca Kernel é o consigliere do Don — orquestra, nunca implementa.
Estado: .cosca/framework/knowledge/cognitive/cognitive-state.md
`

// CoscaKernelRuleContentThirdParty is the Cursor rules (.mdc) content for
// third-party projects (framework in binary embed, no on-disk framework).
const CoscaKernelRuleContentThirdParty = `---
description: Cosca Kernel boot
globs: **/*
---
AO INICIAR a sessão, carregue o Cosca Kernel.
O Cosca Kernel é o consigliere do Don — orquestra, nunca implementa.
Banco: .cosca/knowledge.db | Memória: .cosca/memory/
`

// setupKernelRule writes .cursor/rules/cosca-kernel.mdc in the project.
// It is idempotent: if the rule file already contains the Cosca Kernel
// boot marker, it is left untouched.
func (a *Adapter) setupKernelRule(config types.EditorConfig) error {
	rulesDir := filepath.Join(config.ProjectDir, ".cursor", "rules")
	rulePath := filepath.Join(rulesDir, CoscaKernelRuleFileName)

	if existing, err := os.ReadFile(rulePath); err == nil {
		if strings.Contains(string(existing), "Cosca Kernel boot") {
			log.Info().Str("path", rulePath).Msg("Cosca Kernel rule already in .cursor/rules")
			return nil
		}
	}

	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		return fmt.Errorf("create .cursor/rules directory: %w", err)
	}

	// Select boot content: self (framework on disk) or third-party (self-contained).
	kernelContent := CoscaKernelRuleContentThirdParty
	if embedcosca.IsSelfProject(config.ProjectDir) {
		kernelContent = CoscaKernelRuleContentSelf
	}

	if err := os.WriteFile(rulePath, []byte(kernelContent), 0o644); err != nil {
		return fmt.Errorf("write .cursor/rules/%s: %w", CoscaKernelRuleFileName, err)
	}

	log.Info().Str("path", rulePath).Msg(".cursor/rules/cosca-kernel.mdc created")
	return nil
}

// Version returns the detected Cursor version.
func (a *Adapter) Version() (string, error) {
	return "1.0", nil
}

// Detect checks if Cursor is in use by looking for .cursorrules.
func (a *Adapter) Detect() (bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("get working dir: %w", err)
	}

	dir := cwd
	for i := 0; i < 5; i++ {
		cursorRules := filepath.Join(dir, ".cursorrules")
		if _, err := os.Stat(cursorRules); err == nil {
			return true, nil
		}

		cursorDir := filepath.Join(dir, ".cursor")
		if info, err := os.Stat(cursorDir); err == nil && info.IsDir() {
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

// Setup appends Cosca rules to .cursorrules or creates a new file.
func (a *Adapter) Setup(config types.EditorConfig) error {
	cursorRulesPath := filepath.Join(config.ProjectDir, ".cursorrules")

	if config.BackupExisting {
		if _, err := os.Stat(cursorRulesPath); err == nil {
			data, _ := os.ReadFile(cursorRulesPath)
			backupPath := cursorRulesPath + ".bak"
			if err := os.WriteFile(backupPath, data, 0o644); err == nil {
				log.Debug().Str("backup", backupPath).Msg("backed up .cursorrules")
			}
		}
	}

	var content string
	existing, err := os.ReadFile(cursorRulesPath)
	if err == nil {
		content = string(existing)
		if strings.Contains(content, "# Cosca Integration Rules") {
			log.Info().Str("path", cursorRulesPath).Msg("Cosca rules already in .cursorrules")
			return a.setupKernelRule(config)
		}
		content += "\n" + CoscaRulesContent
	} else {
		content = CoscaRulesContent
	}

	if err := os.WriteFile(cursorRulesPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write .cursorrules: %w", err)
	}

	log.Info().Str("path", cursorRulesPath).Msg(".cursorrules updated with Cosca integration")

	// Boot: create the Cursor rules file that loads the Cosca Kernel on session start.
	if err := a.setupKernelRule(config); err != nil {
		return err
	}

	return nil
}

// Validate checks that .cursorrules contains the Cosca rules.
func (a *Adapter) Validate() error {
	cursorRulesPath := filepath.Join(".", ".cursorrules")

	data, err := os.ReadFile(cursorRulesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(".cursorrules not found: %w", types.ErrEditorNotConfigured)
		}
		return fmt.Errorf("read .cursorrules: %w", err)
	}

	content := string(data)
	if !strings.Contains(content, "Cosca Integration") {
		return fmt.Errorf("Cosca rules not found in .cursorrules: %w",
			types.ErrEditorNotConfigured)
	}

	return nil
}

// Teardown removes Cosca rules from .cursorrules.
func (a *Adapter) Teardown() error {
	cursorRulesPath := filepath.Join(".", ".cursorrules")

	data, err := os.ReadFile(cursorRulesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read .cursorrules: %w", err)
	}

	content := string(data)

	startMarker := "# Cosca Integration Rules"
	startIdx := strings.Index(content, startMarker)
	if startIdx == -1 {
		return nil
	}

	remaining := content[startIdx+len(startMarker):]
	endIdx := strings.Index(remaining, "\n# ")
	if endIdx == -1 {
		content = content[:startIdx]
	} else {
		content = content[:startIdx] + remaining[endIdx:]
	}

	content = strings.TrimSpace(content)
	if content != "" {
		content += "\n"
	}

	if err := os.WriteFile(cursorRulesPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write .cursorrules: %w", err)
	}

	log.Info().Str("path", cursorRulesPath).Msg("Cosca rules removed from .cursorrules")
	return nil
}

// Info returns information about the Cursor integration.
func (a *Adapter) Info() (types.EditorInfo, error) {
	detected := false
	cursorRulesPath := filepath.Join(".", ".cursorrules")
	if _, err := os.Stat(cursorRulesPath); err == nil {
		detected = true
	}

	return types.EditorInfo{
		Name:         "cursor",
		Version:      "1.0",
		Path:         cursorRulesPath,
		Capabilities: a.Capabilities(),
		Detected:     detected,
	}, nil
}
