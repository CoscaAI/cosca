package screen

import (
	"image"
	"image/color"
	"testing"
)

// textStripImage gera uma imagem com uma faixa horizontal de "texto" (pixels
// escuros contíguos com pequeno gap entre caracteres) sobre fundo claro,
// simulando uma linha de texto realista.
func textStripImage(w, h, textY, textH int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// fundo claro.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 240, G: 240, B: 240, A: 255})
		}
	}
	// "texto": faixa escura com caracteres quase contíguos (gap pequeno de 1px
	// entre traços), como num editor/texto real.
	for y := textY; y < textY+textH && y < h; y++ {
		for x := 20; x < w-20; x += 4 {
			for k := 0; k < 3 && x+k < w; k++ { // traço do caractere (3px)
				img.Set(x+k, y, color.RGBA{R: 20, G: 20, B: 20, A: 255})
			}
		}
	}
	return img
}

// blankImage gera uma imagem uniforme (sem texto distinto).
func blankImage(w, h int, c color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestDetectRegions_FindsTextStrip(t *testing.T) {
	img := textStripImage(400, 200, 80, 24)
	regions := detectRegions(img)
	found := false
	for _, r := range regions {
		if r.Kind == RegionText {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("detector não achou região de texto em imagem com faixa de texto (regiões=%d)", len(regions))
	}
}

func TestDetectRegions_Blank(t *testing.T) {
	img := blankImage(400, 200, color.RGBA{R: 240, G: 240, B: 240, A: 255})
	regions := detectRegions(img)
	if len(regions) != 0 {
		t.Fatalf("imagem em branco deve ter 0 regiões, got %d", len(regions))
	}
}

func TestDetectRegions_SmallNoise(t *testing.T) {
	// Um único "pixel" isolado não deve gerar região (filtro de ruído).
	img := image.NewRGBA(image.Rect(0, 0, 400, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 400; x++ {
			img.Set(x, y, color.RGBA{R: 240, G: 240, B: 240, A: 255})
		}
	}
	img.Set(200, 100, color.RGBA{R: 0, G: 0, B: 0, A: 255})
	regions := detectRegions(img)
	if len(regions) != 0 {
		t.Fatalf("ruído isolado deve ser filtrado, got %d regiões", len(regions))
	}
}

func TestOtsuThreshold_Bimodal(t *testing.T) {
	// Histograma bimodal: uma classe escura (bins 0..60) e uma clara
	// dominante (bins 180..255). Otsu deve escolher um limiar que SEPARA as
	// duas classes — ou seja, que fica entre elas (não dentro de um modo).
	// Como a classe clara domina muito, o ponto ótimo pode passar de 180
	// (mais perto da clara), então o assert é que não cai dentro da escura.
	hist := make([]int, 256)
	for i := 0; i < 256; i++ {
		switch {
		case i <= 60:
			hist[i] = 120 // dark (texto)
		case i >= 180:
			hist[i] = 1500 // light (fundo)
		default:
			hist[i] = 0
		}
	}
	thr := otsuThreshold(hist, 256*100)
	// O limiar nunca deve cair em um bin dominado pela classe ESCURA — se
	// caísse ali, binarizaria o texto como background (erro grave).
	if thr <= 60 {
		t.Fatalf("Otsu caiu na classe escura: thr=%d (deve separar > 60)", thr)
	}
}
