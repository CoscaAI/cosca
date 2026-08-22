package ranking

import (
	"testing"
	"time"
)

// testItem implements Rankable for testing.
type testItem struct {
	id            string
	content       string
	score         float64
	timestamp     int64
	refCount      int
	graphDistance int
}

func (t *testItem) ID() string          { return t.id }
func (t *testItem) Content() string     { return t.content }
func (t *testItem) Score() float64      { return t.score }
func (t *testItem) Timestamp() int64    { return t.timestamp }
func (t *testItem) ReferenceCount() int { return t.refCount }
func (t *testItem) GraphDistance() int  { return t.graphDistance }

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	if cfg.BM25Weight != 0.25 {
		t.Errorf("BM25Weight = %f, want 0.25", cfg.BM25Weight)
	}
	if cfg.VectorWeight != 0.35 {
		t.Errorf("VectorWeight = %f, want 0.35", cfg.VectorWeight)
	}
	if cfg.GraphWeight != 0.20 {
		t.Errorf("GraphWeight = %f, want 0.20", cfg.GraphWeight)
	}
	if cfg.FreshnessWeight != 0.10 {
		t.Errorf("FreshnessWeight = %f, want 0.10", cfg.FreshnessWeight)
	}
	if cfg.PopularityWeight != 0.10 {
		t.Errorf("PopularityWeight = %f, want 0.10", cfg.PopularityWeight)
	}
	if cfg.BM25K != 1.2 {
		t.Errorf("BM25K = %f, want 1.2", cfg.BM25K)
	}
	if cfg.BM25B != 0.75 {
		t.Errorf("BM25B = %f, want 0.75", cfg.BM25B)
	}
	if cfg.FreshnessHalfLife != 7*24*3600 {
		t.Errorf("FreshnessHalfLife = %f, want %f", cfg.FreshnessHalfLife, float64(7*24*3600))
	}
	if cfg.NormalizeScores != true {
		t.Error("NormalizeScores should be true")
	}
}

func TestNewRanker(t *testing.T) {
	t.Parallel()

	r := New(DefaultConfig())
	if r == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNewRankerZeroValues(t *testing.T) {
	t.Parallel()

	r := New(Config{})
	if r.cfg.BM25K != 1.2 {
		t.Errorf("BM25K should default to 1.2, got %f", r.cfg.BM25K)
	}
	if r.cfg.BM25B != 0.75 {
		t.Errorf("BM25B should default to 0.75, got %f", r.cfg.BM25B)
	}
	if r.cfg.FreshnessHalfLife <= 0 {
		t.Error("FreshnessHalfLife should have a default value")
	}
}

func TestSetCorpusAvg(t *testing.T) {
	t.Parallel()

	r := New(DefaultConfig())
	r.SetCorpusAvg(500.0)
	if r.corpusAVG != 500.0 {
		t.Errorf("corpusAVG = %f, want 500.0", r.corpusAVG)
	}
}

func TestRankEmptyItems(t *testing.T) {
	t.Parallel()

	r := New(DefaultConfig())
	result := r.Rank(nil, "test")
	if result != nil {
		t.Error("Rank with nil items should return nil")
	}

	result = r.Rank([]Rankable{}, "test")
	if len(result) != 0 {
		t.Error("Rank with empty items should return empty")
	}
}

func TestRankEmptyQuery(t *testing.T) {
	t.Parallel()

	r := New(DefaultConfig())
	items := []Rankable{
		&testItem{id: "a", content: "hello world", score: 1.0},
	}
	result := r.Rank(items, "")
	if len(result) != 1 {
		t.Errorf("got %d items, want 1", len(result))
	}
}

func TestRankBasic(t *testing.T) {
	t.Parallel()

	r := New(DefaultConfig())
	items := []Rankable{
		&testItem{id: "low", content: "hello world", score: 0.3},
		&testItem{id: "high", content: "hello world", score: 0.9},
		&testItem{id: "mid", content: "hello world", score: 0.6},
	}

	result := r.Rank(items, "hello")
	if len(result) != 3 {
		t.Fatalf("got %d items, want 3", len(result))
	}
	if result[0].ID() != "high" {
		t.Errorf("top item = %s, want high", result[0].ID())
	}
	if result[2].ID() != "low" {
		t.Errorf("bottom item = %s, want low", result[2].ID())
	}
}

func TestComputeBM25(t *testing.T) {
	t.Parallel()

	r := New(DefaultConfig())
	score := r.computeBM25("the quick brown fox", "fox")
	if score <= 0 {
		t.Errorf("BM25 score should be > 0, got %f", score)
	}
}

func TestComputeBM25Empty(t *testing.T) {
	t.Parallel()

	r := New(DefaultConfig())
	if r.computeBM25("", "test") != 0 {
		t.Error("BM25 with empty content should return 0")
	}
	if r.computeBM25("content", "") != 0 {
		t.Error("BM25 with empty query should return 0")
	}
}

func TestComputeGraphScore(t *testing.T) {
	t.Parallel()

	r := New(DefaultConfig())

	tests := []struct {
		distance int
		want     float64
	}{
		{0, 1.0},
		{1, 0.716531},
		{3, 0.367879},
		{-1, 0},
	}

	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			got := r.computeGraphScore(tc.distance)
			diff := got - tc.want
			if diff < 0 {
				diff = -diff
			}
			if diff > 0.0001 {
				t.Errorf("computeGraphScore(%d) = %f, want %f (diff=%f)", tc.distance, got, tc.want, diff)
			}
		})
	}
}

