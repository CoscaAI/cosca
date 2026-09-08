package decision

import (
	"testing"

	"cosca/internal/dsms/intelligence"
)

// TestTaskRouting tests task routing.
func TestTaskRouting(t *testing.T) {
	engine := NewEngine()
	tree := TaskRoutingTree()
	engine.RegisterTree(tree)

	// Test security task
	ctx := &intelligence.Context{
		Fields: map[string]interface{}{
			"task": map[string]interface{}{
				"type": "security_review",
			},
		},
	}

	outcome, err := engine.Decide("task-routing", ctx)
	if err != nil {
		t.Fatalf("Failed to decide: %v", err)
	}

	if outcome == nil {
		t.Fatal("Outcome should not be nil")
	}

	if outcome.Decision != "route_to_security" {
		t.Errorf("Expected route_to_security, got %s", outcome.Decision)
	}

	// Test database task
	ctx2 := &intelligence.Context{
		Fields: map[string]interface{}{
			"task": map[string]interface{}{
				"type": "database_migration",
			},
		},
	}

	outcome, err = engine.Decide("task-routing", ctx2)
	if err != nil {
		t.Fatalf("Failed to decide: %v", err)
	}

	if outcome.Decision != "route_to_database" {
		t.Errorf("Expected route_to_database, got %s", outcome.Decision)
	}
}

// TestRiskAssessment tests risk assessment.
func TestRiskAssessment(t *testing.T) {
	engine := NewEngine()
	tree := RiskAssessmentTree()
	engine.RegisterTree(tree)

	// Test high risk
	ctx := &intelligence.Context{
		Metrics: map[string]float64{
			"critical_functions": 1,
			"test_coverage":      0.5,
			"affected_files":     5,
		},
	}

	outcome, err := engine.Decide("risk-assessment", ctx)
	if err != nil {
		t.Fatalf("Failed to decide: %v", err)
	}

	if outcome == nil {
		t.Fatal("Outcome should not be nil")
	}

	if outcome.Decision != "high_risk" {
		t.Errorf("Expected high_risk, got %s", outcome.Decision)
	}

	// Test low risk
	ctx2 := &intelligence.Context{
		Metrics: map[string]float64{
			"critical_functions": 0,
			"test_coverage":      0.9,
			"affected_files":     2,
		},
	}

	outcome, err = engine.Decide("risk-assessment", ctx2)
	if err != nil {
		t.Fatalf("Failed to decide: %v", err)
	}

	if outcome.Decision != "low_risk" {
		t.Errorf("Expected low_risk, got %s", outcome.Decision)
	}
}

// TestUnknownTree tests error handling.
func TestUnknownTree(t *testing.T) {
	engine := NewEngine()

	_, err := engine.Decide("unknown-tree", &intelligence.Context{})
	if err == nil {
		t.Error("Expected error for unknown tree")
	}
}