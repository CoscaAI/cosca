package bus

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/internal/perception"
	"github.com/CoscaAI/cosca/internal/worldmodel"
	"github.com/CoscaAI/cosca/internal/worldmodel/vision"
)

// ──────────────────────────────────────────────────────────────
// Config
// ──────────────────────────────────────────────────────────────

// Config configures a Bus (the multimodal synchroniser).
type Config struct {
	// Window is the ring's temporal horizon. Default DefaultWindow (5s).
	Window time.Duration
	// MaxObs is the ring's hard capacity. Default DefaultMaxObs (256).
	MaxObs int
	// Tolerance is the overlap tolerance for audio↔vision binding.
	// Default DefaultTolerance (300ms).
	Tolerance time.Duration
}

// DefaultTolerance is the default overlap tolerance: a vision frame within
// 300ms of an audio segment still "binds" to it.
const DefaultTolerance = 300 * time.Millisecond

// resolve fills in defaults for any zero value.
func (c Config) resolve() Config {
	if c.Window <= 0 {
		c.Window = DefaultWindow
	}
	if c.MaxObs <= 0 {
		c.MaxObs = DefaultMaxObs
	}
	if c.Tolerance <= 0 {
		c.Tolerance = DefaultTolerance
	}
	return c
}

// ──────────────────────────────────────────────────────────────
// AudioSource (injectable seam)
// ──────────────────────────────────────────────────────────────

// AudioSample is a single audio payload with its monotonic capture-start
// timestamp. The source (Fase B: sherpa ASR) stamps Timestamp at the start of
// the capture, so the bus syncs on capture time — not on the time the ASR
// finished decoding.
type AudioSample struct {
	// Timestamp is the monotonic capture-start time. Zero means "the bus stamps
	// Now() at ingest" (used by the no-op source and by tests).
	Timestamp MonotonicTime
	// Payload is the recognised audio payload.
	Payload AudioPayload
}

// AudioSource produces audio samples for the bus. It is the injectable seam so
// Fase A can run with a no-op source and Fase B can plug in the sherpa ASR
// without touching the bus. Stream MUST be non-blocking: a slow/stalled source
// never backpressures the perception loop.
type AudioSource interface {
	// Stream returns a channel of audio samples. It is consumed by the bus; it
	// should never be closed by the source while the bus is running (the bus
	// treats a closed channel as "source was removed" and stops listening).
	Stream() <-chan AudioSample
	// Close releases any source resources. Idempotent.
	Close() error
}

// NoopAudioSource is a Fase A placeholder: it streams nothing and closes to a
// no-op. It lets the bus run in "vision-only with audio slot ready" mode so the
// multimodal contract is exercised without an ASR backend.
type NoopAudioSource struct{}

// Stream implements AudioSource (returns a channel that never sends nor closes,
// so the bus simply waits on it until ctx is done).
func (NoopAudioSource) Stream() <-chan AudioSample {
	return make(chan AudioSample)
}

// Close implements AudioSource (no-op, idempotent).
func (NoopAudioSource) Close() error { return nil }

// ──────────────────────────────────────────────────────────────
// Bus
// ──────────────────────────────────────────────────────────────

// Bus is the multimodal perception synchroniser. It taps the existing
// Perception Loop (internal/perception) for vision, and injectable AudioSources
// for audio, stamps both on a single monotonic clock, places them on a time
// ring, binds them by temporal overlap, and publishes a projected WorldState to
// its subscribers (and to /v1/perception/bus).
//
// It never modifies the perception.Service it subscribes to, nor the vision
// loop — it is a pure consumer/adapter.
type Bus struct {
	cfg       Config
	ring      *Ring
	visionSvc *perception.Service
	audioSrc  AudioSource
	logger    zerolog.Logger

	mu sync.RWMutex
	// world is the last projected WorldState.
	world *WorldState
	// version is the monotonic revision counter for the projection.
	version uint64
	// lastSeenAt is the most recent monotonic ingest time.
	lastSeenAt MonotonicTime
	lastVision *vision.Observation
	lastAudio  *AudioPayload
	// counts are lifetime ingest counters (process-lifetime in Metrics).
	visionCount uint64
	audioCount  uint64
	degraded    uint64
	// seq is the global observation sequence counter.
	seq uint64

	subMu sync.Mutex
	subs  map[chan *WorldState]struct{}

	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running atomic.Bool
	// started records whether Start was invoked at least once (for Enable state).
	started atomic.Bool
}

