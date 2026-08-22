//go:build windows

package safe

import (
	"context"
	"os"
	"os/exec"
)

// SafeShellExec writes code to a temp file and returns a command to execute it
// with cmd.exe. The caller should defer os.Remove(tmpFile) to clean up.
//
// This avoids command injection via `cmd /c <code>` by ensuring the code is
// never interpolated into a shell command string.
func SafeShellExec(ctx context.Context, code string) (*exec.Cmd, string, error) {
	tmpFile, err := os.CreateTemp("", "cosca-script-*.cmd")
	if err != nil {
		return nil, "", err
	}
	if _, err := tmpFile.WriteString("@echo off\r\n" + code); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, "", err
	}
	tmpFile.Close()
	return exec.CommandContext(ctx, "cmd.exe", "/c", tmpFile.Name()), tmpFile.Name(), nil
}
