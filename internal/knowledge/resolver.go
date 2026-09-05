// Knowledge Resolver — Tarefa → Knowledge Resolver → Gap Detection.
//
// Conforme o Don:
//
//	Tarefa → Knowledge Resolver → 'Tenho conhecimento suficiente?' → SIM →
//	continua / NÃO → Gap Detection → Knowledge Acquisition → Sources →
//	Evidence → Validation → Knowledge → continua a tarefa.
//
// "O agente recebe 'Implemente X usando Prisma 6.1'. O resolver verifica:
// Projeto: Prisma 6.1, Conhecimento: Prisma 6.0 ✓, Prisma 6.1 ✗, API X ? →
// Knowledge gap detected: Missing Prisma 6.1 / API X."
//
// Determinístico e local (sem rede, sem LLM — o wire de aquisição vem depois):
// ResolveTask cruza as dependências do projeto com a PackageStore, decide
// SUFFICIENT ou produz KnowledgeGaps (com a razão em pt-BR e a cascata de
// fontes a usar), e VoiceSummary gera a mensagem 🔊. SuggestSources documenta
// a cascata de 8 níveis que o wire de aquisição percorrerá.
package knowledge

import (
	"errors"
	"fmt"
	"strings"
)

// ── Campos da lacuna ─────────────────────────────────────────────────────────

// GapField é o campo do conhecimento que está em falta na lacuna.
type GapField string

const (
	// GapSyntax é a lacuna de sintaxe/API do pacote ("como chamar X").
	GapSyntax GapField = "syntax"
	// GapOptions é a lacuna de opções/parâmetros/configuração.
	GapOptions GapField = "options"
	// GapErrorBehavior é a lacuna de comportamento de erro/edge cases.
	GapErrorBehavior GapField = "error_behavior"
	// GapVersionCompat é a lacuna de compatibilidade de versão (6.0 ✓ / 6.1 ✗).
	GapVersionCompat GapField = "version_compat"
)

// KnowledgeGap é uma lacuna de conhecimento detectada pelo resolver: o pacote
// (e a versão/API pedida) que o Cosca NÃO cobre, com a razão em pt-BR e a
// cascata de fontes a usar na aquisição.
type KnowledgeGap struct {
	PackageID string     `json:"package_id"`
	Version   string     `json:"version,omitempty"` // "6.1"
	API       string     `json:"api,omitempty"`     // "transaction"
	Fields    []GapField `json:"fields,omitempty"`
	Reason    string     `json:"reason"`            // pt-BR: "Prisma 6.1 não coberto (só 6.0)"
	Sources   []string   `json:"sources,omitempty"` // cascata a usar
}

// ResolveResult é o veredito do resolver: suficiente ou com lacunas, os
// matches por dependência e a próxima ação em pt-BR ("continua" |
// "adquirir: cosca knowledge add prisma").
type ResolveResult struct {
	Sufficient bool              `json:"sufficient"`
	Gaps       []KnowledgeGap    `json:"gaps,omitempty"`
	Matches    []DependencyMatch `json:"matches,omitempty"`

	// ResolvedFollowUp marca que este resolve é um follow-up após a
	// aquisição/validação (os gaps anteriores foram cobertos). O chamador
	// (agente/CLI) seta quando re-executa o resolver depois de adquirir.
	ResolvedFollowUp bool   `json:"resolved_follow_up,omitempty"`
	NextAction       string `json:"next_action"` // pt-BR: "continua" | "adquirir: cosca knowledge add prisma"
}

// ── Cascata de fontes ────────────────────────────────────────────────────────

// sourceCascade é a cascata determinística de 8 níveis que o wire de aquisição
// percorrerá, da fonte mais confiável/barata para a síntese (último recurso):
//
//  1. local-knowledge    — o que já está no .cosca (manifestos, itens, docs)
//  2. existing-evidence  — evidências já adquiridas e validadas (A-XXXX)
//  3. official-docs      — documentação oficial da biblioteca
//  4. official-repository— o repositório oficial (código, exemplos)
//  5. release-notes      — release notes / changelog das versões
//  6. tests-examples     — testes e exemplos (oficiais e de terceiros)
//  7. community          — comunidades/forums/SO (best-effort, baixa confiança)
//  8. llm-synthesis      — síntese por LLM como último recurso
var sourceCascade = []string{
	"local-knowledge",
	"existing-evidence",
	"official-docs",
	"official-repository",
	"release-notes",
	"tests-examples",
	"community",
	"llm-synthesis",
}

