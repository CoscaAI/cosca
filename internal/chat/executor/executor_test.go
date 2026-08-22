// Package executor tests the Tool Executor — the central orchestrator that
// combines the ToolRegistry with the Sandbox Gate to execute tool calls with
// proper sandbox enforcement.
package executor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Mock Implementations ─────────────────────────────────────────────────

// mockTool implements chat.Tool for testing. Each instance can be configured
// with custom name, description, schema, execute behaviour, and validation
// behaviour.
type mockTool struct {
	name         string
	description  string
	schema       json.RawMessage
	executeFunc  func(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error)
	validateFunc func(params json.RawMessage) error
}

func (m *mockTool) Name() string            { return m.name }
func (m *mockTool) Description() string     { return m.description }
func (m *mockTool) Schema() json.RawMessage { return m.schema }

func (m *mockTool) Execute(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, params)
	}
	return &chat.ToolResult{Output: "mock output"}, nil
}

func (m *mockTool) Validate(params json.RawMessage) error {
	if m.validateFunc != nil {
		return m.validateFunc(params)
	}
	return nil
}

// mockRegistry implements executor.ToolRegistry for testing. It stores tools
// in a map keyed by name and provides configurable Definitions.
type mockRegistry struct {
	mu          sync.Mutex
	tools       map[string]chat.Tool
	definitions []chat.ToolDefinition
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		tools: make(map[string]chat.Tool),
	}
}

func (r *mockRegistry) Register(t chat.Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

func (r *mockRegistry) Get(name string) chat.Tool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tools[name]
}

func (r *mockRegistry) List() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

