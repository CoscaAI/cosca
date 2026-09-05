package diagnostics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClassifier(t *testing.T) {
	c := NewClassifier()
	require.NotNil(t, c)
	assert.NotEmpty(t, c.patterns)
}

// =============================================================================
// Go: Compilation Errors
// =============================================================================

func TestClassify_GoUndefined(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("./main.go:10:2: undefined: fmt.Println")

	assert.Equal(t, CatCompilation, result.Category)
	assert.Equal(t, "go", result.Language)
	assert.Equal(t, "./main.go", result.File)
	assert.Equal(t, 10, result.Line)
	assert.Equal(t, "fmt.Println", result.Symbol)
	assert.True(t, result.Confidence >= 0.9)
	assert.True(t, result.IsCompilationError())
}

func TestClassify_GoUndefinedNoFilePrefix(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("undefined: someVariable")

	assert.Equal(t, CatCompilation, result.Category)
	assert.Equal(t, "go", result.Language)
	assert.Equal(t, "someVariable", result.Symbol)
	assert.True(t, result.Confidence >= 0.9)
}

func TestClassify_GoCannotFindPackage(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("internal/handler.go:5:2: cannot find package \"github.com/gin-gonic/gin\"")

	assert.Equal(t, CatDependency, result.Category)
	assert.Equal(t, "go", result.Language)
	assert.Equal(t, "github.com/gin-gonic/gin", result.Symbol)
	assert.Equal(t, "internal/handler.go", result.File)
	assert.Equal(t, 5, result.Line)
	assert.True(t, result.Confidence >= 0.9)
	assert.True(t, result.IsDependencyError())
}

func TestClassify_GoImportedAndNotUsed(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("src/app.go:3:2: imported and not used: \"fmt\" as foo")

	assert.Equal(t, CatCompilation, result.Category)
	assert.Equal(t, "go", result.Language)
	assert.Equal(t, "src/app.go", result.File)
	assert.Equal(t, 3, result.Line)
}

func TestClassify_GoSyntaxError(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("syntax error: unexpected newline, expecting comma or )")

	assert.Equal(t, CatCompilation, result.Category)
	assert.Equal(t, "go", result.Language)
}

func TestClassify_GoTooManyArguments(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("pkg/handler.go:45:5: too many arguments in call to http.ListenAndServe")

	assert.Equal(t, CatCompilation, result.Category)
	assert.Equal(t, "go", result.Language)
	assert.Equal(t, "pkg/handler.go", result.File)
	assert.Equal(t, 45, result.Line)
}

func TestClassify_GoTypeMismatch(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("./cmd/main.go:12:3: cannot use name (variable of type int) as type string")

	assert.Equal(t, CatCompilation, result.Category)
	assert.Equal(t, "go", result.Language)
	assert.Equal(t, "name", result.Symbol)
	assert.Equal(t, "./cmd/main.go", result.File)
	assert.Equal(t, 12, result.Line)
}

// =============================================================================
// Go: Test Failures
// =============================================================================

func TestClassify_GoTestFail(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("--- FAIL: TestUserCreation (0.01s)")

	assert.Equal(t, CatTest, result.Category)
	assert.Equal(t, "go", result.Language)
	assert.Equal(t, "TestUserCreation", result.Symbol)
	assert.True(t, result.Confidence >= 0.9)
	assert.True(t, result.IsTestFailure())
}

func TestClassify_GoTestFailPackage(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("FAIL    github.com/CoscaAI/cosca/internal/diagnostics    0.123s")

	assert.Equal(t, CatTest, result.Category)
	assert.Equal(t, "go", result.Language)
	assert.Equal(t, "github.com/CoscaAI/cosca/internal/diagnostics", result.Symbol)
}

func TestClassify_GoTestWithFileAndLine(t *testing.T) {
	c := NewClassifier()

	raw := "--- FAIL: TestHandler (0.00s)\n    handler_test.go:42: expected 200, got 500"

	result := c.Classify(raw)

	assert.Equal(t, CatTest, result.Category)
	assert.Equal(t, "go", result.Language)
	assert.Equal(t, "TestHandler", result.Symbol)
	assert.Equal(t, "handler_test.go", result.File)
	assert.Equal(t, 42, result.Line)
}

