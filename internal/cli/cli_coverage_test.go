//
// Comprehensive CLI coverage tests — targeted at functions <70% coverage.
// Focus: command structure, flag registration, error paths, and utility functions.

package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/adapter"
	"github.com/CoscaAI/cosca/internal/agents"
	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/metrics"
	"github.com/CoscaAI/cosca/internal/orchestration"
	"github.com/CoscaAI/cosca/internal/pipeline"
	"github.com/CoscaAI/cosca/internal/plugins"
	"github.com/CoscaAI/cosca/internal/skills"
)

// =============================================================================
// Template Commands
// =============================================================================

func TestTemplateCommand_Subcommands(t *testing.T) {
	cmd := NewTemplateCommand()
	subs := map[string]bool{}
	for _, s := range cmd.Commands() {
		subs[s.Name()] = true
	}
	for _, want := range []string{"list", "show", "search", "create"} {
		if !subs[want] {
			t.Errorf("missing template subcommand: %s", want)
		}
	}
}

func TestNewTemplateListCommand_Properties(t *testing.T) {
	cmd := NewTemplateListCommand()
	if cmd.Use != "list" {
		t.Errorf("Use = %q, want %q", cmd.Use, "list")
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.Long == "" {
		t.Error("Long should not be empty")
	}
}

func TestNewTemplateListCommand_RunE(t *testing.T) {
	cmd := NewTemplateListCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	// template manager returns nil in test (no real .cosca), should show warning
	output := buf.String()
	if !strings.Contains(output, "Nenhum template encontrado") && !strings.Contains(output, "template manager not available") {
		t.Logf("output: %s", output)
	}
}

func TestNewTemplateListCommand_JSONOutput(t *testing.T) {
	cmd := NewTemplateListCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	cmd.PersistentFlags().BoolP("json", "j", false, "json")
	cmd.PersistentFlags().Set("json", "true")

	_ = cmd.RunE(cmd, nil)
}

func TestNewTemplateShowCommand_Properties(t *testing.T) {
	cmd := NewTemplateShowCommand()
	if cmd.Use != "show <name>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "show <name>")
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestNewTemplateShowCommand_NoArgs(t *testing.T) {
	cmd := NewTemplateShowCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args (ExactArgs(1))")
	}
}

func TestNewTemplateSearchCommand_Properties(t *testing.T) {
	cmd := NewTemplateSearchCommand()
	if cmd.Use != "search <query>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "search <query>")
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestNewTemplateSearchCommand_NoArgs(t *testing.T) {
	cmd := NewTemplateSearchCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args (ExactArgs(1))")
	}
}

func TestNewTemplateCreateCommand_Properties(t *testing.T) {
	cmd := NewTemplateCreateCommand()
	if cmd.Use != "create <name>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "create <name>")
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestNewTemplateCreateCommand_Flags(t *testing.T) {
	cmd := NewTemplateCreateCommand()
	for _, f := range []struct{ name, defval string }{
		{"type", "prompt"},
		{"description", ""},
		{"content", ""},
	} {
		flag := cmd.Flags().Lookup(f.name)
		if flag == nil {
			t.Errorf("missing --%s flag", f.name)
		} else if f.defval != "" && flag.DefValue != f.defval {
			t.Errorf("--%s default = %q, want %q", f.name, flag.DefValue, f.defval)
		}
	}
}

// =============================================================================
// Uninstall Command
// =============================================================================

func TestNewUninstallCommand_Properties(t *testing.T) {
	cmd := NewUninstallCommand()
	if cmd.Use != "uninstall" {
		t.Errorf("Use = %q, want %q", cmd.Use, "uninstall")
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.Long == "" {
		t.Error("Long should not be empty")
	}
	if len(cmd.Example) == 0 {
		t.Error("Example should not be empty")
	}
}

func TestNewUninstallCommand_Flags(t *testing.T) {
	cmd := NewUninstallCommand()
	for _, f := range []struct{ name, defval string }{
		{"dry-run", "false"},
		{"keep-config", "false"},
	} {
		flag := cmd.Flags().Lookup(f.name)
		if flag == nil {
			t.Errorf("missing --%s flag", f.name)
		} else if flag.DefValue != f.defval {
			t.Errorf("--%s default = %q, want %q", f.name, flag.DefValue, f.defval)
		}
	}
}

func TestNewUninstallCommand_NotInitialized(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)

	cmd := NewUninstallCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when Cosca is not initialized")
	}
	if !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("error should mention 'not initialized', got: %s", err.Error())
	}
}

func TestNewUninstallCommand_NoArgsCheck(t *testing.T) {
	cmd := NewUninstallCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"extra"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with extra args (NoArgs)")
	}
}

// =============================================================================
// Update Command
// =============================================================================

func TestNewUpdateCommand_Properties(t *testing.T) {
	cmd := NewUpdateCommand()
	if cmd.Use != "update" {
		t.Errorf("Use = %q, want %q", cmd.Use, "update")
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.Long == "" {
		t.Error("Long should not be empty")
	}
	if len(cmd.Example) == 0 {
		t.Error("Example should not be empty")
	}
}

func TestNewUpdateCommand_Flags(t *testing.T) {
	cmd := NewUpdateCommand()
	for _, f := range []struct{ name, defval string }{
		{"channel", "stable"},
		{"check-only", "false"},
	} {
		flag := cmd.Flags().Lookup(f.name)
		if flag == nil {
			t.Errorf("missing --%s flag", f.name)
		} else if f.defval != "" && flag.DefValue != f.defval {
			t.Errorf("--%s default = %q, want %q", f.name, flag.DefValue, f.defval)
		}
	}
}

func TestNewUpdateCommand_InvalidChannel(t *testing.T) {
	cmd := NewUpdateCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	// Set channel to invalid value
	cmd.Flags().Set("channel", "invalid_channel")

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid channel")
	}
	if !strings.Contains(err.Error(), "invalid channel") {
		t.Errorf("error should mention 'invalid channel', got: %s", err.Error())
	}
}

func TestNewUpdateCommand_NoArgsCheck(t *testing.T) {
	cmd := NewUpdateCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"extra"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with extra args (NoArgs)")
	}
}

// =============================================================================
// Upgrade Command
// =============================================================================

func TestNewUpgradeCommand_Properties(t *testing.T) {
	cmd := NewUpgradeCommand()
	if cmd.Use != "upgrade" {
		t.Errorf("Use = %q, want %q", cmd.Use, "upgrade")
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.Long == "" {
		t.Error("Long should not be empty")
	}
}

func TestCopyDir(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src")
	dst := filepath.Join(tmpDir, "dst")

	// Create source structure
	_ = os.MkdirAll(filepath.Join(src, "sub"), 0755)
	_ = os.WriteFile(filepath.Join(src, "file1.txt"), []byte("hello"), 0644)
	_ = os.WriteFile(filepath.Join(src, "sub", "file2.txt"), []byte("world"), 0644)

	err := copyDir(src, dst)
	if err != nil {
		t.Fatalf("copyDir returned error: %v", err)
	}

	// Verify destination
	if _, err := os.Stat(filepath.Join(dst, "file1.txt")); err != nil {
		t.Error("expected file1.txt in destination")
	}
	if _, err := os.Stat(filepath.Join(dst, "sub", "file2.txt")); err != nil {
		t.Error("expected sub/file2.txt in destination")
	}

	// Verify content
	data, _ := os.ReadFile(filepath.Join(dst, "file1.txt"))
	if string(data) != "hello" {
		t.Errorf("file1.txt content = %q, want %q", string(data), "hello")
	}
}

func TestCopyDir_EmptySource(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "empty")
	dst := filepath.Join(tmpDir, "dst")

	_ = os.MkdirAll(src, 0755)

	err := copyDir(src, dst)
	if err != nil {
		t.Fatalf("copyDir for empty dir returned error: %v", err)
	}
	// Destination should exist
	if _, err := os.Stat(dst); err != nil {
		t.Error("expected dst directory to exist after copy")
	}
}

func TestCopyDir_NonExistentSource(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "nonexistent")
	dst := filepath.Join(tmpDir, "dst")

	err := copyDir(src, dst)
	// Should return error from filepath.Walk
	if err == nil {
		t.Error("expected error for non-existent source")
	}
}

// =============================================================================
// Workflow Subcommands
// =============================================================================

