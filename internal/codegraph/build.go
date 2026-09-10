// BuildGraph turns a directory tree into a queryable code graph: one node per
// source file (+ stubbed external modules) and "imports" edges between them.
//
// Resolution is deliberately heuristic (v1): imports are matched against local
// files by relative path, basename, and index-file fallback. Everything that
// does not resolve to a file becomes an external "module" node. A file that
// cannot be read or parsed is skipped — the build never aborts (fail-open).
package codegraph

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/CoscaAI/cosca/internal/codeindex"
	"github.com/CoscaAI/cosca/internal/graph"
)

// skipDirs are dependency/build/runtime directories never scanned for source.
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	".cosca":       true,
	"target":       true,
	"__pycache__":  true,
	".gradle":      true,
	"bin":          true,
	"obj":          true,
	"out":          true,
	"laboratory":   true, // experimentos/modelos locais (gitignored) — nunca no índice de código
}

// indexFiles are treated as the entry point of their directory when a bare
// import references the directory (e.g. `import './lib'` -> lib/index.ts).
var indexFiles = map[string]bool{
	"index":    true,
	"__init__": true,
}

type fileInfo struct {
	rel  string // forward-slash relative path, no leading "./"
	lang string
}

// BuildGraph scans root and returns a graph of files and modules.
func BuildGraph(root string) (*graph.Graph, error) {
	files, err := collectFiles(root)
	if err != nil {
		return nil, err
	}

	g := graph.New()

	// Lookup: rel-without-ext -> rel (or dir -> index file), full basename -> rel.
	byRel := make(map[string]string, len(files))
	baseFull := make(map[string]string, len(files))
	for _, fi := range files {
		base := path.Base(fi.rel)
		noExt := trimExt(fi.rel)
		if indexFiles[noExt] {
			byRel[path.Dir(fi.rel)] = fi.rel
		} else {
			byRel[noExt] = fi.rel
		}
		if _, exists := baseFull[base]; !exists {
			baseFull[base] = fi.rel
		}
	}

	for _, fi := range files {
		srcNodeID := fi.rel
		content, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(fi.rel)))
		if readErr != nil {
			continue // fail-open: unreadable file is skipped, not fatal
		}

		absPath := filepath.Join(root, filepath.FromSlash(fi.rel))
		g.AddNode(&graph.Node{
			ID:   srcNodeID,
			Type: "file",
			Name: path.Base(fi.rel),
			Path: fi.rel,
			Metadata: map[string]interface{}{
				"lang":    fi.lang,
				"symbols": extractSymbolsFor(fi, absPath, string(content)),
			},
		})

		for _, imp := range ExtractImports(fi.rel, string(content), fi.lang) {
			targetRel, ok := resolveImport(byRel, baseFull, fi.rel, imp)
			if !ok {
				moduleID := "module:" + imp
				if _, exists := g.GetNode(moduleID); !exists {
					g.AddNode(&graph.Node{
						ID:   moduleID,
						Type: "module",
						Name: imp,
					})
				}
				addImportEdge(g, srcNodeID, moduleID, fi.lang, imp)
				continue
			}
			addImportEdge(g, srcNodeID, targetRel, fi.lang, imp)
		}
	}

	return g, nil
}

// collectFiles walks root and returns all supported-language source files.
func collectFiles(root string) ([]fileInfo, error) {
	var files []fileInfo
	err := filepath.Walk(root, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil // fail-open: unreadable entry is skipped
		}
		if info.IsDir() {
			if p != root && skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		lang, ok := LanguageForFile(rel)
		if !ok {
			return nil
		}
		files = append(files, fileInfo{rel: rel, lang: lang})
		return nil
	})
	return files, err
}

// resolveImport maps an import target to a local file, or reports it external.
func resolveImport(byRel, baseFull map[string]string, fileRel, imp string) (string, bool) {
	if strings.HasPrefix(imp, "./") || strings.HasPrefix(imp, "../") {
		dir := path.Dir(fileRel)
		candidate := path.Clean(path.Join(dir, imp))
		if rel, ok := byRel[candidate]; ok {
			return rel, true
		}
		if noExt := trimExt(candidate); noExt != candidate {
			if rel, ok := byRel[noExt]; ok {
				return rel, true
			}
		}
		if rel, ok := byRel[path.Join(candidate, "index")]; ok {
			return rel, true
		}
		return "", false
	}

	if rel, ok := byRel[imp]; ok {
		return rel, true
	}
	if rel, ok := baseFull[imp]; ok {
		return rel, true
	}
	if rel, ok := baseFull[path.Base(imp)]; ok {
		return rel, true
	}
	return "", false
}

// trimExt strips the final extension (including dot) from a path.
func trimExt(p string) string {
	ext := path.Ext(p)
	if ext == "" {
		return p
	}
	return strings.TrimSuffix(p, ext)
}

func extractSymbolsFor(fi fileInfo, absPath, content string) []string {
	var names []string
	if fi.lang == "go" {
		// Go symbols come from the AST extractor — precise, not heuristic.
		syms, err := codeindex.ExtractFile(absPath)
		if err != nil {
			return nil
		}
		for _, s := range syms {
			names = append(names, s.Name)
		}
		return names
	}
	for _, s := range ExtractSymbols(content, fi.lang) {
		names = append(names, s.Name)
	}
	return names
}

func addImportEdge(g *graph.Graph, src, target, lang, imp string) {
	g.AddEdge(&graph.Edge{
		Source: src,
		Target: target,
		Type:   graph.RelImports,
		Weight: 1.0,
		Metadata: map[string]interface{}{
			"lang":   lang,
			"import": imp,
		},
	})
}
