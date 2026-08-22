package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/chat"
	chatconfig "github.com/CoscaAI/cosca/internal/chat/config"
	"github.com/CoscaAI/cosca/internal/chat/executor"
	"github.com/CoscaAI/cosca/internal/chat/mcp"
	chatprovider "github.com/CoscaAI/cosca/internal/chat/provider"
	"github.com/CoscaAI/cosca/internal/chat/sandbox"
	"github.com/CoscaAI/cosca/internal/chat/tool"
	"github.com/CoscaAI/cosca/internal/engine"
	"github.com/CoscaAI/cosca/internal/execpolicy"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/models"
	"github.com/CoscaAI/cosca/internal/plugins"
	"github.com/CoscaAI/cosca/internal/skills"
)

// ─── Engine Builder ──────────────────────────────────────────────────────────
//
// Constrói o AgentEngine no binário único `cosca` (chat/exec/mcp consolidados
// aqui — o binário standalone cosca-chat foi removido). O registro de
// providers usa o mecanismo do binário `cosca` (chatprovider.RegisterChatProviders
// + seleção via chat registry) — providers lidos de env vars propagadas por
// cmd/cosca/main.go (propagateProviderEnv), com o provider "none"
// (determinístico) sempre registrado.

// buildEngine constructs an AgentEngine with all dependencies wired.
func buildEngine(modelName string) (*engine.AgentEngine, error) {
	return buildEngineWithMode(modelName, false)
}

