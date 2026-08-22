package framework

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// inlineLinkRe matches inline markdown links [text](url). It intentionally
// also matches image links (the leading "!" is not part of the match),
// mirroring the legacy bash grep -oP '\[.*?\]\(\K[^)]+'.
var inlineLinkRe = regexp.MustCompile(`\[([^\]]*)\]\(([^)]*)\)`)

// isExternalLink reports whether a link targets an external resource that
// should not be resolved against the local filesystem.
func isExternalLink(link string) bool {
	switch {
	case strings.HasPrefix(link, "http://"),
		strings.HasPrefix(link, "https://"),
		strings.HasPrefix(link, "#"),
		strings.HasPrefix(link, "mailto:"):
		return true
	}
	return false
}

// normalizeLinkTarget applies the same normalization the legacy bash
// validator performs: anything after a ".md" suffix is truncated, so
// "docs/foo.md#section" becomes "docs/foo.md".
func normalizeLinkTarget(link string) string {
	if idx := strings.Index(link, ".md"); idx >= 0 {
		return link[:idx+len(".md")]
	}
	return link
}

// ValidateCrossReferences walks every markdown file under root, resolves
// each relative link against the directory of the file that contains it,
// and reports links whose target does not exist. External links (http,
// https, anchors, mailto) are ignored. This mirrors the legacy
// validate-cross-references.sh script.
func ValidateCrossReferences(root string) ([]Issue, error) {
	files, err := findMarkdownFiles(root)
	if err != nil {
		return nil, err
	}

	var issues []Issue
	for _, file := range files {
		rel := relativePath(root, file)
		content, err := os.ReadFile(file)
		if err != nil {
			issues = append(issues, Issue{
				Severity: SeverityWarning,
				File:     rel,
				Message:  fmt.Sprintf("unable to read file: %v", err),
			})
			continue
		}

		baseDir := filepath.Dir(file)
		matches := inlineLinkRe.FindAllStringSubmatch(string(content), -1)
		for _, m := range matches {
			if len(m) < 3 {
				continue
			}
			link := m[2]
			if isExternalLink(link) {
				continue
			}
			link = normalizeLinkTarget(link)
			if link == "" {
				continue
			}

			target := filepath.Clean(filepath.Join(baseDir, link))
			if dirEntryExists(target) {
				continue
			}
			issues = append(issues, Issue{
				Severity: SeverityError,
				File:     rel,
				Message:  fmt.Sprintf("broken link %q (target %q does not exist)", link, target),
			})
		}
	}

	return issues, nil
}
