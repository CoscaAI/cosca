// Package intelligence implements the Cosca Intelligence Engine.
// This is the core of the system that learns from knowledge and
// makes deterministic decisions WITHOUT external LLM providers.
// Created: 2026-09-08 | ADR-046/047
package intelligence

import (
	"time"
)

// ============================================================
// CORE TYPES
// ============================================================

// Rule represents a deterministic rule.
type Rule struct {
	ID          string    `json:"id"`
	Domain      string    `json:"domain"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Condition   Condition `json:"condition"`
	Action      Action    `json:"action"`
	Priority    int       `json:"priority"`
	Confidence  float64   `json:"confidence"`
	Version     int       `json:"version"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Condition represents a logical condition tree.
type Condition struct {
	Operator string      `json:"operator"` // AND, OR, NOT, EXISTS, EQUALS, CONTAINS, GT, LT, MATCHES
	Field    string      `json:"field,omitempty"`
	Value    interface{} `json:"value,omitempty"`
	Children []Condition `json:"children,omitempty"`
}

// Action represents the action to take when a rule matches.
type Action struct {
	Type     string                 `json:"type"` // flag, suggest, fix, block, log, route
	Severity string                 `json:"severity"` // info, warning, critical
	Message  string                 `json:"message"`
	Params   map[string]interface{} `json:"params,omitempty"`
}

// RuleResult represents the result of rule evaluation.
type RuleResult struct {
	RuleID      string    `json:"rule_id"`
	RuleName    string    `json:"rule_name"`
	Domain      string    `json:"domain"`
	Matched     bool      `json:"matched"`
	Confidence  float64   `json:"confidence"`
	Severity    string    `json:"severity"`
	Message     string    `json:"message"`
	Action      Action    `json:"action"`
	EvaluatedAt time.Time `json:"evaluated_at"`
}

// Context represents the input data for rule evaluation.
type Context struct {
	// Code analysis
	Language    string                 `json:"language,omitempty"`
	FilePath    string                 `json:"file_path,omitempty"`
	Code        string                 `json:"code,omitempty"`
	AST         interface{}            `json:"ast,omitempty"`
	Metrics     map[string]float64     `json:"metrics,omitempty"`
	Patterns    []string               `json:"patterns,omitempty"` // AST-detected patterns (sql_injection, hardcoded_secret, ...)

	// Knowledge
	Knowledge   map[string]interface{} `json:"knowledge,omitempty"`

	// Task
	Task        map[string]interface{} `json:"task,omitempty"`

	// System
	System      map[string]interface{} `json:"system,omitempty"`

	// Custom fields
	Fields      map[string]interface{} `json:"fields,omitempty"`
}

// ============================================================
// ENGINE
// ============================================================

// Engine is the intelligence engine.
type Engine struct {
	rules       []*Rule
	expertSystems map[string]*ExpertSystem
	decisionTrees map[string]*DecisionTree
}

// NewEngine creates a new intelligence engine.
func NewEngine() *Engine {
	return &Engine{
		rules:         make([]*Rule, 0),
		expertSystems: make(map[string]*ExpertSystem),
		decisionTrees: make(map[string]*DecisionTree),
	}
}

// AddRule adds a rule to the engine.
func (e *Engine) AddRule(rule *Rule) {
	e.rules = append(e.rules, rule)
}

// AddRules adds multiple rules.
func (e *Engine) AddRules(rules []*Rule) {
	e.rules = append(e.rules, rules...)
}

// Rules returns all rules.
func (e *Engine) Rules() []*Rule {
	return e.rules
}

// SetExpertSystem registers an expert system.
func (e *Engine) SetExpertSystem(es *ExpertSystem) {
	e.expertSystems[es.Domain] = es
}

// SetDecisionTree registers a decision tree.
func (e *Engine) SetDecisionTree(dt *DecisionTree) {
	e.decisionTrees[dt.ID] = dt
}

// ============================================================
// RULE EVALUATION
// ============================================================

// Evaluate evaluates all rules against the context.
// Returns matched rules sorted by priority.
func (e *Engine) Evaluate(ctx *Context) []*RuleResult {
	var results []*RuleResult

	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}

		matched := EvaluateCondition(&rule.Condition, ctx)
		if matched {
			results = append(results, &RuleResult{
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

	// Sort by priority (higher first)
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Confidence > results[i].Confidence {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results
}

// EvaluateDomain evaluates rules for a specific domain.
func (e *Engine) EvaluateDomain(ctx *Context, domain string) []*RuleResult {
	var results []*RuleResult

	for _, rule := range e.rules {
		if !rule.Enabled || rule.Domain != domain {
			continue
		}

		if EvaluateCondition(&rule.Condition, ctx) {
			results = append(results, &RuleResult{
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
// CONDITION EVALUATION
// ============================================================

// EvaluateCondition evaluates a condition against context.
func EvaluateCondition(cond *Condition, ctx *Context) bool {
	if cond == nil {
		return true
	}

	switch cond.Operator {
	case "AND":
		for _, child := range cond.Children {
			if !EvaluateCondition(&child, ctx) {
				return false
			}
		}
		return true

	case "OR":
		for _, child := range cond.Children {
			if EvaluateCondition(&child, ctx) {
				return true
			}
		}
		return false

	case "NOT":
		if len(cond.Children) == 0 {
			return false
		}
		return !EvaluateCondition(&cond.Children[0], ctx)

	case "EXISTS":
		val := getField(ctx, cond.Field)
		return val != nil

	case "NOT_EXISTS":
		val := getField(ctx, cond.Field)
		return val == nil

	case "EQUALS":
		val := getField(ctx, cond.Field)
		return val != nil && val == cond.Value

	case "CONTAINS":
		val := getField(ctx, cond.Field)
		if str, ok := val.(string); ok {
			if search, ok := cond.Value.(string); ok {
				return containsFold(str, search)
			}
		}
		return false

	case "MATCHES":
		val := getField(ctx, cond.Field)
		if str, ok := val.(string); ok {
			if pattern, ok := cond.Value.(string); ok {
				return regexMatch(str, pattern)
			}
		}
		return false

	case "HAS_PATTERN":
		// Check if context has a specific AST-detected pattern
		if ctx == nil {
			return false
		}
		pattern, ok := cond.Value.(string)
		if !ok {
			return false
		}
		for _, p := range ctx.Patterns {
			if p == pattern {
				return true
			}
		}
		return false
	case "GT":
		val := getField(ctx, cond.Field)
		if num, ok := toFloat(val); ok {
			if threshold, ok := toFloat(cond.Value); ok {
				return num > threshold
			}
		}
		return false

	case "GTE":
		val := getField(ctx, cond.Field)
		if num, ok := toFloat(val); ok {
			if threshold, ok := toFloat(cond.Value); ok {
				return num >= threshold
			}
		}
		return false

	case "LT":
		val := getField(ctx, cond.Field)
		if num, ok := toFloat(val); ok {
			if threshold, ok := toFloat(cond.Value); ok {
				return num < threshold
			}
		}
		return false

	case "LTE":
		val := getField(ctx, cond.Field)
		if num, ok := toFloat(val); ok {
			if threshold, ok := toFloat(cond.Value); ok {
				return num <= threshold
			}
		}
		return false

	default:
		return false
	}
}

// ============================================================
// FIELD ACCESS
// ============================================================

// getField retrieves a field from context using dot notation.
func getField(ctx *Context, field string) interface{} {
	if field == "" {
		return nil
	}

	// Try direct field access first
	switch field {
	case "language":
		return ctx.Language
	case "file_path":
		return ctx.FilePath
	case "code":
		return ctx.Code
	}

	// Strip map name prefixes and look up in the corresponding map
	// e.g. "fields.task.type" → look "task.type" in ctx.Fields
	// e.g. "metrics.lines" → look "lines" in ctx.Metrics
	type prefixEntry struct {
		prefix string
		m      map[string]interface{}
		f      map[string]float64
	}

	lookups := []prefixEntry{
		{"fields.", ctx.Fields, nil},
		{"task.", ctx.Task, nil},
		{"system.", ctx.System, nil},
		{"knowledge.", ctx.Knowledge, nil},
		{"metrics.", nil, ctx.Metrics},
	}

	for _, lookup := range lookups {
		if len(field) > len(lookup.prefix) && field[:len(lookup.prefix)] == lookup.prefix {
			subPath := field[len(lookup.prefix):]
			if lookup.m != nil {
				if val, ok := getNested(lookup.m, subPath); ok {
					return val
				}
			}
			if lookup.f != nil {
				if val, ok := getNestedFloat(lookup.f, subPath); ok {
					return val
				}
			}
		}
	}

	// Fallback: try each map with the full path (no prefix stripping)
	if val, ok := getNested(ctx.Fields, field); ok {
		return val
	}
	if val, ok := getNestedFloat(ctx.Metrics, field); ok {
		return val
	}
	if val, ok := getNested(ctx.Knowledge, field); ok {
		return val
	}
	if val, ok := getNested(ctx.Task, field); ok {
		return val
	}
	if val, ok := getNested(ctx.System, field); ok {
		return val
	}

	return nil
}

// getNested retrieves a nested value from a map.
func getNested(m map[string]interface{}, path string) (interface{}, bool) {
	if m == nil {
		return nil, false
	}

	// Simple key lookup first
	if val, ok := m[path]; ok {
		return val, true
	}

	// Try dot notation
	parts := splitPath(path)
	if len(parts) == 0 {
		return nil, false
	}

	var current interface{} = m

	// If first part is a key in the map, start from there
	if first, ok := m[parts[0]]; ok {
		current = first
		parts = parts[1:]
		if len(parts) == 0 {
			return current, true
		}
	}

	for _, part := range parts {
		nextMap, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}

		current, ok = nextMap[part]
		if !ok {
			return nil, false
		}
	}

	return current, true
}

// getNestedFloat retrieves a nested float value with dot notation support.
func getNestedFloat(m map[string]float64, path string) (float64, bool) {
	if m == nil {
		return 0, false
	}

	// Simple key lookup first
	if val, ok := m[path]; ok {
		return val, true
	}

	// Try dot notation - get last part
	parts := splitPath(path)
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		if val, ok := m[last]; ok {
			return val, true
		}
	}

	return 0, false
}

// ============================================================
// HELPERS
// ============================================================

func contains(str, sub string) bool {
	return len(str) >= len(sub) && (len(sub) == 0 || indexOf(str, sub) >= 0)
}

func containsFold(str, sub string) bool {
	lowerStr := toLower(str)
	lowerSub := toLower(sub)
	return contains(lowerStr, lowerSub)
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func indexOf(str, sub string) int {
	for i := 0; i+len(sub) <= len(str); i++ {
		if str[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func regexMatch(str, pattern string) bool {
	// Simple pattern matching (substring or wildcard)
	if len(pattern) >= 2 && pattern[0] == '*' && pattern[len(pattern)-1] == '*' {
		mid := pattern[1 : len(pattern)-1]
		return contains(str, mid)
	}
	if len(pattern) >= 1 && pattern[0] == '*' {
		return len(str) >= len(pattern)-1 && str[len(str)-(len(pattern)-1):] == pattern[1:]
	}
	if len(pattern) >= 1 && pattern[len(pattern)-1] == '*' {
		return len(str) >= len(pattern)-1 && str[:len(pattern)-1] == pattern[:len(pattern)-1]
	}
	return str == pattern
}

func toFloat(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		var f float64
		if err := fmtSscan(val, &f); err == nil {
			return f, true
		}
	}
	return 0, false
}

func splitPath(path string) []string {
	var parts []string
	current := ""
	for _, c := range path {
		if c == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// ============================================================
// EXPERT SYSTEM
// ============================================================

// ExpertSystem represents a domain-specific expert system.
type ExpertSystem struct {
	ID          string    `json:"id"`
	Domain      string    `json:"domain"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	RuleIDs     []string  `json:"rule_ids"`
	Version     int       `json:"version"`
	Accuracy    float64   `json:"accuracy"`
	LastTrained time.Time `json:"last_trained"`
	CreatedAt   time.Time `json:"created_at"`
}

// ============================================================
// DECISION TREE
// ============================================================

// DecisionTree represents a decision tree.
type DecisionTree struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Domain      string    `json:"domain"`
	Root        *TreeNode `json:"root"`
	Accuracy    float64   `json:"accuracy"`
	LastTrained time.Time `json:"last_trained"`
	CreatedAt   time.Time `json:"created_at"`
}

// TreeNode represents a node in a decision tree.
type TreeNode struct {
	Field     string      `json:"field,omitempty"`
	Operator  string      `json:"operator,omitempty"`
	Threshold interface{} `json:"threshold,omitempty"`
	Children  []*TreeNode `json:"children,omitempty"`
	Outcome   *Outcome    `json:"outcome,omitempty"`
}

// Outcome represents the result of a decision tree path.
type Outcome struct {
	Decision   string                 `json:"decision"`
	Confidence float64                `json:"confidence"`
	Reason     string                 `json:"reason"`
	Params     map[string]interface{} `json:"params,omitempty"`
}

// Traverse traverses the decision tree with given context.
func (dt *DecisionTree) Traverse(ctx *Context) *Outcome {
	return traverseNode(dt.Root, ctx)
}

func traverseNode(node *TreeNode, ctx *Context) *Outcome {
	if node == nil {
		return nil
	}

	// If this is a leaf node, return outcome
	if node.Outcome != nil {
		return node.Outcome
	}

	// Evaluate condition
	val := getField(ctx, node.Field)
	matched := false

	switch node.Operator {
	case "EQUALS":
		matched = val == node.Threshold
	case "CONTAINS":
		if str, ok := val.(string); ok {
			if search, ok := node.Threshold.(string); ok {
				matched = contains(str, search)
			}
		}
	case "GT":
		if num, ok := toFloat(val); ok {
			if threshold, ok := toFloat(node.Threshold); ok {
				matched = num > threshold
			}
		}
	case "GTE":
		if num, ok := toFloat(val); ok {
			if threshold, ok := toFloat(node.Threshold); ok {
				matched = num >= threshold
			}
		}
	case "LT":
		if num, ok := toFloat(val); ok {
			if threshold, ok := toFloat(node.Threshold); ok {
				matched = num < threshold
			}
		}
	case "LTE":
		if num, ok := toFloat(val); ok {
			if threshold, ok := toFloat(node.Threshold); ok {
				matched = num <= threshold
			}
		}
	}

	// Traverse matching child
	if matched && len(node.Children) > 0 {
		return traverseNode(node.Children[0], ctx)
	}
	if !matched && len(node.Children) > 1 {
		return traverseNode(node.Children[1], ctx)
	}

	return nil
}
