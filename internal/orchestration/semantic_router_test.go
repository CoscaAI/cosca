package orchestration

import (
	"context"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/embeddings"
)

// ─── Mock Embedder ────────────────────────────────────────────────────────────

// mockEmbedder returns predictable embeddings for testing.
type mockEmbedder struct {
	embeddings map[string][]float64
}

func newMockEmbedder() *mockEmbedder {
	return &mockEmbedder{embeddings: make(map[string][]float64)}
}

func (m *mockEmbedder) GenerateEmbedding(_ context.Context, text string) (*embeddings.EmbeddingResult, error) {
	return &embeddings.EmbeddingResult{
		Vector:     m.generateEmbedding(text),
		Model:      "mock",
		Dimensions: 8,
	}, nil
}

// GenerateEmbeddings implements the batch embedding interface for testing.
func (m *mockEmbedder) GenerateEmbeddings(_ context.Context, texts []string) ([]*embeddings.EmbeddingResult, error) {
	results := make([]*embeddings.EmbeddingResult, len(texts))
	for i, text := range texts {
		results[i] = &embeddings.EmbeddingResult{
			Vector:     m.generateEmbedding(text),
			Model:      "mock",
			Dimensions: 8,
		}
	}
	return results, nil
}

// generateEmbedding returns a simple embedding based on text content.
// Words map to dimensions: "database" → high dim0, "frontend" → high dim1, etc.
func (m *mockEmbedder) generateEmbedding(text string) []float64 {
	vec := make([]float64, 8)
	lower := strings.ToLower(text)
	if strings.Contains(lower, "database") || strings.Contains(lower, "sql") || strings.Contains(lower, "schema") || strings.Contains(lower, "tabela") || strings.Contains(lower, "modelar") {
		vec[0] = 1.0
	}
	if strings.Contains(lower, "frontend") || strings.Contains(lower, "ui") || strings.Contains(lower, "react") || strings.Contains(lower, "tela") || strings.Contains(lower, "componente") {
		vec[1] = 1.0
	}
	if strings.Contains(lower, "backend") || strings.Contains(lower, "api") || strings.Contains(lower, "rest") || strings.Contains(lower, "endpoint") || strings.Contains(lower, "rota") || strings.Contains(lower, "serviço") {
		vec[2] = 1.0
	}
	if strings.Contains(lower, "security") || strings.Contains(lower, "auth") || strings.Contains(lower, "vulnerab") || strings.Contains(lower, "proteger") {
		vec[3] = 1.0
	}
	if strings.Contains(lower, "test") || strings.Contains(lower, "verificar") || strings.Contains(lower, "validar") {
		vec[4] = 1.0
	}
	if strings.Contains(lower, "deploy") || strings.Contains(lower, "kubernetes") || strings.Contains(lower, "docker") {
		vec[5] = 1.0
	}
	if strings.Contains(lower, "document") || strings.Contains(lower, "readme") || strings.Contains(lower, "documentar") {
		vec[6] = 1.0
	}
	if strings.Contains(lower, "architecture") || strings.Contains(lower, "design") || strings.Contains(lower, "arquitetura") {
		vec[7] = 1.0
	}
	return vec
}

// ─── Helper ───────────────────────────────────────────────────────────────────

// setupAgents creates agents with descriptions that align with mock embedder dimensions.
func setupAgents() *mockAgentResolver {
	r := newMockAgentResolver()
	r.add("Database Chief", "Database architecture expert", "database", "Designs database schemas, SQL queries, and data models")
	r.add("Backend Chief", "Backend development lead", "backend", "Designs APIs, REST endpoints, backend services, and server-side logic")
	r.add("Frontend Chief", "Frontend development lead", "frontend", "Builds UI components, React apps, and user interfaces")
	r.add("Security Chief", "Security architecture lead", "security", "Handles authentication, authorization, and vulnerability scanning")
	r.add("Testing Chief", "Testing and QA lead", "qa", "Writes unit tests, integration tests, and validates code quality")
	r.add("DevOps Chief", "DevOps and infrastructure lead", "devops", "Manages deployments, Docker, Kubernetes, and CI/CD pipelines")
	r.add("Documentation Chief", "Documentation lead", "docs", "Writes technical documentation, READMEs, guides, and manuals")
	r.add("Architecture Chief", "Architecture lead", "architecture", "Designs system architecture, software patterns, and ADRs")
	return r
}

// ─── Tests: English ───────────────────────────────────────────────────────────

