// Package cost — Fase 1 do ADR-031: cache de resultado de agente VERSION-SAFE.
//
// A tese do Don+professor (2026-08-29): o inimigo não é o volume de tokens,
// é o DESPERDÍCIO — contexto repetido / investigação redundante. A Fase 0
// (internal/cost) mede UsefulWork/tokens. A Fase 1 evita o re-trabalho:
//
//	duas tarefas IGUAIS com o MESMO conhecimento → cache hit → reutiliza
//	duas tarefas iguais em VERSÕES DIFERENTES de conhecimento → cache NÃO mistura
//
// A chave do cache é (fingerprint da tarefa, knowledge_snapshot):
//   - fingerprint: SHA-256 canônico da descrição da tarefa (normalizada) —
//     segue o padrão do cachefingerprint da busca (normalizar antes de hashear);
//   - knowledge_snapshot: resolvido do lock.yaml (ADR-029) — é a âncora
//     version-safe: se o conhecimento mudou, o cache NÃO pode reutilizar o
//     resultado antigo.
//
// Version-safe por construção: o snapshot entra na chave, não no valor.
// Um resultado cacheado sob snapshot A NUNCA é servido sob snapshot B.
package cost

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ResultCache é o cache de resultado de agente version-safe (ADR-031 Fase 1).
// Thread-safe (RWMutex); TTL opcional por entrada.
type ResultCache struct {
	mu      sync.RWMutex
	ttl     time.Duration
	entries map[string]cacheEntry
}

// cacheEntry é uma entrada do cache com validade temporal.
type cacheEntry struct {
	value     string
	createdAt time.Time
}

// NewResultCache cria um cache vazio com TTL (0 = sem expiração).
func NewResultCache(ttl time.Duration) *ResultCache {
	return &ResultCache{
		ttl:     ttl,
		entries: make(map[string]cacheEntry),
	}
}

// key é a chave version-safe: (fingerprint da tarefa, knowledge_snapshot).
// O snapshot entra na chave — nunca no valor — garantindo que versões
// diferentes de conhecimento não misturem resultados.
func key(taskFingerprint, knowledgeSnapshot string) string {
	return taskFingerprint + "@" + knowledgeSnapshot
}

// Get devolve o resultado cacheado para (tarefa, snapshot). Retorna "" e
// false quando não há hit (ou a entrada expirou).
func (c *ResultCache) Get(taskFingerprint, knowledgeSnapshot string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	e, ok := c.entries[key(taskFingerprint, knowledgeSnapshot)]
	if !ok {
		return "", false
	}
	if c.ttl > 0 && time.Since(e.createdAt) > c.ttl {
		return "", false
	}
	return e.value, true
}

// Set armazena o resultado sob (tarefa, snapshot).
func (c *ResultCache) Set(taskFingerprint, knowledgeSnapshot, result string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key(taskFingerprint, knowledgeSnapshot)] = cacheEntry{
		value:     result,
		createdAt: time.Now(),
	}
}

// Len devolve o número de entradas (para telemetria/auditoria).
func (c *ResultCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Clear esvazia o cache (útil após reindex do conhecimento).
func (c *ResultCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]cacheEntry)
}

// TaskFingerprint calcula o SHA-256 canônico da descrição da tarefa.
// Normaliza (lowercase, trim, colapso de whitespace, sem pontuação) antes de
// hashear — tarefas equivalentes compartilham a MESMA fingerprint.
func TaskFingerprint(task string) string {
	norm := normalizeTask(task)
	h := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(h[:])
}

// MarshalResult serializa um resultado para armazenamento. Usa JSON para
// manter a legibilidade/auditoria; o valor pode ser qualquer struct.
func MarshalResult(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// UnmarshalResult desserializa um resultado armazenado.
func UnmarshalResult(raw string, v any) error {
	return json.Unmarshal([]byte(raw), v)
}

// reNonWord colapsa qualquer sequência de não-palavra (espaços, pontuação) em
// um único espaço — a normalização canônica do cachefingerprint.
var reNonWord = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// normalizeTask aplica a normalização canônica: lowercase + trim + colapso de
// whitespace/pontuação. O mesmo padrão do cachefingerprint da busca.
func normalizeTask(task string) string {
	s := strings.ToLower(task)
	s = reNonWord.ReplaceAllString(s, " ")
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}
