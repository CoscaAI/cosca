//go:build stt_sherpa

package cli

// This file compiles only with `-tags stt_sherpa` (the native-Go sherpa STT
// engine). It wires a real stt.AudioSource into the perception bus when
// `perception.audio.stt.provider == "sherpa"`. Without the tag, perception_stt_noop.go
// supplies a nil source so the default build stays cgo-free.

import (
	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/perception/bus"
	"github.com/CoscaAI/cosca/internal/worldmodel/audio/stt"
)

// buildSttAudioSource constructs a sherpa-backed perception bus AudioSource when
// opt-in (stt.provider == "sherpa"); otherwise returns nil (caller falls back to
// NoopAudioSource). A nil engine/source path degrades gracefully (never crashes).
func buildSttAudioSource(cfg config.PerceptionConfig, logger zerolog.Logger) (bus.AudioSource, error) {
	if cfg.Audio.STT.Provider != "sherpa" {
		return nil, nil
	}

	sc := stt.Config{
		ModelDir:        cfg.Audio.STT.ModelDir,
		SampleRate:      cfg.Audio.STT.SampleRate,
		NumThreads:      cfg.Audio.STT.NumThreads,
		Provider:        cfg.Audio.STT.Device,
		DecodingMethod:  cfg.Audio.STT.DecodingMethod,
		EnableEndpoint:  cfg.Audio.STT.EnableEndpoint,
		ModelType:       stt.ModelType(cfg.Audio.STT.ModelType),
		Encoder:         cfg.Audio.STT.Encoder,
		Decoder:         cfg.Audio.STT.Decoder,
		Joiner:          cfg.Audio.STT.Joiner,
		CtcModel:        cfg.Audio.STT.CtcModel,
		Tokens:          cfg.Audio.STT.Tokens,
	}

	engine, err := stt.New(sc)
	if err != nil {
		return nil, err
	}
	src, err := stt.NewAudioSource(engine, sc, cfg.Audio.STT.SampleRate,
		logger.With().Str("component", "perception-bus/stt").Logger())
	if err != nil {
		return nil, err
	}
	logger.Info().
		Str("provider", "sherpa").
		Str("model_dir", cfg.Audio.STT.ModelDir).
		Int("sample_rate", cfg.Audio.STT.SampleRate).
		Msg("perception bus: native-Go STT (sherpa) audio source wired")
	return src, nil
}
