package pipeline

import (
	"fmt"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/orchestration"
)

// ─── AgentRoutingDecision ────────────────────────────────────────────────

type AgentRoutingDecision struct {
	TaskType      string    `json:"task_type"`
	TaskDesc      string    `json:"task_desc"`
	AgentSelected string    `json:"agent_selected"`
	Reason        string    `json:"reason"`
	Confidence    float64   `json:"confidence"`
	Timestamp     time.Time `json:"timestamp"`
}

// ─── AgentRouter ─────────────────────────────────────────────────────────

type AgentRouter struct {
	agents      orchestration.AgentResolver
	departments map[string][]string
	taskHistory []AgentRoutingDecision
}

// NewAgentRouter creates an AgentRouter backed by the given resolver.
func NewAgentRouter(agentResolver orchestration.AgentResolver) *AgentRouter {
	return &AgentRouter{
		agents:      agentResolver,
		departments: defaultDepartments(),
		taskHistory: make([]AgentRoutingDecision, 0),
	}
}

// Route selects the best agent for a task using:
//  1. Keyword matching on task description
//  2. Task-level known mappings
//  3. Agent capability matching via full-text search
//  4. Department-based routing
//  5. Fallback
func (r *AgentRouter) Route(task *TaskNode) (string, float64, string) {
	if task == nil {
		return "", 0, "no task provided"
	}

	desc := strings.ToLower(task.Description)
	taskType := DetectTaskType(desc)

	// 1. Exact agent hint from task
	if task.Agent != "" {
		agent, err := r.agents.Get(task.Agent)
		if err == nil && agent != nil {
			r.record(taskType, desc, agent.Name, "explicit task assignment", 1.0)
			return agent.Name, 1.0, "explicit task assignment"
		}
	}

	// 2. Keyword matching from task description
	if agentName, confidence, reason := r.keywordRoute(desc, taskType); agentName != "" {
		r.record(taskType, desc, agentName, reason, confidence)
		return agentName, confidence, reason
	}

	// 3. Agent capability matching via full-text search
	if agentName, confidence, reason := r.searchRoute(task); agentName != "" {
		r.record(taskType, desc, agentName, reason, confidence)
		return agentName, confidence, reason
	}

	// 4. Department-based routing
	if agentName, confidence, reason := r.departmentRoute(taskType); agentName != "" {
		r.record(taskType, desc, agentName, reason, confidence)
		return agentName, confidence, reason
	}

	// 5. Fallback
	r.record(taskType, desc, "", "no agent found", 0)
	return "", 0, "no agent found"
}

// SearchAndRoute uses the agent resolver's full-text search to find agents
// matching the query, returning the best match with a confidence score.
// This is a public entry point that bypasses keyword/department fallback
// for callers that want direct agent discovery.
func (r *AgentRouter) SearchAndRoute(query string) (*AgentRoutingDecision, error) {
	if r.agents == nil {
		return nil, fmt.Errorf("agent router: no agent resolver configured")
	}

	agents, err := r.agents.Search(query)
	if err != nil || len(agents) == 0 {
		return nil, fmt.Errorf("agent router: no agents found for query %q", query)
	}

	best := agents[0]
	confidence := 0.7
	if len(best.Description) > 0 {
		confidence = 0.75
	}
	if len(agents) > 1 {
		confidence -= 0.05
	}

	decision := &AgentRoutingDecision{
		TaskType:      DetectTaskType(strings.ToLower(query)),
		TaskDesc:      query,
		AgentSelected: best.Name,
		Reason:        fmt.Sprintf("search match: %s (%s)", best.Name, best.Role),
		Confidence:    confidence,
		Timestamp:     time.Now().UTC(),
	}

	r.taskHistory = append(r.taskHistory, *decision)
	return decision, nil
}

// ─── Keyword Routing ─────────────────────────────────────────────────────

