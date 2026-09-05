//
// Docs and additional coverage edge-case tests.
//

package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// stubOpenBrowser troca o openBrowser por um stub e o restaura ao final do teste,
// para que os testes de `cosca docs` NÃO abram janelas reais (browser/explorer).
func stubOpenBrowser(t *testing.T, fn func(string) error) {
	t.Helper()
	prev := openBrowser
	openBrowser = fn
	t.Cleanup(func() { openBrowser = prev })
}

// =============================================================================
// findLocalDocs — additional path (already tested in cli_coverage_test.go)
// =============================================================================

func TestFindLocalDocs_DoesNotPanicWhenNoDocs(t *testing.T) {
	// On test machines, local docs don't exist at the standard paths.
	// This verifies the function returns "" without panicking.
	result := findLocalDocs()
	if result != "" {
		t.Logf("found local docs at: %s", result)
	}
}

// =============================================================================
// openBrowser — coverage for unsupported platform
// =============================================================================

func TestOpenBrowser_EmptyURLErrors(t *testing.T) {
	// defaultOpenBrowser NÃO deve abrir janela com URL vazia (guarda em todas as
	// plataformas). A janela "Este Computador" (rundll32 com URL vazia) era o
	// sintoma — o guarda impede isso.
	if err := defaultOpenBrowser(""); err == nil {
		t.Fatal("defaultOpenBrowser('') deveria falhar (guard de URL vazia)")
	}
}

// =============================================================================
// Docs command — RunE paths (unique tests, not in cli_coverage_test.go)
// =============================================================================

func TestNewDocsCommand_RunE_OnlineMode(t *testing.T) {
	cmd := NewDocsCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	// Stub do openBrowser: NÃO abre janela real; só registra a URL.
	var got []string
	stubOpenBrowser(t, func(u string) error { got = append(got, u); return nil })

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE erro: %v", err)
	}
	if len(got) != 1 || got[0] != "https://cosca.enterprise/docs" {
		t.Fatalf("esperava abrir %q, abriu %v", "https://cosca.enterprise/docs", got)
	}
}

func TestNewDocsCommand_RunE_OfflineMode(t *testing.T) {
	cmd := NewDocsCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	cmd.Flags().Set("offline", "true")

	var got []string
	stubOpenBrowser(t, func(u string) error { got = append(got, u); return nil })

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE erro: %v", err)
	}
	if len(got) != 1 || got[0] == "" {
		t.Fatalf("esperava abrir uma URL de docs, abriu: %v", got)
	}
}

// =============================================================================
// Output Formatter — edge cases for uncovered branches
// =============================================================================

func TestOutputFormatter_PrintYAML_Nil(t *testing.T) {
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatYAML, false, false, false)
	err := f.Print(nil)
	if err != nil {
		t.Logf("Print(nil) with YAML format returned error: %v", err)
	}
}

func TestOutputFormatter_PrintTable_Struct(t *testing.T) {
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatTable, false, false, false)

	type testStruct struct {
		Name string
	}
	err := f.Print(testStruct{Name: "test"})
	if err != nil {
		t.Logf("Print with table format error: %v", err)
	}
}

func TestOutputFormatter_KeyValue_EdgeCases(t *testing.T) {
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
	f.KeyValue("", "value") // empty key
	f.KeyValue("key", "")   // empty value
	_ = buf.String()
}

func TestOutputFormatter_Bullet_EmptyText(t *testing.T) {
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
	f.Bullet("")
}

func TestOutputFormatter_Spinner_StartTwice(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf, "test", false)
	s.Start()
	s.Start() // should not panic
	s.Stop("done")
}

func TestOutputFormatter_Spinner_StopWithoutStart(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf, "test", false)
	s.Stop("done") // should not panic
}

func TestOutputFormatter_Spinner_FailWithoutStart(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf, "test", false)
	s.Fail("failed") // should not panic
}

func TestOutputFormatter_ProgressBar_IncrementOverflow(t *testing.T) {
	var buf bytes.Buffer
	pb := NewProgressBar(&buf, 3, "test", false)
	pb.Increment()
	pb.Increment()
	pb.Increment()
	pb.Increment() // fourth increment past total
	pb.Complete()
}

func TestOutputFormatter_ProgressBar_AddNegative(t *testing.T) {
	var buf bytes.Buffer
	pb := NewProgressBar(&buf, 10, "test", false)
	pb.Add(-1) // negative increment should not break
	pb.Complete()
}

func TestOutputFormatter_ProgressBar_ZeroTotal(t *testing.T) {
	var buf bytes.Buffer
	pb := NewProgressBar(&buf, 0, "zero", false)
	pb.Increment()
	pb.Complete()
}

func TestOutputFormatter_Tree_EmptyItems(t *testing.T) {
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
	f.Tree(nil, "")
	// Should not panic
}

func TestOutputFormatter_Tree_WithChildren(t *testing.T) {
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
	items := []TreeItem{
		{
			Label: "root",
			Children: []TreeItem{
				{Label: "child1"},
				{Label: "child2", Detail: "detail"},
			},
		},
	}
	f.Tree(items, "")
	output := buf.String()
	if !strings.Contains(output, "root") {
		t.Errorf("expected 'root' in tree, got: %q", output)
	}
}

func TestOutputFormatter_Tree_WithPrefix(t *testing.T) {
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
	items := []TreeItem{
		{Label: "item", Detail: "data"},
	}
	f.Tree(items, "custom-prefix")
	output := buf.String()
	if !strings.Contains(output, "custom-prefix") {
		t.Errorf("expected 'custom-prefix' prefix, got: %q", output)
	}
}

