// Tests for the anti-hallucination of tool parameters (normalize.go).
//
// These verify that applyAliases rewrites hallucinated keys back to canonical
// keys (canonical always wins), that it never mutates the caller's map, and
// that the integration point in ValidateToolCall (and the full Execute path)
// lets a call with an hallucinated key pass schema validation.
package executor

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mapPointer returns the internal backing pointer of a (non-nil) map, used to
// prove that a returned map is a fresh allocation rather than the same map.
func mapPointer(m map[string]interface{}) uintptr {
	return reflect.ValueOf(m).Pointer()
}

// ─── applyAliases ────────────────────────────────────────────────────────────

func TestApplyAliases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  map[string]interface{}
		aliases aliasMap
		// check is invoked with the produced output; it must assert the
		// expected shape and, where relevant, that the original was untouched.
		check func(t *testing.T, out map[string]interface{}, input map[string]interface{})
	}{
		{
			name:    "alias moved to canonical",
			input:   map[string]interface{}{"userQuery": "hello"},
			aliases: globalAliases,
			check: func(t *testing.T, out map[string]interface{}, _ map[string]interface{}) {
				assert.Equal(t, "hello", out["query"])
				_, ok := out["userQuery"]
				assert.False(t, ok, "alias key should be removed after move")
			},
		},
		{
			name:    "canonical present wins and alias left untouched",
			input:   map[string]interface{}{"query": "canonical", "userQuery": "alias"},
			aliases: globalAliases,
			check: func(t *testing.T, out map[string]interface{}, _ map[string]interface{}) {
				assert.Equal(t, "canonical", out["query"])
				assert.Equal(t, "alias", out["userQuery"], "alias must be preserved when canonical wins")
			},
		},
		{
			name:    "preserves unrelated keys",
			input:   map[string]interface{}{"userQuery": "v", "extra": 42, "name": "n"},
			aliases: globalAliases,
			check: func(t *testing.T, out map[string]interface{}, _ map[string]interface{}) {
				assert.Equal(t, "v", out["query"])
				assert.Equal(t, 42, out["extra"])
				assert.Equal(t, "n", out["name"])
				_, ok := out["userQuery"]
				assert.False(t, ok)
			},
		},
		{
			name:    "only the first alias wins",
			input:   map[string]interface{}{"userQuery": "a", "question": "b"},
			aliases: globalAliases,
			check: func(t *testing.T, out map[string]interface{}, _ map[string]interface{}) {
				// Exactly one of the two aliases is moved to "query"; the other
				// is left untouched (map iteration order is non-deterministic).
				assert.Contains(t, []interface{}{"a", "b"}, out["query"])
				remaining := 0
				for _, k := range []string{"userQuery", "question"} {
					if _, ok := out[k]; ok {
						remaining++
					}
				}
				assert.Equal(t, 1, remaining, "exactly one alias should remain unmoved")
			},
		},
		{
			name:    "returns a copy and does not mutate the original",
			input:   map[string]interface{}{"userQuery": "v"},
			aliases: globalAliases,
			check: func(t *testing.T, out map[string]interface{}, input map[string]interface{}) {
				// Maps are reference types but not pointers, so compare the
				// internal pointer to prove a fresh map was returned.
				assert.NotEqual(t, mapPointer(input), mapPointer(out), "a new map must be returned")
				assert.Equal(t, "v", input["userQuery"], "original must keep the alias key")
				_, ok := input["query"]
				assert.False(t, ok, "original must NOT acquire the canonical key")
			},
		},
		{
			name:    "empty aliases leaves input unchanged",
			input:   map[string]interface{}{"foo": "bar"},
			aliases: aliasMap{},
			check: func(t *testing.T, out map[string]interface{}, input map[string]interface{}) {
				assert.NotEqual(t, mapPointer(input), mapPointer(out), "still a copy for non-nil input")
				assert.Equal(t, "bar", out["foo"])
			},
		},
		{
			name:    "canonical-only path key never rewritten",
			input:   map[string]interface{}{"path": "a/b/c"},
			aliases: globalAliases,
			check: func(t *testing.T, out map[string]interface{}, _ map[string]interface{}) {
				assert.Equal(t, "a/b/c", out["path"], "path is canonical-only and must never be rewritten")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out := applyAliases(tt.input, tt.aliases)
			tt.check(t, out, tt.input)
		})
	}
}

func TestApplyAliases_NilInput(t *testing.T) {
	t.Parallel()

	out := applyAliases(nil, globalAliases)
	assert.Nil(t, out)
}

// ─── aliasesFor ──────────────────────────────────────────────────────────────

func TestAliasesFor(t *testing.T) {
	t.Parallel()

	// A tool with no per-tool table falls back to the global table.
	assert.Equal(t, globalAliases, aliasesFor("read"))

	// A tool with a registered per-tool table uses its own table (not merged).
	assert.Equal(t, toolAliases["search"], aliasesFor("search"))

	// Unknown/empty names fall back to the global table.
	assert.Equal(t, globalAliases, aliasesFor(""))
}

// ─── normalizeToolCall ───────────────────────────────────────────────────────

