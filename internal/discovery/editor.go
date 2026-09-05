package discovery

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rs/zerolog"
)

// EditorType represents a known code editor/IDE type.
type EditorType string

// Predefined editor types.
const (
	EditorOpenCode   EditorType = "opencode"
	EditorClaudeCode EditorType = "claude-code"
	EditorCodexCLI   EditorType = "codex-cli"
	EditorCursor     EditorType = "cursor"
	EditorVSCode     EditorType = "vscode"
	EditorWindsurf   EditorType = "windsurf"
	EditorZed        EditorType = "zed"
	EditorNeovim     EditorType = "neovim"
	EditorVim        EditorType = "vim"
	EditorEmacs      EditorType = "emacs"
	EditorIntelliJ   EditorType = "intellij"
	EditorUnknown    EditorType = "unknown"
)

// EditorInfo describes a detected editor/IDE.
type EditorInfo struct {
	Type         EditorType        `json:"type" yaml:"type"`
	Name         string            `json:"name" yaml:"name"`
	Version      string            `json:"version,omitempty" yaml:"version,omitempty"`
	PID          int               `json:"pid,omitempty" yaml:"pid,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
	IsMCPEnabled bool              `json:"is_mcp_enabled" yaml:"is_mcp_enabled"`
	EnvVars      map[string]string `json:"env_vars,omitempty" yaml:"env_vars,omitempty"`
}

// editorDetection describes how to detect a specific editor.
type editorDetection struct {
	editorType EditorType
	name       string
	envVars    []string
	checkFn    func(logger zerolog.Logger) (*EditorInfo, bool)
}

// DetectEditor detects the currently running editor/IDE.
func DetectEditor(_ context.Context, logger zerolog.Logger) (*EditorInfo, error) {
	// Collect all possible editors in priority order
	detectors := []editorDetection{
		{
			editorType: EditorOpenCode,
			name:       "OpenCode",
			envVars:    []string{"OPENCODE_API_KEY", "OPENCODE_MODE", "OPENCODE_PID", "OPENCODE_SESSION_ID"},
			checkFn:    detectOpenCode,
		},
		{
			editorType: EditorClaudeCode,
			name:       "Claude Code",
			envVars:    []string{"CLAUDE_CODE_API_KEY", "CLAUDE_CODE_MODE", "CLAUDE_CODE_PID"},
			checkFn:    detectClaudeCode,
		},
		{
			editorType: EditorCodexCLI,
			name:       "Codex CLI",
			envVars:    []string{"CODEX_API_KEY", "CODEX_MODE", "CODEX_PID"},
			checkFn:    detectCodexCLI,
		},
		{
			editorType: EditorCursor,
			name:       "Cursor",
			envVars:    []string{"CURSOR_API_KEY", "CURSOR_MODE", "CURSOR_PID"},
			checkFn:    detectCursor,
		},
		{
			editorType: EditorVSCode,
			name:       "Visual Studio Code",
			envVars:    []string{"VSCODE_API_KEY", "VSCODE_MODE", "VSCODE_PID", "VSCODE_GIT_IPC_HANDLE", "VSCODE_IPC_HOOK_CLI"},
			checkFn:    detectVSCode,
		},
		{
			editorType: EditorWindsurf,
			name:       "Windsurf",
			envVars:    []string{"WINDSURF_API_KEY", "WINDSURF_MODE"},
			checkFn:    detectWindsurf,
		},
		{
			editorType: EditorZed,
			name:       "Zed",
			envVars:    []string{"ZED_API_KEY", "ZED_MODE"},
			checkFn:    detectZed,
		},
		{
			editorType: EditorNeovim,
			name:       "Neovim",
			envVars:    []string{"NVIM", "NVIM_LISTEN_ADDRESS", "VIM"},
			checkFn:    detectNeovim,
		},
		{
			editorType: EditorVim,
			name:       "Vim",
			envVars:    []string{"VIM", "VIMRUNTIME"},
			checkFn:    detectVim,
		},
		{
			editorType: EditorEmacs,
			name:       "Emacs",
			envVars:    []string{"EMACS", "INSIDE_EMACS"},
			checkFn:    detectEmacs,
		},
	}

	// Try each detector in order, return first match
	for _, d := range detectors {
		if info, ok := d.checkFn(logger); ok {
			info.EnvVars = collectEnvVars(d.envVars)
			info.IsMCPEnabled = detectMCP(info.EnvVars)
			logger.Debug().
				Str("type", string(info.Type)).
				Str("name", info.Name).
				Int("pid", info.PID).
				Bool("mcp", info.IsMCPEnabled).
				Msg("editor detected")
			return info, nil
		}
	}

	logger.Debug().Msg("no editor detected")
	return &EditorInfo{Type: EditorUnknown, Name: "Unknown"}, nil
}

// detectOpenCode detects OpenCode from environment variables.
func detectOpenCode(_ zerolog.Logger) (*EditorInfo, bool) {
	pidStr := os.Getenv("OPENCODE_PID")
	if pidStr != "" {
		info := &EditorInfo{
			Type: EditorOpenCode,
			Name: "OpenCode",
		}
		_, _ = fmt.Sscanf(pidStr, "%d", &info.PID)
		info.Capabilities = []string{"mcp", "agent", "context", "memory"}
		return info, true
	}
	if os.Getenv("OPENCODE_API_KEY") != "" || os.Getenv("OPENCODE_MODE") != "" {
		return &EditorInfo{
			Type:         EditorOpenCode,
			Name:         "OpenCode",
			Capabilities: []string{"mcp", "agent", "context", "memory"},
		}, true
	}
	return nil, false
}

// detectClaudeCode detects Claude Code from environment.
func detectClaudeCode(_ zerolog.Logger) (*EditorInfo, bool) {
	pidStr := os.Getenv("CLAUDE_CODE_PID")
	if pidStr != "" {
		info := &EditorInfo{
			Type: EditorClaudeCode,
			Name: "Claude Code",
		}
		_, _ = fmt.Sscanf(pidStr, "%d", &info.PID)
		info.Capabilities = []string{"mcp", "agent", "filesystem"}
		return info, true
	}
	if os.Getenv("CLAUDE_CODE_API_KEY") != "" {
		return &EditorInfo{
			Type:         EditorClaudeCode,
			Name:         "Claude Code",
			Capabilities: []string{"mcp", "agent", "filesystem"},
		}, true
	}
	return nil, false
}

// detectCodexCLI detects Codex CLI from environment.
func detectCodexCLI(_ zerolog.Logger) (*EditorInfo, bool) {
	pidStr := os.Getenv("CODEX_PID")
	if pidStr != "" {
		info := &EditorInfo{
			Type: EditorCodexCLI,
			Name: "Codex CLI",
		}
		_, _ = fmt.Sscanf(pidStr, "%d", &info.PID)
		info.Capabilities = []string{"mcp", "agent"}
		return info, true
	}
	if os.Getenv("CODEX_API_KEY") != "" {
		return &EditorInfo{
			Type:         EditorCodexCLI,
			Name:         "Codex CLI",
			Capabilities: []string{"mcp", "agent"},
		}, true
	}
	return nil, false
}

// detectCursor detects Cursor editor.
func detectCursor(_ zerolog.Logger) (*EditorInfo, bool) {
	pidStr := os.Getenv("CURSOR_PID")
	if pidStr != "" {
		info := &EditorInfo{
			Type: EditorCursor,
			Name: "Cursor",
		}
		_, _ = fmt.Sscanf(pidStr, "%d", &info.PID)
		info.Capabilities = []string{"mcp", "agent", "context"}
		return info, true
	}
	// Check for running cursor process
	if pid, ok := findProcess("cursor"); ok {
		return &EditorInfo{
			Type:         EditorCursor,
			Name:         "Cursor",
			PID:          pid,
			Capabilities: []string{"mcp", "agent", "context"},
		}, true
	}
	return nil, false
}

// detectVSCode detects Visual Studio Code.
func detectVSCode(_ zerolog.Logger) (*EditorInfo, bool) {
	// Check env vars first
	if pidStr := os.Getenv("VSCODE_PID"); pidStr != "" {
		info := &EditorInfo{
			Type: EditorVSCode,
			Name: "Visual Studio Code",
		}
		_, _ = fmt.Sscanf(pidStr, "%d", &info.PID)
		info.Capabilities = []string{"mcp", "agent", "terminal", "debug"}
		return info, true
	}

	// Check VS Code IPC handles
	if os.Getenv("VSCODE_IPC_HOOK_CLI") != "" || os.Getenv("VSCODE_GIT_IPC_HANDLE") != "" {
		return &EditorInfo{
			Type:         EditorVSCode,
			Name:         "Visual Studio Code",
			Capabilities: []string{"mcp", "agent", "terminal", "debug"},
		}, true
	}

	// Check for running process
	if pid, ok := findProcess("code"); ok {
		return &EditorInfo{
			Type:         EditorVSCode,
			Name:         "Visual Studio Code",
			PID:          pid,
			Capabilities: []string{"mcp", "agent", "terminal", "debug"},
		}, true
	}
	return nil, false
}

// detectWindsurf detects Windsurf editor.
func detectWindsurf(_ zerolog.Logger) (*EditorInfo, bool) {
	if os.Getenv("WINDSURF_API_KEY") != "" || os.Getenv("WINDSURF_MODE") != "" {
		return &EditorInfo{
			Type:         EditorWindsurf,
			Name:         "Windsurf",
			Capabilities: []string{"mcp", "agent", "context"},
		}, true
	}
	if pid, ok := findProcess("windsurf"); ok {
		return &EditorInfo{
			Type:         EditorWindsurf,
			Name:         "Windsurf",
			PID:          pid,
			Capabilities: []string{"mcp", "agent", "context"},
		}, true
	}
	return nil, false
}

// detectZed detects Zed editor.
func detectZed(_ zerolog.Logger) (*EditorInfo, bool) {
	if os.Getenv("ZED_API_KEY") != "" || os.Getenv("ZED_MODE") != "" {
		return &EditorInfo{
			Type:         EditorZed,
			Name:         "Zed",
			Capabilities: []string{"mcp", "agent"},
		}, true
	}
	if pid, ok := findProcess("zed"); ok {
		return &EditorInfo{
			Type:         EditorZed,
			Name:         "Zed",
			PID:          pid,
			Capabilities: []string{"mcp", "agent"},
		}, true
	}
	return nil, false
}

// detectNeovim detects Neovim.
func detectNeovim(_ zerolog.Logger) (*EditorInfo, bool) {
	if os.Getenv("NVIM") != "" || os.Getenv("NVIM_LISTEN_ADDRESS") != "" {
		info := &EditorInfo{
			Type:         EditorNeovim,
			Name:         "Neovim",
			Capabilities: []string{"terminal", "lsp"},
		}
		if os.Getenv("NVIM_LISTEN_ADDRESS") != "" {
			info.Capabilities = append(info.Capabilities, "mcp")
		}
		return info, true
	}
	if pid, ok := findProcess("nvim"); ok {
		return &EditorInfo{
			Type:         EditorNeovim,
			Name:         "Neovim",
			PID:          pid,
			Capabilities: []string{"terminal", "lsp"},
		}, true
	}
	return nil, false
}

// detectVim detects Vim.
func detectVim(_ zerolog.Logger) (*EditorInfo, bool) {
	if os.Getenv("VIM") != "" {
		return &EditorInfo{
			Type:         EditorVim,
			Name:         "Vim",
			Capabilities: []string{"terminal"},
		}, true
	}
	if pid, ok := findProcess("vim"); ok {
		return &EditorInfo{
			Type:         EditorVim,
			Name:         "Vim",
			PID:          pid,
			Capabilities: []string{"terminal"},
		}, true
	}
	return nil, false
}

// detectEmacs detects Emacs.
func detectEmacs(_ zerolog.Logger) (*EditorInfo, bool) {
	if os.Getenv("EMACS") != "" || os.Getenv("INSIDE_EMACS") != "" {
		info := &EditorInfo{
			Type:         EditorEmacs,
			Name:         "Emacs",
			Capabilities: []string{"terminal", "lsp"},
		}
		if strings.Contains(os.Getenv("INSIDE_EMACS"), "comint") {
			info.Capabilities = append(info.Capabilities, "mcp")
		}
		return info, true
	}
	if pid, ok := findProcess("emacs"); ok {
		return &EditorInfo{
			Type:         EditorEmacs,
			Name:         "Emacs",
			PID:          pid,
			Capabilities: []string{"terminal", "lsp"},
		}, true
	}
	return nil, false
}

// detectMCP checks if MCP (Model Context Protocol) is available.
func detectMCP(_ map[string]string) bool {
	// Check for MCP-related env vars
	mcpIndicators := []string{
		"MCP_ENABLED", "MCP_API_KEY", "MCP_ENDPOINT",
		"OPENCODE_MCP_ENABLED", "CLAUDE_CODE_MCP_ENABLED",
	}

	for _, indicator := range mcpIndicators {
		if val := os.Getenv(indicator); val != "" && val != "0" && val != "false" {
			return true
		}
	}
	return false
}

// collectEnvVars collects the values of the given environment variables.
func collectEnvVars(vars []string) map[string]string {
	result := make(map[string]string)
	for _, v := range vars {
		if val := os.Getenv(v); val != "" {
			result[v] = val
		}
	}
	return result
}

// findProcess attempts to find a running process by name.
func findProcess(name string) (int, bool) {
	if runtime.GOOS == "windows" {
		name = name + ".exe"
	}

	cmd := exec.Command("pgrep", "-x", name)
	output, err := cmd.Output()
	if err != nil {
		// Try alternative approach with ps
		return findProcessByPS(name)
	}

	pid := 0
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			_, _ = fmt.Sscanf(line, "%d", &pid)
			if pid > 0 {
				return pid, true
			}
		}
	}

	return 0, false
}

// findProcessByPS tries to find a process using ps command.
func findProcessByPS(name string) (int, bool) {
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return 0, false
	}

	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, name) && !strings.Contains(line, "grep") && !strings.Contains(line, "defunct") {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				pid := 0
				_, _ = fmt.Sscanf(fields[1], "%d", &pid)
				if pid > 0 {
					return pid, true
				}
			}
		}
	}

	return 0, false
}

// DetectMCPCapability checks if the current environment has MCP support.
func DetectMCPCapability(logger zerolog.Logger) bool {
	// Check for any MCP-related environment
	mcpEnvVars := []string{
		"MCP_ENABLED", "MCP_API_KEY", "MCP_ENDPOINT",
		"OPENCODE_MCP_ENABLED", "CLAUDE_CODE_MCP_ENABLED",
		"CODEX_MCP_ENABLED", "CURSOR_MCP_ENABLED",
	}

	for _, env := range mcpEnvVars {
		if val := os.Getenv(env); val != "" && val != "0" && val != "false" {
			logger.Debug().Str("env", env).Msg("MCP capability detected")
			return true
		}
	}

	// Check for .mcp directory
	homeDir, _ := os.UserHomeDir()
	mcpPaths := []string{
		filepath.Join(homeDir, ".mcp"),
		filepath.Join(homeDir, ".config", "mcp"),
		".mcp",
	}
	for _, p := range mcpPaths {
		if _, err := os.Stat(p); err == nil {
			logger.Debug().Str("path", p).Msg("MCP directory found")
			return true
		}
	}

	return false
}
