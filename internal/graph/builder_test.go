package graph

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/chunker"
	"github.com/CoscaAI/cosca/internal/markdown"
	"github.com/CoscaAI/cosca/internal/parser"
)

func TestBuilderBasics(t *testing.T) {
	g := New()
	b := NewBuilder(g)
	if b.Graph() != g {
		t.Fatal("Graph() mismatch")
	}
}

func TestBuildFromEntities(t *testing.T) {
	g := New()
	b := NewBuilder(g)

	entities := []parser.Entity{
		{
			ID: "agent:backend", Type: parser.EntityType("agent"), Name: "Backend",
			Path: "departments/backend/SKILL.md", Description: "handles APIs",
			Relationships: []parser.Relationship{
				{TargetID: "skill:api", Type: parser.RelationshipType("can_execute"), Weight: 1.0},
			},
		},
		{ID: "skill:api", Type: parser.EntityType("skill"), Name: "API Design"},
	}
	if err := b.BuildFromEntities(entities); err != nil {
		t.Fatalf("BuildFromEntities: %v", err)
	}

	node, ok := g.GetNode("agent:backend")
	if !ok {
		t.Fatal("node not added")
	}
	if node.Name != "Backend" || node.Type != "agent" {
		t.Fatalf("node: %+v", node)
	}
	if node.Metadata["description"] != "handles APIs" {
		t.Fatalf("metadata: %+v", node.Metadata)
	}
	// Relationship edge added.
	neighbors, err := g.GetNeighbors("agent:backend")
	if err != nil || len(neighbors) != 1 || neighbors[0].ID != "skill:api" {
		t.Fatalf("neighbors: %v, %v", neighbors, err)
	}
}

func TestBuildFromDocumentNilGuard(t *testing.T) {
	g := New()
	b := NewBuilder(g)
	if err := b.BuildFromDocument(nil, nil); err == nil {
		t.Fatal("nil doc must error")
	}
}

func TestBuildFromDocumentAddsNode(t *testing.T) {
	g := New()
	b := NewBuilder(g)
	doc := &markdown.Document{Path: "docs/readme.md", Title: "Readme", TokenCount: 42}
	if err := b.BuildFromDocument(doc, parser.NewEntityParser()); err != nil {
		t.Fatalf("BuildFromDocument: %v", err)
	}
	// The real EntityParser synthesizes an entity from the path, so a node
	// carrying the document path must exist.
	found := false
	for _, n := range g.GetAllNodes() {
		if n.Path == "docs/readme.md" {
			found = true
		}
	}
	if !found {
		t.Fatal("no node for document path")
	}
}

func TestBuildFromChunks(t *testing.T) {
	g := New()
	b := NewBuilder(g)
	chunks := []chunker.Chunk{
		{ID: "chunk-1", Position: 1, SectionType: "text", Heading: "Intro", TokenCount: 10},
		{ID: "chunk-2", Position: 2, SectionType: "code", Heading: "Code", TokenCount: 20},
	}
	if err := b.BuildFromChunks(chunks, "doc-12345678", "docs/a.md"); err != nil {
		t.Fatalf("BuildFromChunks: %v", err)
	}

	if _, ok := g.GetNode("doc-12345678"); !ok {
		t.Fatal("document node missing")
	}
	if _, ok := g.GetNode("chunk-1"); !ok {
		t.Fatal("chunk node missing")
	}
	neighbors, _ := g.GetNeighbors("doc-12345678")
	if len(neighbors) != 2 {
		t.Fatalf("chunk neighbors = %d", len(neighbors))
	}
}

func TestExtractCrossReferences(t *testing.T) {
	g := New()
	b := NewBuilder(g)
	// Two doc nodes.
	_ = b.BuildFromEntities([]parser.Entity{
		{ID: "doc-a", Type: parser.EntityType("document"), Name: "A", Path: "docs/a.md"},
		{ID: "doc-b", Type: parser.EntityType("document"), Name: "B", Path: "docs/b.md"},
	})
	docs := map[string]*markdown.Document{
		"docs/a.md": {Path: "docs/a.md", Links: []markdown.Link{{Text: "to b", URL: "b.md"}}},
	}
	if err := b.ExtractCrossReferences(docs); err != nil {
		t.Fatalf("ExtractCrossReferences: %v", err)
	}
	neighbors, _ := g.GetNeighbors("doc-a")
	if len(neighbors) != 1 || neighbors[0].ID != "doc-b" {
		t.Fatalf("cross-ref neighbors: %v", neighbors)
	}
}

func TestExtractDependencies(t *testing.T) {
	g := New()
	b := NewBuilder(g)
	_ = b.BuildFromEntities([]parser.Entity{
		{ID: "mod-a", Type: parser.EntityType("module"), Name: "module-a", Path: "src/a/main.go"},
		{ID: "mod-b", Type: parser.EntityType("module"), Name: "module-b", Path: "src/b/main.go"},
	})
	doc := &markdown.Document{
		Path: "src/a/main.go",
		Frontmatter: markdown.Frontmatter{
			Data: map[string]interface{}{"depends_on": []interface{}{"module-b"}},
		},
	}
	if err := b.ExtractDependencies("src/a/main.go", doc); err != nil {
		t.Fatalf("ExtractDependencies: %v", err)
	}
	neighbors, _ := g.GetNeighbors("mod-a")
	if len(neighbors) != 1 || neighbors[0].ID != "mod-b" {
		t.Fatalf("dep neighbors: %v", neighbors)
	}
}

