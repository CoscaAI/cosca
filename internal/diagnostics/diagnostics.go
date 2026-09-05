//
// Package diagnostics provides a comprehensive health and configuration
// diagnostic system for the Cosca platform. It runs a suite of checks
// covering runtime, editor, plugins, memory, knowledge, providers, network,
// filesystem, permissions, and configuration.

package diagnostics

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// =============================================================================
// Check Types
// =============================================================================

// Status represents the result of a diagnostic check.
type Status string

const (
	// StatusPass indicates the check passed.
	StatusPass Status = "pass"
	// StatusWarn indicates the check passed with warnings.
	StatusWarn Status = "warn"
	// StatusFail indicates the check failed.
	StatusFail Status = "fail"
	// StatusError indicates the check encountered an error.
	StatusError Status = "error"
	// StatusSkip indicates the check was skipped.
	StatusSkip Status = "skip"
)

// Severity represents the severity level of a check result.
type Severity string

const (
	// SeverityCritical indicates a critical issue.
	SeverityCritical Severity = "critical"
	// SeverityWarning indicates a warning-level issue.
	SeverityWarning Severity = "warning"
	// SeverityInfo indicates informational output.
	SeverityInfo Severity = "info"
)

// CheckResult represents the result of a single diagnostic check.
type CheckResult struct {
	// Name is the name of the check.
	Name string `json:"name" yaml:"name"`
	// Status indicates pass/warn/fail/error/skip.
	Status Status `json:"status" yaml:"status"`
	// Message is a human-readable result message.
	Message string `json:"message" yaml:"message"`
	// Details provides additional context or error details.
	Details string `json:"details,omitempty" yaml:"details,omitempty"`
	// Duration is how long the check took.
	Duration time.Duration `json:"duration" yaml:"duration"`
	// Suggestion offers a remediation suggestion if the check failed.
	Suggestion string `json:"suggestion,omitempty" yaml:"suggestion,omitempty"`
	// Severity indicates the importance of this result.
	Severity Severity `json:"severity" yaml:"severity"`
}

// =============================================================================
// Diagnostics Report
// =============================================================================

// SystemInfo contains information about the host system.
type SystemInfo struct {
	OS              string `json:"os" yaml:"os"`
	Arch            string `json:"arch" yaml:"arch"`
	GoVersion       string `json:"go_version" yaml:"go_version"`
	NumCPU          int    `json:"num_cpu" yaml:"num_cpu"`
	Hostname        string `json:"hostname" yaml:"hostname"`
	CoscaVersion    string `json:"cosca_version" yaml:"cosca_version"`
	CoscaHome       string `json:"cosca_home" yaml:"cosca_home"`
	CoscaRuntimeDir string `json:"cosca_runtime_dir" yaml:"cosca_runtime_dir"`
}

// Summary contains aggregate statistics for a diagnostics report.
type Summary struct {
	Total    int           `json:"total" yaml:"total"`
	Passed   int           `json:"passed" yaml:"passed"`
	Warnings int           `json:"warnings" yaml:"warnings"`
	Failed   int           `json:"failed" yaml:"failed"`
	Errors   int           `json:"errors" yaml:"errors"`
	Skipped  int           `json:"skipped" yaml:"skipped"`
	Critical int           `json:"critical" yaml:"critical"`
	Duration time.Duration `json:"duration" yaml:"duration"`
}

// DiagnosticsReport is the complete result of running all diagnostic checks.
//
//nolint:revive // Stutter name preserved for API compatibility — used as diagnostics.DiagnosticsReport externally.
type DiagnosticsReport struct {
	// Timestamp is when the report was generated.
	Timestamp time.Time `json:"timestamp" yaml:"timestamp"`
	// SystemInfo contains information about the host system.
	SystemInfo SystemInfo `json:"system_info" yaml:"system_info"`
	// Checks is the list of individual check results.
	Checks []CheckResult `json:"checks" yaml:"checks"`
	// Summary provides aggregate statistics.
	Summary Summary `json:"summary" yaml:"summary"`
}

// =============================================================================
// Check Function
// =============================================================================

// CheckFunc is a function that performs a single diagnostic check.
type CheckFunc func(ctx context.Context) CheckResult

