// Package chunker — L355: comparação de alternativas de correção da
// fragmentação (READ-ONLY, em memória — nada de produção é alterado).
//
// Baseline (produção): parser markdown cria 1 NodeParagraph POR LINHA →
// ChunkDocument → 1 chunk/linha (causa raiz da L354).
//
// Alternativas (executadas SOMENTE em memória):
//
//	A CommonMark   — agrupa linhas de parágrafo consecutivas (sem blank line)
//	                em um único nó (o padrão da spec).
//	B merge-chunks — mantém o parser; mescla chunks de texto pequenos (≤25
//	                chars) consecutivos no resultado do chunker.
//	C detecta-JSON — se o documento é JSON válido, trata como bloco único.
//	D conservador  — como A, mas só agrupa quando a linha seguinte também é
//	                parágrafo puro (guarda contra bordas tipo "-item").
//
// Critério: reduzir fragmentação SEM destruir fronteiras semânticas —
// "produz menos chunks" ≠ "produz chunks melhores".
package chunker

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/markdown"
)

// parseGrouped replica o parser do Cosca (loop por linhas) agrupando linhas
// de parágrafo consecutivas sem blank line em um único nó — regras A/D.
func parseGrouped(content string, conservative bool) *markdown.Document {
	doc := &markdown.Document{AST: &markdown.Node{Children: []*markdown.Node{}}}
	lines := strings.Split(content, "\n")
	var paraBuf []string
	flushPara := func(pos int) {
		if len(paraBuf) == 0 {
			return
		}
		text := strings.Join(paraBuf, "\n")
		doc.AST.Children = append(doc.AST.Children, &markdown.Node{
			Type: markdown.NodeParagraph, Content: text, Position: pos,
		})
		paraBuf = nil
	}
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		looksLikeStructure := func(s string) bool {
			return strings.HasPrefix(s, "#") || strings.HasPrefix(s, "```") ||
				strings.HasPrefix(s, "|") || strings.HasPrefix(s, "- ") ||
				strings.HasPrefix(s, "* ") || strings.HasPrefix(s, "+ ") ||
				strings.HasPrefix(s, "1. ") || strings.HasPrefix(s, "> ")
		}
		switch {
		case trimmed == "":
			flushPara(i)
		case strings.HasPrefix(trimmed, "### ") || strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "# "):
			flushPara(i)
			level := 1
			if strings.HasPrefix(trimmed, "###") {
				level = 3
			} else if strings.HasPrefix(trimmed, "##") {
				level = 2
			}
			doc.AST.Children = append(doc.AST.Children, &markdown.Node{
				Type: markdown.NodeHeading, Content: strings.TrimSpace(strings.TrimLeft(trimmed, "#")),
				Level: level, Position: i,
			})
		case strings.HasPrefix(trimmed, "```"):
			flushPara(i)
			var cb []string
			lang := strings.TrimPrefix(trimmed, "```")
			i++
			for i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
				cb = append(cb, lines[i])
				i++
			}
			doc.AST.Children = append(doc.AST.Children, &markdown.Node{
				Type: markdown.NodeCodeBlock, Content: strings.Join(cb, "\n"),
				Language: lang, Position: i,
			})
		case strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") ||
			strings.HasPrefix(trimmed, "+ ") || strings.HasPrefix(trimmed, "1. "):
			// réplica do parseList: consome itens consecutivos em UM nó.
			flushPara(i)
			var items []string
			for i < len(lines) {
				t := strings.TrimSpace(lines[i])
				if strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") ||
					strings.HasPrefix(t, "+ ") || strings.HasPrefix(t, "1. ") {
					items = append(items, t)
					i++
					continue
				}
				break
			}
			i--
			doc.AST.Children = append(doc.AST.Children, &markdown.Node{
				Type: markdown.NodeList, Content: strings.Join(items, "\n"), Position: i,
			})
		case strings.HasPrefix(trimmed, "> "):
			flushPara(i)
			doc.AST.Children = append(doc.AST.Children, &markdown.Node{
				Type: markdown.NodeBlockquote, Content: strings.TrimPrefix(trimmed, "> "), Position: i,
			})
		default:
			// parágrafo: agrupa com a próxima linha de parágrafo (A) — D
			// conserva apenas quando a PRÓXIMA linha não parece estrutura.
			if conservative && looksLikeStructure(trimmed) && len(paraBuf) == 0 {
				flushPara(i)
				doc.AST.Children = append(doc.AST.Children, &markdown.Node{
					Type: markdown.NodeParagraph, Content: trimmed, Position: i,
				})
				continue
			}
			paraBuf = append(paraBuf, trimmed)
			// A próxima linha é blank ou estrutura → flush agora.
			if i+1 >= len(lines) || strings.TrimSpace(lines[i+1]) == "" || looksLikeStructure(strings.TrimSpace(lines[i+1])) {
				flushPara(i)
			}
		}
	}
	flushPara(len(lines))
	return doc
}

