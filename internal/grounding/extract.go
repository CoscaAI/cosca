package grounding

import (
	"regexp"
	"strings"
)

// DefaultCitePatterns são os marcadores de citação reconhecidos por padrão
// pela extração de claims:
//
//	[RAG:...]   citação explícita a um chunk id no formato RAG
//	[Chunk N]   citação posicional a um chunk retornado
//	[K-xxxx]    citação a um item de conhecimento (CL-xxxx é diferente —
//	            aqui é o K- do KnowledgeItem)
//
// O chamador pode passar patterns próprios em ExtractClaims; estes são o
// default conservador.
var DefaultCitePatterns = []string{
	`(?i)\[RAG:[^\]]+\]`,
	`(?i)\[Chunk\s+\d+\]`,
	`(?i)\[K-\d+\]`,
}

// sentenceSplitRe quebra a resposta em sentenças: cada captura é um trecho
// delimitado por pontuação terminal (. ! ? ;) ou quebra de linha. É a forma
// mais simples e determinística de "quebrar em sentenças" — sem NLP, sem LLM.
var sentenceSplitRe = regexp.MustCompile(`[^.!?;\r\n]+[.!?;]*`)

// extractCiteRe compila os patterns de citação num único regex de busca. Um
// pattern vazio é ignorado.
func extractCiteRe(patterns []string) *regexp.Regexp {
	cleaned := make([]string, 0, len(patterns))
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		cleaned = append(cleaned, p)
	}
	if len(cleaned) == 0 {
		cleaned = DefaultCitePatterns
	}
	return regexp.MustCompile(strings.Join(cleaned, "|"))
}

// ExtractClaims quebra o texto da resposta em sentenças e marca cada uma com
// um Claim. HasCitation é true quando a sentença contém um marcador de
// citação reconhecido (citePatterns); CitationMarker guarda o marcador bruto.
//
// A extração é puramente textual e determinística: não chama nenhum modelo, e
// cada claim carrega somente o texto + o marcador de citação (a verificação
// de suporte fica para Verify).
func ExtractClaims(text string, citePatterns []string) []Claim {
	re := extractCiteRe(citePatterns)
	raw := sentenceSplitRe.FindAllString(text, -1)

	claims := make([]Claim, 0, len(raw))
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		hasCite := false
		marker := ""
		if m := re.FindAllString(s, -1); len(m) > 0 {
			hasCite = true
			marker = m[0]
		}
		claims = append(claims, Claim{
			Text:           strings.TrimSpace(strings.TrimSpace(s)),
			HasCitation:    hasCite,
			CitationMarker: marker,
		})
	}
	return claims
}
