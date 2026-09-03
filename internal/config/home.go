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
	"path/filepath"

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

// defaultProjectDBPath resolve o caminho do banco principal do PROJETO a partir
// do diretório do projeto, em vez de cair no global ~/.config/cosca/cosca.db.
//
// Doutrina do Don: cada projeto tem seu próprio banco (isolado). O `cosca init`
// em qualquer pasta deve gravar um `db.path` projeto-local, nunca global — caso
// contrário um `cosca.db` fantasma nasce em ~/.config/cosca (reprodução do bug
// do config.yaml apontando para um banco que não existe).
//
// Resolução: se o diretório do projeto não for definido/relativo, sobe a árvore
// a partir do cwd até achar o `.cosca/` ou um marcador de projeto (go.mod/.git);
// fallback é <projeto>/.cosca/cosca.db.
func defaultProjectDBPath(projectDir string) string {
	// Nenhum caminho do projeto: resolve por walk-up a partir do cwd.
	if projectDir == "" || projectDir == "." {
		projectDir = resolveProjectRoot()
		if projectDir == "" {
			return filepath.Join(projectDir, "cosca.db")
		}
		return filepath.Join(projectDir, ".cosca", "cosca.db")
	}
	return filepath.Join(projectDir, ".cosca", "cosca.db")
}

// resolveProjectRoot sobe a árvore a partir do cwd até achar `.cosca/` ou um
// marcador de projeto. Devolve "" se nada for encontrado (o chamador decide o
// fallback). Nunca aponta para o home global.
func resolveProjectRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	dir := wd
	for {
		if info, err := os.Stat(filepath.Join(dir, ".cosca")); err == nil && info.IsDir() {
			return dir
		}
		for _, marker := range []string{"go.mod", "package.json", ".git", "Cargo.toml", "pyproject.toml"} {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
