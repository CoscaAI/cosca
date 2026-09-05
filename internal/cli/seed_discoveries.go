//
// SEED inicial do Hall da Fama do Conhecimento — as 5 grandes descobertas
// REAIS do Cosca, com impacto medido.
//
// Cada descoberta usa evidências reais do repositório: os testes de
// segurança da jail (TestJail_FailClosed, TestJail_RootUnsafe,
// TestJail_DieWithParent), as auditorias red team (onda-1: A1, C1+C2;
// onda-2: A4) e a conversa do Don que criou o CKL. É o "o que isso mudou?"
// do Cosca: o impacto vem de medição (redução de risco, sessões validadas),
// nunca de opinião (Princípio 3 do CKL).
//
// O seed roda uma única vez: na primeira execução de `cosca knowledge
// discovery list`, quando .cosca/knowledge/hall-of-fame.json ainda não
// existe. É idempotente por construção — um arquivo existente nunca é
// sobrescrito. Usa apenas as assinaturas existentes do HallOfFame
// (Register, Save, Load). NUNCA toca em .cosca/framework.
//

package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/CoscaAI/cosca/internal/knowledge"
)

// ensureSeededHallOfFame garante que o arquivo runtime do Hall da Fama
// exista. Na primeira execução (arquivo inexistente), cria-o com as 5
// descobertas seed via knowledge.SeedHallOfFame + HallOfFame.Save.
// Retorna true se o seed foi aplicado.
//
// Idempotente: um arquivo existente — mesmo vazio — nunca é sobrescrito.
func ensureSeededHallOfFame(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("stat %s: %w", path, err)
	}

	h := knowledge.NewHallOfFame()
	if err := knowledge.SeedHallOfFame(h); err != nil {
		return false, err
	}
	if err := h.Save(path); err != nil {
		return false, fmt.Errorf("salvar seed em %s: %w", path, err)
	}
	return true, nil
}
