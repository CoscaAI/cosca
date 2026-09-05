// Package neovim provides the Neovim editor adapter for Cosca,
// integrating Cosca with Neovim by creating and updating init.lua
// or init.vim configuration files.
package neovim

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/editors/types"
)

// =============================================================================
// Adapter
// =============================================================================

// Adapter integrates Cosca with Neovim by managing
// init.lua or init.vim configuration files.
type Adapter struct {
	types.BaseEditor
}

// NewAdapter creates a new Neovim editor adapter.
func NewAdapter() *Adapter {
	return &Adapter{
		BaseEditor: types.BaseEditor{
			NameValue: "neovim",
			CapabilitiesVal: types.EditorCapabilities{
				SupportsContext:        true,
				SupportsSearch:         true,
				SupportsExecute:        true,
				SupportsWatch:          false,
				SupportsMCP:            false,
				SupportsCustomCommands: true,
				SupportsKeybindings:    true,
			},
		},
	}
}

// CoscaLuaConfig is the Lua configuration snippet for Neovim Cosca integration.
const CoscaLuaConfig = `
-- =============================================================================
-- Cosca Integration
-- =============================================================================

function CoscaSearch(query)
	local cmd = "cosca search " .. vim.fn.shellescape(query)
	local result = vim.fn.system(cmd)
	vim.cmd("new")
	vim.api.nvim_buf_set_lines(0, 0, -1, false, vim.split(result, "\n"))
	vim.bo.buftype = "nofile"
	vim.bo.bufhidden = "wipe"
	vim.bo.modified = false
	vim.api.nvim_buf_set_name(0, "Cosca Search: " .. query)
end

function CoscaContext()
	local file = vim.fn.expand("%:p")
	if file == "" then
		print("Cosca: No file in current buffer")
		return
	end
	local cmd = "cosca context " .. vim.fn.shellescape(file)
	local result = vim.fn.system(cmd)
	vim.cmd("new")
	vim.api.nvim_buf_set_lines(0, 0, -1, false, vim.split(result, "\n"))
	vim.bo.buftype = "nofile"
	vim.bo.bufhidden = "wipe"
	vim.api.nvim_buf_set_name(0, "Cosca Context")
end

function CoscaIndex()
	vim.fn.system("cosca index")
	print("Cosca: Indexing complete")
end

function CoscaKernel()
	local result = vim.fn.system("cosca kernel identity")
	vim.cmd("new")
	vim.api.nvim_buf_set_lines(0, 0, -1, false, vim.split(result, "\n"))
	vim.bo.buftype = "nofile"
	vim.bo.bufhidden = "wipe"
	vim.bo.modified = false
	vim.api.nvim_buf_set_name(0, "Cosca Kernel")
end

vim.api.nvim_create_user_command("CoscaSearch", function(opts)
	CoscaSearch(opts.args)
end, { nargs = 1, desc = "Search codebase with Cosca" })

vim.api.nvim_create_user_command("CoscaContext", function()
	CoscaContext()
end, { desc = "Get Cosca context for current file" })

vim.api.nvim_create_user_command("CoscaIndex", function()
	CoscaIndex()
end, { desc = "Index codebase with Cosca" })

vim.api.nvim_create_user_command("CoscaKernel", function()
	CoscaKernel()
end, { desc = "Carregar o Cosca Kernel — identity, leis, constituição" })

vim.api.nvim_set_keymap("n", "<Leader>as", ":CoscaSearch ", { noremap = true, desc = "Cosca Search" })
vim.api.nvim_set_keymap("n", "<Leader>ac", ":CoscaContext<CR>", { noremap = true, desc = "Cosca Context" })
vim.api.nvim_set_keymap("n", "<Leader>ai", ":CoscaIndex<CR>", { noremap = true, desc = "Cosca Index" })
vim.api.nvim_set_keymap("n", "<Leader>ak", ":CoscaKernel<CR>", { noremap = true, desc = "Cosca Kernel" })
`

