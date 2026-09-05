package visionact

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/internal/worldmodel"
	"github.com/CoscaAI/cosca/internal/worldmodel/vision"
)

// ──────────────────────────────────────────────────────────────
// Test helpers
// ──────────────────────────────────────────────────────────────

// solidPNG builds a small solid-colour PNG frame for tests.
func solidPNG(t *testing.T, r, g, b uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// fakeCaptor returns a fixed frame sequence and records capture calls. It can
// also be made to fail every capture (fine for the degradation tests).
type fakeCaptor struct {
	frames   [][]byte
	idx      int
	fail     bool
	calls    int
	width    int
	height   int
}

func (f *fakeCaptor) Capture(_ context.Context) ([]byte, int, int, error) {
	f.calls++
	if f.fail {
		return nil, 0, 0, errors.New("no display")
	}
	if len(f.frames) == 0 {
		return nil, 0, 0, errors.New("no frames")
	}
	idx := f.idx % len(f.frames)
	f.idx++
	return f.frames[idx], f.width, f.height, nil
}

// fakeVision returns a deterministic Observation (recording the labels) and
// counts invocations. It can be made to fail.
type fakeVision struct {
	label string
	err   error
	calls int
}

func (f *fakeVision) run(_ context.Context, _ []byte, _, _ int) (*vision.Observation, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	label := f.label
	if label == "" {
		label = "objeto"
	}
	return &vision.Observation{
		Latency: 2 * time.Millisecond,
		Entities: []worldmodel.WorldEntity{
			{Label: label, Confidence: 0.91, Depth: 3.0},
		},
		Warnings: nil,
	}, nil
}

// newTestEngine wires an Engine with the given captor/vision (no onnxruntime).
func newTestEngine(t *testing.T, cap *fakeCaptor, vis *fakeVision, interval time.Duration) *Engine {
	t.Helper()
	if interval <= 0 {
		interval = time.Millisecond
	}
	return New(
		WithCaptor(cap),
		WithVision(vis.run),
		WithLogger(zerolog.New(zerolog.Nop())),
		WithRecordInterval(interval),
		WithMaxFrames(50),
	)
}

// ──────────────────────────────────────────────────────────────
// LookAtScreen
// ──────────────────────────────────────────────────────────────

func TestLookAtScreen_Success(t *testing.T) {
	cap := &fakeCaptor{frames: [][]byte{solidPNG(t, 10, 20, 30)}, width: 16, height: 16}
	vis := &fakeVision{label: "pessoa"}
	e := newTestEngine(t, cap, vis, time.Second)

	obs, err := e.LookAtScreen(context.Background())
	if err != nil {
		t.Fatalf("LookAtScreen error: %v", err)
	}
	if obs == nil {
		t.Fatal("expected non-nil observation")
	}
	if len(obs.Entities) != 1 || obs.Entities[0].Label != "pessoa" {
		t.Fatalf("expected entity 'pessoa', got %+v", obs.Entities)
	}
	if vis.calls != 1 {
		t.Errorf("vision calls = %d, want 1 (single frame)", vis.calls)
	}
}

func TestLookAtScreen_CaptureFails(t *testing.T) {
	cap := &fakeCaptor{fail: true}
	vis := &fakeVision{label: "x"}
	e := newTestEngine(t, cap, vis, time.Second)

	obs, err := e.LookAtScreen(context.Background())
	if err == nil {
		t.Fatal("expected an error when capture fails")
	}
	if obs != nil {
		t.Fatal("expected nil observation on capture failure")
	}
}

func TestLookAtScreen_CtxCancelled(t *testing.T) {
	cap := &fakeCaptor{frames: [][]byte{solidPNG(t, 5, 5, 5)}, width: 16, height: 16}
	vis := &fakeVision{label: "x"}
	e := newTestEngine(t, cap, vis, time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := e.LookAtScreen(ctx); err == nil {
		t.Fatal("expected ctx.Err() when context is cancelled")
	}
}

// ──────────────────────────────────────────────────────────────
// RecordAndInterpret
// ──────────────────────────────────────────────────────────────

func TestRecordAndInterpret_StaticScreen(t *testing.T) {
	frame := solidPNG(t, 50, 50, 50)
	cap := &fakeCaptor{frames: [][]byte{frame}, width: 16, height: 16}
	vis := &fakeVision{label: "estatico"}
	e := newTestEngine(t, cap, vis, time.Millisecond)

	report, err := e.RecordAndInterpret(context.Background(), 3*time.Millisecond)
	if err != nil {
		t.Fatalf("RecordAndInterpret error: %v", err)
	}
	if report == "" {
		t.Fatal("expected a non-empty report")
	}
	// A static screen should run vision ONLY on the first (change) frame.
	if vis.calls != 1 {
		t.Errorf("vision calls = %d, want 1 (static screen → change-detection skips)", vis.calls)
	}
}

func TestRecordAndInterpret_ChangingScreen(t *testing.T) {
	frameA := solidPNG(t, 40, 40, 40)
	frameB := solidPNG(t, 200, 40, 40)
	cap := &fakeCaptor{frames: [][]byte{frameA, frameB}, width: 16, height: 16}
	vis := &fakeVision{label: "mudou"}
	e := newTestEngine(t, cap, vis, time.Millisecond)

	report, err := e.RecordAndInterpret(context.Background(), 3*time.Millisecond)
	if err != nil {
		t.Fatalf("RecordAndInterpret error: %v", err)
	}
	// Alternating frames should run vision on every change.
	if vis.calls < 2 {
		t.Errorf("vision calls = %d, want >= 2 (changing screen)", vis.calls)
	}
	if !contains(report, "mudança") {
		t.Errorf("report should mention change(s), got %q", report)
	}
}

func TestRecordAndInterpret_CaptureFailsDegrades(t *testing.T) {
	cap := &fakeCaptor{fail: true}
	vis := &fakeVision{label: "x"}
	e := newTestEngine(t, cap, vis, time.Millisecond)

	report, err := e.RecordAndInterpret(context.Background(), 3*time.Millisecond)
	if err != nil {
		t.Fatalf("RecordAndInterpret should degrade (not error) on capture failure, got %v", err)
	}
	if !contains(report, "degradada") {
		t.Errorf("degraded capture should be reported as such, got %q", report)
	}
}

func TestRecordAndInterpret_DurationZeroDefaults(t *testing.T) {
	cap := &fakeCaptor{frames: [][]byte{solidPNG(t, 1, 2, 3)}, width: 16, height: 16}
	vis := &fakeVision{label: "x"}
	e := newTestEngine(t, cap, vis, time.Millisecond)

	// duration <= 0 → defaults to DefaultRecordDuration (30s), but maxFrames cap
	// (50) and interval keep it bounded and fast in this tiny test.
	if _, err := e.RecordAndInterpret(context.Background(), 0); err != nil {
		t.Fatalf("RecordAndInterpret(0) error: %v", err)
	}
}

func TestRecordAndInterpret_RespectsCtx(t *testing.T) {
	cap := &fakeCaptor{frames: [][]byte{solidPNG(t, 1, 2, 3)}, width: 16, height: 16}
	vis := &fakeVision{label: "x"}
	e := newTestEngine(t, cap, vis, time.Hour) // long interval, so ctx cancels first

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := e.RecordAndInterpret(ctx, time.Second)
	if err == nil {
		t.Fatal("expected ctx.Err() when context is cancelled")
	}
}

// ──────────────────────────────────────────────────────────────
// formatRecordReport (pure function)
// ──────────────────────────────────────────────────────────────

func TestFormatRecordReport_NoEvents(t *testing.T) {
	report := formatRecordReport(30*time.Second, nil)
	if !contains(report, "30") || !contains(report, "mudanças") {
		t.Errorf("no-event report should say no changes, got %q", report)
	}
}

func TestFormatRecordReport_WithEvents(t *testing.T) {
	events := []ChangeEvent{
		{Offset: 0, Summary: "Vision observation: 1 entity(ies)"},
		{Offset: 5 * time.Second, Summary: "Vision observation: 2 entity(ies)"},
	}
	report := formatRecordReport(30*time.Second, events)
	if !contains(report, "2 mudança") {
		t.Errorf("report should count changes, got %q", report)
	}
	if !contains(report, "t=0.0s") || !contains(report, "t=5.0s") {
		t.Errorf("report should include per-change timestamps, got %q", report)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && bytes.Contains([]byte(s), []byte(sub))
}
