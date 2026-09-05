// Package memoryintegrity provides an offline, advisory integrity inventory for
// the files that govern Cosca's memory. It deliberately has no runtime hooks.
package memoryintegrity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	ManifestVersion = "1"
	Algorithm       = "sha256"
	DefaultManifest = ".cosca/audit/memory-integrity-manifest.json"
)

// File is one protected file and its content digest at inventory time.
type File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

// Manifest is intentionally a plain JSON document so it can be reviewed,
// copied, and removed without migration or database changes.
type Manifest struct {
	Version   string `json:"version"`
	Algorithm string `json:"algorithm"`
	Root      string `json:"root"`
	CreatedAt string `json:"created_at"`
	Files     []File `json:"files"`
}

type Result struct {
	Manifest Manifest `json:"manifest"`
	Changed  []string `json:"changed,omitempty"`
	Missing  []string `json:"missing,omitempty"`
	Added    []string `json:"added,omitempty"`
	Errors   []string `json:"errors,omitempty"`
}

func DefaultManifestPath(root string) string { return filepath.Join(root, DefaultManifest) }

// Inventory enumerates only the explicit governance/memory paths. Databases,
// sessions, and generated audit files are intentionally outside this scope.
func Inventory(root string) (Manifest, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return Manifest{}, fmt.Errorf("resolve root: %w", err)
	}
	paths, err := protectedPaths(root)
	if err != nil {
		return Manifest{}, err
	}
	files := make([]File, 0, len(paths))
	for _, path := range paths {
		f, err := hashFile(root, path)
		if errors.Is(err, os.ErrNotExist) {
			// Absence is recorded by Verify as a missing protected file rather
			// than making the scanner unable to produce a useful report.
			continue
		}
		if err != nil {
			return Manifest{}, err
		}
		files = append(files, f)
	}
	return Manifest{Version: ManifestVersion, Algorithm: Algorithm,
		Root: ".", CreatedAt: time.Now().UTC().Format(time.RFC3339), Files: files}, nil
}

func Write(root, manifestPath string) (Manifest, error) {
	return write(root, manifestPath, false)
}

// WriteWithForce writes a new manifest, replacing an existing one only when
// the caller explicitly requests it. The CLI exposes this as --force.
func WriteWithForce(root, manifestPath string) (Manifest, error) {
	return write(root, manifestPath, true)
}