// CoscaVimConfig is the Vimscript configuration snippet for Neovim Cosca integration.
const CoscaVimConfig = `
" =============================================================================
" Cosca Integration
" =============================================================================

function! CoscaSearch(query)
	let cmd = "cosca search " . shellescape(a:query)
	let result = system(cmd)
	new
	call setline(1, split(result, "\n"))
	setlocal buftype=nofile bufhidden=wipe nomodified
	file "Cosca Search: " . a:query
endfunction

function! CoscaContext()
	let file = expand("%:p")
	if file == ""
		echo "Cosca: No file in current buffer"
		return
	endif
	let cmd = "cosca context " . shellescape(file)
	let result = system(cmd)
	new
	call setline(1, split(result, "\n"))
	setlocal buftype=nofile bufhidden=wipe nomodified
	file "Cosca Context"
endfunction

function! CoscaIndex()
	call system("cosca index")
	echo "Cosca: Indexing complete"
endfunction

function! CoscaKernel()
	let result = system("cosca kernel identity")
	new
	call setline(1, split(result, "\n"))
	setlocal buftype=nofile bufhidden=wipe nomodified
	file "Cosca Kernel"
endfunction

command! -nargs=1 CoscaSearch call CoscaSearch(<q-args>)
command! CoscaContext call CoscaContext()
command! CoscaIndex call CoscaIndex()
command! CoscaKernel call CoscaKernel()

nnoremap <Leader>as :CoscaSearch 
nnoremap <Leader>ac :CoscaContext<CR>
nnoremap <Leader>ai :CoscaIndex<CR>
nnoremap <Leader>ak :CoscaKernel<CR>
`

// Version returns the detected Neovim version.
func (a *Adapter) Version() (string, error) {
	paths := []string{
		"/usr/bin/nvim",
		"/usr/local/bin/nvim",
		"/opt/homebrew/bin/nvim",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return "1.0", nil
		}
	}

	return "", fmt.Errorf("neovim not found")
}

// Detect checks if Neovim is installed.
func (a *Adapter) Detect() (bool, error) {
	nvimConfig := a.getConfigDir()
	if nvimConfig == "" {
		homeDir, _ := homedir.Dir()
		candidates := []string{
			filepath.Join(homeDir, ".config", "nvim"),
			filepath.Join(homeDir, ".nvim"),
			filepath.Join(homeDir, "AppData", "Local", "nvim"),
		}

		for _, candidate := range candidates {
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				nvimConfig = candidate
				break
			}
		}
	}

	if nvimConfig == "" {
		if _, err := execLookPath("nvim"); err == nil {
			return true, nil
		}
		return false, nil
	}

	return true, nil
}

// Setup configures Neovim with Cosca integration.
func (a *Adapter) Setup(config types.EditorConfig) error {
	nvimDir := a.getConfigDir()
	if nvimDir == "" {
		homeDir, _ := homedir.Dir()
		nvimDir = filepath.Join(homeDir, ".config", "nvim")
	}

	if err := os.MkdirAll(nvimDir, 0o755); err != nil {
		return fmt.Errorf("create nvim config directory: %w", err)
	}

	initLua := filepath.Join(nvimDir, "init.lua")
	initVim := filepath.Join(nvimDir, "init.vim")

	useLua := false
	var configPath string

	if _, err := os.Stat(initLua); err == nil {
		configPath = initLua
		useLua = true
	} else if _, err := os.Stat(initVim); err == nil {
		configPath = initVim
	} else {
		configPath = initLua
		useLua = true
	}

	if config.BackupExisting {
		if _, err := os.Stat(configPath); err == nil {
			data, _ := os.ReadFile(configPath)
			backupPath := configPath + ".bak"
			if err := os.WriteFile(backupPath, data, 0o644); err == nil {
				log.Debug().Str("backup", backupPath).Msg("backed up nvim config")
			}
		}
	}

	var content string
	existing, err := os.ReadFile(configPath)
	if err == nil {
		content = string(existing)
		if strings.Contains(content, "Cosca Integration") {
			log.Info().Str("path", configPath).Msg("Cosca integration already in nvim config")
			return nil
		}
	}

	if useLua {
		content += CoscaLuaConfig
	} else {
		content += CoscaVimConfig
	}

	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write nvim config: %w", err)
	}

	log.Info().Str("path", configPath).Str("type", map[bool]string{true: "lua", false: "vimscript"}[useLua]).
		Msg("Neovim configured with Cosca integration")
	return nil
}

