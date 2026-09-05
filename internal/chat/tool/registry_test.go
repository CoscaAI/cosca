// Package tool provides tests for the tool Registry, covering registration,
// discovery, execution, definitions, and concurrent access patterns.
package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Mock Tool ───────────────────────────────────────────────────────────────

// mockTool implements chat.Tool for testing the Registry. It captures the
// params passed to Execute so callers can inspect them after the call.
type mockTool struct {
	name        string
	description string
	schema      json.RawMessage

	// executeFn is called by Execute. If nil, a default success handler is used.
	executeFn func(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error)
	// validateFn is called by Validate. If nil, a default pass is used.
	validateFn func(params json.RawMessage) error
}

func (m *mockTool) Name() string            { return m.name }
func (m *mockTool) Description() string     { return m.description }
func (m *mockTool) Schema() json.RawMessage { return m.schema }
func (m *mockTool) Validate(params json.RawMessage) error {
	if m.validateFn != nil {
		return m.validateFn(params)
	}
	return nil
}
func (m *mockTool) Execute(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	if m.executeFn != nil {
		return m.executeFn(ctx, params)
	}
	return &chat.ToolResult{Output: "ok", Duration: time.Millisecond}, nil
}

// newMockTool creates a mockTool with sensible defaults.
func newMockTool(name string) *mockTool {
	return &mockTool{
		name:        name,
		description: "test tool " + name,
		schema:      json.RawMessage(`{"type":"object"}`),
	}
}

// ─── Registry: Register & Get ────────────────────────────────────────────────

func TestRegistry_RegisterAndGet(t *testing.T) {
	t.Parallel()

	t.Run("registers and retrieves a tool", func(t *testing.T) {
		r := NewRegistry()
		tool := newMockTool("read")
		r.Register(tool)

		got := r.Get("read")
		require.NotNil(t, got)
		assert.Equal(t, "read", got.Name())
		assert.Equal(t, "test tool read", got.Description())
	})

	t.Run("returns nil for unknown tool", func(t *testing.T) {
		r := NewRegistry()
		got := r.Get("nonexistent")
		assert.Nil(t, got)
	})

	t.Run("empty registry returns nil", func(t *testing.T) {
		r := NewRegistry()
		assert.Nil(t, r.Get("anything"))
	})
}

// ─── Registry: Register Duplicate ────────────────────────────────────────────

func TestRegistry_RegisterDuplicate(t *testing.T) {
	t.Parallel()

	t.Run("overwrites existing tool with same name", func(t *testing.T) {
		r := NewRegistry()
		r.Register(newMockTool("write"))
		r.Register(newMockTool("write")) // overwrite

		assert.Equal(t, 1, r.Size())
		got := r.Get("write")
		require.NotNil(t, got)
		assert.Equal(t, "write", got.Name())
	})

	t.Run("replacement uses new description", func(t *testing.T) {
		r := NewRegistry()
		r.Register(&mockTool{
			name:        "tool",
			description: "original",
			schema:      json.RawMessage(`{"type":"object"}`),
		})
		r.Register(&mockTool{
			name:        "tool",
			description: "replacement",
			schema:      json.RawMessage(`{"type":"object"}`),
		})

		got := r.Get("tool")
		require.NotNil(t, got)
		assert.Equal(t, "replacement", got.Description())
	})
}

// ─── Registry: GetAll ────────────────────────────────────────────────────────

func TestRegistry_GetAll(t *testing.T) {
	t.Parallel()

	t.Run("returns all registered tools", func(t *testing.T) {
		r := NewRegistry()
		r.Register(newMockTool("a"))
		r.Register(newMockTool("b"))
		r.Register(newMockTool("c"))

		all := r.GetAll()
		assert.Len(t, all, 3)

		names := make([]string, len(all))
		for i, tl := range all {
			names[i] = tl.Name()
		}
		sort.Strings(names)
		assert.Equal(t, []string{"a", "b", "c"}, names)
	})

	t.Run("returns empty slice for empty registry", func(t *testing.T) {
		r := NewRegistry()
		assert.Empty(t, r.GetAll())
	})
}

// ─── Registry: List ──────────────────────────────────────────────────────────

func TestRegistry_List(t *testing.T) {
	t.Parallel()

	t.Run("returns sorted tool names", func(t *testing.T) {
		r := NewRegistry()
		r.Register(newMockTool("z"))
		r.Register(newMockTool("a"))
		r.Register(newMockTool("m"))

		assert.Equal(t, []string{"a", "m", "z"}, r.List())
	})

	t.Run("empty registry returns empty slice", func(t *testing.T) {
		r := NewRegistry()
		assert.Empty(t, r.List())
	})

	t.Run("single tool returns that name", func(t *testing.T) {
		r := NewRegistry()
		r.Register(newMockTool("only"))
		assert.Equal(t, []string{"only"}, r.List())
	})
}

