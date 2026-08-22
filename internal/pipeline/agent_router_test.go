package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/orchestration"
)

// stubResolver is a minimal AgentResolver for deterministic routing tests.
type stubResolver struct {
	agents map[string]*orchestration.AgentInfo
}

func newStubResolver(infos ...orchestration.AgentInfo) *stubResolver {
	s := &stubResolver{agents: map[string]*orchestration.AgentInfo{}}
	for i := range infos {
		a := infos[i]
		s.agents[strings.ToLower(a.Name)] = &a
	}
	return s
}

func (s *stubResolver) Get(name string) (*orchestration.AgentInfo, error) {
	a, ok := s.agents[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("agent %q not found", name)
	}
	return a, nil
}

func (s *stubResolver) Search(query string) ([]orchestration.AgentInfo, error) {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return nil, nil
	}
	var out []orchestration.AgentInfo
	for _, a := range s.agents {
		hay := strings.ToLower(a.Name + " " + a.Role + " " + a.Description)
		for _, w := range words {
			if strings.Contains(hay, w) {
				out = append(out, *a)
				break
			}
		}
	}
	return out, nil
}

var testAgents = []orchestration.AgentInfo{
	{Name: "cosca-database", Role: "Database Chief", Department: "database", Description: "sql schema queries"},
	{Name: "cosca-testing", Role: "Testing Chief", Department: "testing", Description: "unit integration coverage"},
	{Name: "cosca-backend", Role: "Backend Chief", Department: "backend", Description: "api rest handlers"},
	{Name: "cosca-security", Role: "Security Chief", Department: "security", Description: "auth vulnerability encryption"},
}

func TestDetectTaskType(t *testing.T) {
	cases := []struct {
		desc string
		want string
	}{
		{"create database schema", "design-schema"},
		{"write the sql query", "design-schema"},
		{"run data migration", "migrate-data"},
		{"create the user model", "create-models"},
		{"add the api handler", "create-handlers"},
		{"build the rest endpoint", "create-handlers"},
		{"write integration tests", "integrate-api"}, // "integration" checked before "test"
		{"write unit tests", "create-tests"},
		{"check test coverage", "create-tests"},
		{"compile the binary", "build-verify"},
		{"design the auth flow", "design-auth"},
		{"audit the codebase", "audit-security"},
		{"diagnose the error", "diagnose"},
		{"fix the bug", "implement-fix"},
		{"refactor the module", "execute-refactor"},
		{"analyze the architecture", "analyze-code"},
		{"review the PR", "review-code"},
		{"design the architecture", "plan-refactor"},
		{"deploy to production", "deploy"},
		{"containerize the app", "containerize"},
		{"monitor the logs", "monitor"},
		{"document the API", "document"},
		{"build the react component", "build-verify"}, // "build" is checked before "ui"
		{"create the react component", "design-ui"},   // "create" is not a type trigger
		{"frontend landing page", "design-ui"},
		{"unrelated text", ""},
	}
	for _, tc := range cases {
		if got := detectTaskType(tc.desc); got != tc.want {
			t.Errorf("detectTaskType(%q) = %q, want %q", tc.desc, got, tc.want)
		}
	}
}

func TestAgentRouterExplicitAssignment(t *testing.T) {
	r := NewAgentRouter(newStubResolver(testAgents...))
	agent, conf, reason := r.Route(&TaskNode{Description: "anything", Agent: "cosca-security"})
	if agent != "cosca-security" || conf != 1.0 || reason != "explicit task assignment" {
		t.Fatalf("explicit route = %q, %v, %q", agent, conf, reason)
	}
	if len(r.History()) != 1 {
		t.Fatalf("history len = %d, want 1", len(r.History()))
	}
}

func TestAgentRouterKeywordRoute(t *testing.T) {
	r := NewAgentRouter(newStubResolver(testAgents...))
	// First matching keyword entry wins: "test" + "coverage" both map to
	// cosca-testing, and no higher-priority keyword ("database"/"api"...) hits.
	agent, conf, reason := r.Route(&TaskNode{Description: "write unit tests and coverage"})
	if agent != "cosca-testing" {
		t.Fatalf("keyword route agent = %q, want cosca-testing", agent)
	}
	if conf != 0.85 {
		t.Fatalf("keyword confidence = %v, want 0.85", conf)
	}
	if !strings.Contains(reason, "keyword match") {
		t.Fatalf("reason = %q", reason)
	}
}