// NewBus creates a Bus. visionSvc may be nil (vision-only is then degraded);
// audioSrc may be nil (a NoopAudioSource is used). logger may be zero (zlog).
func NewBus(cfg Config, visionSvc *perception.Service, audioSrc AudioSource, logger zerolog.Logger) *Bus {
	cfg = cfg.resolve()
	if audioSrc == nil {
		audioSrc = NoopAudioSource{}
	}
	return &Bus{
		cfg:       cfg,
		ring:      NewRing(cfg.Window, cfg.MaxObs),
		visionSvc: visionSvc,
		audioSrc:  audioSrc,
		logger:    logger,
		subs:      make(map[chan *WorldState]struct{}),
	}
}

// Enabled reports whether the bus is enabled (opt-in). A bus is enabled by
// construction; Start actually begins ingesting.
func (b *Bus) Enabled() bool { return b != nil }

// Start launches the bus ingestion goroutine. If called while already running it
// is a no-op. The `ctx` cancels the bus (use Stop for graceful shutdown).
func (b *Bus) Start(ctx context.Context) {
	if b == nil {
		return
	}
	if b.running.Swap(true) {
		return
	}
	b.started.Store(true)
	cctx, cancel := context.WithCancel(ctx)
	b.cancel = cancel
	b.wg.Add(1)
	go b.run(cctx)
}

// Stop halts the bus and waits for the ingestion goroutine to exit. Safe to call
// multiple times.
func (b *Bus) Stop() {
	if b == nil || !b.running.Load() {
		return
	}
	if b.cancel != nil {
		b.cancel()
	}
	b.wg.Wait()
	b.running.Store(false)
}