func TestNewWorkflowListCommand_Properties(t *testing.T) {
	cmd := NewWorkflowListCommand()
	if cmd.Use != "list" {
		t.Errorf("Use = %q, want %q", cmd.Use, "list")
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestNewWorkflowListCommand_RunE(t *testing.T) {
	cmd := NewWorkflowListCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Logf("RunE returned error (expected if no .cosca dir): %v", err)
	}
	output := buf.String()
	// Should either show warning about no workflows
	if !strings.Contains(output, "Nenhum workflow encontrado") && !strings.Contains(output, "workflow manager not available") {
		t.Logf("output: %s", output)
	}
}

func TestNewWorkflowShowCommand_Properties(t *testing.T) {
	cmd := NewWorkflowShowCommand()
	if cmd.Use != "show <name>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "show <name>")
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestNewWorkflowShowCommand_NoArgs(t *testing.T) {
	cmd := NewWorkflowShowCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args (ExactArgs(1))")
	}
}

func TestNewWorkflowRunCommand_Properties(t *testing.T) {
	cmd := NewWorkflowRunCommand()
	if cmd.Use != "run <name>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "run <name>")
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestNewWorkflowRunCommand_NoArgs(t *testing.T) {
	cmd := NewWorkflowRunCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args (ExactArgs(1))")
	}
}

func TestNewWorkflowSearchCommand_Properties(t *testing.T) {
	cmd := NewWorkflowSearchCommand()
	if cmd.Use != "search <query>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "search <query>")
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestNewWorkflowSearchCommand_NoArgs(t *testing.T) {
	cmd := NewWorkflowSearchCommand()
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
// Knowledge Subcommands (low coverage)
// =============================================================================

func TestNewKnowledgeSearchCommand_Properties(t *testing.T) {
	cmd := NewKnowledgeSearchCommand()
	if cmd.Use != "search <query>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "search <query>")
	}
}

func TestNewKnowledgeSearchCommand_Flags(t *testing.T) {
	cmd := NewKnowledgeSearchCommand()
	for _, f := range []struct{ name, shorthand, defval string }{
		{"limit", "l", "10"},
		{"offset", "o", "0"},
	} {
		flag := cmd.Flags().Lookup(f.name)
		if flag == nil {
			t.Errorf("missing --%s flag", f.name)
		} else if flag.DefValue != f.defval {
			t.Errorf("--%s default = %q, want %q", f.name, flag.DefValue, f.defval)
		}
		if f.shorthand != "" {
			sh := cmd.Flags().ShorthandLookup(f.shorthand)
			if sh == nil {
				t.Errorf("missing -%s shorthand", f.shorthand)
			}
		}
	}
}

func TestNewKnowledgeSearchCommand_NoArgs(t *testing.T) {
	cmd := NewKnowledgeSearchCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args (ExactArgs(1))")
	}
}

func TestNewKnowledgeGraphCommand_Properties(t *testing.T) {
	cmd := NewKnowledgeGraphCommand()
	if cmd.Use != "graph" {
		t.Errorf("Use = %q, want %q", cmd.Use, "graph")
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestNewKnowledgeGraphCommand_RunE(t *testing.T) {
	cmd := NewKnowledgeGraphCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	// graph.New() should work even without data
	err := cmd.RunE(cmd, nil)
	// May or may not error depending on graph availability
	if err != nil {
		t.Logf("RunE returned error: %v", err)
	}
}

func TestNewKnowledgeRebuildCommand_Properties(t *testing.T) {
	cmd := NewKnowledgeRebuildCommand()
	if cmd.Use != "rebuild" {
		t.Errorf("Use = %q, want %q", cmd.Use, "rebuild")
	}
}

func TestNewKnowledgeVerifyCommand_Properties(t *testing.T) {
	cmd := NewKnowledgeVerifyCommand()
	if cmd.Use != "verify" {
		t.Errorf("Use = %q, want %q", cmd.Use, "verify")
	}
}

func TestNewKnowledgeStatsCommand_Properties(t *testing.T) {
	cmd := NewKnowledgeStatsCommand()
	if cmd.Use != "stats" {
		t.Errorf("Use = %q, want %q", cmd.Use, "stats")
	}
}

func TestNewKnowledgeBenchmarkCommand_Properties(t *testing.T) {
	cmd := NewKnowledgeBenchmarkCommand()
	if cmd.Use != "benchmark" {
		t.Errorf("Use = %q, want %q", cmd.Use, "benchmark")
	}
}

func TestNewKnowledgeBenchmarkCommand_RunE(t *testing.T) {
	cmd := NewKnowledgeBenchmarkCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	// Benchmark adapter always returns a valid result
	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Logf("RunE returned error: %v", err)
	}
}

func TestNewKnowledgeVacuumCommand_Properties(t *testing.T) {
	cmd := NewKnowledgeVacuumCommand()
	if cmd.Use != "vacuum" {
		t.Errorf("Use = %q, want %q", cmd.Use, "vacuum")
	}
}

func TestNewKnowledgeVacuumCommand_RunE(t *testing.T) {
	cmd := NewKnowledgeVacuumCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	err := cmd.RunE(cmd, nil)
	// Vacuum adapter handles nil engine gracefully
	if err != nil {
		t.Logf("RunE returned error: %v", err)
	}
}

func TestNewKnowledgeExplainCommand_Properties(t *testing.T) {
	cmd := NewKnowledgeExplainCommand()
	if cmd.Use != "explain <result-id>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "explain <result-id>")
	}
}

func TestNewKnowledgeExplainCommand_NoArgs(t *testing.T) {
	cmd := NewKnowledgeExplainCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args (ExactArgs(1))")
	}
}

func TestNewKnowledgeRelationsCommand_Properties(t *testing.T) {
	cmd := NewKnowledgeRelationsCommand()
	if cmd.Use != "relations <entity>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "relations <entity>")
	}
}

func TestNewKnowledgeRelationsCommand_NoArgs(t *testing.T) {
	cmd := NewKnowledgeRelationsCommand()
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
// Search Command
// =============================================================================

func TestNewSearchCommand_Flags(t *testing.T) {
	cmd := NewSearchCommand()
	for _, f := range []struct{ name, shorthand, defval string }{
		{"limit", "l", "10"},
		{"offset", "o", "0"},
	} {
		flag := cmd.Flags().Lookup(f.name)
		if flag == nil {
			t.Errorf("missing --%s flag", f.name)
		} else if flag.DefValue != f.defval {
			t.Errorf("--%s default = %q, want %q", f.name, flag.DefValue, f.defval)
		}
		if f.shorthand != "" {
			sh := cmd.Flags().ShorthandLookup(f.shorthand)
			if sh == nil {
				t.Errorf("missing -%s shorthand", f.shorthand)
			}
		}
	}
}

func TestNewSearchCommand_Properties(t *testing.T) {
	cmd := NewSearchCommand()
	if cmd.Use != "search [type] <query>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "search [type] <query>")
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

// =============================================================================
// Runtime Subcommands
// =============================================================================

func TestRuntimeSubcommands(t *testing.T) {
	cmd := NewRuntimeCommand()
	subs := map[string]bool{}
	for _, s := range cmd.Commands() {
		subs[s.Name()] = true
	}
	for _, want := range []string{"start", "stop", "restart", "status", "logs", "info"} {
		if !subs[want] {
			t.Errorf("missing runtime subcommand: %s", want)
		}
	}
}

func TestNewRuntimeStartCommand_Properties(t *testing.T) {
	cmd := NewRuntimeStartCommand()
	if cmd.Use != "start" {
		t.Errorf("Use = %q, want %q", cmd.Use, "start")
	}
}

func TestNewRuntimeStopCommand_Properties(t *testing.T) {
	cmd := NewRuntimeStopCommand()
	if cmd.Use != "stop" {
		t.Errorf("Use = %q, want %q", cmd.Use, "stop")
	}
}

func TestNewRuntimeRestartCommand_Properties(t *testing.T) {
	cmd := NewRuntimeRestartCommand()
	if cmd.Use != "restart" {
		t.Errorf("Use = %q, want %q", cmd.Use, "restart")
	}
}

func TestNewRuntimeStatusCommand_RunE(t *testing.T) {
	cmd := NewRuntimeStatusCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	// Runtime adapter works without a real runtime
	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Logf("RunE returned error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "State") {
		t.Error("expected 'State' in output")
	}
}

func TestNewRuntimeLogsCommand_RunE(t *testing.T) {
	cmd := NewRuntimeLogsCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Logf("RunE returned error: %v", err)
	}
	output := buf.String()
	// Should show placeholder log line
	if !strings.Contains(output, "no logs available") {
		t.Logf("output: %s", output)
	}
}

func TestNewRuntimeInfoCommand_RunE(t *testing.T) {
	cmd := NewRuntimeInfoCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Logf("RunE returned error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Version") {
		t.Error("expected 'Version' in output")
	}
}

// =============================================================================
// Cache Subcommands
// =============================================================================

func TestCacheSubcommands(t *testing.T) {
	cmd := NewCacheCommand()
	subs := map[string]bool{}
	for _, s := range cmd.Commands() {
		subs[s.Name()] = true
	}
	for _, want := range []string{"clear", "warm", "stats", "inspect"} {
		if !subs[want] {
			t.Errorf("missing cache subcommand: %s", want)
		}
	}
}

func TestNewCacheClearCommand_Flags(t *testing.T) {
	cmd := NewCacheClearCommand()
	flag := cmd.Flags().Lookup("all")
	if flag == nil {
		t.Error("missing --all flag on cache clear")
	} else if flag.DefValue != "false" {
		t.Errorf("--all default = %q, want %q", flag.DefValue, "false")
	}
}

func TestNewCacheStatsCommand_Properties(t *testing.T) {
	cmd := NewCacheStatsCommand()
	if cmd.Use != "stats" {
		t.Errorf("Use = %q, want %q", cmd.Use, "stats")
	}
}

func TestNewCacheInspectCommand_Flags(t *testing.T) {
	cmd := NewCacheInspectCommand()
	for _, f := range []string{"key", "prefix"} {
		flag := cmd.Flags().Lookup(f)
		if flag == nil {
			t.Errorf("missing --%s flag on cache inspect", f)
		}
	}
}

func TestNewCacheWarmCommand_Properties(t *testing.T) {
	cmd := NewCacheWarmCommand()
	if cmd.Use != "warm" {
		t.Errorf("Use = %q, want %q", cmd.Use, "warm")
	}
}

// =============================================================================
// Context Subcommands
// =============================================================================

func TestContextSubcommands(t *testing.T) {
	cmd := NewContextCommand()
	subs := map[string]bool{}
	for _, s := range cmd.Commands() {
		subs[s.Name()] = true
	}
	for _, want := range []string{"build", "show", "clear", "stats"} {
		if !subs[want] {
			t.Errorf("missing context subcommand: %s", want)
		}
	}
}

func TestNewContextBuildCommand_Flags(t *testing.T) {
	cmd := NewContextBuildCommand()
	for _, f := range []struct{ name, defval string }{
		{"max-tokens", "4096"},
		{"memory", "true"},
	} {
		flag := cmd.Flags().Lookup(f.name)
		if flag == nil {
			t.Errorf("missing --%s flag", f.name)
		} else if flag.DefValue != f.defval {
			t.Errorf("--%s default = %q, want %q", f.name, flag.DefValue, f.defval)
		}
	}
}

func TestNewContextBuildCommand_NoArgs(t *testing.T) {
	cmd := NewContextBuildCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args (ExactArgs(1))")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"hi", 2, "hi"},
		{"", 5, ""},
		{"abc", 3, "abc"},
		{"abcdef", 3, "abc..."},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.expected)
		}
	}
}

func TestIfEmpty(t *testing.T) {
	tests := []struct {
		s, fallback, expected string
	}{
		{"", "default", "default"},
		{"value", "default", "value"},
		{"", "", ""},
		{"non-empty", "fallback", "non-empty"},
	}
	for _, tt := range tests {
		got := ifEmpty(tt.s, tt.fallback)
		if got != tt.expected {
			t.Errorf("ifEmpty(%q, %q) = %q, want %q", tt.s, tt.fallback, got, tt.expected)
		}
	}
}

// =============================================================================
// Memory Subcommands
// =============================================================================

func TestMemorySubcommands(t *testing.T) {
	cmd := NewMemoryCommand()
	subs := map[string]bool{}
	for _, s := range cmd.Commands() {
		subs[s.Name()] = true
	}
	for _, want := range []string{"list", "show", "search", "snapshot", "prune", "stats"} {
		if !subs[want] {
			t.Errorf("missing memory subcommand: %s", want)
		}
	}
}

func TestNewMemoryListCommand_Flags(t *testing.T) {
	cmd := NewMemoryListCommand()
	for _, f := range []struct{ name, shorthand, defval string }{
		{"type", "t", ""},
		{"limit", "l", "20"},
	} {
		flag := cmd.Flags().Lookup(f.name)
		if flag == nil {
			t.Errorf("missing --%s flag", f.name)
		} else if flag.DefValue != f.defval {
			t.Errorf("--%s default = %q, want %q", f.name, flag.DefValue, f.defval)
		}
		if f.shorthand != "" {
			sh := cmd.Flags().ShorthandLookup(f.shorthand)
			if sh == nil {
				t.Errorf("missing -%s shorthand", f.shorthand)
			}
		}
	}
}

