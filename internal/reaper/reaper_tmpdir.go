package reaper

import (
	"context"
	"os"
	"path/filepath"
)

// tmpdirReaper removes a temporary directory only when it is an absolute path
// that canonicalizes (realpath, symlinks resolved) to a strict descendant of an
// allowed root. This is the containment guarantee: it never rm -rf's an
// arbitrary path or an allowed root itself.
type tmpdirReaper struct{}

// Kind returns KindTmpDir.
func (tmpdirReaper) Kind() string { return KindTmpDir }

// TmpDirReaper returns the built-in temporary-directory reaper.
func TmpDirReaper() Reaper { return tmpdirReaper{} }

// Reap disposes a tmpdir resource after containment verification.
func (tmpdirReaper) Reap(ctx context.Context, e Entry, rc ReapContext) (Outcome, error) {
	if !filepath.IsAbs(e.ID) {
		return Outcome{Status: StatusSkipped, Reason: "outside allowed roots"}, nil
	}
	// Resolve symlinks on the longest existing prefix, then re-check containment
	// on the canonical path. This blocks `..` and symlink escapes.
	canonical, err := canonicalPath(e.ID)
	if err != nil {
		return Outcome{Status: StatusSkipped, Reason: "cannot canonicalize"}, nil
	}
	if !rc.containedByAllowedRoot(canonical) {
		return Outcome{Status: StatusSkipped, Reason: "outside allowed roots"}, nil
	}
	if _, err := os.Lstat(e.ID); err != nil {
		if os.IsNotExist(err) {
			return Outcome{Status: StatusMissing}, nil
		}
		return Outcome{Status: StatusError, Reason: err.Error()}, nil
	}
	if err := os.RemoveAll(e.ID); err != nil {
		return Outcome{Status: StatusError, Reason: err.Error()}, nil
	}
	return Outcome{Status: StatusReaped}, nil
}
