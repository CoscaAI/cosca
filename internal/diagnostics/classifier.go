// Error Classifier — classifies build/test/runtime errors by language and category.
//
// When a build or test fails, the error is just raw text. The classifier
// matches error patterns to produce a ClassifiedError with category, language,
// file, line, symbol, and a human-readable suggestion — all deterministic,
// without LLM. The resulting structure can be matched against knowledge gaps
// (internal/knowledge) or bug fingerprints (internal/kernel/bugfingerprint).

package diagnostics

import (
	"regexp"
	"strconv"
)

// ErrorCategory classifies the nature of an error.
type ErrorCategory string

const (
	CatCompilation   ErrorCategory = "compilation"   // syntax, type errors, missing imports
	CatDependency    ErrorCategory = "dependency"    // missing package, version mismatch
	CatTest          ErrorCategory = "test"          // test assertion failures
	CatRuntime       ErrorCategory = "runtime"       // panic, nil pointer, index out of range
	CatNetwork       ErrorCategory = "network"       // connection refused, timeout
	CatPermission    ErrorCategory = "permission"    // access denied, EACCES
	CatConfiguration ErrorCategory = "configuration" // missing env var, wrong config
	CatUnknown       ErrorCategory = "unknown"
)

// ClassifiedError is the structured result of classifying a raw error string.
type ClassifiedError struct {
	Category   ErrorCategory `json:"category"`
	Language   string        `json:"language"` // go, typescript, python, rust, unknown
	File       string        `json:"file"`
	Line       int           `json:"line"`
	Symbol     string        `json:"symbol"`
	RawMessage string        `json:"raw_message"`
	Suggestion string        `json:"suggestion"`
	Confidence float64       `json:"confidence"` // 0-1
}

// Classifier holds compiled error patterns and classifies raw error text.
type Classifier struct {
	patterns []errorPattern
}

type errorPattern struct {
	re         *regexp.Regexp
	category   ErrorCategory
	language   string
	fileIdx    int // capture group index for file (0 = none)
	lineIdx    int // capture group index for line number (0 = none)
	symbolIdx  int // capture group index for symbol name (0 = none)
	confidence float64
}

