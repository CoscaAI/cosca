package permission

import (
	"reflect"
	"strings"
	"testing"
)

// ─── matchWildcard ────────────────────────────────────────────────────────────

func TestMatchWildcard(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		s       string
		want    bool
	}{
		{"exact", "edit", "edit", true},
		{"exact-not", "edit", "read", false},
		{"single-star", "editor/**", "editor/read", true},
		{"single-star-deep", "editor/**", "editor/sub/sub/read", true},
		{"star-matches-everything", "*", "anything", true},
		{"glob", "*.go", "main.go", true},
		{"glob-not", "*.go", "main.rs", false},
		{"path-glob", "docs/**", "docs/guide.md", true},
		{"path-glob-sub", "docs/**", "docs/a/b.md", true},
		{"question-mark", "read?tool", "read_tool", true},
		{"case-insensitive", "Read", "read", true},
		{"case-insensitive-reverse", "read", "READ", true},
		{"star-still-any", "ed**i**t", "edit", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MatchWildcard(c.pattern, c.s); got != c.want {
				t.Errorf("MatchWildcard(%q,%q)=%v, want %v", c.pattern, c.s, got, c.want)
			}
		})
	}
}

// ─── Evaluate ──────────────────────────────────────────────────────────────────

func TestEvaluate_BasicActions(t *testing.T) {
	rs := Ruleset{
		{Permission: "read", Pattern: "*", Action: Allow},
		{Permission: "write", Pattern: "*", Action: Deny},
		{Permission: "edit", Pattern: "*", Action: Ask},
	}
	t.Run("allow", func(t *testing.T) {
		if got := Evaluate("read", "*", rs); got.Action != Allow {
			t.Errorf("expect allow, got %s", got.Action)
		}
	})
	t.Run("deny", func(t *testing.T) {
		if got := Evaluate("write", "*", rs); got.Action != Deny {
			t.Errorf("expect deny, got %s", got.Action)
		}
	})
	t.Run("ask", func(t *testing.T) {
		if got := Evaluate("edit", "*", rs); got.Action != Ask {
			t.Errorf("expect ask, got %s", got.Action)
		}
	})
	t.Run("default ask when no rule matches", func(t *testing.T) {
		r := Evaluate("bash", "*", rs)
		if r.Action != Ask {
			t.Errorf("expect ask default, got %s", r.Action)
		}
		if r.Permission != "bash" {
			t.Errorf("expect requested permission %q in default rule, got %q", "bash", r.Permission)
		}
		if r.Pattern != "*" {
			t.Errorf("expect default pattern *, got %q", r.Pattern)
		}
	})
	t.Run("empty ruleset defaults to ask", func(t *testing.T) {
		if got := Evaluate("read", "*"); got.Action != Ask {
			t.Errorf("expect ask, got %s", got.Action)
		}
	})
}

func TestEvaluate_MostSpecificWins(t *testing.T) {
	rs := Ruleset{
		{Permission: "edit", Pattern: "*", Action: Deny},
		{Permission: "edit", Pattern: "*.go", Action: Allow},
	}
	t.Run("specific overrides general", func(t *testing.T) {
		if got := Evaluate("edit", "main.go", rs); got.Action != Allow {
			t.Errorf("expect allow for *.go, got %s", got.Action)
		}
	})
	t.Run("general still applies to non-matching pattern", func(t *testing.T) {
		if got := Evaluate("edit", "README.md", rs); got.Action != Deny {
			t.Errorf("expect deny for README.md, got %s", got.Action)
		}
	})
	t.Run("wildcard permission most specific wins", func(t *testing.T) {
		rs2 := Ruleset{
			{Permission: "*", Pattern: "*", Action: Ask},
			{Permission: "editor/**", Pattern: "*", Action: Deny},
		}
		if got := Evaluate("editor/tool", "*", rs2); got.Action != Deny {
			t.Errorf("expect deny via editor/**, got %s", got.Action)
		}
	})
}

func TestEvaluate_TieBreakLastWins(t *testing.T) {
	// Equal specificity → the LATER rule wins (project overrides agent/global).
	rs := Ruleset{
		{Permission: "edit", Pattern: "*.go", Action: Deny},
		{Permission: "edit", Pattern: "*.go", Action: Allow},
	}
	if got := Evaluate("edit", "main.go", rs); got.Action != Allow {
		t.Errorf("expect last (allow) wins tie, got %s", got.Action)
	}
}

func TestEvaluate_MultipleRulesets(t *testing.T) {
	global := Ruleset{{Permission: "*", Pattern: "*", Action: Ask}}
	project := Ruleset{{Permission: "read", Pattern: "*", Action: Allow}}
	if got := Evaluate("read", "*", global, project); got.Action != Allow {
		t.Errorf("expect project allow, got %s", got.Action)
	}
	// Specific project rule beats global wildcard regardless of order.
	if got := Evaluate("read", "*", project, global); got.Action != Allow {
		t.Errorf("expect allow even when project first, got %s", got.Action)
	}
}

// ─── Merge ─────────────────────────────────────────────────────────────────────

