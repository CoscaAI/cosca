package orchestration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
)

// ─── Helpers ─────────────────────────────────────────────────────────────────

// newTestToolExecutor creates a ToolExecutor scoped to a temp directory.
func newTestToolExecutor(workspaceDir string) *ToolExecutor {
	return NewToolExecutor(ToolExecutorConfig{
		WorkspaceDir:    workspaceDir,
		AllowedCommands: []string{"echo", "ls", "cat", "pwd", "date", "which", "env", "go", "git"},
		MaxFileSize:     10 * 1024 * 1024,
		Timeout:         5 * time.Second,
	})
}

// makeToolCall creates a chat.ToolCall with the given name and arguments map.
func makeToolCall(id, name string, args map[string]any) chat.ToolCall {
	argsJSON, _ := json.Marshal(args)
	return chat.ToolCall{
		ID:   id,
		Type: "function",
		Function: chat.FunctionCall{
			Name:      name,
			Arguments: string(argsJSON),
		},
	}
}

// mustWriteFile creates a file in the given directory for test setup.
func mustWriteFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	return path
}

// ─── Tests: read_file ────────────────────────────────────────────────────────

func TestToolExecutor_ReadFile(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, dir, "hello.txt", "Hello, World!")

	executor := newTestToolExecutor(dir)
	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "read_file", map[string]any{
		"path": "hello.txt",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}
	if result.Content != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %q", result.Content)
	}
	if result.ToolCallID != "tc-1" {
		t.Errorf("expected ToolCallID='tc-1', got %q", result.ToolCallID)
	}
	if result.Name != "read_file" {
		t.Errorf("expected Name='read_file', got %q", result.Name)
	}
	if result.Duration <= 0 {
		t.Error("expected non-zero duration")
	}
}

func TestToolExecutor_ReadFile_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	executor := newTestToolExecutor(dir)

	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "read_file", map[string]any{
		"path": "../../../etc/passwd",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error for path traversal, got none")
	}
	if !strings.Contains(result.Error, "path traversal") && !strings.Contains(result.Error, "outside workspace") {
		t.Errorf("expected path traversal error, got: %s", result.Error)
	}
}

func TestToolExecutor_ReadFile_PathTraversal_Symlink(t *testing.T) {
	dir := t.TempDir()

	// Create an outside directory with a file.
	outsideDir := t.TempDir()
	mustWriteFile(t, outsideDir, "secret.txt", "top secret!")

	// Create a symlink inside the workspace pointing outside.
	symlinkPath := filepath.Join(dir, "escape")
	if err := os.Symlink(outsideDir, symlinkPath); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	executor := newTestToolExecutor(dir)
	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "read_file", map[string]any{
		"path": "escape/secret.txt",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error for symlink escape, got none")
	}
	if !strings.Contains(result.Error, "path traversal") && !strings.Contains(result.Error, "outside workspace") {
		t.Errorf("expected symlink escape to be blocked, got: %s", result.Error)
	}
}

func TestToolExecutor_ReadFile_NotFound(t *testing.T) {
	dir := t.TempDir()
	executor := newTestToolExecutor(dir)

	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "read_file", map[string]any{
		"path": "nonexistent.txt",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error for missing file, got none")
	}
}

func TestToolExecutor_ReadFile_IsDirectory(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	executor := newTestToolExecutor(dir)
	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "read_file", map[string]any{
		"path": "subdir",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error for directory, got none")
	}
	if !strings.Contains(result.Error, "is a directory") {
		t.Errorf("expected 'is a directory' error, got: %s", result.Error)
	}
}

func TestToolExecutor_ReadFile_MissingPathArg(t *testing.T) {
	dir := t.TempDir()
	executor := newTestToolExecutor(dir)

	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "read_file", map[string]any{}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error for missing path argument")
	}
	if !strings.Contains(result.Error, "missing required parameter") {
		t.Errorf("expected missing parameter error, got: %s", result.Error)
	}
}

func TestToolExecutor_ReadFile_LargeFile(t *testing.T) {
	dir := t.TempDir()

	// Create a small but valid max file size.
	executor := NewToolExecutor(ToolExecutorConfig{
		WorkspaceDir:    dir,
		AllowedCommands: []string{"echo"},
		MaxFileSize:     10, // only 10 bytes
		Timeout:         5 * time.Second,
	})

	// Write a file larger than 10 bytes.
	mustWriteFile(t, dir, "big.txt", "This is more than 10 bytes of content")

	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "read_file", map[string]any{
		"path": "big.txt",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error for file exceeding max size")
	}
	if !strings.Contains(result.Error, "exceeds maximum") {
		t.Errorf("expected size limit error, got: %s", result.Error)
	}
}

func TestToolExecutor_ReadFile_AbsolutePathWithinWorkspace(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, dir, "data.txt", "workspace data")

	executor := newTestToolExecutor(dir)
	absPath := filepath.Join(dir, "data.txt")

	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "read_file", map[string]any{
		"path": absPath,
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}
	if result.Content != "workspace data" {
		t.Errorf("expected 'workspace data', got %q", result.Content)
	}
}

