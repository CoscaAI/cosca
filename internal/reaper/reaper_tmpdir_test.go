package reaper

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func allowedRoot(path string) []string {
	c, err := canonicalPath(path)
	if err != nil {
		return []string{path}
	}
	return []string{c}
}

func TestTmpDirReapsInsideRoot(t *testing.T) {
	base := t.TempDir()
	sub := filepath.Join(base, "work")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	rp := TmpDirReaper()
	out, err := rp.Reap(context.Background(), Entry{Kind: KindTmpDir, ID: sub}, ReapContext{AllowedTmpRoots: allowedRoot(base)})
	if err != nil {
		t.Fatalf("Reap error: %v", err)
	}
	if out.Status != StatusReaped {
		t.Fatalf("status = %s, want reaped (%s)", out.Status, out.Reason)
	}
	if _, err := os.Lstat(sub); !os.IsNotExist(err) {
		t.Fatalf("sub should be removed, Lstat err = %v", err)
	}
}

func TestTmpDirAntiEscapeOutsideRoot(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir() // not below base

	rp := TmpDirReaper()
	out, err := rp.Reap(context.Background(), Entry{Kind: KindTmpDir, ID: outside}, ReapContext{AllowedTmpRoots: allowedRoot(base)})
	if err != nil {
		t.Fatalf("Reap error: %v", err)
	}
	if out.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped (outside allowed roots)", out.Status)
	}
	if _, err := os.Lstat(outside); err != nil {
		t.Fatalf("outside must NOT be removed; Lstat err = %v", err)
	}
}

func TestTmpDirRelativePathSkipped(t *testing.T) {
	rp := TmpDirReaper()
	out, err := rp.Reap(context.Background(), Entry{Kind: KindTmpDir, ID: "relative/../path"}, ReapContext{})
	if err != nil {
		t.Fatalf("Reap error: %v", err)
	}
	if out.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped (not absolute)", out.Status)
	}
}

func TestTmpDirRootItselfNotRemoved(t *testing.T) {
	base := t.TempDir()
	rp := TmpDirReaper()
	// A tmpdir resource that IS the allowed root must never be removed
	// (belowRoot is strict and excludes equality).
	out, err := rp.Reap(context.Background(), Entry{Kind: KindTmpDir, ID: base}, ReapContext{AllowedTmpRoots: allowedRoot(base)})
	if err != nil {
		t.Fatalf("Reap error: %v", err)
	}
	if out.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped (root itself is not below root)", out.Status)
	}
	if _, err := os.Lstat(base); err != nil {
		t.Fatalf("allowed root must not be removed; Lstat err = %v", err)
	}
}

func TestTmpDirMissing(t *testing.T) {
	base := t.TempDir()
	missing := filepath.Join(base, "does-not-exist")
	rp := TmpDirReaper()
	out, err := rp.Reap(context.Background(), Entry{Kind: KindTmpDir, ID: missing}, ReapContext{AllowedTmpRoots: allowedRoot(base)})
	if err != nil {
		t.Fatalf("Reap error: %v", err)
	}
	if out.Status != StatusMissing {
		t.Fatalf("status = %s, want missing", out.Status)
	}
}

func TestTmpDirSymlinkEscapeBlocked(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir() // the real target, outside base
	link := filepath.Join(base, "sneaky")
	if err := os.Symlink(outside, link); err != nil {
		// Creating a symlink may require privilege on some Windows setups/devenv;
		// the containment guarantee is still covered by the other tests.
		t.Skipf("cannot create symlink (%v); skipping symlink-escape test", err)
	}

	rp := TmpDirReaper()
	out, err := rp.Reap(context.Background(), Entry{Kind: KindTmpDir, ID: link}, ReapContext{AllowedTmpRoots: allowedRoot(base)})
	if err != nil {
		t.Fatalf("Reap error: %v", err)
	}
	// canonicalPath resolves `link` to `outside`, which is not below base → skipped.
	if out.Status != StatusSkipped {
		t.Fatalf("status = %s, want skipped (symlink resolves outside allowed roots)", out.Status)
	}
	if _, err := os.Lstat(outside); err != nil {
		t.Fatalf("symlink target must NOT be removed; Lstat err = %v", err)
	}
}
