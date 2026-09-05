// Package scene implementa o Scene Graph do Cosca (fase 2 da ordem do
// Professor, L303) — o grafo de ENTIDADES ESPACIAIS (Camera, Sun, Terrain,
// Tree, ParticleSystem...). A separação conceitual é a pedra angular:
//
//	Node Graph  = grafo de COMPUTAÇÃO (operações)  → internal/nodegraph
//	Scene Graph = grafo de ENTIDADES ESPACIAIS     → este pacote
//
// Cada entidade APONTA para uma operação do node graph (Entity.NodeRef) —
// evita transformar o node graph num monolito. O Visual Compiler (Scene DSL →
// cena) é a PRÓXIMA fase; aqui não existe DSL.
package scene

import "math"

// Vec3 é um vetor/posição 3D em float64 (matemática da casa: nunca float32).
type Vec3 struct {
	X, Y, Z float64
}

// Vec3Add soma dois vetores.
func Vec3Add(a, b Vec3) Vec3 { return Vec3{X: a.X + b.X, Y: a.Y + b.Y, Z: a.Z + b.Z} }

// Vec3Sub subtrai b de a.
func Vec3Sub(a, b Vec3) Vec3 { return Vec3{X: a.X - b.X, Y: a.Y - b.Y, Z: a.Z - b.Z} }

// Vec3Scale multiplica um vetor por um escalar.
func Vec3Scale(a Vec3, s float64) Vec3 { return Vec3{X: a.X * s, Y: a.Y * s, Z: a.Z * s} }

// Vec3Dot devolve o produto escalar.
func Vec3Dot(a, b Vec3) float64 { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }

// Vec3Cross devolve o produto vetorial a×b (perpendicular a ambos).
func Vec3Cross(a, b Vec3) Vec3 {
	return Vec3{
		X: a.Y*b.Z - a.Z*b.Y,
		Y: a.Z*b.X - a.X*b.Z,
		Z: a.X*b.Y - a.Y*b.X,
	}
}

// Vec3Length devolve o comprimento (norma L2).
func Vec3Length(a Vec3) float64 { return math.Sqrt(Vec3Dot(a, a)) }

// Vec3Normalize devolve o vetor unitário. Vetor nulo (comprimento ~0)
// devolve o vetor zero — fail-closed: nunca NaN.
func Vec3Normalize(a Vec3) Vec3 {
	l := Vec3Length(a)
	if l == 0 {
		return Vec3{}
	}
	return Vec3Scale(a, 1.0/l)
}

// Transform é a transformação espacial de uma entidade: translação (Position),
// rotação (Rotation, em GRAUS — UX da casa; convertida internamente para
// radianos) e escala (Scale). A composição é T*R*S (escala → rotação →
// translação aplicadas ao ponto) — decisão da família, ver LocalMatrix.
//
// ATENÇÃO: o valor zero de Transform é DEGENERADO (Scale = {0,0,0} colapsa a
// ponto). Construa sempre a partir de Identity() (os construtores de builder.go
// já fazem isso).
type Transform struct {
	Position Vec3 `json:"position"`
	Rotation Vec3 `json:"rotation"` // graus, eixos X/Y/Z (neste campo, não confundir com a matriz)
	Scale    Vec3 `json:"scale"`
}

// Identity devolve a transformação neutra: posição (0,0,0), rotação 0,
// escala (1,1,1).
func Identity() Transform {
	return Transform{Scale: Vec3{X: 1, Y: 1, Z: 1}}
}

// IsIdentity reports se a transformação é a identidade (tolerância 1e-9).
func (t Transform) IsIdentity() bool {
	const eps = 1e-9
	if math.Abs(t.Position.X) > eps || math.Abs(t.Position.Y) > eps || math.Abs(t.Position.Z) > eps {
		return false
	}
	if math.Abs(t.Rotation.X) > eps || math.Abs(t.Rotation.Y) > eps || math.Abs(t.Rotation.Z) > eps {
		return false
	}
	if math.Abs(t.Scale.X-1) > eps || math.Abs(t.Scale.Y-1) > eps || math.Abs(t.Scale.Z-1) > eps {
		return false
	}
	return true
}

