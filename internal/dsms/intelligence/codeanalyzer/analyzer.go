// Package codeanalyzer provides AST-based code analysis.
// This analyzes source code structurally to extract metrics
// and patterns WITHOUT external LLM providers.
package codeanalyzer

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// ============================================================
// CODE ANALYZER
// ============================================================

// Analyzer analyzes source code.
type Analyzer struct{}

// NewAnalyzer creates a new code analyzer.
func NewAnalyzer() *Analyzer {
	return &Analyzer{}
}

// AnalysisResult represents the result of code analysis.
type AnalysisResult struct {
	Language           string             `json:"language"`
	FilePath           string             `json:"file_path"`
	Lines              int                `json:"lines"`
	Functions          []*FunctionInfo    `json:"functions"`
	FunctionCount      int                `json:"function_count"`
	MaxFunctionLines   int                `json:"max_function_lines"`
	AvgFunctionLines   float64            `json:"avg_function_lines"`
	MaxComplexity      int                `json:"max_complexity"`
	AvgComplexity      float64            `json:"avg_complexity"`
	MaxNestingDepth    int                `json:"max_nesting_depth"`
	AvgNestingDepth    float64            `json:"avg_nesting_depth"`
	Imports            []string           `json:"imports"`
	ImportCount        int                `json:"import_count"`
	StructCount        int                `json:"struct_count"`
	InterfaceCount     int                `json:"interface_count"`
	HasTests           bool               `json:"has_tests"`
	Comments           int                `json:"comments"`
	TodoComments       int                `json:"todo_comments"`
	FixmeComments      int                `json:"fixme_comments"`
	Metrics            map[string]float64 `json:"metrics"`
	Patterns           []*PatternMatch    `json:"patterns"`
}

// FunctionInfo represents information about a function.
type FunctionInfo struct {
	Name           string `json:"name"`
	Lines          int    `json:"lines"`
	Complexity     int    `json:"complexity"`
	NestingDepth   int    `json:"nesting_depth"`
	Parameters     int    `json:"parameters"`
	Returns        int    `json:"returns"`
	HasErrorReturn bool   `json:"has_error_return"`
}

// PatternMatch represents a detected pattern.
type PatternMatch struct {
	Pattern    string `json:"pattern"`
	Severity   string `json:"severity"`
	Line       int    `json:"line"`
	Message    string `json:"message"`
	Category   string `json:"category"`
}

// ============================================================
// ANALYSIS
// ============================================================

// AnalyzeGo analyzes Go source code.
func (a *Analyzer) AnalyzeGo(filePath string, code string) (*AnalysisResult, error) {
	result := &AnalysisResult{
		Language: "go",
		FilePath: filePath,
		Metrics:  make(map[string]float64),
		Patterns: make([]*PatternMatch, 0),
	}

	// Count lines
	result.Lines = countLines(code)

	// Parse AST
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, code, parser.ParseComments)
	if err != nil {
		return result, fmt.Errorf("parse file: %w", err)
	}

	// Analyze imports
	analyzeImports(file, result)

	// Analyze declarations and comments
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			info := analyzeFunction(fset, node)
			result.Functions = append(result.Functions, info)

			// Update max/avg
			if info.Lines > result.MaxFunctionLines {
				result.MaxFunctionLines = info.Lines
			}
			if info.Complexity > result.MaxComplexity {
				result.MaxComplexity = info.Complexity
			}
			if info.NestingDepth > result.MaxNestingDepth {
				result.MaxNestingDepth = info.NestingDepth
			}

		case *ast.TypeSpec:
			switch node.Type.(type) {
			case *ast.StructType:
				result.StructCount++
			case *ast.InterfaceType:
				result.InterfaceCount++
			}

		case *ast.CommentGroup:
			for _, comment := range node.List {
				result.Comments++
				if strings.Contains(comment.Text, "TODO") {
					result.TodoComments++
				}
				if strings.Contains(comment.Text, "FIXME") {
					result.FixmeComments++
				}
			}
		}
		return true
	})

	// Also count comments from file.Comments (top-level comments)
	for _, group := range file.Comments {
		for _, comment := range group.List {
			result.Comments++
			if strings.Contains(comment.Text, "TODO") {
				result.TodoComments++
			}
			if strings.Contains(comment.Text, "FIXME") {
				result.FixmeComments++
			}
		}
	}

	// Calculate averages
	if len(result.Functions) > 0 {
		totalLines := 0
		totalComplexity := 0
		totalNesting := 0

		for _, f := range result.Functions {
			totalLines += f.Lines
			totalComplexity += f.Complexity
			totalNesting += f.NestingDepth
		}

		result.AvgFunctionLines = float64(totalLines) / float64(len(result.Functions))
		result.AvgComplexity = float64(totalComplexity) / float64(len(result.Functions))
		result.AvgNestingDepth = float64(totalNesting) / float64(len(result.Functions))
	}

	result.FunctionCount = len(result.Functions)
	result.ImportCount = len(result.Imports)

	// Check for tests
	result.HasTests = strings.Contains(filePath, "_test.go")

	// Set metrics
	result.Metrics["lines"] = float64(result.Lines)
	result.Metrics["function_count"] = float64(result.FunctionCount)
	result.Metrics["max_function_lines"] = float64(result.MaxFunctionLines)
	result.Metrics["avg_function_lines"] = result.AvgFunctionLines
	result.Metrics["max_cyclomatic_complexity"] = float64(result.MaxComplexity)
	result.Metrics["avg_cyclomatic_complexity"] = result.AvgComplexity
	result.Metrics["max_nesting_depth"] = float64(result.MaxNestingDepth)
	result.Metrics["avg_nesting_depth"] = result.AvgNestingDepth
	result.Metrics["import_count"] = float64(result.ImportCount)
	result.Metrics["struct_count"] = float64(result.StructCount)
	result.Metrics["interface_count"] = float64(result.InterfaceCount)
	result.Metrics["comment_ratio"] = float64(result.Comments) / float64(max(1, result.Lines))

	// Detect patterns
	a.detectPatterns(result)

	// AST-based security detection (high precision)
	detectSQLInjectionAST(file, result)
	detectHardcodedSecretAST(file, result)

	return result, nil
}