func TestSemanticRouter_English_DatabaseSchema(t *testing.T) {
	agents := setupAgents()
	embedder := newMockEmbedder()
	keyword := NewRouter(agents)

	sr := NewSemanticRouter(agents, embedder, keyword, DefaultSemanticRouterConfig())
	// Force compute embeddings
	_ = sr.computeEmbeddings(context.Background())

	pc := NewPipelineContext("req-1", "design a database schema for users")
	result, err := sr.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Database Chief" {
		t.Errorf("expected 'Database Chief', got %v (semantic_score=%.2f)", got, result.Data.SemanticScore)
	}
	if got := result.Data.RouterMethod; got != "semantic_high" && got != "semantic_low" {
		t.Errorf("expected semantic routing method, got %v", got)
	}
}

func TestSemanticRouter_English_BackendAPI(t *testing.T) {
	agents := setupAgents()
	embedder := newMockEmbedder()
	keyword := NewRouter(agents)

	sr := NewSemanticRouter(agents, embedder, keyword, DefaultSemanticRouterConfig())
	_ = sr.computeEmbeddings(context.Background())

	pc := NewPipelineContext("req-2", "create a REST endpoint for authentication")
	result, err := sr.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Backend Chief" {
		t.Errorf("expected 'Backend Chief', got %v (Portuguese 'rota' = 'endpoint')", got)
	}
}

func TestSemanticRouter_English_FrontendUI(t *testing.T) {
	agents := setupAgents()
	embedder := newMockEmbedder()
	keyword := NewRouter(agents)

	sr := NewSemanticRouter(agents, embedder, keyword, DefaultSemanticRouterConfig())
	_ = sr.computeEmbeddings(context.Background())

	pc := NewPipelineContext("req-3", "build a React component for the login page")
	result, err := sr.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Frontend Chief" {
		t.Errorf("expected 'Frontend Chief', got %v", got)
	}
}

func TestSemanticRouter_English_Security(t *testing.T) {
	agents := setupAgents()
	embedder := newMockEmbedder()
	keyword := NewRouter(agents)

	sr := NewSemanticRouter(agents, embedder, keyword, DefaultSemanticRouterConfig())
	_ = sr.computeEmbeddings(context.Background())

	pc := NewPipelineContext("req-4", "scan for vulnerabilities in the auth module")
	result, err := sr.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Security Chief" {
		t.Errorf("expected 'Security Chief', got %v", got)
	}
}

// ─── Tests: Portuguese (multi-language) ──────────────────────────────────────

func TestSemanticRouter_Portuguese_Database(t *testing.T) {
	agents := setupAgents()
	embedder := newMockEmbedder()
	keyword := NewRouter(agents)

	sr := NewSemanticRouter(agents, embedder, keyword, DefaultSemanticRouterConfig())
	_ = sr.computeEmbeddings(context.Background())

	// "modelar as tabelas do banco" should route to Database Chief
	pc := NewPipelineContext("req-pt1", "modelar as tabelas do banco de dados")
	result, err := sr.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Database Chief" {
		t.Errorf("expected 'Database Chief', got %v (Portuguese prompt should match via embeddings)", got)
	}
}

func TestSemanticRouter_Portuguese_Backend(t *testing.T) {
	agents := setupAgents()
	embedder := newMockEmbedder()
	keyword := NewRouter(agents)

	sr := NewSemanticRouter(agents, embedder, keyword, DefaultSemanticRouterConfig())
	_ = sr.computeEmbeddings(context.Background())

	// "construir uma rota HTTP" should route to Backend Chief
	pc := NewPipelineContext("req-pt2", "criar uma rota HTTP para o serviço de login")
	result, err := sr.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Backend Chief" {
		t.Errorf("expected 'Backend Chief', got %v (Portuguese 'rota' = 'endpoint')", got)
	}
}

func TestSemanticRouter_Portuguese_Frontend(t *testing.T) {
	agents := setupAgents()
	embedder := newMockEmbedder()
	keyword := NewRouter(agents)

	sr := NewSemanticRouter(agents, embedder, keyword, DefaultSemanticRouterConfig())
	_ = sr.computeEmbeddings(context.Background())

	// "fazer a tela de login" should route to Frontend Chief
	pc := NewPipelineContext("req-pt3", "fazer a tela de login com components React")
	result, err := sr.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Frontend Chief" {
		t.Errorf("expected 'Frontend Chief', got %v (Portuguese 'tela' = 'screen/UI')", got)
	}
}

