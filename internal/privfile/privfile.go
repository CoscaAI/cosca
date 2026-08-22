// Package privfile guarantees restrictive file permissions for sensitive
// SQLite databases (secrets, tokens, audit). SQLite creates files with the
// process umask (typically 0644 — world readable); these helpers force 0600
// (owner read/write only) on creation and repair legacy files found with
// weaker permissions.
package privfile

import (
	"fmt"
	"os"
)

// EnsurePrivateDBFile guarantees the file at path is owner-only (0600).
// - If the file does not exist, it is created with mode 0600.
// - If it exists with weaker permissions (e.g. 0644 from a legacy umask),
//   the permissions are tightened to 0600.
// - If it exists with stricter/equal permissions, it is left untouched.
//
// Must be called BEFORE opening the SQLite database (SQLite would otherwise
// create the file using the process umask).
func EnsurePrivateDBFile(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("create private db file %s: %w", path, err)
	}
	// Close before chmod is unnecessary — the fd stays valid. We chmod via
	// the file to keep the exact inode.
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return fmt.Errorf("chmod 0600 on %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}
	return nil
}

// EnsurePrivateDir guarantees a directory is not group/world accessible.
// Used for directories holding sensitive state (e.g. keys).
func EnsurePrivateDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create private dir %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("chmod 0700 on %s: %w", path, err)
	}
	return nil
}
