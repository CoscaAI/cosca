//
// Helpers ADD-ON para expor o estado epistemológico (CKL) via REST.
//
// O knowledge.Engine indexa e busca documentos; as leis do CKL vivem no
// PromotionEngine persistido em .cosca/knowledge/laws.json. Estes helpers
// listam os itens desse arquivo runtime — espelhando o caminho de carga do
// CLI `cosca knowledge status` (resolveLawsPath + loadLawsEngine) — sem
// alterar o PromotionEngine nem o knowledge.Engine existentes.
//

package knowledge

import (
	"errors"
	"os"
	"path/filepath"
)

// ProjectLawsPath resolve o caminho do arquivo runtime de leis do CKL para o
// diretório de trabalho atual (<cwd>/.cosca/knowledge/laws.json) — o mesmo
// que o CLI usa. Sem Getwd (caso raro), cai no DefaultLawsPath (home-based).
func ProjectLawsPath() string {
	dir, err := os.Getwd()
	if err != nil {
		return DefaultLawsPath()
	}
	return filepath.Join(dir, ".cosca", "knowledge", "laws.json")
}

// ListKnowledgeItems carrega todos os itens de conhecimento do arquivo JSON
// em path (o laws.json do CKL). Arquivo inexistente → lista vazia, sem erro
// (paridade com o CLI). Devolve cópias (PromotionEngine.All) ordenadas por ID.
// Aditivo — nenhum método existente é alterado.
func ListKnowledgeItems(path string) ([]KnowledgeItem, error) {
	engine := NewPromotionEngine()
	if err := engine.Load(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	all := engine.All()
	items := make([]KnowledgeItem, 0, len(all))
	for _, it := range all {
		if it == nil {
			continue
		}
		items = append(items, *it)
	}
	return items, nil
}
