package rules

import (
	"testing"

	"cosca/internal/dsms/intelligence"
	"cosca/internal/dsms/intelligence/codeanalyzer"
)

// analyzeCode analyzes code and builds a context with AST patterns.
func analyzeCode(code string) *intelligence.Context {
	analyzer := codeanalyzer.NewAnalyzer()
	analysis, err := analyzer.AnalyzeGo("/test/main.go", code)
	if err != nil {
		return &intelligence.Context{Code: code}
	}

	var patterns []string
	for _, p := range analysis.Patterns {
		patterns = append(patterns, p.Pattern)
	}

	return &intelligence.Context{
		Language: analysis.Language,
		FilePath: analysis.FilePath,
		Code:     code,
		Metrics:  analysis.Metrics,
		Patterns: patterns,
	}
}

// TestSecurityRules tests security rule detection.
func TestSecurityRules(t *testing.T) {
	engine := NewEngine()
	if err := engine.RegisterMany(SecurityRules()); err != nil {
		t.Fatalf("Failed to register security rules: %v", err)
	}

	// Test SQL injection detection (real AST-based)
	ctx := analyzeCode(`package main

import "database/sql"

func getUser(db *sql.DB, id string) {
	db.Query("SELECT * FROM users WHERE id = " + id)
}
`)

	results := engine.EvaluateDomain(ctx, "security")
	if len(results) == 0 {
		t.Error("Expected SQL injection detection")
	}

	found := false
	for _, r := range results {
		if r.RuleID == "SEC-001" {
			found = true
			if r.Severity != "critical" {
				t.Errorf("Expected critical severity, got %s", r.Severity)
			}
		}
	}
	if !found {
		t.Error("SEC-001 rule should have matched")
	}
}

// TestHardcodedSecret tests hardcoded secret detection.
func TestHardcodedSecret(t *testing.T) {
	engine := NewEngine()
	if err := engine.RegisterMany(SecurityRules()); err != nil {
		t.Fatalf("Failed to register security rules: %v", err)
	}

	ctx := analyzeCode(`package main

var apiKey = "sk-1234567890abcdef"
`)

	results := engine.EvaluateDomain(ctx, "security")
	if len(results) == 0 {
		t.Error("Expected hardcoded secret detection")
	}

	found := false
	for _, r := range results {
		if r.RuleID == "SEC-002" {
			found = true
		}
	}
	if !found {
		t.Error("SEC-002 rule should have matched")
	}
}

// TestArchitectureRules tests architecture rules.
func TestArchitectureRules(t *testing.T) {
	engine := NewEngine()
	if err := engine.RegisterMany(ArchitectureRules()); err != nil {
		t.Fatalf("Failed to register architecture rules: %v", err)
	}

	// Test god object detection
	ctx := &intelligence.Context{
		Metrics: map[string]float64{
			"lines": 600,
		},
	}

	results := engine.EvaluateDomain(ctx, "architecture")
	if len(results) == 0 {
		t.Error("Expected god object detection")
	}

	found := false
	for _, r := range results {
		if r.RuleID == "ARCH-001" {
			found = true
		}
	}
	if !found {
		t.Error("ARCH-001 rule should have matched")
	}
}

// TestCodeQualityRules tests code quality rules.
func TestCodeQualityRules(t *testing.T) {
	engine := NewEngine()
	if err := engine.RegisterMany(CodeQualityRules()); err != nil {
		t.Fatalf("Failed to register code quality rules: %v", err)
	}

	ctx := &intelligence.Context{
		Code: "// TODO: implement this later",
	}

	results := engine.EvaluateDomain(ctx, "code_quality")
	if len(results) == 0 {
		t.Error("Expected TODO detection")
	}

	found := false
	for _, r := range results {
		if r.RuleID == "CODE-001" {
			found = true
		}
	}
	if !found {
		t.Error("CODE-001 rule should have matched")
	}
}

// TestAllDefaultRules tests all default rules.
func TestAllDefaultRules(t *testing.T) {
	engine := NewEngine()
	if err := engine.RegisterMany(AllDefaultRules()); err != nil {
		t.Fatalf("Failed to register all rules: %v", err)
	}

	ctx := &intelligence.Context{
		Code: `func main() {
	// TODO: add validation
	db.Query("SELECT * FROM users WHERE id = " + id)
}`,
		Metrics: map[string]float64{
			"lines": 600,
		},
	}

	results := engine.Evaluate(ctx)
	if len(results) < 3 {
		t.Errorf("Expected at least 3 rule matches, got %d", len(results))
	}
}