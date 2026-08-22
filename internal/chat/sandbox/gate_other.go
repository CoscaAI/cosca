//go:build !linux

package sandbox

import (
	"context"
	"errors"
	"os/exec"

	"github.com/CoscaAI/cosca/internal/chat"
)

// findBwrap returns an empty string because bwrap is not available
// on non-Linux platforms.
func findBwrap() string {
	return ""
}

func (g *Gate) bwrapCommand(_ context.Context, _ chat.Command, _ chat.SandboxMode) (*exec.Cmd, error) {
	return nil, errors.New("sandbox: bwrap is not supported on this platform")
}

// execBwrap is not supported on non-Linux platforms. This method exists
// solely to satisfy compilation on all platforms; it is never called
// because findBwrap() returns "" and the Execute method always falls back
// to execDirect or execReadOnly.
func (g *Gate) execBwrap(_ context.Context, _ chat.Command, _ chat.SandboxMode) (*chat.SandboxResult, error) {
	return nil, errors.New("sandbox: bwrap is not supported on this platform")
}
