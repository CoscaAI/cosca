// Package oracle — a fronteira semântica do Cofre (ORACLE_PROTOCOL).
//
// O Oráculo reside dentro do Cofre (a jaula). Ele NÃO é o Cosca externo, não
// é um executor, não busca livremente recursos externos. Sua função é:
//
//	receber, interpretar, validar e classificar semanticamente aquilo que o
//	Cosca externo trouxe para dentro do Cofre.
//
// O Cosca externo possui capacidade de exploração. O Oráculo possui
// capacidade de validação. A validação NUNCA se baseia na palavra do Cosca
// externo — baseia-se no pacote semântico (proveniência + evidência +
// contexto) e na memória assinada (raízes verificadas, L418).
//
// Invariantes (ORACLE_PROTOCOL §25-§29):
//  1. O Oráculo valida o SIGNIFICADO, nunca apenas a forma.
//  2. Nenhuma informação externa ganha autoridade só por entrar no Cofre.
//  3. Nenhuma hipótese vira fato sem evidência compatível com sua importância.
//  4. Sem evidência suficiente → INCONCLUSIVE, nunca inventar conclusão.
//  5. O Cosca externo pode explorar; o Oráculo compreende o que foi encontrado.
package oracle

// ─── Protocolo de Busca ───────────────────────────────────────────────────────
//
// O Cosca externo NÃO busca lixo. Toda busca carrega: intenção (por que
// busco), fonte (onde busco), e exige proveniência no resultado. Isso
// implementa a regra do Don: "vc tbm nao ficar buscando lixo e usar uma forma
// de protocolo de busca".

// SearchIntent classifica POR QUE a busca está sendo feita. Uma busca sem
// intenção declarada é rejeitada pelo oráculo antes de sair.
type SearchIntent string

const (
	// IntentDiagnose — descobrir a causa de um sintoma/erro.
	IntentDiagnose SearchIntent = "diagnose"
	// IntentVerify — confirmar ou refutar uma afirmação/estado.
	IntentVerify SearchIntent = "verify"
	// IntentExplore — mapear um domínio desconhecido (descoberta legítima).
	IntentExplore SearchIntent = "explore"
	// IntentCompare — comparar alternativas para decisão.
	IntentCompare SearchIntent = "compare"
	// IntentImplement — encontrar o que é necessário para implementar X.
	IntentImplement SearchIntent = "implement"
)

// Valid reporta se a intenção é conhecida. Intenção desconhecida = busca
// sem propósito → o oráculo bloqueia (fail-closed).
func (i SearchIntent) Valid() bool {
	switch i {
	case IntentDiagnose, IntentVerify, IntentExplore, IntentCompare, IntentImplement:
		return true
	}
	return false
}

// Source identifica ONDE a busca ocorre. Fontes fora da lista permitida são
// bloqueadas — o Cosca externo não busca livremente.
type Source string

const (
	// SourceMemory — a memória assinada da casa (knowledge.db, chain, blocks).
	SourceMemory Source = "memory"
	// SourceCodebase — o código do projeto (workspace).
	SourceCodebase Source = "codebase"
	// SourceDocs — documentação local (docs/, ADRs).
	SourceDocs Source = "docs"
	// SourceExternal — recursos EXTERNOS (web, internet). Sempre tratado
	// como EXTERNAL INPUT até validação (ORACLE_PROTOCOL §11).
	SourceExternal Source = "external"
)

// Valid reporta se a fonte é permitida.
func (s Source) Valid() bool {
	switch s {
	case SourceMemory, SourceCodebase, SourceDocs, SourceExternal:
		return true
	}
	return false
}

// SearchRequest é o protocolo formal de busca. O Cosca externo monta este
// pacote antes de qualquer busca; sem ele, a busca é considerada lixo.
type SearchRequest struct {
	// Intent — POR QUE estou buscando (obrigatório).
	Intent SearchIntent `json:"intent"`
	// Query — O QUE estou procurando (obrigatório, não vazio).
	Query string `json:"query"`
	// Sources — ONDE posso buscar (obrigatório, ≥1).
	Sources []Source `json:"sources"`
	// MaxResults limita o retorno (default 10).
	MaxResults int `json:"max_results,omitempty"`
	// Context descreve o cenário (produção, teste, investigação...).
	Context string `json:"context,omitempty"`
}

