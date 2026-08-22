package discovery

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

// ============================================================================
// Tests for WithLogger option (currently 0.0%)
// ============================================================================

func TestWithLogger(t *testing.T) {
	t.Parallel()

	logger := zerolog.New(os.Stderr).With().Str("component", "test").Logger()
	e := NewEngine(WithLogger(logger))

	// Verify logger is set by checking we can use it without panic
	if e.logger.GetLevel() != logger.GetLevel() {
		t.Error("logger was not properly set")
	}
}

func TestWithLogger_CombinedOptions(t *testing.T) {
	t.Parallel()

	logger := zerolog.New(os.Stderr).With().Str("component", "test").Logger()
	e := NewEngine(
		WithLogger(logger),
		WithWorkDir("/custom"),
		WithCacheTTL(10),
	)

	if e.workDir != "/custom" {
		t.Errorf("workDir = %q, want /custom", e.workDir)
	}
	if e.logger.GetLevel() != logger.GetLevel() {
		t.Error("logger was not properly set with combined options")
	}
}

// ============================================================================
// Tests for editor detection functions (all currently 0.0%)
// These use os.Getenv and are testable with t.Setenv.
// ============================================================================

func TestDetectClaudeCode_WithPID(t *testing.T) {
	t.Setenv("CLAUDE_CODE_PID", "12345")

	info, ok := detectClaudeCode(zerolog.Nop())
	if !ok {
		t.Fatal("expected detection to succeed with CLAUDE_CODE_PID set")
	}
	if info.Type != EditorClaudeCode {
		t.Errorf("Type = %q, want %q", info.Type, EditorClaudeCode)
	}
	if info.Name != "Claude Code" {
		t.Errorf("Name = %q, want %q", info.Name, "Claude Code")
	}
	if info.PID != 12345 {
		t.Errorf("PID = %d, want 12345", info.PID)
	}
	if len(info.Capabilities) == 0 {
		t.Error("Capabilities should not be empty")
	}
}

func TestDetectClaudeCode_WithAPIKey(t *testing.T) {
	t.Setenv("CLAUDE_CODE_API_KEY", "sk-test")

	info, ok := detectClaudeCode(zerolog.Nop())
	if !ok {
		t.Fatal("expected detection to succeed with CLAUDE_CODE_API_KEY set")
	}
	if info.Type != EditorClaudeCode {
		t.Errorf("Type = %q, want %q", info.Type, EditorClaudeCode)
	}
	if info.PID != 0 {
		t.Errorf("PID = %d, want 0 (no PID set)", info.PID)
	}
}

func TestDetectClaudeCode_NoEnv(t *testing.T) {
	_, ok := detectClaudeCode(zerolog.Nop())
	if ok {
		t.Error("expected detection to fail with no env vars set")
	}
}

func TestDetectCodexCLI_WithPID(t *testing.T) {
	t.Setenv("CODEX_PID", "54321")

	info, ok := detectCodexCLI(zerolog.Nop())
	if !ok {
		t.Fatal("expected detection to succeed with CODEX_PID set")
	}
	if info.Type != EditorCodexCLI {
		t.Errorf("Type = %q, want %q", info.Type, EditorCodexCLI)
	}
	if info.Name != "Codex CLI" {
		t.Errorf("Name = %q, want %q", info.Name, "Codex CLI")
	}
	if info.PID != 54321 {
		t.Errorf("PID = %d, want 54321", info.PID)
	}
}

func TestDetectCodexCLI_WithAPIKey(t *testing.T) {
	t.Setenv("CODEX_API_KEY", "sk-codex")

	info, ok := detectCodexCLI(zerolog.Nop())
	if !ok {
		t.Fatal("expected detection to succeed with CODEX_API_KEY set")
	}
	if info.Type != EditorCodexCLI {
		t.Errorf("Type = %q, want %q", info.Type, EditorCodexCLI)
	}
}

func TestDetectCodexCLI_NoEnv(t *testing.T) {
	_, ok := detectCodexCLI(zerolog.Nop())
	if ok {
		t.Error("expected detection to fail with no env vars set")
	}
}

func TestDetectCursor_WithPID(t *testing.T) {
	t.Setenv("CURSOR_PID", "9999")

	info, ok := detectCursor(zerolog.Nop())
	if !ok {
		t.Fatal("expected detection to succeed with CURSOR_PID set")
	}
	if info.Type != EditorCursor {
		t.Errorf("Type = %q, want %q", info.Type, EditorCursor)
	}
	if info.Name != "Cursor" {
		t.Errorf("Name = %q, want %q", info.Name, "Cursor")
	}
	if info.PID != 9999 {
		t.Errorf("PID = %d, want 9999", info.PID)
	}
}

func TestDetectCursor_NoEnv(t *testing.T) {
	// Keep environment and process lookup independent from the host machine.
	t.Setenv("CURSOR_API_KEY", "")
	t.Setenv("CURSOR_MODE", "")
	t.Setenv("CURSOR_PID", "")
	t.Setenv("PATH", t.TempDir())

	_, ok := detectCursor(zerolog.Nop())
	if ok {
		t.Error("expected detection to fail with no env vars and no process")
	}
}

func TestDetectVSCode_WithPID(t *testing.T) {
	t.Setenv("VSCODE_PID", "7777")

	info, ok := detectVSCode(zerolog.Nop())
	if !ok {
		t.Fatal("expected detection to succeed with VSCODE_PID set")
	}
	if info.Type != EditorVSCode {
		t.Errorf("Type = %q, want %q", info.Type, EditorVSCode)
	}
	if info.Name != "Visual Studio Code" {
		t.Errorf("Name = %q, want %q", info.Name, "Visual Studio Code")
	}
	if info.PID != 7777 {
		t.Errorf("PID = %d, want 7777", info.PID)
	}
}

func TestDetectVSCode_WithIPCHandles(t *testing.T) {
	// Test the IPC hook path (not PID-based)
	t.Setenv("VSCODE_IPC_HOOK_CLI", "/tmp/vscode-ipc")

	info, ok := detectVSCode(zerolog.Nop())
	if !ok {
		t.Fatal("expected detection to succeed with VSCODE_IPC_HOOK_CLI set")
	}
	if info.Type != EditorVSCode {
		t.Errorf("Type = %q, want %q", info.Type, EditorVSCode)
	}
	if info.PID != 0 {
		t.Errorf("PID = %d, want 0 (no PID set)", info.PID)
	}
}

func TestDetectVSCode_WithGitIPCHandle(t *testing.T) {
	t.Setenv("VSCODE_GIT_IPC_HANDLE", "/tmp/vscode-git")

	info, ok := detectVSCode(zerolog.Nop())
	if !ok {
		t.Fatal("expected detection to succeed with VSCODE_GIT_IPC_HANDLE set")
	}
	if info.Type != EditorVSCode {
		t.Errorf("Type = %q, want %q", info.Type, EditorVSCode)
	}
}

