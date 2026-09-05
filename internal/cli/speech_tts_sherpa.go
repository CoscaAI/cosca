//go:build tts_sherpa

package cli

// This file compiles only with `-tags tts_sherpa` (the native-Go sherpa TTS
// engine). It wires a real tts.Speaker when `perception.audio.tts.provider ==
// "sherpa"`. Without the tag, speech_tts_noop.go supplies a nil speaker so the
// default build stays cgo-free.

import (
	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/worldmodel/audio/tts"
)

// buildTtsSpeaker constructs a sherpa-backed Speaker when opted-in
// (tts.provider == "sherpa"); otherwise returns nil (caller falls back to a
// silent no-op). A missing engine path degrades gracefully (never crashes).
func buildTtsSpeaker(cfg config.PerceptionConfig, logger zerolog.Logger) (*tts.Speaker, error) {
	ttsCfg := cfg.Audio.TTS
	if ttsCfg.Provider != "sherpa" {
		logger.Debug().Str("component", "speech/tts").Msg("tts.provider != sherpa — speaker disabled")
		return nil, nil
	}

	ec := tts.Config{
		Provider:   "sherpa",
		ModelType:  tts.ModelType(ttsCfg.ModelType),
		ModelDir:   ttsCfg.ModelDir,
		NumThreads: ttsCfg.NumThreads,
		Device:     ttsCfg.Device,
		Speed:      ttsCfg.Speed,
		Sid:        ttsCfg.Sid,
	}

	eng, err := tts.New(ec)
	if err != nil {
		return nil, err
	}
	spk := tts.NewSpeaker(eng, ec, logger.With().Str("component", "speech/tts").Logger())

	// Best-effort open so misconfigured models fail fast (but never crash):
	// a broken model logs a warning; callers still get ErrDisabled-style
	// graceful degradation from the synthesis call itself.
	if err := spk.Open(); err != nil {
		logger.Warn().Err(err).
			Str("model_dir", ttsCfg.ModelDir).
			Str("model_type", ttsCfg.ModelType).
			Msg("tts: speaker opened with warning — synthesis may fail until a valid model is present")
	}

	logger.Info().
		Str("provider", "sherpa").
		Str("model_dir", ttsCfg.ModelDir).
		Str("model_type", ttsCfg.ModelType).
		Int("sample_rate", ttsCfg.SampleRate).
		Float64("speed", ttsCfg.Speed).
		Int("sid", ttsCfg.Sid).
		Msg("speech: native-Go TTS (sherpa) speaker wired — the COSCA can speak in Go")
	return spk, nil
}
