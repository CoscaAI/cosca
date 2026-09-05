package genericmcp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/editors/types"
)

// TestCoscaToolDefinitionsAlignedWithServer verifica que as tools espelhadas
// são exatamente as 11 tools reais do servidor MCP canônico (internal/mcpserver
// registry.go). Ferramenta antiga/inexistente aqui = "tool not found" para quem
// ler o cosca-server.json — nunca pode divergir do servidor real.
func TestCoscaToolDefinitionsAlignedWithServer(t *testing.T) {
	tools := CoscaToolDefinitions()

	if len(tools) != 11 {
		t.Fatalf("expected 11 tools (mirror of internal/mcpserver), got %d", len(tools))
	}

	// Conjunto canônico do servidor (registry.go registerTools).
	want := []string{
		"cosca.recall", "cosca.context", "cosca.learn", "cosca.observe",
		"cosca.reason", "cosca.trace", "cosca.project", "cosca.cost",
		"cosca.cli", "cosca.self", "cosca.web",
	}

	got := make(map[string]bool)
	for _, tool := range tools {
		if tool.Name == "" {
			t.Error("tool with empty name in CoscaToolDefinitions")
		}
		if tool.Description == "" {
			t.Errorf("tool %q has empty description", tool.Name)
		}
		if tool.InputSchema.Type != "object" {
			t.Errorf("tool %q inputSchema.type = %q, want object", tool.Name, tool.InputSchema.Type)
		}
		got[tool.Name] = true
	}

	for _, name := range want {
		if !got[name] {
			t.Errorf("tool %q missing from CoscaToolDefinitions — divergiu do servidor real", name)
		}
	}

	// Nenhuma tool antiga pode sobreviver.
	for _, name := range []string{"cosca_search", "cosca_index", "cosca_context", "cosca_status", "cosca_memory", "cosca_kernel_identity"} {
		if got[name] {
			t.Errorf("tool legada %q ainda presente — remover: servidor real não a implementa", name)
		}
	}
}

func TestGenerateMCPServerDefinitionAlignedWithServer(t *testing.T) {
	def := GenerateMCPServerDefinition()

	if len(def.Tools) != 11 {
		t.Fatalf("expected 11 tools in server definition, got %d", len(def.Tools))
	}

	data, err := json.MarshalIndent(def, "", "  ")
	if err != nil {
		t.Fatalf("marshal MCP server definition: %v", err)
	}
	if !json.Valid(data) {
		t.Fatal("MCP server definition is not valid JSON")
	}
	if !bytes.Contains(data, []byte("cosca.recall")) {
		t.Error("serialized server definition missing cosca.recall")
	}
	if bytes.Contains(data, []byte("cosca_kernel_identity")) {
		t.Error("serialized server definition still contains legacy tool cosca_kernel_identity")
	}
}

func TestSetupWritesServerDefinitionAlignedWithServer(t *testing.T) {
	proj := t.TempDir()
	a := NewAdapter()
	cfg := types.DefaultEditorConfig(proj)

	if err := a.Setup(cfg); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	serverDefPath := filepath.Join(proj, ".mcp", "cosca-server.json")
	data, err := os.ReadFile(serverDefPath)
	if err != nil {
		t.Fatalf("cosca-server.json not created: %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("cosca-server.json is not valid JSON:\n%s", data)
	}

	var serverDef MCPServerDefinition
	if err := json.Unmarshal(data, &serverDef); err != nil {
		t.Fatalf("unmarshal cosca-server.json: %v", err)
	}

	if len(serverDef.Tools) != 11 {
		t.Errorf("cosca-server.json tools = %d, want 11 (mirror of internal/mcpserver)", len(serverDef.Tools))
	}

	// mcp.json client config must still be present.
	if _, err := os.Stat(filepath.Join(proj, ".mcp", "mcp.json")); err != nil {
		t.Errorf("mcp.json not created: %v", err)
	}
}

func containsJSON(data []byte, needle string) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return false
	}
	tools, ok := raw["tools"]
	if !ok {
		return false
	}
	return len(tools) > 0 && json.Valid(tools) && bytes.Contains(tools, []byte(needle))
}

func TestVersion(t *testing.T) {
	a := NewAdapter()
	v, err := a.Version()
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v == "" {
		t.Error("Version should not be empty")
	}
}

func TestInfo(t *testing.T) {
	a := NewAdapter()
	info, err := a.Info()
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name == "" {
		t.Error("Info.Name should not be empty")
	}
}

func TestDetect_NotFound(t *testing.T) {
	a := NewAdapter()
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	t.Cleanup(func() { os.Chdir(origDir) })
	found, err := a.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	_ = found
}