// ─── Tests: write_file ───────────────────────────────────────────────────────

func TestToolExecutor_WriteFile(t *testing.T) {
	dir := t.TempDir()
	executor := newTestToolExecutor(dir)

	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "write_file", map[string]any{
		"path":    "output.txt",
		"content": "Generated content\nLine 2",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}
	if !strings.Contains(result.Content, "Successfully wrote") {
		t.Errorf("expected success message, got: %s", result.Content)
	}

	// Verify the file was actually written.
	data, err := os.ReadFile(filepath.Join(dir, "output.txt"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(data) != "Generated content\nLine 2" {
		t.Errorf("file content mismatch: %q", string(data))
	}
}

func TestToolExecutor_WriteFile_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	executor := newTestToolExecutor(dir)

	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "write_file", map[string]any{
		"path":    "deep/nested/dir/file.txt",
		"content": "nested content",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}

	data, err := os.ReadFile(filepath.Join(dir, "deep/nested/dir/file.txt"))
	if err != nil {
		t.Fatalf("failed to read nested file: %v", err)
	}
	if string(data) != "nested content" {
		t.Errorf("nested file content mismatch: %q", string(data))
	}
}

func TestToolExecutor_WriteFile_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	executor := newTestToolExecutor(dir)

	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "write_file", map[string]any{
		"path":    "../../../etc/malicious",
		"content": "evil",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error for path traversal in write")
	}
}

// ─── Tests: list_files ───────────────────────────────────────────────────────

func TestToolExecutor_ListFiles(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, dir, "a.txt", "a")
	mustWriteFile(t, dir, "b.txt", "bb")
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	executor := newTestToolExecutor(dir)
	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "list_files", map[string]any{
		"path": ".",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}
	if !strings.Contains(result.Content, "a.txt") {
		t.Error("expected a.txt in listing")
	}
	if !strings.Contains(result.Content, "b.txt") {
		t.Error("expected b.txt in listing")
	}
	if !strings.Contains(result.Content, "sub") {
		t.Error("expected sub directory in listing")
	}
	if !strings.Contains(result.Content, "dir") || !strings.Contains(result.Content, "file") {
		t.Logf("listing output: %s", result.Content)
	}
}

func TestToolExecutor_ListFiles_DefaultPath(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, dir, "test.txt", "content")

	executor := newTestToolExecutor(dir)
	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "list_files", map[string]any{}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}
	if !strings.Contains(result.Content, "test.txt") {
		t.Error("expected test.txt in default path listing")
	}
}

func TestToolExecutor_ListFiles_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	executor := newTestToolExecutor(dir)

	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "list_files", map[string]any{
		"path": "../../../",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error for path traversal in list")
	}
}

// ─── Tests: execute_command ──────────────────────────────────────────────────

func TestToolExecutor_ExecuteCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("comandos POSIX (echo) não existem como executáveis no Windows — exec direto retorna command_failed")
	}
	dir := t.TempDir()
	executor := newTestToolExecutor(dir)

	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "execute_command", map[string]any{
		"command": "echo hello world",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}
	if !strings.Contains(result.Content, "hello world") {
		t.Errorf("expected 'hello world' in output, got: %s", result.Content)
	}
}

func TestToolExecutor_ExecuteCommand_Pwd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("comando POSIX 'pwd' não existe como executável no Windows — exec direto retorna command_failed")
	}
	dir := t.TempDir()
	executor := newTestToolExecutor(dir)

	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "execute_command", map[string]any{
		"command": "pwd",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}
	// pwd output should be the workspace directory.
	content := strings.TrimSpace(result.Content)
	if content != dir {
		t.Errorf("expected pwd to be workspace dir %q, got %q", dir, content)
	}
}

func TestToolExecutor_ExecuteCommand_NotAllowed(t *testing.T) {
	dir := t.TempDir()
	executor := newTestToolExecutor(dir)

	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "execute_command", map[string]any{
		"command": "curl http://evil.com",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error for disallowed command")
	}
	if !strings.Contains(result.Error, "not in the allowed list") {
		t.Errorf("expected allowlist error, got: %s", result.Error)
	}
}

func TestToolExecutor_ExecuteCommand_EmptyCommand(t *testing.T) {
	dir := t.TempDir()
	executor := newTestToolExecutor(dir)

	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "execute_command", map[string]any{
		"command": "",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error for empty command")
	}
}

func TestToolExecutor_ExecuteCommand_MissingArg(t *testing.T) {
	dir := t.TempDir()
	executor := newTestToolExecutor(dir)

	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "execute_command", map[string]any{}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error for missing command argument")
	}
}

