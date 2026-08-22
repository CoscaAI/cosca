package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/framework"
)

// ValidateResult holds the result of the validation.
type ValidateResult struct {
	ProjectDir string   `json:"project_dir" yaml:"project_dir"`
	Valid      bool     `json:"valid" yaml:"valid"`
	Checks     []string `json:"checks" yaml:"checks"`
	Errors     []string `json:"errors" yaml:"errors"`
	Warnings   []string `json:"warnings" yaml:"warnings"`
}

// NewValidateCommand creates the `cosca validate` command.
func NewValidateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate the Cosca project setup",
		Long: `Validate that the Cosca project is correctly set up and all components are functional.

Checks include:
  - Project structure (.cosca directory)
  - Configuration file
  - Runtime setup
  - Knowledge base integrity

Framework subcommands validate the versioned framework directory
(.cosca/framework), which remains the source of truth for validation:
  - crossrefs    Validate internal markdown cross-references
  - conventions  Validate CONVENTIONS.md compliance
  - orphans      Detect markdown files not referenced anywhere
`,
		Example: `  cosca validate                    # Validate project setup
  cosca validate --json             # JSON output
  cosca validate crossrefs          # Validate framework cross-references
  cosca validate conventions        # Validate CONVENTIONS.md compliance
  cosca validate orphans            # Detect orphan framework files`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}

			coscaDir := filepath.Join(dir, ".cosca")
			result := ValidateResult{
				ProjectDir: dir,
			}

			// Check .cosca directory
			if info, err := os.Stat(coscaDir); err == nil && info.IsDir() {
				result.Checks = append(result.Checks, ".cosca directory exists")
			} else {
				result.Errors = append(result.Errors, ".cosca directory not found")
			}

			// Check config file
			cfgPath := filepath.Join(coscaDir, "config.yaml")
			if _, err := os.Stat(cfgPath); err == nil {
				result.Checks = append(result.Checks, "config.yaml exists")
			} else {
				result.Warnings = append(result.Warnings, "config.yaml not found")
			}

			// Check runtime directory
			rtDir := filepath.Join(coscaDir, "runtime")
			if info, err := os.Stat(rtDir); err == nil && info.IsDir() {
				result.Checks = append(result.Checks, "runtime directory exists")
			} else {
				result.Warnings = append(result.Warnings, "runtime directory not found")
			}

			result.Valid = len(result.Errors) == 0

			if useJSON {
				return printJSON(cmd, result)
			}

			formatter.Header("Validation Results")
			for _, c := range result.Checks {
				formatter.Success(c)
			}
			for _, w := range result.Warnings {
				formatter.Warning(w)
			}
			for _, e := range result.Errors {
				formatter.Errorf("%s", e)
			}

			if result.Valid {
				formatter.Success("Project validation passed")
			} else {
				formatter.Errorf("Project validation failed")
			}

			return nil
		},
	}

	// Framework validation subcommands. These operate on the versioned
	// framework directory (.cosca/framework), which remains the source of truth.
	cmd.AddCommand(
		NewValidateCrossrefsCommand(),
		NewValidateConventionsCommand(),
		NewValidateOrphansCommand(),
	)

	return cmd
}

// defaultFrameworkRoot returns the versioned framework directory,
// <cwd>/.cosca/framework, which remains the source of truth for validation.
func defaultFrameworkRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return filepath.Join(".", ".cosca", "fallback")
	}
	return filepath.Join(dir, ".cosca", "fallback")
}

// resolveFrameworkRoot returns the framework root to operate on. When the
// --root flag is empty it falls back to <cwd>/.cosca/framework.
func resolveFrameworkRoot(flag string) string {
	if flag != "" {
		return flag
	}
	return defaultFrameworkRoot()
}

// NewValidateCrossrefsCommand creates the `cosca validate crossrefs` command.
func NewValidateCrossrefsCommand() *cobra.Command {
	var root string
	cmd := &cobra.Command{
		Use:   "crossrefs",
		Short: "Validate internal markdown cross-references in the framework",
		Long: `Validate every relative markdown link in the Cosca framework directory.

Resolves each [text](target) link against the directory of the file that
contains it and reports links whose target does not exist. External links
(http, https, anchors, mailto) are ignored.

Exit code is non-zero when any broken cross-reference is found.`,
		Example: `  cosca validate crossrefs                    # Validate the default framework root
  cosca validate crossrefs --root .cosca/framework
  cosca validate crossrefs --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFrameworkIssues(cmd, resolveFrameworkRoot(root), "crossrefs", framework.ValidateCrossReferences)
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "root of the Cosca framework to validate (default: <cwd>/.cosca/framework)")
	return cmd
}

// NewValidateConventionsCommand creates the `cosca validate conventions` command.
func NewValidateConventionsCommand() *cobra.Command {
	var root string
	cmd := &cobra.Command{
		Use:   "conventions",
		Short: "Validate CONVENTIONS.md compliance in the framework",
		Long: `Validate that framework files comply with CONVENTIONS.md.

Checks metadata blocks (Version/Status), semantic versions, mandatory
sections (including HISTORY) for department skills and workflows, and
internal cross-references.

Exit code is non-zero when any error-severity issue is found.`,
		Example: `  cosca validate conventions                  # Validate the default framework root
  cosca validate conventions --root .cosca/framework
  cosca validate conventions --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFrameworkIssues(cmd, resolveFrameworkRoot(root), "conventions", framework.ValidateConventions)
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "root of the Cosca framework to validate (default: <cwd>/.cosca/framework)")
	return cmd
}

