// Package mcp implements a JSON-RPC 2.0 client for the Model Context Protocol
// (MCP). It connects to MCP servers over stdio subprocess transport, discovers
// available tools via tools/list, and executes them via tools/call.
//
// The client adapts external MCP tools into chat.Tool implementations so they
// can be registered in the ToolRegistry and used by the AgentEngine.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/chat/sandbox"
)

// =============================================================================
// JSON-RPC 2.0 Types
// =============================================================================

// request is a JSON-RPC 2.0 request.
type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// response is a JSON-RPC 2.0 response.
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError is a JSON-RPC 2.0 error object.
type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Error returns a human-readable error string.
func (e *rpcError) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

// =============================================================================
// MCP Tool Types
// =============================================================================

// ToolInfo describes a tool exposed by the MCP server.
type ToolInfo struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// toolsListResult is the response payload from tools/list.
type toolsListResult struct {
	Tools []ToolInfo `json:"tools"`
}

// toolCallParams is the params for a tools/call request.
type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// toolCallContentItem represents a content item in a tool call result.
type toolCallContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// toolCallResult is the response payload from tools/call (MCP content format).
type toolCallResult struct {
	Content []toolCallContentItem `json:"content"`
	IsError bool                  `json:"isError,omitempty"`
}

// =============================================================================
// Client
// =============================================================================

// Client communicates with an MCP server over stdio using JSON-RPC 2.0.
// It spawns a subprocess and communicates via its stdin/stdout using
// newline-delimited JSON (NDJSON).
type Client struct {
	command  string
	args     []string
	env      map[string]string
	launcher Launcher

	mu         sync.Mutex
	requestMu  sync.Mutex
	cmdObj     *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	scanner    *bufio.Scanner
	responses  chan []byte
	readerDone chan struct{}
	readerStop chan struct{}
	seqNum     atomic.Int64
	closed     bool

	stderrMu   sync.Mutex
	stderrBuf  bytes.Buffer
	stderrDone chan struct{}
	processMu  sync.Mutex // sole owner boundary for Process.Kill/Wait
}

// Launcher prepares an interactive process. MCP stdio must be launched
// through the same OS sandbox as shell/tools; a nil launcher is retained only
// for compatibility with library callers and is not used by the chat runtime.
type Launcher interface {
	Command(context.Context, chat.Command, chat.SandboxMode) (*exec.Cmd, error)
}

// NewStdioClient creates a new MCP client that communicates with a subprocess
// over stdin/stdout. The command is the executable path and args are its
// arguments. The env map provides additional environment variables (or
// overrides) for the subprocess.
func NewStdioClient(command string, args []string, env map[string]string, launcher ...Launcher) *Client {
	var l Launcher
	if len(launcher) > 0 {
		l = launcher[0]
	}
	return &Client{
		command:  command,
		args:     args,
		env:      env,
		launcher: l,
	}
}

