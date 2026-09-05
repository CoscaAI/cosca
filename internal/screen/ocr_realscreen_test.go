package screen

import (
	"context"
	"testing"
)

// TestOCRRealScreen captura a TELA REAL (caso mais difícil: terminal/ANSI/texto
// pequeno) e valida que o OCR consegue ler, com a sanitização de transporte.
func TestOCRRealScreen(t *testing.T) {
	img, err := capture(context.Background(), 0)
	if err != nil {
		t.Fatalf("captura da tela: %v", err)
	}
	w := NewWinRTOCR()
	regions := []Region{{BBox: Rect{0, 0, img.Bounds().Dx(), img.Bounds().Dy()}, Kind: RegionText}}
	if err := w.Recognize(context.Background(), img, regions); err != nil {
		t.Fatalf("Recognize na tela real: %v", err)
	}
	t.Logf("texto lido: %q", regions[0].Text)
	if regions[0].Text == "" {
		t.Errorf("OCR nao leu texto na tela real")
	}
}
