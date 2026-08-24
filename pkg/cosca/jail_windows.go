//go:build windows

/*
Implementação Windows do auto-jail. Não existe bubblewrap (bwrap) no
Windows — a jaula segue o MESMO contrato fail-closed de jail.go:
jailAvailable() reporta indisponível e ReexecInJail() cai no jailFallback,
que emite o SECURITY WARNING + grava no security log e — SEM o opt-in
explícito — NEGOA a execução (exit 1). SÓ segue sem sandbox com
COSCA_ALLOW_NO_ROOT=1 (opt-in explícito). NUNCA tenta rodar bwrap.

POSTURA (segurança): COSCA_ALLOW_NO_ROOT é um escape-hatch de
responsabilidade EXCLUSIVA do operador e NUNCA deve ser injetado por
default por launcher/script instalado — fazê-lo anula o fail-closed e
transforma o escape-hatch na postura default (a "bomba" do Windows).
Sem bwrap não há isolamento real de FS/rede/recurso: agente/workload
roda com o privilégio do processo. Para código não confiável o caminho
seguro é rodar dentro do Cofre/WSL2+bwrap (zona air-gap), nunca no host.
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
// contrato fail-closed DE VERDADE: imprime SECURITY WARNING em stderr +
// grava no security log; com COSCA_ALLOW_NO_ROOT=1 (opt-in explícito)
// retorna e o processo roda sem sandbox (ainda com alerta). SEM o opt-in,
// o processo é NEGADO (exit 1) — agente/workload nunca roda sem sandbox
// apenas "com um aviso". A responsabilidade do opt-in é do operador; nunca
// deve vir de um default de script/launcher.
func ReexecInJail() {
	if !jailFallback("bubblewrap is not available on Windows") {
		os.Exit(1)
	}
	// Opt-in explícito: segue sem sandbox (com o alerta já emitido).
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
