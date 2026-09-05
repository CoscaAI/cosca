//
// Helper function coverage tests — targets dirExists (0%), chownRecursive (0%),
// parseWsOrigins (0%), runGitCmd (0%), captureGitState (0%), and other
// low-coverage utility functions.
//

package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// dirExists
// =============================================================================

func TestDirExists_ExistingDir(t *testing.T) {
	dir := t.TempDir()
	if !dirExists(dir) {
		t.Error("expected true for existing directory")
	}
}

func TestDirExists_NonExistent(t *testing.T) {
	if dirExists("/nonexistent/path/for/testing") {
		t.Error("expected false for nonexistent path")
	}
}

func TestDirExists_IsFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	_ = os.WriteFile(filePath, []byte("data"), 0644)
	if dirExists(filePath) {
		t.Error("expected false for regular file")
	}
}

func TestDirExists_EmptyPath(t *testing.T) {
	// Empty path — os.Stat("") fails, should return false
	if dirExists("") {
		t.Error("expected false for empty path")
	}
}

// =============================================================================
// chownRecursive
// =============================================================================

func TestChownRecursive_DoesNotPanicOnEmptyDir(t *testing.T) {
	dir := t.TempDir()
	// chownRecursive with current uid/gid should not panic
	// (os.Chown may fail with permission errors but that's fine)
	// Use -1 for uid/gid to skip actual chown
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("chownRecursive panicked: %v", r)
			}
		}()
		chownRecursive(dir, os.Getuid(), os.Getgid())
	}()
}

func TestChownRecursive_WithFiles(t *testing.T) {
	dir := t.TempDir()
	// Create files and subdirs
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644)
	subDir := filepath.Join(dir, "sub")
	_ = os.MkdirAll(subDir, 0755)
	_ = os.WriteFile(filepath.Join(subDir, "b.txt"), []byte("b"), 0644)

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("chownRecursive panicked: %v", r)
			}
		}()
		chownRecursive(dir, os.Getuid(), os.Getgid())
	}()
}

// =============================================================================
// parseWsOrigins
// =============================================================================

func TestParseWsOrigins_Empty(t *testing.T) {
	result := parseWsOrigins("")
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

func TestParseWsOrigins_SingleOrigin(t *testing.T) {
	result := parseWsOrigins("localhost:3000")
	if len(result) != 1 || result[0] != "localhost:3000" {
		t.Errorf("expected [localhost:3000], got %v", result)
	}
}

func TestParseWsOrigins_MultipleOrigins(t *testing.T) {
	result := parseWsOrigins("localhost:3000,example.com,api.test.com")
	expected := []string{"localhost:3000", "example.com", "api.test.com"}
	if len(result) != len(expected) {
		t.Errorf("expected %d origins, got %d", len(expected), len(result))
	}
	for i, v := range expected {
		if i < len(result) && result[i] != v {
			t.Errorf("origin[%d] = %q, want %q", i, result[i], v)
		}
	}
}

func TestParseWsOrigins_WithWhitespace(t *testing.T) {
	result := parseWsOrigins(" localhost:3000 , example.com ,  api.test.com  ")
	if len(result) != 3 || result[0] != "localhost:3000" {
		t.Errorf("expected trimmed origins, got %v", result)
	}
}

func TestParseWsOrigins_EmptyParts(t *testing.T) {
	result := parseWsOrigins(",localhost:3000,,example.com,")
	if len(result) != 2 {
		t.Errorf("expected 2 origins, got %d: %v", len(result), result)
	}
}

func TestParseWsOrigins_OnlyCommas(t *testing.T) {
	result := parseWsOrigins(",,,")
	if result != nil && len(result) != 0 {
		t.Errorf("expected nil or empty for comma-only input, got %v", result)
	}
}

// =============================================================================
// runGitCmd
// =============================================================================

func TestRunGitCmd_NoGitRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := runGitCmd(dir, "status")
	if err == nil {
		t.Error("expected error running git in non-git directory")
	}
	if !strings.Contains(err.Error(), "git status") {
		t.Errorf("error should mention 'git status', got: %v", err)
	}
}

func TestRunGitCmd_InvalidCommand(t *testing.T) {
	dir := t.TempDir()
	_, err := runGitCmd(dir, "__nonexistent_git_command_xyz")
	if err == nil {
		t.Error("expected error for invalid git command")
	}
}

// =============================================================================
// captureGitState
// =============================================================================

func TestCaptureGitState_NoRepo(t *testing.T) {
	dir := t.TempDir()
	gs := captureGitState(dir)
	// Should return zero values when not in a git repository
	if gs.commitCount != 0 {
		t.Errorf("commitCount = %d, want 0 (no git repo)", gs.commitCount)
	}
	if gs.trackedFiles != 0 {
		t.Errorf("trackedFiles = %d, want 0 (no git repo)", gs.trackedFiles)
	}
}

// =============================================================================
// metricsAuthMiddleware
// =============================================================================

func TestMetricsAuthMiddleware_NoAuthHeader(t *testing.T) {
	middleware := metricsAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "secret123")

	req := httptest.NewRequest("GET", "/metrics", nil)
	// No Authorization header
	rec := httptest.NewRecorder()
	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestMetricsAuthMiddleware_EmptyAuthHeader(t *testing.T) {
	middleware := metricsAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "secret123")

	req := httptest.NewRequest("GET", "/metrics", nil)
	req.Header.Set("Authorization", "")
	rec := httptest.NewRecorder()
	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for empty auth, got %d", rec.Code)
	}
}

func TestMetricsAuthMiddleware_ShortAuthHeader(t *testing.T) {
	middleware := metricsAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "secret123")

	req := httptest.NewRequest("GET", "/metrics", nil)
	req.Header.Set("Authorization", "short")
	rec := httptest.NewRecorder()
	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for short auth, got %d", rec.Code)
	}
}

func TestMetricsAuthMiddleware_InvalidSecret(t *testing.T) {
	middleware := metricsAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "secret123")

	req := httptest.NewRequest("GET", "/metrics", nil)
	req.Header.Set("Authorization", "Bearer wrong-secret")
	rec := httptest.NewRecorder()
	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong secret, got %d", rec.Code)
	}
}

func TestMetricsAuthMiddleware_ValidSecret_Additional(t *testing.T) {
	middleware := metricsAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}), "secret123")

	req := httptest.NewRequest("GET", "/metrics", nil)
	req.Header.Set("Authorization", "Bearer secret123")
	rec := httptest.NewRecorder()
	middleware.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}
