package bus

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/internal/perception"
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

// fakeCaptor returns a fixed frame (no real screen / OS capture).
type fakeCaptor struct{ frame []byte }

func (c fakeCaptor) Capture(context.Context) ([]byte, int, int, error) {
	return c.frame, 16, 16, nil
}

// fakeVision returns a VisionRunner returning a fixed Observation (no onnxruntime).
func fakeVision(entities ...worldmodel.WorldEntity) perception.VisionRunner {
	return func(_ context.Context, _ []byte, _, _ int) (*vision.Observation, error) {
		return &vision.Observation{
			Entities: entities,
			Latency:  5 * time.Millisecond,
		}, nil
	}
}

// testAudioSource is an injectable AudioSource driven by a manual channel.
type testAudioSource struct{ ch chan AudioSample }

func (s *testAudioSource) Stream() <-chan AudioSample { return s.ch }
func (s *testAudioSource) Close() error               { return nil }

// ──────────────────────────────────────────────────────────────
// MonotonicClock
// ──────────────────────────────────────────────────────────────

func TestMonotonicTime_NonDecreasing(t *testing.T) {
	a := Now()
	time.Sleep(2 * time.Millisecond)
	b := Now()
	if b < a {
		t.Fatalf("Now() must be monotonic: b(%d) < a(%d)", b, a)
	}
	d := b.Sub(a)
	if d <= 0 {
		t.Fatalf("b.Sub(a) = %v, want > 0", d)
	}
	// Duration round-trips exactly (integer nanoseconds).
	if (a + ToMonotonic(d)).Sub(a) != d {
		t.Errorf("Add/Sub round-trip failed: got %v want %v", (a + ToMonotonic(d)).Sub(a), d)
	}
	// Since() references the current clock and is non-negative.
	if a.Since() < 0 {
		t.Errorf("Since() = %v, want >= 0", a.Since())
	}
}

// ──────────────────────────────────────────────────────────────
// Ring
// ──────────────────────────────────────────────────────────────

func TestRing_WindowEviction(t *testing.T) {
	r := NewRing(100*time.Millisecond, 10)
	base := Now()
	r.Push(Observation{Timestamp: base, Sequence: 1})
	r.Push(Observation{Timestamp: base + ToMonotonic(50*time.Millisecond), Sequence: 2})
	r.Push(Observation{Timestamp: base + ToMonotonic(120*time.Millisecond), Sequence: 3})

	snaps := r.Snapshots()
	// seq1 (at base) is 120ms behind newest (base+120ms) → evicted (> 100ms window).
	if len(snaps) != 2 {
		t.Fatalf("window eviction: len=%d, want 2 (seq1 evicted)", len(snaps))
	}
	if snaps[0].Sequence != 2 || snaps[1].Sequence != 3 {
		t.Errorf("unexpected survivors: %v", []uint64{snaps[0].Sequence, snaps[1].Sequence})
	}
}

func TestRing_CapacityEviction(t *testing.T) {
	r := NewRing(0, 3) // window disabled → capacity-only
	base := Now()
	for i := 0; i < 5; i++ {
		r.Push(Observation{Timestamp: base + ToMonotonic(time.Duration(i)*time.Millisecond), Sequence: uint64(i)})
	}
	snaps := r.Snapshots()
	if len(snaps) != 3 {
		t.Fatalf("capacity eviction: len=%d, want 3", len(snaps))
	}
	// Ring keeps the latest 3 (seq 2,3,4).
	if snaps[0].Sequence != 2 || snaps[1].Sequence != 3 || snaps[2].Sequence != 4 {
		t.Errorf("unexpected survivors: %v", []uint64{snaps[0].Sequence, snaps[1].Sequence, snaps[2].Sequence})
	}
}

func TestRing_WindowAround(t *testing.T) {
	r := NewRing(0, 10)
	base := Now()
	for i := 0; i < 5; i++ {
		r.Push(Observation{Timestamp: base + ToMonotonic(time.Duration(i)*100*time.Millisecond), Sequence: uint64(i)})
	}
	// Around t=base+200ms, before/after=100ms → [base+100ms, base+300ms] → seq 1,2,3.
	got := r.WindowAround(base+ToMonotonic(200*time.Millisecond), 100*time.Millisecond, 100*time.Millisecond)
	if len(got) != 3 {
		t.Fatalf("WindowAround len=%d, want 3", len(got))
	}
	if got[0].Sequence != 1 || got[1].Sequence != 2 || got[2].Sequence != 3 {
		t.Errorf("WindowAround wrong seqs: %v", []uint64{got[0].Sequence, got[1].Sequence, got[2].Sequence})
	}
}

