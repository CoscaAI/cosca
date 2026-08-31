// Package perception implements the continuous Perception Loop for the Cosca
// agent.
//
// The loop turns a one-shot "capability" into "presença contínua": a
// background goroutine captures the screen (or camera) at a configurable
// interval, runs the native Go ONNX vision pipeline over the frame, updates a
// perceptual WorldState (the most recent Observation + timestamp + entities),
// and emits the state to SSE subscribers (/v1/perception/stream) and the
// read endpoint (/v1/perception/state).
//
// Flow:
//
//	SCREEN (CaptureScreen)
//	  → VISU (vision.DetectImageAndRunVision, native Go ONNX: CLIP +
//	    GroundingDINO + Depth)
//	  → WorldState perceptual (entities + summary + degraded flags)
//	  → emit (SSE / subscribers + read endpoint)
//
// Degradation contract (never crash):
//   - capture fails            → Degraded state with a warning, loop continues
//   - vision models absent     → Observation with Warnings (pipeline degrade)
//   - vision errors            → Degraded state, loop continues
//   - a tick exceeds MaxInterval → frame skipped (throttle/backpressure)
package perception

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/worldmodel"
	"github.com/CoscaAI/cosca/internal/worldmodel/vision"
)

// ──────────────────────────────────────────────────────────────
// Defaults
// ──────────────────────────────────────────────────────────────

const (
	// DefaultInterval is the nominal capture/vision cadence when not set
	// (normal mode / conservative presence).
	DefaultInterval = 2 * time.Second
	// DefaultMaxInterval bounds a single tick. When exceeded the frame is
	// skipped instead of accumulating (backpressure).
	DefaultMaxInterval = 30 * time.Second
	// DefaultCapture is the default capture source ("screen").
	DefaultCapture = "screen"

	// ModeNormal is the default: conservative continuous presence (2s).
	ModeNormal = "normal"
	// ModeRealtime tries to keep up with the capture (short cadence, e.g. 150ms)
	// and drops frames it cannot keep up with rather than queuing.
	ModeRealtime = "realtime"
	// ModeBenchmark processes as fast as possible (no limiter) to measure real
	// throughput, dropping frames deliberately.
	ModeBenchmark = "benchmark"
	// DefaultMode is the mode assumed when the config does not set one.
	DefaultMode = ModeNormal
	// DefaultRealtimeInterval is the cadence ModeRealtime falls back to when the
	// caller does not set an explicit interval (~6-7 FPS, keep-up-with-screen).
	DefaultRealtimeInterval = 150 * time.Millisecond
)

// ──────────────────────────────────────────────────────────────
// Configuration
// ──────────────────────────────────────────────────────────────

// Config configures the PerceptionService.
type Config struct {
	// Enabled turns the loop on. When false the service is inert (State()
	// returns an empty state and Start is a no-op).
	Enabled bool
	// Mode is the experimental mode: "normal", "realtime" or "benchmark".
	// Empty defaults to "normal". See the constants ModeNormal/ModeRealtime/
	// ModeBenchmark. The mode drives the *default* cadence and the backpressure
	// policy (realtime/benchmark drop rather than queue).
	Mode string
	// Interval is the capture/vision cadence. Defaults follow the mode:
	// normal → DefaultInterval (2s), realtime → DefaultRealtimeInterval (~150ms),
	// benchmark → 0 (as fast as possible). An explicit `interval: 0` also means
	// "run as fast as possible" (no limiter between ticks).
	Interval time.Duration
	// MaxInterval bounds a single tick. Defaults to DefaultMaxInterval. When a
	// tick exceeds it the frame is cancelled and counted as a dropped frame
	// (drops > queue: never accumulate).
	MaxInterval time.Duration
	// Capture is the capture source ("screen" or "camera"). Only "screen" is
	// implemented by the default captor; "camera" degrades gracefully.
	Capture string
	// ModelsDir optionally overrides the vision .onnx models directory.
	ModelsDir string
}

