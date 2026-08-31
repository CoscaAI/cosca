//go:build !tts_sherpa

package tts

// This file is the pure-Go build (no cgo): the native sherpa-onnx TTS engine
// is NOT compiled in. Every Engine method reports ErrDisabled so callers
// degrade gracefully ("TTS desabilitado") instead of crashing.
//
// The real implementation lives in tts_sherpa.go (//go:build tts_sherpa).

// disabledEngine implements Engine as a no-op that reports ErrDisabled.
type disabledEngine struct{}

// New returns a disabled engine (native TTS not compiled). Callers should check
// ErrDisabled and fall back to a silent/no-op "speaker".
func New(_ Config) (Engine, error) {
	return &disabledEngine{}, nil
}

// Enabled reports whether a real native engine is compiled in. In this build it
// is false, so the speaking loop degrades to a no-op (never emits audio).
func Enabled() bool { return false }

func (d *disabledEngine) Open() error  { return ErrDisabled }
func (d *disabledEngine) Close() error { return nil }

func (d *disabledEngine) Synthesize(string, int, float64) (*SynthesizeResult, error) {
	return nil, ErrDisabled
}

func (d *disabledEngine) SampleRate() int  { return 0 }
func (d *disabledEngine) NumSpeakers() int { return 0 }
