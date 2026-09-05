// Package tts implements native-Go text-to-speech (FASE C) for the COSCA world
// model — closing the loop "percebe → interpreta → fala". It is the speaking
// counterpart of the FASE B STT package (internal/worldmodel/audio/stt).
//
// The engine is backed by sherpa-onnx (non-streaming offline TTS: VITS / Kokoro
// / Matcha / etc.) via its windows cgo binding, and is compiled ONLY behind the
// `tts_sherpa` build tag. Without that tag the package compiles to a benign
// "disabled" stub so pure-Go builds (and CI without a C/mingw toolchain) stay
// green.
//
// Sovereignty: models are loaded from a local directory the caller provides
// (Config.ModelDir / explicit file paths). Nothing is fetched at runtime.
package tts

import (
	"errors"
	"time"
)

// Sentinel errors. They are NOT fatal: a missing model or a disabled engine
// degrades gracefully instead of crashing the "speaking" loop.
var (
	// ErrDisabled reports that the native TTS engine was not compiled in
	// (the binary was built without `-tags tts_sherpa`).
	ErrDisabled = errors.New("tts: native text-to-speech not compiled (build with -tags tts_sherpa)")
	// ErrNoModel reports that the model could not be created / loaded.
	ErrNoModel = errors.New("tts: model not loaded")
)

// ModelType selects the offline TTS architecture.
type ModelType string

const (
	// ModelTypeVits is a VITS / piper model (encoder+decoder in a single .onnx).
	ModelTypeVits ModelType = "vits"
	// ModelTypeKokoro is a Kokoro model (model.onnx + voices.bin).
	ModelTypeKokoro ModelType = "kokoro"
	// ModelTypeMatcha is a MatchaTTS model (acoustic model + vocoder).
	ModelTypeMatcha ModelType = "matcha"
	// ModelTypeZipvoice is a ZipVoice model (flow-matching decoder).
	ModelTypeZipvoice ModelType = "zipvoice"
)

// Config configures the TTS synthesizer.
type Config struct {
	// Provider selects the engine: "sherpa" (native Go) or "" (disabled).
	Provider string `json:"provider" yaml:"provider"`
	// ModelType selects the architecture. Default ModelTypeVits.
	ModelType ModelType `json:"model_type" yaml:"model_type"`
	// ModelDir is the directory holding the model + tokens + espeak-ng-data.
	// Sovereignty: a local path, never a network fetch.
	ModelDir string `json:"model_dir" yaml:"model_dir"`
	// NumThreads is the ONNX runtime worker count. Default 2.
	NumThreads int `json:"num_threads" yaml:"num_threads"`
	// Device is the execution provider: "cpu" (default), "cuda", "coreml".
	Device string `json:"device" yaml:"device"`
	// Speed is the speaking rate (1.0 = normal; <1 faster, >1 slower).
	Speed float64 `json:"speed" yaml:"speed"`
	// Sid is the speaker/voice id. Default 0.
	Sid int `json:"sid" yaml:"sid"`

	// Per-model file overrides (absolute or relative to ModelDir). When empty
	// the engine derives conventional names from ModelDir + ModelType.
	Model    string `json:"model,omitempty" yaml:"model,omitempty"`
	Voices   string `json:"voices,omitempty" yaml:"voices,omitempty"`     // kokoro voices.bin
	Lexicon  string `json:"lexicon,omitempty" yaml:"lexicon,omitempty"`   // lexicon.txt (optional)
	Tokens   string `json:"tokens,omitempty" yaml:"tokens,omitempty"`     // tokens.txt
	DataDir  string `json:"data_dir,omitempty" yaml:"data_dir,omitempty"` // espeak-ng-data dir
	Acoustic string `json:"acoustic,omitempty" yaml:"acoustic,omitempty"` // matcha acoustic model
	Vocoder  string `json:"vocoder,omitempty" yaml:"vocoder,omitempty"`   // matcha vocoder
	Lang     string `json:"lang,omitempty" yaml:"lang,omitempty"`         // kokoro language hint

	// Synthesis parameters (needle values; sensible sherpa defaults applied).
	SilenceScale    float64 `json:"silence_scale,omitempty" yaml:"silence_scale,omitempty"`
	NoiseScaleW     float64 `json:"noise_scale_w,omitempty" yaml:"noise_scale_w,omitempty"`
	LengthScale     float64 `json:"length_scale,omitempty" yaml:"length_scale,omitempty"`
	MaxNumSentences int     `json:"max_num_sentences,omitempty" yaml:"max_num_sentences,omitempty"`
}

// SynthesizeResult is the result of one synthesis call.
type SynthesizeResult struct {
	// Samples is the generated mono PCM in [-1, 1].
	Samples []float32 `json:"-"`
	// SampleRate is the output sample rate (Hz), e.g. 16000.
	SampleRate int `json:"sample_rate"`
	// Duration is the expected playback duration.
	Duration time.Duration `json:"duration"`
}

// ── resolve helpers (defaults applied when unset) ──────────────────────────────

// resolveModelType returns cfg.ModelType or ModelTypeVits.
func (c Config) resolveModelType() ModelType {
	if c.ModelType == "" {
		return ModelTypeVits
	}
	return c.ModelType
}

// resolveThreads returns cfg.NumThreads or the default (2).
func (c Config) resolveThreads() int {
	if c.NumThreads <= 0 {
		return 2
	}
	return c.NumThreads
}

// resolveDevice returns cfg.Device or "cpu".
func (c Config) resolveDevice() string {
	if c.Device == "" {
		return "cpu"
	}
	return c.Device
}

// resolveSpeed returns cfg.Speed or the default (1.0).
func (c Config) resolveSpeed() float64 {
	if c.Speed <= 0 {
		return 1.0
	}
	return c.Speed
}

// resolveSilenceScale returns cfg.SilenceScale or the default (0.9).
func (c Config) resolveSilenceScale() float64 {
	if c.SilenceScale <= 0 {
		return 0.9
	}
	return c.SilenceScale
}

// resolveNoiseScaleW returns cfg.NoiseScaleW or the default (0.8).
func (c Config) resolveNoiseScaleW() float64 {
	if c.NoiseScaleW <= 0 {
		return 0.8
	}
	return c.NoiseScaleW
}

// resolveLengthScale returns cfg.LengthScale or the default (1.0).
func (c Config) resolveLengthScale() float64 {
	if c.LengthScale <= 0 {
		return 1.0
	}
	return c.LengthScale
}

// resolveMaxSentences returns cfg.MaxNumSentences or the default (1).
func (c Config) resolveMaxSentences() int {
	if c.MaxNumSentences <= 0 {
		return 1
	}
	return c.MaxNumSentences
}

// Engine is the TTS lifecycle (open → synthesize → close).
type Engine interface {
	// Open loads the model and creates the synthesizer. Idempotent.
	Open() error
	// Close releases the synthesizer. Idempotent.
	Close() error
	// Synthesize converts text into PCM at the model's sample rate. sid selects
	// the speaker and speed the rate (1.0 = normal; >1 slower, <1 faster).
	Synthesize(text string, sid int, speed float64) (*SynthesizeResult, error)
	// SampleRate returns the engine's output sample rate (Hz).
	SampleRate() int
	// NumSpeakers returns the number of available voices.
	NumSpeakers() int
}
