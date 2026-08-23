//
// Package catalog implements the Cosca CATALOG GATE (generate-and-diff).
//
// The catalog gate is the deterministic, cross-platform engine behind
// `cosca gate catalog`. It exposes TWO orthogonal operations sharing the
// three catalog invariants plus the canonical snapshot contract:
//
//   Invariant A (index):       every catalog column is one of the four
//                              collections (agents/skills/engines/departments)
//                              under .opencode/cosca and MUST ship an INDEX.md.
//   Invariant B (frontmatter): every SKILL.md/PROMPT.md under .opencode/cosca
//                              MUST declare `name` (kebab-case, non-empty) and
//                              `description` (non-empty); `level`, when present,
//                              MUST be an integer 1-5.
//   Invariant C (cross-refs):  every relative link in any *.md under
//                              .opencode/cosca MUST point at an existing file
//                              (http/https/mailto/tel/ftp/data/#/anchor/schemes
//                              are ignored). This closes the dangling-ref gap.
//   Invariant D (mojibake):    every *.md under .opencode/cosca MUST be clean
//                              UTF-8. No double-encoded (mojibake) sequences
//                              from re-encoded em dashes/quotes/nbsp. A lone
//                              `â` (C3 A2) is NOT a finding — it is a legit
//                              PT char ("âmbito"); only the `â€` quote/dash
//                              chains are reported.
//
// CheckDrift  — generate-and-diff: `--generate` writes the canonical snapshot
// (catalog.manifest), the list of expected INDEX paths plus the canonical
// `name`/title of every column. `CheckDrift` diffs the live tree against that
// snapshot; any drift (new/removed column, renamed column) is a manifest-drift
// violation that tells the operator to re-run `--generate` and commit. This is
// the BLOCKING invariant: today it must PASS (drift=0) and enforce only NEW
// drift in the future.
//
// AuditInvariants — evaluates the three invariants (A/B/C) WITHOUT the drift
// diff. Findings are NON-BLOCKING debt: the CLI reports counts + examples and
// exits 0 (unless `--strict`). This is where the framework's accumulated
// catalog debt shows up as visible, actionable backlog instead of a pipeline
// blocker.
//
// Everything is deterministic: files are walked with filepath.Walk, results
// are sorted by path, and output is byte-identical across platforms. No exec,
// no fixed path inside internal/embed/cosca, no LLM. The root is always passed
// in explicitly (default ".opencode/cosca" resolved by the caller).
package catalog

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ManifestFileName is the canonical snapshot file written by --generate and
// diffed by --check. It lives at <root>/catalog.manifest.
const ManifestFileName = "catalog.manifest"

// ManifestVersion is the schema version of the snapshot format.
const ManifestVersion = 1

// Collections are the top-level cataloged directories under .opencode/cosca.
// They are the ONLY directories scanned for catalog columns; everything else
// in .opencode/cosca (memory, shared, scripts, workflows, ...) is bystander.
var Collections = []string{"agents", "skills", "engines", "departments"}

// WhitelistNames are directory names that hold scaffold/transient content and
// are NOT catalog columns. They are ignored when they appear as an immediate
// child of a collection. The names that also happen to be collections
// (skills, memory, knowledge, ...) refer to non-collection levels here.
var WhitelistNames = map[string]bool{
	".cosca-scaffold": true,
	"company":         true,
	"councils":        true,
	"plugins":         true,
	"runtime":         true,
	"scripts":         true,
	"sdk":             true,
	"templates":       true,
	"cli":             true,
	"bootstrap":       true,
	"capabilities":    true,
	"diagrams":        true,
	"metrics":         true,
	"knowledge":       true,
	"memory":          true,
	"shared":          true,
	"skills":          true,
}

// Violation kinds reported by the gate.
const (
	KindIndexMissing  = "index-missing"
	KindFrontmatter   = "frontmatter"
	KindDanglingLink  = "dangling-link"
	KindManifestDrift = "manifest-drift"
	KindMojibake      = "mojibake"
)