func TestToolExecutor_ExecuteCommand_FailingCommand(t *testing.T) {
	dir := t.TempDir()
	executor := newTestToolExecutor(dir)

	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "execute_command", map[string]any{
		"command": "ls /nonexistent/path/xyz123",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// A failing command should still produce a result (with output and error message).
	if result.Error != "" {
		t.Logf("tool error (expected, command output returned): %s", result.Error)
	}
	// Output should contain the ls error message.
	if !strings.Contains(result.Content, "No such file") && !strings.Contains(result.Content, "cannot access") && !strings.Contains(result.Content, "exited with error") {
		t.Logf("command output: %s", result.Content)
	}
}

// ─── Tests: search_codebase ──────────────────────────────────────────────────

func TestToolExecutor_SearchCodebase(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, dir, "main.go", `package main
import "fmt"
func main() {
    fmt.Println("Hello, World!")
}`)
	mustWriteFile(t, dir, "utils.go", `package main
func helper() string {
    return "helper function"
}`)

	executor := newTestToolExecutor(dir)
	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "search_codebase", map[string]any{
		"query": "fmt.Println",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}
	if !strings.Contains(result.Content, "main.go") {
		t.Error("expected main.go in search results")
	}
	if !strings.Contains(result.Content, "fmt.Println") {
		t.Error("expected fmt.Println in search results")
	}
	if strings.Contains(result.Content, "utils.go") {
		t.Error("expected utils.go NOT in results for fmt.Println query")
	}
}

func TestToolExecutor_SearchCodebase_MultipleMatches(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, dir, "a.go", "// TODO: fix this\nfunc a() {}")
	mustWriteFile(t, dir, "b.go", "// TODO: fix that\nfunc b() {}")

	executor := newTestToolExecutor(dir)
	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "search_codebase", map[string]any{
		"query": "TODO",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}
	if !strings.Contains(result.Content, "a.go") {
		t.Error("expected a.go in results")
	}
	if !strings.Contains(result.Content, "b.go") {
		t.Error("expected b.go in results")
	}
}

func TestToolExecutor_SearchCodebase_NoMatch(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, dir, "code.go", "package main")

	executor := newTestToolExecutor(dir)
	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "search_codebase", map[string]any{
		"query": "nonexistent_pattern_xyz",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}
	if !strings.Contains(result.Content, "No matches found") {
		t.Errorf("expected 'No matches found', got: %s", result.Content)
	}
}

func TestToolExecutor_SearchCodebase_SkipsGitDir(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, gitDir, "config", "[core]\n\trepository = todo")
	mustWriteFile(t, dir, "readme.md", "This is a TODO list")

	executor := newTestToolExecutor(dir)
	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "search_codebase", map[string]any{
		"query": "TODO",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}
	// Should find the TODO in readme.md but not in .git/config.
	if !strings.Contains(result.Content, "readme.md") {
		t.Error("expected readme.md in results")
	}
	if strings.Contains(result.Content, ".git") {
		t.Error("expected .git files to be skipped")
	}
}

func TestToolExecutor_SearchCodebase_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, dir, "code.go", "func HelloWorld() {}")

	executor := newTestToolExecutor(dir)
	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "search_codebase", map[string]any{
		"query": "helloworld",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}
	if !strings.Contains(result.Content, "code.go") {
		t.Error("expected case-insensitive match")
	}
}

func TestToolExecutor_SearchCodebase_MissingQuery(t *testing.T) {
	dir := t.TempDir()
	executor := newTestToolExecutor(dir)

	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "search_codebase", map[string]any{}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error for missing query")
	}
}

// ─── Tests: ExecuteAll ───────────────────────────────────────────────────────

func TestToolExecutor_ExecuteAll(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, dir, "file1.txt", "content 1")
	mustWriteFile(t, dir, "file2.txt", "content 2")

	executor := newTestToolExecutor(dir)

	toolCalls := []chat.ToolCall{
		makeToolCall("tc-1", "read_file", map[string]any{"path": "file1.txt"}),
		makeToolCall("tc-2", "read_file", map[string]any{"path": "file2.txt"}),
	}

	results, err := executor.ExecuteAll(context.Background(), toolCalls)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].Content != "content 1" {
		t.Errorf("expected 'content 1', got %q", results[0].Content)
	}
	if results[1].Content != "content 2" {
		t.Errorf("expected 'content 2', got %q", results[1].Content)
	}
}

func TestToolExecutor_ExecuteAll_MixedSuccessAndFailure(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, dir, "exists.txt", "data")

	executor := newTestToolExecutor(dir)

	toolCalls := []chat.ToolCall{
		makeToolCall("tc-1", "read_file", map[string]any{"path": "exists.txt"}),
		makeToolCall("tc-2", "read_file", map[string]any{"path": "missing.txt"}),
		makeToolCall("tc-3", "read_file", map[string]any{"path": "exists.txt"}),
	}

	results, err := executor.ExecuteAll(context.Background(), toolCalls)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if results[0].Error != "" {
		t.Errorf("expected first call to succeed, got error: %s", results[0].Error)
	}
	if results[1].Error == "" {
		t.Error("expected second call to fail")
	}
	if results[2].Error != "" {
		t.Errorf("expected third call to succeed, got error: %s", results[2].Error)
	}
}