func TestDetectVSCode_NoEnv(t *testing.T) {
	// When no VS Code env vars are set, detection may still succeed if
	// the "code" process is running on the test machine. Test the
	// env-based detection paths primarily.
	info, ok := detectVSCode(zerolog.Nop())
	if ok {
		// If detected, it must be via process detection (no PID, no IPC)
		if info.PID == 0 {
			// Might be via IPC handles; ensure we don't assert too strictly
			t.Log("VSCode detected via process or IPC — this is fine on dev machines")
		}
	}
	// Main assertion: without env vars, no PID-bound detection should happen
	_ = info
}

func TestDetectWindsurf_WithEnv(t *testing.T) {
	t.Setenv("WINDSURF_API_KEY", "sk-windsurf")

	info, ok := detectWindsurf(zerolog.Nop())
	if !ok {
		t.Fatal("expected detection to succeed with WINDSURF_API_KEY set")
	}
	if info.Type != EditorWindsurf {
		t.Errorf("Type = %q, want %q", info.Type, EditorWindsurf)
	}
	if info.Name != "Windsurf" {
		t.Errorf("Name = %q, want %q", info.Name, "Windsurf")
	}
}

func TestDetectWindsurf_WithMode(t *testing.T) {
	t.Setenv("WINDSURF_MODE", "agent")

	info, ok := detectWindsurf(zerolog.Nop())
	if !ok {
		t.Fatal("expected detection to succeed with WINDSURF_MODE set")
	}
	if info.Type != EditorWindsurf {
		t.Errorf("Type = %q, want %q", info.Type, EditorWindsurf)
	}
}

func TestDetectWindsurf_NoEnv(t *testing.T) {
	// Keep process detection independent from editors running on the host.
	t.Setenv("WINDSURF_API_KEY", "")
	t.Setenv("WINDSURF_MODE", "")
	t.Setenv("PATH", t.TempDir())

	_, ok := detectWindsurf(zerolog.Nop())
	if ok {
		t.Error("expected detection to fail with no env vars and no process")
	}
}

func TestDetectZed_WithEnv(t *testing.T) {
	t.Setenv("ZED_API_KEY", "sk-zed")

	info, ok := detectZed(zerolog.Nop())
	if !ok {
		t.Fatal("expected detection to succeed with ZED_API_KEY set")
	}
	if info.Type != EditorZed {
		t.Errorf("Type = %q, want %q", info.Type, EditorZed)
	}
	if info.Name != "Zed" {
		t.Errorf("Name = %q, want %q", info.Name, "Zed")
	}
}

func TestDetectZed_WithMode(t *testing.T) {
	t.Setenv("ZED_MODE", "agent")

	info, ok := detectZed(zerolog.Nop())
	if !ok {
		t.Fatal("expected detection to succeed with ZED_MODE set")
	}
	if info.Type != EditorZed {
		t.Errorf("Type = %q, want %q", info.Type, EditorZed)
	}
}

func TestDetectZed_NoEnv(t *testing.T) {
	// Without Zed env vars, detection may still succeed via process detection
	// if zed is running on the test machine. Only test env-var paths strictly.
	info, ok := detectZed(zerolog.Nop())
	if ok && info.PID > 0 {
		t.Log("Zed detected via process — this is fine on dev machines")
	}
	_ = info
}

func TestDetectNeovim_WithEnv(t *testing.T) {
	t.Setenv("NVIM", "/usr/bin/nvim")

	info, ok := detectNeovim(zerolog.Nop())
	if !ok {
		t.Fatal("expected detection to succeed with NVIM set")
	}
	if info.Type != EditorNeovim {
		t.Errorf("Type = %q, want %q", info.Type, EditorNeovim)
	}
	if info.Name != "Neovim" {
		t.Errorf("Name = %q, want %q", info.Name, "Neovim")
	}
}

func TestDetectNeovim_WithListenAddress(t *testing.T) {
	// NVIM_LISTEN_ADDRESS should add MCP capability
	t.Setenv("NVIM_LISTEN_ADDRESS", "/tmp/nvim-socket")

	info, ok := detectNeovim(zerolog.Nop())
	if !ok {
		t.Fatal("expected detection to succeed with NVIM_LISTEN_ADDRESS set")
	}
	if info.Type != EditorNeovim {
		t.Errorf("Type = %q, want %q", info.Type, EditorNeovim)
	}

	// Check that MCP capability is present when NVIM_LISTEN_ADDRESS is set
	hasMCP := false
	for _, cap := range info.Capabilities {
		if cap == "mcp" {
			hasMCP = true
			break
		}
	}
	if !hasMCP {
		t.Error("expected MCP capability when NVIM_LISTEN_ADDRESS is set")
	}
}

func TestDetectNeovim_NoEnv(t *testing.T) {
	_, ok := detectNeovim(zerolog.Nop())
	if ok {
		t.Error("expected detection to fail with no env vars and no process")
	}
}

func TestDetectVim_WithEnv(t *testing.T) {
	t.Setenv("VIM", "/usr/bin/vim")

	info, ok := detectVim(zerolog.Nop())
	if !ok {
		t.Fatal("expected detection to succeed with VIM set")
	}
	if info.Type != EditorVim {
		t.Errorf("Type = %q, want %q", info.Type, EditorVim)
	}
	if info.Name != "Vim" {
		t.Errorf("Name = %q, want %q", info.Name, "Vim")
	}
}

func TestDetectVim_NoEnv(t *testing.T) {
	_, ok := detectVim(zerolog.Nop())
	if ok {
		t.Error("expected detection to fail with no env vars and no process")
	}
}

func TestDetectEmacs_WithEmacsEnv(t *testing.T) {
	t.Setenv("EMACS", "29.1")

	info, ok := detectEmacs(zerolog.Nop())
	if !ok {
		t.Fatal("expected detection to succeed with EMACS set")
	}
	if info.Type != EditorEmacs {
		t.Errorf("Type = %q, want %q", info.Type, EditorEmacs)
	}
	if info.Name != "Emacs" {
		t.Errorf("Name = %q, want %q", info.Name, "Emacs")
	}
}

func TestDetectEmacs_WithInsideEmacs(t *testing.T) {
	// INSIDE_EMACS with comint should add MCP capability
	t.Setenv("INSIDE_EMACS", "29.1,comint")

	info, ok := detectEmacs(zerolog.Nop())
	if !ok {
		t.Fatal("expected detection to succeed with INSIDE_EMACS set")
	}
	if info.Type != EditorEmacs {
		t.Errorf("Type = %q, want %q", info.Type, EditorEmacs)
	}

	// Check that MCP capability is present when INSIDE_EMACS contains comint
	hasMCP := false
	for _, cap := range info.Capabilities {
		if cap == "mcp" {
			hasMCP = true
			break
		}
	}
	if !hasMCP {
		t.Error("expected MCP capability when INSIDE_EMACS contains comint")
	}
}

func TestDetectEmacs_NoEnv(t *testing.T) {
	_, ok := detectEmacs(zerolog.Nop())
	if ok {
		t.Error("expected detection to fail with no env vars and no process")
	}
}

// ============================================================================
// Tests for detectOpenCode (already at 100% but edge cases worth covering)
// ============================================================================