// NewClassifier creates a classifier with built-in patterns ordered by priority.
func NewClassifier() *Classifier {
	c := &Classifier{}
	c.patterns = []errorPattern{
		// ── Go: test failures ──
		{re(`--- FAIL:\s*(\w+)`), CatTest, "go", 0, 0, 1, 0.95},
		{re(`^\s*FAIL\s+(\S+)`), CatTest, "go", 0, 0, 1, 0.8},

		// ── Go: runtime panics (specific before generic) ──
		{re(`nil pointer dereference`), CatRuntime, "go", 0, 0, 0, 0.95},
		{re(`index out of range\s*\[(\d+)\]`), CatRuntime, "go", 0, 0, 1, 0.85},
		{re(`panic:\s*(.+)`), CatRuntime, "go", 0, 0, 1, 0.9},

		// ── Go: compilation errors ──
		{re(`([^\s:]+\.go):(\d+):\d+:\s*undefined:\s*([\w.]+)`), CatCompilation, "go", 1, 2, 3, 0.95},
		{re(`undefined:\s*([\w.]+)`), CatCompilation, "go", 0, 0, 1, 0.9},
		{re(`cannot find package\s+"([^"]+)"`), CatDependency, "go", 0, 0, 1, 0.95},
		{re(`imported and not used`), CatCompilation, "go", 0, 0, 0, 0.85},
		{re(`syntax error`), CatCompilation, "go", 0, 0, 0, 0.8},
		{re(`too many arguments`), CatCompilation, "go", 0, 0, 0, 0.85},
		{re(`not enough arguments`), CatCompilation, "go", 0, 0, 0, 0.85},
		{re(`cannot use\s+(\S+)`), CatCompilation, "go", 0, 0, 1, 0.85},

		// ── TypeScript: specific patterns before generic TS error code ──
		{re(`Cannot find module\s+['"]([^'"]+)['"]`), CatDependency, "typescript", 0, 0, 1, 0.9},
		{re(`Property\s+'(\w+)'\s+does not exist`), CatCompilation, "typescript", 0, 0, 1, 0.85},
		{re(`Type\s+'([^']+)'\s+is missing`), CatCompilation, "typescript", 0, 0, 1, 0.85},
		{re(`error\s+TS(\d{4})`), CatCompilation, "typescript", 0, 0, 1, 0.9},
		{re(`is not assignable to`), CatCompilation, "typescript", 0, 0, 0, 0.85},

		// ── Python: errors ──
		{re(`SyntaxError:\s*(.+)`), CatCompilation, "python", 0, 0, 1, 0.85},
		{re(`ModuleNotFoundError:\s*(.+)`), CatDependency, "python", 0, 0, 1, 0.9},
		{re(`ImportError:\s*(.+)`), CatDependency, "python", 0, 0, 1, 0.85},
		{re(`NameError:\s*(.+)`), CatRuntime, "python", 0, 0, 1, 0.85},
		{re(`TypeError:\s*(.+)`), CatRuntime, "python", 0, 0, 1, 0.8},
		{re(`AttributeError:\s*(.+)`), CatRuntime, "python", 0, 0, 1, 0.8},
		{re(`IndentationError:\s*(.+)`), CatCompilation, "python", 0, 0, 1, 0.85},

		// ── Rust: specific patterns before generic error[E ──
		{re(`unresolved import`), CatDependency, "rust", 0, 0, 0, 0.85},
		{re(`cannot find\s+(\S+)\s+in`), CatCompilation, "rust", 0, 0, 1, 0.85},
		{re(`error\[E(\d{4})\]`), CatCompilation, "rust", 0, 0, 1, 0.9},
		{re(`mismatched types`), CatCompilation, "rust", 0, 0, 0, 0.85},
		{re(`could not compile`), CatCompilation, "rust", 0, 0, 0, 0.8},

		// ── General: permission ──
		{re(`(?i)permission denied`), CatPermission, "unknown", 0, 0, 0, 0.95},
		{re(`EACCES`), CatPermission, "unknown", 0, 0, 0, 0.95},

		// ── General: network ──
		{re(`(?i)connection refused`), CatNetwork, "unknown", 0, 0, 0, 0.9},
		{re(`(?i)(?:i/?o\s*)?timeout`), CatNetwork, "unknown", 0, 0, 0, 0.85},
		{re(`(?i)no route to host`), CatNetwork, "unknown", 0, 0, 0, 0.9},
		{re(`(?i)no such host`), CatNetwork, "unknown", 0, 0, 0, 0.9},
		{re(`(?i)could not resolve host`), CatNetwork, "unknown", 0, 0, 0, 0.9},

		// ── General: dependency ──
		{re(`(?i)could not download`), CatDependency, "unknown", 0, 0, 0, 0.8},
		{re(`(?i)version\s+mismatch`), CatDependency, "unknown", 0, 0, 0, 0.8},

		// ── General: configuration ──
		{re(`(?i)missing\s+(?:env|environment)\s+var\w*:\s*(\w+)`), CatConfiguration, "unknown", 0, 0, 1, 0.85},
		{re(`(?i)missing\s+required\s+(?:env|environment)\s+variable\s+(\w+)`), CatConfiguration, "unknown", 0, 0, 1, 0.85},
	}
	return c
}

func re(pattern string) *regexp.Regexp {
	return regexp.MustCompile(pattern)
}

// Classify matches raw error text against patterns in priority order and returns
// a ClassifiedError. File and line are extracted from the raw text when the
// matching pattern doesn't capture them directly.
func (c *Classifier) Classify(raw string) ClassifiedError {
	ce := ClassifiedError{
		RawMessage: raw,
		Category:   CatUnknown,
		Language:   "unknown",
		Confidence: 0,
	}

	for _, p := range c.patterns {
		matches := p.re.FindStringSubmatch(raw)
		if matches == nil {
			continue
		}
		ce.Category = p.category
		ce.Language = p.language
		ce.Confidence = p.confidence
		if p.fileIdx > 0 && p.fileIdx < len(matches) {
			ce.File = matches[p.fileIdx]
		}
		if p.lineIdx > 0 && p.lineIdx < len(matches) {
			ce.Line, _ = strconv.Atoi(matches[p.lineIdx])
		}
		if p.symbolIdx > 0 && p.symbolIdx < len(matches) {
			ce.Symbol = matches[p.symbolIdx]
		}
		break
	}

	if ce.File == "" || ce.Line == 0 {
		ce.extractFileLine(raw)
	}

	ce.Suggestion = suggestionFor(ce)
	return ce
}

