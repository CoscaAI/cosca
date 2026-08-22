package genericmcp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/editors/types"
)

func TestCoscaToolDefinitionsIncludeKernelIdentity(t *testing.T) {
	tools := CoscaToolDefinitions()

	var kernelTool *MCPToolDefinition
	for i := range tools {
		if tools[i].Name == "cosca_kernel_identity" {
			kernelTool = &tools[i]
			break
		}
	}
	if kernelTool == nil {
		t.Fatalf("tool cosca_kernel_identity not found in CoscaToolDefinitions")
	}

	if want := "Carregar o Cosca Kernel — identity, leis, constituição"; kernelTool.Description != want {
		t.Errorf("description = %q, want %q", kernelTool.Description, want)
	}
	if kernelTool.Command != "cosca" {
		t.Errorf("command = %q, want %q", kernelTool.Command, "cosca")
	}
	if len(kernelTool.Args) != 2 || kernelTool.Args[0] != "kernel" || kernelTool.Args[1] != "identity" {
		t.Errorf("args = %v, want [kernel identity]", kernelTool.Args)
	}
	if kernelTool.InputSchema.Type != "object" {
		t.Errorf("inputSchema.type = %q, want %q", kernelTool.InputSchema.Type, "object")
	}
}

func TestGenerateMCPServerDefinitionIncludesKernelIdentity(t *testing.T) {
	def := GenerateMCPServerDefinition()

	found := false
	for _, tool := range def.Tools {
		if tool.Name == "cosca_kernel_identity" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("cosca_kernel_identity missing from MCP server definition tools")
	}

	data, err := json.MarshalIndent(def, "", "  ")
	if err != nil {
		t.Fatalf("marshal MCP server definition: %v", err)
	}
	if !json.Valid(data) {
		t.Fatal("MCP server definition is not valid JSON")
	}
	if !containsJSON(data, "cosca_kernel_identity") {
		t.Error("serialized server definition missing cosca_kernel_identity")
	}
}

func TestSetupWritesServerDefinitionWithKernelIdentity(t *testing.T) {
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

	found := false
	for _, tool := range serverDef.Tools {
		if tool.Name == "cosca_kernel_identity" {
			found = true
			if tool.Command != "cosca" || len(tool.Args) != 2 || tool.Args[0] != "kernel" || tool.Args[1] != "identity" {
				t.Errorf("kernel identity tool command/args = %q %v, want cosca [kernel identity]", tool.Command, tool.Args)
			}
			break
		}
	}
	if !found {
		t.Error("cosca_kernel_identity missing from written cosca-server.json")
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
