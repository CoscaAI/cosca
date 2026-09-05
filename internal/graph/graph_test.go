package graph

import (
	"testing"
)

func TestNew(t *testing.T) {
	t.Parallel()
	g := New()
	if g == nil {
		t.Fatal("New() returned nil")
	}
	if g.GetNodeCount() != 0 {
		t.Errorf("GetNodeCount = %d, want 0", g.GetNodeCount())
	}
	if g.GetEdgeCount() != 0 {
		t.Errorf("GetEdgeCount = %d, want 0", g.GetEdgeCount())
	}
}

func TestAddNode(t *testing.T) {
	t.Parallel()
	g := New()
	err := g.AddNode(&Node{ID: "node1", Type: "agent", Name: "Agent1"})
	if err != nil {
		t.Fatalf("AddNode error: %v", err)
	}
	if g.GetNodeCount() != 1 {
		t.Errorf("GetNodeCount = %d", g.GetNodeCount())
	}
}

func TestAddNodeDuplicate(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddNode(&Node{ID: "n1", Type: "agent", Name: "A"})
	err := g.AddNode(&Node{ID: "n1", Type: "agent", Name: "A"})
	if err != nil {
		t.Fatalf("AddNode duplicate should overwrite, got error: %v", err)
	}
}

func TestAddNodeNil(t *testing.T) {
	t.Parallel()
	g := New()
	err := g.AddNode(nil)
	if err == nil {
		t.Error("Expected error for nil node")
	}
}

func TestAddNodeEmptyID(t *testing.T) {
	t.Parallel()
	g := New()
	err := g.AddNode(&Node{Type: "agent", Name: "NoID"})
	if err == nil {
		t.Error("Expected error for empty ID")
	}
}

func TestGetNode(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddNode(&Node{ID: "n1", Type: "skill", Name: "Search"})
	node, ok := g.GetNode("n1")
	if !ok {
		t.Fatal("GetNode should return ok")
	}
	if node.Name != "Search" {
		t.Errorf("Name = %q", node.Name)
	}
	if node.Type != "skill" {
		t.Errorf("Type = %q", node.Type)
	}
}

func TestGetNodeNotFound(t *testing.T) {
	t.Parallel()
	g := New()
	_, ok := g.GetNode("nonexistent")
	if ok {
		t.Error("GetNode should return false for nonexistent node")
	}
}

func TestRemoveNode(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddNode(&Node{ID: "n1", Type: "agent", Name: "A"})
	err := g.RemoveNode("n1")
	if err != nil {
		t.Fatalf("RemoveNode error: %v", err)
	}
	if g.GetNodeCount() != 0 {
		t.Errorf("GetNodeCount = %d", g.GetNodeCount())
	}
}

func TestRemoveNodeNotFound(t *testing.T) {
	t.Parallel()
	g := New()
	err := g.RemoveNode("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent node")
	}
}

func TestAddEdge(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddNode(&Node{ID: "a", Type: "agent", Name: "A"})
	_ = g.AddNode(&Node{ID: "b", Type: "skill", Name: "B"})
	err := g.AddEdge(&Edge{Source: "a", Target: "b", Type: RelDependsOn})
	if err != nil {
		t.Fatalf("AddEdge error: %v", err)
	}
	if g.GetEdgeCount() != 1 {
		t.Errorf("GetEdgeCount = %d", g.GetEdgeCount())
	}
}

func TestAddEdgeAutoCreateNodes(t *testing.T) {
	t.Parallel()
	g := New()
	err := g.AddEdge(&Edge{Source: "auto-src", Target: "auto-tgt", Type: RelRelatedTo})
	if err != nil {
		t.Fatalf("AddEdge auto-create error: %v", err)
	}
	if g.GetNodeCount() != 2 {
		t.Errorf("GetNodeCount = %d, want 2", g.GetNodeCount())
	}
}

func TestAddEdgeNil(t *testing.T) {
	t.Parallel()
	g := New()
	err := g.AddEdge(nil)
	if err == nil {
		t.Error("Expected error for nil edge")
	}
}

func TestAddEdgeEmptySource(t *testing.T) {
	t.Parallel()
	g := New()
	err := g.AddEdge(&Edge{Source: "", Target: "t", Type: RelReferences})
	if err == nil {
		t.Error("Expected error for empty source")
	}
}