// resolvedMode returns the canonical mode, normalising empty/unknown values to
// ModeNormal so downstream code never sees a half-configured mode.
func (c Config) resolvedMode() string {
	switch strings.ToLower(c.Mode) {
	case ModeRealtime, ModeBenchmark:
		return strings.ToLower(c.Mode)
	default:
		return ModeNormal
	}
}

// resolvedInterval returns the effective capture cadence after applying the
// mode's default. Rules:
//
//   - an explicit positive interval is honoured exactly;
//   - an explicit `interval: 0` means "as fast as possible" (no limiter);
//   - a negative interval (unset in a raw struct) falls back to the mode default;
//   - in realtime/benchmark, an interval left at the normal default (DefaultInterval)
//     is treated as "not tuned for this mode" and replaced by the mode cadence.
func (c Config) resolvedInterval() time.Duration {
	switch c.resolvedMode() {
	case ModeRealtime:
		if c.Interval <= 0 || c.Interval == DefaultInterval {
			return DefaultRealtimeInterval
		}
	case ModeBenchmark:
		if c.Interval <= 0 || c.Interval == DefaultInterval {
			return 0 // maximum throughput
		}
	default: // normal
		if c.Interval < 0 {
			return DefaultInterval
		}
	}
	return c.Interval
}

// ──────────────────────────────────────────────────────────────
// Options
// ──────────────────────────────────────────────────────────────

// Option customises a PerceptionService (injectable seams for tests).
type Option func(*Service)

// WithCaptor injects a Captor. Default: ScreenCaptor.
func WithCaptor(c Captor) Option {
	return func(s *Service) { s.captor = c }
}

// WithVision injects a VisionRunner. Default: the native Go ONNX pipeline.
func WithVision(v VisionRunner) Option {
	return func(s *Service) { s.vision = v }
}

// WithLogger injects a logger.
func WithLogger(l zerolog.Logger) Option {
	return func(s *Service) { s.logger = l }
}

// VisionRunner runs the vision pipeline over a captured frame and returns the
// Observation. It is the injectable seam so tests can supply a deterministic
// fake without loading onnxruntime.
type VisionRunner func(ctx context.Context, frame []byte, width, height int) (*vision.Observation, error)

// defaultVision runs the production native Go ONNX vision pipeline
// (best-effort: a missing model degrades to Warnings, never errors the loop).
func defaultVision(ctx context.Context, frame []byte, _, _ int) (*vision.Observation, error) {
	obs, err := vision.DetectImageAndRunVision(ctx, frame, "")
	if err != nil {
		return nil, err
	}
	return obs, nil
}

// ──────────────────────────────────────────────────────────────
// Perceptual WorldState
// ──────────────────────────────────────────────────────────────

// State is the perceptual WorldState — the most recent Observation plus its
// timestamp/version. It is what the SSE stream and the read endpoint expose.
//
// Entities/relations mirror the live Vision Observation; Summary is the
// human/machine-readable SummaryText(). Degraded and Warnings signal graceful
// degradation (e.g. no models, capture failure) so consumers know the state is
// a best-effort view, not a full perception.
type State struct {
	// Timestamp is the UTC time of the update.
	Timestamp time.Time `json:"timestamp"`
	// Version is a monotonic revision counter (fencing: consumers can detect
	// a stale view by comparing Version).
	Version uint64 `json:"version"`
	// Capture is the capture source ("screen", "camera").
	Capture string `json:"capture"`
	// Summary is the Observation.SummaryText() — a short textual description.
	Summary string `json:"summary"`
	// Entities are the detected world entities (fresh frame).
	Entities []worldmodel.WorldEntity `json:"entities"`
	// Relations are the inferred spatial relations between entities.
	Relations []worldmodel.SpatialRelation `json:"relations"`
	// Warnings are non-fatal degradation notes.
	Warnings []string `json:"warnings,omitempty"`
	// Degraded is true when capture/vision degraded (models absent, capture
	// failed, etc.) — the state is a best-effort view.
	Degraded bool `json:"degraded"`
	// Width is the captured frame width in pixels.
	Width int `json:"width"`
	// Height is the captured frame height in pixels.
	Height int `json:"height"`
	// LatencyMS is the vision pipeline latency for this frame (0 on degraded).
	LatencyMS int64 `json:"latency_ms,omitempty"`
	// Metrics carries the performance telemetry (FPS, p95 latency, dropped
	// frames, CPU/RAM, per-model timings) over the rolling window. Present on
	// every live update so the professor can benchmark the loop.
	Metrics *Metrics `json:"metrics,omitempty"`
}

