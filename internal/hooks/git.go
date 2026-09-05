// Package hooks provides Git hook installation and post-commit impact
// report generation for Cosca. It replaces the legacy bash hooks that lived
// under the framework's scripts/hooks/ (now .cosca/framework/scripts/hooks/)
// with native Go functionality baked into the cosca binary
// (`cosca hook install` / `cosca hook post-commit`).
//
// The runtime home for everything this package writes is .cosca/ (the legacy
// framework home is now .cosca/framework/): impact reports land in
// .cosca/memory/timeline/impact-reports/{hash}.md and the engineering
// timeline in .cosca/memory/timeline/ENGINEERING_TIMELINE.md.
package hooks

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runGit executes a git command in dir and returns its stdout.
// LC_ALL=C forces the English output so summary lines can be parsed
// reliably regardless of the machine locale.
func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w (%s)",
			strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// ensureGitRepo verifies that dir is inside a Git repository.
func ensureGitRepo(dir string) error {
	if _, err := runGit(dir, "rev-parse", "--git-dir"); err != nil {
		return fmt.Errorf("%s is not a Git repository: %w", dir, err)
	}
	return nil
}

// gitDirFor resolves the .git directory of the repository at repoDir.
// It supports regular repositories (.git/ directory) and worktrees
// (.git file containing "gitdir: <path>"), mirroring the resolution
// performed by the legacy install-hooks.sh.
func gitDirFor(repoDir string) (string, error) {
	dotGit := filepath.Join(repoDir, ".git")

	info, err := os.Stat(dotGit)
	if err == nil && info.IsDir() {
		return dotGit, nil
	}

	// .git is a file — worktree or submodule layout.
	if err == nil && !info.IsDir() {
		data, rerr := os.ReadFile(dotGit)
		if rerr != nil {
			return "", fmt.Errorf("read %s: %w", dotGit, rerr)
		}
		gd := strings.TrimSpace(strings.TrimPrefix(string(data), "gitdir:"))
		if gd == "" {
			return "", fmt.Errorf("malformed gitdir file: %s", dotGit)
		}
		if !filepath.IsAbs(gd) {
			gd = filepath.Join(repoDir, gd)
		}
		return gd, nil
	}

	// Fall back to git itself (bare repositories, exotic layouts).
	out, gerr := runGit(repoDir, "rev-parse", "--git-dir")
	if gerr != nil {
		return "", fmt.Errorf("%s is not a Git repository: %w", repoDir, err)
	}
	gd := strings.TrimSpace(out)
	if !filepath.IsAbs(gd) {
		gd = filepath.Join(repoDir, gd)
	}
	return gd, nil
}
