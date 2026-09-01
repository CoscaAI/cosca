// Package visionact implements PERCEPTION BY ACTION (sob demanda), the opposite
// of the continuous Perception Loop.
//
// The Don asked for a lightweight "agent with tools" model: the COSCa should NOT
// process vision every frame all the time (that weighs the system). Instead,
// when the Don ASKS (via a spoken command), the COSCa uses the perception
// TOOLS:
//
//	mic/STT (sempre ouvindo — barato) → reconhece a AÇÃO → dispara a ferramenta
//	→ brain (qwen2.5-coder) interpreta → responde (TTS)
//
// This package is the "ferramenta de percepção (1 frame OU série de frames +
// change-detection)". It is deliberately decoupled from the continuous loop's
// perception.Service: a tool runs ONCE (or a bounded series) per request, then
// returns. Nothing runs in the background. The vision pipeline is the SAME
// native Go ONNX one (internal/worldmodel/vision) so the defaults reuse the
// memoized, leak-avoiding shared pipeline; the seams are injectable so tests
// can run with a fake Captor + VisionRunner (no onnxruntime).
//
// Degradation contract (never crash, never leak):
//   - capture fails             → LookAtScreen returns an error; RecordAndInterpret
//     treats the failed sample as a degraded change (keeps going).
//   - vision models absent      → vision.DetectImageAndRunVision degrades to an
//     Observation with Warnings (the default VisionRunner). The tool still
//     reports the (degraded) observation.
//   - ctx cancelled             → the tool returns ctx.Err() promptly.
//   - one-off pipeline per call → NEVER: the default runner reuses the process-wide
//     memoized pipeline, so a long host never churns ONNX sessions (the leak fix).
package visionact

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/internal/perception"
	"github.com/CoscaAI/cosca/internal/worldmodel/vision"
)

// ──────────────────────────────────────────────────────────────
// Defaults
// ──────────────────────────────────────────────────────────────

const (
	// DefaultRecordInterval is the frame-sampling cadence for RecordAndInterpret
	// when the caller does not override. 1s is a good balance: cheap enough to
	// sample "presença" and dense enough to catch a ~1s UI change.
	DefaultRecordInterval = 1 * time.Second
	// DefaultRecordDuration is the recording duration when the Don says "grava"
	// without a number (e.g. "grava e interpreta"). 30s is the sensible default.
	DefaultRecordDuration = 30 * time.Second
	// DefaultRecordMaxFrames bounds a single recording (safety harness so a
	// pathological "grava 999999 segundos" cannot allocate unbounded work).
	DefaultRecordMaxFrames = 200
)

// ──────────────────────────────────────────────────────────────
// Reused seams (so tests can inject a deterministic fake)
// ──────────────────────────────────────────────────────────────

// Captor captures a screen (or camera) frame. It is the same interface the
// Perception Loop uses; the production default is perception.ScreenCaptor.
type Captor = perception.Captor

// VisionRunner runs the vision pipeline over a captured frame and returns the
// Observation. It is the same seam the Perception Loop uses; the production
// default is vision.DetectImageAndRunVision (which reuses the memoized, shared
// pipeline — no per-call ONNX session churn, the leak fix).
type VisionRunner = perception.VisionRunner

// ──────────────────────────────────────────────────────────────
// Engine
// ──────────────────────────────────────────────────────────────

// Engine runs the perception tools on demand. A zero-value engine is NOT usable;
// construct one via New (production defaults) or NewWith (injected seams/tests).
type Engine struct {
	captor    Captor
	vision    VisionRunner
	logger    zerolog.Logger
	interval  time.Duration
	maxFrames int
}

// Option customises an Engine (injectable seams for tests).
type Option func(*Engine)

// WithCaptor injects the Captor. Default: perception.ScreenCaptor.
func WithCaptor(c Captor) Option {
	return func(e *Engine) { e.captor = c }
}

// WithVision injects the VisionRunner. Default: vision.DetectImageAndRunVision.
func WithVision(v VisionRunner) Option {
	return func(e *Engine) { e.vision = v }
}

// WithLogger injects a logger.
func WithLogger(l zerolog.Logger) Option {
	return func(e *Engine) { e.logger = l }
}

// WithRecordInterval overrides the RecordAndInterpret frame-sampling cadence.
// <=0 falls back to DefaultRecordInterval.
func WithRecordInterval(d time.Duration) Option {
	return func(e *Engine) { e.interval = d }
}

// WithMaxFrames overrides the hard cap of frames per recording. <=0 falls back
// to DefaultRecordMaxFrames.
func WithMaxFrames(n int) Option {
	return func(e *Engine) { e.maxFrames = n }
}

// New creates a production Engine with the real screen captor + native Go ONNX
// vision pipeline (the memoized, leak-avoiding defaults) and the default
// recording cadence/cap. Tests use NewWith to inject a fake captor/vision.
func New(opts ...Option) *Engine {
	e := &Engine{
		captor:    perception.ScreenCaptor{},
		vision:    defaultVision,
		logger:    zerolog.Nop(),
		interval:  DefaultRecordInterval,
		maxFrames: DefaultRecordMaxFrames,
	}
	for _, o := range opts {
		o(e)
	}
	if e.captor == nil {
		e.captor = perception.ScreenCaptor{}
	}
	if e.vision == nil {
		e.vision = defaultVision
	}
	if e.interval <= 0 {
		e.interval = DefaultRecordInterval
	}
	if e.maxFrames <= 0 {
		e.maxFrames = DefaultRecordMaxFrames
	}
	return e
}

