package policy

import "strings"

// MatchesDangerousPattern detecta padrões perigosos de comando (finding V13).
// A detecção é determinística na camada de matching por substrings e operadores
// de shell — NÃO é um parser de shell/AST completo (over-engineering). Retorna
// o padrão casado e true, ou ("", false) para um comando limpo.
//
// Padrões reconhecidos:
//
//   - "rm -rf" / "rm -r " / "rm -fr"  → destrutivo (P1, L358)
//   - "sudo"                          → elevação de privilégio
//   - "curl ... | sh / bash"          → download-e-executa
//   - "ssh"                           → sem allowlist
//   - "git push --force" / "git push -f" → reescrita de histórico
//   - operadores de shell "|", "&&", ";", "`" → composição/comando injetável
func MatchesDangerousPattern(cmd string) (pattern string, ok bool) {
	if strings.TrimSpace(cmd) == "" {
		return "", false
	}

	// Reusa o helper de matching de substrings usado em DefaultRules.
	has := func(s, sub string) bool { return strings.Contains(s, sub) }

	switch {
	case has(cmd, "rm -rf") || has(cmd, "rm -r ") || has(cmd, "rm -fr"):
		return "rm -rf", true
	case has(cmd, "sudo"):
		return "sudo", true
	case has(cmd, "curl") && has(cmd, "|") && (has(cmd, "sh") || has(cmd, "bash")):
		return "curl|sh", true
	case has(cmd, "ssh"):
		return "ssh", true
	case has(cmd, "git push --force") || has(cmd, "git push -f"):
		return "git push --force", true
	case has(cmd, "|"):
		return "|", true
	case has(cmd, "&&"):
		return "&&", true
	case has(cmd, ";"):
		return ";", true
	case has(cmd, "`"):
		return "`", true
	default:
		return "", false
	}
}
