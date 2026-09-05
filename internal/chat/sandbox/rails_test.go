// Package sandbox provides tests for the Workspace Rails path validation system.
// These tests verify path resolution, workspace boundary enforcement, blocked
// directory detection, and symlink escape prevention.
package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Helpers ─────────────────────────────────────────────────────────────────

// newTestRails creates a Rails bound to the given workspace path.
// The workspace is cleaned via filepath.Clean before use.
func newTestRails(t *testing.T, workspace string) *Rails {
	t.Helper()
	return NewRails(workspace)
}

// createFile is a helper that writes content to a file, creating parent
// directories as needed.
func createFile(t *testing.T, path string) {
	t.Helper()
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(path, []byte("test"), 0644)
	require.NoError(t, err)
}

// ─── IsWithinWorkspace ───────────────────────────────────────────────────────

func TestRails_IsWithinWorkspace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		workspace string // the workspace root
		path      string // the path to check
		want      bool
	}{
		{
			name:      "exact workspace root",
			workspace: "/home/user/project",
			path:      "/home/user/project",
			want:      true,
		},
		{
			name:      "file in workspace",
			workspace: "/home/user/project",
			path:      "/home/user/project/main.go",
			want:      true,
		},
		{
			name:      "deeply nested file",
			workspace: "/home/user/project",
			path:      "/home/user/project/src/pkg/util/helper.go",
			want:      true,
		},
		{
			name:      "parent directory traversal",
			workspace: "/home/user/project",
			path:      "/home/user/project/../../etc/passwd",
			want:      false, // cleaned path is /etc/passwd
		},
		{
			name:      "path completely outside",
			workspace: "/home/user/project",
			path:      "/tmp/foo",
			want:      false,
		},
		{
			name:      "sibling directory",
			workspace: "/home/user/project",
			path:      "/home/user/other/file",
			want:      false,
		},
		{
			name:      "path with trailing slash",
			workspace: "/home/user/project",
			path:      "/home/user/project/",
			want:      true,
		},
		{
			name:      "workspace with trailing slash",
			workspace: "/home/user/project/",
			path:      "/home/user/project/file.txt",
			want:      true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newTestRails(t, tc.workspace)
			got := r.IsWithinWorkspace(tc.path)
			assert.Equal(t, tc.want, got)
		})
	}
}

// ─── IsWithinWorkspace with temp dirs ────────────────────────────────────────

func TestRails_IsWithinWorkspace_TempDirs(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	t.Run("temp dir is workspace, subpath is within", func(t *testing.T) {
		r := newTestRails(t, tmpDir)
		subPath := filepath.Join(tmpDir, "subdir", "file.go")
		assert.True(t, r.IsWithinWorkspace(subPath))
	})

	t.Run("outside temp dir is rejected", func(t *testing.T) {
		r := newTestRails(t, tmpDir)
		assert.False(t, r.IsWithinWorkspace("/etc/passwd"))
	})
}

// ─── IsBlocked ───────────────────────────────────────────────────────────────

func TestRails_IsBlocked(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	r := newTestRails(t, tmpDir)

	tests := []struct {
		name string
		path string // absolute path to check
		want bool
	}{
		{
			name: ".git directory itself is blocked",
			path: filepath.Join(tmpDir, ".git"),
			want: true,
		},
		{
			name: "file inside .git is blocked",
			path: filepath.Join(tmpDir, ".git", "HEAD"),
			want: true,
		},
		{
			// IsBlocked only checks paths relative to workspace root;
			// nested blocked dirs under subdirectories are not detected
			// by this method (Validate catches them via Resolve).
			name: "nested .git inside subdir is NOT blocked by IsBlocked alone",
			path: filepath.Join(tmpDir, "subdir", ".git", "config"),
			want: false,
		},
		{
			name: ".cosca/data directory is blocked",
			path: filepath.Join(tmpDir, ".cosca", "data"),
			want: true,
		},
		{
			name: "file inside .cosca/data is blocked",
			path: filepath.Join(tmpDir, ".cosca", "data", "keys.json"),
			want: true,
		},
		{
			name: "node_modules directory is blocked",
			path: filepath.Join(tmpDir, "node_modules"),
			want: true,
		},
		{
			name: "file inside node_modules is blocked",
			path: filepath.Join(tmpDir, "node_modules", "pkg", "index.js"),
			want: true,
		},
		{
			name: "regular source file is not blocked",
			path: filepath.Join(tmpDir, "src", "main.go"),
			want: false,
		},
		{
			name: "non-blocked .cosca path (not .cosca/data)",
			path: filepath.Join(tmpDir, ".cosca", "config.yaml"),
			want: false,
		},
		{
			name: "non-blocked hidden directory",
			path: filepath.Join(tmpDir, ".vscode", "settings.json"),
			want: false,
		},
		{
			name: "workspace root itself is blocked (when it matches blocked name)",
			path: tmpDir,
			want: false, // workspace root is not blocked even if name matches
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := r.IsBlocked(tc.path)
			assert.Equal(t, tc.want, got)
		})
	}
}

