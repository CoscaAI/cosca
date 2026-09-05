// Package graph provides the graph builder that constructs the knowledge graph
// from indexed documents, entities, and their relationships.
package graph

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/CoscaAI/cosca/internal/chunker"
	"github.com/CoscaAI/cosca/internal/markdown"
	"github.com/CoscaAI/cosca/internal/parser"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Builder constructs and maintains the knowledge graph from indexed content.
type Builder struct {
	graph *Graph
	mu    sync.RWMutex
}

// NewBuilder creates a new graph builder.
func NewBuilder(g *Graph) *Builder {
	return &Builder{
		graph: g,
	}
}

// Graph returns the underlying graph.
func (b *Builder) Graph() *Graph {
	return b.graph
}

// BuildFromEntities adds parsed entities and their relationships to the graph.
func (b *Builder) BuildFromEntities(entities []parser.Entity) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, entity := range entities {
		node := &Node{
			ID:   entity.ID,
			Type: string(entity.Type),
			Name: entity.Name,
			Path: entity.Path,
			Metadata: map[string]interface{}{
				"description": entity.Description,
				"version":     entity.Version,
				"status":      entity.Status,
				"owner":       entity.Owner,
			},
		}

		// Copy entity metadata
		for k, v := range entity.Metadata {
			if node.Metadata == nil {
				node.Metadata = make(map[string]interface{})
			}
			node.Metadata[k] = v
		}

		if err := b.graph.AddNode(node); err != nil {
			log.Warn().Err(err).Str("entity", entity.Name).Msg("failed to add graph node")
			continue
		}

		// Add relationships
		for _, rel := range entity.Relationships {
			edge := &Edge{
				Source:   entity.ID,
				Target:   rel.TargetID,
				Type:     string(rel.Type),
				Weight:   rel.Weight,
				Metadata: rel.Metadata,
			}
			if edge.Source == "" {
				edge.Source = entity.ID
			}
			if edge.Target == "" {
				continue
			}
			if err := b.graph.AddEdge(edge); err != nil {
				log.Warn().Err(err).
					Str("source", edge.Source).
					Str("target", edge.Target).
					Msg("failed to add graph edge")
			}
		}
	}

	return nil
}

// BuildFromDocument parses a markdown document and adds its entities to the graph.
func (b *Builder) BuildFromDocument(doc *markdown.Document, entityParser *parser.EntityParser) error {
	if doc == nil || entityParser == nil {
		return fmt.Errorf("document and entity parser are required")
	}

	// Parse entities from document
	entities, err := entityParser.ParseFile(doc.Path, doc.RawContent)
	if err != nil {
		return fmt.Errorf("parse entities: %w", err)
	}

	if len(entities) == 0 {
		// Create a generic document node
		node := &Node{
			ID:   uuid.New().String(),
			Type: EntityDoc,
			Name: doc.Title,
			Path: doc.Path,
			Metadata: map[string]interface{}{
				"token_count": doc.TokenCount,
				"headings":    len(doc.Headings),
				"links":       len(doc.Links),
			},
		}
		return b.graph.AddNode(node)
	}

	return b.BuildFromEntities(entities)
}

// BuildFromChunks adds chunk-based document structure nodes to the graph.
func (b *Builder) BuildFromChunks(chunks []chunker.Chunk, documentID, documentPath string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Ensure document node exists
	if _, exists := b.graph.nodes[documentID]; !exists {
		b.graph.nodes[documentID] = &Node{
			ID:   documentID,
			Type: EntityDoc,
			Name: filepath.Base(documentPath),
			Path: documentPath,
			Metadata: map[string]interface{}{
				"chunk_count": len(chunks),
			},
		}
	}

	for _, chunk := range chunks {
		chunkNode := &Node{
			ID:   chunk.ID,
			Type: "chunk",
			Name: fmt.Sprintf("%s chunk %d", documentID[:8], chunk.Position),
			Metadata: map[string]interface{}{
				"section_type": chunk.SectionType,
				"heading":      chunk.Heading,
				"position":     chunk.Position,
				"token_count":  chunk.TokenCount,
			},
		}
		if err := b.graph.AddNode(chunkNode); err != nil {
			continue
		}

		// Link chunk to document
		b.graph.edges[documentID] = append(b.graph.edges[documentID], &Edge{
			Source: documentID,
			Target: chunk.ID,
			Type:   RelContains,
			Weight: 1.0,
		})
		b.graph.inEdges[chunk.ID] = append(b.graph.inEdges[chunk.ID], &Edge{
			Source: documentID,
			Target: chunk.ID,
			Type:   RelContains,
			Weight: 1.0,
		})
	}

	b.graph.MarkDirty() // mutação direta abaixo do AddNode — marca para persistência

	return nil
}