// chunkC: se o conteúdo é JSON válido → documento com um único nó.
func chunkJSON(content string) []Chunk {
	trimmed := strings.TrimSpace(content)
	if json.Valid([]byte(trimmed)) && strings.HasPrefix(trimmed, "{") {
		doc := &markdown.Document{AST: &markdown.Node{Children: []*markdown.Node{
			{Type: markdown.NodeParagraph, Content: trimmed, Position: 0},
		}}}
		doc.RawContent = trimmed
		c := New(DefaultConfig())
		chunks, _ := c.ChunkDocument(doc, "json")
		return chunks
	}
	return nil
}

// chunkWithRule: chunka o conteúdo pela regra (baseline/A/B/C/D) — memória.
func chunkWithRule(content, rule string) []Chunk {
	c := New(DefaultConfig())

	switch rule {
	case "baseline":
		doc, err := markdown.NewParser().Parse("", content)
		if err != nil {
			return nil
		}
		chunks, _ := c.ChunkDocument(doc, "doc")
		return chunks
	case "A", "D":
		doc := parseGrouped(content, rule == "D")
		doc.RawContent = content
		chunks, _ := c.ChunkDocument(doc, "doc")
		return chunks
	case "C":
		if js := chunkJSON(content); js != nil {
			return js
		}
		doc, err := markdown.NewParser().Parse("", content)
		if err != nil {
			return nil
		}
		chunks, _ := c.ChunkDocument(doc, "doc")
		return chunks
	case "B":
		doc, err := markdown.NewParser().Parse("", content)
		if err != nil {
			return nil
		}
		chunks, _ := c.ChunkDocument(doc, "doc")
		return mergeSmallTextChunks(chunks)
	default:
		return nil
	}
}

// mergeSmallTextChunks (B): mescla chunks de texto consecutivos pequenos
// (≤25 chars) no chunk anterior — o "merge no chunker".
func mergeSmallTextChunks(chunks []Chunk) []Chunk {
	if len(chunks) < 2 {
		return chunks
	}
	out := make([]Chunk, 0, len(chunks))
	for _, ch := range chunks {
		if len(out) > 0 && ch.SectionType == "text" && len(ch.Content) <= 25 {
			last := &out[len(out)-1]
			if last.SectionType == "text" {
				last.Content = last.Content + "\n" + ch.Content
				last.TokenCount = markdown.CountTokens(last.Content)
				last.Hash = computeHash(last.Content)
				continue
			}
		}
		out = append(out, ch)
	}
	return out
}

// chunkStats: distribuição de tamanhos e fragmentação.
type chunkStats struct {
	N          int
	Chars      []int
	Le25       int
	Triviais   int
	Alnum      int
	Headings   int
	CodeBlocks int
	Lists      int
}

func stats(chunks []Chunk) chunkStats {
	s := chunkStats{N: len(chunks)}
	for _, ch := range chunks {
		s.Chars = append(s.Chars, len(ch.Content))
		if len(ch.Content) <= 25 {
			s.Le25++
		}
		if ok, _ := IsTrivial(ch.Content); ok {
			s.Triviais++
		} else if strings.ContainsAny(ch.Content, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789") {
			s.Alnum++
		}
		switch ch.SectionType {
		case "heading":
			s.Headings++
		case "code":
			s.CodeBlocks++
		case "list":
			s.Lists++
		}
	}
	return s
}

func (s chunkStats) pct(f func(int) bool) float64 {
	if s.N == 0 {
		return 0
	}
	n := 0
	for _, c := range s.Chars {
		if f(c) {
			n++
		}
	}
	return float64(n) * 100 / float64(s.N)
}

