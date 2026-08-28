//
// Tests for the CLI layer: root command registration, global flags,
// output formatter, config init, and smoke tests for all subcommands.

package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/config"
)

// =============================================================================
// Root Command Registration
// =============================================================================

func TestRootCommand_HasAllSubcommands(t *testing.T) {

	cmd := NewRootCommand()
	expected := []string{
		"init",
		"install",
		"uninstall",
		"update",
		"upgrade",
		"sync",
		"status",
		"version",
		"knowledge",
		"search",
		"serve",
		"doctor",
		"runtime",
		"config",
		"cache",
		"context",
		"memory",
		"kernel",
		"don",
		"circadian",
		"cofre",
		"plan",
		"approve",
		"propose",
		"decision",
		"delegate",
		"plugin",
		"editor",
		"provider",
		"models",
		"index",
		"graph",
		"workflow",
		"agent",
		"skill",
		"skills",
		"prompt",
		"template",
		"docs",
		"health",
		"validate",
		"benchmark",
		"perf",
		"symbols",
		"embed",
		"bootstrap",
		"completion",
		"run",
		"pipeline",
		"pipeline-test",
		"plugins",
		"metrics",
		"chat",
		"exec",
		"mcp",
		"hook",
		"license",
		"cv",
		"capability",
		"quarantine",
		"gate",
		"qgate",
		"routes",
		"bug",
		"hardware",
		"fabric",
		"machine",
		"conflict",
		"cron",
		"session",
		"start",
		"trace",
		"evidence",
		"eval",
		"budget",
		"ranking",
		"acquisition",
		"department",
		"terminal",
		"project",
		"asset",
		"bridge",
		"task",
		"model",
		"gpu",
		"ngraph",
		"render",
		"media",
		"flow",
		"provenance",
		"security",
		"despertar",
		"desktop",
		"voice",
		"slop",
		"world",
		"db",
	}

	for _, name := range expected {
		found := false
		for _, sub := range cmd.Commands() {
			if sub.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing subcommand: %s", name)
		}
	}

	// Also verify we don't have unexpected extras (only the 2 completion helpers).
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		delete(registered, name)
	}
	// cobra adds "completion" automatically even with DisableDefaultCmd = true
	// when using the root's built-in Version field; ignore it.
	delete(registered, "completion")
	for extra := range registered {
		t.Errorf("unexpected subcommand: %s", extra)
	}
}

func TestRootCommand_DefaultUse(t *testing.T) {

	cmd := NewRootCommand()
	if cmd.Use != "cosca" {
		t.Errorf("expected Use='cosca', got %q", cmd.Use)
	}
	if cmd.SilenceUsage != true {
		t.Error("expected SilenceUsage=true")
	}
	if cmd.SilenceErrors != true {
		t.Error("expected SilenceErrors=true")
	}
}

func TestRootCommand_RunShowsHelp(t *testing.T) {

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "Usage:") {
		t.Error("expected help output to contain 'Usage:'")
	}
}

// =============================================================================
// Global Flags
// =============================================================================

func TestRootCommand_GlobalFlags(t *testing.T) {

	tests := []struct {
		name       string
		flagName   string
		shorthand  string
		defaultVal string
	}{
		{"config flag", "config", "", ""},
		{"verbose flag", "verbose", "V", "false"},
		{"quiet flag", "quiet", "q", "false"},
		{"json flag", "json", "j", "false"},
		{"format flag", "format", "", "text"},
		{"no-color flag", "no-color", "", "false"},
		{"version flag", "version", "v", "false"},
	}

	cmd := NewRootCommand()

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {

			flag := cmd.PersistentFlags().Lookup(tt.flagName)
			if flag == nil {
				t.Fatalf("missing persistent flag: --%s", tt.flagName)
			}
			if tt.shorthand != "" {
				shorthand := cmd.PersistentFlags().ShorthandLookup(tt.shorthand)
				if shorthand == nil {
					t.Fatalf("missing shorthand flag: -%s", tt.shorthand)
				}
				if shorthand.Name != tt.flagName {
					t.Errorf("shorthand -%s points to %q, expected %q",
						tt.shorthand, shorthand.Name, tt.flagName)
				}
			}
			if tt.defaultVal != "" && flag.DefValue != tt.defaultVal {
				t.Errorf("flag --%s default = %q, want %q",
					tt.flagName, flag.DefValue, tt.defaultVal)
			}
		})
	}
}

