package cli

// knowledge_cache.go — CACHE DE CONHECIMENTO (memory-first, QA).
//
// Objetivo: o LLM (o cérebro local via Ollama) SÓ é chamado quando o COSCa
// ainda NÃO sabe a resposta. Hoje ele é chamado a TODA deliberação de voz.
// Este cache guarda os pares pergunta→resposta que o LLM já produziu, de modo
// que, na próxima vez que o Don fizer a MESMA pergunta (ou uma muito parecida),
// o COSCa responde do cache sem gastar o LLM.
//
// Contrato de degradação graciosa (nunca quebra o loop de voz):
//   - dir vazio / arquivo ausente / JSON corrompido → cache vazio (Lookup
//     retorna false; Store não persiste) — o fluxo segue exatamente como hoje.
//   - os erros de I/O são capturados e logados via zerolog (quando um logger é
//     injetado); sem logger, são silenciosos.
//
// Persistência: um único arquivo JSON `knowledge.json` dentro do dir
// (reutiliza o mesmo data-dir do memoryManagerAdapter — soberania do dir).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// defaultKnowledgeCacheThreshold é o score mínimo de similaridade (token-overlap
// ponderado) para um Lookup do cache ser considerado um HIT (o COSCa responde
// do cache sem chamar o LLM). Abaixo dele, a pergunta é "nova" → flui pro brain.
const defaultKnowledgeCacheThreshold = 0.62

// knowledgeCacheFileName é o nome do arquivo JSON persistido.
const knowledgeCacheFileName = "knowledge.json"

// minSignificantWordLen é o comprimento mínimo de uma palavra para ela contar
// no token-overlap. Palavras muito curtas ("ve", "na", "a") são ruído.
const minSignificantWordLen = 3

// stopwordsPT são as palavras comuns do português ignoradas no token-overlap.
// Já normalizadas (lowercase, sem acentos) — compara-se sempre sobre texto
// normalizado via normalizePT.
var stopwordsPT = map[string]struct{}{
	"o": {}, "a": {}, "os": {}, "as": {},
	"de": {}, "do": {}, "da": {}, "em": {},
	"que": {}, "com": {}, "para": {}, "uma": {}, "um": {},
	"na": {}, "no": {}, "e": {}, "ei": {},
}

// entry é um par pergunta→resposta persistido (a resposta que o LLM já deu).
type entry struct {
	// Query é a pergunta original (não-normalizada) — para exibição/debug.
	Query string `json:"query"`
	// Normalized é a chave de busca (lowercase + sem acentos) via normalizePT.
	Normalized string `json:"normalized"`
	// Answer é a resposta que o LLM já produziu.
	Answer string `json:"answer"`
	// CreatedAt é quando a resposta foi aprendida/atualizada.
	CreatedAt time.Time `json:"created_at"`
	// Hits é o nº de vezes que esta resposta foi servida do cache (telemetria).
	Hits int `json:"hits"`
}

// knowledgeCache guarda os pares pergunta→resposta aprendidos. É thread-safe
// (mutex) porque o loop de voz e a persistência correm em goroutines. Quando o
// dir é vazio ou o arquivo não pôde ser lido, o cache fica vazio (degradação
// graciosa — nunca quebra o fluxo).
type knowledgeCache struct {
	mu      sync.RWMutex
	dir     string
	path    string
	entries []entry
	logger  zerolog.Logger
}

// newKnowledgeCache cria um cache a partir de um diretório. Se o dir for vazio
// ou o arquivo `knowledge.json` não existir/estiver corrompido, o cache começa
// VAZIO — degradação graciosa (Lookup=false; Store vira apenas memória).
func newKnowledgeCache(dir string) *knowledgeCache {
	c := &knowledgeCache{
		dir:    strings.TrimSpace(dir),
		logger: zerolog.Nop(), // loga via zerolog se injetado; senão silencioso.
	}
	if c.dir == "" {
		return c
	}
	c.path = filepath.Join(c.dir, knowledgeCacheFileName)
	c.load()
	return c
}

// load lê o arquivo JSON no construtor. Erros (arquivo ausente/corrompido/de
// permissão) são capturados e NUNCA quebram o fluxo — o cache fica vazio.
func (c *knowledgeCache) load() {
	data, err := os.ReadFile(c.path)
	if err != nil {
		return
	}
	var entries []entry
	if err := json.Unmarshal(data, &entries); err != nil {
		c.logger.Warn().Err(err).Str("path", c.path).Msg("knowledge cache: corrupt json — starting empty")
		return
	}
	// Defesa: só mantém entradas utilizáveis (com chave normalizada e resposta).
	for _, e := range entries {
		if e.Normalized == "" || e.Answer == "" {
			continue
		}
		c.entries = append(c.entries, e)
	}
}

