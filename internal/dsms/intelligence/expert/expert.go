// Package expert provides domain-specific expert systems.
// Each expert system combines rules to make domain decisions.
package expert

import (
	"fmt"
	"strings"

	"cosca/internal/dsms/intelligence"
	"cosca/internal/dsms/intelligence/rules"
)

// ============================================================
// EXPERT SYSTEMS
// ============================================================

// System is a domain-specific expert system.
type System struct {
	Domain      string
	Name        string
	Description string
	Engine      *rules.Engine
	Thresholds  map[string]float64
}

// NewSystem creates a new expert system.
func NewSystem(domain, name, description string) *System {
	return &System{
		Domain:      domain,
		Name:        name,
		Description: description,
		Engine:      rules.NewEngine(),
		Thresholds:  make(map[string]float64),
	}
}

// RegisterRules registers rules for this expert system.
func (s *System) RegisterRules(rules []*intelligence.Rule) error {
	return s.Engine.RegisterMany(rules)
}

// Analyze analyzes context and returns findings.
func (s *System) Analyze(ctx *intelligence.Context) []*intelligence.RuleResult {
	return s.Engine.EvaluateDomain(ctx, s.Domain)
}

// SetThreshold sets a domain-specific threshold.
func (s *System) SetThreshold(name string, value float64) {
	s.Thresholds[name] = value
}

// ============================================================
// BUILT-IN EXPERT SYSTEMS
// ============================================================

// SecurityExpert creates the security expert system.
func SecurityExpert() *System {
	sys := NewSystem("security", "Security Expert", "Security analysis and vulnerability detection")
	sys.RegisterRules(rules.SecurityRules())

	sys.SetThreshold("sql_injection_score", 0.8)
	sys.SetThreshold("hardcoded_secret_score", 0.7)
	sys.SetThreshold("path_traversal_score", 0.7)

	return sys
}

// ArchitectureExpert creates the architecture expert system.
func ArchitectureExpert() *System {
	sys := NewSystem("architecture", "Architecture Expert", "Architecture analysis and design review")
	sys.RegisterRules(rules.ArchitectureRules())

	sys.SetThreshold("max_lines", 500)
	sys.SetThreshold("max_complexity", 10)
	sys.SetThreshold("max_nesting", 5)

	return sys
}

// CodeQualityExpert creates the code quality expert system.
func CodeQualityExpert() *System {
	sys := NewSystem("code_quality", "Code Quality Expert", "Code quality and best practices")
	sys.RegisterRules(rules.CodeQualityRules())

	return sys
}

// PerformanceExpert creates the performance expert system.
func PerformanceExpert() *System {
	sys := NewSystem("performance", "Performance Expert", "Performance analysis and optimization")
	sys.RegisterRules(rules.PerformanceRules())

	sys.SetThreshold("n1_query_score", 0.7)
	sys.SetThreshold("allocation_score", 0.6)

	return sys
}

// TestingExpert creates the testing expert system.
func TestingExpert() *System {
	sys := NewSystem("testing", "Testing Expert", "Test strategy and coverage analysis")
	sys.RegisterRules(rules.TestingRules())

	return sys
}

// ============================================================
// SYSTEM REGISTRY
// ============================================================

// Registry manages all expert systems.
type Registry struct {
	systems map[string]*System
}

// NewRegistry creates a new expert system registry.
func NewRegistry() *Registry {
	return &Registry{
		systems: make(map[string]*System),
	}
}

// Register registers an expert system.
func (r *Registry) Register(system *System) {
	r.systems[system.Domain] = system
}

// Get returns an expert system by domain.
func (r *Registry) Get(domain string) (*System, bool) {
	sys, ok := r.systems[domain]
	return sys, ok
}

// All returns all expert systems.
func (r *Registry) All() []*System {
	var all []*System
	for _, sys := range r.systems {
		all = append(all, sys)
	}
	return all
}

// DefaultRegistry returns a registry with all default expert systems.
func DefaultRegistry() *Registry {
	reg := NewRegistry()
	reg.Register(SecurityExpert())
	reg.Register(ArchitectureExpert())
	reg.Register(CodeQualityExpert())
	reg.Register(PerformanceExpert())
	reg.Register(TestingExpert())
	return reg
}

// ============================================================
// REPORT
// ============================================================

// Report generates an expert system report.
func Report(system *System, results []*intelligence.RuleResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== %s Report ===\n\n", system.Name))

	if len(results) == 0 {
		sb.WriteString("No issues found.\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("Found %d issue(s):\n\n", len(results)))

	for _, result := range results {
		icon := "ℹ"
		switch result.Severity {
		case "critical":
			icon = "✗"
		case "warning":
			icon = "!"
		}

		sb.WriteString(fmt.Sprintf("[%s] %s\n", icon, result.RuleName))
		sb.WriteString(fmt.Sprintf("  %s\n", result.Message))
		sb.WriteString(fmt.Sprintf("  Confidence: %.0f%%\n", result.Confidence*100))
		sb.WriteString("\n")
	}

	return sb.String()
}