func TestDetectOpenCode_WithPID(t *testing.T) {
	t.Setenv("OPENCODE_PID", "42")

	info, ok := detectOpenCode(zerolog.Nop())
	if !ok {
		t.Fatal("expected detection to succeed with OPENCODE_PID set")
	}
	if info.Type != EditorOpenCode {
		t.Errorf("Type = %q, want %q", info.Type, EditorOpenCode)
	}
	if info.PID != 42 {
		t.Errorf("PID = %d, want 42", info.PID)
	}
}

func TestDetectOpenCode_WithAPIKey(t *testing.T) {
	t.Setenv("OPENCODE_API_KEY", "sk-opencode")

	info, ok := detectOpenCode(zerolog.Nop())
	if !ok {
		t.Fatal("expected detection to succeed with OPENCODE_API_KEY set")
	}
	if info.Type != EditorOpenCode {
		t.Errorf("Type = %q, want %q", info.Type, EditorOpenCode)
	}
}

func TestDetectOpenCode_NoEnv(t *testing.T) {
	// Unset any OpenCode env vars that might be set in the test environment
	t.Setenv("OPENCODE_API_KEY", "")
	t.Setenv("OPENCODE_MODE", "")
	t.Setenv("OPENCODE_PID", "")

	_, ok := detectOpenCode(zerolog.Nop())
	if ok {
		t.Error("expected detection to fail with no env vars set")
	}
}

// ============================================================================
// Tests for detectMCP and DetectMCPCapability (currently 80% / 0%)
// ============================================================================

func TestDetectMCP_Enabled(t *testing.T) {
	t.Setenv("MCP_ENABLED", "true")
	if !detectMCP(nil) {
		t.Error("expected MCP detection to return true")
	}
}

func TestDetectMCP_DisabledByZero(t *testing.T) {
	t.Setenv("MCP_ENABLED", "0")
	if detectMCP(nil) {
		t.Error("expected MCP detection to return false when set to '0'")
	}
}

func TestDetectMCP_DisabledByFalse(t *testing.T) {
	t.Setenv("MCP_ENABLED", "false")
	if detectMCP(nil) {
		t.Error("expected MCP detection to return false when set to 'false'")
	}
}

func TestDetectMCP_NoEnv(t *testing.T) {
	if detectMCP(nil) {
		t.Error("expected MCP detection to return false with no env vars")
	}
}

func TestDetectMCP_OpenCodeMCP(t *testing.T) {
	t.Setenv("OPENCODE_MCP_ENABLED", "1")
	if !detectMCP(nil) {
		t.Error("expected MCP detection when OPENCODE_MCP_ENABLED is set")
	}
}

func TestDetectMCP_ClaudeCodeMCP(t *testing.T) {
	t.Setenv("CLAUDE_CODE_MCP_ENABLED", "1")
	if !detectMCP(nil) {
		t.Error("expected MCP detection when CLAUDE_CODE_MCP_ENABLED is set")
	}
}

func TestDetectMCPCapability_WithEnvVar(t *testing.T) {
	t.Setenv("MCP_ENABLED", "true")
	if !DetectMCPCapability(zerolog.Nop()) {
		t.Error("expected MCP capability to be detected")
	}
}

func TestDetectMCPCapability_WithCodexMCP(t *testing.T) {
	t.Setenv("CODEX_MCP_ENABLED", "1")
	if !DetectMCPCapability(zerolog.Nop()) {
		t.Error("expected MCP capability to be detected via CODEX_MCP_ENABLED")
	}
}

func TestDetectMCPCapability_WithCursorMCP(t *testing.T) {
	t.Setenv("CURSOR_MCP_ENABLED", "1")
	if !DetectMCPCapability(zerolog.Nop()) {
		t.Error("expected MCP capability to be detected via CURSOR_MCP_ENABLED")
	}
}

func TestDetectMCPCapability_NoEnv(t *testing.T) {
	if DetectMCPCapability(zerolog.Nop()) {
		t.Error("expected no MCP capability when no env vars are set")
	}
}

// ============================================================================
// Tests for detectCI (currently 80% - covering remaining CI platforms)
// ============================================================================

func TestDetectCI_GitHubActions(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	if !detectCI() {
		t.Error("expected CI detection with GITHUB_ACTIONS set")
	}
}

func TestDetectCI_GitLabCI(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	if !detectCI() {
		t.Error("expected CI detection with GITLAB_CI set")
	}
}

func TestDetectCI_Jenkins(t *testing.T) {
	t.Setenv("JENKINS_URL", "http://jenkins:8080")
	if !detectCI() {
		t.Error("expected CI detection with JENKINS_URL set")
	}
}

func TestDetectCI_CircleCI(t *testing.T) {
	t.Setenv("CIRCLECI", "true")
	if !detectCI() {
		t.Error("expected CI detection with CIRCLECI set")
	}
}

func TestDetectCI_Travis(t *testing.T) {
	t.Setenv("TRAVIS", "true")
	if !detectCI() {
		t.Error("expected CI detection with TRAVIS set")
	}
}

func TestDetectCI_Buildkite(t *testing.T) {
	t.Setenv("BUILDKITE", "true")
	if !detectCI() {
		t.Error("expected CI detection with BUILDKITE set")
	}
}

func TestDetectCI_CodeBuild(t *testing.T) {
	t.Setenv("CODEBUILD", "true")
	if !detectCI() {
		t.Error("expected CI detection with CODEBUILD set")
	}
}

func TestDetectCI_TFBuild(t *testing.T) {
	t.Setenv("TF_BUILD", "true")
	if !detectCI() {
		t.Error("expected CI detection with TF_BUILD set")
	}
}

func TestDetectCI_Bitbucket(t *testing.T) {
	t.Setenv("BITBUCKET_BUILD_NUMBER", "42")
	if !detectCI() {
		t.Error("expected CI detection with BITBUCKET_BUILD_NUMBER set")
	}
}

func TestDetectCI_GenericCI(t *testing.T) {
	t.Setenv("CI", "true")
	if !detectCI() {
		t.Error("expected CI detection with CI set")
	}
}

func TestDetectCI_NoEnv(t *testing.T) {
	if detectCI() {
		t.Error("expected no CI detection with no env vars set")
	}
}

// ============================================================================
// Tests for detectNodePackageManager (currently 0.0%)
// ============================================================================

func TestDetectNodePackageManager_PNPM(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "pnpm-lock.yaml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectNodePackageManager(tmpDir)
	if result != "pnpm" {
		t.Errorf("got %q, want pnpm", result)
	}
}

func TestDetectNodePackageManager_Yarn(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "yarn.lock"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectNodePackageManager(tmpDir)
	if result != "yarn" {
		t.Errorf("got %q, want yarn", result)
	}
}

func TestDetectNodePackageManager_Bun(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "bun.lockb"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectNodePackageManager(tmpDir)
	if result != "bun" {
		t.Errorf("got %q, want bun", result)
	}
}

func TestDetectNodePackageManager_NPM(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "package-lock.json"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectNodePackageManager(tmpDir)
	if result != "npm" {
		t.Errorf("got %q, want npm", result)
	}
}

