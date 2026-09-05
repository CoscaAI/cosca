package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// NewPluginCommand creates the `cosca plugin` command and its subcommands.
func NewPluginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage Cosca plugins",
		Long: `Manage the Cosca plugin system.

Plugins extend Cosca functionality with custom capabilities.
You can install, uninstall, list, update, and search for plugins.

Subcommands:
  install <name>    Install a plugin
  uninstall <name>  Uninstall a plugin
  list              List installed plugins
  update <name>     Update a plugin
  search <query>    Search for plugins
  info <name>       Show plugin information
  enable <name>     Enable a plugin
  disable <name>    Disable a plugin
`,
		Example: `  cosca plugin install my-plugin
  cosca plugin uninstall my-plugin
  cosca plugin list
  cosca plugin update my-plugin
  cosca plugin search "database"
  cosca plugin info my-plugin
  cosca plugin enable my-plugin
  cosca plugin disable my-plugin`,
	}

	cmd.AddCommand(
		NewPluginInstallCommand(),
		NewPluginUninstallCommand(),
		NewPluginListCommand(),
		NewPluginUpdateCommand(),
		NewPluginSearchCommand(),
		NewPluginInfoCommand(),
		NewPluginEnableCommand(),
		NewPluginDisableCommand(),
	)

	return cmd
}

// NewPluginInstallCommand creates the `cosca plugin install` subcommand.
func NewPluginInstallCommand() *cobra.Command {
	var source string

	cmd := &cobra.Command{
		Use:   "install <name>",
		Short: "Install a plugin",
		Long:  `Install a plugin from a registry or local source.`,
		Example: `  cosca plugin install my-plugin
  cosca plugin install my-plugin --source ./local-plugin
  cosca plugin install my-plugin --source github.com/user/plugin`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			pluginDir := filepath.Join(dir, ".cosca", "plugins")

			spinner := formatter.Spinner(fmt.Sprintf("Installing plugin %s", args[0]))
			spinner.Start()

			mgr := newPluginManagerAdapter(pluginDir)
			if mgr == nil {
				spinner.Fail("Plugin manager not available")
				return fmt.Errorf("plugin manager not available")
			}

			plugin, err := mgr.Install(args[0], source)
			if err != nil {
				spinner.Fail(fmt.Sprintf("Installation failed: %v", err))
				return fmt.Errorf("failed to install plugin: %w", err)
			}

			spinner.Stop("Plugin installed")

			if useJSON {
				return printJSON(cmd, plugin)
			}

			formatter.Success(fmt.Sprintf("Plugin %s installed", plugin.Name))
			formatter.KeyValue("Name", plugin.Name)
			formatter.KeyValue("Version", plugin.Version)
			formatter.KeyValue("Description", plugin.Description)

			return nil
		},
	}

	cmd.Flags().StringVarP(&source, "source", "s", "", "plugin source (path or registry URL)")
	return cmd
}

// NewPluginUninstallCommand creates the `cosca plugin uninstall` subcommand.
func NewPluginUninstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall <name>",
		Short: "Uninstall a plugin",
		Long:  `Remove an installed plugin from the system.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			pluginDir := filepath.Join(dir, ".cosca", "plugins")

			spinner := formatter.Spinner(fmt.Sprintf("Uninstalling plugin %s", args[0]))
			spinner.Start()

			mgr := newPluginManagerAdapter(pluginDir)
			if mgr == nil {
				spinner.Fail("Plugin manager not available")
				return fmt.Errorf("plugin manager not available")
			}

			if err := mgr.Uninstall(args[0]); err != nil {
				spinner.Fail(fmt.Sprintf("Uninstall failed: %v", err))
				return fmt.Errorf("failed to uninstall plugin: %w", err)
			}

			spinner.Stop("Plugin uninstalled")

			if useJSON {
				return printJSON(cmd, map[string]string{
					"status": "uninstalled",
					"name":   args[0],
				})
			}

			formatter.Success(fmt.Sprintf("Plugin %s uninstalled", args[0]))
			return nil
		},
	}

	return cmd
}

// NewPluginListCommand creates the `cosca plugin list` subcommand.
func NewPluginListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed plugins",
		Long:  `List all plugins currently installed in the Cosca system.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			pluginDir := filepath.Join(dir, ".cosca", "plugins")

			mgr := newPluginManagerAdapter(pluginDir)
			if mgr == nil {
				return fmt.Errorf("plugin manager not available")
			}

			plugins := mgr.List()

			if useJSON {
				return printJSON(cmd, plugins)
			}

			if len(plugins) == 0 {
				formatter.Warning("No plugins installed")
				formatter.Println("Run 'cosca plugin search' to find plugins")
				return nil
			}

			formatter.Header(fmt.Sprintf("Installed Plugins (%d)", len(plugins)))

			headers := []string{"Name", "Version", "Status", "Description"}
			rows := make([][]string, 0, len(plugins))

			for _, p := range plugins {
				status := "active"
				if !p.Enabled {
					status = "disabled"
				}
				rows = append(rows, []string{p.Name, p.Version, status, p.Description})
			}

			formatter.Table(headers, rows)
			return nil
		},
	}

	return cmd
}