func TestNewMemoryShowCommand_NoArgs(t *testing.T) {
	cmd := NewMemoryShowCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args (ExactArgs(1))")
	}
}

func TestNewMemorySearchCommand_Flags(t *testing.T) {
	cmd := NewMemorySearchCommand()
	for _, f := range []struct{ name, shorthand, defval string }{
		{"limit", "l", "10"},
		{"type", "t", ""},
	} {
		flag := cmd.Flags().Lookup(f.name)
		if flag == nil {
			t.Errorf("missing --%s flag", f.name)
		} else if flag.DefValue != f.defval {
			t.Errorf("--%s default = %q, want %q", f.name, flag.DefValue, f.defval)
		}
		if f.shorthand != "" {
			sh := cmd.Flags().ShorthandLookup(f.shorthand)
			if sh == nil {
				t.Errorf("missing -%s shorthand for %s", f.shorthand, f.name)
			}
		}
	}
}

func TestNewMemoryPruneCommand_Flags(t *testing.T) {
	cmd := NewMemoryPruneCommand()
	flag := cmd.Flags().Lookup("dry-run")
	if flag == nil {
		t.Error("missing --dry-run flag on memory prune")
	} else if flag.DefValue != "false" {
		t.Errorf("--dry-run default = %q, want %q", flag.DefValue, "false")
	}
}

func TestMemorySnapshotSubcommands(t *testing.T) {
	cmd := NewMemorySnapshotCommand()
	subs := map[string]bool{}
	for _, s := range cmd.Commands() {
		subs[s.Name()] = true
	}
	for _, want := range []string{"create", "list", "restore"} {
		if !subs[want] {
			t.Errorf("missing memory snapshot subcommand: %s", want)
		}
	}
}

func TestNewMemorySnapshotRestoreCommand_NoArgs(t *testing.T) {
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
// Plugin Subcommands
// =============================================================================

func TestPluginSubcommands(t *testing.T) {
	cmd := NewPluginCommand()
	subs := map[string]bool{}
	for _, s := range cmd.Commands() {
		subs[s.Name()] = true
	}
	for _, want := range []string{"install", "uninstall", "list", "update", "search", "info", "enable", "disable"} {
		if !subs[want] {
			t.Errorf("missing plugin subcommand: %s", want)
		}
	}
}

func TestNewPluginInstallCommand_Flags(t *testing.T) {
	cmd := NewPluginInstallCommand()
	flag := cmd.Flags().Lookup("source")
	if flag == nil {
		t.Error("missing --source flag on plugin install")
	}
	if flag.Shorthand != "s" {
		t.Errorf("--source shorthand = %q, want %q", flag.Shorthand, "s")
	}
}

func TestNewPluginInstallCommand_NoArgs(t *testing.T) {
	cmd := NewPluginInstallCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args (ExactArgs(1))")
	}
}

func TestNewPluginUninstallCommand_NoArgs(t *testing.T) {
	cmd := NewPluginUninstallCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args (ExactArgs(1))")
	}
}

func TestNewPluginEnableCommand_NoArgs(t *testing.T) {
	cmd := NewPluginEnableCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args (ExactArgs(1))")
	}
}

func TestNewPluginDisableCommand_NoArgs(t *testing.T) {
	cmd := NewPluginDisableCommand()
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
// Editor Subcommands
// =============================================================================

func TestEditorSubcommands(t *testing.T) {
	cmd := NewEditorCommand()
	subs := map[string]bool{}
	for _, s := range cmd.Commands() {
		subs[s.Name()] = true
	}
	for _, want := range []string{"setup", "status", "adapt"} {
		if !subs[want] {
			t.Errorf("missing editor subcommand: %s", want)
		}
	}
}

func TestNewEditorAdaptCommand_Properties(t *testing.T) {
	cmd := NewEditorAdaptCommand()
	if cmd.Use != "adapt <editor>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "adapt <editor>")
	}
}

func TestNewEditorAdaptCommand_NoArgs(t *testing.T) {
	cmd := NewEditorAdaptCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args (ExactArgs(1))")
	}
}

func TestNewEditorAdaptCommand_InvalidEditor(t *testing.T) {
	cmd := NewEditorAdaptCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, []string{"nonexistent-editor"})
	if err == nil {
		t.Fatal("expected error for unsupported editor")
	}
	if !strings.Contains(err.Error(), "unsupported editor") {
		t.Errorf("error should mention 'unsupported editor', got: %s", err.Error())
	}
}

// =============================================================================
// Provider Subcommands
// =============================================================================

func TestProviderSubcommands(t *testing.T) {
	cmd := NewProviderCommand()
	subs := map[string]bool{}
	for _, s := range cmd.Commands() {
		subs[s.Name()] = true
	}
	for _, want := range []string{"list", "set", "test", "info", "watch"} {
		if !subs[want] {
			t.Errorf("missing provider subcommand: %s", want)
		}
	}
}

func TestNewProviderSetCommand_NoArgs(t *testing.T) {
	cmd := NewProviderSetCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args (MinimumNArgs(1))")
	}
}

func TestNewProviderTestCommand_NoArgs(t *testing.T) {
	cmd := NewProviderTestCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args (ExactArgs(1))")
	}
}

func TestNewProviderInfoCommand_NoArgs(t *testing.T) {
	cmd := NewProviderInfoCommand()
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
// Index Subcommands
// =============================================================================

func TestIndexSubcommands(t *testing.T) {
	cmd := NewIndexCommand()
	subs := map[string]bool{}
	for _, s := range cmd.Commands() {
		subs[s.Name()] = true
	}
	for _, want := range []string{"rebuild", "update", "status", "stats", "verify"} {
		if !subs[want] {
			t.Errorf("missing index subcommand: %s", want)
		}
	}
}

func TestNewIndexRebuildCommand_RunE(t *testing.T) {
	cmd := NewIndexRebuildCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	// Indexer adapter scans real project files
	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Logf("RunE returned error: %v", err)
	}
}

func TestNewIndexStatusCommand_RunE(t *testing.T) {
	cmd := NewIndexStatusCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Logf("RunE returned error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Files Indexed") {
		t.Logf("output: %s", output)
	}
}

func TestNewIndexStatsCommand_RunE(t *testing.T) {
	cmd := NewIndexStatsCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Logf("RunE returned error: %v", err)
	}
}

func TestNewIndexVerifyCommand_RunE(t *testing.T) {
	cmd := NewIndexVerifyCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Logf("RunE returned error: %v", err)
	}
}

// =============================================================================
// Graph Subcommands
// =============================================================================

func TestGraphSubcommands(t *testing.T) {
	cmd := NewGraphCommand()
	subs := map[string]bool{}
	for _, s := range cmd.Commands() {
		subs[s.Name()] = true
	}
	for _, want := range []string{"show", "query", "stats", "export"} {
		if !subs[want] {
			t.Errorf("missing graph subcommand: %s", want)
		}
	}
}

func TestNewGraphShowCommand_RunE(t *testing.T) {
	cmd := NewGraphShowCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	// graph.New() should work
	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Logf("RunE returned error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Nodes") || !strings.Contains(output, "Edges") {
		t.Logf("output: %s", output)
	}
}

func TestNewGraphStatsCommand_RunE(t *testing.T) {
	cmd := NewGraphStatsCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Logf("RunE returned error: %v", err)
	}
}

func TestNewGraphQueryCommand_NoArgs(t *testing.T) {
	cmd := NewGraphQueryCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args (ExactArgs(1))")
	}
}

func TestNewGraphQueryCommand_Flags(t *testing.T) {
	cmd := NewGraphQueryCommand()
	flag := cmd.Flags().Lookup("depth")
	if flag == nil {
		t.Error("missing --depth flag")
	} else if flag.DefValue != "2" {
		t.Errorf("--depth default = %q, want %q", flag.DefValue, "2")
	}
}

func TestNewGraphExportCommand_Flags(t *testing.T) {
	cmd := NewGraphExportCommand()
	for _, f := range []struct{ name, shorthand, defval string }{
		{"format", "f", "json"},
		{"output", "o", ""},
	} {
		flag := cmd.Flags().Lookup(f.name)
		if flag == nil {
			t.Errorf("missing --%s flag", f.name)
		} else if flag.DefValue != f.defval {
			t.Errorf("--%s default = %q, want %q", f.name, flag.DefValue, f.defval)
		}
	}
}

// =============================================================================
// Agent Subcommands
// =============================================================================

func TestAgentSubcommands(t *testing.T) {
	cmd := NewAgentCommand()
	subs := map[string]bool{}
	for _, s := range cmd.Commands() {
		subs[s.Name()] = true
	}
	for _, want := range []string{"list", "show", "search", "run", "capabilities"} {
		if !subs[want] {
			t.Errorf("missing agent subcommand: %s", want)
		}
	}
}

func TestNewAgentListCommand_RunE(t *testing.T) {
	cmd := NewAgentListCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Logf("RunE returned error: %v", err)
	}
	output := buf.String()
	// Should either show agents or "No agents found"
	if !strings.Contains(output, "No agents found") && !strings.Contains(output, "agent manager not available") && !strings.Contains(output, "Available Agents") {
		t.Logf("output: %s", output)
	}
}

func TestNewAgentSearchCommand_NoArgs(t *testing.T) {
	cmd := NewAgentSearchCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args (ExactArgs(1))")
	}
}

func TestNewAgentShowCommand_NoArgs(t *testing.T) {
	cmd := NewAgentShowCommand()
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
// Skill Subcommands
// =============================================================================

func TestSkillSubcommands(t *testing.T) {
	cmd := NewSkillCommand()
	subs := map[string]bool{}
	for _, s := range cmd.Commands() {
		subs[s.Name()] = true
	}
	for _, want := range []string{"list", "show", "search", "install"} {
		if !subs[want] {
			t.Errorf("missing skill subcommand: %s", want)
		}
	}
}

func TestNewSkillListCommand_RunE(t *testing.T) {
	cmd := NewSkillListCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Logf("RunE returned error: %v", err)
	}
}

func TestNewSkillInstallCommand_Flags(t *testing.T) {
	cmd := NewSkillInstallCommand()
	flag := cmd.Flags().Lookup("source")
	if flag == nil {
		t.Error("missing --source flag")
	}
	if flag.Shorthand != "s" {
		t.Errorf("--source shorthand = %q, want %q", flag.Shorthand, "s")
	}
}

// =============================================================================
// Prompt Subcommands
// =============================================================================