// ─── Registry: Execute ───────────────────────────────────────────────────────

func TestRegistry_Execute(t *testing.T) {
	t.Parallel()

	t.Run("dispatches to correct tool and returns result", func(t *testing.T) {
		r := NewRegistry()
		var capturedParams json.RawMessage
		r.Register(&mockTool{
			name: "echo",
			executeFn: func(_ context.Context, params json.RawMessage) (*chat.ToolResult, error) {
				capturedParams = params
				// O Registry mede a duração real (time.Since) e ignora o campo
				// Duration retornado pelo tool. Um Sleep curto garante que a
				// medição seja > 0 mesmo no Windows, onde a granularidade do
				// timer pode zerar uma execução instantânea.
				time.Sleep(time.Millisecond)
				return &chat.ToolResult{Output: "hello", Duration: 5 * time.Millisecond}, nil
			},
		})

		params := json.RawMessage(`{"message":"world"}`)
		result := r.Execute(context.Background(), "echo", params)

		assert.Equal(t, "hello", result.Output)
		assert.Empty(t, result.Error)
		assert.Greater(t, result.Duration, time.Duration(0))
		assert.Equal(t, `{"message":"world"}`, string(capturedParams))
	})

	t.Run("returns error for unknown tool", func(t *testing.T) {
		r := NewRegistry()
		result := r.Execute(context.Background(), "unknown", nil)

		assert.Empty(t, result.Output)
		assert.Equal(t, "tool not found: unknown", result.Error)
		assert.Zero(t, result.Duration)
	})

	t.Run("captures tool execution error", func(t *testing.T) {
		r := NewRegistry()
		r.Register(&mockTool{
			name: "failing",
			executeFn: func(_ context.Context, _ json.RawMessage) (*chat.ToolResult, error) {
				return nil, fmt.Errorf("execution failed: something went wrong")
			},
		})

		result := r.Execute(context.Background(), "failing", json.RawMessage(`{}`))
		assert.Empty(t, result.Output)
		assert.Equal(t, "execution failed: something went wrong", result.Error)
	})

	t.Run("tool result error field is propagated even with nil Go error", func(t *testing.T) {
		r := NewRegistry()
		r.Register(&mockTool{
			name: "validate",
			executeFn: func(_ context.Context, _ json.RawMessage) (*chat.ToolResult, error) {
				// Tools report a semantic rejection through result.Error even
				// when the Go error is nil (e.g. the filesystem read-before-
				// write guard). The Registry MUST propagate it — dropping it
				// would make a rejected write look like a successful one.
				return &chat.ToolResult{Output: "partial", Error: "write_file: file already exists"}, nil
			},
		})

		result := r.Execute(context.Background(), "validate", json.RawMessage(`{}`))
		assert.Equal(t, "partial", result.Output)
		assert.Equal(t, "write_file: file already exists", result.Error)
	})
}

// ─── Registry: Execute nil/empty params ──────────────────────────────────────

func TestRegistry_Execute_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("execute with nil params", func(t *testing.T) {
		r := NewRegistry()
		r.Register(&mockTool{
			name: "nilok",
			executeFn: func(_ context.Context, params json.RawMessage) (*chat.ToolResult, error) {
				return &chat.ToolResult{Output: fmt.Sprintf("got nil=%v", params == nil)}, nil
			},
		})

		result := r.Execute(context.Background(), "nilok", nil)
		assert.Contains(t, result.Output, "got nil=true")
	})

	t.Run("execute with empty params", func(t *testing.T) {
		r := NewRegistry()
		r.Register(newMockTool("reader"))
		result := r.Execute(context.Background(), "reader", json.RawMessage(`{}`))
		assert.Equal(t, "ok", result.Output)
	})
}

// ─── Registry: Definitions ───────────────────────────────────────────────────

func TestRegistry_Definitions(t *testing.T) {
	t.Parallel()

	t.Run("returns definitions in OpenAI-compatible format", func(t *testing.T) {
		r := NewRegistry()
		r.Register(&mockTool{
			name:        "read",
			description: "Read a file",
			schema:      json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
		})
		r.Register(&mockTool{
			name:        "write",
			description: "Write a file",
			schema:      json.RawMessage(`{"type":"object","properties":{"content":{"type":"string"}}}`),
		})

		defs := r.Definitions()
		require.Len(t, defs, 2)

		// Should be sorted by name
		assert.Equal(t, "read", defs[0].Function.Name)
		assert.Equal(t, "Read a file", defs[0].Function.Description)
		assert.Equal(t, "function", defs[0].Type)
		assert.Contains(t, defs[0].Function.Parameters, "properties")

		assert.Equal(t, "write", defs[1].Function.Name)
	})

	t.Run("returns empty slice for empty registry", func(t *testing.T) {
		r := NewRegistry()
		assert.Empty(t, r.Definitions())
	})

	t.Run("handles invalid schema gracefully", func(t *testing.T) {
		r := NewRegistry()
		r.Register(&mockTool{
			name:   "bad",
			schema: json.RawMessage(`{invalid json`),
		})

		defs := r.Definitions()
		require.Len(t, defs, 1)
		// Invalid schema results in empty params map
		assert.Empty(t, defs[0].Function.Parameters)
	})
}

