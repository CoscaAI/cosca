// This file implements the secure delegation of permissions to subagent
// sessions (inspired by OpenCode's packages/opencode/src/agent/subagent-permissions.ts).
//
// A subagent spawned via the task tool must NOT inherit the parent session's
// full authority. It keeps only the parent's deny rules (so a restriction the
// parent enforces is never silently lifted) and any external_directory rules
// (which legitimately govern workspace-escape). The subagent's OWN permissions
// determine what it may do; if the subagent does not already permit the
// dangerous `task` and `todowrite` tools, they are explicitly denied.
package agents

import "github.com/CoscaAI/cosca/internal/permission"

// DeriveSubagentSessionPermission builds the permission ruleset for a subagent
// session from the parent session's ruleset and the subagent's own ruleset.
//
// Rules preserved from the parent:
//   - every rule whose action is deny (restrictions are never lifted);
//   - every rule whose permission is "external_directory" (legitimizes the
//     workspace-escape grant required by some subagents).
//
// Rules added to the child:
//   - a deny for "todowrite" when the subagent's own ruleset does not already
//     permit it;
//   - a deny for "task" when the subagent's own ruleset does not already
//     permit it. A subagent cannot recursively spawn subagents unless explicitly
//     allowed.
func DeriveSubagentSessionPermission(parent permission.Ruleset, subagentPermission permission.Ruleset) permission.Ruleset {
	out := make(permission.Ruleset, 0, len(parent)+2)
	for _, r := range parent {
		if r.Action == permission.Deny || r.Permission == "external_directory" {
			out = append(out, r)
		}
	}
	if !permits(subagentPermission, "todowrite") {
		out = append(out, permission.Rule{Permission: "todowrite", Pattern: "*", Action: permission.Deny})
	}
	if !permits(subagentPermission, "task") {
		out = append(out, permission.Rule{Permission: "task", Pattern: "*", Action: permission.Deny})
	}
	return out
}

// permits reports whether the ruleset resolves to "allow" for the given
// permission against the global "any target" pattern. It uses the same
// resolution as the executor (most-specific-wins, default ask), so a subagent
// is considered to permit a tool only when it has an allow rule for it.
func permits(rs permission.Ruleset, perm string) bool {
	if len(rs) == 0 {
		return false
	}
	return permission.Evaluate(perm, "*", rs).Action == permission.Allow
}
