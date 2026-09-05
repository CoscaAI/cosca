package chunker

import (
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/markdown"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	if cfg.MaxTokens != 512 {
		t.Errorf("MaxTokens = %d, want 512", cfg.MaxTokens)
	}
	if cfg.OverlapTokens != 64 {
		t.Errorf("OverlapTokens = %d, want 64", cfg.OverlapTokens)
	}
	if cfg.MinChunkTokens != 50 {
		t.Errorf("MinChunkTokens = %d, want 50", cfg.MinChunkTokens)
	}
	if cfg.PreserveCodeBlocks != true {
		t.Error("PreserveCodeBlocks should be true")
	}
	if cfg.PreserveTables != true {
		t.Error("PreserveTables should be true")
	}
	if cfg.HeadingAware != true {
		t.Error("HeadingAware should be true")
	}
	if cfg.SectionDetection != true {
		t.Error("SectionDetection should be true")
	}
}

func TestNewChunker(t *testing.T) {
	t.Parallel()

	c := New(DefaultConfig())
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if c.cfg.MaxTokens != 512 {
		t.Errorf("MaxTokens = %d, want 512", c.cfg.MaxTokens)
	}
}

func TestNewChunkerZeroValues(t *testing.T) {
	t.Parallel()

	c := New(Config{})
	if c.cfg.MaxTokens != 512 {
		t.Errorf("MaxTokens should default to 512, got %d", c.cfg.MaxTokens)
	}
	if c.cfg.MinChunkTokens != 50 {
		t.Errorf("MinChunkTokens should default to 50, got %d", c.cfg.MinChunkTokens)
	}
}

func TestChunkSize(t *testing.T) {
	t.Parallel()

	c := New(Config{MaxTokens: 256})
	if c.ChunkSize() != 256 {
		t.Errorf("ChunkSize() = %d, want 256", c.ChunkSize())
	}
}

func TestDetectSectionType(t *testing.T) {
	t.Parallel()

	c := New(DefaultConfig())

	tests := []struct {
		content string
		want    string
	}{
		{"hello world", "text"},
		{"```go\ncode\n```", "code"},
		{"| col1 | col2 |", "table"},
		{"- item one", "list"},
		{"# heading one", "heading"},
		{"> quote here", "quote"},
		{"* star item", "list"},
		{"1. ordered", "list"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			got := c.detectSectionType(tc.content)
			if got != tc.want {
				t.Errorf("detectSectionType(%q) = %q, want %q", tc.content, got, tc.want)
			}
		})
	}
}

func TestDetectSectionType_Disabled(t *testing.T) {
	t.Parallel()

	c := New(Config{SectionDetection: false})
	got := c.detectSectionType("```go\ncode\n```")
	if got != "text" {
		t.Errorf("got %q, want text", got)
	}
}

func TestCreateChunk(t *testing.T) {
	t.Parallel()

	c := New(DefaultConfig())
	chunk := c.createChunk("test content", "doc123", "Introduction", "text", 0)
	if chunk.ID == "" {
		t.Error("chunk ID should not be empty")
	}
	if chunk.DocumentID != "doc123" {
		t.Errorf("DocumentID = %q, want doc123", chunk.DocumentID)
	}
	if chunk.Heading != "Introduction" {
		t.Errorf("Heading = %q, want Introduction", chunk.Heading)
	}
	if chunk.SectionType != "text" {
		t.Errorf("SectionType = %q, want text", chunk.SectionType)
	}
	if chunk.Position != 0 {
		t.Errorf("Position = %d, want 0", chunk.Position)
	}
	if chunk.TokenCount <= 0 {
		t.Errorf("TokenCount = %d, want > 0", chunk.TokenCount)
	}
	if chunk.Hash == "" {
		t.Error("Hash should not be empty")
	}
}

func TestCreateChunkDetectSection(t *testing.T) {
	t.Parallel()

	c := New(DefaultConfig())
	chunk := c.createChunk("| col1 | col2 |", "doc1", "", "", 0)
	if chunk.SectionType != "table" {
		t.Errorf("SectionType = %q, want table", chunk.SectionType)
	}
}

