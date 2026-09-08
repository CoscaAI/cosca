// Package taskaffinity deriva o perfil de tarefa do estado de implementação
// em curso e fornece re-ponderação de resultados por afinidade.
//
// É a camada NOVA do Task-Aware Search (ADR-045 §2.2). NÃO substitui o
// search/ranking/modlink — ADICIONA a dimensão de afinidade com a tarefa.
//
// Determinístico: sem LLM, sem ML — regras baseadas em tokens e paths.
// Mesma entrada → mesmo perfil.
package taskaffinity

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// TaskProfile é o perfil de tarefa derivado do estado de implementação real.
// Captura O QUE está sendo implementado, COM QUE stack, ONDE (alvo), em QUE
// fase, e QUAIS termos re-ponderam a busca.
type TaskProfile struct {
	// Task é a intenção bruta do usuário (o prompt original). Não é tokenizado
	// aqui — preserva o texto original para auditabilidade.
	Task string `json:"task"`

	// Stack é o stack tecnológico detectado (ex.: "go+chi+sqlite").
	// Ordenado alfabeticamente, deduplicado. Vazio = stack não detectado.
	Stack []string `json:"stack,omitempty"`

	// Target é o módulo/alvo provável da implementação (ex.: "internal/api/handlers.go").
	// Caminho relativo à raiz do projeto. Vazio = alvo não determinado.
	Target string `json:"target,omitempty"`

	// TargetModule é o módulo extraído do Target (ex.: "api", "search", "ranking").
	// Derivado do segmento mais significativo do caminho. Usado para confinamento
	// adicional (complementa o modlink scope).
	TargetModule string `json:"target_module,omitempty"`

	// Affinity é a lista de termos-chave que re-ponderam a busca. Inclui:
	// termos da Task tokenizados + termos do Stack + termos do TargetModule.
	// Deduplicados e ordenados. Usada como "âncora semântica" da re-ponderação.
	Affinity []string `json:"affinity"`

	// Confidence é a confiança na derivação do perfil (0.0–1.0).
	// 0.0 = perfil não derivado (perfil nil); 1.0 = todos os campos preenchidos.
	// Afeta a FORÇA da re-ponderação (seção 3).
	Confidence float64 `json:"confidence"`
}

// HasStack reports whether the profile detected a technology stack.
func (p *TaskProfile) HasStack() bool {
	return len(p.Stack) > 0
}

// HasTarget reports whether the profile identified an implementation target.
func (p *TaskProfile) HasTarget() bool {
	return p.Target != ""
}

// HasAffinity reports whether the profile has re-weighting terms.
func (p *TaskProfile) HasAffinity() bool {
	return len(p.Affinity) > 0
}

// AffinitySet retorna os termos de afinidade como um set (map[string]struct{}).
// Otimizado para lookup O(1) na re-ponderação.
func (p *TaskProfile) AffinitySet() map[string]struct{} {
	set := make(map[string]struct{}, len(p.Affinity))
	for _, a := range p.Affinity {
		set[a] = struct{}{}
	}
	return set
}

// ImplementaçãoState é o estado de implementação que alimenta a derivação.
// É determinístico — sem LLM, sem API externa.
type ImplementaçãoState struct {
	// Prompt é a intenção bruta do usuário.
	Prompt string

	// WorkingDir é o diretório de trabalho (raiz do projeto).
	WorkingDir string

	// OpenFiles são os caminhos dos arquivos abertos no editor (relativos ao
	// WorkingDir). Pode ser vazio (fase de exploração).
	OpenFiles []string

	// RecentFiles são os arquivos modificados recentemente (ordenados por
	// última modificação descendente). Pode ser vazio.
	RecentFiles []string

	// GoModExists indica se o projeto tem go.mod (sinal forte de stack Go).
	GoModExists bool

	// PackageJSONExists indica se o projeto tem package.json (sinal forte de stack Node/JS).
	PackageJSONExists bool

	// TargetHint é o alvo explícito do usuário (ex.: um path que ele mencionou).
	TargetHint string
}

