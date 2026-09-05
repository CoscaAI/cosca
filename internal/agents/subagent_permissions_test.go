package agents

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/permission"
)

func TestDeriveSubagentSessionPermission_PreservesParentDeny(t *testing.T) {
	parent := permission.Ruleset{
		{Permission: "shell", Pattern: "*", Action: permission.Deny},
		{Permission: "web_fetch", Pattern: "*", Action: permission.Deny},
		{Permission: "read", Pattern: "*", Action: permission.Allow},
	}
	sub := permission.Ruleset{{Permission: "write_file", Pattern: "*", Action: permission.Allow}}

	got := DeriveSubagentSessionPermission(parent, sub)

	if !hasRule(got, "shell", "*", permission.Deny) {
		t.Error("expected parent deny on shell to be preserved")
	}
	if !hasRule(got, "web_fetch", "*", permission.Deny) {
		t.Error("expected parent deny on web_fetch to be preserved")
	}
	if hasRule(got, "read", "*", permission.Allow) {
		t.Error("parent allow on read must NOT be inherited (only deny + external_directory)")
	}
}

func TestDeriveSubagentSessionPermission_PreservesExternalDirectory(t *testing.T) {
	parent := permission.Ruleset{
		{Permission: "external_directory", Pattern: "/data/**", Action: permission.Allow},
	}
	sub := permission.Ruleset{}

	got := DeriveSubagentSessionPermission(parent, sub)
	if !hasRule(got, "external_directory", "/data/**", permission.Allow) {
		t.Error("expected external_directory rule to be preserved")
	}
}

func TestDeriveSubagentSessionPermission_DeniesTaskWhenNotPermitted(t *testing.T) {
	parent := permission.Ruleset{}
	sub := permission.Ruleset{}

	got := DeriveSubagentSessionPermission(parent, sub)
	if !hasRule(got, "task", "*", permission.Deny) {
		t.Error("expected task deny to be added when subagent does not permit it")
	}
	if !hasRule(got, "todowrite", "*", permission.Deny) {
		t.Error("expected todowrite deny to be added when subagent does not permit it")
	}
}

func TestDeriveSubagentSessionPermission_KeepsTaskWhenPermitted(t *testing.T) {
	parent := permission.Ruleset{}
	sub := permission.Ruleset{{Permission: "task", Pattern: "*", Action: permission.Allow}}

	got := DeriveSubagentSessionPermission(parent, sub)
	if hasRule(got, "task", "*", permission.Deny) {
		t.Error("expected no task deny when the subagent already permits task")
	}
}

func TestDeriveSubagentSessionPermission_KeepsTodoWhenPermitted(t *testing.T) {
	parent := permission.Ruleset{}
	sub := permission.Ruleset{
		{Permission: "todowrite", Pattern: "*", Action: permission.Allow},
		{Permission: "task", Pattern: "*", Action: permission.Allow},
	}

	got := DeriveSubagentSessionPermission(parent, sub)
	if hasRule(got, "task", "*", permission.Deny) {
		t.Error("expected no task deny when permitted")
	}
	if hasRule(got, "todowrite", "*", permission.Deny) {
		t.Error("expected no todowrite deny when permitted")
	}
}

func TestDeriveSubagentSessionPermission_ScopedTaskDenyIsNotPermission(t *testing.T) {
	// A scoped rule that does NOT resolve to allow on "*" must not count as
	// permitting task — the child still gets the deny.
	parent := permission.Ruleset{}
	sub := permission.Ruleset{{Permission: "task", Pattern: "internal/**", Action: permission.Deny}}

	got := DeriveSubagentSessionPermission(parent, sub)
	if !hasRule(got, "task", "*", permission.Deny) {
		t.Error("expected task deny to be added (scoped deny does not permit the tool)")
	}
}

func TestDeriveSubagentSessionPermission_EmptyParentDefaultsToDenyChild(t *testing.T) {
	got := DeriveSubagentSessionPermission(nil, nil)
	if !hasRule(got, "task", "*", permission.Deny) {
		t.Error("expected task deny by default")
	}
	if !hasRule(got, "todowrite", "*", permission.Deny) {
		t.Error("expected todowrite deny by default")
	}
}

// hasRule reports whether the ruleset contains a rule with the given fields.
func hasRule(rs permission.Ruleset, perm, pattern string, action permission.Action) bool {
	for _, r := range rs {
		if r.Permission == perm && r.Pattern == pattern && r.Action == action {
			return true
		}
	}
	return false
}
