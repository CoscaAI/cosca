// Package distill implements knowledge distillation.
// This extracts deterministic rules from the knowledge base,
// converting learned patterns into executable rules.
package distill

import (
	"fmt"
	"strings"
	"time"

	"cosca/internal/dsms/intelligence"
)

// ============================================================
// KNOWLEDGE DISTILLATION
// ============================================================

// Distiller extracts rules from knowledge.
type Distiller struct {
	rules []*intelligence.Rule
}

// NewDistiller creates a new distiller.
func NewDistiller() *Distiller {
	return &Distiller{
		rules: make([]*intelligence.Rule, 0),
	}
}

// DistillFromKnowledge converts knowledge entries into rules.
// This is the core of "learning from experience" without LLM.
func (d *Distiller) DistillFromKnowledge(entries []KnowledgeEntry) []*intelligence.Rule {
	var rules []*intelligence.Rule

	for _, entry := range entries {
		// Skip empty entries
		if strings.TrimSpace(entry.Content) == "" {
			continue
		}

		// Extract rule based on content patterns
		rule := d.extractRule(entry)
		if rule != nil {
			rules = append(rules, rule)
		}
	}

	return rules
}

// KnowledgeEntry represents a knowledge base entry.
type KnowledgeEntry struct {
	ID         int     `json:"id"`
	Content    string  `json:"content"`
	Category   string  `json:"category"`
	Domain     string  `json:"domain"`
	Confidence float64 `json:"confidence"`
}

// extractRule extracts a rule from a knowledge entry.
func (d *Distiller) extractRule(entry KnowledgeEntry) *intelligence.Rule {
	content := strings.ToLower(entry.Content)
	now := time.Now()

	// Extract rules based on content patterns
	// Each pattern maps to a domain-specific rule

	// Security patterns
	if containsAny(content, []string{"sql injection", "injection", "cwe-89"}) {
		return &intelligence.Rule{
			ID:          fmt.Sprintf("KD-SEC-%d", entry.ID),
			Domain:      "security",
			Name:        "SQL Injection Pattern",
			Description: "Distilled from knowledge: SQL injection prevention",
			Condition: intelligence.Condition{
				Operator: "AND",
				Children: []intelligence.Condition{
					{Operator: "CONTAINS", Field: "code", Value: "query"},
					{Operator: "NOT", Children: []intelligence.Condition{
						{Operator: "CONTAINS", Field: "code", Value: "?"},
					}},
				},
			},
			Action: intelligence.Action{
				Type:     "flag",
				Severity: "critical",
				Message:  "Potential SQL injection - use parameterized queries",
			},
			Priority:   90,
			Confidence: entry.Confidence,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
	}

	// Performance patterns
	if containsAny(content, []string{"n+1", "query loop", "performance"}) {
		return &intelligence.Rule{
			ID:          fmt.Sprintf("KD-PERF-%d", entry.ID),
			Domain:      "performance",
			Name:        "N+1 Query Pattern",
			Description: "Distilled from knowledge: N+1 query prevention",
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
				Message:  "N+1 query pattern - consider batch loading",
			},
			Priority:   80,
			Confidence: entry.Confidence,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
	}

	// Code quality patterns
	if containsAny(content, []string{"god object", "too large", "refactor"}) {
		return &intelligence.Rule{
			ID:          fmt.Sprintf("KD-CODE-%d", entry.ID),
			Domain:      "code_quality",
			Name:        "Code Size Pattern",
			Description: "Distilled from knowledge: code size management",
			Condition: intelligence.Condition{
				Operator: "GT",
				Field:    "metrics.lines",
				Value:    500.0,
			},
			Action: intelligence.Action{
				Type:     "suggest",
				Severity: "warning",
				Message:  "File too large - consider refactoring",
			},
			Priority:   70,
			Confidence: entry.Confidence,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
	}

	// Architecture patterns
	if containsAny(content, []string{"architecture", "design pattern", "coupling"}) {
		return &intelligence.Rule{
			ID:          fmt.Sprintf("KD-ARCH-%d", entry.ID),
			Domain:      "architecture",
			Name:        "Architecture Pattern",
			Description: "Distilled from knowledge: architecture best practices",
			Condition: intelligence.Condition{
				Operator: "GT",
				Field:    "metrics.coupling",
				Value:    0.7,
			},
			Action: intelligence.Action{
				Type:     "suggest",
				Severity: "warning",
				Message:  "High coupling detected - consider decoupling",
			},
			Priority:   65,
			Confidence: entry.Confidence,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
	}

	return nil
}

// ============================================================
// FEEDBACK LOOP
// ============================================================

// Feedback represents feedback on a rule's effectiveness.
type Feedback struct {
	RuleID     string    `json:"rule_id"`
	Outcome    string    `json:"outcome"` // correct, incorrect, partial
	Confidence float64   `json:"confidence"`
	Context    string    `json:"context"`
	CreatedAt  time.Time `json:"created_at"`
}

// ApplyFeedback adjusts rule confidence based on feedback.
// This is the learning loop - rules get better over time.
func (d *Distiller) ApplyFeedback(rule *intelligence.Rule, feedback *Feedback) {
	if rule == nil || feedback == nil {
		return
	}

	switch feedback.Outcome {
	case "correct":
		// Increase confidence
		rule.Confidence = min(1.0, rule.Confidence+0.05)
		rule.Version++
	case "incorrect":
		// Decrease confidence
		rule.Confidence = max(0.0, rule.Confidence-0.10)
		rule.Version++
		if rule.Confidence < 0.2 {
			// Rule is unreliable - disable it
			rule.Enabled = false
		}
	case "partial":
		// Slight adjustment
		rule.Confidence = min(1.0, max(0.0, rule.Confidence+0.02))
		rule.Version++
	}

	rule.UpdatedAt = time.Now()
}

// ============================================================
// HELPERS
// ============================================================

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