// ─── Resolve ─────────────────────────────────────────────────────────────────

func TestRails_Resolve(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	r := newTestRails(t, tmpDir)

	t.Run("resolves relative path against workspace", func(t *testing.T) {
		got, err := r.Resolve("foo/bar.txt")
		require.NoError(t, err)
		want := filepath.Join(tmpDir, "foo", "bar.txt")
		assert.Equal(t, want, got)
	})

	t.Run("preserves absolute path", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			// "/tmp/foo.txt" não é um path absoluto no Windows (falta o
			// drive), então o Rails resolve contra o workspace — semântica de
			// path absoluto POSIX.
			t.Skip("semântica de path absoluto POSIX — não aplicável no Windows")
		}
		got, err := r.Resolve("/tmp/foo.txt")
		require.NoError(t, err)
		assert.Equal(t, "/tmp/foo.txt", got)
	})

	t.Run("cleans absolute path", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			// Mesma justificativa do subteste "preserves absolute path".
			t.Skip("semântica de path absoluto POSIX — não aplicável no Windows")
		}
		got, err := r.Resolve("/tmp/../tmp/./foo.txt")
		require.NoError(t, err)
		assert.Equal(t, "/tmp/foo.txt", got)
	})

	t.Run("empty path resolves to workspace root", func(t *testing.T) {
		got, err := r.Resolve("")
		require.NoError(t, err)
		assert.Equal(t, filepath.Clean(tmpDir), got)
	})

	t.Run("single dot resolves to workspace root", func(t *testing.T) {
		got, err := r.Resolve(".")
		require.NoError(t, err)
		assert.Equal(t, filepath.Clean(tmpDir), got)
	})

	t.Run("resolves path with parent traversal", func(t *testing.T) {
		got, err := r.Resolve("foo/../bar.txt")
		require.NoError(t, err)
		want := filepath.Join(tmpDir, "bar.txt")
		assert.Equal(t, want, got)
	})
}

// ─── Validate ────────────────────────────────────────────────────────────────

func TestRails_Validate(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	r := newTestRails(t, tmpDir)

	t.Run("valid path passes", func(t *testing.T) {
		createFile(t, filepath.Join(tmpDir, "valid.txt"))
		err := r.Validate("valid.txt")
		assert.NoError(t, err)
	})

	t.Run("valid path in subdirectory passes", func(t *testing.T) {
		createFile(t, filepath.Join(tmpDir, "sub", "deep", "file.go"))
		err := r.Validate("sub/deep/file.go")
		assert.NoError(t, err)
	})

	t.Run("absolute path outside workspace fails", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			// "/etc/passwd" não é um path absoluto no Windows (falta o drive);
			// resolve contra o workspace e o escape não é detectado.
			t.Skip("semântica de path absoluto POSIX — não aplicável no Windows")
		}
		err := r.Validate("/etc/passwd")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "escapes workspace")
	})

	t.Run("path traversal outside workspace fails", func(t *testing.T) {
		err := r.Validate("../../etc/passwd")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "escapes workspace")
	})

	t.Run("path in .git directory fails", func(t *testing.T) {
		createFile(t, filepath.Join(tmpDir, ".git", "HEAD"))
		err := r.Validate(".git/HEAD")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "blocked directory")
	})

	t.Run("path in node_modules fails", func(t *testing.T) {
		createFile(t, filepath.Join(tmpDir, "node_modules", "pkg", "index.js"))
		err := r.Validate("node_modules/pkg/index.js")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "blocked directory")
	})

	t.Run("path in .cosca/data fails", func(t *testing.T) {
		createFile(t, filepath.Join(tmpDir, ".cosca", "data", "secret"))
		err := r.Validate(".cosca/data/secret")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "blocked directory")
	})
}

// ─── Validate: Symlink Escape ────────────────────────────────────────────────

func TestRails_Validate_SymlinkEscape(t *testing.T) {
	// Symlink tests cannot run in parallel because they modify the filesystem
	// in ways that could interfere.

	tmpDir := t.TempDir()
	r := newTestRails(t, tmpDir)

	// Create a target outside the workspace
	outsideFile := filepath.Join(t.TempDir(), "outside.txt")
	createFile(t, outsideFile)

	// Create a symlink inside the workspace pointing outside
	symlinkPath := filepath.Join(tmpDir, "escape_link")
	err := os.Symlink(outsideFile, symlinkPath)
	if err != nil {
		// Symlinks may not be supported on all platforms (e.g. Windows without
		// developer mode). Skip if unsupported.
		t.Skipf("symlink creation not supported: %v", err)
	}

	t.Run("symlink to outside is rejected", func(t *testing.T) {
		err := r.Validate("escape_link")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "escapes workspace")
	})
}