// =============================================================================
// printJSON — edge cases
// =============================================================================

func TestPrintJSON_NilInput(t *testing.T) {
	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	err := printJSON(cmd, nil)
	if err != nil {
		t.Fatalf("printJSON(nil) returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "null") {
		t.Errorf("expected 'null', got: %q", buf.String())
	}
}

func TestPrintJSON_EmptyStruct(t *testing.T) {
	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	type Empty struct{}
	err := printJSON(cmd, Empty{})
	if err != nil {
		t.Fatalf("printJSON returned error: %v", err)
	}
}

// =============================================================================
// additional memory snapshot command properties
// =============================================================================

func TestNewMemorySnapshotRestoreCommand_RequiresArg(t *testing.T) {
	cmd := NewMemorySnapshotRestoreCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args (ExactArgs(1))")
	}
}

// =============================================================================
// Run Command — args validation
// =============================================================================

func TestNewRunCommand_NoArgsError(t *testing.T) {
	cmd := NewRunCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args for run command")
	}
}

func TestNewRunCommand_DryRunFlag(t *testing.T) {
	cmd := NewRunCommand()
	dryRunFlag := cmd.Flags().Lookup("dry-run")
	if dryRunFlag == nil {
		t.Error("missing --dry-run flag")
	}
	if dryRunFlag.DefValue != "false" {
		t.Errorf("--dry-run default = %q, want 'false'", dryRunFlag.DefValue)
	}
}

// =============================================================================
// Chat Command — structure checks
// =============================================================================

func TestNewChatCommand_UseField(t *testing.T) {
	cmd := NewChatCommand()
	if cmd.Use != "chat" {
		t.Errorf("Use = %q, want 'chat'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

// =============================================================================
// Metrics Engine
// =============================================================================

func TestSetMetricsEngine_NilDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("SetMetricsEngine(nil) panicked: %v", r)
		}
	}()
	SetMetricsEngine(nil)
}

// =============================================================================
// workflow show command — test flags
// =============================================================================

func TestNewWorkflowShowCommand_RequiresArg(t *testing.T) {
	cmd := NewWorkflowShowCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args for workflow show")
	}
}

// =============================================================================
// provider list command — RunE
// =============================================================================

func TestNewProviderListCommand_RunE(t *testing.T) {
	cmd := NewProviderListCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Logf("RunE returned error: %v", err)
	}
}

func TestNewProviderListCommand_RunE_JSON(t *testing.T) {
	cmd := NewProviderListCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	cmd.PersistentFlags().BoolP("json", "j", false, "json")
	cmd.PersistentFlags().Set("json", "true")

	_ = cmd.RunE(cmd, nil)
}

// =============================================================================
// provider set command — additional path
// =============================================================================

func TestNewProviderSetCommand_NotEnoughArgs(t *testing.T) {
	cmd := NewProviderSetCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"only-one-arg"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with only 1 arg for provider set")
	}
}

// =============================================================================
// Config Set Command — RunE
// =============================================================================

func TestNewConfigSetCommand_RunE(t *testing.T) {
	cmd := NewConfigSetCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	// With correct args
	cmd.SetArgs([]string{"cache.ttl", "3600"})
	err := cmd.Execute()
	// May fail depending on env, but we exercise the code path
	if err != nil {
		t.Logf("RunE with set args returned error: %v", err)
	}
}

// =============================================================================
// Uninstall Command — flags
// =============================================================================

func TestNewUninstallCommand_FlagsExist(t *testing.T) {
	cmd := NewUninstallCommand()
	for _, name := range []string{"dry-run", "keep-config"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("missing --%s flag on uninstall", name)
		}
	}
}

// =============================================================================
// Upgrade Command — additional checks
// =============================================================================

func TestNewUpgradeCommand_Structure(t *testing.T) {
	cmd := NewUpgradeCommand()
	if cmd.Use != "upgrade" {
		t.Errorf("expected Use='upgrade', got '%s'", cmd.Use)
	}
	if cmd.Args == nil {
		t.Error("upgrade command should have Args validator")
	}
}

// =============================================================================
// Update Command — flags
// =============================================================================

func TestNewUpdateCommand_CheckOnlyFlag(t *testing.T) {
	cmd := NewUpdateCommand()
	checkFlag := cmd.Flags().Lookup("check-only")
	if checkFlag == nil {
		t.Error("missing --check-only flag on update")
	}
	if checkFlag.DefValue != "false" {
		t.Errorf("--check-only default = %q, want 'false'", checkFlag.DefValue)
	}
}

// =============================================================================
// Config List — RunE
// =============================================================================

func TestNewConfigListCommand_RunE(t *testing.T) {
	cmd := NewConfigListCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Logf("RunE returned error: %v", err)
	}
	output := buf.String()
	// Should list config keys
	_ = output
}

// =============================================================================
// Config Reset — flags
// =============================================================================

func TestNewConfigResetCommand_RunE_WithoutForce(t *testing.T) {
	cmd := NewConfigResetCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Logf("RunE returned error (expected without force): %v", err)
	}
}

// =============================================================================
// Version command — RunE JSON
// =============================================================================

func TestNewVersionCommand_RunE_JSON(t *testing.T) {
	cmd := NewVersionCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	cmd.PersistentFlags().BoolP("json", "j", false, "json")
	cmd.PersistentFlags().Set("json", "true")

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, `"version"`) {
		t.Errorf("expected JSON output, got: %s", output)
	}
}
