package expert

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

// TestSecurityExpert tests the security expert system.
func TestSecurityExpert(t *testing.T) {
	sys := SecurityExpert()
	if sys == nil {
		t.Fatal("Security expert should not be nil")
	}

	if sys.Domain != "security" {
		t.Errorf("Expected domain security, got %s", sys.Domain)
	}

	// Test with vulnerable code
	ctx := analyzeCode(`package main

import "database/sql"

func getUser(db *sql.DB, id string) {
	db.Query("SELECT * FROM users WHERE id = " + id)
}
`)

	results := sys.Analyze(ctx)
	if len(results) == 0 {
		t.Error("Expected security findings")
	}
}

// TestExpertRegistry tests the expert system registry.
func TestExpertRegistry(t *testing.T) {
	reg := DefaultRegistry()

	systems := reg.All()
	if len(systems) != 5 {
		t.Errorf("Expected 5 expert systems, got %d", len(systems))
	}

	// Verify each domain exists
	domains := make(map[string]bool)
	for _, sys := range systems {
		domains[sys.Domain] = true
	}

	expected := []string{"security", "architecture", "code_quality", "performance", "testing"}
	for _, domain := range expected {
		if !domains[domain] {
			t.Errorf("Missing expert system for domain: %s", domain)
		}
	}

	// Test Get
	sys, ok := reg.Get("security")
	if !ok {
		t.Error("Security expert should exist")
	}
	if sys.Name != "Security Expert" {
		t.Errorf("Expected Security Expert, got %s", sys.Name)
	}
}

// TestSecurityExpertCleanCode tests that clean code passes.
func TestSecurityExpertCleanCode(t *testing.T) {
	sys := SecurityExpert()

	ctx := analyzeCode(`package main

import "database/sql"

type User struct {
	ID   int
	Name string
}

func getUser(db *sql.DB, id int) (*User, error) {
	var user User
	err := db.QueryRow("SELECT id, name FROM users WHERE id = ?", id).
		Scan(&user.ID, &user.Name)
	return &user, err
}
`)

	results := sys.Analyze(ctx)
	if len(results) != 0 {
		t.Errorf("Expected no findings for clean code, got %d", len(results))
	}
}