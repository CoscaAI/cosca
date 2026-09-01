package bus

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/worldmodel"
	"github.com/rs/zerolog"
)

// sinkWriteTimeout limita uma única gravação episódica (evita travar o worker por
// uma engine lenta). A percepção NUNCA é bloqueada pela memória.
const sinkWriteTimeout = 2 * time.Second

// ──────────────────────────────────────────────────────────────
// MemorySink — FASE D: memória episódica multimodal
// ──────────────────────────────────────────────────────────────

// EpisodicWriter é o seam de persistência usado pelo MemorySink. É satisfeito
// por *memory.MemoryEngine (método StoreEpisodic), mantendo o sink desacoplado
// de um tipo concreto (testável com um mock).
type EpisodicWriter interface {
	StoreEpisodic(ctx context.Context, rec memory.EpisodicRecord) (*memory.EpisodicRecord, error)
}

// MemorySinkConfig configura o MemorySink.
type MemorySinkConfig struct {
	// Enabled liga a gravação. Default false (opt-in).
	Enabled bool
	// MaxBuffered é o tamanho do buffer async (cap de registros em espera).
	// Default 256. Acima disso os registros são descartados (sem backpressure).
	MaxBuffered int
	// MaxSeen é o tamanho do set de sequências já registradas (dedup entre
	// publicações do bus — a mesma observação aparece em vários WorldState).
	// Default 2048.
	MaxSeen int
}

// MemorySink assina o Perception Bus e persiste EpisodicRecords (representação)
// na engine de memória. Ele decide o QUE gravar:
//
//   - cada observação de ÁUDIO com texto → EpisodicRecord{modality:audio}
//   - cada MultiRel (segmento de áudio ligado à visão que o overlap temporal)
//     → EpisodicRecord{modality:multimodal} — o "o que estava vendo quando ouvi"
//   - cada observação de VISÃO com entidades e fora de uma MultiRel →
//     EpisodicRecord{modality:vision}
//
// É NON-BLOCKING: grava em um goroutine de fundo via um buffer canal; se o buffer
// encher, o registro é descartado (o bus nunca é backpressure). O dedup por
// Sequence evita regravar a mesma observação a cada publicação.
//
// PRIVACY (política documentada): apenas a REPRESENTAÇÃO (texto/entidades/
// relações) é persistida — NUNCA o frame bruto. A tela é sensível por natureza.
type MemorySink struct {
	bus     *Bus
	writer  EpisodicWriter
	cfg     MemorySinkConfig
	logger  zerolog.Logger

	seenMu sync.Mutex
	seen   map[uint64]struct{} // sequences de observações já registradas (dedup)
	seenRels map[uint64]struct{} // audio segs já registradas como MultiRel (dedup)

	ch      chan memory.EpisodicRecord
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running atomic.Bool
}

// NewMemorySink cria um MemorySink. b pode ser nil (sink inerte); writer pode
// ser nil (sink vai degradar: não grava, registra warning). cfg.MaxBuffered e
// cfg.MaxSeen com zeros são normalizados para os defaults.
func NewMemorySink(b *Bus, writer EpisodicWriter, cfg MemorySinkConfig, logger zerolog.Logger) *MemorySink {
	if cfg.MaxBuffered <= 0 {
		cfg.MaxBuffered = 256
	}
	if cfg.MaxSeen <= 0 {
		cfg.MaxSeen = 2048
	}
	return &MemorySink{
		bus:      b,
		writer:   writer,
		cfg:      cfg,
		logger:   logger,
		seen:     make(map[uint64]struct{}, cfg.MaxSeen),
		seenRels: make(map[uint64]struct{}, cfg.MaxSeen),
		ch:       make(chan memory.EpisodicRecord, cfg.MaxBuffered),
	}
}

// Enabled reports whether the sink is configured to record.
func (s *MemorySink) Enabled() bool { return s != nil && s.cfg.Enabled }

// Start lança o consumo do bus e o gravador de fundo. Idempotente.
func (s *MemorySink) Start(ctx context.Context) {
	if s == nil || !s.Enabled() {
		return
	}
	if s.running.Swap(true) {
		return
	}
	cctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	if s.bus != nil {
		s.wg.Add(1)
		go s.consume(cctx)
	}
	s.wg.Add(1)
	go s.worker(cctx)
	s.logger.Debug().Msg("perception episodic memory: sink started")
}

