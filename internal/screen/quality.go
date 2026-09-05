package screen

import "image"

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

// upscaleImage redimensiona a imagem por um fator inteiro (2x, 4x). Preserva o
// conteúdo — só aumenta a representação espacial dos pixels existentes, não
// cria detalhe novo. Usa interpolação por vizinho mais próximo (bom o bastante
// para OCR).
func upscaleImage(src image.Image, factor int) image.Image {
	if factor <= 1 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx()*factor, b.Dy()*factor
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		sy := b.Min.Y + y/factor
		for x := 0; x < w; x++ {
			sx := b.Min.X + x/factor
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}
