// Package authority implements the Cosca FROZEN→LIVE authority-reconciliation
// machine, as prescribed by docs/specs/frozen-live-authority-contract.md.
//
// The authority contract is: FROZEN (internal/embed/cosca) > LIVE
// (.opencode/cosca) > RUNTIME (.cosca). FROZEN is the single semantic
// authority; LIVE is the OpenCode surface (edit-able, never removed); RUNTIME
// is derived and regenerable.
//
// The machine is deliberately DETERMINISTIC and ZERO-LLM: every decision is a
// SHA-256 hash plus canonical-path comparison. It is NON-DESTRUCTIVE by
// default: FROZEN is never implicitly overwritten, LIVE diverging content is
// preserved as evidence, and ONLY_FROZEN exclusives are never deleted. The only
// sanctioned write into FROZEN is an explicit, verifiable promotion (SPEC §4).
//
// The four pieces of the contract map to the files of this package:
//
//   - drift.go      detector de drift FROZEN↔LIVE (§2)  → RunDrift/DriftReport
//   - reconcile.go  reconciliação FROZEN→LIVE (§3)       → Plan/ApplyReconcile
//   - promote.go    promoção LIVE→FROZEN (§4)            → NewPromotionProposal/Promote
//   - guard.go      guarda de autoridade (§5)            → GuardWrite/GuardDelete/GuardError
//   - integrity.go  manifestos/rollback (§6)             → Manifest/TreeRevision/Rollback
//
// Dependencies: stdlib + internal/proposal (reused for the zone guard, i.e.
// ProtectedZoneOf). No cobra, no network, no new external dependency.
package authority

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ─── Zonas e estados (SPEC §1 / §2.2) ────────────────────────────────────────

// State is the drift classification of a single path against the FROZEN↔LIVE
// comparison (SPEC §2.2).
type State string

const (
	// StateMatch: file exists in both zones; identical hash (in sync).
	StateMatch State = "MATCH"
	// StateLiveNewer: exists in both, hash differs, LIVE content is newer.
	StateLiveNewer State = "LIVE_NEWER"
	// StateFrozenNewer: exists in both, hash differs, FROZEN content is newer
	// (or no matching proposal).
	StateFrozenNewer State = "FROZEN_NEWER"
	// StateOnlyFrozen: exists only in FROZEN (blocks/, merkle/, chain.dat,
	// protocols). Preserve (G5); never deleted; not copied to LIVE by default.
	StateOnlyFrozen State = "ONLY_FROZEN"
	// StateOnlyLive: exists only in LIVE (un-promoted evolution). Record as
	// drift/evidence; not a new authority.
	StateOnlyLive State = "ONLY_LIVE"
)

// Zone is one of the authority zones (SPEC §1).
type Zone string

// Zone delimiters of the authority contract.
const (
	ZoneFrozen  Zone = "FROZEN"
	ZoneLive    Zone = "LIVE"
	ZoneRuntime Zone = "RUNTIME"
)

// Op is the kind of filesystem operation the authority guard inspects.
type Op string

const (
	OpWrite  Op = "write"
	OpDelete Op = "delete"
	OpMkdir  Op = "mkdir"
)

// ─── Helpers: canonical path + hashing ───────────────────────────────────────

// canonicalRel normalises a relative path to the canonical "/"-separated form
// used as the comparison key. It resolves "." / ".." via filepath.Clean and
// converts any OS separator to "/". This is the anti-regression guard for the
// historical filepath.Rel Windows bug (SPEC §2.1 / §8): FROZEN and LIVE are
// compared by canonical "/" paths, never by OS-native separators.
func canonicalRel(p string) string {
	if p == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(p))
}

// relKey returns the lower-cased canonical key used for map lookups, so the
// same tree is identified identically on any OS (case-insensitive comparison,
// matching the proposal package's canonicalRmPath stance).
func relKey(p string) string {
	return strings.ToLower(canonicalRel(p))
}

// withinRoot reports whether the canonical target t is equal to or under root
// r as a full path segment (no string-prefix false positives: ".cosca-backup"
// is NOT protected by ".cosca"). It accepts relative and absolute paths.
func withinRoot(r, t string) bool {
	root := strings.TrimSuffix(canonicalRel(r), "/")
	target := canonicalRel(t)
	if root == "" || target == "" {
		return false
	}
	if target == root {
		return true
	}
	if strings.HasPrefix(target, root+"/") { // t under root (root is ancestor)
		return true
	}
	if strings.HasSuffix(target, "/"+root) { // root appears at the end (absolute path)
		return true
	}
	return strings.Contains(target, "/"+root+"/") // root as a segment in the middle
}

// ensureWithinRoot errors if target is not equal to or under root, defending
// against path traversal (a proposed promotion path that escapes the FROZEN
// tree). Deterministic and read-only.
func ensureWithinRoot(root, target string) error {
	if !withinRoot(root, target) {
		return fmt.Errorf("authority: target %q escapes root %q (fail-closed)", target, root)
	}
	return nil
}

// sha256Bytes returns the hex SHA-256 of arbitrary bytes.
func sha256Bytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// sha256File returns the hex SHA-256 of a file's content.
func sha256File(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return sha256Bytes(data), nil
}

// fmtTime renders a time as RFC3339, or "" for the zero value — used to keep
// deterministic, JSON-friendly mtime fields (SPEC §2.4).
func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// sortStrings sorts a string slice in place (deterministic ordering).
func sortStrings(s []string) { sort.Strings(s) }

// stringsTrim trims surrounding whitespace (compact alias for readability).
func stringsTrim(s string) string { return strings.TrimSpace(s) }