func TestSplitSentences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		min   int
	}{
		{"one sentence", "Hello world.", 1},
		{"three sentences", "First sentence. Second sentence! Third?", 3},
		{"no punctuation", "No punctuation", 1},
		{"newline split", "Line one\nLine two", 2},
		{"empty", "", 0},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := splitSentences(tc.input)
			if len(result) < tc.min {
				t.Errorf("splitSentences returned %d sentences, want at least %d", len(result), tc.min)
			}
		})
	}
}

func TestComputeHash(t *testing.T) {
	t.Parallel()

	h1 := computeHash("hello")
	h2 := computeHash("hello")
	h3 := computeHash("world")

	if h1 != h2 {
		t.Error("same content should produce same hash")
	}
	if h1 == h3 {
		t.Error("different content should produce different hashes")
	}
	if len(h1) != 64 {
		t.Errorf("hash length = %d, want 64", len(h1))
	}
}

func TestToJSON(t *testing.T) {
	t.Parallel()

	c := New(DefaultConfig())
	chunk := c.createChunk("test", "d1", "h1", "text", 1)
	result, err := chunk.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON error: %v", err)
	}
	if result == "" {
		t.Error("JSON should not be empty")
	}
}

func TestChunkSummary(t *testing.T) {
	t.Parallel()

	c := New(DefaultConfig())
	chunk := c.createChunk("short content", "d1", "Intro", "text", 0)
	summary := chunk.ChunkSummary()
	if summary == "" {
		t.Error("summary should not be empty")
	}
}

func TestChunkSummary_LongContent(t *testing.T) {
	t.Parallel()

	c := New(DefaultConfig())
	longContent := strings.Repeat("a", 200)
	chunk := c.createChunk(longContent, "d1", "H", "text", 0)
	summary := chunk.ChunkSummary()
	if !strings.HasSuffix(summary, "...") {
		t.Error("long content summary should be truncated with ...")
	}
}

// ── ChunkDocument tests ────────────────────────────────────────────────────

func newDocWithAST(children ...*markdown.Node) *markdown.Document {
	return &markdown.Document{
		Path:       "test.md",
		Title:      "Test Document",
		RawContent: "test content",
		AST: &markdown.Node{
			Type:     markdown.NodeDocument,
			Children: children,
		},
		Frontmatter: markdown.Frontmatter{
			Data: make(map[string]interface{}),
		},
	}
}

func TestChunkDocument_NilDoc(t *testing.T) {
	t.Parallel()

	c := New(DefaultConfig())
	chunks, err := c.ChunkDocument(nil, "doc1")
	if err == nil {
		t.Fatal("expected error for nil document")
	}
	if chunks != nil {
		t.Errorf("chunks = %v, want nil", chunks)
	}
}