func TestMerge(t *testing.T) {
	a := Ruleset{{Permission: "read", Pattern: "*", Action: Allow}}
	b := Ruleset{{Permission: "write", Pattern: "*", Action: Deny}}
	c := Ruleset{{Permission: "edit", Pattern: "*.go", Action: Ask}}
	merged := Merge(a, b, c)
	want := Ruleset{
		{Permission: "read", Pattern: "*", Action: Allow},
		{Permission: "write", Pattern: "*", Action: Deny},
		{Permission: "edit", Pattern: "*.go", Action: Ask},
	}
	if !reflect.DeepEqual(merged, want) {
		t.Errorf("Merge got %v, want %v", merged, want)
	}
	if len(Merge()) != 0 {
		t.Errorf("expected empty merge for no args")
	}
}

// ─── FromConfig ────────────────────────────────────────────────────────────────

func TestFromConfig(t *testing.T) {
	cfg := map[string]any{
		"glob":   "allow",
		"grep":   "allow",
		"read":   "allow",
		"edit":   "ask",
		"write":  "deny",
		"scoped": map[string]any{"*.go": "allow", "docs/**": "ask"},
	}
	rs := FromConfig(cfg)
	// sorted by permission name
	var gotPerms []string
	for _, r := range rs {
		gotPerms = append(gotPerms, r.Permission)
	}
	// "scoped" expands to one rule per pattern (2 patterns → 2 entries).
	wantPerms := "edit,glob,grep,read,scoped,scoped,write"
	if got := strings.Join(gotPerms, ","); got != wantPerms {
		t.Errorf("permissions got %s, want %s", got, wantPerms)
	}

	assertRule(t, rs, "edit", "*", Ask)
	assertRule(t, rs, "glob", "*", Allow)
	assertRule(t, rs, "grep", "*", Allow)
	assertRule(t, rs, "read", "*", Allow)
	assertRule(t, rs, "write", "*", Deny)
	// scoped expands into per-pattern rules
	assertRule(t, rs, "scoped", "*.go", Allow)
	assertRule(t, rs, "scoped", "docs/**", Ask)
}

func TestFromConfig_UnknownActionFallsBackToAsk(t *testing.T) {
	rs := FromConfig(map[string]any{"read": "bogus"})
	assertRule(t, rs, "read", "*", Ask)
}

func TestFromConfig_UnsupportedShapeFallsBackToAsk(t *testing.T) {
	rs := FromConfig(map[string]any{"read": []int{1}})
	assertRule(t, rs, "read", "*", Ask)
}

// ─── ToolPermission ────────────────────────────────────────────────────────────

func TestToolPermission(t *testing.T) {
	cases := map[string]string{
		"edit":                        "edit",
		"write":                       "edit",
		"apply_patch":                 "edit",
		"write_file":                  "edit",
		"edit_file":                   "edit",
		"read":                        "read",
		"read_file":                   "read",
		"list_dir":                    "read",
		"glob":                        "read",
		"read_mcp_resource":           "read",
		"list_mcp_resources":          "read",
		"read_mcp_custom_feed":        "read", // prefix read_mcp_*
		"shell":                       "shell",
		"web_fetch":                   "web_fetch",
		"list_mcp_resource_templates": "read",
	}
	for tool, want := range cases {
		if got := ToolPermission(tool); got != want {
			t.Errorf("ToolPermission(%q)=%q, want %q", tool, got, want)
		}
	}
}

// ─── Disabled / VisibleTools ───────────────────────────────────────────────────

func TestDisabled_GlobalDenyHidesTool(t *testing.T) {
	rs := Ruleset{{Permission: "edit", Pattern: "*", Action: Deny}}
	tools := []string{"read_file", "write_file", "edit_file", "shell"}
	disabled := Disabled(tools, rs)
	if !disabled["write_file"] {
		t.Error("write_file should be disabled (permission edit)")
	}
	if !disabled["edit_file"] {
		t.Error("edit_file should be disabled (permission edit)")
	}
	if disabled["read_file"] {
		t.Error("read_file should NOT be disabled")
	}
	if disabled["shell"] {
		t.Error("shell should NOT be disabled")
	}
}

func TestDisabled_ScopedDenyDoesNotHide(t *testing.T) {
	rs := Ruleset{{Permission: "edit", Pattern: "docs/**", Action: Deny}}
	tools := []string{"edit_file"}
	if d := Disabled(tools, rs); d["edit_file"] {
		t.Error("scoped deny (docs/**) must not disable the tool entirely")
	}
}

func TestDisabled_EmptyRulesetIsFailOpen(t *testing.T) {
	if d := Disabled([]string{"edit_file"}, nil); len(d) != 0 {
		t.Errorf("empty ruleset must disable nothing, got %v", d)
	}
}

func TestVisibleTools(t *testing.T) {
	rs := Ruleset{{Permission: "read", Pattern: "*", Action: Deny}}
	tools := []string{"read_file", "write_file", "shell"}
	got := VisibleTools(tools, rs)
	want := []string{"write_file", "shell"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("VisibleTools got %v, want %v", got, want)
	}
}

// ─── helpers ───────────────────────────────────────────────────────────────────

func assertRule(t *testing.T, rs Ruleset, perm, pattern string, want Action) {
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
