package screen

import (
	"image"

	"golang.org/x/image/draw"
)

// qualityScore avalia a "suficiência" de uma leitura de OCR, combinando:
//   - quantidade de texto (total de caracteres lidos),
//   - tamanho médio dos bboxes (texto pequeno = menos confiável),
//   - cobertura das regiões de texto (quantas regiões ganharam texto).
//
// Retorna um score em [0,1]: quanto maior, mais confiável a leitura. É a métrica
// de decisão do pipeline adaptativo: o sensor barato tenta primeiro e só escala
// (upscale) quando a evidência é insuficiente — a filosofia da casa.
func qualityScore(lines []ocrLine, regions []Region) float64 {
	if len(lines) == 0 {
		return 0
	}
	// 1. Quantidade de texto: soma dos caracteres, normalizada por um teto.
	var totalChars int
	for _, l := range lines {
		totalChars += len(l.Text)
	}
	textScore := clamp01(float64(totalChars) / 200.0) // 200+ chars = texto rico

	// 2. Tamanho médio dos bboxes: texto com altura de caractere confortável
	// (>= 14px) é mais confiável que texto minúsculo.
	avgHeight := 0.0
	for _, l := range lines {
		avgHeight += float64(l.Height)
	}
	if len(lines) > 0 {
		avgHeight /= float64(len(lines))
	}
	sizeScore := clamp01(avgHeight / 14.0)

	// 3. Cobertura: quantas regiões de texto foram preenchidas.
	covered := 0
	for _, r := range regions {
		if r.Text != "" {
			covered++
		}
	}
	coverScore := 0.0
	if len(regions) > 0 {
		coverScore = clamp01(float64(covered) / float64(len(regions)))
	}

	// Pesos: texto rico pesa mais, tamanho e cobertura complementam.
	return clamp01(0.5*textScore + 0.3*sizeScore + 0.2*coverScore)
}

// upscaleImage redimensiona a imagem por um fator inteiro (2x, 4x) usando
// interpolação BICUBIC (Catmull-Rom). Ao contrário do nearest-neighbor (que
// apenas duplica pixels e pode deixar bordas serrilhadas), o bicubic suaviza e
// realça as transições — o que melhora significativamente a leitura de texto
// pequeno/mínusculo pelo OCR. É o "zoom" de qualidade da percepção de texto.
func upscaleImage(src image.Image, factor int) image.Image {
	if factor <= 1 {
		return src
	}
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()*factor, b.Dy()*factor))
	// CatmullRom interpola (bicubic); põe o src no canto superior esquerdo.
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

