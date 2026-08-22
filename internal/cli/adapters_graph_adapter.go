package cli

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/graph"

	_ "modernc.org/sqlite"
)

// Graph Adapter
// =============================================================================

// graphAdapter wraps graph.Graph to add methods CLI commands expect.
type graphAdapter struct {
	inner *graph.Graph
	scope string // "local" | "global" | "" (sem fonte)
}

// Scope: origem dos dados do grafo ("local", "global" ou "").
func (a *graphAdapter) Scope() string { return a.scope }

// newGraphAdapter creates a new graph adapter, populating the in-memory graph
// from the project knowledge.db when available (entities + relationships).
// The CLI was showing 0 nodes while the SQLite cofre had 9k+ entities — o grafo
// precisa espelhar o que está persistido (L335).
// Escopo: local (.cosca/knowledge.db) primeiro; se não existir, cai no
// global (~/.config/cosca/knowledge.db) — "o global ta zero" (L338).
func newGraphAdapter(dir string) *graphAdapter {
	a := &graphAdapter{inner: graph.New()}
	if dir == "" {
		return a
	}
	dbPath := filepath.Join(dir, ".cosca", "knowledge.db")
	scope := "local"
	if _, err := os.Stat(dbPath); err != nil {
		// fallback global: conhecimento compartilhado entre projetos
		home, err := os.UserHomeDir()
		if err != nil {
			return a
		}
		global := filepath.Join(home, ".config", "cosca", "knowledge.db")
		if _, err := os.Stat(global); err != nil {
			return a
		}
		dbPath = global
		scope = "global"
	}
	a.scope = scope
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return a
	}
	defer db.Close()

	// Entidades → nós
	rows, err := db.Query(`SELECT id, entity_type, name, path FROM entities`)
	if err != nil {
		return a
	}
	for rows.Next() {
		var id, etype, name, path string
		if err := rows.Scan(&id, &etype, &name, &path); err != nil {
			continue
		}
		_ = a.inner.AddNode(&graph.Node{ID: id, Type: etype, Name: name, Path: path})
	}
	rows.Close()

	// Relações → arestas (source_id → target_id)
	relRows, err := db.Query(`SELECT source_id, target_id, rel_type, weight FROM relationships`)
	if err != nil {
		return a
	}
	for relRows.Next() {
		var src, tgt, rel string
		var weight float64
		if err := relRows.Scan(&src, &tgt, &rel, &weight); err != nil {
			continue
		}
		_ = a.inner.AddEdge(&graph.Edge{Source: src, Target: tgt, Type: rel, Weight: weight})
	}
	relRows.Close()

	return a
}

// Query performs a graph query. CLI expects this method but graph.Graph doesn't have it.
func (a *graphAdapter) Query(entity string, depth int) ([]GraphQueryResult, error) {
	if a.inner == nil {
		return nil, nil
	}
	// Resolve nome/path → ID: os nós do knowledge.db usam UUIDs como ID,
	// mas o usuário busca pelo nome da entidade (ex.: "cosca-kernel",
	// "bug-008-metrics-misdocumented.md").
	startID := entity
	if _, ok := a.inner.GetNode(entity); !ok {
		for _, n := range a.inner.GetAllNodes() {
			if n.Name == entity || n.Path == entity || n.Type == entity {
				startID = n.ID
				break
			}
		}
	}
	if _, ok := a.inner.GetNode(startID); !ok {
		return nil, fmt.Errorf("entity %q not found in knowledge graph", entity)
	}
	// Use BFS to traverse from the entity node
	nodes, err := a.inner.BFS(startID, depth)
	if err != nil {
		return nil, err
	}
	var results []GraphQueryResult
	for _, n := range nodes {
		edges, _ := a.inner.GetEdges(n.ID)
		for _, e := range edges {
			src, dst := e.Source, e.Target
			if sn, ok := a.inner.GetNode(e.Source); ok && sn.Name != "" {
				src = sn.Name
			}
			if dn, ok := a.inner.GetNode(e.Target); ok && dn.Name != "" {
				dst = dn.Name
			}
			results = append(results, GraphQueryResult{
				Source:       src,
				Target:       dst,
				Relationship: e.Type,
			})
		}
	}
	return results, nil
}

// GraphQueryResult holds a single graph query result.
type GraphQueryResult struct {
	Source       string `json:"source"`
	Target       string `json:"target"`
	Relationship string `json:"relationship"`
}

// Export exports the graph. CLI expects Export(exportPath, format) error.
// Real graph has Serialize() for JSON only.
func (a *graphAdapter) Export(exportPath, format string) error {
	if a.inner == nil {
		return fmt.Errorf("graph not available")
	}
	data, err := a.inner.Serialize()
	if err != nil {
		return fmt.Errorf("serialize graph: %w", err)
	}
	switch format {
	case "json":
		return os.WriteFile(exportPath, data, 0644)
	case "graphml":
		// Convert to GraphML (simplified)
		graphml := convertToGraphML(a.inner)
		return os.WriteFile(exportPath, []byte(graphml), 0644)
	case "dot":
		// Convert to DOT format (simplified)
		dot := convertToDOT(a.inner)
		return os.WriteFile(exportPath, []byte(dot), 0644)
	default:
		return fmt.Errorf("unsupported export format: %s", format)
	}
}

