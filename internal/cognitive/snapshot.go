// Package cognitive implements the Cognitive Snapshot / Cognitive Version (CV)
// ledger for the Cosca.
//
// The CV is a hash ledger over the compound behavioral state of the Cosca —
// documentation + memory + rules + structure + model + history. It does NOT
// store file blobs: the manifest at .cosca/cv/CV-XXXX.json is an immutable
// verification ledger (component → SHA-256) used to compare, diagnose and
// guide a rollback. Following the Don's rule, a rollback is always
// "diagnóstico + guia, não reescrita automática": we never overwrite files
// without comparing first.
package cognitive

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Version prefix and zero-padding for CV identifiers (CV-0003).
const (
	versionPrefix = "CV-"
	versionWidth  = 4
	schemaVersion = 1
)

// CVDir is the directory (relative to the project .cosca/) that stores the
// immutable CV manifests.
const CVDir = "cv"

// Component is a named group of cognitive state files.
type Component struct {
	Name  string   // component identifier (e.g. "kernel-docs")
	Paths []string // paths relative to the project root (files or directories)
}

// DefaultComponents is the canonical set of cognitive components tracked by
// the CV ledger.
var DefaultComponents = []Component{
	{
		Name: "kernel-docs",
		Paths: []string{
			".cosca/framework/KERNEL.md",
			".cosca/framework/CONSTITUTION.md",
			".cosca/framework/Cognitive_State_Specification.md",
			".cosca/framework/shared",
		},
	},
	{Name: "knowledge", Paths: []string{".cosca/framework/knowledge"}},
	{Name: "laws", Paths: []string{".cosca/knowledge/laws.json"}},
	{Name: "memory", Paths: []string{".cosca/memory"}},
	{Name: "agents", Paths: []string{".cosca/framework/agents"}},
	{Name: "skills", Paths: []string{".cosca/framework/skills"}},
	{Name: "config", Paths: []string{".cosca/config.yaml"}},
}

// Snapshot is one Cognitive Version: an immutable manifest recording the
// SHA-256 hash of each cognitive component at creation time.
type Snapshot struct {
	ID            string                     `json:"id"`
	Version       string                     `json:"version"`
	CreatedAt     time.Time                  `json:"created_at"`
	SchemaVersion int                        `json:"schema_version"`
	Hashes        map[string]string          `json:"hashes"` // component → aggregate SHA-256
	FilesAffected int                        `json:"files_affected"`
	ManifestPath  string                     `json:"manifest_path,omitempty"`
	Components    map[string]ComponentRecord `json:"components"`
}

// ComponentRecord is the per-component detail of a snapshot: the aggregate
// hash and the per-file hashes (relative path → SHA-256).
type ComponentRecord struct {
	Hash  string            `json:"hash"`
	Files map[string]string `json:"files"`
}

// VerifyReport describes the result of verifying the current state against a
// stored snapshot.
type VerifyReport struct {
	Version    string                     `json:"version"`
	Matches    bool                       `json:"matches"`
	Components map[string]ComponentStatus `json:"components"`
}

// ComponentStatus reports per-component verification state.
type ComponentStatus struct {
	Match   bool     `json:"match"`
	Changed []string `json:"changed"` // files whose hash differs from the snapshot
	Removed []string `json:"removed"` // files present at snapshot time, gone now
	Added   []string `json:"added"`   // files present now, absent at snapshot time
	Files   int      `json:"files"`   // current file count
}

// RollbackGuide is the diagnostic + instruction output of a rollback. It is a
// guide, never an automatic rewrite: the manifest only stores hashes, so the
// rollback relies on git revert / manual restore using the ledger.
type RollbackGuide struct {
	Version     string
	Report      *VerifyReport
	Snapshot    *Snapshot // marker CV recorded before the rollback guidance (best-effort)
	Summary     string
	GitCommands []string
	ManualSteps []string
}

// RollbackError carries the RollbackGuide out of RollbackSnapshot. Rollback is
// "diagnóstico + guia, não reescrita automática" — the function never rewrites
// files and always surfaces the guide this way.
type RollbackError struct {
	Guide *RollbackGuide
}

func (e *RollbackError) Error() string {
	if e == nil || e.Guide == nil {
		return "rollback: no guide available"
	}
	return fmt.Sprintf("rollback %s: %s", e.Guide.Version, e.Guide.Summary)
}

