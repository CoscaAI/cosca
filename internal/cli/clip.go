package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// DefaultMaxOutputLines is the output line cap that kicks in when
// COSCA_OUTPUT_MAX_LINES is not set. It prevents a single tool-output from
// flooding the context and blowing the token/LLM budget.
const DefaultMaxOutputLines = 300

// CoscaMaxOutputLinesEnv is the environment variable that configures the cap.
//
//	unset          -> DefaultMaxOutputLines (300)
//	"300"          -> 300
//	"0"            -> unlimited (automated / script usage)
//	invalid        -> DefaultMaxOutputLines
const CoscaMaxOutputLinesEnv = "COSCA_OUTPUT_MAX_LINES"

// OutputMaxLines returns the configured maximum number of lines to render
// before clipping. A return value of 0 means "no cap" (unlimited output).
func OutputMaxLines() int {
	s := strings.TrimSpace(os.Getenv(CoscaMaxOutputLinesEnv))
	if s == "" {
		return DefaultMaxOutputLines
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return DefaultMaxOutputLines
	}
	return n
}

// ClipLines writes content to w, clipping it to at most maxLines lines.
//
// When full is true (e.g. --full) or maxLines <= 0, the entire content is
// written verbatim and no marker/spill is produced.
//
// When content exceeds maxLines, only the first maxLines lines plus a marker
// line (`... (N lines omitted — use --full)`) are written, and the complete
// output is spilled to a temp file so the Kernel can consult it later without
// losing information. The spill path is echoed on the marker line.
//
// It returns the number of lines omitted and the spill file path (empty when
// nothing was spilled).
func ClipLines(w io.Writer, content string, maxLines int, full bool) (omitted int, spillPath string, err error) {
	if full || maxLines <= 0 {
		_, err = io.WriteString(w, content)
		return 0, "", err
	}

	lines := splitContentLines(content)
	if len(lines) <= maxLines {
		_, err = io.WriteString(w, content)
		return 0, "", err
	}

	omitted = len(lines) - maxLines
	head := strings.Join(lines[:maxLines], "\n")
	marker := fmt.Sprintf("... (%d lines omitted — use --full)", omitted)

	spillPath, spillErr := spillFullOutput(content)
	if spillErr != nil {
		// Could not spill; still clip so the context stays small, and note the loss.
		_, err = fmt.Fprintf(w, "%s\n%s\n", head, marker)
		return omitted, "", err
	}

	_, err = fmt.Fprintf(w, "%s\n%s\nFull output: %s\n", head, marker, spillPath)
	return omitted, spillPath, err
}

// splitContentLines splits content into lines, normalizing CRLF/CR to LF and
// ignoring a single trailing newline so it does not count as an empty line.
func splitContentLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// spillFullOutput writes content to a temp file (preferring .cosca/out/ in the
// current working directory, falling back to os.TempDir()) and returns the path.
func spillFullOutput(content string) (string, error) {
	name := fmt.Sprintf("cosca-out-%d.txt", time.Now().UnixNano())

	var dir string
	if cwd, cwdErr := os.Getwd(); cwdErr == nil {
		candidate := filepath.Join(cwd, ".cosca", "out")
		if mkErr := os.MkdirAll(candidate, 0o755); mkErr == nil {
			dir = candidate
		}
	}
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "cosca-out")
		if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
			return "", mkErr
		}
	}

	path := filepath.Join(dir, name)
	if writeErr := os.WriteFile(path, []byte(content), 0o644); writeErr != nil {
		return "", writeErr
	}
	return path, nil
}
