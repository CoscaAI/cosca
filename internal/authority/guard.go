package authority

import (
	"fmt"

	"github.com/CoscaAI/cosca/internal/proposal"
)

// GuardError is a FAIL-CLOSED authority violation. It is returned by the
// authority guard (§5), which is the only camada that decides whether a
// filesystem write/delete is permitted against a protected zone.
type GuardError struct {
	Zone   Zone
	Target string
	Op     Op
	Reason string
}

// Error renders a deterministic fail-closed message. Wherever a GuardError is
// produced, the operation MUST NOT write anything (SPEC §5.3: FAIL-CLOSED +
// DRIFT; FROZEN prevails).
func (e *GuardError) Error() string {
	reason := e.Reason
	if reason == "" {
		reason = "nenhuma rotina pode sobrescrever o FROZEN implicitamente; a promoção explícita (§4) é o único caminho legítimo"
	}
	return fmt.Sprintf("FAIL-CLOSED: %s bloqueado na zona de autoridade %s (%s) — %s",
		e.Op, e.Zone, e.Target, reason)
}

// GuardOptions carries the actual zone roots a given operation runs against.
// They allow the guard to be correct BOTH for the real repo layout (matched by
// proposal.ProtectedZoneOf) and for hermetic TempDir tests (matched by the
// explicit roots, since a TempDir is not literally "internal/embed/cosca").
type GuardOptions struct {
	// FrozenRoot is the actual FROZEN root tree on disk.
	FrozenRoot string
	// LiveRoot is the actual LIVE root tree on disk.
	LiveRoot string
	// RuntimeRoot is an optional RUNTIME root; protected from delete.
	RuntimeRoot string
	// AllowFrozenWrite is true ONLY for the explicit promotion flow (§4).
	// Any other routine (e.g. reconciliation) MUST keep it false.
	AllowFrozenWrite bool
}

// ZoneOf returns the authority zone of a target using proposal.ProtectedZoneOf
// (the canonical mapping for the real repo layout), or "" when no protected
// zone matches.
func ZoneOf(target string) Zone {
	return Zone(proposal.ProtectedZoneOf(target))
}

// guardZone resolves the zone applying BOTH the repo mapping
// (proposal.ProtectedZoneOf) and the explicit roots — so a write into the
// actual FROZEN tree (even a TempDir) is detected.
func guardZone(target string, o GuardOptions) Zone {
	if z := proposal.ProtectedZoneOf(target); z != "" {
		return Zone(z)
	}
	if withinRoot(o.FrozenRoot, target) {
		return ZoneFrozen
	}
	if withinRoot(o.LiveRoot, target) {
		return ZoneLive
	}
	if o.RuntimeRoot != "" && withinRoot(o.RuntimeRoot, target) {
		return ZoneRuntime
	}
	return ""
}

// GuardWrite fail-closes an implicit write into FROZEN. Writing anywhere in the
// FROZEN zone is allowed ONLY when o.AllowFrozenWrite is true (the explicit
// promotion flow). Writes into LIVE/RUNTIME and into free (non-protected) paths
// are permitted.
func GuardWrite(target string, o GuardOptions) error {
	zone := guardZone(target, o)
	if zone == ZoneFrozen && !o.AllowFrozenWrite {
		return &GuardError{Zone: zone, Target: target, Op: OpWrite,
			Reason: "escrita implícita no FROZEN — exigida a promoção explícita (§4) com base-revision e re-read; fail-closed"}
	}
	return nil
}

// GuardMkdir fail-closes creation of a directory inside FROZEN when it is not
// the sanctioned promotion flow.
func GuardMkdir(target string, o GuardOptions) error {
	zone := guardZone(target, o)
	if zone == ZoneFrozen && !o.AllowFrozenWrite {
		return &GuardError{Zone: zone, Target: target, Op: OpMkdir,
			Reason: "criação implícita de diretório no FROZEN — precisaria de promoção explícita (§4); fail-closed"}
	}
	return nil
}

// GuardDelete fail-closes ANY destructive operation on a protected zone
// (FROZEN/LIVE/RUNTIME). It mirrors cli.GuardedRemoveAll's policy without
// importing the CLI (keeps the dependency surface to stdlib + internal/proposal).
// In particular FROZEN exclusives (blocks/, merkle/, chain.dat, protocols) are
// NEVER deleted (G5).
func GuardDelete(target string, o GuardOptions) error {
	zone := guardZone(target, o)
	if zone == "" {
		return nil
	}
	return &GuardError{Zone: zone, Target: target, Op: OpDelete,
		Reason: "o contrato de autoridade proíbe deletar/conteúdo-destruir alvos protegidos (G2/G5)"}
}