// ─── Validate: Symlink within workspace ──────────────────────────────────────

func TestRails_Validate_SymlinkWithinWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	r := newTestRails(t, tmpDir)

	// Create a target inside workspace
	innerFile := filepath.Join(tmpDir, "target.txt")
	createFile(t, innerFile)

	// Create symlink inside workspace pointing to another inside file
	symlinkPath := filepath.Join(tmpDir, "inner_link")
	err := os.Symlink("target.txt", symlinkPath)
	if err != nil {
		t.Skipf("symlink creation not supported: %v", err)
	}

	t.Run("symlink to within workspace is allowed", func(t *testing.T) {
		err := r.Validate("inner_link")
		assert.NoError(t, err)
	})
}

func TestRails_Validate_SymlinkToBlockedDir(t *testing.T) {
	tmpDir := t.TempDir()
	r := newTestRails(t, tmpDir)

	// Create the blocked dir (.cosca/data holds the encrypted keys)
	blockedDir := filepath.Join(tmpDir, ".cosca", "data")
	if err := os.MkdirAll(blockedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	createFile(t, filepath.Join(blockedDir, "keys.bin"))

	// Create a symlink inside the workspace pointing INTO the blocked dir
	symlinkPath := filepath.Join(tmpDir, "innocent_link")
	if err := os.Symlink(".cosca/data", symlinkPath); err != nil {
		t.Skipf("symlink creation not supported: %v", err)
	}

	t.Run("symlink resolving into blocked directory is rejected", func(t *testing.T) {
		// Regression: previously the IsBlocked check only ran on the
		// pre-resolution path, so this passed and exposed .cosca/data.
		err := r.Validate("innocent_link/keys.bin")
		assert.Error(t, err)
	})

	t.Run("symlink resolving into blocked directory root is rejected", func(t *testing.T) {
		err := r.Validate("innocent_link")
		assert.Error(t, err)
	})
}

// ─── Validate: Edge Cases ────────────────────────────────────────────────────

func TestRails_Validate_EdgeCases(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	r := newTestRails(t, tmpDir)

	t.Run("empty path resolves to workspace root and passes", func(t *testing.T) {
		err := r.Validate("")
		assert.NoError(t, err)
	})

	t.Run("dot path passes", func(t *testing.T) {
		err := r.Validate(".")
		assert.NoError(t, err)
	})

	t.Run("workspace root absolute path passes", func(t *testing.T) {
		err := r.Validate(tmpDir)
		assert.NoError(t, err)
	})

	t.Run("path with special characters passes if within workspace", func(t *testing.T) {
		specialDir := filepath.Join(tmpDir, "dir with spaces")
		specialFile := filepath.Join(specialDir, "file (1).txt")
		createFile(t, specialFile)
		err := r.Validate("dir with spaces/file (1).txt")
		assert.NoError(t, err)
	})

	t.Run("very long path within workspace", func(t *testing.T) {
		longDir := tmpDir
		parts := make([]string, 50)
		for i := range parts {
			parts[i] = fmt.Sprintf("deep%d", i)
		}
		longPath := filepath.Join(longDir, filepath.Join(parts...))
		longRel := filepath.Join(parts...)

		// Only test if the path is not too long for the OS
		if len(longPath) < 4096 {
			createFile(t, longPath)
			err := r.Validate(longRel)
			assert.NoError(t, err)
		}
	})

	t.Run("path with null byte is denied (fail-closed)", func(t *testing.T) {
		// filepath.Clean does not strip null bytes, but filepath.Join keeps them.
		// EvalSymlinks cannot resolve this path (invalid argument), so the
		// fail-closed rail denies it instead of silently allowing it.
		err := r.Validate("file\x00.txt")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "access denied")
	})
}

// ─── Validate: Fail-closed on unresolved paths ───────────────────────────────