// Lookup busca a resposta de maior similaridade para a query. Retorna:
//   - answer: a resposta do cache (quando ok=true);
//   - ok: true se o score >= defaultKnowledgeCacheThreshold (um HIT real);
//   - score: o token-overlap ponderado (0.0–1.0) da melhor entrada.
//
// Degradação graciosa: cache vazio, query vazia ou nenhuma entrada acima do
// threshold → ("", false, score). Se existe uma entrada com o MESMO texto
// normalizado, o score é 1.0 (a pergunta é IDÊNTICA a uma já respondida).
func (c *knowledgeCache) Lookup(query string) (string, bool, float64) {
	q := normalizePT(strings.TrimSpace(query))
	if q == "" {
		return "", false, 0
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) == 0 {
		return "", false, 0
	}

	queryWords := significantWords(q)
	bestIdx := -1
	bestScore := 0.0
	for i := range c.entries {
		e := &c.entries[i]
		if e.Normalized == "" || e.Answer == "" {
			continue
		}
		// Query IDÊNTICA (mesmo texto normalizado) → score 1.0 garantido.
		if e.Normalized == q {
			e.Hits++
			return e.Answer, true, 1.0
		}
		if score := tokenOverlapScore(queryWords, e.Normalized); score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	if bestIdx < 0 || bestScore < defaultKnowledgeCacheThreshold {
		return "", false, bestScore
	}
	c.entries[bestIdx].Hits++
	return c.entries[bestIdx].Answer, true, bestScore
}

// Store aprende uma resposta: normaliza a query, faz upsert (mesma query
// normalizada → substitui a answer e atualiza ts) e persiste o JSON. Se a
// resposta já existe para outra pergunta, vira uma nova entrada. Erros de I/O
// são capturados (nunca quebram o fluxo).
func (c *knowledgeCache) Store(query, answer string) {
	query = strings.TrimSpace(query)
	answer = strings.TrimSpace(answer)
	if query == "" || answer == "" {
		return
	}
	norm := normalizePT(query)

	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.entries {
		if c.entries[i].Normalized == norm {
			// Upsert: substitui a resposta aprendida (e atualiza o ts).
			c.entries[i].Query = query
			c.entries[i].Answer = answer
			c.entries[i].CreatedAt = time.Now()
			c.persistLocked()
			return
		}
	}
	c.entries = append(c.entries, entry{
		Query:      query,
		Normalized: norm,
		Answer:     answer,
		CreatedAt:  time.Now(),
	})
	c.persistLocked()
}

// persistLocked grava o cache em disco (assume c.mu.Lock held). Erros são
// capturados e logados via zerolog, sem nunca quebrar o fluxo. Sem dir → no-op.
func (c *knowledgeCache) persistLocked() {
	if c.dir == "" {
		return
	}
	data, err := json.MarshalIndent(c.entries, "", "  ")
	if err != nil {
		c.logger.Warn().Err(err).Msg("knowledge cache: marshal failed")
		return
	}
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		c.logger.Warn().Err(err).Str("dir", c.dir).Msg("knowledge cache: mkdir failed")
		return
	}
	if err := os.WriteFile(c.path, data, 0o600); err != nil {
		c.logger.Warn().Err(err).Str("path", c.path).Msg("knowledge cache: write failed")
	}
}

// significantWords extrai as palavras significativas de um texto normalizado:
// comprimento >= minSignificantWordLen e não-stopword. Usado no token-overlap.
func significantWords(norm string) []string {
	fields := strings.Fields(norm)
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if len(f) < minSignificantWordLen {
			continue
		}
		if _, stop := stopwordsPT[f]; stop {
			continue
		}
		out = append(out, f)
	}
	return out
}

// tokenOverlapScore é o token-overlap ponderado entre a query normalizada
// (words já tokenizadas) e uma entrada armazenada: a razão de palavras em
// comum sobre as palavras significativas da QUERY. Uma query sem palavras
// significativas nunca casa (score 0), exceto pelo caminho de consulta
// idêntica tratado em Lookup.
func tokenOverlapScore(queryWords []string, storedNorm string) float64 {
	if len(queryWords) == 0 {
		return 0
	}
	storedWords := significantWords(storedNorm)
	if len(storedWords) == 0 {
		return 0
	}
	overlap := 0
	for _, w := range queryWords {
		for _, s := range storedWords {
			if w == s {
				overlap++
				break
			}
		}
	}
	return float64(overlap) / float64(len(queryWords))
}