// Report modes.
const (
	// ModeCheck is the generate-and-diff DRIFT check (--check, default).
	ModeCheck = "check"
	// ModeAudit is the non-blocking invariant audit (--audit).
	ModeAudit = "audit"
)

// Violation is a single gate finding.
type Violation struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`   // file/path the finding refers to (POSIX, relative)
	Detail string `json:"detail"` // human-readable explanation of the problem
}

// Stats summarizes what the gate inspected.
type Stats struct {
	MdFiles   int  `json:"md_files"`   // *.md files traversed (not checked)
	Prompts   int  `json:"prompts"`    // SKILL.md/PROMPT.md files examined
	Columns   int  `json:"columns"`    // catalog columns discovered
	Links     int  `json:"links"`      // relative links resolved
	Manifest  bool `json:"manifest"`   // snapshot present?
	Mojibakes int  `json:"mojibake"`   // mojibake sequences detected
}

// Report is the outcome of a drift check or an invariant audit.
type Report struct {
	Root       string      `json:"root"`
	Mode       string      `json:"mode"`   // "check" (drift) or "audit" (invariants)
	Pass       bool        `json:"pass"`   // no findings
	Drift      bool        `json:"drift"`  // manifest mismatch present (check mode)
	Violations []Violation `json:"violations"`
	Stats      Stats       `json:"stats"`
}

// Column is a single catalog entry: an immediate child dir of a collection.
type Column struct {
	Collection string `json:"collection"` // agents|skills|engines|departments
	Name       string `json:"name"`       // directory name (kebab-case)
	IndexPath  string `json:"index"`      // expected INDEX.md path (POSIX, relative)
	Title      string `json:"title"`      // canonical name from frontmatter (name/agent), else dir
}

// key returns the stable identity of a column used for drift comparison.
func (c Column) key() string {
	return c.Collection + "/" + c.Name
}

// Manifest is the canonical snapshot contract.
type Manifest struct {
	Version int      `json:"version"`
	Entries []Column `json:"entries"` // sorted by key()
}

// ---- Invariant A/B core helpers -------------------------------------------

// collectColumns walks the immediate children of every collection and returns
// the catalog columns (whitelisted scaffold names are skipped). Deterministic:
// sorted by collection then name. It also resolves the canonical Title for
// each column from its SKILL.md/PROMPT.md frontmatter (name -> agent -> dirname).
func collectColumns(root string) ([]Column, error) {
	var cols []Column
	for _, coll := range Collections {
		dir := filepath.Join(root, coll)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read collection %q: %w", coll, err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if WhitelistNames[name] {
				continue
			}
			title := columnTitle(dir, name, coll)
			indexPath := toSlash(filepath.Join(coll, name, "INDEX.md"))
			cols = append(cols, Column{
				Collection: coll,
				Name:       name,
				IndexPath:  indexPath,
				Title:      title,
			})
		}
	}
	sort.Slice(cols, func(i, j int) bool {
		if cols[i].Collection != cols[j].Collection {
			return cols[i].Collection < cols[j].Collection
		}
		return cols[i].Name < cols[j].Name
	})
	return cols, nil
}

// columnTitle derives the canonical name for a column directory. It reads the
// frontmatter of SKILL.md (engines/departments) or PROMPT.md (agents) and falls
// back to the directory name.
func columnTitle(dir, name, coll string) string {
	candidates := []string{"SKILL.md", "PROMPT.md"}
	if coll == "agents" {
		candidates = []string{"PROMPT.md", "SKILL.md"}
	}
	for _, cand := range candidates {
		fm, ok := readFrontmatter(filepath.Join(dir, name, cand))
		if !ok {
			continue
		}
		if v := strings.TrimSpace(fm["name"]); v != "" {
			return v
		}
		if v := strings.TrimSpace(fm["agent"]); v != "" {
			return v
		}
	}
	return name
}

// readFrontmatter parses a YAML frontmatter block at the very top of a file.
// It returns a map of lower-cased key -> trimmed value and true when a valid
// `---` delimited block exists. All other files yield (nil, false).
func readFrontmatter(path string) (map[string]string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	if len(data) == 0 {
		return nil, false
	}
	s := string(data)
	// Strip a UTF-8 BOM (EF BB BF) that some editors prepend to the file.
	s = strings.TrimPrefix(s, "\ufeff")
	if !strings.HasPrefix(s, "---\n") && !strings.HasPrefix(s, "---\r\n") {
		return nil, false
	}
	start := 3
	// Find the closing delimiter on its own line.
	rest := s[start:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return nil, false
	}
	body := rest[:idx]
	// Strip a trailing \r if present (CRLF files).
	body = strings.TrimRight(body, "\r")

	var raw map[string]any
	if err := yaml.Unmarshal([]byte(body), &raw); err != nil {
		return nil, false
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		switch val := v.(type) {
		case string:
			out[strings.ToLower(k)] = val
		case int:
			out[strings.ToLower(k)] = fmt.Sprintf("%d", val)
		case float64:
			out[strings.ToLower(k)] = fmt.Sprintf("%v", val)
		case bool:
			out[strings.ToLower(k)] = fmt.Sprintf("%v", val)
		}
	}
	return out, true
}

// checkIndexPresent runs Invariant A: every live non-whitelisted catalog
// column must ship an INDEX.md.
func checkIndexPresent(root string, cols []Column, rep *Report) error {
	for _, c := range cols {
		indexAbs := filepath.Join(root, filepath.FromSlash(c.IndexPath))
		if fi, err := os.Stat(indexAbs); err != nil || fi.IsDir() {
			rep.Violations = append(rep.Violations, Violation{
				Kind:   KindIndexMissing,
				Path:   c.IndexPath,
				Detail: fmt.Sprintf("coluna %q sem INDEX.md", c.key()),
			})
		}
	}
	return nil
}

// diffManifest compares the live column set against the snapshot set and appends
// manifest-drift violations for new/removed/renamed columns. It returns true
// when any drift was detected.
func diffManifest(live, snapshot map[string]Column, rep *Report) bool {
	drift := false
	for key, liveCol := range live {
		exp, ok := snapshot[key]
		if !ok {
			drift = true
			rep.Violations = append(rep.Violations, Violation{
				Kind:   KindManifestDrift,
				Path:   liveCol.IndexPath,
				Detail: "coluna viva ausente do snapshot — rode `cosca gate catalog --generate` e commite",
			})
			continue
		}
		if exp.Title != liveCol.Title {
			drift = true
			rep.Violations = append(rep.Violations, Violation{
				Kind:   KindManifestDrift,
				Path:   liveCol.IndexPath,
				Detail: fmt.Sprintf("nome canônico mudou (%q -> %q)", exp.Title, liveCol.Title),
			})
		}
	}
	for key := range snapshot {
		if _, ok := live[key]; !ok {
			drift = true
			rep.Violations = append(rep.Violations, Violation{
				Kind:   KindManifestDrift,
				Path:   key + "/INDEX.md",
				Detail: "coluna do snapshot ausente na árvore viva — rode `cosca gate catalog --generate` e commite",
			})
		}
	}
	return drift
}

// columnSet indexes columns by their stable identity key.
func columnSet(cols []Column) map[string]Column {
	m := make(map[string]Column, len(cols))
	for _, c := range cols {
		m[c.key()] = c
	}
	return m
}

// checkFrontmatter runs Invariant B over every SKILL.md/PROMPT.md under root.
func checkFrontmatter(root string, rep *Report) error {
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // best-effort: skip unreadable entries
		}
		if d.IsDir() {
			return nil
		}
		base := filepath.Base(p)
		if base != "SKILL.md" && base != "PROMPT.md" {
			return nil
		}
		rep.Stats.Prompts++
		fm, ok := readFrontmatter(p)
		rel := toSlash(relPath(root, p))
		name := strings.TrimSpace(fm["name"])
		desc := strings.TrimSpace(fm["description"])
		level := strings.TrimSpace(fm["level"])

		if !ok {
			rep.Violations = append(rep.Violations, Violation{
				Kind:   KindFrontmatter,
				Path:   rel,
				Detail: "sem frontmatter YAML (`---` ... `---`): falta `name` e `description`",
			})
			return nil
		}
		if name == "" {
			rep.Violations = append(rep.Violations, Violation{
				Kind:   KindFrontmatter,
				Path:   rel,
				Detail: "frontmatter sem `name` (kebab-case não-vazio)",
			})
		} else if !kebabRe.MatchString(name) {
			rep.Violations = append(rep.Violations, Violation{
				Kind:   KindFrontmatter,
				Path:   rel,
				Detail: fmt.Sprintf("`name` não é kebab-case: %q", name),
			})
		}
		if desc == "" {
			rep.Violations = append(rep.Violations, Violation{
				Kind:   KindFrontmatter,
				Path:   rel,
				Detail: "frontmatter sem `description` (não-vazio)",
			})
		}
		if level != "" {
			n, verr := parseLevel(level)
			if verr != nil || n < 1 || n > 5 {
				rep.Violations = append(rep.Violations, Violation{
					Kind:   KindFrontmatter,
					Path:   rel,
					Detail: fmt.Sprintf("`level` fora de 1-5: %q", level),
				})
			}
		}
		return nil
	})
	return err
}

// checkCrossReferences runs Invariant C over every *.md under root: each
// relative link must resolve to an existing file or directory.
//
// FALSE-POSITIVE FILTERING: a relative link is intentionally NOT reported when
// it falls into one of the following documented categories (see
// falsePositiveLink / outOfTreeReference):
//
//  1. Links inside fenced code blocks (```) or inline code spans (`...`).
//     These are illustrative snippets (Go generics, template examples, rule
//     tables) — e.g. `[name](data T)` in a quick-start. They never point at a
//     real file on disk.
//  2. Placeholder/illustrative destinations used as examples in docs and
//     templates (../path, ../path/to/file.md, ../template-name/TEMPLATE.md,
//     img.png, path, ...). These intentionally reference a conceptual target.
//  3. Out-of-catalog references that CLIMB ABOVE the audit root (e.g.
//     ../../../.github/workflows/ci.yml, or links into internal/embed/cosca/...).
//     These target the monorepo, NOT the .opencode/cosca tree, and are valid
//     cross-tree references. The root is always passed in explicitly.
func checkCrossReferences(root string, rep *Report) error {
	return filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(p) != ".md" {
			return nil
		}
		rep.Stats.MdFiles++
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		rel := toSlash(relPath(root, p))
		dir := filepath.Dir(p)
		checkFileLinks(dir, rel, string(data), root, rep)
		return nil
	})
}

