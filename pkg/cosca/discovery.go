package cosca

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/CoscaAI/cosca/internal/safe"
)

// =============================================================================
// Types
// =============================================================================

// DiscoveryReport aggregates all discovery information into a single report.
type DiscoveryReport struct {
	// Project contains detected project information.
	Project ProjectInfo `json:"project"`
	// Workspace contains detected workspace information.
	Workspace WorkspaceInfo `json:"workspace"`
	// Editor contains detected editor information.
	Editor EditorInfo `json:"editor"`
	// CapturedAt is when the discovery was performed.
	CapturedAt time.Time `json:"capturedAt"`
}

// ProjectInfo describes a detected project.
type ProjectInfo struct {
	// Name is the project name (from package.json, go.mod, etc.).
	Name string `json:"name"`
	// Path is the absolute path to the project root.
	Path string `json:"path"`
	// Language is the primary programming language.
	Language string `json:"language"`
	// Framework is the detected framework, if any.
	Framework string `json:"framework,omitempty"`
	// Version is the project version.
	Version string `json:"version,omitempty"`
	// BuildSystem is the detected build system (maven, npm, go, etc.).
	BuildSystem string `json:"buildSystem,omitempty"`
	// Dependencies lists detected dependencies.
	Dependencies []string `json:"dependencies,omitempty"`
	// Entrypoint is the project entrypoint file.
	Entrypoint string `json:"entrypoint,omitempty"`
	// ConfigFiles lists detected configuration files.
	ConfigFiles []string `json:"configFiles,omitempty"`
	// HasTests indicates whether the project has test files.
	HasTests bool `json:"hasTests"`
	// HasDocker indicates whether the project has Docker config.
	HasDocker bool `json:"hasDocker"`
	// HasCI indicates whether the project has CI configuration.
	HasCI bool `json:"hasCI"`
	// Confidence is the detection confidence (0.0 - 1.0).
	Confidence float64 `json:"confidence"`
}

// WorkspaceInfo describes a detected workspace environment.
type WorkspaceInfo struct {
	// Root is the workspace root directory.
	Root string `json:"root"`
	// IDE is the detected IDE (vscode, intellij, vim, etc.).
	IDE string `json:"ide,omitempty"`
	// Shell is the detected default shell.
	Shell string `json:"shell,omitempty"`
	// Terminal is the detected terminal emulator.
	Terminal string `json:"terminal,omitempty"`
	// OS is the operating system.
	OS string `json:"os"`
	// Arch is the system architecture.
	Arch string `json:"arch"`
	// HomeDir is the user's home directory.
	HomeDir string `json:"homeDir"`
	// TempDir is the system temp directory.
	TempDir string `json:"tempDir"`
	// EnvVars are relevant environment variables.
	EnvVars map[string]string `json:"envVars,omitempty"`
	// OpenBuffers lists currently open files in the editor.
	OpenBuffers []string `json:"openBuffers,omitempty"`
	// GitRoot is the git repository root, if applicable.
	GitRoot string `json:"gitRoot,omitempty"`
	// GitBranch is the current git branch.
	GitBranch string `json:"gitBranch,omitempty"`
}

// EditorInfo describes a detected editor or IDE.
type EditorInfo struct {
	// Name is the editor name (vscode, neovim, intellij, etc.).
	Name string `json:"name"`
	// Version is the editor version.
	Version string `json:"version,omitempty"`
	// Path is the editor executable path.
	Path string `json:"path,omitempty"`
	// PID is the editor process ID, if running.
	PID int `json:"pid,omitempty"`
	// Connected indicates whether the editor is connected to the runtime.
	Connected bool `json:"connected"`
	// Capabilities lists editor capabilities (completion, diagnostics, etc.).
	Capabilities []string `json:"capabilities,omitempty"`
	// Extensions lists detected editor extensions/plugins.
	Extensions []string `json:"extensions,omitempty"`
	// Language is the editor's display language.
	Language string `json:"language,omitempty"`
	// Scheme is the colour theme scheme.
	Scheme string `json:"scheme,omitempty"`
}

// =============================================================================
// Response types
// =============================================================================

// =============================================================================
// DiscoverySDK
// =============================================================================

// DiscoverySDK provides methods for auto-detecting information about the
// current project, workspace, and editor environment. The discovery system
// analyses the file system, process list, and environment variables to
// build a comprehensive picture of the development context.
type DiscoverySDK struct {
	client *Client
}

// DiscoverProject detects and returns information about the current project
// by analysing the file system for project markers (go.mod, package.json,
// Cargo.toml, etc.) and build configuration.
func (s *DiscoverySDK) DiscoverProject() (ProjectInfo, error) {
	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		"/v1/discovery/project",
		nil,
	)
	if err != nil {
		return ProjectInfo{}, err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return ProjectInfo{}, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return ProjectInfo{}, s.decodeError(resp)
	}

	var info ProjectInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return ProjectInfo{}, fmt.Errorf("failed to decode project info: %w", err)
	}

	return info, nil
}

// DiscoverWorkspace detects and returns information about the current
// workspace environment including OS, shell, terminal, and git state.
func (s *DiscoverySDK) DiscoverWorkspace() (WorkspaceInfo, error) {
	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		"/v1/discovery/workspace",
		nil,
	)
	if err != nil {
		return WorkspaceInfo{}, err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return WorkspaceInfo{}, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return WorkspaceInfo{}, s.decodeError(resp)
	}

	var info WorkspaceInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return WorkspaceInfo{}, fmt.Errorf("failed to decode workspace info: %w", err)
	}

	return info, nil
}

// DiscoverEditor detects and returns information about the connected editor
// or IDE, including its name, version, capabilities, and connection state.
func (s *DiscoverySDK) DiscoverEditor() (EditorInfo, error) {
	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		"/v1/discovery/editor",
		nil,
	)
	if err != nil {
		return EditorInfo{}, err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return EditorInfo{}, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return EditorInfo{}, s.decodeError(resp)
	}

	var info EditorInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return EditorInfo{}, fmt.Errorf("failed to decode editor info: %w", err)
	}

	return info, nil
}

// DiscoverAll runs all discovery methods and returns a complete
// DiscoveryReport containing project, workspace, and editor information.
func (s *DiscoverySDK) DiscoverAll() (DiscoveryReport, error) {
	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		"/v1/discovery/all",
		nil,
	)
	if err != nil {
		return DiscoveryReport{}, err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return DiscoveryReport{}, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return DiscoveryReport{}, s.decodeError(resp)
	}

	var report DiscoveryReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		return DiscoveryReport{}, fmt.Errorf("failed to decode discovery report: %w", err)
	}

	return report, nil
}

// decodeError reads an error response body and returns it as a formatted error.
func (s *DiscoverySDK) decodeError(resp *http.Response) error {
	return s.client.decodeError(resp)
}