// CheckDef defines a named diagnostic check.
type CheckDef struct {
	Name     string
	Severity Severity
	Fn       CheckFunc
}

// =============================================================================
// Diagnostics Engine
// =============================================================================

// Engine runs diagnostic checks and produces reports.
type Engine struct {
	mu           sync.RWMutex
	logger       zerolog.Logger
	checks       map[string]CheckDef
	coscaVersion string
}

// Option configures the diagnostics engine.
type Option func(*Engine)

// WithLogger sets the logger for the diagnostics engine.
func WithLogger(logger zerolog.Logger) Option {
	return func(e *Engine) {
		e.logger = logger
	}
}

// WithCoscaVersion sets the Cosca version for diagnostics.
func WithCoscaVersion(version string) Option {
	return func(e *Engine) {
		e.coscaVersion = version
	}
}

// NewEngine creates a new diagnostics engine.
func NewEngine(opts ...Option) *Engine {
	e := &Engine{
		logger:       zerolog.Nop(),
		checks:       make(map[string]CheckDef),
		coscaVersion: "unknown",
	}

	for _, opt := range opts {
		opt(e)
	}

	// Register all standard checks
	e.registerStandardChecks()

	return e
}

// registerStandardChecks registers all built-in diagnostic checks.
func (e *Engine) registerStandardChecks() {
	e.RegisterCheck(CheckDef{
		Name:     "runtime",
		Severity: SeverityCritical,
		Fn:       CheckRuntime,
	})
	e.RegisterCheck(CheckDef{
		Name:     "editor",
		Severity: SeverityWarning,
		Fn:       CheckEditor,
	})
	e.RegisterCheck(CheckDef{
		Name:     "plugins",
		Severity: SeverityWarning,
		Fn:       CheckPlugins,
	})
	e.RegisterCheck(CheckDef{
		Name:     "memory",
		Severity: SeverityWarning,
		Fn:       CheckMemory,
	})
	e.RegisterCheck(CheckDef{
		Name:     "knowledge",
		Severity: SeverityCritical,
		Fn:       CheckKnowledge,
	})
	e.RegisterCheck(CheckDef{
		Name:     "providers",
		Severity: SeverityWarning,
		Fn:       CheckProviders,
	})
	e.RegisterCheck(CheckDef{
		Name:     "network",
		Severity: SeverityWarning,
		Fn:       CheckNetwork,
	})
	e.RegisterCheck(CheckDef{
		Name:     "filesystem",
		Severity: SeverityCritical,
		Fn:       CheckFilesystem,
	})
	e.RegisterCheck(CheckDef{
		Name:     "permissions",
		Severity: SeverityWarning,
		Fn:       CheckPermissions,
	})
	e.RegisterCheck(CheckDef{
		Name:     "configuration",
		Severity: SeverityWarning,
		Fn:       CheckConfiguration,
	})
}

// =============================================================================
// Check Registration
// =============================================================================

// RegisterCheck adds a diagnostic check to the engine.
func (e *Engine) RegisterCheck(def CheckDef) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.checks[def.Name] = def
}

// UnregisterCheck removes a diagnostic check.
func (e *Engine) UnregisterCheck(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.checks, name)
}

