package skilleval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveLoadBenchmark_RoundTrip(t *testing.T) {
	root := t.TempDir()

	orig := &SkillBenchmark{
		Skill: "alpha-skill",
		Date:  "2026-08-22T00:00:00Z",
		With: Condition{
			SolveRate: 0.8, MedianScore: 0.875, IQRS: [2]float64{0.5, 1.0},
			MeanBootstrap: 0.01, MedianTokens: 120, MedianTimeMS: 350,
		},
		Without: Condition{
			SolveRate: 0.2, MedianScore: 0.25, IQRS: [2]float64{0.0, 0.5},
			MeanBootstrap: 0.02, MedianTokens: 60, MedianTimeMS: 150,
		},
		Delta:       Delta{Score: 0.625, ScoreStd: 0.022, Candidate: true},
		IsCandidate: true,
		Gate: &GateResult{
			CatalogAudit: true, RegTests: true, RegDelta: true, Passed: true,
			Details: []string{"catalog ok"},
		},
	}

	require.NoError(t, SaveBenchmark(root, orig), "SaveBenchmark")

	got, err := LoadLatestBenchmark(root, "alpha-skill")
	require.NoError(t, err, "LoadLatestBenchmark")
	assert.Equal(t, orig.Skill, got.Skill)
	assert.Equal(t, orig.Date, got.Date)
	assert.InDelta(t, orig.With.MedianScore, got.With.MedianScore, 1e-9)
	assert.InDelta(t, orig.With.IQRS[0], got.With.IQRS[0], 1e-9)
	assert.InDelta(t, orig.With.IQRS[1], got.With.IQRS[1], 1e-9)
	assert.InDelta(t, orig.Delta.Score, got.Delta.Score, 1e-9)
	assert.Equal(t, orig.IsCandidate, got.IsCandidate)
	require.NotNil(t, got.Gate)
	assert.Equal(t, orig.Gate.Passed, got.Gate.Passed)
	assert.Equal(t, orig.Gate.Details, got.Gate.Details)

	// File lands under .cosca/evals/skills/ with 0600 perms.
	path := filepath.Join(root, ".cosca", "evals", EvalDirName, "alpha-skill.benchmark.json")
	info, err := os.Stat(path)
	require.NoError(t, err, "benchmark file exists")
	if isWindows() {
		return // no POSIX permission bits on Windows
	}
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestAppendHistory_PersistsAllInOrder_WithParentChain(t *testing.T) {
	root := t.TempDir()

	// Append-only: each new entry derives its Parent from the previous Version.
	e1, err := AppendHistory(root, "alpha-skill", "v1", 0.35, "first baseline", true)
	require.NoError(t, err)
	e2, err := AppendHistory(root, "alpha-skill", "v2", 0.55, "improvement", true)
	require.NoError(t, err)
	e3, err := AppendHistory(root, "alpha-skill", "v3", 0.62, "more", false)
	require.NoError(t, err)

	assert.Equal(t, "", e1.Parent, "first entry has no parent")
	assert.Equal(t, "v1", e2.Parent)
	assert.Equal(t, "v2", e3.Parent)
	assert.False(t, e3.IsCurrentBest, "e3 is not the best")

	hist, err := LoadHistory(root, "alpha-skill")
	require.NoError(t, err)
	require.Len(t, hist, 3, "all entries preserved — nothing overwritten")

	// Parent chain preserved in the persisted array.
	assert.Equal(t, "", hist[0].Parent)
	assert.Equal(t, "v1", hist[1].Parent)
	assert.Equal(t, "v2", hist[2].Parent)

	// Immutability of the best flag: only v2 (the last best) claims it.
	assert.True(t, hist[1].IsCurrentBest)
	assert.False(t, hist[0].IsCurrentBest, "previous best flipped off")
	assert.False(t, hist[2].IsCurrentBest)
}

func TestAppendHistory_CurrentBestFlipsPrevious(t *testing.T) {
	root := t.TempDir()

	_, err := AppendHistory(root, "beta-skill", "v1", 0.40, "run 1", true)
	require.NoError(t, err)
	_, err = AppendHistory(root, "beta-skill", "v2", 0.90, "run 2", true)
	require.NoError(t, err)

	hist, err := LoadHistory(root, "beta-skill")
	require.NoError(t, err)
	require.Len(t, hist, 2)
	assert.False(t, hist[0].IsCurrentBest, "v1 demoted when v2 claimed best")
	assert.True(t, hist[1].IsCurrentBest)

	// A non-best append must NOT demote the current best.
	_, err = AppendHistory(root, "beta-skill", "v3", 0.30, "run 3", false)
	require.NoError(t, err)

	hist, err = LoadHistory(root, "beta-skill")
	require.NoError(t, err)
	require.Len(t, hist, 3)
	assert.True(t, hist[1].IsCurrentBest, "v2 stays best")
	assert.False(t, hist[2].IsCurrentBest)
}

func TestAppendHistory_AppendDoesNotOverwrite(t *testing.T) {
	root := t.TempDir()

	// The same "logical" append twice must yield two rows (append-only),
	// never collapse into one — proving we do not overwrite.
	_, err := AppendHistory(root, "gamma-skill", "v1", 0.5, "same", true)
	require.NoError(t, err)
	_, err = AppendHistory(root, "gamma-skill", "v1", 0.5, "same", true)
	require.NoError(t, err)

	hist, err := LoadHistory(root, "gamma-skill")
	require.NoError(t, err)
	require.Len(t, hist, 2, "appending identical rows appends, never overwrites")
	assert.Equal(t, 0.5, hist[0].PassRate)
	assert.Equal(t, 0.5, hist[1].PassRate)
}

func TestSaveLoadBenchmark_NilErrors(t *testing.T) {
	root := t.TempDir()
	require.Error(t, SaveBenchmark(root, nil))

	_, err := LoadLatestBenchmark(root, "missing-skill")
	assert.Error(t, err, "missing benchmark should error")
}

func TestLoadHistory_MissingReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	hist, err := LoadHistory(root, "no-such-skill")
	require.NoError(t, err, "missing history is not an error")
	assert.Empty(t, hist)
}

func TestAppendHistory_CreatesDirTree(t *testing.T) {
	root := t.TempDir()
	_, err := AppendHistory(root, "fresh-skill", "v0", 0.1, "seed", true)
	require.NoError(t, err)

	path := filepath.Join(root, ".cosca", "evals", EvalDirName, "fresh-skill.history.json")
	info, err := os.Stat(path)
	require.NoError(t, err, "history file created under fresh dir tree")
	if isWindows() {
		return
	}
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func isWindows() bool {
	return os.PathSeparator == '\\'
}