func TestNormalizeToolCall(t *testing.T) {
	t.Parallel()

	t.Run("nil tool call is a no-op", func(t *testing.T) {
		t.Parallel()
		require.NotPanics(t, func() { normalizeToolCall(nil) })
	})

	t.Run("nil input is a no-op", func(t *testing.T) {
		t.Parallel()
		tc := &ToolCall{Name: "read", Input: nil}
		normalizeToolCall(tc)
		assert.Nil(t, tc.Input)
	})

	t.Run("rewrites hallucinated keys for a per-tool table", func(t *testing.T) {
		t.Parallel()
		tc := &ToolCall{Name: "search", Input: map[string]interface{}{"searchQuery": "foo"}}
		normalizeToolCall(tc)
		assert.Equal(t, "foo", tc.Input["pattern"])
		_, ok := tc.Input["searchQuery"]
		assert.False(t, ok)
	})

	t.Run("rewrites using the global fallback table", func(t *testing.T) {
		t.Parallel()
		tc := &ToolCall{Name: "read", Input: map[string]interface{}{"userQuery": "v"}}
		normalizeToolCall(tc)
		assert.Equal(t, "v", tc.Input["query"])
	})
}

// ─── Integration: ValidateToolCall ──────────────────────────────────────────

// queryRequiringTool returns a mockTool whose Validate requires the "query"
// canonical key to be present — simulating a schema that rejects any input
// that arrives under a hallucinated key.
func queryRequiringTool(name string, capture *json.RawMessage) *mockTool {
	return &mockTool{
		name: name,
		validateFunc: func(params json.RawMessage) error {
			if capture != nil {
				*capture = params
			}
			var in map[string]interface{}
			if err := json.Unmarshal(params, &in); err != nil {
				return err
			}
			if _, ok := in["query"]; !ok {
				return errors.New("query is required")
			}
			return nil
		},
	}
}

func TestValidateToolCall_NormalizesHallucinatedKey(t *testing.T) {
	t.Parallel()

	// The LLM sends "userQuery" (the prose of the description) instead of the
	// literal canonical "query" key. The normalization pre-pass must rewrite it
	// so schema validation passes.
	var captured json.RawMessage
	reg := newMockRegistry()
	registerTool(t, reg, queryRequiringTool("greeter", &captured))

	exe := newTestExecutor(t, reg, nil, "")
	err := exe.ValidateToolCall(ToolCall{
		Name:  "greeter",
		Input: map[string]interface{}{"userQuery": "world"},
	})

	require.NoError(t, err, "hallucinated key should be normalized before validation")

	// The validator must have received the canonical key, not the hallucinated one.
	var in map[string]interface{}
	require.NoError(t, json.Unmarshal(captured, &in))
	assert.Equal(t, "world", in["query"])
	_, ok := in["userQuery"]
	assert.False(t, ok)
}

func TestValidateToolCall_CanonicalKeyWins(t *testing.T) {
	t.Parallel()

	// When both the canonical and the hallucinated key are present, the
	// canonical value must survive and the hallucinated key must be ignored
	// (canonical wins; validation must not be misled by the alias).
	var captured json.RawMessage
	reg := newMockRegistry()
	registerTool(t, reg, queryRequiringTool("greeter", &captured))

	exe := newTestExecutor(t, reg, nil, "")
	err := exe.ValidateToolCall(ToolCall{
		Name:  "greeter",
		Input: map[string]interface{}{"query": "canonical", "userQuery": "alias"},
	})

	require.NoError(t, err)

	var in map[string]interface{}
	require.NoError(t, json.Unmarshal(captured, &in))
	assert.Equal(t, "canonical", in["query"], "canonical value must not be overwritten by the alias")
}

func TestValidateToolCall_PerToolAlias(t *testing.T) {
	t.Parallel()

	// A tool with a per-tool table (search) uses its own mapping: the
	// hallucinated "searchQuery" key maps to the canonical "pattern".
	reg := newMockRegistry()
	registerTool(t, reg, &mockTool{
		name: "search",
		validateFunc: func(params json.RawMessage) error {
			var in map[string]interface{}
			if err := json.Unmarshal(params, &in); err != nil {
				return err
			}
			if _, ok := in["pattern"]; !ok {
				return errors.New("pattern is required")
			}
			return nil
		},
	})

	exe := newTestExecutor(t, reg, nil, "")
	err := exe.ValidateToolCall(ToolCall{
		Name:  "search",
		Input: map[string]interface{}{"searchQuery": "main.go"},
	})

	require.NoError(t, err)
}

// ─── Integration: full Execute path ─────────────────────────────────────────

func TestExecute_NormalizesHallucinatedParamsBeforeExecution(t *testing.T) {
	t.Parallel()

	// The core problem: the tool call is rejected because validation fails on
	// the hallucinated key. After normalization, validation passes AND the tool
	// actually executes with the canonical key (not the raw hallucination).
	var captured json.RawMessage
	reg := newMockRegistry()
	registerTool(t, reg, &mockTool{
		name: "greeter",
		validateFunc: func(params json.RawMessage) error {
			var in map[string]interface{}
			if err := json.Unmarshal(params, &in); err != nil {
				return err
			}
			if _, ok := in["query"]; !ok {
				return errors.New("query is required")
			}
			return nil
		},
		executeFunc: func(_ context.Context, params json.RawMessage) (*chat.ToolResult, error) {
			captured = params
			return &chat.ToolResult{Output: "greeted"}, nil
		},
	})

	exe := newTestExecutor(t, reg, nil, "")
	result, err := exe.Execute(context.Background(), ToolCall{
		ID:    "call_1",
		Name:  "greeter",
		Input: map[string]interface{}{"userQuery": "world"},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, StatusSuccess, result.Status)
	assert.Equal(t, "greeted", result.Output)

	// The tool must have executed with the canonical key.
	var in map[string]interface{}
	require.NoError(t, json.Unmarshal(captured, &in))
	assert.Equal(t, "world", in["query"])
	_, ok := in["userQuery"]
	assert.False(t, ok)
}