// fileLinePatterns are language-specific regexes for extracting file and line
// from error text when the primary pattern didn't capture them.
var fileLinePatterns = []struct {
	re      *regexp.Regexp
	fileIdx int
	lineIdx int
}{
	// Python: File "file.py", line N
	{re(`File\s+"([^"]+)",\s*line\s+(\d+)`), 1, 2},
	// Rust: --> file.rs:line:col
	{re(`-->\s*([^\s:]+):(\d+):\d+`), 1, 2},
	// TypeScript (VSCode style): file.ts:line:col
	{re(`([^\s:]+\.tsx?):(\d+):\d+`), 1, 2},
	// TypeScript (tsc style): file.ts(line,col)
	{re(`([^\s(]+\.tsx?)\((\d+),\d+\)`), 1, 2},
	// Go / Rust / others: file.ext:line: (column optional — Go test output omits it)
	{re(`([^\s:]+\.go):(\d+):`), 1, 2},
	{re(`([^\s:]+_test\.go):(\d+):`), 1, 2},
	{re(`([^\s:]+\.rs):(\d+):\d+`), 1, 2},
	// Generic source files with file:line:col
	{re(`([^\s:]+\.py):(\d+):\d+`), 1, 2},
	{re(`([^\s:]+\.java):(\d+):\d+`), 1, 2},
}

func (ce *ClassifiedError) extractFileLine(raw string) {
	for _, p := range fileLinePatterns {
		matches := p.re.FindStringSubmatch(raw)
		if matches == nil {
			continue
		}
		ce.File = matches[p.fileIdx]
		ce.Line, _ = strconv.Atoi(matches[p.lineIdx])
		if ce.Language == "unknown" {
			ce.inferLanguageFromFile(ce.File)
		}
		return
	}
}

func (ce *ClassifiedError) inferLanguageFromFile(file string) {
	switch {
	case regexp.MustCompile(`\.go$`).MatchString(file):
		ce.Language = "go"
	case regexp.MustCompile(`\.tsx?$`).MatchString(file):
		ce.Language = "typescript"
	case regexp.MustCompile(`\.py$`).MatchString(file):
		ce.Language = "python"
	case regexp.MustCompile(`\.rs$`).MatchString(file):
		ce.Language = "rust"
	case regexp.MustCompile(`\.java$`).MatchString(file):
		ce.Language = "java"
	case regexp.MustCompile(`\.jsx?$`).MatchString(file):
		ce.Language = "javascript"
	}
}

func suggestionFor(ce ClassifiedError) string {
	switch ce.Category {
	case CatCompilation:
		if ce.Symbol != "" {
			return "Check the definition of '" + ce.Symbol + "' and correct the syntax or type error."
		}
		return "Review the compilation error and fix syntax, types, or imports."
	case CatDependency:
		if ce.Symbol != "" {
			return "Install or update the missing package '" + ce.Symbol + "'."
		}
		return "Install missing dependencies or resolve version mismatches."
	case CatTest:
		if ce.Symbol != "" {
			return "Review the failing test '" + ce.Symbol + "' and fix the assertion or logic."
		}
		return "Review failing tests and fix the assertions or logic errors."
	case CatRuntime:
		if ce.Symbol != "" {
			return "Investigate the runtime error '" + ce.Symbol + "' — check for nil pointers, bounds, or panics."
		}
		return "Investigate the runtime error — check for nil pointers, bounds checks, or panics."
	case CatNetwork:
		return "Check network connectivity, DNS resolution, and firewall settings."
	case CatPermission:
		return "Check file permissions and ensure the process has the required access rights."
	case CatConfiguration:
		if ce.Symbol != "" {
			return "Set the required environment variable '" + ce.Symbol + "' or update the configuration."
		}
		return "Check your environment variables and configuration files."
	default:
		return "Review the error message and context to determine the cause."
	}
}

// ── Convenience methods ──

// IsCompilationError returns true if the error is a compilation error.
func (ce ClassifiedError) IsCompilationError() bool {
	return ce.Category == CatCompilation
}

// IsDependencyError returns true if the error is a dependency error.
func (ce ClassifiedError) IsDependencyError() bool {
	return ce.Category == CatDependency
}

// IsTestFailure returns true if the error is a test failure.
func (ce ClassifiedError) IsTestFailure() bool {
	return ce.Category == CatTest
}

// IsRuntimeError returns true if the error is a runtime error.
func (ce ClassifiedError) IsRuntimeError() bool {
	return ce.Category == CatRuntime
}

// IsNetworkError returns true if the error is a network error.
func (ce ClassifiedError) IsNetworkError() bool {
	return ce.Category == CatNetwork
}

// IsPermissionError returns true if the error is a permission error.
func (ce ClassifiedError) IsPermissionError() bool {
	return ce.Category == CatPermission
}
