package screen

import (
	"image"
	"image/color"
	"testing"
)

func TestQualityScore_NoLines(t *testing.T) {
	if q := qualityScore(nil, nil); q != 0 {
		t.Fatalf("sem linhas deve score 0, got %v", q)
	}
}

func TestQualityScore_RichText(t *testing.T) {
	// Muitas linhas, texto alto, regiões cobertas → score alto.
	lines := []ocrLine{
		{Text: "linha de texto um com bastante conteúdo", Height: 20},
		{Text: "linha de texto dois com bastante conteúdo", Height: 22},
		{Text: "linha de texto três", Height: 18},
	}
	regions := []Region{{Kind: RegionText, Text: "x"}, {Kind: RegionText, Text: "y"}}
	q := qualityScore(lines, regions)
	if q < 0.6 {
		t.Fatalf("texto rico deve score alto, got %.2f", q)
	}
}

func TestQualityScore_TinyText(t *testing.T) {
	// Pouco texto e bbox minúsculo → score baixo.
	lines := []ocrLine{{Text: "ab", Height: 4}}
	regions := []Region{{Kind: RegionText}}
	q := qualityScore(lines, regions)
	if q > 0.3 {
		t.Fatalf("texto minusculo deve score baixo, got %.2f", q)
	}
}

func TestUpscaleImage_Doubles(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 10, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 10; x++ {
			src.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 0, A: 255})
		}
	}
	out := upscaleImage(src, 2)
	b := out.Bounds()
	if b.Dx() != 20 || b.Dy() != 16 {
		t.Fatalf("upscale 2x deve dobrar dims, got %dx%d", b.Dx(), b.Dy())
	}
	// Pixels devem refletir o vizinho mais próximo (preserva conteúdo).
	r, _, _, _ := out.At(0, 0).RGBA()
	if r>>8 != 0 {
		t.Fatalf("pixel original perdido? got %d", r>>8)
	}
}

func TestUpscaleImage_Factor1(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 5, 5))
	out := upscaleImage(src, 1)
	if out != src {
		t.Fatalf("factor=1 deve retornar a mesma imagem")
	}
}
