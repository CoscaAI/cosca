//go:build tts_sherpa

package tts

// This file is the native-Go TTS engine backed by sherpa-onnx (windows cgo).
// It is compiled ONLY behind the `tts_sherpa` build tag. It uses the low-level
// windows binding directly (github.com/k2-fsa/sherpa-onnx-go-windows) so no
// network access is needed at runtime: models are local, inference is CPU/ONNX.
//
// The disabled stub lives in tts.go //go:build !tts_sherpa and is mutually
// exclusive with this file.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	sherpa "github.com/k2-fsa/sherpa-onnx-go-windows"
)

// Enabled reports whether a real native engine is compiled in. In this build it
// is true, so the speaking loop can run the sherpa Speaker.
func Enabled() bool { return true }

// sherpaEngine is the real Engine backed by an offline sherpa TTS synthesizer.
type sherpaEngine struct {
	cfg Config
	tts *sherpa.OfflineTts
	sr  int
	ns  int
}

// New validates the config and returns a (not-yet-opened) engine. Call Open to
// load the model and create the synthesizer. In this build a real engine is
// always returned (modulo config validation); the "disabled" path lives in
// tts.go for the default (non-cgo) build.
func New(cfg Config) (Engine, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &sherpaEngine{cfg: cfg}, nil
}

// resolveFile returns f if set, else ModelDir+defaultName.
func (c Config) resolveFile(f, defaultName string) string {
	if f != "" {
		return f
	}
	if c.ModelDir == "" {
		return ""
	}
	return filepath.Join(c.ModelDir, defaultName)
}

// discoverModels resolves every referenced input file paths, deriving the
// conventional names from ModelDir when a per-file override is empty, and
// scanning for a single *.onnx when the model file name is unknown (e.g. the
// piper/edresson family, whose .onnx carries a language+voice name).
func (c Config) discoverModels() (model, vocab, lexicon, dataDir, voices, lang, acoustic string, err error) {
	voices = c.resolveFile(c.Voices, "voices.bin")
	lexicon = c.resolveFile(c.Lexicon, "lexicon.txt")
	dataDir = c.resolveFile(c.DataDir, "espeak-ng-data")
	lang = c.Lang

	switch c.resolveModelType() {
	case ModelTypeKokoro:
		model = c.resolveFile(c.Model, "model.onnx")
		vocab = c.resolveFile(c.Tokens, "tokens.txt")
		if model == "" || !fileExists(model) {
			model, err = scanOnnx(c.ModelDir)
			if err != nil {
				return "", "", "", "", "", "", "", err
			}
		}
		// Kokoro needs a voices file.
		if _, e := os.Stat(voices); e != nil {
			return "", "", "", "", "", "", "", fmt.Errorf("%w: kokoro voices: %s: %v", ErrNoModel, voices, e)
		}
	case ModelTypeVits, ModelTypeMatcha, ModelTypeZipvoice:
		model = c.resolveFile(c.Model, "model.onnx")
		vocab = c.resolveFile(c.Tokens, "tokens.txt")
		if model == "" || !fileExists(model) {
			model, err = scanOnnx(c.ModelDir)
			if err != nil {
				return "", "", "", "", "", "", "", err
			}
		}
	default:
		return "", "", "", "", "", "", "", fmt.Errorf("tts: unsupported model_type %q", c.ModelType)
	}
	return model, vocab, lexicon, dataDir, voices, lang, acoustic, nil
}