// Stop para o sink e espera os goroutines. Idempotente, nil-safe.
func (s *MemorySink) Stop() {
	if s == nil || !s.running.Load() {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	s.running.Store(false)
	s.logger.Debug().Msg("perception episodic memory: sink stopped")
}

// consume lê cada WorldState publicado e extrai os registros não-vistos.
func (s *MemorySink) consume(ctx context.Context) {
	defer s.wg.Done()
	ch := s.bus.Watch()
	defer s.bus.Unsubscribe(ch)

	for {
		select {
		case <-ctx.Done():
			return
		case ws := <-ch:
			if ws == nil {
				continue
			}
			s.buildRecords(ws)
		}
	}
}

// buildRecords transforma um WorldState em EpisodicRecords não-vistos e os
// enfileira. Dedup por Sequence evita regravar a cada publicação do bus.
func (s *MemorySink) buildRecords(ws *WorldState) {
	if s.writer == nil {
		return
	}

	// Sequências cobertas por uma MultiRel (áudio↔visão) — esses são gravados
	// como registro "multimodal" (sync), não como registros isolados.
	coveredAudio := make(map[uint64]struct{})
	coveredVision := make(map[uint64]struct{})
	for _, rel := range ws.Relations {
		coveredAudio[rel.AudioSeg] = struct{}{}
		for _, o := range rel.Overlap {
			coveredVision[o.Sequence] = struct{}{}
		}
	}

	for i := range ws.Window {
		o := &ws.Window[i]
		if !s.markSeen(o.Sequence) {
			continue // já registrado
		}
		switch o.Modality {
		case ModalityAudio:
			if _, covered := coveredAudio[o.Payload.Audio.SegmentID]; covered {
				continue // será registrado como parte de uma MultiRel multimodal
			}
			if rec := sinkRecordForAudio(o); rec != nil {
				s.enqueue(*rec)
			}
		case ModalityVision:
			if _, covered := coveredVision[o.Sequence]; covered {
				continue
			}
			if rec := sinkRecordForVision(o); rec != nil {
				s.enqueue(*rec)
			}
		}
	}

	// MultiRels: o registro multimodal — o coração da memória episódica
	// ("o que estava vendo quando ouvi X"). Dedup por AudioSeg.
	for _, rel := range ws.Relations {
		if !s.markSeg(rel.AudioSeg) {
			continue // já registrado
		}
		if rec := sinkRecordForMultimodal(rel, ws); rec != nil {
			s.enqueue(*rec)
		}
	}
}

// markSeen registra uma sequência como vista; retorna true se é NOTA (a primeira
// vez), false se já foi vista antes.
func (s *MemorySink) markSeen(seq uint64) bool {
	s.seenMu.Lock()
	defer s.seenMu.Unlock()
	if _, ok := s.seen[seq]; ok {
		return false
	}
	// Cap do set: evita crescimento infinito do dedup (só precisa lembrar das
	// sequências recentes; o bus já evicta por janela).
	if len(s.seen) >= s.cfg.MaxSeen {
		// Simples: limpa tudo (as antigas já foram gravadas). Evita um LRU
		// complexo — o custo é re-gravar as pouquíssimas ainda na janela.
		s.seen = make(map[uint64]struct{}, s.cfg.MaxSeen)
		s.seenRels = make(map[uint64]struct{}, s.cfg.MaxSeen)
	}
	s.seen[seq] = struct{}{}
	return true
}

// markSeg registra um AudioSeg (MultiRel) como vista; true = primeira vez.
func (s *MemorySink) markSeg(seg uint64) bool {
	s.seenMu.Lock()
	defer s.seenMu.Unlock()
	if _, ok := s.seenRels[seg]; ok {
		return false
	}
	if len(s.seenRels) >= s.cfg.MaxSeen {
		s.seenRels = make(map[uint64]struct{}, s.cfg.MaxSeen)
	}
	s.seenRels[seg] = struct{}{}
	return true
}

// enqueue empurra um registro no buffer. Non-blocking: se o buffer está cheio,
// o registro é descartado (o bus nunca é backpressure por memória).
func (s *MemorySink) enqueue(rec memory.EpisodicRecord) {
	select {
	case s.ch <- rec:
	default:
		s.logger.Warn().
			Uint64("sequence", rec.Sequence).
			Str("modality", rec.Modality).
			Msg("episodic memory: sink buffer full, dropping record")
	}
}

// worker drena o buffer e grava via EpisodicWriter.
func (s *MemorySink) worker(ctx context.Context) {
	defer s.wg.Done()
	if s.writer == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case rec := <-s.ch:
			// Best-effort: um erro de gravação é logado e não derruba o bus.
			// Degradação graciosa: a memória episódica falha silenciosamente,
			// a percepção continua intacta.
			wctx, cancel := context.WithTimeout(ctx, sinkWriteTimeout)
			if _, err := s.writer.StoreEpisodic(wctx, rec); err != nil {
				s.logger.Warn().Err(err).
					Uint64("sequence", rec.Sequence).
					Str("modality", rec.Modality).
					Msg("episodic memory: store failed (dropped)")
			}
			cancel()
		}
	}
}