// Stats returns graph statistics. CLI expects (GraphStats, error).
// Real Stats() returns GraphStats directly.
func (a *graphAdapter) Stats() (GraphStatsExtended, error) {
	if a.inner == nil {
		return GraphStatsExtended{}, fmt.Errorf("graph not available")
	}
	s := a.inner.Stats()
	return GraphStatsExtended{
		TotalNodes:           s.Nodes,
		TotalEdges:           s.Edges,
		AvgDegree:            avgDegree(s.Nodes, s.Edges),
		Density:              s.Density,
		Components:           s.Components,
		LargestComponentSize: s.Nodes, // approximate
		GraphSize:            fmt.Sprintf("%d nodes, %d edges", s.Nodes, s.Edges),
		LastBuilt:            time.Now(),
	}, nil
}

// GraphStatsExtended holds detailed graph statistics as expected by CLI.
type GraphStatsExtended struct {
	TotalNodes           int       `json:"total_nodes"`
	TotalEdges           int       `json:"total_edges"`
	AvgDegree            float64   `json:"avg_degree"`
	Density              float64   `json:"density"`
	Components           int       `json:"components"`
	LargestComponentSize int       `json:"largest_component_size"`
	GraphSize            string    `json:"graph_size"`
	LastBuilt            time.Time `json:"last_built"`
}

func avgDegree(nodes, edges int) float64 {
	if nodes == 0 {
		return 0
	}
	return float64(edges) / float64(nodes)
}

// Overview returns graph overview data as expected by graph.go CLI commands.
func (a *graphAdapter) Overview() (GraphOverview, error) {
	if a.inner == nil {
		return GraphOverview{}, fmt.Errorf("graph not available")
	}
	s := a.inner.Stats()

	// Build entity type breakdown
	typeCounts := make(map[string]int)
	for _, node := range a.inner.FilterNodes("") {
		typeCounts[node.Type]++
	}
	relTypeSet := make(map[string]int)
	for _, edge := range a.inner.GetAllEdges() {
		relTypeSet[edge.Type]++
	}
	var relTypes []struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	for name, count := range relTypeSet {
		relTypes = append(relTypes, struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		}{Name: name, Count: count})
	}

	var entityTypes []struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	for typ, count := range typeCounts {
		entityTypes = append(entityTypes, struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		}{Name: typ, Count: count})
	}

	return GraphOverview{
		Nodes:             s.Nodes,
		Edges:             s.Edges,
		EntityTypes:       entityTypes,
		RelationshipTypes: relTypes,
	}, nil
}

// GraphOverview holds graph overview data.
type GraphOverview struct {
	Nodes       int `json:"nodes"`
	Edges       int `json:"edges"`
	EntityTypes []struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	} `json:"entity_types"`
	RelationshipTypes []struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	} `json:"relationship_types"`
}

// Build builds the graph from scratch. Placeholder for install flow.
func (a *graphAdapter) Build() error {
	return nil
}

// Update updates the graph with changes. Placeholder for sync flow.
func (a *graphAdapter) Update(_ interface{}) error {
	return nil
}

// GetRelations returns relationships for an entity. Used by knowledge relations command.
func (a *graphAdapter) GetRelations(entity string, depth int) ([]GraphRelation, error) {
	if a.inner == nil {
		return nil, nil
	}
	nodes, err := a.inner.BFS(entity, depth)
	if err != nil {
		return nil, err
	}
	var relations []GraphRelation
	seen := make(map[string]bool)
	for _, n := range nodes {
		edges, err := a.inner.GetEdges(n.ID)
		if err != nil {
			continue
		}
		for _, e := range edges {
			key := e.Source + "->" + e.Target + ":" + e.Type
			if seen[key] {
				continue
			}
			seen[key] = true
			relations = append(relations, GraphRelation{
				Source: e.Source,
				Target: e.Target,
				Type:   e.Type,
			})
		}
	}
	return relations, nil
}

// GraphRelation holds a single relationship between two entities.
type GraphRelation struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

// --- Graph format converters ---

func convertToGraphML(g *graph.Graph) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<graphml xmlns="http://graphml.graphdrawing.org/xmlns">` + "\n")
	b.WriteString(`  <graph id="G" edgedefault="directed">` + "\n")
	for _, node := range g.FilterNodes("") {
		fmt.Fprintf(&b, `    <node id="%s"/>`+"\n", node.ID)
	}
	for _, edge := range g.GetAllEdges() {
		fmt.Fprintf(&b, `    <edge source="%s" target="%s"/>`+"\n", edge.Source, edge.Target)
	}
	b.WriteString("  </graph>\n")
	b.WriteString("</graphml>\n")
	return b.String()
}

func convertToDOT(g *graph.Graph) string {
	var b strings.Builder
	b.WriteString("digraph G {\n")
	for _, edge := range g.GetAllEdges() {
		fmt.Fprintf(&b, "  %q -> %q;\n", edge.Source, edge.Target)
	}
	b.WriteString("}\n")
	return b.String()
}

// =============================================================================
