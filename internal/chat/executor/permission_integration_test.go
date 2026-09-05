// Package executor — permission gating integration tests. These verify that
// the allow/ask/deny ruleset (internal/permission) wired via SetPermission
// actually governs tool execution: deny blocks, ask fails closed, allow runs,
// scoped rules follow the most-specific-wins resolution, and ListTools hides
// globally-denied tools (additive, only when a ruleset is configured).
package executor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/permission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registerPermTool registers a mock tool with a valid schema and a success handler.
func registerPermTool(t *testing.T, reg *mockRegistry, name string) {
	t.Helper()
	reg.Register(&mockTool{
		name:        name,
		description: "test tool " + name,
		schema:      json.RawMessage(`{"type":"object"}`),
		executeFunc: succeedTool("ok:" + name),
	})
}

func TestExecute_Permission_NoRulesetIsFailOpen(t *testing.T) {
	reg := newMockRegistry()
	registerPermTool(t, reg, "write_file")
	exe := newTestExecutor(t, reg, &mockSandbox{available: false}, t.TempDir())
	// Do NOT call SetPermission → fail-open (current behavior).

	res, _ := exe.Execute(context.Background(), ToolCall{Name: "write_file", Input: map[string]interface{}{}})
	require.NotNil(t, res)
	assert.Equal(t, StatusSuccess, res.Status)
	assert.Equal(t, "ok:write_file", res.Output)
}

func TestExecute_Permission_DenyBlocks(t *testing.T) {
	reg := newMockRegistry()
	registerPermTool(t, reg, "write_file")
	exe := newTestExecutor(t, reg, &mockSandbox{available: false}, t.TempDir())
	exe.SetPermission(permission.Ruleset{{Permission: "edit", Pattern: "*", Action: permission.Deny}})

	res, _ := exe.Execute(context.Background(), ToolCall{Name: "write_file", Input: map[string]interface{}{}})
	require.NotNil(t, res)
	assert.Equal(t, StatusError, res.Status)
	assert.Contains(t, res.Error, "denied")
	// The tool's success handler must NOT have run.
	assert.NotEqual(t, "ok:write_file", res.Output)
}

func TestExecute_Permission_AllowRuns(t *testing.T) {
	reg := newMockRegistry()
	registerPermTool(t, reg, "write_file")
	exe := newTestExecutor(t, reg, &mockSandbox{available: false}, t.TempDir())
	exe.SetPermission(permission.Ruleset{{Permission: "edit", Pattern: "*", Action: permission.Allow}})

	res, _ := exe.Execute(context.Background(), ToolCall{Name: "write_file", Input: map[string]interface{}{}})
	require.NotNil(t, res)
	assert.Equal(t, StatusSuccess, res.Status)
	assert.Equal(t, "ok:write_file", res.Output)
}

func TestExecute_Permission_AskFailsClosed(t *testing.T) {
	reg := newMockRegistry()
	registerPermTool(t, reg, "shell")
	exe := newTestExecutor(t, reg, &mockSandbox{available: false}, t.TempDir())
	// shell maps to permission "shell"; an "ask" rule must fail closed (deny).
	exe.SetPermission(permission.Ruleset{{Permission: "shell", Pattern: "*", Action: permission.Ask}})

	res, _ := exe.Execute(context.Background(), ToolCall{Name: "shell", Input: map[string]interface{}{}})
	require.NotNil(t, res)
	assert.Equal(t, StatusError, res.Status)
	assert.Contains(t, res.Error, "ask")
	assert.Contains(t, res.Error, "fail-closed")
}

func TestExecute_Permission_ScopedRuleMostSpecificWins(t *testing.T) {
	reg := newMockRegistry()
	registerPermTool(t, reg, "write_file")
	exe := newTestExecutor(t, reg, &mockSandbox{available: false}, t.TempDir())
	exe.SetPermission(permission.Ruleset{
		{Permission: "edit", Pattern: "*", Action: permission.Deny},
		{Permission: "edit", Pattern: "*.go", Action: permission.Allow},
	})

	// "main.go" matches the specific allow → runs.
	res, _ := exe.Execute(context.Background(), ToolCall{Name: "write_file", Input: map[string]interface{}{"path": "main.go"}})
	require.NotNil(t, res)
	assert.Equal(t, StatusSuccess, res.Status, "expected allow for *.go")

	// "README.md" only matches the general deny → blocked.
	res2, _ := exe.Execute(context.Background(), ToolCall{Name: "write_file", Input: map[string]interface{}{"path": "README.md"}})
	require.NotNil(t, res2)
	assert.Equal(t, StatusError, res2.Status)
	assert.Contains(t, res2.Error, "denied")
}

func TestListTools_Permission_HidesGloballyDenied(t *testing.T) {
	reg := newMockRegistry()
	reg.definitions = []chat.ToolDefinition{
		{Type: "function", Function: chat.FunctionDef{Name: "read_file"}},
		{Type: "function", Function: chat.FunctionDef{Name: "write_file"}},
		{Type: "function", Function: chat.FunctionDef{Name: "shell"}},
	}

	exe := newTestExecutor(t, reg, &mockSandbox{available: false}, t.TempDir())
	// No ruleset → all tools visible.
	defs, err := exe.ListTools(context.Background())
	require.NoError(t, err)
	assert.Len(t, defs, 3)

	// Deny the "read" permission → read_file is hidden (read_file maps to read).
	exe.SetPermission(permission.Ruleset{{Permission: "read", Pattern: "*", Action: permission.Deny}})
	defs, err = exe.ListTools(context.Background())
	require.NoError(t, err)
	assert.Len(t, defs, 2)
	for _, d := range defs {
		assert.NotEqual(t, "read_file", d.Function.Name, "read_file should be hidden")
	}
}

func TestListTools_Permission_FailOpenWithoutRuleset(t *testing.T) {
	reg := newMockRegistry()
	reg.definitions = []chat.ToolDefinition{
		{Type: "function", Function: chat.FunctionDef{Name: "read_file"}},
		{Type: "function", Function: chat.FunctionDef{Name: "write_file"}},
	}
	exe := newTestExecutor(t, reg, &mockSandbox{available: false}, t.TempDir())
	// no SetPermission → fail-open, all visible
	defs, err := exe.ListTools(context.Background())
	require.NoError(t, err)
	assert.Len(t, defs, 2)
}
