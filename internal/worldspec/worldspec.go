// Package worldspec é o contrato canônico de mundo (ADR-024) — o ponto de solda
// entre o COSCA (cérebro) e os engines (corpos). Compõe os tipos existentes
// (reusa gameengine.Entity como ECS canônico), NÃO duplica.
//
// Decisão (ADR-024 §1.2): estender o ECS do gameengine (declarativo/JSON) com
// terrain/navigation/environment/simulation/provenance. O COSCA emite "este é o
// mundo que quero representar"; o adapter materializa no corpo (Unreal/Roblox/Blender).
//
// Aditivo e avaliaível: não altera world/scene/gameengine.
package worldspec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/CoscaAI/cosca/internal/gameengine"
	"github.com/CoscaAI/cosca/internal/scene"
	"github.com/CoscaAI/cosca/internal/world"
)

// ──────────────────────────────────────────────────────────────
// WorldSpec (ADR-024, Fase 1) — contrato canônico.
// ──────────────────────────────────────────────────────────────

// WorldSpec é a representação determinística de "este é o mundo que quero representar".
type WorldSpec struct {
	SchemaVersion string `json:"schemaVersion"`
	WorldID       string `json:"worldId"`
	Seed          int64  `json:"seed"`
	Name          string `json:"name"`
	WorldType     string `json:"worldType"`

	// Entities reusa o ECS do gameengine (ID/Name/Components) — decisão ADR-024.
	Entities []gameengine.Entity `json:"entities"`

	Terrain     []TerrainRegion `json:"terrain"`
	Relations   []Relation      `json:"relations"`
	Navigation  Navigation      `json:"navigation"`
	Environment Environment     `json:"environment"`
	Simulation  Simulation      `json:"simulation"`
	Provenance  Provenance      `json:"provenance"`

	// Adapters são dicas por target de engine (o corpo materializa conforme).
	Adapters map[string]map[string]any `json:"adapters,omitempty"`
}

// TerrainRegion é uma região do terreno.
type TerrainRegion struct {
	ID        string       `json:"id"`
	Type      string       `json:"type"`
	Polygon   [][2]float64 `json:"polygon"`
	Elevation float64      `json:"elevation"`
	Biome     string       `json:"biome"`
}

// Relation é uma relação semântica entre duas entidades/nós.
type Relation struct {
	Subject string `json:"subject"`
	Object  string `json:"object"`
	Type    string `json:"type"`
}

// Navigation é o grafo navegável.
type Navigation struct {
	Nodes []NavNode `json:"nodes"`
	Edges []NavEdge `json:"edges"`
}

// NavNode é um nó do grafo de navegação.
type NavNode struct {
	ID   string     `json:"id"`
	Pos  [2]float64 `json:"pos"`
	Kind string     `json:"kind"`
}

// NavEdge é uma aresta do grafo.
type NavEdge struct {
	From string  `json:"from"`
	To   string  `json:"to"`
	Type string  `json:"type"`
	Cost float64 `json:"cost"`
}

// Environment é o estado de clima/tempo.
type Environment struct {
	Weather   string  `json:"weather"`
	TimeOfDay float64 `json:"time_of_day"`
	Season    string  `json:"season"`
}

// Simulation é o metadado de runtime (física/tick).
type Simulation struct {
	Mode    string `json:"mode"`
	Hz      int    `json:"hz"`
	Physics string `json:"physics"`
}

// Provenance é a proveniência obrigatória do mundo (I3/I4).
type Provenance struct {
	Source string         `json:"source"`
	Class  string         `json:"class"`
	Tool   string         `json:"tool"`
	Seed   int64          `json:"seed"`
	Params map[string]any `json:"params,omitempty"`
}

// ──────────────────────────────────────────────────────────────
// Construção e conversão (compõe os backbones existentes)
// ──────────────────────────────────────────────────────────────

// New cria um WorldSpec vazio com o schema canônico.
func New(seed int64, name, worldType string) *WorldSpec {
	return &WorldSpec{
		SchemaVersion: "1.0",
		Seed:          seed,
		Name:          name,
		WorldType:     worldType,
	}
}

// FromWorld converte o kernel semântico (internal/world) para o contrato canônico.
// O world.World já carrega proveniência/relations/ambiente; achatamos cada entidade
// rica em um Entity ECS com um componente "world:<type>" preservando as properties.
func FromWorld(w *world.World, seed int64) *WorldSpec {
	if w == nil {
		return nil
	}
	s := New(seed, w.WorldID, "open_world")
	s.WorldID = w.WorldID
	for _, e := range w.Entities {
		comp := gameengine.Component{
			Type: gameengine.ComponentType("world:" + string(e.Type)),
			Params: map[string]any{
				"properties": e.Properties,
			},
		}
		s.Entities = append(s.Entities, gameengine.Entity{
			ID:         e.ID,
			Name:       e.ID,
			Components: []gameengine.Component{comp},
		})
	}
	return s
}

// FromGameEngine converte uma cena ECS do gameengine para o contrato canônico.
func FromGameEngine(g *gameengine.Scene, seed int64) *WorldSpec {
	if g == nil {
		return nil
	}
	s := New(seed, g.Name, "game")
	s.WorldID = g.Name
	for _, e := range g.Entities {
		if e != nil {
			s.Entities = append(s.Entities, *e)
		}
	}
	return s
}

// FromScene converte o grafo visual (internal/scene) achatando a raiz em entidades ECS.
func FromScene(sc *scene.Scene, seed int64) *WorldSpec {
	if sc == nil {
		return nil
	}
	s := New(seed, sc.Name, "scene")
	s.WorldID = sc.Name
	var visit func(e *scene.Entity)
	visit = func(e *scene.Entity) {
		if e == nil {
			return
		}
		comp := gameengine.Component{
			Type: gameengine.ComponentType("scene:" + string(e.Type)),
			Params: map[string]any{
				"transform": e.Transform,
				"params":    e.Params,
			},
		}
		s.Entities = append(s.Entities, gameengine.Entity{
			ID:         e.ID,
			Name:       e.Name,
			Components: []gameengine.Component{comp},
		})
		for _, c := range e.Children {
			visit(c)
		}
	}
	visit(sc.Root)
	return s
}

// ──────────────────────────────────────────────────────────────
// Persistência e validação
// ──────────────────────────────────────────────────────────────

// JSON devolve a serialização indentada (determinística).
func (s *WorldSpec) JSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

// Hash devolve um hash de conteúdo do mundo (para versionamento/idempotência, I5).
func (s *WorldSpec) Hash() string {
	b, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])[:16]
}

// Validate verifica as invariantes do contrato (I1/I4):
// schema version presente; seed definido (determinismo); proveniência presente
// e com classe válida. Fail-closed (I2): retorna erro, nunca "conserta".
func (s *WorldSpec) Validate() error {
	if s == nil {
		return fmt.Errorf("worldspec: nil")
	}
	if s.SchemaVersion == "" {
		return fmt.Errorf("worldspec: schemaVersion obrigatório")
	}
	if s.Seed == 0 {
		return fmt.Errorf("worldspec: seed obrigatório (determinismo I1)")
	}
	if s.Provenance.Class == "" {
		return fmt.Errorf("worldspec: provenance.class obrigatório (I4 — nunca inventar)")
	}
	switch s.Provenance.Class {
	case "generated", "simulated", "observed", "calculated", "hypothesis":
	default:
		return fmt.Errorf("worldspec: class %q inválida", s.Provenance.Class)
	}
	return nil
}