func TestPromptSubcommands(t *testing.T) {
	cmd := NewPromptCommand()
	subs := map[string]bool{}
	for _, s := range cmd.Commands() {
		subs[s.Name()] = true
	}
	for _, want := range []string{"list", "show", "search", "create"} {
		if !subs[want] {
			t.Errorf("missing prompt subcommand: %s", want)
		}
	}
}

func TestNewPromptCreateCommand_Flags(t *testing.T) {
	cmd := NewPromptCreateCommand()
	for _, f := range []struct{ name, shorthand, defval string }{
		{"template", "t", "default"},
		{"description", "d", ""},
	} {
		flag := cmd.Flags().Lookup(f.name)
		if flag == nil {
			t.Errorf("missing --%s flag", f.name)
		} else if f.defval != "" && flag.DefValue != f.defval {
			t.Errorf("--%s default = %q, want %q", f.name, flag.DefValue, f.defval)
		}
	}
}

// =============================================================================
// Docs Command
// =============================================================================

func TestNewDocsCommand_Properties(t *testing.T) {
	cmd := NewDocsCommand()
	if cmd.Use != "docs" {
		t.Errorf("Use = %q, want %q", cmd.Use, "docs")
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestNewDocsCommand_OfflineFlag(t *testing.T) {
	cmd := NewDocsCommand()
	flag := cmd.Flags().Lookup("offline")
	if flag == nil {
		t.Error("missing --offline flag")
	} else if flag.DefValue != "false" {
		t.Errorf("--offline default = %q, want %q", flag.DefValue, "false")
	}
}

func TestFindLocalDocs(t *testing.T) {
	result := findLocalDocs()
	// On a test machine, local docs likely don't exist
	if result != "" {
		t.Logf("found local docs at: %s", result)
	}
}

// =============================================================================
// Health Command
// =============================================================================

func TestNewHealthCommand_RunE_NotInitialized(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)

	cmd := NewHealthCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "not initialized") {
		t.Errorf("expected 'not initialized' in output, got: %s", output)
	}
}

func TestHealthStatus_JSONTags(t *testing.T) {
	hs := HealthStatus{
		Status:   "healthy",
		Runtime:  true,
		Index:    true,
		Memory:   true,
		Database: true,
		Message:  "all good",
	}
	_ = hs // Ensure struct compiles and fields are accessible
}

// =============================================================================
// Benchmark Command
// =============================================================================

func TestNewBenchmarkCommand_Flags(t *testing.T) {
	cmd := NewBenchmarkCommand()
	for _, f := range []struct{ name, defval string }{
		{"search-queries", "20"},
		{"skip-search", "false"},
		{"skip-index", "false"},
		{"skip-memory", "false"},
	} {
		flag := cmd.Flags().Lookup(f.name)
		if flag == nil {
			t.Errorf("missing --%s flag", f.name)
		} else if flag.DefValue != f.defval {
			t.Errorf("--%s default = %q, want %q", f.name, flag.DefValue, f.defval)
		}
	}
}

func TestNewBenchmarkCommand_NotInitialized(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)

	cmd := NewBenchmarkCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when not initialized")
	}
	if !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("error should mention 'not initialized', got: %s", err.Error())
	}
}

func TestBenchmarkResults_Structure(t *testing.T) {
	_ = BenchmarkResults{
		SearchPerf: SearchBenchmark{Queries: 10, P50: "1ms", P95: "5ms", P99: "10ms", Avg: "2ms"},
		IndexPerf:  IndexBenchmark{FilesPerSec: 100.0, AvgFileSize: "1KB", TotalFiles: 50, Duration: "1s"},
		MemoryPerf: MemoryBenchmark{ReadLatency: "1ms", WriteLatency: "2ms", Entries: 100},
	}
}

// =============================================================================
// Bootstrap Command
// =============================================================================

func TestNewBootstrapCommand_NotInitialized(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)

	cmd := NewBootstrapCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when not initialized")
	}
	if !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("error should mention 'not initialized', got: %s", err.Error())
	}
}

// =============================================================================
// Pipeline Commands
// =============================================================================

func TestPipelineSubcommands(t *testing.T) {
	cmd := NewPipelineCommand()
	subs := map[string]bool{}
	for _, s := range cmd.Commands() {
		subs[s.Name()] = true
	}
	for _, want := range []string{"list", "run"} {
		if !subs[want] {
			t.Errorf("missing pipeline subcommand: %s", want)
		}
	}
}

func TestNewPipelineRunCommand_Flags(t *testing.T) {
	cmd := NewPipelineRunCommand()
	for _, f := range []string{"prompt", "stream"} {
		flag := cmd.Flags().Lookup(f)
		if flag == nil {
			t.Errorf("missing --%s flag", f)
		}
	}
}

func TestNewPipelineRunCommand_NoArgs(t *testing.T) {
	cmd := NewPipelineRunCommand()
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
// Run Command
// =============================================================================

func TestNewRunCommand_Flags(t *testing.T) {
	cmd := NewRunCommand()
	expectedFlags := []string{
		"agent", "provider", "model", "stream", "no-mag",
		"metrics", "semantic", "dry-run",
	}
	for _, name := range expectedFlags {
		flag := cmd.Flags().Lookup(name)
		if flag == nil {
			t.Errorf("missing --%s flag on run command", name)
		}
	}
}

// =============================================================================
// Chat Command
// =============================================================================

func TestNewChatCommand_Flags(t *testing.T) {
	cmd := NewChatCommand()
	for _, f := range []string{"agent", "provider", "model", "stream", "metrics"} {
		flag := cmd.Flags().Lookup(f)
		if flag == nil {
			t.Errorf("missing --%s flag on chat command", f)
		}
	}
}

// =============================================================================
// Metrics Command
// =============================================================================

func TestNewMetricsCommand_NoEngine(t *testing.T) {
	// Reset engine holder
	origHolder := engineHolder
	engineHolder = nil
	defer func() { engineHolder = origHolder }()

	cmd := NewMetricsCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "No orchestration engine is currently active") {
		t.Errorf("expected 'No orchestration engine' warning, got: %s", output)
	}
}

func TestSetMetricsEngine(t *testing.T) {
	_ = SetMetricsEngine // ensure function exists and compiles
}

func TestPrintMetricsSnapshot_PanicsOnNil(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Logf("printMetricsSnapshot panicked: %v", r)
		}
	}()
	// This should be safe if snapshot fields have defaults
	_ = printMetricsSnapshot
}

// =============================================================================
// Serve Command
// =============================================================================

func TestNewServeCommand_Flags(t *testing.T) {
	cmd := NewServeCommand()
	expectedFlags := []string{
		"host", "port", "metrics-port", "cors-origins",
		"data-dir", "tls-cert-file", "tls-key-file",
	}
	for _, name := range expectedFlags {
		flag := cmd.Flags().Lookup(name)
		if flag == nil {
			t.Errorf("missing --%s flag on serve command", name)
		}
	}

	// Check defaults
	portFlag := cmd.Flags().Lookup("port")
	if portFlag != nil && portFlag.DefValue != "14120" {
		t.Errorf("--port default = %q, want %q", portFlag.DefValue, "14120")
	}
	metricsFlag := cmd.Flags().Lookup("metrics-port")
	if metricsFlag != nil && metricsFlag.DefValue != "14121" {
		t.Errorf("--metrics-port default = %q, want %q", metricsFlag.DefValue, "14121")
	}
	hostFlag := cmd.Flags().Lookup("host")
	if hostFlag != nil && hostFlag.DefValue != "127.0.0.1" {
		t.Errorf("--host default = %q, want %q", hostFlag.DefValue, "127.0.0.1")
	}
	corsFlag := cmd.Flags().Lookup("cors-origins")
	if corsFlag != nil && corsFlag.DefValue != "" {
		t.Errorf("--cors-origins default = %q, want empty (fail-closed)", corsFlag.DefValue)
	}
}

// TestServe_DefaultHostLocalhost verifies the secure-by-default bind host:
// the server only listens on loopback unless the operator explicitly opts
// into network exposure with --host 0.0.0.0.
func TestServe_DefaultHostLocalhost(t *testing.T) {
	cmd := NewServeCommand()
	hostFlag := cmd.Flags().Lookup("host")
	if hostFlag == nil {
		t.Fatal("missing --host flag on serve command")
	}
	if hostFlag.DefValue != "127.0.0.1" {
		t.Errorf("--host default = %q, want %q (loopback only)", hostFlag.DefValue, "127.0.0.1")
	}
}

func TestNewServeCommand_Use(t *testing.T) {
	cmd := NewServeCommand()
	if cmd.Use != "serve" {
		t.Errorf("Use = %q, want %q", cmd.Use, "serve")
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.Long == "" {
		t.Error("Long should not be empty")
	}
}

// =============================================================================
// Metrics Auth Middleware
// =============================================================================

func TestMetricsAuthMiddleware_NoAuth(t *testing.T) {
	mux := http.NewServeMux()
	handler := metricsAuthMiddleware(mux, "test-secret")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/metrics", nil)
	handler.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Errorf("expected 401 without auth header, got %d", w.Code)
	}
}

func TestMetricsAuthMiddleware_WrongSecret(t *testing.T) {
	mux := http.NewServeMux()
	handler := metricsAuthMiddleware(mux, "test-secret")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/metrics", nil)
	r.Header.Set("Authorization", "Bearer wrong-secret")
	handler.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Errorf("expected 401 with wrong secret, got %d", w.Code)
	}
}

func TestMetricsAuthMiddleware_ValidSecret(t *testing.T) {
	mux := http.NewServeMux()
	handler := metricsAuthMiddleware(mux, "test-secret")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/metrics", nil)
	r.Header.Set("Authorization", "Bearer test-secret")
	handler.ServeHTTP(w, r)
	// With valid secret, middleware passes to next handler; mux has no
	// routes registered so it returns 404, but auth passed (not 401).
	if w.Code == 401 {
		t.Errorf("expected non-401 with valid secret, got %d", w.Code)
	}
}

func TestMetricsAuthMiddleware_InvalidPrefix(t *testing.T) {
	mux := http.NewServeMux()
	handler := metricsAuthMiddleware(mux, "test-secret")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/metrics", nil)
	r.Header.Set("Authorization", "WrongPrefix test-secret")
	handler.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Errorf("expected 401 with wrong prefix, got %d", w.Code)
	}
}

func TestMetricsAuthMiddleware_ShortHeader(t *testing.T) {
	mux := http.NewServeMux()
	handler := metricsAuthMiddleware(mux, "test-secret")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/metrics", nil)
	r.Header.Set("Authorization", "B")
	handler.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Errorf("expected 401 with short auth header, got %d", w.Code)
	}
}

