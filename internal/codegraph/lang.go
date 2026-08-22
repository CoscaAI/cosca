// Package codegraph builds a queryable multi-language codebase graph.
// Heuristic v1 — structural precision, not AST. Pure Go stdlib, zero deps.
package codegraph

import (
	"path/filepath"
	"strings"
)

// langByExt maps file extensions to their canonical language id.
var langByExt = map[string]string{
	".go":   "go",
	".java": "java",
	".ts":   "typescript",
	".tsx":  "typescript",
	".js":   "javascript",
	".jsx":  "javascript",
	".py":   "python",
	".rs":   "rust",
	".cs":   "csharp",
	".cpp":  "cpp",
	".cc":   "cpp",
	".hpp":  "cpp",
	".c":    "c",
	".h":    "c",
}

// LanguageForFile returns the language id for a file path by extension.
// ok is false for extensions outside the supported set (e.g. md, json).
func LanguageForFile(path string) (lang string, ok bool) {
	lang, ok = langByExt[strings.ToLower(filepath.Ext(path))]
	return lang, ok
}