func TestRemoveEdge(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddEdge(&Edge{Source: "a", Target: "b", Type: RelReferences})
	err := g.RemoveEdge("a", "b", RelReferences)
	if err != nil {
		t.Fatalf("RemoveEdge error: %v", err)
	}
	if g.GetEdgeCount() != 0 {
		t.Errorf("GetEdgeCount = %d", g.GetEdgeCount())
	}
}

func TestRemoveEdgeNotFound(t *testing.T) {
	t.Parallel()
	g := New()
	err := g.RemoveEdge("x", "y", RelContains)
	if err == nil {
		t.Error("Expected error for nonexistent edge")
	}
}

func TestGetNeighbors(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddEdge(&Edge{Source: "a", Target: "b", Type: RelDependsOn})
	_ = g.AddEdge(&Edge{Source: "a", Target: "c", Type: RelDependsOn})
	neighbors, err := g.GetNeighbors("a")
	if err != nil {
		t.Fatalf("GetNeighbors error: %v", err)
	}
	if len(neighbors) != 2 {
		t.Errorf("neighbors len = %d, want 2", len(neighbors))
	}
}

func TestGetNeighborsNotFound(t *testing.T) {
	t.Parallel()
	g := New()
	_, err := g.GetNeighbors("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent node")
	}
}

func TestGetIncoming(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddEdge(&Edge{Source: "a", Target: "b", Type: RelReferences})
	_ = g.AddEdge(&Edge{Source: "c", Target: "b", Type: RelReferences})
	nodes, err := g.GetIncoming("b")
	if err != nil {
		t.Fatalf("GetIncoming error: %v", err)
	}
	if len(nodes) != 2 {
		t.Errorf("incoming len = %d, want 2", len(nodes))
	}
}

func TestFilterNodes(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddNode(&Node{ID: "a1", Type: "agent", Name: "Agent1"})
	_ = g.AddNode(&Node{ID: "a2", Type: "agent", Name: "Agent2"})
	_ = g.AddNode(&Node{ID: "s1", Type: "skill", Name: "Skill1"})
	agents := g.FilterNodes("agent")
	if len(agents) != 2 {
		t.Errorf("agent count = %d, want 2", len(agents))
	}
}

func TestFindNodesByType(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddNode(&Node{ID: "s1", Type: "skill", Name: "S1"})
	skills := g.FindNodesByType("skill")
	if len(skills) != 1 {
		t.Errorf("skill count = %d", len(skills))
	}
}

func TestShortestPath(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddEdge(&Edge{Source: "a", Target: "b", Type: RelReferences})
	_ = g.AddEdge(&Edge{Source: "b", Target: "c", Type: RelReferences})
	path, err := g.ShortestPath("a", "c")
	if err != nil {
		t.Fatalf("ShortestPath error: %v", err)
	}
	if len(path) != 3 {
		t.Errorf("path len = %d, want 3", len(path))
	}
	if path[0] != "a" || path[1] != "b" || path[2] != "c" {
		t.Errorf("path = %v", path)
	}
}

func TestShortestPathSameNode(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddNode(&Node{ID: "x", Type: "agent", Name: "X"})
	path, err := g.ShortestPath("x", "x")
	if err != nil {
		t.Fatalf("ShortestPath same node error: %v", err)
	}
	if len(path) != 1 || path[0] != "x" {
		t.Errorf("path = %v", path)
	}
}

func TestShortestPathNoPath(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddNode(&Node{ID: "a", Type: "agent", Name: "A"})
	_ = g.AddNode(&Node{ID: "b", Type: "agent", Name: "B"})
	_, err := g.ShortestPath("a", "b")
	if err == nil {
		t.Error("Expected error for no path")
	}
}

func TestBFS(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddEdge(&Edge{Source: "root", Target: "child1", Type: RelContains})
	_ = g.AddEdge(&Edge{Source: "root", Target: "child2", Type: RelContains})
	nodes, err := g.BFS("root", 2)
	if err != nil {
		t.Fatalf("BFS error: %v", err)
	}
	if len(nodes) < 3 {
		t.Errorf("BFS nodes = %d, want >= 3", len(nodes))
	}
}