// Connect spawns the MCP server subprocess and performs the MCP initialize
// handshake. It returns an error if the process cannot be started or the
// handshake fails.
func (c *Client) Connect(ctx context.Context) error {
	if err := sandbox.ValidateEnvironment(c.env); err != nil {
		return fmt.Errorf("mcp environment: %w", err)
	}
	c.mu.Lock()

	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("mcp client is closed")
	}
	if c.cmdObj != nil {
		c.mu.Unlock()
		return fmt.Errorf("mcp client is already connected")
	}
	// Reserve the connection slot before invoking an external launcher.
	// A launcher is allowed to do slow discovery, so never hold c.mu here.
	c.cmdObj = &exec.Cmd{}
	c.mu.Unlock()

	var cmd *exec.Cmd
	var err error
	if c.launcher == nil {
		c.mu.Lock()
		c.cmdObj = nil
		c.mu.Unlock()
		return fmt.Errorf("mcp sandbox launcher is not configured")
	}
	type launchResult struct {
		cmd *exec.Cmd
		err error
	}
	launched := make(chan launchResult, 1)
	go func() {
		cmd, err := c.launcher.Command(ctx, chat.Command{Args: append([]string{c.command}, c.args...), Env: c.env}, chat.SandboxWorkspace)
		launched <- launchResult{cmd: cmd, err: err}
	}()
	select {
	case result := <-launched:
		cmd, err = result.cmd, result.err
	case <-ctx.Done():
		c.mu.Lock()
		c.cmdObj = nil
		c.mu.Unlock()
		return ctx.Err()
	}
	if err != nil {
		c.mu.Lock()
		c.cmdObj = nil
		c.mu.Unlock()
		return fmt.Errorf("mcp sandbox start: %w", err)
	}

	// Pass through the current environment plus any configured overrides.
	// Do not inherit the caller's environment. The sandbox launcher supplies its
	// own fixed allowlist; only explicitly configured, non-ambient values pass.
	cmd.Env = envSlice(c.env)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		c.mu.Lock()
		c.cmdObj = nil
		c.mu.Unlock()
		return fmt.Errorf("mcp stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.mu.Lock()
		c.cmdObj = nil
		c.mu.Unlock()
		return fmt.Errorf("mcp stdout pipe: %w", err)
	}

	// Capture stderr for diagnostics
	stderr, err := cmd.StderrPipe()
	if err != nil {
		c.mu.Lock()
		c.cmdObj = nil
		c.mu.Unlock()
		return fmt.Errorf("mcp stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		c.mu.Lock()
		c.cmdObj = nil
		c.mu.Unlock()
		return fmt.Errorf("mcp start: %w", err)
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		_ = c.terminateProcess(cmd)
		return fmt.Errorf("mcp client is closed")
	}
	c.cmdObj = cmd
	c.stdin = stdin
	c.stdout = stdout
	c.scanner = bufio.NewScanner(stdout)
	c.scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)
	c.responses = make(chan []byte, 16)
	c.readerDone = make(chan struct{})
	c.readerStop = make(chan struct{})

	// Release the lock before the handshake — sendRequest acquires its own
	// lock for the full request/response cycle and sync.Mutex is not reentrant.
	c.mu.Unlock()
	go c.readResponses(stdout, c.responses, c.readerDone, c.readerStop)

	// Read stderr in the background to avoid blocking the subprocess. The
	// captured output is kept for error diagnostics (see stderrDiagnostic).
	c.stderrDone = make(chan struct{})
	go func() {
		defer close(c.stderrDone)
		sl := bufio.NewScanner(stderr)
		for sl.Scan() {
			c.stderrMu.Lock()
			if c.stderrBuf.Len() < 64*1024 {
				c.stderrBuf.WriteString(sl.Text())
				c.stderrBuf.WriteByte('\n')
			}
			c.stderrMu.Unlock()
		}
	}()

	// Perform MCP initialize handshake.
	if err := c.initialize(ctx); err != nil {
		c.cleanupProcess(cmd)
		return fmt.Errorf("mcp initialize: %w", err)
	}

	return nil
}

// initialize sends the MCP initialize request and waits for the response.
func (c *Client) initialize(ctx context.Context) error {
	initParams := map[string]interface{}{
		"protocolVersion": "0.1.0",
		"clientInfo": map[string]string{
			"name":    "cosca-chat",
			"version": "1.0.0",
		},
	}
	paramsRaw, _ := json.Marshal(initParams)

	_, err := c.sendRequest(ctx, "initialize", paramsRaw)
	return err
}

// DiscoverTools connects to the MCP server (if not already connected) and
// returns the list of available tools. Results are cached for subsequent calls.
func (c *Client) DiscoverTools(ctx context.Context) ([]ToolInfo, error) {
	respRaw, err := c.sendRequest(ctx, "tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("tools/list: %w", err)
	}

	var result toolsListResult
	if err := json.Unmarshal(respRaw, &result); err != nil {
		return nil, fmt.Errorf("tools/list unmarshal: %w", err)
	}

	return result.Tools, nil
}

