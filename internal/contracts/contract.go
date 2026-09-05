// Package contracts implements the Cosca versioned RPC contract registry.
//
// Decision: ADR-7423 — versionamento de contratos por método {major, minor},
// adapted from the Traycer versioned-rpc framework (open-source, MIT).
//
// Invariants enforced by ValidateRegistry at load time (fail-fast, before
// any runtime uses the registry):
//
//  1. Structural integrity — every declared version exists, latestMinor is
//     the highest installed minor, and upgrade paths chain from the previous
//     installed version.
//  2. Golden rule — a minor bump must be purely additive: request/response
//     payloads must not remove or change existing fields.
//  3. A major bump must be genuinely breaking — a major that changes nothing
//     is rejected ("could have shipped as a minor").
//  4. Downgrades and degrades must be well-formed: downgrades target an older
//     major from the latest minor; non-floor methods declare a degrade
//     strategy (unsupported or fallback to a floor method).
package contracts

import (
	"fmt"
)

// SchemaVersion identifies a contract version within a method line.
// Minors within the same major must be additive; majors require a real
// breaking change (see ValidateRegistry).
type SchemaVersion struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
}

// String renders "major.minor".
func (v SchemaVersion) String() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

// RpcContract is the atomic unit of the registry: one method at one version
// with request/response schemas. Payloads are Go structs (no external schema
// language — additivity/breaking are checked structurally via reflection).
type RpcContract struct {
	Method        string
	SchemaVersion SchemaVersion
	Request       any // Go struct or type (nil = empty payload)
	Response      any // Go struct or type (nil = empty payload)
}

// UpgradePath upgrades a request/response from one version to the next.
// Upgrades chain: every version except the first must declare one that
// starts at the previous installed version.
type UpgradePath struct {
	From            SchemaVersion
	To              SchemaVersion
	UpgradeRequest  func(req any) (any, error)
	UpgradeResponse func(resp any) (any, error)
}

// DowngradePath converts from the latest minor of a major back to the
// latest minor of an older major. A downgrade may fail on purpose when the
// old semantics cannot represent the new payload (e.g. new required field).
type DowngradePath struct {
	From              SchemaVersion
	To                SchemaVersion
	DowngradeRequest  func(req any) (any, error)
	DowngradeResponse func(resp any) (any, error)
}

// DegradeStrategy is how a non-floor method behaves with an older peer that
// does not know it.
type DegradeStrategy string

const (
	// DegradeUnsupported: the method is simply unavailable on the old peer.
	DegradeUnsupported DegradeStrategy = "unsupported"
	// DegradeFallback: the method adapts its request/response to a floor
	// method via adapters.
	DegradeFallback DegradeStrategy = "fallback"
)

// FallbackDegrade declares the floor method and adapters used when falling
// back. AdaptRequest/AdaptResponse must be non-nil.
type FallbackDegrade struct {
	ToMethod      string        // name of the floor method being targeted
	ToVersion     SchemaVersion // version of the floor method being targeted
	AdaptRequest  func(req any) (any, error)
	AdaptResponse func(resp any) (any, error)
}

// Degrade describes how a non-floor method degrades.
type Degrade struct {
	Strategy DegradeStrategy
	Fallback *FallbackDegrade // set when Strategy == DegradeFallback
}

// VersionEntry is one installed version of a method within a major line.
type VersionEntry struct {
	Contract            RpcContract
	UpgradeFromPrevious *UpgradePath // nil for the first installed version
}

// MajorLine is all installed versions of a method within one major.
type MajorLine struct {
	LatestMinor              int
	Versions                 map[int]VersionEntry
	DowngradePathsFromLatest map[int]DowngradePath
}

// MethodRegistry is one RPC method across all its majors.
type MethodRegistry struct {
	MajorLines map[int]MajorLine
	Degrade    *Degrade // nil for floor methods
}

// Registry is the full versioned RPC registry, keyed by method name.
type Registry struct {
	Methods map[string]MethodRegistry
}

// NewRegistry creates an empty registry ready to be populated.
func NewRegistry() *Registry {
	return &Registry{Methods: map[string]MethodRegistry{}}
}
