// Package markdown provides a structured Markdown parser for the Cosca Knowledge Engine.
// It parses Markdown files into structured ASTs, extracting headings, code blocks,
// tables, lists, frontmatter, links, and references.
package markdown

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// Document represents a fully parsed Markdown document.
type Document struct {
	Path        string      `json:"path"`
	Title       string      `json:"title"`
	Frontmatter Frontmatter `json:"frontmatter"`
	Headings    []Heading   `json:"headings"`
	CodeBlocks  []CodeBlock `json:"code_blocks"`
	Tables      []Table     `json:"tables"`
	Lists       []List      `json:"lists"`
	Links       []Link      `json:"links"`
	References  []Reference `json:"references"`
	Paragraphs  []Paragraph `json:"paragraphs"`
	AST         *Node       `json:"-"`
	RawContent  string      `json:"-"`
	TokenCount  int         `json:"token_count"`
}

// Frontmatter holds parsed YAML/TOML frontmatter data.
type Frontmatter struct {
	Raw    string                 `json:"raw"`
	Format string                 `json:"format"` // "yaml" or "toml"
	Data   map[string]interface{} `json:"data"`
}

// Heading represents a Markdown heading.
type Heading struct {
	Level    int    `json:"level"`
	Text     string `json:"text"`
	Position int    `json:"position"`
	Slug     string `json:"slug"`
}

// CodeBlock represents a fenced code block.
type CodeBlock struct {
	Language   string `json:"language"`
	Content    string `json:"content"`
	Position   int    `json:"position"`
	TokenCount int    `json:"token_count"`
}

// Table represents a Markdown table.
type Table struct {
	Caption  string     `json:"caption"`
	Headers  []string   `json:"headers"`
	Rows     [][]string `json:"rows"`
	Position int        `json:"position"`
}

// List represents a Markdown list (ordered or unordered).
type List struct {
	Ordered  bool     `json:"ordered"`
	Items    []string `json:"items"`
	Position int      `json:"position"`
	Depth    int      `json:"depth"`
}

// Link represents a Markdown link.
type Link struct {
	Text     string `json:"text"`
	URL      string `json:"url"`
	Position int    `json:"position"`
	IsImage  bool   `json:"is_image"`
}

// Reference represents a reference-style link definition.
type Reference struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Position int    `json:"position"`
}

// Paragraph represents a block of text.
type Paragraph struct {
	Text     string `json:"text"`
	Position int    `json:"position"`
}

// Node represents a node in the Markdown AST.
type Node struct {
	Type     NodeType          `json:"type"`
	Content  string            `json:"content,omitempty"`
	Level    int               `json:"level,omitempty"`
	Position int               `json:"position"`
	Language string            `json:"language,omitempty"`
	Children []*Node           `json:"children,omitempty"`
	Meta     map[string]string `json:"meta,omitempty"`
}

// NodeType enumerates AST node types.
type NodeType string

// Predefined Markdown AST node types.
const (
	NodeDocument      NodeType = "document"
	NodeHeading       NodeType = "heading"
	NodeParagraph     NodeType = "paragraph"
	NodeCodeBlock     NodeType = "code_block"
	NodeTable         NodeType = "table"
	NodeList          NodeType = "list"
	NodeListItem      NodeType = "list_item"
	NodeBlockquote    NodeType = "blockquote"
	NodeThematicBreak NodeType = "thematic_break"
	NodeFrontmatter   NodeType = "frontmatter"
	NodeHTML          NodeType = "html"
	NodeMath          NodeType = "math"
	NodeDiagram       NodeType = "diagram"
)

// Parser parses Markdown content into structured documents.
type Parser struct {
	// Options
	IncludeRawHTML bool
	MaxSize        int64 // Maximum file size to parse (0 = no limit)

	// GroupConsecutiveParagraphLines agrupa linhas de parágrafo consecutivas
	// sem blank line em UM nó (padrão CommonMark — L356). Corrige a
	// fragmentação por linha (L354): JSON indentado deixava de virar ~1.500
	// chunks de 1 linha. Reversível: desligar restaura o comportamento antigo.
	GroupConsecutiveParagraphLines bool
}

// NewParser creates a new Markdown parser with default settings.
func NewParser() *Parser {
	return &Parser{
		IncludeRawHTML:                 false,
		MaxSize:                        10 * 1024 * 1024, // 10MB default
		GroupConsecutiveParagraphLines: true,
	}
}

