package screen

import (
	"image"
	"image/color"
	"testing"
)

// solidImage gera uma imagem RGBA sólida de uma cor.
func solidImage(w, h int, c color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestComputeAesthetic_SolidColor(t *testing.T) {
	// Uma imagem 100% de uma cor sólida deve ter contraste ~0 e simetria alta
	// (tudo igual → correlação perfeita entre esquerda e direita).
	img := solidImage(64, 64, color.RGBA{R: 120, G: 120, B: 120, A: 255})
	a := computeAesthetic(img)

	if a.Contrast > 0.05 {
		t.Fatalf("imagem sólida deve ter contraste ~0, got %.3f", a.Contrast)
	}
	if a.Saturation > 0.05 {
		t.Fatalf("imagem cinza deve ter saturação ~0, got %.3f", a.Saturation)
	}
	if a.Symmetry < 0.8 {
		t.Fatalf("imagem sólida deve ter simetria ~1, got %.3f", a.Symmetry)
	}
	if a.Brightness < 0.40 || a.Brightness > 0.60 {
		// 120/255 ≈ 0.47 (luminância perceptiva de um cinza médio).
		t.Fatalf("brightness inesperado: %.3f", a.Brightness)
	}
}

func TestComputeAesthetic_Asymmetric(t *testing.T) {
	// Metade esquerda branca, direita preta → o contraste alto e simetria baixa.
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			if x < 32 {
				img.Set(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255}) // branco
			} else {
				img.Set(x, y, color.RGBA{R: 0, G: 0, B: 0, A: 255}) // preto
			}
		}
	}
	a := computeAesthetic(img)

	if a.Contrast < 0.4 {
		t.Fatalf("esquerda/direita deve ter contraste alto, got %.3f", a.Contrast)
	}
	if a.Symmetry > 0.3 {
		t.Fatalf("imagem assimétrica deve ter simetria baixa, got %.3f", a.Symmetry)
	}
}

func TestClamp01(t *testing.T) {
	cases := []struct{ in, want float64 }{
		{-1, 0}, {0, 0}, {0.5, 0.5}, {1, 1}, {2, 1},
	}
	for _, c := range cases {
		if got := clamp01(c.in); got != c.want {
			t.Fatalf("clamp01(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestRgbToHue(t *testing.T) {
	// Vermelho puro → 0°, Verde → 120°, Azul → 240°.
	if h := rgbToHue(255, 0, 0); h != 0 {
		t.Fatalf("vermelho hue = %v, want 0", h)
	}
	if h := rgbToHue(0, 255, 0); h != 120 {
		t.Fatalf("verde hue = %v, want 120", h)
	}
	if h := rgbToHue(0, 0, 255); h != 240 {
		t.Fatalf("azul hue = %v, want 240", h)
	}
	// Cinza → 0 (saturação zero, hue irrelevante).
	if h := rgbToHue(128, 128, 128); h != 0 {
		t.Fatalf("cinza hue = %v, want 0", h)
	}
}