func TestDFS(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddEdge(&Edge{Source: "a", Target: "b", Type: RelDependsOn})
	_ = g.AddEdge(&Edge{Source: "b", Target: "c", Type: RelDependsOn})
	nodes, err := g.DFS("a", 3)
	if err != nil {
		t.Fatalf("DFS error: %v", err)
	}
	if len(nodes) < 3 {
		t.Errorf("DFS nodes = %d", len(nodes))
	}
}

func TestStats(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddEdge(&Edge{Source: "a", Target: "b", Type: RelDependsOn})
	_ = g.AddEdge(&Edge{Source: "b", Target: "c", Type: RelDependsOn})
	stats := g.Stats()
	if stats.Nodes != 3 {
		t.Errorf("Nodes = %d", stats.Nodes)
	}
	if stats.Edges != 2 {
		t.Errorf("Edges = %d", stats.Edges)
	}
	if stats.IsEmpty {
		t.Error("IsEmpty should be false")
	}
}

func TestStatsEmpty(t *testing.T) {
	t.Parallel()
	g := New()
	stats := g.Stats()
	if !stats.IsEmpty {
		t.Error("IsEmpty should be true for empty graph")
	}
}

func TestGetEdges(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddEdge(&Edge{Source: "a", Target: "b", Type: RelDependsOn})
	edges, err := g.GetEdges("a")
	if err != nil {
		t.Fatalf("GetEdges error: %v", err)
	}
	if len(edges) != 1 {
		t.Errorf("edges len = %d", len(edges))
	}
}

func TestGetAllEdges(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddEdge(&Edge{Source: "a", Target: "b", Type: RelDependsOn})
	_ = g.AddEdge(&Edge{Source: "c", Target: "d", Type: RelReferences})
	all := g.GetAllEdges()
	if len(all) != 2 {
		t.Errorf("all edges len = %d", len(all))
	}
}

func TestSubgraph(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddNode(&Node{ID: "a1", Type: "agent", Name: "A1"})
	_ = g.AddNode(&Node{ID: "s1", Type: "skill", Name: "S1"})
	sub := g.Subgraph("agent")
	if sub.GetNodeCount() != 1 {
		t.Errorf("subgraph node count = %d", sub.GetNodeCount())
	}
}

func TestSerialization(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddNode(&Node{ID: "n1", Type: "agent", Name: "Agent"})
	_ = g.AddEdge(&Edge{Source: "n1", Target: "n2", Type: RelDependsOn})

	data, err := g.Serialize()
	if err != nil {
		t.Fatalf("Serialize error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("serialized data is empty")
	}

	loaded, err := Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize error: %v", err)
	}
	if loaded.GetNodeCount() != g.GetNodeCount() {
		t.Errorf("loaded node count = %d, want %d", loaded.GetNodeCount(), g.GetNodeCount())
	}
}

func TestString(t *testing.T) {
	t.Parallel()
	g := New()
	s := g.String()
	if s == "" {
		t.Error("String should not be empty")
	}
}

func TestValidEntityTypes(t *testing.T) {
	t.Parallel()
	types := ValidEntityTypes()
	if len(types) == 0 {
		t.Error("ValidEntityTypes should return at least 1 type")
	}
}

func TestValidRelationshipTypes(t *testing.T) {
	t.Parallel()
	types := ValidRelationshipTypes()
	if len(types) == 0 {
		t.Error("ValidRelationshipTypes should return at least 1 type")
	}
}

func TestValidRelationshipTypes_EvidenceTypes(t *testing.T) {
	t.Parallel()
	types := ValidRelationshipTypes()
	want := map[string]bool{
		RelDocumentedIn:   false,
		RelTestedBy:       false,
		RelAffectedBy:     false,
		RelContradictedBy: false,
	}
	for _, typ := range types {
		if _, ok := want[typ]; ok {
			want[typ] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("ValidRelationshipTypes should include %q", name)
		}
	}
}

func TestValidEntityTypes_EvidenceTypes(t *testing.T) {
	t.Parallel()
	types := ValidEntityTypes()
	want := map[string]bool{
		EntityEvidence:      false,
		EntityVulnerability: false,
		EntityTest:          false,
	}
	for _, typ := range types {
		if _, ok := want[typ]; ok {
			want[typ] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("ValidEntityTypes should include %q", name)
		}
	}
}