// AvailableChecks returns the names of all registered checks.
func (e *Engine) AvailableChecks() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	names := make([]string, 0, len(e.checks))
	for name := range e.checks {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// =============================================================================
// Running Checks
// =============================================================================

// RunAll runs all registered diagnostic checks and returns a full report.
func (e *Engine) RunAll(ctx context.Context) DiagnosticsReport {
	start := time.Now()
	e.logger.Info().Int("checks", len(e.checks)).Msg("running all diagnostics")

	sysInfo := e.collectSystemInfo()

	e.mu.RLock()
	checkList := make([]CheckDef, 0, len(e.checks))
	for _, def := range e.checks {
		checkList = append(checkList, def)
	}
	e.mu.RUnlock()

	// Sort checks for deterministic output
	sort.Slice(checkList, func(i, j int) bool {
		return checkList[i].Name < checkList[j].Name
	})

	var mu sync.Mutex
	var wg sync.WaitGroup
	var results []CheckResult

	for _, def := range checkList {
		wg.Add(1)
		go func(def CheckDef) {
			defer wg.Done()

			checkStart := time.Now()
			result := def.Fn(ctx)
			result.Name = def.Name
			result.Duration = time.Since(checkStart)
			if result.Severity == "" {
				result.Severity = def.Severity
			}

			mu.Lock()
			results = append(results, result)
			mu.Unlock()

			e.logger.Debug().
				Str("check", def.Name).
				Str("status", string(result.Status)).
				Dur("duration", result.Duration).
				Msg("diagnostic check completed")
		}(def)
	}

	wg.Wait()

	// Sort results by name for deterministic output
	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	report := DiagnosticsReport{
		Timestamp:  start,
		SystemInfo: sysInfo,
		Checks:     results,
		Summary:    computeSummary(results, time.Since(start)),
	}

	e.logger.Info().
		Int("total", report.Summary.Total).
		Int("passed", report.Summary.Passed).
		Int("failed", report.Summary.Failed).
		Int("warnings", report.Summary.Warnings).
		Dur("duration", report.Summary.Duration).
		Msg("diagnostics complete")

	return report
}

// RunCheck runs a single diagnostic check by name.
func (e *Engine) RunCheck(ctx context.Context, name string) (CheckResult, error) {
	e.mu.RLock()
	def, ok := e.checks[name]
	e.mu.RUnlock()

	if !ok {
		return CheckResult{}, fmt.Errorf("unknown diagnostic check: %s", name)
	}

	e.logger.Debug().Str("check", name).Msg("running single diagnostic check")

	checkStart := time.Now()
	result := def.Fn(ctx)
	result.Name = def.Name
	result.Duration = time.Since(checkStart)
	if result.Severity == "" {
		result.Severity = def.Severity
	}

	return result, nil
}

// =============================================================================
// Helpers
// =============================================================================

// collectSystemInfo gathers information about the host system.
func (e *Engine) collectSystemInfo() SystemInfo {
	hostname, _ := getHostname()
	return SystemInfo{
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		GoVersion:    runtime.Version(),
		NumCPU:       runtime.NumCPU(),
		Hostname:     hostname,
		CoscaVersion: e.coscaVersion,
	}
}

// computeSummary aggregates check results into a summary.
func computeSummary(results []CheckResult, duration time.Duration) Summary {
	s := Summary{
		Total:    len(results),
		Duration: duration,
	}
	for _, r := range results {
		switch r.Status {
		case StatusPass:
			s.Passed++
		case StatusWarn:
			s.Warnings++
		case StatusFail:
			s.Failed++
		case StatusError:
			s.Errors++
		case StatusSkip:
			s.Skipped++
		}
		switch r.Severity {
		case SeverityCritical:
			if r.Status == StatusFail || r.Status == StatusError {
				s.Critical++
			}
		}
	}
	return s
}

// Suggestions returns all suggestions from failed/warning checks.
func (r *DiagnosticsReport) Suggestions() []string {
	var suggestions []string
	for _, check := range r.Checks {
		if (check.Status == StatusFail || check.Status == StatusError || check.Status == StatusWarn) &&
			check.Suggestion != "" {
			suggestions = append(suggestions, fmt.Sprintf("[%s] %s: %s", check.Name, check.Suggestion, check.Message))
		}
	}
	return suggestions
}

// Failed returns all checks that failed or errored.
func (r *DiagnosticsReport) Failed() []CheckResult {
	var failed []CheckResult
	for _, check := range r.Checks {
		if check.Status == StatusFail || check.Status == StatusError {
			failed = append(failed, check)
		}
	}
	return failed
}

// Passed returns all checks that passed.
func (r *DiagnosticsReport) Passed() []CheckResult {
	var passed []CheckResult
	for _, check := range r.Checks {
		if check.Status == StatusPass {
			passed = append(passed, check)
		}
	}
	return passed
}

// getHostname returns the system hostname.
func getHostname() (string, error) {
	return os.Hostname()
}

// Ensure these imports are used by the package
var _ = context.Background
var _ = fmt.Sprintf
var _ = log.Info