func TestSemanticRouter_Portuguese_Security(t *testing.T) {
	agents := setupAgents()
	embedder := newMockEmbedder()
	keyword := NewRouter(agents)

	sr := NewSemanticRouter(agents, embedder, keyword, DefaultSemanticRouterConfig())
	_ = sr.computeEmbeddings(context.Background())

	// "proteger os dados dos usuários" (protect the user data) is a security
	// request. With the stricter LowThreshold the embedding score falls just
	// below the band, so it resolves via keyword fallback: "senha"/"segurança"
	// keywords outrank the analytics "dados" keyword in priority order.
	pc := NewPipelineContext("req-pt4", "proteger os dados dos usuários contra acesso não autorizado e senhas fracas")
	result, err := sr.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Security Chief" {
		t.Errorf("expected 'Security Chief', got %v (Portuguese 'proteger' = 'protect/security')", got)
	}
}

// ─── Tests: Fallback ──────────────────────────────────────────────────────────

func TestSemanticRouter_FallbackToKeyword(t *testing.T) {
	agents := setupAgents()
	embedder := newMockEmbedder()
	keyword := NewRouter(agents)

	cfg := DefaultSemanticRouterConfig()
	// Very high thresholds — forces fallback below both bands.
	cfg.HighThreshold = 0.99
	cfg.LowThreshold = 0.95
	cfg.FallbackToKeyword = true

	sr := NewSemanticRouter(agents, embedder, keyword, cfg)
	_ = sr.computeEmbeddings(context.Background())

	// Partially-matching prompt: dim0 (database) and dim2 (backend) both
	// score ~0.707, below the 0.95 threshold — must fall through.
	pc := NewPipelineContext("req-fb1", "database query for backend services")
	result, err := sr.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fall through to keyword router and match "database/query"
	if got := result.Data.ResolvedAgent; got != "Database Chief" && got != "SQL Database Specialist" {
		t.Errorf("expected database agent via keyword fallback, got %v", got)
	}
}

func TestSemanticRouter_MarginRejectsAmbiguousMatch(t *testing.T) {
	// Two agents with identical descriptions produce identical embeddings,
	// so the prompt ties them at score 1.0 with a 0.0 margin. Even though
	// the top score clears HighThreshold (0.85), the missing margin means
	// the top-1 is arbitrary — the router must NOT accept the semantic
	// match and instead fall back to keyword routing.
	agents := newMockAgentResolver()
	agents.add("Security Chief", "Security architecture lead", "security", "Handles authentication, authorization, and vulnerability scanning")
	agents.add("Security Operations", "Security architecture lead", "security", "Handles authentication, authorization, and vulnerability scanning")

	embedder := newMockEmbedder()
	keyword := NewRouter(agents)
	sr := NewSemanticRouter(agents, embedder, keyword, DefaultSemanticRouterConfig())
	_ = sr.computeEmbeddings(context.Background())

	// Prompt matches both security dims (security + architecture), so both
	// agents score 1.0 — a high score with a zero margin.
	pc := NewPipelineContext("req-margin", "design a security architecture review for authentication vulnerabilities")
	result, err := sr.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.RouterMethod; got == "semantic_high" || got == "semantic_low" {
		t.Errorf("ambiguous top-1 with zero margin must not route semantically, got %v", got)
	}

	// Keyword fallback resolves the security prompt to Security Chief.
	if got := result.Data.ResolvedAgent; got != "Security Chief" {
		t.Errorf("expected keyword fallback to 'Security Chief', got %v", got)
	}
}

func TestSemanticRouter_ExplicitAgent(t *testing.T) {
	agents := setupAgents()
	embedder := newMockEmbedder()
	keyword := NewRouter(agents)

	sr := NewSemanticRouter(agents, embedder, keyword, DefaultSemanticRouterConfig())
	_ = sr.computeEmbeddings(context.Background())

	// Explicit agent hint bypasses all routing
	pc := NewPipelineContext("req-exp", "anything")
	pc = pc.WithContextData("agent", "Security Chief")

	result, err := sr.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Data.ResolvedAgent; got != "Security Chief" {
		t.Errorf("expected explicit 'Security Chief', got %v", got)
	}
	if got := result.Data.RouterMethod; got != "explicit" {
		t.Errorf("expected router_method=explicit, got %v", got)
	}
}

