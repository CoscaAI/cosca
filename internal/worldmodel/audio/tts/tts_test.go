package tts

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNewDisabled verifies the default (non-cgo) build degrades to ErrDisabled:
// New always succeeds (never panics) and the engine reports disabled.
func TestNewDisabled(t *testing.T) {
	if Enabled() {
		t.Skip("native TTS compiled in — this test covers the disabled stub")
	}
	eng, err := New(Config{Provider: "sherpa", ModelDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New() = %v; disabled build must not error", err)
	}
	if err := eng.Open(); err != ErrDisabled {
		t.Fatalf("Open() = %v, want ErrDisabled", err)
	}
	if _, err := eng.Synthesize("olá", 0, 1.0); err != ErrDisabled {
		t.Fatalf("Synthesize() = %v, want ErrDisabled", err)
	}
	if eng.SampleRate() != 0 || eng.NumSpeakers() != 0 {
		t.Fatalf("disabled engine should report 0/0, got sr=%d ns=%d", eng.SampleRate(), eng.NumSpeakers())
	}
	if err := eng.Close(); err != nil {
		t.Fatalf("Close() = %v, want nil", err)
	}
}

// TestConfigValidation covers the config sanity checks. This runs only in the
// native build: the disabled stub's New never validates (it returns a benign
// disabled engine, mirroring the STT package), so the validation logic lives in
// the sherpa engine and is covered when compiled in.
func TestConfigValidation(t *testing.T) {
	if !Enabled() {
		t.Skip("native TTS not compiled — disabled stub does not validate config")
	}
	// Missing model_dir → error.
	if _, err := New(Config{Provider: "sherpa", ModelType: "vits"}); err == nil {
		t.Fatalf("New() with empty model_dir should error, got nil")
	}
	// Unknown provider → error.
	if _, err := New(Config{Provider: "crazy", ModelDir: t.TempDir()}); err == nil {
		t.Fatalf("New() with unknown provider should error, got nil")
	}
}

// TestWriteWAV16File verifies the pure-Go WAV writer produces a valid RIFF/WAVE
// header and the expected payload byte-length for 16-bit mono PCM.
func TestWriteWAV16File(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.wav")
	samples := []float32{0, 0.5, -0.5, 1, -1}
	if err := WriteWAV16File(path, samples, 16000); err != nil {
		t.Fatalf("WriteWAV16File() = %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() = %v", err)
	}
	if len(b) != 44+len(samples)*2 {
		t.Fatalf("wav size = %d, want %d", len(b), 44+len(samples)*2)
	}
	if string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		t.Fatalf("invalid RIFF/WAVE magic: %s/%s", b[0:4], b[8:12])
	}
}

// TestSynthesizeReal is the native smoke test. It only runs under the
// `tts_sherpa` build tag AND when a real model dir is present (see the COSCA
// model install). Without the model it skips so CI stays green.
func TestSynthesizeReal(t *testing.T) {
	if !Enabled() {
		t.Skip("native TTS not compiled (build with -tags tts_sherpa)")
	}
	modelDir := findAnyModel()
	if modelDir == "" {
		t.Skip("no local TTS model dir found — skipping native synthesis")
	}
	eng, err := New(Config{Provider: "sherpa", ModelType: ModelTypeVits, ModelDir: modelDir})
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	defer eng.Close()
	res, err := eng.Synthesize("Olá, eu sou o COSCA.", 0, 1.0)
	if err != nil {
		t.Fatalf("Synthesize() = %v", err)
	}
	if res.SampleRate <= 0 {
		t.Fatalf("SampleRate = %d, want > 0", res.SampleRate)
	}
	if len(res.Samples) == 0 {
		t.Fatalf("Synthesize returned 0 samples")
	}
	t.Logf("synthesized %d samples @ %d Hz (~%s)", len(res.Samples), res.SampleRate, res.Duration)
}

// findAnyModel looks for a conventional vits/piper/kokoro model directory the
// caller may have installed, scanning a few known locations. Empty if none.
func findAnyModel() string {
	candidates := []string{
		filepath.Join(os.Getenv("USERPROFILE"), ".cosca", "models", "tts"),
		filepath.Join(os.Getenv("USERPROFILE"), ".cosca", "models", "tts", "vits-piper-pt_BR-edresson-low"),
		`C:\Users\Henrique\AppData\Local\Temp\opencode\tts-spike-models\vits-piper-pt_BR-edresson-low`,
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if fi, err := os.Stat(filepath.Join(c, "tokens.txt")); err == nil && !fi.IsDir() {
			return c
		}
	}
	return ""
}
