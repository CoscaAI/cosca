package engine

// ═══════════════════════════════════════════════════════════════════════════════
// Memory & Knowledge Port Interfaces
//
// CONTRATO ÚNICO: estas portas vivem em internal/orchestration/ports.go — o
// pacote de orquestração é o DONO dos contratos de memória/conhecimento que
// atravessam camadas. Este arquivo apenas RE-EXPORTA os tipos via aliases para
// que a AgentEngine e seus consumidores (engine_builder) usem a MESMA
// definição — sem fork de structs.
//
// (Antes havia aqui cópias campo-a-campo "para evitar import circular"; isso
// era um sintoma de dependência invertida — orchestration NÃO importa engine,
// então engine pode importar orchestration sem ciclo. A unificação elimina a
// divergência de campos: Scope/NoRoute/Epistemic agora vivem no contrato único.)
//
// Produção implementations live in internal/memory/ and internal/knowledge/,
// adaptadas em internal/adapter/ e internal/engine/.
// ═══════════════════════════════════════════════════════════════════════════════

import (
	"github.com/CoscaAI/cosca/internal/orchestration"
)

// MemoryRetriever searches and retrieves records from the memory engine.
type MemoryRetriever = orchestration.MemoryRetriever

// MemoryStorer persists results into the memory engine.
type MemoryStorer = orchestration.MemoryStorer

// KnowledgeSearcher searches the knowledge engine for information relevant
// to the current request.
type KnowledgeSearcher = orchestration.KnowledgeSearcher

// MemoryRecord is the memory contract shared with orchestration.
type MemoryRecord = orchestration.MemoryRecord

// MemorySearchOptions filters memory searches within the engine layer.
type MemorySearchOptions = orchestration.MemorySearchOptions

// KnowledgeSearchParams carries search parameters for the knowledge engine.
type KnowledgeSearchParams = orchestration.KnowledgeSearchParams

// KnowledgeSearchResult is a single knowledge-base search hit.
type KnowledgeSearchResult = orchestration.KnowledgeSearchResult

// KnowledgeSearchResults bundles search hits with query metadata.
type KnowledgeSearchResults = orchestration.KnowledgeSearchResults
