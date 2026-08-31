package config_test

import (
	"testing"

	chatconfig "github.com/CoscaAI/cosca/internal/chat/config"
	"github.com/CoscaAI/cosca/internal/permission"
	"gopkg.in/yaml.v3"
)

// TestPermissionRuleset_FromYAML verifies that the `permissions:` block in
// `.cosca/config.yaml` is converted into a valid permission.Ruleset via the
// Chat config loader, covering string actions and per-pattern maps.
func TestPermissionRuleset_FromYAML(t *testing.T) {
	raw := `
provider: {primary: deepseek}
permissions:
  read: allow
  edit: ask
  write: deny
  scoped:
    "*.go": allow
    "docs/**": ask
`
	var c chatconfig.Config
	if err := yaml.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	rs := c.PermissionRuleset()
	if len(rs) == 0 {
		t.Fatal("expected a non-empty ruleset")
	}

	assertChatRule(t, rs, "read", "*", permission.Allow)
	assertChatRule(t, rs, "edit", "*", permission.Ask)
	assertChatRule(t, rs, "write", "*", permission.Deny)
	assertChatRule(t, rs, "scoped", "*.go", permission.Allow)
	assertChatRule(t, rs, "scoped", "docs/**", permission.Ask)
}

// TestPermissionRuleset_EmptyIsNil verifies that a chat config without a
// `permissions:` block yields a nil ruleset (fail-open at the executor level).
func TestPermissionRuleset_EmptyIsNil(t *testing.T) {
	c := chatconfig.Config{}
	if rs := c.PermissionRuleset(); rs != nil {
		t.Errorf("expected nil ruleset for empty config, got %v", rs)
	}
}

func TestPermissionRuleset_UnknownActionFallsBackToAsk(t *testing.T) {
	c := chatconfig.Config{Permissions: map[string]interface{}{"shell": "bogus"}}
	rs := c.PermissionRuleset()
	assertChatRule(t, rs, "shell", "*", permission.Ask)
}

func assertChatRule(t *testing.T, rs permission.Ruleset, perm, pattern string, want permission.Action) {
	t.Helper()
	for _, r := range rs {
		if r.Permission == perm && r.Pattern == pattern {
			if r.Action != want {
				t.Errorf("rule %s:%s = %s, want %s", perm, pattern, r.Action, want)
			}
			return
		}
	}
	t.Errorf("rule %s:%s not found in %v", perm, pattern, rs)
}
