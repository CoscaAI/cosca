// Package rules provides the deterministic rule engine.
// This is the core that evaluates rules against code/knowledge
// context to make decisions WITHOUT external LLM providers.
package rules

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"cosca/internal/dsms/intelligence"
)

// ============================================================
// RULE ENGINE
// ============================================================

// Engine is the rule engine.
type Engine struct {
	// Rules organized by domain for fast lookup
	byDomain map[string][]*intelligence.Rule
	// All rules
	all []*intelligence.Rule
}

// NewEngine creates a new rule engine.
func NewEngine() *Engine {
	return &Engine{
		byDomain: make(map[string][]*intelligence.Rule),
		all:      make([]*intelligence.Rule, 0),
	}
}

// Register registers a rule.
func (e *Engine) Register(rule *intelligence.Rule) error {
	if rule == nil {
		return fmt.Errorf("rule is nil")
	}
	if rule.ID == "" {
		return fmt.Errorf("rule ID is required")
	}

	e.all = append(e.all, rule)
	e.byDomain[rule.Domain] = append(e.byDomain[rule.Domain], rule)

	return nil
}

// RegisterMany registers multiple rules.
func (e *Engine) RegisterMany(rules []*intelligence.Rule) error {
	for _, rule := range rules {
		if err := e.Register(rule); err != nil {
			return err
		}
	}
	return nil
}

// Evaluate evaluates all rules against context.
func (e *Engine) Evaluate(ctx *intelligence.Context) []*intelligence.RuleResult {
	var results []*intelligence.RuleResult

	for _, rule := range e.all {
		if !rule.Enabled {
			continue
		}

		if intelligence.EvaluateCondition(&rule.Condition, ctx) {
			results = append(results, &intelligence.RuleResult{
				RuleID:      rule.ID,
				RuleName:    rule.Name,
				Domain:      rule.Domain,
				Matched:     true,
				Confidence:  rule.Confidence,
				Severity:    rule.Action.Severity,
				Message:     rule.Action.Message,
				Action:      rule.Action,
				EvaluatedAt: time.Now(),
			})
		}
	}

	// Sort by confidence descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Confidence > results[j].Confidence
	})

	return results
}

// EvaluateDomain evaluates rules for a specific domain.
func (e *Engine) EvaluateDomain(ctx *intelligence.Context, domain string) []*intelligence.RuleResult {
	var results []*intelligence.RuleResult

	rules := e.byDomain[domain]
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		if intelligence.EvaluateCondition(&rule.Condition, ctx) {
			results = append(results, &intelligence.RuleResult{
				RuleID:      rule.ID,
				RuleName:    rule.Name,
				Domain:      rule.Domain,
				Matched:     true,
				Confidence:  rule.Confidence,
				Severity:    rule.Action.Severity,
				Message:     rule.Action.Message,
				Action:      rule.Action,
				EvaluatedAt: time.Now(),
			})
		}
	}

	return results
}

// ============================================================
// BUILT-IN RULES
// ============================================================

