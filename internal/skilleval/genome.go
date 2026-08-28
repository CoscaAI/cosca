package skilleval

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Genome is the "text-as-genome" view of a skill: the mutable optimization
// target (the body) is separated from the immutable identity (the frontmatter).
//
// The split is the core of phase 2 (GEPA): a GEPA loop may mutate Body freely,
// but it can never touch Identity. Applying the genome always remonte a
// SKILL.md whose frontmatter is derived solely from Identity name/description/
// level, so the skill's identity is preserved no matter how the body evolves.
//
// Genome is deterministic. ParseSkill and Apply are pure over their inputs; no
// randomness or non-deterministic context enters the genome values.
type Genome struct {
	// Identity is the frozen frontmatter (name/description/level) and the
	// original body snapshot. It is NEVER mutated by the evolution loop; only
	// the Body field below is the optimization target.
	Identity Skill
	// Body is the optimizable region — everything after the frontmatter. This
	// is the only field a Mutator is allowed to change.
	Body string
	// Source is the complete, original SKILL.md text (normalized to LF). It is
	// kept for semantic preservation and provenance, so a mutated body can be
	// diffed against the baseline without re-parsing the identity.
	Source string
	// Frontmatter is the raw YAML frontmatter block (between the `---`
	// delimiters), captured verbatim at parse time. It is preserved so Apply
	// can re-emit every frontmatter field already present in the source
	// (version, category, status, license, compatibility, metadata,
	// allowed-tools, ...) instead of dropping them — the legacy behavior only
	// re-marshalled name/description/level, silently destroying versioning
	// during evolution. Since the genome only ever mutates Body, the
	// frontmatter is re-emitted unchanged.
	Frontmatter string
}

// frontmatter is the YAML identity block parsed from the top of a SKILL.md.
// Only the identity-bearing keys are modeled; the rest of the block (if any)
// is not carried forward by Apply.
type frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Level       int    `yaml:"level"`
}

// ParseSkill reads a SKILL.md file and separates its frontmatter (identity)
// from its body. It errors when the file cannot be read, when it has no
// leading `---` frontmatter block, when the block cannot be parsed as YAML, or
// when the frontmatter is missing a non-empty `name`.
//
// The returned Genome holds the identity in Identity, the optimizable region
// in Body, and the raw source in Source.
func ParseSkill(path string) (*Genome, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("skilleval: read skill %s: %w", path, err)
	}

	raw := normalizeNewlines(strings.TrimPrefix(string(data), "\ufeff"))

	fm, fmRaw, body, err := splitFrontmatter(raw)
	if err != nil {
		return nil, fmt.Errorf("skilleval: %s: %w", path, err)
	}

	// The identity is a snapshot: the body captured here is the baseline, not
	// the mutation target. The genome tracks both so the baseline can be
	// preserved for semantic diffing while the body optimizes freely.
	g := &Genome{
		Identity: Skill{
			Name:        fm.Name,
			Path:        path,
			Description: fm.Description,
			Level:       fm.Level,
			Body:        body,
		},
		Body:         body,
		Source:       raw,
		Frontmatter:  fmRaw,
	}
	return g, nil
}

// normalizeNewlines converts CRLF and lone-CR line endings to LF so the genome
// is deterministic and platform-independent (a SKILL.md edited on Windows must
// parse identically to one edited on POSIX).
func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// splitFrontmatter splits a normalized SKILL.md source into its YAML
// frontmatter block and its body. It requires a leading `---` delimiter line
// and a matching closing `---` line, and returns errNoSkillName when the
// frontmatter has no non-empty `name`.
func splitFrontmatter(raw string) (frontmatter, string, string, error) {
	lines := strings.Split(raw, "\n")

	// A SKILL.md must open with a frontmatter delimiter; otherwise there is no
	// identity to preserve and the genome contract cannot be honored.
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return frontmatter{}, "", "", errNoFrontmatter
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return frontmatter{}, "", "", errNoFrontmatter
	}

	var fm frontmatter
	fmRaw := strings.Join(lines[1:end], "\n")
	if err := yaml.Unmarshal([]byte(fmRaw), &fm); err != nil {
		return frontmatter{}, "", "", fmt.Errorf("parse frontmatter: %w", err)
	}
	if strings.TrimSpace(fm.Name) == "" {
		return frontmatter{}, "", "", errNoSkillName
	}

	// The body is everything after the closing delimiter; the standard blank
	// line that separates frontmatter from body is not part of the body itself.
	body := strings.TrimLeft(strings.Join(lines[end+1:], "\n"), "\n")
	return fm, fmRaw, body, nil
}

// Apply remonte a SKILL.md string from the genome's identity + body.
//
// Invariants:
//   - The frontmatter is derived exclusively from Identity (name/description/
//     level), so the identity is always preserved even after a Mutator rewrote
//     Body.
//   - The body appended is exactly Body; the frontmatter is never re-inserted
//     into the body, and the body can never duplicate the frontmatter.
func (g *Genome) Apply() string {
	fm := strings.TrimSpace(g.Frontmatter)
	if fm == "" {
		// Fallback (e.g. a manually constructed Genome): render the identity
		// from name/description/level, matching the historical behavior.
		f := frontmatter{
			Name:        g.Identity.Name,
			Description: g.Identity.Description,
			Level:       g.Identity.Level,
		}
		fmYAML, err := yaml.Marshal(&f)
		if err != nil {
			fmYAML = []byte(fmt.Sprintf("name: %s\nlevel: %d\n", f.Name, f.Level))
		}
		fm = strings.TrimSpace(string(fmYAML))
	}
	return "---\n" + fm + "\n---\n" + g.Body
}
