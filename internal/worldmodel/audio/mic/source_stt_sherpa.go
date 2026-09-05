//go:build stt_sherpa

package mic

// MicrophoneAudioSource closes the audio loop: it wraps a microphone Source and
// a sherpa STT *stt.AudioSource, pumping every captured float32 chunk into
// PushPCM in real time. The STT source runs its own decode goroutine and emits
// bus.AudioSample (partials + finals) on its Stream() — which this source
// re-exposes, so the perception bus synchronises the live transcript with the
// vision loop exactly as it does for the wire-only STT path.
//
// It lives in this (cgo/stt_sherpa) build so the perception/bus packages never
// depend on the C toolchain or the mic capture backend.

import (
	"sync"

	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/internal/perception/bus"
	"github.com/CoscaAI/cosca/internal/worldmodel/audio/stt"
)

// MicrophoneAudioSource implements bus.AudioSource and is fed by a live mic.
type MicrophoneAudioSource struct {
	mic    Source
	stt    *stt.AudioSource
	done   chan struct{}
	wg     sync.WaitGroup
	logger zerolog.Logger
}

// NewMicrophoneAudioSource wires a mic Source to a sherpa STT AudioSource. The
// returned source is bus-ready: Stream() proxies the STT recogniser output and
// Close() tears down both the mic and the recogniser.
func NewMicrophoneAudioSource(micSrc Source, sttSrc *stt.AudioSource, logger zerolog.Logger) *MicrophoneAudioSource {
	s := &MicrophoneAudioSource{
		mic:    micSrc,
		stt:    sttSrc,
		done:   make(chan struct{}),
		logger: logger,
	}
	s.wg.Add(1)
	go s.pump()
	return s
}

// pump forwards every mic chunk to the STT recogniser until closed.
func (s *MicrophoneAudioSource) pump() {
	defer s.wg.Done()
	chunks := s.mic.Chunks()
	for {
		select {
		case <-s.done:
			return
		case ch, ok := <-chunks:
			if !ok {
				return
			}
			if ch.SampleRate <= 0 || len(ch.Samples) == 0 {
				continue
			}
			// PushPCM is non-blocking (drops on a full buffer, matching our
			// own drop-on-slow-consumer policy) and stamps the chunk with the
			// bus monotonic time at push.
			s.stt.PushPCM(ch.Samples, ch.SampleRate)
		}
	}
}

// Stream implements bus.AudioSource: it proxies the STT recogniser's channel of
// partial/final transcriptions.
func (s *MicrophoneAudioSource) Stream() <-chan bus.AudioSample { return s.stt.Stream() }

// Close implements bus.AudioSource. It stops the mic pump, releases the capture
// device, and shuts down the STT recogniser (flushing its tail). Idempotent.
func (s *MicrophoneAudioSource) Close() error {
	close(s.done)
	s.wg.Wait()
	_ = s.mic.Close()
	return s.stt.Close()
}