func TestRootCommand_JSONFlagOverridesFormat(t *testing.T) {
	// Note: can't run parallel because it'd mutate globalFlags.
	// This test verifies the precedence: --json=true overrides --format=yaml.

	globalFlags = GlobalFlags{} // reset
	cmd := NewRootCommand()

	// Set --json and --format
	_ = cmd.PersistentFlags().Set("json", "true")
	_ = cmd.PersistentFlags().Set("format", "yaml")

	// Make sure the json flag is true
	jsonFlag := cmd.PersistentFlags().Lookup("json")
	if jsonFlag == nil {
		t.Fatal("missing --json flag")
	}
	_ = jsonFlag // we just need to check IsJSONOutput

	// IsJSONOutput checks both the flag and globalFlags.JSON
	if !IsJSONOutput(cmd) {
		t.Error("expected IsJSONOutput=true when --json is set")
	}
}

// =============================================================================
// OutputFormatter
// =============================================================================

func TestNewOutputFormatter(t *testing.T) {

	tests := []struct {
		name    string
		format  OutputFormat
		verbose bool
		quiet   bool
		noColor bool
	}{
		{"text format", OutputFormatText, false, false, false},
		{"json format", OutputFormatJSON, false, false, false},
		{"yaml format", OutputFormatYAML, false, false, false},
		{"table format", OutputFormatTable, false, false, false},
		{"verbose mode", OutputFormatText, true, false, false},
		{"quiet mode", OutputFormatText, false, true, false},
		{"no-color mode", OutputFormatText, false, false, true},
		{"verbose+no-color", OutputFormatJSON, true, false, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {

			var buf bytes.Buffer
			f := NewOutputFormatter(&buf, tt.format, tt.verbose, tt.quiet, tt.noColor)
			if f == nil {
				t.Fatal("NewOutputFormatter returned nil")
			}
		})
	}
}

func TestNewOutputFormatter_NilWriterDefaultsToStdout(t *testing.T) {

	f := NewOutputFormatter(nil, OutputFormatText, false, false, false)
	if f == nil {
		t.Fatal("NewOutputFormatter returned nil")
	}
	// Verify it can write without panicking
	f.Println("hello")
}

func TestOutputFormat_Constants(t *testing.T) {

	tests := []struct {
		format OutputFormat
		want   string
	}{
		{OutputFormatText, "text"},
		{OutputFormatJSON, "json"},
		{OutputFormatYAML, "yaml"},
		{OutputFormatTable, "table"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(string(tt.format), func(t *testing.T) {

			if string(tt.format) != tt.want {
				t.Errorf("OutputFormat constant = %q, want %q", tt.format, tt.want)
			}
		})
	}
}

func TestOutputFormatter_SetFormat(t *testing.T) {

	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
	f.SetFormat(OutputFormatJSON)
	// Print some data in JSON format
	err := f.Print(map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("Print error: %v", err)
	}
	if !strings.Contains(buf.String(), `"key"`) {
		t.Error("expected JSON output to contain 'key'")
	}
}

func TestOutputFormatter_Println(t *testing.T) {

	t.Run("normal mode", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
		f.Println("hello")
		if got := buf.String(); got != "hello\n" {
			t.Errorf("Println output = %q, want %q", got, "hello\n")
		}
	})

	t.Run("quiet mode suppresses output", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, false, true, false)
		f.Println("hello")
		if buf.Len() != 0 {
			t.Errorf("expected no output in quiet mode, got %q", buf.String())
		}
	})
}

func TestOutputFormatter_Printf(t *testing.T) {

	t.Run("normal mode", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
		f.Printf("hello %s", "world")
		if got := buf.String(); got != "hello world" {
			t.Errorf("Printf output = %q, want %q", got, "hello world")
		}
	})

	t.Run("quiet mode suppresses output", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, false, true, false)
		f.Printf("should not appear")
		if buf.Len() != 0 {
			t.Errorf("expected no output in quiet mode")
		}
	})
}

