package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testLauncher struct{}

func (testLauncher) Command(ctx context.Context, c chat.Command, _ chat.SandboxMode) (*exec.Cmd, error) {
	return exec.CommandContext(ctx, c.Args[0], c.Args[1:]...), nil
}

// =============================================================================
// TestMain & Fake MCP Server
// =============================================================================

// TestMain starts a fake MCP server subprocess when MCP_TEST_HELPER=1.
// This enables testing stdio-based subprocess communication without an actual
// MCP server binary — the test re-executes itself as the subprocess.
func TestMain(m *testing.M) {
	if os.Getenv("MCP_TEST_HELPER") == "1" {
		fakeMCPServer()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// fakeMCPServer reads NDJSON (newline-delimited JSON) from stdin and writes
// NDJSON responses to stdout. It handles the MCP methods: initialize,
// tools/list, and tools/call. Configuration is done via environment variables:
//
//	MCP_SHOULD_FAIL=1 — return JSON-RPC error for every request
//	MCP_TOOLS_LIST   — JSON array of tools for tools/list response
func fakeMCPServer() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()

		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			// Skip unparseable lines.
			continue
		}

		var result json.RawMessage
		var rpcErr *rpcError
		if os.Getenv("MCP_HANG") == "1" {
			time.Sleep(5 * time.Second)
		}

		switch {
		case os.Getenv("MCP_SHOULD_FAIL") == "1" && req.Method != "initialize":
			// Fail all methods except initialize so the handshake succeeds.
			rpcErr = &rpcError{Code: -32601, Message: "Method not found"}
		case req.Method == "initialize":
			result = json.RawMessage(`{"protocolVersion":"0.1.0"}`)
		case req.Method == "tools/list":
			toolsList := os.Getenv("MCP_TOOLS_LIST")
			if toolsList == "" {
				result = json.RawMessage(`{"tools":[]}`)
			} else {
				result = json.RawMessage(`{"tools":` + toolsList + `}`)
			}
		case req.Method == "tools/call":
			var params toolCallParams
			if err := json.Unmarshal(req.Params, &params); err != nil {
				rpcErr = &rpcError{Code: -32602, Message: "Invalid params"}
				break
			}
			switch params.Name {
			case "echo":
				// Return the arguments JSON as text content.
				textBytes, _ := json.Marshal(string(params.Arguments))
				result = json.RawMessage(`{"content":[{"type":"text","text":` + string(textBytes) + `}]}`)
			case "error_tool":
				result = json.RawMessage(`{"isError":true,"content":[{"type":"text","text":"error message"}]}`)
			case "empty":
				result = json.RawMessage(`{"content":[]}`)
			default:
				result = json.RawMessage(`{"content":[]}`)
			}
		default:
			rpcErr = &rpcError{Code: -32601, Message: "Method not found"}
		}

		resp := response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
			Error:   rpcErr,
		}
		respRaw, _ := json.Marshal(resp)
		fmt.Fprintln(os.Stdout, string(respRaw))
	}
}

// =============================================================================
// Test Helpers
// =============================================================================

// newTestClient creates a Client configured to re-execute the test binary
// as the MCP server subprocess. envOverrides are added to the subprocess
// environment (MCP_TEST_HELPER=1 is set automatically).
func newTestClient(t *testing.T, envOverrides map[string]string) *Client {
	t.Helper()
	env := map[string]string{"MCP_TEST_HELPER": "1"}
	for k, v := range envOverrides {
		env[k] = v
	}
	return NewStdioClient(os.Args[0], nil, env, testLauncher{})
}

// connectTestClient creates a Client, connects it to the fake server, and
// returns it. The caller must close the client (typically via defer).
func connectTestClient(t *testing.T, envOverrides map[string]string) *Client {
	t.Helper()
	c := newTestClient(t, envOverrides)
	ctx := context.Background()
	err := c.Connect(ctx)
	require.NoError(t, err, "Connect should succeed")
	return c
}

// =============================================================================
// Tests — Construction
// =============================================================================

func TestNewStdioClient(t *testing.T) {
	t.Parallel()

	// Arrange
	cmd := "my-server"
	args := []string{"--port", "8080"}
	env := map[string]string{"MCP_KEY": "secret"}

	// Act
	c := NewStdioClient(cmd, args, env)

	// Assert
	assert.Equal(t, cmd, c.command)
	assert.Equal(t, args, c.args)
	assert.Equal(t, env, c.env)
	assert.Nil(t, c.cmdObj, "cmdObj should be nil before Connect")
	assert.Nil(t, c.stdin, "stdin should be nil before Connect")
	assert.Nil(t, c.scanner, "scanner should be nil before Connect")
	assert.False(t, c.closed, "closed should be false initially")
}

