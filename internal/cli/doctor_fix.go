package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Fix describes a suggested correction and whether it is SAFE to
// auto-apply. SAFE fixes never require elevated privileges and only touch
// the workspace .cosca tree; REQUIRES_SUDO fixes (drivers, kernel,
// bootloader, firewall, system packages) are NEVER executed by --fix and
// are printed as commands for the Don to run.
type Fix struct {
	Action  string
	Safe    bool
	Command string
}

// classifyFix classifies a fix using the established convention: fixes
// starting with "sudo" or mentioning drivers/kernels/bootloaders/firewalls
// require elevated privileges; everything else is SAFE.
func classifyFix(fix string) Fix {
	return Fix{
		Action:  fix,
		Safe:    !fixRequiresSudo(fix),
		Command: fix,
	}
}

// fixRequiresSudo reports whether a fix must be run by the Don with sudo.
func fixRequiresSudo(fix string) bool {
	lower := strings.ToLower(fix)
	if strings.HasPrefix(lower, "sudo") {
		return true
	}
	for _, kw := range []string{"driver", "kernel", "bootloader", "firewall", "grub"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// withinCoscaTree reports whether target is inside the workspace .cosca
// tree. --fix never touches anything outside of it.
func withinCoscaTree(coscaDir, target string) bool {
	rel, err := filepath.Rel(coscaDir, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// applySafeFix executes a single SAFE fix string inside the workspace
// .cosca tree. It returns an action description, whether the fix was
// applied, and an error. Fixes that are safe but have no auto-apply
// action (plain "Run 'cosca ...'" suggestions) are reported as not
// applied.
func applySafeFix(coscaDir, fix string) (action string, applied bool, err error) {
	switch {
	case strings.HasPrefix(fix, "Create directory "):
		dir := strings.TrimPrefix(fix, "Create directory ")
		if !withinCoscaTree(coscaDir, dir) {
			return "", false, fmt.Errorf("refusing to create directory outside .cosca: %s", dir)
		}
		if err := os.MkdirAll(dir, 0700); err != nil {
			return "", false, err
		}
		_ = os.Chmod(dir, 0700)
		return "created " + dir, true, nil

	case strings.HasPrefix(fix, "Set permissions "):
		rest := strings.TrimPrefix(fix, "Set permissions ")
		parts := strings.SplitN(rest, " on ", 2)
		if len(parts) != 2 {
			return "", false, nil
		}
		path := strings.TrimSpace(parts[1])
		if !withinCoscaTree(coscaDir, path) {
			return "", false, fmt.Errorf("refusing to chmod outside .cosca: %s", path)
		}
		var mode os.FileMode
		if _, err := fmt.Sscanf(strings.TrimSpace(parts[0]), "%o", &mode); err != nil {
			return "", false, fmt.Errorf("invalid permission mode %q", parts[0])
		}
		if err := os.Chmod(path, mode); err != nil {
			return "", false, err
		}
		return "set permissions on " + path, true, nil

	case strings.HasPrefix(fix, "Rebuild knowledge index"):
		ke := NewKnowledgeEngine(coscaDir)
		if ke == nil {
			return "", false, fmt.Errorf("knowledge engine not available")
		}
		if err := ke.Rebuild(); err != nil {
			return "", false, err
		}
		return "rebuilt knowledge index", true, nil
	}

	return "", false, nil
}

// applyDoctorFixes classifies each fix and applies the SAFE ones. Fixes
// that require sudo are listed for the Don and never executed. It returns
// the number of fixes applied.
func applyDoctorFixes(f *OutputFormatter, coscaDir string, fixes []string) int {
	executed := 0
	for _, fix := range fixes {
		classified := classifyFix(fix)
		if !classified.Safe {
			f.Warning(fmt.Sprintf("requer o Don (sudo): %s", classified.Command))
			continue
		}
		action, applied, err := applySafeFix(coscaDir, fix)
		if err != nil {
			f.Warning(fmt.Sprintf("não foi possível corrigir: %s (%v)", fix, err))
			continue
		}
		if !applied {
			f.Bullet(fmt.Sprintf("sugerido (manual): %s", fix))
			continue
		}
		executed++
		f.Success(fmt.Sprintf("corrigido: %s", action))
	}
	return executed
}

// requiredCoscaDirs lists the .cosca subdirectories that `cosca doctor
// --fix` verifies and safely re-creates when missing.
var requiredCoscaDirs = []string{"memory", "cache", "index", "plugins", "sessions"}

// checkCoscaDirs verifies that the required .cosca subdirectories exist.
// Missing directories are reported as failures with SAFE "Create
// directory" fixes that --fix executes. This check never creates
// anything by itself.
func checkCoscaDirs(coscaDir string) CheckResult {
	result := CheckResult{}
	for _, sub := range requiredCoscaDirs {
		dir := filepath.Join(coscaDir, sub)
		info, err := os.Stat(dir)
		switch {
		case err == nil && info.IsDir():
			result.Checks = append(result.Checks, CheckItem{
				Name: sub + " directory", Status: "pass", Detail: dir,
			})
		case os.IsNotExist(err):
			result.Checks = append(result.Checks, CheckItem{
				Name: sub + " directory", Status: "fail", Detail: "missing: " + dir,
			})
			result.Issues = append(result.Issues, "Missing directory: "+dir)
			result.Fixes = append(result.Fixes, "Create directory "+dir)
		default:
			result.Checks = append(result.Checks, CheckItem{
				Name: sub + " directory", Status: "warning", Detail: fmt.Sprintf("cannot stat %s: %v", dir, err),
			})
		}
	}
	return result
}

// hasFailed returns true when any check item in the result failed.
func hasFailed(result CheckResult) bool {
	for _, c := range result.Checks {
		if c.Status == "fail" {
			return true
		}
	}
	return false
}

// fixDoctorChecks returns the checks run under --fix: the standard set
// plus the Directories check that drives safe directory creation. When a
// subsystem is given, only that subsystem's check runs.
func fixDoctorChecks(coscaDir, subsystem string) []doctorCheck {
	checks := append([]doctorCheck{
		{"Directories", func() CheckResult { return checkCoscaDirs(coscaDir) }},
	}, doctorChecks(coscaDir)...)
	if subsystem != "" {
		for _, c := range checks {
			if strings.EqualFold(c.name, subsystem) {
				return []doctorCheck{c}
			}
		}
	}
	return checks
}

// runDoctorFix runs the checks, applies every SAFE fix of failed checks,
// and lists fixes that require sudo for the Don. It then re-runs the
// checks and prints the updated status. Never uses sudo.
func runDoctorFix(f *OutputFormatter, coscaDir, subsystem string) error {
	f.Println("")
	f.Header("Cosca Doctor --fix")

	checks := fixDoctorChecks(coscaDir, subsystem)

	// Pass 1: run checks and collect fixes from FAILED checks.
	var fixes []string
	seen := map[string]bool{}
	for _, check := range checks {
		result := check.fn()
		if !hasFailed(result) {
			continue
		}
		for _, fix := range result.Fixes {
			if !seen[fix] {
				seen[fix] = true
				fixes = append(fixes, fix)
			}
		}
	}

	if len(fixes) == 0 {
		f.Success("nada a corrigir")
		return nil
	}

	f.Header("Correções")
	applyDoctorFixes(f, coscaDir, fixes)

	// Pass 2: re-run the checks and print the updated status.
	f.Println("")
	f.Header("Status atualizado")
	for _, check := range checks {
		printDoctorCheck(f, check)
	}

	return nil
}
