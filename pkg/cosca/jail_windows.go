//go:build windows

/*
Implementação Windows do auto-jail. Não existe bubblewrap (bwrap) no
Windows — a jaula segue o MESMO contrato fail-closed de jail.go:
jailAvailable() reporta indisponível e ReexecInJail() cai no jailFallback,
que emite o SECURITY WARNING + grava no security log e só roda sem sandbox
com COSCA_ALLOW_NO_ROOT=1 (opt-in explícito). NUNCA tenta rodar bwrap.
*/

package cosca

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// jailAvailable decide se a jaula PODE ser estabelecida no ambiente atual.
// No Windows o bubblewrap (bwrap) não existe — a jaula nunca está disponível
// e a decisão fail-closed segue pelo jailFallback.
func jailAvailable() (bool, string) {
	return false, "bubblewrap (bwrap) is not supported on Windows"
}

// ReexecInJail no Windows não tenta rodar bwrap (inexistente). Segue o
// contrato fail-closed: imprime SECURITY WARNING em stderr + grava no
// security log; com COSCA_ALLOW_NO_ROOT=1 (opt-in explícito) retorna e o
// processo roda sem sandbox. Sem o opt-in, exit 1.
func ReexecInJail() {
	if !jailFallback("bubblewrap is not available on Windows") {
		os.Exit(1)
	}
	// Opt-in explícito: segue sem sandbox.
}

// jailGeteuid não tem significado no Windows (não existe euid). Nunca root:
// sem bwrap não há o risco de jail anulável por root-nullable user namespace.
var jailGeteuid = func() int { return -1 }

// setupSignalForwarding propaga sinais do terminal para o processo filho.
// No Windows não existem grupos de processo (syscall.Getpgid/Kill são
// Unix-only) — a propagação é direta via cmd.Process.Signal.
func setupSignalForwarding(cmd *exec.Cmd) {
	sigCh := make(chan os.Signal, 8)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		for sig := range sigCh {
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		}
	}()
}
