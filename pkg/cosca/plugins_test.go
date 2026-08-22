package cosca

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// PluginSDK Tests
// =============================================================================

func TestPluginSDK_ListPlugins(t *testing.T) {
	t.Parallel()

	expected := []PluginInfo{
		{
			ID:          "plugin-1",
			Name:        "Git Integration",
			Version:     "1.2.0",
			Description: "Integrates Git version control with Cosca",
			Author:      "Cosca Team",
			License:     "MIT",
			Type:        PluginTypeTool,
			APIVersion:  "v1",
			Status:      "loaded",
			Enabled:     true,
			Permissions: []PluginPermission{PluginPermissionFilesystem, PluginPermissionExec},
			Entrypoint:  "plugins/git/main.wasm",
			Runtime:     "wasm",
			Hooks:       []string{"on_commit", "on_push"},
			Config: map[string]interface{}{
				"autoStage": true,
			},
			InstalledAt: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			ID:          "plugin-2",
			Name:        "Slack Notifier",
			Version:     "0.5.0",
			Description: "Sends notifications to Slack",
			Author:      "Community",
			License:     "Apache-2.0",
			Type:        PluginTypeHook,
			APIVersion:  "v1",
			Status:      "disabled",
			Enabled:     false,
			Permissions: []PluginPermission{PluginPermissionNetwork},
			InstalledAt: time.Date(2024, 3, 1, 14, 0, 0, 0, time.UTC),
		},
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/plugins")
		writeJSON(t, w, http.StatusOK, pluginListResponse{
			Plugins: expected,
			Total:   2,
		})
	})

	plugins, err := c.Plugin.ListPlugins()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(plugins))
	}

	p0 := plugins[0]
	if p0.ID != "plugin-1" {
		t.Errorf("expected ID %q, got %q", "plugin-1", p0.ID)
	}
	if p0.Name != "Git Integration" {
		t.Errorf("expected Name %q, got %q", "Git Integration", p0.Name)
	}
	if p0.Version != "1.2.0" {
		t.Errorf("expected Version %q, got %q", "1.2.0", p0.Version)
	}
	if p0.Type != PluginTypeTool {
		t.Errorf("expected Type %q, got %q", PluginTypeTool, p0.Type)
	}
	if !p0.Enabled {
		t.Error("expected Enabled to be true")
	}
	if len(p0.Permissions) != 2 {
		t.Errorf("expected 2 permissions, got %d", len(p0.Permissions))
	}
	if p0.Runtime != "wasm" {
		t.Errorf("expected Runtime %q, got %q", "wasm", p0.Runtime)
	}
	if len(p0.Hooks) != 2 {
		t.Errorf("expected 2 hooks, got %d", len(p0.Hooks))
	}

	p1 := plugins[1]
	if p1.Status != "disabled" {
		t.Errorf("expected Status %q, got %q", "disabled", p1.Status)
	}
	if p1.Enabled {
		t.Error("expected Enabled to be false")
	}
}

func TestPluginSDK_ListPlugins_Empty(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, pluginListResponse{
			Plugins: []PluginInfo{},
			Total:   0,
		})
	})

	plugins, err := c.Plugin.ListPlugins()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("expected empty list, got %d plugins", len(plugins))
	}
}

func TestPluginSDK_ListPlugins_Error(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusInternalServerError, "PLUGIN_ERROR", "failed to load plugins")
	})

	_, err := c.Plugin.ListPlugins()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPluginSDK_GetPlugin(t *testing.T) {
	t.Parallel()

	expected := PluginInfo{
		ID:          "plugin-dark-theme",
		Name:        "Dark Theme",
		Version:     "1.0.0",
		Description: "A beautiful dark theme for Cosca",
		Author:      "Design Team",
		License:     "MIT",
		Type:        PluginTypeUI,
		APIVersion:  "v1",
		Status:      "loaded",
		Enabled:     true,
		Config: map[string]interface{}{
			"colors": map[string]interface{}{
				"primary":   "#1a1a2e",
				"secondary": "#16213e",
			},
		},
		InstalledAt: time.Date(2024, 2, 10, 9, 0, 0, 0, time.UTC),
	}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodGet, "/v1/plugins/plugin-dark-theme")
		writeJSON(t, w, http.StatusOK, expected)
	})

	info, err := c.Plugin.GetPlugin("plugin-dark-theme")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if info.ID != "plugin-dark-theme" {
		t.Errorf("expected ID %q, got %q", "plugin-dark-theme", info.ID)
	}
	if info.Type != PluginTypeUI {
		t.Errorf("expected Type %q, got %q", PluginTypeUI, info.Type)
	}
	if info.Config == nil {
		t.Error("expected non-nil Config")
	}
}

func TestPluginSDK_GetPlugin_EmptyID(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	_, err := c.Plugin.GetPlugin("")
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
	if !strings.Contains(err.Error(), "plugin ID is required") {
		t.Errorf("expected 'plugin ID is required', got %q", err.Error())
	}
}

func TestPluginSDK_GetPlugin_NotFound(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusNotFound, "NOT_FOUND", "plugin not found")
	})

	_, err := c.Plugin.GetPlugin("nonexistent")
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", coscaErr.StatusCode)
	}
}

