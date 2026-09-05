// contentfit.go — Extrator de CONTEÚDO ESSENCIAL (fit-content) para o LLM.
//
// Padrão minerado do crawl4ai (PruningContentFilter + content_scraping): ao
// extrair conteúdo de página, NÃO devolver HTML cru (cheio de script/style/nav)
// — devolver apenas o SEMÂNTICO, reduzindo volume/custo antes de qualquer
// chamada a LLM. Determinístico e barato (sem LLM).
//
// Faz "cleaning → selection" numa passada: remove tags de lixo (script/style/
// nav/footer/form/iframe), extrai texto dos elementos semânticos
// (article/main/section/p/h1-h6/strong/code), e devolve o texto limpo + um
// preview (charCount + snippet) para o caller decidir se vale escalar.
package contentfit

import (
	"strings"

	"golang.org/x/net/html"
)

// FitResult é o resultado da extração.
type FitResult struct {
	Text      string // texto essencial (tags de lixo removidas)
	Snippet   string // preview curto (primeiros ~300 chars)
	CharCount int    // tamanho do texto essencial
}

// tagsToDrop são tags que não carregam conteúdo semântico para o LLM.
var tagsToDrop = map[string]bool{
	"script": true, "style": true, "nav": true, "footer": true,
	"form": true, "iframe": true, "header": true, "aside": true,
	"noscript": true, "svg": true, "canvas": true,
}

// semanticTags são tags cujo texto importa (prioridade alta na extração).
var semanticTags = map[string]bool{
	"p": true, "article": true, "main": true, "section": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"li": true, "td": true, "th": true, "strong": true, "b": true, "code": true,
}

// Extract limpa o HTML e devolve o texto essencial. Útil para cosca.web:
// o caller pede o conteúdo, recebe o "fit" (não o HTML cru).
func Extract(htmlBody string) FitResult {
	doc, err := html.Parse(strings.NewReader(htmlBody))
	if err != nil {
		// Fallback: HTML inválido → devolve o texto cru (não perde nada).
		return FitResult{Text: htmlBody, Snippet: snippet(htmlBody), CharCount: len(htmlBody)}
	}

	var sb strings.Builder
	extractNodes(doc, &sb)

	text := collapseSpaces(sb.String())
	return FitResult{
		Text:      text,
		Snippet:   snippet(text),
		CharCount: len(text),
	}
}

// extractNodes percorre a árvore e acumula texto semântico, pulando lixo.
func extractNodes(n *html.Node, sb *strings.Builder) {
	if n.Type == html.TextNode {
		if n.Parent != nil && !tagsToDrop[n.Parent.Data] {
			// Adiciona texto de nós de texto cuja tag pai é semântica/aceita.
			if semanticTags[n.Parent.Data] || isContainerTag(n.Parent.Data) {
				sb.WriteString(n.Data)
				sb.WriteString(" ")
			}
		}
		return
	}
	if n.Type == html.ElementNode && tagsToDrop[n.Data] {
		return // pula subtree inteira (script/style/nav/...)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractNodes(c, sb)
	}
}

// isContainerTag aceita tags de container semântico (body/div/span com texto).
func isContainerTag(tag string) bool {
	switch tag {
	case "body", "div", "span", "pre", "blockquote", "dl", "dt", "dd", "ul", "ol":
		return true
	default:
		return false
	}
}

// collapseSpaces colapsa múltiplos espaços/quebras em um único espaço.
func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// snippet devolve os primeiros ~300 chars (preview barato).
func snippet(s string) string {
	const max = 300
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
