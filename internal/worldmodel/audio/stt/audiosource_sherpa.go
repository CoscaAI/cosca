//go:build stt_sherpa

package stt

// SherpaAudioSource is a perception-bus AudioSource that runs native-Go sherpa
// streaming STT on injected PCM and emits bus.AudioSample (with per-token
// timestamps). It is the FASE B bridge between "raw audio in" and the
// multimodal bus (transcriptions stamped on the bus monotonic clock).
//
// The source idles (emits nothing) until a producer calls PushPCM with
// normalized samples — a future mic/streaming capture feeds it. It emits a
// partial AudioSample whenever the transcript changes and a final AudioSample
// when sherpa reports an endpoint (trailing silence / utterance end), then it
// starts a fresh segment.
//
// It deliberately lives in this (cgo) package so the bus and perception
// packages never depend on the C toolchain.

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/internal/perception/bus"
)

// AudioSource implements bus.AudioSource and is fed by PushPCM.
type AudioSource struct {
	engine     Engine
	stream     Stream
	cfg        Config
	sampleRate int
	logger     zerolog.Logger

	out chan bus.AudioSample // consumed by the bus via Stream()
	pcm chan pcmChunk        // producers push raw audio

	done chan struct{}
	wg   sync.WaitGroup

	mu       sync.Mutex
	segID    uint64
	segStart bus.MonotonicTime
	lastText string
	closed   bool
	started  bool
}

// pcmChunk is a slice of normalized samples stamped with its capture-start time.
type pcmChunk struct {
	samples []float32
	sr      int
	at      bus.MonotonicTime
}

// NewAudioSource opens the engine+stream and returns a ready SherpaAudioSource
// that implements bus.AudioSource. Close tears it down.
func NewAudioSource(engine Engine, cfg Config, sampleRate int, logger zerolog.Logger) (*AudioSource, error) {
	if engine == nil {
		return nil, fmt.Errorf("stt: nil engine")
	}
	if sampleRate <= 0 {
		sampleRate = cfg.resolveSampleRate()
	}
	src := &AudioSource{
		engine:     engine,
		cfg:        cfg,
		sampleRate: sampleRate,
		logger:     logger,
		out:        make(chan bus.AudioSample, 16),
		pcm:        make(chan pcmChunk, 64),
		done:       make(chan struct{}),
	}
	if err := src.start(); err != nil {
		return nil, err
	}
	return src, nil
}

// start launches the engine and the decode goroutine.
func (s *AudioSource) start() error {
	if err := s.engine.Open(); err != nil {
		return err
	}
	stream, err := s.engine.NewStream()
	if err != nil {
		return err
	}
	s.stream = stream
	s.mu.Lock()
	s.started = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.run()
	return nil
}

// Stream implements bus.AudioSource. It returns the channel of recognised
// AudioSamples (partials + finals). It is non-blocking for the bus.
func (s *AudioSource) Stream() <-chan bus.AudioSample { return s.out }

// PushPCM delivers a chunk of normalized PCM ([-1,1]) at sampleRate to the STT
// engine. It stamps the chunk with the bus monotonic capture time and never
// blocks the producer (a full buffer drops the chunk — no backpressure). If
// sampleRate <= 0 the source's configured rate is used.
func (s *AudioSource) PushPCM(samples []float32, sampleRate int) {
	if samples == nil || len(samples) == 0 {
		return
	}
	if sampleRate <= 0 {
		sampleRate = s.sampleRate
	}
	select {
	case s.pcm <- pcmChunk{samples: samples, sr: sampleRate, at: bus.Now()}:
	case <-s.done:
	default:
		s.logger.Warn().Msg("stt audio source: pcm buffer full, dropping chunk")
	}
}

// Close implements bus.AudioSource. Idempotent.
func (s *AudioSource) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	// Flush the recognizer tail before tearing down.
	if s.stream != nil {
		_ = s.stream.InputFinished()
		for s.stream.Ready() {
			_ = s.stream.Decode()
		}
		if res, err := s.stream.Result(); err == nil && res != nil && res.Text != "" {
			s.emit(partialSample(s.nextSeg(), res, s.segStart, false))
		}
	}

	close(s.done)
	s.wg.Wait()

	if s.stream != nil {
		_ = s.stream.Close()
	}
	_ = s.engine.Close()
	return nil
}

// run is the decode loop: it pulls PCM chunks, feeds the recognizer, decodes,
// emits partials, and finalises segments at endpoints.
func (s *AudioSource) run() {
	defer s.wg.Done()
	for {
		select {
		case <-s.done:
			return
		case ch := <-s.pcm:
			s.handleChunk(ch)
		}
	}
}

func (s *AudioSource) handleChunk(ch pcmChunk) {
	if s.stream == nil {
		return
	}
	s.mu.Lock()
	if s.segStart == 0 {
		s.segStart = ch.at
	}
	s.mu.Unlock()

	if err := s.stream.AcceptWaveform(ch.samples, ch.sr); err != nil {
		s.logger.Warn().Err(err).Msg("stt feed error")
		return
	}
	for s.stream.Ready() {
		if err := s.stream.Decode(); err != nil {
			s.logger.Warn().Err(err).Msg("stt decode error")
			return
		}
	}

	res, err := s.stream.Result()
	if err != nil {
		s.logger.Warn().Err(err).Msg("stt result error")
		return
	}
	if res == nil {
		return
	}

	s.mu.Lock()
	segStart := s.segStart
	segID := s.segID
	s.mu.Unlock()

	// Emit a partial only when the transcript advanced (avoid spam).
	if res.Text != "" && res.Text != s.lastText {
		s.lastText = res.Text
		s.emit(partialSample(segID, res, segStart, false))
	}

	// Finalise on endpoint: emit the final, reset, and start the next segment.
	if s.stream.IsEndpoint() {
		s.emit(partialSample(segID, res, segStart, true))
		_ = s.stream.Reset()
		s.mu.Lock()
		s.segStart = 0
		s.segID++
		s.lastText = ""
		s.mu.Unlock()
	}
}

// nextSeg returns the current segment id (for the Close flush path).
func (s *AudioSource) nextSeg() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.segID
}

// emit publishes an AudioSample to the bus (non-blocking).
func (s *AudioSource) emit(sample bus.AudioSample) {
	select {
	case s.out <- sample:
	case <-s.done:
	default:
		s.logger.Warn().Msg("stt audio source: bus sample buffer full, dropping")
	}
}

// partialSample converts an STT result into a bus.AudioSample. `final` marks an
// endpoint-finalised segment. segStart may be zero (the bus will stamp Now()).
func partialSample(segID uint64, res *Result, segStart bus.MonotonicTime, final bool) bus.AudioSample {
	payload := bus.AudioPayload{
		Text:      res.Text,
		Tokens:    toBusTokens(res.Tokens),
		IsFinal:   final,
		SegmentID: segID,
		Language:  "",
	}
	return bus.AudioSample{
		Timestamp: segStart,
		Payload:   payload,
	}
}

// toBusTokens maps stt.Token to bus.Token (word + offset + confidence).
func toBusTokens(tokens []Token) []bus.Token {
	if len(tokens) == 0 {
		return nil
	}
	out := make([]bus.Token, len(tokens))
	for i, t := range tokens {
		out[i] = bus.Token{
			Word:       t.Word,
			Offset:     t.Offset,
			Confidence: t.Conf,
		}
	}
	return out
}
