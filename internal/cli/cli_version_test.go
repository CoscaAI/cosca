//
// Comprehensive unit tests for the version command and related helpers.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Version Command — Output Formats
// =============================================================================

func TestVersionCommand_TextOutputContainsAllFields(t *testing.T) {
	// Arrange
	root := NewRootCommand()
	cmd, _, err := root.Find([]string{"version"})
	if err != nil {
		t.Fatalf("Find('version') error: %v", err)
	}

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(context.Background())

	// Act
	err = cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	// Assert
	output := buf.String()
	requiredFields := []string{
		"Cosca",
		"Version",
		"Commit",
		"Build Date",
		"Go Version",
		"Platform",
		"Compiler",
	}
	for _, field := range requiredFields {
		if !strings.Contains(output, field) {
			t.Errorf("expected output to contain %q", field)
		}
	}
}

func TestVersionCommand_JSONOutput(t *testing.T) {
	// Arrange
	root := NewRootCommand()
	// Enable JSON output via the root's persistent --json flag
	root.PersistentFlags().Set("json", "true")

	cmd, _, err := root.Find([]string{"version"})
	if err != nil {
		t.Fatalf("Find('version') error: %v", err)
	}

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(context.Background())

	// Act
	err = cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	// Assert — should be valid JSON with expected fields
	var info VersionInfo
	if err := json.Unmarshal(buf.Bytes(), &info); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput:\n%s", err, buf.String())
	}

	if info.Version == "" {
		t.Error("expected non-empty version in JSON output")
	}
	if info.GoVersion == "" {
		t.Error("expected non-empty go_version in JSON output")
	}
	if info.Platform == "" {
		t.Error("expected non-empty platform in JSON output")
	}
}

func TestVersionCommand_JSONOutputContainsBuildInfo(t *testing.T) {
	// Arrange
	root := NewRootCommand()
	root.PersistentFlags().Set("json", "true")

	cmd, _, err := root.Find([]string{"version"})
	if err != nil {
		t.Fatalf("Find('version') error: %v", err)
	}

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(context.Background())

	// Act
	err = cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	// Assert
	var info VersionInfo
	_ = json.Unmarshal(buf.Bytes(), &info)

	// All fields should be populated
	if info.Version != Version {
		t.Errorf("Version = %q, want %q", info.Version, Version)
	}
	if info.Commit != Commit {
		t.Errorf("Commit = %q, want %q", info.Commit, Commit)
	}
	if info.BuildDate != BuildDate {
		t.Errorf("BuildDate = %q, want %q", info.BuildDate, BuildDate)
	}
}

// =============================================================================
// Version Command — Use and Properties
// =============================================================================

