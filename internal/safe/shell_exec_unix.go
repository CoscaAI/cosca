//go:build !windows

package safe

import (
	"context"
	"os"
	"os/exec"
)

// SafeShellExec writes code to a temp file and returns a command to execute it
// with bash. The caller should defer os.Remove(tmpFile) to clean up.
//
// This avoids command injection via `bash -c <code>` by ensuring the code is
// never interpolated into a shell command string.
func SafeShellExec(ctx context.Context, code string) (*exec.Cmd, string, error) {
	tmpFile, err := os.CreateTemp("", "cosca-script-*.sh")
	if err != nil {
		return nil, "", err
	}
	if _, err := tmpFile.WriteString(code); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, "", err
	}
	tmpFile.Close()
	return exec.CommandContext(ctx, "bash", tmpFile.Name()), tmpFile.Name(), nil
}
