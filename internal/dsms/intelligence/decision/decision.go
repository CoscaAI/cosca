// Package decision provides the decision tree engine.
// This routes tasks and makes decisions deterministically.
package decision

import (
	"fmt"
	"strings"
	"time"

	"cosca/internal/dsms/intelligence"
)

// ============================================================
// DECISION ENGINE
// ============================================================

// Engine is the decision engine.
type Engine struct {
	trees map[string]*intelligence.DecisionTree
}

// NewEngine creates a new decision engine.
func NewEngine() *Engine {
	return &Engine{
		trees: make(map[string]*intelligence.DecisionTree),
	}
}

// RegisterTree registers a decision tree.
func (e *Engine) RegisterTree(tree *intelligence.DecisionTree) {
	e.trees[tree.ID] = tree
}

// Decide makes a decision using a tree.
func (e *Engine) Decide(treeID string, ctx *intelligence.Context) (*intelligence.Outcome, error) {
	tree, ok := e.trees[treeID]
	if !ok {
		return nil, fmt.Errorf("decision tree not found: %s", treeID)
	}

	return tree.Traverse(ctx), nil
}

// ============================================================
// BUILT-IN DECISION TREES
// ============================================================

// TaskRoutingTree routes tasks to the appropriate agent.
func TaskRoutingTree() *intelligence.DecisionTree {
	now := time.Now()

	return &intelligence.DecisionTree{
		ID:     "task-routing",
		Name:   "Task Routing",
		Domain: "operations",
		Root: &intelligence.TreeNode{
			Field:    "task.type",
			Operator: "CONTAINS",
			Threshold: "security",
			Children: []*intelligence.TreeNode{
				// Security task
				{
					Outcome: &intelligence.Outcome{
						Decision:   "route_to_security",
						Confidence: 0.95,
						Reason:     "Security-related task detected",
						Params: map[string]interface{}{
							"agent": "cosca-security",
						},
					},
				},
				// Non-security task
				{
					Field:    "task.type",
					Operator: "CONTAINS",
					Threshold: "database",
					Children: []*intelligence.TreeNode{
						{
							Outcome: &intelligence.Outcome{
								Decision:   "route_to_database",
								Confidence: 0.95,
								Reason:     "Database-related task detected",
								Params: map[string]interface{}{
									"agent": "cosca-database",
								},
							},
						},
						{
							Field:    "task.type",
							Operator: "CONTAINS",
							Threshold: "frontend",
							Children: []*intelligence.TreeNode{
								{
									Outcome: &intelligence.Outcome{
										Decision:   "route_to_frontend",
										Confidence: 0.95,
										Reason:     "Frontend-related task detected",
										Params: map[string]interface{}{
											"agent": "cosca-frontend",
										},
									},
								},
								{
									Field:    "task.type",
									Operator: "CONTAINS",
									Threshold: "backend",
									Children: []*intelligence.TreeNode{
										{
											Outcome: &intelligence.Outcome{
												Decision:   "route_to_backend",
												Confidence: 0.95,
												Reason:     "Backend-related task detected",
												Params: map[string]interface{}{
													"agent": "cosca-backend",
												},
											},
										},
										{
											Outcome: &intelligence.Outcome{
												Decision:   "route_to_general",
												Confidence: 0.6,
												Reason:     "General task - no specific domain detected",
												Params: map[string]interface{}{
													"agent": "cosca-general",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		CreatedAt: now,
	}
}

// RiskAssessmentTree assesses risk of a change.
func RiskAssessmentTree() *intelligence.DecisionTree {
	now := time.Now()

	return &intelligence.DecisionTree{
		ID:     "risk-assessment",
		Name:   "Risk Assessment",
		Domain: "security",
		Root: &intelligence.TreeNode{
			Field:    "metrics.critical_functions",
			Operator: "GT",
			Threshold: 0.0,
			Children: []*intelligence.TreeNode{
				// Has critical functions
				{
					Field:    "metrics.test_coverage",
					Operator: "LT",
					Threshold: 0.8,
					Children: []*intelligence.TreeNode{
						{
							Outcome: &intelligence.Outcome{
								Decision:   "high_risk",
								Confidence: 0.9,
								Reason:     "Critical functions with low test coverage",
								Params: map[string]interface{}{
									"requires_review": true,
									"requires_tests":  true,
								},
							},
						},
						{
							Field:    "metrics.affected_files",
							Operator: "GT",
							Threshold: 10.0,
							Children: []*intelligence.TreeNode{
								{
									Outcome: &intelligence.Outcome{
										Decision:   "medium_risk",
										Confidence: 0.7,
										Reason:     "Many files affected",
										Params: map[string]interface{}{
											"requires_review": true,
										},
									},
								},
								{
									Outcome: &intelligence.Outcome{
										Decision:   "low_risk",
										Confidence: 0.8,
										Reason:     "Well-tested critical functions",
										Params: map[string]interface{}{
											"requires_review": false,
										},
									},
								},
							},
						},
					},
				},
				// No critical functions
				{
					Field:    "metrics.affected_files",
					Operator: "GT",
					Threshold: 20.0,
					Children: []*intelligence.TreeNode{
						{
							Outcome: &intelligence.Outcome{
								Decision:   "medium_risk",
								Confidence: 0.6,
								Reason:     "Large change with no critical functions",
								Params: map[string]interface{}{
									"requires_review": true,
								},
							},
						},
						{
							Outcome: &intelligence.Outcome{
								Decision:   "low_risk",
								Confidence: 0.85,
								Reason:     "Small change, no critical functions",
								Params: map[string]interface{}{
									"requires_review": false,
								},
							},
						},
					},
				},
			},
		},
		CreatedAt: now,
	}
}

// ============================================================
// REPORT
// ============================================================

// Report generates a decision report.
func Report(outcome *intelligence.Outcome) string {
	if outcome == nil {
		return "No decision made.\n"
	}

	var sb strings.Builder
	sb.WriteString("=== Decision Report ===\n\n")
	sb.WriteString(fmt.Sprintf("Decision: %s\n", outcome.Decision))
	sb.WriteString(fmt.Sprintf("Confidence: %.0f%%\n", outcome.Confidence*100))
	sb.WriteString(fmt.Sprintf("Reason: %s\n", outcome.Reason))
	sb.WriteString("\n")

	return sb.String()
}