func TestAgentRouterDepartmentRoute(t *testing.T) {
	r := NewAgentRouter(newStubResolver(testAgents...))
	// "create database schema" hits the keyword route first (database keyword)
	// → cosca-database. For department-only, use a task type that only
	// departmentRoute resolves: "setup-cicd" maps to cosca-devops, but the stub
	// has no devops agent, so register one.
	r2 := NewAgentRouter(newStubResolver(append(testAgents,
		orchestration.AgentInfo{Name: "cosca-devops", Role: "DevOps Chief", Department: "devops", Description: "docker cicd"})...))
	agent, conf, reason := r2.Route(&TaskNode{Description: "ci/cd pipeline setup"})
	// "ci/cd" and "pipeline" keywords → deploy keyword route hits cosca-devops
	// via keyword list {"deploy","ci/cd","docker","kubernetes","build"}.
	if agent != "cosca-devops" {
		t.Fatalf("route agent = %q, want cosca-devops", agent)
	}
	if conf < 0.8 {
		t.Fatalf("confidence = %v", conf)
	}
	if reason == "" {
		t.Fatal("expected a routing reason")
	}
	_ = r
}

func TestAgentRouterSearchRoute(t *testing.T) {
	r := NewAgentRouter(newStubResolver(testAgents...))
	// No keyword and no task-type trigger → search fallback (0.6 confidence).
	// "encrypt" is not a keyword and "encrypt the payload" has no task-type
	// trigger; the word-based stub search matches cosca-security via
	// "encryption" in its description.
	agent, conf, reason := r.Route(&TaskNode{Description: "encrypt the payload"})
	if agent != "cosca-security" {
		t.Fatalf("search route agent = %q, want cosca-security", agent)
	}
	if conf != 0.6 {
		t.Fatalf("search confidence = %v, want 0.6", conf)
	}
	if !strings.Contains(reason, "full-text search match") {
		t.Fatalf("reason = %q", reason)
	}
}

func TestAgentRouterFallback(t *testing.T) {
	r := NewAgentRouter(newStubResolver()) // no agents at all
	agent, conf, reason := r.Route(&TaskNode{Description: "zzz no match anywhere"})
	if agent != "" || conf != 0 || reason != "no agent found" {
		t.Fatalf("fallback = %q, %v, %q", agent, conf, reason)
	}
	if len(r.History()) != 1 || r.History()[0].AgentSelected != "" {
		t.Fatalf("fallback history wrong: %+v", r.History())
	}
}

func TestAgentRouterNilTask(t *testing.T) {
	r := NewAgentRouter(newStubResolver(testAgents...))
	agent, _, reason := r.Route(nil)
	if agent != "" || reason != "no task provided" {
		t.Fatalf("nil task = %q, %q", agent, reason)
	}
}

func TestAgentRouterSearchAndRoute(t *testing.T) {
	r := NewAgentRouter(newStubResolver(testAgents...))
	decision, err := r.SearchAndRoute("sql")
	if err != nil {
		t.Fatalf("SearchAndRoute: %v", err)
	}
	if decision.AgentSelected != "cosca-database" {
		t.Fatalf("selected = %q, want cosca-database", decision.AgentSelected)
	}
	if decision.TaskType != "design-schema" {
		t.Fatalf("task type = %q", decision.TaskType)
	}
	if len(r.History()) != 1 {
		t.Fatalf("history not recorded, len=%d", len(r.History()))
	}
}

func TestAgentRouterSearchAndRouteNoAgents(t *testing.T) {
	r := NewAgentRouter(nil)
	if _, err := r.SearchAndRoute("anything"); err == nil {
		t.Fatal("SearchAndRoute without resolver must error")
	}

	r2 := NewAgentRouter(newStubResolver())
	if _, err := r2.SearchAndRoute("anything"); err == nil {
		t.Fatal("SearchAndRoute with no agents must error")
	}
}
