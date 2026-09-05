//go:build windows

package cli

import "os/exec"

// detachDesktopProcess é no-op no Windows: não existe setsid (a separação de
// sessão/console do filho seria via CREATE_NEW_PROCESS_GROUP/CREATE_NEW_CONSOLE,
// fora do escopo do Cosca no Windows). O processo filho roda no mesmo console.
func detachDesktopProcess(_ *exec.Cmd) {}