// =============================================================================
// Go: Runtime Errors
// =============================================================================

func TestClassify_GoPanic(t *testing.T) {
	c := NewClassifier()

	raw := "panic: runtime error: invalid memory address or nil pointer dereference"

	result := c.Classify(raw)

	// nil pointer dereference is matched BEFORE the generic panic pattern
	assert.Equal(t, CatRuntime, result.Category)
	assert.Equal(t, "go", result.Language)
	assert.True(t, result.Confidence >= 0.9)
	assert.True(t, result.IsRuntimeError())
}

func TestClassify_GoNilPointer(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("runtime error: invalid memory address or nil pointer dereference")

	assert.Equal(t, CatRuntime, result.Category)
	assert.Equal(t, "go", result.Language)
	assert.True(t, result.Confidence >= 0.9)
}

func TestClassify_GoIndexOutOfRange(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("panic: runtime error: index out of range [5] with length 3")

	assert.Equal(t, CatRuntime, result.Category)
	assert.Equal(t, "go", result.Language)
	assert.Equal(t, "5", result.Symbol)
	assert.True(t, result.IsRuntimeError())
}

// =============================================================================
// TypeScript Errors
// =============================================================================

func TestClassify_TypeScriptError(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("src/app.ts:10:5 - error TS2345: Argument of type 'string' is not assignable to parameter of type 'number'.")

	assert.Equal(t, CatCompilation, result.Category)
	assert.Equal(t, "typescript", result.Language)
	assert.Equal(t, "2345", result.Symbol)
	assert.Equal(t, "src/app.ts", result.File)
	assert.Equal(t, 10, result.Line)
	assert.True(t, result.Confidence >= 0.9)
}

func TestClassify_TypeScriptCannotFindModule(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("error TS2307: Cannot find module 'express' or its corresponding type declarations.")

	assert.Equal(t, CatDependency, result.Category)
	assert.Equal(t, "typescript", result.Language)
	assert.Equal(t, "express", result.Symbol)
	assert.True(t, result.IsDependencyError())
}

func TestClassify_TypeScriptNotAssignable(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("Type 'number' is not assignable to type 'string'.")

	assert.Equal(t, CatCompilation, result.Category)
	assert.Equal(t, "typescript", result.Language)
}

func TestClassify_TypeScriptTscStyleOutput(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("src/utils.ts(15,42): error TS2532: Object is possibly 'undefined'.")

	assert.Equal(t, CatCompilation, result.Category)
	assert.Equal(t, "typescript", result.Language)
	assert.Equal(t, "2532", result.Symbol)
	assert.Equal(t, "src/utils.ts", result.File)
	assert.Equal(t, 15, result.Line)
}

func TestClassify_TypeScriptPropertyDoesNotExist(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("error TS2339: Property 'foo' does not exist on type '{ bar: string }'.")

	assert.Equal(t, CatCompilation, result.Category)
	assert.Equal(t, "typescript", result.Language)
	assert.Equal(t, "foo", result.Symbol)
}

// =============================================================================
// Python Errors
// =============================================================================

func TestClassify_PythonSyntaxError(t *testing.T) {
	c := NewClassifier()

	raw := "SyntaxError: invalid syntax"

	result := c.Classify(raw)

	assert.Equal(t, CatCompilation, result.Category)
	assert.Equal(t, "python", result.Language)
	assert.Equal(t, "invalid syntax", result.Symbol)
	assert.True(t, result.IsCompilationError())
}

func TestClassify_PythonSyntaxErrorWithFile(t *testing.T) {
	c := NewClassifier()

	raw := `  File "/app/main.py", line 15
    print("hello"
         ^
SyntaxError: unexpected EOF while parsing`

	result := c.Classify(raw)

	assert.Equal(t, CatCompilation, result.Category)
	assert.Equal(t, "python", result.Language)
	assert.Equal(t, "/app/main.py", result.File)
	assert.Equal(t, 15, result.Line)
}

