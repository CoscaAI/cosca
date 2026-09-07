//go:build stt_sherpa

package stt

// This file implements the OFFLINE (non-streaming) sherpa recognizer path.
//
// The native streaming engines (OnlineRecognizer) require streaming models
// (conformer/zipformer with `window_size` in the metadata). Some target models
// for Brazilian-Portuguese ASR — e.g. sherpa-onnx-nemo-stt_pt_fastconformer_hybrid_large_pc
// — are OFFLINE (non-streaming) models WITHOUT streaming metadata. For those,
// sherpa exposes the OfflineRecognizer: you feed the full waveform, call Decode,
// and read the whole-segment result. This file wires that path behind the same
// Engine/Stream interfaces so the Perception Bus and `voice` pipeline work
// unchanged for offline PT models.
//
// All calls are local (CPU, ONNX). No network. No Python.

import (
	"fmt"
	"os"
	"time"

	sherpa "github.com/k2-fsa/sherpa-onnx-go-windows"
)

// offlineEngine is the Engine backed by a sherpa OfflineRecognizer. It is used
// when the configured model type is a single-file offline CTC/Nemo model.
type offlineEngine struct {
	cfg Config
	rec *sherpa.OfflineRecognizer
}

// newOfflineEngine validates config and returns the offline engine (not opened).
func newOfflineEngine(cfg Config) (Engine, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &offlineEngine{cfg: cfg}, nil
}

// buildOfflineConfig resolves the single offline model path (the .onnx) and
// the tokens, verifies they exist (fail-closed), and builds the offline config.
func (e *offlineEngine) buildOfflineConfig() (*sherpa.OfflineRecognizerConfig, error) {
	_, _, _, _, _ = e.cfg.paths() // validate model-type sanity via paths()/resolveModelType
	ctc, tokens := e.cfg.resolveOfflineModel()
	if ctc == "" {
		return nil, fmt.Errorf("stt: offline model requires a ctc model path")
	}

	if _, err := os.Stat(ctc); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrNoModel, ctc, err)
	}
	if tokens != "" {
		if _, err := os.Stat(tokens); err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrNoModel, tokens, err)
		}
	}

	model := sherpa.OfflineModelConfig{
		NemoCTC: sherpa.OfflineNemoEncDecCtcModelConfig{
			Model: ctc,
		},
		Tokens:     tokens,
		NumThreads: e.cfg.resolveThreads(),
		Provider:   e.cfg.resolveProvider(),
	}

	return &sherpa.OfflineRecognizerConfig{
		FeatConfig: sherpa.FeatureConfig{
			SampleRate: e.cfg.resolveSampleRate(),
			FeatureDim: 80,
		},
		ModelConfig:     model,
		DecodingMethod:  e.cfg.resolveDecodingMethod(),
		MaxActivePaths:  4,
	}, nil
}

// Open loads the offline model and creates the recognizer. Idempotent.
func (e *offlineEngine) Open() error {
	if e.rec != nil {
		return nil
	}
	rc, err := e.buildOfflineConfig()
	if err != nil {
		return err
	}
	r := sherpa.NewOfflineRecognizer(rc)
	if r == nil {
		return fmt.Errorf("%w: sherpa offline recognizer creation failed (bad model?)", ErrNoModel)
	}
	e.rec = r
	return nil
}

// Close releases the recognizer. Idempotent.
func (e *offlineEngine) Close() error {
	if e.rec != nil {
		sherpa.DeleteOfflineRecognizer(e.rec)
		e.rec = nil
	}
	return nil
}

// NewStream returns a fresh offline decode session (one whole utterance).
func (e *offlineEngine) NewStream() (Stream, error) {
	if err := e.Open(); err != nil {
		return nil, err
	}
	s := sherpa.NewOfflineStream(e.rec)
	if s == nil {
		return nil, fmt.Errorf("%w: sherpa offline stream creation failed", ErrNoModel)
	}
	return &offlineStream{rec: e.rec, s: s, cfg: e.cfg}, nil
}

// offlineStream is the Stream backed by one sherpa OfflineStream. For offline
// models you feed the WHOLE waveform, call InputFinished(), then Decode() once,
// and read the full-segment Result(). Ready() is always false (nothing partial)
// and IsEndpoint() always false (no streaming endpoint).
type offlineStream struct {
	rec    *sherpa.OfflineRecognizer
	s      *sherpa.OfflineStream
	cfg    Config
	closed bool
}

// AcceptWaveform feeds normalized PCM samples ([-1,1]) at sampleRate.
func (st *offlineStream) AcceptWaveform(samples []float32, sampleRate int) error {
	if st.closed || st.s == nil {
		return fmt.Errorf("stt: stream closed")
	}
	if len(samples) == 0 {
		return nil
	}
	st.s.AcceptWaveform(sampleRate, samples)
	return nil
}

// InputFinished signals no more audio. For offline models this is a no-op in the
// C API (not exposed) — the recognizer finalises on Decode().
func (st *offlineStream) InputFinished() error {
	return nil
}

// Ready reports false: an offline recognizer has no partial/streaming readiness.
func (st *offlineStream) Ready() bool {
	return false
}

// Decode runs the offline recognizer over the accumulated waveform once.
func (st *offlineStream) Decode() error {
	if st.s == nil {
		return fmt.Errorf("stt: stream closed")
	}
	st.rec.Decode(st.s)
	return nil
}

// Result returns the whole-segment transcript.
func (st *offlineStream) Result() (*Result, error) {
	if st.s == nil {
		return nil, fmt.Errorf("stt: stream closed")
	}
	r := st.s.GetResult()
	if r == nil {
		return &Result{}, nil
	}
	return copyOfflineResult(r), nil
}

// IsEndpoint reports false (no streaming endpoint in offline mode).
func (st *offlineStream) IsEndpoint() bool {
	return false
}

// Reset clears the offline stream (no-op for a one-shot offline session).
func (st *offlineStream) Reset() error {
	return nil
}

// Close releases the stream. Idempotent.
func (st *offlineStream) Close() error {
	if !st.closed && st.s != nil {
		sherpa.DeleteOfflineStream(st.s)
		st.s = nil
		st.closed = true
	}
	return nil
}

// copyOfflineResult maps a sherpa OfflineRecognizerResult into our Result,
// per-token timestamps (seconds) to time.Duration offsets.
func copyOfflineResult(r *sherpa.OfflineRecognizerResult) *Result {
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

// resolveOfflineModel returns the (ctc, tokens) paths for the offline engine.
// For nemo_ctc the offline model is a single .onnx + tokens.
func (c Config) resolveOfflineModel() (ctc, tokens string) {
	name := "model.onnx"
	if c.resolveModelType() == ModelTypeNemoCtc {
		name = "model.onnx"
	}
	return c.resolveFile(c.CtcModel, name), c.resolveFile(c.Tokens, "tokens.txt")
}
