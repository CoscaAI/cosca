package framework

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// findMarkdownFiles returns all .md files under root, skipping
// node_modules and .git directories (mirrors the legacy bash `find`).
func findMarkdownFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", root, err)
	}
	return files, nil
}

// relativePath converts an absolute path to a path relative to root,
// falling back to the absolute path when it cannot be made relative.
func relativePath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

// dirEntryExists reports whether path exists as a file or a directory.
func dirEntryExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
