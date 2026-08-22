package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/security"
)

// NewSecurityCommand creates the `cosca security` command group.
func NewSecurityCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "security",
		Short: "Security tooling — dependency vulnerability scanning",
		Long: `Security tooling for Cosca projects.

Subcommands:
  scan    Scan project dependencies for known vulnerabilities (OSV database)

Dependency scanning uses Google's osv-scanner against the OSV.dev vulnerability
database and reports known CVEs/GHSA/OSV advisories affecting the project's
lockfiles and manifests (go.mod, go.sum, package-lock.json, etc).`,
	}
	cmd.AddCommand(NewSecurityScanCommand())
	return cmd
}

// NewSecurityScanCommand creates the `cosca security scan` command.
func NewSecurityScanCommand() *cobra.Command {
	var minSeverity string
	var recursive bool
	var exitZero bool

	cmd := &cobra.Command{
		Use:   "scan [dir]",
		Short: "Scan project dependencies for known vulnerabilities",
		Long: `Scan a directory (default: current dir) for dependency files and report
known vulnerabilities from the OSV.dev database (via Google's osv-scanner).

For each vulnerable package the scan reports the CVE/GHSA/OSV identifiers, the
severity, a short summary and the OSV advisory URL.

Exit codes:
  0  scan completed, no critical/high vulnerabilities (or --exit-zero)
  1  critical or high vulnerabilities found
  2  scan error (network failure, invalid arguments, ...)`,
		Example: `  cosca security scan                 Scan the current directory
  cosca security scan ./services      Scan a specific directory
  cosca security scan --json          Machine-readable JSON output
  cosca security scan --severity high Only report HIGH/CRITICAL
  cosca security scan --exit-zero     Always exit 0 (CI friendly)`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}

			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)
			quiet := globalFlags.Quiet

			result, err := security.Scan(cmd.Context(), security.Options{
				Dir:         dir,
				Recursive:   recursive,
				MinSeverity: minSeverity,
			})
			if err != nil {
				if useJSON {
					if jerr := printJSON(cmd, map[string]interface{}{
						"error": err.Error(),
						"dir":   dir,
					}); jerr != nil {
						return jerr
					}
				} else {
					formatter.Errorf("scan error: %v", err)
				}
				if !exitZero {
					return ExitCodeError{Code: 2}
				}
				return nil
			}

			if useJSON {
				if err := printJSON(cmd, result); err != nil {
					return err
				}
			} else {
				printScanText(cmd, formatter, result, quiet)
			}

			// Exit code: 1 when critical/high vulns found (unless --exit-zero).
			if !exitZero && result.HasCriticalHigh() {
				return ExitCodeError{Code: 1}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&minSeverity, "severity", "low", "minimum severity to report (low, medium, high, critical)")
	cmd.Flags().BoolVar(&recursive, "recursive", true, "recursively scan subdirectories for dependency files")
	cmd.Flags().BoolVar(&exitZero, "exit-zero", false, "always exit with status 0 (for CI)")

	return cmd
}

// ExitCodeError carries an explicit process exit code out of a cobra RunE.
type ExitCodeError struct {
	Code int
}

func (e ExitCodeError) Error() string {
	return fmt.Sprintf("exit code %d", e.Code)
}

// ExitCode extracts the process exit code from a command error. Returns 0 when
// the error does not carry an explicit exit code.
func ExitCode(err error) int {
	var ece ExitCodeError
	if errors.As(err, &ece) {
		return ece.Code
	}
	return 0
}

// printScanText renders the human-readable scan report.
func printScanText(cmd *cobra.Command, f *OutputFormatter, result *security.ScanResult, quiet bool) {
	f.Header("Dependency Security Scan")
	f.KeyValue("Directory", result.Dir)

	if result.NoDependencyFiles {
		f.Warning("No dependency files found — nothing to scan")
		return
	}

	f.KeyValue("Dependency Sources", fmt.Sprintf("%d", result.Sources))
	f.KeyValue("Packages Scanned", fmt.Sprintf("%d", result.PackagesScanned))
	f.KeyValue("Vulnerable Packages", fmt.Sprintf("%d", result.VulnerablePackages))
	f.KeyValue("Total Vulnerabilities", fmt.Sprintf("%d", result.TotalVulns))

	if result.TotalVulns == 0 {
		f.Success("No known vulnerabilities found")
		return
	}

	f.Println("")
	f.Header("Vulnerabilities")
	for _, pv := range result.Vulnerabilities {
		displayName := pv.Package
		if pv.Version != "" {
			displayName += "@" + pv.Version
		}
		f.Printf("  %s\n", displayName)
		if !quiet {
			f.Printf("    %s ecosystem=%s source=%s%s\n", f.colors.Dim, pv.Ecosystem, pv.Source, f.colors.Reset)
		}
		for _, v := range pv.Vulnerabilities {
			sevColor := severityColor(f, v.Severity)
			line := fmt.Sprintf("    %s[%s]%s %s — %s",
				sevColor, v.Severity, f.colors.Reset, v.ID, v.Summary)
			f.Printf("%s\n", line)
			if !quiet {
				f.Printf("      %s%s%s\n", f.colors.Dim, v.URL, f.colors.Reset)
			}
		}
	}

	f.Println("")
	f.Header("Summary")
	for _, sev := range []string{
		security.SeverityCritical,
		security.SeverityHigh,
		security.SeverityMedium,
		security.SeverityLow,
		security.SeverityUnknown,
	} {
		if n := result.SeverityCounts[sev]; n > 0 {
			color := severityColor(f, sev)
			f.Printf("  %s%s%s  %d\n", color, sev, f.colors.Reset, n)
		}
	}

	f.Println("")
	if result.HasCriticalHigh() {
		f.Errorf("CRITICAL/HIGH vulnerabilities found — fix before deploy")
	} else {
		f.Warning("No critical/high vulnerabilities found")
	}
}

// severityColor returns the ANSI color for a severity level.
func severityColor(f *OutputFormatter, sev string) string {
	red := f.colors.Red
	yellow := f.colors.Yellow
	blue := f.colors.Blue
	if red == "" {
		red = "\x1b[31m"
	}
	if yellow == "" {
		yellow = "\x1b[33m"
	}
	if blue == "" {
		blue = "\x1b[34m"
	}
	switch sev {
	case security.SeverityCritical:
		return red
	case security.SeverityHigh:
		return yellow
	case security.SeverityMedium:
		return yellow
	case security.SeverityLow:
		return blue
	default:
		return f.colors.Dim
	}
}

// exitCodeForScan computes the exit code a scan result should produce:
// 0 when clean, 1 when critical/high vulns are present.
func exitCodeForScan(result *security.ScanResult, exitZero bool) int {
	if exitZero {
		return 0
	}
	if result.HasCriticalHigh() {
		return 1
	}
	return 0
}