func TestClassify_PythonModuleNotFoundError(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("ModuleNotFoundError: No module named 'numpy'")

	assert.Equal(t, CatDependency, result.Category)
	assert.Equal(t, "python", result.Language)
	assert.Equal(t, "No module named 'numpy'", result.Symbol)
	assert.True(t, result.IsDependencyError())
}

func TestClassify_PythonImportError(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("ImportError: cannot import name 'Config' from 'settings'")

	assert.Equal(t, CatDependency, result.Category)
	assert.Equal(t, "python", result.Language)
	assert.True(t, result.Confidence >= 0.8)
}

func TestClassify_PythonNameError(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("NameError: name 'foo' is not defined")

	assert.Equal(t, CatRuntime, result.Category)
	assert.Equal(t, "python", result.Language)
	assert.True(t, result.IsRuntimeError())
}

func TestClassify_PythonTypeError(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("TypeError: can only concatenate str (not 'int') to str")

	assert.Equal(t, CatRuntime, result.Category)
	assert.Equal(t, "python", result.Language)
	assert.True(t, result.Confidence >= 0.8)
}

func TestClassify_PythonAttributeError(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("AttributeError: 'NoneType' object has no attribute 'append'")

	assert.Equal(t, CatRuntime, result.Category)
	assert.Equal(t, "python", result.Language)
	assert.True(t, result.IsRuntimeError())
}

func TestClassify_PythonIndentationError(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("IndentationError: expected an indented block")

	assert.Equal(t, CatCompilation, result.Category)
	assert.Equal(t, "python", result.Language)
}

// =============================================================================
// Rust Errors
// =============================================================================

func TestClassify_RustCompileError(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("error[E0308]: mismatched types")

	assert.Equal(t, CatCompilation, result.Category)
	assert.Equal(t, "rust", result.Language)
	assert.Equal(t, "0308", result.Symbol)
	assert.True(t, result.Confidence >= 0.9)
}

func TestClassify_RustCouldNotCompile(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("could not compile `my_crate` (lib) due to 3 previous errors")

	assert.Equal(t, CatCompilation, result.Category)
	assert.Equal(t, "rust", result.Language)
}

func TestClassify_RustErrorWithLocation(t *testing.T) {
	c := NewClassifier()

	raw := `error[E0308]: mismatched types
  --> src/main.rs:10:5
   |
10 |     let x: i32 = "hello";
   |     ^^^^^^^^^^^^^^^^^^^^^ expected ` + "`i32`" + `, found ` + "`&str`"

	result := c.Classify(raw)

	assert.Equal(t, CatCompilation, result.Category)
	assert.Equal(t, "rust", result.Language)
	assert.Equal(t, "0308", result.Symbol)
	assert.Equal(t, "src/main.rs", result.File)
	assert.Equal(t, 10, result.Line)
}

func TestClassify_RustMismatchedTypes(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("mismatched types\n\nexpected `i32`, found `String`")

	assert.Equal(t, CatCompilation, result.Category)
	assert.Equal(t, "rust", result.Language)
}

func TestClassify_RustUnresolvedImport(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("error[E0432]: unresolved import `serde::Deserialize`")

	assert.Equal(t, CatDependency, result.Category)
	assert.Equal(t, "rust", result.Language)
}

// =============================================================================
// Network Errors
// =============================================================================

func TestClassify_ConnectionRefused(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("dial tcp 127.0.0.1:5432: connect: connection refused")

	assert.Equal(t, CatNetwork, result.Category)
	assert.Equal(t, "unknown", result.Language)
	assert.True(t, result.Confidence >= 0.9)
	assert.True(t, result.IsNetworkError())
}

func TestClassify_Timeout(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("context deadline exceeded (Client.Timeout exceeded)")

	assert.Equal(t, CatNetwork, result.Category)
	assert.True(t, result.IsNetworkError())
}

func TestClassify_NoSuchHost(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("dial tcp: lookup invalid-host.example.com: no such host")

	assert.Equal(t, CatNetwork, result.Category)
	assert.True(t, result.Confidence >= 0.9)
}