func TestExtractCodeImports(t *testing.T) {
	g := New()
	b := NewBuilder(g)
	_ = b.BuildFromEntities([]parser.Entity{
		{ID: "main", Type: parser.EntityType("file"), Name: "main", Path: "main.go"},
		// The node path must contain the import path for the substring match.
		{ID: "lib", Type: parser.EntityType("file"), Name: "lib", Path: "github.com/CoscaAI/cosca/internal/lib/lib.go"},
	})
	content := "package main\n\nimport (\n\t\"github.com/CoscaAI/cosca/internal/lib\"\n)\n"
	if err := b.ExtractCodeImports("main.go", content); err != nil {
		t.Fatalf("ExtractCodeImports: %v", err)
	}
	neighbors, _ := g.GetNeighbors("main")
	if len(neighbors) != 1 || neighbors[0].ID != "lib" {
		t.Fatalf("import neighbors: %v", neighbors)
	}

	// No import block → no-op.
	if err := b.ExtractCodeImports("main.go", "package main\n"); err != nil {
		t.Fatal(err)
	}
}

func TestIncrementalUpdateAndRemove(t *testing.T) {
	g := New()
	b := NewBuilder(g)
	_ = b.BuildFromEntities([]parser.Entity{
		{ID: "n1", Type: parser.EntityType("module"), Name: "n1", Path: "src/x.go"},
	})
	if err := b.RemoveDocument("src/x.go"); err != nil {
		t.Fatalf("RemoveDocument: %v", err)
	}
	if _, ok := g.GetNode("n1"); ok {
		t.Fatal("node should be removed")
	}

	// IncrementalUpdate on a path with no node → rebuilds.
	doc := &markdown.Document{Path: "src/y.go", Title: "Y"}
	if err := b.IncrementalUpdate("src/y.go", doc, parser.NewEntityParser()); err != nil {
		t.Fatalf("IncrementalUpdate: %v", err)
	}
	found := false
	for _, n := range g.nodes {
		if n.Path == "src/y.go" {
			found = true
		}
	}
	if !found {
		t.Fatal("incremental update did not add node")
	}
}

func TestClear(t *testing.T) {
	g := New()
	b := NewBuilder(g)
	_ = b.BuildFromEntities([]parser.Entity{{ID: "a", Type: parser.EntityType("module"), Name: "a"}})
	b.Clear()
	if len(g.nodes) != 0 || len(g.edges) != 0 {
		t.Fatal("graph not cleared")
	}
}

func TestResolveDependency(t *testing.T) {
	g := New()
	b := NewBuilder(g)
	_ = b.BuildFromEntities([]parser.Entity{
		{ID: "x", Type: parser.EntityType("module"), Name: "module-x", Path: "src/modules/x/main.go"},
	})
	if b.resolveDependency("module-x") != "x" { // exact name
		t.Fatal("exact name")
	}
	if b.resolveDependency("src/modules/x/main.go") != "x" { // exact path
		t.Fatal("exact path")
	}
	if b.resolveDependency("modules/x/main.go") != "x" { // path suffix
		t.Fatal("path suffix")
	}
	if b.resolveDependency("main") != "x" { // base name — hmm: node path base is "main"? path src/modules/x/main.go → base "main" → matches "main"
		t.Fatal("base name")
	}
	if b.resolveDependency("ghost") != "" {
		t.Fatal("no match")
	}
}

func TestResolveLinkPath(t *testing.T) {
	if got := resolveLinkPath("docs/a.md", "b.md"); got != "docs/b.md" {
		t.Fatalf("relative: %q", got)
	}
	if got := resolveLinkPath("docs/a.md", "/abs.md"); got != "/abs.md" {
		t.Fatalf("absolute: %q", got)
	}
	if got := resolveLinkPath("docs/a.md", "https://x.com/y"); got != "https://x.com/y" {
		t.Fatalf("http: %q", got)
	}
	if got := resolveLinkPath("docs/a.md", "../README.md"); got != "README.md" {
		t.Fatalf("up: %q", got)
	}
}

func TestToStringSlice(t *testing.T) {
	if got := toStringSlice("a,b"); len(got) != 2 || got[0] != "a" {
		t.Fatalf("string: %v", got)
	}
	if got := toStringSlice([]interface{}{"x", 42}); len(got) != 2 || got[1] != "42" {
		t.Fatalf("iface slice: %v", got)
	}
	if got := toStringSlice([]string{"z"}); len(got) != 1 || got[0] != "z" {
		t.Fatalf("string slice: %v", got)
	}
	if got := toStringSlice(123); got != nil {
		t.Fatalf("default: %v", got)
	}
}
