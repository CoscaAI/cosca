// Package nodegraph implementa o Node Graph do Cosca (§21 do manifesto
// Creative/Scientific/Media) — Fase 1, etapa 1.6.
//
// Princípio P1 do Blueprint (ComfyUI): "Grafo = dados puros, execução derivada
// e cacheável por assinatura". O grafo é SERIALIZÁVEL (JSON puro), ordenado
// topologicamente, executado por demanda e cacheado por assinatura de inputs.
//
// Node: INPUT → PROCESS/AI → OUTPUT. Exemplos (§21):
//
//	IMAGE → SEGMENT → REMOVE_BG → UPSCALE → EXPORT
//	VIDEO → EXTRACT_AUDIO → WHISPER → TRANSLATE → SUBTITLE → RENDER
//
// O grafo é o contrato entre os produtos (Editor, Image, Cinema...) e as
// engines (tasks, models, gpu, media). Serializável = versionável,
// compartilhável e reproduzível (§33).
package nodegraph

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// NodeType identifica o tipo de operação de um nó (ex: "load_image",
// "segment", "upscale", "export", "whisper", "render").
type NodeType string

// Node é um nó do grafo — dados puros, sem estado de execução.
type Node struct {
	// ID único dentro do grafo.
	ID string `json:"id" yaml:"id"`
	// Type da operação.
	Type NodeType `json:"type" yaml:"type"`
	// Params são os parâmetros do nó (model, seed, path, format...).
	Params map[string]any `json:"params,omitempty" yaml:"params,omitempty"`
	// Inputs são os IDs dos nós que alimentam este nó.
	Inputs []string `json:"inputs,omitempty" yaml:"inputs,omitempty"`
}

// Graph é um node graph serializável.
type Graph struct {
	// Name do grafo (workflow id no Project Manifest §34).
	Name string `json:"name" yaml:"name"`
	// Nodes do grafo.
	Nodes []*Node `json:"nodes" yaml:"nodes"`
	// EngineVersion rastreia a versão do schema (cache invalida nela).
	EngineVersion string `json:"engine_version,omitempty" yaml:"engine_version,omitempty"`
}

// New cria um grafo vazio.
func New(name string) *Graph {
	return &Graph{Name: name, Nodes: []*Node{}, EngineVersion: "1"}
}

// AddNode adiciona um nó (valida ID único).
func (g *Graph) AddNode(n *Node) error {
	if n == nil || strings.TrimSpace(n.ID) == "" {
		return fmt.Errorf("node id is required")
	}
	if strings.TrimSpace(string(n.Type)) == "" {
		return fmt.Errorf("node %q: type is required", n.ID)
	}
	if _, exists := g.nodeByID(n.ID); exists {
		return fmt.Errorf("node %q already exists", n.ID)
	}
	g.Nodes = append(g.Nodes, n)
	return nil
}

func (g *Graph) nodeByID(id string) (*Node, bool) {
	for _, n := range g.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return nil, false
}

// Node returns a node by ID.
func (g *Graph) Node(id string) (*Node, bool) { return g.nodeByID(id) }

// Len devolve o número de nós.
func (g *Graph) Len() int { return len(g.Nodes) }

// Validate verifica os invariantes: IDs únicos, inputs existentes, sem ciclos.
func (g *Graph) Validate() error {
	if strings.TrimSpace(g.Name) == "" {
		return fmt.Errorf("graph name is required")
	}
	seen := make(map[string]bool, len(g.Nodes))
	for _, n := range g.Nodes {
		if seen[n.ID] {
			return fmt.Errorf("duplicate node id %q", n.ID)
		}
		seen[n.ID] = true
		for _, in := range n.Inputs {
			if !seen[in] && !g.nodeExists(in) {
				return fmt.Errorf("node %q references missing input %q", n.ID, in)
			}
		}
	}
	if err := g.detectCycle(); err != nil {
		return err
	}
	return nil
}

func (g *Graph) nodeExists(id string) bool {
	_, ok := g.nodeByID(id)
	return ok
}

