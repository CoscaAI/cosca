package cli

import (
	"fmt"
	"os"

	"github.com/CoscaAI/cosca/internal/proposal"
)

// GuardedRemoveAll é a trava P0 (política geral FAIL-CLOSED) para operações
// destrutivas do CLI sobre alvos protegidos do Contrato de Autoridade.
//
// Qualquer RemoveAll/rm cujo alvo pertença a uma zona protegida
// (FROZEN=internal/embed/cosca, LIVE=.opencode/cosca, RUNTIME=.cosca) é
// RECUSADO: nada é executado e um erro é devolvido. Assim o código não precisa
// corrigir um RemoveAll por vez — um único ponto de guarda amarra a política.
//
// Para alvos livres (fora das zonas protegidas), delega para os.RemoveAll,
// preservando o comportamento original.
func GuardedRemoveAll(path string) error {
	if zone := proposal.ProtectedZoneOf(path); zone != "" {
		return fmt.Errorf(
			"FAIL-CLOSED: remoção destrutiva bloqueada na zona de autoridade %s (%s) — "+
				"o contrato de autoridade proíbe deletar/conteúdo-destruir alvos protegidos",
			zone, path)
	}
	return os.RemoveAll(path)
}
