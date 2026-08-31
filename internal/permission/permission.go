// Package permission implements an allow/ask/deny permission ruleset system
// inspired by OpenCode's `permission` module. It evaluates which action applies
// to a (permission, pattern) pair using wildcard matching, resolving the most
// specific rule, and defaulting to "ask" (fail-safe) when no rule matches.
//
// Model (mirrors OpenCode packages/opencode/src/permission/index.ts):
//
//	type Rule{Permission, Pattern, Action}
//	type Ruleset []Rule          // a set of rules for one scope (project, agent, tool)
//	func Evaluate(perm, pattern, ...rulesets) Rule  // most specific wins, default ask
//	func Merge(...rulesets) Ruleset               // combine global + agent + project
//	func FromConfig(cfg map[string]any) Ruleset   // config -> ruleset
//	func Disabled / VisibleTools                  // hide denied tools from the LLM
//
// `Action` is one of allow / ask / deny. `deny` never lets the action through.
// `ask` produces a pending approval (in this codebase, with no confirmation UI
// yet, it is treated as a fail-closed deny with a clear log). `allow` executes.
//
// Fail-safe default: when NO rule in any ruleset matches the requested
// (permission, pattern), Evaluate returns a rule whose Action is `ask`. Callers
// that want "fail-open" (no enforcement) must check the absence of configured
// rules themselves (a nil/empty ruleset) rather than relying on Evaluate.
package permission

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

// Action is the outcome of a permission evaluation: allow, ask, or deny.
type Action string

const (
	// Allow permits the action unconditionally.
	Allow Action = "allow"
	// Ask requires an approval. Without a confirmation UI this is treated as a
	// fail-closed deny (with a clear log) by the executor; it is the fail-safe
	// default when no rule matches.
	Ask Action = "ask"
	// Deny rejects the action unconditionally.
	Deny Action = "deny"
)

// NormalizeAction lower-cases and trims a raw action string, returning it as an
// Action. Unknown values fall back to "ask" (fail-safe) so a typo cannot grant
// an unintended permission.
func NormalizeAction(s string) Action {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "allow":
		return Allow
	case "deny":
		return Deny
	case "ask", "":
		return Ask
	default:
		return Ask
	}
}

// Rule is a single permission assertion: Permission/Pattern describing the
// target, and the Action to take when both match.
type Rule struct {
	Permission string
	Pattern    string
	Action     Action
}

// String returns a human-readable representation (e.g. "edit:*.go allow").
func (r Rule) String() string {
	return fmt.Sprintf("%s:%s %s", r.Permission, r.Pattern, r.Action)
}

// Ruleset is an ordered collection of permission rules. Order matters only for
// tie-breaking: when two matching rules have equal specificity, the later rule
// wins. This gives agent/project rules (appended later by Merge) precedence
// over global rules.
type Ruleset []Rule

// MatchWildcard reports whether s matches the wildcard pattern. Both `*` (any
// sequence) and `**` (any sequence, path-aware) are treated as "any sequence";
// `?` matches a single character. Matching is case-insensitive so tools and
// permissions named in mixed case still resolve (aligned with OpenCode's `si`
// flag on Windows and the codebase's case-insensitive agent/tool lookups).
func MatchWildcard(pattern, s string) bool {
	return matchWildcard(pattern, s)
}

// matchWildcard compiles the glob into an anchored regexp and tests s.
func matchWildcard(pattern, s string) bool {
	p := strings.ToLower(pattern)
	str := strings.ToLower(s)
	var re strings.Builder
	re.WriteString("^")
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch c {
		case '*':
			re.WriteString(".*")
		case '?':
			re.WriteString(".")
		default:
			if isRegexMetaChar(c) {
				re.WriteByte('\\')
			}
			re.WriteByte(c)
		}
	}
	re.WriteString("$")
	ok, err := regexp.MatchString(re.String(), str)
	if err != nil {
		return false
	}
	return ok
}

// isRegexMetaChar reports whether c is a regex metacharacter that must be
// escaped when used as a literal in a matchWildcard pattern.
func isRegexMetaChar(c byte) bool {
	switch c {
	case '.', '+', '(', ')', '[', ']', '{', '}', '^', '$', '|', '\\':
		return true
	default:
		return false
	}
}

// globSpecificity returns a higher value for more specific glob patterns. A
// pattern is more specific when it has more literal characters and fewer
// wildcards. The empty/default "*" pattern is the least specific.
func globSpecificity(p string) int {
	lit := 0
	wc := 0
	for i := 0; i < len(p); i++ {
		switch p[i] {
		case '*', '?':
			wc++
		default:
			lit++
		}
	}
	return lit - wc
}

// ruleSpecificity combines the specificity of a rule's permission and pattern
// globs. Higher = more specific; used to resolve "most specific rule wins".
func ruleSpecificity(r Rule) int {
	return globSpecificity(r.Permission) + globSpecificity(r.Pattern)
}

