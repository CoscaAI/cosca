package screen

import (
	"context"
	"image"
	"image/jpeg"
	"os"
	"testing"
)

// TestOCRAdaptiveLive valida o pipeline adaptativo (upscale) na imagem externa
// TEST-final do Downloads — a imagem difícil do Don. Se o OCR de 1x for fraco,
// ele escala e re-lê. (Só roda quando imagem presente; marcado 'short' para
// pular em CI offline.)
func TestOCRAdaptiveLive(t *testing.T) {
	path := os.Getenv("USERPROFILE") + "/Downloads/TEST-final.jpg"
	if _, err := os.Stat(path); err != nil {
		t.Skip("TEST-final.jpg não encontrado")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}

	w := NewWinRTOCR()
	regions := []Region{{BBox: Rect{0, 0, img.Bounds().Dx(), img.Bounds().Dy()}, Kind: RegionText}}
	if err := w.Recognize(context.Background(), image.Image(img), regions); err != nil {
		t.Fatalf("Recognize: %v", err)
	}
	if regions[0].Text == "" {
		t.Errorf("pipeline adaptativo nao leu texto na TEST-final")
	}
	t.Logf("leitura final (apos upscale): %q", regions[0].Text)
}
