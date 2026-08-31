package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/adapter"
	"github.com/CoscaAI/cosca/internal/agents"
	"github.com/CoscaAI/cosca/internal/chat"
	chatprovider "github.com/CoscaAI/cosca/internal/chat/provider"
	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/orchestration"
	"github.com/CoscaAI/cosca/internal/pipeline"
	"github.com/CoscaAI/cosca/internal/skills"
	"github.com/rs/zerolog/log"
)

// ─── Chat Session State ──────────────────────────────────────────────────────

// chatSession holds mutable state for an interactive chat session.
type chatSession struct {
	engine       pipeline.Runner
	agentName    string
	providerName string
	modelName    string
	streamMode   bool
	showMetrics  bool
	history      []chat.Message
	running      bool
}

// ─── Chat Command ────────────────────────────────────────────────────────────

// NewChatCommand creates the `cosca chat` interactive command.
func NewChatCommand() *cobra.Command {
	var (
		agentName    string
		providerName string
		modelName    string
		streamMode   bool
		showMetrics  bool
	)

	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Start an interactive AI chat session",
		Long: `Start an interactive chat session with the Cosca AI Orchestration Engine.

Type your prompts and the engine will route them to the best agent, 
search relevant knowledge, and return responses. 

Commands available during the session:
  /agent <name>     — switch to a specific agent
  /provider <name>  — switch LLM provider  
  /model <name>     — switch model
  /stream           — toggle streaming mode
  /metrics          — show session metrics
  /history          — show conversation history
  /clear            — clear conversation history
  /help             — show available commands
  /exit, /quit      — end the session

Examples:
  cosca chat
  cosca chat --agent "Backend Chief"
  cosca chat --stream --metrics`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			// Handle Ctrl+C gracefully.
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				cancel()
			}()

			// 1. Resolve Cosca directory.
			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			// 2. Initialize subsystems.
			agentMgr := agents.NewManager(coscaDir)
			skillMgr := skills.NewManager(coscaDir)

			agentResolver := adapter.NewAgentResolverAdapter(agentMgr)
			skillResolver := adapter.NewSkillResolverAdapter(skillMgr)

			var memRetriever orchestration.MemoryRetriever
			var memStorer orchestration.MemoryStorer

			memEngine, err := memory.NewEngine(
				memory.WithConfig(memory.EngineConfig{
					DataDir:            coscaDir,
					DefaultTTL:         24 * time.Hour,
					MaxRecordsPerLayer: 1000,
					AutoPrune:          true,
					PruneInterval:      30 * time.Minute,
				}),
			)
			if err == nil {
				defer func() { _ = memEngine.Close() }()
				memAdapter := adapter.NewMemoryAdapter(memEngine)
				memRetriever = memAdapter
				memStorer = memAdapter
			}

			// 3. Select chat provider.
			//
			// Load the project environment (.env + .cosca/config.yaml provider
			// block) BEFORE provider selection. In a bare shell `cosca chat`
			// has none of the OPENAI_* env vars, so without this the OpenAI
			// provider would be selected with the default model pointed at
			// api.openai.com — a 401 with a dummy key and a context leak (P1).
			projectCfg, loadErr := config.Load()
			if loadErr != nil {
				log.Debug().Err(loadErr).Msg("chat env: project config unavailable, using .env only")
			}
			loadChatEnv(projectCfg)

			registry := chat.GetRegistry()
			cfg := chat.DefaultChatRegistryConfig()
			if providerName != "" {
				cfg.Primary = providerName
			}
			// Register the built-in chat provider factories so that
			// auto-detection has candidates to choose from. Non-fatal:
			// registration errors are reported as a warning.
			if err := chatprovider.RegisterChatProviders(registry, nil); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to register chat providers: %v\n", err)
			}
			if err := registry.Select(ctx, cfg); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Warning: no chat provider available: %v\n", err)
				_, _ = fmt.Fprintf(os.Stderr, "Chat mode will have limited functionality.\n")
			}

			// 4. Build orchestration engine.
			orchConfig := orchestration.DefaultOrchestratorConfig()
			// Kernel-First Deliberation (ADR-032): opt-in via config. Absent
			// section / load failure keeps the fail-closed default (disabled).
			if projectCfg != nil {
				orchConfig.DeliberateConfig = orchestration.DeliberateConfigFromConfig(projectCfg.Orchestration.Deliberation)
			}
			orchConfig.EnableMAG = memRetriever != nil
			orchConfig.MAGConfig = orchestration.DefaultMAGConfig()

			engine := orchestration.NewFactory(orchestration.FactoryConfig{
				MemoryRetriever: memRetriever,
				MemoryStorer:    memStorer,
				AgentResolver:   agentResolver,
				SkillResolver:   skillResolver,
				ChatProvider:    registry,
				Config:          orchConfig,
			})

			// 5. Initialize session.
			session := &chatSession{
				engine:       pipeline.NewOrchAdapter(engine),
				agentName:    agentName,
				providerName: providerName,
				modelName:    modelName,
				streamMode:   streamMode,
				showMetrics:  showMetrics,
				history:      make([]chat.Message, 0),
				running:      true,
			}

			// 6. Print welcome banner.
			if !IsJSONOutput(cmd) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "╔══════════════════════════════════════════╗")
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "║       Cosca AI Chat — v1.1.0              ║")
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "║  Type /help for commands, /exit to quit ║")
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "╚══════════════════════════════════════════╝")
				_, _ = fmt.Fprintln(cmd.OutOrStdout())
				if session.agentName != "" {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Agent: %s\n", session.agentName)
				}
				if providerName != "" {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Provider: %s (%s)\n", registry.Name(), registry.Model())
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout())
			}

			// 7. REPL loop.
			scanner := bufio.NewScanner(os.Stdin)
			for session.running {
				select {
				case <-ctx.Done():
					session.running = false
					continue
				default:
				}

				_, _ = fmt.Fprint(cmd.OutOrStdout(), "> ")
				if !scanner.Scan() {
					break // EOF
				}

				input := strings.TrimSpace(scanner.Text())
				if input == "" {
					continue
				}

				// Handle commands.
				if strings.HasPrefix(input, "/") {
					session.handleCommand(cmd, input, formatter)
					continue
				}

				// Process prompt.
				session.handlePrompt(ctx, cmd, input)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\nGoodbye!")
			return nil
		},
	}

	cmd.Flags().StringVar(&agentName, "agent", "", "initial agent for the session")
	cmd.Flags().StringVar(&providerName, "provider", "", "initial chat provider")
	cmd.Flags().StringVar(&modelName, "model", "", "initial model name")
	cmd.Flags().BoolVar(&streamMode, "stream", false, "enable streaming by default")
	cmd.Flags().BoolVar(&showMetrics, "metrics", false, "show metrics after each response")

	return cmd
}

