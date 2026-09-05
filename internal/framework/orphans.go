package framework

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DetectOrphans returns the markdown files under root that are not
// referenced by any other markdown file. INDEX.md files and the memory/
// directory are excluded from the candidate set. A file is considered
// referenced when its basename (without the .md extension) appears in the
// content of any other markdown file, mirroring the legacy
// detect-orphans.sh / validate-orphans.sh scripts.
func DetectOrphans(root string) ([]string, error) {
	files, err := findMarkdownFiles(root)
	if err != nil {
		return nil, err
	}

	// Candidate files: everything except INDEX.md and memory/.
	var candidates []string
	for _, f := range files {
		if filepath.Base(f) == "INDEX.md" {
			continue
		}
		rel, err := filepath.Rel(root, f)
		if err == nil && (rel == "memory" || strings.HasPrefix(rel, "memory"+string(filepath.Separator))) {
			continue
		}
		candidates = append(candidates, f)
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	// Read every file once.
	contents := make(map[string]string, len(files))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		contents[f] = string(data)
	}

	// Group candidates by basename so repeated basenames (e.g. SKILL.md)
	// are checked once per source file.
	byBase := make(map[string][]string)
	for _, cand := range candidates {
		base := strings.TrimSuffix(filepath.Base(cand), ".md")
		if base == "" {
			continue
		}
		byBase[base] = append(byBase[base], cand)
	}

	// A candidate is referenced when its basename appears in the content of
	// any markdown file other than itself.
	referenced := make(map[string]bool, len(candidates))
	for _, src := range files {
		content := contents[src]
		for base, cands := range byBase {
			if !strings.Contains(content, base) {
				continue
			}
			for _, cand := range cands {
				if cand != src {
					referenced[cand] = true
				}
			}
		}
		if len(referenced) == len(candidates) {
			break
		}
	}

	var orphans []string
	for _, cand := range candidates {
		if !referenced[cand] {
			orphans = append(orphans, relativePath(root, cand))
		}
	}
	sort.Strings(orphans)
	return orphans, nil
}
