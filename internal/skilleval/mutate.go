package skilleval

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrMutatorNotWired indicates a mutator whose LLM provider backend is not yet
// connected. See MutatorLLM.
var ErrMutatorNotWired = errors.New("skilleval: mutator provider not wired")

// errNoSkillName is returned when a SKILL.md frontmatter lacks a `name` key.
var errNoSkillName = errors.New("skill frontmatter missing name")

// errNoFrontmatter is returned when a SKILL.md has no leading `---` block.
var errNoFrontmatter = errors.New("skill has no frontmatter")

// Mutator produces a new body from the current body, guided by feedback
// produced by the A/B harness (one entry per rubric condition that passed or
// failed). A Mutator must never alter the identity — the caller owns the
// Genome and only ever replaces Genome.Body with the Mutator's result.
//
// A Mutator returns an error instead of panicking on a transient failure, so
// the evolution loop can treat a failed mutation as "no change" rather than
// crash.
type Mutator func(ctx context.Context, body string, feedback []string) (string, error)

// modelProvider is the contract for an LLM provider used by MutatorLLM. It is
// kept unexported and minimal because wiring a real provider is a later
// increment: for now MutatorLLM returns ErrMutatorNotWired, mirroring the
// LLMJudgeScorer placeholder pattern. Do not invent a provider here.
type modelProvider interface {
	// Generate returns the model's completion for the given prompt.
	Generate(ctx context.Context, prompt string) (string, error)
}

// MutatorLLM returns a Mutator driven by an LLM provider. The mutation is
// guided by the A/B feedback (per-condition pass/fail): the prompt instructs
// the model to preserve the body's purpose and scope, fix the cited failure
// points, honor a size cap (enforced by guardrails in a later phase), and
// return only the new body — never the frontmatter.
//
// The provider is not wired yet, so the returned Mutator always yields
// ErrMutatorNotWired. The contract and the prompt-shape are established here;
// threading a real provider through modelProvider lands in a later increment.
// The Mutator appends nothing and never touches identity.
func MutatorLLM(m modelProvider) Mutator {
	return func(ctx context.Context, body string, feedback []string) (string, error) {
		if m == nil {
			return "", ErrMutatorNotWired
		}
		// Wire the provider here in a later increment. The prompt is assembled
		// and passed for clarity; the contract requires it to return only the
		// new body (no frontmatter), preserving purpose, fixing the cited
		// feedback points, and honoring a size cap.
		return "", ErrMutatorNotWired
	}
}

// MutatorStatic is a deterministic Mutator backed by a pure function. It lets a
// test or fixture supply a rule such as "append a line marking the fix for
// point X" without any LLM. The function receives the current body and the
// A/B feedback and returns the new body; the returned Mutator never errors and
// threads ctx through so a cancelled context can still fast-path as a no-op.
func MutatorStatic(mutate func(body string, feedback []string) string) Mutator {
	return func(ctx context.Context, body string, feedback []string) (string, error) {
		if err := ctx.Err(); err != nil {
			return body, err
		}
		return mutate(body, feedback), nil
	}
}

// MutatorAppendFix is a deterministic, feedback-driven mutation rule for tests:
// it appends a single marker line naming the number of feedback points. It
// proves that the body can change while the identity stays immutable, and that
// the mutation is guided by feedback (the marker content depends on it).
//
// It never returns the frontmatter — it only extends the body — so a body
// produced by it cannot duplicate the identity.
func MutatorAppendFix(body string, feedback []string) string {
	return strings.TrimRight(body, "\n") + "\n<!-- fix: " + fmt.Sprintf("%d feedback points", len(feedback)) + " -->\n"
}