// primarySourceCascade é a cascata primária (hierarquia do Don) embutida nos
// KnowledgeGaps produzidos por ResolveTask: docs oficiais → repositório
// oficial → release notes.
func primarySourceCascade() []string {
	return []string{"official-docs", "official-repository", "release-notes"}
}

// SuggestSources devolve a cascata de fontes (8 níveis) como uma lista
// estática ordenada — a ordem determinística que a aquisição percorrerá,
// da fonte local até a síntese por LLM. O argumento está documentado como
// reservado para o wire de aquisição refinar por tipo de lacuna; hoje a
// cascata é a mesma para qualquer lacuna. Nunca muta o argumento.
func SuggestSources(_ KnowledgeGap) []string {
	return append([]string(nil), sourceCascade...)
}

// ── Extração de versão a partir da dependência ──────────────────────────────

// parseDepVersion extrai a versão/range pedida de uma string de dependência
// ("prisma@6.1" → "6.1", "prisma@^6.1.0" → "^6.1.0"). Best-effort: o último
// '@' separa nome de versão, exceto scopes npm sem versão ("@types/react" →
// sem versão). ok=false quando não há pedido de versão.
func parseDepVersion(dep string) (string, bool) {
	s := strings.TrimSpace(dep)
	i := strings.LastIndex(s, "@")
	if i < 0 {
		return "", false
	}
	candidate := strings.TrimSpace(s[i+1:])
	if candidate == "" {
		return "", false
	}
	if candidate == "*" || candidate == "latest" {
		return candidate, true
	}
	first := candidate[0]
	switch {
	case first >= '0' && first <= '9':
		return candidate, true
	case strings.ContainsRune("^~>=<", rune(first)):
		return candidate, true
	case first == 'v' && len(candidate) > 1 && candidate[1] >= '0' && candidate[1] <= '9':
		return candidate, true
	}
	return "", false
}

// versionFamily reduz uma versão à família major.minor ("6.1.0" → "6.1",
// "6.1" → "6.1", "6" → "6", "6.x" → "6.x", "*" → "*"). Marcadores de range
// (^ ~ > = < v) e sufixos são descartados antes da redução.
func versionFamily(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimLeft(v, "^~>=<")
	v = strings.TrimSpace(v)
	if i := strings.IndexAny(v, " \t"); i >= 0 {
		v = v[:i]
	}
	if strings.HasPrefix(v, "v") {
		v = v[1:]
	}
	parts := strings.Split(v, ".")
	if len(parts) > 2 {
		return parts[0] + "." + parts[1]
	}
	return v
}

// familyCovers diz se uma versão rastreada (ex.: "6.x", "6.0", "*") cobre uma
// versão pedida, comparando componente a componente (wildcards x/*):
//
//	"6.x" vs "6.1" → true   (6.x cobre qualquer 6.N)
//	"6.0" vs "6.1" → false
//	"6.1" vs "6.1.0" → true (mais específica que a rastreada)
//	"6" vs "6.1" → true      (6 cobre qualquer 6.N)
//	"*" vs "6.1" → true
func familyCovers(tracked, requested string) bool {
	t := strings.Split(versionFamily(tracked), ".")
	r := strings.Split(versionFamily(requested), ".")
	if len(r) == 0 || r[0] == "" {
		return true
	}
	for i := 0; i < len(t) && i < len(r); i++ {
		if t[i] == "*" || t[i] == "x" || t[i] == "X" {
			return true
		}
		if t[i] != r[i] {
			return false
		}
	}
	return len(t) <= len(r)
}

