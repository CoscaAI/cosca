// Package edit — Language-driven Scene Editing (World Building, ADR-021;
// mineração SimWorld AssetsRP).
//
// O padrão I1 perfeito: o LLM/agente PROPÕE uma intenção estruturada
// ({asset, reference, relation, offset, surrounding}); o SISTEMA resolve o espaço
// determinístico (posição + orientação). O LLM NUNCA escolhe coordenadas — só
// intenção. Exatamente a separação proposal→gate do Cosca.
//
// Invariantes:
//   - I1: Resolve é determinístico, zero LLM.
//   - I2 (fail-closed): relação inválida / offset <= 0 → ERRO (nunca "placement
//     vazio silencioso").
package edit

import (
	"fmt"
	"math"

	"github.com/CoscaAI/cosca/internal/world"
)

// Relation é a posição relativa ao referencial de referência.
type Relation string

const (
	Front Relation = "front"
	Back  Relation = "back"
	Left  Relation = "left"
	Right Relation = "right"
)

// PlacementIntent é a intenção estruturada proposta pelo LLM/agente.
type PlacementIntent struct {
	Asset       string   `json:"asset"`
	Reference   string   `json:"reference"`
	Relation    Relation `json:"relation"`
	Offset      float64  `json:"offset"`
	Surrounding string   `json:"surrounding,omitempty"`
}

// Placement é o resultado do sistema (posição + orientação no espaço-mundo).
type Placement struct {
	Position world.Vec3 `json:"position"`
	Heading  float64    `json:"heading"` // rad — o asset "olha" para a referência
}

// DefaultForward é a direção canônica "frente" quando a referência não tem
// orientação conhecida. No plano horizontal (XZ, Y-up), norte = +Z.
var DefaultForward = world.Vec3{X: 0, Y: 0, Z: 1}

// Resolve decide o placement no espaço-mundo a partir da intenção. A referência
// é descrita por (posição, forward) providos pelo SISTEMA (fonte de verdade do
// mundo). Determinístico (I1); fail-closed (I2).
func Resolve(referencePos, referenceForward world.Vec3, intent PlacementIntent) (Placement, error) {
	if intent.Offset <= 0 {
		return Placement{}, fmt.Errorf("placement: offset must be > 0 (fail-closed I2)")
	}
	if !validRelation(intent.Relation) {
		return Placement{}, fmt.Errorf("placement: invalid relation %q (fail-closed I2)", intent.Relation)
	}
	fwd := referenceForward
	if fwd == (world.Vec3{}) {
		fwd = DefaultForward
	}
	dir := relationDir(intent.Relation, fwd)
	target := referencePos.Add(dir.Scale(intent.Offset))
	heading := math.Atan2(referencePos.X-target.X, referencePos.Y-target.Y)
	return Placement{Position: target, Heading: heading}, nil
}

// relationDir devolve o vetor direção da relação no frame da referência.
// front=+fwd, back=-fwd, left/right = perpendiculares.
func relationDir(r Relation, fwd world.Vec3) world.Vec3 {
	// perpendicular à esquerda (rotação -90° em XZ): (−fwd.Z, 0, fwd.X)
	perp := world.Vec3{X: -fwd.Z, Y: 0, Z: fwd.X}
	switch r {
	case Front:
		return fwd
	case Back:
		return fwd.Scale(-1)
	case Left:
		return perp
	case Right:
		return perp.Scale(-1)
	default:
		return fwd
	}
}

func validRelation(r Relation) bool {
	return r == Front || r == Back || r == Left || r == Right
}
