package evals

import "strings"

// isCanaryLine reports whether line is a canary marker comment: an HTML
// comment (`<!-- ... -->`) or a hash comment (`# ...`) whose content contains
// the word "canary" (case-insensitive). These markers are embedded by
// benchmark authors to prevent training-data contamination: a pipeline that
// has seen the canary hash during training reproduces it verbatim.
func isCanaryLine(line string) bool {
	s := strings.TrimSpace(line)
	if s == "" || !strings.Contains(strings.ToLower(s), "canary") {
		return false
	}
	switch {
	case strings.HasPrefix(s, "<!--") && strings.Contains(s, "-->"):
		return true
	case strings.HasPrefix(s, "#"):
		return true
	default:
		return false
	}
}

// StripCanary removes canary marker lines from instruction text before it is
// sent to the pipeline (mirroring Harbor's Task stripping): every line that is
// a canary comment is dropped, blank lines that immediately follow a stripped
// canary are collapsed, and trailing blank lines are trimmed. Normal text —
// including paragraph separators between real instructions — is left intact.
func StripCanary(text string) string {
	if text == "" {
		return ""
	}

	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	inCanaryBlock := false
	for _, line := range lines {
		if isCanaryLine(line) {
			inCanaryBlock = true
			continue
		}
		if inCanaryBlock && strings.TrimSpace(line) == "" {
			// Blank line(s) following a canary marker are stripped too.
			continue
		}
		inCanaryBlock = false
		out = append(out, line)
	}

	// Trim trailing blank lines.
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}

	return strings.Join(out, "\n")
}
