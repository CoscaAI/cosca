package screen

import (
	"image"
	"math"
)

// computeAesthetic calcula, de forma determinística (sem modelo), as métricas
// de composição/beleza de uma imagem. Não é "opinião" — é estatística visual
// objetiva (luminância, saturação, contraste, harmonia de cor, simetria e
// complexidade). O Kernel cruza isso com o embedding CLIP para o julgamento.
func computeAesthetic(img image.Image) Aesthetic {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return Aesthetic{}
	}

	// Passada 1: coleta de RGB por pixel (amostrado para performance em telas
	// grandes). Mantemos média e desvio de luminância + saturação.
	const sample = 4 // analisar 1 a cada 4 px em x e y (1/16 dos pixels)
	var (
		sumLum, sumLum2, sumSat float64
		n                       int
	)
	var (
		sumHueCos, sumHueSin float64 // para harmonia de cor (dispersão angular)
	)
	// Para simetria: acumula luminância média por coluna (esq vs dir).
	colLum := make([]float64, w/sample+1)
	colCount := make([]int, w/sample+1)

	for y := b.Min.Y; y < b.Max.Y; y += sample {
		for x := b.Min.X; x < b.Max.X; x += sample {
			r, g, bz, _ := img.At(x, y).RGBA()
			rf, gf, bf := float64(r>>8), float64(g>>8), float64(bz>>8)

			// Luminância perceptiva (rec.601).
			lum := 0.299*rf + 0.587*gf + 0.114*bf
			sumLum += lum
			sumLum2 += lum * lum

			// Saturação (1 - min/ max, com guarda de divisão).
			maxc := math.Max(rf, math.Max(gf, bf))
			minc := math.Min(rf, math.Min(gf, bf))
			if maxc > 0 {
				sumSat += (maxc - minc) / 255.0 / (maxc / 255.0)
			}

			// Matiz (para harmonia): converte RGB → matiz (0..360); acumula em
			// vetores para medir dispersão.
			hue := rgbToHue(rf, gf, bf)
			rad := hue * math.Pi / 180
			sumHueCos += math.Cos(rad)
			sumHueSin += math.Sin(rad)

			// Simetria: acumula média de luminância por coluna.
			ci := (x - b.Min.X) / sample
			if ci < len(colLum) {
				colLum[ci] += lum
				colCount[ci]++
			}

			n++
		}
	}
	if n == 0 {
		return Aesthetic{}
	}

	meanLum := sumLum / float64(n)
	variance := sumLum2/float64(n) - meanLum*meanLum
	if variance < 0 {
		variance = 0
	}
	contrast := math.Sqrt(variance) / 255.0 // normalizado 0..1

	// Harmonia de cor: magnitude do vetor soma dos matizes. Se os matizes
	// estão espalhados, o vetor médio é pequeno (desarmonia); se coesos, é
	// grande (harmonia). Normaliza por n mas o teto é a dispersão.
	colHue := math.Hypot(sumHueCos, sumHueSin) / float64(n)
	harmony := clamp01(colHue)

	// Simetria esquerda/direita: compara a série de luminância média por coluna.
	symmetry := computeSymmetry(colLum, colCount)

	// Complexidade: variedade aproximada — desvio-padrão das luminâncias por
	// coluna (quanto mais variação espacial, mais "denso"/complexo).
	complexity := clamp01(contrast*0.7 + harmony*0.3)

	return Aesthetic{
		Brightness:   clamp01(meanLum / 255.0),
		Saturation:   clamp01(sumSat / float64(n)),
		Contrast:     clamp01(contrast),
		ColorHarmony: harmony,
		Symmetry:     symmetry,
		Complexity:   complexity,
	}
}

// clamp01 limita um valor a [0,1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// rgbToHue converte RGB (0..255) para matiz em graus (0..360). Cinza (max==min)
// devolve 0, que não contribui de forma errada por ter saturação 0.
func rgbToHue(r, g, b float64) float64 {
	maxc := math.Max(r, math.Max(g, b))
	minc := math.Min(r, math.Min(g, b))
	d := maxc - minc
	if d == 0 {
		return 0
	}
	var hue float64
	switch maxc {
	case r:
		hue = 60 * (math.Mod((g-b)/d, 6))
	case g:
		hue = 60 * (((b-r)/d) + 2)
	default: // b
		hue = 60 * (((r-g)/d) + 4)
	}
	if hue < 0 {
		hue += 360
	}
	return hue
}

// computeSymmetry compara a média de luminância das metades esquerda e direita
// da imagem. Quanto mais próximas (considerando a ordem das colunas), maior a
// simetria (0..1). Usa apenas colunas com amostras e ignora o bucket residual
// (borda) para não distorcer o julgamento.
func computeSymmetry(colLum []float64, colCount []int) float64 {
	// Constrói a série normalizada apenas com buckets preenchidos.
	var norm []float64
	for i := range colLum {
		if colCount[i] > 0 {
			norm = append(norm, colLum[i]/float64(colCount[i]))
		}
	}
	if len(norm) < 2 {
		return 0
	}
	// A série real pode ter um número ímpar de colunas (borda). Usamos apenas
	// o maior número par para dividir exatamente ao meio.
	n := len(norm)
	if n%2 != 0 {
		n--
	}
	if n < 2 {
		return 0
	}
	half := n / 2
	left := norm[:half]
	right := norm[n-half:]
	// Simetria: correlação entre a metade esquerda e a metade direita
	// espelhada + quão próximas estão as médias das metades.
	lm, rm := mean(left), mean(right)
	var num, dl, dr float64
	for i := 0; i < half; i++ {
		j := len(right) - 1 - i
		aL := left[i] - lm
		bR := right[j] - rm
		num += aL * bR
		dl += aL * aL
		dr += bR * bR
	}
	var shape float64
	if dl > 0 && dr > 0 {
		shape = num / math.Sqrt(dl*dr)
		if shape < 0 {
			shape = 0
		}
	} else if dl == 0 && dr == 0 {
		// Ambas as metades uniformes. A simetria aqui depende de as MÉDIAS
		// serem iguais (métrica só de forma não captura amplitude). Para uma
		// imagem sólida (lm==rm) é perfeitamente simétrica; para metades
		// uniformes porém diferentes (ex: branco vs preto) NÃO é.
		shape = 1 - clamp01(abs(lm-rm)/128.0)
	}
	amp := 1 - clamp01(abs(lm-rm)/128.0)
	if amp < 0 {
		amp = 0
	}
	return clamp01(0.7*shape + 0.3*amp)
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}