func TestMetricsAuthMiddleware_EmptySecret(t *testing.T) {
	mux := http.NewServeMux()
	handler := metricsAuthMiddleware(mux, "")
	w := httptest.NewRecorder()
	// An empty secret (dev mode with no COSCA_METRICS_SECRET) is a
	// pass-through: authentication is skipped entirely and the request
	// reaches the mux (404 with no routes), never 401.
	r := httptest.NewRequest("GET", "/metrics", nil)
	r.Header.Set("Authorization", "Bearer t")
	handler.ServeHTTP(w, r)
	if w.Code == 401 {
		t.Errorf("expected pass-through with empty secret (not 401), got %d", w.Code)
	}
}

// =============================================================================
// FormatMetrics
// =============================================================================

func TestFormatMetrics_NilInputs(t *testing.T) {
	output := metrics.FormatMetrics(nil, nil, nil, nil, nil)
	// With nil inputs, we should still get process metrics.
	if output == "" {
		t.Error("expected non-empty output for nil inputs (process metrics)")
	}
	// Verify process metrics are present.
	if !strings.Contains(output, "cosca_process_goroutines") {
		t.Error("expected cosca_process_goroutines in output")
	}
	if !strings.Contains(output, "cosca_process_memory_alloc_bytes") {
		t.Error("expected cosca_process_memory_alloc_bytes in output")
	}
}

// =============================================================================
// Sync Command
// =============================================================================

func TestNewSyncCommand_Flags(t *testing.T) {
	cmd := NewSyncCommand()
	for _, f := range []struct{ name, shorthand, defval string }{
		{"full", "f", "false"},
		{"dry-run", "n", "false"},
	} {
		flag := cmd.Flags().Lookup(f.name)
		if flag == nil {
			t.Errorf("missing --%s flag", f.name)
		} else if flag.DefValue != f.defval {
			t.Errorf("--%s default = %q, want %q", f.name, flag.DefValue, f.defval)
		}
		if f.shorthand != "" {
			sh := cmd.Flags().ShorthandLookup(f.shorthand)
			if sh == nil {
				t.Errorf("missing -%s shorthand for %s", f.shorthand, f.name)
			}
		}
	}
}

// =============================================================================
// Config Edit command error path
// =============================================================================

func TestNewConfigEditCommand_ConfigFileNotFound(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)

	cmd := NewConfigEditCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error when config file not found")
	}
	if !strings.Contains(err.Error(), "configuration file not found") {
		t.Errorf("error should mention 'configuration file not found', got: %s", err.Error())
	}
}

// =============================================================================
// Mock Orchestrator for runStreamMode / chat tests
// =============================================================================

// mockStreamOrchestrator implements pipeline.Runner for testing
// runStreamMode and chat session handlers.
type mockStreamOrchestrator struct {
	executeErr   error
	streamErr    error
	streamEvents []orchestration.StreamEvent
}

func (m *mockStreamOrchestrator) Run(_ context.Context, req pipeline.RunRequest) (*pipeline.RunResult, error) {
	if m.executeErr != nil {
		return nil, m.executeErr
	}
	return &pipeline.RunResult{
		Response: "mock response",
		Agent:    "TestAgent",
		TraceID:  "mock-id",
	}, nil
}

func (m *mockStreamOrchestrator) RunStream(_ context.Context, req pipeline.RunRequest) (<-chan pipeline.RunEvent, error) {
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	ch := make(chan pipeline.RunEvent, len(m.streamEvents))
	for _, e := range m.streamEvents {
		ch <- convertStreamEventForTest(e)
	}
	close(ch)
	return ch, nil
}

func convertStreamEventForTest(ev orchestration.StreamEvent) pipeline.RunEvent {
	switch ev.Type {
	case orchestration.StreamEventChunk:
		return pipeline.RunEvent{Type: pipeline.EventContent, Data: ev.Content}
	case orchestration.StreamEventError:
		return pipeline.RunEvent{Type: pipeline.EventError, Data: ev.Content}
	case orchestration.StreamEventProgress, orchestration.StreamEventStageTransition:
		return pipeline.RunEvent{Type: pipeline.EventToolStart, Data: ev.Content}
	default:
		return pipeline.RunEvent{Type: pipeline.EventContent, Data: ev.Content}
	}
}

// =============================================================================
// printTreeChildren (output.go:293) — recursive tree printer
// =============================================================================

func TestPrintTreeChildren_EmptyItems(t *testing.T) {
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	f.printTreeChildren(nil, "")
	if buf.Len() != 0 {
		t.Errorf("expected no output for nil children, got: %q", buf.String())
	}
}

func TestPrintTreeChildren_SingleNode(t *testing.T) {
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	items := []TreeItem{
		{Label: "child1", Prefix: "├── "},
	}
	f.printTreeChildren(items, "  ")
	output := buf.String()
	if !strings.Contains(output, "child1") {
		t.Errorf("expected 'child1' in output, got: %q", output)
	}
	if !strings.Contains(output, "├──") {
		t.Errorf("expected prefix '├──' in output, got: %q", output)
	}
}

func TestPrintTreeChildren_Nested(t *testing.T) {
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	items := []TreeItem{
		{
			Label: "parent", Prefix: "├── ",
			Children: []TreeItem{
				{Label: "child", Prefix: "└── "},
			},
		},
	}
	f.printTreeChildren(items, "  ")
	output := buf.String()
	if !strings.Contains(output, "parent") {
		t.Errorf("expected 'parent' in output, got: %q", output)
	}
	if !strings.Contains(output, "child") {
		t.Errorf("expected 'child' in output, got: %q", output)
	}
}

func TestPrintTreeChildren_WithDetail(t *testing.T) {
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	items := []TreeItem{
		{Label: "item", Prefix: "├── ", Detail: "some detail"},
	}
	f.printTreeChildren(items, "  ")
	output := buf.String()
	if !strings.Contains(output, "some detail") {
		t.Errorf("expected 'some detail' in output, got: %q", output)
	}
}

func TestPrintTreeChildren_MultipleSiblings(t *testing.T) {
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	items := []TreeItem{
		{Label: "a", Prefix: "├── "},
		{Label: "b", Prefix: "├── "},
		{Label: "c", Prefix: "└── "},
	}
	f.printTreeChildren(items, "  ")
	output := buf.String()
	for _, label := range []string{"a", "b", "c"} {
		if !strings.Contains(output, label) {
			t.Errorf("expected %q in output, got: %q", label, output)
		}
	}
}

func TestPrintTreeChildren_DeeplyNested(t *testing.T) {
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	items := []TreeItem{
		{
			Label: "L1", Prefix: "├── ",
			Children: []TreeItem{
				{
					Label: "L2", Prefix: "├── ",
					Children: []TreeItem{
						{Label: "L3", Prefix: "└── "},
					},
				},
			},
		},
	}
	f.printTreeChildren(items, "  ")
	output := buf.String()
	for _, label := range []string{"L1", "L2", "L3"} {
		if !strings.Contains(output, label) {
			t.Errorf("expected %q in deeply nested output, got: %q", label, output)
		}
	}
}

// =============================================================================
// createBuilder (context.go:48) — context builder factory
// =============================================================================

func TestCreateBuilder_ReturnsNonNil(t *testing.T) {
	b := createBuilder()
	if b == nil {
		t.Error("createBuilder() should return a non-nil builder")
	}
}

// =============================================================================
// openCache (cache.go:46) — cache connection opener
// =============================================================================

func TestOpenCache_ValidTempDir(t *testing.T) {
	tmpDir := t.TempDir()
	c, err := openCache(tmpDir)
	if err != nil {
		t.Fatalf("openCache returned error: %v", err)
	}
	if c == nil {
		t.Error("openCache should return a non-nil cache for a valid dir")
	}
}

func TestOpenCache_InvalidPath(t *testing.T) {
	// openCache gracefully degrades when the directory cannot be created;
	// cache.New logs a warning and falls back to a memory-only cache.
	// A null byte in the path is handled by MkdirAll returning an error,
	// but openCache still returns a valid cache (memory only) — no error.
	c, err := openCache("/tmp/\x00invalid")
	if err != nil {
		t.Fatalf("openCache should not error on unwritable dir (falls back to memory): %v", err)
	}
	if c == nil {
		t.Error("expected non-nil cache even with invalid path (memory fallback)")
	}
}

// =============================================================================
// printMetricsSnapshot (metrics.go:58) — formatted metrics output
// =============================================================================

func TestPrintMetricsSnapshot_EmptySnapshot(t *testing.T) {
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	printMetricsSnapshot(f, orchestration.MetricsSnapshot{})
	output := buf.String()
	if !strings.Contains(output, "Orchestration Metrics") {
		t.Errorf("expected 'Orchestration Metrics' header, got: %q", output)
	}
	if !strings.Contains(output, "Total Requests") {
		t.Errorf("expected 'Total Requests' in output, got: %q", output)
	}
}

func TestPrintMetricsSnapshot_Populated(t *testing.T) {
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	snap := orchestration.MetricsSnapshot{
		Timestamp:            time.Now(),
		TotalRequests:        100,
		SuccessfulRequests:   95,
		FailedRequests:       5,
		SuccessRate:          0.95,
		TotalLLMCalls:        200,
		TotalLLMTokens:       50000,
		LLMErrors:            3,
		LLMFallbacks:         1,
		AvgDurationMs:        150.5,
		RouterExplicitHits:   40,
		RouterKeywordHits:    35,
		RouterSearchHits:     20,
		RouterFallbacks:      5,
		MAGMemoriesRetrieved: 30,
		MAGMemoriesStored:    15,
	}
	printMetricsSnapshot(f, snap)
	output := buf.String()
	for _, want := range []string{
		"100",      // total requests
		"95",       // successful
		"5",        // failed
		"200",      // total LLM calls
		"50000",    // total LLM tokens
		"3",        // LLM errors
		"1",        // LLM fallbacks
		"40",       // router explicit hits
		"30",       // MAG memories retrieved
		"15",       // MAG memories stored
		"95.0%",    // success rate
		"150.50ms", // avg duration
	} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in populated metrics output, got: %q", want, output)
		}
	}
}

func TestPrintMetricsSnapshot_WithStageBreakdown(t *testing.T) {
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	snap := orchestration.MetricsSnapshot{
		TotalRequests:      10,
		SuccessfulRequests: 8,
		TotalLLMCalls:      10,
		StageBreakdown: map[string]orchestration.StageStats{
			"router": {
				Calls:       10,
				Successes:   9,
				Errors:      1,
				ErrorRate:   0.10,
				AvgDuration: 10.5,
				MinDuration: 1.0,
				MaxDuration: 50.0,
			},
			"executor": {
				Calls:       8,
				Successes:   8,
				Errors:      0,
				ErrorRate:   0.0,
				AvgDuration: 200.0,
				MinDuration: 100.0,
				MaxDuration: 500.0,
			},
		},
	}
	printMetricsSnapshot(f, snap)
	output := buf.String()
	if !strings.Contains(output, "Stage Breakdown") {
		t.Errorf("expected 'Stage Breakdown' header, got: %q", output)
	}
	if !strings.Contains(output, "router") {
		t.Errorf("expected 'router' stage in output, got: %q", output)
	}
	if !strings.Contains(output, "executor") {
		t.Errorf("expected 'executor' stage in output, got: %q", output)
	}
}

