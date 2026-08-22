package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CoscaAI/cosca/api/rest/handler"
	"github.com/CoscaAI/cosca/internal/agents"
	"github.com/CoscaAI/cosca/internal/providers"
	"github.com/CoscaAI/cosca/internal/runtime"
	"github.com/CoscaAI/cosca/internal/skills"
	"github.com/CoscaAI/cosca/internal/workflows"
)

// TestStatsAllNil verifies the stats endpoint returns zero counts when
// all managers and runtime are nil.
func TestStatsAllNil(t *testing.T) {
	h := handler.NewStatsHandler(nil, nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/v1/stats", nil)
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var stats handler.PlatformStats
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if stats.Agents != 0 {
		t.Errorf("expected 0 agents with nil manager, got %d", stats.Agents)
	}
	if stats.Skills != 0 {
		t.Errorf("expected 0 skills with nil manager, got %d", stats.Skills)
	}
	if stats.Providers.Total != 0 {
		t.Errorf("expected 0 providers with nil manager, got %d", stats.Providers.Total)
	}
	if stats.Workflows != 0 {
		t.Errorf("expected 0 workflows with nil manager, got %d", stats.Workflows)
	}
}

// TestStatsWithAgents verifies agent count is included and that added
// agents are reflected in the count. Note: agents.NewManager("") loads
// embedded Cosca framework agents, so the absolute count is base + added.
func TestStatsWithAgents(t *testing.T) {
	agentsMgr := agents.NewManager("")
	baseCount := len(agentsMgr.List())

	// Add extra test agents.
	agentsMgr.Add(agents.Agent{Name: "custom-agent-1", Role: "tester", Status: "active", Version: "1.0"})
	agentsMgr.Add(agents.Agent{Name: "custom-agent-2", Role: "builder", Status: "active", Version: "1.0"})
	agentsMgr.Add(agents.Agent{Name: "custom-agent-3", Role: "runner", Status: "inactive", Version: "2.0"})

	h := handler.NewStatsHandler(agentsMgr, nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/v1/stats", nil)
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var stats handler.PlatformStats
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	expected := baseCount + 3
	if stats.Agents != expected {
		t.Errorf("expected %d agents (base %d + 3 custom), got %d", expected, baseCount, stats.Agents)
	}
	if stats.Skills != 0 {
		t.Errorf("expected 0 skills, got %d", stats.Skills)
	}
}

// TestStatsWithSkills verifies skill count is included and that added
// skills are reflected in the count.
func TestStatsWithSkills(t *testing.T) {
	skillsMgr := skills.NewManager("")
	baseCount := len(skillsMgr.List())

	skillsMgr.Add(skills.Skill{Name: "custom-skill-a", Description: "First custom skill"})
	skillsMgr.Add(skills.Skill{Name: "custom-skill-b", Description: "Second custom skill"})

	h := handler.NewStatsHandler(nil, skillsMgr, nil, nil, nil)

	req := httptest.NewRequest("GET", "/v1/stats", nil)
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var stats handler.PlatformStats
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	expected := baseCount + 2
	if stats.Skills != expected {
		t.Errorf("expected %d skills (base %d + 2 custom), got %d", expected, baseCount, stats.Skills)
	}
}

// TestStatsWithProviders verifies provider information is correct.
func TestStatsWithProviders(t *testing.T) {
	providersMgr := providers.NewManager()
	h := handler.NewStatsHandler(nil, nil, providersMgr, nil, nil)

	req := httptest.NewRequest("GET", "/v1/stats", nil)
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var stats handler.PlatformStats
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// The default providers manager has built-in providers.
	if stats.Providers.Total == 0 {
		t.Error("expected non-zero provider count")
	}
}

// TestStatsWithWorkflows verifies workflow count is included.
func TestStatsWithWorkflows(t *testing.T) {
	workflowsMgr := workflows.NewManager("")
	baseCount := len(workflowsMgr.List())

	workflowsMgr.Add(workflows.Workflow{Name: "custom-wf-one", Description: "First custom workflow"})
	workflowsMgr.Add(workflows.Workflow{Name: "custom-wf-two", Description: "Second custom workflow"})

	h := handler.NewStatsHandler(nil, nil, nil, workflowsMgr, nil)

	req := httptest.NewRequest("GET", "/v1/stats", nil)
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var stats handler.PlatformStats
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	expected := baseCount + 2
	if stats.Workflows != expected {
		t.Errorf("expected %d workflows (base %d + 2 custom), got %d", expected, baseCount, stats.Workflows)
	}
}

// TestStatsWithRuntime verifies runtime info is included when available.
func TestStatsWithRuntime(t *testing.T) {
	rt := runtime.New(runtime.WithConfig(runtime.RuntimeConfig{
		Name:    "cosca-test",
		Version: "1.2.3",
	}))

	h := handler.NewStatsHandler(nil, nil, nil, nil, rt)

	req := httptest.NewRequest("GET", "/v1/stats", nil)
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var stats handler.PlatformStats
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if stats.Version != "1.2.3" {
		t.Errorf("expected version '1.2.3', got '%s'", stats.Version)
	}
	if stats.Health == "" {
		t.Error("expected non-empty health status")
	}
	if stats.Uptime == "" {
		t.Error("expected non-empty uptime")
	}
}

// TestStatsFullAggregation verifies the full aggregation with all managers.
func TestStatsFullAggregation(t *testing.T) {
	agentsMgr := agents.NewManager("")
	skillsMgr := skills.NewManager("")
	providersMgr := providers.NewManager()
	workflowsMgr := workflows.NewManager("")

	baseAgents := len(agentsMgr.List())
	baseSkills := len(skillsMgr.List())
	baseWorkflows := len(workflowsMgr.List())

	agentsMgr.Add(agents.Agent{Name: "custom-agg-agent", Role: "tester", Status: "active", Version: "1.0"})
	skillsMgr.Add(skills.Skill{Name: "custom-agg-skill", Description: "A custom skill for aggregation test"})
	workflowsMgr.Add(workflows.Workflow{Name: "custom-agg-wf", Description: "A custom workflow for aggregation test"})

	rt := runtime.New(runtime.WithConfig(runtime.RuntimeConfig{
		Name:    "cosca",
		Version: "9.9.9",
	}))

	h := handler.NewStatsHandler(agentsMgr, skillsMgr, providersMgr, workflowsMgr, rt)

	req := httptest.NewRequest("GET", "/v1/stats", nil)
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var stats handler.PlatformStats
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify all fields — accounts for embedded base items.
	if stats.Agents != baseAgents+1 {
		t.Errorf("expected %d agents (base %d + 1 custom), got %d", baseAgents+1, baseAgents, stats.Agents)
	}
	if stats.Skills != baseSkills+1 {
		t.Errorf("expected %d skills (base %d + 1 custom), got %d", baseSkills+1, baseSkills, stats.Skills)
	}
	if stats.Providers.Total == 0 {
		t.Error("expected non-zero total providers")
	}
	if stats.Workflows != baseWorkflows+1 {
		t.Errorf("expected %d workflows (base %d + 1 custom), got %d", baseWorkflows+1, baseWorkflows, stats.Workflows)
	}
	if stats.Version != "9.9.9" {
		t.Errorf("expected version '9.9.9', got '%s'", stats.Version)
	}
	if stats.Health == "" {
		t.Error("expected non-empty health status")
	}
	if stats.Uptime == "" {
		t.Error("expected non-empty uptime")
	}
}

// TestStatsResponseFormat verifies the JSON response structure has all
// expected top-level keys.
func TestStatsResponseFormat(t *testing.T) {
	h := handler.NewStatsHandler(nil, nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/v1/stats", nil)
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	// Verify content type.
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
	}

	// Verify structure.
	var raw map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	requiredKeys := []string{"agents", "skills", "providers", "workflows"}
	for _, key := range requiredKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected key '%s' in response", key)
		}
	}

	// Verify providers is an object.
	providers, ok := raw["providers"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'providers' to be an object")
	}
	if _, ok := providers["total"]; !ok {
		t.Error("expected 'total' in providers object")
	}
}

