// Package stt implements native-Go speech-to-text (FASE B) for the COSCA world
// model — local, ONNX-based, no internet at runtime.
//
// The engine is backed by sherpa-onnx (streaming transducer/Paraformer/CTC)
// via its windows cgo binding, and is compiled ONLY behind the `stt_sherpa`
// build tag. Without that tag the package compiles to a benign "disabled"
// stub so pure-Go builds (and CI without a C/mingw toolchain) stay green.
//
// Sovereignty: models are loaded from a local directory the caller provides
// (Config.ModelDir / explicit file paths). Nothing is fetched at runtime.
package stt

import (
	"errors"
	"time"
)

// Sentinel errors. They are NOT fatal to the perception pipeline: a missing
// model or a disabled engine degrades gracefully instead of crashing.
var (
	// ErrDisabled reports that the native STT engine was not compiled in
	// (the binary was built without `-tags stt_sherpa`).
	ErrDisabled = errors.New("stt: native speech-to-text not compiled (build with -tags stt_sherpa)")
	// ErrNoModel reports that the model could not be created / loaded.
	ErrNoModel = errors.New("stt: model not loaded")
)

// DefaultMaxSegmentDuration is the maximum STT segment length when Config does
// not set one. This is the memory watchdog cap: a streaming recognizer that
// never hits an endpoint must be force-finalised and reset within this window
// so it cannot accumulate features (and memory) without bound.
const DefaultMaxSegmentDuration = 30 * time.Second

// ModelType selects the streaming recognizer architecture.
type ModelType string

const (
	// ModelTypeTransducer is a streaming transducer (encoder+decoder+joiner).
	ModelTypeTransducer ModelType = "transducer"
	// ModelTypeParaformer is a streaming Paraformer (encoder+decoder).
	ModelTypeParaformer ModelType = "paraformer"
	// ModelTypeZipformer2Ctc is a single-file streaming zipformer2 CTC model.
	ModelTypeZipformer2Ctc ModelType = "zipformer2_ctc"
	// ModelTypeNemoCtc is a single-file streaming NeMo CTC model.
	ModelTypeNemoCtc ModelType = "nemo_ctc"
)

// Config configures the STT recognizer.
type Config struct {
	// ModelDir is the directory holding the .onnx model file(s) and tokens.
	// Used only to derive default filenames when the per-file fields below
	// are empty. Sovereignty: a local path, never a network fetch.
	ModelDir string `json:"model_dir" yaml:"model_dir"`
	// SampleRate is the audio sample rate fed to the engine (Hz). Default 16000.
	SampleRate int `json:"sample_rate" yaml:"sample_rate"`
	// NumThreads is the ONNX runtime worker count. Default 2.
	NumThreads int `json:"num_threads" yaml:"num_threads"`
	// Provider is the execution provider: "cpu" (default), "cuda", "coreml".
	Provider string `json:"provider" yaml:"provider"`
	// DecodingMethod is "greedy_search" (default) or "modified_beam_search".
	DecodingMethod string `json:"decoding_method" yaml:"decoding_method"`
	// EnableEndpoint enables sherpa's streaming endpoint detector, which lets
	// the bus finalize a segment on trailing silence. Default true.
	EnableEndpoint bool `json:"enable_endpoint" yaml:"enable_endpoint"`
	// MaxSegmentDuration is the maximum length of a single STT segment. It is a
	// safety watchdog: sherpa's streaming recognizer accumulates the input
	// features it has accepted since the last Reset(); if an endpoint is never
	// hit (continuous speech, silence over noise, a frozen mic feeding constant
	// audio), the internal buffer grows without bound and can exhaust memory.
	// When a segment exceeds this, the source force-finalises it (emits the
	// final transcript) and resets the stream, bounding the memory per segment.
	// 0/negative disables the watchdog (NOT recommended — keep a finite cap).
	MaxSegmentDuration time.Duration `json:"max_segment_duration,omitempty" yaml:"max_segment_duration,omitempty"`
	// ModelType selects the architecture. Default ModelTypeTransducer.
	ModelType ModelType `json:"model_type" yaml:"model_type"`

	// Per-model file overrides (absolute or relative to ModelDir). When empty
	// the engine derives conventional names from ModelDir.
	Encoder  string `json:"encoder,omitempty" yaml:"encoder,omitempty"`
	Decoder  string `json:"decoder,omitempty" yaml:"decoder,omitempty"`
	Joiner   string `json:"joiner,omitempty" yaml:"joiner,omitempty"`
	CtcModel string `json:"ctc_model,omitempty" yaml:"ctc_model,omitempty"`
	Tokens   string `json:"tokens,omitempty" yaml:"tokens,omitempty"`

	// VAD optionally enables in-engine voice activity detection (Silero VAD).
	// When non-nil, audio is gated by VAD before recognition (Fase B optional).
	VAD *VADConfig `json:"vad,omitempty" yaml:"vad,omitempty"`
}