func TestComputeFreshness(t *testing.T) {
	t.Parallel()

	r := New(DefaultConfig())

	// Very recent timestamp
	score := r.computeFreshness(time.Now().Unix())
	if score > 1.0 || score <= 0 {
		t.Errorf("freshness for now = %f, want between 0 and 1", score)
	}

	// Zero timestamp
	score = r.computeFreshness(0)
	if score != 0.5 {
		t.Errorf("freshness for 0 = %f, want 0.5", score)
	}
}

func TestComputePopularity(t *testing.T) {
	t.Parallel()

	r := New(DefaultConfig())

	if r.computePopularity(0) != 0 {
		t.Error("popularity for 0 should be 0")
	}
	if r.computePopularity(10) <= 0 {
		t.Error("popularity for 10 should be > 0")
	}
}

func TestReciprocalRankFusion(t *testing.T) {
	t.Parallel()

	list1 := []Rankable{
		&testItem{id: "a"},
		&testItem{id: "b"},
	}
	list2 := []Rankable{
		&testItem{id: "b"},
		&testItem{id: "c"},
	}

	result := ReciprocalRankFusion([][]Rankable{list1, list2}, 3)
	if len(result) != 3 {
		t.Fatalf("got %d items, want 3", len(result))
	}
}

func TestReciprocalRankFusionEmpty(t *testing.T) {
	t.Parallel()

	result := ReciprocalRankFusion(nil, 10)
	if result != nil {
		t.Error("should return nil for empty input")
	}
}

func TestNormalizeScores(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		scores []float64
		min    float64
		max    float64
	}{
		{"empty", []float64{}, 0, 0},
		{"single", []float64{5.0}, 0.5, 0.5},
		{"multiple", []float64{1.0, 3.0, 5.0}, 0.0, 1.0},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			NormalizeScores(tc.scores)
			if len(tc.scores) == 0 {
				return
			}
			if len(tc.scores) == 1 {
				if tc.scores[0] != 0.5 {
					t.Errorf("single element should be 0.5, got %f", tc.scores[0])
				}
				return
			}
			if tc.scores[0] != tc.min || tc.scores[len(tc.scores)-1] != tc.max {
				t.Errorf("range = [%f, %f], want [%f, %f]", tc.scores[0], tc.scores[len(tc.scores)-1], tc.min, tc.max)
			}
		})
	}
}

func TestCountWords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"   ", 0},
		{"hello", 1},
		{"hello world test", 3},
	}

	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			got := countWords(tc.input)
			if got != tc.want {
				t.Errorf("countWords(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	t.Parallel()

	tokens := tokenize("Hello, World!")
	if len(tokens) != 2 {
		t.Fatalf("got %d tokens, want 2", len(tokens))
	}
	if tokens[0] != "hello" || tokens[1] != "world" {
		t.Errorf("tokens = %v, want [hello world]", tokens)
	}
}
