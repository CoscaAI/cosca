package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/config"
)

// NewConfigCommand creates the `cosca config` command and its subcommands.
func NewConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Cosca configuration",
		Long: `Manage the Cosca configuration settings.

Allows reading, writing, listing, and editing configuration values.
Configuration is stored in .cosca/config.yaml or the global configuration file.

Subcommands:
  get <key>      Get a configuration value
  set <key> <val> Set a configuration value
  list           List all configuration values
  edit           Open configuration in editor
  reset          Reset configuration to defaults
  validate       Validate configuration
`,
		Example: `  cosca config get cache.ttl
  cosca config set cache.ttl 7200
  cosca config list
  cosca config edit
  cosca config reset
  cosca config validate`,
	}

	cmd.AddCommand(
		NewConfigGetCommand(),
		NewConfigSetCommand(),
		NewConfigListCommand(),
		NewConfigEditCommand(),
		NewConfigResetCommand(),
		NewConfigValidateCommand(),
	)

	return cmd
}

// getConfigDir returns the project directory to use for config resolution.
// Inside the jail, COSCA_PROJECT_DIR is set by the auto-jail to the original
// working directory, since os.Getwd() returns "/" inside the user namespace.
func getConfigDir() string {
	if dir := os.Getenv("COSCA_PROJECT_DIR"); dir != "" {
		return dir
	}
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

// loadConfig loads the Cosca configuration from the current project.
// If a project config file exists at .cosca/config.yaml, it is loaded and
// validated strictly — parse or validation errors are returned directly.
// If no project config exists, the fallback loader (user + env) is used.
func loadConfig() (*config.Config, error) {
	cfgPath := configPath()
	cfg, err := config.LoadFromFile(cfgPath)
	if err != nil {
		// Distinguish "file not found" from "file found but invalid".
		// If the file exists, its errors (parse/validation) must be surfaced.
		if _, statErr := os.Stat(cfgPath); statErr == nil {
			return nil, fmt.Errorf("failed to load config from %s: %w", cfgPath, err)
		}
		// File does not exist — fall back to the full loader (user + env).
		cfg2, err2 := config.Load()
		if err2 != nil {
			return config.DefaultConfig(), err2
		}
		return cfg2, nil
	}

	return cfg, nil
}

// configPath returns the path to the project-level config file.
func configPath() string {
	return filepath.Join(getConfigDir(), ".cosca", "config.yaml")
}

// applyKeyToConfig mapeia chaves de config (ex: "provider.name", "providers.deepseek.api_key")
// para os campos correspondentes na struct Config. Necessario porque cfg.Save() usa a struct,
// nao o viper. Chaves nao mapeadas sao ignoradas (viper ainda as segura).
func applyKeyToConfig(cfg *config.Config, key, value string) {
	switch {
	case key == "provider.name":
		cfg.Provider.Name = value
	case key == "provider.api_key":
		cfg.Provider.APIKey = value
	case key == "provider.model":
		cfg.Provider.Model = value
	case key == "provider.base_url":
		cfg.Provider.BaseURL = value
	case strings.HasPrefix(key, "providers."):
		// Ex: "providers.deepseek.api_key" ou "providers.deepseek.model"
		// Mapeia para os campos do struct Provider (singular)
		parts := strings.Split(key, ".")
		if len(parts) >= 3 {
			cfg.Provider.Name = parts[1] // "deepseek"
			field := parts[2]
			switch field {
			case "name":
				cfg.Provider.Name = value
			case "api_key":
				cfg.Provider.APIKey = value
			case "model":
				cfg.Provider.Model = value
			case "base_url":
				cfg.Provider.BaseURL = value
			case "max_tokens":
				if n, err := strconv.Atoi(value); err == nil {
					cfg.Provider.MaxTokens = n
				}
			case "temperature":
				if f, err := strconv.ParseFloat(value, 64); err == nil {
					cfg.Provider.Temperature = f
				}
			case "timeout":
				if d, err := time.ParseDuration(value); err == nil {
					cfg.Provider.Timeout = d
				}
			case "max_retries":
				if n, err := strconv.Atoi(value); err == nil {
					cfg.Provider.MaxRetries = n
				}
			case "rate_limit_per_min":
				if n, err := strconv.Atoi(value); err == nil {
					cfg.Provider.RateLimitPerMin = n
				}
			}
		}
	}
}

// setProviderEnvFromKey mapeia uma chave de config como "providers.deepseek.api_key"
// para a variavel de ambiente do provider (ex: DEEPSEEK_API_KEY) e seta imediatamente.
func setProviderEnvFromKey(key, value string) {
	// So trata chaves de provider: providers.<nome>.api_key
	if !strings.HasPrefix(key, "providers.") || !strings.HasSuffix(key, ".api_key") {
		return
	}
	parts := strings.Split(key, ".")
	if len(parts) < 3 {
		return
	}
	providerName := parts[1]
	if providerName == "" {
		return
	}
	envVar := strings.ToUpper(providerName) + "_API_KEY"
	os.Setenv(envVar, value)
}

// NewConfigGetCommand creates the `cosca config get` subcommand.
func NewConfigGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Long:  `Retrieve a specific configuration value by its key path.`,
		Example: `  cosca config get cache.ttl
  cosca config get provider.name
  cosca config get bootstrap.auto_init`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			key := args[0]
			v := config.GetViper()
			if v == nil {
				return fmt.Errorf("configuration system not initialized")
			}

			value := v.Get(key)
			if value == nil {
				return fmt.Errorf("key %q not found", key)
			}

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"key":   key,
					"value": value,
				})
			}

			formatter.KeyValue(key, fmt.Sprintf("%v", value))
			return nil
		},
	}

	return cmd
}