// buildEngineWithMode constructs the engine. Quando allowNoProvider é true
// (modo interativo), o engine é construído mesmo sem provider — o Don
// configura o modelo dentro da interface. O `cosca exec` usa false.
func buildEngineWithMode(modelName string, allowNoProvider bool) (*engine.AgentEngine, error) {
	workspace, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get workspace: %w", err)
	}
	coscaDir := getCoscaDir(workspace)

	cfg, err := chatconfig.Load(".")
	if err != nil {
		log.Warn().Err(err).Msg("failed to load chat config, using defaults")
		d := chatconfig.DefaultConfig()
		cfg = &d
	}

	// ── 1. Tool Registry & Tool Registration ─────────────────────────────
	toolRegistry := tool.NewRegistry()
	rails := sandbox.NewRails(workspace)

	sandboxMode := parseSandboxMode(cfg.Sandbox.Mode)
	sbGate := sandbox.NewGate(workspace, sandboxMode)

	// Register filesystem tools
	toolRegistry.Register(tool.NewReadTool(workspace, rails))
	toolRegistry.Register(tool.NewWriteTool(workspace, rails))
	toolRegistry.Register(tool.NewEditTool(workspace, rails))
	toolRegistry.Register(tool.NewGlobTool(workspace, rails))

	// Register shell tool (needs sandbox). Se uma exec policy estiver
	// configurada (exec_policy em .cosca/config.yaml), o shell tool avalia
	// cada comando contra ela antes de executar. Sem policy = comportamento
	// atual.
	var shellPolicy *execpolicy.Policy
	if cfg.ExecPolicy != "" {
		shellPolicy, err = execpolicy.Load(cfg.ExecPolicy)
		if err != nil {
			return nil, fmt.Errorf("load exec policy %q: %w", cfg.ExecPolicy, err)
		}
		log.Info().Str("path", cfg.ExecPolicy).Int("rules", len(shellPolicy.Rules)).
			Msg("exec policy loaded")
	}
	toolRegistry.Register(tool.NewShellTool(workspace, sbGate, tool.WithExecPolicy(shellPolicy)))

	// Register search tool
	toolRegistry.Register(tool.NewSearchTool(workspace, rails, sbGate))

	// Register git tool
	toolRegistry.Register(tool.NewGitTool(workspace, sbGate))

	// Register build tool
	toolRegistry.Register(tool.NewBuildTool(workspace, sbGate))

	// Register test tool
	toolRegistry.Register(tool.NewTestTool(workspace, sbGate))

	// Register MCP tools from configured servers
	if cfg.MCP.Servers != nil {
		for _, server := range cfg.MCP.Servers {
			mcpClient := mcp.NewStdioClient(server.Command, server.Args, server.Env, sbGate)
			connectCtx, connectCancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := mcpClient.Connect(connectCtx); err != nil {
				log.Warn().Err(err).Str("server", server.Name).Str("command", server.Command).
					Msg("failed to connect to MCP server, skipping")
				connectCancel()
				continue
			}
			connectCancel()

			discCtx, discCancel := context.WithTimeout(context.Background(), 10*time.Second)
			mcpTools, err := mcpClient.DiscoverTools(discCtx)
			discCancel()
			if err != nil {
				log.Warn().Err(err).Str("server", server.Name).
					Msg("failed to discover MCP tools, closing connection")
				mcpClient.Close()
				continue
			}

			for _, mt := range mcpTools {
				wrapped := tool.NewMCPTool(mt, mcpClient)
				toolRegistry.Register(wrapped)
				log.Debug().Str("tool", wrapped.Name()).Str("server", server.Name).
					Msg("registered MCP tool")
			}
		}
	}

	// ── 1b. Plugin Loading & Tool Registration ───────────────────────────
	hookRegistry := plugins.NewHookRegistry(30 * time.Second)

	pluginDir := filepath.Join(coscaDir, "plugins")
	pluginLoader := plugins.NewLoader(plugins.DefaultLoaderConfig(pluginDir))
	loadedPlugins, err := pluginLoader.LoadAll(pluginDir)
	if err != nil {
		log.Warn().Err(err).Msg("failed to load plugins from disk")
	} else {
		for _, p := range loadedPlugins {
			wrapped := tool.NewPluginTool(p)
			toolRegistry.Register(wrapped)
			log.Debug().Str("tool", wrapped.Name()).Str("plugin", p.ID()).Msg("registered plugin tool")
		}
	}

	log.Debug().Strs("tools", toolRegistry.List()).Msg("tools registered")

	// ── 2. Sandbox Gate & Executor ───────────────────────────────────────
	exec := executor.New(toolRegistry, sbGate, workspace)

	// ── 3. Agent Registry ────────────────────────────────────────────────
	agentReg := engine.NewAgentRegistry()
	if err := agentReg.LoadDefault(); err != nil {
		log.Warn().Err(err).Msg("no agent definitions loaded, using defaults")
	}
	log.Debug().Int("agents", len(agentReg.List())).Msg("agents loaded")

	// ── 4. Context Builder ───────────────────────────────────────────────
	// Use a janela de contexto do modelo. Prefere o REAL context_length do
	// cache models.dev quando o modelo é conhecido lá (o cache é uma melhoria:
	// se faltar/estiver obsoleto, caímos no valor configurado e depois no
	// default de 128K do engine).
	contextLimit := engine.DefaultMaxTokens
	providerCfg := cfg.ResolveProviderConfig(modelName)
	if providerCfg != nil {
		if cl := models.ContextWindowFor(models.DefaultCachePath(), providerCfg.Model); cl > 0 {
			contextLimit = int(cl)
		} else if providerCfg.ContextWindow > 0 {
			contextLimit = providerCfg.ContextWindow
		}
	}
	ctxBuilder := engine.NewContextBuilder(agentReg, contextLimit)

	// ── 4b. Skills Loading ───────────────────────────────────────────────
	skillMgr := skills.NewManager(coscaDir)
	skillList := skillMgr.List()
	skillNames := make([]string, 0, len(skillList))
	for _, s := range skillList {
		skillNames = append(skillNames, s.Name)
	}
	ctxBuilder.SetSkills(skillNames)
	log.Debug().Int("skills", len(skillNames)).Msg("skills loaded")

	// ── 5. Router ────────────────────────────────────────────────────────
	router := engine.NewRouter(agentReg)

	// ── 6. Chat Provider ─────────────────────────────────────────────────
	// Usa o MESMO mecanismo de registro do binário `cosca`
	// (internal/cli/chat.go): chatprovider.RegisterChatProviders lê env vars
	// propagadas por cmd/cosca/main.go (propagateProviderEnv) e o provider
	// "none" (modo determinístico) é sempre registrado.
	reg := chat.GetRegistry()
	if err := chatprovider.RegisterChatProviders(reg, nil); err != nil {
		log.Warn().Err(err).Msg("failed to register chat providers")
	}

	var chatProvider chat.ChatProvider
	if modelName != "" {
		if p, ok := reg.Get(modelName); ok {
			chatProvider = p
		} else {
			// `--model` é o NOME DO MODELO (ex. qwen2.5-coder:14b), não o
			// provider. O registry só conhece providers (ollama, none, ...),
			// então um Get() por modelo sempre falha. Propaga o modelo para a
			// env var do provider local (ollama) ANTES da seleção — sem isso o
			// provider cai no default "llama3" e toda execução falha
			// silenciosamente (llama3 not found).
			log.Debug().Str("model", modelName).Msg("model name is not a provider — propagating to local provider env")
			os.Setenv("COSCA_OLLAMA_MODEL", modelName)
		}
	}
	if chatProvider == nil {
		if err := reg.Select(context.Background(), chat.DefaultChatRegistryConfig()); err != nil {
			if allowNoProvider {
				// Modo interativo: abre mesmo sem provider — o Don
				// configura o modelo depois.
				log.Debug().Msg("no provider available, starting with unconfigured provider")
				chatProvider = &unconfiguredChatProvider{}
			} else {
				return nil, fmt.Errorf("no LLM provider available: %w", err)
			}
		} else {
			chatProvider = reg
		}
	}

	// O NullProvider ("none") é sempre registrado, então um Select() descoberto
	// pode retorná-lo como último recurso mesmo sem provider real configurado.
	// Só honramos o modo determinístico quando pedido explicitamente
	// (--model none ou providers.primary=none); caso contrário reportamos o
	// problema real exatamente como antes.
	if chatProvider != nil && chatProvider.Name() == "none" && modelName != "none" && cfg.Provider.Primary != "none" {
		if allowNoProvider {
			log.Debug().Msg("no real provider available, starting with unconfigured provider")
			chatProvider = &unconfiguredChatProvider{}
		} else {
			return nil, fmt.Errorf("no LLM provider available: configure a provider or run with --model none (deterministic mode)")
		}
	}

	// Modo determinístico (--model none ou providers.primary=none): o
	// NullProvider emite seu erro pt-BR claro como ChatEventError. Em
	// execuções não-interativas, superfície esse erro logo de cara para o
	// Don receber a mensagem acionável em vez de uma resposta em branco.
	if chatProvider != nil && chatProvider.Name() == "none" && !allowNoProvider {
		if err := deterministicModeError(chatProvider); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("deterministic mode: no cognitive capability")
	}

	// Determina o identificador real do modelo para este provider.
	modelID := chatProvider.Model()

	// ── 7. Session Manager ───────────────────────────────────────────────
	sessionMgr := engine.NewSessionManagerDefault()

	// ── 8. Engine Config ─────────────────────────────────────────────────
	engineCfg := engine.EngineConfig{
		Model:       modelID,
		MaxTurns:    cfg.Session.MaxTurns,
		Temperature: engine.DefaultTemperature,
		Ephemeral:   !cfg.Session.AutoSave,
	}
	if engineCfg.MaxTurns <= 0 {
		engineCfg.MaxTurns = 100
	}

	// ── 9. Memory & Knowledge ────────────────────────────────────────────
	var memRetriever engine.MemoryRetriever
	var memStorer engine.MemoryStorer
	var knowSearch engine.KnowledgeSearcher

	memEngine, err := memory.NewEngine(
		memory.WithLogger(log.Logger),
		memory.WithConfig(memory.EngineConfig{
			DataDir: coscaDir,
		}),
	)
	if err != nil {
		log.Warn().Err(err).Msg("failed to initialize memory engine, memory disabled")
	} else {
		memAdapter := engine.NewMemoryEngineAdapter(memEngine)
		memRetriever = memAdapter
		memStorer = memAdapter
		log.Debug().Msg("memory engine initialized")
	}

	knowEngine, err := knowledge.New(knowledge.Config{
		DBPath:      filepath.Join(coscaDir, "knowledge.db"),
		RootDir:     workspace,
		AutoMigrate: true,
	})
	if err != nil {
		log.Warn().Err(err).Msg("failed to initialize knowledge engine, knowledge disabled")
	} else {
		if err := knowEngine.Init(); err != nil {
			log.Warn().Err(err).Msg("failed to init knowledge engine, knowledge disabled")
		} else {
			knowSearch = engine.NewKnowledgeAdapter(knowEngine)
			log.Debug().Msg("knowledge engine initialized")
		}
	}

	// ── 10. Agent Engine ─────────────────────────────────────────────────
	eng := engine.NewAgentEngine(
		agentReg,
		ctxBuilder,
		router,
		chatProvider,
		exec,
		engineCfg,
		memRetriever,
		memStorer,
		knowSearch,
	)
	eng.WithSessionManager(sessionMgr)
	eng.WithPlugins(hookRegistry)

	return eng, nil
}

