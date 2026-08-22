package bootstrap

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

const testModel = "qwen2.5-coder:14b-128k"

// fakeRunner records calls and returns canned results without touching the OS.
// LookPath mirrors the real PATH search so t.Setenv("PATH", ...) controls
// binary detection exactly as it would for the production runner.
type fakeRunner struct {
	mu          sync.Mutex
	calls       []string
	detachCalls []string
	output      []byte
	err         error
	startErr    error
}

func (f *fakeRunner) LookPath(name string) (string, error) {
	return lookPathInPath(name, os.Getenv("PATH"))
}

func (f *fakeRunner) CombinedOutput(_ context.Context, name string, args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, strings.Join(append([]string{name}, args...), " "))
	return f.output, f.err
}

func (f *fakeRunner) StartDetached(name string, args ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.detachCalls = append(f.detachCalls, strings.Join(append([]string{name}, args...), " "))
	return f.startErr
}

// unreachableURL returns a base URL whose port is closed (connection refused).
func unreachableURL(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return "http://" + addr
}

// newDaemonServer simulates an Ollama daemon: /api/tags lists models,
// /api/generate answers "pong" when generateOK, /api/pull reports success.
func newDaemonServer(t *testing.T, models []string, generateOK bool) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			w.Header().Set("Content-Type", "application/json")
			type tagModel struct {
				Name string `json:"name"`
			}
			ms := make([]tagModel, 0, len(models))
			for _, n := range models {
				ms = append(ms, tagModel{Name: n})
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"models": ms})
		case "/api/generate":
			if !generateOK {
				http.Error(w, `{"error":"model not found"}`, http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": "pong", "done": true})
		case "/api/pull":
			w.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = w.Write([]byte(`{"status":"success"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

// createFakeOllama writes a dummy ollama binary (never executed — the fake
// runner records instead) into dir and returns its path.
func createFakeOllama(t *testing.T, dir string) string {
	t.Helper()
	name := filepath.Join(dir, "ollama")
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(name, []byte("#!/bin/sh\necho fake ollama\n"), 0o755); err != nil {
		t.Fatalf("write fake ollama binary: %v", err)
	}
	return name
}

func findStep(t *testing.T, report *Report, name string) Step {
	t.Helper()
	for _, s := range report.Steps {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("step %q not found in report (steps: %v)", name, stepNames(report))
	return Step{}
}

func stepNames(report *Report) []string {
	names := make([]string, 0, len(report.Steps))
	for _, s := range report.Steps {
		names = append(names, s.Name)
	}
	return names
}

// ─── Scenario 1: no Ollama at all, auto-install not authorized ──────────────

func TestBootstrapScenario1_NoOllama_NoAutoInstall(t *testing.T) {
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)
	t.Setenv("LOCALAPPDATA", emptyDir)
	t.Setenv("USERPROFILE", emptyDir)

	cfg := Config{
		BaseURL:          unreachableURL(t),
		Model:            testModel,
		AllowAutoInstall: false,
		AllowAutoPull:    false,
		ReadinessTimeout: 200 * time.Millisecond,
	}

	report, err := EnsureOllama(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.OK {
		t.Error("Report.OK must be false when ollama is absent")
	}
	if report.Provider != "ollama" {
		t.Errorf("provider = %q, want ollama", report.Provider)
	}

	if s := findStep(t, report, "detect_os"); s.Status != StatusOK {
		t.Errorf("detect_os = %s (%s), want ok", s.Status, s.Detail)
	}
	if s := findStep(t, report, "detect_binary"); s.Status != StatusFailed {
		t.Errorf("detect_binary = %s (%s), want failed", s.Status, s.Detail)
	}
	if s := findStep(t, report, "install"); s.Status != StatusSkipped {
		t.Errorf("install = %s (%s), want skipped when AllowAutoInstall=false", s.Status, s.Detail)
	} else if !strings.Contains(s.Detail, "not authorized") {
		t.Errorf("install skipped detail must say 'not authorized', got: %q", s.Detail)
	}
	if s := findStep(t, report, "start_daemon"); s.Status != StatusSkipped {
		t.Errorf("start_daemon = %s (%s), want skipped when nothing installed", s.Status, s.Detail)
	}
	if s := findStep(t, report, "wait_readiness"); s.Status != StatusSkipped {
		t.Errorf("wait_readiness = %s (%s), want skipped when nothing started", s.Status, s.Detail)
	}
}

// ─── Scenario 2: installed but stopped ───────────────────────────────────────

func TestBootstrapScenario2_InstalledButStopped(t *testing.T) {
	dir := t.TempDir()
	createFakeOllama(t, dir)
	t.Setenv("PATH", dir)

	fake := &fakeRunner{}
	cfg := Config{
		BaseURL:          unreachableURL(t),
		Model:            testModel,
		AllowAutoInstall: false,
		AllowAutoPull:    false,
		ReadinessTimeout: 200 * time.Millisecond,
		runner:           fake,
	}

	report, err := EnsureOllama(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.OK {
		t.Error("Report.OK must be false while the daemon is stopped")
	}

	if s := findStep(t, report, "detect_binary"); s.Status != StatusOK {
		t.Errorf("detect_binary = %s (%s), want ok (binary present)", s.Status, s.Detail)
	}
	if s := findStep(t, report, "detect_daemon"); s.Status != StatusFailed ||
		!strings.Contains(s.Detail, "installed but stopped") {
		t.Errorf("detect_daemon = %s (%s), want failed with 'installed but stopped'", s.Status, s.Detail)
	}
	if s := findStep(t, report, "install"); s.Status != StatusSkipped {
		t.Errorf("install = %s (%s), want skipped (already installed)", s.Status, s.Detail)
	}
	if s := findStep(t, report, "start_daemon"); s.Status != StatusOK {
		t.Errorf("start_daemon = %s (%s), want ok (detached 'ollama serve' started)", s.Status, s.Detail)
	}
	if len(fake.detachCalls) == 0 {
		t.Error("start_daemon must attempt to launch 'ollama serve' when binary exists but daemon is stopped")
	}
	if s := findStep(t, report, "wait_readiness"); s.Status != StatusFailed {
		t.Errorf("wait_readiness = %s (%s), want failed (daemon never became ready)", s.Status, s.Detail)
	}
}

// ─── Scenario 3: running daemon, model missing, pull not authorized ─────────

func TestBootstrapScenario3_RunningModelMissing(t *testing.T) {
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)
	ts := newDaemonServer(t, nil, false)

	cfg := Config{
		BaseURL:          ts.URL,
		Model:            testModel,
		AllowAutoInstall: false,
		AllowAutoPull:    false,
		ReadinessTimeout: time.Second,
	}

	report, err := EnsureOllama(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.OK {
		t.Error("Report.OK must be false when the configured model is missing")
	}

	if s := findStep(t, report, "detect_daemon"); s.Status != StatusOK {
		t.Errorf("detect_daemon = %s (%s), want ok (daemon running)", s.Status, s.Detail)
	}
	if s := findStep(t, report, "wait_readiness"); s.Status != StatusSkipped {
		t.Errorf("wait_readiness = %s (%s), want skipped (already ready)", s.Status, s.Detail)
	}
	if s := findStep(t, report, "verify_model"); s.Status != StatusFailed ||
		!strings.Contains(s.Detail, "missing") {
		t.Errorf("verify_model = %s (%s), want failed with 'missing'", s.Status, s.Detail)
	}
	if s := findStep(t, report, "pull_model"); s.Status != StatusSkipped ||
		!strings.Contains(s.Detail, "not authorized") {
		t.Errorf("pull_model = %s (%s), want skipped when AllowAutoPull=false", s.Status, s.Detail)
	}
	if !strings.Contains(report.Error, "bootstrap --pull") {
		t.Errorf("report.Error should recommend --pull, got: %q", report.Error)
	}
}

// ─── Scenario 4: model available, everything verifies ───────────────────────

func TestBootstrapScenario4_ModelAvailable(t *testing.T) {
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)
	ts := newDaemonServer(t, []string{testModel}, true)

	cfg := Config{
		BaseURL:          ts.URL,
		Model:            testModel,
		AllowAutoInstall: false,
		AllowAutoPull:    false,
		ReadinessTimeout: time.Second,
	}

	report, err := EnsureOllama(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.OK {
		t.Errorf("Report.OK must be true when the model is available; error: %q", report.Error)
	}

	if s := findStep(t, report, "detect_daemon"); s.Status != StatusOK {
		t.Errorf("detect_daemon = %s (%s), want ok", s.Status, s.Detail)
	}
	if s := findStep(t, report, "verify_model"); s.Status != StatusOK {
		t.Errorf("verify_model = %s (%s), want ok", s.Status, s.Detail)
	}
	if s := findStep(t, report, "pull_model"); s.Status != StatusSkipped ||
		!strings.Contains(s.Detail, "already present") {
		t.Errorf("pull_model = %s (%s), want skipped with 'already present'", s.Status, s.Detail)
	}
	if s := findStep(t, report, "validate_response"); s.Status != StatusOK {
		t.Errorf("validate_response = %s (%s), want ok", s.Status, s.Detail)
	}
}

// ─── Scenario 5: fully functional provider (all steps green) ────────────────

func TestBootstrapScenario5_FullyFunctional(t *testing.T) {
	dir := t.TempDir()
	createFakeOllama(t, dir)
	t.Setenv("PATH", dir)
	ts := newDaemonServer(t, []string{testModel}, true)

	cfg := Config{
		BaseURL:          ts.URL,
		Model:            testModel,
		AllowAutoInstall: true, // already running — must be irrelevant
		AllowAutoPull:    true, // already present — must be irrelevant
		ReadinessTimeout: time.Second,
	}

	report, err := EnsureOllama(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.OK {
		t.Errorf("Report.OK must be true for a functional provider; error: %q", report.Error)
	}
	for _, s := range report.Steps {
		if s.Status == StatusFailed {
			t.Errorf("step %q must not be failed in a functional provider: %s (%s)", s.Name, s.Status, s.Detail)
		}
		if s.Detail == "" {
			t.Errorf("step %q must carry evidence (empty detail)", s.Name)
		}
	}
	if s := findStep(t, report, "detect_binary"); s.Status != StatusOK {
		t.Errorf("detect_binary = %s (%s), want ok", s.Status, s.Detail)
	}
	if s := findStep(t, report, "verify_model"); s.Status != StatusOK {
		t.Errorf("verify_model = %s (%s), want ok", s.Status, s.Detail)
	}
	if s := findStep(t, report, "validate_response"); s.Status != StatusOK {
		t.Errorf("validate_response = %s (%s), want ok", s.Status, s.Detail)
	}
}

// ─── Scenario 6: install authorized but every mechanism fails ────────────────

func TestBootstrapScenario6_InstallFailure(t *testing.T) {
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)
	t.Setenv("LOCALAPPDATA", emptyDir)
	t.Setenv("USERPROFILE", emptyDir)

	// Download endpoint that always fails.
	dl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer dl.Close()

	cfg := Config{
		BaseURL:           unreachableURL(t),
		Model:             testModel,
		AllowAutoInstall:  true,
		AllowAutoPull:     false,
		ReadinessTimeout:  200 * time.Millisecond,
	}
	cfg.installerURL = dl.URL + "/OllamaSetup.exe"
	cfg.installScriptURL = dl.URL + "/install.sh"

	report, err := EnsureOllama(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.OK {
		t.Error("Report.OK must be false when installation fails")
	}
	if s := findStep(t, report, "detect_binary"); s.Status != StatusFailed {
		t.Errorf("detect_binary = %s (%s), want failed", s.Status, s.Detail)
	}
	s := findStep(t, report, "install")
	if s.Status != StatusFailed {
		t.Fatalf("install = %s (%s), want failed", s.Status, s.Detail)
	}
	if s.Detail == "" {
		t.Error("install failure must carry manual instructions in Detail")
	}
	if !strings.Contains(s.Detail, "failed") {
		t.Errorf("install Detail must describe the failure, got: %q", s.Detail)
	}
}

// ─── DetectOnly: boot hook mode never mutates anything ───────────────────────

func TestBootstrapDetectOnly_NoSideEffects(t *testing.T) {
	dir := t.TempDir()
	createFakeOllama(t, dir)
	t.Setenv("PATH", dir)

	fake := &fakeRunner{}
	cfg := Config{
		BaseURL:          unreachableURL(t),
		Model:            testModel,
		AllowAutoInstall: true, // must be ignored
		AllowAutoPull:    true, // must be ignored
		DetectOnly:       true,
		ReadinessTimeout: 200 * time.Millisecond,
		runner:           fake,
	}

	report, err := EnsureOllama(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.OK {
		t.Error("Report.OK must be false when the daemon is down even in DetectOnly")
	}
	for _, name := range []string{"install", "start_daemon", "pull_model", "validate_response"} {
		if s := findStep(t, report, name); s.Status != StatusSkipped {
			t.Errorf("%s = %s (%s), want skipped in DetectOnly", name, s.Status, s.Detail)
		}
	}
	if len(fake.detachCalls) != 0 {
		t.Errorf("DetectOnly must never start the daemon, got StartDetached calls: %v", fake.detachCalls)
	}
	if len(fake.calls) != 0 {
		t.Errorf("DetectOnly must never run install commands, got calls: %v", fake.calls)
	}
}
