//go:build !stt_sherpa

package cli

import (
	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/perception/bus"
)

// buildMicAudioSource returns a nil source in the default (non-cgo) build: the
// native-Go sherpa STT engine is not compiled in, so there is nothing to wrap
// with a microphone. Callers fall back to the STT-only / NoopAudioSource path,
// leaving Fase A behaviour bit-for-bit unchanged (and CI without a C/mingw
// toolchain stays green).
func buildMicAudioSource(_ config.PerceptionConfig, _ bus.AudioSource, _ zerolog.Logger) (bus.AudioSource, error) {
	return nil, nil
}
