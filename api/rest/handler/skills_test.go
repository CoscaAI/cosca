package handler_test

// Tests for SkillsHandler (skills.go) with a real skills.Manager backed by a
// temporary directory containing markdown skill fixtures.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/api/rest/handler"
	"github.com/CoscaAI/cosca/internal/audit"
	"github.com/CoscaAI/cosca/internal/skills"
)

// writeSkillFixture writes a skill markdown file at root/skills/{name}.md.
func writeSkillFixture(t *testing.T, root, name string, content string) string {
	t.Helper()
	dir := filepath.Join(root, "skills")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to mkdir skills dir: %v", err)
	}
	path := filepath.Join(dir, name+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write skill fixture: %v", err)
	}
	return path
}

// sampleSkillContent is a YAML-frontmatter skill fixture.
func sampleSkillContent() string {
	return `---
name: greeting
description: Sends friendly salutations to users
category: communication
---
# Greeting

Warm and professional greeting instructions.
`
}

// newSkillsHandlerWithFixture creates a skills manager loaded from a temp dir
// that contains one deterministic skill fixture, plus the embedded Cosca
// skills. It returns the handler, an optional audit store, and the fixture
// root directory (used to assert on persisted installs).
func newSkillsHandlerWithFixture(t *testing.T, auditEnabled bool) (*handler.SkillsHandler, *audit.Store, string) {
	t.Helper()
	root := t.TempDir()
	writeSkillFixture(t, root, "greeting", sampleSkillContent())
	mgr := skills.NewManager(root)
	var auditStore *audit.Store
	if auditEnabled {
		auditStore = newTestAuditStore(t)
	}
	return handler.NewSkillsHandler(mgr, auditStore), auditStore, root
}