// Validate checks that Neovim config contains Cosca integration.
func (a *Adapter) Validate() error {
	nvimDir := a.getConfigDir()
	if nvimDir == "" {
		return fmt.Errorf("neovim config directory not found: %w", types.ErrEditorNotConfigured)
	}

	configPaths := []string{
		filepath.Join(nvimDir, "init.lua"),
		filepath.Join(nvimDir, "init.vim"),
	}

	for _, path := range configPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		if strings.Contains(content, "Cosca Integration") {
			return nil
		}
	}

	return fmt.Errorf("Cosca integration not found in neovim config: %w",
		types.ErrEditorNotConfigured)
}

// Teardown removes Cosca integration from Neovim config.
func (a *Adapter) Teardown() error {
	nvimDir := a.getConfigDir()
	if nvimDir == "" {
		return nil
	}

	configPaths := []string{
		filepath.Join(nvimDir, "init.lua"),
		filepath.Join(nvimDir, "init.vim"),
	}

	for _, path := range configPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		content := string(data)

		// Setup always appends the Cosca block at the end of the file, so
		// removing everything from the block's banner line on is exact.
		blockStart := findCoscaBlockStart(content)
		if blockStart == -1 {
			continue
		}

		content = strings.TrimSpace(content[:blockStart])

		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write nvim config: %w", err)
		}

		log.Info().Str("path", path).Msg("Cosca integration removed from neovim config")
		return nil
	}

	return nil
}

// findCoscaBlockStart returns the byte offset of the first line of the
// Cosca integration block (the banner comment line, included), or -1 when
// the block is not present. It identifies the block by the banner comment
// (Lua "-- ====" or Vimscript '" ====') immediately followed by the
// "Cosca Integration" header.
func findCoscaBlockStart(content string) int {
	markers := []string{
		"-- =============================================================================",
		`" =============================================================================`,
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		for _, marker := range markers {
			if !strings.HasPrefix(line, marker) {
				continue
			}
			// The banner must introduce the Cosca block, not some other section.
			if i+1 < len(lines) && strings.Contains(lines[i+1], "Cosca Integration") {
				offset := 0
				for j := 0; j < i; j++ {
					offset += len(lines[j]) + 1
				}
				return offset
			}
		}
	}
	return -1
}

// Info returns information about the Neovim integration.
func (a *Adapter) Info() (types.EditorInfo, error) {
	detected := false
	configPath := ""

	nvimDir := a.getConfigDir()
	if nvimDir != "" {
		detected = true
		configPath = nvimDir
	} else {
		if _, err := execLookPath("nvim"); err == nil {
			detected = true
			configPath = "nvim (binary found)"
		}
	}

	return types.EditorInfo{
		Name:         "neovim",
		Version:      "1.0",
		Path:         configPath,
		Capabilities: a.Capabilities(),
		Detected:     detected,
	}, nil
}

// getConfigDir returns the Neovim config directory.
func (a *Adapter) getConfigDir() string {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		nvimDir := filepath.Join(xdgConfig, "nvim")
		if info, err := os.Stat(nvimDir); err == nil && info.IsDir() {
			return nvimDir
		}
	}

	homeDir, _ := homedir.Dir()
	nvimDir := filepath.Join(homeDir, ".config", "nvim")
	if info, err := os.Stat(nvimDir); err == nil && info.IsDir() {
		return nvimDir
	}

	nvimDir = filepath.Join(homeDir, ".nvim")
	if info, err := os.Stat(nvimDir); err == nil && info.IsDir() {
		return nvimDir
	}

	return ""
}

// execLookPath checks if a binary is in PATH (simplified).
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
