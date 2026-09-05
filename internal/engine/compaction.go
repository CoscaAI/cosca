package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/CoscaAI/cosca/internal/chat"
)

// ─── Defaults ──────────────────────────────────────────────────────────────────

const (
	// defaultCompactionThreshold is the default context-window usage percentage at
	// which auto-compaction is triggered (80%).
	defaultCompactionThreshold = 0.8

	// minKeepMessages is the minimum number of most-recent messages to preserve
	// during compaction.
	minKeepMessages = 4

	// estimateTokenDivisor is a rough character-to-token ratio used for estimating
	// token counts when the provider does not report them.
	estimateTokenDivisor = 4
)

// ─── AutoCompaction ───────────────────────────────────────────────────────────

// AutoCompaction monitors context usage and triggers compaction when the
// estimated token count exceeds the configured percentage of the model's context
// window.
//
// Compaction strategy (from next-gen-cli-design.md §9):
//  1. Tool-output messages (RoleTool) are stripped first — they consume the most
//     tokens and are the least critical for coherence.
//  2. The oldest conversation turns are summarised into a single system message.
//  3. The most recent N messages are kept verbatim (N = total/3, minimum 4).
//  4. If thrashing is detected (compaction resolves immediately and the next
//     Check already finds the context over threshold), an error is returned.
type AutoCompaction struct {
	maxTokens       int     // model's maximum context window in tokens
	compactionPct   float64 // fraction of context window that triggers compaction (default 0.8)
	lastCompactedAt int     // len(messages) at the time of the last compaction
	compactionCount int     // number of compactions performed over the lifetime
	detectThrashing bool    // when true, the next threshold-hit triggers a thrashing error
}

// NewAutoCompaction creates an AutoCompaction with an 80 % threshold.
func NewAutoCompaction(maxTokens int) *AutoCompaction {
	return &AutoCompaction{
		maxTokens:     maxTokens,
		compactionPct: defaultCompactionThreshold,
	}
}

// NewAutoCompactionWithThreshold creates an AutoCompaction with a caller-specified
// threshold (0.0–1.0). Values outside that range are silently clamped to 0.8.
func NewAutoCompactionWithThreshold(maxTokens int, thresholdPct float64) *AutoCompaction {
	if thresholdPct <= 0 || thresholdPct > 1.0 {
		thresholdPct = defaultCompactionThreshold
	}
	return &AutoCompaction{
		maxTokens:     maxTokens,
		compactionPct: thresholdPct,
	}
}

// ─── Check ────────────────────────────────────────────────────────────────────

// Check evaluates whether compaction is needed based on estimated token usage.
//
//   - If estimatedTokens / maxTokens < compactionPct: no compaction is performed
//     and the result has Compacted = false.
//   - If estimatedTokens / maxTokens >= compactionPct: compaction is performed
//     and the result describes what was freed.
//   - If thrashing is detected (a previous compaction resolved but the next Check
//     already exceeds the threshold) a descriptive error is returned.
//
// The context parameter is reserved for future use (e.g. tracing, cancellation
// during LLM-based summarisation) and is not used in the current implementation.
func (ac *AutoCompaction) Check(ctx context.Context, messages []chat.Message, estimatedTokens int) (*CompactionResult, error) {
	// No messages or no tokens → nothing to compact.
	if len(messages) == 0 || estimatedTokens <= 0 {
		ac.detectThrashing = false
		return &CompactionResult{}, nil
	}

	ratio := float64(estimatedTokens) / float64(ac.maxTokens)
	if ratio < ac.compactionPct {
		// Below threshold — normal operation; reset thrashing guard.
		ac.detectThrashing = false
		return &CompactionResult{}, nil
	}

	// Threshold exceeded.
	// Thrrashing guard: if the previous compaction resolved immediately and we
	// are already over threshold again, refuse to loop.
	if ac.detectThrashing {
		return nil, fmt.Errorf(
			"compaction thrashing detected: last compacted at %d messages, "+
				"now at %d messages and still over %d%% of %d-token context window",
			ac.lastCompactedAt, len(messages),
			int(ac.compactionPct*100), ac.maxTokens,
		)
	}

	return ac.Compact(ctx, messages)
}

// ─── Compact ──────────────────────────────────────────────────────────────────

