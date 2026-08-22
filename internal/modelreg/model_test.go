package modelreg

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/aitask"
)

func TestNewValidates(t *testing.T) {
	cases := []struct {
		name    string
		build   func() (*Model, error)
		wantErr bool
	}{
		{"ok", func() (*Model, error) {
			return New("whisper", "openai", "large-v3", FormatSafetensors, QFP16, 8<<30, KindLocal, "MIT", []aitask.Type{aitask.SpeechToText})
		}, false},
		{"empty id", func() (*Model, error) {
			return New("", "openai", "v1", FormatONNX, QNone, 0, KindRemote, "", []aitask.Type{aitask.TextGeneration})
		}, true},
		{"bad format", func() (*Model, error) {
			return New("m", "p", "v", Format("exe"), QNone, 0, KindLocal, "", []aitask.Type{aitask.TextGeneration})
		}, true},
		{"bad quant", func() (*Model, error) {
			return New("m", "p", "v", FormatONNX, Quantization("int3"), 0, KindLocal, "", []aitask.Type{aitask.TextGeneration})
		}, true},
		{"no caps", func() (*Model, error) {
			return New("m", "p", "v", FormatONNX, QNone, 0, KindLocal, "", nil)
		}, true},
		{"bad cap", func() (*Model, error) {
			return New("m", "p", "v", FormatONNX, QNone, 0, KindLocal, "", []aitask.Type{aitask.Type("nope")})
		}, true},
		{"bad kind", func() (*Model, error) {
			return New("m", "p", "v", FormatONNX, QNone, 0, Kind("both"), "", []aitask.Type{aitask.TextGeneration})
		}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := c.build()
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
		})
	}
}

func TestKeyAndSupports(t *testing.T) {
	m, err := New("whisper", "openai", "large-v3", FormatSafetensors, QFP16, 8<<30, KindLocal, "MIT", []aitask.Type{aitask.SpeechToText})
	if err != nil {
		t.Fatal(err)
	}
	if m.Key() != "openai/whisper/large-v3" {
		t.Fatalf("Key() = %q", m.Key())
	}
	if !m.Supports(aitask.SpeechToText) {
		t.Fatal("should support speech_to_text")
	}
	if m.Supports(aitask.ImageGeneration) {
		t.Fatal("should NOT support image_generation")
	}
}

func TestRegisterAndGet(t *testing.T) {
	r, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m, _ := New("whisper", "openai", "large-v3", FormatSafetensors, QFP16, 8<<30, KindLocal, "MIT", []aitask.Type{aitask.SpeechToText})
	if err := r.Register(m); err != nil {
		t.Fatal(err)
	}

	got, ok := r.Get("openai/whisper/large-v3")
	if !ok {
		t.Fatal("Get should find the model")
	}
	if got.License != "MIT" {
		t.Fatalf("license = %q, want MIT", got.License)
	}
	if r.Count() != 1 {
		t.Fatalf("Count = %d, want 1", r.Count())
	}
}

func TestFindForTask(t *testing.T) {
	r, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	w, _ := New("whisper", "openai", "large-v3", FormatSafetensors, QFP16, 8<<30, KindLocal, "MIT", []aitask.Type{aitask.SpeechToText})
	tess, _ := New("tesseract", "local", "5.3", FormatUnknown, QNone, 0, KindLocal, "Apache-2.0", []aitask.Type{aitask.OCR})
	if err := r.Register(w); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(tess); err != nil {
		t.Fatal(err)
	}

	stt := r.FindForTask(aitask.SpeechToText)
	if len(stt) != 1 || stt[0].ID != "whisper" {
		t.Fatalf("FindForTask(STT) = %+v", stt)
	}
	ocr := r.FindForTask(aitask.OCR)
	if len(ocr) != 1 || ocr[0].ID != "tesseract" {
		t.Fatalf("FindForTask(OCR) = %+v", ocr)
	}
	if got := r.FindForTask(aitask.Simulation); len(got) != 0 {
		t.Fatalf("FindForTask(Simulation) should be empty, got %+v", got)
	}
}

func TestPersistence(t *testing.T) {
	root := t.TempDir()
	r, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := New("rembg", "local", "2.0.78", FormatONNX, QFP16, 2<<30, KindLocal, "MIT", []aitask.Type{aitask.Segmentation})
	if err := r.Register(m); err != nil {
		t.Fatal(err)
	}

	r2, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Count() != 1 {
		t.Fatalf("after reopen Count = %d, want 1", r2.Count())
	}
	got, ok := r2.Get("local/rembg/2.0.78")
	if !ok {
		t.Fatal("reopened registry should have the model")
	}
	if !got.Supports(aitask.Segmentation) {
		t.Fatal("rembg should support segmentation")
	}
}

func TestRemove(t *testing.T) {
	r, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m, _ := New("m", "p", "v", FormatONNX, QNone, 0, KindRemote, "", []aitask.Type{aitask.TextGeneration})
	if err := r.Register(m); err != nil {
		t.Fatal(err)
	}
	if err := r.Remove(m.Key()); err != nil {
		t.Fatal(err)
	}
	if r.Count() != 0 {
		t.Fatalf("Count = %d, want 0", r.Count())
	}
	if err := r.Remove(m.Key()); err == nil {
		t.Fatal("expected error removing missing model")
	}
}

func TestGetByID(t *testing.T) {
	r, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a, _ := New("whisper", "openai", "large-v3", FormatSafetensors, QFP16, 8<<30, KindLocal, "MIT", []aitask.Type{aitask.SpeechToText})
	b, _ := New("whisper", "local", "small", FormatGGUF, QInt8, 2<<30, KindLocal, "MIT", []aitask.Type{aitask.SpeechToText})
	_ = r.Register(a)
	_ = r.Register(b)

	got := r.GetByID("whisper")
	if len(got) != 2 {
		t.Fatalf("GetByID(whisper) = %d, want 2", len(got))
	}
}

func TestFormatQuantValid(t *testing.T) {
	for _, f := range []Format{FormatSafetensors, FormatGGUF, FormatONNX, FormatTorch, FormatTensorRT, FormatUnknown} {
		if !f.Valid() {
			t.Fatalf("format %q should be valid", f)
		}
	}
	if Format("exe").Valid() {
		t.Fatal("exe format should be invalid")
	}
	for _, q := range []Quantization{QFP16, QFP32, QInt8, QInt4, QNone} {
		if !q.Valid() {
			t.Fatalf("quant %q should be valid", q)
		}
	}
	if Quantization("int3").Valid() {
		t.Fatal("int3 quant should be invalid")
	}
}
