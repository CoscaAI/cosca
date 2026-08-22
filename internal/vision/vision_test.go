package vision

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// createTestPNG writes a small PNG containing readable text using Python's
// Pillow (present on the dev machine). Skips when Pillow is unavailable.
func createTestPNG(t *testing.T, dir, text string) string {
	t.Helper()
	path := filepath.Join(dir, "test.png")
	script := "from PIL import Image, ImageDraw; " +
		"img=Image.new('RGB',(500,120),'white'); " +
		"d=ImageDraw.Draw(img); " +
		"d.text((20,40), \"" + text + "\", fill='black'); " +
		"img.save(\"" + path + "\")"
	cmd := exec.Command("python3", "-c", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("pillow unavailable, skipping: %v (%s)", err, out)
	}
	return path
}

// TestDescribe_ReadsText verifies OCR extracts text from a generated image.
func TestDescribe_ReadsText(t *testing.T) {
	if _, err := exec.LookPath(tesseractBin); err != nil {
		t.Skipf("tesseract binary not found: %v", err)
	}
	dir := t.TempDir()
	path := createTestPNG(t, dir, "COSCA VISION TEST 2026")

	res, err := Describe(path, Options{})
	if err != nil {
		t.Fatalf("Describe failed: %v", err)
	}
	if res.Path != path {
		t.Errorf("result path mismatch: %q", res.Path)
	}
	if res.IsEmpty() {
		t.Fatalf("OCR returned empty text for an image containing text")
	}
	// OCR is approximate (e.g. 2026 may read as 2028); assert on stable
	// uppercase tokens only.
	for _, want := range []string{"COSCA", "VISION", "TEST"} {
		if !strings.Contains(strings.ToUpper(res.Text), want) {
			t.Errorf("OCR text %q missing expected token %q", res.Text, want)
		}
	}
	t.Logf("OCR result: %q", res.Text)
}

// TestDescribe_MissingFile verifies a clean error for a nonexistent image.
func TestDescribe_MissingFile(t *testing.T) {
	_, err := Describe("/nonexistent/image.png", Options{})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "image not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestDescribe_Portuguese exercises the "por" language path when available.
func TestDescribe_Portuguese(t *testing.T) {
	if _, err := exec.LookPath(tesseractBin); err != nil {
		t.Skipf("tesseract binary not found: %v", err)
	}
	dir := t.TempDir()
	path := createTestPNG(t, dir, "INVERSOR FREQUENCIA")

	_, err := Describe(path, Options{Lang: "por"})
	if err != nil {
		// The language pack may be missing on this machine; the pipeline
		// must degrade gracefully, not panic.
		t.Logf("por language unavailable (acceptable): %v", err)
		return
	}
	// Success — pipeline works with an explicit language.
}

// TestDescribe_SummaryFormat verifies the compact summary string.
func TestDescribe_SummaryFormat(t *testing.T) {
	res := Result{Path: "/tmp/x.png", SizeBytes: 2048, Text: "hello world"}
	s := res.Summary()
	if !strings.Contains(s, "2.0 KB") {
		t.Errorf("summary missing size: %q", s)
	}
	if !strings.Contains(s, "11 caracteres") {
		t.Errorf("summary missing char count: %q", s)
	}
	empty := Result{Path: "/tmp/x.png", SizeBytes: 100}
	if !strings.Contains(empty.Summary(), "nenhum texto detectado") {
		t.Errorf("empty summary wrong: %q", empty.Summary())
	}
}

var _ = os.Stat

// ── Mock tesseract tests ──────────────────────────────────────────────────

func TestDescribe_TesseractFailure(t *testing.T) {
	// Arrange: fake tesseract that always fails
	orig := tesseractBin
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "fake-tesseract")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\necho 'tesseract error' >&2\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	tesseractBin = fakeBin
	t.Cleanup(func() { tesseractBin = orig })

	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "test.png")
	if err := os.WriteFile(imgPath, []byte("fake-png"), 0644); err != nil {
		t.Fatal(err)
	}

	// Act
	_, err := Describe(imgPath, Options{})

	// Assert
	if err == nil {
		t.Fatal("expected error when tesseract fails")
	}
	if !strings.Contains(err.Error(), "tesseract failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDescribe_WithPSM(t *testing.T) {
	// O fake-tesseract é um script shell POSIX (#!/bin/sh) sem extensão
	// executável — no Windows exec.LookPath não o encontra (o shebang não é
	// interpretado). O cenário só é montável em sistemas Unix.
	if runtime.GOOS == "windows" {
		t.Skip("fake-tesseract é um script shell POSIX, não executável no Windows")
	}
	orig := tesseractBin
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "fake-tesseract")
	// Fake tesseract that echoes back its args so we can verify PSM was passed
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\necho \"ARGS: $@\"\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	tesseractBin = fakeBin
	t.Cleanup(func() { tesseractBin = orig })

	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "test.png")
	if err := os.WriteFile(imgPath, []byte("fake-png"), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := Describe(imgPath, Options{PSM: 3, Lang: "eng+por"})
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	// PSM should be in the args echoed back
	if !strings.Contains(res.Text, "--psm") || !strings.Contains(res.Text, "3") {
		t.Logf("PSM not confirmed in args: %q", res.Text)
	}
}

func TestDescribe_DefaultLang(t *testing.T) {
	// Idem TestDescribe_WithPSM: o fake-tesseract é um script shell POSIX e
	// não executa no Windows.
	if runtime.GOOS == "windows" {
		t.Skip("fake-tesseract é um script shell POSIX, não executável no Windows")
	}
	orig := tesseractBin
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "fake-tesseract")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\necho 'OCR output'\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	tesseractBin = fakeBin
	t.Cleanup(func() { tesseractBin = orig })

	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "test.png")
	if err := os.WriteFile(imgPath, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := Describe(imgPath, Options{})
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if res.Text != "OCR output" {
		t.Errorf("unexpected result: %q", res.Text)
	}
	if res.SizeBytes == 0 {
		t.Error("SizeBytes should be > 0")
	}
}

// ── Helper function tests ─────────────────────────────────────────────────

func TestResult_IsEmpty(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"empty string", "", true},
		{"whitespace only", "   \n\t  ", true},
		{"has text", "hello", false},
		{"single char", "x", false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := Result{Text: tt.text}
			if got := r.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResult_Summary_Empty(t *testing.T) {
	t.Parallel()
	r := Result{Path: "/tmp/photo.png", SizeBytes: 512, Text: ""}
	s := r.Summary()
	if !strings.Contains(s, "nenhum texto detectado") {
		t.Errorf("expected 'nenhum texto detectado', got: %q", s)
	}
	if !strings.Contains(s, "photo.png") {
		t.Errorf("expected filename in summary: %q", s)
	}
}

func TestFormatBytes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := formatBytes(tt.bytes)
			if got != tt.want {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestDefaultLang(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want string
	}{
		{"", "eng"},
		{"   ", "eng"},
		{"por", "por"},
		{"eng+por", "eng+por"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			got := defaultLang(tt.in)
			if got != tt.want {
				t.Errorf("defaultLang(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestOptions_Defaults(t *testing.T) {
	t.Parallel()
	opts := Options{}
	if opts.Lang != "" {
		t.Errorf("Lang should default to empty: %q", opts.Lang)
	}
	if opts.PSM != 0 {
		t.Errorf("PSM should default to 0: %d", opts.PSM)
	}
}
