package screen

import (
	"image"
	"math"
)

// detectRegions detecta regiões candidatas a texto em uma imagem usando
// visão computacional clássica (determinística, zero modelo):
//
//  1. Convert para grayscale.
//  2. Binariza por threshold adaptativo (Otsu) — separa foreground/background.
//  3. Projeta as linhas horizontais para achar "blocos de texto" (linhas com
//     alto preenchimento de pixels).
//  4. Acha componentes conectados (characters/blocos) e agrega em regiões
//     horizontais com padding.
//  5. Classifica como text (densidade de conteúdo alto) ou graphic/unknown.
//
// Não É OCR — só responde "aqui provalvemente existe texto". O OCR (plugável)
// decide o que está escrito.
func detectRegions(img image.Image) []Region {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return nil
	}

	// 1. grayscale + acumula histograma para Otsu.
	gray := make([]uint8, w*h)
	hist := make([]int, 256)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			// luminância perceptiva rec.601
			l := uint8(0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(bl>>8))
			idx := (y-b.Min.Y)*w + (x - b.Min.X)
			gray[idx] = l
			hist[l]++
		}
	}

	thr := otsuThreshold(hist, w*h)
	// 2. binariza: 1 = foreground (texto), 0 = background.
	// Usamos <= thr para capturar texto no limite do limiar (pixel escuro). Em
	// uma captura de tela real o texto é claramente mais escuro que o fundo, e
	// o <= garante que pixels no próprio thr não sejam perdidos.
	binary := make([]uint8, w*h)
	for i, l := range gray {
		if l <= thr {
			binary[i] = 1
		}
	}

	// 3. projeção horizontal: quantos pixels de foreground por linha.
	rowFill := make([]int, h)
	for y := 0; y < h; y++ {
		var c int
		for x := 0; x < w; x++ {
			c += int(binary[y*w+x])
		}
		rowFill[y] = c
	}

	// Acha bandas de texto: sequências de linhas com preenchimento acima de
	// um limiar proporcional à largura.
	minFill := int(float64(w) * 0.08)
	bands := imageBands(rowFill, minFill)

	var regions []Region
	for _, band := range bands {
		// 4. dentro de cada banda, agrupa por projeção vertical (colunas) em
		// blocos horizontais de texto.
		cols := projectColumns(binary, band.top, band.bottom, w)
		blocks := columnBlocks(cols, band.top, band.bottom, w, h)
		regions = append(regions, blocks...)
	}

	// Filtra regiões minúsculas (ruído) e classifica.
	out := regions[:0]
	for _, r := range regions {
		if r.BBox.W < 20 || r.BBox.H < 8 {
			continue
		}
		r.Kind = classifyRegion(binary, w, r)
		r.Confidence = 0.6 // detector clássico; confiança heurística
		out = append(out, r)
	}
	return out
}

// otsuThreshold calcula o limiar de binarização de Otsu a partir do histograma.
func otsuThreshold(hist []int, total int) uint8 {
	if total <= 0 {
		return 128
	}
	var sum float64
	for i, c := range hist {
		sum += float64(i) * float64(c)
	}
	var sumB float64
	wB := 0
	maxVar := math.Inf(-1)
	thr := 128
	for i, c := range hist {
		wB += c
		if wB == 0 {
			continue
		}
		wF := total - wB
		if wF == 0 {
			break
		}
		sumB += float64(i) * float64(c)
		mB := sumB / float64(wB)
		mF := (sum - sumB) / float64(wF)
		between := float64(wB) * float64(wF) * (mB - mF) * (mB - mF)
		if between > maxVar {
			maxVar = between
			thr = i
		}
	}
	return uint8(thr)
}

// band é um intervalo vertical [top,bottom) de linhas.
type band struct{ top, bottom int }

// imageBands agrupa linhas com preenchimento >= minFill em bandas contíguas.
func imageBands(rowFill []int, minFill int) []band {
	var bands []band
	in := false
	start := 0
	for y, v := range rowFill {
		if v >= minFill && !in {
			in = true
			start = y
		} else if v < minFill && in {
			in = false
			bands = append(bands, band{top: start, bottom: y})
		}
	}
	if in {
		bands = append(bands, band{top: start, bottom: len(rowFill)})
	}
	return bands
}

// projectColumns conta o foreground por coluna dentro de uma banda.
func projectColumns(binary []uint8, top, bottom, w int) []int {
	h := bottom - top
	if h <= 0 {
		return nil
	}
	cols := make([]int, w)
	for x := 0; x < w; x++ {
		var c int
		for y := top; y < bottom; y++ {
			c += int(binary[y*w+x])
		}
		cols[x] = c
	}
	return cols
}

// columnBlocks agrupa colunas com foreground em blocos horizontais de texto
// (com padding), devolvendo regiões com bounding box. Aplica uma dilatação
// horizontal de 1 coluna para tolerar o pequeno gap entre caracteres de uma
// palavra (não fragmenta um bloco de texto em traços isolados).
func columnBlocks(cols []int, top, bottom, w, h int) []Region {
	_ = h
	// Limiar: coluna "ativa" se tem pelo menos um pixel de foreground por
	// linha útil (heurística).
	var minCol int
	if (bottom - top) > 0 {
		minCol = int(float64(bottom-top) * 0.15)
	}
	// Dilatação: marca colunas ativas, e também as duas seguintes, para
	// preencher o gap entre caracteres/traços de uma mesma palavra (texto real
	// tem caracteres quase contíguos; o gap típico é 1-3px). Isso evita
	// fragmentar um bloco de texto em traços isolados.
	active := make([]bool, len(cols))
	halfWidths := 2
	for x, v := range cols {
		if v >= minCol {
			active[x] = true
			for d := 1; d <= halfWidths && x+d < len(active); d++ {
				active[x+d] = true
			}
		}
	}
	var blocks []Region
	inBlock := false
	start := 0
	flush := func(end int) {
		if end-start < 8 { // descarta blocos muito estreitos (ruído)
			return
		}
		blocks = append(blocks, Region{
			BBox: Rect{X: start, Y: top, W: end - start, H: bottom - top},
		})
	}
	for x, a := range active {
		if a && !inBlock {
			inBlock = true
			start = x
		} else if !a && inBlock {
			inBlock = false
			flush(x)
		}
	}
	if inBlock {
		flush(len(active))
	}
	return blocks
}

// classifyRegion decide text vs graphic pela densidade de foreground e
// regularidade do bloco. Texto tem alta densidade de "traços" (pixels ativos)
// proporcional à área, em blocos estreitos e regulares.
func classifyRegion(binary []uint8, w int, r Region) RegionKind {
	area := r.BBox.W * r.BBox.H
	if area == 0 {
		return RegionUnknown
	}
	var fg int
	top := r.BBox.Y
	bottom := minInt(r.BBox.Y+r.BBox.H, len(binary)/w)
	left := r.BBox.X
	right := minInt(r.BBox.X+r.BBox.W, w)
	for y := top; y < bottom; y++ {
		for x := left; x < right; x++ {
			fg += int(binary[y*w+x])
		}
	}
	density := float64(fg) / float64(area)
	// Aspect ratio (largura/altura): texto tende a ser largo (linhas).
	aspect := float64(r.BBox.W) / float64(r.BBox.H)
	switch {
	case density > 0.12 && aspect > 2.0:
		return RegionText
	case density > 0.18:
		return RegionText
	case aspect > 3.0 && density > 0.05:
		return RegionText
	default:
		return RegionGraphic
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