// checkFileLinks scans a single markdown file for relative links and reports
// those that dangle. It is line-aware so links inside fenced code blocks and
// inline code spans are skipped, and it applies the false-positive filters.
func checkFileLinks(dir, rel, content, root string, rep *Report) {
	inFence := false
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		// A fenced code block opens/closes with ``` (optionally followed by a
		// language tag). While inside a fence every link is illustrative.
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		// Strip inline code spans (backtick-delimited) so `[text](dest)` inside
		// a code span is ignored. The link must be real prose, not a snippet.
		stripped := inlineCodeRe.ReplaceAllString(rawLine, "")
		for _, m := range linkRe.FindAllString(stripped, -1) {
			dest := linkRe.ReplaceAllString(m, "$1")
			target, ok := cleanLinkDest(dest)
			if !ok {
				continue
			}
			// Filter documented false positives: placeholder examples and
			// cross-tree (out-of-root) references.
			if falsePositiveLink(target) || outOfTreeReference(dir, target, root) {
				continue
			}
			rep.Stats.Links++
			if !targetExists(dir, target) {
				rep.Violations = append(rep.Violations, Violation{
					Kind:   KindDanglingLink,
					Path:   rel,
					Detail: fmt.Sprintf("link sem alvo: %s -> %s", dest, target),
				})
			}
		}
	}
}