// CreateSnapshot walks the cognitive components under rootDir, computes the
// SHA-256 of every tracked file, groups them by component and persists an
// immutable manifest at .cosca/cv/CV-XXXX.json. The version auto-increments
// from the highest existing CV. The version claim is atomic (O_CREATE|O_EXCL),
// so concurrent calls never collide.
func CreateSnapshot(rootDir string) (*Snapshot, error) {
	coscaDir := filepath.Join(rootDir, ".cosca")
	if info, err := os.Stat(coscaDir); err != nil {
		return nil, fmt.Errorf("cognitive: no .cosca directory at %q: %w", coscaDir, err)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("cognitive: %q is not a directory", coscaDir)
	}

	// Hash the current state first; only then claim a version.
	componentFiles := map[string]map[string]string{}
	hashes := map[string]string{}
	filesAffected := 0
	for _, comp := range DefaultComponents {
		files, err := hashComponent(rootDir, comp)
		if err != nil {
			return nil, err
		}
		if files == nil {
			files = map[string]string{}
		}
		componentFiles[comp.Name] = files
		hashes[comp.Name] = componentHash(files)
		filesAffected += len(files)
	}

	version, f, err := claimVersion(rootDir)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	snap := &Snapshot{
		ID:            version,
		Version:       version,
		CreatedAt:     time.Now().UTC(),
		SchemaVersion: schemaVersion,
		Hashes:        hashes,
		FilesAffected: filesAffected,
		ManifestPath:  manifestPath(rootDir, version),
		Components:    map[string]ComponentRecord{},
	}
	for _, comp := range DefaultComponents {
		snap.Components[comp.Name] = ComponentRecord{
			Hash:  hashes[comp.Name],
			Files: componentFiles[comp.Name],
		}
	}

	if err := writeManifest(f, snap); err != nil {
		return nil, fmt.Errorf("cognitive: write manifest %s: %w", snap.ManifestPath, err)
	}
	return snap, nil
}

// ListSnapshots returns all stored Cognitive Versions, sorted ascending.
// A missing or empty .cosca/cv directory yields an empty list, not an error.
func ListSnapshots(rootDir string) ([]Snapshot, error) {
	cvDir := filepath.Join(rootDir, ".cosca", CVDir)
	entries, err := os.ReadDir(cvDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cognitive: read %q: %w", cvDir, err)
	}

	var snaps []Snapshot
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(cvDir, e.Name())
		snap, err := LoadSnapshot(path)
		if err != nil {
			return nil, err
		}
		snaps = append(snaps, *snap)
	}
	sort.Slice(snaps, func(i, j int) bool { return snaps[i].Version < snaps[j].Version })
	return snaps, nil
}

// LoadSnapshot reads and parses a snapshot manifest from disk.
func LoadSnapshot(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cognitive: read manifest %q: %w", path, err)
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("cognitive: parse manifest %q: %w", path, err)
	}
	snap.ManifestPath = path
	return &snap, nil
}

// VerifySnapshot re-hashes the current cognitive state and compares it to the
// stored manifest for version. It reports exactly which components (and which
// files) changed, were removed or were added.
func VerifySnapshot(rootDir, version string) (bool, *VerifyReport, error) {
	ver, err := NormalizeVersion(version)
	if err != nil {
		return false, nil, err
	}
	snap, err := LoadSnapshot(manifestPath(rootDir, ver))
	if err != nil {
		return false, nil, err
	}

	current := map[string]map[string]string{}
	for _, comp := range DefaultComponents {
		files, err := hashComponent(rootDir, comp)
		if err != nil {
			return false, nil, err
		}
		current[comp.Name] = files
	}

	report := &VerifyReport{
		Version:    ver,
		Matches:    true,
		Components: map[string]ComponentStatus{},
	}
	for compName, stored := range snap.Components {
		cur := current[compName]
		status := ComponentStatus{
			Changed: nil,
			Removed: nil,
			Added:   nil,
			Files:   len(cur),
			Match:   true,
		}
		for rel, want := range stored.Files {
			got, ok := cur[rel]
			if !ok {
				status.Removed = append(status.Removed, rel)
				status.Match = false
				continue
			}
			if got != want {
				status.Changed = append(status.Changed, rel)
				status.Match = false
			}
		}
		for rel := range cur {
			if _, ok := stored.Files[rel]; !ok {
				status.Added = append(status.Added, rel)
				status.Match = false
			}
		}
		sort.Strings(status.Changed)
		sort.Strings(status.Removed)
		sort.Strings(status.Added)
		if !status.Match {
			report.Matches = false
		}
		report.Components[compName] = status
	}
	return report.Matches, report, nil
}