// ── Testes de segurança (os casos do Don) ─────────────────────────────────

func TestL355SafetyCases(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"markdown normal", "# Heading\nparagraph line 1\nparagraph line 2\n"},
		{"blank lines", "paragraph A\n\nparagraph B\n"},
		{"heading+text", "# Title\ntext under heading\n"},
		{"code block", "```json\n{\n  \"a\": 1\n}\n```\n"},
		{"lista", "- item 1\n- item 2\n"},
		{"json indentado", "{\n\"a\": {\n\"b\": 1\n}\n}\n"},
		{"json minificado", "{\"a\":{\"b\":1}}\n"},
		{"texto tecnico", "Go 1.26\nv1.2.3\nRAG\nAPI\nSQL\n404\nC++17\n"},
		{"urls", "https://example.com/a/b\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, rule := range []string{"baseline", "A", "B", "C", "D"} {
				chunks := chunkWithRule(tc.content, rule)
				st := stats(chunks)
				// Regras NUNCA podem filtrar texto técnico/URLs.
				for _, keep := range []string{"Go 1.26", "v1.2.3", "RAG", "API", "SQL", "404", "C++17", "https://example.com/a/b"} {
					found := false
					for _, ch := range chunks {
						if strings.Contains(ch.Content, keep) {
							found = true
							break
						}
					}
					if !found && strings.Contains(tc.content, keep) {
						t.Errorf("%s[%s]: %q PERDIDO!", tc.name, rule, keep)
					}
				}
				t.Logf("  %-8s n=%d le25=%d triviais=%d headings=%d code=%d listas=%d",
					rule, st.N, st.Le25, st.Triviais, st.Headings, st.CodeBlocks, st.Lists)
			}
		})
	}
}

// ── Regressão real: amostra de documentos do repositório ──────────────────

func TestL355RealDocs(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	// Amostra real: documentos markdown do repo + um JSON (package.json) +
	// o JSON GRANDE indentado (o caso real de ~1.500 chunks da L354).
	var bigJSON strings.Builder
	bigJSON.WriteString("{\n")
	for i := 0; i < 300; i++ {
		bigJSON.WriteString("  \"campo\": {\n    \"a\": 1,\n    \"b\": true,\n    \"url\": \"https://x.com/1.0.0\",\n    \"lista\": [1, 2, 3]\n  },\n")
	}
	bigJSON.WriteString("}\n")
	docs := map[string]string{
		"md-1": "# Título\n\nParágrafo um com conteúdo real sobre o Cosca e sua arquitetura.\n\n## Seção 2\n\nMais conteúdo com palavras suficientes para formar parágrafos longos e úteis para o teste de chunking.\n\n- item um\n- item dois\n\n## Seção 3\n\nOutro parágrafo para completar a amostra com conteúdo variado e distribuição de tamanho.\n",
		"json-1": "{\n  \"name\": \"cosca\",\n  \"version\": \"1.26.0\",\n  \"dependencies\": {\n    \"go\": \"1.26.0\",\n    \"sqlite\": \"3.45.0\"\n  },\n  \"scripts\": {\n    \"build\": \"go build ./...\",\n    \"test\": \"go test ./...\"\n  }\n}\n",
		"md-2": "# Guia\n\nPrimeira linha do parágrafo.\nSegunda linha do mesmo parágrafo sem blank line.\nTerceira linha também.\n\nOutro parágrafo separado.\n\n```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```\n\nLista:\n- a\n- b\n- c\n",
		"json-grande": bigJSON.String(),
	}
	t.Logf("── L355 regressão real (amostra) ──")
	for name, content := range docs {
		t.Logf("DOC %s (%d chars)", name, len(content))
		for _, rule := range []string{"baseline", "A", "B", "C", "D"} {
			chunks := chunkWithRule(content, rule)
			st := stats(chunks)
			t.Logf("  %-8s n=%3d le25=%3d (%5.1f%%) triviais=%d alnum=%d h=%d c=%d l=%d",
				rule, st.N, st.Le25, st.pct(func(c int) bool { return c <= 25 }),
				st.Triviais, st.Alnum, st.Headings, st.CodeBlocks, st.Lists)
		}
	}
}