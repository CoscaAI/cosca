//
// Home directory resolution helpers for the Cosca platform.
//
// The core problem this solves: when `cosca` runs via sudo, the effective
// HOME becomes /root even though the real user is someone else. Every path
// derived from the home (paths.home, db.path, data/cache/logs, crypto salt)
// would then point at /root — breaking the user's config for the lifetime
// of the project.
//
// Resolution order:
//   1. SUDO_UID set → lookup the REAL user's home via os/user
//   2. $HOME explicitly set
//   3. Effective user (os/user.Current)
//   4. homedir.Dir() fallback

package config

import (
	"os"
	"os/user"

	"github.com/mitchellh/go-homedir"
)

// UserHomeDir returns the home directory of the user that actually invoked
// the process. When running under sudo (SUDO_UID present), it resolves the
// home of the original user instead of root's.
func UserHomeDir() string {
	// 1. Via sudo: resolve the real user's home from SUDO_UID.
	if sudoUID := os.Getenv("SUDO_UID"); sudoUID != "" {
		if u, err := user.LookupId(sudoUID); err == nil && u.HomeDir != "" {
			return u.HomeDir
		}
	}

	// 2. $HOME explicitly set — unless it is a sudo-polluted /root on a
	//    non-root process (e.g. su / inherited env without SUDO_UID).
	if home := os.Getenv("HOME"); home != "" && !(home == "/root" && isNonRoot()) {
		return home
	}

	// 3. Effective user.
	if u, err := user.Current(); err == nil && u.HomeDir != "" {
		return u.HomeDir
	}

	// 4. Last resort fallback.
	hd, err := homedir.Dir()
	if err != nil {
		return "."
	}
	return hd
}

// isNonRoot reports whether the effective user is not uid 0.
func isNonRoot() bool {
	return os.Geteuid() != 0
}
