package tool

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/safe"
)

// SandboxTool executes code in an isolated environment.
// Uses E2B code interpreter when available; falls back to local sandbox.
type SandboxTool struct {
	e2bAPIKey string
	useLocal  bool
}

// NewSandboxTool creates a code sandbox tool.
func NewSandboxTool(e2bAPIKey string) *SandboxTool {
	if e2bAPIKey == "" {
		e2bAPIKey = os.Getenv("E2B_API_KEY")
	}
	return &SandboxTool{
		e2bAPIKey: e2bAPIKey,
		useLocal:  e2bAPIKey == "", // fallback to local when no E2B key
	}
}

func (t *SandboxTool) Name() string { return "sandbox" }
func (t *SandboxTool) Description() string {
	return "Execute code in an isolated sandbox. Supports Python, Go, and shell."
}
func (t *SandboxTool) IsAvailable() bool { return true } // always available (local fallback)

// Execute runs code in the sandbox.
// Input format: "language\ncode" where language is python, go, or sh.
func (t *SandboxTool) Execute(ctx context.Context, input string) (string, error) {
	parts := strings.SplitN(input, "\n", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("sandbox: input must be 'language\\ncode'")
	}
	language := strings.TrimSpace(parts[0])
	code := parts[1]

	if !t.useLocal {
		return t.executeE2B(ctx, language, code)
	}
	return t.executeLocal(ctx, language, code)
}

// executeLocal runs code in a local subprocess with a timeout.
func (t *SandboxTool) executeLocal(ctx context.Context, language, code string) (string, error) {
	var cmd *exec.Cmd

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	switch strings.ToLower(language) {
	case "python", "py":
		cmd = exec.CommandContext(ctx, "python3", "-c", code)
	case "go":
		// Write to temp file and run
		tmpFile, err := os.CreateTemp("", "cosca-sandbox-*.go")
		if err != nil {
			return "", err
		}
		defer os.Remove(tmpFile.Name())
		if _, err := tmpFile.WriteString(code); err != nil {
			return "", err
		}
		tmpFile.Close()
		cmd = exec.CommandContext(ctx, "go", "run", tmpFile.Name())
	case "sh", "bash", "shell":
		var err error
		var tmpFile string
		cmd, tmpFile, err = safe.SafeShellExec(ctx, code)
		if err != nil {
			return "", err
		}
		defer os.Remove(tmpFile)
	default:
		return "", fmt.Errorf("sandbox: unsupported language: %s (use python, go, or sh)", language)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Error:\n%s\n\nOutput:\n%s", err.Error(), string(output)), nil
	}
	return string(output), nil
}

// executeE2B runs code in the E2B cloud sandbox.
func (t *SandboxTool) executeE2B(ctx context.Context, language, code string) (string, error) {
	// TODO: Implement E2B API integration
	// https://e2b.dev/docs
	return "", fmt.Errorf("tool sandbox: not implemented (stub) — integrate E2B before use")
}