// TestSkillsListWithManager verifies List returns 200 with the fixture skill
// present among the (embedded + local) results.
func TestSkillsListWithManager(t *testing.T) {
	h, _, _ := newSkillsHandlerWithFixture(t, false)

	req := httptest.NewRequest("GET", "/v1/skills", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	var resp []skills.Skill
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(resp) == 0 {
		t.Fatal("expected non-empty skills list")
	}
	found := false
	for _, s := range resp {
		if s.Name == "greeting" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected fixture skill 'greeting' in list")
	}
}

// TestSkillsSearchMissingQuery verifies Search without ?q= returns 400.
func TestSkillsSearchMissingQuery(t *testing.T) {
	h, _, _ := newSkillsHandlerWithFixture(t, false)

	req := httptest.NewRequest("GET", "/v1/skills/search", nil)
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if !strings.Contains(resp["error"], "q") {
		t.Errorf("expected error mentioning query param 'q', got %q", resp["error"])
	}
}

// TestSkillsSearchHit verifies Search returns the matching fixture skill.
func TestSkillsSearchHit(t *testing.T) {
	h, _, _ := newSkillsHandlerWithFixture(t, false)

	req := httptest.NewRequest("GET", "/v1/skills/search?q=salutations", nil)
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	var resp []skills.Skill
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(resp) == 0 {
		t.Fatal("expected at least one search hit")
	}
	found := false
	for _, s := range resp {
		if s.Name == "greeting" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'greeting' skill in search results")
	}
}

// TestSkillsSearchMiss verifies Search returns an empty array (not null) when
// nothing matches.
func TestSkillsSearchMiss(t *testing.T) {
	h, _, _ := newSkillsHandlerWithFixture(t, false)

	req := httptest.NewRequest("GET", "/v1/skills/search?q=zzz-no-such-skill-xyz", nil)
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	if strings.TrimSpace(w.Body.String()) != "[]" {
		t.Errorf("expected empty array '[]', got %s", w.Body.String())
	}
}

// TestSkillsGetFound verifies Get returns the fixture skill.
func TestSkillsGetFound(t *testing.T) {
	h, _, _ := newSkillsHandlerWithFixture(t, false)

	req := httptest.NewRequest("GET", "/v1/skills/greeting", nil)
	req.SetPathValue("name", "greeting")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	var resp skills.Skill
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Name != "greeting" {
		t.Errorf("expected skill name 'greeting', got %q", resp.Name)
	}
}

// TestSkillsGetCaseInsensitive verifies Get matches names case-insensitively.
func TestSkillsGetCaseInsensitive(t *testing.T) {
	h, _, _ := newSkillsHandlerWithFixture(t, false)

	req := httptest.NewRequest("GET", "/v1/skills/GREETING", nil)
	req.SetPathValue("name", "GREETING")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSkillsGetNotFound verifies Get returns 404 for an unknown skill.
func TestSkillsGetNotFound(t *testing.T) {
	h, _, _ := newSkillsHandlerWithFixture(t, false)

	req := httptest.NewRequest("GET", "/v1/skills/nonexistent", nil)
	req.SetPathValue("name", "nonexistent")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSkillsInstallSuccess verifies Install returns 201, records a success
// audit event, and persists the skill file into the manager's skills dir.
func TestSkillsInstallSuccess(t *testing.T) {
	h, auditStore, root := newSkillsHandlerWithFixture(t, true)

	// The source must live inside the manager's skills directory
	// (containment hardening): write it into <root>/skills.
	source := filepath.Join(root, "skills", "source.md")
	if err := os.WriteFile(source, []byte("# Installed Skill\n\nDoes useful things.\n"), 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// JSON montado com json.Marshal: no Windows o source contém backslashes
	// (ex: C:\Users\...\source.md) e a concatenação manual produzia JSON inválido
	// ("invalid character 'U' in string escape code" → 400).
	payload, err := json.Marshal(map[string]any{"source": source})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest("POST", "/v1/skills/installed/install", bytes.NewReader(payload))
	req.SetPathValue("name", "installed")
	w := httptest.NewRecorder()
	h.Install(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}
	var resp skills.Skill
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Name != "installed" {
		t.Errorf("expected installed skill name 'installed', got %q", resp.Name)
	}

	// The skill must be persisted to <root>/skills/installed.md.
	persistedPath := filepath.Join(root, "skills", "installed.md")
	if _, err := os.Stat(persistedPath); err != nil {
		t.Errorf("expected persisted skill file at %s: %v", persistedPath, err)
	}

	// Verify the success audit event was recorded.
	entries, total, err := auditStore.List(10, 0, auditFiltersFor("skill.install"))
	if err != nil {
		t.Fatalf("failed to list audit entries: %v", err)
	}
	if total != 1 || len(entries) != 1 {
		t.Fatalf("expected 1 audit entry, got total=%d len=%d", total, len(entries))
	}
	if entries[0].Status != "success" {
		t.Errorf("expected status 'success', got %q", entries[0].Status)
	}
}

// TestSkillsInstallMissingFile verifies Install returns 500 and records an
// error audit event when the source file does not exist.
func TestSkillsInstallMissingFile(t *testing.T) {
	h, auditStore, _ := newSkillsHandlerWithFixture(t, true)

	body := `{"source":"/nonexistent/skill-file-xyz.md"}`
	req := httptest.NewRequest("POST", "/v1/skills/ghost/install", strings.NewReader(body))
	req.SetPathValue("name", "ghost")
	w := httptest.NewRecorder()
	h.Install(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 Internal Server Error, got %d: %s", w.Code, w.Body.String())
	}

	entries, total, err := auditStore.List(10, 0, auditFiltersFor("skill.install"))
	if err != nil {
		t.Fatalf("failed to list audit entries: %v", err)
	}
	if total != 1 || len(entries) != 1 {
		t.Fatalf("expected 1 audit entry, got total=%d len=%d", total, len(entries))
	}
	if entries[0].Status != "error" {
		t.Errorf("expected status 'error', got %q", entries[0].Status)
	}
}

// TestSkillsInstallInvalidJSONBody verifies Install returns 400 for a
// malformed request body.
func TestSkillsInstallInvalidJSONBody(t *testing.T) {
	h, _, _ := newSkillsHandlerWithFixture(t, false)

	req := httptest.NewRequest("POST", "/v1/skills/x/install", strings.NewReader("not-json"))
	req.SetPathValue("name", "x")
	w := httptest.NewRecorder()
	h.Install(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSkillsNilManagerGet verifies Get returns 503 with a nil manager.
func TestSkillsNilManagerGet(t *testing.T) {
	h := handler.NewSkillsHandler(nil, nil)

	req := httptest.NewRequest("GET", "/v1/skills/x", nil)
	req.SetPathValue("name", "x")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable, got %d: %s", w.Code, w.Body.String())
	}
}
