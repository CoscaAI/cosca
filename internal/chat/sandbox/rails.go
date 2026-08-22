package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultBlockedDirs is the default set of blocked directory path suffixes.
// These directories are considered sensitive or non-project code and are
// excluded from sandbox access:
//   - .git/         — repository metadata, not project code
//   - .cosca/data/  — contains encrypted keys and sensitive data
//   - node_modules/ — third-party dependencies, too large for most operations
var DefaultBlockedDirs = []string{
	".git",
	".cosca/data",
	"node_modules",
}

// Rails validates file paths to prevent workspace escape and blocks access
// to sensitive directories. It resolves relative paths, follows symlinks
// safely, and enforces workspace boundaries.
type Rails struct {
	workspace   string   // absolute workspace root
	blockedDirs []string // list of blocked directory suffixes
}

// NewRails creates a path validator for the given workspace.
// The workspace should be an absolute path.
func NewRails(workspace string) *Rails {
	return &Rails{
		workspace:   filepath.Clean(workspace),
		blockedDirs: DefaultBlockedDirs,
	}
}

// Validate checks that the given path is within the workspace, not in a
// blocked directory, and does not escape via symlinks. It returns a
// descriptive error if any check fails.
func (r *Rails) Validate(path string) error {
	// Null bytes são sempre rejeitados (fail-closed — injeção de caminho).
	// filepath.Clean não remove \x00; o EvalSymlinks falharia com EINVAL, mas
	// o guard explícito deixa a intenção clara e cobre o caso antes do resolve.
	if strings.ContainsRune(path, '\x00') {
		return fmt.Errorf("path contains null byte — access denied")
	}

	clean, err := r.Resolve(path)
	if err != nil {
		return fmt.Errorf("path resolution: %w", err)
	}

	if !r.IsWithinWorkspace(clean) {
		return fmt.Errorf("path %q escapes workspace", path)
	}

	if r.IsBlocked(clean) {
		return fmt.Errorf("path %q is in a blocked directory", path)
	}

	// Prevent symlink escapes: evaluate all symlinks and verify the
	// resolved path is still within the workspace AND not blocked.
	// IsBlocked(real) closes the gap where a symlink inside the workspace
	// points to a blocked directory (e.g. workspace/.cosca/data with the
	// encrypted keys): "clean" is not blocked and "real" is within the
	// workspace, yet the access must be denied.
	//
	// O caminho pode ainda NÃO existir (ex: um agente escrevendo um arquivo
	// novo) — EvalSymlinks falha para paths inexistentes e negaria uma escrita
	// legítima. A solução: resolver os symlinks apenas da parte do caminho que
	// EXISTE (o ancestral mais próximo), verificando escapes sem bloquear
	// criações futuras.
	existing, err := resolveExistingAncestor(clean)
	if err != nil {
		return fmt.Errorf("path resolution: %w", err)
	}
	real, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return fmt.Errorf("symlink resolution failed — access denied: %w", err)
	}
	if !r.IsWithinWorkspace(real) {
		return fmt.Errorf("symlink %q escapes workspace", path)
	}
	if r.IsBlocked(real) {
		return fmt.Errorf("symlink %q resolves into a blocked directory", path)
	}

	return nil
}

// resolveExistingAncestor devolve o prefixo do caminho que existe no disco
// (o ancestral mais próximo de `path`). Se o próprio path existe, devolve ele;
// senão sobe até achar um componente existente. Nunca falha: no pior caso
// devolve o diretório raiz do caminho absoluto.
func resolveExistingAncestor(path string) (string, error) {
	clean := filepath.Clean(path)
	// Caminho absoluto: sobe até achar o que existe.
	if filepath.IsAbs(clean) {
		cur := clean
		for {
			if _, err := os.Lstat(cur); err == nil {
				return cur, nil
			}
			parent := filepath.Dir(cur)
			if parent == cur {
				return "", fmt.Errorf("no existing ancestor for %q", path)
			}
			cur = parent
		}
	}
	// Caminho relativo: resolve contra o CWD — usa o CWD como base.
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", fmt.Errorf("resolve absolute: %w", err)
	}
	return resolveExistingAncestor(abs)
}

// IsWithinWorkspace checks whether the given (already-resolved) path is
// inside the workspace root. Both paths are cleaned before comparison.
func (r *Rails) IsWithinWorkspace(path string) bool {
	clean := filepath.Clean(path)
	ws := filepath.Clean(r.workspace)
	return strings.HasPrefix(clean, ws+string(filepath.Separator)) || clean == ws
}

// IsBlocked checks whether the given (already-resolved) path falls within
// any of the blocked directories.
func (r *Rails) IsBlocked(path string) bool {
	clean := filepath.Clean(path)
	for _, blocked := range r.blockedDirs {
		blockedPath := filepath.Join(r.workspace, blocked)
		blockedPath = filepath.Clean(blockedPath)
		if strings.HasPrefix(clean, blockedPath+string(filepath.Separator)) || clean == blockedPath {
			return true
		}
	}
	return false
}

// Resolve resolves a relative path against the workspace root.
// If the path is already absolute, it is cleaned and returned as-is.
func (r *Rails) Resolve(path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	resolved := filepath.Join(r.workspace, path)
	return filepath.Clean(resolved), nil
}

// ensureDirExists is a helper that creates a directory if it doesn't exist.
// It is used internally during sandbox setup.
func ensureDirExists(dir string) error {
	info, err := os.Stat(dir)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("sandbox: %q exists but is not a directory", dir)
		}
		return nil
	}
	if os.IsNotExist(err) {
		return os.MkdirAll(dir, 0o755)
	}
	return fmt.Errorf("sandbox: cannot stat %q: %w", dir, err)
}