// ─── Tests: Timeout ──────────────────────────────────────────────────────────

func TestToolExecutor_Timeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("comando POSIX 'sleep' não existe como executável no Windows — o teste valida timeout de processo via sinal")
	}
	dir := t.TempDir()

	// Create executor with very short timeout.
	executor := NewToolExecutor(ToolExecutorConfig{
		WorkspaceDir:    dir,
		AllowedCommands: []string{"sleep"},
		MaxFileSize:     10 * 1024 * 1024,
		Timeout:         50 * time.Millisecond,
	})

	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "execute_command", map[string]any{
		"command": "sleep 10",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The command should have been killed by timeout.
	// The result should contain an error message about being killed or signal.
	t.Logf("timeout result: error=%s content=%s", result.Error, result.Content)
	if result.Error != "" {
		t.Logf("timeout error (may be expected): %s", result.Error)
	}
	// Either the error is set or the content mentions signal/killed.
	hasTimeout := result.Error != "" ||
		strings.Contains(result.Content, "killed") ||
		strings.Contains(result.Content, "signal") ||
		strings.Contains(result.Content, "deadline exceeded")

	if !hasTimeout {
		t.Errorf("expected timeout indication, got error=%q content=%q", result.Error, result.Content)
	}
}

// ─── Tests: Unsupported Tool ─────────────────────────────────────────────────

func TestToolExecutor_UnsupportedTool(t *testing.T) {
	dir := t.TempDir()
	executor := newTestToolExecutor(dir)

	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "nonexistent_tool", map[string]any{
		"arg": "value",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error for unsupported tool")
	}
	if !strings.Contains(result.Error, "unsupported tool") {
		t.Errorf("expected 'unsupported tool', got: %s", result.Error)
	}
}

// ─── Tests: Invalid Arguments ────────────────────────────────────────────────

func TestToolExecutor_InvalidJSONArguments(t *testing.T) {
	dir := t.TempDir()
	executor := newTestToolExecutor(dir)

	toolCall := chat.ToolCall{
		ID:   "tc-1",
		Type: "function",
		Function: chat.FunctionCall{
			Name:      "read_file",
			Arguments: "not valid json {{{",
		},
	}

	result, err := executor.Execute(context.Background(), toolCall)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error for invalid JSON arguments")
	}
	if !strings.Contains(result.Error, "invalid arguments") {
		t.Errorf("expected invalid arguments error, got: %s", result.Error)
	}
}

// ─── Tests: read_memory ──────────────────────────────────────────────────────

func TestToolExecutor_ReadMemory_NotConfigured(t *testing.T) {
	dir := t.TempDir()
	executor := newTestToolExecutor(dir)

	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "read_memory", map[string]any{
		"query": "test query",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error when memory retriever not configured")
	}
	if result.ErrorCode != "tool_execution_failed" || strings.Contains(result.Error, "memory retriever is not configured") {
		t.Errorf("expected safe memory error, got: %+v", result)
	}
}

func TestToolExecutor_ReadMemory_WithRetriever(t *testing.T) {
	dir := t.TempDir()
	executor := newTestToolExecutor(dir)

	mock := &mockMemoryRetriever{
		records: []MemoryRecord{
			{ID: "mem-1", Type: "conversation", Layer: "session", Content: "Previous chat about APIs"},
			{ID: "mem-2", Type: "decision", Layer: "project", Content: "Use Go for backend"},
		},
	}
	executor.SetMemoryRetriever(mock)

	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "read_memory", map[string]any{
		"query": "api design",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}
	if !strings.Contains(result.Content, "Previous chat about APIs") {
		t.Error("expected first memory record in results")
	}
	if !strings.Contains(result.Content, "Use Go for backend") {
		t.Error("expected second memory record in results")
	}
}

func TestToolExecutor_ReadMemory_NoResults(t *testing.T) {
	dir := t.TempDir()
	executor := newTestToolExecutor(dir)

	mock := &mockMemoryRetriever{
		records: []MemoryRecord{},
	}
	executor.SetMemoryRetriever(mock)

	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "read_memory", map[string]any{
		"query": "nonexistent",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}
	if !strings.Contains(result.Content, "No memory records found") {
		t.Errorf("expected 'No memory records found', got: %s", result.Content)
	}
}

func TestToolExecutor_ReadMemory_SearchError(t *testing.T) {
	dir := t.TempDir()
	executor := newTestToolExecutor(dir)

	mock := &mockMemoryRetriever{
		err: errors.New("memory engine down"),
	}
	executor.SetMemoryRetriever(mock)

	result, err := executor.Execute(context.Background(), makeToolCall("tc-1", "read_memory", map[string]any{
		"query": "test",
	}))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error from memory search failure")
	}
	if result.ErrorCode != "tool_execution_failed" || strings.Contains(result.Error, "memory engine down") {
		t.Errorf("expected safe memory error, got: %+v", result)
	}
}

// ─── Tests: ExecuteLoop ──────────────────────────────────────────────────────

