// Knowledge Matching — Projeto → Dependency Detection → Knowledge Matching.
//
// Conforme o Don: "Quando o agente estiver trabalhando num projeto novo, o
// sistema detecta o package.json, go.mod, Cargo.toml, etc., e faz: Projeto →
// Dependency Detection → Knowledge Matching → 'Tenho conhecimento verificado?'
// → SIM → usa / STALE → revalida / NÃO → oferece aquisição. Aí adicionar
// Prisma, Zod, Gin, Fiber, Tokio etc. deixa de ser desenvolver uma feature
// para cada biblioteca."
//
// Determinístico e local (sem LLM): cada dependência do projeto é normalizada
// (NormalizeDependencyName) e procurada na PackageStore. O resultado é um
// DependencyMatch por dependência com o status epistemológico
// (verified|stale|partial|missing) e a ação sugerida em pt-BR (Suggest).
// NUNCA instala nada — apenas diz o que o Cosca sabe (e sabe que NÃO sabe)
// sobre cada dependência do projeto.
package knowledge

import (
	"errors"
	"strings"
)

// ── Status do match ─────────────────────────────────────────────────────────

// MatchStatus é o status epistemológico do match de uma dependência.
type MatchStatus string

const (
	MatchVerified MatchStatus = "verified" // conhecimento presente e validado
	MatchStale    MatchStatus = "stale"    // presente mas requer revalidação
	MatchMissing  MatchStatus = "missing"  // sem conhecimento
	MatchPartial  MatchStatus = "partial"  // manifesto existe, sem validação
)

// DependencyMatch é o resultado do match de uma dependência do projeto contra
// o conhecimento do Cosca. Note é em pt-BR ("conhecimento verificado",
// "requer revalidação", "sem conhecimento — ofereça aquisição", ...).
type DependencyMatch struct {
	Dependency     string      `json:"dependency"`
	PackageID      string      `json:"package_id,omitempty"`
	Status         MatchStatus `json:"status"`
	KnowledgeLevel string      `json:"knowledge_level,omitempty"` // none|partial|validated
	Note           string      `json:"note,omitempty"`            // pt-BR
}

// noteFor devolve o texto pt-BR do status do match.
func noteFor(s MatchStatus) string {
	switch s {
	case MatchVerified:
		return "conhecimento verificado"
	case MatchStale:
		return "requer revalidação"
	case MatchPartial:
		return "manifesto existe, sem validação"
	case MatchMissing:
		return "sem conhecimento — ofereça aquisição"
	}
	return ""
}

// ── Normalização de nomes de dependência ────────────────────────────────────

// isAllDigits devolve true quando todos os caracteres são dígitos.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// NormalizeDependencyName normaliza o nome de uma dependência para o lookup na
// PackageStore — minúsculas, sem versões/ranges/scopes (best-effort):
//
//	"prisma"                 → "prisma"
//	"prisma@6"               → "prisma"
//	"@types/react"           → "react"      (scope npm removido)
//	"Express"                → "express"
//	"github.com/gin-gonic/gin" → "gin"      (último segmento de caminho)
//	"github.com/foo/bar/v2"  → "bar"        (sufixo de versão de módulo Go)
//	"pgx/v5"                 → "pgx"
//
// Regras pragmáticas, na ordem:
//  1. lowercase + trim.
//  2. prefixo de scope npm "@scope/..." → vira caminho comum (o scope é
//     descartado junto com o último segmento adiante).
//  3. primeiro marcador de versão ("@", "<", ">", "=", "~", "^", "[") corta o
//     resto ("react@^18.2" → "react"; "requests>=2.0" → "requests").
//  4. sufixo de versão de módulo Go "/vN" (N numérico) é removido.
//  5. último segmento de caminho vira o nome ("org/lib" → "lib").
func NormalizeDependencyName(dep string) string {
	s := strings.TrimSpace(strings.ToLower(dep))
	if s == "" {
		return ""
	}

	// Scope npm: "@types/react" → "types/react" (o scope cai no passo 5).
	if strings.HasPrefix(s, "@") {
		s = s[1:]
	}

	// Versão/range: corta no primeiro marcador de versão.
	for _, sep := range []byte{'@', '<', '>', '=', '~', '^', '['} {
		if i := strings.IndexByte(s, sep); i >= 0 {
			s = s[:i]
			break
		}
	}

	// Módulo Go com sufixo de versão: "github.com/foo/bar/v2" → bar.
	if i := strings.LastIndex(s, "/v"); i >= 0 {
		if isAllDigits(s[i+2:]) {
			s = s[:i]
		}
	}

	// Último segmento de caminho: "github.com/gin-gonic/gin" → "gin".
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}

	// gopkg.in: "yaml.v3" → "yaml" (sufixo ".vN" de versão no nome).
	if i := strings.LastIndex(s, ".v"); i >= 0 {
		if isAllDigits(s[i+2:]) {
			s = s[:i]
		}
	}

	return s
}

