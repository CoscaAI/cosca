//go:build !stt_sherpa

package cli

import (
	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/perception/bus"
)

// buildSttAudioSource returns a nil source in the default (non-cgo) build: the
// native-Go sherpa STT engine is not compiled in. Callers fall back to
// NoopAudioSource, so Fase A behaviour is bit-for-bit unchanged (and CI without
// a C/mingw toolchain stays green).
func buildSttAudioSource(_ config.PerceptionConfig, _ zerolog.Logger) (bus.AudioSource, error) {
	return nil, nil
}
