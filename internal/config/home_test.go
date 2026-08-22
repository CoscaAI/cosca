package config

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

// TestUserHomeDir_SudoResolvesRealUser verifies that when SUDO_UID is set
// (process running via sudo with HOME=/root), the real user's home is
// resolved instead of root's.
func TestUserHomeDir_SudoResolvesRealUser(t *testing.T) {
	cur, err := user.Current()
	if err != nil {
		t.Skipf("cannot resolve current user: %v", err)
	}
	if cur.Uid == "0" {
		t.Skip("running as root; SUDO_UID path not applicable")
	}

	// Simulate sudo: HOME=/root but SUDO_UID points at the real user.
	t.Setenv("SUDO_UID", cur.Uid)
	t.Setenv("SUDO_GID", cur.Gid)
	t.Setenv("HOME", "/root")

	got := UserHomeDir()
	if got == "/root" {
		t.Fatalf("UserHomeDir() = %q under sudo; expected real user home, got root's", got)
	}
	if got != cur.HomeDir {
		t.Errorf("UserHomeDir() = %q, want %q", got, cur.HomeDir)
	}
}

// TestUserHomeDir_HOMEUsed verifies the explicit HOME env var is honored
// when no SUDO_UID is present.
func TestUserHomeDir_HOMEUsed(t *testing.T) {
	t.Setenv("SUDO_UID", "")
	t.Setenv("HOME", "/home/algum-usuario")

	got := UserHomeDir()
	if got != "/home/algum-usuario" {
		t.Errorf("UserHomeDir() = %q, want %q", got, "/home/algum-usuario")
	}
}

// TestUserHomeDir_NeverRootWithoutSudo verifies that a non-root process
// never resolves to /root just because the environment says so.
func TestUserHomeDir_NeverRootWithoutSudo(t *testing.T) {
	cur, err := user.Current()
	if err != nil {
		t.Skipf("cannot resolve current user: %v", err)
	}
	if cur.Uid == "0" {
		t.Skip("running as root; assertion not applicable")
	}

	// Simulate a broken environment that would previously fall back to /root.
	t.Setenv("SUDO_UID", "")
	t.Setenv("HOME", "/root")

	got := UserHomeDir()
	if got == "/root" {
		t.Errorf("UserHomeDir() = %q for non-root user; must not resolve to /root", got)
	}
}

// TestDefaultConfig_HomeNotRoot verifies DefaultConfig derives paths from the
// real user's home, not from a sudo-polluted HOME.
func TestDefaultConfig_HomeNotRoot(t *testing.T) {
	cur, err := user.Current()
	if err != nil {
		t.Skipf("cannot resolve current user: %v", err)
	}
	if cur.Uid == "0" {
		t.Skip("running as root; assertion not applicable")
	}

	t.Setenv("SUDO_UID", cur.Uid)
	t.Setenv("SUDO_GID", cur.Gid)
	t.Setenv("HOME", "/root")

	cfg := DefaultConfig()
	if cfg.Paths.Home == "/root/.config/cosca" {
		t.Errorf("DefaultConfig().Paths.Home = %q; must not point at /root", cfg.Paths.Home)
	}
	wantPrefix := filepath.Join(cur.HomeDir, ".config", "cosca")
	if cfg.Paths.Home != wantPrefix {
		t.Errorf("Paths.Home = %q, want %q", cfg.Paths.Home, wantPrefix)
	}
	if cfg.DB.Path != filepath.Join(wantPrefix, "cosca.db") {
		t.Errorf("DB.Path = %q, want under %q", cfg.DB.Path, wantPrefix)
	}
}

// TestUserHomeDir_EnvNotPolluted is a guard: running normally (no sudo,
// HOME = real home), the resolved home must match HOME.
func TestUserHomeDir_EnvNotPolluted(t *testing.T) {
	home := os.Getenv("HOME")
	if home == "" {
		t.Skip("HOME not set")
	}
	t.Setenv("SUDO_UID", "")
	t.Setenv("HOME", home)

	got := UserHomeDir()
	if got != home {
		t.Errorf("UserHomeDir() = %q, want %q", got, home)
	}
}
