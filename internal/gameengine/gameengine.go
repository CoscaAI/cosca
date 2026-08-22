// Package gameengine implementa a Game Engine (§10 do manifesto
// Creative/Scientific/Media) — Fase 7.
//
// Modelo ECS (Entity-Component-System): SCENE → ENTITY → COMPONENT → SYSTEM.
// Em Go puro (zero libs — Regra L211). O protótipo de jogo é um GRAFO de
// cena serializável: entidades com componentes declarativos; sistemas
// processam. IA pode gerar a descrição → cena → entidades → lógica (§10:
// "Crie um jogo de plataforma" → game design → scene → entities → logic).
package gameengine

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ComponentType identifica um componente.
type ComponentType string

// Tipos de componente comuns (§10: física, animação, áudio, input, UI...).
const (
	ComponentTransform  ComponentType = "transform"  // posição/rotação/escala
	ComponentPhysics    ComponentType = "physics"    // massa/velocidade/gravidade
	ComponentRender     ComponentType = "render"     // sprite/modelo/cor
	ComponentAnimation  ComponentType = "animation"  // animação/clip
	ComponentAudio      ComponentType = "audio"      // som/trilha
	ComponentInput      ComponentType = "input"      // controle do jogador
	ComponentAI         ComponentType = "ai"         // comportamento
	ComponentCollider   ComponentType = "collider"   // colisão
	ComponentHealth     ComponentType = "health"     // vida/dano
	ComponentScore      ComponentType = "score"      // pontuação
	ComponentUnknown    ComponentType = "unknown"
)

// Valid reports se o tipo de componente é conhecido.
func (c ComponentType) Valid() bool {
	switch c {
	case ComponentTransform, ComponentPhysics, ComponentRender, ComponentAnimation,
		ComponentAudio, ComponentInput, ComponentAI, ComponentCollider,
		ComponentHealth, ComponentScore:
		return true
	}
	return false
}

// Component é um componente declarativo de uma entidade.
type Component struct {
	// Type do componente.
	Type ComponentType `json:"type"`
	// Params do componente (velocidade, sprite, cor, dano...).
	Params map[string]any `json:"params,omitempty"`
}

// Entity é uma entidade com componentes.
type Entity struct {
	// ID único na cena.
	ID string `json:"id"`
	// Name legível (ex.: "player", "enemy_1").
	Name string `json:"name,omitempty"`
	// Components da entidade.
	Components []Component `json:"components,omitempty"`
}

// HasComponent reports se a entidade tem o componente dado.
func (e *Entity) HasComponent(t ComponentType) bool {
	for _, c := range e.Components {
		if c.Type == t {
			return true
		}
	}
	return false
}

// Scene é uma cena do jogo (nível/mapa).
type Scene struct {
	// Name da cena (ex.: "level-1").
	Name string `json:"name"`
	// Width/Height da cena em unidades do mundo (opcional).
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
	// Entities da cena.
	Entities []*Entity `json:"entities,omitempty"`
}

// EntityCount devolve o número de entidades.
func (s *Scene) EntityCount() int { return len(s.Entities) }

// AddEntity adiciona uma entidade (valida ID único).
func (s *Scene) AddEntity(e *Entity) error {
	if e == nil || strings.TrimSpace(e.ID) == "" {
		return fmt.Errorf("entity id is required")
	}
	for _, x := range s.Entities {
		if x.ID == e.ID {
			return fmt.Errorf("entity %q already exists", e.ID)
		}
	}
	for _, c := range e.Components {
		if !c.Type.Valid() {
			return fmt.Errorf("entity %q: invalid component type %q", e.ID, c.Type)
		}
	}
	s.Entities = append(s.Entities, e)
	return nil
}

// EntityByID devolve uma entidade por ID.
func (s *Scene) EntityByID(id string) (*Entity, bool) {
	for _, e := range s.Entities {
		if e.ID == id {
			return e, true
		}
	}
	return nil, false
}

// EntitiesWith devolve as entidades que têm o componente dado.
func (s *Scene) EntitiesWith(t ComponentType) []*Entity {
	var out []*Entity
	for _, e := range s.Entities {
		if e.HasComponent(t) {
			out = append(out, e)
		}
	}
	return out
}

// SortEntities devolve as entidades ordenadas por ID (estável).
func (s *Scene) SortEntities() []*Entity {
	out := append([]*Entity(nil), s.Entities...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// =============================================================================
// Sistemas (§10: SYSTEM processa entidades com componentes)
// =============================================================================

// System processa entidades que têm o componente alvo (padrão ECS).
type System interface {
	// Name do sistema.
	Name() string
	// Component processado.
	Component() ComponentType
	// Process aplica o sistema às entidades da cena. Retorna relatório.
	Process(scene *Scene) (string, error)
}

// CountSystem conta entidades com um componente (sistema de diagnóstico).
type CountSystem struct {
	Comp ComponentType
}

// Name implementa System.
func (s CountSystem) Name() string { return "count:" + string(s.Comp) }

// Component implementa System.
func (s CountSystem) Component() ComponentType { return s.Comp }

// Process conta as entidades com o componente.
func (s CountSystem) Process(scene *Scene) (string, error) {
	n := len(scene.EntitiesWith(s.Comp))
	return fmt.Sprintf("%d entidades com %s", n, s.Comp), nil
}

// =============================================================================
// Serialização (grafo de cena JSON — §10/§21)
// =============================================================================

// Marshal serializa a cena em JSON.
func (s *Scene) Marshal() ([]byte, error) {
	return json.Marshal(s)
}

// MarshalIndent serializa com indentação.
func (s *Scene) MarshalIndent() ([]byte, error) { return json.MarshalIndent(s, "", "  ") }

// UnmarshalScene desserializa uma cena JSON.
func UnmarshalScene(data []byte) (*Scene, error) {
	var s Scene
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("game: parse scene: %w", err)
	}
	if strings.TrimSpace(s.Name) == "" {
		return nil, fmt.Errorf("game: scene name is required")
	}
	// Valida entidades e componentes.
	seen := map[string]bool{}
	for _, e := range s.Entities {
		if e == nil || strings.TrimSpace(e.ID) == "" {
			return nil, fmt.Errorf("game: entity id is required")
		}
		if seen[e.ID] {
			return nil, fmt.Errorf("game: duplicate entity %q", e.ID)
		}
		seen[e.ID] = true
		for _, c := range e.Components {
			if !c.Type.Valid() {
				return nil, fmt.Errorf("game: entity %q: invalid component %q", e.ID, c.Type)
			}
		}
	}
	return &s, nil
}

// ComponentsList devolve a lista legível de componentes válidos.
func ComponentsList() string {
	return strings.Join([]string{
		string(ComponentTransform), string(ComponentPhysics), string(ComponentRender),
		string(ComponentAnimation), string(ComponentAudio), string(ComponentInput),
		string(ComponentAI), string(ComponentCollider), string(ComponentHealth),
		string(ComponentScore),
	}, ", ")
}
