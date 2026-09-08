// Package oracle provides the Cosca Oracle — a deterministic knowledge
// consultation layer for orchestration. The Kernel and Chiefs ASK the oracle;
// it RESPONDS with evidence from the trained rules + knowledge.
//
// SECURITY CONTRACT (fail-closed):
//   - Oracle RESPONDS, never EXECUTES (read-only, no arbitrary commands)
//   - Never returns secret VALUES — only flags their existence + location
//   - Zero network access (Lei do Cofre — 100% local)
//   - Full audit trail of every consultation
//   - If unsure, says "unknown" — never invents
package oracle

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"cosca/internal/dsms/intelligence"
	"cosca/internal/dsms/intelligence/codeanalyzer"
	"cosca/internal/dsms/intelligence/decision"
	"cosca/internal/dsms/intelligence/expert"
	"cosca/internal/dsms/intelligence/rules"
	"cosca/internal/dsms/storage"
)

// ============================================================
// ORACLE
// ============================================================

// Oracle is the deterministic knowledge consultation layer.
type Oracle struct {
	engine     *rules.Engine
	expertReg  *expert.Registry
	decision   *decision.Engine
	rulesDir   string
	audit      []*AuditEntry
	mu         sync.RWMutex
	maxResults int
}

// NewOracle creates an oracle with rules loaded from JSONL.
func NewOracle(rulesDir string) (*Oracle, error) {
	o := &Oracle{
		engine:     rules.NewEngine(),
		expertReg:  expert.DefaultRegistry(),
		decision:   decision.NewEngine(),
		rulesDir:   rulesDir,
		audit:      make([]*AuditEntry, 0),
		maxResults: 20,
	}

	// Register decision trees
	o.decision.RegisterTree(decision.TaskRoutingTree())
	o.decision.RegisterTree(decision.RiskAssessmentTree())

	// Load trained rules
	if rulesDir != "" {
		st := storage.NewStorage(rulesDir)
		loaded, err := st.LoadRules()
		if err != nil {
			return nil, fmt.Errorf("load rules: %w", err)
		}
		for _, r := range loaded {
			o.engine.Register(r)
		}
	}

	return o, nil
}

// NewOracleWithRules creates an oracle with explicit rules (for tests).
func NewOracleWithRules(rulesList []*intelligence.Rule) *Oracle {
	o := &Oracle{
		engine:     rules.NewEngine(),
		expertReg:  expert.DefaultRegistry(),
		decision:   decision.NewEngine(),
		audit:      make([]*AuditEntry, 0),
		maxResults: 20,
	}
	o.decision.RegisterTree(decision.TaskRoutingTree())
	o.decision.RegisterTree(decision.RiskAssessmentTree())
	for _, r := range rulesList {
		o.engine.Register(r)
	}
	return o
}

// ============================================================
// AUDIT TRAIL
// ============================================================

// AuditEntry records a consultation.
type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Caller    string    `json:"caller"`
	Query     string    `json:"query"`
	Response  string    `json:"response"`
}

// audit records a consultation (thread-safe).
func (o *Oracle) auditEntry(caller, query, response string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.audit = append(o.audit, &AuditEntry{
		Timestamp: time.Now(),
		Caller:    caller,
		Query:     query,
		Response:  sanitizeForAudit(response),
	})
}

// AuditLog returns the audit trail.
func (o *Oracle) AuditLog() []*AuditEntry {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.audit
}

// ============================================================
// CORE TYPES
// ============================================================

// Answer is the oracle's response to a query.
type Answer struct {
	Query      string             `json:"query"`
	Confidence float64            `json:"confidence"`
	Evidence   []*intelligence.RuleResult `json:"evidence"`
	Decision   string             `json:"decision,omitempty"`
	Risk       string             `json:"risk,omitempty"`
	Reason     string             `json:"reason,omitempty"`
	AnsweredAt time.Time          `json:"answered_at"`
	Unknown    bool               `json:"unknown"`
}