func TestSemanticRouter_Disabled(t *testing.T) {
	agents := setupAgents()
	embedder := newMockEmbedder()
	keyword := NewRouter(agents)

	cfg := DefaultSemanticRouterConfig()
	cfg.Enabled = false

	sr := NewSemanticRouter(agents, embedder, keyword, cfg)

	pc := NewPipelineContext("req-dis", "create api endpoint")
	result, err := sr.Route(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// When disabled, should delegate to keyword router and match "api"
	if got := result.Data.ResolvedAgent; got != "Backend API Specialist" && got != "Backend Chief" {
		t.Errorf("expected backend agent via keyword fallback, got %v", got)
	}
}

// ─── Tests: Cosine Similarity ─────────────────────────────────────────────────

func TestCosineSimilarity_Identical(t *testing.T) {
	a := []float64{1, 2, 3}
	b := []float64{1, 2, 3}
	score := cosineSimilarity(a, b)
	if score < 0.99 {
		t.Errorf("expected ~1.0 for identical vectors, got %.4f", score)
	}
}

func TestCosineSimilarity_Opposite(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{0, 1, 0}
	score := cosineSimilarity(a, b)
	if score > 0.01 {
		t.Errorf("expected ~0.0 for orthogonal vectors, got %.4f", score)
	}
}

func TestCosineSimilarity_DifferentDimensions(t *testing.T) {
	a := []float64{1, 1}
	b := []float64{1, 1, 1, 1}
	score := cosineSimilarity(a, b)
	// INVARIANTE (Tool Execution Policy / review professor): dimensões
	// divergentes NÃO devem ser truncadas silenciosamente — isso produziria
	// um score numericamente válido mas semanticamente inválido. O COSCA
	// sinaliza retornando -1 (impossível para cosseno válido em [0,1]), o que
	// faz o router descartar o candidato em vez de tomar uma decisão
	// aparentemente válida.
	if score != -1 {
		t.Errorf("expected -1 for mismatched dimensions, got %.4f", score)
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := []float64{0, 0, 0}
	b := []float64{1, 2, 3}
	score := cosineSimilarity(a, b)
	if score != 0 {
		t.Errorf("expected 0 for zero vector, got %.4f", score)
	}
}

// ─── Tests: Builder ───────────────────────────────────────────────────────────

func TestBuildAgentDescription(t *testing.T) {
	agent := AgentInfo{
		Name:        "Backend Chief",
		Role:        "Leads backend development",
		Department:  "backend",
		Description: "Designs APIs and services",
	}

	desc := buildAgentDescription(agent)
	if !strings.Contains(desc, "Backend Chief") {
		t.Error("description should contain agent name")
	}
	if !strings.Contains(desc, "Leads backend development") {
		t.Error("description should contain agent role")
	}
	if !strings.Contains(desc, "backend") {
		t.Error("description should contain department")
	}
	if !strings.Contains(desc, "Designs APIs and services") {
		t.Error("description should contain full description")
	}
}

func TestBuildAgentDescription_NoDepartment(t *testing.T) {
	agent := AgentInfo{
		Name:        "CEO Agent",
		Role:        "Chief Executive Officer",
		Description: "Makes strategic decisions",
	}
	desc := buildAgentDescription(agent)
	if strings.Contains(desc, "Department") {
		t.Error("description should not mention department when empty")
	}
}

// ─── Tests: Config Defaults ───────────────────────────────────────────────────

func TestDefaultSemanticRouterConfig(t *testing.T) {
	cfg := DefaultSemanticRouterConfig()
	if !cfg.Enabled {
		t.Error("default config should have Enabled=true")
	}
	if cfg.HighThreshold != 0.85 {
		t.Errorf("expected HighThreshold=0.85, got %.2f", cfg.HighThreshold)
	}
	if cfg.LowThreshold != 0.72 {
		t.Errorf("expected LowThreshold=0.72, got %.2f", cfg.LowThreshold)
	}
	if cfg.MaxCandidates != 5 {
		t.Errorf("expected MaxCandidates=5, got %d", cfg.MaxCandidates)
	}
	if !cfg.FallbackToKeyword {
		t.Error("default should have FallbackToKeyword=true")
	}
}

func TestNewSemanticRouter_DefaultsApplied(t *testing.T) {
	agents := setupAgents()
	embedder := newMockEmbedder()
	keyword := NewRouter(agents)

	cfg := SemanticRouterConfig{} // all zeros
	sr := NewSemanticRouter(agents, embedder, keyword, cfg)

	if sr.config.HighThreshold != 0.85 {
		t.Errorf("expected default HighThreshold, got %.2f", sr.config.HighThreshold)
	}
	if sr.config.LowThreshold != 0.72 {
		t.Errorf("expected default LowThreshold, got %.2f", sr.config.LowThreshold)
	}
	if sr.config.MaxCandidates != 5 {
		t.Errorf("expected default MaxCandidates, got %d", sr.config.MaxCandidates)
	}
}
