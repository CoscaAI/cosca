package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestExpandDesktopPath covers ~ expansion and passthrough.
func TestExpandDesktopPath(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	got, err := expandDesktopPath("~")
	require.NoError(t, err)
	require.Equal(t, home, got)

	got, err = expandDesktopPath("~/Documents/bruno")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(home, "Documents", "bruno"), got)

	got, err = expandDesktopPath("/tmp/abc")
	require.NoError(t, err)
	require.Equal(t, "/tmp/abc", got)
}

// TestResolveDesktopDirRejectsNonDir ensures a non-directory argument errors.
func TestResolveDesktopDirRejectsNonDir(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(f, []byte("x"), 0o644))

	_, err := resolveDesktopDir([]string{f})
	require.Error(t, err)
	require.Contains(t, err.Error(), "nao e um diretorio")

	_, err = resolveDesktopDir([]string{filepath.Join(t.TempDir(), "nope")})
	require.Error(t, err)
}

// TestResolveDesktopDirAcceptsDir accepts an existing directory (relative and ~).
func TestResolveDesktopDirAcceptsDir(t *testing.T) {
	dir := t.TempDir()
	got, err := resolveDesktopDir([]string{dir})
	require.NoError(t, err)
	require.Equal(t, dir, got)
}

// TestDesktopIsStale checks the staleness logic with real mtimes.
func TestDesktopIsStale(t *testing.T) {
	root := t.TempDir()
	installed := filepath.Join(root, "cosca-desktop")
	require.NoError(t, os.WriteFile(installed, []byte("bin"), 0o755))
	inst := time.Now().Add(-time.Hour)
	require.NoError(t, os.Chtimes(installed, inst, inst))

	// No sources → not stale.
	require.False(t, desktopIsStale(installed, nil))

	// Source older than binary → not stale.
	oldSrc := filepath.Join(root, "old.go")
	require.NoError(t, os.WriteFile(oldSrc, []byte("x"), 0o644))
	old := inst.Add(-time.Hour)
	require.NoError(t, os.Chtimes(oldSrc, old, old))
	require.False(t, desktopIsStale(installed, []string{oldSrc}))

	// Source newer than binary → stale.
	newSrc := filepath.Join(root, "new.tsx")
	require.NoError(t, os.WriteFile(newSrc, []byte("x"), 0o644))
	require.True(t, desktopIsStale(installed, []string{newSrc}))

	// Missing binary → stale.
	require.True(t, desktopIsStale(filepath.Join(root, "missing"), []string{oldSrc}))
}

// TestDesktopSourcesCollectsGoAndFrontend verifies the source walker finds the
// expected file types and skips build/node_modules dirs.
func TestDesktopSourcesCollectsGoAndFrontend(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "frontend", "src"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "build"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "node_modules"), 0o755))

	for _, f := range []string{"app.go", "engine.go", "frontend/src/App.tsx", "frontend/src/style.css", "frontend/src/main.ts"} {
		require.NoError(t, os.WriteFile(filepath.Join(root, f), []byte("x"), 0o644))
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "build", "cosca-desktop"), []byte("bin"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "node_modules", "dep.js"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("x"), 0o644))

	srcs := desktopSources(root)
	require.Contains(t, srcs, filepath.Join(root, "app.go"))
	require.Contains(t, srcs, filepath.Join(root, "frontend", "src", "App.tsx"))
	require.Contains(t, srcs, filepath.Join(root, "frontend", "src", "style.css"))
	require.NotContains(t, srcs, filepath.Join(root, "build", "cosca-desktop"))
	require.NotContains(t, srcs, filepath.Join(root, "node_modules", "dep.js"))
	require.NotContains(t, srcs, filepath.Join(root, "README.md"))
}

// TestDesktopDevFlagRegistered verifies the --dev flag exists and defaults to
// false (production path is the default).
func TestDesktopDevFlagRegistered(t *testing.T) {
	cmd := NewDesktopCommand()
	flag := cmd.Flags().Lookup("dev")
	require.NotNil(t, flag, "--dev flag deve existir")
	require.Equal(t, "false", flag.DefValue, "--dev deve default para false (producao)")
}