// Classification is the result of classifying a task/order.
type Classification struct {
	Type       string  `json:"type"`
	Domain     string  `json:"domain"`
	Agent      string  `json:"agent"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

// PreFlight is the result of pre-delegation analysis.
type PreFlight struct {
	File        string  `json:"file"`
	HasIssues   bool    `json:"has_issues"`
	Critical    int     `json:"critical"`
	Warnings    int     `json:"warnings"`
	Info        int     `json:"info"`
	Risk        string  `json:"risk"`
	Findings    []*intelligence.RuleResult `json:"findings"`
}

// ============================================================
// PUBLIC API — what the Kernel/Chiefs call
// ============================================================

// Classify classifies a task/order into type + domain + target agent.
// This is the deterministic replacement for LLM-based routing.
// Supports Portuguese and English keywords (the Don speaks PT).
func (o *Oracle) Classify(caller, order string) (*Classification, error) {
	lower := strings.ToLower(order)

	// Direct keyword classification (PT + EN) — faster and more reliable
	// than the decision tree for free-text orders.
	cls := classifyByKeywords(lower)
	if cls != nil {
		o.auditEntry(caller, "classify:"+order, cls.Agent)
		return cls, nil
	}

	// Fallback: use decision tree
	ctx := &intelligence.Context{
		Task: map[string]interface{}{
			"type": order,
		},
		Fields: map[string]interface{}{
			"task": map[string]interface{}{
				"type": order,
			},
		},
	}

	outcome, err := o.decision.Decide("task-routing", ctx)
	if err != nil || outcome == nil {
		o.auditEntry(caller, "classify:"+order, "unknown")
		return &Classification{Type: "general", Domain: "general", Agent: "cosca-general", Confidence: 0.3}, nil
	}

	cls = &Classification{
		Type:       outcome.Decision,
		Confidence: outcome.Confidence,
		Reason:     outcome.Reason,
	}
	if agent, ok := outcome.Params["agent"].(string); ok {
		cls.Agent = agent
	}

	// Map agent to domain
	switch cls.Agent {
	case "cosca-security":
		cls.Domain = "security"
	case "cosca-database":
		cls.Domain = "database"
	case "cosca-frontend":
		cls.Domain = "frontend"
	case "cosca-backend":
		cls.Domain = "backend"
	default:
		cls.Domain = "general"
	}

	o.auditEntry(caller, "classify:"+order, cls.Agent)
	return cls, nil
}

// classifyByKeywords classifies free-text orders by keyword matching.
func classifyByKeywords(lower string) *Classification {
	type rule struct {
		keywords []string
		domain   string
		agent    string
		conf     float64
	}

	rules := []rule{
		{[]string{"seguranc", "security", "vulnerab", "injection", "sql", "cwe", "secreta", "secret", "token", "password", "auth", "login", "criptograf"}, "security", "cosca-security", 0.9},
		{[]string{"banco", "database", "schema", "migra", "sql", "query", "index", "tabela", "table", "postgres", "sqlite"}, "database", "cosca-database", 0.9},
		{[]string{"frontend", "interface", "ui", "componente", "tela", "react", "css", "layout"}, "frontend", "cosca-frontend", 0.9},
		{[]string{"backend", "api", "endpoint", "serviço", "servico", "rest", "grpc", "handler"}, "backend", "cosca-backend", 0.85},
		{[]string{"teste", "test", "qa", "cobertura", "coverage"}, "testing", "cosca-qa", 0.85},
		{[]string{"arquitetura", "architecture", "design", "padrão", "padrao", "estrutura"}, "architecture", "cosca-architecture", 0.85},
		{[]string{"performance", "desempenho", "rápido", "rapido", "lento", "benchmark", "otimiza", "n+1"}, "performance", "cosca-performance", 0.85},
		{[]string{"devops", "deploy", "ci/cd", "docker", "container", "infra", "kubernetes"}, "devops", "cosca-devops", 0.85},
	}

	best := &Classification{
		Type:   "general",
		Domain: "general",
		Agent:  "cosca-general",
		Reason: "no specific domain detected",
	}
	bestConf := 0.0
	matchedAny := false

	for _, r := range rules {
		for _, kw := range r.keywords {
			if strings.Contains(lower, kw) {
				if r.conf > bestConf {
					best.Type = r.domain + "_task"
					best.Domain = r.domain
					best.Agent = r.agent
					best.Confidence = r.conf
					best.Reason = "keyword match: " + kw
					bestConf = r.conf
					matchedAny = true
				}
				break
			}
		}
	}

	if !matchedAny {
		return nil
	}

	// Sanity: "sql" appears in both security and database — check order
	// If it matched database first but mentions security terms too, keep both signals.
	return best
}

// PreFlight analyzes code BEFORE delegation.
// Detects issues so the Kernel knows the terrain before spending LLM budget.
func (o *Oracle) PreFlight(caller, filePath string) (*PreFlight, error) {
	result := &PreFlight{
		File:     filePath,
		Findings: make([]*intelligence.RuleResult, 0),
	}

	// Read the file
	code, err := readFileSafe(filePath)
	if err != nil {
		o.auditEntry(caller, "preflight:"+filePath, "error:"+err.Error())
		return result, err
	}

	// Analyze with AST
	analyzer := codeanalyzer.NewAnalyzer()
	analysis, aErr := analyzer.AnalyzeGo(filePath, code)
	if aErr != nil {
		o.auditEntry(caller, "preflight:"+filePath, "parse-error")
		return result, aErr
	}

	// Build context with AST patterns
	ctx := &intelligence.Context{
		Language: analysis.Language,
		FilePath: analysis.FilePath,
		Code:     code,
		Metrics:  analysis.Metrics,
		Patterns: extractPatterns(analysis),
	}

	// Run expert systems
	for _, sys := range o.expertReg.All() {
		findings := sys.Analyze(ctx)
		result.Findings = append(result.Findings, findings...)
	}

	// Count by severity
	for _, f := range result.Findings {
		switch f.Severity {
		case "critical":
			result.Critical++
		case "warning":
			result.Warnings++
		default:
			result.Info++
		}
	}

	result.HasIssues = result.Critical > 0 || result.Warnings > 0

	// Assess risk
	riskCtx := &intelligence.Context{
		Metrics: map[string]float64{
			"critical_functions": float64(result.Critical),
			"affected_files":     1,
		},
	}
	if outcome, err := o.decision.Decide("risk-assessment", riskCtx); err == nil && outcome != nil {
		result.Risk = outcome.Decision
	}

	o.auditEntry(caller, "preflight:"+filePath, fmt.Sprintf("critical=%d warnings=%d", result.Critical, result.Warnings))
	return result, nil
}

// Ask answers a structured query using the knowledge base.
// Returns evidence — which rules matched and why.
func (o *Oracle) Ask(caller, query string, ctx *intelligence.Context) (*Answer, error) {
	// Fail-closed: validate context
	if ctx == nil {
		return nil, fmt.Errorf("context required (fail-closed)")
	}

	// Run rules engine
	results := o.engine.Evaluate(ctx)
	if len(results) == 0 {
		ans := &Answer{
			Query:      query,
			Confidence: 0,
			Unknown:    true,
			AnsweredAt: time.Now(),
		}
		o.auditEntry(caller, "ask:"+query, "unknown")
		return ans, nil
	}

	// Take top results
	if len(results) > o.maxResults {
		results = results[:o.maxResults]
	}

	ans := &Answer{
		Query:      query,
		Confidence: results[0].Confidence,
		Evidence:   results,
		AnsweredAt: time.Now(),
	}

	// Build a decision summary
	o.auditEntry(caller, "ask:"+query, fmt.Sprintf("%d findings, top=%s conf=%.2f", len(results), results[0].RuleName, results[0].Confidence))
	return ans, nil
}

// AskGuarded answers a query through the security guard (fail-closed).
// This is the SAFE entry point for external callers.
func (o *Oracle) AskGuarded(caller, query string, ctx *intelligence.Context) (*Answer, error) {
	// Security guard first
	guard := SecurityGuard(query)
	if !guard.Allowed {
		o.auditEntry(caller, "ask-guarded:"+query, "DENIED: "+guard.Reason)
		return &Answer{
			Query:      query,
			Unknown:    true,
			AnsweredAt: time.Now(),
		}, fmt.Errorf("query denied by security guard: %s", guard.Reason)
	}

	return o.Ask(caller, guard.Sanitized, ctx)
}

// Trust returns a trust assessment for an agent based on rule evidence.
// Higher trust = agent's code has fewer critical findings.
func (o *Oracle) Trust(caller, agent string) (float64, error) {
	// This would normally query historical scan data.
	// For now, return neutral trust (no data = no prejudice).
	// Future: aggregate findings by agent from the audit/scan history.
	o.auditEntry(caller, "trust:"+agent, "neutral-1.0")
	return 1.0, nil
}

// ============================================================
// HELPERS
// ============================================================

func extractPatterns(analysis *codeanalyzer.AnalysisResult) []string {
	if analysis == nil {
		return nil
	}
	var patterns []string
	for _, p := range analysis.Patterns {
		patterns = append(patterns, p.Pattern)
	}
	return patterns
}

func readFileSafe(path string) (string, error) {
	data, err := osReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// sanitizeForAudit removes any secret-like values from audit entries.
func sanitizeForAudit(s string) string {
	// Replace known secret patterns with [REDACTED]
	lower := strings.ToLower(s)
	for _, pattern := range []string{"sk-", "akia", "ghp_", "xoxb-", "eyj", "-----begin"} {
		idx := strings.Index(lower, pattern)
		if idx >= 0 {
			s = s[:idx] + "[REDACTED]"
			break
		}
	}
	return s
}

// osReadFile is an indirection for testability.
var osReadFile = func(path string) ([]byte, error) {
	return readFileImpl(path)
}
