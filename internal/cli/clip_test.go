package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClipLines_UnderLimit_WritesVerbatim(t *testing.T) {
	var buf bytes.Buffer
	content := "a\nb\nc"
	omitted, spill, err := ClipLines(&buf, content, 5, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if omitted != 0 {
		t.Fatalf("expected 0 omitted, got %d", omitted)
	}
	if spill != "" {
		t.Fatalf("expected no spill, got %q", spill)
	}
	if buf.String() != content {
		t.Fatalf("expected verbatim content, got %q", buf.String())
	}
}

func TestClipLines_OverLimit_EmitsMarkerAndSpills(t *testing.T) {
	// Build a > maxLines payload (one line per "record").
	var sb strings.Builder
	for i := 0; i < 20; i++ {
		sb.WriteString("line")
		sb.WriteString(strings.Repeat("x", 10))
		sb.WriteString("\n")
	}
	content := sb.String()

	var buf bytes.Buffer
	omitted, spill, err := ClipLines(&buf, content, 5, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if omitted != 15 {
		t.Fatalf("expected 15 omitted, got %d", omitted)
	}
	if spill == "" {
		t.Fatal("expected a spill file to be created")
	}

	out := buf.String()
	if !strings.Contains(out, "... (15 lines omitted") {
		t.Fatalf("missing clip marker in output: %q", out)
	}
	if !strings.Contains(out, "use --full") {
		t.Fatalf("missing --full hint in marker: %q", out)
	}
	if !strings.Contains(out, spill) {
		t.Fatalf("output should echo the spill path %q: %q", spill, out)
	}
	if got := strings.Count(strings.TrimSuffix(out, "\n"), "\n"); got > 6 {
		t.Fatalf("clipped output has too many lines: %d", got)
	}

	// The spill file must contain the full, untruncated payload.
	data, rerr := os.ReadFile(spill)
	if rerr != nil {
		t.Fatalf("failed to read spill file %q: %v", spill, rerr)
	}
	if string(data) != content {
		t.Fatalf("spill file does not match original content")
	}
	_ = os.Remove(spill)
	cleanupClipSpillDir(spill)
}

func TestClipLines_Full_DisablesClip(t *testing.T) {
	var buf bytes.Buffer
	content := strings.Repeat("payload\n", 50)
	omitted, spill, err := ClipLines(&buf, content, 5, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if omitted != 0 {
		t.Fatalf("expected 0 omitted with --full, got %d", omitted)
	}
	if spill != "" {
		t.Fatalf("expected no spill with --full, got %q", spill)
	}
	if buf.String() != content {
		t.Fatalf("expected full content with --full")
	}
}

func TestClipLines_ZeroLimit_DisablesClip(t *testing.T) {
	var buf bytes.Buffer
	content := strings.Repeat("payload\n", 50)
	omitted, _, err := ClipLines(&buf, content, 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if omitted != 0 {
		t.Fatalf("expected 0 omitted with maxLines=0 (unlimited), got %d", omitted)
	}
	if buf.String() != content {
		t.Fatalf("expected full content with maxLines=0")
	}
}

func TestOutputMaxLines_Default_And_Env(t *testing.T) {
	t.Setenv(CoscaMaxOutputLinesEnv, "") // cannot unset; empty is treated as default
	if got := OutputMaxLines(); got != DefaultMaxOutputLines {
		t.Fatalf("expected default %d, got %d", DefaultMaxOutputLines, got)
	}

	t.Setenv(CoscaMaxOutputLinesEnv, "50")
	if got := OutputMaxLines(); got != 50 {
		t.Fatalf("expected 50, got %d", got)
	}

	t.Setenv(CoscaMaxOutputLinesEnv, "0")
	if got := OutputMaxLines(); got != 0 {
		t.Fatalf("expected 0 (unlimited), got %d", got)
	}

	t.Setenv(CoscaMaxOutputLinesEnv, "not-a-number")
	if got := OutputMaxLines(); got != DefaultMaxOutputLines {
		t.Fatalf("expected default for invalid env, got %d", got)
	}
}

// cleanupClipSpillDir best-effort removes the parent dir of a spilled file when
// it lives under the OS temp dir, so tests do not accumulate artifacts.
func cleanupClipSpillDir(spill string) {
	dir := filepath.Dir(spill)
	if dir == "" {
		return
	}
	_ = os.RemoveAll(dir)
}
