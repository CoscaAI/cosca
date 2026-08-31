package handler

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/internal/perception/bus"
)

// busAudioSource is an injectable AudioSource over a manual channel.
type busAudioSource struct{ ch chan bus.AudioSample }

func (s *busAudioSource) Stream() <-chan bus.AudioSample { return s.ch }
func (s *busAudioSource) Close() error                   { return nil }

// ──────────────────────────────────────────────────────────────
// /v1/perception/bus/state
// ──────────────────────────────────────────────────────────────

func TestBusHandler_State_NilService(t *testing.T) {
	h := NewBusHandler(nil)
	rr := httptest.NewRecorder()
	h.State(rr, httptest.NewRequest(http.MethodGet, "/v1/perception/bus/state", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("State nil → code = %d, want 503", rr.Code)
	}
}

func TestBusHandler_State_Enabled(t *testing.T) {
	src := &busAudioSource{ch: make(chan bus.AudioSample, 1)}
	src.ch <- bus.AudioSample{Timestamp: bus.Now(), Payload: bus.AudioPayload{Text: "olá", SegmentID: 3, IsFinal: true}}
	b := bus.NewBus(bus.Config{Window: time.Second, MaxObs: 16, Tolerance: 300 * time.Millisecond}, nil, src, zerolog.Nop())
	b.Start(context.Background())
	defer b.Stop()

	// Wait until the bus has ingested the audio sample and projected a state.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if b.State() != nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	h := NewBusHandler(b)
	rr := httptest.NewRecorder()
	h.State(rr, httptest.NewRequest(http.MethodGet, "/v1/perception/bus/state", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("State enabled → code = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "olá") {
		t.Errorf("State body does not contain the audio text: %s", rr.Body.String())
	}
}

// ──────────────────────────────────────────────────────────────
// /v1/perception/bus (SSE)
// ──────────────────────────────────────────────────────────────

func TestBusHandler_Stream_NilService(t *testing.T) {
	h := NewBusHandler(nil)
	rr := httptest.NewRecorder()
	h.Stream(rr, httptest.NewRequest(http.MethodGet, "/v1/perception/bus", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Stream nil → code = %d, want 503", rr.Code)
	}
}

func TestBusHandler_Stream_Enabled(t *testing.T) {
	src := &busAudioSource{ch: make(chan bus.AudioSample, 1)}
	src.ch <- bus.AudioSample{Payload: bus.AudioPayload{Text: "olá", SegmentID: 3}}
	b := bus.NewBus(bus.Config{Window: time.Second, MaxObs: 16, Tolerance: 300 * time.Millisecond}, nil, src, zerolog.Nop())
	b.Start(context.Background())
	defer b.Stop()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		NewBusHandler(b).Stream(w, r)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Stream enabled → status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	// Read the first SSE event — it must be a `bus` frame.
	br := bufio.NewReader(resp.Body)
	sawBusEvent := false
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			break // context cancelled — we already read what we needed
		}
		if strings.HasPrefix(strings.TrimSpace(line), "event:") && strings.Contains(line, "bus") {
			sawBusEvent = true
			break
		}
	}
	cancel() // stop the SSE loop so the server can shut down

	if !sawBusEvent {
		t.Error("expected a `event: bus` SSE frame from /v1/perception/bus")
	}
}