// ExtractCrossReferences finds cross-references between documents and adds
// reference relationships to the graph.
func (b *Builder) ExtractCrossReferences(docs map[string]*markdown.Document) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Map of path -> node ID for resolving references
	pathToID := make(map[string]string)
	for _, node := range b.graph.nodes {
		if node.Path != "" {
			pathToID[node.Path] = node.ID
		}
	}

	for path, doc := range docs {
		sourceID, sourceExists := pathToID[path]
		if !sourceExists {
			continue
		}

		// Extract links from document
		for _, link := range doc.Links {
			targetPath := resolveLinkPath(path, link.URL)
			if targetID, ok := pathToID[targetPath]; ok {
				edge := &Edge{
					Source: sourceID,
					Target: targetID,
					Type:   RelReferences,
					Weight: 1.0,
					Metadata: map[string]interface{}{
						"link_text": link.Text,
					},
				}
				b.graph.edges[sourceID] = append(b.graph.edges[sourceID], edge)
				b.graph.inEdges[targetID] = append(b.graph.inEdges[targetID], edge)
			}
		}
	}

	b.graph.MarkDirty() // mutação direta de edges/inEdges — marca para persistência

	return nil
}

// ExtractDependencies finds dependency declarations in frontmatter and adds
// dependency relationships to the graph.
func (b *Builder) ExtractDependencies(path string, doc *markdown.Document) error {
	if doc == nil || doc.Frontmatter.Data == nil {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Find the node for this path
	var sourceID string
	for _, node := range b.graph.nodes {
		if node.Path == path {
			sourceID = node.ID
			break
		}
	}
	if sourceID == "" {
		return nil
	}

	// Extract dependencies from frontmatter
	for key, val := range doc.Frontmatter.Data {
		keyLower := strings.ToLower(key)

		if keyLower == "depends_on" || keyLower == "dependencies" {
			deps := toStringSlice(val)
			for _, dep := range deps {
				dep = strings.TrimSpace(dep)
				if dep == "" {
					continue
				}
				// Try to resolve the dependency as a path or name
				targetID := b.resolveDependency(dep)
				if targetID != "" {
					edge := &Edge{
						Source: sourceID,
						Target: targetID,
						Type:   RelDependsOn,
						Weight: 1.0,
						Metadata: map[string]interface{}{
							"dependency": dep,
						},
					}
					b.graph.edges[sourceID] = append(b.graph.edges[sourceID], edge)
					b.graph.inEdges[targetID] = append(b.graph.inEdges[targetID], edge)
				}
			}
		}
	}

	b.graph.MarkDirty() // mutação direta de edges/inEdges — marca para persistência

	return nil
}

// ExtractCodeImports finds Go import statements and adds them as relationships.
func (b *Builder) ExtractCodeImports(path string, content string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	var sourceID string
	for _, node := range b.graph.nodes {
		if node.Path == path {
			sourceID = node.ID
			break
		}
	}
	if sourceID == "" {
		return nil
	}

	// Match Go import patterns. Needs (?s) (dotall): the import block spans
	// multiple lines, and without it the lazy (.*?) never crosses a newline,
	// so multi-line import blocks were never matched.
	importRe := regexp.MustCompile(`(?ms)^\s*import\s+\((.*?)\)`)
	importMatches := importRe.FindStringSubmatch(content)
	if len(importMatches) < 2 {
		return nil
	}

	// Match individual import paths
	pathRe := regexp.MustCompile(`"([^"]+)"`)
	pathMatches := pathRe.FindAllStringSubmatch(importMatches[1], -1)

	for _, match := range pathMatches {
		if len(match) < 2 {
			continue
		}
		importPath := match[1]

		// Try to find a node matching this import
		for _, node := range b.graph.nodes {
			if strings.Contains(node.Path, importPath) || strings.Contains(node.Name, importPath) {
				edge := &Edge{
					Source: sourceID,
					Target: node.ID,
					Type:   RelImports,
					Weight: 1.0,
					Metadata: map[string]interface{}{
						"import_path": importPath,
					},
				}
				b.graph.edges[sourceID] = append(b.graph.edges[sourceID], edge)
				b.graph.inEdges[node.ID] = append(b.graph.inEdges[node.ID], edge)
			}
		}
	}

	b.graph.MarkDirty() // mutação direta de edges/inEdges — marca para persistência

	return nil
}

// IncrementalUpdate updates the graph when a single document changes.
// It removes old nodes/edges for the document and re-adds them.
func (b *Builder) IncrementalUpdate(path string, doc *markdown.Document, entityParser *parser.EntityParser) error {
	// Remove existing nodes for this path
	b.mu.Lock()
	for id, node := range b.graph.nodes {
		if node.Path == path {
			delete(b.graph.nodes, id)
			delete(b.graph.edges, id)
			delete(b.graph.inEdges, id)
		}
	}

	// Also clean up edges referencing removed nodes
	for source, edgeList := range b.graph.edges {
		filtered := make([]*Edge, 0, len(edgeList))
		for _, e := range edgeList {
			if _, exists := b.graph.nodes[e.Source]; exists && b.graph.nodes[e.Target] != nil {
				// Check if target still exists
				if _, ok := b.graph.nodes[e.Target]; ok {
					filtered = append(filtered, e)
				}
			}
		}
		b.graph.edges[source] = filtered
	}
	b.mu.Unlock()
	b.graph.MarkDirty() // remoção direta de nodes/edges — marca para persistência

	// Re-add
	return b.BuildFromDocument(doc, entityParser)
}

// RemoveDocument removes all graph nodes associated with a document path.
func (b *Builder) RemoveDocument(path string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	idsToRemove := make(map[string]bool)
	for id, node := range b.graph.nodes {
		if node.Path == path {
			idsToRemove[id] = true
		}
	}

	for id := range idsToRemove {
		delete(b.graph.nodes, id)
		delete(b.graph.edges, id)
		delete(b.graph.inEdges, id)
	}

	// Clean up orphaned edges
	for source, edgeList := range b.graph.edges {
		filtered := make([]*Edge, 0, len(edgeList))
		for _, e := range edgeList {
			if !idsToRemove[e.Source] && !idsToRemove[e.Target] {
				filtered = append(filtered, e)
			}
		}
		b.graph.edges[source] = filtered
	}

	b.graph.MarkDirty() // remoção direta de nodes/edges — marca para persistência

	return nil
}

// Clear removes all nodes and edges from the graph.
func (b *Builder) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.graph.nodes = make(map[string]*Node)
	b.graph.edges = make(map[string][]*Edge)
	b.graph.inEdges = make(map[string][]*Edge)
	b.graph.MarkDirty() // limpeza total é mutação — marca para persistência
}

