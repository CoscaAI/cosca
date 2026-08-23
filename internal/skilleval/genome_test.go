package skilleval

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skillDoc is a minimal SKILL.md fixture with proper frontmatter.
const skillDoc = `---
name: alpha-skill
description: handles retries with backoff
level: 3
---

# ALPHA SKILL

## Purpose
Do the thing.
`

// writeSkill writes content to a fresh SKILL.md under a temp dir and returns its path.
func writeSkill(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestParseSkill_ReadsIdentityAndBody(t *testing.T) {
	path := writeSkill(t, skillDoc)

	g, err := ParseSkill(path)
	require.NoError(t, err)
	require.NotNil(t, g)

	// Identity is derived from the frontmatter.
	assert.Equal(t, "alpha-skill", g.Identity.Name)
	assert.Equal(t, "handles retries with backoff", g.Identity.Description)
	assert.Equal(t, 3, g.Identity.Level)
	assert.Equal(t, path, g.Identity.Path)

	// Body is everything after the frontmatter (the optimizable region),
	// preserving the file's trailing newline.
	assert.Equal(t, "# ALPHA SKILL\n\n## Purpose\nDo the thing.\n", g.Body)

	// The source preserves the original full text (for semantic diffing).
	assert.Equal(t, skillDoc, g.Source)
}

func TestParseSkill_BodyNeverContainsFrontmatter(t *testing.T) {
	g, err := ParseSkill(writeSkill(t, skillDoc))
	require.NoError(t, err)

	assert.NotContains(t, g.Body, "---", "body must not contain the frontmatter delimiter")
	assert.NotContains(t, g.Body, "name: alpha-skill", "body must not duplicate the frontmatter name")
	assert.NotContains(t, g.Body, "description: handles retries", "body must not duplicate the frontmatter description")
}

func TestParseSkill_NoFrontmatter_Error(t *testing.T) {
	_, err := ParseSkill(writeSkill(t, "just a body\nno frontmatter here\n"))
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoFrontmatter)
}

func TestParseSkill_UnclosedFrontmatter_Error(t *testing.T) {
	_, err := ParseSkill(writeSkill(t, "---\nname: alpha-skill\nno closing delimiter\n"))
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoFrontmatter)
}

func TestParseSkill_MissingName_Error(t *testing.T) {
	_, err := ParseSkill(writeSkill(t, "---\ndescription: no name here\nlevel: 1\n---\nbody\n"))
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoSkillName)
}

func TestParseSkill_MissingFile_Error(t *testing.T) {
	_, err := ParseSkill(filepath.Join(t.TempDir(), "nope.md"))
	require.Error(t, err)
}

func TestApply_RoundTrip_PreservesIdentity(t *testing.T) {
	g, err := ParseSkill(writeSkill(t, skillDoc))
	require.NoError(t, err)

	// Apply before any mutation reproduces a valid SKILL.md with the same identity.
	out := g.Apply()
	require.NotEmpty(t, out)

	// Re-parsing the applied output yields the exact same identity.
	reparsed, err := ParseSkill(writeSkill(t, out))
	require.NoError(t, err)
	assert.Equal(t, g.Identity.Name, reparsed.Identity.Name)
	assert.Equal(t, g.Identity.Description, reparsed.Identity.Description)
	assert.Equal(t, g.Identity.Level, reparsed.Identity.Level)
	assert.Equal(t, g.Body, reparsed.Body, "body survives the round trip")
}

func TestApply_AfterMutation_IdentityStillImmutable(t *testing.T) {
	g, err := ParseSkill(writeSkill(t, skillDoc))
	require.NoError(t, err)

	// The mutation rewrites the body but must never touch the identity.
	mutator := MutatorStatic(MutatorAppendFix)
	newBody, err := mutator(context.Background(), g.Body, []string{"failing condition", "another fail"})
	require.NoError(t, err)

	// Record the pre-mutation identity.
	origName, origDesc, origLevel := g.Identity.Name, g.Identity.Description, g.Identity.Level

	g.Body = newBody
	out := g.Apply()

	// Identity fields are untouched by the whole mutation+apply pipeline.
	assert.Equal(t, origName, g.Identity.Name)
	assert.Equal(t, origDesc, g.Identity.Description)
	assert.Equal(t, origLevel, g.Identity.Level)
	assert.NotEqual(t, g.Body, "# ALPHA SKILL\n\n## Purpose\nDo the thing.", "body must actually change")
	assert.Contains(t, out, "name: alpha-skill", "applied output still carries the frontmatter name")
	assert.Contains(t, out, "level: 3", "applied output still carries the frontmatter level")
	assert.Contains(t, out, newBody, "applied output embeds the mutated body")
}