// ─── Command Handler ─────────────────────────────────────────────────────────

func (s *chatSession) handleCommand(cmd *cobra.Command, input string, _ *OutputFormatter) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}

	switch strings.ToLower(parts[0]) {
	case "/exit", "/quit", "/q":
		s.running = false

	case "/help", "/h":
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), `
Available commands:
  /agent <name>     — switch to a specific agent
  /provider <name>  — switch LLM provider
  /model <name>     — switch model
  /stream           — toggle streaming mode
  /metrics          — show session metrics
  /history          — show conversation history
  /clear            — clear conversation history
  /help             — show this help
  /exit, /quit      — end the session`)

	case "/agent":
		if len(parts) > 1 {
			s.agentName = strings.Join(parts[1:], " ")
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Agent set to: %s\n", s.agentName)
		} else {
			s.agentName = ""
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Agent cleared (auto-selection)")
		}

	case "/provider":
		if len(parts) > 1 {
			s.providerName = parts[1]
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Provider set to: %s (restart chat to apply)\n", s.providerName)
		} else {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Usage: /provider <name>")
		}

	case "/model":
		if len(parts) > 1 {
			s.modelName = parts[1]
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Model set to: %s (restart chat to apply)\n", s.modelName)
		} else {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Usage: /model <name>")
		}

	case "/stream":
		s.streamMode = !s.streamMode
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Streaming: %v\n", s.streamMode)

	case "/metrics":
		s.showMetrics = !s.showMetrics
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Metrics display: %v\n", s.showMetrics)

	case "/history":
		if len(s.history) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No conversation history.")
		} else {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Conversation history (%d messages):\n", len(s.history))
			for i, msg := range s.history {
				role := string(msg.Role)
				content := msg.Content
				if len(content) > 80 {
					content = content[:80] + "..."
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d. [%s] %s\n", i+1, role, content)
			}
		}

	case "/clear":
		s.history = s.history[:0]
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Conversation history cleared.")

	default:
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Unknown command: %s. Type /help for available commands.\n", parts[0])
	}
}

