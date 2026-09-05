//go:build stt_sherpa

package stt

// This file is the native-Go STT engine backed by sherpa-onnx (windows cgo).
// It is compiled ONLY behind the `stt_sherpa` build tag. It uses the low-level
// windows binding directly (github.com/k2-fsa/sherpa-onnx-go-windows) so the
// wrapper module (which pulls linux/macos deps) is not required and no network
// access is needed at runtime: models are local, inference is CPU/ONNX.
//
// SO, there is no `//go:build !stt_sherpa` conflict here: the disabled stub
// lives in stt.go and is mutually exclusive with this file.

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	sherpa "github.com/k2-fsa/sherpa-onnx-go-windows"
)

// sandbox note: sherpa_onnx (the binding) is the declared package name; we alias
// it as `sherpa` for readability. All calls are local (CPU, ONNX).

// Enabled reports whether a real native engine is compiled in. In this build it
// is true, so the perception bus can wire the sherpa AudioSource.
func Enabled() bool { return true }

// engineType is satisfied by *sherpaEngine (see types.go Engine interface).

// sherpaEngine is the real Engine backed by an online sherpa recognizer.
type sherpaEngine struct {
	cfg Config
	rec *sherpa.OnlineRecognizer
}

// New validates the config and returns a (not-yet-opened) engine. Call Open to
// load the model and create the recognizer. In this build a real engine is
// always returned (modulo config validation); the "disabled" path lives in
// stt.go for the default (non-cgo) build.
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

// paths returns the resolved model file paths for the configured model type.
func (c Config) paths() (encoder, decoder, joiner, ctc, tokens string) {
	switch c.resolveModelType() {
	case ModelTypeParaformer:
		return c.resolveFile(c.Encoder, "encoder.onnx"),
			c.resolveFile(c.Decoder, "decoder.onnx"), "", "", c.resolveFile(c.Tokens, "tokens.txt")
	case ModelTypeZipformer2Ctc, ModelTypeNemoCtc:
		name := "ctc.onnx"
		if c.resolveModelType() == ModelTypeNemoCtc {
			name = "model.onnx"
		}
		return "", "", "", c.resolveFile(c.CtcModel, name), c.resolveFile(c.Tokens, "tokens.txt")
	default: // transducer
		return c.resolveFile(c.Encoder, "encoder.onnx"),
			c.resolveFile(c.Decoder, "decoder.onnx"),
			c.resolveFile(c.Joiner, "joiner.onnx"), "", c.resolveFile(c.Tokens, "tokens.txt")
	}
}

// validate checks basic config sanity (not file existence, which Open checks).
func (c Config) validate() error {
	switch c.resolveModelType() {
	case ModelTypeTransducer, ModelTypeParaformer, ModelTypeZipformer2Ctc, ModelTypeNemoCtc:
	default:
		return fmt.Errorf("stt: unsupported model_type %q", c.ModelType)
	}
	if c.resolveSampleRate() <= 0 {
		return fmt.Errorf("stt: sample_rate must be > 0")
	}
	return nil
}

// buildConfig resolves file paths, verifies they exist, and builds the sherpa
// recognizer config.
func (e *sherpaEngine) buildConfig() (*sherpa.OnlineRecognizerConfig, error) {
	enc, dec, join, ctc, tokens := e.cfg.paths()

	model := sherpa.OnlineModelConfig{
		Tokens:     tokens,
		NumThreads: e.cfg.resolveThreads(),
		Provider:   e.cfg.resolveProvider(),
	}

	switch e.cfg.resolveModelType() {
	case ModelTypeParaformer:
		if enc == "" || dec == "" {
			return nil, fmt.Errorf("stt: paraformer requires encoder+decoder paths")
		}
		model.Paraformer = sherpa.OnlineParaformerModelConfig{Encoder: enc, Decoder: dec}
	case ModelTypeZipformer2Ctc:
		if ctc == "" {
			return nil, fmt.Errorf("stt: zipformer2_ctc requires a ctc model path")
		}
		model.Zipformer2Ctc = sherpa.OnlineZipformer2CtcModelConfig{Model: ctc}
	case ModelTypeNemoCtc:
		if ctc == "" {
			return nil, fmt.Errorf("stt: nemo_ctc requires a ctc model path")
		}
		model.NemoCtc = sherpa.OnlineNemoCtcModelConfig{Model: ctc}
	default: // transducer
		if enc == "" || dec == "" || join == "" {
			return nil, fmt.Errorf("stt: transducer requires encoder+decoder+joiner paths")
		}
		model.Transducer = sherpa.OnlineTransducerModelConfig{Encoder: enc, Decoder: dec, Joiner: join}
	}

	// Verify every referenced .onnx exists (fail-closed before loading).
	refs := []string{tokens}
	switch e.cfg.resolveModelType() {
	case ModelTypeParaformer:
		refs = append(refs, enc, dec)
	case ModelTypeZipformer2Ctc, ModelTypeNemoCtc:
		refs = append(refs, ctc)
	default:
		refs = append(refs, enc, dec, join)
	}
	for _, r := range refs {
		if r == "" {
			continue
		}
		if _, err := os.Stat(r); err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrNoModel, r, err)
		}
	}

	endpoint := 0
	if e.cfg.EnableEndpoint {
		endpoint = 1
	}

	return &sherpa.OnlineRecognizerConfig{
		FeatConfig: sherpa.FeatureConfig{
			SampleRate: e.cfg.resolveSampleRate(),
			FeatureDim: 80,
		},
		ModelConfig:     model,
		DecodingMethod:  e.cfg.resolveDecodingMethod(),
		EnableEndpoint:  endpoint,
		MaxActivePaths:  4,
	}, nil
}