// ============================================================
// FUNCTION ANALYSIS
// ============================================================

func analyzeFunction(fset *token.FileSet, fn *ast.FuncDecl) *FunctionInfo {
	info := &FunctionInfo{
		Name:       fn.Name.Name,
		Complexity: 1, // Base complexity
	}

	// Count parameters
	if fn.Type.Params != nil {
		info.Parameters = len(fn.Type.Params.List)
	}

	// Count returns
	if fn.Type.Results != nil {
		info.Returns = len(fn.Type.Results.List)

		// Check for error return
		for _, field := range fn.Type.Results.List {
			if ident, ok := field.Type.(*ast.Ident); ok && ident.Name == "error" {
				info.HasErrorReturn = true
			}
		}
	}

	// Count lines
	if fn.Body != nil {
		start := fset.Position(fn.Pos()).Line
		end := fset.Position(fn.End()).Line
		info.Lines = end - start + 1
	}

	// Calculate cyclomatic complexity
	info.Complexity = calculateComplexity(fn)

	// Calculate nesting depth
	info.NestingDepth = calculateNestingDepth(fn.Body)

	return info
}

func calculateComplexity(fn *ast.FuncDecl) int {
	complexity := 1

	ast.Inspect(fn, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.IfStmt:
			complexity++
		case *ast.ForStmt:
			complexity++
		case *ast.RangeStmt:
			complexity++
		case *ast.CaseClause:
			complexity++
		case *ast.CommClause:
			complexity++
		case *ast.BinaryExpr:
			if node.Op == token.LAND || node.Op == token.LOR {
				complexity++
			}
		}
		return true
	})

	return complexity
}

func calculateNestingDepth(body *ast.BlockStmt) int {
	maxDepth := 0

	var walkStmt func(stmt ast.Stmt, depth int)
	var walkNode func(n ast.Node, depth int)

	walkStmt = func(stmt ast.Stmt, depth int) {
		if depth > maxDepth {
			maxDepth = depth
		}

		switch node := stmt.(type) {
		case *ast.IfStmt:
			walkNode(node.Body, depth+1)
			if node.Else != nil {
				walkNode(node.Else, depth+1)
			}
		case *ast.ForStmt:
			walkNode(node.Body, depth+1)
		case *ast.RangeStmt:
			walkNode(node.Body, depth+1)
		case *ast.SwitchStmt:
			walkNode(node.Body, depth+1)
		case *ast.BlockStmt:
			for _, s := range node.List {
				walkStmt(s, depth)
			}
		}
	}

	walkNode = func(n ast.Node, depth int) {
		if block, ok := n.(*ast.BlockStmt); ok {
			walkStmt(block, depth)
		}
	}

	if body != nil {
		for _, stmt := range body.List {
			walkStmt(stmt, 0)
		}
	}

	return maxDepth
}