var taskKeywordAgents = []struct {
	keywords   []string
	agents     []string
	confidence float64
}{
	{keywords: []string{"database", "sql", "schema", "migration", "query"}, agents: []string{"cosca-database", "Backend Chief"}, confidence: 0.9},
	{keywords: []string{"api", "endpoint", "rest", "graphql", "handler"}, agents: []string{"cosca-backend", "Backend API Specialist"}, confidence: 0.85},
	{keywords: []string{"test", "coverage", "unit", "integration", "e2e"}, agents: []string{"cosca-testing", "Unit Test Specialist"}, confidence: 0.85},
	{keywords: []string{"security", "auth", "vulnerability", "encryption"}, agents: []string{"cosca-security", "Security Chief"}, confidence: 0.9},
	{keywords: []string{"frontend", "ui", "component", "react", "vue"}, agents: []string{"cosca-frontend", "Frontend Component Specialist"}, confidence: 0.85},
	{keywords: []string{"deploy", "ci/cd", "docker", "kubernetes", "build"}, agents: []string{"cosca-devops", "DevOps Chief"}, confidence: 0.85},
	{keywords: []string{"architecture", "design", "pattern", "refactor"}, agents: []string{"cosca-backend", "Architecture Chief"}, confidence: 0.8},
	{keywords: []string{"review", "audit", "quality"}, agents: []string{"cosca-critic", "Code Reviewer"}, confidence: 0.8},
	{keywords: []string{"documentation", "docs", "readme", "manual"}, agents: []string{"cosca-backend", "Documentation Chief"}, confidence: 0.75},
	{keywords: []string{"monitoring", "logs", "alert", "observability"}, agents: []string{"cosca-devops", "Monitoring Chief"}, confidence: 0.8},
}

func (r *AgentRouter) keywordRoute(desc, taskType string) (string, float64, string) {
	for _, entry := range taskKeywordAgents {
		for _, kw := range entry.keywords {
			if strings.Contains(desc, kw) {
				for _, candidate := range entry.agents {
					agent, err := r.agents.Get(candidate)
					if err == nil && agent != nil {
						return agent.Name, entry.confidence,
							fmt.Sprintf("keyword match: %q in task description", kw)
					}
				}
			}
		}
	}
	return "", 0, ""
}

// ─── Search Route ────────────────────────────────────────────────────────

func (r *AgentRouter) searchRoute(task *TaskNode) (string, float64, string) {
	if r.agents == nil {
		return "", 0, ""
	}

	agents, err := r.agents.Search(task.Description)
	if err != nil || len(agents) == 0 {
		return "", 0, ""
	}

	best := agents[0]
	return best.Name, 0.6, fmt.Sprintf("full-text search match: %s", best.Name)
}

// ─── Department Routing ──────────────────────────────────────────────────

func defaultDepartments() map[string][]string {
	return map[string][]string{
		"design-schema":     {"cosca-database", "SQL Database Specialist"},
		"create-models":     {"cosca-backend", "Backend API Specialist"},
		"create-handlers":   {"cosca-backend", "Backend API Specialist"},
		"create-tests":      {"cosca-testing", "Unit Test Specialist"},
		"build-verify":      {"cosca-backend", "DevOps Chief"},
		"design-auth":       {"cosca-security", "Security Chief"},
		"diagnose":          {"cosca-backend", "Backend API Specialist"},
		"locate-code":       {"cosca-backend", "Backend API Specialist"},
		"implement-fix":     {"cosca-backend", "Backend API Specialist"},
		"add-tests":         {"cosca-testing", "Unit Test Specialist"},
		"verify":            {"cosca-backend", "Testing Chief"},
		"analyze-code":      {"cosca-backend", "Architecture Chief"},
		"identify-smells":   {"cosca-critic", "Code Reviewer"},
		"plan-refactor":     {"cosca-backend", "Architecture Chief"},
		"execute-refactor":  {"cosca-backend", "Backend API Specialist"},
		"run-tests":         {"cosca-testing", "Testing Chief"},
		"analyze-coverage":  {"cosca-testing", "Unit Test Specialist"},
		"identify-gaps":     {"cosca-testing", "Unit Test Specialist"},
		"write-tests":       {"cosca-testing", "Unit Test Specialist"},
		"run-verify":        {"cosca-testing", "Testing Chief"},
		"deploy":            {"cosca-devops", "DevOps Chief"},
		"monitor":           {"cosca-devops", "Monitoring Chief"},
		"document":          {"cosca-backend", "Documentation Chief"},
		"review-code":       {"cosca-critic", "Code Reviewer"},
		"audit-security":    {"cosca-security", "Security Chief"},
		"setup-cicd":        {"cosca-devops", "DevOps Chief"},
		"design-ui":         {"cosca-frontend", "Frontend Component Specialist"},
		"build-frontend":    {"cosca-frontend", "Frontend Component Specialist"},
		"integrate-api":     {"cosca-backend", "Backend API Specialist"},
		"containerize":      {"cosca-devops", "DevOps Chief"},
		"migrate-data":      {"cosca-database", "SQL Database Specialist"},
		"optimize-query":    {"cosca-database", "SQL Database Specialist"},
		"generate-docs":     {"cosca-backend", "Documentation Chief"},
		"setup-monitoring":  {"cosca-devops", "Monitoring Chief"},
	}
}

