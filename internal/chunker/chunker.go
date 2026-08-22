// Package chunker provides document chunking for the Cosca Knowledge Engine.
// It implements smart chunking that respects document structure, preserving
// code blocks, tables, and heading boundaries.
package chunker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/CoscaAI/cosca/internal/markdown"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Chunk represents a single chunk of a document.
type Chunk struct {
	ID          string            `json:"id"`
	DocumentID  string            `json:"document_id"`
	Content     string            `json:"content"`
	Heading     string            `json:"heading"`
	SectionType string            `json:"section_type"` // text, code, table, list, heading, frontmatter
	Position    int               `json:"position"`
	Hash        string            `json:"hash"`
	TokenCount  int               `json:"token_count"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Config defines chunking parameters.
type Config struct {
	// MaxTokens is the maximum number of tokens per chunk (default: 512).
	MaxTokens int

	// OverlapTokens is the number of overlapping tokens between chunks (default: 64).
	OverlapTokens int

	// MinChunkTokens is the minimum number of tokens for a chunk (default: 50).
	MinChunkTokens int

	// PreserveCodeBlocks ensures code blocks are never split (default: true).
	PreserveCodeBlocks bool

	// PreserveTables ensures tables are never split (default: true).
	PreserveTables bool

	// HeadingAware splits at heading boundaries when possible (default: true).
	HeadingAware bool

	// SectionDetection enables automatic section type detection (default: true).
	SectionDetection bool

	// FilterTrivial enables the T1-T4 trivial-chunk filter on NEW chunks
	// (L353). Preventive, behavior-future only: nothing historical is touched.
	// Default: true (stops producing new junk; reversible via config).
	FilterTrivial bool
}

// DefaultConfig returns sensible default chunking configuration.
func DefaultConfig() Config {
	return Config{
		MaxTokens:          512,
		OverlapTokens:      64,
		MinChunkTokens:     50,
		PreserveCodeBlocks: true,
		PreserveTables:     true,
		HeadingAware:       true,
		SectionDetection:   true,
		FilterTrivial:      true,
	}
}

// Chunker splits documents into semantically meaningful chunks.
type Chunker struct {
	cfg     Config
	trivial *TrivialFilter
}

// New creates a new Chunker with the given configuration.
func New(cfg Config) *Chunker {
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 512
	}
	if cfg.OverlapTokens < 0 {
		cfg.OverlapTokens = 64
	}
	if cfg.MinChunkTokens <= 0 {
		cfg.MinChunkTokens = 50
	}

	return &Chunker{cfg: cfg, trivial: NewTrivialFilter()}
}

// TrivialStats expõe os contadores do filtro de triviais (observabilidade,
// L353): filtered_total, filtered_t1..t4, by_source, by_document. Apenas
// contagens — nenhum conteúdo é exposto.
func (c *Chunker) TrivialStats() TrivialStats {
	return c.trivial.Snapshot()
}

// ChunkDocument splits a parsed Markdown document into chunks.
func (c *Chunker) ChunkDocument(doc *markdown.Document, documentID string) ([]Chunk, error) {
	if doc == nil {
		return nil, fmt.Errorf("document is nil")
	}

	var chunks []Chunk
	currentHeading := ""
	position := 0

	// Walk the AST to create chunks
	for _, node := range doc.AST.Children {
		if node == nil {
			continue
		}

		// Track current heading context
		if node.Type == markdown.NodeHeading {
			currentHeading = node.Content
		}

		var sectionChunks []Chunk

		switch node.Type {
		case markdown.NodeCodeBlock:
			sectionChunks = c.chunkCodeBlock(node, documentID, currentHeading, position)

		case markdown.NodeTable:
			sectionChunks = c.chunkTable(node, doc, documentID, currentHeading, position)

		case markdown.NodeList:
			sectionChunks = c.chunkList(node, documentID, currentHeading, position)

		case markdown.NodeHeading:
			sectionChunks = c.chunkHeading(node, documentID, position)

		case markdown.NodeParagraph:
			sectionChunks = c.chunkText(node, documentID, currentHeading, position)

		case markdown.NodeBlockquote:
			sectionChunks = c.chunkText(node, documentID, currentHeading, position)

		default:
			sectionChunks = c.chunkText(node, documentID, currentHeading, position)
		}

		for i := range sectionChunks {
			sectionChunks[i].Position = position
			position++
		}

		chunks = append(chunks, sectionChunks...)
	}

	// Ensure we have at least one chunk
	if len(chunks) == 0 {
		// Create a chunk from the raw content
		chunk := c.createChunk(doc.RawContent, documentID, currentHeading, "text", 0)
		chunks = append(chunks, chunk)
	}

	// Filtro preventivo de triviais (L353): comportamento-futuro — descarta
	// chunks NOVOS sem conteúdo semântico (T1-T4). Nada histórico é tocado.
	if c.cfg.FilterTrivial {
		kept := make([]Chunk, 0, len(chunks))
		for _, ch := range chunks {
			if trivial, rule := IsTrivial(ch.Content); trivial {
				c.trivial.Record(rule, ch.SectionType, ch.DocumentID)
				continue
			}
			kept = append(kept, ch)
		}
		chunks = kept
	}

	log.Debug().
		Str("document_id", documentID).
		Int("chunks", len(chunks)).
		Msg("document chunked")

	return chunks, nil
}

// chunkCodeBlock creates a chunk for a code block, ensuring it's never split.
func (c *Chunker) chunkCodeBlock(node *markdown.Node, docID, heading string, pos int) []Chunk {
	if !c.cfg.PreserveCodeBlocks {
		return c.chunkLargeBlock(node.Content, docID, heading, "code", pos)
	}

	chunk := c.createChunk(node.Content, docID, heading, "code", pos)
	if node.Language != "" {
		chunk.Metadata["language"] = node.Language
	}
	return []Chunk{chunk}
}

// chunkTable creates a chunk for a table, ensuring it's never split.
func (c *Chunker) chunkTable(node *markdown.Node, doc *markdown.Document, docID, heading string, pos int) []Chunk {
	if !c.cfg.PreserveTables {
		return c.chunkLargeBlock(node.Content, docID, heading, "table", pos)
	}

	// Look up the actual table from the parsed tables
	var tableContent string
	for _, t := range doc.Tables {
		if t.Position <= pos && t.Position+len(t.Rows)+2 >= pos {
			var b strings.Builder
			b.WriteString("| " + strings.Join(t.Headers, " | ") + " |\n")
			b.WriteString("|" + strings.Repeat("---|", len(t.Headers)) + "\n")
			for _, row := range t.Rows {
				b.WriteString("| " + strings.Join(row, " | ") + " |\n")
			}
			tableContent = b.String()
			break
		}
	}

	if tableContent == "" {
		tableContent = node.Content
	}

	chunk := c.createChunk(tableContent, docID, heading, "table", pos)
	return []Chunk{chunk}
}

// chunkList creates a chunk for a list.
func (c *Chunker) chunkList(node *markdown.Node, docID, heading string, pos int) []Chunk {
	return c.chunkLargeBlock(node.Content, docID, heading, "list", pos)
}

// chunkHeading creates a small heading chunk for navigation.
func (c *Chunker) chunkHeading(node *markdown.Node, docID string, pos int) []Chunk {
	content := fmt.Sprintf("%s %s", strings.Repeat("#", node.Level), node.Content)
	chunk := c.createChunk(content, docID, node.Content, "heading", pos)
	chunk.Metadata["level"] = fmt.Sprintf("%d", node.Level)
	return []Chunk{chunk}
}

// chunkText creates chunks for paragraph text, splitting if necessary.
func (c *Chunker) chunkText(node *markdown.Node, docID, heading string, pos int) []Chunk {
	return c.chunkLargeBlock(node.Content, docID, heading, "text", pos)
}

// chunkLargeBlock splits content larger than MaxTokens into smaller chunks.
func (c *Chunker) chunkLargeBlock(content string, docID, heading, sectionType string, pos int) []Chunk {
	tokenCount := markdown.CountTokens(content)

	if tokenCount <= c.cfg.MaxTokens {
		return []Chunk{c.createChunk(content, docID, heading, sectionType, pos)}
	}

	// Split by sentences/paragraphs
	chunks := c.splitContent(content, docID, heading, sectionType, pos)
	return chunks
}

// splitContent splits content respecting structural boundaries.
func (c *Chunker) splitContent(content string, docID, heading, sectionType string, startPos int) []Chunk {
	var chunks []Chunk
	pos := startPos

	// Try to split on paragraph boundaries first
	paragraphs := strings.Split(content, "\n\n")
	var currentBuilder strings.Builder
	currentTokens := 0

	flush := func() {
		if currentBuilder.Len() == 0 {
			return
		}
		text := strings.TrimSpace(currentBuilder.String())
		if text == "" {
			return
		}
		tokens := markdown.CountTokens(text)
		if tokens < c.cfg.MinChunkTokens && len(chunks) > 0 {
			// Merge with last chunk if too small
			last := &chunks[len(chunks)-1]
			last.Content += "\n\n" + text
			last.TokenCount = markdown.CountTokens(last.Content)
			last.Hash = computeHash(last.Content)
		} else {
			chunks = append(chunks, c.createChunk(text, docID, heading, sectionType, pos))
			pos++
		}
		currentBuilder.Reset()
		currentTokens = 0
	}

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		paraTokens := markdown.CountTokens(para)

		if currentTokens+paraTokens <= c.cfg.MaxTokens {
			if currentBuilder.Len() > 0 {
				currentBuilder.WriteString("\n\n")
			}
			currentBuilder.WriteString(para)
			currentTokens += paraTokens
		} else {
			flush()
			// If a single paragraph exceeds MaxTokens, split it
			if paraTokens > c.cfg.MaxTokens {
				subChunks := c.splitLongParagraph(para, docID, heading, sectionType, pos)
				chunks = append(chunks, subChunks...)
				pos += len(subChunks)
			} else {
				currentBuilder.WriteString(para)
				currentTokens = paraTokens
			}
		}
	}

	flush()

	// Add overlap between consecutive chunks
	if c.cfg.OverlapTokens > 0 && len(chunks) > 1 {
		chunks = c.addOverlap(chunks)
	}

	return chunks
}

// splitLongParagraph splits a single long paragraph into smaller chunks.
func (c *Chunker) splitLongParagraph(paragraph string, docID, heading, sectionType string, startPos int) []Chunk {
	var chunks []Chunk
	pos := startPos

	// Split on sentence boundaries when possible
	sentences := splitSentences(paragraph)
	var currentBuilder strings.Builder
	currentTokens := 0

	for _, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if sentence == "" {
			continue
		}

		sentTokens := markdown.CountTokens(sentence)

		if currentTokens+sentTokens <= c.cfg.MaxTokens {
			if currentBuilder.Len() > 0 {
				currentBuilder.WriteString(" ")
			}
			currentBuilder.WriteString(sentence)
			currentTokens += sentTokens
		} else {
			if currentBuilder.Len() > 0 {
				chunks = append(chunks, c.createChunk(
					strings.TrimSpace(currentBuilder.String()),
					docID, heading, sectionType, pos,
				))
				pos++
			}
			currentBuilder.Reset()
			currentBuilder.WriteString(sentence)
			currentTokens = sentTokens
		}
	}

	if currentBuilder.Len() > 0 {
		chunks = append(chunks, c.createChunk(
			strings.TrimSpace(currentBuilder.String()),
			docID, heading, sectionType, pos,
		))
	}

	return chunks
}

// addOverlap adds overlapping text between consecutive chunks.
func (c *Chunker) addOverlap(chunks []Chunk) []Chunk {
	for i := 1; i < len(chunks); i++ {
		prev := &chunks[i-1]
		curr := &chunks[i]

		// Take overlap tokens from end of previous chunk
		prevTokens := prev.TokenCount
		if prevTokens > c.cfg.OverlapTokens {
			// Find overlap content
			words := strings.Fields(prev.Content)
			wordCount := len(words)
			overlapWordCount := (c.cfg.OverlapTokens * wordCount) / prevTokens
			if overlapWordCount > 0 && overlapWordCount < wordCount {
				overlapText := strings.Join(words[wordCount-overlapWordCount:], " ")
				curr.Content = overlapText + "\n\n" + curr.Content
				curr.TokenCount = markdown.CountTokens(curr.Content)
				curr.Hash = computeHash(curr.Content)
			}
		}
	}
	return chunks
}

// createChunk creates a single chunk with metadata.
func (c *Chunker) createChunk(content, docID, heading, sectionType string, position int) Chunk {
	if sectionType == "" {
		sectionType = c.detectSectionType(content)
	}

	chunk := Chunk{
		ID:          uuid.New().String(),
		DocumentID:  docID,
		Content:     content,
		Heading:     heading,
		SectionType: sectionType,
		Position:    position,
		Hash:        computeHash(content),
		TokenCount:  markdown.CountTokens(content),
		Metadata:    make(map[string]string),
	}

	return chunk
}

// detectSectionType attempts to auto-detect the section type from content.
func (c *Chunker) detectSectionType(content string) string {
	if !c.cfg.SectionDetection {
		return "text"
	}

	trimmed := strings.TrimSpace(content)

	if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "    ") {
		return "code"
	}
	if strings.HasPrefix(trimmed, "|") && strings.Contains(trimmed, "|") {
		return "table"
	}
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "1. ") {
		return "list"
	}
	if strings.HasPrefix(trimmed, "#") && strings.HasPrefix(trimmed, "# ") {
		return "heading"
	}
	if strings.HasPrefix(trimmed, ">") {
		return "quote"
	}

	return "text"
}

// splitSentences splits text into sentences.
func splitSentences(text string) []string {
	// Simple sentence splitting on common delimiters
	var sentences []string
	current := strings.Builder{}

	for _, ch := range text {
		current.WriteRune(ch)
		if ch == '.' || ch == '!' || ch == '?' || ch == '\n' {
			sentences = append(sentences, current.String())
			current.Reset()
		}
	}

	if current.Len() > 0 {
		sentences = append(sentences, current.String())
	}

	return sentences
}

// computeHash returns a SHA-256 hash of the content.
func computeHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// ChunkBatch processes multiple documents into chunks.
func (c *Chunker) ChunkBatch(docs map[string]*markdown.Document) ([]Chunk, error) {
	var allChunks []Chunk
	for docID, doc := range docs {
		chunks, err := c.ChunkDocument(doc, docID)
		if err != nil {
			log.Warn().Err(err).Str("document_id", docID).Msg("failed to chunk document")
			continue
		}
		allChunks = append(allChunks, chunks...)
	}
	return allChunks, nil
}

// ChunkSize returns the current max chunk size in tokens.
func (c *Chunker) ChunkSize() int {
	return c.cfg.MaxTokens
}

// ToJSON serializes a chunk to JSON.
func (chunk *Chunk) ToJSON() (string, error) {
	data, err := json.Marshal(chunk)
	if err != nil {
		return "", fmt.Errorf("marshal chunk: %w", err)
	}
	return string(data), nil
}

// ChunkSummary returns a compact summary of the chunk.
func (chunk *Chunk) ChunkSummary() string {
	content := chunk.Content
	if len(content) > 100 {
		content = content[:100] + "..."
	}
	return fmt.Sprintf("[%s] heading=%q type=%s tokens=%d pos=%d\n%s",
		chunk.ID[:8], chunk.Heading, chunk.SectionType, chunk.TokenCount, chunk.Position, content)
}