func TestClassify_CouldNotResolveHost(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("could not resolve host: api.github.com")

	assert.Equal(t, CatNetwork, result.Category)
	assert.True(t, result.IsNetworkError())
}

func TestClassify_IOTimeout(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("read tcp 10.0.0.1:443: i/o timeout")

	assert.Equal(t, CatNetwork, result.Category)
	assert.True(t, result.IsNetworkError())
}

// =============================================================================
// Permission Errors
// =============================================================================

func TestClassify_PermissionDenied(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("open /etc/config.yaml: permission denied")

	assert.Equal(t, CatPermission, result.Category)
	assert.True(t, result.Confidence >= 0.9)
	assert.True(t, result.IsPermissionError())
}

func TestClassify_EACCES(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("Error: EACCES: permission denied, mkdir '/usr/local/lib/cosca'")

	assert.Equal(t, CatPermission, result.Category)
	assert.True(t, result.IsPermissionError())
}

// =============================================================================
// Configuration Errors
// =============================================================================

func TestClassify_MissingEnvVar(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("missing env var: DATABASE_URL")

	assert.Equal(t, CatConfiguration, result.Category)
	assert.Equal(t, "DATABASE_URL", result.Symbol)
}

func TestClassify_MissingRequiredEnvVariable(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("missing required environment variable OPENAI_API_KEY")

	assert.Equal(t, CatConfiguration, result.Category)
	assert.Equal(t, "OPENAI_API_KEY", result.Symbol)
}

// =============================================================================
// Unknown / Edge Cases
// =============================================================================

func TestClassify_UnknownEmpty(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("")

	assert.Equal(t, CatUnknown, result.Category)
	assert.Equal(t, "unknown", result.Language)
	assert.Equal(t, float64(0), result.Confidence)
}

func TestClassify_UnknownGarbled(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("xyzzy florb glunk 1234")

	assert.Equal(t, CatUnknown, result.Category)
	assert.Equal(t, 0.0, result.Confidence)
}

func TestClassify_GoImportCycle(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("import cycle not allowed in test")

	assert.Equal(t, CatUnknown, result.Category) // not yet covered
}

// =============================================================================
// Convenience Methods
// =============================================================================

func TestIsCompilationError(t *testing.T) {
	ce := ClassifiedError{Category: CatCompilation}
	assert.True(t, ce.IsCompilationError())
	assert.False(t, ce.IsDependencyError())
	assert.False(t, ce.IsTestFailure())
	assert.False(t, ce.IsRuntimeError())
	assert.False(t, ce.IsNetworkError())
	assert.False(t, ce.IsPermissionError())
}

func TestIsDependencyError(t *testing.T) {
	ce := ClassifiedError{Category: CatDependency}
	assert.True(t, ce.IsDependencyError())
	assert.False(t, ce.IsCompilationError())
}

func TestIsTestFailure(t *testing.T) {
	ce := ClassifiedError{Category: CatTest}
	assert.True(t, ce.IsTestFailure())
	assert.False(t, ce.IsCompilationError())
}

func TestIsRuntimeError(t *testing.T) {
	ce := ClassifiedError{Category: CatRuntime}
	assert.True(t, ce.IsRuntimeError())
	assert.False(t, ce.IsCompilationError())
}

func TestIsNetworkError(t *testing.T) {
	ce := ClassifiedError{Category: CatNetwork}
	assert.True(t, ce.IsNetworkError())
	assert.False(t, ce.IsCompilationError())
}

func TestIsPermissionError(t *testing.T) {
	ce := ClassifiedError{Category: CatPermission}
	assert.True(t, ce.IsPermissionError())
	assert.False(t, ce.IsCompilationError())
}

// =============================================================================
// Suggestion Generation
// =============================================================================

func TestSuggestion_Compilation(t *testing.T) {
	c := NewClassifier()
	ce := c.Classify("./main.go:1:2: undefined: fmt.Println")
	assert.Contains(t, ce.Suggestion, "fmt.Println")
}

func TestSuggestion_Dependency(t *testing.T) {
	c := NewClassifier()
	ce := c.Classify(`cannot find package "express"`)
	assert.Contains(t, ce.Suggestion, "express")
}

