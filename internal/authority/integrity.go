package authority

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ─── Integridade (SPEC §6) ───────────────────────────────────────────────────
//
// Cada zona (FROZEN/LIVE/RUNTIME) expõe um manifesto de caminhos + SHA-256 e um
// id de revisão (hash raiz da árvore) que permite detectar "mudou desde a base"
// e ancorar o rollback determinístico.

// ManifestFileName is the canonical name for a zone manifest under its root.
const ManifestFileName = "manifest.json"

// ManifestEntry is a single path→hash pair in a zone manifest.
type ManifestEntry struct {
	Path string `json:"path"` // canonical relative path ("/")
	SHA  string `json:"sha"`  // sha256 of content
}

// Manifest is the zone inventory used as the authority anchor (SPEC §6.1).
type Manifest struct {
	Zone     string          `json:"zone"`     // FROZEN|LIVE|RUNTIME
	Root     string          `json:"root"`
	Revision string          `json:"revision"` // tree hash (root link of the tree)
	Entries  []ManifestEntry `json:"entries"`
}

// BuildManifest walks root and produces the zone manifest (path→sha, sorted
// deterministically by canonical path). The zone is supplied by the caller
// because a TempDir test root is not literally a repo layout path.
func BuildManifest(root, zone string) (*Manifest, error) {
	metas, err := fileMetas(root)
	if err != nil {
		return nil, err
	}
	m := &Manifest{Zone: zone, Root: root, Entries: []ManifestEntry{}}
	keys := make([]string, 0, len(metas))
	for k := range metas {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fm := metas[k]
		m.Entries = append(m.Entries, ManifestEntry{Path: fm.Path, SHA: fm.SHA})
	}
	m.Revision = revisionOf(m.Entries)
	return m, nil
}

// revisionOf computes the tree revision hash: sha256 over the sorted
// "path:SHA\n" lines. Deterministic and cross-platform (paths already "/").
func revisionOf(entries []ManifestEntry) string {
	sorted := make([]ManifestEntry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	var b strings.Builder
	for _, e := range sorted {
		b.WriteString(e.Path)
		b.WriteByte(':')
		b.WriteString(e.SHA)
		b.WriteByte('\n')
	}
	return sha256Bytes([]byte(b.String()))
}

// TreeRevision computes the tree revision of a root directly from disk
// (SPEC §6.1: an id that detects "changed since the base"). Equivalent to
// BuildManifest(root, "").Revision.
func TreeRevision(root string) (string, error) {
	metas, err := fileMetas(root)
	if err != nil {
		return "", err
	}
	keys := make([]string, 0, len(metas))
	for k := range metas {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	entries := make([]ManifestEntry, 0, len(keys))
	for _, k := range keys {
		if metas[k].Path == ManifestFileName {
			continue // manifest.json é metadata do snapshot, não conteúdo
		}
		entries = append(entries, ManifestEntry{Path: metas[k].Path, SHA: metas[k].SHA})
	}
	return revisionOf(entries), nil
}

// WriteManifest serialises a manifest to the destination path (JSON).
func WriteManifest(m *Manifest, dest string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("authority: marshal manifest: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return fmt.Errorf("authority: mkdir manifest dir: %w", err)
	}
	if err := os.WriteFile(dest, data, 0o600); err != nil {
		return fmt.Errorf("authority: write manifest: %w", err)
	}
	return nil
}

// ReadManifest reads a zone manifest from a path.
func ReadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("authority: read manifest: %w", err)
	}
	m := &Manifest{}
	if err := json.Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("authority: unmarshal manifest: %w", err)
	}
	return m, nil
}

// SnapshotTree copies a whole tree to destRoot and writes a manifest.json with
// the exact inventory (path→sha + tree revision). It is the "before/after"
// snapshot of any write (SPEC §6.3). The destRoot MUST NOT be inside srcRoot.
// Returns the manifest of the snapshot.
func SnapshotTree(srcRoot, destRoot string) (*Manifest, error) {
	if withinRoot(srcRoot, filepath.Join(destRoot, "manifest.json")) {
		return nil, fmt.Errorf("authority: snapshot dest %q is inside source %q (would recurse)", destRoot, srcRoot)
	}
	if err := copyDirTree(srcRoot, destRoot); err != nil {
		return nil, err
	}
	m, err := BuildManifest(destRoot, zoneName(srcRoot))
	if err != nil {
		return nil, err
	}
	if err := WriteManifest(m, filepath.Join(destRoot, ManifestFileName)); err != nil {
		return nil, err
	}
	return m, nil
}

// zoneName plausibly names a zone from its root for manifest provenance. It
// prefers the authority mapping and falls back to a friendly label.
func zoneName(root string) string {
	if z := proposalZone(root); z != "" {
		return z
	}
	return "UNKNOWN"
}

// proposalZone maps a root to a zone via the protected-zone mapping.
func proposalZone(root string) string {
	return string(ZoneOf(root))
}

