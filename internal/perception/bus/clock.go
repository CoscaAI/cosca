// Package bus implements the Perception Bus — the multimodal (vision + audio)
// synchronisation layer of the COSCA runtime.
//
// The bus is the "shared NTP" of perception: a single monotonic clock domain
// onto which every Observation (vision or audio) is stamped, a temporal ring
// buffer that remembers what was seen/heard over the last few seconds, and an
// overlap matcher that answers "what was the agent looking at when it heard
// X?".
//
// Design principles:
//   - Additive + opt-in: the bus rides on top of the existing Perception Loop
//     (internal/perception) and never changes its contract. It is created only
//     when perception.audio.enabled is true.
//   - A single monotonic clock: every observation is stamped with a
//     process-relative MonotonicTime from the same source, so vision and audio
//     timestamps are directly comparable/interpolable without wall-clock NTP
//     drift.
//   - Degrade gracefully, never crash: a missing source, a nil payload, or a
//     failed pipeline yields a Degraded WorldState with Warnings, never a panic.
package bus

import (
	"sync"
	"time"
)

// ──────────────────────────────────────────────────────────────
// Monotonic time
// ──────────────────────────────────────────────────────────────

// MonotonicTime is a process-relative monotonic timestamp in nanoseconds,
// measured from the process's start (the first call to Now). It is comparable
// and subtractable, and — critically — it does NOT follow wall-clock jumps
// (NTP slews, manual clock changes). That is what lets vision and audio
// observations be synchronised with a stable, drift-free reference.
//
// Zero is a valid "not yet set" sentinel (treated as "unset").
type MonotonicTime int64

var (
	monoOnce  sync.Once
	monoEpoch time.Time
)

// Now returns the current process-relative monotonic time in nanoseconds.
//
// Design decision (clock route): the plan offered two options —
//
//  1. `//go:linkname runtime.nanotime` — the true OS monotonic nanosecond, but
//     it is an unsafe linker hook (no type/signature contract, breaks across GO
//     versions, and is disallowed under some build modes / race detectors).
//  2. Go's own embedded monotonic clock — capture `epoch := time.Now()` once
//     and take `time.Since(epoch)`. `time.Since` uses the monotonic reading
//     carried by `time.Time` (on platforms that support it), so it is
//     monotonic, drift-free, allocation-free, and syscall-free.
//
// We chose the SECOND route: it is idiomatic, safe, portable, and requires no
// unsafe linker tricks. The only trade-off is that the epoch is process-start
// rather than OS-boot, which is exactly what a syncing bus needs (all sources
// are in-process). The nanosecond resolution still gives sub-microsecond
// comparability.
//
// Note: the clock is measured from the FIRST call to Now() (lazy epoch), so a
// short-lived bus occupies a tiny slice near t=0. Callers that need an
// absolute anchor should call Now() once at startup.
func Now() MonotonicTime {
	monoOnce.Do(func() { monoEpoch = time.Now() })
	return MonotonicTime(time.Since(monoEpoch).Nanoseconds())
}

// Add returns t+d (moving forward by a duration).
func (t MonotonicTime) Add(d time.Duration) MonotonicTime { return t + MonotonicTime(d) }

// Sub returns t-o as a duration. Negative when o is in the future of t.
func (t MonotonicTime) Sub(o MonotonicTime) time.Duration { return time.Duration(t - o) }

// Since returns the elapsed duration from t to Now().
func (t MonotonicTime) Since() time.Duration { return time.Duration(Now() - t) }

// Duration converts a MonotonicTime back into a time.Duration (useful when a
// MonotonicTime is used to represent an interval length rather than a point).
func (t MonotonicTime) Duration() time.Duration { return time.Duration(t) }

// ToMonotonic converts a time.Duration (interval length) into its MonotonicTime
// representation (nanoseconds). Both are int64 nanoseconds, so it is a plain
// cast; kept explicit for readability at call sites.
func ToMonotonic(d time.Duration) MonotonicTime { return MonotonicTime(d) }
