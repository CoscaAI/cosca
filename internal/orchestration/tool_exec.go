package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/contenttrust"
	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/chat"
)

// ─── Tool Call Result ─────────────────────────────────────────────────────────

// ToolCallResult is the result of executing a tool call.
type ToolCallResult struct {
	ToolCallID string        `json:"tool_call_id"`
	Name       string        `json:"name"`
	Content    string        `json:"content"`
	Error      string        `json:"error,omitempty"`
	ErrorCode  string        `json:"error_code,omitempty"`
	Duration   time.Duration `json:"duration"`
}

// ─── Tool Executor Configuration ──────────────────────────────────────────────

// ToolExecutorConfig configures the tool executor.
type ToolExecutorConfig struct {
	// WorkspaceDir is the root directory for file operations (sandbox).
	// Default: "." (current directory).
	WorkspaceDir string

	// AllowedCommands is a list of allowed shell commands.
	// Empty = allow the default safe set.
	AllowedCommands []string

	// MaxFileSize is the maximum file size for read operations.
	// Default: 10MB.
	MaxFileSize int64

	// Timeout is the maximum duration for tool execution.
	// Default: 30s.
	Timeout time.Duration

	// Sandbox is the optional per-command execution sandbox (chat.Sandbox,
	// backed by sandbox.NewGate + bwrap). When set, every execute_command tool
	// call runs inside the sandbox confined to the workspace with network off.
	// When nil, commands run directly — the enclosing jail provides isolation.
	// Only the terminal wiring (which runs OUTSIDE the jail) sets this; the
	// jail-scoped commands (workflow run, pipeline run, run) keep nil.
	Sandbox chat.Sandbox
}

// DefaultToolExecutorConfig returns a configuration with sensible defaults.
func DefaultToolExecutorConfig() ToolExecutorConfig {
	return ToolExecutorConfig{
		WorkspaceDir: ".",
		AllowedCommands: []string{
			"ls", "cat", "head", "tail", "wc", "find", "grep",
			"mkdir", "touch", "cp", "mv",
			"echo", "date", "which", "pwd", "env",
		},
		MaxFileSize: 10 * 1024 * 1024, // 10MB
		Timeout:     30 * time.Second,
	}
}

// ─── Tool Executor ────────────────────────────────────────────────────────────

// ToolExecutor executes tool calls returned by LLMs. Each tool handler
// validates inputs, enforces security boundaries (path sandboxing, command
// allowlisting), and respects execution timeouts.
type ToolExecutor struct {
	// workspaceDir is the root directory for file operations (sandbox).
	workspaceDir string

	// workspaceAbs is the pre-resolved absolute path of workspaceDir.
	workspaceAbs string

	// allowedCommands is a list of allowed shell commands.
	allowedCommands []string

	// allowedCommandsSet enables O(1) lookups.
	allowedCommandsSet map[string]bool

	// maxFileSize is the maximum file size for read operations.
	maxFileSize int64

	// timeout is the maximum duration for tool execution.
	timeout time.Duration

	// sandbox is the optional per-command execution sandbox (chat.Sandbox).
	sandbox chat.Sandbox

	// memoryRetriever is an optional dependency for the read_memory tool.
	memoryRetriever MemoryRetriever
}

