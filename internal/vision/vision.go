// Package vision gives the Cosca Kernel eyes.
//
// It turns images (photos, screenshots, diagrams, panels) into text the
// Kernel can reason about — OCR via the local `tesseract` binary (no C++
// binding, no sudo, no cloud API). The pipeline is 100% local.
//
// Upgrade decision (Don, 2026-08-01): the active provider (deepseek-v4-flash)
// does not declare the "vision" capability, so instead of swapping models we
// give the Kernel a local "eye": image → OCR → text → reasoning.
package vision

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Options configures a single OCR pass.
type Options struct {
	// Lang is the tesseract language (e.g. "eng", "por", "eng+por").
	// Empty means the tesseract default.
	Lang string
	// PSM is the tesseract page segmentation mode (0-13).
	// 0 (OSD) and 3 (auto) are the most useful defaults. 0 means "not set".
	PSM int
}

// tesseractBin is the path to the local tesseract binary. Overridable for
// tests.
var tesseractBin = "tesseract"

// Describe reads an image file and returns a text description of what it
// contains — the Kernel's "sight". It runs tesseract locally and returns the
// extracted text plus metadata.
func Describe(path string, opts Options) (Result, error) {
	if _, err := os.Stat(path); err != nil {
		return Result{}, fmt.Errorf("image not found: %w", err)
	}

	args := []string{path, "stdout", "-l", defaultLang(opts.Lang)}
	if opts.PSM != 0 {
		args = append(args, "--psm", fmt.Sprintf("%d", opts.PSM))
	}

	cmd := exec.Command(tesseractBin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return Result{}, fmt.Errorf("tesseract failed: %v (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	text := strings.TrimSpace(stdout.String())
	info, _ := os.Stat(path)

	return Result{
		Path:      path,
		SizeBytes: info.Size(),
		Text:      text,
	}, nil
}

// Result is the outcome of an OCR pass.
type Result struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	Text      string `json:"text"`
}

// IsEmpty reports whether the OCR pass found no readable text.
func (r Result) IsEmpty() bool {
	return strings.TrimSpace(r.Text) == ""
}

// Summary returns a compact one-line description of the result.
func (r Result) Summary() string {
	if r.IsEmpty() {
		return fmt.Sprintf("imagem %s (%s): nenhum texto detectado",
			filepath.Base(r.Path), formatBytes(r.SizeBytes))
	}
	return fmt.Sprintf("imagem %s (%s): %d caracteres OCR",
		filepath.Base(r.Path), formatBytes(r.SizeBytes), len(r.Text))
}

func defaultLang(lang string) string {
	if strings.TrimSpace(lang) == "" {
		return "eng"
	}
	return lang
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
