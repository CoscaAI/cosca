package scene

import (
	"encoding/json"
	"fmt"
	"strings"
)

// EntityType identifica o papel espacial da entidade no Scene Graph.
type EntityType string

// Tipos canônicos de entidade. Camera/Light são entidades de visualização;
// Mesh/Terrain/Particle/Procedural são entidades de conteúdo; Group é o
// agrupador puro (não gera computação própria).
const (
	Camera     EntityType = "camera"
	Light      EntityType = "light"
	Mesh       EntityType = "mesh"
	Terrain    EntityType = "terrain"
	Particle   EntityType = "particle"
	Group      EntityType = "group"
	Procedural EntityType = "procedural"
	Unknown    EntityType = "unknown"
)

// Entity é uma entidade espacial do Scene Graph — dados puros, serializáveis
// (contrato igual ao do node graph). A entidade APONTA para a computação:
// NodeRef referencia um nó do node graph (ex: "fbm") e Params carrega os
// dados que o executor daquele nó vai consumir. A hierarquia pai-filho
// (Children) vive no espaço local de cada pai.
type Entity struct {
	// ID é único na cena (global).
	ID string `json:"id"`
	// Name é o nome legível (ex: "Tree_001").
	Name string `json:"name,omitempty"`
	// Type é o papel espacial (camera/light/mesh/terrain/particle/group/...).
	Type EntityType `json:"type"`
	// Transform é a transformação local (posição/rotação/escala).
	Transform Transform `json:"transform,omitempty"`
	// NodeRef é o ID de um nó do node graph (computação) — opcional. Quando
	// vazio, a entidade é só espacial (camera, light, group).
	NodeRef string `json:"node_ref,omitempty"`
	// Params são os parâmetros da entidade (ex: scale do terreno, densidade
	// de vegetação) — o que o executor do NodeRef vai consumir.
	Params map[string]any `json:"params,omitempty"`
	// Children são os filhos (espaço local).
	Children []*Entity `json:"children,omitempty"`
}

// sceneRootID é o ID do grupo sintético usado quando há mais de uma raiz
// (invariante da casa: uma cena tem UMA raiz — AddRoot promove à raiz única).
const sceneRootID = "scene-root"

// Scene é o grafo de entidades espaciais: nome + raiz única (que pode ser um
// Group, carregando toda a hierarquia da cena).
type Scene struct {
	Name string  `json:"name"`
	Root *Entity `json:"root,omitempty"`
}

// NewScene cria uma cena vazia.
func NewScene(name string) *Scene {
	return &Scene{Name: name}
}

// AddRoot adiciona uma entidade raiz, validando ID único (global). Invariante:
// a cena tem UMA raiz — se já houver raiz, as raízes são promovidas para um
// Group sintético "scene-root" (a hierarquia pai-filho é sempre uma árvore).
func (s *Scene) AddRoot(e *Entity) error {
	if e == nil {
		return fmt.Errorf("scene: entity is required")
	}
	existing := s.collectIDs()
	sub := map[string]bool{}
	if err := collectSubtreeIDs(e, sub, nil); err != nil {
		return fmt.Errorf("scene: add root %q: %w", e.ID, err)
	}
	for id := range sub {
		if existing[id] {
			return fmt.Errorf("scene: duplicate entity id %q", id)
		}
	}

	if s.Root == nil {
		s.Root = e
		return nil
	}
	if s.Root.Type == Group && s.Root.ID == sceneRootID {
		s.Root.Children = append(s.Root.Children, e)
		return nil
	}
	group := &Entity{ID: sceneRootID, Name: s.Name + " root", Type: Group}
	group.Children = append(group.Children, s.Root, e)
	s.Root = group
	return nil
}

// collectIDs devolve todos os IDs já presentes na cena (defensivo contra
// ciclos na hora de validar — o Validate pega o ciclo de verdade).
func (s *Scene) collectIDs() map[string]bool {
	ids := map[string]bool{}
	if s.Root != nil {
		_ = collectSubtreeIDs(s.Root, ids, nil)
	}
	return ids
}

// collectSubtreeIDs coleta os IDs da subárvore. visited evita loop infinito em
// estruturas cíclicas (inválidas); o erro é propagado como ciclo.
func collectSubtreeIDs(e *Entity, ids map[string]bool, visited map[*Entity]bool) error {
	if e == nil {
		return nil
	}
	if visited == nil {
		visited = map[*Entity]bool{}
	}
	if visited[e] {
		return fmt.Errorf("entity %q: cyclic hierarchy", e.ID)
	}
	visited[e] = true
	ids[e.ID] = true
	for _, c := range e.Children {
		if err := collectSubtreeIDs(c, ids, visited); err != nil {
			return err
		}
	}
	return nil
}

// AddChild anexa um filho a uma entidade (hierarquia pai-filho, espaço local).
func (e *Entity) AddChild(c *Entity) error {
	if e == nil {
		return fmt.Errorf("scene: parent is required")
	}
	if c == nil {
		return fmt.Errorf("scene: child is required")
	}
	e.Children = append(e.Children, c)
	return nil
}

// Find busca uma entidade pelo ID na hierarquia toda (recursiva).
func (s *Scene) Find(id string) (*Entity, bool) {
	return findIn(s.Root, id)
}

func findIn(e *Entity, id string) (*Entity, bool) {
	if e == nil {
		return nil, false
	}
	if e.ID == id {
		return e, true
	}
	for _, c := range e.Children {
		if f, ok := findIn(c, id); ok {
			return f, true
		}
	}
	return nil, false
}

// Validate verifica os invariantes da cena: nome presente, IDs únicos no grafo
// todo, hierarquia sem ciclos (árvore pai-filho). NodeRef não é validado
// contra o grafo externo aqui — só o formato (sem espaços em branco).
func (s *Scene) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("scene name is required")
	}
	if s.Root == nil {
		return nil
	}

	seen := make(map[string]bool)
	visiting := make(map[*Entity]bool)
	visited := make(map[*Entity]bool)
	var walk func(e *Entity) error
	walk = func(e *Entity) error {
		if e == nil {
			return nil
		}
		if visiting[e] {
			return fmt.Errorf("scene: cyclic hierarchy at entity %q", e.ID)
		}
		if visited[e] {
			return fmt.Errorf("scene: entity %q appears more than once in the hierarchy", e.ID)
		}
		if seen[e.ID] {
			return fmt.Errorf("scene: duplicate entity id %q", e.ID)
		}
		if strings.ContainsAny(e.NodeRef, " \t\r\n") {
			return fmt.Errorf("scene: entity %q has malformed node_ref %q", e.ID, e.NodeRef)
		}
		seen[e.ID] = true
		visiting[e] = true
		for _, c := range e.Children {
			if err := walk(c); err != nil {
				return err
			}
		}
		visiting[e] = false
		visited[e] = true
		return nil
	}
	return walk(s.Root)
}

// MarshalJSON serializa a cena (JSON puro — contrato igual ao do node graph).
func (s *Scene) MarshalJSON() ([]byte, error) {
	type alias Scene
	return json.Marshal((*alias)(s))
}

// Marshal serializa a cena em JSON compacto.
func (s *Scene) Marshal() ([]byte, error) { return s.MarshalJSON() }

// MarshalIndent serializa com indentação (para arquivos de cena).
func (s *Scene) MarshalIndent() ([]byte, error) { return json.MarshalIndent(s, "", "  ") }

// Unmarshal desserializa JSON em uma cena (e valida).
func Unmarshal(data []byte) (*Scene, error) {
	var s Scene
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse scene: %w", err)
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return &s, nil
}