// coversVersion diz se o pacote cobre a versão pedida: alguma família
// rastreada em Versions cobre a pedida, OU alguma entrada de VersionDiffs
// (chave "from->to") menciona a família pedida. "latest"/"*"/vazio pedido ⇒
// coberto. Determinístico, sem rede.
func (p *KnowledgePackage) coversVersion(requested string) bool {
	req := versionFamily(requested)
	if req == "" || req == "*" || req == "latest" {
		return true
	}
	for _, v := range p.Versions {
		if familyCovers(v, requested) {
			return true
		}
	}
	for key := range p.VersionDiffs {
		for _, endpoint := range strings.SplitN(key, "->", 2) {
			if familyCovers(endpoint, requested) {
				return true
			}
		}
	}
	return false
}

// ── Extração best-effort da API a partir da tarefa ──────────────────────────

// taskStopwords são verbos de ação e conectivos que não são API (cortados da
// extração). Best-effort, sem LLM.
var taskStopwords = map[string]bool{
	// verbos de ação (en)
	"create": true, "implement": true, "use": true, "add": true,
	"update": true, "remove": true, "make": true, "set": true, "get": true,
	"configure": true, "fix": true, "refactor": true, "test": true,
	"deploy": true, "install": true, "migrate": true, "continue": true,
	// verbos de ação (pt-BR)
	"implemente": true, "implementar": true, "criar": true, "usar": true,
	"adicionar": true, "remover": true, "atualizar": true, "configurar": true,
	"fazer": true, "corrigir": true, "testar": true, "instalar": true,
	"migrar": true, "continuar": true,
	// conectivos / artigos / preposições
	"the": true, "a": true, "an": true, "and": true, "or": true,
	"de": true, "da": true, "do": true, "das": true, "dos": true,
	"em": true, "no": true, "na": true, "com": true, "para": true,
	"por": true, "usando": true, "um": true, "uma": true, "o": true,
	"os": true, "as": true, "e": true, "que": true,
}

// versionLikeToken diz se um token parece versão (dígito inicial, range ou
// "v<N>") — cortado da extração de API.
func versionLikeToken(s string) bool {
	if s == "" {
		return false
	}
	switch {
	case s[0] >= '0' && s[0] <= '9':
		return true
	case strings.ContainsRune("^~>=<", rune(s[0])):
		return true
	case s[0] == 'v' && len(s) > 1 && s[1] >= '0' && s[1] <= '9':
		return true
	}
	return false
}

// detectAPIFromTask extrai candidatos de API da tarefa ("create transaction" →
// "transaction"; "Implemente X usando Prisma 6.1" → "x" com prisma/6.1
// excluídos). Best-effort e determinístico — o wire de validação real fica
// para depois; aqui apenas melhora o KnowledgeGap.
func detectAPIFromTask(task string, exclude map[string]bool) string {
	var api []string
	for _, t := range strings.Fields(strings.ToLower(task)) {
		t = strings.Trim(t, ",.;:()[]{}<>\"'`!?")
		if t == "" || taskStopwords[t] || exclude[t] || versionLikeToken(t) {
			continue
		}
		api = append(api, t)
	}
	return strings.Join(api, "/")
}

// nextActionFor monta a próxima ação em pt-BR para as lacunas: a aquisição
// dos pacotes em falta, deduplicada e preservando a ordem.
func nextActionFor(gaps []KnowledgeGap) string {
	seen := map[string]bool{}
	var actions []string
	for _, g := range gaps {
		id := g.PackageID
		if id == "" {
			continue
		}
		action := "cosca knowledge add " + id
		if !seen[action] {
			seen[action] = true
			actions = append(actions, action)
		}
	}
	if len(actions) == 0 {
		return "adquirir conhecimento"
	}
	return "adquirir: " + strings.Join(actions, " + ")
}

// ── ResolveTask ─────────────────────────────────────────────────────────────

