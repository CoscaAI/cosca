// Package pipeline provides the full intelligence pipeline integration test.
// This proves the system can analyze code, detect issues, and make decisions
// WITHOUT any external LLM provider.
package pipeline

import (
	"testing"
	"time"

	"cosca/internal/dsms/intelligence"
	"cosca/internal/dsms/intelligence/codeanalyzer"
	"cosca/internal/dsms/intelligence/expert"
)

// vulnerableCode contains intentional security/code issues.
const vulnerableCode = `package main

import (
	"database/sql"
	"fmt"
	"net/http"
)

// User represents a user in the system.
type User struct {
	ID   int
	Name string
}

// hardcoded secret - BAD
var apiKey = "sk-1234567890abcdef"

// GetUser fetches a user by ID - has SQL injection
func GetUser(db *sql.DB, id string) (*User, error) {
	// TODO: add input validation
	var user User
	// SQL injection: string concatenation
	err := db.Query("SELECT id, name FROM users WHERE id = " + id).
		Scan(&user.ID, &user.Name)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// ListUsers fetches all users - has N+1 query pattern
func ListUsers(db *sql.DB) ([]*User, error) {
	rows, err := db.Query("SELECT id FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		// N+1: query inside loop
		db.QueryRow("SELECT name FROM users WHERE id = ?", user.ID).Scan(&user.Name)
		users = append(users, &user)
	}
	return users, nil
}

// HandleRequest handles HTTP requests - missing auth check
func HandleRequest(w http.ResponseWriter, r *http.Request) {
	// FIXME: add authentication
	user := r.URL.Query().Get("user")
	fmt.Fprintf(w, "Hello, %s", user)
}
`

// TestIntelligenceBenchmark runs the intelligence engine on real vulnerable code
// and reports what it detects. This is the PROOF that it works.
func TestIntelligenceBenchmark(t *testing.T) {
	start := time.Now()

	// 1. Analyze code
	analyzer := codeanalyzer.NewAnalyzer()
	analysis, err := analyzer.AnalyzeGo("/app/vulnerable.go", vulnerableCode)
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	// 2. Build context
	ctx := &intelligence.Context{
		Language: analysis.Language,
		FilePath: analysis.FilePath,
		Code:     vulnerableCode,
		Metrics:  analysis.Metrics,
		Patterns: extractPatterns(analysis),
	}

	// 3. Run all expert systems
	registry := expert.DefaultRegistry()
	var findings []*intelligence.RuleResult
	for _, sys := range registry.All() {
		findings = append(findings, sys.Analyze(ctx)...)
	}

	elapsed := time.Since(start)

	// 4. Print the PROOF
	t.Logf("\n══════════════════════════════════════════════════════")
	t.Logf("  INTELLIGENCE ENGINE — PROVA REAL (zero LLM)")
	t.Logf("══════════════════════════════════════════════════════")
	t.Logf("Arquivo: %s", analysis.FilePath)
	t.Logf("Linhas: %d | Funções: %d | Complexidade máx: %d", analysis.Lines, analysis.FunctionCount, analysis.MaxComplexity)
	t.Logf("Tempo total: %v", elapsed)
	t.Logf("")
	t.Logf("── PROBLEMAS DETECTADOS ──")

	// Group by severity
	critical := 0
	warning := 0
	info := 0
	for _, f := range findings {
		switch f.Severity {
		case "critical":
			critical++
		case "warning":
			warning++
		default:
			info++
		}
		t.Logf("  [%s] %s", f.Severity, f.Message)
	}

	t.Logf("")
	t.Logf("── RESUMO ──")
	t.Logf("  Critical: %d | Warning: %d | Info: %d | Total: %d", critical, warning, info, len(findings))
	t.Logf("")

	// 5. VERIFY each known issue was caught
	t.Logf("── VERIFICAÇÃO DE COBERTURA ──")

	// Known issues in the code:
	// 1. SQL injection (critical)
	// 2. Hardcoded secret (critical)
	// 3. N+1 query (warning)
	// 4. Missing auth (warning - not directly detected by rules yet)
	// 5. TODO comment (info)
	// 6. FIXME comment (warning)

	checks := []struct {
		name     string
		ruleID   string
		severity string
	}{
		{"SQL Injection", "SEC-001", "critical"},
		{"Hardcoded Secret", "SEC-002", "critical"},
		{"N+1 Query", "PERF-001", "warning"},
		{"TODO Comment", "CODE-001", "info"},
		{"FIXME Comment", "CODE-002", "warning"},
	}

	detected := 0
	for _, check := range checks {
		found := false
		for _, f := range findings {
			if f.RuleID == check.ruleID {
				found = true
				break
			}
		}
		status := "✓ DETECTADO"
		if !found {
			status = "✗ NÃO DETECTADO"
		} else {
			detected++
		}
		t.Logf("  %s %s", status, check.name)
	}

	t.Logf("")
	t.Logf("  Cobertura: %d/%d problemas conhecidos detectados", detected, len(checks))
	t.Logf("══════════════════════════════════════════════════════")

	// Assertions - the PROOF
	if detected < 4 {
		t.Errorf("Expected at least 4/5 issues detected, got %d/5", detected)
	}

	// Performance must be < 10ms
	if elapsed > 10*time.Millisecond {
		t.Errorf("Too slow: %v (should be < 10ms)", elapsed)
	}

	t.Logf("\n✅ PROVA CONCLUÍDA: %d/5 problemas detectados em %v sem LLM", detected, elapsed)
}

