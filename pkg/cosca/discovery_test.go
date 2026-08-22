package cosca

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// DiscoverySDK Tests
// =============================================================================

func TestDiscoverySDK_DiscoverProject(t *testing.T) {
	t.Parallel()

	expected := ProjectInfo{
		Name:        "cosca-enterprise",
		Path:        "/home/dev/projects/cosca",
		Language:    "go",
		Framework:   "none",
		Version:     "1.3.0",
		BuildSystem: "go modules",
		Dependencies: []string{
			"github.com/spf13/cobra",
			"github.com/rs/zerolog",
		},
		Entrypoint:  "cmd/cosca/main.go",
		ConfigFiles: []string{".cosca.yaml", "go.mod", "go.sum"},
		HasTests:    true,
		HasDocker:   true,
		HasCI:       true,
		Confidence:  0.95,
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/discovery/project")
		writeJSON(t, w, http.StatusOK, expected)
	})

	info, err := c.Discovery.DiscoverProject()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if info.Name != "cosca-enterprise" {
		t.Errorf("expected Name %q, got %q", "cosca-enterprise", info.Name)
	}
	if info.Language != "go" {
		t.Errorf("expected Language %q, got %q", "go", info.Language)
	}
	if info.Version != "1.3.0" {
		t.Errorf("expected Version %q, got %q", "1.3.0", info.Version)
	}
	if len(info.Dependencies) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(info.Dependencies))
	}
	if !info.HasTests {
		t.Error("expected HasTests to be true")
	}
	if !info.HasDocker {
		t.Error("expected HasDocker to be true")
	}
	if info.Confidence != 0.95 {
		t.Errorf("expected Confidence %f, got %f", 0.95, info.Confidence)
	}
}

func TestDiscoverySDK_DiscoverProject_NotFound(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusNotFound, "NOT_FOUND", "no project detected in current directory")
	})

	_, err := c.Discovery.DiscoverProject()
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", coscaErr.StatusCode)
	}
}

func TestDiscoverySDK_DiscoverProject_InvalidJSON(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{broken`))
	})

	_, err := c.Discovery.DiscoverProject()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to decode") {
		t.Errorf("expected 'failed to decode', got %q", err.Error())
	}
}

func TestDiscoverySDK_DiscoverProject_ServerError(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := c.Discovery.DiscoverProject()
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestDiscoverySDK_DiscoverWorkspace(t *testing.T) {
	t.Parallel()

	expected := WorkspaceInfo{
		Root:        "/home/dev/projects/cosca",
		IDE:         "vscode",
		Shell:       "zsh",
		Terminal:    "kitty",
		OS:          "linux",
		Arch:        "amd64",
		HomeDir:     "/home/dev",
		TempDir:     "/tmp",
		EnvVars:     map[string]string{"EDITOR": "code", "GOPATH": "/home/dev/go"},
		OpenBuffers: []string{"main.go", "README.md", "config.yaml"},
		GitRoot:     "/home/dev/projects/cosca",
		GitBranch:   "main",
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/discovery/workspace")
		writeJSON(t, w, http.StatusOK, expected)
	})

	info, err := c.Discovery.DiscoverWorkspace()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if info.Root != "/home/dev/projects/cosca" {
		t.Errorf("expected Root %q, got %q", "/home/dev/projects/cosca", info.Root)
	}
	if info.IDE != "vscode" {
		t.Errorf("expected IDE %q, got %q", "vscode", info.IDE)
	}
	if info.Shell != "zsh" {
		t.Errorf("expected Shell %q, got %q", "zsh", info.Shell)
	}
	if info.OS != "linux" {
		t.Errorf("expected OS %q, got %q", "linux", info.OS)
	}
	if info.GitBranch != "main" {
		t.Errorf("expected GitBranch %q, got %q", "main", info.GitBranch)
	}
	if len(info.EnvVars) != 2 {
		t.Errorf("expected 2 env vars, got %d", len(info.EnvVars))
	}
	if len(info.OpenBuffers) != 3 {
		t.Errorf("expected 3 open buffers, got %d", len(info.OpenBuffers))
	}
	if info.HomeDir != "/home/dev" {
		t.Errorf("expected HomeDir %q, got %q", "/home/dev", info.HomeDir)
	}
}

func TestDiscoverySDK_DiscoverWorkspace_Error(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusBadRequest, "DETECTION_FAILED", "unable to determine workspace root")
	})

	_, err := c.Discovery.DiscoverWorkspace()
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.Code != "DETECTION_FAILED" {
		t.Errorf("expected Code %q, got %q", "DETECTION_FAILED", coscaErr.Code)
	}
}

func TestDiscoverySDK_DiscoverWorkspace_InvalidJSON(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{not valid`))
	})

	_, err := c.Discovery.DiscoverWorkspace()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to decode") {
		t.Errorf("expected 'failed to decode', got %q", err.Error())
	}
}