// State returns the latest projected WorldState (defensive copy), or nil if no
// projection has been published yet (e.g. before Start or before any ingest).
func (b *Bus) State() *WorldState {
	if b == nil {
		return nil
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.world == nil {
		return nil
	}
	return copyWorldState(b.world)
}

// Subscribe returns a buffered channel that receives each new WorldState. The
// consumer MUST call Unsubscribe to avoid a leak. Slow consumers are dropped
// (non-blocking send).
func (b *Bus) Subscribe() chan *WorldState {
	ch := make(chan *WorldState, 8)
	b.subMu.Lock()
	defer b.subMu.Unlock()
	b.subs[ch] = struct{}{}
	return ch
}

// Watch is an alias for Subscribe (reads more naturally at call sites).
func (b *Bus) Watch() chan *WorldState { return b.Subscribe() }

// Unsubscribe removes a previously-subscribed channel and closes it.
func (b *Bus) Unsubscribe(ch chan *WorldState) {
	if ch == nil {
		return
	}
	b.subMu.Lock()
	defer b.subMu.Unlock()
	delete(b.subs, ch)
	close(ch)
}

// WindowAround exposes the ring's temporal query to callers who want to answer
// "what was being seen around instant t" (e.g. on demand, outside a live
// WorldState). before/after are durations around t.
func (b *Bus) WindowAround(t MonotonicTime, before, after time.Duration) []Observation {
	if b == nil || b.ring == nil {
		return nil
	}
	return b.ring.WindowAround(t, before, after)
}

// ──────────────────────────────────────────────────────────────
// Run loop
// ──────────────────────────────────────────────────────────────

func (b *Bus) run(ctx context.Context) {
	defer b.wg.Done()

	var visCh chan *perception.State
	if b.visionSvc != nil && b.visionSvc.Enabled() {
		visCh = b.visionSvc.Subscribe()
		defer b.visionSvc.Unsubscribe(visCh)
	}

	var audioCh <-chan AudioSample
	if b.audioSrc != nil {
		audioCh = b.audioSrc.Stream()
	}

	b.logger.Debug().
		Dur("window", b.cfg.Window).
		Dur("tol", b.cfg.Tolerance).
		Int("max_obs", b.cfg.MaxObs).
		Bool("vision", visCh != nil).
		Bool("audio", audioCh != nil).
		Msg("perception bus: started")

	// Publish an initial (possibly empty) projection so a subscriber sees a
	// WorldState immediately rather than waiting for the first ingest.
	b.computeAndPublish()

	for {
		select {
		case <-ctx.Done():
			b.logger.Debug().Msg("perception bus: stopped")
			return
		case st, ok := <-visCh:
			if !ok {
				visCh = nil
				continue
			}
			b.ingestVision(st)
		case sample, ok := <-audioCh:
			if !ok {
				audioCh = nil
				continue
			}
			b.ingestAudio(sample)
		}
	}
}

// ──────────────────────────────────────────────────────────────
// Ingest
// ──────────────────────────────────────────────────────────────

// ingestVision adapts a perception.State (from the vision loop) into a bus
// Observation and pushes it onto the ring.
func (b *Bus) ingestVision(st *perception.State) {
	obs := b.visionObservationFromState(st)
	b.ring.Push(obs)

	b.mu.Lock()
	if obs.Timestamp > b.lastSeenAt {
		b.lastSeenAt = obs.Timestamp
	}
	b.visionCount++
	if obs.Payload.Vision != nil {
		b.lastVision = obs.Payload.Vision
	}
	if obs.Confidence < 0.5 || len(obs.Payload.Vision.Warnings) > 0 {
		b.degraded++
	}
	b.mu.Unlock()

	b.computeAndPublish()
}

// visionObservationFromState builds a bus Observation from a perception.State.
// The perception State does not carry the raw vision.Observation, so we
// reconstruct the Observable projection from its exposed fields (entities,
// relations, warnings, latency). Duration uses the vision latency.
func (b *Bus) visionObservationFromState(st *perception.State) Observation {
	latency := time.Duration(st.LatencyMS) * time.Millisecond
	if latency < 0 {
		latency = 0
	}
	now := Now()
	inner := &vision.Observation{
		Entities:  append([]worldmodel.WorldEntity(nil), st.Entities...),
		Relations: append([]worldmodel.SpatialRelation(nil), st.Relations...),
		Warnings:  append([]string(nil), st.Warnings...),
		Latency:   latency,
	}
	conf := 1.0
	if st.Degraded {
		conf = 0.5
	}
	obs := Observation{
		Timestamp:  now,
		Modality:   ModalityVision,
		Duration:   latency,
		Sequence:   b.nextSeq(),
		Confidence: conf,
		Payload:    Payload{Vision: inner},
		Meta:       map[string]string{"source": "perception_loop"},
	}
	if st.Degraded {
		obs.Meta["degraded"] = "true"
	}
	return obs
}

// ingestAudio adapts an AudioSample into a bus Observation and pushes it.
func (b *Bus) ingestAudio(sample AudioSample) {
	ts := sample.Timestamp
	if ts == 0 {
		ts = Now()
	}
	payload := &sample.Payload
	obs := Observation{
		Timestamp:  ts,
		Modality:   ModalityAudio,
		Duration:   payload.duration(),
		Sequence:   b.nextSeq(),
		Confidence: 1.0,
		Payload:    Payload{Audio: payload},
		Meta:       map[string]string{"source": "audio_source"},
	}
	b.ring.Push(obs)

	b.mu.Lock()
	if ts > b.lastSeenAt {
		b.lastSeenAt = ts
	}
	b.audioCount++
	if payload.Text == "" && len(payload.Tokens) == 0 {
		b.degraded++
	}
	b.lastAudio = payload
	b.mu.Unlock()

	b.computeAndPublish()
}

// nextSeq returns the next global observation sequence number.
func (b *Bus) nextSeq() uint64 {
	return atomic.AddUint64(&b.seq, 1)
}

// ──────────────────────────────────────────────────────────────
// Projection
// ──────────────────────────────────────────────────────────────

// computeAndPublish rebuilds the WorldState projection and fans it out.
func (b *Bus) computeAndPublish() {
	b.mu.Lock()
	window := b.ring.Snapshots()
	now := Now()
	lastSeen := b.lastSeenAt
	if lastSeen == 0 {
		lastSeen = now
	}
	b.version++
	version := b.version
	rel := b.computeRelationsLocked(window)
	ws := &WorldState{
		Timestamp:  now,
		Version:    version,
		LastSeenAt: lastSeen,
		Window:     window,
		Vision:     b.lastVision,
		Audio:      b.lastAudio,
		Relations:  rel,
		Degraded:   b.degraded > 0,
		Warnings:   b.warningsLocked(),
		Metrics: &Metrics{
			Observations:  uint64(len(window)),
			Vision:        b.visionCount,
			Audio:         b.audioCount,
			WindowSeconds: b.cfg.Window.Seconds(),
			Degraded:      b.degraded,
		},
	}
	b.world = ws
	cp := copyWorldState(ws)
	b.mu.Unlock()

	b.publish(cp)
}

// computeRelationsLocked binds each audio observation in the window to the
// vision observations that overlapped it. Called under b.mu; it reads the ring
// snapshots passed in.
func (b *Bus) computeRelationsLocked(window []Observation) []MultiRel {
	var rels []MultiRel
	for i := range window {
		a := &window[i]
		if a.Modality != ModalityAudio {
			continue
		}
		if m := match(*a, window, b.cfg.Tolerance); m != nil {
			rels = append(rels, *m)
		}
	}
	return rels
}

// warningsLocked computes non-fatal degradation notes. Called under b.mu.
func (b *Bus) warningsLocked() []string {
	var w []string
	if b.visionSvc == nil || !b.visionSvc.Enabled() {
		w = append(w, "perception bus: vision source not wired")
	}
	if b.audioSrc == nil {
		w = append(w, "perception bus: audio source not wired")
	}
	if b.degraded > 0 {
		w = append(w, "perception bus: degraded observations detected")
	}
	return w
}

// publish fans a WorldState out to every subscriber. Non-blocking: a slow
// subscriber is dropped rather than backpressuring the bus.
func (b *Bus) publish(ws *WorldState) {
	b.subMu.Lock()
	defer b.subMu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- ws:
		default:
			// slow consumer — drop this update for it
		}
	}
}