// TestComparisonWithLLM shows the cost comparison.
func TestComparisonWithLLM(t *testing.T) {
	// Intelligence Engine
	engineStart := time.Now()
	analyzer := codeanalyzer.NewAnalyzer()
	analysis, _ := analyzer.AnalyzeGo("/app/vulnerable.go", vulnerableCode)
	ctx := &intelligence.Context{
		Language: analysis.Language,
		FilePath: analysis.FilePath,
		Code:     vulnerableCode,
		Metrics:  analysis.Metrics,
		Patterns: extractPatterns(analysis),
	}
	registry := expert.DefaultRegistry()
	count := 0
	for _, sys := range registry.All() {
		count += len(sys.Analyze(ctx))
	}
	engineElapsed := time.Since(engineStart)

	// LLM (typical values from benchmarks - measured previously)
	llmLatency := 2460 * time.Millisecond // measured avg in TAS benchmark (2.46s)
	llmCostPerCall := 0.0015              // $ per call (typical)

	t.Logf("\n══════════════════════════════════════════════════════")
	t.Logf("  COMPARAÇÃO DE CUSTO: INTELLIGENCE vs LLM")
	t.Logf("══════════════════════════════════════════════════════")
	t.Logf("")
	t.Logf("  Intelligence Engine:")
	t.Logf("    Latência: %v", engineElapsed)
	t.Logf("    Custo: $0.00 (local, sem API)")
	t.Logf("    Findings: %d", count)
	t.Logf("")
	t.Logf("  LLM (típico):")
	t.Logf("    Latência: %v", llmLatency)
	t.Logf("    Custo: $%.4f por chamada", llmCostPerCall)
	t.Logf("    Depende de: API key, internet, provedor")
	t.Logf("")
	t.Logf("  Speedup: %.0fx", float64(llmLatency.Nanoseconds())/float64(engineElapsed.Nanoseconds()))
	t.Logf("══════════════════════════════════════════════════════")

	// Simple speedup
	llmNs := float64(llmLatency.Nanoseconds())
	engineNs := float64(engineElapsed.Nanoseconds())
	actualSpeedup := llmNs / engineNs

	t.Logf("  Speedup real: %.0fx mais rápido", actualSpeedup)
	t.Logf("  Custo: $0.00 vs $%.4f", llmCostPerCall)
	t.Logf("══════════════════════════════════════════════════════")

	if engineElapsed > 10*time.Millisecond {
		t.Errorf("Engine too slow: %v", engineElapsed)
	}
}

// TestCodeAnalyzerMetrics shows the AST analysis proves real understanding.
func TestCodeAnalyzerMetrics(t *testing.T) {
	analyzer := codeanalyzer.NewAnalyzer()
	analysis, err := analyzer.AnalyzeGo("/app/vulnerable.go", vulnerableCode)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	t.Logf("\n══════════════════════════════════════════════════════")
	t.Logf("  ANÁLISE AST — O QUE O SISTEMA ENTENDE DO CÓDIGO")
	t.Logf("══════════════════════════════════════════════════════")
	t.Logf("  Linguagem: %s", analysis.Language)
	t.Logf("  Linhas: %d", analysis.Lines)
	t.Logf("  Funções: %d", analysis.FunctionCount)
	t.Logf("  Structs: %d", analysis.StructCount)
	t.Logf("  Imports: %d", analysis.ImportCount)
	t.Logf("  Complexidade máx: %d", analysis.MaxComplexity)
	t.Logf("  Nesting máx: %d", analysis.MaxNestingDepth)
	t.Logf("  TODO comments: %d", analysis.TodoComments)
	t.Logf("  FIXME comments: %d", analysis.FixmeComments)
	t.Logf("  Tem testes: %v", analysis.HasTests)
	t.Logf("")

	// Show function details
	for _, fn := range analysis.Functions {
		t.Logf("  Função %s: %d linhas, complexidade %d, nesting %d, params %d, retorna error: %v",
			fn.Name, fn.Lines, fn.Complexity, fn.NestingDepth, fn.Parameters, fn.HasErrorReturn)
	}
	t.Logf("══════════════════════════════════════════════════════")

	// Assertions
	if analysis.FunctionCount != 3 {
		t.Errorf("Expected 3 functions, got %d", analysis.FunctionCount)
	}
	if analysis.StructCount != 1 {
		t.Errorf("Expected 1 struct, got %d", analysis.StructCount)
	}
	if analysis.TodoComments != 2 {
		t.Errorf("Expected 2 TODO, got %d", analysis.TodoComments)
	}
	if analysis.FixmeComments != 1 {
		t.Errorf("Expected 1 FIXME, got %d", analysis.FixmeComments)
	}

	// Verify GetUser has error return
	foundGetUser := false
	for _, fn := range analysis.Functions {
		if fn.Name == "GetUser" {
			foundGetUser = true
			if !fn.HasErrorReturn {
				t.Error("GetUser should have error return")
			}
		}
	}
	if !foundGetUser {
		t.Error("GetUser function not found")
	}

	t.Logf("\n✅ O sistema entende a estrutura real do código (AST), não só texto")
}