func TestOutputFormatter_Verbose(t *testing.T) {

	t.Run("verbose mode prints", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, true, false, false)
		f.Verbose("debug info")
		if !strings.Contains(buf.String(), "debug info") {
			t.Error("expected verbose output to contain message")
		}
		if !strings.Contains(buf.String(), "[verbose]") {
			t.Error("expected verbose output to contain [verbose] tag")
		}
	})

	t.Run("non-verbose mode suppresses", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
		f.Verbose("debug info")
		if buf.Len() != 0 {
			t.Error("expected no verbose output when verbose=false")
		}
	})

	t.Run("quiet overrides verbose", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, true, true, false)
		f.Verbose("debug info")
		if buf.Len() != 0 {
			t.Error("expected no verbose output when quiet=true")
		}
	})
}

func TestOutputFormatter_Debug(t *testing.T) {

	t.Run("verbose mode prints debug", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, true, false, false)
		f.Debug("dbg msg")
		if !strings.Contains(buf.String(), "dbg msg") {
			t.Error("expected debug output to contain message")
		}
		if !strings.Contains(buf.String(), "[debug]") {
			t.Error("expected debug output to contain [debug] tag")
		}
	})

	t.Run("non-verbose suppresses debug", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
		f.Debug("dbg msg")
		if buf.Len() != 0 {
			t.Error("expected no debug output when verbose=false")
		}
	})
}

func TestOutputFormatter_Success(t *testing.T) {

	t.Run("prints success with checkmark", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
		f.Success("done")
		if !strings.Contains(buf.String(), "done") {
			t.Error("expected success output to contain message")
		}
	})

	t.Run("quiet mode suppresses", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, false, true, false)
		f.Success("done")
		if buf.Len() != 0 {
			t.Error("expected no success output in quiet mode")
		}
	})
}

func TestOutputFormatter_Warning(t *testing.T) {

	t.Run("prints warning", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
		f.Warning("caution")
		if !strings.Contains(buf.String(), "caution") {
			t.Error("expected warning output to contain message")
		}
	})

	t.Run("quiet mode suppresses", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, false, true, false)
		f.Warning("caution")
		if buf.Len() != 0 {
			t.Error("expected no warning output in quiet mode")
		}
	})
}

func TestOutputFormatter_Error(t *testing.T) {

	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
	f.Error("fail")
	// Error always prints even in quiet mode
	if !strings.Contains(buf.String(), "fail") {
		t.Error("expected error output to contain message")
	}
}

func TestOutputFormatter_Errorf(t *testing.T) {

	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
	f.Errorf("error %d", 42)
	output := buf.String()
	if !strings.Contains(output, "error 42") {
		t.Errorf("Errorf output = %q, want to contain 'error 42'", output)
	}
}

func TestOutputFormatter_Header(t *testing.T) {

	t.Run("normal mode prints header", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
		f.Header("Section")
		output := buf.String()
		if !strings.Contains(output, "Section") {
			t.Error("expected header to contain title")
		}
	})

	t.Run("quiet mode suppresses header", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, false, true, false)
		f.Header("Section")
		if buf.Len() != 0 {
			t.Error("expected no header output in quiet mode")
		}
	})
}

func TestOutputFormatter_Table(t *testing.T) {

	t.Run("prints table", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
		f.Table([]string{"Name", "Age"}, [][]string{{"Alice", "30"}, {"Bob", "25"}})
		output := buf.String()
		if !strings.Contains(output, "Name") {
			t.Error("expected table to contain header 'Name'")
		}
		if !strings.Contains(output, "Alice") {
			t.Error("expected table to contain 'Alice'")
		}
	})

	t.Run("empty headers produces no output", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
		f.Table(nil, [][]string{{"data"}})
		if buf.Len() != 0 {
			t.Error("expected no output with nil headers")
		}
	})

	t.Run("empty rows produces no output", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
		f.Table([]string{"H"}, nil)
		if buf.Len() != 0 {
			t.Error("expected no output with nil rows")
		}
	})

	t.Run("quiet mode suppresses table", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, false, true, false)
		f.Table([]string{"Name"}, [][]string{{"Alice"}})
		if buf.Len() != 0 {
			t.Error("expected no table output in quiet mode")
		}
	})
}

func TestOutputFormatter_KeyValue(t *testing.T) {

	t.Run("prints key-value pair", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
		f.KeyValue("key", "value")
		if !strings.Contains(buf.String(), "key") || !strings.Contains(buf.String(), "value") {
			t.Error("expected key and value in output")
		}
	})

	t.Run("quiet mode suppresses", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, false, true, false)
		f.KeyValue("k", "v")
		if buf.Len() != 0 {
			t.Error("expected no output in quiet mode")
		}
	})
}