func TestRing_SinceFilters(t *testing.T) {
	r := NewRing(0, 10)
	base := Now()
	r.Push(Observation{Timestamp: base, Sequence: 1})
	r.Push(Observation{Timestamp: base + ToMonotonic(100*time.Millisecond), Sequence: 2})
	r.Push(Observation{Timestamp: base + ToMonotonic(200*time.Millisecond), Sequence: 3})
	got := r.Since(base + ToMonotonic(100*time.Millisecond))
	if len(got) != 2 {
		t.Fatalf("Since len=%d, want 2", len(got))
	}
}

// ──────────────────────────────────────────────────────────────
// match / overlap (sync)
// ──────────────────────────────────────────────────────────────

func TestMatch_OverlapBindsAudioToVision(t *testing.T) {
	const tol = 300 * time.Millisecond
	base := Now()

	audio := Observation{
		Timestamp:  base,
		Modality:   ModalityAudio,
		Sequence:   100,
		Duration:   400 * time.Millisecond,
		Confidence: 0.9,
		Payload:    Payload{Audio: &AudioPayload{Text: "voltar", SegmentID: 1, IsFinal: true, Language: "pt-BR"}},
	}
	visionObs := Observation{
		Timestamp:  base + ToMonotonic(100*time.Millisecond), // inside [0,400ms]
		Modality:   ModalityVision,
		Sequence:   5,
		Duration:   50 * time.Millisecond,
		Confidence: 0.95,
		Payload:    Payload{Vision: &vision.Observation{Entities: []worldmodel.WorldEntity{{ID: "e1", Label: "Voltar", Confidence: 0.9}}}},
	}

	m := match(audio, []Observation{visionObs}, tol)
	if m == nil {
		t.Fatal("expected a MultiRel for overlapping audio/vision")
	}
	if m.AudioSeg != 1 {
		t.Errorf("AudioSeg = %d, want 1", m.AudioSeg)
	}
	if m.TextHint != "voltar" {
		t.Errorf("TextHint = %q, want 'voltar'", m.TextHint)
	}
	if len(m.Overlap) != 1 {
		t.Fatalf("Overlap len = %d, want 1", len(m.Overlap))
	}
	if m.Overlap[0].Sequence != 5 {
		t.Errorf("Overlap[0].Sequence = %d, want 5", m.Overlap[0].Sequence)
	}
	if m.Overlap[0].Entities[0].Label != "Voltar" {
		t.Errorf("Overlap[0].Entities[0].Label = %q, want 'Voltar'", m.Overlap[0].Entities[0].Label)
	}
	if m.Confidence <= 0 || m.Confidence > 1 {
		t.Errorf("Confidence = %f, want in (0,1]", m.Confidence)
	}
}

func TestMatch_NoOverlapOutsideTolerance(t *testing.T) {
	const tol = 300 * time.Millisecond
	base := Now()
	audio := Observation{
		Timestamp: base,
		Modality:  ModalityAudio,
		Duration:  200 * time.Millisecond,
		Payload:   Payload{Audio: &AudioPayload{Text: "x", SegmentID: 1}},
	}
	visionFar := Observation{
		Timestamp: base + ToMonotonic(2*time.Second), // far beyond tol
		Modality:  ModalityVision,
		Duration:  10 * time.Millisecond,
		Payload:   Payload{Vision: &vision.Observation{Entities: []worldmodel.WorldEntity{{ID: "e9", Label: "nope"}}}},
	}
	if m := match(audio, []Observation{visionFar}, tol); m != nil {
		t.Errorf("expected nil MultiRel for non-overlapping observations, got %+v", m)
	}
}

func TestMatch_IgnoresNonAudio(t *testing.T) {
	// A vision observation is never itself an audio reference.
	vis := Observation{
		Timestamp: Now(),
		Modality:  ModalityVision,
		Duration:  10 * time.Millisecond,
		Payload:   Payload{Vision: &vision.Observation{Entities: []worldmodel.WorldEntity{{}}}},
	}
	if m := match(vis, []Observation{vis}, 300*time.Millisecond); m != nil {
		t.Errorf("match(non-audio) should be nil, got %+v", m)
	}
}

// ──────────────────────────────────────────────────────────────
// Bus — projection (end-to-end, no goroutines)
// ──────────────────────────────────────────────────────────────

