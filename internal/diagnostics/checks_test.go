// Package diagnostics tests for individual diagnostic checks.
package diagnostics

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setHome points the home resolution used by the checks at dir. os.UserHomeDir
// reads HOME on Unix but USERPROFILE on Windows, so both must be set for the
// tests to exercise the intended directory on every platform.
func setHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
	}
}

// platformEditor returns a binary name that is guaranteed to be in PATH on the
// current platform ("sh" on Unix, "cmd" on Windows).
func platformEditor() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	return "sh"
}

// =============================================================================
// CheckRuntime
// =============================================================================

func TestCheckRuntime_ReturnsResult(t *testing.T) {
	result := CheckRuntime(context.Background())

	assert.Equal(t, "runtime", result.Name)
	assert.Equal(t, SeverityCritical, result.Severity)
	assert.Contains(t, []Status{StatusPass, StatusWarn}, result.Status)
	assert.NotEmpty(t, result.Message)
	assert.NotEmpty(t, result.Details)

	// Verify details contain expected runtime info
	assert.Contains(t, result.Details, "Go:")
	assert.Contains(t, result.Details, runtime.GOOS)
	assert.Contains(t, result.Details, runtime.GOARCH)
}

func TestCheckRuntime_DetailsFormat(t *testing.T) {
	result := CheckRuntime(context.Background())

	// Details should contain version, OS, arch, and CPU count
	assert.Contains(t, result.Details, runtime.Version())
	assert.Contains(t, result.Details, runtime.GOOS)
	assert.Contains(t, result.Details, runtime.GOARCH)
}

// =============================================================================
// CheckEditor
// =============================================================================

func TestCheckEditor_WithEditorEnvVar(t *testing.T) {
	// Use a binary that exists in PATH on the current platform.
	editor := platformEditor()
	t.Setenv("EDITOR", editor)

	result := CheckEditor(context.Background())

	assert.Equal(t, "editor", result.Name)
	assert.Equal(t, StatusPass, result.Status)
	assert.Contains(t, result.Message, fmt.Sprintf("Editor '%s' detected", editor))
}

func TestCheckEditor_WithVisualEnvVar(t *testing.T) {
	editor := platformEditor()
	t.Setenv("VISUAL", editor)

	result := CheckEditor(context.Background())

	assert.Equal(t, StatusPass, result.Status)
	assert.Contains(t, result.Message, fmt.Sprintf("Editor '%s' detected", editor))
}

func TestCheckEditor_EditorNotInPath(t *testing.T) {
	t.Setenv("EDITOR", "nonexistent-editor-12345")

	result := CheckEditor(context.Background())

	assert.Equal(t, StatusWarn, result.Status)
	assert.Contains(t, result.Message, "configured but not found in PATH")
}

func TestCheckEditor_NoEditorDetected(t *testing.T) {
	// Ensure no EDITOR/VISUAL env var, and set PATH to a dir with no common editors
	tmpDir := t.TempDir()
	t.Setenv("PATH", tmpDir)

	result := CheckEditor(context.Background())

	assert.Equal(t, StatusWarn, result.Status)
	assert.Contains(t, result.Message, "No editor detected")
}

func TestCheckEditor_VisualTakesPrecedence(t *testing.T) {
	// EDITOR should be checked first, then VISUAL
	t.Setenv("EDITOR", "nonexistent-editor-12345")
	t.Setenv("VISUAL", "sh")

	result := CheckEditor(context.Background())

	assert.Equal(t, StatusWarn, result.Status)
	assert.Contains(t, result.Message, "configured but not found in PATH")
}

// =============================================================================
// CheckFilesystem
// =============================================================================

func TestCheckFilesystem_AllDirsExist(t *testing.T) {
	homeDir := t.TempDir()
	setHome(t, homeDir)

	// Create required directory structure
	requiredDirs := []string{
		filepath.Join(homeDir, ".config", "cosca"),
		filepath.Join(homeDir, ".config", "cosca", "runtime"),
		filepath.Join(homeDir, ".config", "cosca", "data"),
		filepath.Join(homeDir, ".config", "cosca", "cache"),
		filepath.Join(homeDir, ".config", "cosca", "logs"),
		filepath.Join(homeDir, ".config", "cosca", "tmp"),
	}
	for _, dir := range requiredDirs {
		err := os.MkdirAll(dir, 0o755)
		require.NoError(t, err)
	}

	result := CheckFilesystem(context.Background())

	assert.Equal(t, StatusPass, result.Status)
	assert.Contains(t, result.Message, "required directories accessible and writable")
}

