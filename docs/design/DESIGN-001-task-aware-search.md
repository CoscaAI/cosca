# DESIGN-001: Task-Aware Search (TAS) — Design Técnico Detalhado

> **Owner:** cosca-architecture | **ADR:** ADR-045 (APROVADO) | **Status:** DESIGN PRONTO PARA IMPLEMENTAÇÃO
> **Data:** 2026-09-08 | **Autor:** cosca-architecture (Architecture Chief)

---

## Índice

1. [Visão Geral e Princípios](#1-visão-geral-e-princípios)
2. [TaskProfile — Perfil de Tarefa (taskaffinity)](#2-taskprofile--perfil-de-tarefa)
3. [Re-ponderação por Afinidade (taskaffinity)](#3-re-ponderação-por-afinidade)
4. [TaskPhase — Detecção de Fase (taskphase)](#4-taskphase--detecção-de-fase)
5. [Integração no Pipeline](#5-integração-no-pipeline)
6. [Pontos de Teste e Validação](#6-pontos-de-teste-e-validação)
7. [Mapeamento de Implementação por Fase](#7-mapeamento-de-implementação-por-fase)

---

## 1. Visão Geral e Princípios

### 1.1 O que é NOVO vs. EXISTENTE

| Componente | Status | Pacote Go |
|---|---|---|
| `TaskProfile` (perfil de tarefa) | **NOVO** | `internal/taskaffinity` |
| `DeriveProfile()` (derivação) | **NOVO** | `internal/taskaffinity` |
| `AffinityRerank()` (re-ponderação) | **NOVO** | `internal/taskaffinity` |
| `TaskPhase` (detecção de fase) | **NOVO** | `internal/taskphase` |
| `DetectPhase()` (detecção) | **NOVO** | `internal/taskphase` |
| `PhaseSearchParams()` (ajuste por fase) | **NOVO** | `internal/taskphase` |
| `Engine.Search()` | **EXISTENTE** | `internal/search` |
| `modlink.SearchScope` | **EXISTENTE** | `internal/modlink` |
| `ranking.Ranker` | **EXISTENTE** | `internal/ranking` |
| `contextcompile.Compile()` | **EXISTENTE** | `internal/contextcompile` |
| `contextpipeline.BuildContext()` | **EXISTENTE** (adaptada) | `internal/contextpipeline` |

### 1.2 Princípios de Design

1. **Aditivo, não substitutivo**: TAS re-pondera RESULTADOS existentes, não substitui o search/ranking/modlink.
2. **Determinístico**: sem LLM, sem ML — regras baseadas em tokens e paths.
3. **Testável**: cada componente tem entrada→saída previsível.
4. **Retrocompatível**: sem TaskProfile, tudo funciona exatamente como hoje (nil-safe).
5. **Reusa o existente**: `modlink.SearchScope` roteia; `ranking.Ranker` re-rankeia; TAS adiciona a dimensão de afinidade e fase.

### 1.3 Fluxo de Dados

```
Estado de Implementação (prompt, workspace, arquivos)
        │
        ▼
  ┌─────────────────────────────┐
  │  DeriveProfile(state)        │  ← NOVO: internal/taskaffinity
  │  → TaskProfile               │
  │    {Task, Stack, Target,     │
  │     Phase, Affinity}         │
  └──────────┬──────────────────┘
             │
     ┌───────┴────────┐
     ▼                ▼
  ┌────────────┐  ┌───────────────────────┐
  │ DetectPhase │  │ PhaseSearchParams()    │
  │ (taskphase) │  │ → SearchParams ajustado│
  └─────┬──────┘  └───────────┬───────────┘
        │                     │
        └──────────┬──────────┘
                   ▼
  ┌────────────────────────────────────────────┐
  │  Engine.Search(ctx, adjustedParams)         │  ← EXISTENTE (search)
  │  → []SearchResult                           │
  └──────────────────┬─────────────────────────┘
                     ▼
  ┌────────────────────────────────────────────┐
  │  AffinityRerank(results, profile)           │  ← NOVO: internal/taskaffinity
  │  → []SearchResult (re-ponderados)           │
  └──────────────────┬─────────────────────────┘
                     ▼
  ┌────────────────────────────────────────────┐
  │  contextcompile.Compile(input)              │  ← EXISTENTE (fact/evidence orientados)
  └────────────────────────────────────────────┘
```

---

## 2. TaskProfile — Perfil de Tarefa

### 2.1 Estrutura de Dados (Go Type)

```go
// Package taskaffinity deriva o perfil de tarefa do estado de implementação
// em curso e fornece re-ponderação de resultados por afinidade.
//
// É a camada NOVA do Task-Aware Search (ADR-045 §2.2). NÃO substitui o
// search/ranking/modlink — ADICIONA a dimensão de afinidade com a tarefa.
package taskaffinity

import (
    "path/filepath"
    "sort"
    "strings"
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
```

### 2.2 Input para Derivação

```go
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
```

### 2.3 Derivação — Assinatura de Função

```go
// DeriveProfile é a função PRIMÁRIA de derivação do perfil de tarefa.
// Recebe o estado de implementação e devolve o perfil com confiança.
// Determinística: mesma entrada → mesmo perfil.
//
// Nil-safe: ImplementaçãoState nil → retorna nil (comportamento retrocompatível).
func DeriveProfile(state *ImplementaçãoState) *TaskProfile
```

### 2.4 Algoritmo de Derivação

#### 2.4.1 TASK → extração de termos

```go
func extractTaskTokens(prompt string) []string
```

**Algoritmo:**
1. Tokenizar o prompt (reutilizar `tokenize()` do `internal/ranking`).
2. Remover stopwords (artigos, preposições, pronomes — lista fixa determinística).
3. Deduplicar e ordenar alfabeticamente.

**Exemplo:**
```
"implementar endpoint REST em Go para o módulo de autenticação"
→ ["autenticação", "endpoint", "go", "implementar", "módulo", "rest"]
```

#### 2.4.2 STACK → detecção determinística

```go
func detectStack(state *ImplementaçãoState) []string
```

**Algoritmo (regras em ordem de prioridade):**

| Sinal | Stack detectado | Confiança |
|---|---|---|
| `state.GoModExists` | `["go"]` | 0.9 |
| `state.PackageJSONExists` | `["node", "typescript"]` (se `tsconfig.json` existe senão `["node"]`) | 0.8 |
| Extensão `.go` em OpenFiles/RecentFiles | `["go"]` | 0.7 |
| Extensão `.ts`/`.tsx` em OpenFiles/RecentFiles | `["typescript"]` | 0.7 |
| Extensão `.py` em OpenFiles/RecentFiles | `["python"]` | 0.7 |
| Extensão `.rs` em OpenFiles/RecentFiles | `["rust"]` | 0.7 |
| Path contém `internal/` | `["go"]` (heurística do projeto Cosca) | 0.6 |
| Nenhum sinal | `[]` (vazio) | 0.0 |

**Detecção de framework (específico do projeto):**

| Path em OpenFiles/RecentFiles | Framework |
|---|---|
| Contém `chi` ou `_chi_` | `["chi"]` |
| Contém `sqlite` ou `_sqlite_` | `["sqlite"]` |
| Contém `grpc` ou `_grpc_` | `["grpc"]` |
| Contém `ollama` ou `_ollama_` | `["ollama"]` |
| Contém `onnx` ou `_onnx_` | `["onnxruntime"]` |

**Regra de deduplicação:** stacks ordenados alfabeticamente, sem duplicatas.

#### 2.4.3 TARGET → detecção de alvo

```go
func detectTarget(state *ImplementaçãoState) (target string, module string)
```

**Algoritmo (ordem de prioridade):**

1. **`state.TargetHint` não-vazio**: usar como Target. Extrair TargetModule do segmento mais significativo do path.
2. **`state.OpenFiles` não-vazio**: usar o PRIMEIRO arquivo (o mais recentemente aberto). TargetModule = segmento pai mais significativo.
3. **`state.RecentFiles` não-vazio**: usar o PRIMEIRO arquivo (o mais recentemente modificado).
4. **Nenhum sinal**: Target = "", TargetModule = "".

**Extração de TargetModule (regra determinística):**

```
"internal/api/handlers.go" → TargetModule = "api"
"internal/search/search.go" → TargetModule = "search"
"internal/ranking/ranking.go" → TargetModule = "ranking"
"internal/contextpipeline/contextpipeline.go" → TargetModule = "contextpipeline"
```

**Regra:** pegar o SEGMENTO do path que está IMEDIATAMENTE DEPOIS de `internal/`. Se o path não contém `internal/`, pegar o penúltimo segmento não-vazio.

#### 2.4.4 AFFINITY → geração de termos

```go
func generateAffinity(taskTokens, stack []string, targetModule string) []string
```

**Algoritmo:**
1. Começar com os taskTokens (já tokenizados e limpos).
2. Adicionar os termos do stack.
3. Se TargetModule não-vazio: adicionar o TargetModule e seu "pai" (ex.: `["contextpipeline", "context"]` para `contextpipeline`).
4. Adicionar variações comuns do TargetModule:
   - Se TargetModule = `"api"` → adicionar `"handler"`, `"endpoint"`, `"rest"`.
   - Se TargetModule = `"search"` → adicionar `"query"`, `"retrieval"`, `"ft"`, `"vector"`.
   - Se TargetModule = `"ranking"` → adicionar `"rerank"`, `"score"`, `"relevance"`.
   - Se TargetModule = `"modlink"` → adicionar `"route"`, `"scope"`, `"module"`.
   - Para módulos desconhecidos: adicionar apenas o TargetModule normalizado.
5. Deduplicar e ordenar alfabeticamente.

**Tabela de expansão canônica:**

```go
// moduleSynonyms mapeia módulos do projeto para termos associados que
// ampliam o alcance da afinidade. A tabela é determinística e fechada —
// novos módulos são adicionados explicitamente.
var moduleSynonyms = map[string][]string{
    "api":              {"handler", "endpoint", "rest", "route"},
    "search":           {"query", "retrieval", "fts", "vector", "bm25", "cosine"},
    "ranking":          {"rerank", "score", "relevance", "bm25", "weight"},
    "modlink":          {"route", "scope", "module", "trigger", "domain"},
    "contextcompile":   {"context", "compiler", "facts", "evidence", "section"},
    "contextrouter":    {"context", "level", "confidence", "layer"},
    "contextpipeline":  {"context", "pipeline", "compile", "budget"},
    "orchestration":    {"orchestrate", "executor", "agent", "task"},
    "vector":           {"embedding", "cosine", "similarity", "store"},
    "graph":            {"node", "edge", "neighbor", "bfs", "traversal"},
    "knowledge":        {"knowledge", "fact", "evidence", "epistemic"},
    "sqlite":           {"database", "fts5", "schema", "migration"},
    "memory":           {"memory", "learning", "snapshot"},
    "ranking":          {"score", "bm25", "graph", "freshness", "popularity"},
}
```

#### 2.4.5 Confidence → cálculo

```go
func computeConfidence(profile *TaskProfile) float64
```

**Fórmula:**

```
confidence = 0.0
if profile.Task != "":           confidence += 0.30
if len(profile.Stack) > 0:       confidence += 0.25
if profile.Target != "":         confidence += 0.25
if len(profile.Affinity) > 3:    confidence += 0.20
else if len(profile.Affinity) > 0: confidence += 0.10
```

**Semântica:** confidence ∈ [0.0, 1.0]. Quanto maior, mais confiança na derivação → mais força na re-ponderação.

### 2.5 Exemplo Completo de Derivação

```go
state := &ImplementaçãoState{
    Prompt:       "implementar endpoint REST em Go para o módulo de autenticação",
    WorkingDir:   "C:/Users/Henrique/Documents/cosca",
    OpenFiles:    []string{"internal/api/handlers.go", "internal/api/middleware.go"},
    RecentFiles:  []string{"internal/api/handlers.go"},
    GoModExists:  true,
    TargetHint:   "",
}

// Resultado:
TaskProfile{
    Task:         "implementar endpoint REST em Go para o módulo de autenticação",
    Stack:        ["go"],                         // detectado por go.mod
    Target:       "internal/api/handlers.go",     // primeiro OpenFile
    TargetModule: "api",                          // segmento após "internal/"
    Affinity:     [                             // taskTokens + stack + module expansion
        "api", "autenticação", "endpoint", "go",
        "handler", "implementar", "rest", "route",
    ],
    Confidence:   0.95,                           // todos os campos preenchidos
}
```

---

## 3. Re-ponderação por Afinidade

### 3.1 Princípio

A re-ponderação é um **pós-processamento** sobre os resultados JÁ re-rankeados pelo `ranking.Ranker`. Ela NÃO substitui o ranking — ela APLICA um bônus proporcional à sobreposição entre os termos do resultado e os termos de afinidade do perfil da tarefa.

**Posição no pipeline de busca** (search.go):

```
FTS + Vector + Graph → Scope Confinement → Epistemic Confinement → Re-rank (ranking.Ranker) → Meaning-First → AFFINITY RERANK → Offset/Limit
```

A affinity rerank acontere DEPOIS do ranking multi-fator e ANTES do offset/limit. Isso é crucial: ela opera sobre os resultados já ordenados por relevância, não sobre os brutos.

### 3.2 Assinatura de Função

```go
// AffinityRerank aplica a re-ponderação por afinidade sobre resultados
// JÁ re-rankeados. É um pós-processamento determinístico e puro:
// mesmas entradas → mesmos resultados.
//
// Nil-safe: profile nil ou sem Affinity → retorna results inalterados.
// O peso da re-ponderação é modulado pela Confidence do perfil.
func AffinityRerank(results []search.SearchResult, profile *TaskProfile) []search.SearchResult
```

### 3.3 Fórmula de Re-ponderação

Para cada resultado `r` na lista já ordenada:

```
score_original = r.Score  // resultado do ranking.Ranker (já normalizado 0–1)
affinity_boost = computeAffinityBoost(r, profile)
r.Score = score_original + affinity_boost
```

Onde:

```go
// computeAffinityBoost calcula o bônus de afinidade para um resultado.
// O bônus é proporcional à fração de termos de afinidade que aparecem no
// resultado, modulado pela confiança do perfil.
func computeAffinityBoost(r search.SearchResult, profile *TaskProfile) float64 {
    if !profile.HasAffinity() || profile.Confidence <= 0 {
        return 0
    }

    // 1. Extrair tokens do resultado (normalizados)
    resultTokens := tokenizeResult(r)

    // 2. Calcular sobreposição
    affinitySet := profile.AffinitySet()
    overlap := 0
    for token := range resultTokens {
        if _, ok := affinitySet[token]; ok {
            overlap++
        }
    }

    // 3. Fração de cobertura (0.0–1.0)
    coverage := float64(overlap) / float64(len(profile.Affinity))

    // 4. Boost ponderado pela confiança
    // Máximo boost = 0.15 (15% do range de score).
    // Isso garante que a afinidade SOBE resultados relevantes mas NÃO
    // domina o ranking multi-fator (que tem BM25/vector/graph/fresh/pop).
    const maxAffinityBoost = 0.15
    return maxAffinityBoost * coverage * profile.Confidence
}
```

**Por que 0.15 como teto?**

O ranking existente usa pesos que somam 1.0 (BM25 0.25 + Vector 0.35 + Graph 0.20 + Fresh 0.10 + Pop 0.10). O affinity boost de até 0.15 adiciona ~15% ao range, o que é suficiente para:
- **Subir** um resultado do projeto (match alto de afinidade) sobre um genérico.
- **Não dominar** — um resultado genérico com score de ranking 0.9 não será superado por um resultado do projeto com score 0.3 apenas por afinidade.

**Exemplo numérico (cenário realista — scores de ranking próximos):**

```
Resultado A (genérico "REST", score ranking = 0.72):
  affinity coverage = 1/8 = 0.125 (só "rest" bate)
  affinity boost = 0.15 * 0.125 * 0.95 = 0.018
  score final = 0.72 + 0.018 = 0.738

Resultado B (do projeto "internal/api/handlers.go", score ranking = 0.70):
  affinity coverage = 6/8 = 0.75 ("api", "endpoint", "go", "handler", "rest", "route" batem)
  affinity boost = 0.15 * 0.75 * 0.95 = 0.107
  score final = 0.70 + 0.107 = 0.807

→ Resultado B (do projeto) SOBRE Resultado A (genérico) ✅
```

> **⚠️ Nota de calibração (correção 2026-09-08):** o boost de afinidade (teto
> 0.15) **consolida** a posição de resultados do projeto e **resolve empates**,
> mas **NÃO reordena** resultados com grande diferença de score inicial. No
> exemplo acima, a diferença de score de ranking é pequena (0.72 vs 0.70) — o
> boost de 0.107 do projeto supera o de 0.018 do genérico e vira a ordem. Se a
> diferença fosse grande (ex.: 0.80 vs 0.60), o boost não seria suficiente. O
> TAS é um refinamento final sobre o ranking multi-fator — não um "salvador" de
> resultados ruins. Isso é intencional: evita que um perfil mal derivado domine
> o ranking.

### 3.4 Tokenização do Resultado

```go
// tokenizeResult extrai tokens normalizados de um SearchResult para comparação
// com os termos de afinidade. Usa os campos mais ricos: Title, Content,
// DocumentPath e EntityType.
func tokenizeResult(r search.SearchResult) map[string]struct{} {
    text := strings.ToLower(r.Title + " " + r.Content + " " + r.DocumentPath + " " + r.EntityType)
    tokens := tokenize(text) // reutiliza tokenize() do ranking
    set := make(map[string]struct{}, len(tokens))
    for _, t := range tokens {
        set[t] = struct{}{}
    }
    return set
}
```

### 3.5 Impacto no ScoreBreakdown (Auditoria)

Para manter a auditoria do ranking (explain.go), o affinity boost é registrado como um **signal adicional** no `ScoreBreakdown`:

```go
// Extensão do ScoreBreakdown (ranking/explain.go) — O CAMPO É NOVO,
// o struct existente é estendido com um campo opcional.
//
// NOTA: Esta extensão é ADITIVA — o campo Affinity é zero quando TAS
// não está ativo, mantendo retrocompatibilidade total.
```

Na prática, o `AffinityRerank` NÃO modifica o ranking.Ranker — ele opera DEPOIS. A extensão do ScoreBreakdown será feita quando a F5 (medição) integrar os sinais.

### 3.6 Interação com Meaning-First

O `meaningFirstRank` do search.go ordena: vector results primeiro, depois os demais. O `AffinityRerank` acontece DEPOIS do meaning-first:

```
... → Re-rank (multi-fator) → Meaning-First → Affinity Rerank → Offset/Limit
```

Isso preserva o princípio do meaning-first (significado lidera) e adiciona a camada de afinidade como refinamento final.

---

## 4. TaskPhase — Detecção de Fase

### 4.1 Estrutura de Dados (Go Type)

```go
// Package taskphase detecta a fase da implementação em curso e ajusta
// os parâmetros de busca de acordo.
//
// É a camada NOVA do Task-Aware Search (ADR-045 §2.3). NÃO substitui o
// search — AJUSTA os parâmetros de busca (Limit, EnableGraph, etc.) conforme
// a fase detectada.
package taskphase

// Phase representa a fase da implementação.
type Phase int

const (
    // PhaseExploration é o início da tarefa: recall amplo, entendimento leve.
    // O sistema precisa de visão geral do contexto.
    PhaseExploration Phase = iota

    // PhaseImplementation é o meio da tarefa: entendimento profundo.
    // O sistema precisa de padrões do projeto e arquivos relacionados.
    PhaseImplementation

    // PhaseVerification é o fim da tarefa: precisão cirúrgica.
    // O sistema precisa de confinamento ao alvo específico.
    PhaseVerification
)

// String retorna o nome legível da fase.
func (p Phase) String() string {
    switch p {
    case PhaseExploration:
        return "exploration"
    case PhaseImplementation:
        return "implementation"
    case PhaseVerification:
        return "verification"
    default:
        return "unknown"
    }
}

// PhaseSignals são os sinais determinísticos do estado de implementação
// que alimentam a detecção de fase.
type PhaseSignals struct {
    // HasOpenFiles indica se há arquivos abertos no editor.
    HasOpenFiles bool

    // HasRecentModifications indica se há arquivos modificados recentemente.
    HasRecentModifications bool

    // OpenFileCount é o número de arquivos abertos.
    OpenFileCount int

    // TargetIsSpecific indica se o alvo é um arquivo específico (não um módulo).
    // Ex.: "internal/api/handlers.go" = específico; "internal/api" = genérico.
    TargetIsSpecific bool

    // HasTestTerms indica se a query contém termos de verificação.
    // Ex.: "test", "verify", "check", "validate", "assert".
    HasTestTerms bool

    // HasImplementationTerms indica se a query contém termos de implementação.
    // Ex.: "implementar", "criar", "adicionar", "criar endpoint".
    HasImplementationTerms bool

    // HasExplorationTerms indica se a query contém termos de exploração.
    // Ex.: "como funciona", "ver", "mostrar", "buscar", "entender".
    HasExplorationTerms bool

    // SearchHistoryCount é o número de buscas anteriores nesta sessão.
    // 0 = primeira busca (exploração); >2 = possível verificação.
    SearchHistoryCount int

    // TargetHasTests indica se o alvo (ou seu diretório) já tem testes.
    TargetHasTests bool
}

// PhaseDetection é o resultado da detecção de fase.
type PhaseDetection struct {
    // Phase é a fase detectada.
    Phase Phase

    // Confidence é a confiança na detecção (0.0–1.0).
    Confidence float64

    // Reason é a justificativa determinística (pt-BR, para auditoria).
    Reason string
}
```

### 4.2 Detecção — Assinatura de Função

```go
// DetectPhase detecta a fase da implementação de forma determinística.
// Nil-safe: signals nil → retorna PhaseExploration com confiança 0.
func DetectPhase(signals *PhaseSignals) PhaseDetection
```

### 4.3 Algoritmo de Detecção

A detecção é uma **árvore de decisão ponderada** com pontuação:

```go
func DetectPhase(signals *PhaseSignals) PhaseDetection {
    if signals == nil {
        return PhaseDetection{Phase: PhaseExploration, Confidence: 0, Reason: "sem sinais"}
    }

    // ── Sinais de Verificação (prioridade alta) ──
    scoreVerify := 0.0
    if signals.HasTestTerms {
        scoreVerify += 0.40
    }
    if signals.TargetIsSpecific && signals.TargetHasTests {
        scoreVerify += 0.30
    }
    if signals.SearchHistoryCount >= 3 {
        scoreVerify += 0.15
    }
    if signals.OpenFileCount == 1 {
        scoreVerify += 0.15 // foco em um único arquivo = verificação
    }

    // ── Sinais de Implementação ──
    scoreImplement := 0.0
    if signals.HasImplementationTerms {
        scoreImplement += 0.35
    }
    if signals.HasOpenFiles && signals.OpenFileCount >= 2 {
        scoreImplement += 0.25
    }
    if signals.HasRecentModifications {
        scoreImplement += 0.25
    }
    if signals.TargetIsSpecific {
        scoreImplement += 0.15
    }

    // ── Sinais de Exploração ──
    scoreExplore := 0.0
    if signals.HasExplorationTerms {
        scoreExplore += 0.35
    }
    if !signals.HasOpenFiles {
        scoreExplore += 0.30
    }
    if signals.SearchHistoryCount == 0 {
        scoreExplore += 0.20
    }
    if signals.OpenFileCount == 0 {
        scoreExplore += 0.15
    }

    // ── Decisão: maior score vence ──
    max := scoreVerify
    phase := PhaseVerification
    reason := "termos de verificação e alvo específico"

    if scoreImplement > max {
        max = scoreImplement
        phase = PhaseImplementation
        reason = "termos de implementação e arquivos abertos"
    }
    if scoreExplore > max {
        max = scoreExplore
        phase = PhaseExploration
        reason = "busca inicial sem contexto de implementação"
    }

    // Confidence = score do vencedor (0.0–1.0)
    conf := math.Min(max, 1.0)

    return PhaseDetection{Phase: phase, Confidence: conf, Reason: reason}
}
```

### 4.4 Tabela de Sinais e Pesos

| Sinal | Exploração | Implementação | Verificação |
|---|---|---|---|
| `HasExplorationTerms` | +0.35 | — | — |
| `!HasOpenFiles` | +0.30 | — | — |
| `SearchHistoryCount == 0` | +0.20 | — | — |
| `OpenFileCount == 0` | +0.15 | — | — |
| `HasImplementationTerms` | — | +0.35 | — |
| `HasOpenFiles && OpenFileCount >= 2` | — | +0.25 | — |
| `HasRecentModifications` | — | +0.25 | — |
| `TargetIsSpecific` | — | +0.15 | — |
| `HasTestTerms` | — | — | +0.40 |
| `TargetIsSpecific && TargetHasTests` | — | — | +0.30 |
| `SearchHistoryCount >= 3` | — | — | +0.15 |
| `OpenFileCount == 1` | — | — | +0.15 |

### 4.5 Terms Lists (Determinísticas)

```go
// explorationTerms são termos que indicam fase de exploração.
var explorationTerms = map[string]struct{}{
    "como funciona": {}, "ver": {}, "mostrar": {}, "buscar": {},
    "entender": {}, "explorar": {}, "analisar": {}, "qual é": {},
    "onde está": {}, "o que é": {}, "listar": {}, "buscar": {},
}

// implementationTerms são termos que indicam fase de implementação.
var implementationTerms = map[string]struct{}{
    "implementar": {}, "criar": {}, "adicionar": {}, "escrever": {},
    "desenvolver": {}, "construir": {}, "montar": {}, "gerar": {},
    "endpoint": {}, "handler": {}, "rota": {}, "função": {},
}

// verificationTerms são termos que indicam fase de verificação.
var verificationTerms = map[string]struct{}{
    "test": {}, "testar": {}, "verify": {}, "verificar": {},
    "check": {}, "checar": {}, "validate": {}, "validar": {},
    "assert": {}, "assegurar": {}, "rodar": {}, "executar": {},
    "passou": {}, "falhou": {}, "bug": {}, "erro": {},
}
```

### 4.6 Ajuste de SearchParams por Fase

```go
// PhaseSearchParams ajusta os SearchParams conforme a fase detectada.
// Cada fase muda os parâmetros de busca para otimizar recall/precisão/custo.
func PhaseSearchParams(params search.SearchParams, detection PhaseDetection) search.SearchParams {
    switch detection.Phase {
    case PhaseExploration:
        return explorationParams(params)
    case PhaseImplementation:
        return implementationParams(params)
    case PhaseVerification:
        return verificationParams(params)
    default:
        return params
    }
}
```

#### 4.6.1 Parâmetros por Fase

**Exploração (recall amplo, entendimento leve):**

```go
func explorationParams(p search.SearchParams) search.SearchParams {
    p.Limit = 30              // mais resultados para visão ampla
    p.EnableFTS = true        // FTS primário (barato)
    p.EnableVector = true     // vetor complementar
    p.EnableGraph = false     // sem grafo (custo alto, pouco valor em exploração)
    p.EnableFacets = true     // facets para entender o espaço
    // MinScore não definido: aceita tudo (recall amplo)
    return p
}
```

**Implementação (entendimento profundo):**

```go
func implementationParams(p search.SearchResult) search.SearchParams {
    p.Limit = 20              // padrão
    p.EnableFTS = true        // FTS para recall lexical
    p.EnableVector = true     // vetor primário (entendimento semântico)
    p.EnableGraph = true      // GRAFO ATIVADO: padrões do projeto, arquivos relacionados
    p.EnableFacets = false    // facets desnecessárias (já sabe o que procura)
    // MinScore moderado: filtra ruído genérico
    p.MinScore = 0.15
    return p
}
```

**Verificação (precisão cirúrgica):**

```go
func verificationParams(p search.SearchParams) search.SearchParams {
    p.Limit = 10              // poucos resultados, focados
    p.EnableFTS = true        // FTS para match exato
    p.EnableVector = true     // vetor para refinamento semântico
    p.EnableGraph = false     // sem grafo (já sabe onde está)
    p.EnableFacets = false    // sem facets
    // MinScore alto: só resultados muito relevantes
    p.MinScore = 0.30
    return p
}
```

#### 4.6.2 Tabela Resumo: Fase × Parâmetro

| Parâmetro | Exploração | Implementação | Verificação |
|---|---|---|---|
| `Limit` | 30 | 20 | 10 |
| `EnableFTS` | true | true | true |
| `EnableVector` | true | true | true |
| `EnableGraph` | false | **true** | false |
| `EnableFacets` | true | false | false |
| `MinScore` | 0 (não definido) | 0.15 | 0.30 |

**Mudança-chave:** O grafo (`EnableGraph`) só é ativado na fase de Implementação — é onde o "entendimento profundo" do padrão do projeto importa (arquivos relacionados, dependências entre módulos).

---

## 5. Integração no Pipeline

### 5.1 Ponto de Injeção

O `contextpipeline.BuildContext()` é adaptado para aceitar o `TaskProfile` e usá-lo ANTES de compilar o contexto. A adaptação é **aditiva** — quando o profile é nil, nada muda.

### 5.2 Novos Tipos

```go
// TaskContext é o que o Task-Aware Search fornece ao pipeline.
// É opcional: nil = pipeline se comporta como hoje.
type TaskContext struct {
    // Profile é o perfil de tarefa derivado.
    Profile *taskaffinity.TaskProfile

    // PhaseDetection é a detecção de fase.
    PhaseDetection *taskphase.PhaseDetection
}
```

### 5.3 Adaptação do PipelineData

```go
// Extensão do PipelineData (orchestration/types.go) — campo NOVO, opcional.
// Nil-safe: quando ausente, o pipeline ignora TAS completamente.
//
// Adicionado ao PipelineData:
//   TaskContext *TaskContext `json:"task_context,omitempty"`
```

### 5.4 Fluxo Adaptado do BuildContext

```go
// BuildContext — versão adaptada (adaptada, não substituída).
//
// MUDANÇAS vs. versão atual:
// 1. ANTES de chamar engine.Search: detecta fase e ajusta SearchParams.
// 2. DEPOIS de engine.Search: aplica AffinityRerank sobre os resultados.
// 3. Feed ao contextcompile: inclui TaskProfile no CompileInput.
//
// Quando TaskContext é nil: fluxo idêntico ao atual (zero regressão).
func (p *Pipeline) BuildContext(prompt string, data *orchestration.PipelineData) (string, orchestration.PipelineLevel) {
    // ... fluxo existente (nível, confiança) ...

    // ═══ NOVO: Task-Aware Search ═══
    if data.TaskContext != nil && data.TaskContext.Profile != nil {
        profile := data.TaskContext.Profile
        detection := data.TaskContext.PhaseDetection

        // 1. Ajustar SearchParams pela fase (ANTES da busca)
        // (isto afeta como o knowledge search é feito downstream)
        if detection != nil {
            data.SearchParams = taskphase.PhaseSearchParams(data.SearchParams, *detection)
        }

        // 2. AAffinityRerank sobre os resultados (DEPOIS da busca)
        if data.KnowledgeResults != nil && len(data.KnowledgeResults.Results) > 0 {
            data.KnowledgeResults.Results = taskaffinity.AffinityRerank(
                data.KnowledgeResults.Results, profile,
            )
        }
    }
    // ═══ FIM NOVO ═══

    // ... fluxo existente (compile, budget, metrics) ...

    // ═══ NOVO: Profile no CompileInput ═══
    if data.TaskContext != nil && data.TaskContext.Profile != nil {
        // Enriquecer a seção STATE com informações do perfil de tarefa
        // (ex.: "stack=go · target=internal/api/handlers.go · phase=implementation")
        cc.EnrichWithProfile(data.TaskContext.Profile)
    }
    // ═══ FIM NOVO ═══

    return cc.Text, orchestration.PipelineLevel{Name: level.String(), Deterministic: deterministic}
}
```

### 5.5 Enriquecimento do CompiledContext

```go
// EnrichWithProfile enriquece o contexto compilado com informações do perfil
// de tarefa. Adiciona à seção STATE o stack, target e fase detectada.
func (cc *CompiledContext) EnrichWithProfile(profile *taskaffinity.TaskProfile) {
    if profile == nil || cc == nil {
        return
    }

    // Encontrar a seção STATE
    for i := range cc.Sections {
        if cc.Sections[i].Name == "STATE" {
            var extras []string
            if profile.HasStack() {
                extras = append(extras, "stack="+strings.Join(profile.Stack, "+"))
            }
            if profile.HasTarget() {
                extras = append(extras, "target="+profile.Target)
            }
            if len(extras) > 0 {
                cc.Sections[i].Content += " · " + strings.Join(extras, " · ")
            }
            break
        }
    }
}
```

### 5.6 Construção do TaskContext (Fora do Pipeline)

O `TaskContext` é construído **ANTES** de chamar `BuildContext`, pelo caller (executor/orchestrator):

```go
// BuildTaskContext é a função que o executor chama para construir o contexto
// de tarefa antes de invocar o pipeline. É o "ponto de entrada" do TAS.
//
// Nil-safe: ImplementaçãoState nil → retorna nil (pipeline ignora TAS).
func BuildTaskContext(state *taskaffinity.ImplementaçãoState) *TaskContext {
    if state == nil {
        return nil
    }

    profile := taskaffinity.DeriveProfile(state)
    if profile == nil {
        return nil
    }

    // Detectar fase a partir dos sinais derivados do state
    signals := buildPhaseSignals(state)
    detection := taskphase.DetectPhase(signals)

    return &TaskContext{
        Profile:         profile,
        PhaseDetection:  &detection,
    }
}

// buildPhaseSignals converte ImplementaçãoState em PhaseSignals.
func buildPhaseSignals(state *taskaffinity.ImplementaçãoState) *taskphase.PhaseSignals {
    return &taskphase.PhaseSignals{
        HasOpenFiles:          len(state.OpenFiles) > 0,
        HasRecentModifications: len(state.RecentFiles) > 0,
        OpenFileCount:         len(state.OpenFiles),
        TargetIsSpecific:      isSpecificTarget(state.TargetHint, state.OpenFiles),
        HasTestTerms:          containsAny(strings.ToLower(state.Prompt), taskphase.VerificationTerms),
        HasImplementationTerms: containsAny(strings.ToLower(state.Prompt), taskphase.ImplementationTerms),
        HasExplorationTerms:   containsAny(strings.ToLower(state.Prompt), taskphase.ExplorationTerms),
        SearchHistoryCount:    0, // será incrementado pelo executor por sessão
        TargetHasTests:        false, // será detectado por filesystem check
    }
}
```

---

## 6. Pontos de Teste e Validação

### 6.1 Testes Unitários por Componente

#### taskaffinity (perfil + re-ponderação)

| Teste | O que valida | Critério |
|---|---|---|
| `TestDeriveProfile_CompleteState` | Derivação com todos os sinais | Todos os campos preenchidos, Confidence ≥ 0.8 |
| `TestDeriveProfile_EmptyState` | Derivação sem sinais | TaskProfile nil ou Confidence = 0 |
| `TestDeriveProfile_StackDetection` | Detecção de stack por go.mod/package.json | Stack correta para cada sinal |
| `TestDeriveProfile_TargetDetection` | Detecção de alvo por OpenFiles | Target e TargetModule corretos |
| `TestDeriveProfile_AffinityGeneration` | Geração de termos de afinidade | Termos incluem taskTokens + stack + moduleExpansion |
| `TestDeriveProfile_Determinism` | Mesma entrada → mesmo resultado | Profile1 == Profile2 (deep equal) |
| `TestAffinityRerank_BoostProjectResults` | Resultados do projeto sobem | Resultado do projeto com alta sobreposição sobe ≥ 2 posições |
| `TestAffinityRerank_DampGenericResults` | Resultados genéricos descem | Resultado genérico com baixa sobreposição desce ≥ 1 posição |
| `TestAffinityRerank_NilSafe` | Profile nil não quebra | Resultados inalterados quando profile é nil |
| `TestAffinityRerank_ZeroAffinity` | Affinity vazia não altera | Resultados inalterados quando Affinity é vazio |
| `TestAffinityRerank_MaxBoostBound` | Boost nunca excede 0.15 | Para qualquer entrada, boost ≤ 0.15 |
| `TestAffinityRerank_Determinism` | Mesma entrada → mesmo resultado | Ordenação idêntica em duas execuções |

#### taskphase (detecção de fase)

| Teste | O que valida | Critério |
|---|---|---|
| `TestDetectPhase_Exploration` | Detecção correta: exploração | Phase = Exploration com Confidence > 0.5 |
| `TestDetectPhase_Implementation` | Detecção correta: implementação | Phase = Implementation com Confidence > 0.5 |
| `TestDetectPhase_Verification` | Detecção correta: verificação | Phase = Verification com Confidence > 0.5 |
| `TestDetectPhase_Ambiguous` | Sinais mistos → fase razoável | Phase é uma das três; não panics |
| `TestDetectPhase_NilSignals` | Nil-safe | Phase = Exploration, Confidence = 0 |
| `TestDetectPhase_Determinism` | Mesma entrada → mesma fase | Identical em duas execuções |
| `TestPhaseSearchParams_Exploration` | Parâmetros de exploração | Limit=30, EnableGraph=false, Facets=true |
| `TestPhaseSearchParams_Implementation` | Parâmetros de implementação | Limit=20, EnableGraph=true, MinScore=0.15 |
| `TestPhaseSearchParams_Verification` | Parâmetros de verificação | Limit=10, EnableGraph=false, MinScore=0.30 |

### 6.2 Testes de Regressão (NÃO REGREDIR)

| Teste | O que valida | Critério |
|---|---|---|
| `TestSearch_BackwardCompatibility` | Busca sem TAS é idêntica | SearchParams sem TaskContext → mesmos resultados |
| `TestRanking_Unmodified` | ranking.Ranker não foi alterado | Todos os testes existentes de ranking passam |
| `TestModlink_Unmodified` | modlink não foi alterado | Todos os testes existentes de modlink passam |
| `TestContextCompile_Unmodified` | contextcompile não foi alterado | Todos os testes existentes de compile passam |
| `TestPipeline_NilTaskContext` | Pipeline sem TAS funciona | BuildContext com TaskContext nil → comportamento idêntico |

### 6.3 Testes de Integração (E2E)

| Teste | O que valida | Critério |
|---|---|---|
| `TestTAS_FullFlow_Exploration` | Fluxo completo: exploração | Query genérica → PhaseExploration → Recall amplo |
| `TestTAS_FullFlow_Implementation` | Fluxo completo: implementação | Query com arquivos abertos → PhaseImplementation → Entendimento profundo |
| `TestTAS_FullFlow_Verification` | Fluxo completo: verificação | Query de teste → PhaseVerification → Precisão cirúrgica |
| `TestTAS_FACTS_OrientedToTask` | FACTS/EVIDENCE orientados à tarefa | ResultadosFACTS contêm mais itens do projeto vs. baseline |

### 6.4 Benchmarks (Performance)

| Benchmark | O que mede | Critério |
|---|---|---|
| `BenchmarkDeriveProfile` | Custo de derivação | < 1ms (CPU puro, sem I/O) |
| `BenchmarkAffinityRerank` | Custo de re-ponderação | < 500μs para 30 resultados |
| `BenchmarkDetectPhase` | Custo de detecção | < 100μs |
| `BenchmarkTAS_Overhead` | Overhead total do TAS | < 2ms (derivação + detecção + re-ponderação) |

---

## 7. Mapeamento de Implementação por Fase

Conforme ADR-045 §4 (fases incrementais):

### F1 — taskaffinity: perfil

**Arquivos NOVOS:**
- `internal/taskaffinity/taskaffinity.go` — TaskProfile, ImplementaçãoState, DeriveProfile()
- `internal/taskaffinity/taskaffinity_test.go` — Testes unitários

**Dependências:**
- `internal/ranking/tokenize()` (reutilizado, não modificado)
- Nenhuma dependência circular (taskaffinity não importa search, ranking, ou modlink)

**Gate:** testes de derivação passam; recall não regride.

### F2 — taskaffinity: re-ponderação

**Arquivos NOVOS:**
- `internal/taskaffinity/rerank.go` — AffinityRerank(), computeAffinityBoost()
- Atualização de `taskaffinity_test.go` com testes de re-ponderação

**Dependências:**
- `internal/search.SearchResult` (tipo existente, não modificado)
- `internal/taskaffinity.TaskProfile` (criado em F1)

**Gate:** recall/precisão sobem; custo não sobe.

### F3 — taskphase: detecção de fase

**Arquivos NOVOS:**
- `internal/taskphase/taskphase.go` — Phase, PhaseSignals, DetectPhase(), PhaseSearchParams()
- `internal/taskphase/taskphase_test.go` — Testes unitários

**Dependências:**
- Nenhuma dependência circular (taskphase não importa search, ranking, ou modlink)
- `internal/search.SearchParams` (tipo existente, não modificado)

**Gate:** regras determinísticas testadas.

### F4 — Integração no pipeline

**Arquivos ADAPTADOS:**
- `internal/contextpipeline/contextpipeline.go` — adaptação aditiva de BuildContext()
- `internal/orchestration/types.go` — campo adicional TaskContext no PipelineData
- `internal/contextcompile/contextcompile.go` — método EnrichWithProfile()

**Arquivos NOVOS:**
- `internal/taskaffinity/buildcontext.go` — BuildTaskContext() (ponto de entrada do TAS)

**Gate:** E2E: FACTS/EVIDENCE orientados à tarefa.

### F5 — Medição

**Arquivos ADAPTADOS:**
- `internal/ranking/explain.go` — campo Affinity no ScoreBreakdown (opcional)
- `internal/contextmetrics/` — métricas de TAS (overhead, boost médio por fase)

**Gate:** recall/precisão/custo por fase mensuráveis.

---

## 8. Diagrama de Dependências

```
                    ┌──────────────────────┐
                    │  internal/orchestration │
                    │  (PipelineData → TC)    │
                    └──────────┬───────────┘
                               │ TaskContext
                               ▼
┌─────────────┐    ┌───────────────────────────┐    ┌─────────────────┐
│ taskaffinity │◄───│  contextpipeline           │───►│ contextcompile  │
│ (perfil +    │    │  (BuildContext adaptado)    │    │ (EnrichWithProfile)│
│  re-pondera) │    └───────────┬───────────────┘    └─────────────────┘
└──────┬──────┘                │ SearchParams ajustado
       │                       ▼
       │              ┌─────────────────┐
       │              │  taskphase       │
       │              │  (DetectPhase +  │
       │              │   PhaseParams)   │
       │              └────────┬────────┘
       │                       │
       ▼                       ▼
┌──────────────────────────────────────┐
│         search.Engine.Search()       │
│         (EXISTENTE, não modificado)  │
└──────────────────────────────────────┘
```

---

## 9. Decisões de Design e Justificativas

### 9.1 Por que pós-processamento e não integração no ranking?

**Decisão:** AffinityRerank é um pós-processamento DEPOIS do ranking.Ranker, não um signal adicional dentro do Rank.

**Justificativa:**
- O ranking.Ranker é um componente genérico e bem testado. Adicionar um signal de "afinidade" acoplaria o ranking a um conceito de tarefa que pode mudar.
- O pós-processamento é mais fácil de testar isoladamente.
- O boost de 0.15 é calibrável sem alterar os pesos do ranking (BM25/vector/graph/fresh/pop).
- Retrocompatível: sem TaskProfile, o pós-processamento não roda.

### 9.2 Por que Confidence modula o boost?

**Decisão:** O affinity_boost é multiplicado pela Confidence do perfil.

**Justificativa:**
- Um perfil com alta confiança (todos os campos derivados) deve ter mais impacto na busca.
- Um perfil com baixa confiança (só o prompt, sem stack/target) deve ter impacto mínimo — é basicamente a busca atual.
- Isso previne enviesamento: um perfil mal derivado não domina o ranking.

### 9.3 Por que o grafo só é ativado na fase de Implementação?

**Decisão:** `EnableGraph = true` apenas na fase de Implementação.

**Justificativa:**
- **Exploração:** o grafo é caro (BFS por resultado) e o usuário quer visão geral — FTS+vetor bastam.
- **Implementação:** o usuário precisa entender o padrão do projeto (dependências, arquivos relacionados) — o grafo é valioso aqui.
- **Verificação:** o usuário já sabe onde está — o grafo adiciona custo sem valor.
- Isso otimiza custo por fase (ADR-031).

### 9.4 Por que a re-ponderação é determinística?

**Decisão:** AffinityRerank não usa LLM, ML, ou任何非确定性 logic.

**Justificativa:**
- O Cosca valoriza determinismo (contrato do modlink, ranking explicável).
- Testabilidade: mesma entrada → mesmo resultado.
- Auditabilidade: o bônus é calculável e explicável.

### 9.5 Por que moduleSynonyms é tabela fechada?

**Decisão:** A tabela de expansão de módulos é explícita e fechada, não aprendida.

**Justificativa:**
- O projeto é pequeno e controlado. A tabela é auditável.
- Uma tabela "aprendida" introduziria não-determinismo e complexidade.
- Novos módulos são adicionados explicitamente (um PR de 5 linhas).

---

## 10. Riscos e Mitigações

| Risco | Mitigação |
|---|---|
| Perfil mal derivado enviesa busca | Confidence modula o boost; boost ≤ 0.15; testes de regressão |
| Fase mal detectada muda comportamento indevido | Regras determinísticas conservadoras; fallback para Exploration |
| Overhead de computação | Todos os cálculos são O(n) com n pequeno; benchmark < 2ms |
| Breaking change no pipeline | Campo TaskContext é nil-safe; pipeline sem ele funciona idêntico |
| Complexidade de manutenção | Dois pacotes pequenos e isolados; zero acoplamento com search/ranking |

---

> **Lei da Família:** Este design é o mapa. A implementação é o território.
> Os especialistas (`cosca-ai` para taskaffinity, `cosca-backend` para integração)
> recebem este design como contrato — e devem validá-lo contra os testes da seção 6
> antes de declarar "marcha completada".
