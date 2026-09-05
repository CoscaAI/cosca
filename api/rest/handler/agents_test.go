package handler_test

// Tests for AgentsHandler (agents.go) with a real agents.Manager backed by a
// temporary directory containing department SKILL.md fixtures.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/api/rest/handler"
	"github.com/CoscaAI/cosca/internal/agents"
)

// writeAgentFixture writes a department SKILL.md at root/departments/dept/.
func writeAgentFixture(t *testing.T, root, dept, content string) string {
	t.Helper()
	dir := filepath.Join(root, "departments", dept)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to mkdir dept dir: %v", err)
	}
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write SKILL.md fixture: %v", err)
	}
	return path
}

// sampleAgentSkill returns a deterministic SKILL.md fixture that the agents
// parser resolves to the agent "OPS BOT" (department "ops").
func sampleAgentSkill() string {
	return `> **Version**: 1.2.0 | **Status**: active

# OPS BOT — Operations Specialist

## PURPOSE
Automates operational workflows with precision.

## RESPONSIBILITIES
1. Run scheduled checks
2. Escalate incidents
`
}

// newAgentsHandlerWithFixture creates an agents manager loaded from a temp dir
// containing one deterministic department fixture, plus the embedded Cosca
// agents.
func newAgentsHandlerWithFixture(t *testing.T) *handler.AgentsHandler {
	t.Helper()
	root := t.TempDir()
	writeAgentFixture(t, root, "ops", sampleAgentSkill())
	return handler.NewAgentsHandler(agents.NewManager(root))
}

// TestAgentsListWithManager verifies List returns 200 with the fixture agent
// present among the (embedded + local) results.
func TestAgentsListWithManager(t *testing.T) {
	h := newAgentsHandlerWithFixture(t)

	req := httptest.NewRequest("GET", "/v1/agents", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	var resp []agents.Agent
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(resp) == 0 {
		t.Fatal("expected non-empty agents list")
	}
	found := false
	for _, a := range resp {
		if a.Name == "OPS BOT" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected fixture agent 'OPS BOT' in list")
	}
}

// TestAgentsSearchMissingQuery verifies Search without ?q= returns 400.
func TestAgentsSearchMissingQuery(t *testing.T) {
	h := newAgentsHandlerWithFixture(t)

	req := httptest.NewRequest("GET", "/v1/agents/search", nil)
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAgentsSearchHit verifies Search returns the fixture agent.
func TestAgentsSearchHit(t *testing.T) {
	h := newAgentsHandlerWithFixture(t)

	req := httptest.NewRequest("GET", "/v1/agents/search?q=operational", nil)
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	var resp []agents.Agent
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(resp) == 0 {
		t.Fatal("expected at least one agent in search results")
	}
	found := false
	for _, a := range resp {
		if a.Name == "OPS BOT" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'OPS BOT' in search results")
	}
}

// TestAgentsSearchMiss verifies Search returns an empty array (not null) when
// nothing matches.
func TestAgentsSearchMiss(t *testing.T) {
	h := newAgentsHandlerWithFixture(t)

	req := httptest.NewRequest("GET", "/v1/agents/search?q=zzz-no-such-agent-xyz", nil)
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	if strings.TrimSpace(w.Body.String()) != "[]" {
		t.Errorf("expected empty array '[]', got %s", w.Body.String())
	}
}

// TestAgentsGetFound verifies Get returns the fixture agent with its parsed
// metadata.
func TestAgentsGetFound(t *testing.T) {
	h := newAgentsHandlerWithFixture(t)

	req := httptest.NewRequest("GET", "/v1/agents/OPS%20BOT", nil)
	req.SetPathValue("name", "OPS BOT")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	var resp agents.Agent
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Name != "OPS BOT" {
		t.Errorf("expected agent name 'OPS BOT', got %q", resp.Name)
	}
	if resp.Department != "ops" {
		t.Errorf("expected department 'ops', got %q", resp.Department)
	}
}

// TestAgentsGetCaseInsensitive verifies Get matches names case-insensitively.
func TestAgentsGetCaseInsensitive(t *testing.T) {
	h := newAgentsHandlerWithFixture(t)

	req := httptest.NewRequest("GET", "/v1/agents/ops%20bot", nil)
	req.SetPathValue("name", "ops bot")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAgentsGetNotFound verifies Get returns 404 for an unknown agent.
func TestAgentsGetNotFound(t *testing.T) {
	h := newAgentsHandlerWithFixture(t)

	req := httptest.NewRequest("GET", "/v1/agents/nonexistent", nil)
	req.SetPathValue("name", "nonexistent")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAgentsSearchNilManager verifies Search returns an empty result (200)
// with a nil manager.
func TestAgentsSearchNilManager(t *testing.T) {
	h := handler.NewAgentsHandler(nil)

	req := httptest.NewRequest("GET", "/v1/agents/search?q=anything", nil)
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
}
