package config

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
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
	// Doutrina do Don (projeto-local): o db.path NUNCA deve cair no home global
	// ~/.config/cosca/cosca.db. Com o SUDO_UID setado mas um cwd de projeto, o DB
	// resolve sob o projeto (.cosca/cosca.db), não sob o home do usuário.
	if cfg.DB.Path == filepath.Join(wantPrefix, "cosca.db") {
		t.Errorf("DB.Path = %q; must not be the global ~/.config/cosca/cosca.db (projeto-local doctrine)", cfg.DB.Path)
	}
	if err := configValidateProjectLocalDB(cfg.DB.Path); err != nil {
		t.Errorf("DB.Path project-local validation: %v", err)
	}
}

// configValidateProjectLocalDB assegura que o db.path default não aponte para
// o home global (~/.config/cosca) — apenas para um diretório .cosca de projeto.
func configValidateProjectLocalDB(dbPath string) error {
	if strings.Contains(filepath.ToSlash(dbPath), ".config/cosca/") {
		return fmt.Errorf("db.path %q points into global home config", dbPath)
	}
	if !strings.Contains(filepath.ToSlash(dbPath), ".cosca/") {
		return fmt.Errorf("db.path %q is not under a project .cosca/", dbPath)
	}
	return nil
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