// Valid valida o protocolo de busca ANTES de executar. Falhou = bloqueado
// (fail-closed): a busca nem sai.
func (r SearchRequest) Valid() (bool, string) {
	if !r.Intent.Valid() {
		return false, "intent inválida ou ausente — busca sem propósito é lixo"
	}
	if len(r.Query) == 0 {
		return false, "query vazia — não se busca nada"
	}
	if len(r.Sources) == 0 {
		return false, "sem fontes declaradas — não se busca no vazio"
	}
	for _, s := range r.Sources {
		if !s.Valid() {
			return false, "fonte inválida: " + string(s)
		}
	}
	return true, ""
}

// ─── Proveniência ─────────────────────────────────────────────────────────────

// Provenance responde: de onde veio? quando? como? qual versão? Sem
// proveniência verificável, o resultado é EXTERNAL INPUT até validação.
type Provenance struct {
	// Source é a origem (memory/codebase/docs/external).
	Source Source `json:"source"`
	// Ref é a referência verificável: hash de commit, hash de bloco,
	// caminho de arquivo, URL.
	Ref string `json:"ref"`
	// Method descreve COMO o resultado foi obtido (comando, busca, leitura).
	Method string `json:"method"`
	// Timestamp quando foi obtido (RFC3339).
	Timestamp string `json:"timestamp"`
}

// Verifiable reporta se a proveniência tem referência concreta. Sem Ref, o
// resultado não pode ser verificado — o oráculo não o aceita como FACT.
func (p Provenance) Verifiable() bool {
	return p.Ref != ""
}

// ─── Resultado da Busca ────────────────────────────────────────────────────────

// EvidenceClass separa FATO de INFERÊNCIA (ORACLE_PROTOCOL §8). NUNCA
// promover automaticamente Hypothesis → Fact.
type EvidenceClass string

const (
	// Fact — observado diretamente (arquivo existe, hash = X).
	Fact EvidenceClass = "fact"
	// Measured — medido por comando/instrumento (benchmark, hash calculado).
	Measured EvidenceClass = "measured"
	// Evidence — saída de um comando/ferramenta que sustenta uma afirmação.
	Evidence EvidenceClass = "evidence"
	// Inferred — concluído por raciocínio, não observado.
	Inferred EvidenceClass = "inferred"
	// Hypothesis — suposição a ser testada.
	Hypothesis EvidenceClass = "hypothesis"
)

// SearchResult é um item encontrado, com classificação de evidência.
type SearchResult struct {
	// Intent ecoa a intenção da busca que o produziu (rastreabilidade).
	Intent SearchIntent `json:"intent"`
	// Title é o título/identificação do achado.
	Title string `json:"title"`
	// Content é o conteúdo relevante.
	Content string `json:"content"`
	// Class classifica o tipo de evidência (fact/measured/inferred/...).
	Class EvidenceClass `json:"class"`
	// Provenance é a origem verificável (obrigatória para Fact/Measured).
	Provenance Provenance `json:"provenance"`
	// Confidence HIGH/MEDIUM/LOW — nunca promovido além da evidência.
	Confidence string `json:"confidence"`
}

// Valid reporta se o resultado é aceitável. Resultado sem proveniência não
// pode ser Fact/Measured — o oráculo o rebaixa a Hypothesis/Inferred.
func (r SearchResult) Valid() bool {
	if r.Title == "" && r.Content == "" {
		return false
	}
	if (r.Class == Fact || r.Class == Measured) && !r.Provenance.Verifiable() {
		return false
	}
	return true
}

