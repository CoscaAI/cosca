package handler

// White-box tests for unexported security helpers: isSecureRequest (auth.go),
// promptAuditInfo (run.go), and the knowledge search pagination caps
// (knowledge.go). These functions are unexported, so the tests live in
// package handler — the same pattern as websocket_test.go.

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/search"
)

// =============================================================================
// isSecureRequest — TLS / X-Forwarded-Proto detection
// =============================================================================

// TestAuth_IsSecureRequest verifies that isSecureRequest only reports true
// for a TLS connection or when a TLS-terminating proxy announced
// X-Forwarded-Proto: https — which decides whether auth cookies get the
// Secure flag.
func TestAuth_IsSecureRequest(t *testing.T) {
	t.Run("plain HTTP with no proxy header is insecure", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/v1/auth/login", nil)
		if isSecureRequest(req) {
			t.Error("plain HTTP request must not be considered secure")
		}
	})

	t.Run("X-Forwarded-Proto https marks secure", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/v1/auth/login", nil)
		req.Header.Set("X-Forwarded-Proto", "https")
		if !isSecureRequest(req) {
			t.Error("X-Forwarded-Proto: https must be considered secure")
		}
	})

	t.Run("X-Forwarded-Proto http stays insecure", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/v1/auth/login", nil)
		req.Header.Set("X-Forwarded-Proto", "http")
		if isSecureRequest(req) {
			t.Error("X-Forwarded-Proto: http must not be considered secure")
		}
	})

	t.Run("TLS request is secure regardless of header", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/v1/auth/login", nil)
		req.TLS = &tls.ConnectionState{}
		if !isSecureRequest(req) {
			t.Error("TLS request must be considered secure")
		}
	})
}

// =============================================================================
// promptAuditInfo — prompts never land in cleartext in audit logs
// =============================================================================

// TestPromptAuditInfo verifies that promptAuditInfo returns only a SHA-256
// hash (for correlation) and a short preview — never the full prompt — so
// secrets pasted into prompts do not land in the audit database.
func TestPromptAuditInfo(t *testing.T) {
	// A prompt long enough that the preview must truncate it, with a unique
	// marker at the very end.
	longPrompt := strings.Repeat("x", 500) + "ENDMARKER123"

	info := promptAuditInfo(longPrompt)

	// A SHA-256 hex digest is present and matches the full prompt.
	hash, ok := info["prompt_hash"]
	if !ok {
		t.Fatal("expected prompt_hash key")
	}
	expected := sha256.Sum256([]byte(longPrompt))
	if hash != hex.EncodeToString(expected[:]) {
		t.Error("prompt_hash does not match SHA-256 of the full prompt")
	}

	// The preview is a short, truncated prefix — never the tail.
	preview, ok := info["prompt_preview"]
	if !ok {
		t.Fatal("expected prompt_preview key")
	}
	if len([]rune(preview)) > promptPreviewLength {
		t.Errorf("preview longer than %d runes: %d", promptPreviewLength, len([]rune(preview)))
	}
	if strings.Contains(preview, "ENDMARKER123") {
		t.Error("preview must not include the tail of a long prompt")
	}

	// The full prompt is never stored in the map.
	for k, v := range info {
		if v == longPrompt {
			t.Errorf("full prompt leaked under key %q", k)
		}
		if k == "prompt" || k == "prompt_full" {
			t.Errorf("unexpected full-prompt key %q", k)
		}
	}
	if len(info) != 2 {
		t.Errorf("expected exactly 2 fields (prompt_hash, prompt_preview), got %d", len(info))
	}

	// Short prompts are previewed in full (no truncation).
	short := "hello world"
	shortInfo := promptAuditInfo(short)
	if shortInfo["prompt_preview"] != short {
		t.Errorf("expected full preview for short prompt, got %q", shortInfo["prompt_preview"])
	}
	if shortInfo["prompt_hash"] == "" {
		t.Error("expected a non-empty prompt_hash for short prompts too")
	}
}

// =============================================================================
// Knowledge search pagination caps (M9b — DoS hardening)
// =============================================================================

// newWhiteBoxTestKnowledgeEngine creates a fully initialized knowledge engine
// for the white-box handler tests (the external helper newTestKnowledgeEngine
// lives in package handler_test and is not visible here).
func newWhiteBoxTestKnowledgeEngine(t *testing.T) *knowledge.Engine {
	t.Helper()
	cfg := knowledge.Config{
		DBPath:            filepath.Join(t.TempDir(), "knowledge.db"),
		RootDir:           t.TempDir(),
		AutoMigrate:       true,
		WatchEnabled:      false,
		EmbeddingProvider: "auto",
		IndexerConfig:     knowledge.DefaultConfig().IndexerConfig,
		CacheConfig:       knowledge.DefaultConfig().CacheConfig,
		RankingConfig:     knowledge.DefaultConfig().RankingConfig,
		SearchConfig:      search.DefaultSearchParams(),
	}
	engine, err := knowledge.New(cfg)
	if err != nil {
		t.Fatalf("failed to create knowledge engine: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	if err := engine.Init(); err != nil {
		t.Fatalf("failed to initialize knowledge engine: %v", err)
	}
	return engine
}

// TestKnowledgeSearch_LimitCapped verifies the M9b DoS hardening: a
// client-supplied search limit larger than the hard cap is clamped to
// maxSearchLimit before it reaches the engine, so a single request can never
// force the engine to materialize an unbounded result set.
func TestKnowledgeSearch_LimitCapped(t *testing.T) {
	// The cap constants themselves are part of the security contract.
	if maxSearchLimit != 100 {
		t.Errorf("expected maxSearchLimit=100, got %d", maxSearchLimit)
	}
	if maxSearchOffset != 1000 {
		t.Errorf("expected maxSearchOffset=1000, got %d", maxSearchOffset)
	}

	// Behavioral check through the real handler + engine: index more
	// documents than the cap so an uncapped query would return >100 results,
	// then verify a limit=999999 request is clamped to at most maxSearchLimit.
	engine := newWhiteBoxTestKnowledgeEngine(t)
	root := engine.RootDir()
	const docs = 130
	for i := 0; i < docs; i++ {
		name := fmt.Sprintf("cap_doc_%03d.md", i)
		content := "# Cap Doc\n\nThis document contains the unique term coscacapclampmarker for the cap test.\n"
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := engine.IndexDirectory(context.Background(), root); err != nil {
		t.Fatalf("IndexDirectory: %v", err)
	}

	h := NewKnowledgeHandler(engine, nil)
	body := `{"query":"coscacapclampmarker","limit":999999,"enable_fts":true,"enable_vector":false,"enable_graph":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/knowledge/search", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("search: got %d: %s", w.Code, w.Body.String())
	}
	var resp SearchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Total <= maxSearchLimit {
		t.Fatalf("test setup invalid: expected >%d total matches, got %d", maxSearchLimit, resp.Total)
	}
	if len(resp.Results) > maxSearchLimit {
		t.Errorf("limit=999999 returned %d results, want at most %d (clamped)", len(resp.Results), maxSearchLimit)
	}
}