// defaultVision runs the production native Go ONNX vision pipeline. It reuses
// the process-wide memoized pipeline (no per-call session churn). A missing
// model degrades to Warnings (never an error) inside the returned Observation.
func defaultVision(ctx context.Context, frame []byte, _, _ int) (*vision.Observation, error) {
	return vision.DetectImageAndRunVision(ctx, frame, "")
}

// ──────────────────────────────────────────────────────────────
// LookAtScreen — "olha a tela" (1 frame, agora)
// ──────────────────────────────────────────────────────────────

// LookAtScreen captures ONE frame and runs the vision pipeline, returning the
// Observation (what is seen NOW). It is the "percepção por ação" primitive: it
// runs once per call and returns — no background work, no continuous loop.
//
// Degradation contract:
//   - capture fails     → returns (nil, err) so the caller can degrade gracefully.
//   - vision degrades   → vision.DetectImageAndRunVision returns an Observation
//     carrying Warnings (missing models etc.); we pass that through so the
//     caller can still speak something honest ("percepção degradada").
func (e *Engine) LookAtScreen(ctx context.Context) (*vision.Observation, error) {
	if e == nil {
		return nil, fmt.Errorf("visionact: nil engine")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	frame, _, _, err := e.captor.Capture(ctx)
	if err != nil {
		return nil, fmt.Errorf("capture failed: %w", err)
	}

	obs, err := e.vision(ctx, frame, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("vision failed: %w", err)
	}
	return obs, nil
}

// ──────────────────────────────────────────────────────────────
// RecordAndInterpret — "grava N segundos e interpreta"
// ──────────────────────────────────────────────────────────────

// ChangeEvent is a single detected change during a recording. It carries the
// relative offset (from the start of the recording) and a textual summary of
// what the screen showed at that moment.
type ChangeEvent struct {
	// Offset is the time offset from the start of the recording.
	Offset time.Duration
	// Summary is the vision.SummaryText() of the changed frame, or a degradation
	// note when the vision pipeline could not interpret it.
	Summary string
}

// RecordAndInterpret captures frames at a cadence for `duration`, uses the
// pure-Go change-detection gate to detect MOVEMENT/CHANGE between frames, runs
// the vision pipeline only on changed frames, and returns a textual report of
// what appeared/changed over the interval ("a tela mudou: apareceu X em t=…").
//
// Degradation contract (never crash):
//   - a capture that fails is treated as a degraded change (keeps recording);
//   - a change whose vision pipeline degrades is recorded with a degradation note;
//   - the recording is bounded by maxFrames (safety cap) and by ctx;
//   - the first frame is always a change (no reference yet) — it establishes the
//     "estado inicial" of the interval.
func (e *Engine) RecordAndInterpret(ctx context.Context, duration time.Duration) (string, error) {
	if e == nil {
		return "", fmt.Errorf("visionact: nil engine")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if duration <= 0 {
		duration = DefaultRecordDuration
	}
	// A fresh detector per recording, so two recordings do not leak their
	// reference frame into each other.
	detector := perception.NewChangeDetector(perception.DefaultChangeDetectionThreshold)

	start := time.Now()
	var events []ChangeEvent
	seen := 0

	// sample captures one frame and, if it is a change, runs vision on it. It is
	// never fatal: a bad frame is recorded as a degraded change.
	sample := func() {
		frame, _, _, err := e.captor.Capture(ctx)
		if err != nil {
			e.logger.Warn().Err(err).Msg("visionact: record: capture failed (degraded change)")
			events = append(events, ChangeEvent{
				Offset:  time.Since(start),
				Summary: "mudança detectada (captura de tela degradada)",
			})
			return
		}
		if !detector.Evaluate(frame, 0, 0) {
			// No meaningful change → skip the heavy vision inference.
			return
		}
		obs, err := e.vision(ctx, frame, 0, 0)
		summary := ""
		if err != nil {
			e.logger.Warn().Err(err).Msg("visionact: record: vision degraded")
			summary = "mudança detectada (percepção de visão degradada)"
		} else if obs != nil {
			summary = obs.SummaryText()
		} else {
			summary = "mudança detectada"
		}
		events = append(events, ChangeEvent{Offset: time.Since(start), Summary: summary})
	}

	// Immediate first sample (estado inicial), then tick on the cadence.
	sample()
	seen++

	if seen >= e.maxFrames || time.Since(start) >= duration {
		return formatRecordReport(duration, events), nil
	}

	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return formatRecordReport(duration, events), ctx.Err()
		case <-ticker.C:
			sample()
			seen++
			if seen >= e.maxFrames || time.Since(start) >= duration {
				return formatRecordReport(duration, events), nil
			}
		}
	}
}

// ──────────────────────────────────────────────────────────────
// Report formatting
// ──────────────────────────────────────────────────────────────

// formatRecordReport renders a recording as a short, factual PT-BR narrative.
// It is a pure function (no I/O), so it is trivially unit-testable.
func formatRecordReport(duration time.Duration, events []ChangeEvent) string {
	var b strings.Builder
	n := len(events)
	if n == 0 {
		fmt.Fprintf(&b, "Durante %.0f segundos a tela não apresentou mudanças detectáveis.",
			duration.Seconds())
		return b.String()
	}

	fmt.Fprintf(&b, "Registrei a tela por %.0f segundos e detectei %d mudança(s) significativa(s):",
		duration.Seconds(), n)
	for _, ev := range events {
		sum := strings.TrimSpace(ev.Summary)
		if sum == "" {
			sum = "mudança visual"
		}
		fmt.Fprintf(&b, "\n  em t=%.1fs: %s", ev.Offset.Seconds(), sum)
	}
	return b.String()
}