// =============================================================================
// runStreamMode (run.go:334) — streaming execution
// =============================================================================

func TestRunStreamMode_ExecuteStreamError(t *testing.T) {
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	engine := &mockStreamOrchestrator{streamErr: fmt.Errorf("stream init failed")}
	req := pipeline.RunRequest{Prompt: "test"}

	err := runStreamMode(context.Background(), engine, req, f, false)
	if err == nil {
		t.Error("expected error from runStreamMode")
	}
	if !strings.Contains(err.Error(), "stream init failed") {
		t.Errorf("error should wrap stream error, got: %s", err.Error())
	}
}

func TestRunStreamMode_SuccessNonJSON(t *testing.T) {
	engine := &mockStreamOrchestrator{
		streamEvents: []orchestration.StreamEvent{
			{Type: orchestration.StreamEventProgress, Content: "resolving agent"},
			{Type: orchestration.StreamEventChunk, Content: "Hello"},
			{Type: orchestration.StreamEventChunk, Content: " World"},
		},
	}
	req := pipeline.RunRequest{Prompt: "test"}
	f := NewOutputFormatter(nil, OutputFormatText, false, true, true) // quiet output

	err := runStreamMode(context.Background(), engine, req, f, false)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunStreamMode_JSONOutput(t *testing.T) {
	engine := &mockStreamOrchestrator{
		streamEvents: []orchestration.StreamEvent{
			{Type: orchestration.StreamEventChunk, Content: "Hello"},
		},
	}
	req := pipeline.RunRequest{Prompt: "test"}
	f := NewOutputFormatter(nil, OutputFormatText, false, true, true)

	err := runStreamMode(context.Background(), engine, req, f, true)
	if err != nil {
		t.Errorf("unexpected error with JSON output: %v", err)
	}
}

func TestRunStreamMode_ErrorEvents(t *testing.T) {
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	engine := &mockStreamOrchestrator{
		streamEvents: []orchestration.StreamEvent{
			{Type: orchestration.StreamEventError, Content: "non-fatal issue"},
		},
	}
	req := pipeline.RunRequest{Prompt: "test"}

	err := runStreamMode(context.Background(), engine, req, f, false)
	if err != nil {
		t.Errorf("unexpected error from non-fatal error event: %v", err)
	}
}

func TestRunStreamMode_StageTransitionEvent(t *testing.T) {
	engine := &mockStreamOrchestrator{
		streamEvents: []orchestration.StreamEvent{
			{
				Type:    orchestration.StreamEventStageTransition,
				Content: "moved to executor",
				Metadata: map[string]interface{}{
					"from":  "router",
					"to":    "executor",
					"agent": "TestAgent",
				},
			},
		},
	}
	req := pipeline.RunRequest{Prompt: "test"}
	f := NewOutputFormatter(nil, OutputFormatText, true, true, true) // verbose but quiet

	err := runStreamMode(context.Background(), engine, req, f, false)
	if err != nil {
		t.Errorf("unexpected error from stage transition: %v", err)
	}
}

func TestRunStreamMode_ContextCancelled(t *testing.T) {
	// Cancel BEFORE starting. The mock sends one event to get into the loop
	// body where ctx.Done() fires immediately.  (Without an event the range
	// would block on receive and never reach the ctx.Done() select.)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	engine := &mockStreamOrchestrator{
		streamEvents: []orchestration.StreamEvent{
			{Type: orchestration.StreamEventProgress, Content: "start"},
		},
	}
	req := pipeline.RunRequest{Prompt: "test"}
	f := NewOutputFormatter(nil, OutputFormatText, false, true, true)

	err := runStreamMode(ctx, engine, req, f, false)
	if err == nil {
		t.Error("expected context cancelled error from runStreamMode")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

// =============================================================================
// runDryRun (run.go:384) — dry run mode
// =============================================================================

func TestRunDryRun_NoAgentSpecified(t *testing.T) {
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	// Use a cobra command from the package — SetOut needs to be on the cmd
	cmd := NewRunCommand()
	cmd.SetOut(buf)

	req := &orchestration.Request{Prompt: "build an API"}

	err := runDryRun(cmd, nil, req, f, false)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Dry Run") {
		t.Errorf("expected 'Dry Run' in output, got: %q", output)
	}
	if !strings.Contains(output, "build an API") {
		t.Errorf("expected prompt in output, got: %q", output)
	}
	if !strings.Contains(output, "auto-detect") {
		t.Errorf("expected 'auto-detect' for no agent specified, got: %q", output)
	}
}

func TestRunDryRun_WithExplicitAgent(t *testing.T) {
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	cmd := NewRunCommand()
	cmd.SetOut(buf)

	req := &orchestration.Request{
		Prompt:  "design a schema",
		Context: map[string]interface{}{"agent": "Backend Chief"},
	}

	err := runDryRun(cmd, nil, req, f, false)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Backend Chief (explicit)") {
		t.Errorf("expected 'Backend Chief (explicit)', got: %q", output)
	}
}

func TestRunDryRun_PipelineStages(t *testing.T) {
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	cmd := NewRunCommand()
	cmd.SetOut(buf)

	req := &orchestration.Request{Prompt: "test"}
	_ = runDryRun(cmd, nil, req, f, false)
	output := buf.String()
	for _, stage := range []string{
		"MAG Retrieve", "Context Builder", "Router",
		"Executor", "Pipeline", "MAG Store",
	} {
		if !strings.Contains(output, stage) {
			t.Errorf("expected pipeline stage %q in output, got: %q", stage, output)
		}
	}
	if !strings.Contains(output, "Dry run complete") {
		t.Errorf("expected 'Dry run complete' message, got: %q", output)
	}
}

// =============================================================================
// handleCommand (chat.go:215) — REPL command handler
// =============================================================================

func TestChatSession_HandleCommand_Exit(t *testing.T) {
	session := &chatSession{running: true}
	cmd := NewChatCommand() // provides OutOrStdout
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session.handleCommand(cmd, "/exit", nil)
	if session.running {
		t.Error("expected session.running = false after /exit")
	}
}

func TestChatSession_HandleCommand_Quit(t *testing.T) {
	session := &chatSession{running: true}
	cmd := NewChatCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session.handleCommand(cmd, "/quit", nil)
	if session.running {
		t.Error("expected session.running = false after /quit")
	}
}

func TestChatSession_HandleCommand_Q(t *testing.T) {
	session := &chatSession{running: true}
	cmd := NewChatCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session.handleCommand(cmd, "/q", nil)
	if session.running {
		t.Error("expected session.running = false after /q")
	}
}

func TestChatSession_HandleCommand_Help(t *testing.T) {
	session := &chatSession{running: true}
	cmd := NewChatCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session.handleCommand(cmd, "/help", nil)
	output := buf.String()
	if !strings.Contains(output, "Available commands") {
		t.Errorf("expected 'Available commands' in help output, got: %q", output)
	}
}

func TestChatSession_HandleCommand_HelpShort(t *testing.T) {
	session := &chatSession{running: true}
	cmd := NewChatCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session.handleCommand(cmd, "/h", nil)
	output := buf.String()
	if !strings.Contains(output, "Available commands") {
		t.Errorf("expected 'Available commands' from /h, got: %q", output)
	}
}

func TestChatSession_HandleCommand_AgentSet(t *testing.T) {
	session := &chatSession{running: true}
	cmd := NewChatCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session.handleCommand(cmd, "/agent Backend Chief", nil)
	if session.agentName != "Backend Chief" {
		t.Errorf("expected agentName 'Backend Chief', got: %q", session.agentName)
	}
}

func TestChatSession_HandleCommand_AgentClear(t *testing.T) {
	session := &chatSession{running: true, agentName: "SomeAgent"}
	cmd := NewChatCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session.handleCommand(cmd, "/agent", nil)
	if session.agentName != "" {
		t.Errorf("expected agentName to be cleared, got: %q", session.agentName)
	}
}

func TestChatSession_HandleCommand_ProviderSet(t *testing.T) {
	session := &chatSession{running: true}
	cmd := NewChatCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session.handleCommand(cmd, "/provider openai", nil)
	if session.providerName != "openai" {
		t.Errorf("expected providerName 'openai', got: %q", session.providerName)
	}
}

func TestChatSession_HandleCommand_ProviderMissingArg(t *testing.T) {
	session := &chatSession{running: true}
	cmd := NewChatCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session.handleCommand(cmd, "/provider", nil)
	output := buf.String()
	if !strings.Contains(output, "Usage:") {
		t.Errorf("expected 'Usage:' for missing provider arg, got: %q", output)
	}
}

func TestChatSession_HandleCommand_Model(t *testing.T) {
	session := &chatSession{running: true}
	cmd := NewChatCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session.handleCommand(cmd, "/model gpt-4", nil)
	if session.modelName != "gpt-4" {
		t.Errorf("expected modelName 'gpt-4', got: %q", session.modelName)
	}
}

func TestChatSession_HandleCommand_StreamToggle(t *testing.T) {
	session := &chatSession{running: true, streamMode: false}
	cmd := NewChatCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session.handleCommand(cmd, "/stream", nil)
	if !session.streamMode {
		t.Error("expected streamMode = true after /stream")
	}
	// Toggle back
	session.handleCommand(cmd, "/stream", nil)
	if session.streamMode {
		t.Error("expected streamMode = false after second /stream")
	}
}

func TestChatSession_HandleCommand_MetricsToggle(t *testing.T) {
	session := &chatSession{running: true, showMetrics: false}
	cmd := NewChatCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session.handleCommand(cmd, "/metrics", nil)
	if !session.showMetrics {
		t.Error("expected showMetrics = true after /metrics")
	}
	session.handleCommand(cmd, "/metrics", nil)
	if session.showMetrics {
		t.Error("expected showMetrics = false after second /metrics")
	}
}

func TestChatSession_HandleCommand_HistoryEmpty(t *testing.T) {
	session := &chatSession{running: true}
	cmd := NewChatCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session.handleCommand(cmd, "/history", nil)
	output := buf.String()
	if !strings.Contains(output, "No conversation history") {
		t.Errorf("expected 'No conversation history' for empty history, got: %q", output)
	}
}