// VADConfig configures Silero VAD gating. Optional.
type VADConfig struct {
	// Enabled turns VAD gating on.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Model is the Silero VAD .onnx path (optional; empty → sherpa default).
	Model string `json:"model,omitempty" yaml:"model,omitempty"`
	// Threshold is the speech probability threshold in [0,1]. Default 0.5.
	Threshold float32 `json:"threshold" yaml:"threshold"`
	// BufferSeconds is the VAD buffer length in seconds. Default 0.3.
	BufferSeconds float32 `json:"buffer_seconds" yaml:"buffer_seconds"`
}

// Token is one recognised word with its timestamp relative to the segment start.
type Token struct {
	// Word is the recognised word.
	Word string `json:"word"`
	// Offset is the offset of the word from the segment start (seconds).
	Offset time.Duration `json:"offset"`
	// Conf is the recogniser confidence for this word in [0,1]. sherpa does not
	// emit per-token confidence, so this is 1.0 (best-effort) unless set by VAD.
	Conf float64 `json:"conf"`
}

// Result is a recognition result for a stream.
type Result struct {
	// Text is the full transcript (partial or final).
	Text string `json:"text"`
	// Tokens are the word-level timings.
	Tokens []Token `json:"tokens"`
}

// Engine is the STT recognizer lifecycle (open → streams → close).
type Engine interface {
	// Open loads the model and creates the recognizer. Idempotent.
	Open() error
	// Close releases the recognizer. Idempotent.
	Close() error
	// NewStream returns a fresh online decode session.
	NewStream() (Stream, error)
}

// Stream is one online decode session. Feed normalized PCM [-1,1], then decode
// in a `for Ready() { Decode() }` loop, calling Result() for the partial/final
// transcript and Reset() at an endpoint to start a new segment.
type Stream interface {
	// AcceptWaveform feeds normalized PCM samples ([-1,1]) at sampleRate.
	// sherpa resamples internally if sampleRate differs from the model's.
	AcceptWaveform(samples []float32, sampleRate int) error
	// InputFinished signals no more audio for this stream (flushes the tail).
	InputFinished() error
	// Ready reports whether enough frames are buffered to decode.
	Ready() bool
	// Decode advances the recognizer one step. Call in `for Ready()`.
	Decode() error
	// Result returns the current transcript (partial until finalised).
	Result() (*Result, error)
	// IsEndpoint reports whether an endpoint (silence/utterance end) was hit.
	IsEndpoint() bool
	// Reset clears the stream state to begin a new segment. Returns the last
	// segment's text if the caller wants it.
	Reset() error
	// Close releases the stream. Idempotent.
	Close() error
}

// resolveMaxSegmentDuration returns cfg.MaxSegmentDuration or the default
// (DefaultMaxSegmentDuration). The watchdog is ALWAYS on: a 0/negative value
// falls back to the default cap so no build can silently run unbounded.
func (c Config) resolveMaxSegmentDuration() time.Duration {
	if c.MaxSegmentDuration <= 0 {
		return DefaultMaxSegmentDuration
	}
	return c.MaxSegmentDuration
}

// resolveSampleRate returns cfg.SampleRate or the default (16000).
func (c Config) resolveSampleRate() int {
	if c.SampleRate <= 0 {
		return 16000
	}
	return c.SampleRate
}

// resolveThreads returns cfg.NumThreads or the default (2).
func (c Config) resolveThreads() int {
	if c.NumThreads <= 0 {
		return 2
	}
	return c.NumThreads
}

// resolveProvider returns cfg.Provider or "cpu".
func (c Config) resolveProvider() string {
	if c.Provider == "" {
		return "cpu"
	}
	return c.Provider
}

// resolveDecodingMethod returns cfg.DecodingMethod or "greedy_search".
func (c Config) resolveDecodingMethod() string {
	if c.DecodingMethod == "" {
		return "greedy_search"
	}
	return c.DecodingMethod
}

// resolveModelType returns cfg.ModelType or ModelTypeTransducer.
func (c Config) resolveModelType() ModelType {
	if c.ModelType == "" {
		return ModelTypeTransducer
	}
	return c.ModelType
}
