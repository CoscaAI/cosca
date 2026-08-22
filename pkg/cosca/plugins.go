package cosca

import (
	"bytes"
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

// PluginType categorises the type of plugin.
type PluginType string

// Predefined plugin types.
const (
	PluginTypeTool       PluginType = "tool"
	PluginTypeHook       PluginType = "hook"
	PluginTypeTransport  PluginType = "transport"
	PluginTypeMiddleware PluginType = "middleware"
	PluginTypeStorage    PluginType = "storage"
	PluginTypeUI         PluginType = "ui"
)

// PluginPermission represents a permission a plugin may request.
type PluginPermission string

// Predefined plugin permissions.
const (
	PluginPermissionNetwork    PluginPermission = "network"
	PluginPermissionFilesystem PluginPermission = "filesystem"
	PluginPermissionExec       PluginPermission = "exec"
	PluginPermissionEnv        PluginPermission = "env"
	PluginPermissionDB         PluginPermission = "database"
	PluginPermissionSecrets    PluginPermission = "secrets"
)

// PluginInfo represents a plugin installed in the Cosca Runtime.
type PluginInfo struct {
	// ID is the unique plugin identifier.
	ID string `json:"id"`
	// Name is the plugin display name.
	Name string `json:"name"`
	// Version is the plugin semantic version.
	Version string `json:"version"`
	// Description explains the plugin's purpose.
	Description string `json:"description"`
	// Author is the plugin author.
	Author string `json:"author"`
	// License is the plugin license.
	License string `json:"license"`
	// Type is the plugin type.
	Type PluginType `json:"type"`
	// APIVersion is the Cosca API version this plugin targets.
	APIVersion string `json:"apiVersion"`
	// Status is the plugin load status (loaded, error, disabled, etc.).
	Status string `json:"status"`
	// Enabled indicates whether the plugin is active.
	Enabled bool `json:"enabled"`
	// Permissions lists the permissions the plugin requires.
	Permissions []PluginPermission `json:"permissions,omitempty"`
	// Entrypoint is the plugin entrypoint path.
	Entrypoint string `json:"entrypoint,omitempty"`
	// Runtime is the plugin runtime (wasm, native, js, py).
	Runtime string `json:"runtime,omitempty"`
	// Hooks lists lifecycle hooks the plugin implements.
	Hooks []string `json:"hooks,omitempty"`
	// Config is the plugin-specific configuration.
	Config map[string]interface{} `json:"config,omitempty"`
	// InstalledAt is when the plugin was installed.
	InstalledAt time.Time `json:"installedAt"`
}

// =============================================================================
// Request / Response types
// =============================================================================

type installPluginRequest struct {
	Source      string                 `json:"source"`
	Name        string                 `json:"name,omitempty"`
	Version     string                 `json:"version,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Permissions []PluginPermission     `json:"permissions,omitempty"`
}

type pluginListResponse struct {
	Plugins []PluginInfo `json:"plugins"`
	Total   int          `json:"total"`
}

// =============================================================================
// PluginSDK
// =============================================================================

// PluginSDK provides methods for managing Cosca Runtime plugins. It supports
// installing plugins from various sources, uninstalling, listing, and
// inspecting installed plugins.
type PluginSDK struct {
	client *Client
}

// InstallPlugin installs a plugin from the given source. The source can be
// a local file path, a URL to a plugin archive, or a registry package name.
func (s *PluginSDK) InstallPlugin(src string) error {
	if src == "" {
		return fmt.Errorf("plugin source is required")
	}

	body := installPluginRequest{Source: src}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal install request: %w", err)
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodPost,
		"/v1/plugins/install",
		bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.doRequest(req)
	if err != nil {
		return err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return s.decodeError(resp)
	}

	return nil
}

// UninstallPlugin removes a previously installed plugin by its identifier.
func (s *PluginSDK) UninstallPlugin(id string) error {
	if id == "" {
		return fmt.Errorf("plugin ID is required")
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodDelete,
		fmt.Sprintf("/v1/plugins/%s", id),
		nil,
	)
	if err != nil {
		return err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return s.decodeError(resp)
	}

	return nil
}

// ListPlugins returns all plugins currently installed in the runtime.
func (s *PluginSDK) ListPlugins() ([]PluginInfo, error) {
	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		"/v1/plugins",
		nil,
	)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, s.decodeError(resp)
	}

	var result pluginListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode plugin list: %w", err)
	}

	return result.Plugins, nil
}

// GetPlugin returns detailed information about a specific plugin by ID.
func (s *PluginSDK) GetPlugin(id string) (PluginInfo, error) {
	if id == "" {
		return PluginInfo{}, fmt.Errorf("plugin ID is required")
	}

	req, err := s.client.newRequest(
		context.Background(),
		http.MethodGet,
		fmt.Sprintf("/v1/plugins/%s", id),
		nil,
	)
	if err != nil {
		return PluginInfo{}, err
	}

	resp, err := s.client.doRequest(req)
	if err != nil {
		return PluginInfo{}, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return PluginInfo{}, s.decodeError(resp)
	}

	var info PluginInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return PluginInfo{}, fmt.Errorf("failed to decode plugin info: %w", err)
	}

	return info, nil
}

// decodeError reads an error response body and returns it as a formatted error.
func (s *PluginSDK) decodeError(resp *http.Response) error {
	return s.client.decodeError(resp)
}