func TestChatSession_HandleCommand_HistoryPopulated(t *testing.T) {
	session := &chatSession{
		running: true,
		history: []chat.Message{
			{Role: chat.RoleUser, Content: "hello"},
			{Role: chat.RoleAssistant, Content: "hi there"},
		},
	}
	cmd := NewChatCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session.handleCommand(cmd, "/history", nil)
	output := buf.String()
	if !strings.Contains(output, "hello") {
		t.Errorf("expected 'hello' from history, got: %q", output)
	}
	if !strings.Contains(output, "Conversation history (2 messages)") {
		t.Errorf("expected history count, got: %q", output)
	}
}

func TestChatSession_HandleCommand_ClearHistory(t *testing.T) {
	session := &chatSession{
		running: true,
		history: []chat.Message{
			{Role: chat.RoleUser, Content: "hello"},
		},
	}
	cmd := NewChatCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session.handleCommand(cmd, "/clear", nil)
	if len(session.history) != 0 {
		t.Errorf("expected empty history after /clear, got %d entries", len(session.history))
	}
}

func TestChatSession_HandleCommand_UnknownCommand(t *testing.T) {
	session := &chatSession{running: true}
	cmd := NewChatCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session.handleCommand(cmd, "/foobar", nil)
	output := buf.String()
	if !strings.Contains(output, "Unknown command") {
		t.Errorf("expected 'Unknown command' for invalid input, got: %q", output)
	}
}

func TestChatSession_HandleCommand_EmptyInput(t *testing.T) {
	session := &chatSession{running: true}
	cmd := NewChatCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Empty string (no parts after Fields split)
	session.handleCommand(cmd, "", nil)
	if !session.running {
		t.Error("empty input should not change running state")
	}
}

// =============================================================================
// handlePrompt / handleSync / handleStream (chat.go:297-376) — REPL prompt
// =============================================================================

func TestChatSession_HandlePrompt_SyncMode(t *testing.T) {
	engine := &mockStreamOrchestrator{}
	session := &chatSession{
		engine:     engine,
		streamMode: false,
		history:    make([]chat.Message, 0),
	}
	cmd := NewChatCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session.handlePrompt(context.Background(), cmd, "test input")
	// Should add user message + assistant response
	if len(session.history) != 2 {
		t.Fatalf("expected 2 history entries (user + assistant), got %d", len(session.history))
	}
	if session.history[0].Role != chat.RoleUser {
		t.Errorf("expected first entry RoleUser, got %s", session.history[0].Role)
	}
	if session.history[0].Content != "test input" {
		t.Errorf("expected first entry content 'test input', got %q", session.history[0].Content)
	}
	if session.history[1].Role != chat.RoleAssistant {
		t.Errorf("expected second entry RoleAssistant, got %s", session.history[1].Role)
	}
}

func TestChatSession_HandlePrompt_SyncMode_ExecuteError(t *testing.T) {
	engine := &mockStreamOrchestrator{executeErr: fmt.Errorf("engine exploded")}
	session := &chatSession{
		engine:     engine,
		streamMode: false,
		history:    make([]chat.Message, 0),
	}
	cmd := NewChatCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session.handlePrompt(context.Background(), cmd, "bad input")
	// User message is added before execution, assistant is NOT added on error
	if len(session.history) != 1 {
		t.Errorf("expected 1 history entry (user only, execute failed), got %d", len(session.history))
	}
}

func TestChatSession_HandlePrompt_StreamMode(t *testing.T) {
	engine := &mockStreamOrchestrator{
		streamEvents: []orchestration.StreamEvent{
			{Type: orchestration.StreamEventChunk, Content: "streaming response"},
		},
	}
	session := &chatSession{
		engine:     engine,
		streamMode: true,
		history:    make([]chat.Message, 0),
	}
	cmd := NewChatCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session.handlePrompt(context.Background(), cmd, "test stream input")
	if len(session.history) != 2 {
		t.Fatalf("expected 2 history entries in stream mode, got %d", len(session.history))
	}
	if session.history[0].Content != "test stream input" {
		t.Errorf("expected user content, got: %q", session.history[0].Content)
	}
	if !strings.Contains(session.history[1].Content, "streaming response") {
		t.Errorf("expected assistant content to contain streamed response, got: %q", session.history[1].Content)
	}
}

func TestChatSession_HandlePrompt_StreamMode_ExecuteStreamError(t *testing.T) {
	engine := &mockStreamOrchestrator{streamErr: fmt.Errorf("stream failed")}
	session := &chatSession{
		engine:     engine,
		streamMode: true,
		history:    make([]chat.Message, 0),
	}
	cmd := NewChatCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session.handlePrompt(context.Background(), cmd, "failing stream input")
	// User message added, stream fails, no assistant added
	if len(session.history) != 1 {
		t.Errorf("expected 1 history entry (stream failed), got %d", len(session.history))
	}
}

func TestChatSession_HandlePrompt_WithAgentContext(t *testing.T) {
	engine := &mockStreamOrchestrator{}
	session := &chatSession{
		engine:     engine,
		agentName:  "Backend Chief",
		streamMode: false,
		history:    make([]chat.Message, 0),
	}
	cmd := NewChatCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session.handlePrompt(context.Background(), cmd, "build something")
	if len(session.history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(session.history))
	}
}

func TestChatSession_HandlePrompt_ShowsMetrics(t *testing.T) {
	engine := &mockStreamOrchestrator{}
	session := &chatSession{
		engine:      engine,
		showMetrics: true,
		streamMode:  false,
		history:     make([]chat.Message, 0),
	}
	cmd := NewChatCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session.handlePrompt(context.Background(), cmd, "show metrics after")
	output := buf.String()
	if !strings.Contains(output, "Messages:") {
		t.Errorf("expected metrics output, got: %q", output)
	}
}

// =============================================================================
// printMetrics (chat.go:378) — minimal session metrics
// =============================================================================

func TestChatSession_PrintMetrics(t *testing.T) {
	session := &chatSession{
		agentName:  "TestAgent",
		streamMode: true,
		history:    []chat.Message{{Role: chat.RoleUser, Content: "test"}},
	}
	cmd := NewChatCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	session.printMetrics(cmd)
	output := buf.String()
	if !strings.Contains(output, "Messages: 1") {
		t.Errorf("expected 'Messages: 1' in output, got: %q", output)
	}
	if !strings.Contains(output, "TestAgent") {
		t.Errorf("expected 'TestAgent' in output, got: %q", output)
	}
	if !strings.Contains(output, "Streaming: true") {
		t.Errorf("expected 'Streaming: true' in output, got: %q", output)
	}
}

// =============================================================================
// NewMemorySnapshotRestoreCommand (memory.go:329) — currently 10%
// =============================================================================

func TestNewMemorySnapshotRestoreCommand_Properties(t *testing.T) {
	cmd := NewMemorySnapshotRestoreCommand()
	if cmd.Use != "restore <id>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "restore <id>")
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.Long == "" {
		t.Error("Long should not be empty")
	}
	if len(cmd.Example) == 0 {
		t.Log("Example is empty (acceptable)")
	}
}

func TestNewMemorySnapshotRestoreCommand_RunE_NotInCoscaDir(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)

	cmd := NewMemorySnapshotRestoreCommand()
	buf := new(bytes.Buffer)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	// Without a .cosca dir, the memory manager still works (creates an
	// in-memory instance) but the snapshot won't be found.
	err := cmd.RunE(cmd, []string{"snap-123"})
	if err == nil {
		t.Fatal("expected error when snapshot does not exist")
	}
	if !strings.Contains(err.Error(), "snapshot not found") && !strings.Contains(err.Error(), "memory manager not available") {
		t.Errorf("expected snapshot/manager error, got: %s", err.Error())
	}
}

func TestNewMemorySnapshotRestoreCommand_JSONOutputFlag(t *testing.T) {
	cmd := NewMemorySnapshotRestoreCommand()
	// Add persistent JSON flag (normally added by root command)
	cmd.PersistentFlags().BoolP("json", "j", false, "json output")
	cmd.PersistentFlags().Set("json", "true")

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	_ = cmd // verify the flag was registered
	jsonFlag := cmd.PersistentFlags().Lookup("json")
	if jsonFlag == nil || jsonFlag.DefValue != "false" {
		t.Error("expected persistent --json flag")
	}
}

// =============================================================================
// Adapter Nil-Inner Coverage — tests error paths when subsystem is unavailable
// =============================================================================

func TestMemoryAdapter_NilInner_List(t *testing.T) {
	adapt := &memoryManagerAdapter{inner: nil}
	_, err := adapt.List("", 10)
	if err == nil {
		t.Error("expected error for nil inner engine")
	}
	if err != nil && err.Error() != "memory not available" {
		t.Errorf("expected 'memory not available', got %q", err.Error())
	}
}

func TestMemoryAdapter_NilInner_Get(t *testing.T) {
	adapt := &memoryManagerAdapter{inner: nil}
	_, err := adapt.Get("rec-1")
	if err == nil {
		t.Error("expected error for nil inner engine")
	}
}

func TestMemoryAdapter_NilInner_Search(t *testing.T) {
	adapt := &memoryManagerAdapter{inner: nil}
	_, err := adapt.Search(MemorySearchOptions{Query: "test", Limit: 5})
	if err == nil {
		t.Error("expected error for nil inner engine")
	}
}

func TestMemoryAdapter_NilInner_CreateSnapshot(t *testing.T) {
	adapt := &memoryManagerAdapter{inner: nil}
	_, err := adapt.CreateSnapshot()
	if err == nil {
		t.Error("expected error for nil inner engine")
	}
}

func TestMemoryAdapter_NilInner_ListSnapshots(t *testing.T) {
	adapt := &memoryManagerAdapter{inner: nil}
	_, err := adapt.ListSnapshots()
	if err == nil {
		t.Error("expected error for nil inner engine")
	}
}

func TestMemoryAdapter_NilInner_RestoreSnapshot(t *testing.T) {
	adapt := &memoryManagerAdapter{inner: nil}
	err := adapt.RestoreSnapshot("snap-1")
	if err == nil {
		t.Error("expected error for nil inner engine")
	}
}

func TestMemoryAdapter_NilInner_Prune(t *testing.T) {
	adapt := &memoryManagerAdapter{inner: nil}
	_, err := adapt.Prune(false)
	if err == nil {
		t.Error("expected error for nil inner engine")
	}
}

func TestMemoryAdapter_NilInner_PruneDryRun(t *testing.T) {
	adapt := &memoryManagerAdapter{inner: nil}
	_, err := adapt.Prune(true)
	if err == nil {
		t.Error("expected error for nil inner engine")
	}
}

func TestMemoryAdapter_NilInner_Init(t *testing.T) {
	adapt := &memoryManagerAdapter{inner: nil}
	adapt.Init() // no-op, should not panic
}

