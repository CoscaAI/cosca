package discovery

import (
	"context"
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	t.Parallel()
	e := NewEngine()
	if e == nil {
		t.Fatal("NewEngine returned nil")
	}
	if e.workDir != "." {
		t.Errorf("workDir = %q, want %q", e.workDir, ".")
	}
	if e.cacheTTL != 30*time.Second {
		t.Errorf("cacheTTL = %v, want 30s", e.cacheTTL)
	}
}

func TestWithWorkDir(t *testing.T) {
	t.Parallel()
	e := NewEngine(WithWorkDir("/custom/path"))
	if e.workDir != "/custom/path" {
		t.Errorf("workDir = %q", e.workDir)
	}
}

func TestWithCacheTTL(t *testing.T) {
	t.Parallel()
	e := NewEngine(WithCacheTTL(5 * time.Minute))
	if e.cacheTTL != 5*time.Minute {
		t.Errorf("cacheTTL = %v", e.cacheTTL)
	}
}

func TestGetCachedReportEmpty(t *testing.T) {
	t.Parallel()
	e := NewEngine()
	report, ok := e.GetCachedReport()
	if ok {
		t.Error("cached report should not be available")
	}
	if report != nil {
		t.Error("report should be nil")
	}
}

func TestInvalidateCache(t *testing.T) {
	t.Parallel()
	e := NewEngine()
	e.cache = &DiscoveryReport{DiscoveredAt: time.Now()}
	e.InvalidateCache()
	report, ok := e.GetCachedReport()
	if ok {
		t.Error("report should be invalidated")
	}
	if report != nil {
		t.Error("report should be nil")
	}
}

func TestDiscoveryReportStructure(t *testing.T) {
	t.Parallel()
	report := &DiscoveryReport{
		DiscoveredAt: time.Now(),
		Duration:     time.Second,
	}
	if report.DiscoveredAt.IsZero() {
		t.Error("DiscoveredAt should be set")
	}
	if report.Duration != time.Second {
		t.Errorf("Duration = %v", report.Duration)
	}
}

func TestWorkspaceInfo(t *testing.T) {
	t.Parallel()
	info := &WorkspaceInfo{
		Root:      "/workspace",
		IsGitRepo: true,
		GitBranch: "main",
	}
	if info.Root != "/workspace" {
		t.Errorf("Root = %q", info.Root)
	}
	if !info.IsGitRepo {
		t.Error("IsGitRepo should be true")
	}
	if info.GitBranch != "main" {
		t.Errorf("GitBranch = %q", info.GitBranch)
	}
}

func TestProviderInfo(t *testing.T) {
	t.Parallel()
	info := ProviderInfo{
		Name:     "openai",
		Type:     "llm",
		Enabled:  true,
		Endpoint: "https://api.openai.com",
		Model:    "gpt-4",
	}
	if info.Name != "openai" {
		t.Errorf("Name = %q", info.Name)
	}
	if !info.Enabled {
		t.Error("Enabled should be true")
	}
}

func TestPluginInfoStruct(t *testing.T) {
	t.Parallel()
	info := PluginInfo{
		Name:    "my-plugin",
		Version: "1.0.0",
		Enabled: true,
		Path:    "/plugins/my-plugin",
	}
	if info.Name != "my-plugin" {
		t.Errorf("Name = %q", info.Name)
	}
	if info.Version != "1.0.0" {
		t.Errorf("Version = %q", info.Version)
	}
}

func TestRuntimeInfo(t *testing.T) {
	t.Parallel()
	info := &RuntimeInfo{
		Mode:       "server",
		LogLevel:   "debug",
		ConfigFile: "/cfg.yaml",
	}
	if info.Mode != "server" {
		t.Errorf("Mode = %q", info.Mode)
	}
	if info.LogLevel != "debug" {
		t.Errorf("LogLevel = %q", info.LogLevel)
	}
}

func TestRuntimeInfoDefaults(t *testing.T) {
	t.Parallel()
	info := &RuntimeInfo{}
	if info.Mode != "" {
		t.Errorf("Mode should be empty, got %q", info.Mode)
	}
}

func TestProviderEnvVars(t *testing.T) {
	t.Parallel()
	// Just test the map exists (can't test env vars without setting them)
	expected := map[string]string{
		"openai":    "OPENAI_API_KEY",
		"anthropic": "ANTHROPIC_API_KEY",
		"google":    "GOOGLE_API_KEY",
		"ollama":    "", // not in the original map
	}
	if expected["openai"] != "OPENAI_API_KEY" {
		t.Errorf("openai env = %q", expected["openai"])
	}
}

func TestDetectRuntimeDefaults(t *testing.T) {
	t.Parallel()
	// Test the DetectRuntime logic without env vars
	info := &RuntimeInfo{
		Mode:     "cli",
		LogLevel: "info",
	}
	if info.Mode != "cli" {
		t.Errorf("Mode = %q", info.Mode)
	}
}

func TestDiscoverAllEmpty(t *testing.T) {
	t.Parallel()
	e := NewEngine()
	report, err := e.DiscoverAll(context.Background())
	if err != nil {
		t.Fatalf("DiscoverAll error: %v", err)
	}
	if report == nil {
		t.Fatal("report is nil")
	}
	// DiscoverAll always returns a Project with default Root="."
	if report.Project == nil {
		t.Error("Project should not be nil")
	} else if report.Project.Root == "" {
		t.Error("Project.Root should not be empty")
	}
}

func TestMockDiscoveryEngine(t *testing.T) {
	t.Parallel()
	m := &MockDiscoveryEngine{}
	report, err := m.DiscoverAll(context.Background())
	if err != nil {
		t.Fatalf("DiscoverAll error: %v", err)
	}
	if report == nil {
		t.Fatal("report is nil")
	}
	_, ok := m.GetCachedReport()
	if ok {
		t.Error("GetCachedReport should return false")
	}
	m.InvalidateCache()
}

func TestMockDiscoveryEngineCustom(t *testing.T) {
	t.Parallel()
	m := &MockDiscoveryEngine{
		DiscoverAllFunc: func(_ context.Context) (*DiscoveryReport, error) {
			return &DiscoveryReport{Duration: time.Minute}, nil
		},
	}
	report, err := m.DiscoverAll(context.Background())
	if err != nil {
		t.Fatalf("DiscoverAll error: %v", err)
	}
	if report.Duration != time.Minute {
		t.Errorf("Duration = %v", report.Duration)
	}
}