// ──────────────────────────────────────────────────────────────
// Conversões Observation/MultiRel → EpisodicRecord
// ──────────────────────────────────────────────────────────────

func sinkRecordForAudio(o *Observation) *memory.EpisodicRecord {
	if o == nil || o.Payload.Audio == nil {
		return nil
	}
	a := o.Payload.Audio
	if a.Text == "" && len(a.Tokens) == 0 {
		return nil // sem conteúdo reconhecido
	}
	toks := make([]memory.EpisodicToken, 0, len(a.Tokens))
	for _, t := range a.Tokens {
		toks = append(toks, memory.EpisodicToken{
			Word:       t.Word,
			Offset:     int64(t.Offset),
			Confidence: t.Confidence,
		})
	}
	return &memory.EpisodicRecord{
		ID:         "",
		Timestamp:  MonoToWall(o.Timestamp),
		Monotonic:  int64(o.Timestamp),
		Modality:   string(ModalityAudio),
		Sequence:   o.Sequence,
		Confidence: o.Confidence,
		AudioText:  a.Text,
		Tokens:     toks,
		Context:    "perception_bus",
	}
}

func sinkRecordForVision(o *Observation) *memory.EpisodicRecord {
	if o == nil || o.Payload.Vision == nil {
		return nil
	}
	v := o.Payload.Vision
	if len(v.Entities) == 0 {
		return nil // sem entidades — nada significativo
	}
	return &memory.EpisodicRecord{
		ID:            "",
		Timestamp:     MonoToWall(o.Timestamp),
		Monotonic:     int64(o.Timestamp),
		Modality:      string(ModalityVision),
		Sequence:      o.Sequence,
		Confidence:    o.Confidence,
		VisionSummary: v.SummaryText(),
		Entities:      episodicEntities(v.Entities),
		Context:       "perception_bus",
	}
}

// sinkRecordForMultimodal constrói o registro sincronizado a partir de uma
// MultiRel: o áudio (TextHint) + as observações de visão que overlap nele.
func sinkRecordForMultimodal(rel MultiRel, ws *WorldState) *memory.EpisodicRecord {
	if rel.TextHint == "" && len(rel.Overlap) == 0 {
		return nil
	}

	overlaps := make([]memory.EpisodicWindowRef, 0, len(rel.Overlap))
	var entities []memory.EpisodicEntity
	var firstTs MonotonicTime
	for _, o := range rel.Overlap {
		if firstTs == 0 || o.Timestamp < firstTs {
			firstTs = o.Timestamp
		}
		es := episodicEntities(o.Entities)
		entities = append(entities, es...)
		overlaps = append(overlaps, memory.EpisodicWindowRef{
			Sequence:  o.Sequence,
			Timestamp: int64(o.Timestamp),
			Entities:  es,
		})
	}
	// Timestamp do registro: usa a janela de visão mais antiga do overlap (o
	// instante em que o áudio estava sendo ouvido sincronizado com a visão).
	ts := firstTs
	if ts == 0 && ws != nil {
		ts = ws.Timestamp
	}
	if ts == 0 {
		ts = Now()
	}

	return &memory.EpisodicRecord{
		ID:          "",
		Timestamp:   MonoToWall(ts),
		Monotonic:   int64(ts),
		Modality:    "multimodal",
		Sequence:    rel.AudioSeg,
		Confidence:  rel.Confidence,
		AudioText:   rel.TextHint,
		MultiRels:   []memory.EpisodicMultiRel{
			{
				AudioSeg:   rel.AudioSeg,
				TextHint:   rel.TextHint,
				AudioConf:  rel.AudioConf,
				Confidence: rel.Confidence,
				Overlap:    overlaps,
			},
		},
		Entities: entities,
		Context:  "perception_bus_sync",
	}
}

// episodicEntities converte worldmodel.WorldEntity → EpisodicEntity (leve).
func episodicEntities(in []worldmodel.WorldEntity) []memory.EpisodicEntity {
	out := make([]memory.EpisodicEntity, 0, len(in))
	for _, e := range in {
		out = append(out, memory.EpisodicEntity{
			ID:         e.ID,
			Label:      e.Label,
			Type:       string(e.Type),
			Confidence: e.Confidence,
		})
	}
	return out
}