// ─── Prompt Handler ──────────────────────────────────────────────────────────

func (s *chatSession) handlePrompt(ctx context.Context, cmd *cobra.Command, input string) {
	startTime := time.Now()

	// Build pipeline request.
	pipeReq := pipeline.RunRequest{
		Prompt: input,
		Agent:  s.agentName,
	}

	// Add to history.
	s.history = append(s.history, chat.Message{Role: chat.RoleUser, Content: input})

	if s.streamMode {
		s.handleStream(ctx, cmd, pipeReq, startTime)
	} else {
		s.handleSync(ctx, cmd, pipeReq, startTime)
	}
}

func (s *chatSession) handleSync(ctx context.Context, cmd *cobra.Command, req pipeline.RunRequest, startTime time.Time) {
	result, err := s.engine.Run(ctx, req)
	if err != nil {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nError: %v\n\n", err)
		return
	}

	s.history = append(s.history, chat.Message{Role: chat.RoleAssistant, Content: result.Response})

	elapsed := time.Since(startTime)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n[%s · %s]\n\n", result.Agent, elapsed.Round(time.Millisecond))
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), result.Response)
	_, _ = fmt.Fprintln(cmd.OutOrStdout())

	if s.showMetrics {
		s.printMetrics(cmd)
	}
}

func (s *chatSession) handleStream(ctx context.Context, cmd *cobra.Command, req pipeline.RunRequest, startTime time.Time) {
	eventCh, err := s.engine.RunStream(ctx, req)
	if err != nil {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nStream error: %v\n\n", err)
		return
	}

	var fullResponse strings.Builder
	var agentName string

	_, _ = fmt.Fprintln(cmd.OutOrStdout())

	for event := range eventCh {
		switch event.Type {
		case pipeline.EventToolStart:
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\r  %v", event.Data)
		case pipeline.EventContent:
			if s, ok := event.Data.(string); ok {
				_, _ = fmt.Fprint(cmd.OutOrStdout(), s)
				fullResponse.WriteString(s)
			}
		case pipeline.EventError:
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n  [!] %v", event.Data)
		case pipeline.EventDone:
		}
	}

	s.history = append(s.history, chat.Message{Role: chat.RoleAssistant, Content: fullResponse.String()})

	elapsed := time.Since(startTime)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n\n[%s · %s]\n\n", agentName, elapsed.Round(time.Millisecond))

	if s.showMetrics {
		s.printMetrics(cmd)
	}
}

func (s *chatSession) printMetrics(cmd *cobra.Command) {
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "---")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Messages: %d | Streaming: %v | Agent: %s\n",
		len(s.history), s.streamMode, s.agentName)
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "---")
}

// ─── Chat Environment ─────────────────────────────────────────────────────────

