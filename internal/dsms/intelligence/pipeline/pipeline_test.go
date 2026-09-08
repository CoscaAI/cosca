// Package pipeline provides the full intelligence pipeline integration test.
// This proves the system can analyze code, detect issues, and make decisions
// WITHOUT any external LLM provider.
package pipeline

import (
	"testing"
	"time"

	"cosca/internal/dsms/intelligence"
	"cosca/internal/dsms/intelligence/codeanalyzer"
	"cosca/internal/dsms/intelligence/decision"
	"cosca/internal/dsms/intelligence/expert"
	"cosca/internal/dsms/intelligence/rules"
)

// extractPatterns extracts pattern names from analysis results.
func extractPatterns(analysis *codeanalyzer.AnalysisResult) []string {
	if analysis == nil {
		return nil
	}
	var patterns []string
	for _, p := range analysis.Patterns {
		patterns = append(patterns, p.Pattern)
	}
	return patterns
}

// TestFullIntelligencePipeline tests the complete intelligence pipeline
// WITHOUT any external LLM provider.
func TestFullIntelligencePipeline(t *testing.T) {
	start := time.Now()

	// 1. Analyze code with AST
	analyzer := codeanalyzer.NewAnalyzer()

	code := `package main

import (
	"database/sql"
	"fmt"
)

// User represents a user.
type User struct {
	ID   int
	Name string
}

// GetUser fetches a user by ID.
func GetUser(db *sql.DB, id string) (*User, error) {
	// TODO: add caching
	var user User
	err := db.Query("SELECT id, name FROM users WHERE id = " + id).
		Scan(&user.ID, &user.Name)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
`

	analysis, err := analyzer.AnalyzeGo("/app/main.go", code)
	if err != nil {
		t.Fatalf("Failed to analyze code: %v", err)
	}

	// 2. Build intelligence context from analysis
	ctx := &intelligence.Context{
		Language: analysis.Language,
		FilePath: analysis.FilePath,
		Code:     code,
		Metrics:  analysis.Metrics,
		Patterns: extractPatterns(analysis),
	}

	// 3. Run expert systems
	registry := expert.DefaultRegistry()
	var allFindings []*intelligence.RuleResult

	for _, sys := range registry.All() {
		findings := sys.Analyze(ctx)
		allFindings = append(allFindings, findings...)
	}

	// 4. Run decision trees
	decisionEngine := decision.NewEngine()
	decisionEngine.RegisterTree(decision.TaskRoutingTree())
	decisionEngine.RegisterTree(decision.RiskAssessmentTree())

	taskCtx := &intelligence.Context{
		Fields: map[string]interface{}{
			"task": map[string]interface{}{
				"type": "security_review",
			},
		},
	}

	routing, err := decisionEngine.Decide("task-routing", taskCtx)
	if err != nil {
		t.Fatalf("Failed to route task: %v", err)
	}

	riskCtx := &intelligence.Context{
		Metrics: map[string]float64{
			"critical_functions": 1,
			"test_coverage":      0.5,
			"affected_files":     3,
		},
	}

	risk, err := decisionEngine.Decide("risk-assessment", riskCtx)
	if err != nil {
		t.Fatalf("Failed to assess risk: %v", err)
	}

	// 5. Verify results
	elapsed := time.Since(start)

	t.Logf("=== INTELLIGENCE PIPELINE (zero LLM) ===\n")
	t.Logf("Code analyzed: %s (%d lines, %d functions)\n", analysis.FilePath, analysis.Lines, analysis.FunctionCount)
	t.Logf("Findings: %d\n", len(allFindings))
	for _, f := range allFindings {
		t.Logf("  [%s] %s: %s\n", f.Severity, f.RuleName, f.Message)
	}
	t.Logf("Task routing: %s (%.0f%%)\n", routing.Decision, routing.Confidence*100)
	t.Logf("Risk assessment: %s (%.0f%%)\n", risk.Decision, risk.Confidence*100)
	t.Logf("Total time: %v\n", elapsed)

	// Assertions
	if len(allFindings) == 0 {
		t.Error("Expected findings from code analysis")
	}

	// Verify SQL injection detected
	foundSQLi := false
	for _, f := range allFindings {
		if f.RuleID == "SEC-001" {
			foundSQLi = true
		}
	}
	if !foundSQLi {
		t.Error("SQL injection should have been detected")
	}

	// Verify routing
	if routing.Decision != "route_to_security" {
		t.Errorf("Expected route_to_security, got %s", routing.Decision)
	}

	// Verify risk
	if risk.Decision != "high_risk" {
		t.Errorf("Expected high_risk, got %s", risk.Decision)
	}

	// Performance: should be well under 100ms
	if elapsed > 100*time.Millisecond {
		t.Errorf("Pipeline too slow: %v (should be < 100ms)", elapsed)
	}

	t.Logf("\n✅ Full intelligence pipeline completed in %v WITHOUT LLM", elapsed)
}

// TestDistillationIntegration tests knowledge distillation into the engine.
func TestDistillationIntegration(t *testing.T) {
	// Create rules engine
	engine := rules.NewEngine()
	engine.RegisterMany(rules.AllDefaultRules())

	// Distill rules from knowledge entries
	distilled := []struct {
		content    string
		domain     string
		confidence float64
	}{
		{"SQL injection prevention: always use parameterized queries", "security", 0.95},
		{"N+1 query problem: batch load related data", "performance", 0.85},
		{"God object anti-pattern: split large files", "code_quality", 0.75},
	}

	for i, d := range distilled {
		rule := &intelligence.Rule{
			ID:          string(rune('A'+i)) + "-DISTILLED",
			Domain:      d.domain,
			Name:        "Distilled Rule",
			Description: d.content,
			Condition: intelligence.Condition{
				Operator: "CONTAINS",
				Field:    "code",
				Value:    d.content,
			},
			Action: intelligence.Action{
				Type:     "flag",
				Severity: "warning",
				Message:  d.content,
			},
			Priority:   50,
			Confidence: d.confidence,
			Version:    1,
			Enabled:    true,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		engine.Register(rule)
	}

	// Verify distilled rules work
	ctx := &intelligence.Context{
		Code: "SQL injection prevention: always use parameterized queries",
	}
	results := engine.Evaluate(ctx)
	if len(results) == 0 {
		t.Error("Expected distilled rule to match")
	}

	t.Log("✅ Distillation integration complete")
}