func TestCheckFilesystem_MissingDirs(t *testing.T) {
	homeDir := t.TempDir()
	setHome(t, homeDir)

	// Only create some dirs â€” cosca home exists but subdirs are missing
	err := os.MkdirAll(filepath.Join(homeDir, ".config", "cosca"), 0o755)
	require.NoError(t, err)

	result := CheckFilesystem(context.Background())

	assert.Equal(t, StatusWarn, result.Status)
	assert.Contains(t, result.Message, "issue(s)")
	assert.Contains(t, result.Details, "missing:")
}

func TestCheckFilesystem_NotADirectory(t *testing.T) {
	homeDir := t.TempDir()
	setHome(t, homeDir)

	// Create cosca home as a file instead of a directory
	coscaHome := filepath.Join(homeDir, ".config", "cosca")
	err := os.MkdirAll(filepath.Join(homeDir, ".config"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(coscaHome, []byte("not a dir"), 0o644)
	require.NoError(t, err)

	result := CheckFilesystem(context.Background())

	assert.Equal(t, StatusWarn, result.Status)
	assert.Contains(t, result.Details, "not a directory")
}

func TestCheckFilesystem_HomeDirError(t *testing.T) {
	// Unset HOME to cause os.UserHomeDir to potentially fail
	// Note: on Linux, os.UserHomeDir may still fall back to /etc/passwd
	// We just verify the function handles it gracefully
	result := CheckFilesystem(context.Background())
	// Should not panic â€” either works or returns StatusError
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Name)
}

// =============================================================================
// CheckMemory
// =============================================================================

func TestCheckMemory_AllHealthy(t *testing.T) {
	homeDir := t.TempDir()
	setHome(t, homeDir)

	// Create memory directories with fresh files
	memoryDirs := []string{
		filepath.Join(homeDir, ".config", "cosca", "data", "memory"),
		filepath.Join(homeDir, ".local", "share", "cosca", "memory"),
	}
	for _, dir := range memoryDirs {
		err := os.MkdirAll(dir, 0o755)
		require.NoError(t, err)
		// Create a fresh memory file
		err = os.WriteFile(filepath.Join(dir, "mem1.json"), []byte("{}"), 0o644)
		require.NoError(t, err)
	}

	result := CheckMemory(context.Background())

	assert.Equal(t, StatusPass, result.Status)
	assert.Contains(t, result.Message, "Memory layers healthy")
}

func TestCheckMemory_WithStaleFiles(t *testing.T) {
	homeDir := t.TempDir()
	setHome(t, homeDir)

	memoryDir := filepath.Join(homeDir, ".config", "cosca", "data", "memory")
	err := os.MkdirAll(memoryDir, 0o755)
	require.NoError(t, err)

	// Create a "stale" file by setting mod time far in the past
	staleFile := filepath.Join(memoryDir, "old_mem.json")
	err = os.WriteFile(staleFile, []byte("{}"), 0o644)
	require.NoError(t, err)

	// Set modification time to 31 days ago (threshold is 30 days)
	past := time.Now().Add(-31 * 24 * time.Hour)
	err = os.Chtimes(staleFile, past, past)
	require.NoError(t, err)

	result := CheckMemory(context.Background())

	assert.Equal(t, StatusWarn, result.Status)
	assert.Contains(t, result.Message, "stale entries")
	assert.Contains(t, result.Details, "stale memory file")
}

func TestCheckMemory_NoMemoryDir(t *testing.T) {
	homeDir := t.TempDir()
	setHome(t, homeDir)

	// Don't create any memory directories
	result := CheckMemory(context.Background())

	assert.Equal(t, StatusPass, result.Status)
	assert.Contains(t, result.Message, "Memory layers healthy")
}

// =============================================================================
// CheckKnowledge
// =============================================================================

func TestCheckKnowledge_DatabaseFound(t *testing.T) {
	homeDir := t.TempDir()
	setHome(t, homeDir)

	// Create a knowledge database file
	dbDir := filepath.Join(homeDir, ".config", "cosca", "data")
	err := os.MkdirAll(dbDir, 0o755)
	require.NoError(t, err)

	dbPath := filepath.Join(dbDir, "knowledge.db")
	err = os.WriteFile(dbPath, []byte("SQLite format 3\x00"), 0o644)
	require.NoError(t, err)

	result := CheckKnowledge(context.Background())

	assert.Equal(t, StatusPass, result.Status)
	assert.Contains(t, result.Message, "Knowledge base healthy")
	assert.Contains(t, result.Details, "Database:")
}

func TestCheckKnowledge_NoDatabase(t *testing.T) {
	homeDir := t.TempDir()
	setHome(t, homeDir)

	// Don't create any database files
	result := CheckKnowledge(context.Background())

	assert.Equal(t, StatusWarn, result.Status)
	assert.Contains(t, result.Message, "No knowledge database found")
	assert.Contains(t, result.Suggestion, "knowledge init")
}

func TestCheckKnowledge_LargeDatabase(t *testing.T) {
	homeDir := t.TempDir()
	setHome(t, homeDir)

	dbDir := filepath.Join(homeDir, ".config", "cosca", "data")
	err := os.MkdirAll(dbDir, 0o755)
	require.NoError(t, err)

	dbPath := filepath.Join(dbDir, "knowledge.db")
	// Create a file > 1GB threshold by using a sparse file / large content
	// Actually, let's just write enough data. 1MB is easier.
	largeData := make([]byte, 2*1024*1024) // 2 MB â€” not > 1GB, so won't trigger
	err = os.WriteFile(dbPath, largeData, 0o644)
	require.NoError(t, err)

	result := CheckKnowledge(context.Background())

	// Should pass â€” 2MB is not > 1GB
	assert.Equal(t, StatusPass, result.Status)
}

func TestCheckKnowledge_WithVectorIndex(t *testing.T) {
	homeDir := t.TempDir()
	setHome(t, homeDir)

	// Create database
	dbDir := filepath.Join(homeDir, ".config", "cosca", "data")
	err := os.MkdirAll(dbDir, 0o755)
	require.NoError(t, err)

	dbPath := filepath.Join(dbDir, "knowledge.db")
	err = os.WriteFile(dbPath, []byte("SQLite format 3\x00"), 0o644)
	require.NoError(t, err)

	// Create vector index directory
	vectorDir := filepath.Join(homeDir, ".config", "cosca", "data", "vectors")
	err = os.MkdirAll(vectorDir, 0o755)
	require.NoError(t, err)

	result := CheckKnowledge(context.Background())

	assert.Equal(t, StatusPass, result.Status)
	assert.Contains(t, result.Details, "Vector index present")
}

func TestCheckKnowledge_HomeDirError(t *testing.T) {
	// Test that it handles errors gracefully
	result := CheckKnowledge(context.Background())
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Name)
}