// falsePositiveLink reports whether a link destination is a known
// illustrative/placeholder reference. These are NARROW exact matches so a real
// broken link can never be hidden by this filter. They are authored in the
// docs/templates on purpose and point at a conceptual, non-existent target.
func falsePositiveLink(dest string) bool {
	switch strings.TrimSpace(dest) {
	case "../path",
		"../path/to/file.md",
		"../template-name/TEMPLATE.md",
		"img.png",
		"path",
		"data T":
		return true
	}
	return false
}

// outOfTreeReference reports whether a (cleaned) link destination resolves to a
// path that CLIMBS ABOVE the audit root. Such links target the monorepo (e.g.
// ../../../.github/workflows/ci.yml) or another tree (internal/embed/cosca/...)
// rather than the .opencode/cosca catalog, so they are valid cross-tree
// references and must NOT be reported as dangling.
func outOfTreeReference(dir, dest, root string) bool {
	absTarget := filepath.Clean(filepath.Join(dir, filepath.FromSlash(dest)))
	absRoot := filepath.Clean(root)
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return true
	}
	// The target is outside the root when the relative path climbs up (starts
	// with "..") or is a totally different absolute path.
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel)
}

// ---- Invariant D (mojibake) ------------------------------------------------

// mojibakePatterns are the known double-encoded UTF-8 sequences (mojibake) left
// behind when a UTF-8 document is re-read/mis-encoded once too many times. Each
// entry carries the corrupt BYTE pattern (the detector matches raw bytes, never
// the console/display), a short label of the corrupt token and the intended
// character it was meant to be. All patterns share the `â` (C3 A2) lead byte
// followed by a re-encoded quote/dash/ellipsis/nbsp — never a lone `â` (which is
// legitimate in Portuguese: "âmbito").
//
// Common prefix (the `â€` motif) decodes as:
//
//	C3 A2            = â  (U+00E2)
//	E2 82 AC         = €  (U+20AC)
//	E2 80 XX         = re-encoded punctuation/dash/ellipsis
//
// A variant replaces the euro (E2 82 AC) with `†` (E2 80 A0), and a 4-byte
// pattern covers the re-encoded non-breaking space.
var mojibakePatterns = []mojibakePattern{
	{bytes: []byte{0xc3, 0xa2, 0xe2, 0x82, 0xac, 0xe2, 0x80, 0x9d}, label: "â€\u201d", orig: "travessão —"},
	{bytes: []byte{0xc3, 0xa2, 0xe2, 0x82, 0xac, 0xe2, 0x80, 0x99}, label: "â€™", orig: "apóstrofo ’"},
	{bytes: []byte{0xc3, 0xa2, 0xe2, 0x82, 0xac, 0xe2, 0x80, 0x9c}, label: "â€œ", orig: "aspas “"},
	{bytes: []byte{0xc3, 0xa2, 0xe2, 0x82, 0xac, 0xe2, 0x80, 0x98}, label: "â€˜", orig: "aspas ‘"},
	{bytes: []byte{0xc3, 0xa2, 0xe2, 0x82, 0xac, 0xe2, 0x80, 0xa6}, label: "â€¦", orig: "reticências …"},
	{bytes: []byte{0xc3, 0xa2, 0xe2, 0x82, 0xac, 0xe2, 0x80, 0x93}, label: "â€“", orig: "travessão –"},
	{bytes: []byte{0xc3, 0xa2, 0xe2, 0x82, 0xac, 0xe2, 0x80, 0x94}, label: "â€”", orig: "travessão —"},
	// ê-variante: the re-encoded char after `â` is † (E2 80 A0) instead of €.
	{bytes: []byte{0xc3, 0xa2, 0xe2, 0x80, 0xa0, 0xe2, 0x80, 0x9d}, label: "â†\u201d", orig: "travessão —"},
	{bytes: []byte{0xc3, 0xa2, 0xe2, 0x80, 0xa0, 0xe2, 0x80, 0x99}, label: "â†™", orig: "apóstrofo ’"},
	{bytes: []byte{0xc3, 0xa2, 0xe2, 0x80, 0xa0, 0xe2, 0x80, 0x9c}, label: "â†œ", orig: "aspas “"},
	{bytes: []byte{0xc3, 0xa2, 0xe2, 0x80, 0xa0, 0xe2, 0x80, 0x98}, label: "â†˜", orig: "aspas ‘"},
	{bytes: []byte{0xc3, 0xa2, 0xe2, 0x80, 0xa0, 0xe2, 0x80, 0xa6}, label: "â†¦", orig: "reticências …"},
	{bytes: []byte{0xc3, 0xa2, 0xe2, 0x80, 0xa0, 0xe2, 0x80, 0x93}, label: "â†“", orig: "travessão –"},
	{bytes: []byte{0xc3, 0xa2, 0xe2, 0x80, 0xa0, 0xe2, 0x80, 0x94}, label: "â†”", orig: "travessão —"},
	// Re-encoded non-breaking space.
	{bytes: []byte{0xc3, 0xa2, 0xc2, 0xa0}, label: "â\u00a0", orig: "espaço não quebrável (nbsp)"},
}

