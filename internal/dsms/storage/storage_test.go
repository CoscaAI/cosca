package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"cosca/internal/dsms/intelligence"
)

// TestSaveAndLoad tests round-trip persistence.
func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	s := NewStorage(dir)

	// Create test rules
	now := time.Now()
	rules := []*intelligence.Rule{
		{
			ID:          "TEST-001",
			Domain:      "security",
			Name:        "SQL Injection",
			Description: "Test rule",
			Condition: intelligence.Condition{
				Operator: "HAS_PATTERN",
				Field:    "patterns",
				Value:    "sql_injection",
			},
			Action: intelligence.Action{
				Type:     "flag",
				Severity: "critical",
				Message:  "SQL injection detected",
			},
			Priority:   100,
			Confidence: 0.95,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		{
			ID:          "TEST-002",
			Domain:      "performance",
			Name:        "N+1 Query",
			Description: "Test rule 2",
			Condition: intelligence.Condition{
				Operator: "AND",
				Children: []intelligence.Condition{
					{Operator: "CONTAINS", Field: "code", Value: "for "},
					{Operator: "CONTAINS", Field: "code", Value: "query"},
				},
			},
			Action: intelligence.Action{
				Type:     "flag",
				Severity: "warning",
				Message:  "N+1 detected",
			},
			Priority:   80,
			Confidence: 0.8,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	}

	// Save
	path, err := s.SaveRules("test", rules)
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	t.Logf("Salvo em: %s", path)

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not found: %v", err)
	}

	// Load
	loaded, err := s.LoadRules()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("Expected 2 rules, got %d", len(loaded))
	}

	// Verify content
	if loaded[0].ID != "TEST-001" {
		t.Errorf("Expected TEST-001, got %s", loaded[0].ID)
	}
	if loaded[0].Domain != "security" {
		t.Errorf("Expected security, got %s", loaded[0].Domain)
	}
	if loaded[0].Condition.Operator != "HAS_PATTERN" {
		t.Errorf("Expected HAS_PATTERN, got %s", loaded[0].Condition.Operator)
	}
	if loaded[1].ID != "TEST-002" {
		t.Errorf("Expected TEST-002, got %s", loaded[1].ID)
	}

	t.Logf("✅ Round-trip OK: %d regras salvas e carregadas", len(loaded))
}

// TestLoadFile tests loading a specific file.
func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	s := NewStorage(dir)

	now := time.Now()
	rule := &intelligence.Rule{
		ID:          "SINGLE-001",
		Domain:      "testing",
		Name:        "Single Rule",
		Confidence:  0.5,
		Enabled:     true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	path, err := s.SaveRules("single", []*intelligence.Rule{rule})
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load file: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(loaded))
	}

	if loaded[0].ID != "SINGLE-001" {
		t.Errorf("Expected SINGLE-001, got %s", loaded[0].ID)
	}

	t.Logf("✅ LoadFile OK")
}

// TestDefaultDir verifies default directory.
func TestDefaultDir(t *testing.T) {
	dir := DefaultDir()
	if dir == "" {
		t.Error("DefaultDir should not be empty")
	}
	t.Logf("Default dir: %s", filepath.Clean(dir))
}