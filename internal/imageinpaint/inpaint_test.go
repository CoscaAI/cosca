package imageinpaint

import (
	"image"
	"image/color"
	"testing"
)

func TestInpaint_PreencheRegiao(t *testing.T) {
	// Fonte: borda RED, mas a região central será preenchida pela vizinhança.
	src := image.NewRGBA(image.Rect(0, 0, 20, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			c := color.RGBA{255, 0, 0, 255} // vermelho ao redor
			src.SetRGBA(x, y, c)
		}
	}
	// Máscara: retângulo central (x 8-11, y 3-6) = preencher.
	mask := image.NewRGBA(image.Rect(0, 0, 20, 10))
	for y := 3; y <= 6; y++ {
		for x := 8; x <= 11; x++ {
			mask.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}

	dst := Inpaint(src, mask)
	// A região mascarada deve ter sido preenchida com a cor da vizinhança
	// (vermelho) — ou seja, NÃO permanecer como cor original genérica.
	got := dst.RGBAAt(9, 4)
	if got.R < 200 || got.G > 50 {
		t.Fatalf("regiao preenchida esperada vermelha (vizinhanca), got %+v", got)
	}
	// Pixel fora da máscara não pode ter sido alterado.
	out := dst.RGBAAt(1, 1)
	if out != (color.RGBA{255, 0, 0, 255}) {
		t.Fatalf("pixel fora da mascara alterado: %+v", out)
	}
}

func TestInpaint_SemMascara(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 8, 8))
	mask := image.NewRGBA(image.Rect(0, 0, 8, 8)) // toda 0 = nada a preencher
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			src.SetRGBA(x, y, color.RGBA{10, 20, 30, 255})
		}
	}
	dst := Inpaint(src, mask)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if c := dst.RGBAAt(x, y); c != (color.RGBA{10, 20, 30, 255}) {
				t.Fatalf("sem mascara nao deve mudar: (%d,%d)=%+v", x, y, c)
			}
		}
	}
}

func TestInpaint_RegiaoGrandeCentro(t *testing.T) {
	// Garante que não trava/estoura com região maior e é determinístico.
	src := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			src.SetRGBA(x, y, color.RGBA{0, 200, 0, 255})
		}
	}
	mask := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for y := 10; y < 30; y++ {
		for x := 10; x < 30; x++ {
			mask.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	// Inpaint com limite de iterações (não trava).
	dst := Inpaint(src, mask)
	if c := dst.RGBAAt(20, 20); c.R > 50 || c.G < 150 {
		t.Fatalf("centro deveria ser verde (vizinhanca): %+v", c)
	}
}
