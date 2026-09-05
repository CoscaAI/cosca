package scene

import (
	"fmt"

	"github.com/CoscaAI/cosca/internal/nodegraph"
)

// Compile é a ponte ENTIDADES → NODE GRAPH — a prova do conceito do Professor
// (L303): "entidade APONTA para operação". Transforma uma cena em um grafo de
// computação executável:
//
//	Scene → Compile → Graph → Run
//
// Para cada entidade com NodeRef não vazio (Type=Procedural, Terrain, ...),
// garante um nó no grafo com Type=NodeRef (o tipo procedural, ex: "fbm") e
// params = cópia dos Params da entidade (que já carregam seed etc. — o que o
// executor do nó vai consumir). Entidades espaciais puras (Camera, Light,
// Group) não viram nós — o Scene Graph não vira um monolito do node graph.
//
// Fail-closed: se o registry não conhecer um NodeRef, erro explícito — nunca
// silencioso. O grafo pode ser novo (g=nil, nome = nome da cena) ou
// aumentado (g dado: nós com ID já existente são preservados — ensure).
func Compile(s *Scene, g *nodegraph.Graph, reg map[nodegraph.NodeType]nodegraph.Executor) (*nodegraph.Graph, error) {
	if s == nil {
		return nil, fmt.Errorf("scene: scene is required")
	}
	if reg == nil {
		return nil, fmt.Errorf("scene: registry is required")
	}
	if g == nil {
		g = nodegraph.New(s.Name)
	}

	var walk func(e *Entity) error
	walk = func(e *Entity) error {
		if e == nil {
			return nil
		}
		if e.NodeRef != "" {
			nt := nodegraph.NodeType(e.NodeRef)
			if _, ok := reg[nt]; !ok {
				return fmt.Errorf("scene: entity %q references unknown node type %q (registry has %d types)", e.ID, e.NodeRef, len(reg))
			}
			if _, exists := g.Node(e.ID); !exists {
				node := &nodegraph.Node{ID: e.ID, Type: nt, Params: cloneParams(e.Params)}
				if err := g.AddNode(node); err != nil {
					return fmt.Errorf("scene: compile entity %q: %w", e.ID, err)
				}
			}
		}
		for _, c := range e.Children {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(s.Root); err != nil {
		return nil, err
	}
	if err := g.Validate(); err != nil {
		return nil, err
	}
	return g, nil
}

// cloneParams copia os Params da entidade para o nó do grafo — o grafo é o
// contrato imutável; a entidade não deve ser mutada depois do Compile.
func cloneParams(params map[string]any) map[string]any {
	if len(params) == 0 {
		return nil
	}
	cp := make(map[string]any, len(params))
	for k, v := range params {
		cp[k] = v
	}
	return cp
}
