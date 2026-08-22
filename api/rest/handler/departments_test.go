package handler_test

// Tests for DepartmentsHandler (departments.go) — inter-department
// conversations (Dept→Dept) over the append-only conversation ledger
// (internal/department). Covers List/Thread/Ask, the empty-store and
// unknown-thread (404 pt-BR) cases, department validation (400 pt-BR),
// nil-store 503, and the server route registration.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/api/rest"
	"github.com/CoscaAI/cosca/api/rest/handler"
	internalauth "github.com/CoscaAI/cosca/internal/auth"
	"github.com/CoscaAI/cosca/internal/department"
)

// newTestDeptStore creates a temporary SQLite conversation ledger for testing.
func newTestDeptStore(t *testing.T) *department.ConversationStore {
	t.Helper()
	s, err := department.NewConversationStore(filepath.Join(t.TempDir(), "department.db"))
	if err != nil {
		t.Fatalf("newTestDeptStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// seedDeptMessage appends a conversation message to the given store with a
// deterministic timestamp (ascending across calls within a test).
func seedDeptMessage(t *testing.T, s *department.ConversationStore, m department.ConversationMessage) string {
	t.Helper()
	if m.From == "" {
		m.From = "developer"
	}
	if m.To == "" {
		m.To = "security"
	}
	if m.Topic == "" {
		m.Topic = "release-approval"
	}
	if m.ThreadID == "" {
		m.ThreadID = department.ThreadKey(m.Topic, time.Unix(1754188800, 0))
	}
	if m.Kind == "" {
		m.Kind = "question"
	}
	id, err := s.Send(m)
	if err != nil {
		t.Fatalf("seedDeptMessage: %v", err)
	}
	return id
}

// newDeptRouteServer builds a real REST server (rest.New) with a department
// store and returns its mux, so the registered routes can be exercised
// end-to-end.
func newDeptRouteServer(t *testing.T, ds *department.ConversationStore) *http.ServeMux {
	t.Helper()
	userStore := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	srv := rest.New(nil, nil, nil, nil, nil, nil, nil,
		userStore, nil, nil, rest.DefaultConfig(), nil, nil, nil, nil, nil,
		false, nil, nil, ds, nil)
	return srv.Mux()
}

// =============================================================================
// List
// =============================================================================

func TestDepartmentsList_ReturnsKnownAndRecent(t *testing.T) {
	s := newTestDeptStore(t)
	h := handler.NewDepartmentsHandler(s)

	thread := department.ThreadKey("release-approval", time.Unix(1754188800, 0))
	seedDeptMessage(t, s, department.ConversationMessage{
		Message: "podemos liberar a versão?", Kind: "question", ThreadID: thread,
	})
	seedDeptMessage(t, s, department.ConversationMessage{
		From: "security", To: "developer", Message: "há 2 riscos críticos",
		Kind: "answer", ThreadID: thread,
	})

	req := httptest.NewRequest("GET", "/v1/departments", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Departments []string `json:"departments"`
		Recent      []struct {
			ID      string `json:"id"`
			From    string `json:"from"`
			To      string `json:"to"`
			Topic   string `json:"topic"`
			Message string `json:"message"`
			Kind    string `json:"kind"`
			Thread  string `json:"thread_id"`
		} `json:"recent"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Known departments static list must be present.
	want := []string{"developer", "security", "executive", "research", "operations", "kernel"}
	if len(resp.Departments) != len(want) {
		t.Fatalf("departments = %v, want %v", resp.Departments, want)
	}
	for i, d := range want {
		if resp.Departments[i] != d {
			t.Errorf("departments[%d] = %q, want %q", i, resp.Departments[i], d)
		}
	}

	// Most recent first (created_at DESC) — the answer is newer.
	if len(resp.Recent) != 2 {
		t.Fatalf("expected 2 recent messages, got %d", len(resp.Recent))
	}
	if resp.Recent[0].From != "security" || resp.Recent[0].Kind != "answer" {
		t.Errorf("most recent message mismatch: %+v", resp.Recent[0])
	}
	if resp.Recent[1].From != "developer" || resp.Recent[1].Kind != "question" {
		t.Errorf("older message mismatch: %+v", resp.Recent[1])
	}
}

func TestDepartmentsList_EmptyStore(t *testing.T) {
	s := newTestDeptStore(t)
	h := handler.NewDepartmentsHandler(s)

	req := httptest.NewRequest("GET", "/v1/departments", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Departments []string `json:"departments"`
		Recent      []string `json:"recent"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Recent) != 0 {
		t.Errorf("empty store must yield empty recent array, got %v", resp.Recent)
	}
	if len(resp.Departments) == 0 {
		t.Error("known departments list must always be present")
	}
}

func TestDepartmentsList_NilStore(t *testing.T) {
	h := handler.NewDepartmentsHandler(nil)

	req := httptest.NewRequest("GET", "/v1/departments", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
	if !jsonHasError(w.Body.Bytes()) {
		t.Errorf("expected pt-BR error message, got %s", w.Body.String())
	}
}

// =============================================================================
// Thread
// =============================================================================

func TestDepartmentsThread_OrderedAuditTrail(t *testing.T) {
	s := newTestDeptStore(t)
	h := handler.NewDepartmentsHandler(s)

	thread := department.ThreadKey("release-approval", time.Unix(1754188800, 0))
	// Seed out-of-order timestamps; the ledger must return them ASC.
	seedDeptMessage(t, s, department.ConversationMessage{
		Message: "validação passou", Kind: "answer", ThreadID: thread,
		CreatedAt: time.Unix(1754188900, 0).UTC(),
	})
	seedDeptMessage(t, s, department.ConversationMessage{
		Message: "podemos liberar a versão?", Kind: "question", ThreadID: thread,
		CreatedAt: time.Unix(1754188800, 0).UTC(),
	})
	seedDeptMessage(t, s, department.ConversationMessage{
		Message: "release aprovado", Kind: "approval", ThreadID: thread,
		CreatedAt: time.Unix(1754189000, 0).UTC(),
	})

	req := httptest.NewRequest("GET", "/v1/departments/threads/"+thread, nil)
	req.SetPathValue("thread", thread)
	w := httptest.NewRecorder()
	h.Thread(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		ThreadID string `json:"thread_id"`
		Messages []struct {
			Message string `json:"message"`
			Kind    string `json:"kind"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.ThreadID != thread {
		t.Errorf("thread_id = %q, want %q", resp.ThreadID, thread)
	}
	if len(resp.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(resp.Messages))
	}
	// Ordered by created_at ASC — question, answer, approval.
	if resp.Messages[0].Kind != "question" || resp.Messages[0].Message != "podemos liberar a versão?" {
		t.Errorf("first message mismatch: %+v", resp.Messages[0])
	}
	if resp.Messages[1].Kind != "answer" {
		t.Errorf("second message mismatch: %+v", resp.Messages[1])
	}
	if resp.Messages[2].Kind != "approval" || resp.Messages[2].Message != "release aprovado" {
		t.Errorf("last message mismatch: %+v", resp.Messages[2])
	}
}

func TestDepartmentsThread_Unknown(t *testing.T) {
	s := newTestDeptStore(t)
	h := handler.NewDepartmentsHandler(s)

	req := httptest.NewRequest("GET", "/v1/departments/threads/ghost:20260802", nil)
	req.SetPathValue("thread", "ghost:20260802")
	w := httptest.NewRecorder()
	h.Thread(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	if !jsonHasError(w.Body.Bytes()) {
		t.Errorf("expected pt-BR error message, got %s", w.Body.String())
	}
}

func TestDepartmentsThread_NilStore(t *testing.T) {
	h := handler.NewDepartmentsHandler(nil)

	req := httptest.NewRequest("GET", "/v1/departments/threads/x:y", nil)
	req.SetPathValue("thread", "x:y")
	w := httptest.NewRecorder()
	h.Thread(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

// =============================================================================
// Ask
// =============================================================================

func TestDepartmentsAsk_CreatesThread(t *testing.T) {
	s := newTestDeptStore(t)
	h := handler.NewDepartmentsHandler(s)

	body := `{"from":"developer","to":"security","topic":"release-approval","message":"podemos liberar a versão?"}`
	req := httptest.NewRequest("POST", "/v1/departments/ask", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Ask(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		MessageID string `json:"message_id"`
		ThreadID  string `json:"thread_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.MessageID == "" {
		t.Error("expected a message_id (DM-YYYYMMDD-XXXX)")
	}
	if resp.ThreadID == "" {
		t.Error("expected a thread_id")
	}

	// The question must be persisted in the ledger, retrievable as the
	// thread's audit trail.
	msgs, err := s.Thread(resp.ThreadID)
	if err != nil {
		t.Fatalf("store.Thread: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message in new thread, got %d", len(msgs))
	}
	if msgs[0].From != "developer" || msgs[0].To != "security" || msgs[0].Kind != "question" {
		t.Errorf("persisted message mismatch: %+v", msgs[0])
	}
}

func TestDepartmentsAsk_InvalidDepartment(t *testing.T) {
	s := newTestDeptStore(t)
	h := handler.NewDepartmentsHandler(s)

	body := `{"from":"developer","to":"nao-existe","topic":"release-approval","message":"ola?"}`
	req := httptest.NewRequest("POST", "/v1/departments/ask", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Ask(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !jsonHasError(w.Body.Bytes()) {
		t.Errorf("expected pt-BR error message, got %s", w.Body.String())
	}
}

func TestDepartmentsAsk_InvalidBody(t *testing.T) {
	s := newTestDeptStore(t)
	h := handler.NewDepartmentsHandler(s)

	req := httptest.NewRequest("POST", "/v1/departments/ask", bytes.NewReader([]byte(`not-json`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Ask(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDepartmentsAsk_NilStore(t *testing.T) {
	h := handler.NewDepartmentsHandler(nil)

	body := `{"from":"developer","to":"security","topic":"t","message":"m"}`
	req := httptest.NewRequest("POST", "/v1/departments/ask", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Ask(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
	if !jsonHasError(w.Body.Bytes()) {
		t.Errorf("expected pt-BR error message, got %s", w.Body.String())
	}
}

// =============================================================================
// Route registration (server.go wiring)
// =============================================================================

func TestDepartmentRoutes_Registered(t *testing.T) {
	s := newTestDeptStore(t)
	mux := newDeptRouteServer(t, s)

	thread := department.ThreadKey("release-approval", time.Unix(1754188800, 0))
	seedDeptMessage(t, s, department.ConversationMessage{
		Message: "podemos liberar a versão?", Kind: "question", ThreadID: thread,
	})

	// List/Thread are public-read (no role gate) — the mux serves them even
	// without claims context.
	t.Run("list", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/departments", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET /v1/departments = %d, want 200: %s", w.Code, w.Body.String())
		}
	})

	t.Run("thread", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/departments/threads/"+thread, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET /v1/departments/threads/%s = %d, want 200: %s", thread, w.Code, w.Body.String())
		}
	})

	// Ask is a write — editor+ gate.
	t.Run("ask editor", func(t *testing.T) {
		body := `{"from":"security","to":"developer","topic":"release-approval","message":"há 2 riscos críticos"}`
		req := httptest.NewRequest("POST", "/v1/departments/ask", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(editorClaimsContext())
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("POST /v1/departments/ask (editor) = %d, want 201: %s", w.Code, w.Body.String())
		}
	})

	t.Run("ask unauthorized", func(t *testing.T) {
		body := `{"from":"developer","to":"security","topic":"t","message":"m"}`
		req := httptest.NewRequest("POST", "/v1/departments/ask", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("POST /v1/departments/ask (no claims) = %d, want 401: %s", w.Code, w.Body.String())
		}
	})
}

func TestDepartmentRoutes_NilStore_ServiceUnavailable(t *testing.T) {
	mux := newDeptRouteServer(t, nil)

	req := httptest.NewRequest("GET", "/v1/departments", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /v1/departments with nil store = %d, want 503: %s", w.Code, w.Body.String())
	}
	if !jsonHasError(w.Body.Bytes()) {
		t.Errorf("expected pt-BR error message, got %s", w.Body.String())
	}
}
