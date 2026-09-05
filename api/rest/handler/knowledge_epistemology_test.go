package handler_test

// Tests for the epistemologia REST endpoints (knowledge.go):
// GET /v1/knowledge/epistemology (resumo) and
// GET /v1/knowledge/epistemology/{status} (itens por estado).
//
// Covers: counts + problematic list from a seeded laws.json, items per status,
// invalid status → 400 pt-BR, empty store → counts 0 without 500, corrupted
// store → graceful degrade, and route registration through the real mux.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/api/rest"
	"github.com/CoscaAI/cosca/api/rest/handler"
	internalauth "github.com/CoscaAI/cosca/internal/auth"
)

// epistemologySampleLawsJSON é um laws.json representativo com todos os 6
// estados epistemológicos (1 de cada), para o resumo e o detalhe por status.
const epistemologySampleLawsJSON = `{
  "version": 1,
  "items": [
    {
      "id": "K-01",
      "title": "Nunca executar como root automaticamente",
      "level": "learning",
      "evidence": [{"id":"F001","kind":"incident","source":"F001","description":"fuga","timestamp":"2026-07-30T09:15:00Z"}],
      "projects": 1,
      "confidence": 0.95,
      "rollbacks": 0,
      "created_at": "2026-07-30T09:15:00Z",
      "status": "SUPPORTED",
      "last_verified": "2026-07-30T12:00:00Z",
      "verification_count": 3,
      "contradiction_count": 0
    },
    {
      "id": "K-02",
      "title": "Registro público desabilitado por padrão",
      "level": "law",
      "evidence": [{"id":"ev-1","kind":"audit","source":"red-team-A2","description":"público","timestamp":"2026-07-31T10:10:00Z"}],
      "projects": 1,
      "confidence": 0.99,
      "rollbacks": 0,
      "created_at": "2026-07-31T10:10:00Z",
      "status": "KNOWN",
      "last_verified": "2026-07-31T11:00:00Z",
      "verification_count": 12,
      "contradiction_count": 0
    },
    {
      "id": "K-03",
      "title": "API X envelheceu",
      "level": "law",
      "evidence": [{"id":"ev-2","kind":"project","source":"projeto-alpha","description":"release nova mudou o hash","timestamp":"2026-06-01T08:00:00Z","path":"api/x.go","sha256":"abc123","retrieved":"2026-06-01T08:00:00Z"}],
      "projects": 1,
      "confidence": 0.99,
      "rollbacks": 0,
      "created_at": "2026-05-01T08:00:00Z",
      "status": "STALE",
      "last_verified": "2026-06-01T08:00:00Z",
      "verification_count": 4,
      "contradiction_count": 1
    },
    {
      "id": "K-04",
      "title": "Fonte divergente",
      "level": "learning",
      "evidence": [{"id":"ev-3","kind":"test","source":"test/","description":"conflito aberto","timestamp":"2026-07-31T14:30:00Z"}],
      "projects": 1,
      "confidence": 0.80,
      "rollbacks": 0,
      "created_at": "2026-07-31T14:30:00Z",
      "status": "CONFLICTING",
      "verification_count": 2,
      "contradiction_count": 2
    },
    {
      "id": "K-05",
      "title": "Observação solta",
      "evidence": [{"id":"ev-4","kind":"test","source":"test/","description":"primeira","timestamp":"2026-07-31T15:00:00Z"}],
      "confidence": 0.10,
      "created_at": "2026-07-31T15:00:00Z",
      "status": "UNKNOWN",
      "verification_count": 0,
      "contradiction_count": 0
    },
    {
      "id": "K-06",
      "title": "Hipótese incerta",
      "level": "learning",
      "evidence": [{"id":"ev-5","kind":"test","source":"test/","description":"fraca","timestamp":"2026-07-30T16:00:00Z"}],
      "projects": 1,
      "confidence": 0.40,
      "rollbacks": 0,
      "created_at": "2026-07-30T16:00:00Z",
      "status": "UNCERTAIN",
      "verification_count": 1,
      "contradiction_count": 0
    }
  ]
}`

// writeEpistemologyLaws escreve o laws.json dado sob uma árvore temp e
// devolve o caminho absoluto do arquivo (dir/.cosca/knowledge/laws.json).
func writeEpistemologyLaws(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".cosca", "knowledge", "laws.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir laws dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write laws.json: %v", err)
	}
	return path
}

// newEpistemologyHandler devolve um KnowledgeHandler apontando para um
// laws.json seedado com o conteúdo dado.
func newEpistemologyHandler(t *testing.T, content string) *handler.KnowledgeHandler {
	t.Helper()
	h := handler.NewKnowledgeHandler(nil, nil)
	h.SetLawsPath(writeEpistemologyLaws(t, content))
	return h
}

// chdir muda o diretório de trabalho atual e o restaura no fim do teste.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatal(err)
		}
	})
}

// =============================================================================
// Epistemology (resumo)
// =============================================================================

