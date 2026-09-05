// Package filesystem provides cross-platform filesystem tools (write_file,
// read_file, edit_file, list_dir, glob) that an LLM can invoke through the
// Cosca Agent runtime. The tools are pure Go (os/io/filepath) and therefore
// work identically on Linux and Windows — they never depend on the Linux-only
// bubblewrap sandbox.
//
// Every tool validates the requested path against the workspace root BEFORE
// touching the filesystem, so an agent can never read/write outside the
// project (path traversal prevention). The validation is re-used from the
// sandbox.Rails semantics (prefix containment + traversal rejection + null
// byte guard + symlink resolution) but is self-contained here so the tools
// have no dependency on the OS sandbox layer.
package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DefaultBlockedDirs mirrors the sensitive directory suffixes that should
// never be accessed by an agent even when they sit inside the workspace.
// Kept in sync with sandbox.DefaultBlockedDirs.
var DefaultBlockedDirs = []string{
	".git",
	".cosca/data",
	"node_modules",
}

// Validator resolves and validates a path so it stays inside the workspace
// root (path traversal prevention). It is cross-platform by construction: it
// uses filepath.Abs/Clean (which normalise Windows backslashes and POSIX
// forward slashes) plus a canonical prefix check.
//
// It also resolves symbolic links: a symlink inside the workspace can point
// outside, so the lexical prefix check alone is insufficient. `ResolveVerified`
// follows real symlinks and re-verifies containment against the symlink-free
// namespace, mirroring sandbox.Rails.Validate.
type Validator struct {
	// workspaceAbs is the absolute, cleaned workspace root (lexical form).
	workspaceAbs string

	// workspaceReal is the workspace root with its own symlinks resolved, so
	// real-path comparisons share the same namespace as filepath.EvalSymlinks
	// (e.g. macOS /var → /private/var).
	workspaceReal string

	// blockedDirs are workspace-relative directory suffixes that are blocked
	// even when nested inside the workspace (e.g. .git, node_modules).
	blockedDirs []string
}

// NewValidator creates a Validator bound to the given workspace directory.
// The workspace is resolved to an absolute path; relative paths passed to
// Resolve/Validate are joined against it.
func NewValidator(workspace string) *Validator {
	abs := workspace
	if a, err := filepath.Abs(workspace); err == nil {
		abs = a
	}
	abs = filepath.Clean(abs)

	real := abs
	if r, err := filepath.EvalSymlinks(abs); err == nil {
		real = filepath.Clean(r)
	}

	return &Validator{
		workspaceAbs:  abs,
		workspaceReal: real,
		blockedDirs:   DefaultBlockedDirs,
	}
}

// Workspace returns the absolute, cleaned (lexical) workspace root.
func (v *Validator) Workspace() string {
	return v.workspaceAbs
}

// Validate checks that the requested path stays inside the workspace and is
// not in a blocked directory. It returns a descriptive error on failure. It is
// the lexical check only; use ResolveVerified for the full symlink-aware check.
func (v *Validator) Validate(path string) error {
	_, err := v.Resolve(path)
	return err
}

// Resolve joins a (relative or absolute) path with the workspace root,
// cleans it, and verifies the result is inside the workspace. It returns the
// absolute resolved path on success.
//
// Security guarantees:
//   - Null bytes are always rejected (fail-closed — path injection guard).
//   - A relative path such as "../outside.txt" or "src/../../evil.txt" resolves
//     through filepath.Join + Clean to a location OUTSIDE the workspace, which
//     the prefix check rejects.
//   - An absolute path must itself live inside the workspace.
//   - Blocked directories (.git, .cosca/data, node_modules) are denied even
//     when nested inside the workspace.
//
// NOTE: this is the LEXICAL check. It does not follow symbolic links. Use
// ResolveVerified for an operation that will read/write the filesystem, so a
// symlink inside the workspace cannot escape it.
func (v *Validator) Resolve(path string) (string, error) {
	if strings.ContainsRune(path, '\x00') {
		return "", fmt.Errorf("path contains null byte — access denied")
	}

	var joined string
	if filepath.IsAbs(path) {
		joined = filepath.Clean(path)
	} else {
		joined = filepath.Join(v.workspaceAbs, path)
	}
	joined = filepath.Clean(joined)

	if !withinWorkspace(v.workspaceAbs, joined) {
		return "", fmt.Errorf("path %q escapes workspace", path)
	}
	if v.isBlocked(joined) {
		return "", fmt.Errorf("path %q is in a blocked directory", path)
	}
	return joined, nil
}

