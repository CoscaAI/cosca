package pipeline

import (
	"context"
	"fmt"
	"strings"
)

// ── Permission Framework (Backstage Pattern #8) ─────────────────────────
//
// PermissionPolicy → PermissionEvaluator → PermissionRules with AND/OR/NOT.
// Substitutes binary JWT roles with conditional, context-aware authorization.

// PermissionAction defines what a user is trying to do.
type PermissionAction string

const (
	ActionPlanCreate   PermissionAction = "plan.create"
	ActionPlanExecute  PermissionAction = "plan.execute"
	ActionPlanApprove  PermissionAction = "plan.approve"
	ActionPlanRead     PermissionAction = "plan.read"
	ActionPluginManage PermissionAction = "plugin.manage"
)

// PermissionContext carries the evaluation context.
type PermissionContext struct {
	User      string
	Role      string
	Resource  string
	Action    PermissionAction
	Plan      *Plan
	AgentName string
	Metadata  map[string]interface{}
}

// PermissionRule is a single condition in a policy.
type PermissionRule struct {
	// Description explains what this rule does.
	Description string

	// Evaluate returns true if the rule allows the action.
	Evaluate func(ctx context.Context, pc *PermissionContext) bool
}

// PermissionPolicy evaluates a set of rules for a given action.
type PermissionPolicy struct {
	Name  string
	Rules []PermissionRule
}

// Evaluate runs all rules. Returns (allowed, reason).
// Rules are OR'd: any rule that passes allows the action.
// If no rule passes, the action is denied.
func (p *PermissionPolicy) Evaluate(ctx context.Context, pc *PermissionContext) (bool, string) {
	if len(p.Rules) == 0 {
		return false, fmt.Sprintf("policy %q has no rules", p.Name)
	}

	for i, rule := range p.Rules {
		if rule.Evaluate(ctx, pc) {
			return true, fmt.Sprintf("policy %q, rule %d: %s", p.Name, i, rule.Description)
		}
	}

	return false, fmt.Sprintf("policy %q: no matching rule for %s on %s", p.Name, pc.Action, pc.Resource)
}

// PermissionEvaluator evaluates multiple policies against a context.
type PermissionEvaluator struct {
	policies []*PermissionPolicy
}

// NewPermissionEvaluator creates an evaluator with the given policies.
func NewPermissionEvaluator(policies ...*PermissionPolicy) *PermissionEvaluator {
	return &PermissionEvaluator{policies: policies}
}

// AddPolicy appends a policy to the evaluator.
func (e *PermissionEvaluator) AddPolicy(p *PermissionPolicy) {
	e.policies = append(e.policies, p)
}

// Authorize checks all policies. Returns true if any policy allows the action.
// Policies are AND'd: ALL must pass.
func (e *PermissionEvaluator) Authorize(ctx context.Context, pc *PermissionContext) (bool, string) {
	if len(e.policies) == 0 {
		return true, "no policies — allow all"
	}

	for _, policy := range e.policies {
		allowed, reason := policy.Evaluate(ctx, pc)
		if !allowed {
			return false, reason
		}
	}

	return true, "all policies passed"
}

// ── Built-in Policies ────────────────────────────────────────────────────

// DefaultPermissionPolicy returns a role-based policy (backward compatible).
func DefaultPermissionPolicy() *PermissionPolicy {
	return &PermissionPolicy{
		Name: "default-rbac",
		Rules: []PermissionRule{
			{
				Description: "don can do anything",
				Evaluate: func(_ context.Context, pc *PermissionContext) bool {
					return pc.Role == "don" || pc.Role == "admin"
				},
			},
			{
				Description: "editor can execute plans",
				Evaluate: func(_ context.Context, pc *PermissionContext) bool {
					return pc.Role == "editor" && (pc.Action == ActionPlanExecute || pc.Action == ActionPlanRead)
				},
			},
			{
				Description: "viewer can only read",
				Evaluate: func(_ context.Context, pc *PermissionContext) bool {
					return pc.Role == "viewer" && pc.Action == ActionPlanRead
				},
			},
		},
	}
}

// AgentPermissionPolicy restricts agent execution based on confidence and domain.
func AgentPermissionPolicy() *PermissionPolicy {
	return &PermissionPolicy{
		Name: "agent-conditional",
		Rules: []PermissionRule{
			{
				Description: "allow if agent confidence >= 0.70",
				Evaluate: func(_ context.Context, pc *PermissionContext) bool {
					conf, ok := pc.Metadata["agent_confidence"].(float64)
					if !ok {
						return false
					}
					return conf >= 0.70
				},
			},
			{
				Description: "allow if action is read-only",
				Evaluate: func(_ context.Context, pc *PermissionContext) bool {
					return pc.Action == ActionPlanRead
				},
			},
		},
	}
}

// ResourcePermissionPolicy restricts access to specific resources.
func ResourcePermissionPolicy(allowedResources ...string) *PermissionPolicy {
	allowed := make(map[string]bool)
	for _, r := range allowedResources {
		allowed[strings.ToLower(r)] = true
	}

	return &PermissionPolicy{
		Name: "resource-access",
		Rules: []PermissionRule{
			{
				Description: fmt.Sprintf("allow resources: %v", allowedResources),
				Evaluate: func(_ context.Context, pc *PermissionContext) bool {
					return allowed[strings.ToLower(pc.Resource)]
				},
			},
		},
	}
}
