// Package graph provides a knowledge graph implementation for the Cosca Knowledge Engine.
// It stores entities as nodes and their relationships as edges, supporting traversal,
// path-finding, and serialization.
package graph

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
)

// Entity type constants used across the Cosca Knowledge Engine.
const (
	EntityAgent         = "agent"
	EntitySkill         = "skill"
	EntityPrompt        = "prompt"
	EntityWorkflow      = "workflow"
	EntityTemplate      = "template"
	EntityProvider      = "provider"
	EntityPlugin        = "plugin"
	EntityADR           = "adr"
	EntityDoc           = "doc"
	EntitySymbol        = "symbol"
	EntityModule        = "module"
	EntityComponent     = "component"
	EntityAPI           = "api"
	EntityPattern       = "pattern"
	EntityPlaybook      = "playbook"
	EntityRunbook       = "runbook"
	EntityIncident      = "incident"
	EntityBenchmark     = "benchmark"
	EntityReferenceArch = "reference_architecture"
	EntityMemoryRecord  = "memory_record"
	EntityConfig        = "config"
	EntityCapability    = "capability"
	EntityEvidence      = "evidence"
	EntityVulnerability = "vulnerability"
	EntityTest          = "test"
)

// Relationship type constants.
const (
	RelDependsOn      = "depends_on"
	RelExtends        = "extends"
	RelImplements     = "implements"
	RelInvokes        = "invokes"
	RelReferences     = "references"
	RelDefines        = "defines"
	RelContains       = "contains"
	RelRelatedTo      = "related_to"
	RelImports        = "imports"
	RelReportsTo      = "reports_to"
	RelTriggers       = "triggers"
	RelDocuments      = "documents"
	RelSupersedes     = "supersedes"
	RelConsumes       = "consumes"
	RelProduces       = "produces"
	RelDeploysTo      = "deploys_to"
	RelDocumentedIn   = "documented_in"   // entidade → fonte/repo
	RelTestedBy       = "tested_by"       // entidade → teste
	RelAffectedBy     = "affected_by"     // entidade → CVE/vulnerabilidade
	RelContradictedBy = "contradicted_by" // claim → CONFLICT
	RelCaused         = "caused"          // evento causal → consequência (causal graph do trace)
)

// ValidEntityTypes returns all valid entity type constants.
func ValidEntityTypes() []string {
	return []string{
		EntityAgent, EntitySkill, EntityPrompt, EntityWorkflow, EntityTemplate,
		EntityProvider, EntityPlugin, EntityADR, EntityDoc, EntitySymbol,
		EntityModule, EntityComponent, EntityAPI, EntityPattern, EntityPlaybook,
		EntityRunbook, EntityIncident, EntityBenchmark, EntityReferenceArch,
		EntityMemoryRecord, EntityConfig, EntityCapability,
		EntityEvidence, EntityVulnerability, EntityTest,
	}
}

// ValidRelationshipTypes returns all valid relationship type constants.
func ValidRelationshipTypes() []string {
	return []string{
		RelDependsOn, RelExtends, RelImplements, RelInvokes, RelReferences,
		RelDefines, RelContains, RelRelatedTo, RelImports, RelReportsTo,
		RelTriggers, RelDocuments, RelSupersedes, RelConsumes, RelProduces,
		RelDeploysTo, RelDocumentedIn, RelTestedBy, RelAffectedBy,
		RelContradictedBy, RelCaused,
	}
}

// Node represents a single node in the knowledge graph.
type Node struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Name     string                 `json:"name"`
	Label    string                 `json:"label,omitempty"`
	Path     string                 `json:"path,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Edge represents a directed edge between two nodes.
type Edge struct {
	Source   string                 `json:"source"`
	Target   string                 `json:"target"`
	Type     string                 `json:"type"`
	Weight   float64                `json:"weight"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Graph represents an in-memory directed knowledge graph with thread-safe operations.