// ─── Classificação de Relevância (SEARCH_PROTOCOL §6) ─────────────────────────
//
// TEXT MATCH ≠ SEMANTIC RELEVANCE (§5). Todo resultado recebe uma classe que
// decide se ele é apresentado ou descartado.

// RelevanceClass classifica a relação semântica do resultado com a intenção.
type RelevanceClass string

const (
	// Direct — responde diretamente à intenção.
	Direct RelevanceClass = "DIRECT"
	// Related — diretamente relacionado, mas não responde sozinho.
	Related RelevanceClass = "RELATED"
	// Contextual — ajuda a compreender o problema.
	Contextual RelevanceClass = "CONTEXTUAL"
	// Weak — relação superficial.
	Weak RelevanceClass = "WEAK"
	// Noise — coincidência textual ou relação irrelevante (§7: NÃO apresentar).
	Noise RelevanceClass = "NOISE"
)

// Presentable reporta se o resultado deve ser apresentado ao usuário (§7):
// NOISE nunca; WEAK só quando necessário para explicar ausência.
func (c RelevanceClass) Presentable() bool {
	return c == Direct || c == Related || c == Contextual
}

// ─── Score Semântico (SEARCH_PROTOCOL §8) ─────────────────────────────────────

// SemanticScore ordena resultados: RELEVANCE + INTENT_MATCH + ENTITY_MATCH +
// CONTEXT_MATCH + RECENCY + PROVENANCE + EVIDENCE_VALUE − penalidades.
type SemanticScore struct {
	// Total é o score final (maior = mais relevante).
	Total float64 `json:"total"`
	// Relevance — relação semântica com a intenção (0-1).
	Relevance float64 `json:"relevance"`
	// IntentMatch — quão bem responde à intenção (0-1).
	IntentMatch float64 `json:"intent_match"`
	// EntityMatch — correspondência com a entidade alvo (0-1).
	EntityMatch float64 `json:"entity_match"`
	// ContextMatch — coerência com o contexto (0-1).
	ContextMatch float64 `json:"context_match"`
	// Recency — quão recente é (0-1).
	Recency float64 `json:"recency"`
	// Provenance — qualidade da origem (0-1).
	Provenance float64 `json:"provenance"`
	// EvidenceValue — valor como evidência (0-1).
	EvidenceValue float64 `json:"evidence_value"`
	// Penalties acumula as penalidades (keyword_only, duplicate, outdated...).
	Penalties []string `json:"penalties,omitempty"`
}

// Compute soma os componentes e subtrai penalidades.
func (s *SemanticScore) Compute() {
	total := s.Relevance + s.IntentMatch + s.EntityMatch + s.ContextMatch +
		s.Recency + s.Provenance + s.EvidenceValue
	// Penalidade por componente ausente: sem proveniência forte, sem
	// evidência, sem relação com a intenção.
	if len(s.Penalties) > 0 {
		total -= 0.5 * float64(len(s.Penalties))
	}
	if total < 0 {
		total = 0
	}
	s.Total = total
}

// ─── Busca em Camadas (SEARCH_PROTOCOL §3) ────────────────────────────────────

// SearchLayer define a camada de expansão da busca. Nunca começar pelo
// universo inteiro — expandir só quando a camada anterior for insuficiente.
type SearchLayer int

const (
	// LayerIdentity — identidade/conceito exato (primeiro).
	LayerIdentity SearchLayer = iota
	// LayerRelated — entidades relacionadas.
	LayerRelated
	// LayerSynonyms — sinônimos/conceitos equivalentes.
	LayerSynonyms
	// LayerStructural — busca estrutural ampla.
	LayerStructural
	// LayerExplore — exploração geral (último recurso).
	LayerExplore
)

// String retorna o nome da camada.
func (l SearchLayer) String() string {
	switch l {
	case LayerIdentity:
		return "identity"
	case LayerRelated:
		return "related"
	case LayerSynonyms:
		return "synonyms"
	case LayerStructural:
		return "structural"
	case LayerExplore:
		return "explore"
	default:
		return "unknown"
	}
}