// executeLoopMockProvider is a chat provider that returns controlled responses
// for testing the ExecuteLoop.
type executeLoopMockProvider struct {
	name      string
	model     string
	responses []*chat.ChatResponse
	callCount int
	afterChat func() // optional callback after each Chat call
}

func (m *executeLoopMockProvider) Chat(_ context.Context, _ []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
	if m.callCount >= len(m.responses) {
		return nil, fmt.Errorf("no more responses at call %d", m.callCount)
	}
	resp := m.responses[m.callCount]
	m.callCount++
	if m.afterChat != nil {
		m.afterChat()
	}
	return resp, nil
}

func (m *executeLoopMockProvider) ChatStream(_ context.Context, _ []chat.Message, _ chat.ChatOptions) (chat.ChatStream, error) {
	return nil, errors.New("not implemented")
}

func (m *executeLoopMockProvider) Model() string { return m.model }
func (m *executeLoopMockProvider) Name() string  { return m.name }
func (m *executeLoopMockProvider) Close() error  { return nil }

func makeResponse(content string, toolCalls ...chat.ToolCall) *chat.ChatResponse {
	return &chat.ChatResponse{
		ID:    "resp-1",
		Model: "test-model",
		Choices: []chat.Choice{
			{
				Index: 0,
				Message: chat.Message{
					Role:      chat.RoleAssistant,
					Content:   content,
					ToolCalls: toolCalls,
				},
				FinishReason: chat.FinishReasonStop,
			},
		},
		Usage: chat.Usage{TotalTokens: 10},
	}
}

