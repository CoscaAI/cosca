// Package memory — FASE D: memória episódica multimodal.
//
// Este arquivo define a unidade persistida de memória episódica (EpisodicRecord)
// e o storage que a reutiliza (o MemoryEngine existente, com uma camada dedicada
// LayerEpisodic). A memória episódica faz o COSCA LEMBRAR o que viu/ouviu
// sincronizado entre sessões: um segmento de áudio (uma frase falada) ligado às
// observações de visão que overlap temporalmente nele (MultiRel).
//
// PRIVACY (decisão política documentada): persiste-se apenas a REPRESENTAÇÃO
// (texto/entidades/relações/relações temporais), NUNCA o frame bruto. A tela é
// sensível por natureza; guardar o vídeo/PNG da tela seria um risco de privacidade
// inaceitável. Só o entendimento semântico (resumos, rótulos, confianças, timings)
// é gravado — nunca pixels.
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ──────────────────────────────────────────────────────────────
// Constantes da camada episódica
// ──────────────────────────────────────────────────────────────

const (
	// LayerEpisodic é a camada de memória de longo-termo que guarda as
	// observações multimodais sincronizadas (visão+áudio). Ela NÃO entra na
	// lista default do LayerManager (para não quebrar o contrato das 6 camadas
	// existentes) — é registrada on-demand por EnsureEpisodicLayer. Retenção:
	// DefaultEpisodicTTL (30 dias) com cap DefaultEpisodicMaxRecords.
	LayerEpisodic MemoryLayer = "episodic"

	// MemoryTypeEpisodic é o tipo de memória usado por EpisodicRecord.
	MemoryTypeEpisodic MemoryType = "episodic"

	// DefaultEpisodicTTL é a retenção da memória episódica: 30 dias. NADA cresce
	// infinito — o prune (auto + manual) remove expirados.
	DefaultEpisodicTTL = 30 * 24 * time.Hour

	// DefaultEpisodicMaxRecords é o cap soft de registros episódicos por sessão
	// de persistência. Acima disso os mais antigos são descartados (mundo real
	// não precisa lembrar cada frame por anos).
	DefaultEpisodicMaxRecords = 5000

	// episodicLayerTTL é o TTL da camada (usado na definição da camada). Bate
	// com DefaultEpisodicTTL.
	episodicLayerTTL = "720h"
)

// EpisodicRecord é a unidade persistida da memória episódica multimodal.
//
// Ela responde à pergunta do professor: "o COSCA sabia o que estava vendo quando
// ouviu determinada coisa". Um EpisodicRecord é uma observação sincronizada (ou
// um grupo de observações ligadas por MultiRel) — a representação semântica de
// um instante percebido.
//
// IMPORTANTE (privacy): carrega apenas REPRESENTAÇÃO (resumos, texto, entidades,
// relações temporais). Nunca um frame/audio bruto.
type EpisodicRecord struct {
	// ID é o identificador estável (UUID). Reusa o formato de ID da engine.
	ID string `json:"id"`
	// Timestamp é o wall-clock (para persistência/consulta por faixa temporal).
	Timestamp time.Time `json:"timestamp"`
	// Monotonic é o timestamp monotônico em ns (ordenação intra-processo; não é
	// comparável entre reinícios, por isso o Timestamp wall-clock é o canônico).
	Monotonic int64 `json:"monotonic,omitempty"`
	// Modality é o canal sensorial: "vision", "audio" ou "multimodal" (sync).
	Modality string `json:"modality,omitempty"`
	// Sequence é a sequência monotônica global da observação no bus.
	Sequence uint64 `json:"sequence,omitempty"`
	// Confidence é a confiança da percepção em [0,1].
	Confidence float64 `json:"confidence,omitempty"`
	// VisionSummary é o resumo semântico do que foi visto (SummaryText).
	VisionSummary string `json:"vision_summary,omitempty"`
	// AudioText é o texto transcrito (o que foi ouvido).
	AudioText string `json:"audio_text,omitempty"`
	// Tokens são os timings palavra-a-palavra do áudio (offset em ns).
	Tokens []EpisodicToken `json:"tokens,omitempty"`
	// MultiRels são as ligações áudio↔visão (o que estava vendo quando ouviu X).
	MultiRels []EpisodicMultiRel `json:"multi_rels,omitempty"`
	// Entities são as entidades percebidas na janela de visão.
	Entities []EpisodicEntity `json:"entities,omitempty"`
	// Context é contexto livre (ex.: "sessão 2026-08-31", "perception bus").
	Context string `json:"context,omitempty"`
	// CreatedAt é quando o registro foi gravado (para prune/ordenação).
	CreatedAt time.Time `json:"created_at"`
}