func TestChunkDocument_EmptyAST(t *testing.T) {
	t.Parallel()

	// Comportamento atual (filtro de triviais ATIVO, L353): documento vazio
	// não gera chunk — o chunk vazio é exatamente o lixo T1 que o filtro
	// preventivo impede de entrar no corpus (comportamento-futuro).
	c := New(DefaultConfig())
	doc := newDocWithAST()
	doc.RawContent = ""
	chunks, err := c.ChunkDocument(doc, "doc1")
	if err != nil {
		t.Fatalf("ChunkDocument error: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("filtro ativo: documento vazio gerou %d chunks, want 0 (lixo T1)", len(chunks))
	}
	if st := c.TrivialStats(); st.FilteredT1 == 0 {
		t.Error("contador T1 não incrementado para chunk vazio")
	}
}

func TestChunkDocument_EmptyAST_FilterDisabled(t *testing.T) {
	t.Parallel()

	// Reversibilidade: com o filtro DESLIGADO, o comportamento antigo
	// permanece (1 chunk a partir do raw content, mesmo vazio).
	cfg := DefaultConfig()
	cfg.FilterTrivial = false
	c := New(cfg)
	doc := newDocWithAST()
	doc.RawContent = ""
	chunks, err := c.ChunkDocument(doc, "doc1")
	if err != nil {
		t.Fatalf("ChunkDocument error: %v", err)
	}
	if len(chunks) == 0 {
		t.Error("filtro desligado: esperado pelo menos 1 chunk (comportamento antigo)")
	}
}

func TestChunkDocument_NilASTNode(t *testing.T) {
	t.Parallel()

	c := New(DefaultConfig())
	doc := newDocWithAST(nil)
	chunks, err := c.ChunkDocument(doc, "doc1")
	if err != nil {
		t.Fatalf("ChunkDocument error: %v", err)
	}
	if len(chunks) == 0 {
		t.Error("expected at least one chunk")
	}
}

func TestChunkDocument_Paragraph(t *testing.T) {
	t.Parallel()

	c := New(DefaultConfig())
	doc := newDocWithAST(&markdown.Node{
		Type:    markdown.NodeParagraph,
		Content: "This is a simple paragraph of text for testing chunking.",
	})

	chunks, err := c.ChunkDocument(doc, "doc1")
	if err != nil {
		t.Fatalf("ChunkDocument error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}
	if chunks[0].SectionType != "text" {
		t.Errorf("SectionType = %q, want text", chunks[0].SectionType)
	}
	if chunks[0].DocumentID != "doc1" {
		t.Errorf("DocumentID = %q, want doc1", chunks[0].DocumentID)
	}
}

func TestChunkDocument_Heading(t *testing.T) {
	t.Parallel()

	c := New(DefaultConfig())
	doc := newDocWithAST(&markdown.Node{
		Type:    markdown.NodeHeading,
		Content: "Introduction",
		Level:   2,
	})

	chunks, err := c.ChunkDocument(doc, "doc1")
	if err != nil {
		t.Fatalf("ChunkDocument error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}
	if chunks[0].SectionType != "heading" {
		t.Errorf("SectionType = %q, want heading", chunks[0].SectionType)
	}
	if chunks[0].Heading != "Introduction" {
		t.Errorf("Heading = %q, want Introduction", chunks[0].Heading)
	}
	if chunks[0].Metadata["level"] != "2" {
		t.Errorf("level = %q, want 2", chunks[0].Metadata["level"])
	}
}

func TestChunkDocument_CodeBlock(t *testing.T) {
	t.Parallel()

	c := New(DefaultConfig())
	doc := newDocWithAST(&markdown.Node{
		Type:     markdown.NodeCodeBlock,
		Content:  "func main() {\n\tfmt.Println(\"hello\")\n}",
		Language: "go",
	})

	chunks, err := c.ChunkDocument(doc, "doc1")
	if err != nil {
		t.Fatalf("ChunkDocument error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}
	if chunks[0].SectionType != "code" {
		t.Errorf("SectionType = %q, want code", chunks[0].SectionType)
	}
	if chunks[0].Metadata["language"] != "go" {
		t.Errorf("language = %q, want go", chunks[0].Metadata["language"])
	}
}

func TestChunkDocument_CodeBlock_NoPreserve(t *testing.T) {
	t.Parallel()

	c := New(Config{
		MaxTokens:          512,
		PreserveCodeBlocks: false,
		SectionDetection:   true,
	})
	doc := newDocWithAST(&markdown.Node{
		Type:     markdown.NodeCodeBlock,
		Content:  "func hello() {}",
		Language: "go",
	})

	chunks, err := c.ChunkDocument(doc, "doc1")
	if err != nil {
		t.Fatalf("ChunkDocument error: %v", err)
	}
	if len(chunks) < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestChunkDocument_Table(t *testing.T) {
	t.Parallel()

	c := New(DefaultConfig())
	doc := &markdown.Document{
		Path:       "test.md",
		Title:      "Test",
		RawContent: "test",
		AST: &markdown.Node{
			Type: markdown.NodeDocument,
			Children: []*markdown.Node{{
				Type:     markdown.NodeTable,
				Content:  "| name | age |\n|------|-----|\n| alice | 30 |",
				Position: 0,
			}},
		},
		Tables: []markdown.Table{{
			Headers:  []string{"name", "age"},
			Rows:     [][]string{{"alice", "30"}},
			Position: 0,
		}},
		Frontmatter: markdown.Frontmatter{
			Data: make(map[string]interface{}),
		},
	}

	chunks, err := c.ChunkDocument(doc, "doc1")
	if err != nil {
		t.Fatalf("ChunkDocument error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}
	if chunks[0].SectionType != "table" {
		t.Errorf("SectionType = %q, want table", chunks[0].SectionType)
	}
	// The table content should contain headers and rows
	if !strings.Contains(chunks[0].Content, "name") {
		t.Errorf("table content missing header: %q", chunks[0].Content)
	}
}

func TestChunkDocument_Table_NoPreserve(t *testing.T) {
	t.Parallel()

	c := New(Config{
		MaxTokens:      512,
		PreserveTables: false,
	})
	doc := &markdown.Document{
		Path:       "test.md",
		Title:      "Test",
		RawContent: "test",
		AST: &markdown.Node{
			Type: markdown.NodeDocument,
			Children: []*markdown.Node{{
				Type:     markdown.NodeTable,
				Content:  "| a | b |\n|---|---|\n| 1 | 2 |",
				Position: 0,
			}},
		},
		Tables: []markdown.Table{{
			Headers:  []string{"a", "b"},
			Rows:     [][]string{{"1", "2"}},
			Position: 0,
		}},
		Frontmatter: markdown.Frontmatter{
			Data: make(map[string]interface{}),
		},
	}

	chunks, err := c.ChunkDocument(doc, "doc1")
	if err != nil {
		t.Fatalf("ChunkDocument error: %v", err)
	}
	if len(chunks) < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestChunkDocument_List(t *testing.T) {
	t.Parallel()

	c := New(DefaultConfig())
	doc := newDocWithAST(&markdown.Node{
		Type:    markdown.NodeList,
		Content: "- item one\n- item two\n- item three",
	})

	chunks, err := c.ChunkDocument(doc, "doc1")
	if err != nil {
		t.Fatalf("ChunkDocument error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}
	if chunks[0].SectionType != "list" {
		t.Errorf("SectionType = %q, want list", chunks[0].SectionType)
	}
}

func TestChunkDocument_Blockquote(t *testing.T) {
	t.Parallel()

	c := New(DefaultConfig())
	doc := newDocWithAST(&markdown.Node{
		Type:    markdown.NodeBlockquote,
		Content: "A quoted piece of text.",
	})

	chunks, err := c.ChunkDocument(doc, "doc1")
	if err != nil {
		t.Fatalf("ChunkDocument error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}
}

func TestChunkDocument_MultipleNodes(t *testing.T) {
	t.Parallel()

	c := New(DefaultConfig())
	doc := newDocWithAST(
		&markdown.Node{
			Type:    markdown.NodeHeading,
			Content: "Section 1",
			Level:   2,
		},
		&markdown.Node{
			Type:    markdown.NodeParagraph,
			Content: "First paragraph content.",
		},
		&markdown.Node{
			Type:    markdown.NodeParagraph,
			Content: "Second paragraph content.",
		},
	)

	chunks, err := c.ChunkDocument(doc, "doc1")
	if err != nil {
		t.Fatalf("ChunkDocument error: %v", err)
	}
	if len(chunks) != 3 {
		t.Fatalf("got %d chunks, want 3", len(chunks))
	}
	// First chunk should be heading
	if chunks[0].SectionType != "heading" {
		t.Errorf("chunk 0 type = %q, want heading", chunks[0].SectionType)
	}
	// Positions should be sequential
	if chunks[0].Position != 0 {
		t.Errorf("chunk 0 position = %d, want 0", chunks[0].Position)
	}
	if chunks[1].Position != 1 {
		t.Errorf("chunk 1 position = %d, want 1", chunks[1].Position)
	}
	if chunks[2].Position != 2 {
		t.Errorf("chunk 2 position = %d, want 2", chunks[2].Position)
	}
}

func TestChunkDocument_HeadingContext(t *testing.T) {
	t.Parallel()

	c := New(DefaultConfig())
	doc := newDocWithAST(
		&markdown.Node{
			Type:    markdown.NodeHeading,
			Content: "Introduction",
			Level:   2,
		},
		&markdown.Node{
			Type:    markdown.NodeParagraph,
			Content: "Paragraph under Introduction heading.",
		},
	)

	chunks, err := c.ChunkDocument(doc, "doc1")
	if err != nil {
		t.Fatalf("ChunkDocument error: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(chunks))
	}
	// Paragraph chunk should have heading context from the heading before it
	if chunks[1].Heading != "Introduction" {
		t.Errorf("paragraph heading = %q, want Introduction", chunks[1].Heading)
	}
}

// ── Large content splitting ────────────────────────────────────────────────

func TestChunkDocument_LargeContent(t *testing.T) {
	t.Parallel()

	c := New(Config{
		MaxTokens:        10,
		OverlapTokens:    2,
		MinChunkTokens:   5,
		SectionDetection: true,
	})

	// Create content that will exceed MaxTokens (10)
	longPara := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 10)

	doc := newDocWithAST(&markdown.Node{
		Type:    markdown.NodeParagraph,
		Content: longPara,
	})

	chunks, err := c.ChunkDocument(doc, "doc1")
	if err != nil {
		t.Fatalf("ChunkDocument error: %v", err)
	}
	if len(chunks) < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestSplitContent_Small(t *testing.T) {
	t.Parallel()

	c := New(Config{
		MaxTokens:        1000,
		OverlapTokens:    0,
		MinChunkTokens:   5,
		SectionDetection: false,
	})

	smallContent := "This is a small piece of text."
	chunks := c.splitContent(smallContent, "doc1", "Intro", "text", 0)
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}
	if chunks[0].Content != smallContent {
		t.Errorf("Content = %q, want %q", chunks[0].Content, smallContent)
	}
}

func TestSplitContent_WithOverlap(t *testing.T) {
	t.Parallel()

	c := New(Config{
		MaxTokens:        10,
		OverlapTokens:    5,
		MinChunkTokens:   5,
		SectionDetection: false,
	})

	// Two paragraphs that each exceed MaxTokens individually
	content := strings.Repeat("aaaaa ", 20) + "\n\n" + strings.Repeat("bbbbb ", 20)

	chunks := c.splitContent(content, "doc1", "Test", "text", 0)
	if len(chunks) < 2 {
		t.Fatalf("got %d chunks, want at least 2", len(chunks))
	}
}

func TestSplitLongParagraph(t *testing.T) {
	t.Parallel()

	c := New(Config{
		MaxTokens:        10,
		OverlapTokens:    0,
		MinChunkTokens:   3,
		SectionDetection: false,
	})

	longPara := "First sentence. Second sentence. Third sentence. Fourth sentence. Fifth sentence."
	chunks := c.splitLongParagraph(longPara, "doc1", "H", "text", 0)
	if len(chunks) < 2 {
		t.Fatalf("got %d chunks, want at least 2", len(chunks))
	}
}

func TestAddOverlap(t *testing.T) {
	t.Parallel()

	c := New(Config{
		MaxTokens:      512,
		OverlapTokens:  5,
		MinChunkTokens: 5,
	})

	chunks := []Chunk{
		{
			ID:      "1",
			Content: "first chunk with some words to overlap",
		},
		{
			ID:      "2",
			Content: "second chunk of content",
		},
	}

	// Set token counts for overlap calculation
	chunks[0].TokenCount = markdown.CountTokens(chunks[0].Content)
	chunks[1].TokenCount = markdown.CountTokens(chunks[1].Content)

	originalSecondContent := chunks[1].Content

	result := c.addOverlap(chunks)
	if len(result) != 2 {
		t.Fatalf("got %d chunks, want 2", len(result))
	}
	// The second chunk should have overlap prepended
	if result[1].Content == originalSecondContent {
		t.Error("expected overlap to modify second chunk")
	}
}

func TestAddOverlap_NoOverlapNeeded(t *testing.T) {
	t.Parallel()

	c := New(Config{
		MaxTokens:      512,
		OverlapTokens:  0,
		MinChunkTokens: 5,
	})

	chunks := []Chunk{
		{ID: "1", Content: "first", TokenCount: 5},
		{ID: "2", Content: "second", TokenCount: 6},
	}

	result := c.addOverlap(chunks)
	if result[1].Content != "second" {
		t.Errorf("Content changed when overlap is 0")
	}
}

// ── ChunkBatch ──────────────────────────────────────────────────────────────

func TestChunkBatch(t *testing.T) {
	t.Parallel()

	c := New(DefaultConfig())

	doc1 := newDocWithAST(&markdown.Node{
		Type:    markdown.NodeParagraph,
		Content: "Document 1 content.",
	})
	doc2 := newDocWithAST(&markdown.Node{
		Type:    markdown.NodeParagraph,
		Content: "Document 2 content.",
	})

	docs := map[string]*markdown.Document{
		"doc1": doc1,
		"doc2": doc2,
	}

	chunks, err := c.ChunkBatch(docs)
	if err != nil {
		t.Fatalf("ChunkBatch error: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(chunks))
	}
	// Map iteration order is non-deterministic — use ID-based lookup
	found := make(map[string]bool)
	for _, c := range chunks {
		found[c.DocumentID] = true
	}
	if !found["doc1"] {
		t.Errorf("chunks missing doc1, got IDs: %v", found)
	}
	if !found["doc2"] {
		t.Errorf("chunks missing doc2, got IDs: %v", found)
	}
}

func TestChunkBatch_WithNilDoc(t *testing.T) {
	t.Parallel()

	c := New(DefaultConfig())

	docs := map[string]*markdown.Document{
		"ok": newDocWithAST(&markdown.Node{
			Type:    markdown.NodeParagraph,
			Content: "OK content.",
		}),
		"bad": nil,
	}

	chunks, err := c.ChunkBatch(docs)
	if err != nil {
		t.Fatalf("ChunkBatch error: %v", err)
	}
	// Should skip the nil doc and succeed for the ok one
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}
	if chunks[0].DocumentID != "ok" {
		t.Errorf("DocumentID = %q, want ok", chunks[0].DocumentID)
	}
}

// ── Edge cases ──────────────────────────────────────────────────────────────

func TestChunkDocument_UnknownNodeType(t *testing.T) {
	t.Parallel()

	c := New(DefaultConfig())
	doc := newDocWithAST(&markdown.Node{
		Type:    "unknown_type",
		Content: "some content",
	})

	chunks, err := c.ChunkDocument(doc, "doc1")
	if err != nil {
		t.Fatalf("ChunkDocument error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}
	if chunks[0].SectionType != "text" {
		t.Errorf("unknown node SectionType = %q, want text", chunks[0].SectionType)
	}
}

func TestCreateChunk_AutoDetectSection(t *testing.T) {
	t.Parallel()

	c := New(DefaultConfig())

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"heading by pattern", "# heading", "heading"},
		{"list by dash", "- item", "list"},
		{"list by star", "* item", "list"},
		{"list by number", "1. item", "list"},
		{"table by pipe", "| col |", "table"},
		{"quote by angle", "> quoted", "quote"},
		{"default text", "plain text", "text"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			chunk := c.createChunk(tc.content, "doc1", "", "", 0)
			if chunk.SectionType != tc.want {
				t.Errorf("SectionType = %q, want %q", chunk.SectionType, tc.want)
			}
		})
	}
}

func TestEmptyContentChunk(t *testing.T) {
	t.Parallel()

	c := New(DefaultConfig())
	chunk := c.createChunk("", "doc1", "", "", 0)
	if chunk.Hash == "" {
		t.Error("hash should not be empty even for empty content")
	}
	if chunk.ID == "" {
		t.Error("ID should not be empty")
	}
}