// NewValidateOrphansCommand creates the `cosca validate orphans` command.
func NewValidateOrphansCommand() *cobra.Command {
	var root string
	cmd := &cobra.Command{
		Use:   "orphans",
		Short: "Detect orphan markdown files in the framework",
		Long: `Detect markdown files in the Cosca framework that are not referenced
by any other markdown file.

INDEX.md files and the memory/ directory are excluded from the candidate set.

Exit code is non-zero when orphan files are found.`,
		Example: `  cosca validate orphans                      # Detect orphans in the default framework root
  cosca validate orphans --root .cosca/framework
  cosca validate orphans --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFrameworkOrphans(cmd, resolveFrameworkRoot(root))
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "root of the Cosca framework to validate (default: <cwd>/.cosca/framework)")
	return cmd
}

// frameworkIssuesResult is the JSON payload emitted by the issue-based
// framework validators (crossrefs, conventions).
type frameworkIssuesResult struct {
	Root     string            `json:"root"`
	Issues   []framework.Issue `json:"issues"`
	Errors   int               `json:"errors"`
	Warnings int               `json:"warnings"`
	Infos    int               `json:"infos"`
}

// runFrameworkIssues runs an issue-based framework validator (crossrefs or
// conventions), prints a Severity/File/Message table plus a per-severity
// summary, and returns an error when error-severity issues are found so the
// process exits non-zero.
func runFrameworkIssues(cmd *cobra.Command, root, name string, validate func(string) ([]framework.Issue, error)) error {
	formatter := GetFormatter(cmd)
	useJSON := IsJSONOutput(cmd)

	issues, err := validate(root)
	if err != nil {
		return fmt.Errorf("framework %s validation failed: %w", name, err)
	}

	var errors, warnings, infos int
	rows := make([][]string, 0, len(issues))
	for _, iss := range issues {
		switch iss.Severity {
		case framework.SeverityError:
			errors++
		case framework.SeverityWarning:
			warnings++
		case framework.SeverityInfo:
			infos++
		}
		rows = append(rows, []string{string(iss.Severity), iss.File, iss.Message})
	}

	if useJSON {
		if err := printJSON(cmd, frameworkIssuesResult{
			Root:     root,
			Issues:   issues,
			Errors:   errors,
			Warnings: warnings,
			Infos:    infos,
		}); err != nil {
			return err
		}
		if errors > 0 {
			return fmt.Errorf("framework %s validation failed: %d error(s) found", name, errors)
		}
		return nil
	}

	formatter.Header(fmt.Sprintf("Framework Validation — %s", name))
	formatter.KeyValue("Root", root)

	if len(rows) == 0 {
		formatter.Success("No issues found")
		return nil
	}

	formatter.Table([]string{"Severity", "File", "Message"}, rows)

	formatter.Header("Summary")
	formatter.KeyValue("Errors", fmt.Sprint(errors))
	formatter.KeyValue("Warnings", fmt.Sprint(warnings))
	formatter.KeyValue("Info", fmt.Sprint(infos))
	formatter.KeyValue("Total", fmt.Sprint(len(issues)))

	if errors > 0 {
		return fmt.Errorf("framework %s validation failed: %d error(s) found", name, errors)
	}
	return nil
}

// frameworkOrphansResult is the JSON payload emitted by `validate orphans`.
type frameworkOrphansResult struct {
	Root    string   `json:"root"`
	Orphans []string `json:"orphans"`
	Count   int      `json:"count"`
}

// runFrameworkOrphans runs DetectOrphans, prints the orphan list plus a
// count, and returns an error when orphan files are found so the process
// exits non-zero.
func runFrameworkOrphans(cmd *cobra.Command, root string) error {
	formatter := GetFormatter(cmd)
	useJSON := IsJSONOutput(cmd)

	orphans, err := framework.DetectOrphans(root)
	if err != nil {
		return fmt.Errorf("framework orphan detection failed: %w", err)
	}

	if useJSON {
		if err := printJSON(cmd, frameworkOrphansResult{
			Root:    root,
			Orphans: orphans,
			Count:   len(orphans),
		}); err != nil {
			return err
		}
		if len(orphans) > 0 {
			return fmt.Errorf("framework orphan detection found %d orphan file(s)", len(orphans))
		}
		return nil
	}

	formatter.Header("Framework Orphan Detection")
	formatter.KeyValue("Root", root)

	if len(orphans) == 0 {
		formatter.Success("No orphan files found")
		return nil
	}

	for _, o := range orphans {
		formatter.Bullet(o)
	}
	formatter.KeyValue("Orphans", fmt.Sprint(len(orphans)))

	return fmt.Errorf("framework orphan detection found %d orphan file(s)", len(orphans))
}
