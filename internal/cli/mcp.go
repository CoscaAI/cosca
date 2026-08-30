package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
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

			engine := buildMCPServerEngine(cmd.Context())
			if engine == nil {
				return fmt.Errorf("mcp serve: falha ao montar dependências do runtime (cérebro indisponível)")
			}
			defer func() { _ = engine.Close() }()

			// O servidor MCP fala JSON-RPC 2.0 sobre stdio (NDJSON) — o mesmo
			// transporte que o cliente MCP (internal/chat/mcp) usa. O STDOUT é
			// território EXCLUSIVO do protocolo: nenhum log/banner pode ir pra
			// lá (regra ADR-028 §3 — qualquer ruído quebra o handshake JSON-RPC).
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
func buildMCPServerEngine(ctx context.Context) *mcpserver.Engine {
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
		DBPath:       filepath.Join(coscaDir, "knowledge.db"),
		RootDir:      workspace,
		AutoMigrate:  true,
		WatchEnabled: true,
	}); kErr == nil {
		if iErr := ke.Init(); iErr == nil {
			opts = append(opts, mcpserver.WithKnowledge(ke))

			// GAP P0 — Auto-reindexação (best-effort): watcher no workspace
			// reindexa create/modify/delete/rename sem reindexação manual.
			// Best-effort: um erro NUNCA pode derrubar o MCP — apenas logado.
			if wErr := ke.WatchDirectory(workspace); wErr != nil {
				log.Warn().Err(wErr).Str("dir", workspace).
					Msg("mcp serve: file watcher não iniciado — o índice dependerá de reindexação manual")
			}

			// GAP P1 — Loop de aprendizado (best-effort) em BACKGROUND.
			//
			// CORREÇÃO CRÍTICA (do tópico do Don): a indexação inicial estava no
			// caminho SÍNCRONO do boot, com timeout de initialIndexTimeout. Num
			// workspace grande (ex: Cosca com ~22k arquivos + knowledge.db de
			// ~368MB), o servidor ainda varria o diretório quando o host já havia
			// aberto mão (timeout de 30s no handshake initialize). Resultado:
			// "cosca Operation timed out after 30000ms".
			//
			// Agora a indexação roda numa GOROUTINE em background, ligada ao
			// ctx do comando. O Serve() nasce imediatamente, o handshake
			// initialize/tools-list responde em milissegundos, e o índice é
			// populado em paralelo — o watcher (P0) cobre mudanças ao vivo
			// enquanto isso. Best-effort: falha ou estouro de timeout apenas
			// loga; o MCP NUNCA trava o boot por causa da indexação.
			go indexKnowledgeOnBoot(ctx, ke, workspace)
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

// indexKnowledgeOnBoot popula o índice de conhecimento em BACKGROUND, fora do
// caminho crítico do handshake MCP. Roda numa goroutine ligada ao ctx do
// comando: se o comando termina (ctx cancelado), a indexação é interrompida —
// o servidor nunca fica órfão segurando o engine vivo.
//
// Cobre dois alvos best-effort:
//  1. O workspace inteiro (arquivos suportados entram no índice sem reindex
//     manual). Um workspace grande ou um provider de embeddings lento NÃO
//     bloqueia o boot: o timeout initialIndexTimeout limita a varredura e o
//     watcher (P0) cobre as mudanças ao vivo.
//  2. A raiz dos learnings de agente em .opencode/cosca/memory/agent/ — um
//     diretório OCULTO (prefixo ".") que IndexDirectory(workspace) pula.
//
// Best-effort: qualquer falha ou estouro apenas loga; o MCP segue vivo.
func indexKnowledgeOnBoot(ctx context.Context, ke *knowledge.Engine, workspace string) {
	// Indexa com limite por índice (initialIndexTimeout) + cancelamento do
	// comando (ctx): um embedding lento ou workspace gigante não pendura a
	// goroutine além do limite, e o comando encerrando derruba tudo.
	//
	// Índice 1 — workspace inteiro.
	idxCtx, idxCancel := context.WithTimeout(ctx, initialIndexTimeout)
	if idxErr := ke.IndexDirectory(idxCtx, workspace); idxErr != nil {
		log.Warn().Err(idxErr).Str("dir", workspace).
			Msg("mcp serve: indexação inicial falhou (best-effort) — recall dependerá do watcher")
	}
	idxCancel()

	// Índice 2 — learnings de agente (diretório oculto).
	if learningsDir := filepath.Join(workspace, ".opencode", "cosca", "memory", "agent"); dirExists(learningsDir) {
		lrCtx, lrCancel := context.WithTimeout(ctx, initialIndexTimeout)
		if lrErr := ke.IndexDirectory(lrCtx, learningsDir); lrErr != nil {
			log.Warn().Err(lrErr).Str("dir", learningsDir).
				Msg("mcp serve: indexação de learnings falhou (best-effort)")
		}
		lrCancel()
	}
}

// initialIndexTimeout limita a indexação best-effort na subida do MCP: de um
// workspace grande ou de um provider de embeddings lento, o servidor não pode
// esperar indefinidamente. Ao estourar, o MCP segue e o watcher (P0) cobre as
// mudanças ao vivo.
const initialIndexTimeout = time.Minute

// isTruthy interpreta uma string de ambiente como booleano (fail-closed).
func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

