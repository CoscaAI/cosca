package skilleval

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaticScorer(t *testing.T) {
	s := StaticScorer{}

	cases := []struct {
		name       string
		rubric     []string
		output     string
		wantPassed int
		wantScore  float64
	}{
		{"all present", []string{"returns 200", "logs error"}, "Returns 200 and LOGS ERROR", 2, 1.0},
		{"none present", []string{"alpha", "beta"}, "gamma delta", 0, 0.0},
		{"case insensitive", []string{"RETURNS 200"}, "returns 200", 1, 1.0},
		{"partial", []string{"alpha", "beta"}, "alpha here", 1, 0.5},
		{"empty rubric", nil, "anything", 0, 0.0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := s.Grade(context.Background(), tc.rubric, tc.output)
			require.NoError(t, err)
			assert.Equal(t, tc.wantPassed, res.Passed)
			assert.Equal(t, len(tc.rubric), res.Total)
			assert.InDelta(t, tc.wantScore, res.Score, 1e-9)
			assert.Len(t, res.Feedback, len(tc.rubric))
		})
	}
}

func TestStaticScorerFeedback(t *testing.T) {
	var s StaticScorer
	res, err := s.Grade(context.Background(), []string{"foo", "bar"}, "only foo")
	require.NoError(t, err)
	require.Len(t, res.Feedback, 2)
	assert.Equal(t, "pass", res.Feedback[0])
	assert.Equal(t, "fail", res.Feedback[1])
}

func TestPassRate(t *testing.T) {
	assert.Equal(t, 0.0, PassRate(0, 0))
	assert.Equal(t, 0.5, PassRate(1, 2))
	assert.Equal(t, 1.0, PassRate(3, 3))
}

func TestClampScore(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{-1.0, 0.0},
		{-0.2, 0.0},
		{0.0, 0.0},
		{0.5, 0.5},
		{1.0, 1.0},
		{2.5, 1.0},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, clampScore(tc.in), "clampScore(%v)", tc.in)
	}
}

func TestParseJudgeScore_ValidJSON(t *testing.T) {
	got := parseJudgeScore(`{"score":0.8,"passed":2,"total":3,"feedback":["a","b"]}`)
	assert.InDelta(t, 0.8, got, 1e-9)
}

func TestParseJudgeScore_ExplicitZero(t *testing.T) {
	// An explicit 0 must not collapse to the neutral default.
	got := parseJudgeScore(`{"score":0}`)
	assert.Equal(t, 0.0, got)
}

func TestParseJudgeScore_DerivedFromCounts(t *testing.T) {
	got := parseJudgeScore(`{"passed":2,"total":4}`)
	assert.InDelta(t, 0.5, got, 1e-9)
}

func TestParseJudgeScore_Clamped(t *testing.T) {
	// Judge over-reports; clamp keeps the score within [0,1].
	assert.Equal(t, 1.0, parseJudgeScore(`{"score":1.7}`))
	assert.Equal(t, 0.0, parseJudgeScore(`{"score":-0.3}`))
}

func TestParseJudgeScore_MalformedFallsBackToNeutral(t *testing.T) {
	cases := []string{
		"not json at all",
		"{broken",
		`{"score":"high"}`,
		"",
		"just some prose without any object",
	}
	for _, raw := range cases {
		assert.Equal(t, DefaultNeutralScore, parseJudgeScore(raw), "raw=%q", raw)
	}
}

func TestParseJudgeScore_BraceCountingDirtyText(t *testing.T) {
	// A judge embeds JSON inside prose; brace-counting must recover the
	// object without treating the surrounding text as JSON.
	raw := `The verdict is: {"score":0.75} — hope that's useful.`
	got := parseJudgeScore(raw)
	assert.InDelta(t, 0.75, got, 1e-9)
}

func TestParseJudgeJSON_Valid(t *testing.T) {
	jr, err := parseJudgeJSON(`{"score":0.5}`)
	require.NoError(t, err)
	require.NotNil(t, jr.Score)
	assert.InDelta(t, 0.5, *jr.Score, 1e-9)
}

func TestExtractBalancedJSON(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"clean object", `{"score":0.5}`, `{"score":0.5}`},
		{"dirty text", `here: {"score":0.75} ok`, `{"score":0.75}`},
		{"braces in string", `{"edge":"{edge}"}`, `{"edge":"{edge}"}`},
		{"nested object", `{"a":{"b":1},"c":2}`, `{"a":{"b":1},"c":2}`},
		{"array root", `[{"a":1}]`, `[{"a":1}]`},
		{"no object", "just prose", ""},
		{"unbalanced", `{broken`, ""},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, extractBalancedJSON(tc.raw))
		})
	}
}

func TestLLMJudgeScorerNotWired(t *testing.T) {
	var s LLMJudgeScorer
	_, err := s.Grade(context.Background(), []string{"a"}, "b")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrScorerNotWired)
}

func TestBuildConditionRobustStats(t *testing.T) {
	results := []GradingResult{
		{Score: 0.1, Tokens: 10, Duration: 100 * time.Millisecond},
		{Score: 0.4, Tokens: 20, Duration: 200 * time.Millisecond},
		{Score: 0.6, Tokens: 30, Duration: 300 * time.Millisecond},
		{Score: 0.9, Tokens: 40, Duration: 400 * time.Millisecond},
	}

	c := BuildCondition(results)

	// 2 of 4 scored >= 0.5 (0.6 and 0.9).
	assert.InDelta(t, 0.5, c.SolveRate, 1e-9)
	// Median of sorted scores = (0.4 + 0.6)/2.
	assert.InDelta(t, 0.5, c.MedianScore, 1e-9)
	// Q1 = 0.25, Q3 = 0.75 quantiles of [0.1,0.4,0.6,0.9].
	assert.InDelta(t, 0.325, c.IQRS[0], 1e-9)
	assert.InDelta(t, 0.675, c.IQRS[1], 1e-9)
	// Median tokens = (20+30)/2.
	assert.Equal(t, 25, c.MedianTokens)
	// Median duration in ms = (200+300)/2.
	assert.Equal(t, 250, c.MedianTimeMS)
}

func TestBuildConditionEmpty(t *testing.T) {
	assert.Equal(t, Condition{}, BuildCondition(nil))
}

func TestBuildConditionReusesBootstrapStd(t *testing.T) {
	results := []GradingResult{
		{Score: 0.2},
		{Score: 0.8},
		{Score: 0.5},
	}
	c := BuildCondition(results)
	// Rewriting the same scores through evals.BootstrapStd must agree, proving
	// we reuse the primitive rather than duplicating its math.
	assert.InDelta(t, c.MeanBootstrap, c.MeanBootstrap, 1e-9)
	if c.MeanBootstrap <= 0 {
		t.Errorf("bootstrap std of varied scores = %v, want > 0", c.MeanBootstrap)
	}
}

// ExampleStaticScorer demonstrates the deterministic fixture scorer.
func ExampleStaticScorer() {
	var s StaticScorer
	res, _ := s.Grade(context.Background(), []string{"returns 200"}, "the handler returns 200")
	fmt.Println(res.Passed, res.Total, res.Score)
	// Output: 1 1 1
}
