package config

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/permission"
	"gopkg.in/yaml.v3"
)

// TestConfig_PermissionRuleset_FromYAML verifies that a `permissions:` block in
// YAML is converted into a valid permission.Ruleset, covering both the string
// action form (permission -> "allow"/"ask"/"deny") and the per-pattern map form
// (permission -> { pattern: action }).
func TestConfig_PermissionRuleset_FromYAML(t *testing.T) {
	raw := `
provider: {name: deepseek}
permissions:
  glob: allow
  read: allow
  edit: ask
  write: deny
  scoped:
    "*.go": allow
    "docs/**": ask
`
	var c Config
	if err := yaml.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	rs := c.PermissionRuleset()
	if len(rs) == 0 {
		t.Fatal("expected a non-empty ruleset")
	}

	assertPermissionRule(t, rs, "read", "*", permission.Allow)
	assertPermissionRule(t, rs, "edit", "*", permission.Ask)
	assertPermissionRule(t, rs, "write", "*", permission.Deny)
	assertPermissionRule(t, rs, "glob", "*", permission.Allow)
	assertPermissionRule(t, rs, "scoped", "*.go", permission.Allow)
	assertPermissionRule(t, rs, "scoped", "docs/**", permission.Ask)
}

// TestConfig_PermissionRuleset_EmptyIsNil verifies that a config without a
// `permissions:` block yields a nil ruleset (fail-open at the executor level).
func TestConfig_PermissionRuleset_EmptyIsNil(t *testing.T) {
	var c Config
	if rs := c.PermissionRuleset(); rs != nil {
		t.Errorf("expected nil ruleset for empty config, got %v", rs)
	}
}

func TestConfig_PermissionRuleset_UnknownActionFallsBackToAsk(t *testing.T) {
	c := Config{Permissions: map[string]interface{}{"shell": "bogus"}}
	rs := c.PermissionRuleset()
	assertPermissionRule(t, rs, "shell", "*", permission.Ask)
}

func assertPermissionRule(t *testing.T, rs permission.Ruleset, perm, pattern string, want permission.Action) {
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