// Open loads the model and creates the recognizer. Idempotent.
func (e *sherpaEngine) Open() error {
	if e.rec != nil {
		return nil
	}
	rc, err := e.buildConfig()
	if err != nil {
		return err
	}
	r := sherpa.NewOnlineRecognizer(rc)
	if r == nil {
		return fmt.Errorf("%w: sherpa recognizer creation failed (bad model?)", ErrNoModel)
	}
	e.rec = r
	return nil
}

// Close releases the recognizer. Idempotent.
func (e *sherpaEngine) Close() error {
	if e.rec != nil {
		sherpa.DeleteOnlineRecognizer(e.rec)
		e.rec = nil
	}
	return nil
}

// NewStream returns a fresh online decode session.
func (e *sherpaEngine) NewStream() (Stream, error) {
	if err := e.Open(); err != nil {
		return nil, err
	}
	s := sherpa.NewOnlineStream(e.rec)
	if s == nil {
		return nil, fmt.Errorf("%w: sherpa stream creation failed", ErrNoModel)
	}
	return &sherpaStream{rec: e.rec, s: s, cfg: e.cfg}, nil
}

// sherpaStream is the Stream backed by one sherpa OnlineStream.
type sherpaStream struct {
	rec    *sherpa.OnlineRecognizer
	s      *sherpa.OnlineStream
	cfg    Config
	closed bool
}

// AcceptWaveform feeds normalized PCM samples ([-1,1]) at sampleRate.
func (st *sherpaStream) AcceptWaveform(samples []float32, sampleRate int) error {
	if st.closed || st.s == nil {
		return fmt.Errorf("stt: stream closed")
	}
	if len(samples) == 0 {
		return nil
	}
	// sherpa resamples internally if sampleRate != the model's feature rate.
	st.s.AcceptWaveform(sampleRate, samples)
	return nil
}

// InputFinished signals no more audio for this stream (flushes tail).
func (st *sherpaStream) InputFinished() error {
	if st.s != nil {
		st.s.InputFinished()
	}
	return nil
}

// Ready reports whether enough frames are buffered to decode.
func (st *sherpaStream) Ready() bool {
	return st.s != nil && st.rec != nil && st.rec.IsReady(st.s)
}

// Decode advances the recognizer one step. Call in `for Ready()`.
func (st *sherpaStream) Decode() error {
	if st.s == nil {
		return fmt.Errorf("stt: stream closed")
	}
	st.rec.Decode(st.s)
	return nil
}

// Result returns the current transcript (partial until finalised).
func (st *sherpaStream) Result() (*Result, error) {
	if st.s == nil {
		return nil, fmt.Errorf("stt: stream closed")
	}
	r := st.rec.GetResult(st.s)
	if r == nil {
		return &Result{}, nil
	}
	return copyResult(r), nil
}

// IsEndpoint reports whether an endpoint (silence/utterance end) was hit.
func (st *sherpaStream) IsEndpoint() bool {
	return st.s != nil && st.rec != nil && st.rec.IsEndpoint(st.s)
}

// Reset clears the stream state to begin a new segment.
func (st *sherpaStream) Reset() error {
	if st.s != nil {
		st.rec.Reset(st.s)
	}
	return nil
}

// Close releases the stream. Idempotent.
func (st *sherpaStream) Close() error {
	if !st.closed && st.s != nil {
		sherpa.DeleteOnlineStream(st.s)
		st.s = nil
		st.closed = true
	}
	return nil
}

// copyResult converts a sherpa result into our Result, mapping per-token
// timestamps (seconds) to time.Duration offsets.
func copyResult(r *sherpa.OnlineRecognizerResult) *Result {
	out := &Result{Text: r.Text}
	n := len(r.Tokens)
	if n == 0 {
		return out
	}
	out.Tokens = make([]Token, n)
	for i := 0; i < n; i++ {
		off := float64(0)
		if i < len(r.Timestamps) {
			off = float64(r.Timestamps[i])
		}
		out.Tokens[i] = Token{
			Word:   r.Tokens[i],
			Offset: time.Duration(off * float64(time.Second)),
			Conf:   1.0, // sherpa exposes no per-token confidence
		}
	}
	return out
}