// DeriveProfile é a função PRIMÁRIA de derivação do perfil de tarefa.
// Recebe o estado de implementação e devolve o perfil com confiança.
// Determinística: mesma entrada → mesmo perfil.
//
// Nil-safe: ImplementaçãoState nil → retorna nil (comportamento retrocompatível).
func DeriveProfile(state *ImplementaçãoState) *TaskProfile {
	if state == nil {
		return nil
	}

	profile := &TaskProfile{Task: state.Prompt}
	profile.Stack = detectStack(state)
	profile.Target, profile.TargetModule = detectTarget(state)
	profile.Affinity = generateAffinity(extractTaskTokens(state.Prompt), profile.Stack, profile.TargetModule)
	profile.Confidence = computeConfidence(profile)
	return profile
}

// ─── 2.4.1 TASK → extração de termos ─────────────────────────────────────────

// extractTaskTokens extrai os termos-chave da intenção bruta do usuário
// (DESIGN-001 §2.4.1):
//
//  1. Tokenizar o prompt (mesmo algoritmo de tokenize() do internal/ranking).
//  2. Remover stopwords (artigos, preposições, pronomes — lista fixa).
//  3. Deduplicar e ordenar alfabeticamente.
func extractTaskTokens(prompt string) []string {
	tokens := tokenize(prompt)
	kept := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if _, stop := stopwords[t]; !stop {
			kept = append(kept, t)
		}
	}
	return dedupeSorted(kept)
}

// stopwords é a lista fixa e determinística de palavras removidas da Task
// (artigos, preposições, pronomes — pt-BR e en). Termos de domínio (ex.:
// "endpoint", "módulo") NÃO são stopwords: permanecem na afinidade.
var stopwords = map[string]struct{}{
	// Português
	"a": {}, "ao": {}, "aos": {}, "as": {}, "à": {}, "às": {},
	"com": {}, "da": {}, "das": {}, "de": {}, "do": {}, "dos": {},
	"e": {}, "em": {}, "na": {}, "nas": {}, "no": {}, "nos": {},
	"o": {}, "os": {}, "para": {}, "por": {}, "que": {}, "se": {},
	"um": {}, "uma": {}, "uns": {}, "umas": {},
	// Inglês
	"an": {}, "and": {}, "for": {}, "in": {}, "of": {},
	"on": {}, "the": {}, "to": {}, "with": {},
}

// ─── 2.4.2 STACK → detecção determinística ───────────────────────────────────

// detectStack detecta o stack tecnológico de forma determinística
// (DESIGN-001 §2.4.2). As regras são avaliadas em ordem de prioridade e
// ACUMULADAS: um projeto pode ter múltiplos sinais (ex.: go.mod + arquivos
// .py). O resultado é deduplicado e ordenado alfabeticamente.
func detectStack(state *ImplementaçãoState) []string {
	if state == nil {
		return nil
	}

	var stack []string
	files := allFiles(state)

	// Sinais fortes (prioridade alta).
	if state.GoModExists {
		stack = append(stack, "go")
	}
	if state.PackageJSONExists {
		stack = append(stack, "node")
		if hasTSConfig(state.WorkingDir) {
			stack = append(stack, "typescript")
		}
	}

	// Sinais por extensão de arquivo (OpenFiles/RecentFiles).
	for _, f := range files {
		switch strings.ToLower(filepath.Ext(f)) {
		case ".go":
			stack = append(stack, "go")
		case ".ts", ".tsx":
			stack = append(stack, "typescript")
		case ".py":
			stack = append(stack, "python")
		case ".rs":
			stack = append(stack, "rust")
		}
	}

	// Heurística do projeto Cosca: path contém "internal/" ⇒ Go.
	if !contains(stack, "go") {
		for _, f := range files {
			if strings.Contains(filepath.ToSlash(f), "internal/") {
				stack = append(stack, "go")
				break
			}
		}
	}

	// Frameworks específicos do projeto (tabela fechada).
	stack = append(stack, detectFrameworks(files)...)

	return dedupeSorted(stack)
}