// mojibakePattern is a known corrupt byte sequence plus its human labels.
type mojibakePattern struct {
	bytes []byte
	label string // the visible corrupt token (for the report detail)
	orig  string // the intended character the bytes were meant to be
}

// checkMojibake runs Invariant D over every *.md under root: it reads the raw
// BYTES and reports one `mojibake` violation per occurrence of a known
// double-encoded sequence. It is deliberately byte-based (never console-decoded)
// so it is deterministic and cross-platform. A lone C3 A2 ("â") is NOT a
// finding — only the recognized `â€`/`Â ` chains are flagged.
func checkMojibake(root string, rep *Report) error {
	return filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // best-effort: skip unreadable entries
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(p) != ".md" {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		rel := toSlash(relPath(root, p))
		scanMojibake(data, rel, rep)
		return nil
	})
}

// scanMojibake scans data for every known mojibake byte pattern, appending a
// `mojibake` violation per occurrence and incrementing Stats.Mojibakes. The
// offset is the absolute byte index of the sequence start (deterministic,
// cross-platform; output never depends on a console codepage).
func scanMojibake(data []byte, rel string, rep *Report) {
	for i := 0; i < len(data); {
		matched := false
		for _, pat := range mojibakePatterns {
			if bytes.HasPrefix(data[i:], pat.bytes) {
				rep.Stats.Mojibakes++
				rep.Violations = append(rep.Violations, Violation{
					Kind:   KindMojibake,
					Path:   rel,
					Detail: fmt.Sprintf("sequência mojibake %s (bytes %x) no byte %d — era %s",
						pat.label, pat.bytes, i, pat.orig),
				})
				// Skip past the whole sequence so overlapping re-scans are
				// impossible and counts stay exact.
				i += len(pat.bytes)
				matched = true
				break
			}
		}
		if !matched {
			i++
		}
	}
}