func TestToolExecutor_Loop_SingleCall(t *testing.T) {
	dir := t.TempDir()
	executor := newTestToolExecutor(dir)

	// Provider that returns a simple text response with no tool calls.
	provider := &executeLoopMockProvider{
		name:  "test",
		model: "test-model",
		responses: []*chat.ChatResponse{
			makeResponse("This is the final answer."),
		},
	}

	messages := []chat.Message{
		{Role: chat.RoleSystem, Content: "You are a helpful assistant."},
		{Role: chat.RoleUser, Content: "Hello!"},
	}

	content, err := executor.ExecuteLoop(context.Background(), provider, messages, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "This is the final answer." {
		t.Errorf("expected 'This is the final answer.', got %q", content)
	}
	if provider.callCount != 1 {
		t.Errorf("expected 1 LLM call, got %d", provider.callCount)
	}
}

func TestToolExecutor_Loop_WithToolCalls(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, dir, "greeting.txt", "Hello from the file!")

	executor := newTestToolExecutor(dir)

	// Provider that:
	//   1. Returns a tool call to read greeting.txt
	//   2. Returns a final response based on the tool result
	provider := &executeLoopMockProvider{
		name:  "test",
		model: "test-model",
		responses: []*chat.ChatResponse{
			// First call: tool_call to read_file.
			{
				ID:    "resp-1",
				Model: "test-model",
				Choices: []chat.Choice{
					{
						Index: 0,
						Message: chat.Message{
							Role:    chat.RoleAssistant,
							Content: "",
							ToolCalls: []chat.ToolCall{
								makeToolCall("tc-1", "read_file", map[string]any{"path": "greeting.txt"}),
							},
						},
						FinishReason: chat.FinishReasonToolCalls,
					},
				},
				Usage: chat.Usage{TotalTokens: 15},
			},
			// Second call: final response with tool result context.
			makeResponse("The file says: Hello from the file!"),
		},
	}

	messages := []chat.Message{
		{Role: chat.RoleSystem, Content: "You can read files."},
		{Role: chat.RoleUser, Content: "What's in greeting.txt?"},
	}

	content, err := executor.ExecuteLoop(context.Background(), provider, messages, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "The file says: Hello from the file!" {
		t.Errorf("unexpected final content: %q", content)
	}
	if provider.callCount != 2 {
		t.Errorf("expected 2 LLM calls (1 tool + 1 final), got %d", provider.callCount)
	}
}

func TestToolExecutor_Loop_MultipleToolCallsInOneTurn(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, dir, "file_a.txt", "Alpha")
	mustWriteFile(t, dir, "file_b.txt", "Beta")

	executor := newTestToolExecutor(dir)

	provider := &executeLoopMockProvider{
		name:  "test",
		model: "test-model",
		responses: []*chat.ChatResponse{
			// First call: two tool calls.
			{
				ID:    "resp-1",
				Model: "test-model",
				Choices: []chat.Choice{
					{
						Index: 0,
						Message: chat.Message{
							Role:    chat.RoleAssistant,
							Content: "",
							ToolCalls: []chat.ToolCall{
								makeToolCall("tc-1", "read_file", map[string]any{"path": "file_a.txt"}),
								makeToolCall("tc-2", "read_file", map[string]any{"path": "file_b.txt"}),
							},
						},
						FinishReason: chat.FinishReasonToolCalls,
					},
				},
				Usage: chat.Usage{TotalTokens: 20},
			},
			// Second call: final.
			makeResponse("Read Alpha and Beta."),
		},
	}

	messages := []chat.Message{
		{Role: chat.RoleUser, Content: "Read both files."},
	}

	content, err := executor.ExecuteLoop(context.Background(), provider, messages, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "Read Alpha and Beta." {
		t.Errorf("unexpected final content: %q", content)
	}
	if provider.callCount != 2 {
		t.Errorf("expected 2 LLM calls, got %d", provider.callCount)
	}
}

func TestToolExecutor_Loop_MaxIterations(t *testing.T) {
	dir := t.TempDir()
	executor := newTestToolExecutor(dir)

	// Provider that always returns tool calls, never a final answer.
	provider := &executeLoopMockProvider{
		name:  "test",
		model: "test-model",
		responses: func() []*chat.ChatResponse {
			var resps []*chat.ChatResponse
			for i := 0; i < 20; i++ {
				resps = append(resps, &chat.ChatResponse{
					ID:    fmt.Sprintf("resp-%d", i),
					Model: "test-model",
					Choices: []chat.Choice{
						{
							Index: 0,
							Message: chat.Message{
								Role: chat.RoleAssistant,
								ToolCalls: []chat.ToolCall{
									makeToolCall(fmt.Sprintf("tc-%d", i), "list_files", map[string]any{}),
								},
							},
							FinishReason: chat.FinishReasonToolCalls,
						},
					},
					Usage: chat.Usage{TotalTokens: 5},
				})
			}
			return resps
		}(),
	}

	messages := []chat.Message{
		{Role: chat.RoleUser, Content: "List files repeatedly."},
	}

	_, err := executor.ExecuteLoop(context.Background(), provider, messages, 3)
	if err == nil {
		t.Fatal("expected error for exceeding max iterations")
	}
	if !strings.Contains(err.Error(), "exceeded maximum iterations") {
		t.Errorf("expected max iterations error, got: %v", err)
	}
}

func TestToolExecutor_Loop_ContextCancellation(t *testing.T) {
	dir := t.TempDir()
	executor := newTestToolExecutor(dir)

	provider := &executeLoopMockProvider{
		name:  "test",
		model: "test-model",
		responses: []*chat.ChatResponse{
			{
				ID:    "resp-1",
				Model: "test-model",
				Choices: []chat.Choice{
					{
						Index: 0,
						Message: chat.Message{
							Role: chat.RoleAssistant,
							ToolCalls: []chat.ToolCall{
								makeToolCall("tc-1", "list_files", map[string]any{}),
							},
						},
						FinishReason: chat.FinishReasonToolCalls,
					},
				},
				Usage: chat.Usage{TotalTokens: 5},
			},
			// Provider would return more, but context is cancelled.
			makeResponse("Should not reach here."),
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	messages := []chat.Message{
		{Role: chat.RoleUser, Content: "List files."},
	}

	// Cancel after the first call completes (before the tool results are sent back).
	cancelAfterFirstCall := make(chan struct{}, 1)
	go func() {
		<-cancelAfterFirstCall
		cancel()
	}()

	content, err := executor.ExecuteLoop(ctx, provider, messages, 10)

	// Either we get an error or the loop completes before cancellation.
	if err != nil {
		if !strings.Contains(err.Error(), "cancelled") && !strings.Contains(err.Error(), "canceled") {
			t.Errorf("expected cancellation error, got: %v", err)
		}
	} else {
		t.Logf("loop completed before cancellation, final content: %q", content)
	}
}

// ─── Tests: DefaultToolExecutorConfig ────────────────────────────────────────

func TestDefaultToolExecutorConfig_Defaults(t *testing.T) {
	cfg := DefaultToolExecutorConfig()

	if cfg.WorkspaceDir != "." {
		t.Errorf("expected WorkspaceDir='.', got %q", cfg.WorkspaceDir)
	}
	if cfg.MaxFileSize != 10*1024*1024 {
		t.Errorf("expected MaxFileSize=10MB, got %d", cfg.MaxFileSize)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("expected Timeout=30s, got %v", cfg.Timeout)
	}
	if len(cfg.AllowedCommands) == 0 {
		t.Error("expected non-empty AllowedCommands")
	}
}

func TestNewToolExecutor_DefaultsApplied(t *testing.T) {
	exec := NewToolExecutor(ToolExecutorConfig{})
	if exec.workspaceDir != "." {
		t.Errorf("expected default workspaceDir, got %q", exec.workspaceDir)
	}
	if exec.maxFileSize != 10*1024*1024 {
		t.Errorf("expected default maxFileSize, got %d", exec.maxFileSize)
	}
	if exec.timeout != 30*time.Second {
		t.Errorf("expected default timeout, got %v", exec.timeout)
	}
	if len(exec.allowedCommands) == 0 {
		t.Error("expected default allowedCommands")
	}
}

func TestNewToolExecutor_CustomAllowedCommands(t *testing.T) {
	custom := []string{"echo", "ls"}
	exec := NewToolExecutor(ToolExecutorConfig{
		WorkspaceDir:    "/tmp",
		AllowedCommands: custom,
	})

	if len(exec.allowedCommands) != 2 {
		t.Errorf("expected 2 allowed commands, got %d", len(exec.allowedCommands))
	}
	if !exec.allowedCommandsSet["echo"] {
		t.Error("expected 'echo' in allowed set")
	}
	if !exec.allowedCommandsSet["ls"] {
		t.Error("expected 'ls' in allowed set")
	}
	if exec.allowedCommandsSet["curl"] {
		t.Error("did not expect 'curl' in allowed set")
	}
}

// ─── Tests: Tool Integration in Executor ─────────────────────────────────────

func TestExecutor_WithToolExecutor_ExecutesToolCalls(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, dir, "secret.txt", "the secret is 42")

	toolExec := newTestToolExecutor(dir)

	// Mock provider that returns a tool_call for read_file.
	provider := newMockChatProvider("test", "test-model")
	provider.chatResponse = &chat.ChatResponse{
		ID:    "resp-tools",
		Model: "test-model",
		Choices: []chat.Choice{
			{
				Index: 0,
				Message: chat.Message{
					Role:    chat.RoleAssistant,
					Content: "",
					ToolCalls: []chat.ToolCall{
						makeToolCall("tc-1", "read_file", map[string]any{"path": "secret.txt"}),
					},
				},
				FinishReason: chat.FinishReasonToolCalls,
			},
		},
		Usage: chat.Usage{TotalTokens: 10},
	}

	// Second response: the follow-up call after tool results.
	secondCallDone := false
	provider.chatFn = func(_ context.Context, _ []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
		if !secondCallDone {
			secondCallDone = true
			return provider.chatResponse, nil
		}
		// Second call: final answer.
		return &chat.ChatResponse{
			ID:    "resp-final",
			Model: "test-model",
			Choices: []chat.Choice{
				{
					Index:        0,
					Message:      chat.Message{Role: chat.RoleAssistant, Content: "The secret is 42!"},
					FinishReason: chat.FinishReasonStop,
				},
			},
			Usage: chat.Usage{TotalTokens: 8},
		}, nil
	}

	exec := NewExecutor(provider, DefaultExecutorConfig(), toolExec)

	pc := NewPipelineContext("req-tool-integration", "read the secret")
	pc = pc.WithContextData("resolved_agent", "Backend Chief")
	pc = pc.WithContextData("agent_role", "Backend Chief")

	result, err := exec.Execute(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have tool results in context.
	toolResults, ok := result.Data.ToolResults.([]*ToolCallResult)
	if !ok {
		t.Fatal("expected tool_results in context data")
	}
	if len(toolResults) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(toolResults))
	}
	if toolResults[0].Content != "the secret is 42" {
		t.Errorf("expected 'the secret is 42', got %q", toolResults[0].Content)
	}

	// The llm_response should be from the follow-up call.
	finalResp := result.Data.LLMResponse
	if finalResp != "The secret is 42!" {
		t.Errorf("expected final response, got %q", finalResp)
	}
}

func TestExecutor_WithToolExecutor_ToolFailureFollowUp(t *testing.T) {
	dir := t.TempDir()
	toolExec := newTestToolExecutor(dir)

	// Mock provider: tool call for a missing file.
	provider := newMockChatProvider("test", "test-model")
	provider.chatResponse = &chat.ChatResponse{
		ID:    "resp-tools",
		Model: "test-model",
		Choices: []chat.Choice{
			{
				Index: 0,
				Message: chat.Message{
					Role:    chat.RoleAssistant,
					Content: "",
					ToolCalls: []chat.ToolCall{
						makeToolCall("tc-1", "read_file", map[string]any{"path": "missing.txt"}),
					},
				},
				FinishReason: chat.FinishReasonToolCalls,
			},
		},
		Usage: chat.Usage{TotalTokens: 10},
	}

	secondCallDone := false
	provider.chatFn = func(_ context.Context, _ []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
		if !secondCallDone {
			secondCallDone = true
			return provider.chatResponse, nil
		}
		return &chat.ChatResponse{
			ID:    "resp-final",
			Model: "test-model",
			Choices: []chat.Choice{
				{
					Index:        0,
					Message:      chat.Message{Role: chat.RoleAssistant, Content: "The file was not found."},
					FinishReason: chat.FinishReasonStop,
				},
			},
			Usage: chat.Usage{TotalTokens: 8},
		}, nil
	}

	exec := NewExecutor(provider, DefaultExecutorConfig(), toolExec)

	pc := NewPipelineContext("req-tool-fail", "read missing file")
	pc = pc.WithContextData("resolved_agent", "Backend Chief")
	pc = pc.WithContextData("agent_role", "Backend Chief")

	result, err := exec.Execute(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The follow-up should have succeeded.
	finalResp := result.Data.LLMResponse
	if finalResp != "The file was not found." {
		t.Errorf("expected 'The file was not found.', got %q", finalResp)
	}
}

func TestExecutor_NilToolExecutor_NoToolExecution(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	provider.chatResponse = &chat.ChatResponse{
		ID:    "resp-tools",
		Model: "test-model",
		Choices: []chat.Choice{
			{
				Index: 0,
				Message: chat.Message{
					Role:    chat.RoleAssistant,
					Content: "",
					ToolCalls: []chat.ToolCall{
						makeToolCall("tc-1", "read_file", map[string]any{"path": "test.txt"}),
					},
				},
				FinishReason: chat.FinishReasonToolCalls,
			},
		},
		Usage: chat.Usage{TotalTokens: 5},
	}

	exec := NewExecutor(provider, DefaultExecutorConfig(), nil) // nil toolExecutor

	pc := NewPipelineContext("req-nil-tools", "test")
	pc = pc.WithContextData("resolved_agent", "Backend Chief")
	pc = pc.WithContextData("agent_role", "Backend Chief")

	result, err := exec.Execute(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Without a tool executor, tool calls should still be stored but not executed.
	toolCalls, ok := result.Data.ToolCalls.([]chat.ToolCall)
	if !ok {
		t.Fatal("expected tool_calls in context")
	}
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}

	// tool_results should NOT be present.
	if result.Data.ToolResults != nil {
		t.Error("did not expect tool_results when toolExecutor is nil")
	}
}

// ─── Tests: Helper Functions ─────────────────────────────────────────────────

func TestFormatSize(t *testing.T) {
	tests := []struct {
		size     int64
		expected string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := formatSize(tt.size)
			if got != tt.expected {
				t.Errorf("formatSize(%d) = %q, want %q", tt.size, got, tt.expected)
			}
		})
	}
}

