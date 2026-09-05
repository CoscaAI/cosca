package orchestration

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// ─── Mock Agent Resolver ─────────────────────────────────────────────────────

// mockAgentResolver implements AgentResolver with a controllable set of agents.
type mockAgentResolver struct {
	agents map[string]AgentInfo
}

func newMockAgentResolver() *mockAgentResolver {
	return &mockAgentResolver{agents: make(map[string]AgentInfo)}
}

func (m *mockAgentResolver) add(name, role, department, description string) {
	m.agents[name] = AgentInfo{
		Name:        name,
		Role:        role,
		Department:  department,
		Description: description,
	}
}

func (m *mockAgentResolver) Get(name string) (*AgentInfo, error) {
	// Case-insensitive
	lower := strings.ToLower(name)
	for k, v := range m.agents {
		if strings.ToLower(k) == lower {
			return &v, nil
		}
	}
	return nil, errors.New("agent not found")
}

func (m *mockAgentResolver) Search(query string) ([]AgentInfo, error) {
	var results []AgentInfo
	lower := strings.ToLower(query)
	for _, v := range m.agents {
		if strings.Contains(strings.ToLower(v.Name), lower) ||
			strings.Contains(strings.ToLower(v.Role), lower) ||
			strings.Contains(strings.ToLower(v.Department), lower) ||
			strings.Contains(strings.ToLower(v.Description), lower) {
			results = append(results, v)
		}
	}
	return results, nil
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestRouter_Route_ExplicitAgent(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Leads backend dev")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-1", "build an api")
	pc = pc.WithContextData("agent", "Backend Chief")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Backend Chief" {
		t.Errorf("expected 'Backend Chief', got %v", got)
	}
	if got := result.Data.AgentRole; got != "Backend Chief" {
		t.Errorf("expected role 'Backend Chief', got %v", got)
	}
}

func TestRouter_Route_ExplicitAgent_NotFound(t *testing.T) {
	// Agent in hint does not exist; router returns an error because no
	// other agent matches either keyword or search.
	resolver := newMockAgentResolver()
	resolver.add("CEO", "CEO", "ceo", "CEO agent")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-1", "build an api")
	pc = pc.WithContextData("agent", "Nonexistent Agent")

	_, err := router.Route(context.Background(), pc)
	if err == nil {
		t.Fatal("expected an error when no agent matches, got nil")
	}
	t.Logf("expected error: %v", err)
}

func TestRouter_Route_KeywordMatching_API(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Leads backend development")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-1", "build a REST api endpoint for user management")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Backend Chief" {
		t.Errorf("expected 'Backend Chief', got %v", got)
	}
}

func TestRouter_Route_KeywordMatching_Frontend(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("Frontend Chief", "Frontend Chief", "frontend", "Leads frontend dev")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-2", "create a react component for the navbar")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Frontend Chief" {
		t.Errorf("expected 'Frontend Chief', got %v", got)
	}
}

func TestRouter_Route_KeywordMatching_Database(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("Database Chief", "Database Chief", "database", "Database expert")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-3", "write a sql migration to add a new column")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Database Chief" {
		t.Errorf("expected 'Database Chief', got %v", got)
	}
}

func TestRouter_Route_KeywordMatching_Testing(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("Testing Chief", "Testing Chief", "qa", "Testing expert")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-4", "write unit test for the auth module")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Testing Chief" {
		t.Errorf("expected 'Testing Chief', got %v", got)
	}
}

func TestRouter_Route_KeywordMatching_Security(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("Security Chief", "Security Chief", "security", "Security expert")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-5", "scan for vulnerabilities in the auth module")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Security Chief" {
		t.Errorf("expected 'Security Chief', got %v", got)
	}
}

// ─── Tests: Portuguese keyword routing ───────────────────────────────────────

func TestRouter_Route_KeywordMatching_Security_Portuguese(t *testing.T) {
	// "audite a segurança do sistema" must reach the Security Chief via the
	// PT-BR keywords even though no English keyword is present.
	resolver := newMockAgentResolver()
	resolver.add("Security Chief", "Security Chief", "security", "Security expert")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-pt-sec", "audite a segurança do sistema")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Security Chief" {
		t.Errorf("expected 'Security Chief', got %v", got)
	}
}

