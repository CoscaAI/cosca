// Package embed provides access to the embedded Cosca framework assets.
// All Cosca markdown files (skills, workflows, departments, templates, etc.)
// are compiled directly into the binary, making the CLI fully self-contained
// with no external dependencies.
//
// The assets are embedded using Go's //go:embed directive and accessed
// through the CoscaAssets filesystem.
package embed

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

//go:embed cosca
var coscaAssets embed.FS

// CoscaAssets is the embedded filesystem containing all Cosca framework files.
// Use it to read skills, workflows, departments, templates, and other
// framework assets at runtime.
var CoscaAssets = coscaAssets

// CoscaRoot is the root directory within the embedded filesystem.
const CoscaRoot = "cosca"

// ReadFile reads a single file from the embedded Cosca assets.
// Name is relative to the cosca/ directory (e.g., "skills/architecture/ANALYSIS.md").
//
// Usa path.Join (não filepath.Join): o embed.FS exige separador "/" em TODAS
// as plataformas — filepath.Join produziria "cosca\knowledge" no Windows.
func ReadFile(name string) ([]byte, error) {
	fullPath := path.Join(CoscaRoot, name)
	return CoscaAssets.ReadFile(fullPath)
}

// ReadDir reads a directory from the embedded Cosca assets.
// Name is relative to the cosca/ directory (e.g., "skills" or "workflows").
func ReadDir(name string) ([]fs.DirEntry, error) {
	fullPath := path.Join(CoscaRoot, name)
	return CoscaAssets.ReadDir(fullPath)
}

// Glob finds all files matching the given pattern within the embedded Cosca assets.
// Pattern is relative to the cosca/ directory (e.g., "skills/**/*.md").
func Glob(pattern string) ([]string, error) {
	fullPattern := path.Join(CoscaRoot, pattern)
	return fs.Glob(CoscaAssets, fullPattern)
}

// WalkDir walks the embedded Cosca filesystem starting at the given path.
func WalkDir(name string, fn fs.WalkDirFunc) error {
	fullPath := path.Join(CoscaRoot, name)
	return fs.WalkDir(CoscaAssets, fullPath, fn)
}

// ListFilesRecursive returns all .md files in a directory tree, recursively.
// Dir is relative to the cosca/ directory.
func ListFilesRecursive(dir string) ([]string, error) {
	var files []string
	root := path.Join(CoscaRoot, dir)
	err := fs.WalkDir(CoscaAssets, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			// Return path relative to CoscaRoot
			relPath := strings.TrimPrefix(p, CoscaRoot+"/")
			files = append(files, relPath)
		}
		return nil
	})
	return files, err
}

// Exists checks if a file or directory exists in the embedded assets.
func Exists(name string) bool {
	fullPath := path.Join(CoscaRoot, name)
	_, err := CoscaAssets.Open(fullPath)
	return err == nil
}

// ReadString returns the content of a file as a string.
func ReadString(name string) (string, error) {
	data, err := ReadFile(name)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FallbackDirs are the embedded framework subtrees materialized under
// <root>/.cosca/fallback/ so standalone consumers (cosca-indexer, circadian
// ORC, knowledge compiler) can read the canonical content off disk without
// walking the go:embed filesystem.
var FallbackDirs = []string{"knowledge", "memory", "workflows", "engines"}

// MaterializeFallback copies the canonical framework trees from the embedded
// filesystem into <root>/.cosca/fallback/, keeping every consumer of the
// fallback tree functional even on fresh installs. It is idempotent:
// embedded files are (re)written on every run, and stale extra files are
// left untouched. It is safe to call concurrently with a running daemon.
func MaterializeFallback(root string) error {
	fallbackDir := filepath.Join(root, ".cosca", "fallback")
	if err := os.MkdirAll(fallbackDir, 0o700); err != nil {
		return fmt.Errorf("mkdir fallback: %w", err)
	}
	if err := os.Chmod(fallbackDir, 0o700); err != nil {
		return fmt.Errorf("restrict fallback permissions: %w", err)
	}

	for _, sub := range FallbackDirs {
		err := WalkDir(sub, func(p string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			// O path do WalkDir do embed.FS já vem com separador "/" (canônico
			// do embed.FS em qualquer plataforma). filepath.Rel converteria o
			// prefixo "cosca/" para separador Windows — usamos TrimPrefix para
			// manter o "/" e só convertemos para separador do SO no target.
			rel := strings.TrimPrefix(p, CoscaRoot+"/")
			target := filepath.Join(fallbackDir, filepath.FromSlash(rel))
			if d.IsDir() {
				if err := os.MkdirAll(target, 0o700); err != nil {
					return fmt.Errorf("mkdir %q: %w", target, err)
				}
				return os.Chmod(target, 0o700)
			}
			data, readErr := ReadFile(rel)
			if readErr != nil {
				return fmt.Errorf("read embed %q: %w", rel, readErr)
			}
			if writeErr := os.WriteFile(target, data, 0o600); writeErr != nil {
				return fmt.Errorf("write %q: %w", target, writeErr)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("materialize fallback/%s: %w", sub, err)
		}
	}

	return nil
}
