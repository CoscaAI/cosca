package pipeline

import (
	"strings"
	"unicode/utf8"
)

// Budget tracks token usage and enforces limits during agent execution.
// When the context approaches the limit, it triggers compaction (summarization)
// to keep the prompt within the model's context window.
type Budget struct {
	maxTokens   int
	usedTokens  int
	warningPct  float64 // trigger compaction at this % of max (default 0.75)
	compactions int     // number of times compaction was triggered
}

// BudgetConfig configures a context budget.
type BudgetConfig struct {
	// MaxTokens is the maximum number of tokens allowed.
	MaxTokens int

	// WarningPct triggers compaction at this fraction of MaxTokens (0.0-1.0).
	// Default: 0.75 (compact at 75% full).
	WarningPct float64
}

// DefaultBudgetConfig returns a safe default for most models.
// 128K tokens with compaction at 75%.
func DefaultBudgetConfig() BudgetConfig {
	return BudgetConfig{
		MaxTokens:  128000,
		WarningPct: 0.75,
	}
}

// NewBudget creates a context budget tracker.
func NewBudget(cfg BudgetConfig) *Budget {
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 128000
	}
	if cfg.WarningPct <= 0 || cfg.WarningPct > 1.0 {
		cfg.WarningPct = 0.75
	}
	return &Budget{
		maxTokens:  cfg.MaxTokens,
		warningPct: cfg.WarningPct,
	}
}

// Add records token usage. Returns true if compaction is needed.
func (b *Budget) Add(tokens int) bool {
	b.usedTokens += tokens
	return b.ShouldCompact()
}

// Used returns the current token count.
func (b *Budget) Used() int { return b.usedTokens }

// Remaining returns how many tokens are left.
func (b *Budget) Remaining() int {
	remaining := b.maxTokens - b.usedTokens
	if remaining < 0 {
		return 0
	}
	return remaining
}

// ShouldCompact returns true when token usage exceeds the warning threshold.
func (b *Budget) ShouldCompact() bool {
	return float64(b.usedTokens) >= float64(b.maxTokens)*b.warningPct
}

// IsFull returns true when the budget is exhausted.
func (b *Budget) IsFull() bool {
	return b.usedTokens >= b.maxTokens
}

// Compactions returns the number of times compaction was triggered.
func (b *Budget) Compactions() int { return b.compactions }

// RecordCompaction increments the compaction counter.
func (b *Budget) RecordCompaction() { b.compactions++ }

// Reset clears the budget for a new execution.
func (b *Budget) Reset() {
	b.usedTokens = 0
	b.compactions = 0
}

// UsagePct returns the current usage as a percentage (0.0-1.0).
func (b *Budget) UsagePct() float64 {
	if b.maxTokens == 0 {
		return 0
	}
	return float64(b.usedTokens) / float64(b.maxTokens)
}

// ── Token estimation ────────────────────────────────────────────────────

// EstimateTokens provides a rough token count for a string.
// Uses the heuristic: ~4 characters per token for English text,
// ~1 character per token for code. This is a fast approximation;
// for exact counting, integrate tiktoken-go.
func EstimateTokens(text string) int {
	// Count words (split by whitespace) as a rough proxy.
	// English: ~0.75 tokens per word. Code: ~0.5 tokens per word.
	words := strings.Fields(text)
	if len(words) == 0 {
		return utf8.RuneCountInString(text) / 4
	}
	// Use word count * 1.3 as a reasonable approximation
	return int(float64(len(words)) * 1.3)
}

// EstimateTokensPrecise provides a more accurate count using rune-based
// heuristics: each rune ≈ 0.25 tokens for Latin scripts, ≈ 0.5 for CJK.
func EstimateTokensPrecise(text string) int {
	runes := utf8.RuneCountInString(text)
	// Rough: 4 characters ≈ 1 token for most models
	return runes / 4
}

// ── Compactor ───────────────────────────────────────────────────────────

// Compactor summarizes conversation history when context is full.
// It keeps the system prompt and recent messages, summarizing older turns.
type Compactor struct {
	keepLast int // number of recent messages to preserve
}

// NewCompactor creates a context compactor.
func NewCompactor(keepLast int) *Compactor {
	if keepLast <= 0 {
		keepLast = 4
	}
	return &Compactor{keepLast: keepLast}
}

// Compact reduces a message list to fit within the budget.
// Strategy: keep system message + last N messages, summarize the middle.
// Returns the compacted message list and a summary string.
func (c *Compactor) Compact(messages []Message, budget *Budget) ([]Message, string) {
	if len(messages) <= c.keepLast {
		return messages, ""
	}

	// Keep first message (system prompt) and last N messages.
	keep := make([]Message, 0, c.keepLast+1)
	keep = append(keep, messages[0]) // system prompt

	// Summarize the middle.
	middle := messages[1 : len(messages)-c.keepLast]
	summary := c.summarize(middle)

	if summary != "" {
		keep = append(keep, Message{
			Role:    "system",
			Content: "[Previous conversation summarized]: " + summary,
		})
	}

	// Add recent messages.
	keep = append(keep, messages[len(messages)-c.keepLast:]...)

	budget.RecordCompaction()
	return keep, summary
}

// summarize creates a brief summary of a message list.
// In production, this would call an LLM. For now, extract key snippets.
func (c *Compactor) summarize(messages []Message) string {
	if len(messages) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("Summary of " + itoa(len(messages)) + " previous turns: ")

	// Extract first sentence from each message as a summary.
	for i, m := range messages {
		if i >= 5 { // cap at 5 summarized messages
			break
		}
		content := firstSentence(m.Content)
		if len(content) > 100 {
			content = content[:100] + "..."
		}
		if content != "" {
			b.WriteString("[")
			b.WriteString(m.Role)
			b.WriteString("]: ")
			b.WriteString(content)
			b.WriteString(" | ")
		}
	}

	return strings.TrimSuffix(b.String(), " | ")
}

// firstSentence extracts text up to the first period, newline, or 200 chars.
func firstSentence(text string) string {
	text = strings.TrimSpace(text)
	for i, r := range text {
		if r == '.' || r == '\n' || r == '!' || r == '?' {
			return text[:i+1]
		}
		if i > 200 {
			return text[:200]
		}
	}
	if len(text) > 200 {
		return text[:200]
	}
	return text
}

// itoa is a simple int-to-string helper.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
