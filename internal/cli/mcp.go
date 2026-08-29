package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/chat/config"
	"github.com/CoscaAI/cosca/internal/kernel"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/mcpserver"
	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/runtime"
	"github.com/CoscaAI/cosca/internal/trace"
	"github.com/CoscaAI/cosca/internal/vision"
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

	mcpServeCmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the Cosca MCP stdio server (cognitivas tools)",
		Long: `Serve the Cosca MCP stdio server (JSON-RPC 2.0 over stdio/NDJSON).

The COSCA is a consultable brain — NOT a runtime inside the OpenCode. This
launches the MCP SERVER (the "sistema nervoso") that exposes the cognitive
tools: cosca.recall, cosca.context, cosca.learn, cosca.observe, cosca.reason,
cosca.trace, cosca.project.

Each tool returns an honest context packet ({query, context[{content,source,
relevance}], confidence, trace_id}) built from the Knowledge/Memory/Runtime
of the current workspace (.cosca). Zero-LLM: confidence is derived
deterministically from the epistemic class + relevance.

Read-only-first: cosca.learn is the ONLY write and is GATED — it fails closed
unless COSCA_MCP_ALLOW_WRITE=1 is set.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := mcpEnabled(); err != nil {
				return err
			}

			engine := buildMCPServerEngine()
			if engine == nil {
				return fmt.Errorf("mcp serve: falha ao montar dependências do runtime (cérebro indisponível)")
			}
			defer func() { _ = engine.Close() }()

			// O servidor MCP fala JSON-RPC 2.0 sobre stdio (NDJSON) — o mesmo
			// transporte que o cliente MCP (internal/chat/mcp) usa.
			srv := mcpserver.NewServer(engine, os.Stdin, os.Stdout)
			if err := srv.Serve(cmd.Context()); err != nil {
				return fmt.Errorf("mcp serve: %w", err)
			}
			return nil
		},
	}
	mcpCmd.AddCommand(mcpServeCmd)

	return mcpCmd
}

// buildMCPServerEngine monta o Engine do servidor MCP ligado ao runtime atual:
// knowledge, memory, runtime (corpo), kernel (cérebro / kill-switch), trace
// (flight recorder) e a percepção determinística (vision). Nil-safe e
// best-effort: um órgão que falha ao inicializar é apenas omitido — as tools
// que dele dependem devolvem erro claro em vez de pânico.
func buildMCPServerEngine() *mcpserver.Engine {
	workspace, err := os.Getwd()
	if err != nil {
		return nil
	}
	coscaDir := filepath.Join(workspace, ".cosca")

	var opts []mcpserver.Option

	// Corpo (runtime) + cérebro (kernel kill-switch).
	rtCfg := runtime.DefaultRuntimeConfig()
	rtCfg.DataDir = coscaDir
	opts = append(opts,
		mcpserver.WithRuntime(runtime.New(runtime.WithConfig(rtCfg))),
		mcpserver.WithKernel(kernel.NewEmergencyManager()),
		mcpserver.WithAllowWrite(isTruthy(os.Getenv("COSCA_MCP_ALLOW_WRITE"))),
	)

	// Conhecimento (órgão de busca semântica).
	if ke, kErr := knowledge.New(knowledge.Config{
		DBPath:      filepath.Join(coscaDir, "knowledge.db"),
		RootDir:     workspace,
		AutoMigrate: true,
	}); kErr == nil {
		if iErr := ke.Init(); iErr == nil {
			opts = append(opts, mcpserver.WithKnowledge(ke))
		}
	}

	// Memória (órgão de aprendizado).
	if me, mErr := memory.NewEngine(memory.WithConfig(memory.EngineConfig{
		DataDir:   coscaDir,
		AutoPrune: false,
	})); mErr == nil {
		opts = append(opts, mcpserver.WithMemory(me))
	}

	// Trace (flight recorder append-only).
	if ts, tErr := trace.NewStore(filepath.Join(coscaDir, "trace.db")); tErr == nil {
		opts = append(opts, mcpserver.WithTrace(ts))
	}

	// Percepção determinística frame-a-frame.
	opts = append(opts, mcpserver.WithVision(vision.AnalyzeVideo))

	return mcpserver.NewEngine(opts...)
}

// isTruthy interpreta uma string de ambiente como booleano (fail-closed).
func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