func (r *mockRegistry) GetAll() []chat.Tool {
	r.mu.Lock()
	defer r.mu.Unlock()
	tools := make([]chat.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

func (r *mockRegistry) Execute(ctx context.Context, name string, params json.RawMessage) *chat.ToolResult {
	r.mu.Lock()
	t, ok := r.tools[name]
	r.mu.Unlock()
	if !ok {
		return &chat.ToolResult{Error: "tool not found: " + name}
	}
	result, err := t.Execute(ctx, params)
	if err != nil {
		return &chat.ToolResult{Error: err.Error()}
	}
	return result
}

func (r *mockRegistry) Definitions() []chat.ToolDefinition {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.definitions
}

// mockSandbox implements executor.SandboxGate for testing.
type mockSandbox struct {
	available   bool
	executeFunc func(ctx context.Context, cmd chat.Command, mode chat.SandboxMode) (*chat.SandboxResult, error)
}

func (s *mockSandbox) IsAvailable() bool { return s.available }

func (s *mockSandbox) Execute(ctx context.Context, cmd chat.Command, mode chat.SandboxMode) (*chat.SandboxResult, error) {
	if s.executeFunc != nil {
		return s.executeFunc(ctx, cmd, mode)
	}
	return &chat.SandboxResult{Stdout: "mock sandbox output"}, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────

// newTestExecutor creates an Executor with the given dependencies for testing.
// If registry is nil, a default empty mockRegistry is used.
// If sandboxGate is nil, it is passed as-is (nil sandbox).
// If workspace is empty, a temp dir is created.
func newTestExecutor(t *testing.T, registry ToolRegistry, sandboxGate SandboxGate, workspace string) *Executor {
	t.Helper()
	if registry == nil {
		registry = newMockRegistry()
	}
	if workspace == "" {
		workspace = t.TempDir()
	}
	return New(registry, sandboxGate, workspace)
}

// registerTool is a convenience helper to add a mock tool to a mock registry.
func registerTool(t *testing.T, registry *mockRegistry, tool *mockTool) {
	t.Helper()
	registry.Register(tool)
}

// succeedTool returns an executeFunc that returns a successful result with the
// given output string.
func succeedTool(output string) func(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	return func(_ context.Context, _ json.RawMessage) (*chat.ToolResult, error) {
		return &chat.ToolResult{Output: output}, nil
	}
}

// succeedToolWithParams returns an executeFunc that captures the raw params so
// tests can inspect what was passed to the tool.
func succeedToolWithParams(capture *json.RawMessage) func(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	return func(_ context.Context, params json.RawMessage) (*chat.ToolResult, error) {
		*capture = params
		return &chat.ToolResult{Output: "ok"}, nil
	}
}

// failTool returns an executeFunc that returns the given error.
func failTool(err error) func(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	return func(_ context.Context, _ json.RawMessage) (*chat.ToolResult, error) {
		return nil, err
	}
}

// failToolWithResult returns an executeFunc that returns a result with
// a non-empty Error field (no top-level error).
func failToolWithResult(errMsg string) func(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	return func(_ context.Context, _ json.RawMessage) (*chat.ToolResult, error) {
		return &chat.ToolResult{Error: errMsg}, nil
	}
}

// ─── Test: New ────────────────────────────────────────────────────────────

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("creates executor with dependencies", func(t *testing.T) {
		t.Parallel()

		reg := newMockRegistry()
		sbox := &mockSandbox{available: false}
		ws := t.TempDir()

		exe := New(reg, sbox, ws)
		require.NotNil(t, exe)
		assert.NotNil(t, exe.registry)
		assert.NotNil(t, exe.sandbox)
		assert.NotNil(t, exe.rails)
		assert.Equal(t, ws, exe.workspace)
	})

	t.Run("creates executor with nil sandbox", func(t *testing.T) {
		t.Parallel()

		reg := newMockRegistry()
		ws := t.TempDir()

		exe := New(reg, nil, ws)
		require.NotNil(t, exe)
		assert.NotNil(t, exe.registry)
		assert.Nil(t, exe.sandbox)
		assert.NotNil(t, exe.rails)
	})

	t.Run("creates executor with empty workspace", func(t *testing.T) {
		t.Parallel()

		reg := newMockRegistry()
		exe := New(reg, nil, "")
		require.NotNil(t, exe)
		assert.Equal(t, "", exe.workspace)
		// Rails is still created with an empty workspace.
		assert.NotNil(t, exe.rails)
	})
}

// ─── Test: Execute — Success Path ─────────────────────────────────────────

func TestExecute_Success(t *testing.T) {
	t.Parallel()

	t.Run("executes tool and returns success", func(t *testing.T) {
		t.Parallel()

		reg := newMockRegistry()
		registerTool(t, reg, &mockTool{
			name:        "test_tool",
			description: "a test tool",
			executeFunc: succeedTool("hello world"),
		})

		exe := newTestExecutor(t, reg, nil, "")
		result, err := exe.Execute(context.Background(), ToolCall{
			ID:    "call_1",
			Name:  "test_tool",
			Input: map[string]interface{}{"foo": "bar"},
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "call_1", result.ToolCallID)
		assert.Equal(t, StatusSuccess, result.Status)
		assert.Equal(t, "hello world", result.Output)
		assert.Empty(t, result.Error)
		assert.GreaterOrEqual(t, result.DurationMs, int64(0))
	})

	t.Run("passes marshaled input to tool", func(t *testing.T) {
		t.Parallel()

		var capturedParams json.RawMessage
		reg := newMockRegistry()
		registerTool(t, reg, &mockTool{
			name:        "echo",
			executeFunc: succeedToolWithParams(&capturedParams),
		})

		exe := newTestExecutor(t, reg, nil, "")
		input := map[string]interface{}{"key": "value", "num": float64(42)}
		_, err := exe.Execute(context.Background(), ToolCall{
			ID:    "call_2",
			Name:  "echo",
			Input: input,
		})
		require.NoError(t, err)

		var decoded map[string]interface{}
		err = json.Unmarshal(capturedParams, &decoded)
		require.NoError(t, err)
		assert.Equal(t, "value", decoded["key"])
		assert.Equal(t, float64(42), decoded["num"])
	})

	t.Run("nil tool result yields empty output", func(t *testing.T) {
		t.Parallel()

		reg := newMockRegistry()
		registerTool(t, reg, &mockTool{
			name: "nil_result",
			executeFunc: func(_ context.Context, _ json.RawMessage) (*chat.ToolResult, error) {
				return nil, nil
			},
		})

		exe := newTestExecutor(t, reg, nil, "")
		result, err := exe.Execute(context.Background(), ToolCall{
			ID:    "call_3",
			Name:  "nil_result",
			Input: map[string]interface{}{},
		})
		require.NoError(t, err)
		assert.Equal(t, StatusSuccess, result.Status)
		// When result is nil, Output defaults to "".
		assert.Equal(t, "", result.Output)
	})
}

// ─── Test: Execute — Error Paths ──────────────────────────────────────────

func TestExecute_ToolNotFound(t *testing.T) {
	t.Parallel()

	reg := newMockRegistry()
	exe := newTestExecutor(t, reg, nil, "")
	result, err := exe.Execute(context.Background(), ToolCall{
		ID:    "call_1",
		Name:  "nonexistent",
		Input: map[string]interface{}{},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "call_1", result.ToolCallID)
	assert.Equal(t, StatusError, result.Status)
	assert.Contains(t, result.Error, "tool not found")
}

func TestExecute_EmptyName(t *testing.T) {
	t.Parallel()

	reg := newMockRegistry()
	exe := newTestExecutor(t, reg, nil, "")
	result, err := exe.Execute(context.Background(), ToolCall{
		ID:    "call_1",
		Name:  "",
		Input: map[string]interface{}{},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, StatusError, result.Status)
	assert.Contains(t, result.Error, "tool name is required")
}

func TestExecute_NilInput(t *testing.T) {
	t.Parallel()

	reg := newMockRegistry()
	registerTool(t, reg, &mockTool{name: "my_tool"})
	exe := newTestExecutor(t, reg, nil, "")
	result, err := exe.Execute(context.Background(), ToolCall{
		ID:    "call_1",
		Name:  "my_tool",
		Input: nil,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, StatusError, result.Status)
	assert.Contains(t, result.Error, "input is required")
}

func TestExecute_InvalidSchema(t *testing.T) {
	t.Parallel()

	reg := newMockRegistry()
	registerTool(t, reg, &mockTool{
		name: "validated_tool",
		validateFunc: func(_ json.RawMessage) error {
			return errors.New("schema violation: missing required field 'x'")
		},
	})

	exe := newTestExecutor(t, reg, nil, "")
	result, err := exe.Execute(context.Background(), ToolCall{
		ID:    "call_1",
		Name:  "validated_tool",
		Input: map[string]interface{}{"y": 1},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, StatusError, result.Status)
	assert.Contains(t, result.Error, "validation failed")
}

func TestExecute_PathValidation(t *testing.T) {
	t.Parallel()

	ws := t.TempDir()
	reg := newMockRegistry()
	registerTool(t, reg, &mockTool{
		name: "read_tool",
		validateFunc: func(_ json.RawMessage) error {
			return nil // schema passes, path validation is done by executor
		},
	})

	exe := newTestExecutor(t, reg, nil, ws)

	t.Run("path outside workspace fails", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			// "/etc/passwd" não é um path absoluto no Windows (falta o drive);
			// o Rails resolve contra o workspace e o escape não é detectado.
			t.Skip("semântica de path absoluto POSIX — não aplicável no Windows")
		}

		// Use an absolute path outside the temp workspace.
		outsidePath := "/etc/passwd"
		result, err := exe.Execute(context.Background(), ToolCall{
			ID:   "call_1",
			Name: "read_tool",
			Input: map[string]interface{}{
				"path": outsidePath,
			},
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, StatusError, result.Status)
		assert.Contains(t, result.Error, "path validation")
		assert.Contains(t, result.Error, "escapes workspace")
	})

	t.Run("path inside workspace succeeds", func(t *testing.T) {
		t.Parallel()

		insidePath := "some/file.txt"
		result, err := exe.Execute(context.Background(), ToolCall{
			ID:   "call_2",
			Name: "read_tool",
			Input: map[string]interface{}{
				"path": insidePath,
			},
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, StatusSuccess, result.Status)
	})

	t.Run("no path field skips validation", func(t *testing.T) {
		t.Parallel()

		result, err := exe.Execute(context.Background(), ToolCall{
			ID:   "call_3",
			Name: "read_tool",
			Input: map[string]interface{}{
				"content": "hello",
			},
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, StatusSuccess, result.Status)
	})
}

// ─── Test: Execute — Context Deadline / Cancel ────────────────────────────

func TestExecute_ContextDeadline(t *testing.T) {
	t.Parallel()

	reg := newMockRegistry()
	registerTool(t, reg, &mockTool{
		name: "slow_tool",
		executeFunc: func(_ context.Context, _ json.RawMessage) (*chat.ToolResult, error) {
			return nil, context.DeadlineExceeded
		},
	})

	exe := newTestExecutor(t, reg, nil, "")
	result, err := exe.Execute(context.Background(), ToolCall{
		ID:    "call_1",
		Name:  "slow_tool",
		Input: map[string]interface{}{},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, StatusTimeout, result.Status)
	assert.Contains(t, result.Error, "context deadline exceeded")
}

func TestExecute_ContextCancel(t *testing.T) {
	t.Parallel()

	reg := newMockRegistry()
	registerTool(t, reg, &mockTool{
		name: "slow_tool",
		executeFunc: func(_ context.Context, _ json.RawMessage) (*chat.ToolResult, error) {
			return nil, context.Canceled
		},
	})

	exe := newTestExecutor(t, reg, nil, "")
	result, err := exe.Execute(context.Background(), ToolCall{
		ID:    "call_1",
		Name:  "slow_tool",
		Input: map[string]interface{}{},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, StatusTimeout, result.Status)
	assert.Contains(t, result.Error, "context canceled")
}

func TestExecute_ToolErrorInResult(t *testing.T) {
	t.Parallel()

	reg := newMockRegistry()
	registerTool(t, reg, &mockTool{
		name:        "fragile_tool",
		executeFunc: failToolWithResult("something went wrong inside the tool"),
	})

	exe := newTestExecutor(t, reg, nil, "")
	result, err := exe.Execute(context.Background(), ToolCall{
		ID:    "call_1",
		Name:  "fragile_tool",
		Input: map[string]interface{}{},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, StatusError, result.Status)
	assert.Equal(t, "something went wrong inside the tool", result.Error)
}

// ─── Test: Execute — Sandbox Enforcement ──────────────────────────────────

func TestExecute_SandboxAvailable(t *testing.T) {
	t.Parallel()

	ws := t.TempDir()

	t.Run("sandbox denies path-containing call", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			// O padrão "/etc/passwd" não é absoluto no Windows (sem drive);
			// a semântica POSIX de path absoluto não se aplica.
			t.Skip("semântica de path absoluto POSIX — não aplicável no Windows")
		}

		reg := newMockRegistry()
		registerTool(t, reg, &mockTool{
			name:         "write_tool",
			validateFunc: func(_ json.RawMessage) error { return nil },
		})

		// Sandbox is available; enforceSandbox checks all path-like fields.
		sbox := &mockSandbox{available: true}

		exe := newTestExecutor(t, reg, sbox, ws)

		// Use "pattern" (not "path") so ValidateToolCall doesn't catch it
		// at step 1; it is only caught by enforceSandbox at step 3.
		result, err := exe.Execute(context.Background(), ToolCall{
			ID:   "call_1",
			Name: "write_tool",
			Input: map[string]interface{}{
				"pattern": "/etc/passwd",
			},
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, StatusError, result.Status)
		assert.Contains(t, result.Error, "sandbox denied")
	})

	t.Run("sandbox passes valid path", func(t *testing.T) {
		t.Parallel()

		reg := newMockRegistry()
		registerTool(t, reg, &mockTool{
			name:        "read_tool",
			executeFunc: succeedTool("safe data"),
		})

		sbox := &mockSandbox{available: true}
		exe := newTestExecutor(t, reg, sbox, ws)

		result, err := exe.Execute(context.Background(), ToolCall{
			ID:   "call_2",
			Name: "read_tool",
			Input: map[string]interface{}{
				"path": "safe/file.txt",
			},
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, StatusSuccess, result.Status)
		assert.Equal(t, "safe data", result.Output)
	})

	t.Run("sandbox denies via pattern field", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			// Padrão "/etc/shadow" — semântica de path absoluto POSIX.
			t.Skip("semântica de path absoluto POSIX — não aplicável no Windows")
		}

		reg := newMockRegistry()
		registerTool(t, reg, &mockTool{
			name:         "find_tool",
			validateFunc: func(_ json.RawMessage) error { return nil },
		})

		sbox := &mockSandbox{available: true}
		exe := newTestExecutor(t, reg, sbox, ws)

		result, err := exe.Execute(context.Background(), ToolCall{
			ID:   "call_3",
			Name: "find_tool",
			Input: map[string]interface{}{
				"pattern": "/etc/shadow",
			},
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, StatusError, result.Status)
		assert.Contains(t, result.Error, "sandbox denied")
	})

	t.Run("sandbox denies via directory field", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			// Diretório "/root" — semântica de path absoluto POSIX.
			t.Skip("semântica de path absoluto POSIX — não aplicável no Windows")
		}

		reg := newMockRegistry()
		registerTool(t, reg, &mockTool{
			name:         "list_tool",
			validateFunc: func(_ json.RawMessage) error { return nil },
		})

		sbox := &mockSandbox{available: true}
		exe := newTestExecutor(t, reg, sbox, ws)

		result, err := exe.Execute(context.Background(), ToolCall{
			ID:   "call_4",
			Name: "list_tool",
			Input: map[string]interface{}{
				"directory": "/root",
			},
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, StatusError, result.Status)
		assert.Contains(t, result.Error, "sandbox denied")
	})

	t.Run("non-string path field is skipped by enforceSandbox", func(t *testing.T) {
		t.Parallel()

		reg := newMockRegistry()
		registerTool(t, reg, &mockTool{
			name:        "misc_tool",
			executeFunc: succeedTool("ignored non-string path"),
		})

		sbox := &mockSandbox{available: true}
		exe := newTestExecutor(t, reg, sbox, ws)

		// "pattern" is not a string, so enforceSandbox skips it.
		// "path" would be caught by ValidateToolCall (step 1) which
		// checks the type; "pattern" is only checked in enforceSandbox.
		result, err := exe.Execute(context.Background(), ToolCall{
			ID:   "call_5",
			Name: "misc_tool",
			Input: map[string]interface{}{
				"pattern": 123,
			},
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, StatusSuccess, result.Status)
	})
}

func TestExecute_SandboxUnavailable(t *testing.T) {
	t.Parallel()

	ws := t.TempDir()
	reg := newMockRegistry()
	registerTool(t, reg, &mockTool{
		name:        "tool",
		executeFunc: succeedTool("no sandbox needed"),
	})

	t.Run("nil sandbox skips enforcement", func(t *testing.T) {
		t.Parallel()

		exe := newTestExecutor(t, reg, nil, ws)
		// Use "pattern" instead of "path" so ValidateToolCall (step 1)
		// does NOT check it — only enforceSandbox (step 3) would check it.
		result, err := exe.Execute(context.Background(), ToolCall{
			ID:   "call_1",
			Name: "tool",
			Input: map[string]interface{}{
				"pattern": "/etc/passwd", // would be denied if sandbox present
			},
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, StatusSuccess, result.Status)
	})

	t.Run("sandbox not available skips enforcement", func(t *testing.T) {
		t.Parallel()

		sbox := &mockSandbox{available: false}
		exe := newTestExecutor(t, reg, sbox, ws)
		result, err := exe.Execute(context.Background(), ToolCall{
			ID:   "call_2",
			Name: "tool",
			Input: map[string]interface{}{
				"pattern": "/etc/passwd",
			},
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, StatusSuccess, result.Status)
	})
}

// ─── Test: Execute — Marshal Failure ──────────────────────────────────────

func TestExecute_MarshalFailure(t *testing.T) {
	t.Parallel()

	// JSON marshal can only fail on channels, functions, or complex
	// recursive structures. Use a channel to trigger the error.
	reg := newMockRegistry()
	registerTool(t, reg, &mockTool{
		name: "tool",
	})

	exe := newTestExecutor(t, reg, nil, "")
	result, err := exe.Execute(context.Background(), ToolCall{
		ID:   "call_1",
		Name: "tool",
		Input: map[string]interface{}{
			"ch": make(chan int),
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, StatusError, result.Status)
	assert.Contains(t, result.Error, "invalid input")
}

// ─── Test: Execute — General Tool Error ───────────────────────────────────

func TestExecute_ToolExecuteError(t *testing.T) {
	t.Parallel()

	reg := newMockRegistry()
	registerTool(t, reg, &mockTool{
		name: "crash",
		executeFunc: func(_ context.Context, _ json.RawMessage) (*chat.ToolResult, error) {
			return nil, errors.New("internal crash: disk full")
		},
	})

	exe := newTestExecutor(t, reg, nil, "")
	result, err := exe.Execute(context.Background(), ToolCall{
		ID:    "call_1",
		Name:  "crash",
		Input: map[string]interface{}{},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, StatusError, result.Status)
	assert.Contains(t, result.Error, "internal crash: disk full")
}

// ─── Test: ExecuteBatch — All Success ─────────────────────────────────────

func TestExecuteBatch_AllSuccess(t *testing.T) {
	t.Parallel()

	reg := newMockRegistry()
	registerTool(t, reg, &mockTool{
		name:        "tool_a",
		executeFunc: succeedTool("result_a"),
	})
	registerTool(t, reg, &mockTool{
		name:        "tool_b",
		executeFunc: succeedTool("result_b"),
	})
	registerTool(t, reg, &mockTool{
		name:        "tool_c",
		executeFunc: succeedTool("result_c"),
	})

	exe := newTestExecutor(t, reg, nil, "")
	calls := []ToolCall{
		{ID: "1", Name: "tool_a", Input: map[string]interface{}{}},
		{ID: "2", Name: "tool_b", Input: map[string]interface{}{}},
		{ID: "3", Name: "tool_c", Input: map[string]interface{}{}},
	}

	results := exe.ExecuteBatch(context.Background(), calls)

	require.Len(t, results, 3)
	for i, r := range results {
		assert.Equal(t, calls[i].ID, r.ToolCallID, "index %d", i)
		assert.Equal(t, StatusSuccess, r.Status, "index %d", i)
		assert.Empty(t, r.Error, "index %d", i)
	}
	assert.Equal(t, "result_a", results[0].Output)
	assert.Equal(t, "result_b", results[1].Output)
	assert.Equal(t, "result_c", results[2].Output)
}

// ─── Test: ExecuteBatch — Partial Failure ─────────────────────────────────

func TestExecuteBatch_PartialFailure(t *testing.T) {
	t.Parallel()

	reg := newMockRegistry()
	registerTool(t, reg, &mockTool{
		name:        "good",
		executeFunc: succeedTool("ok"),
	})
	registerTool(t, reg, &mockTool{
		name: "bad",
		executeFunc: func(_ context.Context, _ json.RawMessage) (*chat.ToolResult, error) {
			return nil, errors.New("epic fail")
		},
	})
	registerTool(t, reg, &mockTool{
		name:        "also_good",
		executeFunc: succeedTool("also ok"),
	})

	exe := newTestExecutor(t, reg, nil, "")
	calls := []ToolCall{
		{ID: "1", Name: "good", Input: map[string]interface{}{}},
		{ID: "2", Name: "bad", Input: map[string]interface{}{}},
		{ID: "3", Name: "also_good", Input: map[string]interface{}{}},
	}

	results := exe.ExecuteBatch(context.Background(), calls)

	require.Len(t, results, 3)
	assert.Equal(t, StatusSuccess, results[0].Status)
	assert.Equal(t, "ok", results[0].Output)

	assert.Equal(t, StatusError, results[1].Status)
	assert.Contains(t, results[1].Error, "epic fail")

	assert.Equal(t, StatusSuccess, results[2].Status)
	assert.Equal(t, "also ok", results[2].Output)
}

// ─── Test: ExecuteBatch — Empty Input ─────────────────────────────────────

func TestExecuteBatch_EmptyInput(t *testing.T) {
	t.Parallel()

	exe := newTestExecutor(t, newMockRegistry(), nil, "")
	results := exe.ExecuteBatch(context.Background(), nil)

	require.NotNil(t, results)
	assert.Empty(t, results)

	results = exe.ExecuteBatch(context.Background(), []ToolCall{})

	require.NotNil(t, results)
	assert.Empty(t, results)
}

// ─── Test: ExecuteBatch — Context Cancel ──────────────────────────────────

func TestExecuteBatch_ContextCancel(t *testing.T) {
	t.Parallel()

	t.Run("cancel before semaphore acquire", func(t *testing.T) {
		t.Parallel()

		// Fill the only semaphore slot before canceling. This makes the second
		// call observe only ctx.Done() while waiting to acquire the semaphore.
		started := make(chan struct{})
		var executions atomic.Int32

		reg := newMockRegistry()
		registerTool(t, reg, &mockTool{
			name: "tool",
			executeFunc: func(ctx context.Context, _ json.RawMessage) (*chat.ToolResult, error) {
				if executions.Add(1) == 1 {
					close(started)
					<-ctx.Done()
				}
				return nil, ctx.Err()
			},
		})

		exe := newTestExecutor(t, reg, nil, "")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		calls := []ToolCall{
			{ID: "1", Name: "tool", Input: map[string]interface{}{}},
			{ID: "2", Name: "tool", Input: map[string]interface{}{}},
		}

		resultCh := make(chan []*ToolResult, 1)
		go func() {
			resultCh <- exe.ExecuteBatch(ctx, calls, WithConcurrency(1))
		}()
		<-started
		cancel()
		results := <-resultCh

		require.Len(t, results, 2)
		// Both calls must observe the canceled context, including if the
		// semaphore and ctx.Done() become ready at the same time.
		timeoutCount := 0
		for _, r := range results {
			if r.Status == StatusTimeout {
				timeoutCount++
				assert.Contains(t, r.Error, "context canceled")
			}
		}
		assert.Equal(t, 2, timeoutCount,
			"expected both calls to time out from canceled context")
	})

	t.Run("cancel with custom concurrency", func(t *testing.T) {
		t.Parallel()

		// Use a blocking tool to ensure the context is canceled while
		// goroutines are waiting on the semaphore.
		blocked := make(chan struct{})
		reg := newMockRegistry()
		registerTool(t, reg, &mockTool{
			name: "block",
			executeFunc: func(ctx context.Context, _ json.RawMessage) (*chat.ToolResult, error) {
				// Block until either the test cleanup or context cancelation.
				select {
				case <-blocked:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				return &chat.ToolResult{Output: "ran"}, nil
			},
		})

		exe := newTestExecutor(t, reg, nil, "")
		ctx, cancel := context.WithCancel(context.Background())

		// Spawn 5 calls; with concurrency=2, some will wait on the semaphore.
		calls := make([]ToolCall, 5)
		for i := range calls {
			calls[i] = ToolCall{ID: string(rune('A' + i)), Name: "block", Input: map[string]interface{}{}}
		}

		resultCh := make(chan []*ToolResult)
		go func() {
			resultCh <- exe.ExecuteBatch(ctx, calls, WithConcurrency(2))
		}()

		// Give goroutines time to start and acquire semaphore slots.
		time.Sleep(50 * time.Millisecond)

		// Cancel the context — waiting goroutines should return timeout.
		cancel()

		results := <-resultCh
		close(blocked) // cleanup: unblock any running tools

		require.Len(t, results, 5)

		// At least some should be timeout (those that couldn't acquire semaphore).
		timeoutCount := 0
		for _, r := range results {
			if r.Status == StatusTimeout {
				timeoutCount++
			}
		}
		assert.GreaterOrEqual(t, timeoutCount, 3,
			"expected at least 3 timeout results (5 calls - 2 semaphore slots)")
	})
}

// ─── Test: ExecuteBatch — Custom Concurrency ──────────────────────────────

func TestExecuteBatch_WithConcurrency(t *testing.T) {
	t.Parallel()

	reg := newMockRegistry()
	concurrent := int32(0)
	maxConcurrent := int32(0)

	for i := 0; i < 10; i++ {
		name := string(rune('A' + i))
		registerTool(t, reg, &mockTool{
			name: "tool_" + name,
			executeFunc: func(_ context.Context, _ json.RawMessage) (*chat.ToolResult, error) {
				v := atomicAddInt32(&concurrent, 1)
				atomicMaxInt32(&maxConcurrent, v)
				time.Sleep(10 * time.Millisecond)
				atomicAddInt32(&concurrent, -1)
				return &chat.ToolResult{Output: "ok"}, nil
			},
		})
	}

	exe := newTestExecutor(t, reg, nil, "")

	// Allow up to 10 concurrent executions.
	calls := make([]ToolCall, 10)
	for i := range calls {
		calls[i] = ToolCall{
			ID:    string(rune('A' + i)),
			Name:  "tool_" + string(rune('A'+i)),
			Input: map[string]interface{}{},
		}
	}

	// Use low concurrency (2) to verify the semaphore is respected.
	_ = exe.ExecuteBatch(context.Background(), calls, WithConcurrency(2))

	// With 2 concurrency, max concurrent should be at most 2 at any point.
	assert.LessOrEqual(t, maxConcurrent, int32(2),
		"expected max concurrency of 2 but got %d", maxConcurrent)
}

// ─── Test: ExecuteBatch — Default Concurrency ─────────────────────────────

func TestExecuteBatch_DefaultConcurrency(t *testing.T) {
	t.Parallel()

	reg := newMockRegistry()
	concurrent := int32(0)
	maxConcurrent := int32(0)

	for i := 0; i < 10; i++ {
		name := string(rune('A' + i))
		registerTool(t, reg, &mockTool{
			name: "tool_" + name,
			executeFunc: func(_ context.Context, _ json.RawMessage) (*chat.ToolResult, error) {
				v := atomicAddInt32(&concurrent, 1)
				atomicMaxInt32(&maxConcurrent, v)
				time.Sleep(10 * time.Millisecond)
				atomicAddInt32(&concurrent, -1)
				return &chat.ToolResult{Output: "ok"}, nil
			},
		})
	}

	exe := newTestExecutor(t, reg, nil, "")

	calls := make([]ToolCall, 10)
	for i := range calls {
		calls[i] = ToolCall{
			ID:    string(rune('A' + i)),
			Name:  "tool_" + string(rune('A'+i)),
			Input: map[string]interface{}{},
		}
	}

	_ = exe.ExecuteBatch(context.Background(), calls)

	// Default concurrency is 3, so max concurrent should be at most 3.
	assert.LessOrEqual(t, maxConcurrent, int32(3),
		"expected max concurrency of 3 but got %d", maxConcurrent)
}

// ─── Test: ExecuteBatch — WithConcurrency Zero Is Ignored ─────────────────

func TestExecuteBatch_WithConcurrencyZero(t *testing.T) {
	t.Parallel()

	// WithConcurrency(0) should be ignored (defaults to 3).
	reg := newMockRegistry()
	registerTool(t, reg, &mockTool{
		name:        "tool",
		executeFunc: succeedTool("ok"),
	})

	exe := newTestExecutor(t, reg, nil, "")
	calls := []ToolCall{
		{ID: "1", Name: "tool", Input: map[string]interface{}{}},
	}

	results := exe.ExecuteBatch(context.Background(), calls, WithConcurrency(0))
	require.Len(t, results, 1)
	assert.Equal(t, StatusSuccess, results[0].Status)
}

// ─── Test: ValidateToolCall ───────────────────────────────────────────────

func TestValidateToolCall_Valid(t *testing.T) {
	t.Parallel()

	reg := newMockRegistry()
	registerTool(t, reg, &mockTool{
		name: "greeter",
		validateFunc: func(_ json.RawMessage) error {
			return nil
		},
	})

	exe := newTestExecutor(t, reg, nil, "")
	err := exe.ValidateToolCall(ToolCall{
		Name:  "greeter",
		Input: map[string]interface{}{"name": "world"},
	})

	assert.NoError(t, err)
}

func TestValidateToolCall_EmptyName(t *testing.T) {
	t.Parallel()

	reg := newMockRegistry()
	exe := newTestExecutor(t, reg, nil, "")
	err := exe.ValidateToolCall(ToolCall{
		Name:  "",
		Input: map[string]interface{}{},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool name is required")
}

func TestValidateToolCall_UnknownTool(t *testing.T) {
	t.Parallel()

	reg := newMockRegistry()
	exe := newTestExecutor(t, reg, nil, "")
	err := exe.ValidateToolCall(ToolCall{
		Name:  "unknown",
		Input: map[string]interface{}{},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool not found")
}

func TestValidateToolCall_NilInput(t *testing.T) {
	t.Parallel()

	reg := newMockRegistry()
	registerTool(t, reg, &mockTool{name: "tool"})
	exe := newTestExecutor(t, reg, nil, "")
	err := exe.ValidateToolCall(ToolCall{
		Name:  "tool",
		Input: nil,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "input is required")
}

func TestValidateToolCall_PathOutsideWorkspace(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		// "/etc/passwd" não é um path absoluto no Windows (falta o drive);
		// o Rails resolve contra o workspace e o escape não é detectado.
		t.Skip("semântica de path absoluto POSIX — não aplicável no Windows")
	}

	ws := t.TempDir()
	reg := newMockRegistry()
	registerTool(t, reg, &mockTool{
		name: "reader",
		validateFunc: func(_ json.RawMessage) error {
			return nil
		},
	})

	exe := newTestExecutor(t, reg, nil, ws)
	err := exe.ValidateToolCall(ToolCall{
		Name: "reader",
		Input: map[string]interface{}{
			"path": "/etc/passwd",
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "path validation")
	assert.Contains(t, err.Error(), "escapes workspace")
}

func TestValidateToolCall_InvalidPathType(t *testing.T) {
	t.Parallel()

	ws := t.TempDir()
	reg := newMockRegistry()
	registerTool(t, reg, &mockTool{
		name: "reader",
		validateFunc: func(_ json.RawMessage) error {
			return nil
		},
	})

	exe := newTestExecutor(t, reg, nil, ws)
	err := exe.ValidateToolCall(ToolCall{
		Name: "reader",
		Input: map[string]interface{}{
			"path": 42, // not a string
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "path must be a string")
}

func TestValidateToolCall_SchemaValidationFails(t *testing.T) {
	t.Parallel()

	reg := newMockRegistry()
	registerTool(t, reg, &mockTool{
		name: "strict",
		validateFunc: func(_ json.RawMessage) error {
			return errors.New("field 'x' is required")
		},
	})

	exe := newTestExecutor(t, reg, nil, "")
	err := exe.ValidateToolCall(ToolCall{
		Name:  "strict",
		Input: map[string]interface{}{"y": 1},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
	assert.Contains(t, err.Error(), "field 'x' is required")
}

func TestValidateToolCall_PathWithinWorkspace(t *testing.T) {
	t.Parallel()

	ws := t.TempDir()
	reg := newMockRegistry()
	registerTool(t, reg, &mockTool{
		name: "reader",
		validateFunc: func(_ json.RawMessage) error {
			return nil
		},
	})

	exe := newTestExecutor(t, reg, nil, ws)
	err := exe.ValidateToolCall(ToolCall{
		Name: "reader",
		Input: map[string]interface{}{
			"path": "relative/file.txt",
		},
	})

	assert.NoError(t, err)
}

func TestValidateToolCall_BlockedDirectory(t *testing.T) {
	t.Parallel()

	ws := t.TempDir()
	// Create the .git directory so rails can detect it.
	require.NoError(t, os.MkdirAll(filepath.Join(ws, ".git"), 0o755))

	reg := newMockRegistry()
	registerTool(t, reg, &mockTool{
		name: "reader",
		validateFunc: func(_ json.RawMessage) error {
			return nil
		},
	})

	exe := newTestExecutor(t, reg, nil, ws)
	err := exe.ValidateToolCall(ToolCall{
		Name: "reader",
		Input: map[string]interface{}{
			"path": ".git/config",
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocked directory")
}

// ─── Test: ListTools ──────────────────────────────────────────────────────

func TestListTools(t *testing.T) {
	t.Parallel()

	t.Run("returns definitions from registry", func(t *testing.T) {
		t.Parallel()

		defs := []chat.ToolDefinition{
			{
				Type: "function",
				Function: chat.FunctionDef{
					Name:        "read",
					Description: "Read a file",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"path": map[string]any{"type": "string"},
						},
					},
				},
			},
			{
				Type: "function",
				Function: chat.FunctionDef{
					Name:        "write",
					Description: "Write a file",
				},
			},
		}

		reg := newMockRegistry()
		reg.definitions = defs
		exe := newTestExecutor(t, reg, nil, "")

		got, err := exe.ListTools(context.Background())
		require.NoError(t, err)
		assert.Equal(t, defs, got)
	})

	t.Run("empty definitions", func(t *testing.T) {
		t.Parallel()

		reg := newMockRegistry()
		reg.definitions = []chat.ToolDefinition{}
		exe := newTestExecutor(t, reg, nil, "")

		got, err := exe.ListTools(context.Background())
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("nil definitions", func(t *testing.T) {
		t.Parallel()

		reg := newMockRegistry()
		exe := newTestExecutor(t, reg, nil, "")

		got, err := exe.ListTools(context.Background())
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}

// ─── Test: isPathField ────────────────────────────────────────────────────

func TestIsPathField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key  string
		want bool
	}{
		{"path", true},
		{"pattern", true},
		{"directory", true},
		{"dir", true},
		{"file", true},
		{"", false},
		{"name", false},
		{"content", false},
		{"url", false},
		{"Path", false},      // case-sensitive
		{"DIRECTORY", false}, // case-sensitive
		{"filepath", false},
		{"filename", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := isPathField(tt.key)
			assert.Equal(t, tt.want, got, "isPathField(%q)", tt.key)
		})
	}
}

// ─── Atomic Helpers ───────────────────────────────────────────────────────

// atomicAddInt32 atomically adds delta to the int32 and returns the new value.
func atomicAddInt32(ptr *int32, delta int32) int32 {
	return atomic.AddInt32(ptr, delta)
}

// atomicMaxInt32 atomically sets *ptr to max(*ptr, val).
func atomicMaxInt32(ptr *int32, val int32) {
	for {
		cur := atomic.LoadInt32(ptr)
		if val <= cur {
			return
		}
		if atomic.CompareAndSwapInt32(ptr, cur, val) {
			return
		}
	}
}