// =============================================================================
// Tests — Connect
// =============================================================================

func TestConnect_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	c := connectTestClient(t, nil)
	defer c.Close()

	// Act — use the client after a successful connection to verify it works.
	ctx := context.Background()
	tools, err := c.DiscoverTools(ctx)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, tools, "no tools configured, should return empty slice")
}

func TestConnect_AlreadyConnected(t *testing.T) {
	t.Parallel()

	// Arrange
	c := connectTestClient(t, nil)
	defer c.Close()

	// Act
	ctx := context.Background()
	err := c.Connect(ctx)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already connected")
}

func TestConnect_Closed(t *testing.T) {
	t.Parallel()

	// Arrange — simulate a client that was already closed.
	c := newTestClient(t, nil)
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()

	// Act
	ctx := context.Background()
	err := c.Connect(ctx)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

func TestConnect_NilLauncherDoesNotDeadlock(t *testing.T) {
	c := NewStdioClient("unused", nil, nil)
	done := make(chan error, 1)
	go func() { done <- c.Connect(context.Background()) }()
	select {
	case err := <-done:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "launcher is not configured")
	case <-time.After(time.Second):
		t.Fatal("Connect deadlocked with nil launcher")
	}
}

func TestConnect_CancelableInitialize(t *testing.T) {
	c := newTestClient(t, map[string]string{"MCP_HANG": "1"})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := c.Connect(ctx)
	if err == nil {
		t.Fatal("Connect should fail when initialize exceeds context")
	}
	if time.Since(start) > time.Second {
		t.Fatal("Connect did not honor initialize timeout")
	}
	_ = c.Close()
}

func TestSanitizeDiagnosticRedactsAndBounds(t *testing.T) {
	got := sanitizeDiagnostic("token=super-secret-value\n"+strings.Repeat("x", 2000), map[string]string{"TOKEN": "super-secret-value"})
	assert.NotContains(t, got, "super-secret-value")
	assert.LessOrEqual(t, len(got), maxDiagnosticLength+3)
}

func TestConnect_BadCommand(t *testing.T) {
	t.Parallel()

	// Arrange — use a binary that does not exist.
	c := NewStdioClient("nonexistent-binary-xyz-12345", nil, nil, testLauncher{})

	// Act
	ctx := context.Background()
	err := c.Connect(ctx)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mcp start")
}

// =============================================================================
// Tests — DiscoverTools
// =============================================================================

func TestDiscoverTools(t *testing.T) {
	t.Parallel()

	// Arrange
	toolsJSON := `[
		{"name":"math_calc","description":"Perform arithmetic","inputSchema":{"type":"object"}},
		{"name":"weather","description":"Get weather forecast","inputSchema":{"type":"object","properties":{"city":{"type":"string"}}}}
	]`
	c := connectTestClient(t, map[string]string{"MCP_TOOLS_LIST": toolsJSON})
	defer c.Close()

	// Act
	ctx := context.Background()
	tools, err := c.DiscoverTools(ctx)

	// Assert
	require.NoError(t, err)
	require.Len(t, tools, 2)

	assert.Equal(t, "math_calc", tools[0].Name)
	assert.Equal(t, "Perform arithmetic", tools[0].Description)
	assert.JSONEq(t, `{"type":"object"}`, string(tools[0].InputSchema))

	assert.Equal(t, "weather", tools[1].Name)
	assert.Equal(t, "Get weather forecast", tools[1].Description)
	assert.JSONEq(t, `{"type":"object","properties":{"city":{"type":"string"}}}`, string(tools[1].InputSchema))
}

func TestDiscoverTools_Error(t *testing.T) {
	t.Parallel()

	// Arrange — server returns JSON-RPC error for all methods.
	c := connectTestClient(t, map[string]string{"MCP_SHOULD_FAIL": "1"})
	defer c.Close()

	// Act
	ctx := context.Background()
	tools, err := c.DiscoverTools(ctx)

	// Assert
	require.Error(t, err)
	assert.Nil(t, tools)
	assert.Contains(t, err.Error(), "JSON-RPC error")
	assert.Contains(t, err.Error(), "-32601")
	assert.Contains(t, err.Error(), "Method not found")
}

// =============================================================================
// Tests — ExecuteTool
// =============================================================================

