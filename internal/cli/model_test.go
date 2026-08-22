package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/aitask"
	"github.com/CoscaAI/cosca/internal/modelreg"
)

// TestModelRegistry_AddListInfo exercita o fluxo via Registry core.
func TestModelRegistry_AddListInfo(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".cosca"), 0o755); err != nil {
		t.Fatal(err)
	}

	r, err := modelreg.Open(root)
	if err != nil {
		t.Fatal(err)
	}

	// Registra whisper (STT) e rembg (segmentation).
	w, err := modelreg.New("whisper", "openai", "large-v3", modelreg.FormatSafetensors, modelreg.QFP16, 8<<30, modelreg.KindLocal, "MIT", []aitask.Type{aitask.SpeechToText})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Register(w); err != nil {
		t.Fatal(err)
	}
	rm, err := modelreg.New("rembg", "local", "2.0.78", modelreg.FormatONNX, modelreg.QFP16, 2<<30, modelreg.KindLocal, "MIT", []aitask.Type{aitask.Segmentation})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Register(rm); err != nil {
		t.Fatal(err)
	}

	// Busca por task.
	stt := r.FindForTask(aitask.SpeechToText)
	if len(stt) != 1 || stt[0].ID != "whisper" {
		t.Fatalf("FindForTask(STT) = %+v", stt)
	}

	// Resolve por prefixo de chave.
	key, err := resolveModelKey(r, "openai/whisper")
	if err != nil {
		t.Fatalf("resolveModelKey prefix: %v", err)
	}
	if key != "openai/whisper/large-v3" {
		t.Fatalf("resolved = %q", key)
	}

	// Remove e confirma.
	if err := r.Remove("local/rembg/2.0.78"); err != nil {
		t.Fatal(err)
	}
	if r.Count() != 1 {
		t.Fatalf("Count = %d, want 1", r.Count())
	}
}