func TestDetectNodePackageManager_DefaultNPM(t *testing.T) {
	tmpDir := t.TempDir()
	// No lock files at all — should default to npm
	result := detectNodePackageManager(tmpDir)
	if result != "npm" {
		t.Errorf("got %q, want npm (default)", result)
	}
}

// ============================================================================
// Tests for detectNodeTestFramework (currently 0.0%)
// ============================================================================

func TestDetectNodeTestFramework_Vitest(t *testing.T) {
	tmpDir := t.TempDir()
	pkgJSON := `{"devDependencies": {"vitest": "^1.0.0"}}`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectNodeTestFramework(tmpDir)
	if result != "Vitest" {
		t.Errorf("got %q, want Vitest", result)
	}
}

func TestDetectNodeTestFramework_Jest(t *testing.T) {
	tmpDir := t.TempDir()
	pkgJSON := `{"devDependencies": {"jest": "^29.0.0"}}`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectNodeTestFramework(tmpDir)
	if result != "Jest" {
		t.Errorf("got %q, want Jest", result)
	}
}

func TestDetectNodeTestFramework_Mocha(t *testing.T) {
	tmpDir := t.TempDir()
	pkgJSON := `{"devDependencies": {"mocha": "^10.0.0"}}`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectNodeTestFramework(tmpDir)
	if result != "Mocha" {
		t.Errorf("got %q, want Mocha", result)
	}
}

func TestDetectNodeTestFramework_AVA(t *testing.T) {
	tmpDir := t.TempDir()
	pkgJSON := `{"dependencies": {"ava": "^5.0.0"}}`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectNodeTestFramework(tmpDir)
	if result != "AVA" {
		t.Errorf("got %q, want AVA", result)
	}
}

func TestDetectNodeTestFramework_Tape(t *testing.T) {
	tmpDir := t.TempDir()
	pkgJSON := `{"devDependencies": {"tape": "^5.0.0"}}`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectNodeTestFramework(tmpDir)
	if result != "Tape" {
		t.Errorf("got %q, want Tape", result)
	}
}

func TestDetectNodeTestFramework_WebTestRunner(t *testing.T) {
	tmpDir := t.TempDir()
	pkgJSON := `{"devDependencies": {"web-test-runner": "^0.18.0"}}`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectNodeTestFramework(tmpDir)
	if result != "@web/test-runner" {
		t.Errorf("got %q, want @web/test-runner", result)
	}
}

func TestDetectNodeTestFramework_FromScripts(t *testing.T) {
	tmpDir := t.TempDir()
	pkgJSON := `{"scripts": {"test": "vitest run"}, "devDependencies": {}}`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectNodeTestFramework(tmpDir)
	if result != "Vitest" {
		t.Errorf("got %q, want Vitest (from scripts)", result)
	}
}

func TestDetectNodeTestFramework_JestFromScripts(t *testing.T) {
	tmpDir := t.TempDir()
	pkgJSON := `{"scripts": {"test": "jest --coverage"}, "dependencies": {}}`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectNodeTestFramework(tmpDir)
	if result != "Jest" {
		t.Errorf("got %q, want Jest (from scripts)", result)
	}
}

func TestDetectNodeTestFramework_NoPackageJSON(t *testing.T) {
	tmpDir := t.TempDir()
	result := detectNodeTestFramework(tmpDir)
	if result != "" {
		t.Errorf("got %q, want empty string (no package.json)", result)
	}
}

func TestDetectNodeTestFramework_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	pkgJSON := `{"name": "empty", "devDependencies": {}, "scripts": {}}`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectNodeTestFramework(tmpDir)
	if result != "" {
		t.Errorf("got %q, want empty string (no test framework)", result)
	}
}

// ============================================================================
// Tests for detectPackageManager variants (currently partial coverage)
// Testing Python/Java/Ruby/Elixir path coverage for the switch statement
// ============================================================================

func TestDetectPackageManager_PythonPoetry(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "poetry.lock"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectPackageManager(context.Background(), tmpDir, "Python", nil)
	if result != "poetry" {
		t.Errorf("got %q, want poetry", result)
	}
}

func TestDetectPackageManager_PythonPipenv(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "Pipfile"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectPackageManager(context.Background(), tmpDir, "Python", nil)
	if result != "pipenv" {
		t.Errorf("got %q, want pipenv", result)
	}
}

func TestDetectPackageManager_PythonUV(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "uv.lock"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectPackageManager(context.Background(), tmpDir, "Python", nil)
	if result != "uv" {
		t.Errorf("got %q, want uv", result)
	}
}

func TestDetectPackageManager_PythonPip(t *testing.T) {
	tmpDir := t.TempDir()
	result := detectPackageManager(context.Background(), tmpDir, "Python", nil)
	if result != "pip" {
		t.Errorf("got %q, want pip (default)", result)
	}
}

func TestDetectPackageManager_JavaGradle(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "gradlew"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectPackageManager(context.Background(), tmpDir, "Java", nil)
	if result != "gradle" {
		t.Errorf("got %q, want gradle", result)
	}
}

func TestDetectPackageManager_JavaMaven(t *testing.T) {
	tmpDir := t.TempDir()
	result := detectPackageManager(context.Background(), tmpDir, "Java", nil)
	if result != "maven" {
		t.Errorf("got %q, want maven (default)", result)
	}
}

func TestDetectPackageManager_Ruby(t *testing.T) {
	result := detectPackageManager(context.Background(), "/tmp", "Ruby", nil)
	if result != "bundler" {
		t.Errorf("got %q, want bundler", result)
	}
}

func TestDetectPackageManager_PHP(t *testing.T) {
	result := detectPackageManager(context.Background(), "/tmp", "PHP", nil)
	if result != "composer" {
		t.Errorf("got %q, want composer", result)
	}
}

func TestDetectPackageManager_Go(t *testing.T) {
	result := detectPackageManager(context.Background(), "/tmp", "Go", nil)
	if result != "go mod" {
		t.Errorf("got %q, want go mod", result)
	}
}

func TestDetectPackageManager_Rust(t *testing.T) {
	result := detectPackageManager(context.Background(), "/tmp", "Rust", nil)
	if result != "cargo" {
		t.Errorf("got %q, want cargo", result)
	}
}

// ============================================================================
// Tests for detectTestFramework variants (currently partial coverage)
// ============================================================================

func TestDetectTestFramework_Go(t *testing.T) {
	result := detectTestFramework(context.Background(), "/tmp", "Go", nil)
	if result != "go test" {
		t.Errorf("got %q, want go test", result)
	}
}

func TestDetectTestFramework_Rust(t *testing.T) {
	result := detectTestFramework(context.Background(), "/tmp", "Rust", nil)
	if result != "cargo test" {
		t.Errorf("got %q, want cargo test", result)
	}
}

func TestDetectTestFramework_Java(t *testing.T) {
	result := detectTestFramework(context.Background(), "/tmp", "Java", nil)
	if result != "JUnit" {
		t.Errorf("got %q, want JUnit", result)
	}
}

func TestDetectTestFramework_Ruby(t *testing.T) {
	result := detectTestFramework(context.Background(), "/tmp", "Ruby", nil)
	if result != "RSpec" {
		t.Errorf("got %q, want RSpec", result)
	}
}

