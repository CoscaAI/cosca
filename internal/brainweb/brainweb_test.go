package brainweb

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/agents"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/skills"
	"github.com/CoscaAI/cosca/internal/trace"
)

// helper agents para construir um grafo de teste determinístico.
func testAgents() *agents.Manager {
	m := agents.NewManager("")
	m.Add(agents.Agent{
		Name: "Don", Role: "Patriarca da Famiglia", Status: "active",
		Department: "kernel", Description: "Autoridade máxima",
	})
	m.Add(agents.Agent{
		Name: "Backend Chief", Role: "Backend Chief", Status: "active",
		Department: "backend", ReportsTo: "CTO",
		Tools: []agents.Tool{{Name: "write", Category: "fs", Purpose: "secreto"}}, // NÃO deve vazar
		Capabilities: []agents.Capability{{Name: "api", Description: "secreto"}},
	})
	m.Add(agents.Agent{
		Name: "Security Chief", Role: "Security Chief", Status: "active",
		Department: "security", ReportsTo: "CTO",
	})
	return m
}

func testSkills() *skills.Manager {
	m := skills.NewManager("")
	m.Add(skills.Skill{
		Name: "adb-generation", Category: "architecture", Description: "Gera ADR",
		Instructions: "CONTEÚDO SECRETO", // NÃO deve vazar
		Governance:   skills.SkillGovernance{Origin: "evolution", SuccessRate: 0.9},
	})
	m.Add(skills.Skill{
		Name: "unit-testing", Category: "testing", Description: "Testes AAA",
	})
	return m
}

// TestBuild_SanitizaCamposSensiveis garante que a projeção mínima NUNCA
// expõe tools/capabilities/dependencies de agents nem instructions/governance
// de skills (a regra de segurança aprovada pelo Don).
func TestBuild_SanitizaCamposSensiveis(t *testing.T) {
	b := NewBuilder(testAgents(), testSkills(), "1.5.0")
	g := b.Build()

	// A sanitização é por CONSTRUÇÃO: as projeções Node e SkillVz só têm
	// campos mínimos. Validamos via reflection que nenhum campo sensível
	// existe nos tipos de projeção.
	sensitiveFields := []string{"Instructions", "Governance", "Tools", "Capabilities",
		"Dependencies", "Inputs", "Outputs", "License", "Resources", "Metadata"}

	walkFields(t, reflect.TypeOf(Node{}), sensitiveFields)
	walkFields(t, reflect.TypeOf(SkillVz{}), sensitiveFields)

	// Assim o grafo não pode vazar nem por struct nem por dados concretos.
	for _, n := range g.Nodes {
		if n.ID == "" {
			t.Fatal("nó sem id")
		}
	}
}

// walkFields garante que um tipo de projeção não contém campos sensíveis.
func walkFields(t *testing.T, typ reflect.Type, sensitive []string) {
	t.Helper()
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i).Name
		for _, s := range sensitive {
			if f == s {
				t.Fatalf("projeção %s contém campo sensível %s", typ.Name(), f)
			}
		}
	}
}

// TestBuild_TemNucleoDon garante que o núcleo (Don) sempre existe.
func TestBuild_TemNucleoDon(t *testing.T) {
	b := NewBuilder(testAgents(), testSkills(), "1.5.0")
	g := b.Build()

	found := false
	for _, n := range g.Nodes {
		if n.ID == "don" && n.IsRoot {
			found = true
		}
	}
	if !found {
		t.Fatal("núcleo Don não encontrado")
	}
	if g.Meta.Agents != len(g.Nodes) {
		t.Fatalf("meta.agents=%d != nodes=%d", g.Meta.Agents, len(g.Nodes))
	}
}

// TestBuild_ArestasReportsTo valida as arestas de organograma.
func TestBuild_ArestasReportsTo(t *testing.T) {
	b := NewBuilder(testAgents(), testSkills(), "1.5.0")
	g := b.Build()

	hasBackendToCto := false
	for _, e := range g.Edges {
		if e.Source == "backend chief" && e.Kind == "reports_to" {
			hasBackendToCto = true
		}
	}
	// CTO não está no grafo de teste, então o reports_to deve cair no Don.
	if !hasBackendToCto {
		// Poderia apontar para don caso o CTO não exista — verificamos que há
		// alguma aresta reports_to partindo do backend chief.
		hasAny := false
		for _, e := range g.Edges {
			if e.Source == "backend chief" && e.Kind == "reports_to" {
				hasAny = true
			}
		}
		if !hasAny {
			t.Fatal("backend chief sem aresta reports_to")
		}
	}
}

// TestHandler_ServeIndex garante que GET /brain devolve HTML.
func TestHandler_ServeIndex(t *testing.T) {
	h := NewHandler(testAgents(), testSkills(), "1.5.0")
	req := httptest.NewRequest(http.MethodGet, "/brain", nil)
	rec := httptest.NewRecorder()
	h.Serve(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type=%q, esperado text/html", ct)
	}
	if !strings.Contains(rec.Body.String(), "COSCA") {
		t.Fatal("index.html não contém a marca COSCA")
	}
}

