// Package memory provides session memory auto-recording utilities.
//
// Functions in this file are NOT part of the MemoryEngine API surface.
// They are convenience helpers that write Markdown files with YAML
// frontmatter directly to the filesystem, matching the existing storage
// conventions used by the FileStore.
package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// SessionSummary captures the key metrics of an Cosca serve session
// for automatic recording on graceful shutdown.
type SessionSummary struct {
	// Date is the ISO-8601 date of the session (e.g. "2026-07-27").
	Date string `yaml:"date" json:"date"`

	// Commits is the number of git commits made during the session.
	Commits int `yaml:"commits" json:"commits"`

	// FilesChanged is the approximate number of files modified.
	FilesChanged int `yaml:"files_changed" json:"files_changed"`

	// Deliverables lists key outputs produced during the session.
	Deliverables []string `yaml:"deliverables" json:"deliverables"`

	// ScoreBefore is the project quality score at session start.
	ScoreBefore int `yaml:"score_before" json:"score_before"`

	// ScoreAfter is the project quality score at session end.
	ScoreAfter int `yaml:"score_after" json:"score_after"`

	// UptimeSeconds is how long the server was running.
	UptimeSeconds int64 `yaml:"uptime_seconds" json:"uptime_seconds"`

	// ShutdownReason describes what triggered the shutdown.
	ShutdownReason string `yaml:"shutdown_reason" json:"shutdown_reason"`
}

// DefaultSessionSummary returns a SessionSummary with sensible defaults.
func DefaultSessionSummary() SessionSummary {
	return SessionSummary{
		Date:           time.Now().Format("2006-01-02"),
		ShutdownReason: "signal",
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Session auto-recording
// ──────────────────────────────────────────────────────────────────────────────

// sessionFrontmatter is the YAML envelope written at the top of every
// auto-recorded session memory file.
type sessionFrontmatter struct {
	ID             string    `yaml:"id"`
	Type           string    `yaml:"type"`
	Layer          string    `yaml:"layer"`
	Date           string    `yaml:"date"`
	Commits        int       `yaml:"commits"`
	FilesChanged   int       `yaml:"files_changed"`
	ScoreBefore    int       `yaml:"score_before"`
	ScoreAfter     int       `yaml:"score_after"`
	UptimeSeconds  int64     `yaml:"uptime_seconds"`
	ShutdownReason string    `yaml:"shutdown_reason"`
	CreatedAt      time.Time `yaml:"created_at"`
}

// RecordSession writes an auto-generated session memory record into
// <coscaDir>/memory/session/<date>-session-auto.md with YAML frontmatter.
//
// This function is intended to be called from serve shutdown handlers.
// It is safe when coscaDir does not exist (no-op).
func RecordSession(coscaDir string, summary SessionSummary) error {
	if coscaDir == "" {
		return nil
	}

	sessionDir := filepath.Join(coscaDir, "memory", "session")
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		return fmt.Errorf("create session memory directory: %w", err)
	}

	now := time.Now()
	date := summary.Date
	if date == "" {
		date = now.Format("2006-01-02")
	}

	filename := fmt.Sprintf("%s-session-auto.md", date)
	filePath := filepath.Join(sessionDir, filename)

	fm := sessionFrontmatter{
		ID:             fmt.Sprintf("auto-%s", now.Format("20060102-150405")),
		Type:           string(MemoryTypeSession),
		Layer:          string(LayerSession),
		Date:           date,
		Commits:        summary.Commits,
		FilesChanged:   summary.FilesChanged,
		ScoreBefore:    summary.ScoreBefore,
		ScoreAfter:     summary.ScoreAfter,
		UptimeSeconds:  summary.UptimeSeconds,
		ShutdownReason: summary.ShutdownReason,
		CreatedAt:      now,
	}

	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return fmt.Errorf("marshal session frontmatter: %w", err)
	}

	body := buildSessionBody(summary)

	content := fmt.Sprintf("---\n%s---\n\n%s\n", string(fmBytes), body)

	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write session memory file: %w", err)
	}

	return nil
}

// buildSessionBody constructs the human-readable Markdown body.
func buildSessionBody(summary SessionSummary) string {
	var b strings.Builder

	b.WriteString("# Session Auto-Record\n\n")
	b.WriteString(fmt.Sprintf("**Date**: %s  \n", summary.Date))
	if summary.UptimeSeconds > 0 {
		mins := summary.UptimeSeconds / 60
		b.WriteString(fmt.Sprintf("**Uptime**: %d minutes  \n", mins))
	}
	b.WriteString(fmt.Sprintf("**Shutdown**: %s  \n\n", summary.ShutdownReason))

	b.WriteString("## Summary\n\n")

	if summary.Commits > 0 {
		b.WriteString(fmt.Sprintf("- Commits: **%d**\n", summary.Commits))
	}
	if summary.FilesChanged > 0 {
		b.WriteString(fmt.Sprintf("- Files changed: **%d**\n", summary.FilesChanged))
	}

	delta := summary.ScoreAfter - summary.ScoreBefore
	if summary.ScoreBefore > 0 || summary.ScoreAfter > 0 {
		b.WriteString(fmt.Sprintf("- Quality score: %d → %d", summary.ScoreBefore, summary.ScoreAfter))
		if delta != 0 {
			sign := "+"
			if delta < 0 {
				sign = ""
			}
			b.WriteString(fmt.Sprintf(" (%s%d)", sign, delta))
		}
		b.WriteString("\n")
	}

	if len(summary.Deliverables) > 0 {
		b.WriteString("\n## Deliverables\n\n")
		for _, d := range summary.Deliverables {
			b.WriteString(fmt.Sprintf("- %s\n", d))
		}
	}

	b.WriteString("\n---\n*Auto-generated by cosca serve on shutdown.*\n")

	return b.String()
}