// RollbackSnapshot is the Don-sanctioned rollback entrypoint: it NEVER rewrites
// files. It verifies the current state against the target version, records the
// current (possibly regressed) state as a NEW marker snapshot, and returns a
// *RollbackError carrying the diagnosis + git/manual restore guide.
func RollbackSnapshot(rootDir, version string) error {
	ver, err := NormalizeVersion(version)
	if err != nil {
		return err
	}

	matches, report, err := VerifySnapshot(rootDir, ver)
	if err != nil {
		return err
	}

	guide := &RollbackGuide{
		Version: ver,
		Report:  report,
	}

	// Safe marker: freeze the current state so the experiment can always be
	// revisited. Best-effort — a failure must not block the guidance.
	if marker, cerr := CreateSnapshot(rootDir); cerr != nil {
		guide.Summary = fmt.Sprintf(
			"diagnóstico concluído para %s; aviso: falha ao registrar o estado atual como novo snapshot (%v)",
			ver, cerr)
	} else {
		guide.Snapshot = marker
	}

	if matches {
		guide.Summary = fmt.Sprintf(
			"%s já confere com o estado atual — nada a reverter. Rollback é diagnóstico + guia, não reescrita automática.",
			ver)
	} else {
		guide.Summary = fmt.Sprintf(
			"%s NÃO confere com o estado atual — veja o diff abaixo. Rollback é diagnóstico + guia, não reescrita automática.",
			ver)
	}

	guide.GitCommands = rollbackGitCommands(report, guide.Snapshot)
	guide.ManualSteps = rollbackManualSteps(report)
	return &RollbackError{Guide: guide}
}

// NormalizeVersion accepts "CV-0003", "CV-3", "0003", "cv-0003" and returns
// the canonical zero-padded form "CV-0003".
func NormalizeVersion(version string) (string, error) {
	digits := strings.TrimPrefix(strings.TrimSpace(strings.ToUpper(version)), versionPrefix)
	n, err := strconv.Atoi(digits)
	if err != nil || n < 0 {
		return "", fmt.Errorf("cognitive: invalid CV version %q (esperava CV-0003)", version)
	}
	return fmt.Sprintf("%s%0*d", versionPrefix, versionWidth, n), nil
}

// ---------------------------------------------------------------------------
// Internals
// ---------------------------------------------------------------------------

// hashComponent computes the SHA-256 of every regular file under the
// component's paths. Missing paths are skipped (resilience for partial
// installs). Keys are project-root-relative paths, stable across machines.
func hashComponent(rootDir string, comp Component) (map[string]string, error) {
	files := map[string]string{}
	for _, relPath := range comp.Paths {
		full := filepath.Join(rootDir, filepath.FromSlash(relPath))
		info, err := os.Lstat(full)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("cognitive: stat %q: %w", full, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue // never follow symlinks inside the ledger
		}
		if !info.IsDir() {
			key, err := filepath.Rel(rootDir, full)
			if err != nil {
				return nil, err
			}
			h, err := fileSHA256(full)
			if err != nil {
				return nil, err
			}
			files[key] = h
			continue
		}
		err = filepath.WalkDir(full, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			key, err := filepath.Rel(rootDir, path)
			if err != nil {
				return err
			}
			h, err := fileSHA256(path)
			if err != nil {
				return err
			}
			files[key] = h
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("cognitive: walk %q: %w", full, err)
		}
	}
	return files, nil
}

// componentHash is a deterministic aggregate SHA-256 over the sorted
// "relpath:hash" entries of a component.
func componentHash(files map[string]string) string {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	h := sha256.New()
	for _, p := range paths {
		io.WriteString(h, p)
		io.WriteString(h, ":")
		io.WriteString(h, files[p])
		io.WriteString(h, "\n")
	}
	return hex.EncodeToString(h.Sum(nil))
}

