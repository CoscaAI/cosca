package skilleval

import (
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// splitFixture builds n distinct, trivially-distinguishable SkillCases so tests
// can reason about identity via the unique ID.
func splitFixture(n int) []SkillCase {
	cases := make([]SkillCase, 0, n)
	for i := 0; i < n; i++ {
		cases = append(cases, SkillCase{
			ID:     string(rune('a'+i%26)) + "_" + strconv.Itoa(i),
			Task:   "case " + strconv.Itoa(i),
			Rubric: []string{"fix: " + strconv.Itoa(i)},
		})
	}
	return cases
}

// splitIDs returns the ordered IDs of each set so tests can compare partitions
// structurally and deterministically.
func splitIDs(s *EvalSplit) (train, val, holdout []string) {
	train = make([]string, 0, len(s.Train))
	val = make([]string, 0, len(s.Val))
	holdout = make([]string, 0, len(s.Holdout))
	for _, c := range s.Train {
		train = append(train, c.ID)
	}
	for _, c := range s.Val {
		val = append(val, c.ID)
	}
	for _, c := range s.Holdout {
		holdout = append(holdout, c.ID)
	}
	return train, val, holdout
}

func TestSplitEval_DefaultRatio(t *testing.T) {
	cases := splitFixture(100)
	split, err := SplitEval("demo-skill", cases, DefaultSplitRatio, 12345)
	require.NoError(t, err)
	require.NotNil(t, split)

	// Default 0.5 train fraction => 50/25/25 on 100 cases.
	assert.Len(t, split.Train, 50)
	assert.Len(t, split.Val, 25)
	assert.Len(t, split.Holdout, 25)
	assert.Equal(t, len(cases), len(split.Train)+len(split.Val)+len(split.Holdout))
}

func TestSplitEval_CustomRatio(t *testing.T) {
	cases := splitFixture(100)
	// 0.6 train => 60 train, 20 val, 20 holdout (complement split evenly).
	split, err := SplitEval("demo-skill", cases, 0.6, 7)
	require.NoError(t, err)
	require.NotNil(t, split)

	assert.Len(t, split.Train, 60)
	assert.Len(t, split.Val, 20)
	assert.Len(t, split.Holdout, 20)
	assert.Equal(t, len(cases), len(split.Train)+len(split.Val)+len(split.Holdout))
}

func TestSplitEval_TinyDataset_GuaranteesHoldout(t *testing.T) {
	// Even the minimum of 3 cases yields a non-empty holdout and val.
	cases := splitFixture(3)
	split, err := SplitEval("demo-skill", cases, DefaultSplitRatio, 42)
	require.NoError(t, err)

	assert.Len(t, split.Holdout, 1)
	assert.Len(t, split.Val, 1)
	assert.Len(t, split.Train, 1)
	assert.Equal(t, 3, len(split.Train)+len(split.Val)+len(split.Holdout))
}

func TestSplitEval_DeterministicAcrossRuns(t *testing.T) {
	cases := splitFixture(60)
	first, err := SplitEval("demo-skill", cases, DefaultSplitRatio, 999)
	require.NoError(t, err)
	second, err := SplitEval("demo-skill", cases, DefaultSplitRatio, 999)
	require.NoError(t, err)

	// Same (skill, seed) => identical partition, in the same order.
	t1, v1, h1 := splitIDs(first)
	t2, v2, h2 := splitIDs(second)
	assert.Equal(t, t1, t2, "train must be identical for the same seed")
	assert.Equal(t, v1, v2, "val must be identical for the same seed")
	assert.Equal(t, h1, h2, "holdout must be identical for the same seed")
}

func TestSplitEval_SeedChangesSplit(t *testing.T) {
	cases := splitFixture(60)
	a, err := SplitEval("demo-skill", cases, DefaultSplitRatio, 1)
	require.NoError(t, err)
	b, err := SplitEval("demo-skill", cases, DefaultSplitRatio, 2)
	require.NoError(t, err)

	ta, va, ha := splitIDs(a)
	tb, vb, hb := splitIDs(b)
	// Different numeric seed must produce a different partition (the streams are
	// distinct), while still being a valid covering partition of the same set.
	assert.False(t,
		sameIDs(ta, tb) && sameIDs(va, vb) && sameIDs(ha, hb),
		"a different seed must shuffle the dataset differently")
}

func TestSplitEval_SkillHashVariesSplit(t *testing.T) {
	cases := splitFixture(60)
	a, err := SplitEval("skill-a", cases, DefaultSplitRatio, 5)
	require.NoError(t, err)
	b, err := SplitEval("skill-b", cases, DefaultSplitRatio, 5)
	require.NoError(t, err)

	// Same numeric seed but a different skill name must decorrelate the
	// partition (the skill hash is mixed into the stream).
	ta, va, ha := splitIDs(a)
	tb, vb, hb := splitIDs(b)
	assert.False(t,
		sameIDs(ta, tb) && sameIDs(va, vb) && sameIDs(ha, hb),
		"a different skill must not share the same partition for the same seed")
}

func TestSplitEval_MinCasesError(t *testing.T) {
	_, err := SplitEval("demo-skill", splitFixture(2), DefaultSplitRatio, 1)
	require.Error(t, err)

	_, err = SplitEval("demo-skill", nil, DefaultSplitRatio, 1)
	require.Error(t, err)
}

func TestSplitEval_InvalidRatioError(t *testing.T) {
	cases := splitFixture(20)
	for name, ratio := range map[string]float64{
		"zero":     0,
		"one":      1,
		"negative": -0.25,
		"over one": 1.5,
		"nan":      math.NaN(),
		"tiny":     -0.0001,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := SplitEval("demo-skill", cases, ratio, 1)
			require.Error(t, err, "ratio %v must be rejected", ratio)
		})
	}
}

func TestSplitEval_DisjointAndCovering(t *testing.T) {
	for _, n := range []int{3, 4, 5, 11, 50, 101} {
		cases := splitFixture(n)
		split, err := SplitEval("demo-skill", cases, DefaultSplitRatio, 31415)
		require.NoError(t, err, "n=%d", n)

		// Covering: the three sets together hold every case exactly once.
		assert.Equal(t, n, len(split.Train)+len(split.Val)+len(split.Holdout), "n=%d", n)

		// Disjoint: no case appears in more than one set, and the holdout never
		// leaks into the train or val sets.
		trainSet := idSet(split.Train)
		valSet := idSet(split.Val)
		holdSet := idSet(split.Holdout)
		for id := range holdSet {
			_, inTrain := trainSet[id]
			_, inVal := valSet[id]
			assert.False(t, inTrain, "holdout case %q leaked into train (n=%d)", id, n)
			assert.False(t, inVal, "holdout case %q leaked into val (n=%d)", id, n)
		}
		for id := range trainSet {
			_, inVal := valSet[id]
			assert.False(t, inVal, "case %q appears in both train and val (n=%d)", id, n)
		}
	}
}

// sameIDs reports whether two ordered ID slices are element-wise identical.
func sameIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// idSet returns the set of IDs in a group, for O(1) membership checks.
func idSet(group []SkillCase) map[string]struct{} {
	set := make(map[string]struct{}, len(group))
	for _, c := range group {
		set[c.ID] = struct{}{}
	}
	return set
}