// ResolveVerified is Resolve plus a real (symlink-resolved) containment check.
// A symlink inside the workspace can point OUTSIDE it; the lexical prefix check
// in Resolve would not catch that. ResolveVerified evaluates all symbolic links
// and re-verifies the real path stays inside the workspace (and that it is not
// blocked). It tolerates paths that do not exist yet (e.g. a file about to be
// created) by resolving the deepest existing ancestor — mirroring
// sandbox.Rails.Validate.
func (v *Validator) ResolveVerified(path string) (string, error) {
	resolved, err := v.Resolve(path)
	if err != nil {
		return "", err
	}

	// Evaluate the deepest existing ancestor's symlinks and confirm the real
	// location is still inside the workspace. A non-existent child can never be
	// a symlink, so verifying the existing ancestor closes every escape the OS
	// would follow when creating the file/dirs.
	existing, err := resolveExistingAncestor(resolved)
	if err != nil {
		return "", fmt.Errorf("path resolution: %w", err)
	}
	real, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", fmt.Errorf("symlink resolution failed — access denied: %w", err)
	}
	if !withinWorkspace(v.workspaceReal, real) {
		return "", fmt.Errorf("path %q resolves outside the workspace via symlink", path)
	}
	if v.isBlockedReal(real) {
		return "", fmt.Errorf("path %q resolves into a blocked directory via symlink", path)
	}
	return resolved, nil
}

// resolveExistingAncestor returns the existing on-disk prefix of path (the
// closest existing ancestor). If the path itself exists it is returned; else it
// walks up until it finds an existing component. It never fails on an absolute
// path: at worst it returns the filesystem root.
func resolveExistingAncestor(path string) (string, error) {
	clean := filepath.Clean(path)
	if filepath.IsAbs(clean) {
		cur := clean
		for {
			if _, err := os.Lstat(cur); err == nil {
				return cur, nil
			}
			parent := filepath.Dir(cur)
			if parent == cur {
				return clean, nil // reached root; Resolve already vetted containment
			}
			cur = parent
		}
	}
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", fmt.Errorf("resolve absolute: %w", err)
	}
	return resolveExistingAncestor(abs)
}

// isBlocked reports whether the absolute (lexical) path sits inside a blocked
// directory resolved against the workspace root.
func (v *Validator) isBlocked(path string) bool {
	for _, blocked := range v.blockedDirs {
		blockedPath := filepath.Clean(filepath.Join(v.workspaceAbs, blocked))
		if withinWorkspace(blockedPath, path) {
			return true
		}
	}
	return false
}

// isBlockedReal reports whether a real (symlink-resolved) path sits inside a
// blocked directory, using the symlink-free workspace namespace.
func (v *Validator) isBlockedReal(real string) bool {
	for _, blocked := range v.blockedDirs {
		blockedPath := filepath.Clean(filepath.Join(v.workspaceReal, blocked))
		if withinWorkspace(blockedPath, real) {
			return true
		}
	}
	return false
}

// withinWorkspace reports whether path is equal to or a descendant of ws.
// Both paths are cleaned before comparison so `..` segments are already
// resolved and separator style (Windows \ vs POSIX /) is normalised. On
// Windows (case-insensitive filesystem) the comparison is case-insensitive —
// the returned/used paths are never mutated, only the comparison.
func withinWorkspace(ws, path string) bool {
	return pathWithinOrEqual(ws, path)
}

// pathWithinOrEqual reports whether path is equal to or a descendant of root,
// normalising for case on Windows where the filesystem is case-insensitive.
// The caller's original path strings are never altered; only the comparison.
func pathWithinOrEqual(root, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		root = strings.ToLower(root)
		path = strings.ToLower(path)
	}
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(filepath.Separator))
}