func TestSuggestion_Test(t *testing.T) {
	c := NewClassifier()
	ce := c.Classify("--- FAIL: TestFoo (0.00s)")
	assert.Contains(t, ce.Suggestion, "TestFoo")
}

func TestSuggestion_Runtime(t *testing.T) {
	c := NewClassifier()
	ce := c.Classify("panic: runtime error: index out of range [5]")
	assert.Contains(t, ce.Suggestion, "runtime")
}

func TestSuggestion_Network(t *testing.T) {
	c := NewClassifier()
	ce := c.Classify("dial tcp: connect: connection refused")
	assert.Contains(t, ce.Suggestion, "network")
}

func TestSuggestion_Permission(t *testing.T) {
	c := NewClassifier()
	ce := c.Classify("open /etc/config: permission denied")
	assert.Contains(t, ce.Suggestion, "permission")
}

func TestSuggestion_Configuration(t *testing.T) {
	c := NewClassifier()
	ce := c.Classify("missing env var: API_KEY")
	assert.Contains(t, ce.Suggestion, "API_KEY")
}

func TestSuggestion_Unknown(t *testing.T) {
	c := NewClassifier()
	ce := c.Classify("something went wrong")
	assert.NotEmpty(t, ce.Suggestion)
}

// =============================================================================
// File/Line Extraction Edge Cases
// =============================================================================

func TestClassify_FileExtractionGo(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("pkg/db/conn.go:100:5: cannot use result (variable of type *sql.Rows)")

	assert.Equal(t, CatCompilation, result.Category)
	assert.Equal(t, "go", result.Language)
	assert.Equal(t, "pkg/db/conn.go", result.File)
	assert.Equal(t, 100, result.Line)
}

func TestClassify_FileExtractionPython(t *testing.T) {
	c := NewClassifier()

	raw := `Traceback (most recent call last):
  File "/app/main.py", line 42, in <module>
    handler()
  File "/app/main.py", line 38, in handler
    raise Exception("oops")
Exception: oops`

	result := c.Classify(raw)

	assert.Equal(t, CatUnknown, result.Category) // generic Exception not matched
	assert.Equal(t, "/app/main.py", result.File)
	assert.Equal(t, 42, result.Line)
}

func TestClassify_MultiplePatternsFirstMatchWins(t *testing.T) {
	c := NewClassifier()

	result := c.Classify("nil pointer dereference in goroutine 42")

	assert.Equal(t, CatRuntime, result.Category)
	assert.Equal(t, "go", result.Language)
}

func TestClassify_ConfidenceForHighCertainty(t *testing.T) {
	c := NewClassifier()

	highConfTests := []struct {
		name  string
		input string
	}{
		{"permission denied", "permission denied"},
		{"EACCES", "EACCES"},
		{"nil pointer", "nil pointer dereference"},
		{"cannot find package", `cannot find package "os"`},
		{"Go undefined with file", "./main.go:1:2: undefined: fmt"},
		{"--- FAIL test", "--- FAIL: TestX"},
	}

	for _, tc := range highConfTests {
		t.Run(tc.name, func(t *testing.T) {
			result := c.Classify(tc.input)
			assert.GreaterOrEqual(t, result.Confidence, 0.9,
				"expected high confidence for %q, got %.2f", tc.input, result.Confidence)
		})
	}
}

func TestClassify_LanguageInferenceFromFile(t *testing.T) {
	c := NewClassifier()

	tests := []struct {
		input    string
		language string
		file     string
	}{
		{"src/main.go:10:2: something", "go", "src/main.go"},
		{"app.ts:5:1: error TS2322", "typescript", "app.ts"},
		{"lib/utils.rs:42:10: error[E0308]", "rust", "lib/utils.rs"},
		{`File "script.py", line 10`, "python", "script.py"},
	}

	for _, tc := range tests {
		t.Run(tc.language, func(t *testing.T) {
			result := c.Classify(tc.input)
			assert.Equal(t, tc.language, result.Language,
				"expected language %s for %q", tc.language, tc.input)
		})
	}
}