func TestApply_MutatedBody_ReparseKeepsIdentityOnly(t *testing.T) {
	feedback := []string{"missing error handling", "no retry"}
	g, err := ParseSkill(writeSkill(t, skillDoc))
	require.NoError(t, err)

	mutator := MutatorStatic(MutatorAppendFix)
	g.Body, err = mutator(context.Background(), g.Body, feedback)
	require.NoError(t, err)

	out := g.Apply()
	reparsed, err := ParseSkill(writeSkill(t, out))
	require.NoError(t, err)

	// Identity is identical even after a feedback-guided body rewrite.
	assert.Equal(t, g.Identity.Name, reparsed.Identity.Name)
	assert.Equal(t, g.Identity.Description, reparsed.Identity.Description)
	assert.Equal(t, g.Identity.Level, reparsed.Identity.Level)
	// The mutated body is what survived — and it never carries frontmatter.
	assert.Equal(t, g.Body, reparsed.Body)
	assert.NotContains(t, reparsed.Body, "---")
}

func TestMutatorStatic_AppendFix_GuidedByFeedback(t *testing.T) {
	mutator := MutatorStatic(MutatorAppendFix)

	// Two separate feedback sets yield different markers: the mutation is
	// clearly feedback-driven, not blind.
	one, err := mutator(context.Background(), "body\n", []string{"a"})
	require.NoError(t, err)
	two, err := mutator(context.Background(), "body\n", []string{"a", "b", "c"})
	require.NoError(t, err)

	assert.Contains(t, one, "1 feedback points")
	assert.Contains(t, two, "3 feedback points")
	assert.NotEqual(t, one, two, "different feedback must produce different bodies")
	assert.NotContains(t, one, "---", "the appended body never introduces frontmatter")
}

func TestMutatorStatic_DoesNotReturnFrontmatter(t *testing.T) {
	mutator := MutatorStatic(MutatorAppendFix)
	body := "# ALPHA SKILL\n\n## Purpose\nDo the thing.\n"
	out, err := mutator(context.Background(), body, []string{"x"})
	require.NoError(t, err)
	assert.NotContains(t, out, "name: alpha-skill", "a mutator must return only the body, never the frontmatter")
	assert.Contains(t, out, "# ALPHA SKILL", "the body content is preserved")
}

func TestMutatorStatic_ContextCancel_NoOp(t *testing.T) {
	mutator := MutatorStatic(MutatorAppendFix)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	body := "stable body"
	out, err := mutator(ctx, body, []string{"a"})
	require.Error(t, err)
	assert.Equal(t, body, out, "a cancelled context yields the unchanged body (no partial mutation)")
}

func TestMutatorLLM_NotWired(t *testing.T) {
	// No provider is available yet: the mutator must fail closed with
	// ErrMutatorNotWired rather than inventing one.
	mutator := MutatorLLM(nil)
	_, err := mutator(context.Background(), "body", []string{"x"})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMutatorNotWired)
}

func TestMutatorLLM_NilProvider_NotWired(t *testing.T) {
	// A concrete provider type is not defined in this increment (only the
	// unexported interface contract), so passing a non-nil value is impossible;
	// both the nil and non-nil path must report NotWired.
	provider := stubProvider{}
	mutator := MutatorLLM(provider)
	_, err := mutator(context.Background(), "body", []string{"x"})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMutatorNotWired)
}

// stubProvider satisfies modelProvider so the non-nil path can be exercised
// without wiring a real LLM. It is never actually called.
type stubProvider struct{}

func (stubProvider) Generate(context.Context, string) (string, error) {
	return "", errors.New("never called in this increment")
}