func TestTruncateBytes(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		maxLen  int
		wantLen int
	}{
		{"no truncation needed", []byte("hello"), 10, 5},
		{"exact truncation", []byte("hello"), 5, 5},
		{"truncation needed", []byte("hello world"), 5, 5},
		{"empty data", []byte{}, 10, 0},
		{"zero maxLen", []byte("hello"), 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateBytes(tt.data, tt.maxLen)
			if len(got) != tt.wantLen {
				t.Errorf("truncateBytes len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestParseToolArgs(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
		wantLen int
	}{
		{"valid json", `{"key": "value"}`, false, 1},
		{"empty string", "", false, 0},
		{"whitespace only", "   ", false, 0},
		{"invalid json", `{bad}`, true, 0},
		{"nested json", `{"outer": {"inner": "val"}}`, false, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, err := parseToolArgs(tt.raw)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if len(args) != tt.wantLen {
				t.Errorf("expected %d args, got %d", tt.wantLen, len(args))
			}
		})
	}
}

func TestExtractStringArg(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		key     string
		wantVal string
		wantOk  bool
	}{
		{"found", map[string]any{"key": "value"}, "key", "value", true},
		{"missing", map[string]any{}, "key", "", false},
		{"wrong type", map[string]any{"key": 42}, "key", "", false},
		{"empty string", map[string]any{"key": ""}, "key", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := extractStringArg(tt.args, tt.key)
			if ok != tt.wantOk {
				t.Errorf("expected ok=%v, got %v", tt.wantOk, ok)
			}
			if val != tt.wantVal {
				t.Errorf("expected val=%q, got %q", tt.wantVal, val)
			}
		})
	}
}

