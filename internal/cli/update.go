package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/updater"
)

// UpdateInfo holds information about available updates.
type UpdateInfo struct {
	CurrentVersion  string   `json:"current_version" yaml:"current_version"`
	LatestVersion   string   `json:"latest_version" yaml:"latest_version"`
	Channel         string   `json:"channel" yaml:"channel"`
	UpdateAvailable bool     `json:"update_available" yaml:"update_available"`
	Changelog       []string `json:"changelog,omitempty" yaml:"changelog,omitempty"`
}

// NewUpdateCommand creates the `cosca update` command.
func NewUpdateCommand() *cobra.Command {
	var channel string
	var checkOnly bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Check for and apply updates to Cosca",
		Long: `Check for updates to the Cosca and apply them.

Supports multiple update channels:
  - stable:   Production-ready releases (default)
  - beta:     Pre-release features with testing
  - nightly:  Latest development builds

Use --check-only to only check for updates without applying them.
`,
		Example: `  cosca update                  # Check and update from stable channel
  cosca update --channel beta   # Update from beta channel
  cosca update --channel nightly # Update from nightly channel
  cosca update --check-only     # Check for updates only
  cosca update --json           # Output in JSON format`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			formatter.Verbose(fmt.Sprintf("Checking for updates (channel: %s)...", channel))

			// Validate channel
			validChannels := map[string]bool{"stable": true, "beta": true, "nightly": true}
			if !validChannels[channel] {
				return fmt.Errorf("invalid channel: %s (valid: stable, beta, nightly)", channel)
			}

			// Check for updates
			upd := updater.NewUpdater(Version, channel)
			if upd == nil {
				return fmt.Errorf("failed to initialize updater")
			}

			updateInfo, err := upd.Check()
			if err != nil {
				return fmt.Errorf("failed to check for updates: %w", err)
			}

			if !updateInfo.UpdateAvailable {
				if !useJSON {
					formatter.Success(fmt.Sprintf("You are on the latest version (%s) on the %s channel", Version, channel))
				} else {
					return printJSON(cmd, UpdateInfo{
						CurrentVersion:  Version,
						LatestVersion:   Version,
						Channel:         channel,
						UpdateAvailable: false,
					})
				}
				return nil
			}

			if !useJSON {
				formatter.Header("Update Available")
				formatter.KeyValue("Current", Version)
				formatter.KeyValue("Latest", updateInfo.LatestVersion)
				formatter.KeyValue("Channel", channel)

				if len(updateInfo.Changelog) > 0 {
					formatter.Println("")
					formatter.Header("Changelog")
					for _, entry := range updateInfo.Changelog {
						formatter.Bullet(entry)
					}
				}
			}

			if checkOnly {
				if useJSON {
					return printJSON(cmd, UpdateInfo{
						CurrentVersion:  Version,
						LatestVersion:   updateInfo.LatestVersion,
						Channel:         channel,
						UpdateAvailable: true,
						Changelog:       updateInfo.Changelog,
					})
				}
				formatter.Println("")
				formatter.Println("Run 'cosca update' to apply this update.")
				return nil
			}

			// Apply update
			formatter.Verbose("Applying update...")
			if err := upd.Apply(); err != nil {
				return fmt.Errorf("update failed: %w", err)
			}

			if !useJSON {
				formatter.Success(fmt.Sprintf("Updated to version %s", updateInfo.LatestVersion))
				formatter.KeyValue("Channel", channel)
			}

			if useJSON {
				return printJSON(cmd, UpdateInfo{
					CurrentVersion:  Version,
					LatestVersion:   updateInfo.LatestVersion,
					Channel:         channel,
					UpdateAvailable: true,
					Changelog:       updateInfo.Changelog,
				})
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&channel, "channel", "stable", "update channel (stable, beta, nightly)")
	cmd.Flags().BoolVar(&checkOnly, "check-only", false, "only check for updates, don't apply")
	return cmd
}