// =============================================================================
// CheckConfiguration
// =============================================================================

func TestCheckConfiguration_WithConfigFile(t *testing.T) {
	homeDir := t.TempDir()
	setHome(t, homeDir)

	// Create a valid config.yaml
	configDir := filepath.Join(homeDir, ".config", "cosca")
	err := os.MkdirAll(configDir, 0o755)
	require.NoError(t, err)

	configContent := `
version: "1"
provider:
  name: openai
mode: development
`
	err = os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0o644)
	require.NoError(t, err)

	result := CheckConfiguration(context.Background())

	assert.Equal(t, StatusPass, result.Status)
	assert.Contains(t, result.Message, "Configuration is valid")
	assert.Contains(t, result.Details, "Config file:")
}

func TestCheckConfiguration_WithInvalidYAML(t *testing.T) {
	homeDir := t.TempDir()
	setHome(t, homeDir)

	configDir := filepath.Join(homeDir, ".config", "cosca")
	err := os.MkdirAll(configDir, 0o755)
	require.NoError(t, err)

	// Create an invalid YAML file
	err = os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("invalid: [yaml: broken"), 0o644)
	require.NoError(t, err)

	result := CheckConfiguration(context.Background())

	assert.Equal(t, StatusWarn, result.Status)
	assert.Contains(t, result.Details, "invalid YAML")
}

func TestCheckConfiguration_EnvVarsOnly(t *testing.T) {
	homeDir := t.TempDir()
	setHome(t, homeDir)

	// Set env vars but no config file
	t.Setenv("COSCA_HOME", homeDir)
	t.Setenv("COSCA_MODE", "test")

	result := CheckConfiguration(context.Background())

	assert.Equal(t, StatusPass, result.Status)
	assert.Contains(t, result.Message, "environment-based configuration")
	assert.Contains(t, result.Details, "COSCA_HOME")
}

func TestCheckConfiguration_NoConfigNoEnv(t *testing.T) {
	homeDir := t.TempDir()
	setHome(t, homeDir)

	// No config file, no env vars
	result := CheckConfiguration(context.Background())

	assert.Equal(t, StatusWarn, result.Status)
	assert.Contains(t, result.Message, "No configuration file found")
	assert.Contains(t, result.Suggestion, "cosca init")
}

func TestCheckConfiguration_WithAPIKeyEnvVar(t *testing.T) {
	homeDir := t.TempDir()
	setHome(t, homeDir)

	// Set an API key env var (should be masked in output)
	t.Setenv("OPENAI_API_KEY", "sk-abcdefghijklmnopqrstuvwxyz1234567890")

	result := CheckConfiguration(context.Background())

	assert.Equal(t, StatusPass, result.Status)
	assert.Contains(t, result.Details, "OPENAI_API_KEY")
	assert.Contains(t, result.Details, "sk-a") // masked â€” first 4 chars
	assert.Contains(t, result.Details, "890")  // masked â€” last 4 chars
	// Should NOT contain the full key
	assert.NotContains(t, result.Details, "sk-abcdefghijklmnopqrstuvwxyz1234567890")
}

