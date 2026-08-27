// Anti-hallucination of tool parameters.
//
// LLMs frequently echo the prose of a tool's parameter description instead of
// the literal key of the JSON Schema. For example, a schema that declares the
// canonical key "query" is often invoked with "userQuery" or "question"
// (the words used to describe the field), which makes the JSON Schema
// validation fail BEFORE the tool runs — producing the "tool call rejected"
// class of errors.
//
// This file implements a conservative "Normalize" pre-pass: it rewrites known
// hallucinated keys back to their canonical key inside a tool call's input map,
// so validation (and execution) operate on the keys the schema actually
// expects.
//
// Design contract:
//   - The interface chat.Tool (internal/chat/ports.go) is NOT touched.
//   - Individual tool implementations (filesystem.go, search.go, ...) are NOT
//     touched: a single central alias table is used instead.
//   - The registry's Execute (registry.go) is NOT touched — the active path is
//     the Executor (internal/chat/executor/executor.go).
//
// Consistency rules:
//   - applyAliases NEVER mutates the caller's map; it returns a copy.
//   - If the canonical key is already present, it wins and the alias is left
//     untouched (canonical is never overwritten by a guess).
//   - The normalization is idempotent (running it twice is a no-op).
package executor

// aliasMap maps a canonical parameter key to the list of hallucinated alias
// keys that should be rewritten to it. The canonical key is the literal name
// declared in a tool's JSON Schema; the aliases are the descriptive phrases an
// LLM is known to echo instead.
type aliasMap map[string][]string

// globalAliases is the central, conservative alias table applied to every tool
// call that does not have a more specific per-tool table.
//
// The set is deliberately minimal and well-documented: an alias is added only
// when there is strong evidence that LLMs echo a descriptive phrase instead of
// the canonical key, and never when the alias could collide with another
// tool's legitimate parameter name.
var globalAliases = aliasMap{
	// "query" is a common canonical-shaped key. LLMs frequently echo the prose
	// of a parameter description ("the user's query") as the key "userQuery",
	// or paraphrase it as "question". Neither alias is a legitimate key for any
	// built-in tool, so the mapping is safe to apply globally.
	"query": {"userQuery", "question"},

	// "path" is canonical-only: it deliberately declares NO aliases. This entry
	// documents the intent that the path field must never be rewritten from a
	// guess, because rewriting it could redirect an unrelated field and break a
	// tool's path-validation or workspace-escape checks. The empty list is a
	// no-op in applyAliases — it exists purely as documentation of intent.
	"path": {},
}

// toolAliases holds per-tool alias tables, keyed by the registered tool name.
// When a tool has a table here it is used (falling back to the global table
// otherwise). Tables are not merged with the global table: an alias that is
// meaningful for one tool should never accidentally rewrite a field of a
// different tool, so each tool gets an explicit, self-contained mapping.
var toolAliases = map[string]aliasMap{
	// "search" uses "pattern" as the canonical search term. LLMs commonly send
	// the query under a name that echoes the tool description ("Search for
	// files and content ... using patterns") rather than the literal "pattern"
	// key. All aliases below are safe: none is a legitimate search schema key.
	"search": {
		"pattern": {"searchQuery", "searchPattern", "query"},
	},

	// "glob" also uses "pattern" for the glob expression ("List files matching
	// a glob pattern ..."). "globPattern" is the echo LLMs produce for the
	// descriptive text; "query" is a common generic miss for the search term.
	"glob": {
		"pattern": {"globPattern", "query"},
	},
}

// aliasesFor returns the alias table to apply for the given tool name. If a
// per-tool table is registered it is used; otherwise the global table applies.
func aliasesFor(toolName string) aliasMap {
	if perTool, ok := toolAliases[toolName]; ok {
		return perTool
	}
	return globalAliases
}

// applyAliases returns a COPY of input with any known hallucinated alias keys
// rewritten to their canonical key. The original map is never mutated.
//
// Rules (in priority order):
//   - If the canonical key is already present in input, it WINS: any present
//     alias for that canonical key is left untouched (not moved, not removed).
//   - Otherwise, if a known alias is present, its value is moved to the
//     canonical key and the alias key is removed.
//   - Only the FIRST alias found for a given canonical key wins: any additional
//     aliases for that same canonical key are left as-is.
//   - All non-alias keys are preserved bit-for-bit.
//   - A nil input returns nil.
//
// The returned map is always a fresh allocation for non-nil input, so callers
// can rely on the caller's map being unmodified.
func applyAliases(input map[string]interface{}, aliases aliasMap) map[string]interface{} {
	if input == nil {
		return nil
	}

	// Always return a fresh copy so the caller's map is never mutated.
	out := make(map[string]interface{}, len(input))
	for k, v := range input {
		out[k] = v
	}

	if len(aliases) == 0 {
		return out
	}

	// Build the reverse lookup: alias -> canonical. If two canonical keys ever
	// claim the same alias, the conflict is prevented by keeping the tables
	// conservative (no alias is shared across canonical keys).
	aliasToCanonical := make(map[string]string)
	for canonical, aliasKeys := range aliases {
		for _, alias := range aliasKeys {
			if _, exists := aliasToCanonical[alias]; !exists {
				aliasToCanonical[alias] = canonical
			}
		}
	}
	if len(aliasToCanonical) == 0 {
		return out
	}

	for alias, canonical := range aliasToCanonical {
		// Canonical key already present: canonical wins, skip this alias.
		if _, exists := out[canonical]; exists {
			continue
		}
		// No known alias present for this key — nothing to rewrite.
		aliasVal, exists := out[alias]
		if !exists {
			continue
		}
		// Move the value to the canonical key and drop the alias. Because the
		// canonical key is now present, any further alias for the same
		// canonical key is skipped by the check above — i.e. only the first
		// alias wins.
		out[canonical] = aliasVal
		delete(out, alias)
	}

	return out
}

// normalizeToolCall rewrites the input map of a tool call so hallucinated
// parameter keys (see the package doc) are mapped to their canonical keys
// before schema validation and execution.
//
// The per-tool alias table is selected by toolCall.Name, falling back to the
// global table when no per-tool table is registered. The function is a no-op
// for a nil tool call or a nil Input map.
//
// NOTE on percolation: ToolCall is passed by value, but Input is a map
// (reference type). This function reassigns toolCall.Input to a normalized
// copy. For the single function that must both validate AND execute a call,
// the caller should normalize the input before delegating (see
// Executor.Execute and Executor.ValidateToolCall) so the canonical keys are
// visible to BOTH schema validation and the actual tool execution.
func normalizeToolCall(toolCall *ToolCall) {
	if toolCall == nil || toolCall.Input == nil {
		return
	}
	toolCall.Input = applyAliases(toolCall.Input, aliasesFor(toolCall.Name))
}
