package memoryintegrity

import (
	"fmt"
	"os"
	"path/filepath"
)

// ProjectRoot resolves the project root from a starting directory.
func ProjectRoot(start string) (string, error) {
	if start == "" {
		start = "."
	}
	root, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve project root: %w", err)
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return "", fmt.Errorf("project root %q is not a directory", root)
	}
	for {
		if info, err := os.Stat(filepath.Join(root, ".cosca")); err == nil && info.IsDir() {
			return root, nil
		}
		for _, marker := range []string{"go.mod", "package.json", ".git", "Cargo.toml", "pyproject.toml"} {
			if _, err := os.Stat(filepath.Join(root, marker)); err == nil {
				return root, nil
			}
		}
		parent := filepath.Dir(root)
		if parent == root {
			return filepath.Abs(start)
		}
		root = parent
	}
}