// fileSHA256 streams a file through SHA-256.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("cognitive: open %q: %w", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("cognitive: hash %q: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// claimVersion atomically claims the next CV version by creating its manifest
// file with O_CREATE|O_EXCL. On collision (a concurrent snapshot took the
// version), it retries with the next one. The returned file is open for
// writing and is the caller's responsibility to close.
func claimVersion(rootDir string) (string, *os.File, error) {
	cvDir := filepath.Join(rootDir, ".cosca", CVDir)
	if err := os.MkdirAll(cvDir, 0o755); err != nil {
		return "", nil, fmt.Errorf("cognitive: create %q: %w", cvDir, err)
	}
	for i := 0; i < 1_000_000; i++ {
		ver, err := nextVersion(rootDir)
		if err != nil {
			return "", nil, err
		}
		path := manifestPath(rootDir, ver)
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			return ver, f, nil
		}
		if !os.IsExist(err) {
			return "", nil, fmt.Errorf("cognitive: claim %s: %w", path, err)
		}
		// Another snapshot claimed this version concurrently — retry.
	}
	return "", nil, fmt.Errorf("cognitive: could not claim a unique CV version after many attempts")
}

// nextVersion scans .cosca/cv/CV-*.json and returns the next sequential
// version (CV-0001 when none exists).
func nextVersion(rootDir string) (string, error) {
	cvDir := filepath.Join(rootDir, ".cosca", CVDir)
	entries, err := os.ReadDir(cvDir)
	if err != nil {
		return "", fmt.Errorf("cognitive: read %q: %w", cvDir, err)
	}
	max := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, versionPrefix) || !strings.HasSuffix(name, ".json") {
			continue
		}
		digits := strings.TrimSuffix(strings.TrimPrefix(name, versionPrefix), ".json")
		n, convErr := strconv.Atoi(digits)
		if convErr != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return fmt.Sprintf("%s%0*d", versionPrefix, versionWidth, max+1), nil
}

// manifestPath returns the manifest location for a normalized version.
func manifestPath(rootDir, version string) string {
	return filepath.Join(rootDir, ".cosca", CVDir, version+".json")
}

// writeManifest marshals the snapshot to the already-claimed file.
func writeManifest(f *os.File, snap *Snapshot) error {
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if _, err := f.Write(data); err != nil {
		return err
	}
	return f.Sync()
}

// rollbackGitCommands builds the git guidance for restoring the cognitive
// state of a target CV. Pure instructions — nothing is executed.
func rollbackGitCommands(report *VerifyReport, marker *Snapshot) []string {
	cmds := []string{
		"git status --short -- .cosca/",
		"git log --oneline -15 -- .cosca/",
		"git diff --stat HEAD -- .cosca/",
	}

	changed := map[string]bool{}
	for _, st := range report.Components {
		for _, rel := range append(append([]string{}, st.Changed...), st.Removed...) {
			changed[rel] = true
		}
	}
	paths := make([]string, 0, len(changed))
	for rel := range changed {
		paths = append(paths, filepath.ToSlash(rel))
	}
	sort.Strings(paths)

	// Group the affected paths by their git root so a single revert covers them.
	if len(paths) > 0 {
		cmds = append(cmds,
			"# escolha o commit da evolução (git log acima) e reverta os arquivos cognitivos:",
			"git revert <commit>",
			fmt.Sprintf("git checkout <commit>^ -- %s", strings.Join(paths, " ")),
			"git commit -m \"chore(cv): rollback do estado cognitivo para "+report.Version+"\"",
		)
	} else {
		cmds = append(cmds, "# nenhuma divergência de arquivos: sem comandos git de restauração necessários.")
	}

	if marker != nil {
		cmds = append(cmds,
			fmt.Sprintf("# estado atual congelado em %s antes do rollback — referência para comparar.",
				marker.Version))
	}
	return cmds
}

// rollbackManualSteps lists the manual restore steps for each affected file.
func rollbackManualSteps(report *VerifyReport) []string {
	var steps []string
	for _, comp := range DefaultComponents {
		st, ok := report.Components[comp.Name]
		if !ok || st.Match {
			continue
		}
		steps = append(steps, fmt.Sprintf("COMPONENTE %s:", comp.Name))
		for _, rel := range st.Changed {
			steps = append(steps, fmt.Sprintf("  mudou:      %s (compare antes de restaurar)", filepath.ToSlash(rel)))
		}
		for _, rel := range st.Removed {
			steps = append(steps, fmt.Sprintf("  removido:   %s (restaurar via git checkout <commit> -- <arquivo>)", filepath.ToSlash(rel)))
		}
		for _, rel := range st.Added {
			steps = append(steps, fmt.Sprintf("  adicionado: %s (verificar se deve ser mantido/removido)", filepath.ToSlash(rel)))
		}
	}
	return steps
}
