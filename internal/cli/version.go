package cli

import (
	"fmt"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/pkg/cosca"
)

// Version, Commit, and BuildDate mirror pkg/cosca, which is the single source
// of truth for build metadata (populated via ldflags and the runtime
// build-info fallback). These aliases are kept for callers in this package
// that reference the shorter names.
var (
	Version   = cosca.Version
	Commit    = cosca.CommitHash
	BuildDate = cosca.BuildDate
)

// VersionInfo holds structured version information.
type VersionInfo struct {
	Version   string `json:"version" yaml:"version"`
	Commit    string `json:"commit" yaml:"commit"`
	BuildDate string `json:"build_date" yaml:"build_date"`
	GoVersion string `json:"go_version" yaml:"go_version"`
	Platform  string `json:"platform" yaml:"platform"`
	Arch      string `json:"arch" yaml:"arch"`
	Compiler  string `json:"compiler" yaml:"compiler"`
}

// NewVersionCommand creates the `cosca version` command.
func NewVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Display version information",
		Long:  `Display the Cosca version, build date, commit hash, and runtime information.`,
		Example: `  cosca version
  cosca version --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			output := cmd.Root().PersistentFlags().Lookup("json")
			useJSON := output != nil && output.Value.String() == "true"

			info := VersionInfo{
				Version:   cosca.Version,
				Commit:    cosca.CommitHash,
				BuildDate: cosca.BuildDate,
				GoVersion: runtime.Version(),
				Platform:  runtime.GOOS,
				Arch:      runtime.GOARCH,
				Compiler:  runtime.Compiler,
			}

			if useJSON {
				return printJSON(cmd, info)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Cosca\n")
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", formatKey("Version"), formatValue(cosca.Version))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", formatKey("Commit"), formatValue(cosca.CommitHash))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", formatKey("Build Date"), formatValue(cosca.BuildDate))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", formatKey("Go Version"), formatValue(runtime.Version()))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s/%s\n", formatKey("Platform"), formatValue(runtime.GOOS), formatValue(runtime.GOARCH))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", formatKey("Compiler"), formatValue(runtime.Compiler))

			return nil
		},
	}

	return cmd
}

func formatKey(s string) string {
	return fmt.Sprintf("\033[36m%s\033[0m", s)
}

func formatValue(s string) string {
	return fmt.Sprintf("\033[33m%s\033[0m", s)
}

// GetVersionInfo returns the current version information.
func GetVersionInfo() VersionInfo {
	return VersionInfo{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS,
		Arch:      runtime.GOARCH,
		Compiler:  runtime.Compiler,
	}
}

// GetBuildDate parses the build date string into a time.Time.
func GetBuildDate() time.Time {
	t, err := time.Parse(time.RFC3339, BuildDate)
	if err != nil {
		return time.Time{}
	}
	return t
}