// ---- Cross-reference link resolution --------------------------------------

var (
	// linkRe matches [text](destination). The separator empty-char class
	// matches the destination up to the closing paren.
	linkRe = regexp.MustCompile(`\[[^\]]*\]\(([^)]*)\)`)
	// inlineCodeRe matches a single backtick-delimited inline code span. Links
	// inside inline code (e.g. `[name](data T)`) are illustrative snippets and
	// must be skipped by the link resolver.
	inlineCodeRe = regexp.MustCompile("`[^`]*`")
	// kebabRe validates kebab-case identifiers (name frontmatter).
	kebabRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

// cleanLinkDest normalizes a markdown link destination. It returns the relative
// path to check and whether the link should be validated (false => ignore).
func cleanLinkDest(raw string) (string, bool) {
	d := strings.TrimSpace(raw)
	if d == "" {
		return "", false
	}
	// Unwrap <...>.
	if strings.HasPrefix(d, "<") && strings.HasSuffix(d, ">") && len(d) > 2 {
		d = d[1 : len(d)-1]
	}
	// Strip markdown title and fragments.
	if idx := strings.IndexByte(d, '"'); idx >= 0 {
		d = strings.TrimRight(d[:idx], " \t")
	} else if idx := strings.IndexByte(d, '\''); idx >= 0 {
		d = strings.TrimRight(d[:idx], " \t")
	}
	if idx := strings.IndexAny(d, "#?"); idx >= 0 {
		d = d[:idx]
	}
	d = strings.TrimSpace(d)
	if d == "" {
		return "", false
	}

	// External schemes / anchors / protocol-relative are ignored.
	lower := strings.ToLower(d)
	for _, scheme := range []string{"http://", "https://", "mailto:", "tel:", "ftp://",
		"sftp://", "data:", "file:", "ed2k://", "irc://", "ws://", "wss://"} {
		if strings.HasPrefix(lower, scheme) {
			return "", false
		}
	}
	if strings.HasPrefix(d, "//") || strings.HasPrefix(d, "#") {
		return "", false
	}
	// Root-absolute links are ambiguous in a docs tree; treat as external.
	if strings.HasPrefix(d, "/") {
		return "", false
	}
	return d, true
}

// targetExists reports whether a (relative) link destination resolves to an
// existing file or directory relative to dir. A fallback tries appending ".md"
// for extension-less paths (common for [text](page) -> page.md).
func targetExists(dir, dest string) bool {
	base := filepath.Dir(dest)
	if dest == "." || dest == ".." || base == "." {
		t := filepath.Clean(filepath.Join(dir, filepath.FromSlash(dest)))
		if _, err := os.Stat(t); err == nil {
			return true
		}
	}
	t := filepath.Clean(filepath.Join(dir, filepath.FromSlash(dest)))
	if fi, err := os.Stat(t); err == nil && (fi.IsDir() || fi.Mode().IsRegular()) {
		return true
	}
	// Attempt .md fallback when the path has no extension.
	if filepath.Ext(t) == "" {
		if _, err := os.Stat(t + ".md"); err == nil {
			return true
		}
	}
	return false
}

// ---- Manifest read / render -----------------------------------------------

// LoadManifest reads and parses the snapshot at root/catalog.manifest.
// It returns (nil, nil) when the snapshot does not exist.
func LoadManifest(root string) (*Manifest, error) {
	path := filepath.Join(root, ManifestFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	m := &Manifest{}
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		switch fields[0] {
		case "version":
			fmt.Sscanf(fields[1], "%d", &m.Version)
		case "index":
			indexPath := fields[1]
			coll, name := splitIndexPath(indexPath)
			if coll == "" || name == "" {
				continue
			}
			m.Entries = append(m.Entries, Column{
				Collection: coll,
				Name:       name,
				IndexPath:  indexPath,
				Title:      strings.Join(fields[2:], " "),
			})
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan manifest: %w", err)
	}
	sortEntries(m.Entries)
	return m, nil
}

// renderManifest returns the deterministic text of the snapshot.
func renderManifest(m *Manifest) string {
	sortEntries(m.Entries)
	var b strings.Builder
	fmt.Fprintf(&b, "# Cosca Catalog Manifest v%d\n", ManifestVersion)
	b.WriteString("# Generated by `cosca gate catalog --generate`. DO NOT EDIT by hand.\n")
	b.WriteString("# Contract for `cosca gate catalog --check` (generate-and-diff).\n")
	fmt.Fprintf(&b, "# Fields: TARGET PATH TITLE  (one line per directory column)\n")
	fmt.Fprintf(&b, "version %d\n", ManifestVersion)
	for _, e := range m.Entries {
		fmt.Fprintf(&b, "index %s %s\n", e.IndexPath, e.Title)
	}
	return b.String()
}

// BuildManifest constructs the snapshot Manifest from the live tree (used by
// --generate). Deterministic: columns are sorted by key().
func BuildManifest(root string) (*Manifest, error) {
	cols, err := collectColumns(root)
	if err != nil {
		return nil, err
	}
	return &Manifest{Version: ManifestVersion, Entries: cols}, nil
}

// ---- Public API ------------------------------------------------------------

// CheckDrift diffs the live catalog tree against the committed snapshot
// (catalog.manifest) and reports ONLY manifest drift (new/removed column or a
// changed canonical name). It does NOT evaluate invariants A/B/C. Pass is true
// when the live tree matches the snapshot. When no snapshot exists the check
// reports a single drift violation directing the operator to run --generate.
func CheckDrift(root string) (*Report, error) {
	rep := &Report{Root: root, Mode: ModeCheck, Violations: []Violation{}}
	cols, err := collectColumns(root)
	if err != nil {
		return nil, err
	}
	rep.Stats.Columns = len(cols)

	manifest, err := LoadManifest(root)
	if err != nil {
		return nil, err
	}
	rep.Stats.Manifest = manifest != nil

	if manifest == nil {
		rep.Drift = true
		rep.Violations = append(rep.Violations, Violation{
			Kind:   KindManifestDrift,
			Path:   ManifestFileName,
			Detail: "snapshot canônico ausente — rode `cosca gate catalog --generate` e commite",
		})
		sortViolations(rep.Violations)
		rep.Pass = false
		return rep, nil
	}

	rep.Drift = diffManifest(columnSet(cols), columnSet(manifest.Entries), rep)
	sortViolations(rep.Violations)
	rep.Pass = !rep.Drift
	return rep, nil
}

// AuditInvariants evaluates the four catalog invariants (INDEX presence,
// frontmatter, cross-references, mojibake) WITHOUT the manifest drift diff.
// Findings are code-smell / debt candidates. Pass is true when there are zero
// findings. The CLI decides whether findings are blocking (--strict) or merely
// advisory (the default). The manifest snapshot is NOT required by this
// operation.
func AuditInvariants(root string) (*Report, error) {
	rep := &Report{Root: root, Mode: ModeAudit, Violations: []Violation{}}
	cols, err := collectColumns(root)
	if err != nil {
		return nil, err
	}
	rep.Stats.Columns = len(cols)

	// Invariant A: INDEX presence.
	if err := checkIndexPresent(root, cols, rep); err != nil {
		return nil, err
	}
	// Invariant B: frontmatter.
	if err := checkFrontmatter(root, rep); err != nil {
		return nil, err
	}
	// Invariant C: cross-references.
	if err := checkCrossReferences(root, rep); err != nil {
		return nil, err
	}
	// Invariant D: mojibake / double-encoded UTF-8.
	if err := checkMojibake(root, rep); err != nil {
		return nil, err
	}

	sortViolations(rep.Violations)
	rep.Pass = len(rep.Violations) == 0
	return rep, nil
}

// Generate builds the canonical snapshot text for the given root. It never
// writes to disk — the caller (CLI) decides where to persist it, so the core
// stays pure and testable.
func Generate(root string) (string, error) {
	m, err := BuildManifest(root)
	if err != nil {
		return "", err
	}
	return renderManifest(m), nil
}

// ---- small helpers --------------------------------------------------------

func splitIndexPath(indexPath string) (coll, name string) {
	parts := strings.Split(filepath.ToSlash(indexPath), "/")
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

func sortEntries(entries []Column) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Collection != entries[j].Collection {
			return entries[i].Collection < entries[j].Collection
		}
		return entries[i].Name < entries[j].Name
	})
}

// sortViolations orders findings deterministically by kind, path, then detail.
func sortViolations(v []Violation) {
	sort.SliceStable(v, func(i, j int) bool {
		a, b := v[i], v[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Detail < b.Detail
	})
}

func parseLevel(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func toSlash(p string) string {
	return filepath.ToSlash(p)
}

// relPath returns the POSIX-relative path of p under root.
func relPath(root, p string) string {
	r, err := filepath.Rel(root, p)
	if err != nil {
		return toSlash(p)
	}
	return toSlash(r)
}