// looksLikeMarkdownBlock detecta se a linha parece iniciar um bloco
// estrutural (heading, code, table, list, blockquote) — usada pelo
// agrupamento de parágrafos (L356) para NÃO absorver fronteiras semânticas.
func looksLikeMarkdownBlock(trimmed string) bool {
	return strings.HasPrefix(trimmed, "### ") || strings.HasPrefix(trimmed, "## ") ||
		strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "```") ||
		strings.HasPrefix(trimmed, "|") || strings.HasPrefix(trimmed, "- ") ||
		strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") ||
		strings.HasPrefix(trimmed, "1. ") || strings.HasPrefix(trimmed, "2. ") ||
		strings.HasPrefix(trimmed, "> ")
}

// Parse parses raw Markdown content into a structured Document.
func (p *Parser) Parse(path string, content string) (*Document, error) {
	if p.MaxSize > 0 && int64(len(content)) > p.MaxSize {
		return nil, fmt.Errorf("file too large: %d bytes (max %d)", len(content), p.MaxSize)
	}

	doc := &Document{
		Path:       path,
		RawContent: content,
		TokenCount: CountTokens(content),
		AST: &Node{
			Type:    NodeDocument,
			Content: path,
		},
	}

	// Extract frontmatter
	content = p.parseFrontmatter(doc, content)

	// Parse blocks sequentially
	lines := strings.Split(content, "\n")
	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(trimmed, "### "):
			heading := p.parseHeading(lines, i, 3)
			doc.Headings = append(doc.Headings, heading)
			doc.AST.Children = append(doc.AST.Children, &Node{
				Type: NodeHeading, Content: heading.Text,
				Level: heading.Level, Position: heading.Position,
			})
			i++

		case strings.HasPrefix(trimmed, "## "):
			heading := p.parseHeading(lines, i, 2)
			doc.Headings = append(doc.Headings, heading)
			doc.AST.Children = append(doc.AST.Children, &Node{
				Type: NodeHeading, Content: heading.Text,
				Level: heading.Level, Position: heading.Position,
			})
			i++

		case strings.HasPrefix(trimmed, "# "):
			heading := p.parseHeading(lines, i, 1)
			doc.Title = heading.Text
			doc.Headings = append(doc.Headings, heading)
			doc.AST.Children = append(doc.AST.Children, &Node{
				Type: NodeHeading, Content: heading.Text,
				Level: heading.Level, Position: heading.Position,
			})
			i++

		case strings.HasPrefix(trimmed, "```"):
			cb, consumed := p.parseCodeBlock(lines, i)
			doc.CodeBlocks = append(doc.CodeBlocks, cb)
			doc.AST.Children = append(doc.AST.Children, &Node{
				Type: NodeCodeBlock, Content: cb.Content,
				Language: cb.Language, Position: cb.Position,
			})
			i += consumed

		case strings.HasPrefix(trimmed, "|"):
			table, consumed := p.parseTable(lines, i)
			if table != nil {
				doc.Tables = append(doc.Tables, *table)
				doc.AST.Children = append(doc.AST.Children, &Node{
					Type: NodeTable, Content: table.Caption, Position: table.Position,
				})
				i += consumed
			} else {
				i++
			}

		case strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") ||
			strings.HasPrefix(trimmed, "1. ") || strings.HasPrefix(trimmed, "2. "):
			list, consumed := p.parseList(lines, i)
			doc.Lists = append(doc.Lists, list)
			doc.AST.Children = append(doc.AST.Children, &Node{
				Type: NodeList, Content: strings.Join(list.Items, "\n"),
				Position: list.Position,
			})
			i += consumed

		case strings.HasPrefix(trimmed, "> "):
			// Blockquote - extract paragraph
			bqContent := strings.TrimPrefix(trimmed, "> ")
			doc.Paragraphs = append(doc.Paragraphs, Paragraph{
				Text: bqContent, Position: i,
			})
			doc.AST.Children = append(doc.AST.Children, &Node{
				Type: NodeBlockquote, Content: bqContent, Position: i,
			})
			i++

		case trimmed == "":
			i++

		default:
			// Regular paragraph - also extract links from it
			// L356: agrupa linhas de parágrafo consecutivas sem blank line em
			// um único nó (padrão CommonMark). Corrige a fragmentação por
			// linha (L354: JSON indentado virava 1 chunk por linha).
			paraText := trimmed
			startPos := i
			if p.GroupConsecutiveParagraphLines {
				joined := []string{trimmed}
				for i+1 < len(lines) {
					next := strings.TrimSpace(lines[i+1])
					if next == "" || looksLikeMarkdownBlock(next) {
						break
					}
					i++
					joined = append(joined, strings.TrimSpace(lines[i]))
				}
				paraText = strings.Join(joined, "\n")
			}
			para := Paragraph{Text: paraText, Position: startPos}
			doc.Paragraphs = append(doc.Paragraphs, para)
			doc.AST.Children = append(doc.AST.Children, &Node{
				Type: NodeParagraph, Content: paraText, Position: startPos,
			})

			// Extract links from paragraph
			links := extractLinks(paraText, startPos)
			doc.Links = append(doc.Links, links...)
			i++
		}
	}

	// Extract references (reference-style links [id]: url)
	doc.References = extractReferences(content)

	// Clean empty frontmatter
	if doc.Frontmatter.Data == nil {
		doc.Frontmatter.Data = make(map[string]interface{})
	}

	return doc, nil
}

