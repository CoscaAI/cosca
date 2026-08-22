package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/chat/config"
)

// ─── Feature Gate ────────────────────────────────────────────────────────────
//
// O `cosca mcp` vive no binário único `cosca` (o standalone cosca-chat foi
// removido) e está DESATIVADO POR PADRÃO (regra do Don). Sem COSCA_ENABLE_MCP=1/true
// o comando nunca executa trabalho (fail-closed).

// mcpEnabled retorna erro fail-closed quando o feature gate está fechado.
func mcpEnabled() error {
	v := strings.ToLower(os.Getenv("COSCA_ENABLE_MCP"))
	if v != "1" && v != "true" {
		return fmt.Errorf("comando desativado por padrão: defina COSCA_ENABLE_MCP=1 para habilitar 'cosca mcp'")
	}
	return nil
}

// NewMCPCommand creates the `cosca mcp` command group.
func NewMCPCommand() *cobra.Command {
	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "Manage MCP servers",
		Long:  `Manage Model Context Protocol servers for external tool integration.`,
	}

	mcpListCmd := &cobra.Command{
		Use:   "list",
		Short: "List configured MCP servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := mcpEnabled(); err != nil {
				return err
			}

			cfg, err := config.Load(".")
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if len(cfg.MCP.Servers) == 0 {
				fmt.Println("No MCP servers configured.")
				return nil
			}
			fmt.Printf("Configured MCP servers (%d):\n", len(cfg.MCP.Servers))
			for _, s := range cfg.MCP.Servers {
				fmt.Printf("  ─ %s\n", s.Name)
				if s.Command != "" {
					fmt.Printf("    Command: %s %s\n", s.Command, strings.Join(s.Args, " "))
				}
				if s.URL != "" {
					fmt.Printf("    URL: %s\n", s.URL)
				}
			}
			return nil
		},
	}
	mcpCmd.AddCommand(mcpListCmd)

	mcpAddCmd := &cobra.Command{
		Use:   "add <name> <command> [args...]",
		Short: "Add an MCP server (stdio transport)",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := mcpEnabled(); err != nil {
				return err
			}

			cfg, err := config.Load(".")
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			name := args[0]
			command := args[1]
			var extraArgs []string
			if len(args) > 2 {
				extraArgs = args[2:]
			}

			// Check for duplicate
			for _, s := range cfg.MCP.Servers {
				if s.Name == name {
					return fmt.Errorf("MCP server %q already exists", name)
				}
			}

			cfg.MCP.Servers = append(cfg.MCP.Servers, config.MCPServerConf{
				Name:    name,
				Command: command,
				Args:    extraArgs,
			})

			if err := config.Save(".", cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
			fmt.Printf("✓ Added MCP server %q\n", name)
			return nil
		},
	}
	mcpCmd.AddCommand(mcpAddCmd)

	mcpRemoveCmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an MCP server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := mcpEnabled(); err != nil {
				return err
			}

			cfg, err := config.Load(".")
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			name := args[0]
			found := false
			filtered := make([]config.MCPServerConf, 0, len(cfg.MCP.Servers))
			for _, s := range cfg.MCP.Servers {
				if s.Name == name {
					found = true
					continue
				}
				filtered = append(filtered, s)
			}
			if !found {
				return fmt.Errorf("MCP server %q not found", name)
			}

			cfg.MCP.Servers = filtered
			if err := config.Save(".", cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
			fmt.Printf("✓ Removed MCP server %q\n", name)
			return nil
		},
	}
	mcpCmd.AddCommand(mcpRemoveCmd)

	return mcpCmd
}
