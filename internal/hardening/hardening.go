// Package hardening hardens the Cosca process at startup, adapted from OpenAI's
// Codex CLI process hardening (Apache-2.0).
//
// PreMain disables core dumps, disables ptrace attach, prevents privilege
// escalation via exec, and strips environment variables that could inject
// libraries (LD_PRELOAD, LD_LIBRARY_PATH, DYLD_*). All operations are
// best-effort and never fail: on unsupported platforms or when the kernel
// refuses a syscall, the process simply continues unmodified.
package hardening

import (
	"os"
	"strings"
)

// isDangerousEnvVar reports whether an environment variable can be used to
// inject libraries or hijack dynamic loading. LD_PRELOAD and LD_LIBRARY_PATH
// affect ELF loaders on Linux; every DYLD_* variable affects the dyld loader
// on macOS.
func isDangerousEnvVar(key string) bool {
	switch key {
	case "LD_PRELOAD", "LD_LIBRARY_PATH":
		return true
	}
	return strings.HasPrefix(key, "DYLD_")
}

// RemoveDangerousEnv returns a filtered copy of environ (a list of
// "KEY=VALUE" strings as returned by os.Environ) with dangerous variables
// removed. The input slice is never modified.
func RemoveDangerousEnv(environ []string) []string {
	out := make([]string, 0, len(environ))
	for _, entry := range environ {
		key := entry
		if idx := strings.IndexByte(entry, '='); idx >= 0 {
			key = entry[:idx]
		}
		if isDangerousEnvVar(key) {
			continue
		}
		out = append(out, entry)
	}
	return out
}

// PreMain hardens the current process: it applies platform-specific hardening
// (core dump disable, ptrace disable, no-new-privileges) and unsets dangerous
// environment variables from the process environment. It is safe to call at
// the very top of main(). Best-effort — never fails.
func PreMain() {
	applyPlatformHardening()
	for _, entry := range os.Environ() {
		key := entry
		if idx := strings.IndexByte(entry, '='); idx >= 0 {
			key = entry[:idx]
		}
		if isDangerousEnvVar(key) {
			_ = os.Unsetenv(key)
		}
	}
}