func TestEpistemology_Summary(t *testing.T) {
	h := newEpistemologyHandler(t, epistemologySampleLawsJSON)

	req := httptest.NewRequest("GET", "/v1/knowledge/epistemology", nil)
	w := httptest.NewRecorder()
	h.Epistemology(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Total       int            `json:"total"`
		ByStatus    map[string]int `json:"by_status"`
		Problematic []struct {
			ID                string `json:"id"`
			Title             string `json:"title"`
			Status            string `json:"status"`
			StatusDescription string `json:"status_description"`
		} `json:"problematic"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Total != 6 {
		t.Errorf("total = %d, want 6", resp.Total)
	}

	wantCounts := map[string]int{
		"KNOWN":       1,
		"SUPPORTED":   1,
		"UNCERTAIN":   1,
		"CONFLICTING": 1,
		"UNKNOWN":     1,
		"STALE":       1,
	}
	for s, want := range wantCounts {
		if resp.ByStatus[s] != want {
			t.Errorf("by_status[%s] = %d, want %d", s, resp.ByStatus[s], want)
		}
	}

	if len(resp.Problematic) != 3 {
		t.Fatalf("problematic len = %d, want 3 (CONFLICTING/UNKNOWN/STALE)", len(resp.Problematic))
	}

	byID := map[string]string{}
	byStatus := map[string]bool{}
	for _, p := range resp.Problematic {
		if p.ID == "" || p.Status == "" || p.StatusDescription == "" {
			t.Errorf("problematic item missing fields: %+v", p)
		}
		byID[p.ID] = p.Status
		byStatus[p.Status] = true
	}
	if byID["K-03"] != "STALE" || byID["K-04"] != "CONFLICTING" || byID["K-05"] != "UNKNOWN" {
		t.Errorf("problematic IDs/status mismatch: %v", byID)
	}
	if !byStatus["CONFLICTING"] || !byStatus["UNKNOWN"] || !byStatus["STALE"] {
		t.Errorf("problematic must include CONFLICTING/UNKNOWN/STALE, got %v", byStatus)
	}
}

func TestEpistemology_ProblematicDescription(t *testing.T) {
	h := newEpistemologyHandler(t, epistemologySampleLawsJSON)

	req := httptest.NewRequest("GET", "/v1/knowledge/epistemology", nil)
	w := httptest.NewRecorder()
	h.Epistemology(w, req)

	var resp struct {
		Problematic []struct {
			ID                string `json:"id"`
			Status            string `json:"status"`
			StatusDescription string `json:"status_description"`
		} `json:"problematic"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, p := range resp.Problematic {
		if p.ID == "K-03" && p.StatusDescription != "expirado — requer revalidação" {
			t.Errorf("K-03 description = %q, want pt-BR STALE description", p.StatusDescription)
		}
	}
}

func TestEpistemology_EmptyStore(t *testing.T) {
	h := newEpistemologyHandler(t, `{"version": 1, "items": []}`)

	req := httptest.NewRequest("GET", "/v1/knowledge/epistemology", nil)
	w := httptest.NewRecorder()
	h.Epistemology(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Total       int            `json:"total"`
		ByStatus    map[string]int `json:"by_status"`
		Problematic []interface{}  `json:"problematic"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Total != 0 {
		t.Errorf("total = %d, want 0", resp.Total)
	}
	for _, s := range []string{"KNOWN", "SUPPORTED", "UNCERTAIN", "CONFLICTING", "UNKNOWN", "STALE"} {
		if resp.ByStatus[s] != 0 {
			t.Errorf("by_status[%s] = %d, want 0", s, resp.ByStatus[s])
		}
	}
	if resp.Problematic == nil || len(resp.Problematic) != 0 {
		t.Errorf("problematic = %v, want empty array", resp.Problematic)
	}
}

func TestEpistemology_CorruptedStore_No500(t *testing.T) {
	// Arquivo corrompido: o resumo deve degradar para contagens 0, nunca 500.
	h := newEpistemologyHandler(t, `not-json-{`)

	req := httptest.NewRequest("GET", "/v1/knowledge/epistemology", nil)
	w := httptest.NewRecorder()
	h.Epistemology(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (graceful degrade), got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Total != 0 {
		t.Errorf("total = %d, want 0", resp.Total)
	}
}

func TestEpistemology_MissingStore_No500(t *testing.T) {
	// Caminho inexistente (nenhum laws.json): mesma degradação — 200, total 0.
	h := handler.NewKnowledgeHandler(nil, nil)
	h.SetLawsPath(filepath.Join(t.TempDir(), "nao-existe", "laws.json"))

	req := httptest.NewRequest("GET", "/v1/knowledge/epistemology", nil)
	w := httptest.NewRecorder()
	h.Epistemology(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Total != 0 {
		t.Errorf("total = %d, want 0", resp.Total)
	}
}

// =============================================================================
// EpistemologyByStatus (itens por estado)
// =============================================================================

func TestEpistemologyByStatus_Items(t *testing.T) {
	h := newEpistemologyHandler(t, epistemologySampleLawsJSON)

	cases := []struct {
		status string
		wantID string
	}{
		{"STALE", "K-03"},
		{"CONFLICTING", "K-04"},
		{"UNKNOWN", "K-05"},
		{"KNOWN", "K-02"},
	}
	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/knowledge/epistemology/"+tc.status, nil)
			req.SetPathValue("status", tc.status)
			w := httptest.NewRecorder()
			h.EpistemologyByStatus(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
			}

			var resp struct {
				Status string `json:"status"`
				Items  []struct {
					ID                 string `json:"id"`
					Title              string `json:"title"`
					Status             string `json:"status"`
					LastVerified       string `json:"last_verified"`
					VerificationCount  int    `json:"verification_count"`
					ContradictionCount int    `json:"contradiction_count"`
				} `json:"items"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if resp.Status != tc.status {
				t.Errorf("status = %q, want %q", resp.Status, tc.status)
			}
			if len(resp.Items) != 1 {
				t.Fatalf("items len = %d, want 1", len(resp.Items))
			}
			it := resp.Items[0]
			if it.ID != tc.wantID || it.Status != tc.status {
				t.Errorf("item = %+v, want ID %s with status %s", it, tc.wantID, tc.status)
			}
		})
	}
}