// detectCycle usa ordenação topológica (Kahn) — grafo deve ser DAG.
func (g *Graph) detectCycle() error {
	indegree := make(map[string]int, len(g.Nodes))
	adj := make(map[string][]string, len(g.Nodes))
	for _, n := range g.Nodes {
		indegree[n.ID] = 0
	}
	for _, n := range g.Nodes {
		for _, in := range n.Inputs {
			// in → n (dependência: in deve executar antes)
			adj[in] = append(adj[in], n.ID)
			indegree[n.ID]++
		}
	}
	// Kahn
	queue := make([]string, 0)
	for id, d := range indegree {
		if d == 0 {
			queue = append(queue, id)
		}
	}
	visited := 0
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		visited++
		for _, next := range adj[cur] {
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	if visited != len(g.Nodes) {
		return fmt.Errorf("graph contains a cycle (%d of %d nodes reachable)", visited, len(g.Nodes))
	}
	return nil
}

// TopoOrder devolve os nós em ordem topológica (dependências primeiro).
// Estável: nós independentes saem em ordem de inserção.
func (g *Graph) TopoOrder() ([]*Node, error) {
	if err := g.Validate(); err != nil {
		return nil, err
	}
	indegree := make(map[string]int, len(g.Nodes))
	adj := make(map[string][]string, len(g.Nodes))
	byID := make(map[string]*Node, len(g.Nodes))
	for _, n := range g.Nodes {
		indegree[n.ID] = 0
		byID[n.ID] = n
	}
	for _, n := range g.Nodes {
		for _, in := range n.Inputs {
			adj[in] = append(adj[in], n.ID)
			indegree[n.ID]++
		}
	}
	// Fila com ordem de inserção (estabilidade).
	queue := make([]string, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		if indegree[n.ID] == 0 {
			queue = append(queue, n.ID)
		}
	}
	var order []*Node
	processed := 0
	for processed < len(queue) {
		cur := queue[processed]
		processed++
		order = append(order, byID[cur])
		succ := adj[cur]
		sort.Strings(succ) // estabilidade determinística
		for _, next := range succ {
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	if len(order) != len(g.Nodes) {
		return nil, fmt.Errorf("graph contains a cycle")
	}
	return order, nil
}

// =============================================================================
// Cache por assinatura de inputs (P1 do ComfyUI)
// =============================================================================

// Signature calcula a assinatura canônica de um nó: hash de (type + params +
// assinaturas dos inputs). A assinatura recursiva garante que a mudança em
// QUALQUER ancestral invalida o nó (§23 do manifesto — nunca resultado
// obsoleto). É a chave do cache.
func (g *Graph) Signature(nodeID string) (string, error) {
	n, ok := g.nodeByID(nodeID)
	if !ok {
		return "", fmt.Errorf("node %q not found", nodeID)
	}
	return g.signatureOf(n)
}

func (g *Graph) signatureOf(n *Node) (string, error) {
	// Material serializável de forma determinística.
	parts := []string{string(n.Type), canonicalParams(n.Params)}
	for _, in := range n.Inputs {
		child, ok := g.nodeByID(in)
		if !ok {
			return "", fmt.Errorf("node %q references missing input %q", n.ID, in)
		}
		childSig, err := g.signatureOf(child)
		if err != nil {
			return "", err
		}
		parts = append(parts, childSig)
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:]), nil
}

// canonicalParams serializa params de forma determinística (keys ordenadas).
func canonicalParams(params map[string]any) string {
	if len(params) == 0 {
		return ""
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		v, err := json.Marshal(params[k])
		if err != nil {
			v = []byte(fmt.Sprintf("%v", params[k]))
		}
		fmt.Fprintf(&sb, "%s=%s;", k, v)
	}
	return sb.String()
}

// =============================================================================
// Serialização
// =============================================================================

// MarshalJSON serializa o grafo (JSON puro — contract do §21).
func (g *Graph) MarshalJSON() ([]byte, error) {
	type alias Graph
	return json.Marshal((*alias)(g))
}

// Marshal serializa o grafo em JSON compacto.
func (g *Graph) Marshal() ([]byte, error) { return g.MarshalJSON() }

// MarshalIndent serializa com indentação (para arquivos de workflow).
func (g *Graph) MarshalIndent() ([]byte, error) { return json.MarshalIndent(g, "", "  ") }

// Unmarshal desserializa JSON em um grafo.
func Unmarshal(data []byte) (*Graph, error) {
	var g Graph
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("parse node graph: %w", err)
	}
	if err := g.Validate(); err != nil {
		return nil, err
	}
	return &g, nil
}

// Build monta um grafo a partir de nós e valida.
func Build(name string, nodes []*Node) (*Graph, error) {
	g := New(name)
	for _, n := range nodes {
		if err := g.AddNode(n); err != nil {
			return nil, err
		}
	}
	if err := g.Validate(); err != nil {
		return nil, err
	}
	return g, nil
}
