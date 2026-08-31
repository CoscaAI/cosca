//go:build stt_sherpa

package cli

// This file compiles only with `-tags stt_sherpa` (native-Go sherpa STT). When
// `perception.audio.mic.enabled` is true, it wraps the sherpa STT AudioSource
// with a live microphone capture (native Windows winmm/WaveIn), closing the
// loop: mic → PCM → PushPCM → STT streaming → bus.AudioSample → perception bus.
//
// Without the tag, mic_noop.go supplies the nil twin so the default build stays
// cgo-free.

import (
	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/perception/bus"
	"github.com/CoscaAI/cosca/internal/worldmodel/audio/mic"
	"github.com/CoscaAI/cosca/internal/worldmodel/audio/stt"
)

// buildMicAudioSource wraps a sherpa STT AudioSource with live microphone
// capture when opt-in (mic.enabled). It returns nil when the mic is not enabled
// or there is no sherpa source to wrap, so the caller falls back to the
// STT-only path. A capture error is propagated so the caller can degrade
// gracefully (keep the STT-only source) instead of crashing.
func buildMicAudioSource(cfg config.PerceptionConfig, sttSrc bus.AudioSource, logger zerolog.Logger) (bus.AudioSource, error) {
	if !cfg.Audio.Mic.Enabled {
		return nil, nil
	}
	if sttSrc == nil {
		return nil, nil
	}
	sttSource, ok := sttSrc.(*stt.AudioSource)
	if !ok {
		logger.Warn().Msg("perception bus: mic enabled but STT source is not a sherpa AudioSource — ignoring mic")
		return nil, nil
	}

	mc, err := mic.NewCapture(mic.Config{
		Device:     cfg.Audio.Mic.Device,
		SampleRate: cfg.Audio.Mic.SampleRate,
		Channels:   cfg.Audio.Mic.Channels,
		ChunkMS:    cfg.Audio.Mic.ChunkMS,
	})
	if err != nil {
		return nil, err
	}

	wrapped := mic.NewMicrophoneAudioSource(mc, sttSource,
		logger.With().Str("component", "perception-bus/mic").Logger())
	logger.Info().
		Str("backend", "winmm").
		Int("device", cfg.Audio.Mic.Device).
		Int("sample_rate", cfg.Audio.Mic.SampleRate).
		Msg("perception bus: microphone capture wired (mic → PCM → STT → bus)")
	return wrapped, nil
}
