package distill

import (
	"math"
	"testing"

	"cosca/internal/dsms/intelligence"
)

// TestDistillFromKnowledge tests knowledge distillation.
func TestDistillFromKnowledge(t *testing.T) {
	distiller := NewDistiller()

	entries := []KnowledgeEntry{
		{
			ID:         1,
			Content:    "SQL injection prevention: always use parameterized queries",
			Category:   "security",
			Domain:     "security",
			Confidence: 0.95,
		},
		{
			ID:         2,
			Content:    "N+1 query problem: batch load related data",
			Category:   "performance",
			Domain:     "performance",
			Confidence: 0.85,
		},
		{
			ID:         3,
			Content:    "God object anti-pattern: split large files",
			Category:   "code_quality",
			Domain:     "code_quality",
			Confidence: 0.75,
		},
		{
			ID:         4,
			Content:    "Architecture best practices: low coupling",
			Category:   "architecture",
			Domain:     "architecture",
			Confidence: 0.8,
		},
		{
			ID:         5,
			Content:    "Unrelated content that doesn't match any pattern",
			Category:   "general",
			Domain:     "general",
			Confidence: 0.5,
		},
	}

	rules := distiller.DistillFromKnowledge(entries)

	// Should extract 4 rules (entry 5 doesn't match)
	if len(rules) != 4 {
		t.Errorf("Expected 4 rules, got %d", len(rules))
	}

	// Verify domains
	domains := make(map[string]bool)
	for _, rule := range rules {
		domains[rule.Domain] = true
	}

	if !domains["security"] {
		t.Error("Expected security rule")
	}
	if !domains["performance"] {
		t.Error("Expected performance rule")
	}
	if !domains["code_quality"] {
		t.Error("Expected code_quality rule")
	}
	if !domains["architecture"] {
		t.Error("Expected architecture rule")
	}
}

// TestApplyFeedback tests the feedback loop.
func TestApplyFeedback(t *testing.T) {
	distiller := NewDistiller()

	rule := &intelligence.Rule{
		ID:         "TEST-001",
		Domain:     "security",
		Name:       "Test Rule",
		Confidence: 0.5,
		Version:    1,
		Enabled:    true,
	}

	// Apply correct feedback
	distiller.ApplyFeedback(rule, &Feedback{
		RuleID:  "TEST-001",
		Outcome: "correct",
	})

	if rule.Confidence != 0.55 {
		t.Errorf("Expected confidence 0.55, got %f", rule.Confidence)
	}
	if rule.Version != 2 {
		t.Errorf("Expected version 2, got %d", rule.Version)
	}

	// Apply incorrect feedback multiple times
	for i := 0; i < 5; i++ {
		distiller.ApplyFeedback(rule, &Feedback{
			RuleID:  "TEST-001",
			Outcome: "incorrect",
		})
	}

	// Confidence should be 0.55 - 5*0.10 = 0.05
	if math.Abs(rule.Confidence-0.05) > 0.0001 {
		t.Errorf("Expected confidence ~0.05, got %f", rule.Confidence)
	}

	// One more incorrect should disable the rule
	distiller.ApplyFeedback(rule, &Feedback{
		RuleID:  "TEST-001",
		Outcome: "incorrect",
	})

	if rule.Enabled {
		t.Error("Rule should be disabled after repeated failures")
	}
}