// Compact performs the compaction algorithm on the provided messages and returns
// a CompactionResult describing what was freed.
//
// Algorithm:
//  1. Strip all RoleTool (tool output) messages — they are the most verbose and
//     least critical for conversational coherence.
//  2. Keep the last N messages where N = max(minKeepMessages, total / 3).
//  3. Summarise everything older than the kept window into a single system message.
//  4. Return the result with metrics.
func (ac *AutoCompaction) Compact(ctx context.Context, messages []chat.Message) (*CompactionResult, error) {
	if len(messages) == 0 {
		return &CompactionResult{}, nil
	}

	originalCount := len(messages)
	originalTokens := estimateTokenCount(messages)

	compacted := ac.compactMessages(messages)

	newTokens := estimateTokenCount(compacted)
	tokensSaved := originalTokens - newTokens
	if tokensSaved < 0 {
		tokensSaved = 0
	}

	summary := extractSummaryFromCompacted(compacted)

	ac.lastCompactedAt = originalCount
	ac.compactionCount++
	ac.detectThrashing = true

	return &CompactionResult{
		Compacted:         true,
		Summary:           summary,
		MessagesRemaining: len(compacted),
		TokensSaved:       tokensSaved,
	}, nil
}

// compactMessages implements the core compaction algorithm on a copy.
func (ac *AutoCompaction) compactMessages(messages []chat.Message) []chat.Message {
	// ── Step 1: Strip tool-output messages ──────────────────────────────────
	filtered := make([]chat.Message, 0, len(messages))
	for _, m := range messages {
		if m.Role != chat.RoleTool {
			filtered = append(filtered, m)
		}
	}

	// If after stripping there's nothing to compact, return as-is.
	if len(filtered) <= minKeepMessages {
		return filtered
	}

	// ── Step 2: Determine keep window ───────────────────────────────────────
	keep := len(filtered) / 3
	if keep < minKeepMessages {
		keep = minKeepMessages
	}

	oldPart := filtered[:len(filtered)-keep]
	keepPart := filtered[len(filtered)-keep:]

	// ── Step 3: Build summary from the old part ─────────────────────────────
	summary := buildSummary(oldPart)
	summaryMsg := chat.Message{
		Role:    chat.RoleSystem,
		Content: "Previous conversation summary: " + summary,
	}

	// ── Step 4: Assemble result ─────────────────────────────────────────────
	result := make([]chat.Message, 0, 1+len(keepPart))
	result = append(result, summaryMsg)
	result = append(result, keepPart...)

	return result
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// buildSummary produces a human-readable summary of the provided messages.
// In the current implementation this is a simulated summarisation — it
// concatenates user-message content as bullet points. A future version should
// delegate this to an LLM call.
func buildSummary(messages []chat.Message) string {
	if len(messages) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("Previous conversation covered:")

	for _, m := range messages {
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}

		switch m.Role {
		case chat.RoleUser:
			// Truncate very long user messages to avoid blowing out the budget.
			label := content
			if len(label) > 120 {
				label = label[:120] + "..."
			}
			b.WriteString("\n- ")
			b.WriteString(label)

		case chat.RoleAssistant:
			// Only include a brief hint for assistant messages.
			label := content
			if len(label) > 80 {
				label = label[:80] + "..."
			}
			if label != "" {
				b.WriteString("\n- Assistant: ")
				b.WriteString(label)
			}

		case chat.RoleSystem:
			label := content
			if len(label) > 80 {
				label = label[:80] + "..."
			}
			if label != "" {
				b.WriteString("\n- Instruction: ")
				b.WriteString(label)
			}
		}
	}

	return b.String()
}

// extractSummaryFromCompacted finds the first system message in the compacted
// list (which is the summary we inserted) and returns it.
func extractSummaryFromCompacted(compacted []chat.Message) string {
	for _, m := range compacted {
		if m.Role == chat.RoleSystem && strings.HasPrefix(m.Content, "Previous conversation summary:") {
			return strings.TrimPrefix(m.Content, "Previous conversation summary: ")
		}
	}
	return ""
}

// estimateTokenCount returns a rough token estimate for a message slice using
// a 4:1 character-to-token ratio. This is intentionally simplistic — real
// tokenization should be done by the provider's tokeniser.
func estimateTokenCount(messages []chat.Message) int {
	var total int
	for _, m := range messages {
		total += len(m.Content) / estimateTokenDivisor
		// Account for tool-call payloads.
		for _, tc := range m.ToolCalls {
			total += len(tc.Function.Name) / estimateTokenDivisor
			total += len(tc.Function.Arguments) / estimateTokenDivisor
		}
	}
	return total
}