func TestExecuteTool_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	c := connectTestClient(t, nil)
	defer c.Close()

	// Act
	ctx := context.Background()
	args := json.RawMessage(`{"message":"hello"}`)
	result, err := c.ExecuteTool(ctx, "echo", args)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, `{"message":"hello"}`, result.Output)
	assert.Empty(t, result.Error)
	assert.Zero(t, result.Duration, "Duration should be zero-valued for fresh results")
}

func TestExecuteTool_Error(t *testing.T) {
	t.Parallel()

	// Arrange
	c := connectTestClient(t, nil)
	defer c.Close()

	// Act
	ctx := context.Background()
	result, err := c.ExecuteTool(ctx, "error_tool", nil)

	// Assert — the JSON-RPC request itself succeeds, but the tool reports
	// an error via isError:true.
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "error message", result.Error)
	assert.Empty(t, result.Output)
}

func TestExecuteTool_EmptyContent(t *testing.T) {
	t.Parallel()

	// Arrange
	c := connectTestClient(t, nil)
	defer c.Close()

	// Act
	ctx := context.Background()
	result, err := c.ExecuteTool(ctx, "empty", nil)

	// Assert — empty content array triggers the fallback path which
	// returns the raw result JSON as the output string.
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, `{"content":[]}`, result.Output)
	assert.Empty(t, result.Error)
}

// =============================================================================
// Tests — Close
// =============================================================================

func TestClose_Idempotent(t *testing.T) {
	t.Parallel()

	// Arrange
	c := connectTestClient(t, nil)

	// Act — close multiple times.
	err1 := c.Close()
	err2 := c.Close()
	err3 := c.Close()

	// Assert — all close calls must succeed (idempotent).
	assert.NoError(t, err1, "first Close should succeed")
	assert.NoError(t, err2, "second Close should succeed (idempotent)")
	assert.NoError(t, err3, "third Close should succeed (idempotent)")
}

func TestCloseStopsBlockedResponseReader(t *testing.T) {
	c := connectTestClient(t, nil)
	c.mu.Lock()
	readerDone := c.readerDone
	c.mu.Unlock()
	require.NotNil(t, readerDone)

	start := time.Now()
	require.NoError(t, c.Close())
	select {
	case <-readerDone:
		assert.Less(t, time.Since(start), time.Second)
	case <-time.After(time.Second):
		t.Fatal("response reader remained blocked after Close")
	}
}

// =============================================================================
// Tests — Precondition Failures
// =============================================================================

func TestExecuteTool_NotConnected(t *testing.T) {
	t.Parallel()

	// Arrange — never call Connect.
	c := NewStdioClient(os.Args[0], nil, map[string]string{"MCP_TEST_HELPER": "1"}, testLauncher{})

	// Act
	ctx := context.Background()
	result, err := c.ExecuteTool(ctx, "echo", json.RawMessage(`{}`))

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not connected")
}

// =============================================================================
// Tests — Concurrency
// =============================================================================

func TestConcurrentRequests(t *testing.T) {
	t.Parallel()

	// Arrange
	c := connectTestClient(t, nil)
	defer c.Close()

	ctx := context.Background()
	const workers = 10

	type callResult struct {
		idx int
		res *chat.ToolResult
		err error
	}

	results := make(chan callResult, workers)
	var wg sync.WaitGroup

	// Act — launch multiple goroutines that each make an echo request with
	// unique arguments. The mutex in sendRequest serializes the writes, but
	// each request/response cycle must correctly match IDs.
	for i := 0; i < workers; i++ {
		wg.Add(1)
		i := i // capture loop variable
		go func() {
			defer wg.Done()
			args := json.RawMessage(fmt.Sprintf(`{"n":%d}`, i))
			res, err := c.ExecuteTool(ctx, "echo", args)
			results <- callResult{idx: i, res: res, err: err}
		}()
	}

	wg.Wait()
	close(results)

	// Assert
	seen := make(map[int]bool)
	for r := range results {
		require.NoError(t, r.err, "request %d should succeed", r.idx)
		require.NotNil(t, r.res, "request %d should have a result", r.idx)
		expected := fmt.Sprintf(`{"n":%d}`, r.idx)
		assert.Equal(t, expected, r.res.Output, "request %d output mismatch", r.idx)
		assert.Empty(t, r.res.Error, "request %d should not have an error", r.idx)
		assert.False(t, seen[r.idx], "duplicate result for index %d", r.idx)
		seen[r.idx] = true
	}
	assert.Len(t, seen, workers, "all %d workers should produce unique results", workers)
}