func TestRouter_Route_KeywordMatching_Testing_Portuguese(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("Testing Chief", "Testing Chief", "qa", "Testing expert")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-pt-test", "escreva testes unitarios")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Testing Chief" {
		t.Errorf("expected 'Testing Chief', got %v", got)
	}
}

func TestRouter_Route_KeywordMatching_Backend_Portuguese(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Backend expert")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-pt-back", "implemente um endpoint REST de api")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Backend Chief" {
		t.Errorf("expected 'Backend Chief', got %v", got)
	}
}

func TestRouter_Route_KeywordMatching_NoMatch_Portuguese(t *testing.T) {
	// Generic question: "api" must NOT match inside "capital". No keyword
	// matches and full-text search finds nothing, so the router errors.
	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Backend expert")
	resolver.add("CEO", "CEO", "ceo", "CEO agent")

	router := NewRouter(resolver)
	prompt := "qual é a capital do brasil"

	if matched := router.matchKeywords(strings.ToLower(prompt)); len(matched) != 0 {
		t.Errorf("expected no keyword match for generic PT query, got %d entries", len(matched))
	}

	pc := NewPipelineContext("req-pt-no", prompt)
	_, err := router.Route(context.Background(), pc)
	if err == nil {
		t.Fatal("expected an error when no agent matches, got nil")
	}
	if !strings.Contains(err.Error(), "no agent found") {
		t.Errorf("expected error to contain 'no agent found', got: %v", err)
	}
}

func TestRouter_Route_KeywordMatching_Architecture(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("Architecture Chief", "Architecture Chief", "architecture", "Architecture expert")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-6", "write an ADR for the new design pattern")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Architecture Chief" {
		t.Errorf("expected 'Architecture Chief', got %v", got)
	}
}

func TestRouter_Route_KeywordMatching_DevOps(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("DevOps Chief", "DevOps Chief", "devops", "DevOps expert")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-7", "deploy the app using docker and kubernetes")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "DevOps Chief" {
		t.Errorf("expected 'DevOps Chief', got %v", got)
	}
}

func TestRouter_Route_KeywordMatching_Documentation(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("Documentation Chief", "Documentation Chief", "documentation", "Docs expert")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-8", "update the readme and api docs")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Documentation Chief" {
		t.Errorf("expected 'Documentation Chief', got %v", got)
	}
}

func TestRouter_Route_KeywordMatching_Review(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("Review Chief", "Review Chief", "review", "Review expert")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-9", "do a code review of the latest PR")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Review Chief" {
		t.Errorf("expected 'Review Chief', got %v", got)
	}
}

func TestRouter_Route_KeywordMatching_Monitoring(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("Monitoring Chief", "Monitoring Chief", "monitoring", "Monitoring expert")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-10", "set up observability and alerting")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Monitoring Chief" {
		t.Errorf("expected 'Monitoring Chief', got %v", got)
	}
}

func TestRouter_Route_KeywordMatching_Analytics(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("Analytics Chief", "Analytics Chief", "analytics", "Analytics expert")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-11", "create a dashboard for metrics and data analysis")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Analytics Chief" {
		t.Errorf("expected 'Analytics Chief', got %v", got)
	}
}

func TestRouter_Route_KeywordMatching_AI(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("AI Chief", "AI Chief", "ai", "AI expert")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-12", "implement a RAG pipeline with embeddings")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "AI Chief" {
		t.Errorf("expected 'AI Chief', got %v", got)
	}
}

func TestRouter_Route_KeywordMatching_Mobile(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("Mobile Chief", "Mobile Chief", "mobile", "Mobile expert")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-13", "build an ios and android app")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Mobile Chief" {
		t.Errorf("expected 'Mobile Chief', got %v", got)
	}
}

func TestRouter_Route_KeywordMatching_Release(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("Release Chief", "Release Chief", "release", "Release expert")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-14", "update the changelog for version 2.0")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Release Chief" {
		t.Errorf("expected 'Release Chief', got %v", got)
	}
}

func TestRouter_Route_KeywordMatching_Infrastructure(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("Infrastructure Chief", "Infrastructure Chief", "infrastructure", "Infra expert")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-15", "design the cloud networking architecture")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Infrastructure Chief" {
		t.Errorf("expected 'Infrastructure Chief', got %v", got)
	}
}

