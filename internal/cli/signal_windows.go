//go:build windows

package cli

import (
	"errors"
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

// signalProcess envia um sinal ao processo com o PID dado. O Windows não tem
// sinais POSIX: o único sinal implementado pelo pacote os é os.Kill
// (TerminateProcess) — os demais (ex: SIGTERM) são mapeados para terminação
// forçada. Não existe graceful shutdown por sinal no Windows.
// ErrProcessDone (o processo já saiu entre a checagem e o sinal) é tratado
// como sucesso: o objetivo (processo não está mais rodando) já foi atingido.
func signalProcess(pid int, sig syscall.Signal) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := p.Signal(os.Kill); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		return err
	}
	return nil
}

// processAlive reports whether the process with the given PID is running.
// No Windows não existe o probe kill(pid, 0); usamos OpenProcess com
// PROCESS_QUERY_LIMITED_INFORMATION (sem privilégio de administrador) +
// GetExitCodeProcess. Best-effort documentado:
//   - ERROR_ACCESS_DENIED: o processo EXISTE (apenas sem permissão de query)
//     → vivo (equivalente ao EPERM do Unix);
//   - OpenProcess falhou por outro motivo: provavelmente não existe → morto;
//   - exit code == STATUS_PENDING (259): ainda rodando;
//   - exit code != 259: o processo já terminou.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return errors.Is(err, windows.ERROR_ACCESS_DENIED)
	}
	defer windows.CloseHandle(handle)
	var exitCode uint32
	if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
		return true // handle válido → o processo existe
	}
	return exitCode == uint32(windows.STATUS_PENDING)
}
