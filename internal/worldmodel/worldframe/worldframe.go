// Package worldframe — sistema de coordenadas de MUNDO compartilhado
// (World Building, ADR-021; mineração see-through-walls / VPS).
//
// O insight: várias entidades/agentes podem concordar num PONTO ABSOLUTO do mundo
// mesmo sem se verem — se todos estão ancorados num MESMO frame (a "origem do
// mundo"). Um Anchor define um frame de referência; um Registry deixa N agentes
// registrarem suas poses relativas e resolverem cada um para o frame-absoluto.
//
// Determinístico (I1), zero LLM. O Cosca é dono do frame (I7/I8 — não aluga a
// verdade espacial de terceiros).
package worldframe

import (
	"fmt"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// Anchor é um frame de referência do mundo (origem + orientação).
type Anchor struct {
	ID         string         `json:"id"`
	Origin     worldmodel.Vec3 `json:"origin"`
	Orientation worldmodel.Quat `json:"orientation"`
}

// Registry mantém âncoras e poses registradas de agentes/entidades.
// Determinístico (I1) — nada de julgamento externo.
type Registry struct {
	anchors map[string]Anchor
	poses   map[string]Pose
}

// Pose é a pose de um agente/entidade ANCORADA (registrada num frame).
type Pose struct {
	Agent    string         `json:"agent"`
	AnchorID string         `json:"anchor_id"`
	Local    worldmodel.Vec3 `json:"local"` // posição relativa ao anchor
}

// NewRegistry cria um registry vazio.
func NewRegistry() *Registry {
	return &Registry{anchors: map[string]Anchor{}, poses: map[string]Pose{}}
}

// AddAnchor registra um frame de referência.
func (r *Registry) AddAnchor(a Anchor) {
	r.anchors[a.ID] = a
}

// RegisterPose ancorar um agente a um anchor (pose local→frame). Determinístico.
func (r *Registry) RegisterPose(p Pose) error {
	if _, ok := r.anchors[p.AnchorID]; !ok {
		return fmt.Errorf("anchor %q not found (fail-closed I2)", p.AnchorID)
	}
	r.poses[p.Agent] = p
	return nil
}

// Resolve converte a pose local de um agente para o frame ABSOLUTO do mundo.
// Dois agentes no MESMO anchor → mesma origem absoluta (concordam no ponto).
func (r *Registry) Resolve(agent string) (worldmodel.Vec3, error) {
	p, ok := r.poses[agent]
	if !ok {
		return worldmodel.Vec3{}, fmt.Errorf("agent %q not anchored (fail-closed)", agent)
	}
	a, ok := r.anchors[p.AnchorID]
	if !ok {
		return worldmodel.Vec3{}, fmt.Errorf("anchor %q missing (fail-closed)", p.AnchorID)
	}
	return a.Origin.Add(p.Local), nil
}

// Anchored informa se um agente está ancorado no frame compartilhado.
func (r *Registry) Anchored(agent string) bool {
	_, ok := r.poses[agent]
	return ok
}

// AgreeAbsolute informa se dois agentes compartilham o MESMO anchor (mesmo
// espaço absoluto) — a essência do "ver através da parede" (concordam no ponto
// mesmo sem se verem).
func (r *Registry) AgreeAbsolute(a, b string) bool {
	pa, oka := r.poses[a]
	pb, okb := r.poses[b]
	return oka && okb && pa.AnchorID == pb.AnchorID
}
