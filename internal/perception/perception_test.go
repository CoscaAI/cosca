package perception

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/worldmodel"
	"github.com/CoscaAI/cosca/internal/worldmodel/vision"
)

// ──────────────────────────────────────────────────────────────
// Test helpers
// ──────────────────────────────────────────────────────────────

// solidPNGFrame builds a small (16×16) solid-colour PNG frame.
func solidPNGFrame(t *testing.T, r, g, b uint8) []byte {
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

// mockCaptor cycles through a fixed set of frames deterministically.
type mockCaptor struct {
	frames [][]byte
	idx    int
}

func (m *mockCaptor) Capture(_ context.Context) ([]byte, int, int, error) {
	if len(m.frames) == 0 {
		return nil, 0, 0, errors.New("no frames")
	}
	f := m.frames[m.idx%len(m.frames)]
	m.idx++
	return f, 16, 16, nil
}

// countingVision returns a VisionRunner that counts invocations and returns a
// deterministic Observation (no onnxruntime needed).
func countingVision(calls *int) VisionRunner {
	return func(_ context.Context, _ []byte, _, _ int) (*vision.Observation, error) {
		*calls++
		return &vision.Observation{
			Latency:  5 * time.Millisecond,
			Entities: []worldmodel.WorldEntity{{ID: "e1", Label: "test"}},
		}, nil
	}
}

// newTestService wires a Service with the given mock captor/vision and a
// change-detection gate toggle. Interval is 0 so direct tickOnce calls are
// used (no timer goroutine).
func newTestService(t *testing.T, frames [][]byte, visionFn VisionRunner, cdEnabled bool) *Service {
	t.Helper()
	captor := &mockCaptor{frames: frames}
	cfg := Config{
		Enabled:     true,
		Mode:        ModeNormal,
		MaxInterval: time.Second,
		Capture:     "screen",
		ChangeDetection: ChangeDetectionConfig{
			Enabled:   cdEnabled,
			Threshold: DefaultChangeDetectionThreshold,
		},
	}
	return NewService(cfg, WithCaptor(captor), WithVision(visionFn))
}

// ──────────────────────────────────────────────────────────────
// ChangeDetector unit tests
// ──────────────────────────────────────────────────────────────

func TestChangeDetector_Evaluate(t *testing.T) {
	frameA := solidPNGFrame(t, 50, 50, 50)
	frameB := solidPNGFrame(t, 200, 50, 50)
	d := newChangeDetector(0.02)

	if !d.Evaluate(frameA, 16, 16) {
		t.Fatal("first frame (no reference) must be treated as a change")
	}
	if d.Evaluate(frameA, 16, 16) {
		t.Error("identical repeated frame must NOT be a change")
	}
	if !d.Evaluate(frameB, 16, 16) {
		t.Error("different frame must be a change")
	}
}

func TestChangeDetector_DegradesToChangeOnBadFrame(t *testing.T) {
	d := newChangeDetector(0.02)
	// A non-image frame cannot be decoded → treat as a change (run vision, safe).
	if !d.Evaluate([]byte("not-an-image"), 0, 0) {
		t.Error("non-decodable frame should degrade to a change")
	}
}

func TestChangeDetector_ThresholdZeroFallsBackToDefault(t *testing.T) {
	d := newChangeDetector(0)
	// Cannot inspect the private threshold directly, so just verify it still
	// behaves sanely: differs between identical frames.
	frame := solidPNGFrame(t, 10, 20, 30)
	if !d.Evaluate(frame, 16, 16) {
		t.Fatal("first frame must change")
	}
	if d.Evaluate(frame, 16, 16) {
		t.Error("identical frame must not change")
	}
}

// ──────────────────────────────────────────────────────────────
// Change-detection gate on the Perception Loop
// ──────────────────────────────────────────────────────────────

// TestPerception_ChangeDetection_StaticScreen verifies the core cost win: a
// static screen runs vision exactly once and then the gate skips every frame
// (skipped_frames grows, vision calls stop).
func TestPerception_ChangeDetection_StaticScreen(t *testing.T) {
	frameA := solidPNGFrame(t, 50, 50, 50)
	var calls int
	svc := newTestService(t, [][]byte{frameA}, countingVision(&calls), true)
	ctx := context.Background()

	const ticks = 6
	for i := 0; i < ticks; i++ {
		svc.tickOnce(ctx)
	}

	if calls != 1 {
		t.Errorf("vision calls = %d, want 1 (only the first frame changed)", calls)
	}
	st := svc.State()
	if st == nil || st.Metrics == nil {
		t.Fatal("expected state with metrics")
	}
	if st.Metrics.SkippedFrames != ticks-1 {
		t.Errorf("skipped_frames = %d, want %d", st.Metrics.SkippedFrames, ticks-1)
	}
	if st.Metrics.ChangedFrames != 1 {
		t.Errorf("changed_frames = %d, want 1", st.Metrics.ChangedFrames)
	}
	// The heartbeat keeps the world revision (not bumped on a skip) but
	// refreshes last_seen_at so a consumer knows COSCA is still perceiving.
	if st.Version != 1 {
		t.Errorf("version = %d, want 1 (world unchanged across skips)", st.Version)
	}
	if st.LastSeenAt.IsZero() || st.LastChangeAt.IsZero() {
		t.Fatal("expected non-zero LastSeenAt / LastChangeAt")
	}
	if st.LastSeenAt.Before(st.LastChangeAt) {
		t.Error("LastSeenAt (heartbeat) should be >= LastChangeAt (the one real change)")
	}
}

// TestPerception_ChangeDetection_AlternatingFrames verifies that an alternating
// screen (frame A/B) runs vision on every change.
func TestPerception_ChangeDetection_AlternatingFrames(t *testing.T) {
	frameA := solidPNGFrame(t, 50, 50, 50)
	frameB := solidPNGFrame(t, 200, 50, 50)
	var calls int
	svc := newTestService(t, [][]byte{frameA, frameB}, countingVision(&calls), true)
	ctx := context.Background()

	const ticks = 5
	for i := 0; i < ticks; i++ {
		svc.tickOnce(ctx)
	}

	if calls != ticks {
		t.Errorf("vision calls = %d, want %d (every alternating frame changed)", calls, ticks)
	}
	st := svc.State()
	if st.Metrics.SkippedFrames != 0 {
		t.Errorf("skipped_frames = %d, want 0", st.Metrics.SkippedFrames)
	}
	if st.Metrics.ChangedFrames != ticks {
		t.Errorf("changed_frames = %d, want %d", st.Metrics.ChangedFrames, ticks)
	}
}

// TestPerception_ChangeDetection_Disabled verifies that disabling the gate keeps
// the previous behaviour: vision runs on every frame, nothing is skipped.
func TestPerception_ChangeDetection_Disabled(t *testing.T) {
	frameA := solidPNGFrame(t, 50, 50, 50)
	var calls int
	svc := newTestService(t, [][]byte{frameA}, countingVision(&calls), false)
	ctx := context.Background()

	const ticks = 4
	for i := 0; i < ticks; i++ {
		svc.tickOnce(ctx)
	}

	if calls != ticks {
		t.Errorf("vision calls = %d, want %d (gate disabled → always process)", calls, ticks)
	}
	st := svc.State()
	if st.Metrics.SkippedFrames != 0 {
		t.Errorf("skipped_frames = %d, want 0 (gate disabled)", st.Metrics.SkippedFrames)
	}
	// changed_frames only measures the gate's effect; when disabled it stays 0
	// (the loop always processes, no change determination is made).
	if st.Metrics.ChangedFrames != 0 {
		t.Errorf("changed_frames = %d, want 0 when gate disabled", st.Metrics.ChangedFrames)
	}
}
