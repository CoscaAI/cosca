// Package diagnostics tests for the diagnostics engine.
package diagnostics

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Engine Creation
// =============================================================================

func TestNewEngine_Default(t *testing.T) {
	e := NewEngine()
	require.NotNil(t, e)
	assert.NotNil(t, e.checks)
	assert.Equal(t, "unknown", e.coscaVersion)

	// Should have all standard checks registered
	checks := e.AvailableChecks()
	assert.Contains(t, checks, "runtime")
	assert.Contains(t, checks, "editor")
	assert.Contains(t, checks, "plugins")
	assert.Contains(t, checks, "memory")
	assert.Contains(t, checks, "knowledge")
	assert.Contains(t, checks, "providers")
	assert.Contains(t, checks, "network")
	assert.Contains(t, checks, "filesystem")
	assert.Contains(t, checks, "permissions")
	assert.Contains(t, checks, "configuration")
	assert.Len(t, checks, 10)
}

func TestNewEngine_WithOptions(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	version := "v1.5.0"

	e := NewEngine(
		WithLogger(logger),
		WithCoscaVersion(version),
	)
	require.NotNil(t, e)
	assert.Equal(t, version, e.coscaVersion)
}

func TestNewEngine_WithVersionOnly(t *testing.T) {
	e := NewEngine(WithCoscaVersion("v2.0.0"))
	require.NotNil(t, e)
	assert.Equal(t, "v2.0.0", e.coscaVersion)
}

// =============================================================================
// Check Registration
// =============================================================================

func TestRegisterCheck(t *testing.T) {
	e := NewEngine()

	// Remove all checks first
	for _, name := range e.AvailableChecks() {
		e.UnregisterCheck(name)
	}
	require.Empty(t, e.AvailableChecks())

	// Register a new check
	e.RegisterCheck(CheckDef{
		Name:     "my-check",
		Severity: SeverityInfo,
		Fn: func(_ context.Context) CheckResult {
			return CheckResult{Status: StatusPass, Message: "ok"}
		},
	})

	checks := e.AvailableChecks()
	assert.Contains(t, checks, "my-check")
	assert.Len(t, checks, 1)
}

func TestRegisterCheck_Overwrite(t *testing.T) {
	e := NewEngine()

	// Clear all standard checks
	for _, name := range e.AvailableChecks() {
		e.UnregisterCheck(name)
	}
	require.Empty(t, e.AvailableChecks())

	// Register a check
	e.RegisterCheck(CheckDef{
		Name:     "test-check",
		Severity: SeverityInfo,
		Fn: func(_ context.Context) CheckResult {
			return CheckResult{Status: StatusPass, Message: "original"}
		},
	})

	// Overwrite with same name
	e.RegisterCheck(CheckDef{
		Name:     "test-check",
		Severity: SeverityCritical,
		Fn: func(_ context.Context) CheckResult {
			return CheckResult{Status: StatusFail, Message: "overwritten"}
		},
	})

	// Verify overwritten — only 1 check with that name
	checks := e.AvailableChecks()
	assert.Contains(t, checks, "test-check")
	assert.Len(t, checks, 1)

	// Run it to verify the new function is used
	result, err := e.RunCheck(context.Background(), "test-check")
	require.NoError(t, err)
	assert.Equal(t, StatusFail, result.Status)
	assert.Equal(t, "overwritten", result.Message)
}

// =============================================================================
// Check Unregistration
// =============================================================================

func TestUnregisterCheck(t *testing.T) {
	e := NewEngine()

	// Remove runtime check
	e.UnregisterCheck("runtime")
	checks := e.AvailableChecks()
	assert.NotContains(t, checks, "runtime")
	assert.Len(t, checks, 9)

	// Removing nonexistent check should not panic
	e.UnregisterCheck("nonexistent")
	assert.Len(t, checks, 9)
}

func TestUnregisterCheck_All(t *testing.T) {
	e := NewEngine()

	for _, name := range e.AvailableChecks() {
		e.UnregisterCheck(name)
	}
	assert.Empty(t, e.AvailableChecks())
}

// =============================================================================
// AvailableChecks
// =============================================================================

func TestAvailableChecks_Sorted(t *testing.T) {
	e := NewEngine()
	checks := e.AvailableChecks()

	// Verify sorted alphabetically
	for i := 1; i < len(checks); i++ {
		assert.Greater(t, checks[i], checks[i-1],
			"checks should be sorted alphabetically")
	}
}

func TestAvailableChecks_Empty(t *testing.T) {
	e := NewEngine()
	// Unregister all checks
	for _, name := range e.AvailableChecks() {
		e.UnregisterCheck(name)
	}
	assert.Empty(t, e.AvailableChecks())
}

// =============================================================================
// RunCheck
// =============================================================================