// parseFrontmatter extracts YAML/TOML frontmatter from the document.
func (p *Parser) parseFrontmatter(doc *Document, content string) string {
	content = strings.TrimLeft(content, "\n\r\t ")
	if !strings.HasPrefix(content, "---") {
		return content
	}

	endIdx := strings.Index(content[3:], "\n---")
	if endIdx < 0 {
		return content
	}
	endIdx += 3 // Adjust for the leading "---"

	fmRaw := content[3:endIdx]
	doc.Frontmatter = Frontmatter{
		Raw:    strings.TrimSpace(fmRaw),
		Format: "yaml",
		Data:   make(map[string]interface{}),
	}

	// Parse YAML
	if err := yaml.Unmarshal([]byte(fmRaw), &doc.Frontmatter.Data); err != nil {
		// If YAML fails, try TOML-like simple parsing
		doc.Frontmatter.Format = "unknown"
		doc.Frontmatter.Data = parseSimpleFrontmatter(fmRaw)
	}

	// Check for TOML frontmatter (+++ delimiters)
	if strings.HasPrefix(content, "+++") {
		endIdx = strings.Index(content[3:], "\n+++")
		if endIdx >= 0 {
			endIdx += 3
			fmRaw = content[3:endIdx]
			doc.Frontmatter = Frontmatter{
				Raw:    strings.TrimSpace(fmRaw),
				Format: "toml",
				Data:   make(map[string]interface{}),
			}
			parseSimpleTOML(doc.Frontmatter.Data, fmRaw)
		}
	}

	// Return content after frontmatter
	afterEnd := endIdx + 4 // skip trailing ---\n
	if afterEnd >= len(content) {
		return ""
	}
	return content[afterEnd:]
}

// parseHeading extracts a heading from the current line.
func (p *Parser) parseHeading(lines []string, idx int, level int) Heading {
	line := strings.TrimSpace(lines[idx])
	text := strings.TrimPrefix(line, strings.Repeat("#", level)+" ")
	text = strings.TrimSpace(text)

	return Heading{
		Level:    level,
		Text:     text,
		Position: idx,
		Slug:     slugify(text),
	}
}

// parseCodeBlock extracts a fenced code block spanning multiple lines.
func (p *Parser) parseCodeBlock(lines []string, idx int) (CodeBlock, int) {
	firstLine := strings.TrimSpace(lines[idx])
	lang := strings.TrimPrefix(firstLine, "```")
	lang = strings.TrimSpace(lang)

	var contentLines []string
	consumed := 1
	for j := idx + 1; j < len(lines); j++ {
		consumed++
		if strings.TrimSpace(lines[j]) == "```" {
			break
		}
		contentLines = append(contentLines, lines[j])
	}

	content := strings.Join(contentLines, "\n")
	return CodeBlock{
		Language:   lang,
		Content:    content,
		Position:   idx,
		TokenCount: CountTokens(content),
	}, consumed
}

// parseTable extracts a Markdown table.
func (p *Parser) parseTable(lines []string, idx int) (*Table, int) {
	if idx+1 >= len(lines) {
		return nil, 1
	}

	headerLine := strings.TrimSpace(lines[idx])
	headers := parseTableRow(headerLine)
	if len(headers) == 0 {
		return nil, 1
	}

	sepLine := strings.TrimSpace(lines[idx+1])
	if !strings.Contains(sepLine, "---") {
		return nil, 1
	}

	table := &Table{
		Headers:  headers,
		Position: idx,
	}

	consumed := 2
	for j := idx + 2; j < len(lines); j++ {
		rowLine := strings.TrimSpace(lines[j])
		if !strings.HasPrefix(rowLine, "|") {
			break
		}
		row := parseTableRow(rowLine)
		if len(row) > 0 {
			table.Rows = append(table.Rows, row)
		}
		consumed++
	}

	return table, consumed
}

// parseTableRow parses a Markdown table row into cells.
func parseTableRow(line string) []string {
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")
	cells := make([]string, 0, len(parts))
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}

// parseList extracts a Markdown list (ordered or unordered).
func (p *Parser) parseList(lines []string, idx int) (List, int) {
	list := List{Position: idx}
	consumed := 0
	orderedItemRe := regexp.MustCompile(`^\d+\.\s`)

	for j := idx; j < len(lines); j++ {
		line := lines[j]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			break
		}

		// Check for unordered list
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
			list.Ordered = false
			item := strings.TrimLeft(trimmed, "-*+ ")
			list.Items = append(list.Items, item)
			consumed++
			continue
		}

		// Check for ordered list
		if orderedItemRe.MatchString(trimmed) {
			list.Ordered = true
			item := orderedItemRe.ReplaceAllString(trimmed, "")
			list.Items = append(list.Items, item)
			consumed++
			continue
		}

		break
	}

	return list, consumed
}

