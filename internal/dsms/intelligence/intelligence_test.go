package intelligence

import (
	"testing"
	"time"
)

// TestConditionEvaluation tests condition evaluation.
func TestConditionEvaluation(t *testing.T) {
	ctx := &Context{
		Language: "go",
		FilePath: "/path/to/main.go",
		Code:     "func main() { db.Query(\"SELECT * FROM users WHERE id = \" + id) }",
		Metrics: map[string]float64{
			"lines":       600,
			"complexity":  15,
			"nesting":     6,
		},
		Fields: map[string]interface{}{
			"task": map[string]interface{}{
				"type": "security_review",
			},
		},
	}

	// Test EQUALS
	cond := &Condition{Operator: "EQUALS", Field: "language", Value: "go"}
	if !EvaluateCondition(cond, ctx) {
		t.Error("EQUALS should match")
	}

	// Test CONTAINS
	cond = &Condition{Operator: "CONTAINS", Field: "code", Value: "query"}
	if !EvaluateCondition(cond, ctx) {
		t.Error("CONTAINS should match")
	}

	// Test GT
	cond = &Condition{Operator: "GT", Field: "metrics.lines", Value: 500.0}
	if !EvaluateCondition(cond, ctx) {
		t.Error("GT should match")
	}

	// Test AND
	cond = &Condition{
		Operator: "AND",
		Children: []Condition{
			{Operator: "GT", Field: "metrics.lines", Value: 500.0},
			{Operator: "GT", Field: "metrics.complexity", Value: 10.0},
		},
	}
	if !EvaluateCondition(cond, ctx) {
		t.Error("AND should match")
	}

	// Test OR
	cond = &Condition{
		Operator: "OR",
		Children: []Condition{
			{Operator: "EQUALS", Field: "language", Value: "python"},
			{Operator: "EQUALS", Field: "language", Value: "go"},
		},
	}
	if !EvaluateCondition(cond, ctx) {
		t.Error("OR should match")
	}

	// Test NOT
	cond = &Condition{
		Operator: "NOT",
		Children: []Condition{
			{Operator: "EQUALS", Field: "language", Value: "python"},
		},
	}
	if !EvaluateCondition(cond, ctx) {
		t.Error("NOT should match")
	}

	// Test nested field access
	cond = &Condition{Operator: "CONTAINS", Field: "fields.task.type", Value: "security"}
	if !EvaluateCondition(cond, ctx) {
		t.Error("Nested field access should work")
	}
}

// TestRuleEngine tests the rule engine.
func TestRuleEngine(t *testing.T) {
	now := time.Now()

	engine := NewEngine()

	// Add a test rule
	engine.AddRule(&Rule{
		ID:          "TEST-001",
		Domain:      "test",
		Name:        "Test Rule",
		Description: "Test rule for engine",
		Condition: Condition{
			Operator: "CONTAINS",
			Field:    "code",
			Value:    "TODO",
		},
		Action: Action{
			Type:     "flag",
			Severity: "warning",
			Message:  "TODO found",
		},
		Priority:   50,
		Confidence: 0.8,
		Version:    1,
		Enabled:    true,
		CreatedAt:  now,
		UpdatedAt:  now,
	})

	// Test with matching context
	ctx := &Context{
		Code: "// TODO: fix this",
	}

	results := engine.Evaluate(ctx)
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if results[0].RuleID != "TEST-001" {
		t.Errorf("Expected TEST-001, got %s", results[0].RuleID)
	}

	// Test with non-matching context
	ctx2 := &Context{
		Code: "// All good here",
	}

	results = engine.Evaluate(ctx2)
	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

// TestDecisionTree tests the decision tree.
func TestDecisionTree(t *testing.T) {
	tree := &DecisionTree{
		ID:     "test-tree",
		Name:   "Test Tree",
		Domain: "test",
		Root: &TreeNode{
			Field:     "metrics.lines",
			Operator:  "GT",
			Threshold: 500.0,
			Children: []*TreeNode{
				{
					Outcome: &Outcome{
						Decision:   "large_file",
						Confidence: 0.9,
						Reason:     "File is large",
					},
				},
				{
					Outcome: &Outcome{
						Decision:   "normal_file",
						Confidence: 0.9,
						Reason:     "File is normal size",
					},
				},
			},
		},
	}

	// Test large file
	ctx := &Context{
		Metrics: map[string]float64{
			"lines": 600,
		},
	}

	outcome := tree.Traverse(ctx)
	if outcome == nil {
		t.Fatal("Outcome should not be nil")
	}

	if outcome.Decision != "large_file" {
		t.Errorf("Expected large_file, got %s", outcome.Decision)
	}

	// Test normal file
	ctx2 := &Context{
		Metrics: map[string]float64{
			"lines": 100,
		},
	}

	outcome = tree.Traverse(ctx2)
	if outcome == nil {
		t.Fatal("Outcome should not be nil")
	}

	if outcome.Decision != "normal_file" {
		t.Errorf("Expected normal_file, got %s", outcome.Decision)
	}
}