//go:build !stt_sherpa

package stt

// This file is the pure-Go build (no cgo): the native sherpa-onnx STT engine
// is NOT compiled in. Every Engine/Stream method reports ErrDisabled so callers
// degrade gracefully ("STT desabilitado") instead of crashing.
//
// The real implementation lives in stt_sherpa.go (//go:build stt_sherpa).

// disabledEngine implements Engine as a no-op that reports ErrDisabled.
type disabledEngine struct{ cfg Config }

// disabledStream implements Stream as a no-op that reports ErrDisabled.
type disabledStream struct{}

// New returns a disabled engine (native STT not compiled). Callers should check
// ErrDisabled and fall back to a no-op audio source / cosca-voice.
func New(_ Config) (Engine, error) {
	return &disabledEngine{}, nil
}

// Enabled reports whether a real native engine is compiled in. In this build it
// is false, so the perception bus falls back to NoopAudioSource.
func Enabled() bool { return false }

func (d *disabledEngine) Open() error              { return ErrDisabled }
func (d *disabledEngine) Close() error             { return nil }
func (d *disabledEngine) NewStream() (Stream, error) { return nil, ErrDisabled }

func (d *disabledStream) AcceptWaveform([]float32, int) error { return ErrDisabled }
func (d *disabledStream) InputFinished() error                { return ErrDisabled }
func (d *disabledStream) Ready() bool                         { return false }
func (d *disabledStream) Decode() error                       { return ErrDisabled }
func (d *disabledStream) Result() (*Result, error)            { return nil, ErrDisabled }
func (d *disabledStream) IsEndpoint() bool                    { return false }
func (d *disabledStream) Reset() error                        { return ErrDisabled }
func (d *disabledStream) Close() error                        { return nil }
