package pipeline

import (
	"strings"
	"testing"
)

func TestBudgetDefaults(t *testing.T) {
	b := NewBudget(BudgetConfig{})
	if b.maxTokens != 128000 {
		t.Fatalf("maxTokens = %d, want 128000", b.maxTokens)
	}
	if b.warningPct != 0.75 {
		t.Fatalf("warningPct = %v, want 0.75", b.warningPct)
	}
	// Invalid config falls back to defaults.
	bad := NewBudget(BudgetConfig{MaxTokens: 0, WarningPct: 2.0})
	if bad.maxTokens != 128000 || bad.warningPct != 0.75 {
		t.Fatalf("invalid config not normalized: %+v", bad)
	}
	if d := DefaultBudgetConfig(); d.MaxTokens != 128000 || d.WarningPct != 0.75 {
		t.Fatalf("unexpected defaults: %+v", d)
	}
}

func TestBudgetAddAndShouldCompact(t *testing.T) {
	b := NewBudget(BudgetConfig{MaxTokens: 1000, WarningPct: 0.5})

	if b.Used() != 0 {
		t.Fatalf("initial Used = %d", b.Used())
	}
	if b.ShouldCompact() {
		t.Fatal("empty budget should not compact")
	}
	// Below threshold.
	if b.Add(400) {
		t.Fatal("400/1000 should not trigger compaction")
	}
	// At threshold (500 = 50%).
	if !b.Add(100) {
		t.Fatal("500/1000 should trigger compaction")
	}
	if b.Compactions() != 0 {
		t.Fatal("Add should not increment compaction counter itself")
	}
	b.RecordCompaction()
	if b.Compactions() != 1 {
		t.Fatalf("Compactions = %d, want 1", b.Compactions())
	}
}

func TestBudgetIsFullAndRemaining(t *testing.T) {
	b := NewBudget(BudgetConfig{MaxTokens: 100})
	if b.IsFull() {
		t.Fatal("empty budget is not full")
	}
	b.Add(100)
	if !b.IsFull() {
		t.Fatal("100/100 should be full")
	}
	if b.Remaining() != 0 {
		t.Fatalf("Remaining = %d, want 0", b.Remaining())
	}
	// Remaining never goes negative.
	b.Add(500)
	if b.Remaining() != 0 {
		t.Fatalf("Remaining after overflow = %d, want 0", b.Remaining())
	}
	// UsagePct reflects the raw ratio and is not clamped (600/100 = 6.0).
	if got := b.UsagePct(); got != 6.0 {
		t.Fatalf("UsagePct = %v, want 6.0", got)
	}
}

func TestBudgetReset(t *testing.T) {
	b := NewBudget(BudgetConfig{MaxTokens: 100, WarningPct: 0.1})
	b.Add(50)
	b.RecordCompaction()
	b.Reset()
	if b.Used() != 0 || b.Compactions() != 0 {
		t.Fatalf("Reset failed: used=%d compactions=%d", b.Used(), b.Compactions())
	}
	if b.UsagePct() != 0 {
		t.Fatalf("UsagePct after reset = %v", b.UsagePct())
	}
}

func TestEstimateTokens(t *testing.T) {
	if EstimateTokens("") != 0 {
		t.Fatal("empty string should estimate 0 tokens")
	}
	// 10 words → 10 * 1.3 = 13.
	if got := EstimateTokens("one two three four five six seven eight nine ten"); got != 13 {
		t.Fatalf("EstimateTokens(10 words) = %d, want 13", got)
	}
	// Precise: runes/4.
	if got := EstimateTokensPrecise("abcdefgh"); got != 2 {
		t.Fatalf("EstimateTokensPrecise(8 runes) = %d, want 2", got)
	}
}

func TestCompactorKeepsShortLists(t *testing.T) {
	c := NewCompactor(4)
	msgs := []Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hello"},
	}
	kept, summary := c.Compact(msgs, NewBudget(BudgetConfig{MaxTokens: 1000}))
	if len(kept) != 2 || summary != "" {
		t.Fatalf("short list must be returned unchanged: kept=%d summary=%q", len(kept), summary)
	}
}

func TestCompactorSummarizesMiddle(t *testing.T) {
	c := NewCompactor(2)
	msgs := []Message{
		{Role: "system", Content: "identity"},
		{Role: "user", Content: "turn one."},
		{Role: "assistant", Content: "answer one."},
		{Role: "user", Content: "turn two."},
		{Role: "assistant", Content: "answer two."},
	}
	budget := NewBudget(BudgetConfig{MaxTokens: 1000})
	kept, summary := c.Compact(msgs, budget)

	if len(kept) != 4 { // system + summary + last 2
		t.Fatalf("kept = %d messages, want 4", len(kept))
	}
	if kept[0].Role != "system" || kept[0].Content != "identity" {
		t.Fatal("system message must be preserved")
	}
	if !strings.HasPrefix(kept[1].Content, "[Previous conversation summarized]") {
		t.Fatalf("expected summary message, got %q", kept[1].Content)
	}
	if kept[2].Content != "turn two." || kept[3].Content != "answer two." {
		t.Fatalf("last messages not preserved: %+v", kept)
	}
	if !strings.Contains(summary, "turn one") || !strings.Contains(summary, "answer one") {
		t.Fatalf("summary missing middle turns: %q", summary)
	}
	if budget.Compactions() != 1 {
		t.Fatalf("Compactions = %d, want 1", budget.Compactions())
	}
}

func TestCompactorDefaultKeepLast(t *testing.T) {
	c := NewCompactor(0) // falls back to 4
	if c.keepLast != 4 {
		t.Fatalf("keepLast = %d, want 4", c.keepLast)
	}
}

func TestFirstSentence(t *testing.T) {
	if got := firstSentence("Hello world. More text"); got != "Hello world." {
		t.Fatalf("firstSentence = %q", got)
	}
	if got := firstSentence("Line one\nLine two"); got != "Line one\n" {
		t.Fatalf("firstSentence newline = %q", got)
	}
	if got := firstSentence("Question?"); got != "Question?" {
		t.Fatalf("firstSentence question = %q", got)
	}
	// Long text truncated at 200 runes.
	long := strings.Repeat("a", 300)
	if got := firstSentence(long); len([]rune(got)) != 200 {
		t.Fatalf("firstSentence long = %d runes, want 200", len([]rune(got)))
	}
}

func TestItoa(t *testing.T) {
	if itoa(0) != "0" {
		t.Fatalf("itoa(0) = %q", itoa(0))
	}
	if itoa(12345) != "12345" {
		t.Fatalf("itoa(12345) = %q", itoa(12345))
	}
}