// loadChatEnv loads the chat provider environment before the chat registry
// selects a provider. In a bare shell `cosca chat` has no OPENAI_* env vars,
// which would otherwise select the OpenAI provider with the default model
// pointed at api.openai.com (401 with a dummy key and a context leak).
//
// Precedence (highest first): explicit process env > project .env > config
// provider block (.cosca/config.yaml). Only keys not already set in the
// process environment are applied, so shell exports always win. Logs key
// NAMES only — values are never logged.
func loadChatEnv(cfg *config.Config) {
	applied := make([]string, 0)

	// 1. Project .env at the project root (explicit env wins over .env).
	cwd, err := os.Getwd()
	if err == nil {
		if envData, readErr := os.ReadFile(filepath.Join(cwd, ".env")); readErr == nil {
			for _, line := range strings.Split(string(envData), "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				if k, v, ok := strings.Cut(line, "="); ok {
					k = strings.TrimSpace(k)
					v = strings.TrimSpace(strings.Trim(v, `"'`))
					if os.Getenv(k) == "" {
						os.Setenv(k, v)
						applied = append(applied, k)
					}
				}
			}
		}
	}

	// 2. Config provider block — fill any remaining unset env vars so the
	// local provider declared in .cosca/config.yaml is used out of the box.
	if cfg != nil && strings.EqualFold(cfg.Provider.Name, "openai") {
		if cfg.Provider.Model != "" && os.Getenv("COSCA_OPENAI_MODEL") == "" {
			os.Setenv("COSCA_OPENAI_MODEL", cfg.Provider.Model)
			applied = append(applied, "COSCA_OPENAI_MODEL")
		}
		if cfg.Provider.BaseURL != "" && os.Getenv("OPENAI_BASE_URL") == "" {
			os.Setenv("OPENAI_BASE_URL", cfg.Provider.BaseURL)
			applied = append(applied, "OPENAI_BASE_URL")
		}
		if cfg.Provider.APIKey != "" && os.Getenv("OPENAI_API_KEY") == "" {
			os.Setenv("OPENAI_API_KEY", cfg.Provider.APIKey)
			applied = append(applied, "OPENAI_API_KEY")
		}
	}

	// Ollama is a local provider with no API key. Its model/URL live in the
	// config under provider.model/base_url but were NOT propagated to the
	// COSCA_OLLAMA_MODEL/OLLAMA_HOST env vars — so the ollama provider factory
	// fell back to its built-in "llama3" default (often not installed), which
	// made every LLM execution fail with executor_failed. Propagate them here
	// so the declared ollama model is used out of the box.
	if cfg != nil && strings.EqualFold(cfg.Provider.Name, "ollama") {
		if cfg.Provider.Model != "" && os.Getenv("COSCA_OLLAMA_MODEL") == "" {
			os.Setenv("COSCA_OLLAMA_MODEL", cfg.Provider.Model)
			applied = append(applied, "COSCA_OLLAMA_MODEL")
		}
		if cfg.Provider.BaseURL != "" && os.Getenv("OLLAMA_HOST") == "" {
			os.Setenv("OLLAMA_HOST", cfg.Provider.BaseURL)
			applied = append(applied, "OLLAMA_HOST")
		}
	}

	// DeepSeek is an OpenAI-compatible cloud provider. Propagate the declared
	// model/base_url so the deepseek factory (chat/provider/register_chat.go)
	// honours the config out of the box. The API key is propagated from the
	// config only when the config actually holds one (config.Load resolves
	// api_key_env); if it is empty the key must come from the external env —
	// that is the fail-closed boundary: no key, no cloud call.
	if cfg != nil && strings.EqualFold(cfg.Provider.Name, "deepseek") {
		if cfg.Provider.Model != "" && os.Getenv("COSCA_DEEPSEEK_MODEL") == "" {
			os.Setenv("COSCA_DEEPSEEK_MODEL", cfg.Provider.Model)
			applied = append(applied, "COSCA_DEEPSEEK_MODEL")
		}
		if cfg.Provider.BaseURL != "" && os.Getenv("DEEPSEEK_BASE_URL") == "" {
			os.Setenv("DEEPSEEK_BASE_URL", cfg.Provider.BaseURL)
			applied = append(applied, "DEEPSEEK_BASE_URL")
		}
		if cfg.Provider.APIKey != "" && os.Getenv("DEEPSEEK_API_KEY") == "" {
			os.Setenv("DEEPSEEK_API_KEY", cfg.Provider.APIKey)
			applied = append(applied, "DEEPSEEK_API_KEY")
		}
	}

	if len(applied) > 0 {
		log.Debug().Strs("keys", applied).Msg("chat env applied")
	}
}