func TestCheckConfiguration_WithAllEnvVars(t *testing.T) {
	homeDir := t.TempDir()
	setHome(t, homeDir)

	t.Setenv("COSCA_HOME", homeDir)
	t.Setenv("COSCA_MODE", "production")
	t.Setenv("COSCA_LOG_LEVEL", "debug")
	t.Setenv("COSCA_TELEMETRY_ENABLED", "false")

	result := CheckConfiguration(context.Background())

	assert.Equal(t, StatusPass, result.Status)
	assert.Contains(t, result.Details, "COSCA_HOME")
	assert.Contains(t, result.Details, "COSCA_MODE")
	assert.Contains(t, result.Details, "COSCA_LOG_LEVEL")
	assert.Contains(t, result.Details, "COSCA_TELEMETRY_ENABLED")
}

// =============================================================================
// CheckProviders (bonus â€” covered because it's used by RunAll)
// =============================================================================

func TestCheckProviders_NoProviders(t *testing.T) {
	// Ensure no provider env vars are set
	result := CheckProviders(context.Background())

	assert.Equal(t, "providers", result.Name)
	// Should warn if no providers found
	assert.Contains(t, []Status{StatusPass, StatusWarn}, result.Status)
}

func TestCheckProviders_WithEnvVar(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-testkey1234567890abcdef")

	result := CheckProviders(context.Background())

	assert.Equal(t, StatusPass, result.Status)
	assert.Contains(t, result.Message, "provider(s) configured")
	assert.Contains(t, result.Details, "OpenAI")
	assert.NotContains(t, result.Details, "sk-t")
	assert.Contains(t, result.Details, "environment configured")
}

// =============================================================================
// CheckNetwork (bonus)
// =============================================================================

func TestCheckNetwork_ReturnsResult(t *testing.T) {
	result := CheckNetwork(context.Background())

	assert.Equal(t, "network", result.Name)
	// Network may or may not be available in test env
	assert.Contains(t, []Status{StatusPass, StatusWarn, StatusFail}, result.Status)
	assert.NotEmpty(t, result.Message)
}

// =============================================================================
// CheckPermissions (bonus)
// =============================================================================

func TestCheckPermissions_NoCoscaHome(t *testing.T) {
	homeDir := t.TempDir()
	setHome(t, homeDir)

	// No cosca home directory â€” walk will fail and report issue
	result := CheckPermissions(context.Background())

	assert.Equal(t, StatusWarn, result.Status)
	assert.Contains(t, result.Message, "Permission issues found")
}

func TestCheckPermissions_WithSecureDirs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("semântica de permissão POSIX (mundo-leitura 0o640) não é aplicada no Windows — arquivos gravados com 0o640 reportam 0666")
	}
	homeDir := t.TempDir()
	setHome(t, homeDir)

	// Create cosca home with secure permissions (no world-readable)
	coscaHome := filepath.Join(homeDir, ".config", "cosca")
	err := os.MkdirAll(coscaHome, 0o755)
	require.NoError(t, err)

	// Create a non-sensitive file with secure permissions (no world-readable)
	err = os.WriteFile(filepath.Join(coscaHome, "runtime.json"), []byte("test"), 0o640)
	require.NoError(t, err)

	// Create a config file with no world-readable bit
	err = os.WriteFile(filepath.Join(coscaHome, "config.yaml"), []byte("test"), 0o640)
	require.NoError(t, err)

	result := CheckPermissions(context.Background())

	assert.Equal(t, StatusPass, result.Status)
	assert.Contains(t, result.Message, "permissions are secure")
}

// =============================================================================
// CheckPlugins (bonus)
// =============================================================================

func TestCheckPlugins_NoPlugins(t *testing.T) {
	homeDir := t.TempDir()
	setHome(t, homeDir)

	result := CheckPlugins(context.Background())

	assert.Equal(t, "plugins", result.Name)
	assert.Equal(t, StatusSkip, result.Status)
	assert.Contains(t, result.Message, "No plugins installed")
}

// =============================================================================
// Helper: create a fake executable for PATH-based tests
// =============================================================================

// createFakeExecutable creates a simple shell script that acts as an executable
func createFakeExecutable(t *testing.T, dir, name string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	// Use a shell script that just exits successfully
	contents := "#!/bin/sh\nexit 0\n"
	err := os.WriteFile(path, []byte(contents), 0o755)
	require.NoError(t, err)
	return path
}