func (r *AgentRouter) departmentRoute(taskType string) (string, float64, string) {
	candidates, ok := r.departments[taskType]
	if !ok {
		return "", 0, ""
	}

	for _, candidate := range candidates {
		agent, err := r.agents.Get(candidate)
		if err == nil && agent != nil {
			return agent.Name, 0.7, fmt.Sprintf("department route: task type %q → %s", taskType, agent.Name)
		}
	}
	return "", 0, ""
}

// ─── Task Type Detection ─────────────────────────────────────────────────

func DetectTaskType(desc string) string {
	// Normaliza para lowercase: os callers (workflowToPlan, AgentRouter,
	// SearchAndRoute) passam o texto do workflow com maiúscula ("Implement
	// backend changes"), e as keywords são minúsculas. Sem o ToLower, o
	// "Implement backend changes" não casava "implement" — a causa raiz do
	// determinístico interceptar tasks de AÇÃO (o "pedreiro não constrói").
	desc = strings.ToLower(desc)
	if strings.Contains(desc, "schema") || strings.Contains(desc, "database") || strings.Contains(desc, "sql") || strings.Contains(desc, "query") {
		return "design-schema"
	}
	if strings.Contains(desc, "migration") || strings.Contains(desc, "migrate") {
		return "migrate-data"
	}
	if strings.Contains(desc, "model") || strings.Contains(desc, "struct") {
		return "create-models"
	}
	if strings.Contains(desc, "document") || strings.Contains(desc, "readme") || strings.Contains(desc, "manual") {
		return "document"
	}
	if strings.Contains(desc, "handler") || strings.Contains(desc, "api") || strings.Contains(desc, "endpoint") || strings.Contains(desc, "rest") || strings.Contains(desc, "graphql") {
		return "create-handlers"
	}
	if strings.Contains(desc, "integration") {
		return "integrate-api"
	}
	if strings.Contains(desc, "test") || strings.Contains(desc, "coverage") {
		return "create-tests"
	}
	if strings.Contains(desc, "build") || strings.Contains(desc, "compile") {
		return "build-verify"
	}
	// `audit` ANTES de `auth/security`: "Security audit" / "Auditar" é
	// AVALIAÇÃO (não-ação, audit-security), não auth (ação, design-auth).
	// A ordem contrária faria "Security audit of all changes" cair em
	// design-auth (uma ACTION), o que é semanticamente errado.
	if strings.Contains(desc, "audit") {
		return "audit-security"
	}
	if strings.Contains(desc, "auth") || strings.Contains(desc, "security") || strings.Contains(desc, "login") {
		return "design-auth"
	}
	if strings.Contains(desc, "diagnose") || strings.Contains(desc, "debug") || strings.Contains(desc, "error") {
		return "diagnose"
	}
	if strings.Contains(desc, "fix") || strings.Contains(desc, "repair") {
		return "implement-fix"
	}
	if strings.Contains(desc, "refactor") || strings.Contains(desc, "restructure") {
		return "execute-refactor"
	}
	if strings.Contains(desc, "analyze") || strings.Contains(desc, "analysis") {
		return "analyze-code"
	}
	if strings.Contains(desc, "review") || strings.Contains(desc, "critic") {
		return "review-code"
	}
	if strings.Contains(desc, "design") || strings.Contains(desc, "architecture") {
		return "plan-refactor"
	}
	if strings.Contains(desc, "deploy") || strings.Contains(desc, "release") || strings.Contains(desc, "cicd") || strings.Contains(desc, "pipeline") {
		return "deploy"
	}
	if strings.Contains(desc, "container") || strings.Contains(desc, "docker") || strings.Contains(desc, "kubernetes") {
		return "containerize"
	}
	if strings.Contains(desc, "monitor") || strings.Contains(desc, "log") || strings.Contains(desc, "alert") || strings.Contains(desc, "observability") {
		return "monitor"
	}
	if strings.Contains(desc, "frontend") || strings.Contains(desc, "ui") || strings.Contains(desc, "component") || strings.Contains(desc, "react") || strings.Contains(desc, "vue") {
		return "design-ui"
	}
	// ── Keywords dos workflows de featura/bugfix (texto REAL da esteira) ──
	// Sem estas, "Implement backend changes" (workflow feature-development),
	// "Implement the fix", "Merge feature branch to main" e "Investigate the
	// root cause" retornavam TaskType="", então IsActionIntentType=false e o
	// deterministicResponse interceptava a task de AÇÃO ANTES do LLM (a causa
	// raiz do "pedreiro não constrói"). "implement" cobre os tasks de
	// implementação; "merge" cobre o handoff de branch; "root cause"
	// é investigação (não ação — fica false). Cobrem os textos exatos dos
	// workflows, garantindo que a intenção de AÇÃO chegue ao executor.
	if strings.Contains(desc, "implement") || strings.Contains(desc, "implementation") {
		return "implement"
	}
	if strings.Contains(desc, "merge") || strings.Contains(desc, "handoff") {
		return "merge"
	}
	return ""
}