// ResolveTask responde "tenho conhecimento suficiente?" para uma tarefa e uma
// lista de dependências do projeto, cruzando-as com a PackageStore.
//
// Determinístico, sem rede e sem LLM: cada dependência é normalizada
// (NormalizeDependencyName) e classificada (MatchProject). Um pacote é
// SUFFICIENT quando MatchVerified E (não há pedido de versão OU o pacote cobre
// a versão pedida em Versions/VersionDiffs). Caso contrário um KnowledgeGap é
// produzido com a razão em pt-BR e a cascata primária de fontes a usar
// (official-docs → official-repository → release-notes, a hierarquia do Don).
//
// A API do gap é extraída da tarefa por heurística best-effort ("create
// transaction" → "transaction"). PackageStore nil é um erro. global é
// opcional — consultado como fallback quando o package não está na store local.
func ResolveTask(task string, deps []string, store *PackageStore, global *PackageStore) (*ResolveResult, error) {
	if store == nil {
		return nil, errors.New("knowledge: PackageStore nil")
	}

	matches, err := MatchProject(deps, store, global)
	if err != nil {
		return nil, err
	}

	result := &ResolveResult{
		Sufficient: true,
		Matches:    matches,
		NextAction: "continua",
	}

	// IDs de pacote conhecidos (matches + nomes normalizados) são excluídos da
	// extração best-effort da API.
	exclude := map[string]bool{}
	for _, m := range matches {
		if m.PackageID != "" {
			exclude[m.PackageID] = true
		}
		if id := NormalizeDependencyName(m.Dependency); id != "" {
			exclude[id] = true
		}
	}
	api := detectAPIFromTask(task, exclude)

	for _, m := range matches {
		dep := m.Dependency
		id := NormalizeDependencyName(dep)
		if id == "" {
			id = dep
		}
		requested, hasVersion := parseDepVersion(dep)

		switch m.Status {
		case MatchMissing:
			result.Gaps = append(result.Gaps, KnowledgeGap{
				PackageID: id,
				API:       api,
				Fields:    []GapField{GapSyntax},
				Reason:    fmt.Sprintf("sem conhecimento para %s — ofereça aquisição", id),
				Sources:   primarySourceCascade(),
			})
		case MatchPartial:
			result.Gaps = append(result.Gaps, KnowledgeGap{
				PackageID: id,
				API:       api,
				Fields:    []GapField{GapOptions},
				Reason:    fmt.Sprintf("conhecimento parcial para %s — manifesto sem validação", id),
				Sources:   primarySourceCascade(),
			})
		case MatchStale:
			result.Gaps = append(result.Gaps, KnowledgeGap{
				PackageID: id,
				API:       api,
				Fields:    []GapField{GapErrorBehavior},
				Reason:    fmt.Sprintf("conhecimento envelhecido para %s — requer revalidação", id),
				Sources:   primarySourceCascade(),
			})
		case MatchVerified:
			pkg, _ := store.Get(m.PackageID)
			if hasVersion && pkg != nil && !pkg.coversVersion(requested) {
				covered := strings.Join(pkg.Versions, ", ")
				prefix := "conhecimento cobre"
				if len(pkg.Versions) == 1 {
					prefix = "só"
				}
				result.Gaps = append(result.Gaps, KnowledgeGap{
					PackageID: m.PackageID,
					Version:   requested,
					API:       api,
					Fields:    []GapField{GapVersionCompat},
					Reason:    fmt.Sprintf("%s %s não coberto (%s %s)", m.PackageID, requested, prefix, covered),
					Sources:   primarySourceCascade(),
				})
			}
		}
	}

	if len(result.Gaps) > 0 {
		result.Sufficient = false
		result.NextAction = nextActionFor(result.Gaps)
	}
	return result, nil
}

// ── VoiceSummary (mensagem 🔊) ───────────────────────────────────────────────

// VoiceSummary devolve a mensagem pt-BR da 🔊 para o veredito:
//
//	suficiente (primeira resolução)        → "" (nada a dizer)
//	lacunas                                → "Encontrei uma lacuna de
//	                                           conhecimento: {pkg} {version}/{api}.
//	                                           Estou validando antes de continuar."
//	follow-up que resolve as lacunas       → "validado. continuei a implementação."
func (r *ResolveResult) VoiceSummary() string {
	if r == nil {
		return ""
	}
	if !r.Sufficient {
		if len(r.Gaps) == 0 {
			return "Encontrei uma lacuna de conhecimento. Estou validando antes de continuar."
		}
		g := r.Gaps[0]
		label := g.PackageID
		if g.Version != "" {
			label += " " + g.Version
		}
		if g.API != "" {
			label += "/" + g.API
		}
		return fmt.Sprintf("Encontrei uma lacuna de conhecimento: %s. Estou validando antes de continuar.", label)
	}
	if r.ResolvedFollowUp {
		return "validado. continuei a implementação."
	}
	return ""
}