func write(root, manifestPath string, force bool) (Manifest, error) {
	path := manifestPath
	if path == "" {
		path = DefaultManifestPath(root)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	if !force {
		if _, err := os.Lstat(path); err == nil {
			return Manifest{}, fmt.Errorf("manifest %q already exists; use --force to replace it", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return Manifest{}, fmt.Errorf("check existing manifest: %w", err)
		}
	}
	m, err := Inventory(root)
	if err != nil {
		return Manifest{}, err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Manifest{}, fmt.Errorf("create audit directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return Manifest{}, fmt.Errorf("secure audit directory: %w", err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return Manifest{}, err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".memory-integrity-*.tmp")
	if err != nil {
		return Manifest{}, fmt.Errorf("create manifest temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return Manifest{}, fmt.Errorf("secure manifest temporary file: %w", err)
	}
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Chmod(0o600)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return Manifest{}, fmt.Errorf("write manifest: %w", err)
	}
	if force {
		if err := os.Rename(tmpName, path); err != nil {
			return Manifest{}, fmt.Errorf("publish manifest: %w", err)
		}
	} else if err := os.Link(tmpName, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return Manifest{}, fmt.Errorf("manifest %q already exists; use --force to replace it", path)
		}
		return Manifest{}, fmt.Errorf("publish manifest without replacing existing file: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return Manifest{}, fmt.Errorf("secure manifest: %w", err)
	}
	return m, nil
}

func Verify(root, manifestPath string) (Result, error) {
	path := manifestPath
	if path == "" {
		path = DefaultManifestPath(root)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, fmt.Errorf("read manifest %q: %w", path, err)
	}
	var expected Manifest
	if err := json.Unmarshal(data, &expected); err != nil {
		return Result{}, fmt.Errorf("parse manifest: %w", err)
	}
	if expected.Version != ManifestVersion || expected.Algorithm != Algorithm {
		return Result{}, fmt.Errorf("unsupported manifest version or algorithm")
	}
	if expected.Root != "." {
		return Result{}, fmt.Errorf("manifest root must be %q", ".")
	}
	seen := make(map[string]struct{}, len(expected.Files))
	for _, f := range expected.Files {
		clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(f.Path)))
		if f.Path == "" || filepath.IsAbs(filepath.FromSlash(f.Path)) || clean != f.Path || strings.HasPrefix(f.Path, "../") || f.SHA256 == "" || len(f.SHA256) != sha256.Size*2 || f.Bytes < 0 {
			return Result{}, fmt.Errorf("invalid file entry in manifest: %q", f.Path)
		}
		if _, ok := seen[f.Path]; ok {
			return Result{}, fmt.Errorf("duplicate file entry in manifest: %q", f.Path)
		}
		seen[f.Path] = struct{}{}
	}
	actual, err := Inventory(root)
	if err != nil {
		return Result{}, err
	}
	result := Result{Manifest: expected}
	want := make(map[string]File, len(expected.Files))
	for _, f := range expected.Files {
		want[f.Path] = f
	}
	have := make(map[string]File, len(actual.Files))
	for _, f := range actual.Files {
		have[f.Path] = f
	}
	for p, f := range want {
		got, ok := have[p]
		if !ok {
			result.Missing = append(result.Missing, p)
			continue
		}
		if got.SHA256 != f.SHA256 || got.Bytes != f.Bytes {
			result.Changed = append(result.Changed, p)
		}
	}
	for p := range have {
		if _, ok := want[p]; !ok {
			result.Added = append(result.Added, p)
		}
	}
	sort.Strings(result.Changed)
	sort.Strings(result.Missing)
	sort.Strings(result.Added)
	return result, nil
}

func hashFile(root, path string) (File, error) {
	full := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Lstat(full)
	if err != nil {
		return File{}, fmt.Errorf("protected file %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return File{}, fmt.Errorf("protected path %q is not a regular file", path)
	}
	h := sha256.New()
	f, err := os.Open(full)
	if err != nil {
		return File{}, err
	}
	n, copyErr := io.Copy(h, f)
	closeErr := f.Close()
	if copyErr != nil {
		return File{}, copyErr
	}
	if closeErr != nil {
		return File{}, closeErr
	}
	return File{Path: filepath.ToSlash(path), SHA256: hex.EncodeToString(h.Sum(nil)), Bytes: n}, nil
}

func protectedPaths(root string) ([]string, error) {
	paths := []string{".cosca/framework/KERNEL.md", ".cosca/framework/CONSTITUTION.md", ".cosca/framework/MEMORY_MODEL.md", ".cosca/framework/shared/AUTO_EVOLUTION_PROTOCOL.md", ".cosca/memory/LEARNING_PROTOCOL.md"}
	pattern := filepath.Join(root, ".cosca", "fallback", "memory", "agent", "**", "*.md")
	// filepath.Glob has no ** semantics; Walk keeps the rule portable and
	// makes the exclusion of databases/session data explicit.
	base := filepath.Join(root, ".cosca", "fallback", "memory", "agent")
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			return filepath.SkipDir
		}
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".md") {
			rel, e := filepath.Rel(root, path)
			if e != nil {
				return e
			}
			paths = append(paths, filepath.ToSlash(rel))
		}
		return nil
	})
	_ = pattern // documents the intended scope without relying on shell glob semantics.
	if err != nil {
		return nil, fmt.Errorf("walk protected memory: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}
