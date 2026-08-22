//go:build !linux && !darwin

// Package hardening hardens the Cosca process at startup.
package hardening

// applyPlatformHardening is a no-op on platforms without the hardening
// syscalls. PreMain remains safe and effective (env var stripping) everywhere.
func applyPlatformHardening() {}