// NewPluginUpdateCommand creates the `cosca plugin update` subcommand.
func NewPluginUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update a plugin",
		Long:  `Update a specific plugin to its latest version.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			pluginDir := filepath.Join(dir, ".cosca", "plugins")

			spinner := formatter.Spinner(fmt.Sprintf("Updating plugin %s", args[0]))
			spinner.Start()

			mgr := newPluginManagerAdapter(pluginDir)
			if mgr == nil {
				spinner.Fail("Plugin manager not available")
				return fmt.Errorf("plugin manager not available")
			}

			updated, err := mgr.Update(args[0])
			if err != nil {
				spinner.Fail(fmt.Sprintf("Update failed: %v", err))
				return fmt.Errorf("failed to update plugin: %w", err)
			}

			spinner.Stop("Plugin updated")

			if useJSON {
				return printJSON(cmd, updated)
			}

			formatter.Success(fmt.Sprintf("Plugin %s updated to v%s", args[0], updated.Version))
			return nil
		},
	}

	return cmd
}

// NewPluginSearchCommand creates the `cosca plugin search` subcommand.
func NewPluginSearchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search for plugins",
		Long:  `Search the plugin registry for available plugins matching the query.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			pluginDir := filepath.Join(dir, ".cosca", "plugins")

			mgr := newPluginManagerAdapter(pluginDir)
			if mgr == nil {
				return fmt.Errorf("plugin manager not available")
			}

			results, err := mgr.Search(args[0])
			if err != nil {
				return fmt.Errorf("plugin search failed: %w", err)
			}

			if useJSON {
				return printJSON(cmd, results)
			}

			if len(results) == 0 {
				formatter.Warning(fmt.Sprintf("No plugins found matching %q", args[0]))
				return nil
			}

			formatter.Header(fmt.Sprintf("Plugin Search Results for %q", args[0]))
			for _, r := range results {
				formatter.KeyValue(r.Name, r.Description)
				formatter.KeyValue("  Version", r.Version)
				formatter.KeyValue("  Downloads", fmt.Sprintf("%d", r.Downloads))
				formatter.Println("")
			}

			return nil
		},
	}

	return cmd
}

// NewPluginInfoCommand creates the `cosca plugin info` subcommand.
func NewPluginInfoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <name>",
		Short: "Show plugin information",
		Long:  `Display detailed information about a specific plugin.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			pluginDir := filepath.Join(dir, ".cosca", "plugins")

			mgr := newPluginManagerAdapter(pluginDir)
			if mgr == nil {
				return fmt.Errorf("plugin manager not available")
			}

			info, err := mgr.Info(args[0])
			if err != nil {
				return fmt.Errorf("plugin %q not found: %w", args[0], err)
			}

			if useJSON {
				return printJSON(cmd, info)
			}

			formatter.Header(fmt.Sprintf("Plugin: %s", info.Name))
			formatter.KeyValue("Name", info.Name)
			formatter.KeyValue("Version", info.Version)
			formatter.KeyValue("Description", info.Description)
			formatter.KeyValue("Author", info.Author)
			formatter.KeyValue("License", info.License)
			formatter.KeyValue("Homepage", info.Homepage)
			formatter.KeyValue("Enabled", fmt.Sprintf("%v", info.Enabled))
			formatter.KeyValue("Installed At", info.InstalledAt.Format("2006-01-02 15:04:05"))

			if len(info.Dependencies) > 0 {
				formatter.Println("")
				formatter.KeyValue("Dependencies", strings.Join(info.Dependencies, ", "))
			}

			return nil
		},
	}

	return cmd
}

// NewPluginEnableCommand creates the `cosca plugin enable` subcommand.
func NewPluginEnableCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable a plugin",
		Long:  `Enable a previously disabled plugin.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)

			dir, _ := os.Getwd()
			pluginDir := filepath.Join(dir, ".cosca", "plugins")

			mgr := newPluginManagerAdapter(pluginDir)
			if mgr == nil {
				return fmt.Errorf("plugin manager not available")
			}

			if err := mgr.Enable(args[0]); err != nil {
				return fmt.Errorf("failed to enable plugin: %w", err)
			}

			formatter.Success(fmt.Sprintf("Plugin %s enabled", args[0]))
			return nil
		},
	}

	return cmd
}

// NewPluginDisableCommand creates the `cosca plugin disable` subcommand.
func NewPluginDisableCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable a plugin",
		Long:  `Disable a plugin without uninstalling it.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)

			dir, _ := os.Getwd()
			pluginDir := filepath.Join(dir, ".cosca", "plugins")

			mgr := newPluginManagerAdapter(pluginDir)
			if mgr == nil {
				return fmt.Errorf("plugin manager not available")
			}

			if err := mgr.Disable(args[0]); err != nil {
				return fmt.Errorf("failed to disable plugin: %w", err)
			}

			formatter.Success(fmt.Sprintf("Plugin %s disabled", args[0]))
			return nil
		},
	}

	return cmd
}
