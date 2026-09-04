package authority

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// ─── Detector de drift FROZEN ↔ LIVE (SPEC §2) ───────────────────────────────
//
// Compara as duas árvores por caminho relativo canônico (sempre "/", nunca o
// separador nativo do SO — o bug histórico do filepath.Rel no Windows não se
// repete aqui) + SHA-256. O detector é apenas-leitura, idempotente e
// NÃO-destrutivo: não escolhe vencedor; apenas classifica e orienta. Durante
// qualquer drift o FROZEN CONTINUA AUTORIDADE (SPEC §1/§2.3).

// fileMeta is the minimal content fingerprint of a single file for one zone.
type fileMeta struct {
	Path string    // canonical relative path ("/")
	SHA  string    // sha256 of content
	Mtim time.Time // modification time (zero if unavailable)
}

// fileMetas walks a root and returns the map keyed by the canonical relative
// path (lower-cased canonical key) for every regular file. Deterministic and
// read-only. Unreadable entries are skipped (best-effort), consistent with the
// choose to never fail on a single bad file.
func fileMetas(root string) (map[string]fileMeta, error) {
	out := make(map[string]fileMeta)
	// Raiz inexistente (ex.: FROZEN só embutido, sem árvore em disco) DEVE ser
	// reportada como erro os.IsNotExist — senão o RunDrift assume "FROZEN vazio"
	// e marca Comparable=true errado.
	if _, err := os.Stat(root); err != nil {
		return nil, err
	}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // best-effort: skip unreadable entries
		}
		if d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		data, derr := os.ReadFile(p)
		if derr != nil {
			return nil
		}
		out[relKey(rel)] = fileMeta{
			Path: filepath.ToSlash(rel),
			SHA:  sha256Bytes(data),
			Mtim: info.ModTime(),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DriftEntry is the per-path classification (SPEC §2.4).
type DriftEntry struct {
	Path      string `json:"path"`                // canonical relative path ("/")
	State     State  `json:"state"`               // MATCH|LIVE_NEWER|FROZEN_NEWER|ONLY_FROZEN|ONLY_LIVE
	FrozenSHA string `json:"frozen_sha,omitempty"` // sha256 of the FROZEN side, or "" when ONLY_LIVE
	LiveSHA   string `json:"live_sha,omitempty"`   // sha256 of the LIVE side, or "" when ONLY_FROZEN
	FrozenM   string `json:"frozen_m,omitempty"`   // RFC3339 mtime of the FROZEN side, or ""
	LiveM     string `json:"live_m,omitempty"`     // RFC3339 mtime of the LIVE side, or ""
	Action    string `json:"action"`               // none|report|preserve|propose_promotion (suggestion only)
}

// DriftReport aggregates the drift per state plus totals (SPEC §2.2/§2.4). It
// NEVER resolves the drift silently — it only classifies and orients.
type DriftReport struct {
	FrozenRoot string            `json:"frozen_root"`
	LiveRoot   string            `json:"live_root"`
	Comparable bool              `json:"comparable"` // FROZEN tree exists on disk?
	Entries    []DriftEntry      `json:"entries"`
	Totals     map[string]int    `json:"totals"` // state → count
}

// DriftCount is the number of paths that require attention (divergent or
// only-live), i.e. everything that is not MATCH and not ONLY_FROZEN.
func (r *DriftReport) DriftCount() int {
	n := 0
	for _, e := range r.Entries {
		if e.State == StateLiveNewer || e.State == StateFrozenNewer || e.State == StateOnlyLive {
			n++
		}
	}
	return n
}

// HasDrift reports whether any path diverges between FROZEN and LIVE.
func (r *DriftReport) HasDrift() bool { return r.DriftCount() > 0 }

// actionFor maps a drift state to the detector's guidance action (SPEC §2.4).
func actionFor(s State) string {
	switch s {
	case StateMatch:
		return "none"
	case StateFrozenNewer:
		return "report"
	case StateLiveNewer:
		return "propose_promotion" // sinaliza "candidato a promoção" (§4)
	case StateOnlyFrozen:
		return "preserve" // G5
	case StateOnlyLive:
		return "report" // drift latente — evidência de evolução a promover
	default:
		return "report"
	}
}

// classifyDivergent decides LIVE_NEWER vs FROZEN_NEWER when hashes differ,
// using the LIVE vs FROZEN mtime relation (SPEC §2.2). The tie-break — equal or
// unknown mtime — falls to FROZEN_NEWER, because during any drift the FROZEN
// remains authority and there is no matching explicit proposal request. This is
// deterministic and never "ties" silently.
func classifyDivergent(live, frozen fileMeta) State {
	if live.Mtim.After(frozen.Mtim) {
		return StateLiveNewer
	}
	return StateFrozenNewer
}

// RunDrift compares FROZEN↔LIVE by canonical path + SHA-256. Deterministic,
// idempotent, read-only. It never resolves silently and never picks a winner
// based on "most recent" — it classifies (FROZEN stays authority). When the
// FROZEN tree is not present on disk (e.g. a third-party project whose
// framework lives only inside the binary), every LIVE file is reported as
// ONLY_LIVE and Comparable is false.
func RunDrift(frozenRoot, liveRoot string) (*DriftReport, error) {
	rep := &DriftReport{FrozenRoot: frozenRoot, LiveRoot: liveRoot, Entries: []DriftEntry{}, Totals: map[string]int{}}

	live, err := fileMetas(liveRoot)
	if err != nil {
		return nil, fmt.Errorf("authority: walk live %q: %w", liveRoot, err)
	}
	frozen, err := fileMetas(frozenRoot)
	if err != nil {
		if os.IsNotExist(err) || !dirExists(frozenRoot) {
			// FROZEN embutido no binário (não há árvore em disco).
			rep.Comparable = false
			for _, fm := range live {
				rep.Entries = append(rep.Entries, DriftEntry{
					Path: fm.Path, State: StateOnlyLive, LiveSHA: fm.SHA, LiveM: fmtTime(fm.Mtim),
					Action: actionFor(StateOnlyLive),
				})
				rep.Totals[string(StateOnlyLive)]++
			}
			sortDriftEntries(rep.Entries)
			return rep, nil
		}
		return nil, fmt.Errorf("authority: walk frozen %q: %w", frozenRoot, err)
	}
	rep.Comparable = true

	// Union of canonical paths.
	all := make(map[string]struct{}, len(live)+len(frozen))
	for k := range live {
		all[k] = struct{}{}
	}
	for k := range frozen {
		all[k] = struct{}{}
	}

	// Sort keys for deterministic output.
	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		lf, okLive := live[k]
		ff, okFrozen := frozen[k]

		var e DriftEntry
		switch {
		case okLive && okFrozen && lf.SHA == ff.SHA:
			e = DriftEntry{Path: lf.Path, State: StateMatch, FrozenSHA: ff.SHA, LiveSHA: lf.SHA,
				FrozenM: fmtTime(ff.Mtim), LiveM: fmtTime(lf.Mtim), Action: actionFor(StateMatch)}
		case okLive && okFrozen:
			e = DriftEntry{Path: lf.Path, State: classifyDivergent(lf, ff), FrozenSHA: ff.SHA, LiveSHA: lf.SHA,
				FrozenM: fmtTime(ff.Mtim), LiveM: fmtTime(lf.Mtim), Action: actionFor(classifyDivergent(lf, ff))}
		case okFrozen:
			e = DriftEntry{Path: ff.Path, State: StateOnlyFrozen, FrozenSHA: ff.SHA, FrozenM: fmtTime(ff.Mtim),
				LiveSHA: "", Action: actionFor(StateOnlyFrozen)}
		case okLive:
			e = DriftEntry{Path: lf.Path, State: StateOnlyLive, LiveSHA: lf.SHA, LiveM: fmtTime(lf.Mtim),
				FrozenSHA: "", Action: actionFor(StateOnlyLive)}
		}
		rep.Entries = append(rep.Entries, e)
		rep.Totals[string(e.State)]++
	}

	return rep, nil
}

// sortDriftEntries orders findings deterministically by path.
func sortDriftEntries(entries []DriftEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Path != entries[j].Path {
			return entries[i].Path < entries[j].Path
		}
		return entries[i].State < entries[j].State
	})
}

// dirExists reports whether a path exists and is a directory.
func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