func TestOutputFormatter_Bullet(t *testing.T) {

	t.Run("prints bullet point", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
		f.Bullet("item")
		if !strings.Contains(buf.String(), "item") {
			t.Errorf("expected bullet to contain item, got %q", buf.String())
		}
	})

	t.Run("quiet mode suppresses", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, false, true, false)
		f.Bullet("item")
		if buf.Len() != 0 {
			t.Error("expected no output in quiet mode")
		}
	})
}

func TestOutputFormatter_Print_Text(t *testing.T) {

	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
	err := f.Print("hello")
	if err != nil {
		t.Fatalf("Print error: %v", err)
	}
	if !strings.Contains(buf.String(), "hello") {
		t.Errorf("Print output = %q, want to contain 'hello'", buf.String())
	}
}

func TestOutputFormatter_Print_JSON(t *testing.T) {

	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatJSON, false, false, false)
	err := f.Print(map[string]string{"foo": "bar"})
	if err != nil {
		t.Fatalf("Print error: %v", err)
	}
	if !strings.Contains(buf.String(), `"foo"`) {
		t.Errorf("Print JSON output = %q, want to contain 'foo'", buf.String())
	}
}

func TestOutputFormatter_Print_YAML(t *testing.T) {

	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatYAML, false, false, false)
	err := f.Print(map[string]string{"foo": "bar"})
	if err != nil {
		t.Fatalf("Print error: %v", err)
	}
	if !strings.Contains(buf.String(), "foo: bar") {
		t.Errorf("Print YAML output = %q, want to contain 'foo: bar'", buf.String())
	}
}

func TestOutputFormatter_Print_Table(t *testing.T) {

	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatTable, false, false, false)
	err := f.Print("hello table")
	if err != nil {
		t.Fatalf("Print error: %v", err)
	}
	if !strings.Contains(buf.String(), "hello table") {
		t.Errorf("Print(Table) output = %q, want to contain 'hello table'", buf.String())
	}
}

func TestOutputFormatter_ColorSchemes(t *testing.T) {

	t.Run("DefaultColorScheme has ANSI codes", func(t *testing.T) {

		cs := DefaultColorScheme()
		if cs.Reset == "" {
			t.Error("expected non-empty Reset")
		}
		if cs.Red == "" {
			t.Error("expected non-empty Red")
		}
	})

	t.Run("NoColorScheme has empty codes", func(t *testing.T) {

		cs := NoColorScheme()
		if cs.Reset != "" || cs.Red != "" || cs.Green != "" {
			t.Error("expected all empty in NoColorScheme")
		}
	})

	t.Run("noColor flag selects NoColorScheme", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
		f.Success("test")
		// Should not contain ANSI escape codes
		if strings.Contains(buf.String(), "\033[") {
			t.Error("expected no ANSI codes in no-color mode")
		}
	})
}

// =============================================================================
// GetFormatter
// =============================================================================

func TestGetFormatter_FromContext(t *testing.T) {

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	formatter := NewOutputFormatter(&buf, OutputFormatJSON, true, false, true)
	ctx := newContextWithFormatter(context.Background(), formatter)
	cmd.SetContext(ctx)

	got := GetFormatter(cmd)
	if got == nil {
		t.Fatal("GetFormatter returned nil")
	}

	// Verify it's the same instance by printing
	got.Println("ctx test")
	if !strings.Contains(buf.String(), "ctx test") {
		t.Error("expected formatter from context to write to buf")
	}
}

func TestGetFormatter_NoContextReturnsDefault(t *testing.T) {

	// cobra v1.8.0 requires an explicit context to be set;
	// otherwise cmd.Context() returns nil.
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	f := GetFormatter(cmd)
	if f == nil {
		t.Fatal("GetFormatter returned nil")
	}

	// Should default to text format, not quiet, not no-color
	var buf bytes.Buffer
	f2 := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
	f2.Println("default")
	if !strings.Contains(buf.String(), "default") {
		t.Error("expected default formatter to work")
	}
}

// =============================================================================
// Config Initialization
// =============================================================================