// extractLinks parses inline links from text: [text](url) and [text][ref]
func extractLinks(text string, position int) []Link {
	var links []Link

	// Match [text](url)
	inlineRe := regexp.MustCompile(`\[([^\]]*)\]\(([^)]*)\)`)
	matches := inlineRe.FindAllStringSubmatchIndex(text, -1)
	for _, match := range matches {
		if len(match) >= 4 {
			links = append(links, Link{
				Text:     text[match[2]:match[3]],
				URL:      text[match[4]:match[5]],
				Position: position + match[0],
			})
		}
	}

	// Match ![text](url) - images
	imgRe := regexp.MustCompile(`!\[([^\]]*)\]\(([^)]*)\)`)
	imgMatches := imgRe.FindAllStringSubmatchIndex(text, -1)
	for _, match := range imgMatches {
		if len(match) >= 4 {
			links = append(links, Link{
				Text:     text[match[2]:match[3]],
				URL:      text[match[4]:match[5]],
				Position: position + match[0],
				IsImage:  true,
			})
		}
	}

	return links
}

// extractReferences finds reference-style link definitions: [id]: url "title"
func extractReferences(content string) []Reference {
	var refs []Reference
	re := regexp.MustCompile(`(?m)^\[([^\]]+)\]:\s+(\S+)(?:\s+"([^"]*)")?`)
	matches := re.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			ref := Reference{
				ID:  match[1],
				URL: match[2],
			}
			if len(match) >= 4 {
				ref.Title = match[3]
			}
			refs = append(refs, ref)
		}
	}
	return refs
}

// parseSimpleFrontmatter does basic key: value parsing for malformed frontmatter.
func parseSimpleFrontmatter(raw string) map[string]interface{} {
	data := make(map[string]interface{})
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if parts := strings.SplitN(line, ":", 2); len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, "\"'")
			data[key] = val
		}
	}
	return data
}

// parseSimpleTOML does basic key = "value" parsing for TOML.
func parseSimpleTOML(data map[string]interface{}, raw string) {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, "\"'")
			data[key] = val
		}
	}
}

// slugify converts text to a URL-friendly slug.
func slugify(text string) string {
	text = strings.ToLower(text)
	text = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(text, "-")
	text = strings.Trim(text, "-")
	return text
}

// CountTokens counts approximate tokens in text (4 chars ≈ 1 token).
func CountTokens(text string) int {
	return utf8.RuneCountInString(text) / 4
}

// ToJSON serializes the document to JSON for embedding storage.
func (d *Document) ToJSON() (string, error) {
	data, err := json.Marshal(d)
	if err != nil {
		return "", fmt.Errorf("marshal document: %w", err)
	}
	return string(data), nil
}

// DocumentSummary returns a compact summary of the document for display.
func (d *Document) DocumentSummary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Title: %s\n", d.Title)
	fmt.Fprintf(&b, "Headings: %d\n", len(d.Headings))
	fmt.Fprintf(&b, "Code Blocks: %d\n", len(d.CodeBlocks))
	fmt.Fprintf(&b, "Tables: %d\n", len(d.Tables))
	fmt.Fprintf(&b, "Links: %d\n", len(d.Links))
	fmt.Fprintf(&b, "Tokens: ~%d\n", d.TokenCount)
	if len(d.Frontmatter.Data) > 0 {
		fmt.Fprintf(&b, "Frontmatter keys: %d\n", len(d.Frontmatter.Data))
	}
	return b.String()
}

// IsCodeBlockAt checks if the given line index starts a code block.
func IsCodeBlockAt(lines []string, idx int) bool {
	if idx < 0 || idx >= len(lines) {
		return false
	}
	trimmed := strings.TrimSpace(lines[idx])
	return strings.HasPrefix(trimmed, "```")
}

// GetHeadingLevel returns the heading level of a line (0 if not a heading).
func GetHeadingLevel(line string) int {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "# ") {
		return 1
	}
	if strings.HasPrefix(trimmed, "## ") {
		return 2
	}
	if strings.HasPrefix(trimmed, "### ") {
		return 3
	}
	if strings.HasPrefix(trimmed, "#### ") {
		return 4
	}
	if strings.HasPrefix(trimmed, "##### ") {
		return 5
	}
	if strings.HasPrefix(trimmed, "###### ") {
		return 6
	}
	return 0
}

// ensure int parsing available
var _ = strconv.Atoi