type Graph struct {
	mu    sync.RWMutex
	nodes map[string]*Node
	edges map[string][]*Edge // source -> outgoing edges

	// adjacency stores incoming edges for reverse traversal
	inEdges map[string][]*Edge // target -> incoming edges

	// dirty tracks mutation since last MarkClean: saveGraph persiste o grafo
	// para SQL apenas quando houve mudança (busca é read-only — Close ficava
	// re-gravando 13k entidades sem nada ter mudado, ~1.8s por comando CLI).
	dirty bool
}

// New creates a new empty knowledge graph.
func New() *Graph {
	return &Graph{
		nodes:   make(map[string]*Node),
		edges:   make(map[string][]*Edge),
		inEdges: make(map[string][]*Edge),
	}
}

// AddNode adds a node to the graph.
func (g *Graph) AddNode(node *Node) error {
	if node == nil {
		return fmt.Errorf("node is nil")
	}
	if node.ID == "" {
		return fmt.Errorf("node ID is required")
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if node.Metadata == nil {
		node.Metadata = make(map[string]interface{})
	}

	g.nodes[node.ID] = node
	return nil
}

// RemoveNode removes a node and all its incident edges.
func (g *Graph) RemoveNode(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.nodes[id]; !exists {
		return fmt.Errorf("node %s not found", id)
	}

	delete(g.nodes, id)
	delete(g.edges, id)
	delete(g.inEdges, id)

	// Remove edges that reference this node
	for source, edgeList := range g.edges {
		filtered := make([]*Edge, 0, len(edgeList))
		for _, e := range edgeList {
			if e.Target != id {
				filtered = append(filtered, e)
			}
		}
		g.edges[source] = filtered
	}

	// Remove incoming edges
	for target, edgeList := range g.inEdges {
		filtered := make([]*Edge, 0, len(edgeList))
		for _, e := range edgeList {
			if e.Source != id {
				filtered = append(filtered, e)
			}
		}
		g.inEdges[target] = filtered
	}
	g.dirty = true

	return nil
}

// AddEdge adds a directed edge to the graph.
func (g *Graph) AddEdge(edge *Edge) error {
	if edge == nil {
		return fmt.Errorf("edge is nil")
	}
	if edge.Source == "" || edge.Target == "" {
		return fmt.Errorf("edge source and target are required")
	}
	if edge.Type == "" {
		edge.Type = RelRelatedTo
	}
	if edge.Weight == 0 {
		edge.Weight = 1.0
	}
	if edge.Metadata == nil {
		edge.Metadata = make(map[string]interface{})
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// Auto-create missing nodes as stubs
	if _, exists := g.nodes[edge.Source]; !exists {
		g.nodes[edge.Source] = &Node{ID: edge.Source, Type: "unknown", Name: edge.Source, Metadata: make(map[string]interface{})}
	}
	if _, exists := g.nodes[edge.Target]; !exists {
		g.nodes[edge.Target] = &Node{ID: edge.Target, Type: "unknown", Name: edge.Target, Metadata: make(map[string]interface{})}
	}

	g.edges[edge.Source] = append(g.edges[edge.Source], edge)
	g.inEdges[edge.Target] = append(g.inEdges[edge.Target], edge)
	g.dirty = true

	return nil
}

// RemoveEdge removes an edge from the graph.
func (g *Graph) RemoveEdge(source, target, edgeType string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	removed := false

	// Remove from outgoing
	if edges, ok := g.edges[source]; ok {
		filtered := make([]*Edge, 0, len(edges))
		for _, e := range edges {
			if e.Target != target || e.Type != edgeType {
				filtered = append(filtered, e)
			} else {
				removed = true
			}
		}
		g.edges[source] = filtered
	}

	// Remove from incoming
	if edges, ok := g.inEdges[target]; ok {
		filtered := make([]*Edge, 0, len(edges))
		for _, e := range edges {
			if e.Source != source || e.Type != edgeType {
				filtered = append(filtered, e)
			}
		}
		g.inEdges[target] = filtered
	}
	if removed {
		g.dirty = true
	}

	if !removed {
		return fmt.Errorf("edge not found: %s -> %s (%s)", source, target, edgeType)
	}
	return nil
}

// GetNode retrieves a node by ID.
func (g *Graph) GetNode(id string) (*Node, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	node, ok := g.nodes[id]
	if !ok {
		return nil, false
	}
	return node, true
}

// GetNeighbors returns all nodes directly reachable from the given node.
func (g *Graph) GetNeighbors(id string) ([]*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, exists := g.nodes[id]; !exists {
		return nil, fmt.Errorf("node %s not found", id)
	}

	var neighbors []*Node
	seen := make(map[string]bool)

	for _, edge := range g.edges[id] {
		if !seen[edge.Target] {
			if node, ok := g.nodes[edge.Target]; ok {
				neighbors = append(neighbors, node)
				seen[edge.Target] = true
			}
		}
	}

	return neighbors, nil
}

// GetIncoming returns all nodes that have edges pointing to the given node.
func (g *Graph) GetIncoming(id string) ([]*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, exists := g.nodes[id]; !exists {
		return nil, fmt.Errorf("node %s not found", id)
	}

	var nodes []*Node
	seen := make(map[string]bool)

	for _, edge := range g.inEdges[id] {
		if !seen[edge.Source] {
			if node, ok := g.nodes[edge.Source]; ok {
				nodes = append(nodes, node)
				seen[edge.Source] = true
			}
		}
	}

	return nodes, nil
}

// GetEdges returns all edges from the given source node.
func (g *Graph) GetEdges(source string) ([]*Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, exists := g.nodes[source]; !exists {
		return nil, fmt.Errorf("node %s not found", source)
	}

	edges := g.edges[source]
	result := make([]*Edge, len(edges))
	copy(result, edges)
	return result, nil
}

// GetAllEdges returns a copy of all edges in the graph.
func (g *Graph) GetAllEdges() []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var all []*Edge
	for _, edgeList := range g.edges {
		all = append(all, edgeList...)
	}
	return all
}

// ShortestPath finds the shortest path between two nodes using BFS.
// Returns the sequence of node IDs from source to target.
func (g *Graph) ShortestPath(source, target string) ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, exists := g.nodes[source]; !exists {
		return nil, fmt.Errorf("source node %s not found", source)
	}
	if _, exists := g.nodes[target]; !exists {
		return nil, fmt.Errorf("target node %s not found", target)
	}
	if source == target {
		return []string{source}, nil
	}

	// BFS
	queue := []string{source}
	visited := map[string]bool{source: true}
	parent := map[string]string{}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, edge := range g.edges[current] {
			if !visited[edge.Target] {
				visited[edge.Target] = true
				parent[edge.Target] = current
				queue = append(queue, edge.Target)

				if edge.Target == target {
					// Reconstruct path
					path := []string{target}
					for at := target; at != source; at = parent[at] {
						path = append([]string{parent[at]}, path...)
					}
					return path, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no path from %s to %s", source, target)
}

// BFS performs a breadth-first traversal starting from the given node.
func (g *Graph) BFS(start string, maxDepth int) ([]*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, exists := g.nodes[start]; !exists {
		return nil, fmt.Errorf("node %s not found", start)
	}

	if maxDepth <= 0 {
		maxDepth = 10
	}

	type queueItem struct {
		node  string
		depth int
	}

	queue := []queueItem{{node: start, depth: 0}}
	visited := map[string]bool{start: true}
	var result []*Node

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if node, ok := g.nodes[item.node]; ok {
			result = append(result, node)
		}

		if item.depth >= maxDepth {
			continue
		}

		for _, edge := range g.edges[item.node] {
			if !visited[edge.Target] {
				visited[edge.Target] = true
				queue = append(queue, queueItem{node: edge.Target, depth: item.depth + 1})
			}
		}
	}

	return result, nil
}

// DFS performs a depth-first traversal starting from the given node.
func (g *Graph) DFS(start string, maxDepth int) ([]*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, exists := g.nodes[start]; !exists {
		return nil, fmt.Errorf("node %s not found", start)
	}

	if maxDepth <= 0 {
		maxDepth = 10
	}

	type stackItem struct {
		node  string
		depth int
	}

	stack := []stackItem{{node: start, depth: 0}}
	visited := map[string]bool{start: true}
	var result []*Node

	for len(stack) > 0 {
		item := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if node, ok := g.nodes[item.node]; ok {
			result = append(result, node)
		}

		if item.depth >= maxDepth {
			continue
		}

		for _, edge := range g.edges[item.node] {
			if !visited[edge.Target] {
				visited[edge.Target] = true
				stack = append(stack, stackItem{node: edge.Target, depth: item.depth + 1})
			}
		}
	}

	return result, nil
}

// FilterNodes returns all nodes matching the given type.
func (g *Graph) FilterNodes(entityType string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var result []*Node
	for _, node := range g.nodes {
		if node.Type == entityType {
			result = append(result, node)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// GetAllNodes returns a slice of all nodes in the graph (unsorted).
func (g *Graph) GetAllNodes() []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]*Node, 0, len(g.nodes))
	for _, node := range g.nodes {
		result = append(result, node)
	}
	return result
}

// FindNodesByType returns all nodes of a given entity type.
func (g *Graph) FindNodesByType(entityType string) []*Node {
	return g.FilterNodes(entityType)
}

// Stats returns graph statistics.
func (g *Graph) Stats() GraphStats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodeCount := len(g.nodes)
	edgeCount := 0
	typeCounts := make(map[string]int)

	for _, node := range g.nodes {
		typeCounts[node.Type]++
	}

	for _, edgeList := range g.edges {
		edgeCount += len(edgeList)
	}

	// Calculate density
	density := 0.0
	if nodeCount > 1 {
		maxEdges := nodeCount * (nodeCount - 1)
		if maxEdges > 0 {
			density = float64(edgeCount) / float64(maxEdges)
		}
	}

	// Count connected components
	components := g.countComponents()

	return GraphStats{
		Nodes:      nodeCount,
		Edges:      edgeCount,
		TypeCounts: typeCounts,
		Density:    density,
		Components: components,
		IsEmpty:    nodeCount == 0,
	}
}

// countComponents counts the number of weakly connected components.
func (g *Graph) countComponents() int {
	visited := make(map[string]bool)
	components := 0

	for id := range g.nodes {
		if visited[id] {
			continue
		}
		components++

		// BFS to mark all nodes in this component
		queue := []string{id}
		visited[id] = true
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]

			for _, edge := range g.edges[current] {
				if !visited[edge.Target] {
					visited[edge.Target] = true
					queue = append(queue, edge.Target)
				}
			}
			for _, edge := range g.inEdges[current] {
				if !visited[edge.Source] {
					visited[edge.Source] = true
					queue = append(queue, edge.Source)
				}
			}
		}
	}

	return components
}