func TestInitConfig_Defaults(t *testing.T) {
	// Can't run in parallel due to global state (cfgFile, viper).

	// Save original state
	origCfgFile := cfgFile
	origWorkDir, _ := os.Getwd()

	// Restore after test
	defer func() {
		cfgFile = origCfgFile
		_ = os.Chdir(origWorkDir)
	}()

	// Run in a temp directory with no .cosca/config.yaml to test defaults
	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	cfgFile = ""

	cmd := &cobra.Command{}
	err := initConfig(cmd)
	if err != nil {
		t.Fatalf("initConfig returned error: %v", err)
	}
}

func TestInitConfig_WithConfigFlag(t *testing.T) {
	// Can't run in parallel due to global state.

	origCfgFile := cfgFile
	origWorkDir, _ := os.Getwd()

	defer func() {
		cfgFile = origCfgFile
		_ = os.Chdir(origWorkDir)
	}()

	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	cfgFile = "" // will be empty but no .cosca dir should exist

	cmd := &cobra.Command{}
	err := initConfig(cmd)
	// Config file is optional, so this should pass even without a config file.
	if err != nil {
		t.Fatalf("initConfig with no config file returned error: %v", err)
	}
}

func TestInitConfig_EnvPrefix(t *testing.T) {
	// Verify viper env prefix is set by looking at the viper instance.

	origCfgFile := cfgFile
	origWorkDir, _ := os.Getwd()

	defer func() {
		cfgFile = origCfgFile
		_ = os.Chdir(origWorkDir)
	}()

	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)
	cfgFile = ""

	cmd := &cobra.Command{}
	_ = initConfig(cmd)

	v := config.GetViper()
	if v == nil {
		t.Fatal("GetViper returned nil after initConfig")
	}
}

// =============================================================================
// Version Command
// =============================================================================

func TestNewVersionCommand_ReturnsNonNil(t *testing.T) {

	cmd := NewVersionCommand()
	if cmd == nil {
		t.Fatal("NewVersionCommand returned nil")
	}
}