// copyWorldState returns a defensive deep-ish copy of a WorldState so callers
// cannot mutate the bus's internal state.
func copyWorldState(st *WorldState) *WorldState {
	if st == nil {
		return nil
	}
	c := *st
	c.Window = make([]Observation, len(st.Window))
	for i := range st.Window {
		c.Window[i] = copyObservation(st.Window[i])
	}
	c.Vision = copyVision(st.Vision)
	c.Audio = copyAudio(st.Audio)
	c.Relations = make([]MultiRel, len(st.Relations))
	for i := range st.Relations {
		r := st.Relations[i]
		r.Overlap = make([]WindowRef, len(r.Overlap))
		for j := range st.Relations[i].Overlap {
			r.Overlap[j] = copyWindowRef(st.Relations[i].Overlap[j])
		}
		c.Relations[i] = r
	}
	c.Warnings = append([]string(nil), st.Warnings...)
	if st.Metrics != nil {
		m := *st.Metrics
		c.Metrics = &m
	}
	return &c
}

// copyObservation returns a defensive copy of an Observation (payload pointers
// are shared, read-only).
func copyObservation(o Observation) Observation {
	n := o
	if o.Payload.Vision != nil {
		v := *o.Payload.Vision
		v.Entities = append([]worldmodel.WorldEntity(nil), o.Payload.Vision.Entities...)
		v.Relations = append([]worldmodel.SpatialRelation(nil), o.Payload.Vision.Relations...)
		v.Warnings = append([]string(nil), o.Payload.Vision.Warnings...)
		n.Payload.Vision = &v
	}
	if o.Payload.Audio != nil {
		a := *o.Payload.Audio
		a.Tokens = append([]Token(nil), o.Payload.Audio.Tokens...)
		n.Payload.Audio = &a
	}
	return n
}

// copyWindowRef returns a defensive copy of a WindowRef.
func copyWindowRef(r WindowRef) WindowRef {
	n := r
	n.Entities = append([]worldmodel.WorldEntity(nil), r.Entities...)
	return n
}

// copyVision returns a defensive copy of a vision observation.
func copyVision(v *vision.Observation) *vision.Observation {
	if v == nil {
		return nil
	}
	n := *v
	n.Entities = append([]worldmodel.WorldEntity(nil), v.Entities...)
	n.Relations = append([]worldmodel.SpatialRelation(nil), v.Relations...)
	n.Warnings = append([]string(nil), v.Warnings...)
	return &n
}

// copyAudio returns a defensive copy of an audio payload.
func copyAudio(a *AudioPayload) *AudioPayload {
	if a == nil {
		return nil
	}
	n := *a
	n.Tokens = append([]Token(nil), a.Tokens...)
	return &n
}
