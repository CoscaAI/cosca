package framework

import (
	"os"
	"path/filepath"
	"time"
)

// Report builds a health inventory of the Cosca framework under root,
// mirroring the legacy health-report.sh script: total markdown files,
// departments, skills, engines, workflows, templates, agents, memory
// records, ADRs, broken cross-reference links and orphan files.
func Report(root string) (*HealthReport, error) {
	report := &HealthReport{
		Root:        root,
		GeneratedAt: time.Now(),
	}

	mds, err := findMarkdownFiles(root)
	if err != nil {
		return nil, err
	}
	report.TotalMarkdownFiles = len(mds)

	// Departments: departments/*/SKILL.md.
	if deps, err := filepath.Glob(filepath.Join(root, "departments", "*", "SKILL.md")); err == nil {
		report.Departments = len(deps)
	}

	// Skills: skills/**/*.md, excluding SKILLS_CATALOG.md.
	report.Skills = countMarkdownFiles(filepath.Join(root, "skills"), func(path string) bool {
		return filepath.Base(path) == "SKILLS_CATALOG.md"
	})

	// Engines: engines/*/SKILL.md.
	if engs, err := filepath.Glob(filepath.Join(root, "engines", "*", "SKILL.md")); err == nil {
		report.Engines = len(engs)
	}

	// Workflows: workflows/*.md.
	if wfs, err := filepath.Glob(filepath.Join(root, "workflows", "*.md")); err == nil {
		report.Workflows = len(wfs)
	}

	// Templates: templates/*/TEMPLATE.md.
	if tpls, err := filepath.Glob(filepath.Join(root, "templates", "*", "TEMPLATE.md")); err == nil {
		report.Templates = len(tpls)
	}

	// Agents: directories under agents/.
	report.Agents = countDirs(filepath.Join(root, "agents"))

	// Memory records: memory/**/*.md, excluding INDEX.md.
	report.MemoryRecords = countMarkdownFiles(filepath.Join(root, "memory"), func(path string) bool {
		return filepath.Base(path) == "INDEX.md"
	})

	// ADRs: memory/architecture/adr/*.md.
	if adrs, err := filepath.Glob(filepath.Join(root, "memory", "architecture", "adr", "*.md")); err == nil {
		report.ADRs = len(adrs)
	}

	broken, err := ValidateCrossReferences(root)
	if err != nil {
		return nil, err
	}
	report.BrokenLinks = len(broken)

	orphans, err := DetectOrphans(root)
	if err != nil {
		return nil, err
	}
	report.OrphanCount = len(orphans)

	return report, nil
}

// countMarkdownFiles counts .md files under root, optionally skipping paths
// matched by skip. Non-existent directories yield zero.
func countMarkdownFiles(root string, skip func(path string) bool) int {
	count := 0
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(d.Name()) != ".md" {
			return nil
		}
		if skip != nil && skip(path) {
			return nil
		}
		count++
		return nil
	})
	return count
}

// countDirs counts immediate subdirectories of root.
func countDirs(root string) int {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() {
			count++
		}
	}
	return count
}
