//go:build !tts_sherpa

package cli

import (
	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/worldmodel/audio/tts"
)

// buildTtsSpeaker returns a nil speaker in the default (non-cgo) build: the
// native-Go sherpa TTS engine is not compiled in. Callers degrade gracefully
// (the "falar" loop becomes a silent no-op), so Fase A behaviour is unchanged
// and CI without a C/mingw toolchain stays green.
func buildTtsSpeaker(_ config.PerceptionConfig, logger zerolog.Logger) (*tts.Speaker, error) {
	logger.Debug().Str("component", "speech/tts").Msg("native-Go TTS not compiled (build with -tags tts_sherpa) — speaker disabled")
	return nil, nil
}