// ── Helper methods ────────────────────────────────────────────────────────

// resolveDependency attempts to find a node matching a dependency name/path.
func (b *Builder) resolveDependency(dep string) string {
	// Exact match on name
	for _, node := range b.graph.nodes {
		if node.Name == dep || node.ID == dep {
			return node.ID
		}
	}

	// Match on path suffix
	for _, node := range b.graph.nodes {
		if strings.HasSuffix(node.Path, dep) || strings.HasSuffix(node.Path, dep+".md") {
			return node.ID
		}
	}

	// Match on file name (without extension)
	for _, node := range b.graph.nodes {
		base := filepath.Base(node.Path)
		base = strings.TrimSuffix(base, filepath.Ext(base))
		if base == dep {
			return node.ID
		}
	}

	return ""
}

// resolveLinkPath resolves a relative link URL to an absolute path.
//
// Document paths in the graph are slash-separated logical keys (e.g.
// "docs/a.md"), not OS filesystem paths, so path manipulation must use the
// "path" package. Using "path/filepath" here produced backslash-separated
// results on Windows (e.g. "docs\b.md"), breaking the pathToID lookup.
func resolveLinkPath(sourcePath, linkURL string) string {
	if strings.HasPrefix(linkURL, "/") || strings.HasPrefix(linkURL, "http") {
		return linkURL
	}

	dir := path.Dir(sourcePath)
	return path.Clean(path.Join(dir, linkURL))
}

// toStringSlice converts an interface{} to a string slice.
func toStringSlice(val interface{}) []string {
	switch v := val.(type) {
	case string:
		return strings.Split(v, ",")
	case []interface{}:
		result := make([]string, len(v))
		for i, item := range v {
			result[i] = fmt.Sprintf("%v", item)
		}
		return result
	case []string:
		return v
	default:
		return nil
	}
}