// ============================================================
// IMPORTS
// ============================================================

func analyzeImports(file *ast.File, result *AnalysisResult) {
	for _, imp := range file.Imports {
		if imp.Path != nil {
			path := strings.Trim(imp.Path.Value, "\"")
			result.Imports = append(result.Imports, path)
		}
	}
}

// ============================================================
// PATTERN DETECTION
// ============================================================

func (a *Analyzer) detectPatterns(result *AnalysisResult) {
	// Check for god object
	if result.Lines > 500 {
		result.Patterns = append(result.Patterns, &PatternMatch{
			Pattern:  "god_object",
			Severity: "warning",
			Message:  "File has more than 500 lines - consider splitting",
			Category: "architecture",
		})
	}

	// Check for high complexity
	if result.MaxComplexity > 10 {
		result.Patterns = append(result.Patterns, &PatternMatch{
			Pattern:  "high_complexity",
			Severity: "warning",
			Message:  fmt.Sprintf("Function has complexity %d - consider refactoring", result.MaxComplexity),
			Category: "architecture",
		})
	}

	// Check for deep nesting
	if result.MaxNestingDepth > 5 {
		result.Patterns = append(result.Patterns, &PatternMatch{
			Pattern:  "deep_nesting",
			Severity: "warning",
			Message:  fmt.Sprintf("Nesting depth %d exceeds recommended 5", result.MaxNestingDepth),
			Category: "code_quality",
		})
	}

	// Check for TODO comments
	if result.TodoComments > 0 {
		result.Patterns = append(result.Patterns, &PatternMatch{
			Pattern:  "todo_comments",
			Severity: "info",
			Message:  fmt.Sprintf("%d TODO comments found", result.TodoComments),
			Category: "code_quality",
		})
	}

	// Check for FIXME comments
	if result.FixmeComments > 0 {
		result.Patterns = append(result.Patterns, &PatternMatch{
			Pattern:  "fixme_comments",
			Severity: "warning",
			Message:  fmt.Sprintf("%d FIXME comments found - needs attention", result.FixmeComments),
			Category: "code_quality",
		})
	}

	// Check for missing tests
	if !result.HasTests {
		result.Patterns = append(result.Patterns, &PatternMatch{
			Pattern:  "missing_tests",
			Severity: "info",
			Message:  "File has no corresponding test file",
			Category: "testing",
		})
	}
}

// ============================================================
// AST-BASED SECURITY DETECTION (high precision)
// ============================================================

// detectSQLInjectionAST detects REAL SQL injection using the AST.
// A real SQL injection is: a call to Query/QueryRow/Exec where the
// SQL argument is built with string concatenation (+) and does NOT
// use parameter placeholders (? or $1).
func detectSQLInjectionAST(file *ast.File, result *AnalysisResult) {
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Identify query-like function calls
		fnName := callFuncName(call)
		if !isQueryCall(fnName) {
			return true
		}

		// Check the first argument for string concatenation
		if len(call.Args) == 0 {
			return true
		}

		firstArg := call.Args[0]

		// Case 1: direct concatenation: db.Query("SELECT " + id)
		if isStringConcat(firstArg) {
			// Verify it's a real SQL statement (has SELECT/INSERT/UPDATE/DELETE)
			if containsSQLKeyword(firstArg) {
				result.Patterns = append(result.Patterns, &PatternMatch{
					Pattern:  "sql_injection",
					Severity: "critical",
					Message:  "SQL injection: query built with string concatenation - use parameterized queries",
					Category: "security",
				})
			}
			return true
		}

		// Case 2: concatenation inside: db.Query("SELECT * FROM t WHERE id = " + id)
		if binExpr, ok := firstArg.(*ast.BinaryExpr); ok && binExpr.Op == token.ADD {
			if containsSQLKeyword(binExpr) && !containsPlaceholder(binExpr) {
				result.Patterns = append(result.Patterns, &PatternMatch{
					Pattern:  "sql_injection",
					Severity: "critical",
					Message:  "SQL injection: query built with string concatenation - use parameterized queries",
					Category: "security",
				})
			}
		}

		return true
	})
}