// IsActionIntentType reporta se um task de intenção (detectTaskType) é de
// AÇÃO/IMPLEMENTAÇÃO — deve ir ao LLM com tools (write/edit/shell/build/test)
// e NÃO ser resolvido pelo knowledge determinístico. Tipos de consulta/
// avaliação (analyze/review/diagnose/audit/document/monitor) retornam false e
// podem continuar no caminho determinístico. É o predicado usado pelo executor
// (decisão por DADO, não heurística espalhada).
func IsActionIntentType(taskType string) bool {
	switch taskType {
	case "design-schema", "migrate-data", "create-models", "create-handlers",
		"integrate-api", "create-tests", "build-verify", "design-auth",
		"implement-fix", "execute-refactor", "deploy", "containerize", "design-ui",
		// Novos tipos de AÇÃO dos workflows reais: "implement" (Backend/
		// Frontend/Fix Implementation) e "merge" (handoff de branch). Sem
		// eles, o determinístico interceptava tasks de implementação.
		"implement", "merge":
		return true
	default:
		return false
	}
}

// ─── History ─────────────────────────────────────────────────────────────

func (r *AgentRouter) record(taskType, taskDesc, agent, reason string, confidence float64) {
	r.taskHistory = append(r.taskHistory, AgentRoutingDecision{
		TaskType:      taskType,
		TaskDesc:      taskDesc,
		AgentSelected: agent,
		Reason:        reason,
		Confidence:    confidence,
		Timestamp:     time.Now().UTC(),
	})
}

func (r *AgentRouter) History() []AgentRoutingDecision {
	return r.taskHistory
}