func TestIsPathWithin(t *testing.T) {
	tests := []struct {
		parent   string
		child    string
		expected bool
	}{
		{"/home/user/project", "/home/user/project/file.txt", true},
		{"/home/user/project", "/home/user/project/sub/dir/file.txt", true},
		{"/home/user/project", "/home/user/project", true},
		{"/home/user/project", "/home/user/other/file.txt", false},
		{"/home/user/project", "/etc/passwd", false},
		{"/home/user/project", "/home/user/project/../../../etc/passwd", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s contains %s", tt.parent, tt.child), func(t *testing.T) {
			got := isPathWithin(tt.parent, tt.child)
			if got != tt.expected {
				t.Errorf("isPathWithin(%q, %q) = %v, want %v", tt.parent, tt.child, got, tt.expected)
			}
		})
	}
}

func TestIsBinaryFile(t *testing.T) {
	dir := t.TempDir()

	// Text file.
	textPath := filepath.Join(dir, "text.txt")
	_ = os.WriteFile(textPath, []byte("hello world"), 0o644)
	if isBinaryFile(textPath) {
		t.Error("text file should not be detected as binary")
	}

	// Binary file (with null byte).
	binPath := filepath.Join(dir, "binary.bin")
	_ = os.WriteFile(binPath, []byte{0x48, 0x65, 0x00, 0x6c, 0x6f}, 0o644)
	if !isBinaryFile(binPath) {
		t.Error("binary file should be detected as binary")
	}

	// Non-existent file.
	if !isBinaryFile(filepath.Join(dir, "nonexistent")) {
		t.Error("non-existent file should be treated as binary")
	}
}