func TestRails_Validate_FailClosed(t *testing.T) {
	tmpDir := t.TempDir()
	r := newTestRails(t, tmpDir)

	// Symlink whose target does not exist: EvalSymlinks fails, so access MUST
	// be denied. Previously the err==nil guard treated this as VALID (return nil),
	// leaving a broken/malicious symlink able to slip past the rail.
	symlinkPath := filepath.Join(tmpDir, "broken_link")
	if err := os.Symlink(filepath.Join(tmpDir, "does_not_exist.txt"), symlinkPath); err != nil {
		t.Skipf("symlink creation not supported: %v", err)
	}

	t.Run("symlink to missing target is denied", func(t *testing.T) {
		err := r.Validate("broken_link")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "access denied")
	})

	t.Run("nonexistent path is allowed (file creation)", func(t *testing.T) {
		// Um caminho inexistente comum é uma ESCRITA NOVA legítima (agente
		// criando arquivo). Negá-lo bloquearia o executor (bug real: o rails
		// usava EvalSymlinks no path inteiro, que falha para paths inexistentes
		// e negava a criação). O que DEVE ser negado é o SYMLINK QUEBRADO (o
		// link existe mas aponta para nada — potencialmente malicioso), coberto
		// no subteste acima. O ancestral existente é resolvido e verificado.
		err := r.Validate("never_existed.txt")
		assert.NoError(t, err)
	})

	t.Run("existing valid path still passes (happy path intact)", func(t *testing.T) {
		createFile(t, filepath.Join(tmpDir, "ok.txt"))
		err := r.Validate("ok.txt")
		assert.NoError(t, err)
	})
}

// ─── NewRails ────────────────────────────────────────────────────────────────

func TestNewRails(t *testing.T) {
	t.Parallel()

	t.Run("cleans workspace path", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			// "/tmp/../tmp/./workspace/" é um path POSIX; no Windows o
			// filepath.Clean resolve para \tmp\workspace (semântica de path
			// absoluto POSIX não se aplica).
			t.Skip("semântica de path absoluto POSIX — não aplicável no Windows")
		}
		r := NewRails("/tmp/../tmp/./workspace/")
		assert.Equal(t, "/tmp/workspace", r.workspace)
	})

	t.Run("has default blocked dirs", func(t *testing.T) {
		r := NewRails("/tmp")
		assert.ElementsMatch(t, DefaultBlockedDirs, r.blockedDirs)
	})
}

// ─── ensureDirExists ─────────────────────────────────────────────────────────

func TestEnsureDirExists(t *testing.T) {
	t.Parallel()

	t.Run("creates non-existent directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		newDir := filepath.Join(tmpDir, "new", "nested", "dir")

		err := ensureDirExists(newDir)
		require.NoError(t, err)

		info, err := os.Stat(newDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("existing directory is a no-op", func(t *testing.T) {
		tmpDir := t.TempDir()
		err := ensureDirExists(tmpDir)
		assert.NoError(t, err)
	})

	t.Run("returns error when path is a file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "notadir")
		createFile(t, filePath)

		err := ensureDirExists(filePath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "but is not a directory")
	})
}

// ─── DefaultBlockedDirs ──────────────────────────────────────────────────────

func TestDefaultBlockedDirs(t *testing.T) {
	t.Parallel()

	assert.Contains(t, DefaultBlockedDirs, ".git")
	assert.Contains(t, DefaultBlockedDirs, ".cosca/data")
	assert.Contains(t, DefaultBlockedDirs, "node_modules")
	assert.Len(t, DefaultBlockedDirs, 3)
}

// ─── Rails: Empty workspace ──────────────────────────────────────────────────

func TestRails_EmptyWorkspace(t *testing.T) {
	t.Parallel()

	t.Run("empty workspace string is handled", func(t *testing.T) {
		r := NewRails("")
		assert.Equal(t, ".", r.workspace) // filepath.Clean("") returns "."
	})
}

// ─── Rails: Workspace with blocked name ──────────────────────────────────────

func TestRails_WorkspaceWithBlockedName(t *testing.T) {
	t.Parallel()

	// Create a workspace whose base name matches a blocked dir but is actually
	// the workspace root.
	tmpDir := t.TempDir()
	gitLikeWorkspace := filepath.Join(tmpDir, ".git")
	err := os.MkdirAll(gitLikeWorkspace, 0o755)
	require.NoError(t, err)

	r := NewRails(gitLikeWorkspace)

	t.Run("workspace root matching blocked name is accessible", func(t *testing.T) {
		createFile(t, filepath.Join(gitLikeWorkspace, "file.txt"))
		err := r.Validate("file.txt")
		assert.NoError(t, err)
	})

	t.Run("child of blocked-named workspace is blocked for actual blocked dirs", func(t *testing.T) {
		// A workspace named .git won't block its own children because the check
		// is: is the child inside {workspace}/{blocked}? Since workspace IS .git,
		// workspace/.git would be a child that's blocked. But .git itself is OK.
		childGit := filepath.Join(gitLikeWorkspace, ".git", "HEAD")
		os.MkdirAll(filepath.Dir(childGit), 0o755)
		createFile(t, childGit)
		// We need to check the relative path .git/HEAD from the workspace .git
		// This should resolve to {tmpDir}/.git/.git/HEAD which is within workspace
		// but also matches blocked pattern.
		err := r.Validate(".git/HEAD")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "blocked directory")
	})
}