func TestEpistemologyByStatus_StaleFullShape(t *testing.T) {
	h := newEpistemologyHandler(t, epistemologySampleLawsJSON)

	req := httptest.NewRequest("GET", "/v1/knowledge/epistemology/STALE", nil)
	req.SetPathValue("status", "STALE")
	w := httptest.NewRecorder()
	h.EpistemologyByStatus(w, req)

	var resp struct {
		Items []struct {
			ID                 string `json:"id"`
			LastVerified       string `json:"last_verified"`
			VerificationCount  int    `json:"verification_count"`
			ContradictionCount int    `json:"contradiction_count"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(resp.Items))
	}
	it := resp.Items[0]
	if it.ID != "K-03" {
		t.Errorf("id = %q, want K-03", it.ID)
	}
	if it.LastVerified != "2026-06-01T08:00:00Z" {
		t.Errorf("last_verified = %q, want 2026-06-01T08:00:00Z", it.LastVerified)
	}
	if it.VerificationCount != 4 {
		t.Errorf("verification_count = %d, want 4", it.VerificationCount)
	}
	if it.ContradictionCount != 1 {
		t.Errorf("contradiction_count = %d, want 1", it.ContradictionCount)
	}
}

func TestEpistemologyByStatus_InvalidStatus(t *testing.T) {
	h := newEpistemologyHandler(t, epistemologySampleLawsJSON)

	req := httptest.NewRequest("GET", "/v1/knowledge/epistemology/NOPE", nil)
	req.SetPathValue("status", "NOPE")
	w := httptest.NewRecorder()
	h.EpistemologyByStatus(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !jsonHasError(w.Body.Bytes()) {
		t.Errorf("expected pt-BR error message, got %s", w.Body.String())
	}
}

func TestEpistemologyByStatus_MissingStatusParam(t *testing.T) {
	h := newEpistemologyHandler(t, epistemologySampleLawsJSON)

	req := httptest.NewRequest("GET", "/v1/knowledge/epistemology/", nil)
	w := httptest.NewRecorder()
	h.EpistemologyByStatus(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestEpistemologyByStatus_EmptyStore(t *testing.T) {
	h := newEpistemologyHandler(t, `{"version": 1, "items": []}`)

	req := httptest.NewRequest("GET", "/v1/knowledge/epistemology/STALE", nil)
	req.SetPathValue("status", "STALE")
	w := httptest.NewRecorder()
	h.EpistemologyByStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Items []interface{} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Items == nil || len(resp.Items) != 0 {
		t.Errorf("items = %v, want empty array", resp.Items)
	}
}

// =============================================================================
// Route registration (server.go wiring)
// =============================================================================

// newEpistemologyRouteServer builds a real REST server (rest.New) and returns
// its mux, so the registered routes can be exercised end-to-end.
func newEpistemologyRouteServer(t *testing.T) *http.ServeMux {
	t.Helper()
	userStore := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	srv := rest.New(nil, nil, nil, nil, nil, nil, nil,
		userStore, nil, nil, rest.DefaultConfig(), nil, nil, nil, nil, nil,
		false, nil, nil, nil, nil)
	return srv.Mux()
}

func TestEpistemologyRoutes_Registered(t *testing.T) {
	// O handler criado dentro do server resolve o laws.json pelo cwd
	// (espelhando o CLI) — chdir para a árvore seedada.
	path := writeEpistemologyLaws(t, epistemologySampleLawsJSON)
	chdir(t, filepath.Dir(filepath.Dir(filepath.Dir(path))))

	mux := newEpistemologyRouteServer(t)

	cases := []struct {
		name   string
		path   string
		status int
	}{
		{"summary", "/v1/knowledge/epistemology", http.StatusOK},
		{"by status", "/v1/knowledge/epistemology/STALE", http.StatusOK},
		{"invalid status", "/v1/knowledge/epistemology/NOPE", http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("GET %s = %d, want %d: %s", tc.path, w.Code, tc.status, w.Body.String())
			}
		})
	}
}
