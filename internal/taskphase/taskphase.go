// Package taskphase detecta a fase da implementação em curso e ajusta
// os parâmetros de busca de acordo.
//
// É a camada NOVA do Task-Aware Search (ADR-045 §2.3). NÃO substitui o
// search — AJUSTA os parâmetros de busca (Limit, EnableGraph, etc.) conforme
// a fase detectada.
//
// Determinístico: sem LLM, sem ML — regras baseadas em sinais do estado
// de implementação. Mesma entrada → mesmo resultado.
package taskphase

import (
	"math"
	"strings"

	"github.com/CoscaAI/cosca/internal/search"
)

// ─── Phase enum ──────────────────────────────────────────────────────────────

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

// ─── Signals ─────────────────────────────────────────────────────────────────

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

// ─── Detection result ────────────────────────────────────────────────────────

// PhaseDetection é o resultado da detecção de fase.
type PhaseDetection struct {
	// Phase é a fase detectada.
	Phase Phase

	// Confidence é a confiança na detecção (0.0–1.0).
	Confidence float64

	// Reason é a justificativa determinística (pt-BR, para auditoria).
	Reason string
}

// ─── Detection algorithm ─────────────────────────────────────────────────────

// DetectPhase detecta a fase da implementação de forma determinística.
// Nil-safe: signals nil → retorna PhaseExploration com confiança 0.
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

// ─── SearchParams adjustment ─────────────────────────────────────────────────

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

// explorationParams: recall amplo, entendimento leve.
//   - Limit=30 (mais resultados)
//   - FTS + Vector (sem grafo)
//   - Facets habilitadas (entender o espaço)
//   - MinScore não definido (aceita tudo)
func explorationParams(p search.SearchParams) search.SearchParams {
	p.Limit = 30
	p.EnableFTS = true
	p.EnableVector = true
	p.EnableGraph = false
	p.EnableFacets = true
	p.MinScore = 0
	return p
}

// implementationParams: entendimento profundo.
//   - Limit=20 (padrão)
//   - FTS + Vector + Graph (padrões do projeto)
//   - Sem facets
//   - MinScore moderado (filtra ruído genérico)
func implementationParams(p search.SearchParams) search.SearchParams {
	p.Limit = 20
	p.EnableFTS = true
	p.EnableVector = true
	p.EnableGraph = true
	p.EnableFacets = false
	p.MinScore = 0.15
	return p
}

// verificationParams: precisão cirúrgica.
//   - Limit=10 (poucos, focados)
//   - FTS + Vector (sem grafo)
//   - Sem facets
//   - MinScore alto (só resultados muito relevantes)
func verificationParams(p search.SearchParams) search.SearchParams {
	p.Limit = 10
	p.EnableFTS = true
	p.EnableVector = true
	p.EnableGraph = false
	p.EnableFacets = false
	p.MinScore = 0.30
	return p
}

// ─── Term lists (determinísticas, exportadas) ────────────────────────────────

// ExplorationTerms são termos que indicam fase de exploração.
// Usado por buildPhaseSignals (F4) para classificar a query.
var ExplorationTerms = map[string]struct{}{
	"como funciona": {}, "ver": {}, "mostrar": {}, "buscar": {},
	"entender": {}, "explorar": {}, "analisar": {}, "qual é": {},
	"onde está": {}, "o que é": {}, "listar": {}, "procurar": {},
}

// ImplementationTerms são termos que indicam fase de implementação.
var ImplementationTerms = map[string]struct{}{
	"implementar": {}, "criar": {}, "adicionar": {}, "escrever": {},
	"desenvolver": {}, "construir": {}, "montar": {}, "gerar": {},
	"endpoint": {}, "handler": {}, "rota": {}, "função": {},
}

// VerificationTerms são termos que indicam fase de verificação.
var VerificationTerms = map[string]struct{}{
	"test": {}, "testar": {}, "verify": {}, "verificar": {},
	"check": {}, "checar": {}, "validate": {}, "validar": {},
	"assert": {}, "assegurar": {}, "rodar": {}, "executar": {},
	"passou": {}, "falhou": {}, "bug": {}, "erro": {},
}

// ContainsTerm verifica se o texto (já em lower-case) contém algum dos termos
// do mapa. É uma helper pública usada por F4 (buildPhaseSignals).
func ContainsTerm(lowerText string, terms map[string]struct{}) bool {
	for term := range terms {
		if strings.Contains(lowerText, term) {
			return true
		}
	}
	return false
}
