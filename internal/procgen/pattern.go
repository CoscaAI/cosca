package procgen

import "math"

// Família PATTERN (os 5 do Professor) — cada função devolve um valor em um
// ponto (x,y) normalizado 0..1, para ser usado em shader/imagem.

// Gradient devolve um gradiente diagonal (canto inferior-esquerdo →
// superior-direito): (x+y)/2, [0,1].
func Gradient(x, y float64) float64 {
	return Clamp((x+y)*0.5, 0, 1)
}

// Checker devolve 0 ou 1 (quadrados xadrez) com `scale` células por unidade.
func Checker(x, y float64, scale int) float64 {
	if scale < 1 {
		scale = 1
	}
	cx := int(math.Floor(x * float64(scale)))
	cy := int(math.Floor(y * float64(scale)))
	if (cx+cy)&1 == 0 {
		return 1
	}
	return 0
}

// Stripes devolve 0 ou 1 (listras verticais) com `scale` listras por unidade.
func Stripes(x, y float64, scale int) float64 {
	if scale < 1 {
		scale = 1
	}
	if int(math.Floor(x*float64(scale)))&1 == 0 {
		return 1
	}
	return 0
}

// Cells devolve um valor aleatório determinístico por célula Voronoi — células
// de cor constante (o hash da célula via RNG seed).
func Cells(x, y float64, rng *RNG, scale int) float64 {
	if scale < 1 {
		scale = 1
	}
	cx := int(math.Floor(x * float64(scale)))
	cy := int(math.Floor(y * float64(scale)))
	return Hash2(cx, cy, rng.state)
}

// Rings devolve anéis concêntricos em torno do centro (0.5,0.5):
// sin(dist*scale)*0.5+0.5 — distância euclidiana ao centro.
func Rings(x, y float64, scale float64) float64 {
	dx := x - 0.5
	dy := y - 0.5
	return math.Sin(math.Hypot(dx, dy)*scale)*0.5 + 0.5
}