// deterministicModeError drains the given provider's Chat and returns its
// terminal error, or nil if the provider has none. Usado para superfície do
// NullProvider: a mensagem "capacidade cognitiva indisponível..." vem
// exatamente como definida em internal/chat/provider/null.go.
func deterministicModeError(p chat.ChatProvider) error {
	if p == nil {
		return nil
	}
	_, err := p.Chat(context.Background(), []chat.Message{}, chat.ChatOptions{})
	if err != nil {
		return err
	}
	return nil
}

// unconfiguredChatProvider é o provider usado quando a interface abre sem
// modelo configurado. Qualquer tentativa de conversa retorna erro amigável.
// Não é usado pelo `cosca exec` (que usa buildEngine com allowNoProvider=false).
type unconfiguredChatProvider struct{}

func (p *unconfiguredChatProvider) Name() string { return "unconfigured" }

func (p *unconfiguredChatProvider) Model() string { return "nenhum — configure via cosca config" }

func (p *unconfiguredChatProvider) Close() error { return nil }

func (p *unconfiguredChatProvider) Chat(ctx context.Context, _ []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
	return nil, fmt.Errorf("nenhum modelo configurado: configure um provider via `cosca config`")
}

func (p *unconfiguredChatProvider) ChatStream(ctx context.Context, _ []chat.Message, _ chat.ChatOptions) (chat.ChatStream, error) {
	return nil, fmt.Errorf("nenhum modelo configurado: configure um provider via `cosca config`")
}

// parseSandboxMode converts a config string to a chat.SandboxMode value.
func parseSandboxMode(mode string) chat.SandboxMode {
	switch strings.ToLower(mode) {
	case "read-only", "readonly":
		return chat.SandboxReadOnly
	case "workspace":
		return chat.SandboxWorkspace
	case "full":
		return chat.SandboxFull
	default:
		return chat.SandboxWorkspace
	}
}

// getCoscaDir returns the path to the .cosca data directory within the workspace.
func getCoscaDir(workspace string) string {
	return filepath.Join(workspace, ".cosca")
}