// ──────────────────────────────────────────────────────────────
// Service
// ──────────────────────────────────────────────────────────────

// Service is the Perception Loop. Start it to begin capturing → vision →
// world-state → emit. Stop it to halt. State() returns the latest perceptual
// WorldState.
type Service struct {
	cfg    Config
	captor Captor
	vision VisionRunner
	logger zerolog.Logger

	mu      sync.RWMutex
	state   *State
	version uint64
	metrics *metricTracker

	subMu sync.Mutex
	subs  map[chan *State]struct{}

	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running atomic.Bool
	// busy marks an in-flight tick so a new slot that fires before the previous
	// tick finishes is dropped (never queued) — the "dropped frames > infinite
	// queue" guarantee.
	busy atomic.Bool
}

// NewService creates a PerceptionService from a Config and options. When the
// captor/vision are not injected, production defaults (ScreenCaptor + native
// ONNX vision) are used. The mode is normalised and the interval is resolved
// from the mode (see Config.resolvedInterval).
func NewService(cfg Config, opts ...Option) *Service {
	cfg.Mode = cfg.resolvedMode()
	if cfg.MaxInterval <= 0 {
		cfg.MaxInterval = DefaultMaxInterval
	}
	if cfg.Capture == "" {
		cfg.Capture = DefaultCapture
	}
	cfg.Interval = cfg.resolvedInterval()
	s := &Service{
		cfg:     cfg,
		captor:  ScreenCaptor{},
		vision:  defaultVision,
		logger:  log.Logger,
		subs:    make(map[chan *State]struct{}),
		metrics: newMetricTracker(),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Mode returns the (normalised) mode of the service.
func (s *Service) Mode() string {
	if s == nil {
		return DefaultMode
	}
	return s.cfg.Mode
}

// Interval returns the effective capture/vision cadence.
func (s *Service) Interval() time.Duration {
	if s == nil {
		return DefaultInterval
	}
	return s.cfg.Interval
}

// Start launches the perception loop as a background goroutine. If the service
// is disabled (cfg.Enabled false) it is a no-op. Calls are idempotent: a
// second Start while running is ignored.
func (s *Service) Start(ctx context.Context) {
	if s == nil {
		return
	}
	if !s.cfg.Enabled {
		return
	}
	if s.running.Swap(true) {
		return
	}
	cctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.wg.Add(1)
	go s.run(cctx)
}

// Stop halts the loop and waits for the running goroutine to exit. Safe to
// call multiple times.
func (s *Service) Stop() {
	if s == nil || !s.running.Load() {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	s.running.Store(false)
}

// State returns the latest perceptual WorldState (deep-ish copy) plus a fresh
// metrics snapshot. When the service is not started yet, it returns an empty
// (zero-version) state with empty metrics.
func (s *Service) State() *State {
	if s == nil {
		return &State{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.stateOrEmptyLocked()
	st.Metrics = s.metrics.snapshot()
	return copyState(st)
}

// Subscribe returns a buffered channel that receives each new State. The
// consumer MUST call Unsubscribe to avoid leaking. Slow consumers are dropped
// (non-blocking send) so a stalled SSE client never blocks the loop.
func (s *Service) Subscribe() chan *State {
	ch := make(chan *State, 8)
	s.subMu.Lock()
	defer s.subMu.Unlock()
	s.subs[ch] = struct{}{}
	return ch
}

// Unsubscribe removes a previously-subscribed channel.
func (s *Service) Unsubscribe(ch chan *State) {
	if ch == nil {
		return
	}
	s.subMu.Lock()
	defer s.subMu.Unlock()
	delete(s.subs, ch)
	close(ch)
}

// Enabled reports whether the loop is enabled by config.
func (s *Service) Enabled() bool {
	return s != nil && s.cfg.Enabled
}

// ──────────────────────────────────────────────────────────────
// Loop
// ──────────────────────────────────────────────────────────────

func (s *Service) run(ctx context.Context) {
	defer s.wg.Done()
	interval := s.cfg.Interval
	if interval < 0 {
		interval = DefaultInterval
	}
	s.logger.Info().
		Dur("interval", interval).
		Str("capture", s.cfg.Capture).
		Str("mode", s.cfg.Mode).
		Msg("perception loop started")

	// Immediate first tick so the world-state is available right away.
	s.tickOnce(ctx)

	if interval <= 0 {
		// Max-speed mode (benchmark / interval: 0): no limiter between ticks.
		// Each tick is still bounded by MaxInterval (a tick that exceeds it is
		// cancelled and counted as a dropped frame). Runs until ctx is done.
		s.logger.Debug().Msg("perception loop running at max speed (interval=0)")
		for {
			if ctx.Err() != nil {
				break
			}
			s.tickOnce(ctx)
		}
		s.logger.Info().Msg("perception loop stopped")
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info().Msg("perception loop stopped")
			return
		case <-ticker.C:
			s.executeTick(ctx)
		}
	}
}

// executeTick runs a single loop tick. It never queues: if a previous tick is
// still running when this slot fires, the slot is dropped (backpressure) — a
// frame in real time is worth more than queuing a stale one.
func (s *Service) executeTick(ctx context.Context) {
	if s.busy.Swap(true) {
		// The previous tick has not finished; drop this slot rather than queue.
		s.recordDrop()
		return
	}
	defer s.busy.Store(false)
	s.tickOnce(ctx)
}

// tickOnce performs a single capture → vision → state emission. It never
// panics and never blocks longer than MaxInterval (a slow vision is dropped).
func (s *Service) tickOnce(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	timeout := s.cfg.MaxInterval
	if timeout <= 0 {
		timeout = DefaultMaxInterval
	}
	tickCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()

	// 1. Capture the screen (or camera).
	frame, w, h, err := s.captor.Capture(tickCtx)
	if err != nil {
		if tickCtx.Err() == context.DeadlineExceeded {
			// The tick exceeded its safety window → counted as a dropped frame.
			s.recordDrop()
		} else {
			s.recordCapFail()
		}
		s.logger.Warn().Err(err).Msg("perception: capture failed — degrading")
		s.publish(s.degradedState("capture_failed", err))
		return
	}

	// 2. Run the vision pipeline over the frame.
	if s.vision == nil {
		st := s.emptyState(tickCtx)
		st.Capture = s.cfg.Capture
		st.Width, st.Height = w, h
		st.Degraded = true
		st.Warnings = []string{"perception degraded: vision runner is nil"}
		st.Summary = "Perception degraded: vision runner is nil"
		s.publish(st)
		return
	}
	obs, err := s.vision(tickCtx, frame, w, h)
	if err != nil {
		if tickCtx.Err() == context.DeadlineExceeded {
			s.recordDrop()
		} else {
			s.recordVisFail()
		}
		s.logger.Warn().Err(err).Msg("perception: vision failed — degrading")
		st := s.degradedState("vision_failed", err)
		st.Width, st.Height = w, h
		s.publish(st)
		return
	}

	// 3. Record performance telemetry (per-model timings when the pipeline
	//    exposes them), then build + store the perceptual world-state.
	elapsed := time.Since(start)
	var clip, grounding, depth, sam float64
	if obs != nil {
		clip = float64(obs.ClipMS)
		grounding = float64(obs.GroundingMS)
		depth = float64(obs.DepthMS)
		sam = float64(obs.SAMMS)
	}
	s.recordMetrics(elapsed, clip, grounding, depth, sam)

	// 4. Build the perceptual world-state from the observation.
	st := s.buildState(obs, w, h)
	s.logger.Debug().
		Uint64("version", st.Version).
		Int("entities", len(st.Entities)).
		Int("width", st.Width).
		Int("height", st.Height).
		Int64("latency_ms", st.LatencyMS).
		Bool("degraded", st.Degraded).
		Msg("perception: state updated")
	s.publish(st)
}

// buildState constructs + stores the perceptual world-state from an Observation
// and returns it (for publication). It also attaches a fresh metrics snapshot.
func (s *Service) buildState(obs *vision.Observation, w, h int) *State {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version++

	summary := ""
	if obs != nil {
		summary = obs.SummaryText()
	}
	st := &State{
		Timestamp: time.Now().UTC(),
		Version:   s.version,
		Capture:   s.cfg.Capture,
		Summary:   summary,
		Width:     w,
		Height:    h,
		Degraded:  obs != nil && len(obs.Warnings) > 0,
		Metrics:   s.metrics.snapshot(),
	}
	if obs != nil {
		st.Entities = obs.Entities
		st.Relations = obs.Relations
		st.Warnings = obs.Warnings
		st.LatencyMS = obs.Latency.Milliseconds()
	}
	s.state = st
	return copyState(st)
}

// degradedState constructs + stores a Degraded state (capture/vision failed)
// and returns it.
func (s *Service) degradedState(reason string, err error) *State {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version++

	st := &State{
		Timestamp: time.Now().UTC(),
		Version:   s.version,
		Capture:   s.cfg.Capture,
		Summary:   "Perception degraded: " + reason,
		Degraded:  true,
		Metrics:   s.metrics.snapshot(),
	}
	st.Warnings = []string{"perception degraded: " + reason}
	if err != nil {
		st.Warnings = append(st.Warnings, err.Error())
	}
	s.state = st
	return copyState(st)
}

// emptyState builds a fresh zero-version state with the current timestamp. It
// acquires the lock because it touches the metrics tracker.
func (s *Service) emptyState(_ context.Context) *State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &State{
		Timestamp: time.Now().UTC(),
		Capture:   s.cfg.Capture,
		Metrics:   s.metrics.snapshot(),
	}
}

// emptyLocked returns an empty not-yet-updated state (called under Lock; the
// caller attaches a fresh metrics snapshot).
func (s *Service) emptyLocked() *State {
	return &State{
		Timestamp: time.Now().UTC(),
		Capture:   s.cfg.Capture,
	}
}

// stateOrEmptyLocked returns the stored state or an empty one when the loop has
// not produced a state yet (called under Lock).
func (s *Service) stateOrEmptyLocked() *State {
	if s.state != nil {
		return s.state
	}
	return s.emptyLocked()
}

// recordMetrics records a completed tick into the performance tracker.
func (s *Service) recordMetrics(elapsed time.Duration, clip, grounding, depth, sam float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics.record(elapsed, clip, grounding, depth, sam)
}

// recordDrop counts a frame that was dropped instead of queued (a tick slot
// skipped because the previous tick was still running, or a safety-timeout).
func (s *Service) recordDrop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics.recordDrop()
}

// recordCapFail counts a capture failure.
func (s *Service) recordCapFail() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics.recordCapFail()
}

// recordVisFail counts a vision-pipeline failure (excluding safety-timeout drops).
func (s *Service) recordVisFail() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics.recordVisFail()
}

// publish fans a new state out to every subscriber. Non-blocking: a slow
// subscriber (e.g. a stalled SSE client) is dropped rather than letting it
// apply backpressure to the perception loop.
func (s *Service) publish(st *State) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	for ch := range s.subs {
		select {
		case ch <- st:
		default:
			// slow consumer — drop this update for it
		}
	}
}

// copyState returns a defensive copy so callers cannot mutate the internal
// state (entities/relations/warnings/metrics slices are copied).
func copyState(st *State) *State {
	if st == nil {
		return &State{}
	}
	c := *st
	c.Entities = append([]worldmodel.WorldEntity(nil), st.Entities...)
	c.Relations = append([]worldmodel.SpatialRelation(nil), st.Relations...)
	c.Warnings = append([]string(nil), st.Warnings...)
	if st.Metrics != nil {
		m := *st.Metrics
		c.Metrics = &m
	}
	return &c
}