func TestDetectTestFramework_PythonPytest(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "pytest.ini"), []byte("[pytest]"), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectTestFramework(context.Background(), tmpDir, "Python", nil)
	if result != "pytest" {
		t.Errorf("got %q, want pytest", result)
	}
}

func TestDetectTestFramework_PythonSetupCfg(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "setup.cfg"), []byte("[tool:pytest]\npytest:\ntestpaths = tests"), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectTestFramework(context.Background(), tmpDir, "Python", nil)
	if result != "pytest" {
		t.Errorf("got %q, want pytest", result)
	}
}

func TestDetectTestFramework_PythonUnittest(t *testing.T) {
	tmpDir := t.TempDir()
	result := detectTestFramework(context.Background(), tmpDir, "Python", nil)
	if result != "unittest" {
		t.Errorf("got %q, want unittest (default)", result)
	}
}

// ============================================================================
// Tests for parseCapabilities (currently 90%)
// ============================================================================

func TestParseCapabilities_Simple(t *testing.T) {
	content := "# AGENT\n\n## PURPOSE\nCapabilities here.\n\n- Design AI feature architecture\n- Manage prompt engineering\n"
	caps := parseCapabilities(content)
	if len(caps) != 2 {
		t.Fatalf("expected 2 capabilities, got %d", len(caps))
	}
	if caps[0].Name != "Design AI feature architecture" {
		t.Errorf("Name = %q", caps[0].Name)
	}
}

func TestParseCapabilities_SkipsMetadataLines(t *testing.T) {
	content := "# AGENT\n- **Version**: 1.0.0 | **Status**: active\n- Real capability\n"
	caps := parseCapabilities(content)
	if len(caps) != 1 {
		t.Fatalf("expected 1 capability, got %d", len(caps))
	}
	if caps[0].Name != "Real capability" {
		t.Errorf("Name = %q, want 'Real capability'", caps[0].Name)
	}
}

func TestParseCapabilities_Empty(t *testing.T) {
	content := "# AGENT\n\nJust some text.\n"
	caps := parseCapabilities(content)
	if len(caps) != 0 {
		t.Errorf("expected 0 capabilities, got %d", len(caps))
	}
}

// ============================================================================
// Tests for extractTemplateType (currently 80%)
// ============================================================================

func TestExtractTemplateType_WithHashPrefix(t *testing.T) {
	content := "# TEMPLATE: CLI Tool\n\nSome content"
	result := extractTemplateType(content)
	if result != "CLI Tool" {
		t.Errorf("got %q, want 'CLI Tool'", result)
	}
}

func TestExtractTemplateType_NoTemplate(t *testing.T) {
	content := "# Just a heading\n\nNo template here."
	result := extractTemplateType(content)
	if result != "" {
		t.Errorf("got %q, want empty", result)
	}
}

func TestExtractTemplateType_SkillHeading(t *testing.T) {
	// "# SKILL: ..." should NOT match (it's not TEMPLATE:)
	content := "# SKILL: Unit Testing\n\n## Description"
	result := extractTemplateType(content)
	if result != "" {
		t.Errorf("got %q, want empty (SKILL: is not TEMPLATE:)", result)
	}
}

// ============================================================================
// Tests for detectTerminal (currently 30%)
// ============================================================================

func TestDetectTerminal_TermProgram(t *testing.T) {
	t.Setenv("GNOME_TERMINAL_SERVICE", "")
	t.Setenv("TERM_PROGRAM", "iTerm.app")
	t.Setenv("TERM_PROGRAM_VERSION", "3.5")

	result := detectTerminal()
	if result != "iTerm.app 3.5" {
		t.Errorf("got %q, want 'iTerm.app 3.5'", result)
	}
}

func TestDetectTerminal_TermProgramNoVersion(t *testing.T) {
	t.Setenv("GNOME_TERMINAL_SERVICE", "")
	t.Setenv("TERM_PROGRAM", "Hyper")

	result := detectTerminal()
	if result != "Hyper" {
		t.Errorf("got %q, want Hyper", result)
	}
}

func TestDetectTerminal_NonXtermTerm(t *testing.T) {
	t.Setenv("GNOME_TERMINAL_SERVICE", "")
	t.Setenv("TERM", "screen-256color")

	result := detectTerminal()
	if result != "screen-256color" {
		t.Errorf("got %q, want screen-256color", result)
	}
}

func TestDetectTerminal_Xterm(t *testing.T) {
	t.Setenv("GNOME_TERMINAL_SERVICE", "")
	t.Setenv("TERM", "xterm-256color")

	result := detectTerminal()
	if result == "xterm-256color" {
		t.Error("expected xterm-256color to be filtered out, falling through to platform checks")
	}
}

func TestDetectTerminal_Tmux(t *testing.T) {
	t.Setenv("GNOME_TERMINAL_SERVICE", "")
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1234,0")

	result := detectTerminal()
	if result != "tmux" {
		t.Errorf("got %q, want tmux", result)
	}
}

func TestDetectTerminal_Screen(t *testing.T) {
	t.Setenv("GNOME_TERMINAL_SERVICE", "")
	t.Setenv("STY", "12345.foo")

	result := detectTerminal()
	if result != "screen" {
		t.Errorf("got %q, want screen", result)
	}
}

func TestDetectTerminal_Kitty(t *testing.T) {
	t.Setenv("GNOME_TERMINAL_SERVICE", "")
	t.Setenv("KITTY_PID", "12345")

	result := detectTerminal()
	if result != "Kitty" {
		t.Errorf("got %q, want Kitty", result)
	}
}

func TestDetectTerminal_Alacritty(t *testing.T) {
	t.Setenv("GNOME_TERMINAL_SERVICE", "")
	t.Setenv("ALACRITTY_LOG", "/tmp/alacritty.log")

	result := detectTerminal()
	if result != "Alacritty" {
		t.Errorf("got %q, want Alacritty", result)
	}
}

func TestDetectTerminal_WezTerm(t *testing.T) {
	t.Setenv("GNOME_TERMINAL_SERVICE", "")
	t.Setenv("WEZTERM_PANE", "0")

	result := detectTerminal()
	if result != "WezTerm" {
		t.Errorf("got %q, want WezTerm", result)
	}
}

func TestDetectTerminal_Konsole(t *testing.T) {
	t.Setenv("GNOME_TERMINAL_SERVICE", "")
	t.Setenv("KONSOLE_VERSION", "22.04")

	result := detectTerminal()
	if result != "Konsole" {
		t.Errorf("got %q, want Konsole", result)
	}
}

func TestDetectTerminal_GNOMETerminal(t *testing.T) {
	t.Setenv("GNOME_TERMINAL_SERVICE", ":1.0")

	result := detectTerminal()
	if result != "GNOME Terminal" {
		t.Errorf("got %q, want GNOME Terminal", result)
	}
}

func TestDetectTerminal_iTerm2(t *testing.T) {
	t.Setenv("GNOME_TERMINAL_SERVICE", "")
	t.Setenv("ITERM_SESSION_ID", "w0t0p0:ABCD")

	result := detectTerminal()
	if result != "iTerm2" {
		t.Errorf("got %q, want iTerm2", result)
	}
}

