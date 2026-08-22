package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PostTaskHook runs after every pipeline task completes.
// It enforces AUTO_EVOLUTION_PROTOCOL stages 7-8.
type PostTaskHook struct {
	memoryDir string // .cosca/memory/agent/
}

// NewPostTaskHook creates a PostTaskHook rooted at the given directory.
func NewPostTaskHook(memoryDir string) *PostTaskHook {
	return &PostTaskHook{memoryDir: memoryDir}
}

// EvolutionRecord captures the learnings extracted from a completed task.
type EvolutionRecord struct {
	TaskID    string
	AgentName string
	TaskType  string
	Outcome   string   // success, failed, partial
	Technique string   // what technique was used
	Level     int      // capability level demonstrated
	Learned   []string // key learnings
	Pattern   string   // extracted pattern (if any)
	UpdatedAt time.Time
}

// RecordTask extracts learnings from a completed task.
// This is stage 7 (EXTRACT PATTERN) of the auto-evolution protocol.
func (h *PostTaskHook) RecordTask(ctx context.Context, record EvolutionRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = time.Now().UTC()
	}

	agentDir := filepath.Join(h.memoryDir, record.AgentName)
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		return fmt.Errorf("evolution: create agent dir %q: %w", agentDir, err)
	}

	if err := h.appendLearning(agentDir, record); err != nil {
		return err
	}

	if record.Pattern != "" {
		if err := h.appendPattern(agentDir, record); err != nil {
			return err
		}
	}

	return nil
}

// UpdateCapability updates the agent's capability profile.
// This is stage 8 (UPDATE CAPABILITY MODEL) of the auto-evolution protocol.
func (h *PostTaskHook) UpdateCapability(ctx context.Context, agentName string, outcome string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	agentDir := filepath.Join(h.memoryDir, agentName)
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		return fmt.Errorf("evolution: create agent dir %q: %w", agentDir, err)
	}

	profilePath := filepath.Join(agentDir, "capability-profile.md")
	entry := h.formatCapabilityUpdate(agentName, outcome)
	return appendToFile(profilePath, entry)
}

func (h *PostTaskHook) appendLearning(agentDir string, record EvolutionRecord) error {
	learningsPath := filepath.Join(agentDir, "learnings.md")
	timestamp := record.UpdatedAt.Format("2006-01-02 15:04")

	tags := h.buildTags(record)
	learned := strings.Join(record.Learned, "; ")
	if learned == "" {
		learned = record.Technique
	}

	entry := fmt.Sprintf(`

### %s — %s

| Field | Value |
|-------|-------|
| **Agent** | %s |
| **Task** | %s |
| **Technique** | %s |
| **Level** | %d |
| **Outcome** | %s |
| **Tags** | %s |
| **Related** | %s |
| **Learned** | %s |
| **Next** | %s |
| **Wisdom Decay Category** | STABLE |
| **Last Validated** | %s |
| **Confidence** | %.2f |
| **Expires At** | %s |
`,
		timestamp, sanitizeName(record.Technique),
		record.AgentName,
		record.TaskType,
		record.Technique,
		record.Level,
		record.Outcome,
		tags,
		record.TaskID,
		learned,
		h.nextStep(record),
		record.UpdatedAt.Format("2006-01-02"),
		h.outcomeConfidence(record.Outcome),
		record.UpdatedAt.AddDate(1, 0, 0).Format("2006-01-02"),
	)

	return appendToFile(learningsPath, entry)
}

func (h *PostTaskHook) appendPattern(agentDir string, record EvolutionRecord) error {
	patternsPath := filepath.Join(agentDir, "patterns.md")
	timestamp := record.UpdatedAt.Format("2006-01-02")

	entry := fmt.Sprintf(`

### %s — %s

| Field | Value |
|-------|-------|
| **Agent** | %s |
| **Source Task** | %s |
| **Pattern** | %s |
| **Level** | %d |
| **Outcome** | %s |
| **Extracted** | %s |
`,
		timestamp, sanitizeName(record.Technique),
		record.AgentName,
		record.TaskID,
		record.Pattern,
		record.Level,
		record.Outcome,
		timestamp,
	)

	return appendToFile(patternsPath, entry)
}

func (h *PostTaskHook) formatCapabilityUpdate(agentName, outcome string) string {
	now := time.Now().UTC()
	confidenceDelta := ""
	switch outcome {
	case "success":
		confidenceDelta = "Confidence +0.05 per adjustment rules."
	case "failed":
		confidenceDelta = "Confidence -0.10 per adjustment rules."
	case "partial":
		confidenceDelta = "Confidence unchanged (partial outcome)."
	}

	return fmt.Sprintf(`

## Post-task capability update — %s

- **Q4 — My confidence/skills changed?** %s
- **Capability status:** auto-updated by PostTaskHook (stage 8 of AUTO_EVOLUTION_PROTOCOL).
`,
		now.Format("2006-01-02"),
		confidenceDelta,
	)
}

func (h *PostTaskHook) buildTags(record EvolutionRecord) string {
	tags := []string{"#" + record.Outcome}
	if record.TaskType != "" {
		tags = append(tags, "#"+strings.ReplaceAll(strings.ToLower(record.TaskType), " ", "-"))
	}
	if record.Level > 0 {
		tags = append(tags, fmt.Sprintf("#level-%d", record.Level))
	}
	return strings.Join(tags, " ")
}

func (h *PostTaskHook) nextStep(record EvolutionRecord) string {
	if record.Outcome == "success" && record.Level < 5 {
		return fmt.Sprintf("Increase complexity — attempt Level %d variation.", record.Level+1)
	}
	if record.Outcome == "failed" || record.Outcome == "partial" {
		return "Reattempt with refined approach after reviewing failure mode."
	}
	return "Generalize pattern and document for cross-agent reuse."
}

func (h *PostTaskHook) outcomeConfidence(outcome string) float64 {
	switch outcome {
	case "success":
		return 0.85
	case "partial":
		return 0.65
	case "failed":
		return 0.40
	default:
		return 0.70
	}
}

func sanitizeName(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "/")
	return strings.TrimSpace(s)
}

func appendToFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("evolution: open %q: %w", path, err)
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("evolution: write %q: %w", path, err)
	}
	return nil
}

// MemoryDir returns the configured memory directory.
func (h *PostTaskHook) MemoryDir() string {
	return h.memoryDir
}
