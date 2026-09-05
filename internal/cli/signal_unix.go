//go:build unix

package cli

import "syscall"

// signalProcess envia um sinal POSIX ao processo com o PID dado.
func signalProcess(pid int, sig syscall.Signal) error {
	return syscall.Kill(pid, sig)
}

// processAlive reports whether the process with the given PID is running.
// syscall.Kill(pid, 0) retorna nil quando o processo existe; EPERM tambem
// significa que ele existe (apenas sem permissao de sinal); ESRCH e morto.
func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	return err == syscall.EPERM
}