func TestVersionCommand_Use(t *testing.T) {
	cmd := NewVersionCommand()
	if cmd.Use != "version" {
		t.Errorf("Use = %q, want %q", cmd.Use, "version")
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.Long == "" {
		t.Error("Long should not be empty")
	}
	if len(cmd.Example) == 0 {
		t.Error("Example should not be empty")
	}
}

// =============================================================================
// formatKey / formatValue
// =============================================================================

func TestFormatKey_RendersWithANSIColors(t *testing.T) {
	result := formatKey("Version")
	if !strings.Contains(result, "\033[36m") {
		t.Error("formatKey should contain cyan ANSI code")
	}
	if !strings.Contains(result, "Version") {
		t.Error("formatKey should contain the key text")
	}
	if !strings.Contains(result, "\033[0m") {
		t.Error("formatKey should contain reset ANSI code")
	}
}

func TestFormatValue_RendersWithANSIColors(t *testing.T) {
	result := formatValue("dev")
	if !strings.Contains(result, "\033[33m") {
		t.Error("formatValue should contain yellow ANSI code")
	}
	if !strings.Contains(result, "dev") {
		t.Error("formatValue should contain the value text")
	}
	if !strings.Contains(result, "\033[0m") {
		t.Error("formatValue should contain reset ANSI code")
	}
}

func TestFormatKey_EmptyString(t *testing.T) {
	result := formatKey("")
	// Should still wrap with ANSI codes even for empty key
	if !strings.Contains(result, "\033[36m") {
		t.Error("formatKey should wrap even empty strings with ANSI")
	}
}

func TestFormatValue_EmptyString(t *testing.T) {
	result := formatValue("")
	if !strings.Contains(result, "\033[33m") {
		t.Error("formatValue should wrap even empty strings with ANSI")
	}
}

// =============================================================================
// GetVersionInfo
// =============================================================================

func TestGetVersionInfo_AllFieldsPopulated(t *testing.T) {
	info := GetVersionInfo()

	if info.Version == "" {
		t.Error("Version should not be empty")
	}
	if info.GoVersion == "" {
		t.Error("GoVersion should not be empty")
	}
	if info.Platform == "" {
		t.Error("Platform should not be empty")
	}
	if info.Arch == "" {
		t.Error("Arch should not be empty")
	}
	if info.Compiler == "" {
		t.Error("Compiler should not be empty")
	}
}

func TestGetVersionInfo_ReflectsPackageVars(t *testing.T) {
	// Save current values
	origVersion := Version
	origCommit := Commit
	origBuildDate := BuildDate
	defer func() {
		Version = origVersion
		Commit = origCommit
		BuildDate = origBuildDate
	}()

	// Set custom values
	Version = "2.5.1"
	Commit = "abc123def"
	BuildDate = "2025-07-01T12:00:00Z"

	info := GetVersionInfo()
	if info.Version != "2.5.1" {
		t.Errorf("Version = %q, want %q", info.Version, "2.5.1")
	}
	if info.Commit != "abc123def" {
		t.Errorf("Commit = %q, want %q", info.Commit, "abc123def")
	}
	if info.BuildDate != "2025-07-01T12:00:00Z" {
		t.Errorf("BuildDate = %q, want %q", info.BuildDate, "2025-07-01T12:00:00Z")
	}
}

// =============================================================================
// GetBuildDate
// =============================================================================

func TestGetBuildDate_ValidRFC3339(t *testing.T) {
	origBuildDate := BuildDate
	defer func() { BuildDate = origBuildDate }()

	BuildDate = "2025-03-15T14:30:00Z"
	expected := time.Date(2025, 3, 15, 14, 30, 0, 0, time.UTC)

	result := GetBuildDate()
	if !result.Equal(expected) {
		t.Errorf("GetBuildDate = %v, want %v", result, expected)
	}
}

func TestGetBuildDate_InvalidFormatReturnsZero(t *testing.T) {
	origBuildDate := BuildDate
	defer func() { BuildDate = origBuildDate }()

	BuildDate = "not-a-date"

	result := GetBuildDate()
	if !result.IsZero() {
		t.Errorf("expected zero time for invalid date, got %v", result)
	}
}

func TestGetBuildDate_EmptyStringReturnsZero(t *testing.T) {
	origBuildDate := BuildDate
	defer func() { BuildDate = origBuildDate }()

	BuildDate = ""

	result := GetBuildDate()
	if !result.IsZero() {
		t.Errorf("expected zero time for empty date, got %v", result)
	}
}

func TestGetBuildDate_DefaultUnknownReturnsZero(t *testing.T) {
	// BuildDate defaults to "unknown" — that is not valid RFC3339
	origBuildDate := BuildDate
	defer func() { BuildDate = origBuildDate }()

	BuildDate = "unknown"

	result := GetBuildDate()
	if !result.IsZero() {
		t.Errorf("expected zero time for 'unknown', got %v", result)
	}
}

// =============================================================================
// VersionInfo struct JSON tags
// =============================================================================

func TestVersionInfo_JSONTags(t *testing.T) {
	info := VersionInfo{
		Version:   "1.0.0",
		Commit:    "abcdef",
		BuildDate: "2025-01-01T00:00:00Z",
		GoVersion: "go1.25.0",
		Platform:  "linux",
		Arch:      "amd64",
		Compiler:  "gc",
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("failed to marshal VersionInfo: %v", err)
	}

	var result map[string]interface{}
	_ = json.Unmarshal(data, &result)

	expectedKeys := []string{"version", "commit", "build_date", "go_version", "platform", "arch", "compiler"}
	for _, key := range expectedKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q in marshaled output", key)
		}
	}
}