func TestRouter_Route_KeywordMatching_Product(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("Product Chief", "Product Chief", "product", "Product expert")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-16", "prioritize the backlog based on user requirements")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Product Chief" {
		t.Errorf("expected 'Product Chief', got %v", got)
	}
}

func TestRouter_Route_KeywordMatching_Workflow(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("Workflow Chief", "Workflow Chief", "workflow", "Workflow expert")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-17", "automate the CI pipeline")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Workflow Chief" {
		t.Errorf("expected 'Workflow Chief', got %v", got)
	}
}

func TestRouter_Route_KeywordMatching_Context(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("Context Chief", "Context Chief", "context", "Context expert")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-18", "check the current environment and session state")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Context Chief" {
		t.Errorf("expected 'Context Chief', got %v", got)
	}
}

func TestRouter_Route_SpecialistBeforeChief(t *testing.T) {
	// When both specialist and chief exist, specialist should be preferred.
	resolver := newMockAgentResolver()
	resolver.add("Backend API Specialist", "Backend API Specialist", "backend", "API specialist")
	resolver.add("Backend Chief", "Backend Chief", "backend", "Backend chief")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-19", "create a new rest endpoint")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Backend API Specialist" {
		t.Errorf("expected 'Backend API Specialist' (specialist), got %v", got)
	}
}

func TestRouter_Route_FallbackToCEO(t *testing.T) {
	// When no keyword matches and no search results, the router returns an error
	// instead of silently falling back to the CEO. No hardcoded fallback anymore.
	resolver := newMockAgentResolver()
	resolver.add("CEO", "Chief Executive Officer", "ceo", "Makes strategic decisions")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-20", "do something completely unrelated to any department")

	_, err := router.Route(context.Background(), pc)
	if err == nil {
		t.Fatal("expected an error when no agent matches, got nil")
	}
	if !strings.Contains(err.Error(), "no agent found") {
		t.Errorf("expected error to contain 'no agent found', got: %v", err)
	}
}

func TestRouter_Route_FallbackToCEOAgent(t *testing.T) {
	// When no keyword matches and no search results, the router returns an error
	// instead of a hardcoded fallback. No hardcoded "CEO Agent" fallback anymore.
	resolver := newMockAgentResolver()
	resolver.add("CEO Agent", "CEO", "ceo", "The CEO")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-21", "do something random")

	_, err := router.Route(context.Background(), pc)
	if err == nil {
		t.Fatal("expected an error when no agent matches, got nil")
	}
	if !strings.Contains(err.Error(), "no agent found") {
		t.Errorf("expected error to contain 'no agent found', got: %v", err)
	}
}

func TestRouter_Route_HardcodedFallback(t *testing.T) {
	// When absolutely no agents exist in the resolver, the router returns an error
	// instead of a hardcoded "CEO Agent" fallback (which was intentionally removed).
	resolver := newMockAgentResolver()
	router := NewRouter(resolver)
	pc := NewPipelineContext("req-22", "anything")

	_, err := router.Route(context.Background(), pc)
	if err == nil {
		t.Fatal("expected an error when no agents available, got nil")
	}
	if !strings.Contains(err.Error(), "no agent found") {
		t.Errorf("expected error to contain 'no agent found', got: %v", err)
	}
}

func TestRouter_Route_SearchFallback(t *testing.T) {
	// When keyword matching fails but Search finds something.
	resolver := newMockAgentResolver()
	resolver.add("Specialized Agent", "Custom Role", "custom", "Handles custom stuff that relates to special things")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-23", "special things")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Specialized Agent" {
		t.Errorf("expected 'Specialized Agent' from search, got %v", got)
	}
}

func TestRouter_Route_SkillsDerived(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Leads backend development")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-24", "build a REST api with database migrations")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	skills := result.Data.SkillsUsed

	if len(skills) == 0 {
		t.Error("expected at least one skill to be derived")
	}

	// Should have api-related skills from keyword matching.
	foundAPI := false
	for _, s := range skills {
		if s == "api-design" || s == "rest-api" {
			foundAPI = true
			break
		}
	}
	if !foundAPI {
		t.Errorf("expected api-related skills, got: %v", skills)
	}
}

func TestRouter_Route_AgentRoleStored(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Development Lead", "backend", "Leads backend")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-25", "build an api")

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.AgentRole; got != "Backend Development Lead" {
		t.Errorf("expected 'Backend Development Lead', got %v", got)
	}
	if got := result.Data.AgentDepartment; got != "backend" {
		t.Errorf("expected 'backend', got %v", got)
	}
	if got := result.Data.AgentDescription; got != "Leads backend" {
		t.Errorf("expected 'Leads backend', got %v", got)
	}
}

