package procgen

import "math"

// Hash2 devolve um hash determinístico de uma célula do grid em [0,1).
// Mistura as coordenadas inteiras com o seed via splitmix64 (puramente
// inteiro → portátil byte a byte). Usado para gradientes e feature points.
func Hash2(x, y int, seed uint64) float64 {
	h := splitmix64(seed ^ uint64(uint32(x))*0x9E3779B97F4A7C15 ^ uint64(uint32(y))*0xBF58476D1CE4E5B9)
	return float64(h>>11) * (1.0 / (1 << 53))
}

// fade é a curva de interpolação 6t^5-15t^4+10t^3 (smoothstep de Perlin).
func fade(t float64) float64 { return t * t * t * (t*(t*6-15) + 10) }

// grad2 devolve o produto escalar do gradiente da célula (ângulo do hash) com
// o vetor (x,y) — o "gradient dot" clássico de Perlin.
func grad2(angle, x, y float64) float64 {
	dx := math.Cos(angle)
	dy := math.Sin(angle)
	return dx*x + dy*y
}

// Perlin2D — Perlin clássico 2D. Gradiente por hash do grid cell; retorna
// aproximadamente [-1,1].
func Perlin2D(rng *RNG, x, y float64) float64 {
	x0 := math.Floor(x)
	y0 := math.Floor(y)
	xf := x - x0
	yf := y - y0
	u := fade(xf)
	v := fade(yf)

	seed := rng.state
	ix := int(x0)
	iy := int(y0)

	n00 := grad2(Hash2(ix, iy, seed)*math.Pi*2, xf, yf)
	n10 := grad2(Hash2(ix+1, iy, seed)*math.Pi*2, xf-1, yf)
	n01 := grad2(Hash2(ix, iy+1, seed)*math.Pi*2, xf, yf-1)
	n11 := grad2(Hash2(ix+1, iy+1, seed)*math.Pi*2, xf-1, yf-1)

	nx0 := n00 + u*(n10-n00)
	nx1 := n01 + u*(n11-n01)
	return nx0 + v*(nx1-nx0)
}

// Simplex2D — Simplex noise 2D (skew/unskew + gradientes por hash).
// Retorna aproximadamente [-1,1] (fator de normalização 70).
func Simplex2D(rng *RNG, x, y float64) float64 {
	const (
		F2 = 0.3660254037844386  // (sqrt(3)-1)/2
		G2 = 0.21132486540518713 // (3-sqrt(3))/6
	)
	s := (x + y) * F2
	i := math.Floor(x + s)
	j := math.Floor(y + s)
	t := (i + j) * G2
	x0 := x - (i - t)
	y0 := y - (j - t)

	i1, j1 := 1, 0
	if x0 > y0 {
		i1, j1 = 1, 0
	} else {
		i1, j1 = 0, 1
	}
	x1 := x0 - float64(i1) + G2
	y1 := y0 - float64(j1) + G2
	x2 := x0 - 1 + 2*G2
	y2 := y0 - 1 + 2*G2

	seed := rng.state
	ii := int(i)
	jj := int(j)

	contribute := func(cx, cy int, gx, gy float64) float64 {
		t2 := 0.5 - gx*gx - gy*gy
		if t2 < 0 {
			return 0
		}
		t4 := t2 * t2
		return t4 * t4 * grad2(Hash2(cx, cy, seed)*math.Pi*2, gx, gy)
	}

	n0 := contribute(ii, jj, x0, y0)
	n1 := contribute(ii+i1, jj+j1, x1, y1)
	n2 := contribute(ii+1, jj+1, x2, y2)
	return 70 * (n0 + n1 + n2)
}