// EpisodicToken é um token de ASR com offset (ns) do início do segmento.
type EpisodicToken struct {
	Word       string  `json:"word"`
	Offset     int64   `json:"offset"` // ns
	Confidence float64 `json:"confidence,omitempty"`
}

// EpisodicEntity é a projeção leve de uma entidade percebida (posição/depth são
// omitidos para manter o registro enxuto e reduzir volume). Guardamos o que
// importa para busca e para responder "o que estava na tela".
type EpisodicEntity struct {
	ID         string  `json:"id,omitempty"`
	Label      string  `json:"label"`
	Type       string  `json:"type,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

// EpisodicMultiRel é uma ligação áudio↔visão persistida (espelho do bus.MultiRel,
// sem acoplar o package memory ao package perception/bus — evita ciclo de import).
type EpisodicMultiRel struct {
	AudioSeg   uint64               `json:"audio_seg"`
	TextHint   string               `json:"text_hint"`
	AudioConf  float64              `json:"audio_conf"`
	Confidence float64              `json:"confidence"`
	Overlap    []EpisodicWindowRef  `json:"overlap,omitempty"`
}

// EpisodicWindowRef referencia uma observação de visão que cobriu um segmento de
// áudio no intervalo temporal (com as entidades sendo vistas naquele instante).
type EpisodicWindowRef struct {
	Sequence  uint64 `json:"sequence"`
	Timestamp int64  `json:"timestamp"` // monotônico ns
	Entities  []EpisodicEntity `json:"entities,omitempty"`
}

// EpisodicQuery é a consulta de memória episódica por faixa temporal e/ou texto.
type EpisodicQuery struct {
	// Since é o limite inferior da janela temporal (inclusive). Zero = sem limite.
	Since time.Time `json:"since,omitempty"`
	// Until é o limite superior da janela temporal (inclusive). Zero = sem limite.
	Until time.Time `json:"until,omitempty"`
	// Query é o texto/entidade a buscar (AND por palavra sobre texto+entidades).
	// Vazio = todos os registros dentro da janela temporal.
	Query string `json:"query,omitempty"`
	// Modality filtra por canal ("vision"|"audio"|"multimodal"). Vazio = todos.
	Modality string `json:"modality,omitempty"`
	// Limit é o máximo de resultados. Default 100.
	Limit int `json:"limit,omitempty"`
}

// ──────────────────────────────────────────────────────────────
// Camada episódica (on-demand, para não quebrar o contrato das 6 camadas)
// ──────────────────────────────────────────────────────────────

// EnsureEpisodicLayer registra a camada LayerEpisodic e cria o store, se ainda
// não existir. É idempotente e on-demand: a camada episódica NÃO faz parte das
// 6 camadas default do LayerManager (evita quebrar testes/contratos existentes),
// mas quando ativada ela ganha store e participa do prune auto.
//
// Deve ser chamada antes de qualquer StoreEpisodic/QueryEpisodic.
func (e *MemoryEngine) EnsureEpisodicLayer() error {
	if e == nil {
		return fmt.Errorf("episodic: nil engine")
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.stores[LayerEpisodic]; ok {
		return nil
	}

	if _, exists := e.layers.LayerDefinition(LayerEpisodic); !exists {
		if err := e.layers.RegisterLayer(LayerDefinition{
			Layer:    LayerEpisodic,
			Priority: 45,
			MaxSize:  DefaultEpisodicMaxRecords,
			Persist:  true,
			TTL:      episodicLayerTTL,
		}); err != nil {
			return fmt.Errorf("episodic: register layer: %w", err)
		}
	}

	if e.config.DataDir == "" {
		return fmt.Errorf("episodic: no data dir configured for engine")
	}
	store, err := NewFileStore(e.config.DataDir, LayerEpisodic, e.logger)
	if err != nil {
		return fmt.Errorf("episodic: create store: %w", err)
	}
	e.stores[LayerEpisodic] = store

	e.logger.Debug().
		Str("layer", string(LayerEpisodic)).
		Str("dir", store.dir).
		Msg("episodic memory layer enabled")
	return nil
}

// EpisodicEnabled reports whether the episodic layer is wired on this engine.
func (e *MemoryEngine) EpisodicEnabled() bool {
	if e == nil {
		return false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, ok := e.stores[LayerEpisodic]
	return ok
}

// ConfigureEpisodicRetention define a retenção (TTL) e o cap de registros da
// camada episódica. Chamado pelo runtime quando perception.episodic é ativado;
// defaults DefaultEpisodicTTL / DefaultEpisodicMaxRecords. Idempotente.
func (e *MemoryEngine) ConfigureEpisodicRetention(ttl time.Duration, maxRecords int) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if ttl > 0 {
		e.episodicTTL = ttl
	}
	if maxRecords > 0 {
		e.episodicMaxRecords = maxRecords
	}
}

// StoreEpisodic persiste um EpisodicRecord (representação, nunca frame bruto).
// Os registros são gravados como JSON no campo Content de um MemoryRecord da
// camada LayerEpisodic, reutilizando o FileStore + índice SQLite FTS da engine.
func (e *MemoryEngine) StoreEpisodic(ctx context.Context, rec EpisodicRecord) (*EpisodicRecord, error) {
	if err := e.EnsureEpisodicLayer(); err != nil {
		return nil, err
	}

	if rec.ID == "" {
		rec.ID = uuid.New().String()
	}
	if err := ValidateMemoryID(rec.ID); err != nil {
		return nil, fmt.Errorf("episodic: invalid id: %w", err)
	}
	if rec.Timestamp.IsZero() {
		rec.Timestamp = time.Now()
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now()
	}

	data, err := json.Marshal(rec)
	if err != nil {
		return nil, fmt.Errorf("episodic: marshal record: %w", err)
	}

	// Retenção configurável (config.Episodic.TTL ou default 30 dias).
	ttl := e.episodicTTL
	if ttl <= 0 {
		ttl = DefaultEpisodicTTL
	}

	// Owner vazio = memória de sistema/agente, compartilhada globalmente
	// (não é memória privada de um usuário — é a percepção do próprio COSCA).
	mr := MemoryRecord{
		ID:        rec.ID,
		Type:      MemoryTypeEpisodic,
		Layer:     LayerEpisodic,
		Owner:     "",
		Content:   string(data),
		CreatedAt: rec.Timestamp,
		UpdatedAt: rec.Timestamp,
		TTL:       ttl,
		Priority:  50,
		Metadata: map[string]string{
			"modality": rec.Modality,
			"sequence": fmt.Sprintf("%d", rec.Sequence),
			"audio":    truncateStr(rec.AudioText, 512),
			"vision":   truncateStr(rec.VisionSummary, 512),
		},
	}

	saved, err := e.Store(ctx, mr)
	if err != nil {
		return nil, fmt.Errorf("episodic: store: %w", err)
	}
	rec.ID = saved.ID
	return &rec, nil
}

// QueryEpisodic consulta a memória episódica por faixa temporal (`since`, `until`)
// e/ou por texto/entidade (`query` — AND por palavra sobre texto+labels). É a API
// que faz o COSCa "lembrar": "o que estava vendo quando ouvi X às 10:32".
//
// A filtragem temporal é feita em Go sobre a representação persistida (exact),
// NÃO por comparação de strings no SQLite (evita fragilidade de zona/offset).
func (e *MemoryEngine) QueryEpisodic(ctx context.Context, q EpisodicQuery) ([]EpisodicRecord, error) {
	if err := e.EnsureEpisodicLayer(); err != nil {
		return nil, err
	}
	if q.Limit <= 0 {
		q.Limit = 100
	}

	candidates, err := e.listEpisodicRecords(ctx)
	if err != nil {
		return nil, err
	}

	words := tokenizeQuery(q.Query)
	out := make([]EpisodicRecord, 0, len(candidates))
	for _, mr := range candidates {
		var er EpisodicRecord
		if err := json.Unmarshal([]byte(mr.Content), &er); err != nil {
			// Registro corrompido → pula (não quebra a consulta).
			continue
		}
		if !q.Since.IsZero() && er.Timestamp.Before(q.Since) {
			continue
		}
		if !q.Until.IsZero() && er.Timestamp.After(q.Until) {
			continue
		}
		if q.Modality != "" && er.Modality != q.Modality {
			continue
		}
		if len(words) > 0 && !matchesEpisodic(er, words) {
			continue
		}
		out = append(out, er)
	}

	// Mais recentes primeiro.
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Timestamp.After(out[j].Timestamp)
	})
	if len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

// PruneEpisodic remove registros episódicos expirados (TTL) e, se maxRecords > 0,
// trima os mais antigos além do cap. Retorna quantos foram removidos. Quando
// maxRecords <= 0, usa a retenção configurada (config.Episodic.MaxRecords).
func (e *MemoryEngine) PruneEpisodic(ctx context.Context, maxRecords int) (int, error) {
	if err := e.EnsureEpisodicLayer(); err != nil {
		return 0, err
	}
	if maxRecords <= 0 {
		maxRecords = e.episodicMaxRecords
		if maxRecords <= 0 {
			maxRecords = DefaultEpisodicMaxRecords
		}
	}
	removed, err := e.Prune(ctx, LayerEpisodic)
	if err != nil {
		return removed, err
	}
	if maxRecords < 0 {
		return removed, nil
	}
	recs, err := e.listEpisodicRecords(ctx)
	if err != nil {
		return removed, err
	}
	sort.SliceStable(recs, func(i, j int) bool {
		return recs[i].CreatedAt.After(recs[j].CreatedAt)
	})
	for i := maxRecords; i < len(recs); i++ {
		if err := e.Delete(ctx, recs[i].ID, LayerEpisodic); err == nil {
			removed++
		}
	}
	return removed, nil
}

// listEpisodicRecords lê todos os registros .md da camada episódica a partir do
// store em disco (representação). Usada pela consulta para filtragem exact.
func (e *MemoryEngine) listEpisodicRecords(ctx context.Context) ([]MemoryRecord, error) {
	e.mu.RLock()
	store, ok := e.stores[LayerEpisodic].(*FileStore)
	e.mu.RUnlock()
	if !ok || store == nil {
		return nil, fmt.Errorf("episodic: store not ready")
	}

	entries, err := os.ReadDir(store.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("episodic: read dir: %w", err)
	}

	var out []MemoryRecord
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".md") {
			continue
		}
		rec, err := store.parseFile(filepath.Join(store.dir, ent.Name()))
		if err != nil {
			continue
		}
		if rec.Layer != LayerEpisodic {
			continue
		}
		out = append(out, *rec)
	}
	return out, nil
}

// ──────────────────────────────────────────────────────────────
// Helpers de busca
// ──────────────────────────────────────────────────────────────

// tokenizeQuery extrai as palavras (letras/dígitos) de uma query, para o
// matching AND. Esvazia a query que seja só pontuação/símbolos.
func tokenizeQuery(query string) []string {
	q := strings.ToLower(query)
	var words []string
	var sb strings.Builder
	flush := func() {
		if sb.Len() > 0 {
			words = append(words, sb.String())
			sb.Reset()
		}
	}
	for _, r := range q {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
			(r >= '\u00e0' && r <= '\u00ff') { // acentos latinos (pt-BR)
			sb.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return words
}

// searchableText junta todo o texto buscável de um EpisodicRecord (áudio +
// visão + entidades + hints das MultiRels) em minúsculas.
func searchableText(er EpisodicRecord) string {
	var parts []string
	if er.AudioText != "" {
		parts = append(parts, er.AudioText)
	}
	if er.VisionSummary != "" {
		parts = append(parts, er.VisionSummary)
	}
	for _, e := range er.Entities {
		parts = append(parts, e.Label)
	}
	for _, rel := range er.MultiRels {
		parts = append(parts, rel.TextHint)
		for _, o := range rel.Overlap {
			for _, e := range o.Entities {
				parts = append(parts, e.Label)
			}
		}
	}
	return strings.ToLower(strings.Join(parts, " "))
}

// matchesEpisodic reports whether every query word is present no texto buscável.
func matchesEpisodic(er EpisodicRecord, words []string) bool {
	text := searchableText(er)
	for _, w := range words {
		if !containsWord(text, w) {
			return false
		}
	}
	return true
}

// containsWord faz um token match (palavra inteira) sobre o texto minúsculo:
// procura o token delimitado por não-letra/dígito. Isso evita falso-positivo de
// "casa" casando com "casaco".
func containsWord(text, word string) bool {
	if word == "" {
		return true
	}
	n := len(text)
	w := len(word)
	for i := 0; i+w <= n; i++ {
		if text[i:i+w] != word {
			continue
		}
		beforeOK := i == 0 || !isTextByte(text[i-1])
		afterOK := i+w == n || !isTextByte(text[i+w])
		if beforeOK && afterOK {
			return true
		}
	}
	return false
}

func isTextByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b >= 0x80
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