// detectFrameworks detecta frameworks específicos do projeto a partir dos
// paths (DESIGN-001 §2.4.2, tabela de frameworks). Tabela fechada e
// determinística. "chi" usa correspondência por segmento de path (evita o
// falso positivo de "architecture"); os demais usam substring (baixo risco).
func detectFrameworks(files []string) []string {
	var fw []string
	for _, f := range files {
		lower := strings.ToLower(filepath.ToSlash(f))
		if hasPathToken(lower, "chi") {
			fw = append(fw, "chi")
		}
		if strings.Contains(lower, "sqlite") {
			fw = append(fw, "sqlite")
		}
		if strings.Contains(lower, "grpc") {
			fw = append(fw, "grpc")
		}
		if strings.Contains(lower, "ollama") {
			fw = append(fw, "ollama")
		}
		if strings.Contains(lower, "onnx") {
			fw = append(fw, "onnxruntime")
		}
	}
	return fw
}

// hasPathToken reports whether path contém token como um segmento inteiro
// (delimitado por /, ., _, -). Evita falsos positivos de substring: o path
// "internal/architecture" NÃO contém o token "chi" como segmento.
func hasPathToken(path, token string) bool {
	for _, seg := range strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '.' || r == '_' || r == '-'
	}) {
		if seg == token {
			return true
		}
	}
	return false
}

// hasTSConfig verifica a existência de tsconfig.json no WorkingDir (sinal de
// TypeScript, DESIGN-001 §2.4.2). Determinístico dado o estado do filesystem;
// WorkingDir vazio → false (sem sinal).
func hasTSConfig(workingDir string) bool {
	if workingDir == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(workingDir, "tsconfig.json"))
	return err == nil && !info.IsDir()
}

// ─── 2.4.3 TARGET → detecção de alvo ─────────────────────────────────────────

// detectTarget detecta o alvo provável da implementação (DESIGN-001 §2.4.3).
// Ordem de prioridade: TargetHint → primeiro OpenFile → primeiro RecentFile.
// TargetModule é extraído do segmento imediatamente após "internal/"; sem
// "internal/", o penúltimo segmento não-vazio do path.
func detectTarget(state *ImplementaçãoState) (target, module string) {
	if state == nil {
		return "", ""
	}

	switch {
	case state.TargetHint != "":
		target = state.TargetHint
	case len(state.OpenFiles) > 0:
		target = state.OpenFiles[0]
	case len(state.RecentFiles) > 0:
		target = state.RecentFiles[0]
	}
	if target == "" {
		return "", ""
	}
	return target, extractTargetModule(target)
}

// extractTargetModule extrai o módulo-alvo de um path (DESIGN-001 §2.4.3):
// o segmento IMEDIATAMENTE DEPOIS de "internal/"; sem "internal/", o
// penúltimo segmento não-vazio.
//
//	"internal/api/handlers.go"            → "api"
//	"internal/contextpipeline/x.go"       → "contextpipeline"
//	"api/handlers.go"                     → "api"
//	"handlers.go"                         → ""
func extractTargetModule(target string) string {
	segs := nonEmptySegments(filepath.ToSlash(target))
	for i, s := range segs {
		if s == "internal" && i+1 < len(segs) {
			return segs[i+1]
		}
	}
	if len(segs) >= 2 {
		return segs[len(segs)-2]
	}
	return ""
}

// nonEmptySegments divide um path em segmentos, ignorando vazios.
func nonEmptySegments(path string) []string {
	raw := strings.Split(path, "/")
	segs := make([]string, 0, len(raw))
	for _, s := range raw {
		if s != "" {
			segs = append(segs, s)
		}
	}
	return segs
}

// ─── 2.4.4 AFFINITY → geração de termos ──────────────────────────────────────

// generateAffinity gera os termos de afinidade (DESIGN-001 §2.4.4):
// taskTokens + stack + TargetModule (+ "pai" do módulo) + expansão da tabela
// moduleSynonyms. Deduplicado e ordenado alfabeticamente.
func generateAffinity(taskTokens, stack []string, targetModule string) []string {
	terms := make([]string, 0, len(taskTokens)+len(stack)+8)
	terms = append(terms, taskTokens...)
	terms = append(terms, stack...)
	if targetModule != "" {
		terms = append(terms, targetModule)
		if parent := moduleParent(targetModule); parent != "" {
			terms = append(terms, parent)
		}
		if syns, ok := moduleSynonyms[targetModule]; ok {
			terms = append(terms, syns...)
		}
	}
	return dedupeSorted(terms)
}

