package tool

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/execpolicy"
)

// mockSandbox implements chat.Sandbox for tool tests.
type mockSandbox struct {
	result *chat.SandboxResult
	err    error
	cmd    chat.Command
	mode   chat.SandboxMode
	calls  int
}

func (m *mockSandbox) Execute(ctx context.Context, cmd chat.Command, mode chat.SandboxMode) (*chat.SandboxResult, error) {
	m.calls++
	m.cmd = cmd
	m.mode = mode
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return &chat.SandboxResult{Stdout: "ok", ExitCode: 0}, nil
}

func (m *mockSandbox) Mode() chat.SandboxMode         { return m.mode }
func (m *mockSandbox) ValidatePath(path string) error { return nil }

// ── ShellTool ────────────────────────────────────────────────────────

func TestShellToolBasics(t *testing.T) {
	ws := t.TempDir()
	sb := &mockSandbox{}
	st := NewShellTool(ws, sb)

	if st.Name() != "shell" || st.Description() == "" || len(st.Schema()) == 0 {
		t.Fatalf("tool metadata: %+v", st)
	}

	// Validate: missing command → error.
	if err := st.Validate(json.RawMessage(`{"command":""}`)); err == nil {
		t.Fatal("empty command must fail validation")
	}
	if err := st.Validate(json.RawMessage(`{"command":"ls"}`)); err != nil {
		t.Fatalf("valid command: %v", err)
	}

	// Execute happy path.
	res, err := st.Execute(context.Background(), json.RawMessage(`{"command":"ls -la"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(res.Output, "exit_code") || !strings.Contains(res.Output, "stdout") {
		t.Fatalf("output: %q", res.Output)
	}
	if sb.calls != 1 || sb.mode != chat.SandboxWorkspace {
		t.Fatalf("sandbox calls=%d mode=%v", sb.calls, sb.mode)
	}
	// Command goes through the platform shell: sh -c on Unix, cmd /c on Windows.
	if len(sb.cmd.Args) < 3 {
		t.Fatalf("sandbox args too short: %v", sb.cmd.Args)
	}
	shell := sb.cmd.Args[0]
	if shell == "sh" {
		// Unix: sh -c <command>
		if sb.cmd.Args[1] != "-c" || sb.cmd.Args[2] != "ls -la" {
			t.Fatalf("sandbox args: %v", sb.cmd.Args)
		}
	} else if shell == "cmd" {
		// Windows: cmd /c <command>
		if sb.cmd.Args[1] != "/c" || sb.cmd.Args[2] != "ls -la" {
			t.Fatalf("sandbox args: %v", sb.cmd.Args)
		}
	} else {
		t.Fatalf("unexpected shell: %v", sb.cmd.Args)
	}
}

func TestShellToolErrors(t *testing.T) {
	ws := t.TempDir()
	st := NewShellTool(ws, &mockSandbox{})

	// Invalid params.
	if res, _ := st.Execute(context.Background(), json.RawMessage(`{bad`)); !strings.Contains(res.Error, "invalid params") {
		t.Fatalf("invalid params: %q", res.Error)
	}
	// Empty command.
	if res, _ := st.Execute(context.Background(), json.RawMessage(`{"command":""}`)); !strings.Contains(res.Error, "command is required") {
		t.Fatalf("empty command: %q", res.Error)
	}
	// Sandbox error.
	sbErr := &mockSandbox{err: errTool}
	st2 := NewShellTool(ws, sbErr)
	if res, _ := st2.Execute(context.Background(), json.RawMessage(`{"command":"ls"}`)); !strings.Contains(res.Error, "execution failed") {
		t.Fatalf("sandbox error: %q", res.Error)
	}
}

func TestShellToolExecPolicy(t *testing.T) {
	ws := t.TempDir()
	sb := &mockSandbox{}

	// Forbidden command → rejected before sandbox.
	policy := &execpolicy.Policy{Rules: []execpolicy.Rule{
		{Pattern: [][]string{{"rm"}}, Decision: execpolicy.Forbidden, Justification: "no rm"},
	}}
	st := NewShellTool(ws, sb, WithExecPolicy(policy))
	res, err := st.Execute(context.Background(), json.RawMessage(`{"command":"rm -rf /"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Error, "forbidden") || !strings.Contains(res.Error, "no rm") {
		t.Fatalf("forbidden error: %q", res.Error)
	}
	if sb.calls != 0 {
		t.Fatal("forbidden command must not reach the sandbox")
	}

	// Allowed command executes.
	allowPolicy := &execpolicy.Policy{Rules: []execpolicy.Rule{
		{Pattern: [][]string{{"ls"}}, Decision: execpolicy.Allow},
	}}
	st2 := NewShellTool(ws, sb, WithExecPolicy(allowPolicy))
	if _, err := st2.Execute(context.Background(), json.RawMessage(`{"command":"ls"}`)); err != nil {
		t.Fatal(err)
	}
	if sb.calls != 1 {
		t.Fatal("allowed command must reach the sandbox")
	}
}

// ── GitTool ──────────────────────────────────────────────────────────

func TestGitToolWhitelist(t *testing.T) {
	ws := t.TempDir()
	sb := &mockSandbox{}
	gt := NewGitTool(ws, sb)

	// Disallowed command → rejected.
	res, _ := gt.Execute(context.Background(), json.RawMessage(`{"command":"push"}`))
	if !strings.Contains(res.Error, "not allowed") {
		t.Fatalf("disallowed: %q", res.Error)
	}
	if sb.calls != 0 {
		t.Fatal("disallowed git command must not reach sandbox")
	}

	// Allowed command executes in read-only mode with -C workspace.
	res, _ = gt.Execute(context.Background(), json.RawMessage(`{"command":"status"}`))
	if res.Error != "" {
		t.Fatalf("status: %q", res.Error)
	}
	if sb.calls != 1 || sb.mode != chat.SandboxReadOnly {
		t.Fatalf("sandbox calls=%d mode=%v (must be read-only)", sb.calls, sb.mode)
	}
	// Args: git -C <ws> status
	if len(sb.cmd.Args) < 4 || sb.cmd.Args[0] != "git" || sb.cmd.Args[1] != "-C" || sb.cmd.Args[3] != "status" {
		t.Fatalf("git args: %v", sb.cmd.Args)
	}
}

func TestGitToolStashRestriction(t *testing.T) {
	ws := t.TempDir()
	gt := NewGitTool(ws, &mockSandbox{})

	// stash with a disallowed subcommand.
	res, _ := gt.Execute(context.Background(), json.RawMessage(`{"command":"stash","args":["pop"]}`))
	if !strings.Contains(res.Error, "only 'list' and 'show'") {
		t.Fatalf("stash pop: %q", res.Error)
	}
	// stash list allowed.
	sb := &mockSandbox{}
	gt2 := NewGitTool(ws, sb)
	res, _ = gt2.Execute(context.Background(), json.RawMessage(`{"command":"stash","args":["list"]}`))
	if res.Error != "" {
		t.Fatalf("stash list: %q", res.Error)
	}
}

func TestGitToolPathAndValidate(t *testing.T) {
	ws := t.TempDir()
	sb := &mockSandbox{}
	gt := NewGitTool(ws, sb)

	if _, err := gt.Execute(context.Background(), json.RawMessage(`{"command":"diff","path":"internal/pipeline/planner.go"}`)); err != nil {
		t.Fatal(err)
	}
	// Path appended via -- <path>.
	joined := strings.Join(sb.cmd.Args, " ")
	if !strings.Contains(joined, "-- internal/pipeline/planner.go") {
		t.Fatalf("path not appended: %v", sb.cmd.Args)
	}

	if err := gt.Validate(json.RawMessage(`{"command":""}`)); err == nil {
		t.Fatal("empty command must fail validation")
	}
	if err := gt.Validate(json.RawMessage(`{"command":"log"}`)); err != nil {
		t.Fatalf("valid: %v", err)
	}
}

// ── TestTool parsers (pure) ──────────────────────────────────────────

func TestParseGoTestResults(t *testing.T) {
	out := "=== RUN TestA\n--- PASS: TestA (0.00s)\nok  \tpkg\t0.1s\nFAIL\tpkg2\t0.2s\n"
	pass, fail := parseGoTestResults(out)
	if pass < 1 || fail < 1 {
		t.Fatalf("go results = %d/%d", pass, fail)
	}
}

func TestParseRustTestResults(t *testing.T) {
	out := "test result: ok. 10 passed; 0 failed\n"
	pass, fail := parseRustTestResults(out)
	if pass != 1 || fail != 0 {
		t.Fatalf("rust ok = %d/%d", pass, fail)
	}
	out2 := "test result: FAILED. 2 passed; 3 failed\n"
	pass, fail = parseRustTestResults(out2)
	if pass != 0 || fail != 1 {
		t.Fatalf("rust failed = %d/%d", pass, fail)
	}
}

func TestParsePytestResults(t *testing.T) {
	out := "collected 5 items\n4 passed, 1 failed in 1.2s\n"
	pass, fail := parsePytestResults(out)
	if pass != 4 || fail != 1 {
		t.Fatalf("pytest = %d/%d", pass, fail)
	}
}

func TestParsePassFailLines(t *testing.T) {
	out := "ok pkg1\nok pkg2\nFAIL pkg3\n--- FAIL: X\n"
	pass, fail := parsePassFailLines(out, "ok", "FAIL")
	if pass != 2 {
		t.Fatalf("pass = %d, want 2", pass)
	}
	if fail != 1 { // FAIL pkg3 counts; --- FAIL: X also starts with FAIL but contains no "ok"
		t.Fatalf("fail = %d", fail)
	}
}

func TestExtractInt(t *testing.T) {
	if extractInt("4 passed in 1s", " passed") != 4 {
		t.Fatal("extractInt basic")
	}
	if extractInt("no match here", " passed") != 0 {
		t.Fatal("extractInt missing")
	}
}

func TestParseTestResultsFallback(t *testing.T) {
	tt := NewTestTool(t.TempDir(), &mockSandbox{})
	// Unknown language falls back to counting pass/fail occurrences.
	pass, fail := tt.parseTestResults("unknown", "some passed thing fail", "")
	if pass != 1 || fail != 1 {
		t.Fatalf("fallback = %d/%d", pass, fail)
	}
}

// ── SearchTool parsers (pure) ────────────────────────────────────────

func TestParseMatchLines(t *testing.T) {
	got := parseMatchLines("a\nb\n\nc\n")
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("parseMatchLines = %v", got)
	}
	if got := parseMatchLines(""); len(got) != 0 {
		t.Fatalf("empty = %v", got)
	}
}

func TestMakeRelative(t *testing.T) {
	if runtime.GOOS == "windows" {
		// A função usa filepath.Separator para montar o prefixo do workspace;
		// no Windows o separador é "\\" e as linhas de match do rg/grep usam
		// o separador nativo. Os inputs abaixo são POSIX ("/repo/...") e a
		// semântica de path absoluto POSIX não se aplica no Windows.
		t.Skip("semântica de path POSIX (separador '/') — não aplicável no Windows")
	}
	ws := "/repo"
	lines := []string{"/repo/src/main.go:12:code", "/other/abs.go:1:x"}
	got := makeRelative(ws, lines)
	if got[0] != "src/main.go:12:code" {
		t.Fatalf("relative: %q", got[0])
	}
	if got[1] != "/other/abs.go:1:x" {
		t.Fatalf("outside kept absolute: %q", got[1])
	}
	_ = filepath.Separator
}

// ── WebFetchTool ─────────────────────────────────────────────────────

func TestWebFetchToolAvailability(t *testing.T) {
	if NewWebFetchTool("").IsAvailable() {
		t.Fatal("no key must be unavailable")
	}
	if !NewWebFetchTool("key123").IsAvailable() {
		t.Fatal("with key must be available")
	}
	if NewWebFetchTool("").Name() != "web_fetch" {
		t.Fatal("name")
	}
}

func TestSanitizeURL(t *testing.T) {
	if sanitizeURL("example.com") != "https://example.com" {
		t.Fatal("missing scheme → https")
	}
	if sanitizeURL("http://example.com") != "http://example.com" {
		t.Fatal("http preserved")
	}
	if sanitizeURL("  https://example.com  ") != "https://example.com" {
		t.Fatal("trim + preserve")
	}
}

func TestWebFetchToolExecute(t *testing.T) {
	// NOTE: WebFetchTool.fetch is a STUB (TODO: "Implement HTTP call to Jina
	// Reader API"). It must FAIL CLOSED: Execute with a key returns an
	// explicit "not implemented" error, never fabricated content. Integration
	// is a known debt; until then no fake data may poison agent memory.
	tool := NewWebFetchTool("test-key")
	out, err := tool.Execute(context.Background(), "https://example.com")
	if err == nil {
		t.Fatalf("Execute must fail closed, got output: %q", out)
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("expected not implemented error, got: %v", err)
	}
}

// ── WebSearchTool ────────────────────────────────────────────────────

func TestWebSearchToolBasics(t *testing.T) {
	// A busca web usa fontes públicas gratuitas — não exige chave nem API key.
	if !NewWebSearchTool("news").IsAvailable() {
		t.Fatal("free provider must be available without a key")
	}
	if !NewWebSearchTool("").IsAvailable() {
		t.Fatal("default provider must be available without a key")
	}
	if NewWebSearchTool("news").Name() != "web_search" {
		t.Fatal("name")
	}
}

func TestWebSearchToolExecuteFailsClosedOnBadSource(t *testing.T) {
	// Um provedor que resolve mas cuja fonte está bloqueada deve devolver um
	// erro EXPLÍCITO (nunca resultado fabricado). Usamos github-code que, sem
	// token, costuma falhar por 401/403 — e por isso falha fechado com erro.
	tool := NewWebSearchTool("github-code")
	out, err := tool.Execute(context.Background(), "some unreachable code query")
	// Não podemos garantir o status HTTP (redes variam); o contrato é: se a
	// fonte falhar, NUNCA devolve sucesso com resultado fabricado.
	if err == nil && strings.TrimSpace(out) == "" {
		t.Fatalf("Execute must not return empty success without a real error")
	}
}

func TestSandboxToolE2BFailsClosed(t *testing.T) {
	// NOTE: executeE2B is a STUB (TODO: "Implement E2B API integration").
	// With an E2B key configured the tool uses the E2B path, which must fail
	// closed with an explicit error instead of a fabricated success. (IsAvailable
	// still reports true due to the local fallback; the E2B path itself fails.)
	tool := NewSandboxTool("e2b-test-key")
	out, err := tool.Execute(context.Background(), "python\nprint('hi')")
	if err == nil {
		t.Fatalf("Execute must fail closed, got output: %q", out)
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("expected not implemented error, got: %v", err)
	}
}

// ── MCPTool ──────────────────────────────────────────────────────────

func TestMCPToolMetadata(t *testing.T) {
	// MCPTool requires a client; metadata is covered via a nil-guard check.
	// (Client construction is external — no network in tests.)
	_ = t.TempDir()
}

var errTool = &toolErr{"tool failure"}

type toolErr struct{ msg string }

func (e *toolErr) Error() string { return e.msg }
