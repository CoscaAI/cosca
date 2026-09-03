package cli

import (
	"bytes"
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
	"github.com/CoscaAI/cosca/internal/cost"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/mcpserver"
	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/runtime"
	"github.com/CoscaAI/cosca/internal/trace"
	"github.com/CoscaAI/cosca/internal/vision"
)

// â”€â”€â”€ Feature Gate â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
//
// O `cosca mcp` vive no binÃ¡rio Ãºnico `cosca` (o standalone cosca-chat foi
// removido) e estÃ¡ DESATIVADO POR PADRÃƒO (regra do Don). Sem COSCA_ENABLE_MCP=1/true
// o comando nunca executa trabalho (fail-closed).

// mcpEnabled retorna erro fail-closed quando o feature gate estÃ¡ fechado.
func mcpEnabled() error {
	v := strings.ToLower(os.Getenv("COSCA_ENABLE_MCP"))
	if v != "1" && v != "true" {
		return fmt.Errorf("comando desativado por padrÃ£o: defina COSCA_ENABLE_MCP=1 para habilitar 'cosca mcp'")
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
				fmt.Printf("  â”€ %s\n", s.Name)
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
			fmt.Printf("âœ“ Added MCP server %q\n", name)
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
			fmt.Printf("âœ“ Removed MCP server %q\n", name)
			return nil
		},
	}
	mcpCmd.AddCommand(mcpRemoveCmd)

	mcpServeCmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the Cosca MCP stdio server (cognitivas tools)",
		Long: `Serve the Cosca MCP stdio server (JSON-RPC 2.0 over stdio/NDJSON).

The COSCA is a consultable brain â€” NOT a runtime inside the OpenCode. This
launches the MCP SERVER (the "sistema nervoso") that exposes the cognitive
tools: cosca.recall, cosca.context, cosca.learn, cosca.observe, cosca.reason,
cosca.trace, cosca.project.

Each tool returns an honest context packet ({query, context[{content,source,
relevance}], confidence, trace_id}) built from the Knowledge/Memory/Runtime
of the current workspace (.cosca). Zero-LLM: confidence is derived
deterministically from the epistemic class + relevance.

Read-only-first: cosca.learn is the ONLY write and is GATED â€” it fails closed
unless COSCA_MCP_ALLOW_WRITE=1 is set.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := mcpEnabled(); err != nil {
				return err
			}

			engine := buildMCPServerEngine(cmd.Context())
			if engine == nil {
				return fmt.Errorf("mcp serve: falha ao montar dependÃªncias do runtime (cÃ©rebro indisponÃ­vel)")
			}
			defer func() { _ = engine.Close() }()

			// O servidor MCP fala JSON-RPC 2.0 sobre stdio (NDJSON) â€” o mesmo
			// transporte que o cliente MCP (internal/chat/mcp) usa. O STDOUT Ã©
			// territÃ³rio EXCLUSIVO do protocolo: nenhum log/banner pode ir pra
			// lÃ¡ (regra ADR-028 Â§3 â€” qualquer ruÃ­do quebra o handshake JSON-RPC).
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

// findProjectRootWithFallback resolve a raiz do projeto a partir do cwd,
// subindo a árvore até achar um diretório `.cosca/` ou um marcador de projeto
// (go.mod/package.json/.git). Se nada for encontrado, cai no cwd literal —
// mas **nunca** cria um `.cosca` em subpasta: um `os.Getwd()` cego era a causa
// dos `.cosca` órfãos (ex: `internal\cli\.cosca`, `bin\.cosca`, `.cosca\.cosca`).
func findProjectRootWithFallback() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if root, err := findProjectRoot(wd); err == nil {
		return root, nil
	}
	// Fallback: usa o primeiro marcador de projeto subindo a árvore (mesma
	// heurística do resolveKnowledgeDBPath do compute), ou o cwd se nada existir.
	dir := wd
	for {
		for _, marker := range []string{"go.mod", "package.json", ".git", "Cargo.toml", "pyproject.toml"} {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return wd, nil
		}
		dir = parent
	}
}

// buildMCPServerEngine monta o Engine do servidor MCP ligado ao runtime atual:
// knowledge, memory, runtime (corpo), kernel (cÃ©rebro / kill-switch), trace
// (flight recorder) e a percepÃ§Ã£o determinÃ­stica (vision). Nil-safe e
// best-effort: um Ã³rgÃ£o que falha ao inicializar Ã© apenas omitido â€” as tools
// que dele dependem devolvem erro claro em vez de pÃ¢nico.
func buildMCPServerEngine(ctx context.Context) *mcpserver.Engine {
	// Raiz do projeto: sobe a árvore até achar `.cosca/` (ou um marcador de
	// projeto) em vez de usar o `os.Getwd()` cego. O cwd pode ser uma subpasta
	// (ex: `internal\cli`), e um `Getwd()` literal criaria um `.cosca` fantasma
	// ali (reprodução do bug dos .cosca órfãos). Reusa a mesma convenção do
	// `findProjectRoot` do pacote: subir até a raiz do projeto.
	workspace, err := findProjectRootWithFallback()
	if err != nil {
		return nil
	}
	coscaDir := filepath.Join(workspace, ".cosca")

	var opts []mcpserver.Option

	// Corpo (runtime) + cÃ©rebro (kernel kill-switch).
	rtCfg := runtime.DefaultRuntimeConfig()
	rtCfg.DataDir = coscaDir
	opts = append(opts,
		mcpserver.WithRuntime(runtime.New(runtime.WithConfig(rtCfg))),
		mcpserver.WithKernel(kernel.NewEmergencyManager()),
		mcpserver.WithAllowWrite(isTruthy(os.Getenv("COSCA_MCP_ALLOW_WRITE"))),
	)

	// Conhecimento (Ã³rgÃ£o de busca semÃ¢ntica).
	if ke, kErr := knowledge.New(knowledge.Config{
		DBPath:       filepath.Join(coscaDir, "knowledge.db"),
		RootDir:      workspace,
		AutoMigrate:  true,
		WatchEnabled: true,
	}); kErr == nil {
		if iErr := ke.Init(); iErr == nil {
			opts = append(opts, mcpserver.WithKnowledge(ke))

			// GAP P0 â€” Auto-reindexaÃ§Ã£o (best-effort): watcher no workspace
			// reindexa create/modify/delete/rename sem reindexaÃ§Ã£o manual.
			// Best-effort: um erro NUNCA pode derrubar o MCP â€” apenas logado.
			if wErr := ke.WatchDirectory(workspace); wErr != nil {
				log.Warn().Err(wErr).Str("dir", workspace).
					Msg("mcp serve: file watcher nÃ£o iniciado â€” o Ã­ndice dependerÃ¡ de reindexaÃ§Ã£o manual")
			}

			// GAP P1 â€” Loop de aprendizado (best-effort) em BACKGROUND.
			//
			// CORREÃ‡ÃƒO CRÃTICA (do tÃ³pico do Don): a indexaÃ§Ã£o inicial estava no
			// caminho SÃNCRONO do boot, com timeout de initialIndexTimeout. Num
			// workspace grande (ex: Cosca com ~22k arquivos + knowledge.db de
			// ~368MB), o servidor ainda varria o diretÃ³rio quando o host jÃ¡ havia
			// aberto mÃ£o (timeout de 30s no handshake initialize). Resultado:
			// "cosca Operation timed out after 30000ms".
			//
			// Agora a indexaÃ§Ã£o roda numa GOROUTINE em background, ligada ao
			// ctx do comando. O Serve() nasce imediatamente, o handshake
			// initialize/tools-list responde em milissegundos, e o Ã­ndice Ã©
			// populado em paralelo â€” o watcher (P0) cobre mudanÃ§as ao vivo
			// enquanto isso. Best-effort: falha ou estouro de timeout apenas
			// loga; o MCP NUNCA trava o boot por causa da indexaÃ§Ã£o.
			go indexKnowledgeOnBoot(ctx, ke, workspace)
		}
	}

	// MemÃ³ria (Ã³rgÃ£o de aprendizado).
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

	// Custo (ADR-031) â€” tool cosca.cost (Ãºtil work / tokens por agente/task).
	opts = append(opts, mcpserver.WithCost(cost.ForCoscaDir(coscaDir)))

	// Operar o CLI (allowlist segura de comandos de leitura/gestÃ£o).
	opts = append(opts, mcpserver.WithCLIExec(NewCLIExecFunc()))

	// PercepÃ§Ã£o determinÃ­stica frame-a-frame.
	opts = append(opts, mcpserver.WithVision(vision.AnalyzeVideo))

	return mcpserver.NewEngine(opts...)
}

// indexKnowledgeOnBoot popula o Ã­ndice de conhecimento em BACKGROUND, fora do
// caminho crÃ­tico do handshake MCP. Roda numa goroutine ligada ao ctx do
// comando: se o comando termina (ctx cancelado), a indexaÃ§Ã£o Ã© interrompida â€”
// o servidor nunca fica Ã³rfÃ£o segurando o engine vivo.
//
// Cobre dois alvos best-effort:
//  1. O workspace inteiro (arquivos suportados entram no Ã­ndice sem reindex
//     manual). Um workspace grande ou um provider de embeddings lento NÃƒO
//     bloqueia o boot: o timeout initialIndexTimeout limita a varredura e o
//     watcher (P0) cobre as mudanÃ§as ao vivo.
//  2. A raiz dos learnings de agente em .opencode/cosca/memory/agent/ â€” um
//     diretÃ³rio OCULTO (prefixo ".") que IndexDirectory(workspace) pula.
//
// Best-effort: qualquer falha ou estouro apenas loga; o MCP segue vivo.
func indexKnowledgeOnBoot(ctx context.Context, ke *knowledge.Engine, workspace string) {
	// Indexa com limite por Ã­ndice (initialIndexTimeout) + cancelamento do
	// comando (ctx): um embedding lento ou workspace gigante nÃ£o pendura a
	// goroutine alÃ©m do limite, e o comando encerrando derruba tudo.
	//
	// Ãndice 1 â€” workspace inteiro.
	idxCtx, idxCancel := context.WithTimeout(ctx, initialIndexTimeout)
	if idxErr := ke.IndexDirectory(idxCtx, workspace); idxErr != nil {
		log.Warn().Err(idxErr).Str("dir", workspace).
			Msg("mcp serve: indexaÃ§Ã£o inicial falhou (best-effort) â€” recall dependerÃ¡ do watcher")
	}
	idxCancel()

	// Ãndice 2 â€” learnings de agente (diretÃ³rio oculto).
	if learningsDir := filepath.Join(workspace, ".opencode", "cosca", "memory", "agent"); dirExists(learningsDir) {
		lrCtx, lrCancel := context.WithTimeout(ctx, initialIndexTimeout)
		if lrErr := ke.IndexDirectory(lrCtx, learningsDir); lrErr != nil {
			log.Warn().Err(lrErr).Str("dir", learningsDir).
				Msg("mcp serve: indexaÃ§Ã£o de learnings falhou (best-effort)")
		}
		lrCancel()
	}
}

// initialIndexTimeout limita a indexaÃ§Ã£o best-effort na subida do MCP: de um
// workspace grande ou de um provider de embeddings lento, o servidor nÃ£o pode
// esperar indefinidamente. Ao estourar, o MCP segue e o watcher (P0) cobre as
// mudanÃ§as ao vivo.
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


// NewCLIExecFunc cria a função que o MCP usa para "operar o CLI" com allowlist
// segura (read-only/gestão). Roda o NewRootCommand com os args e captura o
// stdout. Somente comandos permitidos pelo mcpserver.allowedCLIRoot passam;
// a allowlist é aplicada no MCP (default-deny) — esta função é o executor.
func NewCLIExecFunc() mcpserver.CLIExecFunc {
	return func(args []string) (string, error) {
		root := NewRootCommand()
		var buf bytes.Buffer
		root.SetOut(&buf)
		root.SetErr(&buf)
		root.SetArgs(args)
		if err := root.ExecuteContext(context.Background()); err != nil {
			// Comandos de erro conhecido escrevem no buffer; devolvemos o
			// buffer + erro para o MCP reportar com honestidade.
			return buf.String(), fmt.Errorf("%w", err)
		}
		return buf.String(), nil
	}
}