// copyDirTree copies every file entry under src to dest (creating dirs). It
// refuses to copy into a dest that is inside src (prevents self-recursion).
func copyDirTree(src, dest string) error {
	if withinRoot(src, filepath.Clean(dest)) || dest == src {
		return fmt.Errorf("authority: copy target %q is inside source %q", dest, src)
	}
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(src, p)
		if rerr != nil {
			return rerr
		}
		tgt := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(tgt, 0o700)
		}
		data, derr := os.ReadFile(p)
		if derr != nil {
			return derr
		}
		return os.WriteFile(tgt, data, 0o700)
	})
}

// ─── Rollback determinístico (SPEC §6.3) ─────────────────────────────────────

// RollbackOptions configures the deterministic rollback.
type RollbackOptions struct {
	// DestroyExtras removes files present in root but absent from the snapshot,
	// restoring the EXACT prior state (needed for T10: "estado volta ao hash de
	// snapshot_before"). Default false: rollback is non-destructive (upsert-only).
	DestroyExtras bool
}

// RollbackResult is the FACT of a deterministic rollback.
type RollbackResult struct {
	Root             string `json:"root"`
	SnapshotRoot     string `json:"snapshot_root"`
	Restored         int    `json:"restored"`          // files (re)written from the snapshot
	Removed          int    `json:"removed"`           // extras removed (only when DestroyExtras)
	FinalRevision    string `json:"final_revision"`
	SnapshotRevision string `json:"snapshot_revision"`
	Exact            bool   `json:"exact"` // final revision == snapshot revision
}

// Rollback restores root to the exact state captured in snapshotRoot. It reads
// the snapshot manifest (manifest.json) when present, otherwise walks the
// snapshot tree. Deterministic: for every snapshot file it upserts the snapshot
// content; with DestroyExtras it additionally removes files that are not part
// of the snapshot (the one explicit, authorized destructive step).
func Rollback(root, snapshotRoot string, o RollbackOptions) (*RollbackResult, error) {
	res := &RollbackResult{Root: root, SnapshotRoot: snapshotRoot}

	// Resolve the snapshot inventory (manifest preferred).
	var snapshotEntries map[string]ManifestEntry
	if m, err := ReadManifest(filepath.Join(snapshotRoot, ManifestFileName)); err == nil && m != nil {
		res.SnapshotRevision = m.Revision
		snapshotEntries = make(map[string]ManifestEntry, len(m.Entries))
		for _, e := range m.Entries {
			snapshotEntries[e.Path] = e
		}
	} else {
		metas, werr := fileMetas(snapshotRoot)
		if werr != nil {
			return nil, fmt.Errorf("authority: rollback: read snapshot inventory: %w", werr)
		}
		snapshotEntries = make(map[string]ManifestEntry, len(metas))
		for _, fm := range metas {
			snapshotEntries[fm.Path] = ManifestEntry{Path: fm.Path, SHA: fm.SHA}
		}
		res.SnapshotRevision = revisionOf(manifestEntriesOf(snapshotEntries))
	}

	// Upsert every snapshot file (recreate directories).
	for _, e := range snapshotEntries {
		src := filepath.Join(snapshotRoot, filepath.FromSlash(e.Path))
		data, err := os.ReadFile(src)
		if err != nil {
			// Fall back to manifest-only: content may not be copied. Skip.
			continue
		}
		tgt := filepath.Join(root, filepath.FromSlash(e.Path))
		if err := os.MkdirAll(filepath.Dir(tgt), 0o700); err != nil {
			return nil, err
		}
		if err := os.WriteFile(tgt, data, 0o700); err != nil {
			return nil, err
		}
		res.Restored++
	}

	// Optionally remove extras (exact restore, T10).
	if o.DestroyExtras {
		curMetas, cerr := fileMetas(root)
		if cerr != nil {
			return nil, cerr
		}
		keep := make(map[string]bool, len(snapshotEntries))
		for k := range snapshotEntries {
			keep[strings.ToLower(k)] = true
			// also match canonical key variants
		}
		for _, fm := range curMetas {
			if keep[strings.ToLower(fm.Path)] {
				continue
			}
			// Never remove the snapshot dir itself if it happens to be under root.
			if withinRoot(snapshotRoot, root) {
				continue
			}
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(fm.Path))); err != nil {
				return nil, err
			}
			res.Removed++
		}
	}

	res.FinalRevision, _ = TreeRevision(root)
	res.Exact = res.SnapshotRevision != "" && res.FinalRevision == res.SnapshotRevision
	return res, nil
}

// manifestEntriesOf converts a map[path]ManifestEntry into a sorted slice.
func manifestEntriesOf(m map[string]ManifestEntry) []ManifestEntry {
	out := make([]ManifestEntry, 0, len(m))
	for _, e := range m {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// timestampName is a lexicographically-ordered unique snapshot dir name.
func timestampName() string {
	return time.Now().UTC().Format("20060102-150405.000000000")
}