// NewConfigSetCommand creates the `cosca config set` subcommand.
func NewConfigSetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long:  `Set a specific configuration value by its key path. Supports nested keys with dot notation.`,
		Example: `  cosca config set cache.ttl 7200
  cosca config set provider.name "openai"
  cosca config set bootstrap.auto_init false`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)

			key := args[0]
			value := args[1]

			// 1. Tenta salvar no config do projeto (COSCA_PROJECT_DIR/.cosca/config.yaml)
			//    Se falhar (ex: dentro da jaula sem bind do projeto), cai no global.
			cfgPath := configPath()
			cfg, err := config.LoadFromFile(cfgPath)
			if err != nil {
				cfg = config.DefaultConfig()
				cfg.Paths.Project = getConfigDir()
				// ProjectDir nao existe dentro da jaula — usa config global
				if _, statErr := os.Stat(getConfigDir()); os.IsNotExist(statErr) || os.IsPermission(statErr) {
					cfgPath = filepath.Join(config.UserHomeDir(), ".config", "cosca", "config.yaml")
				}
			}

			// 2. Mapeia chaves conhecidas para a struct (o Save() usa a struct, nao o viper)
			applyKeyToConfig(cfg, key, value)

			// Use viper to set the value, then save the config
			v := config.GetViper()
			if v != nil {
				v.Set(key, value)
			}

			if err := cfg.Save(cfgPath); err != nil {
				return fmt.Errorf("failed to write config: %w", err)
			}

			// 3. Se a chave for de provider, seta a env var IMEDIATAMENTE
			//    pra funcionar no mesmo session (ex: provider test)
			setProviderEnvFromKey(key, value)

			formatter.Success(fmt.Sprintf("%s set to %s", key, value))
			return nil
		},
	}

	return cmd
}

// NewConfigListCommand creates the `cosca config list` subcommand.
func NewConfigListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all configuration values",
		Long:  `Display all current configuration values in a readable format.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			cfg, err := loadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if useJSON {
				return printJSON(cmd, cfg)
			}

			formatter.Header("Current Configuration")

			// Convert config to map[string]interface{} via YAML marshaling
			formatter.KeyValue("Version", cfg.Version)
			formatter.KeyValue("Profile", cfg.Profile)
			formatter.KeyValue("Mode", cfg.Mode)
			formatter.KeyValue("Verbose", fmt.Sprintf("%v", cfg.Verbose))
			formatter.Println("")
			formatter.Header("Paths")
			formatter.KeyValue("Home", cfg.Paths.Home)
			formatter.KeyValue("Project", cfg.Paths.Project)
			formatter.KeyValue("Data", cfg.Paths.Data)
			formatter.KeyValue("Cache", cfg.Paths.Cache)
			formatter.KeyValue("Logs", cfg.Paths.Logs)
			formatter.KeyValue("Plugins", cfg.Paths.Plugins)

			return nil
		},
	}

	return cmd
}

// NewConfigEditCommand creates the `cosca config edit` subcommand.
func NewConfigEditCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Open configuration in editor",
		Long:  `Open the Cosca configuration file in the default system editor.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

			cfgPath := configPath()

			if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
				return fmt.Errorf("configuration file not found at %s", cfgPath)
			}

			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = os.Getenv("VISUAL")
			}
			if editor == "" {
				editor = "vi"
			}

			formatter.Verbose(fmt.Sprintf("Opening %s with %s", cfgPath, editor))

			editorCmd := exec.Command(editor, cfgPath)
			editorCmd.Stdin = os.Stdin
			editorCmd.Stdout = os.Stdout
			editorCmd.Stderr = os.Stderr

			if err := editorCmd.Run(); err != nil {
				return fmt.Errorf("failed to open editor: %w", err)
			}

			formatter.Success("Configuration saved")
			return nil
		},
	}

	return cmd
}

// NewConfigResetCommand creates the `cosca config reset` subcommand.
func NewConfigResetCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset configuration to defaults",
		Long:  `Reset the Cosca configuration to factory defaults. All custom settings will be lost.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

			if !force {
				formatter.Warning("This will reset all configuration to defaults")
				formatter.Println("Use --force to confirm")
				return nil
			}

			cfgPath := configPath()

			defaultCfg := config.DefaultConfig()
			if err := defaultCfg.Save(cfgPath); err != nil {
				return fmt.Errorf("failed to reset config: %w", err)
			}

			formatter.Success("Configuration reset to defaults")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "confirm reset without prompting")
	return cmd
}

// NewConfigValidateCommand creates the `cosca config validate` subcommand.
func NewConfigValidateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration",
		Long:  `Validate the current configuration for correctness and completeness.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			cfg, err := loadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			validationErr := cfg.Validate()

			if useJSON {
				valid := validationErr == nil
				issues := []string{}
				if validationErr != nil {
					issues = append(issues, validationErr.Error())
				}
				return printJSON(cmd, map[string]interface{}{
					"valid":  valid,
					"issues": issues,
				})
			}

			if validationErr == nil {
				formatter.Success("Configuration is valid")
			} else {
				formatter.Errorf("Configuration is invalid: %v", validationErr)
			}

			return nil
		},
	}

	return cmd
}
