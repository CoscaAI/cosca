package authority

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ─── Wrappers de teste (herméticos, t.TempDir) ──────────────────────────────

// mkdirAll wraps os.MkdirAll with a restrictive mode.
func mkdirAll(path string) error { return os.MkdirAll(path, 0o700) }

// os_WriteFile wraps os.WriteFile with a restrictive mode.
func os_WriteFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o600)
}

// osChtimes sets atime=mtime= tm on a file (deterministic NEWER ordering).
func osChtimes(path string, tm time.Time) error {
	return os.Chtimes(path, tm, tm)
}

// mustRead returns the file content, failing the test on error.
func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	return string(b)
}

// exists reports whether a path exists (file or dir).
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// mustMkdir creates a directory, failing the test on error.
func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("mkdir %q: %v", path, err)
	}
}