func TestBus_MultimodalProjection(t *testing.T) {
	b := NewBus(Config{
		Window:    5 * time.Second,
		MaxObs:    16,
		Tolerance: 300 * time.Millisecond,
	}, nil, &testAudioSource{ch: make(chan AudioSample)}, zerolog.Nop())

	// vision: a frame with a "Voltar" button.
	b.ingestVision(&perception.State{
		Entities:  []worldmodel.WorldEntity{{ID: "e1", Label: "Voltar", Confidence: 0.9}},
		LatencyMS: 150,
	})
	// audio: someone says "voltar", overlapping in time.
	b.ingestAudio(AudioSample{
		Timestamp: Now(),
		Payload:   AudioPayload{Text: "voltar", SegmentID: 1, IsFinal: true, Language: "pt-BR", SegmentDuration: 400 * time.Millisecond},
	})

	st := b.State()
	if st == nil {
		t.Fatal("expected a WorldState projection")
	}
	if st.Vision == nil {
		t.Error("expected Vision payload in projected WorldState")
	}
	if st.Audio == nil {
		t.Fatal("expected Audio payload in projected WorldState")
	}
	if st.Audio.Text != "voltar" {
		t.Errorf("Audio.Text = %q, want 'voltar'", st.Audio.Text)
	}
	if len(st.Window) < 2 {
		t.Errorf("Window len = %d, want >= 2 (vision + audio)", len(st.Window))
	}
	if len(st.Relations) == 0 {
		t.Error("expected at least one MultiRel binding audio 'voltar' to the overlapping vision frame")
	}
	if st.Relations[0].TextHint != "voltar" {
		t.Errorf("Relations[0].TextHint = %q, want 'voltar'", st.Relations[0].TextHint)
	}
	if st.Relations[0].Overlap[0].Entities[0].Label != "Voltar" {
		t.Errorf("bound vision entities not 'Voltar': %q", st.Relations[0].Overlap[0].Entities[0].Label)
	}
	if st.Version == 0 {
		t.Error("Version should be > 0")
	}
}

func TestBus_SubscribesToPerceptionLoop(t *testing.T) {
	frame := solidPNGFrame(t, 50, 50, 50)
	var calls int
	vis := perception.NewService(perception.Config{
		Enabled:         true,
		Mode:            perception.ModeNormal,
		Interval:        5 * time.Millisecond,
		MaxInterval:     time.Second,
		Capture:         "screen",
		ChangeDetection: perception.ChangeDetectionConfig{Enabled: false},
	}, perception.WithCaptor(fakeCaptor{frame: frame}),
		perception.WithVision(func(_ context.Context, _ []byte, _, _ int) (*vision.Observation, error) {
			calls++
			return &vision.Observation{
				Entities: []worldmodel.WorldEntity{{ID: "e1", Label: "test-button", Confidence: 0.95}},
				Latency:  5 * time.Millisecond,
			}, nil
		}),
		perception.WithLogger(zerolog.Nop()))

	b := NewBus(Config{
		Window: 2 * time.Second, MaxObs: 32, Tolerance: 300 * time.Millisecond,
	}, vis, NoopAudioSource{}, zerolog.Nop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	vis.Start(ctx)
	b.Start(ctx)
	defer b.Stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if st := b.State(); st != nil && st.Vision != nil && len(st.Window) > 0 {
			if st.Vision.Entities[0].Label == "test-button" {
				if calls < 1 {
					t.Errorf("expected the perception loop to have run, calls=%d", calls)
				}
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("bus did not produce a vision WorldState from the perception loop")
}

func TestBus_WatchReceivesWorldState(t *testing.T) {
	b := NewBus(Config{
		Window: 2 * time.Second, MaxObs: 16, Tolerance: 300 * time.Millisecond,
	}, nil, NoopAudioSource{}, zerolog.Nop())
	ch := b.Watch()
	defer b.Unsubscribe(ch)

	b.ingestVision(&perception.State{
		Entities:  []worldmodel.WorldEntity{{ID: "e1", Label: "seen"}},
		LatencyMS: 10,
	})

	select {
	case st := <-ch:
		if st == nil || st.Vision == nil {
			t.Fatalf("expected a vision-marked WorldState on Watch, got %+v", st)
		}
		if len(st.Window) == 0 {
			t.Error("expected a non-empty Window on the published WorldState")
		}
	case <-time.After(time.Second):
		t.Fatal("no WorldState delivered on Watch")
	}
}

func TestBus_StateNilBeforeIngest(t *testing.T) {
	b := NewBus(Config{}, nil, NoopAudioSource{}, zerolog.Nop())
	if st := b.State(); st != nil {
		t.Errorf("State() before ingest should be nil, got %+v", st)
	}
}

func TestBus_WindowAroundQuery(t *testing.T) {
	b := NewBus(Config{Window: 2 * time.Second, MaxObs: 32, Tolerance: 300 * time.Millisecond}, nil, NoopAudioSource{}, zerolog.Nop())
	base := Now()
	b.ring.Push(Observation{Timestamp: base, Modality: ModalityVision, Sequence: 1, Payload: Payload{Vision: &vision.Observation{}}})
	b.ring.Push(Observation{Timestamp: base + ToMonotonic(200*time.Millisecond), Modality: ModalityVision, Sequence: 2, Payload: Payload{Vision: &vision.Observation{}}})
	got := b.WindowAround(base+ToMonotonic(100*time.Millisecond), 150*time.Millisecond, 150*time.Millisecond)
	if len(got) != 2 {
		t.Fatalf("WindowAround len=%d, want 2", len(got))
	}
}