// moduleParent deriva o "pai" de um módulo composto (DESIGN-001 §2.4.4 passo 3):
// ex.: "contextpipeline" → "context". Regra determinística: se o módulo começa
// com um prefixo conhecido (e não é o próprio prefixo), o pai é o prefixo.
// Prefixos fechados — novos são adicionados explicitamente.
var modulePrefixes = []string{"context", "knowledge", "vector", "graph", "orchestration", "memory"}

func moduleParent(module string) string {
	for _, p := range modulePrefixes {
		if strings.HasPrefix(module, p) && module != p {
			return p
		}
	}
	return ""
}

// moduleSynonyms mapeia módulos do projeto para termos associados que
// ampliam o alcance da afinidade. A tabela é determinística e fechada —
// novos módulos são adicionados explicitamente (DESIGN-001 §2.4.4).
//
// NOTA: o design lista "ranking" duas vezes (linhas 304 e 315) com conjuntos
// distintos; as duas entradas foram mescladas aqui (chave duplicada não é
// válida em Go).
var moduleSynonyms = map[string][]string{
	"api":             {"handler", "endpoint", "rest", "route"},
	"search":          {"query", "retrieval", "fts", "vector", "bm25", "cosine"},
	"ranking":         {"rerank", "score", "relevance", "bm25", "weight", "graph", "freshness", "popularity"},
	"modlink":         {"route", "scope", "module", "trigger", "domain"},
	"contextcompile":  {"context", "compiler", "facts", "evidence", "section"},
	"contextrouter":   {"context", "level", "confidence", "layer"},
	"contextpipeline": {"context", "pipeline", "compile", "budget"},
	"orchestration":   {"orchestrate", "executor", "agent", "task"},
	"vector":          {"embedding", "cosine", "similarity", "store"},
	"graph":           {"node", "edge", "neighbor", "bfs", "traversal"},
	"knowledge":       {"knowledge", "fact", "evidence", "epistemic"},
	"sqlite":          {"database", "fts5", "schema", "migration"},
	"memory":          {"memory", "learning", "snapshot"},
}

// ─── 2.4.5 Confidence → cálculo ──────────────────────────────────────────────

// computeConfidence calcula a confiança na derivação (DESIGN-001 §2.4.5).
// Fórmula: Task presente +0.30; Stack presente +0.25; Target presente +0.25;
// Affinity com >3 termos +0.20 (1–3 termos +0.10). Range [0.0, 1.0].
//
// NOTA: o exemplo do design (§2.5) mostra Confidence 0.95 para o caso
// completo, mas a fórmula §2.4.5 soma 1.0 (0.30+0.25+0.25+0.20) — a fórmula é
// autoritativa.
func computeConfidence(profile *TaskProfile) float64 {
	if profile == nil {
		return 0
	}
	confidence := 0.0
	if profile.Task != "" {
		confidence += 0.30
	}
	if len(profile.Stack) > 0 {
		confidence += 0.25
	}
	if profile.Target != "" {
		confidence += 0.25
	}
	if len(profile.Affinity) > 3 {
		confidence += 0.20
	} else if len(profile.Affinity) > 0 {
		confidence += 0.10
	}
	return confidence
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// tokenize divide o texto em tokens minúsculos (letras e dígitos), com o MESMO
// algoritmo de tokenize() do internal/ranking (que é não exportado). Replicado
// aqui para manter a semântica idêntica sem modificar o pacote existente
// (DESIGN-001 §2.4.1: "reutilizar tokenize() do internal/ranking").
func tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	var current strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

// dedupeSorted deduplica (ignorando vazios) e ordena alfabeticamente.
func dedupeSorted(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, it := range items {
		if it == "" {
			continue
		}
		if _, ok := seen[it]; ok {
			continue
		}
		seen[it] = struct{}{}
		out = append(out, it)
	}
	sort.Strings(out)
	return out
}

// contains reports whether items contém want.
func contains(items []string, want string) bool {
	for _, it := range items {
		if it == want {
			return true
		}
	}
	return false
}

// allFiles concatena OpenFiles e RecentFiles (sem duplicar referências).
func allFiles(state *ImplementaçãoState) []string {
	if state == nil {
		return nil
	}
	files := make([]string, 0, len(state.OpenFiles)+len(state.RecentFiles))
	files = append(files, state.OpenFiles...)
	files = append(files, state.RecentFiles...)
	return files
}