// GetNodeCount returns the total number of nodes.
func (g *Graph) GetNodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}

// GetEdgeCount returns the total number of edges.
func (g *Graph) GetEdgeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	count := 0
	for _, edgeList := range g.edges {
		count += len(edgeList)
	}
	return count
}

// GraphStats holds statistics about the graph.
//
//nolint:revive // Stutter name preserved for API compatibility — used as graph.GraphStats externally.
type GraphStats struct {
	Nodes      int            `json:"nodes"`
	Edges      int            `json:"edges"`
	TypeCounts map[string]int `json:"type_counts"`
	Density    float64        `json:"density"`
	Components int            `json:"components"`
	IsEmpty    bool           `json:"is_empty"`
}

// Serialize exports the graph to JSON bytes.
func (g *Graph) Serialize() ([]byte, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	export := struct {
		Nodes []*Node `json:"nodes"`
		Edges []*Edge `json:"edges"`
	}{
		Nodes: make([]*Node, 0, len(g.nodes)),
		Edges: make([]*Edge, 0),
	}

	for _, node := range g.nodes {
		export.Nodes = append(export.Nodes, node)
	}

	for _, edgeList := range g.edges {
		export.Edges = append(export.Edges, edgeList...)
	}

	return json.Marshal(export)
}

// Deserialize imports a graph from JSON bytes.
func Deserialize(data []byte) (*Graph, error) {
	imported := struct {
		Nodes []*Node `json:"nodes"`
		Edges []*Edge `json:"edges"`
	}{}

	if err := json.Unmarshal(data, &imported); err != nil {
		return nil, fmt.Errorf("unmarshal graph: %w", err)
	}

	g := New()
	for _, node := range imported.Nodes {
		if err := g.AddNode(node); err != nil {
			return nil, fmt.Errorf("add node %s: %w", node.ID, err)
		}
	}
	for _, edge := range imported.Edges {
		if err := g.AddEdge(edge); err != nil {
			return nil, fmt.Errorf("add edge: %w", err)
		}
	}

	return g, nil
}

