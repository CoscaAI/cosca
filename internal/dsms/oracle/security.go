package oracle

import (
	"fmt"
	"strings"
)

// ============================================================
// SECURITY LAYER — fail-closed
// ============================================================

// forbidden queries that the oracle ALWAYS refuses.
// The intent that matters is EXTRACTION/EXFILTRATION of secrets,
// not mere mention. "analisa api key handling" is legitimate;
// "give me the api keys" is not.
var forbiddenPatterns = []string{
	"give me the secret",
	"give me the api",
	"give me the password",
	"show me the secret",
	"show me the api",
	"show passwords",
	"dumps all secrets",
	"dumps all api",
	"list all secrets",
	"list all api",
	"reveal token",
	"reveal secret",
	"reveal api",
	"password in config",
	"all the api keys",
	"all the secrets",
	"execute",
	"run command",
	"rm -rf",
	"delete ",
	"drop table",
	"access the internet",
	"http://",
	"https://",
	"curl ",
	"wget ",
}

// GuardResult is the result of the security guard check.
type GuardResult struct {
	Allowed   bool   `json:"allowed"`
	Reason    string `json:"reason"`
	Sanitized string `json:"sanitized"`
}

// SecurityGuard checks a query against the fail-closed policy.
func SecurityGuard(query string) *GuardResult {
	lower := strings.ToLower(strings.TrimSpace(query))

	// Empty query → deny (fail-closed)
	if lower == "" {
		return &GuardResult{Allowed: false, Reason: "empty query denied (fail-closed)"}
	}

	// Check forbidden patterns
	for _, pattern := range forbiddenPatterns {
		if strings.Contains(lower, pattern) {
			return &GuardResult{
				Allowed: false,
				Reason:  fmt.Sprintf("query matches forbidden pattern: %q", pattern),
			}
		}
	}

	// Sanitize output (redact secrets from any result)
	return &GuardResult{
		Allowed:   true,
		Sanitized: RedactSecrets(query),
	}
}

// RedactSecrets removes secret-like values from a string.
// The oracle NEVER returns secret VALUES — only flags their existence.
func RedactSecrets(s string) string {
	result := s
	for _, pattern := range []string{
		"sk-", "sk_", "AKIA", "ASAS", "ghp_", "gho_", "ghu_",
		"xoxb-", "xoxp-", "eyJ", "-----BEGIN", "AKIA",
	} {
		result = redactPattern(result, pattern)
	}
	return result
}

// redactPattern redacts a value that follows a known secret prefix.
func redactPattern(s, prefix string) string {
	idx := 0
	for {
		pos := strings.Index(s[idx:], prefix)
		if pos < 0 {
			break
		}
		pos += idx

		// Find end of the token (up to whitespace, quote, comma, etc.)
		end := pos + len(prefix)
		for end < len(s) {
			c := s[end]
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' ||
				c == '"' || c == '\'' || c == ',' || c == ')' || c == ']' || c == '}' {
				break
			}
			end++
		}

		// Replace with [REDACTED]
		s = s[:pos] + "[REDACTED]" + s[end:]
		idx = pos + len("[REDACTED]")
	}
	return s
}
