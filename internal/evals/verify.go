package evals

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// maxVerifyTail caps how much of a verify command's output is retained in a
// report so huge build logs do not bloat the JSON payload.
const maxVerifyTail = 600

// VerifyResult is the outcome of a single verify command.
type VerifyResult struct {
	Command    string `json:"command"`
	OK         bool   `json:"ok"`
	OutputTail string `json:"output_tail,omitempty"`
	Error      string `json:"error,omitempty"`
}

// RunVerifyCommands executes each command through `sh -c` in dir. Every
// command gets its own timeout derived from the parent context. A command
// passes when it exits 0; otherwise it fails with a tail of its output.
func RunVerifyCommands(ctx context.Context, dir string, commands []string, timeout time.Duration) []VerifyResult {
	results := make([]VerifyResult, 0, len(commands))
	for _, cmdline := range commands {
		cmdline = strings.TrimSpace(cmdline)
		if cmdline == "" {
			continue
		}

		vctx, cancel := context.WithTimeout(ctx, timeout)
		cmd := exec.CommandContext(vctx, "sh", "-c", cmdline)
		cmd.Dir = dir
		configureProcessGroup(cmd)
		out, err := cmd.CombinedOutput()
		cancel()

		vr := VerifyResult{Command: cmdline, OutputTail: tail(string(out), maxVerifyTail)}
		if err != nil {
			vr.OK = false
			switch {
			case vctx.Err() == context.DeadlineExceeded:
				vr.Error = "verify command exceeded timeout"
			case vctx.Err() == context.Canceled:
				vr.Error = "verify command canceled"
			default:
				vr.Error = err.Error()
			}
		} else {
			vr.OK = true
		}
		results = append(results, vr)
	}
	return results
}

// tail returns the last n runes of s, or the whole string when shorter.
func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