func TestDiscoverySDK_DiscoverEditor(t *testing.T) {
	t.Parallel()

	expected := EditorInfo{
		Name:         "Visual Studio Code",
		Version:      "1.92.0",
		Path:         "/usr/bin/code",
		PID:          12345,
		Connected:    true,
		Capabilities: []string{"completion", "diagnostics", "hover", "references"},
		Extensions:   []string{"go", "rust-analyzer", "copilot"},
		Language:     "en",
		Scheme:       "dark",
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/discovery/editor")
		writeJSON(t, w, http.StatusOK, expected)
	})

	info, err := c.Discovery.DiscoverEditor()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if info.Name != "Visual Studio Code" {
		t.Errorf("expected Name %q, got %q", "Visual Studio Code", info.Name)
	}
	if info.Version != "1.92.0" {
		t.Errorf("expected Version %q, got %q", "1.92.0", info.Version)
	}
	if !info.Connected {
		t.Error("expected Connected to be true")
	}
	if info.PID != 12345 {
		t.Errorf("expected PID %d, got %d", 12345, info.PID)
	}
	if len(info.Capabilities) != 4 {
		t.Errorf("expected 4 capabilities, got %d", len(info.Capabilities))
	}
	if len(info.Extensions) != 3 {
		t.Errorf("expected 3 extensions, got %d", len(info.Extensions))
	}
	if info.Scheme != "dark" {
		t.Errorf("expected Scheme %q, got %q", "dark", info.Scheme)
	}
}

func TestDiscoverySDK_DiscoverEditor_NotConnected(t *testing.T) {
	t.Parallel()

	expected := EditorInfo{
		Name:      "Neovim",
		Version:   "0.10.0",
		Path:      "/usr/bin/nvim",
		Connected: false,
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, expected)
	})

	info, err := c.Discovery.DiscoverEditor()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if info.Connected {
		t.Error("expected Connected to be false")
	}
	if info.Name != "Neovim" {
		t.Errorf("expected Name %q, got %q", "Neovim", info.Name)
	}
}

func TestDiscoverySDK_DiscoverEditor_Error(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusNotFound, "NOT_FOUND", "no editor detected")
	})

	_, err := c.Discovery.DiscoverEditor()
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.Code != "NOT_FOUND" {
		t.Errorf("expected Code %q, got %q", "NOT_FOUND", coscaErr.Code)
	}
}

func TestDiscoverySDK_DiscoverEditor_InvalidJSON(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{broken-json`))
	})

	_, err := c.Discovery.DiscoverEditor()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to decode") {
		t.Errorf("expected 'failed to decode', got %q", err.Error())
	}
}

func TestDiscoverySDK_DiscoverAll(t *testing.T) {
	t.Parallel()

	now := time.Now().Truncate(time.Second)
	expected := DiscoveryReport{
		Project: ProjectInfo{
			Name:       "my-project",
			Language:   "typescript",
			HasTests:   true,
			Confidence: 0.98,
		},
		Workspace: WorkspaceInfo{
			Root:    "/home/dev",
			OS:      "linux",
			HomeDir: "/home/dev",
			TempDir: "/tmp",
		},
		Editor: EditorInfo{
			Name:      "vscode",
			Connected: true,
		},
		CapturedAt: now,
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/discovery/all")
		writeJSON(t, w, http.StatusOK, expected)
	})

	report, err := c.Discovery.DiscoverAll()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if report.Project.Name != "my-project" {
		t.Errorf("expected project Name %q, got %q", "my-project", report.Project.Name)
	}
	if report.Workspace.OS != "linux" {
		t.Errorf("expected workspace OS %q, got %q", "linux", report.Workspace.OS)
	}
	if report.Editor.Name != "vscode" {
		t.Errorf("expected editor Name %q, got %q", "vscode", report.Editor.Name)
	}
	if !report.CapturedAt.Equal(now) {
		t.Errorf("expected CapturedAt %v, got %v", now, report.CapturedAt)
	}
}

func TestDiscoverySDK_DiscoverAll_ServerError(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := c.Discovery.DiscoverAll()
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestDiscoverySDK_DiscoverAll_InvalidJSON(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{invalid json!!`))
	})

	_, err := c.Discovery.DiscoverAll()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to decode") {
		t.Errorf("expected 'failed to decode', got %q", err.Error())
	}
}

func TestDiscoverySDK_DiscoverWorkspace_Minimal(t *testing.T) {
	t.Parallel()

	expected := WorkspaceInfo{
		Root:    "/tmp/test",
		OS:      "darwin",
		Arch:    "arm64",
		HomeDir: "/Users/test",
		TempDir: "/tmp",
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, expected)
	})

	info, err := c.Discovery.DiscoverWorkspace()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if info.OS != "darwin" {
		t.Errorf("expected OS %q, got %q", "darwin", info.OS)
	}
	if info.Arch != "arm64" {
		t.Errorf("expected Arch %q, got %q", "arm64", info.Arch)
	}
}