// ExecuteTool sends a tools/call request to the MCP server and returns the
// tool's output as a ToolResult.
func (c *Client) ExecuteTool(ctx context.Context, toolName string, arguments json.RawMessage) (*chat.ToolResult, error) {
	params := toolCallParams{
		Name:      toolName,
		Arguments: arguments,
	}
	paramsRaw, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal tool call params: %w", err)
	}

	respRaw, err := c.sendRequest(ctx, "tools/call", paramsRaw)
	if err != nil {
		return nil, fmt.Errorf("tools/call %q: %w", toolName, err)
	}

	// Try to parse as MCP content format first.
	var contentResult toolCallResult
	if err := json.Unmarshal(respRaw, &contentResult); err == nil && len(contentResult.Content) > 0 {
		// Build output from all content items.
		output := ""
		for _, item := range contentResult.Content {
			if item.Type == "text" {
				if output != "" {
					output += "\n"
				}
				output += item.Text
			}
		}
		if contentResult.IsError {
			return &chat.ToolResult{Error: output}, nil
		}
		return &chat.ToolResult{Output: output}, nil
	}

	// Fallback: marshal the raw result as a string.
	output := string(respRaw)
	return &chat.ToolResult{Output: output}, nil
}

// Close terminates the MCP server subprocess and releases resources. It is
// safe to call multiple times.
func (c *Client) Close() error {
	c.mu.Lock()

	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true

	cmd := c.cmdObj
	stdin := c.stdin
	stdout := c.stdout
	readerStop := c.readerStop
	readerDone := c.readerDone
	stderrDone := c.stderrDone
	c.cmdObj = nil
	c.stdin = nil
	c.stdout = nil
	c.scanner = nil
	c.responses = nil
	c.readerStop = nil
	c.mu.Unlock()
	if readerStop != nil {
		close(readerStop)
	}
	if stdin != nil {
		_ = stdin.Close()
	}
	// Scanner.Scan blocks in the kernel while waiting for stdout. Closing the
	// pipe is required to make cancellation/Close interrupt that read; the
	// stop channel alone cannot interrupt an already-blocked Scan.
	if stdout != nil {
		_ = stdout.Close()
	}
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	err := c.terminateProcess(cmd)
	if readerDone != nil {
		<-readerDone
	}
	if stderrDone != nil {
		<-stderrDone
	}
	return err
}

// =============================================================================
// Internal: JSON-RPC Request/Response
// =============================================================================

// sendRequest sends a JSON-RPC 2.0 request and waits for the matching response.
func (c *Client) sendRequest(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	c.requestMu.Lock()
	defer c.requestMu.Unlock()
	c.mu.Lock()
	if c.closed || c.cmdObj == nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("mcp client not connected")
	}
	stdin := c.stdin
	responses := c.responses
	c.mu.Unlock()

	// Generate a unique request ID.
	id := c.seqNum.Add(1)
	idRaw, _ := json.Marshal(id)

	req := request{
		JSONRPC: "2.0",
		ID:      idRaw,
		Method:  method,
		Params:  params,
	}

	reqRaw, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Write the request to stdin (NDJSON: one JSON object per line).
	writeDone := make(chan error, 1)
	go func() { _, err := stdin.Write(append(reqRaw, '\n')); writeDone <- err }()
	select {
	case err := <-writeDone:
		if err != nil {
			return nil, fmt.Errorf("write request: %w", err)
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Read responses until we find one matching our request ID.
	idStr := string(idRaw)
	for {
		var line []byte
		select {
		case line = <-responses:
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-c.readerDone:
			return nil, fmt.Errorf("mcp server closed connection%s", c.stderrDiagnostic())
		}

		// Try to extract the ID from the raw response without full unmarshal.
		// For robustness, we unmarshal to check if it matches our request.
		var resp response
		if err := json.Unmarshal(line, &resp); err != nil {
			// Skip unparseable lines.
			continue
		}

		// Check if this response matches our request ID.
		respIDStr := string(resp.ID)
		if respIDStr != idStr {
			continue
		}

		if resp.Error != nil {
			return nil, resp.Error
		}

		return resp.Result, nil
	}
}

func (c *Client) readResponses(stdout io.Reader, responses chan<- []byte, done chan struct{}, stop <-chan struct{}) {
	defer close(done)
	s := bufio.NewScanner(stdout)
	s.Buffer(make([]byte, 0, 256*1024), 256*1024)
	for s.Scan() {
		line := append([]byte(nil), s.Bytes()...)
		select {
		case responses <- line:
		case <-stop:
			return
		}
	}
}

func (c *Client) cleanupProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	c.mu.Lock()
	stdout := c.stdout
	readerDone := c.readerDone
	stderrDone := c.stderrDone
	if c.readerStop != nil {
		close(c.readerStop)
		c.readerStop = nil
	}
	if stdout != nil {
		_ = stdout.Close()
	}
	c.mu.Unlock()
	_ = c.terminateProcess(cmd)
	if readerDone != nil {
		<-readerDone
	}
	if stderrDone != nil {
		<-stderrDone
	}
	c.mu.Lock()
	if c.cmdObj == cmd {
		c.cmdObj = nil
		c.stdin = nil
		c.stdout = nil
		c.scanner = nil
		c.responses = nil
	}
	c.mu.Unlock()
}