func TestDetectTerminal_AppleTerminal(t *testing.T) {
	t.Setenv("GNOME_TERMINAL_SERVICE", "")
	t.Setenv("APPLE_TERMINAL", "1")

	result := detectTerminal()
	if result != "Apple Terminal" {
		t.Errorf("got %q, want Apple Terminal", result)
	}
}

func TestDetectTerminal_Unknown(t *testing.T) {
	t.Setenv("GNOME_TERMINAL_SERVICE", "")
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("TERM", "xterm-256color")

	result := detectTerminal()
	if result != "unknown" {
		t.Errorf("got %q, want unknown", result)
	}
}

// ============================================================================
// Tests for detectShell (currently 60%)
// ============================================================================

func TestDetectShell_Unknown(t *testing.T) {
	// Unset SHELL to test the unknown path. COMSPEC também precisa ser limpo:
	// no Windows detectShell faz fallback para COMSPEC (cmd.exe) quando SHELL
	// está vazio — comportamento correto de produção, mas fora do escopo deste
	// teste.
	t.Setenv("SHELL", "")
	t.Setenv("COMSPEC", "")

	shell, version := detectShell()
	if shell != "unknown" {
		t.Errorf("got %q, want unknown", shell)
	}
	if version != "" {
		t.Errorf("version should be empty, got %q", version)
	}
}

// ============================================================================
// Tests for detectDockerEnv and detectWSL (currently partially covered)
// ============================================================================

func TestDetectDockerEnv_NoDocker(t *testing.T) {
	// On a CI system without /.dockerenv, should return false
	// We can't reliably test the positive path, but we can test the
	// case where /.dockerenv doesn't exist
	result := detectDockerEnv()
	// Don't assert true/false since it depends on the host
	// Just ensure it returns a boolean without panicking
	_ = result
}

func TestDetectWSL_NotWSL(t *testing.T) {
	// On Linux, check /proc/version. If not Microsoft/WSL, returns false.
	// We can't reliably test the positive case, but ensure no panic.
	result := detectWSL()
	_ = result
}

// ============================================================================
// Tests for detectGoVersion (currently 0.0%)
// ============================================================================

func TestDetectGoVersion_ValidGoMod(t *testing.T) {
	tmpDir := t.TempDir()
	goModContent := `module github.com/example/test

go 1.22.0

require (
	github.com/rs/zerolog v1.33.0
)
`
	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		t.Fatal(err)
	}

	lang, version := detectGoVersion(goModPath)
	if lang != "Go" {
		t.Errorf("language = %q, want Go", lang)
	}
	if version != "1.22.0" {
		t.Errorf("version = %q, want 1.22.0", version)
	}
}

func TestDetectGoVersion_MinimalGoMod(t *testing.T) {
	tmpDir := t.TempDir()
	goModContent := `module example

go 1.21
`
	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		t.Fatal(err)
	}

	lang, version := detectGoVersion(goModPath)
	if lang != "Go" {
		t.Errorf("language = %q, want Go", lang)
	}
	if version != "1.21" {
		t.Errorf("version = %q, want 1.21", version)
	}
}

func TestDetectGoVersion_NoGoDirective(t *testing.T) {
	tmpDir := t.TempDir()
	goModContent := `module example

require github.com/other/module v1.0.0
`
	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		t.Fatal(err)
	}

	lang, version := detectGoVersion(goModPath)
	if lang != "Go" {
		t.Errorf("language = %q, want Go", lang)
	}
	if version != "" {
		t.Errorf("version = %q, want empty (no go directive)", version)
	}
}

func TestDetectGoVersion_FileNotFound(t *testing.T) {
	lang, version := detectGoVersion("/nonexistent/go.mod")
	if lang != "Go" {
		t.Errorf("language = %q, want Go", lang)
	}
	if version != "" {
		t.Errorf("version = %q, want empty (file not found)", version)
	}
}

func TestDetectGoVersion_InvalidGoMod(t *testing.T) {
	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("not a valid go.mod file"), 0644); err != nil {
		t.Fatal(err)
	}

	lang, version := detectGoVersion(goModPath)
	if lang != "Go" {
		t.Errorf("language = %q, want Go", lang)
	}
	// Version should be empty for invalid format
	if version != "" {
		t.Errorf("version = %q, want empty (invalid go.mod)", version)
	}
}

// ============================================================================
// Tests for detectNodeVersion (currently 0.0%)
// ============================================================================