// Worley2D — distância ao feature point mais próximo (F1). Cada célula do grid
// tem um feature point determinístico (hash da célula). Retorna [0,~1]
// (normalizado pela diagonal √2 da célula).
func Worley2D(rng *RNG, x, y float64) float64 {
	cx := int(math.Floor(x))
	cy := int(math.Floor(y))
	seed := rng.state
	minDist := math.Inf(1)

	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			cellX := cx + dx
			cellY := cy + dy
			fx := float64(cellX) + Hash2(cellX, cellY, seed)
			fy := float64(cellY) + Hash2(cellX, cellY, seed^0x9E3779B97F4A7C15)
			dxf := x - fx
			dyf := y - fy
			d := dxf*dxf + dyf*dyf
			if d < minDist {
				minDist = d
			}
		}
	}
	return math.Sqrt(minDist) / math.Sqrt2
}

// Voronoi2D — campo de bordas de células (F2-F1). Retorna ~0 no feature point
// e próximo do máximo na fronteira entre células — o campo clássico para
// bordas de células (split/fracture). Retorna [0,~1].
//
// Decisão documentada: o spec pede "distância à célula mais próxima (para
// bordas de células)"; Worley2D já é o F1. Para diferenciar e atender o uso
// de bordas, Voronoi2D é o campo F2-F1 (distância ao segundo menos o primeiro
// feature point) — zera nas células e pica nas bordas.
func Voronoi2D(rng *RNG, x, y float64) float64 {
	cx := int(math.Floor(x))
	cy := int(math.Floor(y))
	seed := rng.state
	f1 := math.Inf(1)
	f2 := math.Inf(1)

	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			cellX := cx + dx
			cellY := cy + dy
			fx := float64(cellX) + Hash2(cellX, cellY, seed)
			fy := float64(cellY) + Hash2(cellX, cellY, seed^0xBF58476D1CE4E5B9)
			dxf := x - fx
			dyf := y - fy
			d := dxf*dxf + dyf*dyf
			switch {
			case d < f1:
				f2 = f1
				f1 = d
			case d < f2:
				f2 = d
			}
		}
	}
	return math.Sqrt(f2-f1) / math.Sqrt2
}

// FBM2D — fractal brownian motion genérico sobre qualquer base noise.
// Soma octaves com frequência *= lacunarity e amplitude *= gain, partindo de
// amplitude = persistence; normaliza pela soma das amplitudes → mesmo range
// da base. Com 1 octave devolve exatamente a base (rng não é avançado na
// primeira camada); a partir da segunda, avança o rng por octave para
// decorrelacionar as camadas (determinismo P1 preservado: seed + sequência de
// operações fixos).
func FBM2D(rng *RNG, x, y float64, octaves int, lacunarity, gain, persistence float64, base func(*RNG, float64, float64) float64) float64 {
	if octaves < 1 {
		octaves = 1
	}
	if lacunarity <= 0 {
		lacunarity = 2.0
	}
	if gain <= 0 {
		gain = 0.5
	}
	if persistence <= 0 {
		persistence = 1.0
	}
	var sum, norm, amp, freq float64
	amp = persistence
	freq = 1.0
	for i := 0; i < octaves; i++ {
		if i > 0 {
			_ = rng.Uint64() // decorrelaciona o seed das camadas seguintes
		}
		sum += amp * base(rng, x*freq, y*freq)
		norm += amp
		amp *= gain
		freq *= lacunarity
	}
	return sum / norm
}

// FbmPerlin — atalho de FBM sobre Perlin com lacunarity 2.0 e gain =
// persistence (a formulacão clássica do Professor).
func FbmPerlin(rng *RNG, x, y float64, octaves int, persistence float64) float64 {
	return FBM2D(rng, x, y, octaves, 2.0, persistence, persistence, Perlin2D)
}

// FbmSimplex — atalho de FBM sobre Simplex com lacunarity 2.0.
func FbmSimplex(rng *RNG, x, y float64, octaves int, persistence float64) float64 {
	return FBM2D(rng, x, y, octaves, 2.0, persistence, persistence, Simplex2D)
}