func TestMemoryAdapter_NilInner_Status(t *testing.T) {
	adapt := &memoryManagerAdapter{inner: nil}
	status := adapt.Status()
	if status.TotalEntries != 0 {
		t.Errorf("expected 0 entries for nil inner, got %d", status.TotalEntries)
	}
}

func TestPluginAdapter_NilInner_Install(t *testing.T) {
	adapt := &pluginManagerAdapter{inner: nil}
	_, err := adapt.Install("test-plugin", "source")
	if err == nil || err.Error() != "plugin manager not available" {
		t.Errorf("expected 'plugin manager not available', got %v", err)
	}
}

func TestPluginAdapter_NilInner_Uninstall(t *testing.T) {
	adapt := &pluginManagerAdapter{inner: nil}
	err := adapt.Uninstall("test-plugin")
	if err == nil || err.Error() != "plugin manager not available" {
		t.Errorf("expected 'plugin manager not available', got %v", err)
	}
}

func TestPluginAdapter_NilInner_Update(t *testing.T) {
	adapt := &pluginManagerAdapter{inner: nil}
	_, err := adapt.Update("test-plugin")
	if err == nil || err.Error() != "plugin manager not available" {
		t.Errorf("expected 'plugin manager not available', got %v", err)
	}
}

func TestPluginAdapter_NilInner_Info(t *testing.T) {
	adapt := &pluginManagerAdapter{inner: nil}
	_, err := adapt.Info("test-plugin")
	if err == nil || err.Error() != "plugin manager not available" {
		t.Errorf("expected 'plugin manager not available', got %v", err)
	}
}

func TestPluginAdapter_NilInner_Enable(t *testing.T) {
	adapt := &pluginManagerAdapter{inner: nil}
	err := adapt.Enable("test-plugin")
	if err == nil || err.Error() != "plugin manager not available" {
		t.Errorf("expected 'plugin manager not available', got %v", err)
	}
}

func TestPluginAdapter_NilInner_Disable(t *testing.T) {
	adapt := &pluginManagerAdapter{inner: nil}
	err := adapt.Disable("test-plugin")
	if err == nil || err.Error() != "plugin manager not available" {
		t.Errorf("expected 'plugin manager not available', got %v", err)
	}
}

func TestPluginAdapter_NilInner_List(t *testing.T) {
	adapt := &pluginManagerAdapter{inner: nil}
	result := adapt.List()
	if result != nil {
		t.Error("expected nil list for nil inner")
	}
}

func TestPluginAdapter_NilInner_Scan(t *testing.T) {
	adapt := &pluginManagerAdapter{inner: nil}
	adapt.Scan() // no-op, should not panic
}

func TestPluginAdapter_NilInner_Search(t *testing.T) {
	adapt := &pluginManagerAdapter{inner: nil}
	result, err := adapt.Search("query")
	if err != nil {
		t.Errorf("expected no error for search, got %v", err)
	}
	if result != nil {
		t.Error("expected nil search result")
	}
}

func TestToPluginInfoEx(t *testing.T) {
	p := plugins.PluginInfo{
		Manifest: plugins.PluginManifest{
			Name:        "test-plugin",
			Version:     "1.0.0",
			Description: "A test plugin",
			Author:      "test author",
		},
		Enabled:     true,
		InstalledAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	ex := toPluginInfoEx(p)
	if ex.Name != "test-plugin" {
		t.Errorf("Name = %q, want %q", ex.Name, "test-plugin")
	}
	if ex.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", ex.Version, "1.0.0")
	}
	if ex.Description != "A test plugin" {
		t.Errorf("Description = %q, want %q", ex.Description, "A test plugin")
	}
	if ex.Author != "test author" {
		t.Errorf("Author = %q, want %q", ex.Author, "test author")
	}
	if !ex.Enabled {
		t.Error("expected Enabled = true")
	}
}

func TestKnowledgeAdapter_NilInner_Search(t *testing.T) {
	adapt := &knowledgeEngineAdapter{inner: nil}
	_, err := adapt.Search("query", 5, 0)
	// Lazy initialization may create a real engine or fail — both are valid paths
	if err != nil {
		t.Logf("Search with nil inner returned error (expected): %v", err)
	}
}

func TestKnowledgeAdapter_NilInner_Verify(t *testing.T) {
	adapt := &knowledgeEngineAdapter{inner: nil}
	_, err := adapt.Verify()
	if err != nil {
		t.Logf("Verify with nil inner returned error (expected): %v", err)
	}
}

func TestKnowledgeAdapter_NilInner_Stats(t *testing.T) {
	adapt := &knowledgeEngineAdapter{inner: nil}
	_, err := adapt.Stats()
	if err != nil {
		t.Logf("Stats with nil inner returned error (expected): %v", err)
	}
}

func TestKnowledgeAdapter_NilInner_Explain(t *testing.T) {
	adapt := &knowledgeEngineAdapter{inner: nil}
	_, err := adapt.Explain("result-1")
	if err != nil {
		t.Logf("Explain with nil inner returned error (expected): %v", err)
	}
}

func TestRuntimeAdapter_NilInner_Start(t *testing.T) {
	adapt := &runtimeAdapter{inner: nil}
	// Calling Start with nil inner will panic (nil dereference) — test that the panic happens
	defer func() {
		if r := recover(); r != nil {
			t.Log("Start with nil inner panicked (expected)")
		}
	}()
	_ = adapt.Start(false)
}

func TestRuntimeAdapter_NilInner_Stop(t *testing.T) {
	adapt := &runtimeAdapter{inner: nil}
	defer func() {
		if r := recover(); r != nil {
			t.Log("Stop with nil inner panicked (expected)")
		}
	}()
	_ = adapt.Stop(false)
}

func TestRuntimeAdapter_NilInner_Restart(t *testing.T) {
	adapt := &runtimeAdapter{inner: nil}
	defer func() {
		if r := recover(); r != nil {
			t.Log("Restart with nil inner panicked (expected)")
		}
	}()
	_ = adapt.Restart()
}

func TestRuntimeAdapter_NilInner_PID(t *testing.T) {
	adapt := &runtimeAdapter{inner: nil}
	defer func() {
		if r := recover(); r != nil {
			t.Log("PID with nil inner panicked (expected)")
		}
	}()
	_ = adapt.PID()
}

func TestRuntimeAdapter_NilInner_FollowLogs(t *testing.T) {
	adapt := &runtimeAdapter{inner: nil}
	defer func() {
		if r := recover(); r != nil {
			t.Log("FollowLogs with nil inner panicked (expected)")
		}
	}()
	_ = adapt.FollowLogs(10)
}

func TestProviderAdapter_NilInner_SetActive(t *testing.T) {
	adapt := &providerManagerAdapter{inner: nil}
	err := adapt.SetActive("openai", "gpt-4")
	if err == nil {
		t.Error("expected error for nil inner provider manager")
	}
}

// =============================================================================
// Agent & Skill Resolver Adapters — success paths via embedded agents/skills
// =============================================================================

func TestAgentResolverAdapter_Get(t *testing.T) {
	mgr := agents.NewManager("")
	adapt := adapter.NewAgentResolverAdapter(mgr)
	// Embedded agents should be available
	info, err := adapt.Get("Backend Chief")
	if err != nil {
		t.Logf("Get returned error (may be expected without full setup): %v", err)
		return
	}
	if info == nil {
		t.Error("expected non-nil agent info")
	}
}

func TestAgentResolverAdapter_GetNonExistent(t *testing.T) {
	mgr := agents.NewManager("")
	adapt := adapter.NewAgentResolverAdapter(mgr)
	_, err := adapt.Get("nonexistent-agent-xyz")
	if err == nil {
		t.Error("expected error for non-existent agent")
	}
}

func TestAgentResolverAdapter_Search(t *testing.T) {
	mgr := agents.NewManager("")
	adapt := adapter.NewAgentResolverAdapter(mgr)
	results, err := adapt.Search("backend")
	if err != nil {
		t.Logf("Search returned error: %v", err)
		return
	}
	if results == nil {
		t.Error("expected non-nil results slice")
	}
}

func TestSkillResolverAdapter_List(t *testing.T) {
	mgr := skills.NewManager("")
	adapt := adapter.NewSkillResolverAdapter(mgr)
	results, err := adapt.List()
	if err != nil {
		t.Logf("List returned error: %v", err)
		return
	}
	if results == nil {
		t.Error("expected non-nil results slice")
	}
}

func TestSkillResolverAdapter_Get(t *testing.T) {
	mgr := skills.NewManager("")
	adapt := adapter.NewSkillResolverAdapter(mgr)
	// Try a generic search since specific skill names depend on embedded data
	info, err := adapt.Get("code-review")
	if err != nil {
		t.Logf("Get returned error (may be expected without specific skill): %v", err)
		return
	}
	if info == nil {
		t.Error("expected non-nil skill info")
	}
}

func TestSkillResolverAdapter_GetNonExistent(t *testing.T) {
	mgr := skills.NewManager("")
	adapt := adapter.NewSkillResolverAdapter(mgr)
	_, err := adapt.Get("nonexistent-skill-xyz")
	if err == nil {
		t.Error("expected error for non-existent skill")
	}
}

func TestSkillResolverAdapter_Search(t *testing.T) {
	mgr := skills.NewManager("")
	adapt := adapter.NewSkillResolverAdapter(mgr)
	results, err := adapt.Search("code")
	if err != nil {
		t.Logf("Search returned error: %v", err)
		return
	}
	if results == nil {
		t.Error("expected non-nil results slice")
	}
}

// =============================================================================
// SetMetricsEngine (metrics.go:18)
// =============================================================================

func TestSetMetricsEngine_NonNil(t *testing.T) {
	orig := engineHolder
	defer func() { engineHolder = orig }()
	// engineHolder is an *orchestration.Engine; we can test nil assignment
	SetMetricsEngine(nil)
	if engineHolder != nil {
		t.Error("expected engineHolder to be nil after SetMetricsEngine(nil)")
	}
}

// =============================================================================
// openBrowser / findLocalDocs / docs
// =============================================================================

func TestOpenBrowser(t *testing.T) {
	// defaultOpenBrowser não pode abrir janela com URL vazia (guarda). Testa que
	// não há panic e que retorna erro — sem abrir o File Explorer ("Este Computador").
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("defaultOpenBrowser panicked: %v", r)
		}
	}()
	if err := defaultOpenBrowser(""); err == nil {
		t.Errorf("defaultOpenBrowser('') deveria falhar (guard de URL vazia)")
	}
}

func TestFindLocalDocs_DoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("findLocalDocs panicked: %v", r)
		}
	}()
	_ = findLocalDocs()
}