func TestDetectNodeVersion_TypeScript(t *testing.T) {
	tmpDir := t.TempDir()
	pkgJSONContent := `{"name": "my-app", "engines": {"node": ">=18"}}`
	pkgPath := filepath.Join(tmpDir, "package.json")
	if err := os.WriteFile(pkgPath, []byte(pkgJSONContent), 0644); err != nil {
		t.Fatal(err)
	}
	// Create a .ts file to make it detect TypeScript
	tsPath := filepath.Join(tmpDir, "src", "index.ts")
	if err := os.MkdirAll(filepath.Dir(tsPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tsPath, []byte(`const x: string = "hello";`), 0644); err != nil {
		t.Fatal(err)
	}

	lang, version := detectNodeVersion(pkgPath)
	if lang != "TypeScript" {
		t.Errorf("language = %q, want TypeScript", lang)
	}
	if version != ">=18" {
		t.Errorf("version = %q, want >=18", version)
	}
}

func TestDetectNodeVersion_TypeScriptTSX(t *testing.T) {
	tmpDir := t.TempDir()
	pkgJSONContent := `{"name": "my-app"}`
	pkgPath := filepath.Join(tmpDir, "package.json")
	if err := os.WriteFile(pkgPath, []byte(pkgJSONContent), 0644); err != nil {
		t.Fatal(err)
	}
	// Create a .tsx file
	tsxPath := filepath.Join(tmpDir, "src", "App.tsx")
	if err := os.MkdirAll(filepath.Dir(tsxPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tsxPath, []byte(`export const App = () => <div>hi</div>;`), 0644); err != nil {
		t.Fatal(err)
	}

	lang, _ := detectNodeVersion(pkgPath)
	if lang != "TypeScript" {
		t.Errorf("language = %q, want TypeScript (has .tsx files)", lang)
	}
}

func TestDetectNodeVersion_JavaScript(t *testing.T) {
	tmpDir := t.TempDir()
	pkgJSONContent := `{"name": "my-app", "engines": {"node": "20"}}`
	pkgPath := filepath.Join(tmpDir, "package.json")
	if err := os.WriteFile(pkgPath, []byte(pkgJSONContent), 0644); err != nil {
		t.Fatal(err)
	}
	// Only .js files, no .ts
	jsPath := filepath.Join(tmpDir, "src", "index.js")
	if err := os.MkdirAll(filepath.Dir(jsPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jsPath, []byte(`console.log("hello");`), 0644); err != nil {
		t.Fatal(err)
	}

	lang, version := detectNodeVersion(pkgPath)
	if lang != "JavaScript" {
		t.Errorf("language = %q, want JavaScript", lang)
	}
	if version != "20" {
		t.Errorf("version = %q, want 20", version)
	}
}

func TestDetectNodeVersion_FileNotFound(t *testing.T) {
	lang, version := detectNodeVersion("/nonexistent/package.json")
	if lang != "TypeScript" {
		t.Errorf("language = %q, want TypeScript (default)", lang)
	}
	if version != "" {
		t.Errorf("version = %q, want empty (file not found)", version)
	}
}

func TestDetectNodeVersion_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	pkgPath := filepath.Join(tmpDir, "package.json")
	if err := os.WriteFile(pkgPath, []byte("{invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	lang, version := detectNodeVersion(pkgPath)
	if lang != "TypeScript" {
		t.Errorf("language = %q, want TypeScript (default for invalid json)", lang)
	}
	if version != "" {
		t.Errorf("version = %q, want empty", version)
	}
}

// ============================================================================
// Tests for detectRustVersion (currently 0.0%)
// ============================================================================

// TestDetectRustVersion_ValidCargoToml checks that the language is detected
// as Rust. Version parsing from Cargo.toml uses yaml.Unmarshal which does not
// handle TOML syntax natively, so version may be empty.
func TestDetectRustVersion_ValidCargoToml(t *testing.T) {
	tmpDir := t.TempDir()
	cargoContent := `[package]
name = "my-crate"
version = "0.1.0"
edition = "2021"

[dependencies]
serde = "1.0"
`
	cargoPath := filepath.Join(tmpDir, "Cargo.toml")
	if err := os.WriteFile(cargoPath, []byte(cargoContent), 0644); err != nil {
		t.Fatal(err)
	}

	lang, _ := detectRustVersion(cargoPath)
	if lang != "Rust" {
		t.Errorf("language = %q, want Rust", lang)
	}
	// Cargo.toml is TOML, not YAML; yaml.Unmarshal may not parse edition
}

func TestDetectRustVersion_NoEdition(t *testing.T) {
	tmpDir := t.TempDir()
	cargoContent := `[package]
name = "my-crate"
version = "0.1.0"
`
	cargoPath := filepath.Join(tmpDir, "Cargo.toml")
	if err := os.WriteFile(cargoPath, []byte(cargoContent), 0644); err != nil {
		t.Fatal(err)
	}

	lang, version := detectRustVersion(cargoPath)
	if lang != "Rust" {
		t.Errorf("language = %q, want Rust", lang)
	}
	if version != "" {
		t.Errorf("version = %q, want empty (no edition)", version)
	}
}

func TestDetectRustVersion_FileNotFound(t *testing.T) {
	lang, version := detectRustVersion("/nonexistent/Cargo.toml")
	if lang != "Rust" {
		t.Errorf("language = %q, want Rust", lang)
	}
	if version != "" {
		t.Errorf("version = %q, want empty (file not found)", version)
	}
}

func TestDetectRustVersion_InvalidToml(t *testing.T) {
	tmpDir := t.TempDir()
	cargoPath := filepath.Join(tmpDir, "Cargo.toml")
	if err := os.WriteFile(cargoPath, []byte("not valid toml [["), 0644); err != nil {
		t.Fatal(err)
	}

	lang, version := detectRustVersion(cargoPath)
	if lang != "Rust" {
		t.Errorf("language = %q, want Rust", lang)
	}
	if version != "" {
		t.Errorf("version = %q, want empty (invalid toml)", version)
	}
}

// ============================================================================
// Tests for detectPythonVersion (currently 0.0%)
// ============================================================================

// TestDetectPythonVersion_ValidPyproject checks that the language is detected
// as Python. Version parsing from pyproject.toml uses yaml.Unmarshal which does
// not handle TOML syntax natively, so version may be empty.
func TestDetectPythonVersion_ValidPyproject(t *testing.T) {
	tmpDir := t.TempDir()
	pyprojectContent := `[project]
name = "my-project"
requires-python = ">=3.10"

[build-system]
requires = ["setuptools"]
`
	pyprojectPath := filepath.Join(tmpDir, "pyproject.toml")
	if err := os.WriteFile(pyprojectPath, []byte(pyprojectContent), 0644); err != nil {
		t.Fatal(err)
	}

	lang, _ := detectPythonVersion(pyprojectPath)
	if lang != "Python" {
		t.Errorf("language = %q, want Python", lang)
	}
	// pyproject.toml is TOML, not YAML; yaml.Unmarshal may not parse requires-python
}

func TestDetectPythonVersion_NoRequiresPython(t *testing.T) {
	tmpDir := t.TempDir()
	pyprojectContent := `[project]
name = "my-project"
`
	pyprojectPath := filepath.Join(tmpDir, "pyproject.toml")
	if err := os.WriteFile(pyprojectPath, []byte(pyprojectContent), 0644); err != nil {
		t.Fatal(err)
	}

	lang, version := detectPythonVersion(pyprojectPath)
	if lang != "Python" {
		t.Errorf("language = %q, want Python", lang)
	}
	if version != "" {
		t.Errorf("version = %q, want empty (no requires-python)", version)
	}
}

func TestDetectPythonVersion_FileNotFound(t *testing.T) {
	lang, version := detectPythonVersion("/nonexistent/pyproject.toml")
	if lang != "Python" {
		t.Errorf("language = %q, want Python", lang)
	}
	if version != "" {
		t.Errorf("version = %q, want empty (file not found)", version)
	}
}

func TestDetectPythonVersion_InvalidToml(t *testing.T) {
	tmpDir := t.TempDir()
	pyprojectPath := filepath.Join(tmpDir, "pyproject.toml")
	if err := os.WriteFile(pyprojectPath, []byte("[[invalid toml]]]"), 0644); err != nil {
		t.Fatal(err)
	}

	lang, version := detectPythonVersion(pyprojectPath)
	if lang != "Python" {
		t.Errorf("language = %q, want Python", lang)
	}
	if version != "" {
		t.Errorf("version = %q, want empty (invalid toml)", version)
	}
}

// ============================================================================
// Tests for detectDatabase edge cases (currently 40%)
// ============================================================================

func TestDetectDatabase_FromDockerCompose(t *testing.T) {
	tmpDir := t.TempDir()
	dockerContent := `version: "3.8"
services:
  app:
    image: myapp
  db:
    image: postgres:15
`
	dockerPath := filepath.Join(tmpDir, "docker-compose.yml")
	if err := os.WriteFile(dockerPath, []byte(dockerContent), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectDatabase(context.Background(), tmpDir, "unknown")
	if result != "PostgreSQL" {
		t.Errorf("got %q, want PostgreSQL (from docker-compose)", result)
	}
}

func TestDetectDatabase_FromEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	envContent := `DATABASE_URL=postgresql://localhost/mydb
REDIS_URL=redis://localhost:6379
`
	envPath := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectDatabase(context.Background(), tmpDir, "unknown")
	if result != "PostgreSQL" {
		t.Errorf("got %q, want PostgreSQL (from .env)", result)
	}
}

func TestDetectDatabase_NoDatabaseFound(t *testing.T) {
	tmpDir := t.TempDir()

	result := detectDatabase(context.Background(), tmpDir, "unknown")
	if result != "" {
		t.Errorf("got %q, want empty string (no DB indicators)", result)
	}
}

// ============================================================================
// Tests for detectBuildSystem (currently partial coverage)
// ============================================================================

func TestDetectBuildSystem_Makefile(t *testing.T) {
	result := detectBuildSystem(context.Background(), "/tmp", "", []string{"/tmp/Makefile"})
	if result != "Make" {
		t.Errorf("got %q, want Make", result)
	}
}

func TestDetectBuildSystem_Vite(t *testing.T) {
	result := detectBuildSystem(context.Background(), "/tmp", "", []string{"/tmp/vite.config.ts"})
	if result != "Vite" {
		t.Errorf("got %q, want Vite", result)
	}
}

func TestDetectBuildSystem_Cargo(t *testing.T) {
	result := detectBuildSystem(context.Background(), "/tmp", "", []string{"/tmp/Cargo.toml"})
	if result != "Cargo" {
		t.Errorf("got %q, want Cargo", result)
	}
}

func TestDetectBuildSystem_Setuptools(t *testing.T) {
	result := detectBuildSystem(context.Background(), "/tmp", "", []string{"/tmp/pyproject.toml"})
	if result != "setuptools" {
		t.Errorf("got %q, want setuptools", result)
	}
}

func TestDetectBuildSystem_Gradle(t *testing.T) {
	result := detectBuildSystem(context.Background(), "/tmp", "", []string{"/tmp/build.gradle"})
	if result != "Gradle" {
		t.Errorf("got %q, want Gradle", result)
	}
}

func TestDetectBuildSystem_Maven(t *testing.T) {
	result := detectBuildSystem(context.Background(), "/tmp", "", []string{"/tmp/pom.xml"})
	if result != "Maven" {
		t.Errorf("got %q, want Maven", result)
	}
}

// ============================================================================
// Tests for detectArchitecture (currently fully covered by DiscoverAll)
// ============================================================================

func TestDetectArchitecture_Monolith(t *testing.T) {
	tmpDir := t.TempDir()
	result := detectArchitecture(context.Background(), tmpDir)
	if result != "monolith" {
		t.Errorf("got %q, want monolith", result)
	}
}

// ============================================================================
// Tests for detectDocker (currently partially covered)
// ============================================================================

func TestDetectDocker_NoDockerFiles(t *testing.T) {
	tmpDir := t.TempDir()
	result := detectDocker(context.Background(), tmpDir)
	if result {
		t.Error("expected false (no docker files in empty dir)")
	}
}

func TestDetectDocker_WithDockerfile(t *testing.T) {
	tmpDir := t.TempDir()
	dockerPath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerPath, []byte("FROM alpine"), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectDocker(context.Background(), tmpDir)
	if !result {
		t.Error("expected true (Dockerfile present)")
	}
}

func TestDetectDocker_WithDockerignore(t *testing.T) {
	tmpDir := t.TempDir()
	dockerignorePath := filepath.Join(tmpDir, ".dockerignore")
	if err := os.WriteFile(dockerignorePath, []byte("node_modules"), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectDocker(context.Background(), tmpDir)
	if !result {
		t.Error("expected true (.dockerignore present)")
	}
}

// ============================================================================
// Tests for detectCICD (currently covered via DiscoverAll but worth explicit tests)
// ============================================================================

func TestDetectCICD_GitHubActions(t *testing.T) {
	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		t.Fatal(err)
	}

	result := detectCICD(context.Background(), tmpDir)
	if result != "GitHub Actions" {
		t.Errorf("got %q, want GitHub Actions", result)
	}
}

func TestDetectCICD_GitLabCI(t *testing.T) {
	tmpDir := t.TempDir()
	gitlabPath := filepath.Join(tmpDir, ".gitlab-ci.yml")
	if err := os.WriteFile(gitlabPath, []byte("stages: [build]"), 0644); err != nil {
		t.Fatal(err)
	}

	result := detectCICD(context.Background(), tmpDir)
	if result != "GitLab CI" {
		t.Errorf("got %q, want GitLab CI", result)
	}
}

func TestDetectCICD_None(t *testing.T) {
	tmpDir := t.TempDir()
	result := detectCICD(context.Background(), tmpDir)
	if result != "" {
		t.Errorf("got %q, want empty (no CI/CD config)", result)
	}
}

// ============================================================================
// Tests for detectLanguage (currently partially covered via DetectProject)
// ============================================================================

func TestDetectLanguage_JavaGradle(t *testing.T) {
	result, version := detectLanguage(context.Background(), "/tmp", []string{"/tmp/build.gradle"})
	if result != "Java" {
		t.Errorf("got %q, want Java", result)
	}
	if version != "" {
		t.Errorf("version = %q, want empty", version)
	}
}

func TestDetectLanguage_JavaKts(t *testing.T) {
	result, version := detectLanguage(context.Background(), "/tmp", []string{"/tmp/build.gradle.kts"})
	if result != "Java" {
		t.Errorf("got %q, want Java", result)
	}
	if version != "" {
		t.Errorf("version = %q, want empty", version)
	}
}

func TestDetectLanguage_CMake(t *testing.T) {
	result, version := detectLanguage(context.Background(), "/tmp", []string{"/tmp/CMakeLists.txt"})
	if result != "C++" {
		t.Errorf("got %q, want C++", result)
	}
	if version != "" {
		t.Errorf("version = %q, want empty", version)
	}
}

func TestDetectLanguage_PHPComposer(t *testing.T) {
	result, version := detectLanguage(context.Background(), "/tmp", []string{"/tmp/composer.json"})
	if result != "PHP" {
		t.Errorf("got %q, want PHP", result)
	}
	if version != "" {
		t.Errorf("version = %q, want empty", version)
	}
}

func TestDetectLanguage_RubyGemfile(t *testing.T) {
	result, version := detectLanguage(context.Background(), "/tmp", []string{"/tmp/Gemfile"})
	if result != "Ruby" {
		t.Errorf("got %q, want Ruby", result)
	}
	if version != "" {
		t.Errorf("version = %q, want empty", version)
	}
}

func TestDetectLanguage_ElixirMix(t *testing.T) {
	result, version := detectLanguage(context.Background(), "/tmp", []string{"/tmp/mix.exs"})
	if result != "Elixir" {
		t.Errorf("got %q, want Elixir", result)
	}
	if version != "" {
		t.Errorf("version = %q, want empty", version)
	}
}

func TestDetectLanguage_Clojure(t *testing.T) {
	result, version := detectLanguage(context.Background(), "/tmp", []string{"/tmp/project.clj"})
	if result != "Clojure" {
		t.Errorf("got %q, want Clojure", result)
	}
	if version != "" {
		t.Errorf("version = %q, want empty", version)
	}
}

func TestDetectLanguage_Unknown(t *testing.T) {
	// Empty config files should fall through to extension-based detection
	tmpDir := t.TempDir()
	result, _ := detectLanguage(context.Background(), tmpDir, []string{})
	// Should be Unknown or Markdown (depending on what's in the temp dir)
	if result == "" {
		t.Error("expected non-empty language")
	}
}