func TestRunCheck_Success(t *testing.T) {
	e := NewEngine()

	result, err := e.RunCheck(context.Background(), "runtime")
	require.NoError(t, err)
	assert.Equal(t, "runtime", result.Name)
	assert.Contains(t, []Status{StatusPass, StatusWarn}, result.Status)
	assert.NotEmpty(t, result.Message)
	assert.NotEmpty(t, result.Details)
	// Duration may be 0 for instant checks on coarse clocks (Windows
	// time.Now() granularity ~0.5ms).
	assert.GreaterOrEqual(t, result.Duration, time.Duration(0))
}

func TestRunCheck_UnknownCheck(t *testing.T) {
	e := NewEngine()

	_, err := e.RunCheck(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown diagnostic check: nonexistent")
}

func TestRunCheck_AllRegisteredChecks(t *testing.T) {
	e := NewEngine()

	for _, name := range e.AvailableChecks() {
		t.Run(name, func(t *testing.T) {
			result, err := e.RunCheck(context.Background(), name)
			require.NoError(t, err)
			assert.Equal(t, name, result.Name)
			assert.Contains(t, []Status{StatusPass, StatusWarn, StatusFail, StatusError, StatusSkip}, result.Status)
			assert.NotEmpty(t, result.Message)
			// Duration may be 0 for instant checks on coarse clocks (Windows
			// time.Now() granularity ~0.5ms).
			assert.GreaterOrEqual(t, result.Duration, time.Duration(0))
		})
	}
}

// =============================================================================
// RunAll
// =============================================================================

func TestRunAll_Basic(t *testing.T) {
	e := NewEngine()
	ctx := context.Background()

	report := e.RunAll(ctx)

	// Verify report structure
	assert.False(t, report.Timestamp.IsZero())
	assert.NotZero(t, report.Summary.Total)
	assert.Equal(t, len(e.AvailableChecks()), report.Summary.Total)
	assert.Equal(t, report.Summary.Total, report.Summary.Passed+report.Summary.Warnings+
		report.Summary.Failed+report.Summary.Errors+report.Summary.Skipped)
	// Summary.Duration may be 0 for instant checks on coarse clocks (Windows
	// time.Now() granularity ~0.5ms).
	assert.GreaterOrEqual(t, report.Summary.Duration, time.Duration(0))

	// Verify SystemInfo
	assert.NotEmpty(t, report.SystemInfo.OS)
	assert.NotEmpty(t, report.SystemInfo.Arch)
	assert.NotEmpty(t, report.SystemInfo.GoVersion)
	assert.Greater(t, report.SystemInfo.NumCPU, 0)

	// Verify checks are sorted
	for i := 1; i < len(report.Checks); i++ {
		assert.GreaterOrEqual(t, report.Checks[i].Name, report.Checks[i-1].Name,
			"checks should be sorted by name")
	}

	// Verify each check has valid fields
	for _, check := range report.Checks {
		assert.NotEmpty(t, check.Name)
		assert.NotEmpty(t, check.Message)
		// Duration may be 0 for instant checks on coarse clocks (Windows
		// time.Now() granularity ~0.5ms).
		assert.GreaterOrEqual(t, check.Duration, time.Duration(0))
		assert.Contains(t, []Status{StatusPass, StatusWarn, StatusFail, StatusError, StatusSkip}, check.Status)
	}
}

func TestRunAll_WithOptions(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	e := NewEngine(
		WithLogger(logger),
		WithCoscaVersion("test-version"),
	)
	ctx := context.Background()

	report := e.RunAll(ctx)

	assert.Equal(t, "test-version", report.SystemInfo.CoscaVersion)
	assert.Equal(t, len(e.AvailableChecks()), report.Summary.Total)
}

func TestRunAll_EmptyEngine(t *testing.T) {
	e := NewEngine()
	// Remove all checks
	for _, name := range e.AvailableChecks() {
		e.UnregisterCheck(name)
	}
	require.Empty(t, e.AvailableChecks())

	ctx := context.Background()
	report := e.RunAll(ctx)

	assert.Equal(t, 0, report.Summary.Total)
	assert.Empty(t, report.Checks)
	assert.NotEmpty(t, report.SystemInfo.OS) // Should still collect system info
}

func TestRunAll_CheckResultFields(t *testing.T) {
	e := NewEngine()
	ctx := context.Background()

	report := e.RunAll(ctx)

	for _, check := range report.Checks {
		// Verify severity is set (either from the check or default)
		assert.Contains(t, []Severity{SeverityCritical, SeverityWarning, SeverityInfo, ""}, check.Severity)

		// Duration should be non-negative. Checks that complete in less than
		// the clock granularity (Windows time.Now() jumps in ~0.5ms steps)
		// legitimately measure 0s.
		assert.GreaterOrEqual(t, check.Duration, time.Duration(0))
	}
}

// =============================================================================
// collectSystemInfo (same-package access)
// =============================================================================

func TestCollectSystemInfo(t *testing.T) {
	e := NewEngine(WithCoscaVersion("v1.0.0"))
	info := e.collectSystemInfo()

	assert.NotEmpty(t, info.OS)
	assert.NotEmpty(t, info.Arch)
	assert.NotEmpty(t, info.GoVersion)
	assert.Greater(t, info.NumCPU, 0)
	assert.NotEmpty(t, info.Hostname)
	assert.Equal(t, "v1.0.0", info.CoscaVersion)
}

// =============================================================================
// computeSummary
// =============================================================================

func TestComputeSummary_AllPass(t *testing.T) {
	results := []CheckResult{
		{Name: "a", Status: StatusPass},
		{Name: "b", Status: StatusPass},
		{Name: "c", Status: StatusPass},
	}
	duration := 5 * time.Second

	s := computeSummary(results, duration)
	assert.Equal(t, 3, s.Total)
	assert.Equal(t, 3, s.Passed)
	assert.Equal(t, 0, s.Warnings)
	assert.Equal(t, 0, s.Failed)
	assert.Equal(t, 0, s.Errors)
	assert.Equal(t, 0, s.Skipped)
	assert.Equal(t, 0, s.Critical)
	assert.Equal(t, duration, s.Duration)
}

func TestComputeSummary_Mixed(t *testing.T) {
	results := []CheckResult{
		{Name: "a", Status: StatusPass},
		{Name: "b", Status: StatusWarn},
		{Name: "c", Status: StatusFail, Severity: SeverityCritical},
		{Name: "d", Status: StatusError, Severity: SeverityCritical},
		{Name: "e", Status: StatusSkip},
		{Name: "f", Status: StatusFail, Severity: SeverityWarning},
	}

	s := computeSummary(results, 1*time.Second)
	assert.Equal(t, 6, s.Total)
	assert.Equal(t, 1, s.Passed)
	assert.Equal(t, 1, s.Warnings)
	assert.Equal(t, 2, s.Failed)
	assert.Equal(t, 1, s.Errors)
	assert.Equal(t, 1, s.Skipped)
	assert.Equal(t, 2, s.Critical) // Both StatusFail+SeverityCritical and StatusError+SeverityCritical
}

func TestComputeSummary_Empty(t *testing.T) {
	s := computeSummary(nil, 0)
	assert.Equal(t, 0, s.Total)
	assert.Equal(t, 0, s.Passed)
	assert.Equal(t, 0, s.Warnings)
	assert.Equal(t, 0, s.Failed)
	assert.Equal(t, 0, s.Errors)
	assert.Equal(t, 0, s.Skipped)
	assert.Equal(t, 0, s.Critical)
}

func TestComputeSummary_CriticalNotCountedWithoutFailError(t *testing.T) {
	// Critical severity with Pass/Warn should not count as critical
	results := []CheckResult{
		{Name: "a", Status: StatusPass, Severity: SeverityCritical},
		{Name: "b", Status: StatusWarn, Severity: SeverityCritical},
	}
	s := computeSummary(results, 0)
	assert.Equal(t, 0, s.Critical)
}

// =============================================================================
// Report convenience methods
// =============================================================================

func TestDiagnosticsReport_Suggestions(t *testing.T) {
	report := DiagnosticsReport{
		Checks: []CheckResult{
			{Name: "ok", Status: StatusPass, Message: "all good"},
			{Name: "warn", Status: StatusWarn, Message: "careful", Suggestion: "be careful"},
			{Name: "fail", Status: StatusFail, Message: "broken", Suggestion: "fix it"},
			{Name: "error", Status: StatusError, Message: "err", Suggestion: "check err"},
			{Name: "skip", Status: StatusSkip, Message: "skipped"},
		},
	}

	suggestions := report.Suggestions()
	assert.Len(t, suggestions, 3)
	assert.Contains(t, suggestions[0], "warn")
	assert.Contains(t, suggestions[0], "be careful")
	assert.Contains(t, suggestions[1], "fail")
	assert.Contains(t, suggestions[1], "fix it")
}

func TestDiagnosticsReport_Suggestions_NoSuggestions(t *testing.T) {
	report := DiagnosticsReport{
		Checks: []CheckResult{
			{Name: "ok", Status: StatusPass, Message: "all good"},
			{Name: "fail", Status: StatusFail, Message: "broken"}, // no Suggestion field
		},
	}

	suggestions := report.Suggestions()
	assert.Empty(t, suggestions)
}

func TestDiagnosticsReport_Failed(t *testing.T) {
	report := DiagnosticsReport{
		Checks: []CheckResult{
			{Name: "a", Status: StatusPass},
			{Name: "b", Status: StatusFail},
			{Name: "c", Status: StatusError},
			{Name: "d", Status: StatusWarn},
		},
	}

	failed := report.Failed()
	assert.Len(t, failed, 2)
	assert.Equal(t, "b", failed[0].Name)
	assert.Equal(t, "c", failed[1].Name)
}

func TestDiagnosticsReport_Passed(t *testing.T) {
	report := DiagnosticsReport{
		Checks: []CheckResult{
			{Name: "a", Status: StatusPass},
			{Name: "b", Status: StatusFail},
			{Name: "c", Status: StatusPass},
		},
	}

	passed := report.Passed()
	assert.Len(t, passed, 2)
	assert.Equal(t, "a", passed[0].Name)
	assert.Equal(t, "c", passed[1].Name)
}