func TestAddEdge_EvidenceTypes(t *testing.T) {
	t.Parallel()
	g := New()
	edges := []*Edge{
		{Source: "Bubblewrap", Target: "repo-A", Type: RelDocumentedIn},
		{Source: "Bubblewrap", Target: "test-B", Type: RelTestedBy},
		{Source: "Bubblewrap", Target: "CVE-2024-2947", Type: RelAffectedBy, Weight: 0.9},
		{Source: "Bubblewrap", Target: "CONFLICT-102", Type: RelContradictedBy},
	}
	for _, e := range edges {
		if err := g.AddEdge(e); err != nil {
			t.Fatalf("AddEdge(%s) error: %v", e.Type, err)
		}
	}
	if got := g.GetEdgeCount(); got != 4 {
		t.Errorf("GetEdgeCount = %d, want 4", got)
	}
	got, err := g.GetEdges("Bubblewrap")
	if err != nil {
		t.Fatalf("GetEdges error: %v", err)
	}
	byType := make(map[string]bool)
	for _, e := range got {
		byType[e.Type] = true
	}
	for _, typ := range []string{RelDocumentedIn, RelTestedBy, RelAffectedBy, RelContradictedBy} {
		if !byType[typ] {
			t.Errorf("GetEdges should include type %q", typ)
		}
	}
}

func TestEvidenceMapQuery(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddEdge(&Edge{Source: "Bubblewrap", Target: "namespaces", Type: RelImplements, Weight: 0.8})
	_ = g.AddEdge(&Edge{Source: "Bubblewrap", Target: "Linux", Type: RelDependsOn})
	_ = g.AddEdge(&Edge{Source: "Bubblewrap", Target: "CVE-2024-2947", Type: RelAffectedBy})
	_ = g.AddEdge(&Edge{Source: "Bubblewrap", Target: "repo-A", Type: RelDocumentedIn})
	_ = g.AddEdge(&Edge{Source: "Bubblewrap", Target: "test-B", Type: RelTestedBy})
	_ = g.AddEdge(&Edge{Source: "Bubblewrap", Target: "CONFLICT-102", Type: RelContradictedBy})

	edges, err := g.GetEdges("Bubblewrap")
	if err != nil {
		t.Fatalf("GetEdges error: %v", err)
	}
	if len(edges) != 6 {
		t.Errorf("evidence map edges = %d, want 6", len(edges))
	}
	byType := make(map[string]int)
	for _, e := range edges {
		byType[e.Type]++
	}
	for _, typ := range []string{RelImplements, RelDependsOn, RelAffectedBy, RelDocumentedIn, RelTestedBy, RelContradictedBy} {
		if byType[typ] != 1 {
			t.Errorf("evidence map should have exactly 1 %q edge, got %d", typ, byType[typ])
		}
	}

	incoming, err := g.GetIncoming("CONFLICT-102")
	if err != nil {
		t.Fatalf("GetIncoming error: %v", err)
	}
	if len(incoming) != 1 || incoming[0].ID != "Bubblewrap" {
		t.Errorf("contradicted_by incoming = %v, want [Bubblewrap]", incoming)
	}
}

func TestEvidenceGraphSerializeRoundTrip(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddEdge(&Edge{Source: "Bubblewrap", Target: "CVE-2024-2947", Type: RelAffectedBy, Weight: 0.9})
	_ = g.AddEdge(&Edge{Source: "Bubblewrap", Target: "CONFLICT-102", Type: RelContradictedBy})

	data, err := g.Serialize()
	if err != nil {
		t.Fatalf("Serialize error: %v", err)
	}
	loaded, err := Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize error: %v", err)
	}
	if loaded.GetEdgeCount() != 2 {
		t.Errorf("round-trip edge count = %d, want 2", loaded.GetEdgeCount())
	}
	got, err := loaded.GetEdges("Bubblewrap")
	if err != nil {
		t.Fatalf("GetEdges error: %v", err)
	}
	if len(got) != 2 || got[0].Type != RelAffectedBy || got[0].Weight != 0.9 {
		t.Errorf("round-trip evidence edge mismatch: %+v", got)
	}
}

func TestDijkstra(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddEdge(&Edge{Source: "a", Target: "b", Type: RelReferences, Weight: 1})
	_ = g.AddEdge(&Edge{Source: "b", Target: "c", Type: RelReferences, Weight: 2})
	path, dist, err := g.Dijkstra("a", "c")
	if err != nil {
		t.Fatalf("Dijkstra error: %v", err)
	}
	if len(path) != 3 {
		t.Errorf("path len = %d", len(path))
	}
	if dist != 3 {
		t.Errorf("dist = %f, want 3", dist)
	}
}