// Next retorna a próxima camada (expansão). A última não expande.
func (l SearchLayer) Next() (SearchLayer, bool) {
	if l >= LayerExplore {
		return l, false
	}
	return l + 1, true
}

// ─── Resultado da Busca (com classificação) ───────────────────────────────────

// RankedResult é um SearchResult com classificação de relevância e score.
type RankedResult struct {
	Result  SearchResult   `json:"result"`
	Relevance RelevanceClass `json:"relevance"`
	Score   SemanticScore  `json:"score"`
	// Layer em que foi encontrado (para rastrear expansão).
	Layer SearchLayer `json:"layer"`
	// DuplicateOf aponta para o título do resultado principal quando este
	// é uma duplicata agrupada (§9).
	DuplicateOf string `json:"duplicate_of,omitempty"`
}

// ─── Saída da Busca (SEARCH_PROTOCOL §15) ─────────────────────────────────────

// SearchOutcome é o resultado SEMÂNTICO da busca — ausência também é resultado.
type SearchOutcome string

const (
	// OutcomeFound — evidência suficiente encontrada.
	OutcomeFound SearchOutcome = "FOUND"
	// OutcomeNotFound — nenhuma evidência relevante (§15: não inventar).
	OutcomeNotFound SearchOutcome = "NOT_FOUND"
	// OutcomeInsufficient — há algo, mas insuficiente para decidir.
	OutcomeInsufficient SearchOutcome = "INSUFFICIENT_EVIDENCE"
	// OutcomeConflict — resultados relevantes discordam (§10: investigar).
	OutcomeConflict SearchOutcome = "CONFLICT"
)

// SearchResponse é o pacote final da busca: o que foi encontrado, classificado
// e deduplicado — nunca o dump cru.
type SearchResponse struct {
	Outcome  SearchOutcome   `json:"outcome"`
	Results  []RankedResult  `json:"results"`
	// Presentable são só os DIRECT/RELATED/CONTEXTUAL (NOISE fora).
	Presentable []RankedResult `json:"presentable"`
	// Conflict descreve a discordância quando Outcome == CONFLICT (§10).
	Conflict *Conflict `json:"conflict,omitempty"`
	// StoppedAt é a camada onde a busca parou (§3, §14).
	StoppedAt SearchLayer `json:"stopped_at"`
	// Reason explica por que parou (critério de parada, §14).
	Reason string `json:"reason,omitempty"`
}

// ─── Conflito (SEARCH_PROTOCOL §10) ───────────────────────────────────────────

// Conflict é uma discordância entre resultados relevantes — vira objeto de
// investigação, nunca escolha do primeiro.
type Conflict struct {
	// Question é a pergunta que os resultados discordam.
	Question string `json:"question"`
	// Sides são as versões em conflito.
	Sides []ConflictSide `json:"sides"`
}

// ConflictSide é um lado do conflito com seus atributos de investigação.
type ConflictSide struct {
	Claim      string `json:"claim"`
	Provenance string `json:"provenance"`
	Timestamp  string `json:"timestamp"`
	Evidence   string `json:"evidence"`
}

// ─── Anti-Confirmação (SEARCH_PROTOCOL §26-§27) ───────────────────────────────

// Hypothesis é uma suposição a testar — a busca investiga evidência A FAVOR e
// CONTRA (§27: o objetivo é a verdade operacional, não confirmar a primeira).
type SearchHypothesis struct {
	// Statement é a hipótese (ex.: "X causa Y").
	Statement string `json:"statement"`
	// Supporting é a evidência a favor.
	Supporting []string `json:"supporting,omitempty"`
	// Contradicting é a evidência CONTRA (obrigatória de procurar).
	Contradicting []string `json:"contradicting,omitempty"`
	// Verified é true quando a evidência é suficiente para decidir.
	Verified bool `json:"verified"`
	// Verdict é o resultado da investigação (confirmada/refutada/inconclusiva).
	Verdict string `json:"verdict"`
}
