package ui

import "strings"

// renderStreaming renderiza o buffer da resposta em andamento com um cursor
// de atividade (▍) para dar feedback visual de streaming.
func renderStreaming(role, text string) string {
	if text == "" {
		return ""
	}
	switch role {
	case "assistant":
		return assistantBubble.Render(text) + headerSubStyle.Render("▍")
	case "tool":
		return toolBubble.Render(text) + headerSubStyle.Render("▍")
	case "subagent":
		return subagentBubble.Render(text) + headerSubStyle.Render("▍")
	default:
		return text + headerSubStyle.Render("▍")
	}
}

// renderStreamingPlain é uma variante sem estilos, útil para testes de
// conteúdo puro (verifica que o texto do buffer aparece com o cursor).
func renderStreamingPlain(role, text string) string {
	return renderStreaming(role, text)
}

// ─── Helpers de strings usados nos testes ─────────────────────────────────────

// cleanRendered remove códigos ANSI e seqüências de estilo para inspeção
// textual em testes. Não é usado em produção.
func cleanRendered(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == 'm' {
				inEscape = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
