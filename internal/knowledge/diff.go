// Knowledge Diff — o que mudou entre versões de um Knowledge Package.
//
// O Cosca sabe o que mudou no conhecimento que ele já tinha: o manifesto
// guarda VersionDiffs (a fonte determinística do diff, registrada por um
// humano ou pelo pipeline de aquisição), e o diff cruza com os KnowledgeItems
// do CKL para responder "isso me afeta?".
//
// Conforme o Don: "Prisma 6.0 → 6.1: NEW + recurso X, CHANGED ~ comportamento
// Z, DEPRECATED - API A, REMOVED - API B, RISK ⚠️ 3 padrões existentes podem
// ser afetados, AFFECTED KNOWLEDGE K-182 K-219 K-441. E dá para cruzar com o
// projeto: 'Chef, a atualização altera uma API utilizada em 4 pontos do
// projeto. Não atualizei nada. Preparei a análise.'"
//
// Determinístico, sem LLM: Diff consulta a tabela VersionDiffs; AssessRisk
// cruza símbolos com o texto dos itens; ProjectImpact conta ocorrências.
package knowledge

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Kind do DiffEntry.
const (
	DiffKindNew        = "new"        // "+ recurso X"
	DiffKindChanged    = "changed"    // "~ comportamento Z"
	DiffKindDeprecated = "deprecated" // "- API A (ainda existe, mas é legado)"
	DiffKindRemoved    = "removed"    // "- API B (não existe mais)"
)

// Níveis de risco do KnowledgeDiff (escalada pelo número de itens afetados):
// 0 → none, 1-2 → low, 3-5 → medium, >5 → high.
const (
	RiskNone   = "none"
	RiskLow    = "low"
	RiskMedium = "medium"
	RiskHigh   = "high"
)

// DiffEntry é uma mudança pontual entre duas versões do pacote.
type DiffEntry struct {
	Kind   string `json:"kind"`   // "new" | "changed" | "deprecated" | "removed"
	Symbol string `json:"symbol"` // "API X", "schema.migrations", ...
	Detail string `json:"detail"` // contexto em texto livre (pt-BR)
}

// KnowledgeDiff é o resultado da comparação de duas versões, já cruzado com
// o conhecimento (risco) e com o projeto (impacto). Summary é em pt-BR:
// "3 padrões existentes podem ser afetados" quando há itens afetados.
type KnowledgeDiff struct {
	PackageID         string      `json:"package_id"`
	FromVersion       string      `json:"from_version"`
	ToVersion         string      `json:"to_version"`
	Entries           []DiffEntry `json:"entries"`
	RiskLevel         string      `json:"risk_level"`         // "none"|"low"|"medium"|"high"
	AffectedKnowledge []string    `json:"affected_knowledge"` // ["K-182", "K-219", "K-441"]
	ProjectImpact     int         `json:"project_impact"`     // pontos no projeto afetados
	Summary           string      `json:"summary"`            // pt-BR
}

// versionDiffKey monta a chave canônica de VersionDiffs ("6.0->6.1").
func versionDiffKey(from, to string) string {
	return from + "->" + to
}

// negateKind devolve o kind do diff no sentido reverso: o que era novo vira
// removido e vice-versa; changed e deprecated não são simétricos (o símbolo
// continua existindo nos dois sentidos).
func negateKind(kind string) string {
	switch kind {
	case DiffKindNew:
		return DiffKindRemoved
	case DiffKindRemoved:
		return DiffKindNew
	default:
		return kind
	}
}

// Diff devolve o KnowledgeDiff entre from e to a partir do VersionDiffs do
// manifesto (a fonte determinística). Chave exata "from->to" tem prioridade;
// se apenas o caminho reverso "to->from" estiver registrado, ele é usado com
// os kinds negados (new↔removed). Nenhum diff registrado ⇒ diff vazio com
// Summary "nenhuma mudança registrada entre from e to" — nunca um erro.
func (p *KnowledgePackage) Diff(from, to string) (*KnowledgeDiff, error) {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from == "" || to == "" {
		return nil, errors.New("knowledge: from e to são obrigatórios (ex.: \"6.0\" \"6.1\")")
	}
	if from == to {
		return nil, fmt.Errorf("knowledge: versões iguais (%q) — o diff precisa de duas versões distintas", from)
	}

	d := &KnowledgeDiff{
		PackageID:   p.ID,
		FromVersion: from,
		ToVersion:   to,
		RiskLevel:   RiskNone,
	}

	entries, ok := p.VersionDiffs[versionDiffKey(from, to)]
	if !ok {
		// Caminho reverso registrado → negar os kinds (new↔removed).
		if rev, ok := p.VersionDiffs[versionDiffKey(to, from)]; ok {
			entries = make([]DiffEntry, len(rev))
			for i, e := range rev {
				entries[i] = DiffEntry{Kind: negateKind(e.Kind), Symbol: e.Symbol, Detail: e.Detail}
			}
		}
	}

	d.Entries = entries
	if len(entries) == 0 {
		d.Summary = fmt.Sprintf("nenhuma mudança registrada entre %s e %s", from, to)
	}
	return d, nil
}