func TestDijkstraNoPath(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddNode(&Node{ID: "a", Type: "agent", Name: "A"})
	_ = g.AddNode(&Node{ID: "b", Type: "agent", Name: "B"})
	_, _, err := g.Dijkstra("a", "b")
	if err == nil {
		t.Error("Expected error for no path")
	}
}

func TestDijkstraSameNode(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddNode(&Node{ID: "a", Type: "agent", Name: "A"})
	path, dist, err := g.Dijkstra("a", "a")
	if err != nil {
		t.Fatalf("Dijkstra same node error: %v", err)
	}
	if len(path) != 1 {
		t.Errorf("path len = %d", len(path))
	}
	if dist != 0 {
		t.Errorf("dist = %f", dist)
	}
}

func TestNodeDefaultMetadata(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddNode(&Node{ID: "n1", Type: "agent", Name: "Test"})
	node, _ := g.GetNode("n1")
	if node.Metadata == nil {
		t.Error("Metadata should be initialized")
	}
}

func TestEdgeDefaultValues(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddEdge(&Edge{Source: "a", Target: "b"})
	edges, _ := g.GetEdges("a")
	if len(edges) == 0 {
		t.Fatal("no edges found")
	}
	if edges[0].Type != RelRelatedTo {
		t.Errorf("default Type = %q, want %q", edges[0].Type, RelRelatedTo)
	}
	if edges[0].Weight != 1.0 {
		t.Errorf("default Weight = %f", edges[0].Weight)
	}
}

func TestSubgraph_NoMatchingType(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddNode(&Node{ID: "a", Type: "agent", Name: "A"})
	sub := g.Subgraph("skill")
	if sub.GetNodeCount() != 0 {
		t.Errorf("subgraph should be empty for no matches, got %d", sub.GetNodeCount())
	}
}

func TestDijkstra_StartNodeNotFound(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddNode(&Node{ID: "b", Type: "agent", Name: "B"})
	_, _, err := g.Dijkstra("nonexistent", "b")
	if err == nil {
		t.Error("expected error for unknown start node")
	}
}

func TestDijkstra_TargetNodeNotFound(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddNode(&Node{ID: "a", Type: "agent", Name: "A"})
	_, _, err := g.Dijkstra("a", "nonexistent")
	if err == nil {
		t.Error("expected error for unknown target node")
	}
}

func TestRemoveEdge_LastEdgeOfNode(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddEdge(&Edge{Source: "a", Target: "b", Type: RelDependsOn})
	if err := g.RemoveEdge("a", "b", RelDependsOn); err != nil {
		t.Fatalf("RemoveEdge: %v", err)
	}
	// Node "a" should have zero outgoing edges
	edges, err := g.GetEdges("a")
	if err != nil {
		t.Fatalf("GetEdges: %v", err)
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges for a, got %d", len(edges))
	}
}

func TestBFS_StartNodeNotFound(t *testing.T) {
	t.Parallel()
	g := New()
	_, err := g.BFS("nonexistent", 2)
	if err == nil {
		t.Error("expected error for unknown BFS start node")
	}
}

func TestDFS_StartNodeNotFound(t *testing.T) {
	t.Parallel()
	g := New()
	_, err := g.DFS("nonexistent", 3)
	if err == nil {
		t.Error("expected error for unknown DFS start node")
	}
}

func TestShortestPath_StartNodeNotFound(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddNode(&Node{ID: "b", Type: "agent", Name: "B"})
	_, err := g.ShortestPath("nonexistent", "b")
	if err == nil {
		t.Error("expected error for unknown start node in shortest path")
	}
}

func TestShortestPath_TargetNodeNotFound(t *testing.T) {
	t.Parallel()
	g := New()
	_ = g.AddNode(&Node{ID: "a", Type: "agent", Name: "A"})
	_, err := g.ShortestPath("a", "nonexistent")
	if err == nil {
		t.Error("expected error for unknown target node in shortest path")
	}
}

func TestGetNeighbors_EmptyGraph(t *testing.T) {
	t.Parallel()
	g := New()
	// Non-existent node in empty graph returns error
	_, err := g.GetNeighbors("x")
	if err == nil {
		t.Error("expected error for non-existent node in empty graph")
	}
}