// terminateProcess is the only place allowed to Kill and Wait a process.
// This prevents Close, handshake cleanup, and late Connect cleanup from
// concurrently owning exec.Cmd's lifecycle.
func (c *Client) terminateProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	c.processMu.Lock()
	defer c.processMu.Unlock()
	if err := cmd.Process.Kill(); err != nil {
		// An already-exited process still needs its single Wait call.
		// On Unix Kill reports "already finished"; on Windows, TerminateProcess
		// returns ERROR_ACCESS_DENIED (syscall.Errno 5) for an already-
		// terminated process. Treat both as "the process already exited".
		if !processAlreadyFinished(err) {
			_ = cmd.Wait()
			return err
		}
	}
	_ = cmd.Wait()
	return nil
}

// processAlreadyFinished reports whether a Kill error means the process has
// already exited. On Unix that is os.ErrProcessDone ("already finished"); on
// Windows, TerminateProcess on an already-terminated process fails with
// ERROR_ACCESS_DENIED (syscall.Errno 5), not with ErrProcessDone.
func processAlreadyFinished(err error) bool {
	if errors.Is(err, os.ErrProcessDone) {
		return true
	}
	if runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(5)) {
		return true
	}
	return false
}

// =============================================================================
// Helpers
// =============================================================================

// envSlice converts a map of environment variables to a slice of "key=value"
// strings suitable for exec.Cmd.Env.
func envSlice(env map[string]string) []string {
	slice := make([]string, 0, len(env))
	for k, v := range env {
		slice = append(slice, k+"="+v)
	}
	return slice
}

// stderrDiagnostic returns the captured MCP server stderr as a diagnostic
// suffix for error messages, or an empty string when there is none. It waits
// briefly for any in-flight stderr output (e.g. a crash message) to be drained
// before building the diagnostic.
func (c *Client) stderrDiagnostic() string {
	select {
	case <-c.stderrDone:
	case <-time.After(50 * time.Millisecond):
	}
	c.stderrMu.Lock()
	defer c.stderrMu.Unlock()
	if c.stderrBuf.Len() == 0 {
		return ""
	}
	return "; server stderr: " + sanitizeDiagnostic(c.stderrBuf.String(), c.env)
}

const maxDiagnosticLength = 1024

// sanitizeDiagnostic keeps diagnostics useful without allowing subprocess
// output (which may contain credentials or terminal control sequences) to be
// copied into user-visible errors.
func sanitizeDiagnostic(s string, env map[string]string) string {
	for _, value := range env {
		if len(value) >= 4 {
			s = strings.ReplaceAll(s, value, "[REDACTED]")
		}
	}
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' || (r >= 0x20 && r != 0x7f) {
			return r
		}
		return ' '
	}, s)
	s = strings.TrimSpace(s)
	if len(s) > maxDiagnosticLength {
		s = s[:maxDiagnosticLength] + "..."
	}
	return s
}