// Convenção de matriz — documentada:
//
//	4x4 column-major (padrão de gráficos): elemento (linha r, coluna c) vive
//	em m[c*4+r]. O vetor é COLUNA: v' = M·v.
//	Composição local = T * R * S, com R = Rz * Ry * Rx (rotação em graus).
//	Ordem aplicada ao ponto: S primeiro, depois R, depois T.

// identity4 devolve a matriz identidade 4x4 column-major.
func identity4() [16]float64 {
	return [16]float64{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
}

// mul4 multiplica duas matrizes 4x4 column-major (M = A·B).
func mul4(a, b [16]float64) [16]float64 {
	var m [16]float64
	for c := 0; c < 4; c++ {
		for r := 0; r < 4; r++ {
			var sum float64
			for k := 0; k < 4; k++ {
				sum += a[k*4+r] * b[c*4+k]
			}
			m[c*4+r] = sum
		}
	}
	return m
}

func translation4(p Vec3) [16]float64 {
	m := identity4()
	m[12], m[13], m[14] = p.X, p.Y, p.Z
	return m
}

// rotation4 monta R = Rz * Ry * Rx a partir da rotação em GRAUS.
func rotation4(r Vec3) [16]float64 {
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	x, y, z := rad(r.X), rad(r.Y), rad(r.Z)
	cx, sx := math.Cos(x), math.Sin(x)
	cy, sy := math.Cos(y), math.Sin(y)
	cz, sz := math.Cos(z), math.Sin(z)
	// Rx (coluna-major)
	rx := [16]float64{1, 0, 0, 0, 0, cx, sx, 0, 0, -sx, cx, 0, 0, 0, 0, 1}
	// Ry
	ry := [16]float64{cy, 0, -sy, 0, 0, 1, 0, 0, sy, 0, cy, 0, 0, 0, 0, 1}
	// Rz
	rz := [16]float64{cz, sz, 0, 0, -sz, cz, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
	return mul4(rz, mul4(ry, rx))
}

func scale4(s Vec3) [16]float64 {
	m := identity4()
	m[0], m[5], m[10] = s.X, s.Y, s.Z
	return m
}

// transformPoint aplica a matriz 4x4 (coluna-major) a um ponto: v' = M·v.
// Assume w=1 (ponto afim — translação sempre aplicada).
func transformPoint(m [16]float64, v Vec3) Vec3 {
	return Vec3{
		X: m[0]*v.X + m[4]*v.Y + m[8]*v.Z + m[12],
		Y: m[1]*v.X + m[5]*v.Y + m[9]*v.Z + m[13],
		Z: m[2]*v.X + m[6]*v.Y + m[10]*v.Z + m[14],
	}
}

// LocalMatrix devolve a matriz 4x4 column-major local: T * R * S. A ordem é a
// decisão da família (regra 9): um ponto local p vira T·(R·(S·p)) — escala
// primeiro, depois rotação, depois translação.
func (t Transform) LocalMatrix() [16]float64 {
	rs := mul4(rotation4(t.Rotation), scale4(t.Scale))
	return mul4(translation4(t.Position), rs)
}

// WorldMatrix compõe a transformação com a do pai: world = pai_local · local.
// É a hierarquia local→mundo: um ponto local do filho é levado ao espaço do
// pai, e recursivamente ao mundo pela cadeia de pais.
func (t Transform) WorldMatrix(parent Transform) [16]float64 {
	return mul4(parent.LocalMatrix(), t.LocalMatrix())
}

// Apply transforma um ponto do espaço LOCAL da transformação para o espaço
// do pai (local→mundo relativo): usa LocalMatrix, v' = M·v.
func (t Transform) Apply(v Vec3) Vec3 {
	return transformPoint(t.LocalMatrix(), v)
}
