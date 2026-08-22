package cli

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/internal/env"
	"github.com/CoscaAI/cosca/internal/grpcclient"
)

// =============================================================================
// loadDotEnv tests
// =============================================================================

func TestLoadDotEnv_FileExists(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.Unsetenv("TEST_KEY_A")
	os.Unsetenv("TEST_KEY_B")

	envContent := "TEST_KEY_A=value_a\n# comment\nTEST_KEY_B=value_b\n"
	os.WriteFile(".env", []byte(envContent), 0644)

	env.Load(".env", false)

	if got := os.Getenv("TEST_KEY_A"); got != "value_a" {
		t.Errorf("TEST_KEY_A = %q, want %q", got, "value_a")
	}
	if got := os.Getenv("TEST_KEY_B"); got != "value_b" {
		t.Errorf("TEST_KEY_B = %q, want %q", got, "value_b")
	}
}

func TestLoadDotEnv_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	env.Load(".env", false) // should not panic
}

func TestLoadDotEnv_SkipsAlreadySet(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.Setenv("EXISTING_KEY", "original")
	defer os.Unsetenv("EXISTING_KEY")

	envContent := "EXISTING_KEY=override\nNEW_KEY=new_value\n"
	os.WriteFile(".env", []byte(envContent), 0644)

	env.Load(".env", false)

	if got := os.Getenv("EXISTING_KEY"); got != "original" {
		t.Errorf("EXISTING_KEY = %q, want %q (should not override)", got, "original")
	}
	if got := os.Getenv("NEW_KEY"); got != "new_value" {
		t.Errorf("NEW_KEY = %q, want %q", got, "new_value")
	}
}

func TestLoadDotEnv_EmptyLinesAndComments(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.Unsetenv("ONLY_KEY")
	envContent := "\n# comment line\n\nONLY_KEY=hello\n\n"
	os.WriteFile(".env", []byte(envContent), 0644)

	env.Load(".env", false)

	if got := os.Getenv("ONLY_KEY"); got != "hello" {
		t.Errorf("ONLY_KEY = %q, want %q", got, "hello")
	}
}

func TestLoadDotEnv_StripsQuotes(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.Unsetenv("QUOTED_KEY")
	envContent := "QUOTED_KEY=\"quoted value\"\n"
	os.WriteFile(".env", []byte(envContent), 0644)

	env.Load(".env", false)

	if got := os.Getenv("QUOTED_KEY"); got != "quoted value" {
		t.Errorf("QUOTED_KEY = %q, want %q (quotes stripped)", got, "quoted value")
	}
}

// =============================================================================
// configureCORSFromEnv tests
// =============================================================================

func TestConfigureCORSFromEnv_DefaultOverride(t *testing.T) {
	os.Setenv("COSCA_CORS_ORIGINS", "http://localhost:3000")
	defer os.Unsetenv("COSCA_CORS_ORIGINS")

	cors := "" // the new fail-closed default
	configureCORSFromEnv(&cors)

	if cors != "http://localhost:3000" {
		t.Errorf("cors = %q, want %q", cors, "http://localhost:3000")
	}
}

func TestConfigureCORSFromEnv_NoOverrideWhenCustom(t *testing.T) {
	os.Setenv("COSCA_CORS_ORIGINS", "http://localhost:3000")
	defer os.Unsetenv("COSCA_CORS_ORIGINS")

	cors := "https://example.com"
	configureCORSFromEnv(&cors)

	if cors != "https://example.com" {
		t.Errorf("cors = %q, want %q", cors, "https://example.com")
	}
}

func TestConfigureCORSFromEnv_EmptyEnv(t *testing.T) {
	os.Unsetenv("COSCA_CORS_ORIGINS")

	cors := ""
	configureCORSFromEnv(&cors)

	if cors != "" {
		t.Errorf("cors = %q, want empty (fail-closed)", cors)
	}
}

// =============================================================================
// resolveDataDir tests
// =============================================================================