// Evaluate resolves the action for a given (permission, pattern) across the
// provided rulesets. Among all rules whose Permission AND Pattern both match,
// the MOST SPECIFIC rule wins; ties are broken by the later rule (so rules
// merged last — agent/project — override equally-specific earlier rules). When
// no rule matches, it returns default rule {ask} (fail-safe).
func Evaluate(permission, pattern string, rulesets ...Ruleset) Rule {
	resolved := defaultRule(permission)
	bestScore := math.MinInt
	bestPos := -1
	pos := 0
	for _, rs := range rulesets {
		for _, r := range rs {
			if matchWildcard(r.Permission, permission) && matchWildcard(r.Pattern, pattern) {
				score := ruleSpecificity(r)
				if score > bestScore || (score == bestScore && pos > bestPos) {
					resolved = r
					bestScore = score
					bestPos = pos
				}
			}
			pos++
		}
	}
	return resolved
}

// defaultRule is the fail-safe rule returned when nothing matches. It carries
// the requested permission and an "ask" action so callers can tell what was
// being evaluated.
func defaultRule(permission string) Rule {
	return Rule{Permission: permission, Pattern: "*", Action: Ask}
}

// Merge combines multiple rulesets into one flattened ruleset. The order of the
// variadic arguments is preserved: rules passed later appear later, and thus
// win equal-specificity ties against earlier rules. This is how the config
// builder composes global → agent → project precedence.
func Merge(rulesets ...Ruleset) Ruleset {
	var out Ruleset
	for _, rs := range rulesets {
		out = append(out, rs...)
	}
	return out
}

// FromConfig converts a decoded YAML/JSON permissions block into a Ruleset.
// Each top-level key is a permission name:
//
//   - string value  -> { permission: key, pattern: "*", action: value }
//   - map   value  -> one rule per (pattern, action) entry under that permission
//
// Example:
//
//	permissions:
//	  glob: allow           -> {glob, *, allow}
//	  edit: {*.go: allow}   -> {edit, *.go, allow}
//
// Invalid or unknown action strings fall back to "ask" (fail-safe).
func FromConfig(cfg map[string]any) Ruleset {
	var rs Ruleset
	keys := make([]string, 0, len(cfg))
	for k := range cfg {
		keys = append(keys, k)
	}
	// Deterministic iteration order (config maps are unordered).
	sort.Strings(keys)
	for _, perm := range keys {
		switch v := cfg[perm].(type) {
		case string:
			rs = append(rs, Rule{Permission: perm, Pattern: "*", Action: NormalizeAction(v)})
		case map[string]any:
			rs = append(rs, mapToRules(perm, v)...)
		case map[string]string:
			m := make(map[string]any, len(v))
			for k, val := range v {
				m[k] = val
			}
			rs = append(rs, mapToRules(perm, m)...)
		default:
			// Unsupported shape: fail-safe, treat as ask for the permission.
			rs = append(rs, Rule{Permission: perm, Pattern: "*", Action: Ask})
		}
	}
	return rs
}

// mapToRules flattens a pattern→action map under a permission into rules.
func mapToRules(perm string, m map[string]any) Ruleset {
	patterns := make([]string, 0, len(m))
	for p := range m {
		patterns = append(patterns, p)
	}
	sort.Strings(patterns)
	var rs Ruleset
	for _, p := range patterns {
		rs = append(rs, Rule{Permission: perm, Pattern: p, Action: NormalizeAction(fmt.Sprint(m[p]))})
	}
	return rs
}

// ToolPermission maps a tool name to the permission namespace used to evaluate
// it. Mutating tools (edit/write/apply_patch and the filesystem write_file/
// edit_file) map to "edit"; read-only tools (read, read_file, list_dir, glob)
// and MCP readers map to "read". Unmapped tools use their own name as the
// permission, so config can gate them precisely (e.g. "shell", "web_fetch").
func ToolPermission(tool string) string {
	switch tool {
	case "edit", "write", "apply_patch", "write_file", "edit_file":
		return "edit"
	case "read", "read_file", "list_dir", "glob":
		return "read"
	default:
		if strings.HasPrefix(tool, "read_mcp_") {
			return "read"
		}
	}
	// MCP resource readers are also read-permission tools.
	switch tool {
	case "list_mcp_resources", "list_mcp_resource_templates", "read_mcp_resource":
		return "read"
	default:
		return tool
	}
}

// Disabled returns a set (as map[name]bool) of the given tools that are
// globally denied by the ruleset. A tool is disabled only when the resolved
// rule for its permission (with pattern "*") is a GLOBAL deny (pattern "*" and
// action "deny"). Scoped denies (e.g. edit denied only on docs/**) do not
// remove the tool from the LLM's view — they are enforced at call time. A nil
// or empty ruleset disables nothing (fail-open).
func Disabled(tools []string, ruleset Ruleset) map[string]bool {
	out := make(map[string]bool)
	if len(ruleset) == 0 {
		return out
	}
	for _, t := range tools {
		perm := ToolPermission(t)
		rule := Evaluate(perm, "*", ruleset)
		if rule.Action == Deny && rule.Pattern == "*" {
			out[t] = true
		}
	}
	return out
}

// VisibleTools filters a list of tool names, removing those that Disabled
// marks as globally denied. Order of the remaining tools is preserved.
func VisibleTools(tools []string, ruleset Ruleset) []string {
	hidden := Disabled(tools, ruleset)
	out := make([]string, 0, len(tools))
	for _, t := range tools {
		if !hidden[t] {
			out = append(out, t)
		}
	}
	return out
}