// SecurityRules returns default security rules.
func SecurityRules() []*intelligence.Rule {
	now := time.Now()

	return []*intelligence.Rule{
		{
			ID:          "SEC-001",
			Domain:      "security",
			Name:        "SQL Injection",
			Description: "Detect SQL injection via AST analysis (query + concatenation + no placeholder)",
			Condition: intelligence.Condition{
				Operator: "HAS_PATTERN",
				Field:    "patterns",
				Value:    "sql_injection",
			},
			Action: intelligence.Action{
				Type:     "flag",
				Severity: "critical",
				Message:  "SQL injection: query built with string concatenation - use parameterized queries",
				Params: map[string]interface{}{
					"cwe": "CWE-89",
				},
			},
			Priority:   100,
			Confidence: 0.95,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		{
			ID:          "SEC-002",
			Domain:      "security",
			Name:        "Hardcoded Secret",
			Description: "Detect hardcoded secrets via AST analysis (real secret values)",
			Condition: intelligence.Condition{
				Operator: "HAS_PATTERN",
				Field:    "patterns",
				Value:    "hardcoded_secret",
			},
			Action: intelligence.Action{
				Type:     "flag",
				Severity: "critical",
				Message:  "Hardcoded secret detected - use environment variables",
				Params: map[string]interface{}{
					"cwe": "CWE-798",
				},
			},
			Priority:   95,
			Confidence: 0.95,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		{
			ID:          "SEC-003",
			Domain:      "security",
			Name:        "Path Traversal",
			Description: "Detect path traversal vulnerability",
			Condition: intelligence.Condition{
				Operator: "AND",
				Children: []intelligence.Condition{
					{
						Operator: "CONTAINS",
						Field:    "code",
						Value:    "filepath.Join",
					},
					{
						Operator: "CONTAINS",
						Field:    "code",
						Value:    "..",
					},
				},
			},
			Action: intelligence.Action{
				Type:     "flag",
				Severity: "critical",
				Message:  "Potential path traversal vulnerability",
				Params: map[string]interface{}{
					"cwe": "CWE-22",
				},
			},
			Priority:   90,
			Confidence: 0.8,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	}
}

// ArchitectureRules returns default architecture rules.
func ArchitectureRules() []*intelligence.Rule {
	now := time.Now()

	return []*intelligence.Rule{
		{
			ID:          "ARCH-001",
			Domain:      "architecture",
			Name:        "God Object",
			Description: "Detect classes/files that are too large",
			Condition: intelligence.Condition{
				Operator: "GT",
				Field:    "metrics.lines",
				Value:    500.0,
			},
			Action: intelligence.Action{
				Type:     "suggest",
				Severity: "warning",
				Message:  "File has more than 500 lines - consider splitting",
			},
			Priority:   70,
			Confidence: 0.7,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		{
			ID:          "ARCH-002",
			Domain:      "architecture",
			Name:        "High Cyclomatic Complexity",
			Description: "Detect functions with high complexity",
			Condition: intelligence.Condition{
				Operator: "GT",
				Field:    "metrics.cyclomatic_complexity",
				Value:    10.0,
			},
			Action: intelligence.Action{
				Type:     "suggest",
				Severity: "warning",
				Message:  "Function has high cyclomatic complexity - consider refactoring",
			},
			Priority:   65,
			Confidence: 0.75,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		{
			ID:          "ARCH-003",
			Domain:      "architecture",
			Name:        "Deep Nesting",
			Description: "Detect excessive nesting depth",
			Condition: intelligence.Condition{
				Operator: "GT",
				Field:    "metrics.nesting_depth",
				Value:    5.0,
			},
			Action: intelligence.Action{
				Type:     "flag",
				Severity: "warning",
				Message:  "Excessive nesting depth detected",
			},
			Priority:   60,
			Confidence: 0.7,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	}
}

// CodeQualityRules returns default code quality rules.
func CodeQualityRules() []*intelligence.Rule {
	now := time.Now()

	return []*intelligence.Rule{
		{
			ID:          "CODE-001",
			Domain:      "code_quality",
			Name:        "TODO Comment",
			Description: "Detect TODO comments in code",
			Condition: intelligence.Condition{
				Operator: "CONTAINS",
				Field:    "code",
				Value:    "TODO",
			},
			Action: intelligence.Action{
				Type:     "log",
				Severity: "info",
				Message:  "TODO comment found",
			},
			Priority:   30,
			Confidence: 0.9,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		{
			ID:          "CODE-002",
			Domain:      "code_quality",
			Name:        "FIXME Comment",
			Description: "Detect FIXME comments in code",
			Condition: intelligence.Condition{
				Operator: "CONTAINS",
				Field:    "code",
				Value:    "FIXME",
			},
			Action: intelligence.Action{
				Type:     "flag",
				Severity: "warning",
				Message:  "FIXME comment found - needs attention",
			},
			Priority:   40,
			Confidence: 0.9,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		{
			ID:          "CODE-003",
			Domain:      "code_quality",
			Name:        "Duplicate Code",
			Description: "Detect duplicated code blocks",
			Condition: intelligence.Condition{
				Operator: "AND",
				Children: []intelligence.Condition{
					{
						Operator: "CONTAINS",
						Field:    "code",
						Value:    "func ",
					},
					{
						Operator: "CONTAINS",
						Field:    "code",
						Value:    "func ",
					},
				},
			},
			Action: intelligence.Action{
				Type:     "suggest",
				Severity: "info",
				Message:  "Possible duplicate code detected",
			},
			Priority:   50,
			Confidence: 0.6,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	}
}

// PerformanceRules returns default performance rules.
func PerformanceRules() []*intelligence.Rule {
	now := time.Now()

	return []*intelligence.Rule{
		{
			ID:          "PERF-001",
			Domain:      "performance",
			Name:        "N+1 Query",
			Description: "Detect N+1 query pattern",
			Condition: intelligence.Condition{
				Operator: "AND",
				Children: []intelligence.Condition{
					{
						Operator: "CONTAINS",
						Field:    "code",
						Value:    "for ",
					},
					{
						Operator: "CONTAINS",
						Field:    "code",
						Value:    "query",
					},
				},
			},
			Action: intelligence.Action{
				Type:     "flag",
				Severity: "warning",
				Message:  "Potential N+1 query pattern detected",
			},
			Priority:   80,
			Confidence: 0.7,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		{
			ID:          "PERF-002",
			Domain:      "performance",
			Name:        "Large Allocation",
			Description: "Detect large memory allocations",
			Condition: intelligence.Condition{
				Operator: "CONTAINS",
				Field:    "code",
				Value:    "make([]",
			},
			Action: intelligence.Action{
				Type:     "suggest",
				Severity: "info",
				Message:  "Large slice allocation - consider pre-sizing",
			},
			Priority:   55,
			Confidence: 0.6,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	}
}

// TestingRules returns default testing rules.
func TestingRules() []*intelligence.Rule {
	now := time.Now()

	return []*intelligence.Rule{
		{
			ID:          "TEST-001",
			Domain:      "testing",
			Name:        "Missing Test",
			Description: "Detect files without corresponding tests",
			Condition: intelligence.Condition{
				Operator: "AND",
				Children: []intelligence.Condition{
					{
						Operator: "EXISTS",
						Field:    "file_path",
					},
					{
						Operator: "NOT",
						Children: []intelligence.Condition{
							{
								Operator: "CONTAINS",
								Field:    "file_path",
								Value:    "_test",
							},
						},
					},
				},
			},
			Action: intelligence.Action{
				Type:     "suggest",
				Severity: "info",
				Message:  "File has no corresponding test file",
			},
			Priority:   45,
			Confidence: 0.7,
			Version:    1,
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	}
}

// AllDefaultRules returns all default rules.
func AllDefaultRules() []*intelligence.Rule {
	var all []*intelligence.Rule
	all = append(all, SecurityRules()...)
	all = append(all, ArchitectureRules()...)
	all = append(all, CodeQualityRules()...)
	all = append(all, PerformanceRules()...)
	all = append(all, TestingRules()...)
	return all
}

// ============================================================
// REPORT
// ============================================================

// Report generates a rule evaluation report.
func Report(results []*intelligence.RuleResult) string {
	if len(results) == 0 {
		return "No rules matched.\n"
	}

	var sb strings.Builder
	sb.WriteString("=== Rule Evaluation Report ===\n\n")

	for _, result := range results {
		icon := "ℹ"
		switch result.Severity {
		case "critical":
			icon = "✗"
		case "warning":
			icon = "!"
		case "info":
			icon = "ℹ"
		}

		sb.WriteString(fmt.Sprintf("[%s] %s (%s)\n", icon, result.RuleName, result.Domain))
		sb.WriteString(fmt.Sprintf("  Message: %s\n", result.Message))
		sb.WriteString(fmt.Sprintf("  Confidence: %.0f%%\n", result.Confidence*100))
		sb.WriteString("\n")
	}

	return sb.String()
}