// NewToolExecutor creates a ToolExecutor with the given configuration.
func NewToolExecutor(config ToolExecutorConfig) *ToolExecutor {
	if config.WorkspaceDir == "" {
		config.WorkspaceDir = "."
	}
	if config.MaxFileSize <= 0 {
		config.MaxFileSize = 10 * 1024 * 1024
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	if len(config.AllowedCommands) == 0 {
		config.AllowedCommands = DefaultToolExecutorConfig().AllowedCommands
	}

	// Resolve workspace to an absolute path early so we don't do it on every call.
	workspaceAbs, err := filepath.Abs(config.WorkspaceDir)
	if err != nil {
		workspaceAbs = config.WorkspaceDir
	}
	workspaceAbs = filepath.Clean(workspaceAbs)

	allowedSet := make(map[string]bool, len(config.AllowedCommands))
	for _, cmd := range config.AllowedCommands {
		allowedSet[strings.ToLower(strings.TrimSpace(cmd))] = true
	}

	return &ToolExecutor{
		workspaceDir:       config.WorkspaceDir,
		workspaceAbs:       workspaceAbs,
		allowedCommands:    config.AllowedCommands,
		allowedCommandsSet: allowedSet,
		maxFileSize:        config.MaxFileSize,
		timeout:            config.Timeout,
		sandbox:            config.Sandbox,
	}
}

// SetMemoryRetriever wires the memory engine into the tool executor so that
// the read_memory tool can delegate searches. Passing nil disables the tool.
func (e *ToolExecutor) SetMemoryRetriever(mr MemoryRetriever) {
	e.memoryRetriever = mr
}

// ─── Tool Dispatch ────────────────────────────────────────────────────────────

// Execute runs a single tool call and returns the result.
func (e *ToolExecutor) Execute(ctx context.Context, toolCall chat.ToolCall) (*ToolCallResult, error) {
	start := time.Now()
	logger := log.Ctx(ctx).With().
		Str("tool_name", toolCall.Function.Name).
		Str("tool_call_id", toolCall.ID).
		Logger()

	// Parse arguments from JSON.
	args, err := parseToolArgs(toolCall.Function.Arguments)
	if err != nil {
		info := safeError("tool_arguments_invalid", err)
		logger.Error().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("failed to parse tool arguments")
		return &ToolCallResult{
			ToolCallID: toolCall.ID,
			Name:       toolCall.Function.Name,
			Error:      safeToolMessage(err),
			ErrorCode:  "tool_arguments_invalid",
			Duration:   time.Since(start),
		}, nil
	}

	// Dispatch to the appropriate handler.
	var content string
	switch toolCall.Function.Name {
	case "read_file":
		content, err = e.readFile(ctx, args)
	case "write_file":
		content, err = e.writeFile(ctx, args)
	case "list_files":
		content, err = e.listFiles(ctx, args)
	case "execute_command":
		content, err = e.executeCommand(ctx, args)
	case "search_codebase":
		content, err = e.searchCodebase(ctx, args)
	case "read_memory":
		content, err = e.readMemory(ctx, args)
	default:
		err = fmt.Errorf("unsupported tool: %q", toolCall.Function.Name)
	}

	duration := time.Since(start)

	result := &ToolCallResult{
		ToolCallID: toolCall.ID,
		Name:       toolCall.Function.Name,
		Content:    content,
		Duration:   duration,
	}

	if err != nil {
		info := safeError("tool_execution_failed", err)
		result.Error = safeToolMessage(err)
		result.ErrorCode = info.Code
		logger.Error().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Dur("duration", duration).Msg("tool execution failed")
	} else {
		logger.Debug().Dur("duration", duration).Int("content_len", len(content)).Msg("tool executed successfully")
	}

	return result, nil
}

// ExecuteAll runs a batch of tool calls sequentially and returns all results.
// Even if one tool fails, subsequent tools are still executed.
func (e *ToolExecutor) ExecuteAll(ctx context.Context, toolCalls []chat.ToolCall) ([]*ToolCallResult, error) {
	results := make([]*ToolCallResult, 0, len(toolCalls))
	var firstErr error

	for _, tc := range toolCalls {
		result, err := e.Execute(ctx, tc)
		if result != nil {
			results = append(results, result)
		}
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return results, firstErr
}

// ─── Tool: read_file ──────────────────────────────────────────────────────────

func (e *ToolExecutor) readFile(_ context.Context, args map[string]any) (string, error) {
	pathArg, ok := extractStringArg(args, "path")
	if !ok || pathArg == "" {
		return "", fmt.Errorf("read_file: missing required parameter \"path\"")
	}

	resolved, err := e.resolveAndValidate(pathArg)
	if err != nil {
		return "", fmt.Errorf("read_file: %w", err)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("read_file: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("read_file: %q is a directory", pathArg)
	}
	if info.Size() > e.maxFileSize {
		return "", fmt.Errorf("read_file: file size %d exceeds maximum %d", info.Size(), e.maxFileSize)
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return "", fmt.Errorf("read_file: %w", err)
	}

	// Truncate to 100KB for response.
	const maxResponse = 100 * 1024
	if len(data) > maxResponse {
		return string(data[:maxResponse]), nil
	}
	return string(data), nil
}

// ─── Tool: write_file ─────────────────────────────────────────────────────────

func (e *ToolExecutor) writeFile(_ context.Context, args map[string]any) (string, error) {
	pathArg, ok := extractStringArg(args, "path")
	if !ok || pathArg == "" {
		return "", fmt.Errorf("write_file: missing required parameter \"path\"")
	}
	contentArg, _ := extractStringArg(args, "content")

	resolved, err := e.resolveAndValidate(pathArg)
	if err != nil {
		return "", fmt.Errorf("write_file: %w", err)
	}

	// Ensure parent directory exists.
	dir := filepath.Dir(resolved)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("write_file: failed to create parent directory: %w", err)
	}

	if err := os.WriteFile(resolved, []byte(contentArg), 0o644); err != nil {
		return "", fmt.Errorf("write_file: %w", err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(contentArg), pathArg), nil
}

// ─── Tool: list_files ─────────────────────────────────────────────────────────

func (e *ToolExecutor) listFiles(_ context.Context, args map[string]any) (string, error) {
	pathArg, _ := extractStringArg(args, "path")
	if pathArg == "" {
		pathArg = e.workspaceDir
	}

	resolved, err := e.resolveAndValidate(pathArg)
	if err != nil {
		return "", fmt.Errorf("list_files: %w", err)
	}

	entries, err := os.ReadDir(resolved)
	if err != nil {
		return "", fmt.Errorf("list_files: %w", err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Contents of %s:\n", pathArg)

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			info := safeError("list_entry_info_failed", err)
			fmt.Fprintf(&sb, "  %-40s [error: %s]\n", entry.Name(), safeErrorMessage(info.Code))
			continue
		}
		typeTag := "file"
		if entry.IsDir() {
			typeTag = "dir "
		}
		size := info.Size()
		sizeStr := formatSize(size)
		fmt.Fprintf(&sb, "  %-40s %s %10s\n", entry.Name(), typeTag, sizeStr)
	}

	return sb.String(), nil
}

// ─── Tool: execute_command ────────────────────────────────────────────────────

func (e *ToolExecutor) executeCommand(ctx context.Context, args map[string]any) (string, error) {
	cmdStr, ok := extractStringArg(args, "command")
	if !ok || cmdStr == "" {
		return "", fmt.Errorf("execute_command: missing required parameter \"command\"")
	}

	// Extract the first word (the command name) for allowlist check.
	fields := strings.Fields(cmdStr)
	if len(fields) == 0 {
		return "", fmt.Errorf("execute_command: empty command")
	}

	commandName := strings.ToLower(fields[0])
	if !e.allowedCommandsSet[commandName] {
		return "", fmt.Errorf("execute_command: command %q is not in the allowed list", fields[0])
	}

	// Create a context with timeout.
	execCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// Sandboxed path: the command runs inside the per-command sandbox (bwrap)
	// confined to the workspace with network off. This is the security layer
	// for the terminal when it runs OUTSIDE the jail: an arbitrary command
	// from the LLM never executes as the terminal process itself.
	if e.sandbox != nil {
		result, sbErr := e.sandbox.Execute(execCtx, chat.Command{
			Args:    fields,
			WorkDir: e.workspaceAbs,
		}, chat.SandboxWorkspace)
		if sbErr != nil {
			if execCtx.Err() != nil {
				return "command killed after timeout", nil
			}
			// The gate reports non-zero exits as an error; map to the same
			// stable, non-sensitive category as the direct path.
			_ = safeError("command_failed", sbErr)
			return safeErrorMessage("command_failed"), nil
		}
		output := result.Stdout
		if result.Stderr != "" {
			if output != "" {
				output += "\n"
			}
			output += result.Stderr
		}
		return string(truncateBytes([]byte(output), 50*1024)), nil
	}

	// Direct path: inside the jail the enclosing sandbox provides isolation.
	// #nosec G204 — the command name is validated via allowlist above.
	cmd := exec.CommandContext(execCtx, fields[0], fields[1:]...)
	cmd.Dir = e.workspaceAbs

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Return output even on error — the LLM may want to see stderr.
		truncated := truncateBytes(output, 50*1024)
		if execCtx.Err() != nil {
			return "command killed after timeout", nil
		}
		info := safeError("command_failed", err)
		_ = truncated // command stderr may contain secrets or prompt content.
		return safeErrorMessage(info.Code), nil
	}

	truncated := truncateBytes(output, 50*1024)
	return string(truncated), nil
}

// ─── Tool: search_codebase ────────────────────────────────────────────────────

func (e *ToolExecutor) searchCodebase(ctx context.Context, args map[string]any) (string, error) {
	query, ok := extractStringArg(args, "query")
	if !ok || query == "" {
		return "", fmt.Errorf("search_codebase: missing required parameter \"query\"")
	}

	searchPath, _ := extractStringArg(args, "path")
	if searchPath == "" {
		searchPath = e.workspaceDir
	}

	resolved, err := e.resolveAndValidate(searchPath)
	if err != nil {
		return "", fmt.Errorf("search_codebase: %w", err)
	}

	// Check context before walking.
	if err := ctx.Err(); err != nil {
		return "", err
	}

	const maxResults = 50
	var (
		results    []string
		walkErr    error
		lowerQuery = strings.ToLower(query)
		skipDirs   = map[string]bool{".git": true, "node_modules": true, "vendor": true}
	)

	walkErr = filepath.Walk(resolved, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip binary files by heuristic.
		if isBinaryFile(path) {
			return nil
		}

		// Check context periodically.
		if ctx.Err() != nil {
			return ctx.Err()
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		lines := strings.Split(string(data), "\n")
		relPath, _ := filepath.Rel(resolved, path)
		if relPath == "" {
			relPath = filepath.Base(path)
		}

		for lineNum, line := range lines {
			if strings.Contains(strings.ToLower(line), lowerQuery) {
				trimmed := strings.TrimSpace(line)
				if len(trimmed) > 200 {
					trimmed = trimmed[:200]
				}
				results = append(results, fmt.Sprintf("%s:%d: %s", relPath, lineNum+1, trimmed))
				if len(results) >= maxResults {
					return filepath.SkipAll // sentinel — stop walking
				}
			}
		}

		return nil
	})

	// SkipAll is not really an error.
	if walkErr == filepath.SkipAll {
		walkErr = nil
	}
	if walkErr != nil {
		return "", fmt.Errorf("search_codebase: walk error: %w", walkErr)
	}

	if len(results) == 0 {
		return fmt.Sprintf("No matches found for query: %q", query), nil
	}

	header := fmt.Sprintf("Found %d matches for %q:\n", len(results), query)
	return header + strings.Join(results, "\n"), nil
}

// ─── Tool: read_memory ────────────────────────────────────────────────────────

func (e *ToolExecutor) readMemory(ctx context.Context, args map[string]any) (string, error) {
	if e.memoryRetriever == nil {
		return "", fmt.Errorf("read_memory: memory retriever is not configured")
	}

	query, ok := extractStringArg(args, "query")
	if !ok || query == "" {
		return "", fmt.Errorf("read_memory: missing required parameter \"query\"")
	}

	records, err := e.memoryRetriever.Search(ctx, query, MemorySearchOptions{
		Limit: 10,
	})
	if err != nil {
		return "", fmt.Errorf("read_memory: search failed: %w", err)
	}

	if len(records) == 0 {
		return fmt.Sprintf("No memory records found for query: %q", query), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d memory records:\n\n", len(records))
	for i, rec := range records {
		fmt.Fprintf(&sb, "--- Record %d (ID: %s, Type: %s, Layer: %s) ---\n", i+1, rec.ID, rec.Type, rec.Layer)
		sb.WriteString(rec.Content)
		sb.WriteString("\n\n")
	}

	return sb.String(), nil
}

// ─── Execute Loop ─────────────────────────────────────────────────────────────

// ExecuteLoop runs the tool execution loop:
// LLM → Tool Calls → Execute → Send Results → LLM → Response.
// It returns the final response after all tool calls are resolved.
func (e *ToolExecutor) ExecuteLoop(
	ctx context.Context,
	provider chat.ChatProvider,
	messages []chat.Message,
	maxIterations int,
) (string, error) {
	if maxIterations <= 0 {
		maxIterations = 10
	}

	// Work on a copy to avoid mutating the caller's slice.
	msgs := make([]chat.Message, len(messages))
	copy(msgs, messages)

	for iteration := 0; iteration < maxIterations; iteration++ {
		select {
		case <-ctx.Done():
			return "", safeContextError("tool_loop_cancelled", ctx.Err())
		default:
		}

		logger := log.Ctx(ctx).With().
			Int("iteration", iteration).
			Int("message_count", len(msgs)).
			Logger()

		// Call the LLM with current messages.
		resp, err := provider.Chat(ctx, msgs, chat.ChatOptions{
			Temperature: 0.7,
		})
		if err != nil {
			return "", safeContextError("tool_loop_chat_failed", err)
		}

		if resp == nil || len(resp.Choices) == 0 {
			return "", fmt.Errorf("execute loop: empty response at iteration %d", iteration)
		}

		choice := resp.Choices[0]
		assistantMsg := choice.Message

		// If no tool calls, this is the final response.
		if len(assistantMsg.ToolCalls) == 0 {
			return assistantMsg.Content, nil
		}

		logger.Info().
			Int("tool_call_count", len(assistantMsg.ToolCalls)).
			Strs("tool_names", toolCallNames(assistantMsg.ToolCalls)).
			Msg("executing tool calls")

		// Add the assistant message (with tool calls) to the conversation.
		msgs = append(msgs, assistantMsg)

		// Execute all tool calls.
		results, err := e.ExecuteAll(ctx, assistantMsg.ToolCalls)
		if err != nil {
			info := safeError("tool_batch_partial_failure", err)
			logger.Warn().Str("error_code", info.Code).Str("error_hash", info.Hash).Int("error_length", info.Length).Msg("some tool calls failed, continuing loop")
		}

		// Add tool result messages to the conversation.
		for _, result := range results {
			toolContent := result.Content
			if result.Error != "" {
				toolContent = fmt.Sprintf("Tool execution error: %s\nTool output:\n%s", result.Error, toolContent)
			}
			msgs = append(msgs, chat.Message{
				Role:       chat.RoleTool,
				Content:    contenttrust.Envelope(contenttrust.Default(contenttrust.OriginTool, toolContent, result.Name)),
				ToolCallID: result.ToolCallID,
			})
		}
	}

	return "", safePublicError("execute loop", "tool_loop_max_iterations", fmt.Errorf("maximum iterations exceeded"))
}

// ─── Path Resolution & Validation ─────────────────────────────────────────────

// resolveAndValidate resolves a relative or absolute path and validates that it
// is within the workspace directory (path traversal prevention).
func (e *ToolExecutor) resolveAndValidate(pathArg string) (string, error) {
	// Join with workspaceDir if relative.
	joined := pathArg
	if !filepath.IsAbs(joined) {
		joined = filepath.Join(e.workspaceAbs, joined)
	}

	// Clean and resolve symlinks.
	cleaned := filepath.Clean(joined)

	// Use EvalSymlinks so that symlink tricks can't escape the sandbox.
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		// If the file doesn't exist yet (e.g. for write_file), use the cleaned
		// path but still validate it.
		if os.IsNotExist(err) {
			resolved = cleaned
		} else {
			return "", fmt.Errorf("path resolution failed: %w", err)
		}
	}

	// For non-existent files, validate the parent directory exists within
	// the workspace.
	if resolved != cleaned {
		// Resolved via symlinks — verify.
		if !isPathWithin(e.workspaceAbs, resolved) {
			return "", fmt.Errorf("path traversal blocked: %q resolves outside workspace", pathArg)
		}
	} else {
		// File doesn't exist — check that the path prefix is within workspace.
		if !isPathWithin(e.workspaceAbs, filepath.Clean(joined)) {
			return "", fmt.Errorf("path traversal blocked: %q is outside workspace", pathArg)
		}
		// Also check the parent directory for extra safety,
		// but only if the parent is below the workspace (not at or above it).
		parent := filepath.Dir(filepath.Clean(joined))
		parentInWorkspace := isPathWithin(e.workspaceAbs, parent)
		if parentInWorkspace {
			if _, err := os.Stat(parent); err == nil {
				parentResolved, err := filepath.EvalSymlinks(parent)
				if err == nil && !isPathWithin(e.workspaceAbs, parentResolved) {
					return "", fmt.Errorf("path traversal blocked: parent of %q resolves outside workspace", pathArg)
				}
			}
		}
	}

	return resolved, nil
}

// isPathWithin returns true if child is equal to or a descendant of parent.
func isPathWithin(parent, child string) bool {
	// Normalize separators.
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)

	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}

	// If rel starts with "..", child is outside parent.
	return !strings.HasPrefix(rel, "..")
}

// ─── Binary File Detection ────────────────────────────────────────────────────

// isBinaryFile returns true if the file at the given path is likely binary.
// It reads the first 512 bytes and checks for null bytes.
func isBinaryFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return true // assume binary on error
	}
	defer func() { _ = f.Close() }()

	var buf [512]byte
	n, _ := f.Read(buf[:])
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}
	return false
}

// ─── Argument Parsing Helpers ─────────────────────────────────────────────────

// parseToolArgs parses a JSON string into a map of arguments.
func parseToolArgs(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return nil, fmt.Errorf("invalid JSON arguments: %w", err)
	}
	return args, nil
}

// extractStringArg extracts a string argument from a tool arguments map.
// Returns the value and true if the key exists and is a string.
func extractStringArg(args map[string]any, key string) (string, bool) {
	v, ok := args[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	return s, true
}

// ─── Utility Functions ────────────────────────────────────────────────────────

// formatSize returns a human-readable file size string.
func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.1f %s", float64(size)/float64(div), units[exp])
}

// truncateBytes truncates a byte slice to maxLen bytes.
func truncateBytes(data []byte, maxLen int) []byte {
	if len(data) <= maxLen {
		return data
	}
	return data[:maxLen]
}