// String returns a compact string representation of graph stats.
func (g *Graph) String() string {
	stats := g.Stats()
	return fmt.Sprintf("Graph{nodes:%d edges:%d components:%d density:%.4f}",
		stats.Nodes, stats.Edges, stats.Components, stats.Density)
}

// Dijkstra finds the shortest weighted path between two nodes.
// Uses edge weights as distances (lower is closer).
func (g *Graph) Dijkstra(source, target string) ([]string, float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, exists := g.nodes[source]; !exists {
		return nil, 0, fmt.Errorf("source node %s not found", source)
	}
	if _, exists := g.nodes[target]; !exists {
		return nil, 0, fmt.Errorf("target node %s not found", target)
	}
	if source == target {
		return []string{source}, 0, nil
	}

	dist := make(map[string]float64)
	prev := make(map[string]string)
	unvisited := make(map[string]bool)

	for id := range g.nodes {
		dist[id] = math.Inf(1)
		unvisited[id] = true
	}
	dist[source] = 0

	for len(unvisited) > 0 {
		// Find unvisited node with smallest distance
		var current string
		minDist := math.Inf(1)
		for id := range unvisited {
			if dist[id] < minDist {
				minDist = dist[id]
				current = id
			}
		}

		if current == "" || math.IsInf(minDist, 1) {
			break
		}

		if current == target {
			break
		}

		delete(unvisited, current)

		for _, edge := range g.edges[current] {
			if !unvisited[edge.Target] {
				continue
			}
			weight := edge.Weight
			if weight <= 0 {
				weight = 1
			}
			newDist := dist[current] + weight
			if newDist < dist[edge.Target] {
				dist[edge.Target] = newDist
				prev[edge.Target] = current
			}
		}
	}

	if math.IsInf(dist[target], 1) {
		return nil, 0, fmt.Errorf("no path from %s to %s", source, target)
	}

	path := []string{target}
	for at := target; at != source; at = prev[at] {
		path = append([]string{prev[at]}, path...)
	}

	return path, dist[target], nil
}

// Subgraph returns a subgraph containing only nodes of the specified types.
func (g *Graph) Subgraph(entityTypes ...string) *Graph {
	g.mu.RLock()
	defer g.mu.RUnlock()

	typeSet := make(map[string]bool)
	for _, t := range entityTypes {
		typeSet[t] = true
	}

	sub := New()

	for _, node := range g.nodes {
		if len(typeSet) == 0 || typeSet[node.Type] {
			sub.nodes[node.ID] = &Node{
				ID: node.ID, Type: node.Type, Name: node.Name,
				Label: node.Label, Path: node.Path, Metadata: node.Metadata,
			}
		}
	}

	for source, edgeList := range g.edges {
		if _, ok := sub.nodes[source]; !ok {
			continue
		}
		for _, edge := range edgeList {
			if _, ok := sub.nodes[edge.Target]; ok {
				sub.edges[source] = append(sub.edges[source], edge)
				sub.inEdges[edge.Target] = append(sub.inEdges[edge.Target], edge)
			}
		}
	}

	return sub
}


// IsDirty reports whether the graph was mutated since the last MarkClean.
func (g *Graph) IsDirty() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.dirty
}

// MarkClean clears the mutation flag (called after persisting).
func (g *Graph) MarkClean() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.dirty = false
}

