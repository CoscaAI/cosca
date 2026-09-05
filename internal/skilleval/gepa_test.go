package skilleval

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gepaEvaluate returns a deterministic evaluate func for GEPA: it scores a body
// by how many "fix:" markers it contains (more markers => higher score), so a
// variant that accumulates fix markers always out-scores the immutable baseline
// body. The benchmark's without arm reflects the baseline body (never mutated),
// and the IQRs are kept narrow so any strict improvement separates cleanly and
// is marked a candidate. This proves convergence deterministically, no LLM.
func gepaEvaluate(baselineBody string) func(context.Context, string) (*SkillBenchmark, error) {
	return func(_ context.Context, body string) (*SkillBenchmark, error) {
		withCount := strings.Count(body, "fix:")
		withoutCount := strings.Count(baselineBody, "fix:")
		with := Condition{
			MedianScore:   float64(withCount),
			MeanBootstrap: 0.02,
			IQRS:          [2]float64{float64(withCount) - 0.5, float64(withCount) + 0.5},
		}
		without := Condition{
			MedianScore:   float64(withoutCount),
			MeanBootstrap: 0.02,
			IQRS:          [2]float64{float64(withoutCount) - 0.5, float64(withoutCount) + 0.5},
		}
		delta := abDelta(with, without, MinTrials)
		return &SkillBenchmark{
			With:        with,
			Without:     without,
			Delta:       delta,
			IsCandidate: delta.Candidate,
		}, nil
	}
}

func TestRunGEPA_ImprovesFitness(t *testing.T) {
	base, err := ParseSkill(writeSkill(t, skillDoc))
	require.NoError(t, err)

	mutator := MutatorStatic(MutatorAppendFix)
	evaluate := gepaEvaluate(base.Body)
	baselineScore := float64(strings.Count(base.Body, "fix:")) // baseline has no fix markers
	baseBody := base.Body

	res, err := RunGEPA(context.Background(), base, mutator, baselineScore, evaluate, 5)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotNil(t, res.Best)

	// The mutator appends a fix marker each iteration and the deterministic
	// evaluate scores strictly higher with more markers, so the loop must
	// converge to a body full of them.
	assert.True(t, res.Candidate, "an improving variant must be marked a candidate")
	assert.Greater(t, res.Score, baselineScore, "best fitness must exceed the baseline")
	assert.Greater(t, strings.Count(res.Best.Body, "fix:"), strings.Count(base.Body, "fix:"),
		"the best body must have accumulated fix markers")
	assert.Equal(t, 5, res.Iterations)
	assert.Len(t, res.History, 5)

	// The baseline genome is never mutated in place.
	assert.Equal(t, baseBody, base.Body)
	assert.False(t, res.Best.Body == baseBody, "the best body must differ from the baseline body")
}

func TestRunGEPA_PreservesIdentity(t *testing.T) {
	base, err := ParseSkill(writeSkill(t, skillDoc))
	require.NoError(t, err)

	mutator := MutatorStatic(MutatorAppendFix)
	res, err := RunGEPA(context.Background(), base, mutator, 0, gepaEvaluate(base.Body), 5)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotNil(t, res.Best)

	// Identity is locked: it survives the whole mutation+evaluation loop.
	assert.Equal(t, base.Identity.Name, res.Best.Identity.Name)
	assert.Equal(t, base.Identity.Description, res.Best.Identity.Description)
	assert.Equal(t, base.Identity.Level, res.Best.Identity.Level)
	assert.Equal(t, base.Identity.Path, res.Best.Identity.Path)
	// The body is the only thing that changed.
	assert.NotEqual(t, base.Body, res.Best.Body)
}

func TestRunGEPA_GateAlwaysFails_KeepsBase(t *testing.T) {
	base, err := ParseSkill(writeSkill(t, skillDoc))
	require.NoError(t, err)

	mutator := MutatorStatic(MutatorAppendFix)
	// The variant always scores high (it looks like an improvement), but the
	// regression gate always fails, so it must never be adopted: the best stays
	// the baseline and the loop does not regress.
	evaluate := func(context.Context, string) (*SkillBenchmark, error) {
		return &SkillBenchmark{
			With:    Condition{MedianScore: 100, MeanBootstrap: 0.01, IQRS: [2]float64{99.5, 100.5}},
			Without: Condition{MedianScore: 0, MeanBootstrap: 0.01, IQRS: [2]float64{-0.5, 0.5}},
			Delta:   Delta{Score: 100, Candidate: true},
			// A failing gate forces IsCandidate false even with a strong delta.
			IsCandidate: false,
			Gate:        &GateResult{Passed: false},
		}, nil
	}

	res, err := RunGEPA(context.Background(), base, mutator, 0, evaluate, 5)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotNil(t, res.Best)

	// Best is unchanged: it is still the base genome (no regression).
	assert.Equal(t, base.Body, res.Best.Body)
	assert.Equal(t, base.Identity.Name, res.Best.Identity.Name)
	assert.Equal(t, 0.0, res.Score)
	assert.False(t, res.Candidate)
	assert.Equal(t, 5, res.Iterations)
	assert.Len(t, res.History, 5)
	// Every recorded step reflects a gate that did not let the variant through.
	for _, step := range res.History {
		assert.False(t, step.GatePassed)
		assert.NotEmpty(t, step.Feedback)
	}
}

func TestRunGEPA_MaxIterRespectedAndMutatorError(t *testing.T) {
	base, err := ParseSkill(writeSkill(t, skillDoc))
	require.NoError(t, err)

	calls := 0
	failing := func(context.Context, string, []string) (string, error) {
		calls++
		return "", errors.New("mutator boom")
	}
	evaluate := gepaEvaluate(base.Body)

	res, err := RunGEPA(context.Background(), base, failing, 0, evaluate, 3)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotNil(t, res.Best)

	// maxIter caps the loop: the mutator is called exactly once per iteration.
	assert.Equal(t, 3, calls, "mutator must be called exactly once per iteration")
	assert.Equal(t, 3, res.Iterations)
	assert.Len(t, res.History, 3)

	// A failing mutator never crashes the loop and keeps the best unchanged.
	assert.Equal(t, base.Body, res.Best.Body)
	assert.Equal(t, base.Identity.Name, res.Best.Identity.Name)
	assert.Equal(t, 0.0, res.Score)
	assert.False(t, res.Candidate)
	for _, step := range res.History {
		assert.False(t, step.GatePassed)
		assert.NotEmpty(t, step.Feedback)
	}
}
