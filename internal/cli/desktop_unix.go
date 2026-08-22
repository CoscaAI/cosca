//go:build unix

package cli

import (
	"os/exec"
	"syscall"
)

// detachDesktopProcess coloca o processo filho do desktop em uma nova sessão
// (setsid): o cosca-desktop sobrevive ao fechamento do terminal que o iniciou.
func detachDesktopProcess(proc *exec.Cmd) {
	proc.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