func TestPluginSDK_GetPlugin_InvalidJSON(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{bad json`))
	})

	_, err := c.Plugin.GetPlugin("test-id")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to decode") {
		t.Errorf("expected 'failed to decode', got %q", err.Error())
	}
}

func TestPluginSDK_InstallPlugin(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodPost, "/v1/plugins/install")

		var req installPluginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			return
		}
		if req.Source != "https://plugins.cosca.dev/git-latest.wasm" {
			t.Errorf("expected Source %q, got %q", "https://plugins.cosca.dev/git-latest.wasm", req.Source)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", r.Header.Get("Content-Type"))
		}

		w.WriteHeader(http.StatusCreated)
	})

	err := c.Plugin.InstallPlugin("https://plugins.cosca.dev/git-latest.wasm")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestPluginSDK_InstallPlugin_OK(t *testing.T) {
	t.Parallel()

	// HTTP 200 should also be accepted.
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	err := c.Plugin.InstallPlugin("file://plugins/demo.wasm")
	if err != nil {
		t.Fatalf("expected no error for 200, got %v", err)
	}
}

func TestPluginSDK_InstallPlugin_EmptySource(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	err := c.Plugin.InstallPlugin("")
	if err == nil {
		t.Fatal("expected error for empty source")
	}
	if !strings.Contains(err.Error(), "plugin source is required") {
		t.Errorf("expected 'plugin source is required', got %q", err.Error())
	}
}

func TestPluginSDK_InstallPlugin_BadRequest(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusBadRequest, "INVALID_SOURCE", "source format not supported")
	})

	err := c.Plugin.InstallPlugin("invalid-scheme://bad")
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", coscaErr.StatusCode)
	}
}

func TestPluginSDK_UninstallPlugin(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assertMethodPath(t, r, http.MethodDelete, "/v1/plugins/plugin-to-remove")
		w.WriteHeader(http.StatusOK)
	})

	err := c.Plugin.UninstallPlugin("plugin-to-remove")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestPluginSDK_UninstallPlugin_NoContent(t *testing.T) {
	t.Parallel()

	// HTTP 204 should also be accepted.
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	err := c.Plugin.UninstallPlugin("clean-remove")
	if err != nil {
		t.Fatalf("expected no error for 204, got %v", err)
	}
}

func TestPluginSDK_UninstallPlugin_EmptyID(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called")
	})

	err := c.Plugin.UninstallPlugin("")
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
	if !strings.Contains(err.Error(), "plugin ID is required") {
		t.Errorf("expected 'plugin ID is required', got %q", err.Error())
	}
}

func TestPluginSDK_UninstallPlugin_NotFound(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeErrorJSON(t, w, http.StatusNotFound, "NOT_FOUND", "plugin not installed")
	})

	err := c.Plugin.UninstallPlugin("ghost-plugin")
	coscaErr, ok := err.(*CoscaError)
	if !ok {
		t.Fatalf("expected *CoscaError, got %T: %v", err, err)
	}
	if coscaErr.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", coscaErr.StatusCode)
	}
}

func TestPluginSDK_InstallPlugin_ServerError(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	err := c.Plugin.InstallPlugin("https://example.com/plugin.wasm")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// =============================================================================
// Plugin Types and Constants
// =============================================================================

func TestPluginTypeConstants(t *testing.T) {
	t.Parallel()

	if PluginTypeTool != "tool" {
		t.Errorf("expected PluginTypeTool 'tool', got %q", PluginTypeTool)
	}
	if PluginTypeHook != "hook" {
		t.Errorf("expected PluginTypeHook 'hook', got %q", PluginTypeHook)
	}
	if PluginTypeTransport != "transport" {
		t.Errorf("expected PluginTypeTransport 'transport', got %q", PluginTypeTransport)
	}
	if PluginTypeMiddleware != "middleware" {
		t.Errorf("expected PluginTypeMiddleware 'middleware', got %q", PluginTypeMiddleware)
	}
	if PluginTypeStorage != "storage" {
		t.Errorf("expected PluginTypeStorage 'storage', got %q", PluginTypeStorage)
	}
	if PluginTypeUI != "ui" {
		t.Errorf("expected PluginTypeUI 'ui', got %q", PluginTypeUI)
	}
}

func TestPluginPermissionConstants(t *testing.T) {
	t.Parallel()

	if PluginPermissionNetwork != "network" {
		t.Errorf("expected PluginPermissionNetwork 'network', got %q", PluginPermissionNetwork)
	}
	if PluginPermissionFilesystem != "filesystem" {
		t.Errorf("expected PluginPermissionFilesystem 'filesystem', got %q", PluginPermissionFilesystem)
	}
	if PluginPermissionExec != "exec" {
		t.Errorf("expected PluginPermissionExec 'exec', got %q", PluginPermissionExec)
	}
	if PluginPermissionEnv != "env" {
		t.Errorf("expected PluginPermissionEnv 'env', got %q", PluginPermissionEnv)
	}
	if PluginPermissionDB != "database" {
		t.Errorf("expected PluginPermissionDB 'database', got %q", PluginPermissionDB)
	}
	if PluginPermissionSecrets != "secrets" {
		t.Errorf("expected PluginPermissionSecrets 'secrets', got %q", PluginPermissionSecrets)
	}
}

func TestPluginType_String(t *testing.T) {
	t.Parallel()

	// Verify PluginType can be used as a string
	pt := PluginTypeTool
	if string(pt) != "tool" {
		t.Errorf("expected 'tool', got %q", string(pt))
	}
}