// TestStatsNilAgentManager verifies that a nil agents manager returns 0.
func TestStatsNilAgentManager(t *testing.T) {
	skillsMgr := skills.NewManager("")
	h := handler.NewStatsHandler(nil, skillsMgr, nil, nil, nil)

	req := httptest.NewRequest("GET", "/v1/stats", nil)
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var stats handler.PlatformStats
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if stats.Agents != 0 {
		t.Errorf("expected 0 agents with nil manager, got %d", stats.Agents)
	}
	// Skills should still come from the non-nil manager.
	if stats.Skills == 0 {
		t.Error("expected non-zero skills from non-nil skills manager")
	}
}

// TestStatsNoProvidersActive verifies the providers stats shape even
// when no active provider is set.
func TestStatsNoProvidersActive(t *testing.T) {
	providersMgr := providers.NewManager()
	h := handler.NewStatsHandler(nil, nil, providersMgr, nil, nil)

	req := httptest.NewRequest("GET", "/v1/stats", nil)
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var stats handler.PlatformStats
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if stats.Providers.Total == 0 {
		t.Error("expected non-zero total providers")
	}
	// Active can be empty when nothing is selected.
	// This is fine — the field exists but may be empty.
	t.Logf("providers total=%d active='%s'", stats.Providers.Total, stats.Providers.Active)
}