// ── Match ───────────────────────────────────────────────────────────────────

// MatchDependency devolve o status epistemológico do match de uma dependência
// contra um KnowledgePackage:
//
//	pkg nil                                        → Missing
//	Status/KL "stale"/"STALE" (aging)              → Stale (override: mesmo um
//	                                                 pacote já validado que
//	                                                 envelheceu DEVE ser
//	                                                 revalidado, não usado)
//	Status validated OU KnowledgeLevel validated   → Verified
//	qualquer outro estado do manifesto             → Partial
func MatchDependency(_ string, pkg *KnowledgePackage) MatchStatus {
	if pkg == nil {
		return MatchMissing
	}
	switch {
	case strings.EqualFold(pkg.Status, "stale") || strings.EqualFold(pkg.KnowledgeLevel, "stale"):
		return MatchStale
	case pkg.Status == PackageStatusValidated || pkg.KnowledgeLevel == PackageKnowledgeValidated:
		return MatchVerified
	default:
		return MatchPartial
	}
}

// MatchProject cruza a lista de dependências do projeto com a PackageStore:
// cada dependência é normalizada e procurada (primeiro o nome normalizado,
// depois o nome cru) para decidir o status. Nunca instala nada — apenas
// classifica. store é a store local (obrigatória). Se global for fornecida,
// é consultada como fallback quando o package não está na store local.
func MatchProject(deps []string, store *PackageStore, global *PackageStore) ([]DependencyMatch, error) {
	if store == nil {
		return nil, errors.New("knowledge: PackageStore nil")
	}

	// lookup procura um package pelo nome normalizado e cru, primeiro na
	// store local, depois na global (se fornecida).
	lookup := func(name string) *KnowledgePackage {
		if normalized := NormalizeDependencyName(name); normalized != "" {
			if pkg, _ := store.Get(normalized); pkg != nil {
				return pkg
			}
			if global != nil {
				if pkg, _ := global.Get(normalized); pkg != nil {
					return pkg
				}
			}
		}
		if raw := strings.TrimSpace(name); raw != "" && raw != NormalizeDependencyName(name) {
			if pkg, _ := store.Get(raw); pkg != nil {
				return pkg
			}
			if global != nil {
				if pkg, _ := global.Get(raw); pkg != nil {
					return pkg
				}
			}
		}
		return nil
	}

	matches := make([]DependencyMatch, 0, len(deps))
	for _, dep := range deps {
		m := DependencyMatch{
			Dependency:     dep,
			KnowledgeLevel: PackageKnowledgeNone,
			Status:         MatchMissing,
		}

		pkg := lookup(dep)

		if pkg == nil {
			m.Note = noteFor(MatchMissing)
			matches = append(matches, m)
			continue
		}

		m.PackageID = pkg.ID
		m.KnowledgeLevel = pkg.KnowledgeLevel
		m.Status = MatchDependency(dep, pkg)
		m.Note = noteFor(m.Status)
		matches = append(matches, m)
	}
	return matches, nil
}

// Suggest devolve a ação sugerida em pt-BR para o status do match. Verified
// não exige ação (string vazia).
func (m DependencyMatch) Suggest() string {
	switch m.Status {
	case MatchMissing:
		dep := NormalizeDependencyName(m.Dependency)
		if dep == "" {
			dep = m.Dependency
		}
		return "ofereça: cosca knowledge add " + dep
	case MatchStale:
		return "revalide: cosca knowledge revalidate"
	case MatchPartial:
		return "complete a aquisição"
	default:
		return ""
	}
}