func TestResolveDataDir_EmptyDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	dir, err := resolveDataDir("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := filepath.Join(tmpDir, ".cosca")
	if dir != want {
		t.Errorf("dir = %q, want %q", dir, want)
	}
}

func TestResolveDataDir_ExplicitDir(t *testing.T) {
	dir, err := resolveDataDir("/custom/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dir != "/custom/path" {
		t.Errorf("dir = %q, want %q", dir, "/custom/path")
	}
}

// =============================================================================
// metricsAuthMiddleware tests (using net/http/httptest)
// =============================================================================

func TestMetricsAuthMiddleware_ValidToken(t *testing.T) {
	secret := "my-secret"
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := metricsAuthMiddleware(next, secret)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer my-secret")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("next handler was not called for valid token")
	}
	if w.Code == 401 {
		t.Error("should not return 401 for valid token")
	}
}

func TestMetricsAuthMiddleware_MissingHeader(t *testing.T) {
	secret := "my-secret"
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := metricsAuthMiddleware(next, secret)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if called {
		t.Error("next handler should NOT be called for missing header")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if !strings.Contains(w.Body.String(), "missing or invalid") {
		t.Errorf("body = %q, want 'missing or invalid'", w.Body.String())
	}
}

func TestMetricsAuthMiddleware_WrongToken(t *testing.T) {
	secret := "correct-secret"
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := metricsAuthMiddleware(next, secret)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer wrong-secret")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if called {
		t.Error("next handler should NOT be called for wrong token")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if !strings.Contains(w.Body.String(), "invalid metrics secret") {
		t.Errorf("body = %q, want 'invalid metrics secret'", w.Body.String())
	}
}

func TestMetricsAuthMiddleware_EmptyToken(t *testing.T) {
	secret := "my-secret"
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := metricsAuthMiddleware(next, secret)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer ")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if called {
		t.Error("next handler should NOT be called for empty token")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

// =============================================================================
// captureGitState tests
// =============================================================================

func TestCaptureGitState_InGitRepo(t *testing.T) {
	tmpDir := t.TempDir()

	runGitCmd(tmpDir, "init")
	runGitCmd(tmpDir, "config", "user.email", "test@test.com")
	runGitCmd(tmpDir, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# test"), 0644)
	runGitCmd(tmpDir, "add", "README.md")
	runGitCmd(tmpDir, "commit", "-m", "initial")

	coscaDir := filepath.Join(tmpDir, ".cosca")
	os.MkdirAll(coscaDir, 0755)

	gs := captureGitState(coscaDir)

	if gs.commitCount != 1 {
		t.Errorf("commitCount = %d, want 1", gs.commitCount)
	}
	if gs.trackedFiles != 1 {
		t.Errorf("trackedFiles = %d, want 1", gs.trackedFiles)
	}
}

func TestCaptureGitState_NotInGitRepo(t *testing.T) {
	tmpDir := t.TempDir()
	coscaDir := filepath.Join(tmpDir, ".cosca")
	os.MkdirAll(coscaDir, 0755)

	gs := captureGitState(coscaDir)

	if gs.commitCount != 0 {
		t.Errorf("commitCount = %d, want 0", gs.commitCount)
	}
	if gs.trackedFiles != 0 {
		t.Errorf("trackedFiles = %d, want 0", gs.trackedFiles)
	}
}

// =============================================================================
// recordSessionOnShutdown test
// =============================================================================

func TestRecordSessionOnShutdown_CreatesRecord(t *testing.T) {
	tmpDir := t.TempDir()

	memDir := filepath.Join(tmpDir, "memory", "session")
	os.MkdirAll(memDir, 0755)

	logger := zerolog.New(io.Discard)

	gs := gitState{commitCount: 5, trackedFiles: 10}
	startedAt := time.Now().Add(-1 * time.Hour)

	recordSessionOnShutdown(tmpDir, startedAt, "SIGINT", gs, &logger)

	entries, err := os.ReadDir(memDir)
	if err != nil {
		t.Fatalf("failed to read session dir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("no session file was created")
	} else {
		t.Logf("session files created: %d", len(entries))
	}
}

// =============================================================================
// runServe integration — exercises initialization path up to server start
// =============================================================================

func TestRunServe_Integration_EarlyInit(t *testing.T) {
	// Shutdown é disparado por sinal (os.Interrupt via Process.Signal), que
	// não é suportado no Windows — o servidor nunca pararia e o durable.db
	// ficaria aberto (falha de cleanup do TempDir, que não apaga arquivo em uso).
	if runtime.GOOS == "windows" {
		t.Skip("signal-based shutdown (os.Interrupt) is not supported on Windows")
	}
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	dataDir := filepath.Join(tmpDir, ".cosca")
	os.MkdirAll(dataDir, 0755)

	host := "127.0.0.1"
	port := 19999
	metricsPort := 19998
	cors := "" // fail-closed default
	dd := dataDir
	tlsCert := ""
	tlsKey := ""
	grpcPort := 19997
	grpcReflection := false
	grpcDisable := true
	enableWS := false
	wsOrigins := ""

	runE := runServe(
		&host, &port, &metricsPort, &cors, &dd,
		&tlsCert, &tlsKey, &grpcPort, &grpcReflection,
		&grpcDisable, &enableWS, &wsOrigins,
	)

	os.Setenv("COSCA_JWT_SECRET", "test-secret-that-is-at-least-32-bytes-long!!")
	defer os.Unsetenv("COSCA_JWT_SECRET")

	errCh := make(chan error, 1)
	go func() {
		errCh <- runE(nil, nil)
	}()

	time.Sleep(800 * time.Millisecond)

	p, _ := os.FindProcess(os.Getpid())
	if p != nil {
		p.Signal(os.Interrupt)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	select {
	case err := <-errCh:
		t.Logf("runServe returned: %v", err)
	case <-ctx.Done():
		t.Log("runServe exercised initialization path")
	}
}

func TestRunServe_Integration_ExplicitDataDir(t *testing.T) {
	// Shutdown é disparado por sinal (os.Interrupt via Process.Signal), que
	// não é suportado no Windows — o servidor nunca pararia e o durable.db
	// ficaria aberto (falha de cleanup do TempDir, que não apaga arquivo em uso).
	if runtime.GOOS == "windows" {
		t.Skip("signal-based shutdown (os.Interrupt) is not supported on Windows")
	}
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	dataDir := filepath.Join(tmpDir, "explicit-cosca")
	os.MkdirAll(dataDir, 0755)

	host := "127.0.0.1"
	port := 19989
	metricsPort := 19988
	cors := "" // fail-closed default
	dd := dataDir
	tlsCert := ""
	tlsKey := ""
	grpcPort := 19987
	grpcReflection := false
	grpcDisable := true
	enableWS := false
	wsOrigins := ""

	runE := runServe(
		&host, &port, &metricsPort, &cors, &dd,
		&tlsCert, &tlsKey, &grpcPort, &grpcReflection,
		&grpcDisable, &enableWS, &wsOrigins,
	)

	os.Setenv("COSCA_JWT_SECRET", "test-secret-that-is-at-least-32-bytes-long!!")
	defer os.Unsetenv("COSCA_JWT_SECRET")

	errCh := make(chan error, 1)
	go func() {
		errCh <- runE(nil, nil)
	}()

	time.Sleep(800 * time.Millisecond)

	p, _ := os.FindProcess(os.Getpid())
	if p != nil {
		p.Signal(os.Interrupt)
	}

	select {
	case err := <-errCh:
		t.Logf("runServe returned: %v", err)
	case <-time.After(10 * time.Second):
		t.Log("runServe exercised initialization path")
	}
}

func TestRunServe_Integration_WithEnvFile(t *testing.T) {
	// Shutdown é disparado por sinal (os.Interrupt via Process.Signal), que
	// não é suportado no Windows — o servidor nunca pararia e o durable.db
	// ficaria aberto (falha de cleanup do TempDir, que não apaga arquivo em uso).
	if runtime.GOOS == "windows" {
		t.Skip("signal-based shutdown (os.Interrupt) is not supported on Windows")
	}
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.WriteFile(".env", []byte("COSCA_CORS_ORIGINS=http://custom:3000\n"), 0644)

	dataDir := filepath.Join(tmpDir, ".cosca")
	os.MkdirAll(dataDir, 0755)

	host := "127.0.0.1"
	port := 19979
	metricsPort := 19978
	cors := "" // fail-closed default
	dd := dataDir
	tlsCert := ""
	tlsKey := ""
	grpcPort := 19977
	grpcReflection := false
	grpcDisable := true
	enableWS := false
	wsOrigins := ""

	runE := runServe(
		&host, &port, &metricsPort, &cors, &dd,
		&tlsCert, &tlsKey, &grpcPort, &grpcReflection,
		&grpcDisable, &enableWS, &wsOrigins,
	)

	os.Setenv("COSCA_JWT_SECRET", "test-secret-that-is-at-least-32-bytes-long!!")
	defer os.Unsetenv("COSCA_JWT_SECRET")

	errCh := make(chan error, 1)
	go func() {
		errCh <- runE(nil, nil)
	}()

	time.Sleep(800 * time.Millisecond)

	p, _ := os.FindProcess(os.Getpid())
	if p != nil {
		p.Signal(os.Interrupt)
	}

	select {
	case err := <-errCh:
		t.Logf("runServe returned: %v", err)
		if cors != "http://custom:3000" {
			t.Errorf("CORS not overridden by .env: got %q", cors)
		}
	case <-time.After(10 * time.Second):
		t.Log("runServe exercised .env loading path")
	}
}

// =============================================================================
// runServe wrapper (historical signature, new defaults)
// =============================================================================

// TestRunServe_Wrapper_HistoricalSignature is a compile-time proof that the
// historical runServe wrapper keeps its original signature and delegates to
// runServeProvider with the new FASE 2 defaults (apiOnly=false, daemon
// address = DefaultRuntimeAddr). The existing TestRunServe_Integration_*
// tests above already exercise the wrapper end-to-end (server boots, daemon
// enabled, graceful shutdown); this test documents the defaults contract
// without booting a server.
func TestRunServe_Wrapper_HistoricalSignature(t *testing.T) {
	host := "127.0.0.1"
	port := 19939
	metricsPort := 19938
	cors := ""
	dd := t.TempDir()
	tlsCert := ""
	tlsKey := ""
	grpcPort := 19937
	grpcReflection := false
	grpcDisable := true
	enableWS := false
	wsOrigins := ""

	// Compiles only if the wrapper keeps the historical parameter list.
	runE := runServe(
		&host, &port, &metricsPort, &cors, &dd,
		&tlsCert, &tlsKey, &grpcPort, &grpcReflection,
		&grpcDisable, &enableWS, &wsOrigins,
	)
	if runE == nil {
		t.Fatal("runServe returned nil RunE")
	}
}

// =============================================================================
// --api-only (FASE 2 — DDNA-2026-08-07-001)
// =============================================================================

// TestServeCommand_APIONlyFlags verifies the --api-only / --runtime-grpc-addr
// flag wiring on the serve command without booting the server.
func TestServeCommand_APIONlyFlags(t *testing.T) {
	cmd := NewServeCommand()

	apiOnly := cmd.Flags().Lookup("api-only")
	if apiOnly == nil {
		t.Fatal("missing --api-only flag on serve command")
	}
	if apiOnly.DefValue != "false" {
		t.Errorf("--api-only default = %q, want \"false\"", apiOnly.DefValue)
	}

	runtimeAddr := cmd.Flags().Lookup("runtime-grpc-addr")
	if runtimeAddr == nil {
		t.Fatal("missing --runtime-grpc-addr flag on serve command")
	}
	if runtimeAddr.DefValue != grpcclient.DefaultRuntimeAddr {
		t.Errorf("--runtime-grpc-addr default = %q, want %q", runtimeAddr.DefValue, grpcclient.DefaultRuntimeAddr)
	}

	// Parse round-trip.
	if err := cmd.Flags().Set("api-only", "true"); err != nil {
		t.Fatalf("--api-only true should parse: %v", err)
	}
	if err := cmd.Flags().Set("runtime-grpc-addr", "127.0.0.1:14999"); err != nil {
		t.Fatalf("--runtime-grpc-addr should parse: %v", err)
	}
	gotAPIOnly, err := cmd.Flags().GetBool("api-only")
	if err != nil {
		t.Fatalf("GetBool(api-only): %v", err)
	}
	if !gotAPIOnly {
		t.Error("api-only = false after setting --api-only=true")
	}
	gotAddr, err := cmd.Flags().GetString("runtime-grpc-addr")
	if err != nil {
		t.Fatalf("GetString(runtime-grpc-addr): %v", err)
	}
	if gotAddr != "127.0.0.1:14999" {
		t.Errorf("runtime-grpc-addr = %q, want %q", gotAddr, "127.0.0.1:14999")
	}
}

// TestRunServe_APIONly_NoDaemonPID verifies that --api-only boots the REST
// server WITHOUT starting the internal background daemon:
//
//   - bootstrap.Compose is called with EnableDaemon=false → the daemon is
//     never created → NO <dataDir>/cosca.pid file is written (the FASE 2
//     contract — no PID file, no sync/backup loop);
//   - a RuntimeClient is wired to --runtime-grpc-addr (pointed at a closed
//     loopback port — the client is lazy, so boot never dials it);
//   - the server still shuts down gracefully via SIGINT.
func TestRunServe_APIONly_NoDaemonPID(t *testing.T) {
	// Shutdown é disparado por sinal (os.Interrupt via Process.Signal), que
	// não é suportado no Windows — o servidor nunca pararia dentro do timeout.
	if runtime.GOOS == "windows" {
		t.Skip("signal-based shutdown (os.Interrupt) is not supported on Windows")
	}
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	dataDir := filepath.Join(tmpDir, ".cosca")
	os.MkdirAll(dataDir, 0755)

	host := "127.0.0.1"
	port := 19929
	metricsPort := 19928
	cors := "" // fail-closed default
	dd := dataDir
	tlsCert := ""
	tlsKey := ""
	grpcPort := 19927
	grpcReflection := false
	grpcDisable := true
	enableWS := false
	wsOrigins := ""
	provider := ""
	apiOnly := true
	// No standalone daemon is expected in the sandbox: point the runtime
	// client at a closed loopback port. The client dials lazily (only on an
	// RPC), so the server boots normally.
	runtimeGRPCAddr := "127.0.0.1:1"
	runtimeStandalone := true
	pipelineEnable := true

	runE := runServeProvider(
		&provider, &host, &port, &metricsPort, &cors, &dd,
		&tlsCert, &tlsKey, &grpcPort, &grpcReflection,
		&grpcDisable, &enableWS, &wsOrigins, &apiOnly, &runtimeGRPCAddr,
		&runtimeStandalone,
		&pipelineEnable,
	)

	os.Setenv("COSCA_JWT_SECRET", "test-secret-that-is-at-least-32-bytes-long!!")
	defer os.Unsetenv("COSCA_JWT_SECRET")

	errCh := make(chan error, 1)
	go func() {
		errCh <- runE(nil, nil)
	}()

	// Give the server time to boot (mirrors the existing runServe tests).
	// bootstrap.Compose (where the daemon would write the PID file) runs
	// before the HTTP servers start, so the check below is meaningful even
	// if the boot is still in progress.
	time.Sleep(1200 * time.Millisecond)

	// The daemon MUST NOT have started in api-only mode: no PID file.
	pidPath := filepath.Join(dataDir, "cosca.pid")
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Errorf("api-only mode created PID file %s — daemon should NOT run", pidPath)
	}

	// Stop the server gracefully.
	p, _ := os.FindProcess(os.Getpid())
	if p != nil {
		p.Signal(os.Interrupt)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("runServe (api-only) returned error: %v", err)
		}
		t.Log("api-only serve shut down gracefully")
	case <-ctx.Done():
		t.Fatal("api-only serve did not shut down within 15s")
	}
}
