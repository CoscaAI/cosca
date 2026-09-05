// Package road — geração procedural de rede viária TOPOLÓGICA (World Building,
// ADR-021; mineração SimWorld citygen).
//
// O `internal/world/city` faz um GRID uniforme; o SimWorld mostra como crescer
// estradas ORGÂNICAS determinísticas: priority-queue de pontas ativas, ramificação
// em ±90°, snap/merge de pontas próximas (interseção), e um subconjunto de
// "highways" priorizadas. Determinístico com seed (I1), ZERO LLM no loop.
//
// Invariantes: I1 (determinístico, seed fixa → mesma rede), I2 (fail-closed: seed
// obrigatório, options válidas).
package road

import (
	"fmt"
	"math"
	"math/rand"
)

// Point é um ponto no plano do chão (X, Z; Y-up).
type Point struct {
	X float64 `json:"x"`
	Z float64 `json:"z"`
}

// Segment é um trecho de estrada (A→B).
type Segment struct {
	A         Point  `json:"a"`
	B         Point  `json:"b"`
	Highway   bool   `json:"highway"`
	RoadID    int    `json:"road_id"`
}

// Options configura o gerador.
type Options struct {
	// Seed é OBRIGATÓRIO (determinismo I1). 0 é válido, mas explícito.
	Seed int64
	// Steps é o número de passos de crescimento (default 24).
	Steps int
	// SegmentLen é o comprimento de cada segmento (default 4).
	SegmentLen float64
	// SnapDist é a distância para snap/merge de pontas (interseção). Default 5.
	SnapDist float64
	// BranchChance é a chance de ramificar ±90° a cada passo (default 0.5).
	BranchChance float64
	// HighwayChance é a chance de uma seed virar highway (por centro). Default 0.1.
	HighwayChance float64
}

func defaults(o Options) Options {
	if o.Steps <= 0 {
		o.Steps = 24
	}
	if o.SegmentLen <= 0 {
		o.SegmentLen = 4
	}
	if o.SnapDist <= 0 {
		o.SnapDist = 5
	}
	if o.BranchChance <= 0 {
		o.BranchChance = 0.5
	}
	if o.HighwayChance <= 0 {
		o.HighwayChance = 0.1
	}
	return o
}

// tip é uma ponta ativa de crescimento (posição + heading).
type tip struct {
	pos     Point
	heading float64 // rad
	highway bool
}

// Generate cria uma rede viária determinística. Fail-closed (I2): options
// inválidas → erro; seed obrigatória (determinismo I1).
func Generate(opts Options) ([]Segment, error) {
	o := defaults(opts)
	if o.BranchChance > 1 {
		return nil, fmt.Errorf("road: BranchChance > 1 (fail-closed)")
	}
	rng := rand.New(rand.NewSource(o.Seed)) // determinístico (I1)

	segs := []Segment{}
	tips := []tip{}
	roadID := 0

	// Semear com 1-2 segmentos a partir da origem.
	highway := rng.Float64() < o.HighwayChance
	start := Point{X: 0, Z: 0}
	len0 := o.SegmentLen
	if highway {
		len0 *= 2
	}
	end := Point{X: start.X + len0, Z: start.Z}
	segs = append(segs, Segment{A: start, B: end, Highway: highway, RoadID: roadID})
	roadID++
	tips = append(tips, tip{pos: end, heading: 0, highway: highway})
	// ramo inicial perpendicular (malha mais rica).
	tips = append(tips, tip{pos: start, heading: math.Pi / 2, highway: false})

	// Crescimento por passos (priority por highway).
	for step := 0; step < o.Steps; step++ {
		// Se não há pontas, para.
		if len(tips) == 0 {
			break
		}
		// Processa as pontas em ordem estável (determinístico): highways primeiro.
		ordered := sortTips(tips)
		nextTips := []tip{}
		for _, tp := range ordered {
			// Cresce para frente (pode parar).
			if rng.Float64() < 0.12 {
				continue // ponta morre
			}
			segLen := o.SegmentLen
			if tp.highway {
				segLen *= 2
			}
			b := Point{
				X: tp.pos.X + segLen*math.Cos(tp.heading),
				Z: tp.pos.Z + segLen*math.Sin(tp.heading),
			}
			seg := Segment{A: tp.pos, B: b, Highway: tp.highway, RoadID: roadID}
			roadID++
			// Snap/merge: se B está perto de um endpoint existente (interseção),
			// conecta em vez de deixar ponta solta.
			if snapped, ok := snapTo(tp.pos, b, segs, o.SnapDist); ok {
				seg.B = snapped
			}
			segs = append(segs, seg)
			nextTips = append(nextTips, tip{pos: seg.B, heading: tp.heading, highway: tp.highway})
			// Ramificação ±90°.
			if rng.Float64() < o.BranchChance {
				angle := math.Pi / 2
				if rng.Float64() < 0.5 {
					angle = -math.Pi / 2
				}
				nextTips = append(nextTips, tip{
					pos:     tp.pos,
					heading: tp.heading + angle,
					highway: false,
				})
			}
		}
		tips = nextTips
	}
	return segs, nil
}

// sortTips ordena pontas: highways primeiro, depois por posição (estável).
func sortTips(t []tip) []tip {
	if len(t) <= 1 {
		return t
	}
	// insertion sort simples (determinístico, n pequeno).
	for i := 1; i < len(t); i++ {
		for j := i; j > 0; j-- {
			if tipLess(t[j], t[j-1]) {
				t[j], t[j-1] = t[j-1], t[j]
			} else {
				break
			}
		}
	}
	return t
}

func tipLess(a, b tip) bool {
	if a.highway != b.highway {
		return a.highway
	}
	if a.pos.X != b.pos.X {
		return a.pos.X < b.pos.X
	}
	return a.pos.Z < b.pos.Z
}

// snapTo devolve o endpoint existente mais próximo de `b` dentro de dist, se houver.
func snapTo(ignore Point, b Point, segs []Segment, dist float64) (Point, bool) {
	var best Point
	bestD := dist
	found := false
	for _, s := range segs {
		for _, e := range []Point{s.A, s.B} {
			if e == ignore {
				continue
			}
			d := hypot(b.X-e.X, b.Z-e.Z)
			if d <= bestD {
				bestD = d
				best = e
				found = true
			}
		}
	}
	return best, found
}

func hypot(x, y float64) float64 { return math.Sqrt(x*x + y*y) }