func TestRouter_Route_CaseInsensitiveExplicitAgent(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("BACKEND CHIEF", "Backend Chief", "backend", "Leads backend")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-26", "do something")
	pc = pc.WithContextData("agent", "backend chief") // lowercase

	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "BACKEND CHIEF" {
		t.Errorf("expected 'BACKEND CHIEF', got %v", got)
	}
}

func TestRouter_Route_EmptyExplicitAgent(t *testing.T) {
	// Empty/whitespace agent hint should be ignored, and when no keywords match
	// nor search finds anything, the router returns an error (no hardcoded fallback).
	resolver := newMockAgentResolver()
	resolver.add("CEO", "CEO", "ceo", "CEO")

	router := NewRouter(resolver)
	pc := NewPipelineContext("req-27", "do something")
	pc = pc.WithContextData("agent", "  ") // whitespace only

	_, err := router.Route(context.Background(), pc)
	if err == nil {
		t.Fatal("expected an error when no agent matches and no fallback, got nil")
	}
	if !strings.Contains(err.Error(), "no agent found") {
		t.Errorf("expected error to contain 'no agent found', got: %v", err)
	}
}

// ─── Tests: Accent-insensitive (normalized) keyword routing ──────────────────

func TestRouter_NormalizeText(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Segurança", "seguranca"},
		{"autenticação", "autenticacao"},
		{"memória", "memoria"},
		{"governança", "governanca"},
		{"otimização", "otimizacao"},
		{"direção", "direcao"},
		{"QUAL É A CAPITAL DO BRASIL", "qual e a capital do brasil"},
	}
	for _, tt := range tests {
		if got := normalizeText(tt.in); got != tt.want {
			t.Errorf("normalizeText(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func routeToAgent(t *testing.T, resolver *mockAgentResolver, prompt string) string {
	t.Helper()
	router := NewRouter(resolver)
	pc := NewPipelineContext("req-norm", prompt)
	result, err := router.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("prompt %q: unexpected error: %v", prompt, err)
	}
	return result.Data.ResolvedAgent
}

func TestRouter_Route_Normalized_Security_NoAccents(t *testing.T) {
	// "seguranca" (no ç) must reach Security Chief exactly like "segurança".
	resolver := newMockAgentResolver()
	resolver.add("Security Chief", "Security Chief", "security", "Security expert")
	if got := routeToAgent(t, resolver, "seguranca do sistema"); got != "Security Chief" {
		t.Errorf("expected 'Security Chief', got %v", got)
	}
}

func TestRouter_Route_Normalized_Authentication(t *testing.T) {
	// "autenticacao" (no ç) must reach Security Chief.
	resolver := newMockAgentResolver()
	resolver.add("Security Chief", "Security Chief", "security", "Security expert")
	if got := routeToAgent(t, resolver, "autenticacao"); got != "Security Chief" {
		t.Errorf("expected 'Security Chief', got %v", got)
	}
}

func TestRouter_Route_Normalized_Memory(t *testing.T) {
	// "memoria" (no accent) must reach Memory Chief.
	resolver := newMockAgentResolver()
	resolver.add("Memory Chief", "Memory Chief", "memory", "Memory expert")
	if got := routeToAgent(t, resolver, "memoria"); got != "Memory Chief" {
		t.Errorf("expected 'Memory Chief', got %v", got)
	}
}

func TestRouter_Route_Normalized_Governance(t *testing.T) {
	// "governanca" (no ç) must reach Governance Chief.
	resolver := newMockAgentResolver()
	resolver.add("Governance Chief", "Governance Chief", "governance", "Governance expert")
	if got := routeToAgent(t, resolver, "governanca"); got != "Governance Chief" {
		t.Errorf("expected 'Governance Chief', got %v", got)
	}
}

func TestRouter_Route_Normalized_Performance(t *testing.T) {
	// "otimizacao de performance" (no accents) must reach Performance Chief.
	resolver := newMockAgentResolver()
	resolver.add("Performance Chief", "Performance Chief", "performance", "Performance expert")
	if got := routeToAgent(t, resolver, "otimizacao de performance"); got != "Performance Chief" {
		t.Errorf("expected 'Performance Chief', got %v", got)
	}
}

// ─── Tests: New enriched registry agents ─────────────────────────────────────

func TestRouter_Route_Compliance(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("Compliance Chief", "Compliance Chief", "compliance", "Compliance expert")
	if got := routeToAgent(t, resolver, "conformidade com lgpd"); got != "Compliance Chief" {
		t.Errorf("expected 'Compliance Chief', got %v", got)
	}
}

func TestRouter_Route_Messaging(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("Messaging Chief", "Messaging Chief", "messaging", "Messaging expert")
	if got := routeToAgent(t, resolver, "estrutura de mensageria com filas"); got != "Messaging Chief" {
		t.Errorf("expected 'Messaging Chief', got %v", got)
	}
}

func TestRouter_Route_CEO(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("CEO Agent", "CEO", "ceo", "The CEO")
	if got := routeToAgent(t, resolver, "estrategia de negocio e roadmap"); got != "CEO Agent" {
		t.Errorf("expected 'CEO Agent', got %v", got)
	}
}

func TestRouter_Route_CTO(t *testing.T) {
	resolver := newMockAgentResolver()
	resolver.add("CTO Agent", "CTO", "cto", "The CTO")
	if got := routeToAgent(t, resolver, "qual tecnologia devemos adotar no stack"); got != "CTO Agent" {
		t.Errorf("expected 'CTO Agent', got %v", got)
	}
}

// ─── Tests: Greetings route to the Kernel Agent ──────────────────────────────

func TestRouter_Route_Greeting_ToKernel(t *testing.T) {
	// Greetings and small talk must reach the COSCA KERNEL (the family's front
	// door) and never full-text match a random specialist ("Oi" used to match
	// the Testing Integration Specialist).
	resolver := newMockAgentResolver()
	resolver.add("COSCA KERNEL", "Kernel", "kernel", "The family's front door")
	resolver.add("Testing Integration Specialist", "Testing", "qa", "Runs integration tests")

	greetings := []string{"Oi", "olá", "Ola", "Bom dia", "Boa tarde", "Boa noite", "hello", "hi", "hey", "e aí", "e ai", "tudo bem", "como vai"}
	for _, prompt := range greetings {
		if got := routeToAgent(t, resolver, prompt); got != "COSCA KERNEL" {
			t.Errorf("greeting %q: expected 'COSCA KERNEL', got %v", prompt, got)
		}
	}
}

func TestRouter_Route_Greeting_WithCooccurringKeyword(t *testing.T) {
	// A greeting plus a specialist keyword still routes to the Kernel because
	// the greeting entry has highest priority.
	resolver := newMockAgentResolver()
	resolver.add("COSCA KERNEL", "Kernel", "kernel", "The family's front door")
	resolver.add("Backend API Specialist", "Backend API Specialist", "backend", "API specialist")

	if got := routeToAgent(t, resolver, "Oi, preciso de uma api"); got != "COSCA KERNEL" {
		t.Errorf("expected 'COSCA KERNEL', got %v", got)
	}
}

func TestRouter_Route_Greeting_KernelUnavailable_FallsBackToSearch(t *testing.T) {
	// When Kernel Agent is not resolvable, greetings keep the current behavior
	// (keyword entry skipped → defensive guard skipped → full-text search). No
	// panic, no hard error. Description contains "oi" so Search("Oi") matches.
	resolver := newMockAgentResolver()
	resolver.add("Testing Integration Specialist", "Testing", "qa", "Points to the right team")

	if got := routeToAgent(t, resolver, "Oi"); got != "Testing Integration Specialist" {
		t.Errorf("expected 'Testing Integration Specialist' via search fallback, got %v", got)
	}
}

func TestRouter_Route_NonGreeting_KeepsSearchPath(t *testing.T) {
	// A non-greeting prompt with no keyword match must keep the full-text
	// search path — the Kernel is not the destination for real work.
	resolver := newMockAgentResolver()
	resolver.add("COSCA KERNEL", "Kernel", "kernel", "The family's front door")
	resolver.add("Specialized Agent", "Custom Role", "custom", "Handles custom stuff that relates to special things")

	if got := routeToAgent(t, resolver, "special things"); got != "Specialized Agent" {
		t.Errorf("expected 'Specialized Agent' via search, got %v", got)
	}
}