// fileExists reports whether the path exists on disk (used to fall back to a
// scan when a conventional file name is absent, e.g. a piper voice whose .onnx
// carries a language+voice name instead of a literal "model.onnx").
func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// scanOnnx returns the single *.onnx file in dir, or an error if the count is
// not exactly one (caller should then set Config.Model explicitly).
func scanOnnx(dir string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.onnx"))
	if err != nil {
		return "", fmt.Errorf("%w: glob *.onnx in %s: %v", ErrNoModel, dir, err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("%w: no .onnx model found in %s (set model path)", ErrNoModel, dir)
	}
	if len(matches) > 1 {
		sort.Strings(matches)
		return "", fmt.Errorf("%w: multiple .onnx models in %s (%v); set model path explicitly", ErrNoModel, dir, matches)
	}
	return matches[0], nil
}

// validate checks basic config sanity (not file existence, which Open checks).
func (c Config) validate() error {
	if c.Provider != "" && !strings.EqualFold(c.Provider, "sherpa") {
		return fmt.Errorf("tts: unsupported provider %q (want \"sherpa\" or \"\")", c.Provider)
	}
	switch c.resolveModelType() {
	case ModelTypeVits, ModelTypeKokoro, ModelTypeMatcha, ModelTypeZipvoice:
	default:
		return fmt.Errorf("tts: unsupported model_type %q", c.ModelType)
	}
	if c.ModelDir == "" && c.Model == "" {
		return fmt.Errorf("tts: model_dir (or model) must be set")
	}
	return nil
}

// buildConfig resolves file paths, verifies they exist, and builds the sherpa
// offline TTS config.
func (e *sherpaEngine) buildConfig() (*sherpa.OfflineTtsConfig, error) {
	model, vocab, lexicon, dataDir, voices, _, _, err := e.cfg.discoverModels()
	if err != nil {
		return nil, err
	}

	// Fail-closed: verify every referenced model input exists before loading.
	refs := []string{model, vocab, dataDir}
	for _, r := range refs {
		if r == "" {
			continue
		}
		if fi, e := os.Stat(r); e != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrNoModel, r, e)
		} else if fi.IsDir() && e == nil {
			// dataDir / espeak-ng-data is a directory — that's fine.
		}
	}

	oc := &sherpa.OfflineTtsConfig{
		MaxNumSentences: e.cfg.resolveMaxSentences(),
		SilenceScale:    float32(e.cfg.resolveSilenceScale()),
	}

	switch e.cfg.resolveModelType() {
	case ModelTypeKokoro:
		oc.Model.Kokoro = sherpa.OfflineTtsKokoroModelConfig{
			Model:       model,
			Voices:      voices,
			Tokens:      vocab,
			DataDir:     dataDir,
			Lexicon:     lexicon,
			Lang:        e.cfg.Lang,
			LengthScale: float32(e.cfg.resolveLengthScale()),
		}
	case ModelTypeMatcha:
		acoustic, vocoder := e.cfg.resolveMatcha()
		oc.Model.Matcha = sherpa.OfflineTtsMatchaModelConfig{
			AcousticModel: acoustic,
			Vocoder:       vocoder,
			Lexicon:       lexicon,
			Tokens:        vocab,
			DataDir:       dataDir,
			NoiseScale:    float32(e.cfg.resolveNoiseScaleW()),
			LengthScale:   float32(e.cfg.resolveLengthScale()),
		}
	default: // vits (and zipvoice-like handled as vits unless model_type=zipvoice)
		oc.Model.Vits = sherpa.OfflineTtsVitsModelConfig{
			Model:       model,
			Lexicon:     lexicon,
			Tokens:      vocab,
			DataDir:     dataDir,
			NoiseScale:  float32(e.cfg.resolveNoiseScaleW()),
			NoiseScaleW: float32(e.cfg.resolveNoiseScaleW()),
			LengthScale: float32(e.cfg.resolveLengthScale()),
		}
	}

	oc.Model.NumThreads = e.cfg.resolveThreads()
	oc.Model.Provider = e.cfg.resolveDevice()
	return oc, nil
}

// resolveMatcha returns the acoustic model + vocoder paths for MatchaTTS.
func (c Config) resolveMatcha() (acoustic, vocoder string) {
	acoustic = c.resolveFile(c.Acoustic, "acoustic.onnx")
	vocoder = c.resolveFile(c.Vocoder, "vocoder.onnx")
	if acoustic == "" {
		acoustic = c.resolveFile(c.Model, "model.onnx")
	}
	return acoustic, vocoder
}

// Open loads the model and creates the synthesizer. Idempotent.
func (e *sherpaEngine) Open() error {
	if e.tts != nil {
		return nil
	}
	oc, err := e.buildConfig()
	if err != nil {
		return err
	}
	t := sherpa.NewOfflineTts(oc)
	if t == nil {
		return fmt.Errorf("%w: sherpa tts creation failed (bad/incomplete model?)", ErrNoModel)
	}
	e.tts = t
	e.sr = t.SampleRate()
	e.ns = t.NumSpeakers()
	return nil
}

// Close releases the synthesizer. Idempotent.
func (e *sherpaEngine) Close() error {
	if e.tts != nil {
		sherpa.DeleteOfflineTts(e.tts)
		e.tts = nil
	}
	return nil
}

// Synthesize converts text into PCM at the model's sample rate. sid selects the
// speaker, speed the rate (1.0 = normal; >1 slower, <1 faster).
func (e *sherpaEngine) Synthesize(text string, sid int, speed float64) (*SynthesizeResult, error) {
	if err := e.Open(); err != nil {
		return nil, err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return &SynthesizeResult{SampleRate: e.sr, Duration: 0}, nil
	}
	if sid < 0 {
		sid = 0
	}
	if speed <= 0 {
		speed = e.cfg.resolveSpeed()
	}
	audio := e.tts.Generate(text, sid, float32(speed))
	if audio == nil {
		return nil, fmt.Errorf("%w: sherpa tts Generate returned nil", ErrNoModel)
	}
	return &SynthesizeResult{
		Samples:    audio.Samples,
		SampleRate: audio.SampleRate,
		Duration:   time.Duration(float64(len(audio.Samples)) / float64(audio.SampleRate) * float64(time.Second)),
	}, nil
}

// SampleRate returns the engine's output sample rate (Hz). Opens lazily.
func (e *sherpaEngine) SampleRate() int {
	_ = e.Open()
	return e.sr
}

// NumSpeakers returns the number of available voices. Opens lazily.
func (e *sherpaEngine) NumSpeakers() int {
	_ = e.Open()
	return e.ns
}