// TestHandler_GraphJSON garante que GET /brain/graph devolve JSON válido.
func TestHandler_GraphJSON(t *testing.T) {
	h := NewHandler(testAgents(), testSkills(), "1.5.0")
	req := httptest.NewRequest(http.MethodGet, "/brain/graph", nil)
	rec := httptest.NewRecorder()
	h.Graph(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type=%q, esperado application/json", ct)
	}
	var g Graph
	if err := json.Unmarshal(rec.Body.Bytes(), &g); err != nil {
		t.Fatalf("json inválido: %v", err)
	}
	if g.Meta.Root != "Don" {
		t.Fatalf("meta.root=%q, esperado Don", g.Meta.Root)
	}
}

// TestHandler_ServeAsset garante que GET /brain/app.js devolve JS.
func TestHandler_ServeAsset(t *testing.T) {
	h := NewHandler(testAgents(), testSkills(), "1.5.0")
	req := httptest.NewRequest(http.MethodGet, "/brain/app.js", nil)
	rec := httptest.NewRecorder()
	h.Serve(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/javascript") {
		t.Fatalf("content-type=%q, esperado application/javascript", ct)
	}
}

// TestHandler_ServeESMAssets garante que o módulo ESM self-hostado
// (three.module.js) é servido com o MIME correto.
func TestHandler_ServeESMAssets(t *testing.T) {
	h := NewHandler(testAgents(), testSkills(), "1.5.0")

	for _, path := range []string{
		"/brain/three.module.js",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.Serve(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status=%d, esperado 200", path, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/javascript") {
			t.Fatalf("%s: content-type=%q, esperado application/javascript", path, ct)
		}
	}
}

// TestHandler_ActivityNilSeguro garante que GET /brain/activity devolve JSON
// válido mesmo sem fonte de ações (nil-safe).
func TestHandler_ActivityNilSeguro(t *testing.T) {
	h := NewHandler(testAgents(), testSkills(), "1.5.0")
	req := httptest.NewRequest(http.MethodGet, "/brain/activity", nil)
	rec := httptest.NewRecorder()
	h.Activity(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type=%q, esperado application/json", ct)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json inválido: %v", err)
	}
	if _, ok := body["activities"]; !ok {
		t.Fatal("resposta não contém chave 'activities'")
	}
}