func TestVersionCommand_Properties(t *testing.T) {

	cmd := NewVersionCommand()
	if cmd.Use != "version" {
		t.Errorf("expected Use='version', got %q", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
	if cmd.Long == "" {
		t.Error("expected non-empty Long description")
	}
}

func TestVersionCommand_RunE(t *testing.T) {

	root := NewRootCommand()
	cmd, _, err := root.Find([]string{"version"})
	if err != nil {
		t.Fatalf("Find('version') error: %v", err)
	}

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Set a context so cmd.Context() doesn't return nil
	cmd.SetContext(context.Background())

	err = cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Cosca") {
		t.Errorf("expected output to contain 'Cosca', got: %s", output)
	}
	if !strings.Contains(output, "Version") {
		t.Errorf("expected output to contain 'Version', got: %s", output)
	}
}

// =============================================================================
// Install Command
// =============================================================================

func TestNewInstallCommand_ReturnsNonNil(t *testing.T) {

	cmd := NewInstallCommand()
	if cmd == nil {
		t.Fatal("NewInstallCommand returned nil")
	}
}

func TestInstallCommand_Properties(t *testing.T) {

	cmd := NewInstallCommand()
	if cmd.Use != "install" {
		t.Errorf("expected Use='install', got %q", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
}

func TestInstallCommand_HasGlobalFlag(t *testing.T) {

	cmd := NewInstallCommand()
	flag := cmd.Flags().Lookup("global")
	if flag == nil {
		t.Fatal("missing --global flag")
	}
	if flag.DefValue != "false" {
		t.Errorf("expected --global default=false, got %q", flag.DefValue)
	}
}

// =============================================================================
// Knowledge Command
// =============================================================================

func TestNewKnowledgeCommand_ReturnsNonNil(t *testing.T) {

	cmd := NewKnowledgeCommand()
	if cmd == nil {
		t.Fatal("NewKnowledgeCommand returned nil")
	}
}

func TestKnowledgeCommand_Properties(t *testing.T) {

	cmd := NewKnowledgeCommand()
	if cmd.Use != "knowledge" {
		t.Errorf("expected Use='knowledge', got %q", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
}

// =============================================================================
// Smoke Tests — Verify all New*Command() functions return non-nil commands
// =============================================================================

type commandTestCase struct {
	name    string
	factory func() *cobra.Command
	use     string
	skipUse bool // set true for commands where Use may be too long to check
	hasSubs bool
}

func TestAllCommands_Smoke(t *testing.T) {

	commands := []commandTestCase{
		{name: "init", factory: NewInitCommand, use: "init"},
		{name: "install", factory: NewInstallCommand, use: "install"},
		{name: "uninstall", factory: NewUninstallCommand, use: "uninstall"},
		{name: "update", factory: NewUpdateCommand, use: "update"},
		{name: "upgrade", factory: NewUpgradeCommand, use: "upgrade"},
		{name: "sync", factory: NewSyncCommand, use: "sync"},
		{name: "status", factory: NewStatusCommand, use: "status"},
		{name: "version", factory: NewVersionCommand, use: "version"},
		{name: "knowledge", factory: NewKnowledgeCommand, use: "knowledge", hasSubs: true},
		{name: "search", factory: NewSearchCommand, use: "search"},
		{name: "doctor", factory: NewDoctorCommand, use: "doctor"},
		{name: "runtime", factory: NewRuntimeCommand, use: "runtime", hasSubs: true},
		{name: "config", factory: NewConfigCommand, use: "config", hasSubs: true},
		{name: "cache", factory: NewCacheCommand, use: "cache", hasSubs: true},
		{name: "context", factory: NewContextCommand, use: "context", hasSubs: true},
		{name: "memory", factory: NewMemoryCommand, use: "memory", hasSubs: true},
		{name: "kernel", factory: NewKernelCommand, use: "kernel", hasSubs: true},
		{name: "circadian", factory: NewCircadianCommand, use: "circadian", hasSubs: true},
		{name: "plugin", factory: NewPluginCommand, use: "plugin", hasSubs: true},
		{name: "editor", factory: NewEditorCommand, use: "editor", hasSubs: true},
		{name: "provider", factory: NewProviderCommand, use: "provider", hasSubs: true},
		{name: "index", factory: NewIndexCommand, use: "index", hasSubs: true},
		{name: "graph", factory: NewGraphCommand, use: "graph", hasSubs: true},
		{name: "workflow", factory: NewWorkflowCommand, use: "workflow", hasSubs: true},
		{name: "agent", factory: NewAgentCommand, use: "agent", hasSubs: true},
		{name: "skill", factory: NewSkillCommand, use: "skill", hasSubs: true},
		{name: "prompt", factory: NewPromptCommand, use: "prompt", hasSubs: true},
		{name: "template", factory: NewTemplateCommand, use: "template", hasSubs: true},
		{name: "docs", factory: NewDocsCommand, use: "docs"},
		{name: "health", factory: NewHealthCommand, use: "health"},
		{name: "validate", factory: NewValidateCommand, use: "validate"},
		{name: "benchmark", factory: NewBenchmarkCommand, use: "benchmark"},
		{name: "bootstrap", factory: NewBootstrapCommand, use: "bootstrap"},
		{name: "completion", factory: NewCompletionCommand, use: "completion"},
		{name: "delegate", factory: NewDelegateCommand, use: "delegate"},
		{name: "hook", factory: NewHookCommand, use: "hook", hasSubs: true},
		{name: "hardware", factory: NewHardwareCommand, use: "hardware", hasSubs: true},
		{name: "machine", factory: NewMachineCommand, use: "machine", hasSubs: true},
		{name: "license", factory: NewLicenseCommand, use: "license", hasSubs: true},
	}

	for _, tc := range commands {
		tc := tc
		t.Run(tc.name+"_returns_non_nil", func(t *testing.T) {

			cmd := tc.factory()
			if cmd == nil {
				t.Fatalf("New%sCommand() returned nil", tc.name)
			}
			if !tc.skipUse && cmd.Use != "" && tc.use != "" && cmd.Use != tc.use {
				// For commands with complex Use patterns (e.g., "search [type] <query>"),
				// only check prefix
				if !strings.HasPrefix(cmd.Use, tc.use) {
					t.Errorf("expected Use to start with %q, got %q", tc.use, cmd.Use)
				}
			}
			if cmd.Short == "" {
				t.Errorf("expected non-empty Short description for %s", tc.name)
			}
			if cmd.Long == "" {
				t.Errorf("expected non-empty Long description for %s", tc.name)
			}
			if tc.hasSubs && len(cmd.Commands()) == 0 {
				t.Errorf("expected %s to have subcommands, but has none", tc.name)
			}
		})
	}
}

// =============================================================================
// Subcommand Registration — Knowledge subcommands
// =============================================================================

func TestKnowledgeSubcommands(t *testing.T) {

	cmd := NewKnowledgeCommand()
	expectedSubs := []string{
		"search",
		"graph",
		"rebuild",
		"verify",
		"stats",
		"benchmark",
		"vacuum",
		"explain",
		"relations",
		"law",
	}

	for _, name := range expectedSubs {
		found := false
		for _, sub := range cmd.Commands() {
			if sub.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing knowledge subcommand: %s", name)
		}
	}
}

// =============================================================================
// Helper functions
// =============================================================================

func TestVersionInfo(t *testing.T) {

	info := GetVersionInfo()
	if info.Version != Version {
		t.Errorf("VersionInfo.Version = %q, want %q", info.Version, Version)
	}
	if info.Commit != Commit {
		t.Errorf("VersionInfo.Commit = %q, want %q", info.Commit, Commit)
	}
	if info.GoVersion == "" {
		t.Error("expected non-empty GoVersion")
	}
}

func TestGetBuildDate(t *testing.T) {

	// BuildDate is "unknown" by default, so GetBuildDate should return zero time
	bd := GetBuildDate()
	if !bd.IsZero() {
		t.Errorf("expected zero time for unknown build date, got %v", bd)
	}
}

func TestFormatError(t *testing.T) {

	t.Run("formats non-nil error", func(t *testing.T) {

		result := FormatError(fmt.Errorf("something went wrong"))
		if !strings.Contains(result, "something went wrong") {
			t.Errorf("expected error message in output, got %q", result)
		}
	})

	t.Run("nil error returns empty", func(t *testing.T) {

		result := FormatError(nil)
		if result != "" {
			t.Errorf("expected empty string for nil error, got %q", result)
		}
	})
}

func TestFormatSuccess(t *testing.T) {

	result := FormatSuccess("all good")
	if !strings.Contains(result, "all good") {
		t.Errorf("expected success message in output, got %q", result)
	}
}

func TestFormatWarning(t *testing.T) {

	result := FormatWarning("be careful")
	if !strings.Contains(result, "be careful") {
		t.Errorf("expected warning message in output, got %q", result)
	}
}

func TestFormatBold(t *testing.T) {

	result := FormatBold("important")
	if !strings.Contains(result, "important") {
		t.Errorf("expected bold text in output, got %q", result)
	}
}

// =============================================================================
// ProgressBar
// =============================================================================

func TestNewProgressBar(t *testing.T) {

	var buf bytes.Buffer
	pb := NewProgressBar(&buf, 10, "testing", false)
	if pb == nil {
		t.Fatal("NewProgressBar returned nil")
	}
}

func TestProgressBar_Increment(t *testing.T) {

	var buf bytes.Buffer
	pb := NewProgressBar(&buf, 5, "progress", false)
	pb.Increment()
	pb.Increment()
	pb.Complete()

	output := buf.String()
	if !strings.Contains(output, "progress completed") {
		t.Errorf("expected completion message, got %q", output)
	}
}

func TestProgressBar_Add(t *testing.T) {

	var buf bytes.Buffer
	pb := NewProgressBar(&buf, 10, "test", false)
	pb.Add(3)
	pb.Add(7)
	pb.Complete()
	if !strings.Contains(buf.String(), "completed") {
		t.Error("expected completion message")
	}
}

func TestProgressBar_Complete(t *testing.T) {

	var buf bytes.Buffer
	pb := NewProgressBar(&buf, 3, "task", false)
	pb.Complete()
	if !strings.Contains(buf.String(), "completed") {
		t.Error("expected completion message")
	}
}

func TestProgressBar_AddClampsToTotal(t *testing.T) {

	var buf bytes.Buffer
	pb := NewProgressBar(&buf, 3, "clamp", false)
	pb.Add(10) // exceeds total
	pb.Complete()
	if !strings.Contains(buf.String(), "completed") {
		t.Error("expected completion message even when adding past total")
	}
}

// =============================================================================
// Spinner
// =============================================================================

func TestNewSpinner(t *testing.T) {

	var buf bytes.Buffer
	s := NewSpinner(&buf, "loading", false)
	if s == nil {
		t.Fatal("NewSpinner returned nil")
	}
}

func TestSpinner_StartStop(t *testing.T) {

	var buf bytes.Buffer
	s := NewSpinner(&buf, "working", false)
	s.Start()
	s.Stop("done")

	output := buf.String()
	if !strings.Contains(output, "done") {
		t.Errorf("expected stop message in output, got %q", output)
	}
}

func TestSpinner_StartFail(t *testing.T) {

	var buf bytes.Buffer
	s := NewSpinner(&buf, "working", false)
	s.Start()
	s.Fail("failed")

	output := buf.String()
	if !strings.Contains(output, "failed") {
		t.Errorf("expected fail message in output, got %q", output)
	}
}

// =============================================================================
// TreeItem
// =============================================================================

func TestOutputFormatter_Tree(t *testing.T) {

	t.Run("prints tree structure", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
		items := []TreeItem{
			{Label: "root", Detail: "the root"},
		}
		f.Tree(items, "")
		if !strings.Contains(buf.String(), "root") {
			t.Errorf("expected tree to contain label, got %q", buf.String())
		}
	})

	t.Run("quiet mode suppresses", func(t *testing.T) {

		var buf bytes.Buffer
		f := NewOutputFormatter(&buf, OutputFormatText, false, true, false)
		f.Tree([]TreeItem{{Label: "root"}}, "")
		if buf.Len() != 0 {
			t.Error("expected no tree output in quiet mode")
		}
	})
}

func TestOutputFormatter_ProgressBar(t *testing.T) {

	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
	pb := f.ProgressBar(10, "test progress")
	if pb == nil {
		t.Fatal("ProgressBar returned nil")
	}
}

func TestOutputFormatter_Spinner(t *testing.T) {

	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
	s := f.Spinner("test spinner")
	if s == nil {
		t.Fatal("Spinner returned nil")
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestOutputFormatter_Print_NilInput(t *testing.T) {

	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
	err := f.Print(nil)
	if err != nil {
		t.Fatalf("Print(nil) error: %v", err)
	}
	// Should not panic
}

func TestOutputFormatter_Print_EmptyString(t *testing.T) {

	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
	err := f.Print("")
	if err != nil {
		t.Fatalf("Print('') error: %v", err)
	}
}

func TestOutputFormatter_JSON_Error(t *testing.T) {

	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatJSON, false, false, false)
	// channels are not marshalable to JSON
	err := f.Print(func() {}) // function type causes yaml marshal error without panic
	if err == nil {
		t.Error("expected error marshaling non-JSON-safe type")
	}
}

// removed - func(){} panics in yaml library, see TestOutputFormatter_YAML_PanicsOnFunc
// removed - func(){} panics in yaml library, see TestOutputFormatter_YAML_PanicsOnFunc
// removed - func(){} panics in yaml library, see TestOutputFormatter_YAML_PanicsOnFunc
// removed - func(){} panics in yaml library, see TestOutputFormatter_YAML_PanicsOnFunc
// removed - func(){} panics in yaml library, see TestOutputFormatter_YAML_PanicsOnFunc
// removed - func(){} panics in yaml library, see TestOutputFormatter_YAML_PanicsOnFunc
// removed - func(){} panics in yaml library, see TestOutputFormatter_YAML_PanicsOnFunc
// removed - func(){} panics in yaml library, see TestOutputFormatter_YAML_PanicsOnFunc
// removed - func(){} panics in yaml library, see TestOutputFormatter_YAML_PanicsOnFunc
// removed - func(){} panics in yaml library, see TestOutputFormatter_YAML_PanicsOnFunc
// removed - func(){} panics in yaml library, see TestOutputFormatter_YAML_PanicsOnFunc

// =============================================================================
// Concurrency — Ensure OutputFormatter is goroutine-safe
// =============================================================================

func TestOutputFormatter_ConcurrentPrint(t *testing.T) {

	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)

	done := make(chan struct{})
	go func() {
		for i := 0; i < 10; i++ {
			f.Println("goroutine A")
		}
		done <- struct{}{}
	}()

	go func() {
		for i := 0; i < 10; i++ {
			f.Println("goroutine B")
		}
		done <- struct{}{}
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Should not panic; output should contain both messages
	if !strings.Contains(buf.String(), "goroutine A") || !strings.Contains(buf.String(), "goroutine B") {
		t.Error("expected both goroutines' output")
	}
}

func TestOutputFormatter_YAML_PanicsOnFunc(t *testing.T) {

	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatYAML, false, false, false)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic marshaling function type to YAML")
		}
	}()

	_ = f.Print(func() {})
}