// itemReferencesPackage diz se um KnowledgeItem referencia o pacote: pelo id
// no título ou em alguma evidência (source/description). Best-effort, sem LLM.
func itemReferencesPackage(it *KnowledgeItem, pkgID string) bool {
	needle := strings.ToLower(pkgID)
	if needle == "" {
		return false
	}
	if strings.Contains(strings.ToLower(it.Title), needle) {
		return true
	}
	for _, ev := range it.Evidence {
		if strings.Contains(strings.ToLower(ev.Description), needle) ||
			strings.Contains(strings.ToLower(ev.Source), needle) {
			return true
		}
	}
	return false
}

// itemMentionsSymbol diz se o item menciona um símbolo do diff (título ou
// alguma evidência). Best-effort, sem LLM.
func itemMentionsSymbol(it *KnowledgeItem, symbol string) bool {
	needle := strings.ToLower(strings.TrimSpace(symbol))
	if needle == "" {
		return false
	}
	if strings.Contains(strings.ToLower(it.Title), needle) {
		return true
	}
	for _, ev := range it.Evidence {
		if strings.Contains(strings.ToLower(ev.Description), needle) ||
			strings.Contains(strings.ToLower(ev.Source), needle) {
			return true
		}
	}
	return false
}

// AssessRisk cruza o diff com os KnowledgeItems que referenciam o pacote:
// todo item que menciona um símbolo afetado (changed/deprecated/removed —
// "new" não afeta conhecimento existente) entra em AffectedKnowledge. O risco
// escala pelo número de itens afetados: 0 → none, 1-2 → low, 3-5 → medium,
// >5 → high. Determinístico: itens em ordem, AffectedKnowledge ordenado e
// deduplicado, sem LLM. Não muta o argumento; devolve um KnowledgeDiff novo.
func (p *KnowledgePackage) AssessRisk(diff *KnowledgeDiff, items []KnowledgeItem) KnowledgeDiff {
	out := KnowledgeDiff{
		PackageID:         diff.PackageID,
		FromVersion:       diff.FromVersion,
		ToVersion:         diff.ToVersion,
		Entries:           diff.Entries,
		RiskLevel:         diff.RiskLevel,
		AffectedKnowledge: diff.AffectedKnowledge,
		ProjectImpact:     diff.ProjectImpact,
		Summary:           diff.Summary,
	}

	seen := make(map[string]bool)
	var affected []string
	for i := range items {
		it := &items[i]
		if !itemReferencesPackage(it, p.ID) {
			continue
		}
		for _, e := range diff.Entries {
			if e.Kind == DiffKindNew {
				continue
			}
			if itemMentionsSymbol(it, e.Symbol) {
				if !seen[it.ID] {
					seen[it.ID] = true
					affected = append(affected, it.ID)
				}
				break
			}
		}
	}
	sort.Strings(affected)

	out.AffectedKnowledge = affected
	switch n := len(affected); {
	case n == 0:
		out.RiskLevel = RiskNone
	case n <= 2:
		out.RiskLevel = RiskLow
	case n <= 5:
		out.RiskLevel = RiskMedium
	default:
		out.RiskLevel = RiskHigh
	}
	if len(affected) > 0 {
		out.Summary = fmt.Sprintf("%d padrões existentes podem ser afetados", len(affected))
	}
	return out
}

// ProjectImpact conta quantos símbolos changed/removed do diff aparecem na
// lista de dependências do projeto (ou em itens relacionados, best-effort).
// Cada símbolo conta uma vez. Determinístico, sem LLM.
func (p *KnowledgePackage) ProjectImpact(diff *KnowledgeDiff, dependencies []string) int {
	matched := 0
	for _, e := range diff.Entries {
		if e.Kind != DiffKindChanged && e.Kind != DiffKindRemoved {
			continue
		}
		needle := strings.ToLower(strings.TrimSpace(e.Symbol))
		if needle == "" {
			continue
		}
		for _, dep := range dependencies {
			if strings.Contains(strings.ToLower(dep), needle) {
				matched++
				break
			}
		}
	}
	return matched
}