// TestHandler_ActivityInjected garante que uma fonte de ações injetada é
// servida no feed (read-only).
func TestHandler_ActivityInjected(t *testing.T) {
	src := fakeActivitySource{activities: []Activity{
		{ID: "a1", Agent: "Backend Chief", Status: "success", DurationMs: 1200},
	}}
	h := NewHandler(testAgents(), testSkills(), "1.5.0").WithActivity(src)
	req := httptest.NewRequest(http.MethodGet, "/brain/activity", nil)
	rec := httptest.NewRecorder()
	h.Activity(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", rec.Code)
	}
	var body struct {
		Activities []Activity `json:"activities"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json inválido: %v", err)
	}
	if len(body.Activities) != 1 || body.Activities[0].Agent != "Backend Chief" {
		t.Fatalf("activities inesperadas: %+v", body.Activities)
	}
}

// fakeActivitySource é uma fonte de ações de teste.
type fakeActivitySource struct {
	activities []Activity
}

func (f fakeActivitySource) Recent(limit int) []Activity {
	if limit < len(f.activities) {
		return f.activities[:limit]
	}
	return f.activities
}

// TestObservatory_NilSeguro garante que o observatório com fontes nil não
// causa pânico e devolve estruturas válidas.
func TestObservatory_NilSeguro(t *testing.T) {
	b := NewObservatoryBuilder(nil, nil, nil)
	obs := b.Build()
	if obs.GeneratedAt.IsZero() {
		t.Error("generated_at deve ser preenchido")
	}
	if obs.Cognitive.Epistemology == nil {
		t.Error("epistemology deve ser inicializada (vazia, não nil)")
	}
}

// TestObservatory_Constellations garante que itens são agrupados por estado
// epistemológico (ex.: KNOW, SUPPORTED) — a base da "física semântica".
func TestObservatory_Constellations(t *testing.T) {
	b := NewObservatoryBuilder(func() []knowledge.KnowledgeItem {
		return []knowledge.KnowledgeItem{
			{ID: "K-1", Title: "Postgres obrigatório", Status: knowledge.StatusKnown, Confidence: 0.95, Evidence: []knowledge.Evidence{{ID: "E1"}}},
			{ID: "K-2", Title: "SQLite como fallback", Status: knowledge.StatusSupported, Confidence: 0.8},
			{ID: "K-3", Title: "Pode usar MySQL", Status: knowledge.StatusUnknown},
		}
	}, nil, nil)
	obs := b.Build()

	if len(obs.Constellations) == 0 {
		t.Fatal("esperava constelações por epistemologia")
	}
	foundKnown, foundSupported, foundUnknown := false, false, false
	for _, c := range obs.Constellations {
		if c.Epistemic == "KNOWN" && c.Count == 1 {
			foundKnown = true
		}
		if c.Epistemic == "SUPPORTED" && c.Count == 1 {
			foundSupported = true
		}
		if c.Epistemic == "UNKNOWN" && c.Count == 1 {
			foundUnknown = true
		}
	}
	if !foundKnown || !foundSupported || !foundUnknown {
		t.Fatalf("constelações incompletas: %+v", obs.Constellations)
	}
	if obs.Cognitive.KnowledgeItems != 3 {
		t.Fatalf("knowledge_items=%d, esperado 3", obs.Cognitive.KnowledgeItems)
	}
}

// TestObservatory_TraceReplay garante que a cadeia causal é injetada.
func TestObservatory_TraceReplay(t *testing.T) {
	b := NewObservatoryBuilder(nil, func() ([]trace.CausalNode, []trace.CausalEdge, string) {
		nodes := []trace.CausalNode{
			{ID: "trace:T:0", Action: "OBSERVAR", Actor: "kernel"},
			{ID: "trace:T:1", Action: "DECIDIR", Actor: "agent-x"},
		}
		edges := []trace.CausalEdge{{From: "trace:T:0", To: "trace:T:1", Type: "caused"}}
		return nodes, edges, "TRACE-20260829-0001"
	}, nil)
	obs := b.Build()

	if obs.TraceReplay == nil {
		t.Fatal("trace_replay deve ser preenchido")
	}
	if obs.TraceReplay.TraceID != "TRACE-20260829-0001" {
		t.Fatalf("trace_id=%q", obs.TraceReplay.TraceID)
	}
	if len(obs.TraceReplay.Nodes) != 2 || len(obs.TraceReplay.Edges) != 1 {
		t.Fatalf("replay nodes/edges: %d/%d", len(obs.TraceReplay.Nodes), len(obs.TraceReplay.Edges))
	}
}

// TestHandler_ObservatoryNilSeguro garante que GET /brain/observatory responde
// JSON válido mesmo sem fontes (nil-safe).
func TestHandler_ObservatoryNilSeguro(t *testing.T) {
	h := NewHandler(testAgents(), testSkills(), "1.5.0")
	req := httptest.NewRequest(http.MethodGet, "/brain/observatory", nil)
	rec := httptest.NewRecorder()
	h.Observatory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type=%q", ct)
	}
	var obs Observatory
	if err := json.Unmarshal(rec.Body.Bytes(), &obs); err != nil {
		t.Fatalf("json inválido: %v", err)
	}
}

// TestHandler_IndexTemImportMap garante que o index.html define o import map
// (pre-requisito para os ES Modules carregarem como 'three').
func TestHandler_IndexTemImportMap(t *testing.T) {
	h := NewHandler(testAgents(), testSkills(), "1.5.0")
	req := httptest.NewRequest(http.MethodGet, "/brain", nil)
	rec := httptest.NewRecorder()
	h.Serve(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `type="importmap"`) {
		t.Fatal("index.html não contém o import map (`type=\"importmap\"`)")
	}
	if !strings.Contains(body, "three.module.js") {
		t.Fatal("import map não mapeia 'three' para three.module.js")
	}
	if !strings.Contains(body, `type="module"`) {
		t.Fatal("index.html não carrega app.js como type=module")
	}
}

// TestHandler_TraversalBloqueado garante proteção contra path traversal.
func TestHandler_TraversalBloqueado(t *testing.T) {
	h := NewHandler(testAgents(), testSkills(), "1.5.0")
	req := httptest.NewRequest(http.MethodGet, "/brain/../../etc/passwd", nil)
	rec := httptest.NewRecorder()
	h.Serve(rec, req)

	// Deve retornar 404 (NotFound) e não vazar outro arquivo.
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, esperado 404 (traversal deve ser bloqueado)", rec.Code)
	}
}

// TestHandler_NilSeguro garante que managers nil não causam pânico.
func TestHandler_NilSeguro(t *testing.T) {
	h := NewHandler(nil, nil, "1.5.0")
	req := httptest.NewRequest(http.MethodGet, "/brain/graph", nil)
	rec := httptest.NewRecorder()
	h.Graph(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, esperado 200", rec.Code)
	}
	var g Graph
	if err := json.Unmarshal(rec.Body.Bytes(), &g); err != nil {
		t.Fatalf("json inválido: %v", err)
	}
	// Mesmo vazio, o núcleo Don deve estar presente.
	found := false
	for _, n := range g.Nodes {
		if n.ID == "don" {
			found = true
		}
	}
	if !found {
		t.Fatal("grafo nil-safe sem núcleo Don")
	}
}