// ─── Registry: Remove ────────────────────────────────────────────────────────

func TestRegistry_Remove(t *testing.T) {
	t.Parallel()

	t.Run("removes existing tool", func(t *testing.T) {
		r := NewRegistry()
		r.Register(newMockTool("tmp"))
		r.Register(newMockTool("keep"))

		r.Remove("tmp")

		assert.Nil(t, r.Get("tmp"))
		assert.NotNil(t, r.Get("keep"))
		assert.Equal(t, 1, r.Size())
	})

	t.Run("removing non-existent tool is no-op", func(t *testing.T) {
		r := NewRegistry()
		r.Register(newMockTool("a"))

		r.Remove("nonexistent")
		assert.Equal(t, 1, r.Size())
	})

	t.Run("remove from empty registry is no-op", func(t *testing.T) {
		r := NewRegistry()
		assert.NotPanics(t, func() { r.Remove("anything") })
	})
}

// ─── Registry: Size ──────────────────────────────────────────────────────────

func TestRegistry_Size(t *testing.T) {
	t.Parallel()

	t.Run("returns correct count", func(t *testing.T) {
		r := NewRegistry()
		assert.Equal(t, 0, r.Size())

		r.Register(newMockTool("a"))
		assert.Equal(t, 1, r.Size())

		r.Register(newMockTool("b"))
		assert.Equal(t, 2, r.Size())

		r.Remove("a")
		assert.Equal(t, 1, r.Size())
	})
}

// ─── Registry: Concurrent Access ─────────────────────────────────────────────

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewRegistry()

	// Pre-populate with some tools
	const initialTools = 10
	for i := range initialTools {
		r.Register(newMockTool(fmt.Sprintf("pre-%d", i)))
	}

	var wg sync.WaitGroup
	const readers = 20
	const writers = 10

	// Concurrent reads
	for range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.List()
			_ = r.GetAll()
			_ = r.Size()
			_ = r.Definitions()
			for i := range initialTools {
				_ = r.Get(fmt.Sprintf("pre-%d", i))
			}
		}()
	}

	// Concurrent writes (register + remove)
	for i := range writers {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("concurrent-%d", idx)
			r.Register(newMockTool(name))
			_ = r.Get(name)
			r.Remove(name)
		}(i)
	}

	// Concurrent executions
	for range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.Execute(context.Background(), "pre-0", json.RawMessage(`{}`))
		}()
	}

	wg.Wait()

	// Verify no corruption: initial tools should still be present
	for i := range initialTools {
		assert.NotNil(t, r.Get(fmt.Sprintf("pre-%d", i)), "pre-%d should still exist", i)
	}
}

// ─── Registry: ToolResult fields ─────────────────────────────────────────────

func TestRegistry_Execute_Duration(t *testing.T) {
	t.Parallel()

	t.Run("duration is recorded and non-zero for successful exec", func(t *testing.T) {
		r := NewRegistry()
		r.Register(&mockTool{
			name: "slow",
			executeFn: func(_ context.Context, _ json.RawMessage) (*chat.ToolResult, error) {
				time.Sleep(2 * time.Millisecond)
				return &chat.ToolResult{Output: "slow"}, nil
			},
		})

		result := r.Execute(context.Background(), "slow", json.RawMessage(`{}`))
		assert.Equal(t, "slow", result.Output)
		assert.GreaterOrEqual(t, result.Duration, 2*time.Millisecond)
	})

	t.Run("duration recorded even on error", func(t *testing.T) {
		r := NewRegistry()
		r.Register(&mockTool{
			name: "err",
			executeFn: func(_ context.Context, _ json.RawMessage) (*chat.ToolResult, error) {
				time.Sleep(time.Millisecond)
				return nil, fmt.Errorf("error after delay")
			},
		})

		result := r.Execute(context.Background(), "err", json.RawMessage(`{}`))
		assert.Contains(t, result.Error, "error after delay")
		assert.GreaterOrEqual(t, result.Duration, time.Millisecond)
	})
}

// ─── Registry: Tool interface conformance ────────────────────────────────────

func TestMockTool_ImplementsTool(t *testing.T) {
	t.Parallel()
	var _ chat.Tool = (*mockTool)(nil)
}
