// Package filesystem — Validator security regression tests.
//
// These cover the symlink-resolution and case-insensitivity fixes made to the
// path validator (ResolveVerified + Windows-normalised comparison), which are
// what prevent an agent from escaping the workspace through a symlink or from
// being falsely blocked by a case mismatch on Windows.
package filesystem

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidator_ResolveVerifiedAllowsNormalPath(t *testing.T) {
	ws := t.TempDir()
	writeFixture(t, ws, "sub/file.txt", "x")

	v := NewValidator(ws)
	resolved, err := v.ResolveVerified("sub/file.txt")
	require.NoError(t, err)
	assert.True(t, pathWithinOrEqual(ws, resolved))
}

func TestValidator_SymlinkEscapeRejected(t *testing.T) {
	ws := t.TempDir()
	outside := t.TempDir()
	writeFixture(t, outside, "secret.txt", "x")

	link := filepath.Join(ws, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink not permitted on this platform: %v", err)
	}

	v := NewValidator(ws)

	run(t, "Resolve passes (lexical) but ResolveVerified rejects", func(t *testing.T) {
		// The lexical check alone would allow it — this is the gap.
		_, err := v.Resolve("escape/secret.txt")
		require.NoError(t, err, "lexical Resolve should pass so the test proves ResolveVerified adds real value")

		_, err = v.ResolveVerified("escape/secret.txt")
		require.Error(t, err, "ResolveVerified must reject a symlink escaping")
		assert.Contains(t, err.Error(), "symlink")
	})

	run(t, "writing a new file through the symlink is rejected", func(t *testing.T) {
		_, err := v.ResolveVerified("escape/new.txt")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "symlink")
	})
}

func TestValidator_CaseInsensitiveOnWindows(t *testing.T) {
	// withinWorkspace is the container primitive; on Windows the filesystem is
	// case-insensitive, so a path whose case differs from the workspace root
	// must not be treated as an escape.
	assert.True(t, pathWithinOrEqual(`C:\Users\X\ws`, `c:\users\x\ws\file.go`))
	assert.True(t, pathWithinOrEqual(`C:\Users\X\ws\sub`, `c:\users\x\ws\sub\inner.go`))
	assert.True(t, pathWithinOrEqual(`C:\USERS\X\WS`, `c:\users\x\ws\file.go`))
	assert.True(t, pathWithinOrEqual(`/var/ws`, `/var/ws/file.go`), "POSIX stays case-sensitive but identical case passes")

	if runtime.GOOS != "windows" {
		t.Skip("the Resolve integration below is Windows-only (NTFS case-insensitivity)")
	}

	ws := t.TempDir()
	v := NewValidator(ws)

	// Build an absolute path that refers to the same filesystem location but in
	// different case (Windows resolves `..` and case-insensitively anyway).
	mixed := swapCase(ws)
	resolved, err := v.Resolve(filepath.Join(mixed, "anything.go"))
	require.NoError(t, err, "case-variant path must not be reported as an escape")
	assert.True(t, pathWithinOrEqual(ws, resolved))
}

// swapCase toggles the case of ASCII letters in s. It flips the case of the
// workspace path so the resulting string refers to the same location on a
// case-insensitive filesystem (Windows) while being distinct as a string.
func swapCase(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
			out[i] = c - 'a' + 'A'
		case c >= 'A' && c <= 'Z':
			out[i] = c - 'A' + 'a'
		default:
			out[i] = c
		}
	}
	return string(out)
}