// detectHardcodedSecretAST detects REAL hardcoded secrets using the AST.
// A real secret is: an assignment (= or :=) where the value is a string
// literal that looks like a secret (sk-, AKIA, eyJ, password=, etc).
func detectHardcodedSecretAST(file *ast.File, result *AnalysisResult) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			// Check each assignment
			for i, rhs := range node.Rhs {
				// Value must be a string literal
				lit, ok := rhs.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}

				// Strip quotes
				val := strings.Trim(lit.Value, "\"`")

				// Check if it looks like a real secret
				if looksLikeSecret(val) {
					// Get variable name if available
					varName := ""
					if i < len(node.Lhs) {
						if ident, ok := node.Lhs[i].(*ast.Ident); ok {
							varName = ident.Name
						}
					}

					result.Patterns = append(result.Patterns, &PatternMatch{
						Pattern:  "hardcoded_secret",
						Severity: "critical",
						Message:  fmt.Sprintf("Hardcoded secret detected in %s - use environment variables", varName),
						Category: "security",
					})
				}
			}

		case *ast.ValueSpec:
			// var apiKey = "sk-..."
			for _, value := range node.Values {
				lit, ok := value.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}

				val := strings.Trim(lit.Value, "\"`")
				if looksLikeSecret(val) {
					varName := ""
					if len(node.Names) > 0 {
						varName = node.Names[0].Name
					}

					result.Patterns = append(result.Patterns, &PatternMatch{
						Pattern:  "hardcoded_secret",
						Severity: "critical",
						Message:  fmt.Sprintf("Hardcoded secret detected in %s - use environment variables", varName),
						Category: "security",
					})
				}
			}
		}

		return true
	})
}

// ============================================================
// AST HELPERS
// ============================================================

// callFuncName extracts the function name from a call expression.
func callFuncName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	case *ast.IndexExpr:
		return callFuncName(&ast.CallExpr{Fun: fn.X})
	}
	return ""
}

// isQueryCall checks if the function name is a database query call.
func isQueryCall(name string) bool {
	lower := strings.ToLower(name)
	switch lower {
	case "query", "queryrow", "querycontext", "queryrowcontext",
		"exec", "execcontext", "prepare", "preparecontext":
		return true
	}
	return false
}

// isStringConcat checks if an expression is string concatenation.
func isStringConcat(expr ast.Expr) bool {
	binExpr, ok := expr.(*ast.BinaryExpr)
	if !ok {
		return false
	}
	return binExpr.Op == token.ADD
}

// containsSQLKeyword checks if the expression contains SQL keywords.
func containsSQLKeyword(expr ast.Expr) bool {
	text := exprToString(expr)
	lower := strings.ToLower(text)
	for _, kw := range []string{"select", "insert", "update", "delete", "from", "where"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// containsPlaceholder checks if the expression contains parameter placeholders.
func containsPlaceholder(expr ast.Expr) bool {
	text := exprToString(expr)
	return strings.Contains(text, "?") || strings.Contains(text, "$1") ||
		strings.Contains(text, "$2") || strings.Contains(text, ":")
}

// exprToString converts an expression to its source text.
func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return e.Value
	case *ast.BinaryExpr:
		return exprToString(e.X) + " " + e.Op.String() + " " + exprToString(e.Y)
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.CallExpr:
		return callFuncName(e) + "()"
	case *ast.ParenExpr:
		return "(" + exprToString(e.X) + ")"
	case *ast.IndexExpr:
		return exprToString(e.X) + "[...]"
	default:
		return ""
	}
}

// looksLikeSecret checks if a string value looks like a real secret.
func looksLikeSecret(val string) bool {
	lower := strings.ToLower(val)

	// Known secret patterns
	secretPrefixes := []string{
		"sk-", "sk_", "ak-", "pk-", // OpenAI/Stripe style
		"akia", "asas", // AWS access key
		"eyj", // JWT
		"ghp_", "gho_", "ghu_", // GitHub
		"xoxb-", "xoxp-", // Slack
		"-----begin", // Private key
	}

	for _, prefix := range secretPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}

	// Password/secret assignment patterns
	if strings.Contains(lower, "password=") || strings.Contains(lower, "secret=") {
		// Must not be a placeholder
		if !strings.Contains(lower, "getenv") && !strings.Contains(lower, "os.getenv") {
			return true
		}
	}

	// Long hex/base64 strings assigned to secret-like names
	if len(val) >= 32 {
		// Check if it's mostly hex or base64 chars
		hexChars := 0
		for _, c := range val {
			if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
				hexChars++
			}
		}
		if float64(hexChars)/float64(len(val)) > 0.8 {
			return true
		}
	}

	return false
}

// ============================================================
// HELPERS
// ============================================================

func countLines(code string) int {
	if code == "" {
		return 0
	}
	count := 1
	for _, c := range code {
		if c == '\n' {
			count++
		}
	}
	return count
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
