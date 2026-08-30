package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/adapter"
	"github.com/CoscaAI/cosca/internal/agents"
	"github.com/CoscaAI/cosca/internal/chat"
	chatprovider "github.com/CoscaAI/cosca/internal/chat/provider"
	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/cost"
	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/orchestration"
	"github.com/CoscaAI/cosca/internal/pipeline"
	"github.com/CoscaAI/cosca/internal/skills"
)

// ─── Run Command ─────────────────────────────────────────────────────────────

// NewRunCommand creates the `cosca run` command.
func NewRunCommand() *cobra.Command {
	var (
		agentName    string
		providerName string
		modelName    string
		streamFlag   bool
		noMAG        bool
		showMetrics  bool
		semanticFlag bool
		dryRun       bool
	)

	cmd := &cobra.Command{
		Use:   "run <prompt>",
		Short: "Execute a prompt through the AI Orchestration Engine",
		Long: `Execute a natural language prompt through the Cosca AI Orchestration Engine.

The engine:
  1. Searches knowledge and memory for relevant context
  2. Routes to the most appropriate agent
  3. Executes via the LLM provider
  4. Returns the agent's response

Without --agent, the engine automatically selects the best agent based on
the prompt content. Use --stream to see the response as it is generated.

Examples:
  cosca run "build an authentication API"
  cosca run --agent "Backend Chief" "design a database schema"
  cosca run --provider openai --model gpt-4 "explain microservices"
  cosca run --stream "tell me a story"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := strings.Join(args, " ")
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)
			ctx := cmd.Context()

			// 1. Resolve Cosca directory
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}
			coscaDir := filepath.Join(dir, ".cosca")

			// Propagate the configured provider (model/base_url) to env vars so
			// the chat provider factory selects the right model. Without this,
			// `cosca run` in a directory with no .env falls back to built-in
			// defaults (e.g. ollama "llama3", often not installed) and fails.
			if projectCfg, cfgErr := config.Load(); cfgErr == nil {
				loadChatEnv(projectCfg)
				// Boot-time Ollama detection (side-effect-free): when the
				// configured provider is ollama but the daemon is not
				// functional, surface a clear recommendation. The deterministic
				// flow (provider=none) is untouched.
				if report := ollamaBootHint(ctx, projectCfg); report != nil && !report.OK {
					formatter.Warning("Ollama provider not functional: " + report.Error)
				}
			}

			// 2. Initialize agent resolver
			agentMgr := agents.NewManager(coscaDir)
			agentResolver := adapter.NewAgentResolverAdapter(agentMgr)

			// 3. Initialize skill resolver
			skillMgr := skills.NewManager(coscaDir)
			skillResolver := adapter.NewSkillResolverAdapter(skillMgr)

			// 4. Initialize knowledge engine (optional — graceful degradation)
			var knowledgeSearcher orchestration.KnowledgeSearcher
			// Knowledge engine requires full DB+vector setup; skip for now.
			_ = knowledgeSearcher // nil is handled gracefully by the engine

			// 5. Initialize memory engine (optional — graceful degradation)
			var memRetriever orchestration.MemoryRetriever
			var memStorer orchestration.MemoryStorer

			if !noMAG {
				memEngine, err := memory.NewEngine(
					memory.WithConfig(memory.EngineConfig{
						DataDir:            coscaDir,
						DefaultTTL:         24 * time.Hour,
						MaxRecordsPerLayer: 1000,
						AutoPrune:          true,
						PruneInterval:      30 * time.Minute,
						SnapshotOnPromote:  true,
					}),
				)
				if err == nil {
					defer func() { _ = memEngine.Close() }()
					memAdapter := adapter.NewMemoryAdapter(memEngine)
					memRetriever = memAdapter
					memStorer = memAdapter
				}
			}

			// 6. Select chat provider
			registry := chat.GetRegistry()
			cfg := chat.DefaultChatRegistryConfig()

			if providerName != "" {
				cfg.Primary = providerName
			}

			// Register the built-in chat provider factories so that
			// auto-detection has candidates to choose from. Non-fatal:
			// if registration fails we still attempt Select below and
			// report the outcome clearly.
			if err := chatprovider.RegisterChatProviders(registry, nil); err != nil {
				formatter.Verbose(fmt.Sprintf("Warning: failed to register chat providers: %v", err))
			}

			// Attempt auto-selection; it is acceptable if no provider is
			// registered yet — we report a clear error to the user.
			if err := registry.Select(ctx, cfg); err != nil {
				return fmt.Errorf("failed to select chat provider: %w; hint: use 'cosca provider list' to see available providers, or 'cosca provider set <name>' to configure one", err)
			}

			if !useJSON {
				formatter.Verbose(fmt.Sprintf("Using provider: %s (model: %s)", registry.Name(), registry.Model()))
			}

			// Start hot-reload watcher for provider config changes.
			hotReload, hotReloadErr := chat.NewHotReload(registry, chat.DefaultHotReloadConfig())
			if hotReloadErr != nil {
				formatter.Verbose(fmt.Sprintf("Hot-reload not available: %v", hotReloadErr))
			} else if hotReload != nil {
				hotReload.Start()
				defer hotReload.Stop()
			}

			// 7. Build orchestration config
			orchConfig := orchestration.DefaultOrchestratorConfig()
			if !noMAG && memRetriever != nil {
				orchConfig.EnableMAG = true
				orchConfig.MAGConfig = orchestration.DefaultMAGConfig()
			}
			orchConfig.EnableStreaming = streamFlag

			// Allowlist de comandos ampliável POR PROJETO via env (segura:
			// confinada ao WorkspaceDir + allowlist, nunca comando arbitrário).
			// Usada para scaffloding/build de monorepos JS/Go (npm, npx, node,
			// nest, prisma, git, tsx, go). Sem a env, mantém o default seguro.
			if ac := os.Getenv("COSCA_ALLOWED_COMMANDS"); ac != "" {
				for _, c := range strings.Split(ac, ",") {
					if c = strings.TrimSpace(c); c != "" {
						orchConfig.AllowedCommands = append(orchConfig.AllowedCommands, c)
					}
				}
			}

			if semanticFlag {
				// Select an actual embedding provider before wiring semantic routing.
				// GetRegistry always returns a singleton, including an empty one, so
				// checking only for a non-nil registry would enable a router that can
				// never produce embeddings.
				embRegistry := embeddings.GetRegistry()
				embCfg := embeddings.DefaultProviderRegistryConfig()
				if err := embRegistry.Select(ctx, embCfg); err == nil {
					semCfg := orchestration.DefaultSemanticRouterConfig()
					semCfg.Enabled = true
					orchConfig.Embedder = embRegistry
					orchConfig.SemanticRouterConfig = &semCfg
					if !useJSON {
						formatter.Verbose("Semantic routing enabled (embedding-based agent selection)")
					}
				} else {
					formatter.Warning("Semantic routing requested but no embedding provider available. Using keyword routing.")
				}
			}

			// 8. Build orchestration engine
			engine := orchestration.NewFactory(orchestration.FactoryConfig{
				Knowledge:       knowledgeSearcher,
				MemoryRetriever: memRetriever,
				MemoryStorer:    memStorer,
				AgentResolver:   agentResolver,
				SkillResolver:   skillResolver,
				ChatProvider:    registry,
				Config:          orchConfig,
				// WorkspaceDir liga as ferramentas de sistema (write_file,
				// read_file, execute_command, etc.) ao executor: sem isso o
				// agente só "conversa" e não escreve software. Usa o cwd do
				// cosca run — rode de dentro do projeto alvo para construir nele.
				WorkspaceDir: dir,
			})

			// Store engine reference for metrics command.
			SetMetricsEngine(engine)

			// Wrap the engine in the unified Runner interface.
			runner := pipeline.NewOrchAdapter(engine)

			// 9. Build pipeline request
			pipeReq := pipeline.RunRequest{
				Prompt:  prompt,
				Agent:   agentName,
				Options: pipeline.RunOptions{},
			}

			// 10. Check dry-run mode before executing
			if dryRun {
				orchReq := &orchestration.Request{
					Prompt:  prompt,
					Context: make(map[string]interface{}),
				}
				if agentName != "" {
					orchReq.Context["agent"] = agentName
				}
				return runDryRun(cmd, engine, orchReq, formatter, useJSON)
			}

			// 11. Execute through unified Runner
			if streamFlag {
				return runStreamMode(ctx, runner, pipeReq, formatter, useJSON)
			}

			result, err := runner.Run(ctx, pipeReq)
			if err != nil {
				return fmt.Errorf("execution failed: %w", err)
			}

			// 12. Print result
			if useJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			formatter.Header("Execution Result")
			formatter.KeyValue("Request ID", result.TraceID)
			formatter.KeyValue("Agent", result.Agent)
			formatter.KeyValue("Tokens", fmt.Sprintf("%d input / %d output", result.TokenUsage.Input, result.TokenUsage.Output))
			// Fase 0 ADR-031 — instrumentação best-effort: registra a execução
			// no log de custo (Useful Work / Tokens). Nunca falha o run.
			recordRunCost(cmd, result)
			formatter.Println("")
			formatter.Header("Response")
			if result.Response != "" {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), result.Response)
			} else {
				formatter.Warning("No response content returned from the agent.")
			}

			if showMetrics {
				snapshot := engine.Metrics().Snapshot()
				printMetricsSnapshot(formatter, snapshot)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&agentName, "agent", "", "explicitly specify the agent to use (e.g. \"Backend Chief\")")
	cmd.Flags().StringVar(&providerName, "provider", "", "chat provider to use (default: auto-detect)")
	cmd.Flags().StringVar(&modelName, "model", "", "model name to use")
	cmd.Flags().BoolVar(&streamFlag, "stream", false, "enable streaming output")
	cmd.Flags().BoolVar(&noMAG, "no-mag", false, "disable Memory-Augmented Generation")
	cmd.Flags().BoolVar(&showMetrics, "metrics", false, "show execution metrics after completion")
	cmd.Flags().BoolVar(&semanticFlag, "semantic", false, "enable semantic (embedding-based) agent routing")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show pipeline plan without executing LLM calls")

	return cmd
}

// ─── Stream Mode ─────────────────────────────────────────────────────────────

// runStreamMode executes a request in streaming mode via the unified Runner
// interface and prints events progressively to the terminal.
func runStreamMode(ctx context.Context, runner pipeline.Runner, req pipeline.RunRequest, formatter *OutputFormatter, useJSON bool) error {
	eventCh, err := runner.RunStream(ctx, req)
	if err != nil {
		return fmt.Errorf("stream execution failed: %w", err)
	}

	var fullResponse strings.Builder

	for event := range eventCh {
		// Check for context cancellation.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if useJSON {
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintln(os.Stdout, string(data))
			continue
		}

		switch event.Type {
		case pipeline.EventContent:
			if s, ok := event.Data.(string); ok {
				_, _ = fmt.Fprint(os.Stdout, s)
				fullResponse.WriteString(s)
			}

		case pipeline.EventToolStart:
			formatter.Verbose(fmt.Sprintf("⚡ %v", event.Data))

		case pipeline.EventToolResult:
			formatter.Verbose(fmt.Sprintf("  ✓ %v", event.Data))

		case pipeline.EventError:
			if s, ok := event.Data.(string); ok {
				formatter.Warning(s)
			}

		case pipeline.EventDone:
			// stream finished
		}
	}

	if !useJSON {
		_, _ = fmt.Fprintln(os.Stdout) // Final newline after all chunks.
	}

	return nil
}

// runDryRun shows what the pipeline WOULD do without actually calling the LLM.
func runDryRun(cmd *cobra.Command, engine *orchestration.Engine, req *orchestration.Request, formatter *OutputFormatter, _ bool) error {
	formatter.Header("Dry Run — Pipeline Plan")
	formatter.KeyValue("Prompt", req.Prompt)

	if agent, ok := req.Context["agent"].(string); ok {
		formatter.KeyValue("Agent", agent+" (explicit)")
	} else {
		formatter.KeyValue("Agent", "auto-detect (semantic or keyword routing)")
	}

	formatter.Println("")
	formatter.Header("Pipeline Stages (will execute in order):")
	stages := []struct{ name, desc string }{
		{"1. MAG Retrieve", "Search memory for relevant context"},
		{"2. Context Builder", "Search knowledge engine + augment prompt"},
		{"3. Router", "Select best agent for the request"},
		{"4. Executor", "Send prompt to LLM provider"},
		{"5. Pipeline", "Execute skill chain (if configured)"},
		{"6. MAG Store", "Persist result as memory"},
	}

	for _, s := range stages {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %-20s → %s\n", s.name, s.desc)
	}

	formatter.Println("")
	formatter.Success("Dry run complete. Use 'cosca run' without --dry-run to execute.")
	return nil
}

// recordRunCost registra a execução no log de custo (ADR-031, Fase 0) —
// best-effort, NUNCA falha o run. Recebe o `RunResult` completo para mapear a
// EVIDÊNCIA DE VALOR determinística (build/test verificados + memória
// persistida) para o vetor decomposto. Se não houver tokens ou o .cosca não
// existir, silencia.
func recordRunCost(cmd *cobra.Command, run *pipeline.RunResult) {
	// Sem consumo de tokens → nada a registrar com valor de métrica.
	if run.TokenUsage.Input == 0 && run.TokenUsage.Output == 0 {
		return
	}
	coscaDir := resolveCoscaDir()
	if coscaDir == "" {
		return
	}
	store := cost.ForCoscaDir(coscaDir)
	rec := recordFromRun(run)
	if err := store.Append(rec); err != nil {
		// Log de custo é instrumentação, nunca pré-requisito.
		_ = cmd
		return
	}
}

// recordFromRun constrói o Record determinístico a partir do RunResult,
// aplicando a evidência de valor (build/test/memória) ao vetor decomposto.
// Nunca inventa: dimensões sem evidência real ficam 0 (honesto).
func recordFromRun(run *pipeline.RunResult) cost.Record {
	rec := cost.Record{
		AgentID:      run.Agent,
		TaskID:       run.TraceID,
		InputTokens:  run.TokenUsage.Input,
		OutputTokens: run.TokenUsage.Output,
		TokensTotal:  run.TokenUsage.Input + run.TokenUsage.Output,
		At:           time.Now(),
	}
	rec.ApplyValue(cost.ValueEvidence{
		BuildOK:     run.BuildResult != nil && run.BuildResult.Success,
		TestsRun:    run.TestResult != nil,
		TestsPassed: testsPassed(run.TestResult),
		MemStored:   run.MemoryID != "",
	})
	return rec
}

// testsPassed devolve o número de testes aprovados apenas quando a verificação
// de teste realmente executou E passou (determinístico). Caso contrário, 0 —
// não infla evidence_gain com base em TestResult ausente/falho.
func testsPassed(t *pipeline.TestResult) int {
	if t == nil || !t.Success {
		return 0
	}
	return t.Passed
}
