package cli

import (
	"bufio"
	"context"
	"encoding/json"
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
	"github.com/CoscaAI/cosca/internal/chat/sandbox"
	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/diagnostics"
	"github.com/CoscaAI/cosca/internal/grpcclient"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/orchestration"
	"github.com/CoscaAI/cosca/internal/pipeline"
	"github.com/CoscaAI/cosca/internal/skills"
	"github.com/CoscaAI/cosca/internal/trace"
	"github.com/CoscaAI/cosca/internal/workflows"

	"github.com/CoscaAI/cosca/internal/chat/ui/terminal"
	"github.com/CoscaAI/cosca/internal/compute"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rs/zerolog/log"
)

// ─── terminalServices bundles all wired components for the terminal. ──────
type terminalServices struct {
	runner             pipeline.Runner
	planner            *pipeline.Planner
	stepRunner         *pipeline.StepRunner
	recoveryLoop       *pipeline.RecoveryLoop
	genCtx             *pipeline.GeneralContext
	agentRouter        *pipeline.AgentRouter
	modelRouter        *pipeline.ModelRouter
	selfHealer         *pipeline.SelfHealer
	reviewPipeline     *pipeline.ReviewPipeline
	verificationRunner *pipeline.VerificationRunner
	costTracker        *pipeline.CostTracker
	perfTracker        *pipeline.PerfTracker
	benchmark          *pipeline.BenchmarkReport
	termCtx            *pipeline.TerminalContext
	handoffCoord       *pipeline.HandoffCoordinator
	workflowMgr        *workflows.Manager
}

// NewTerminalCommand creates the `cosca terminal` TUI command.
func NewTerminalCommand() *cobra.Command {
	var (
		agentName    string
		providerName string
		modelName    string
		advancedMode bool
		headlessTask string
		outputFormat string
		skipSession  bool
		forceSession bool
	)

	cmd := &cobra.Command{
		Use:   "terminal",
		Short: "Launch the Cosca Terminal TUI",
		Long: `Launch the interactive Cosca Terminal — a full-featured TUI for the 
Cosca AI Orchestration Platform.

The terminal provides:
  - Interactive prompt with pipeline execution
  - Task panel for monitoring active tasks
  - HUD with token usage, cost, and agent status
  - Slash commands for system introspection (/agents, /context, etc.)
  - Streaming responses with real-time progress

SECURITY NOTE (OpenCode mode):
  cosca terminal runs OUTSIDE the sandbox (jail) for TUI quality — clean
  rendering, colors, and network. Security is preserved per-command: every
  command issued by the LLM executes inside a bubblewrap sandbox confined to
  the workspace (the same sandbox model used by the chat engine). Daemons
  (cosca serve / cosca runtime) remain jailed.

Headless mode:
  cosca terminal --task "create a Flask API" --output json

Session resume:
  cosca terminal                  # auto-detects and prompts to resume
  cosca terminal --no-resume       # skip session detection
  cosca terminal --force-resume    # resume last session without prompt

Shortcuts:
  Ctrl+T    Toggle task panel
  Ctrl+F    Toggle files panel  
  Ctrl+D    Toggle diff panel
  Ctrl+C    Cancel current operation / exit
  Ctrl+Q    Exit terminal
  /help     Show available commands

Examples:
  cosca terminal
  cosca terminal --agent "Backend Chief"
  cosca terminal --advanced
  cosca terminal --task "create a Flask API with /health endpoint" --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				cancel()
			}()

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			// ── Per-command sandbox gate ─────────────────────────────
			// The terminal runs OUTSIDE the jail (OpenCode mode) for TUI
			// quality. The security replacement is the per-command sandbox:
			// every command the LLM issues through execute_command runs inside
			// this gate (bwrap) confined to the workspace — never as the
			// terminal process itself.
			gate := buildTerminalSandboxGate(dir)

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

			var knowledgeSearcher orchestration.KnowledgeSearcher
			knowledgeCfg := knowledge.DefaultConfig()
			knowledgeCfg.DBPath = filepath.Join(coscaDir, "knowledge.db")
			knowledgeCfg.RootDir = dir
			knowledgeEngine, kerr := knowledge.New(knowledgeCfg)
			if kerr == nil {
				if initErr := knowledgeEngine.Init(); initErr != nil {
					log.Debug().Err(initErr).Msg("terminal: knowledge engine init failed")
				} else {
					knowledgeSearcher = adapter.NewKnowledgeAdapter(knowledgeEngine)
				}
			} else {
				log.Debug().Err(kerr).Msg("terminal: knowledge engine creation failed")
			}

			projectCfg, loadErr := config.Load()
			if loadErr != nil {
				log.Debug().Err(loadErr).Msg("terminal: project config unavailable")
			}
			loadChatEnv(projectCfg)

			registry := chat.GetRegistry()
			chatCfg := chat.DefaultChatRegistryConfig()
			if providerName != "" {
				chatCfg.Primary = providerName
			}
			if err := chatprovider.RegisterChatProviders(registry, nil); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to register chat providers: %v\n", err)
			}
			if err := registry.Select(ctx, chatCfg); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Warning: no chat provider available: %v\n", err)
				_, _ = fmt.Fprintf(os.Stderr, "Terminal will have limited functionality.\n")
			}

			orchConfig := orchestration.DefaultOrchestratorConfig()
			// Kernel-First Deliberation (ADR-032): opt-in via config. Absent
			// section / load failure keeps the fail-closed default (disabled).
			if projectCfg != nil {
				orchConfig.DeliberateConfig = orchestration.DeliberateConfigFromConfig(projectCfg.Orchestration.Deliberation)
			}
			orchConfig.EnableMAG = memRetriever != nil
			orchConfig.MAGConfig = orchestration.DefaultMAGConfig()
			orchConfig.WorkspaceDir = dir
			orchConfig.Sandbox = gate

			engine := orchestration.NewFactory(orchestration.FactoryConfig{
				Knowledge:       knowledgeSearcher,
				MemoryRetriever: memRetriever,
				MemoryStorer:    memStorer,
				AgentResolver:   agentResolver,
				SkillResolver:   skillResolver,
				ChatProvider:    registry,
				Config:          orchConfig,
			})

			runner := pipeline.NewOrchAdapter(engine)
			runner.SetWorkDir(dir)

			// ── Wire all cognitive components ─────────────────────────
			planner := pipeline.NewPlanner(knowledgeSearcher, agentResolver)
			stepRunner := pipeline.NewStepRunner(runner)
			// Durable execution: append-only event log + flight recorder.
			if err := stepRunner.EnableDurableLog(filepath.Join(coscaDir, "durable-events")); err != nil {
				log.Debug().Err(err).Msg("terminal: durable event log unavailable")
			}
			if ts, tsErr := trace.NewStore(filepath.Join(coscaDir, "trace.db")); tsErr == nil {
				stepRunner.SetTraceStore(ts)
			} else {
				log.Debug().Err(tsErr).Msg("terminal: trace store unavailable")
			}

			classifier := diagnostics.NewClassifier()
			recoveryLoop := pipeline.NewRecoveryLoop(classifier)
			recoveryLoop.SetWorkDir(dir)
			if projectCfg != nil && projectCfg.Pipeline.AutoFix {
				recoveryLoop.AutoFix = true
			}

			agentRouter := pipeline.NewAgentRouter(agentResolver)
			modelRouter := pipeline.NewModelRouter(registry)
			selfHealer := pipeline.NewSelfHealer(classifier, recoveryLoop, runner, nil)
			reviewPipeline := pipeline.NewReviewPipeline(runner, nil)

			genCtx := pipeline.NewGeneralContext(
				"terminal session",
				knowledgeSearcher,
				memRetriever,
				planner,
				agentResolver,
			)

			costTracker := pipeline.NewCostTracker()
			perfTracker := pipeline.NewPerfTracker()
			benchmark := pipeline.NewBenchmark("terminal session")

			termCtx := pipeline.NewTerminalContext("terminal-" + time.Now().Format("20060102-150405"))
			termCtx.ProjectPath = dir
			termCtx.DetectProject()

			if modelName != "" {
				termCtx.ActiveModel = modelName
			} else {
				termCtx.ActiveModel = registry.Model()
			}
			if agentName != "" {
				termCtx.ActiveAgent = agentName
			}

			handoffStore, _ := pipeline.NewHandoffStore(filepath.Join(coscaDir, "handoffs"))
			handoffCoord := pipeline.NewHandoffCoordinator(handoffStore)

			// Runtime health check
			runtimeClient := grpcclient.NewRuntimeClient("")
			rctx, rcancel := context.WithTimeout(ctx, 2*time.Second)
			_, healthErr := runtimeClient.Health(rctx)
			rcancel()
			if healthErr == nil {
				formatter.Verbose("Runtime: connected")
			} else {
				formatter.Verbose("Runtime: unavailable (limited functionality)")
			}

			// Workflow manager
			workflowMgr := workflows.NewManager(coscaDir)
			wfList := workflowMgr.List()
			if len(wfList) > 0 {
				formatter.Verbose(fmt.Sprintf("Workflows: %d available", len(wfList)))
			}

			// Verification runner
			verificationRunner := pipeline.NewVerificationRunner(dir)

			svc := &terminalServices{
				runner:             runner,
				planner:            planner,
				stepRunner:         stepRunner,
				recoveryLoop:       recoveryLoop,
				genCtx:             genCtx,
				agentRouter:        agentRouter,
				modelRouter:        modelRouter,
				selfHealer:         selfHealer,
				reviewPipeline:     reviewPipeline,
				verificationRunner: verificationRunner,
				costTracker:        costTracker,
				perfTracker:        perfTracker,
				benchmark:          benchmark,
				termCtx:            termCtx,
				handoffCoord:       handoffCoord,
				workflowMgr:        workflowMgr,
			}

			// ── Session Resume ──────────────────────────────────────
			if !skipSession && headlessTask == "" {
				sessionsDir := filepath.Join(coscaDir, "sessions")
				resumed := tryResumeSession(sessionsDir, dir, termCtx, forceSession)
				if resumed {
					termCtx.SessionState = "resumed"
				}
			}

			// ── Durable stale-run notice ───────────────────────────
			if headlessTask == "" {
				noticeStaleRuns(formatter, stepRunner)
			}

			// ── Headless Mode ───────────────────────────────────────
			if headlessTask != "" {
				return svc.runHeadless(ctx, headlessTask, outputFormat)
			}

			// ── Interactive TUI Mode ────────────────────────────────
			output := os.Stdout

			// Background compute fabric: tasks are submitted to a worker pool
			// so the terminal stays interactive while work runs in the
			// background. Falls back to plain goroutines if it cannot start.
			var fabric *compute.Fabric
			if fb := compute.NewFabric(compute.LoadFabricConfig()); fb != nil {
				if ferr := fb.Start(ctx); ferr != nil {
					log.Debug().Err(ferr).Msg("terminal: fabric start failed (falling back to plain goroutines)")
				} else {
					fabric = fb
					defer fb.Stop(context.Background())
				}
			}

			m := terminal.New(runner, planner, stepRunner, recoveryLoop, termCtx, terminal.ModelConfig{
				AdvancedMode: advancedMode,
				CurrentModel: termCtx.ActiveModel,
				CurrentAgent: termCtx.ActiveAgent,
				Output:       output,
				Fabric:       fabric,
			})

			p := tea.NewProgram(m, tea.WithOutput(output), tea.WithAltScreen(), tea.WithContext(ctx))

			if _, err := p.Run(); err != nil {
				return fmt.Errorf("terminal: %w", err)
			}

			if sstore, serr := pipeline.NewSessionStore(filepath.Join(coscaDir, "sessions")); serr == nil {
				record := pipeline.RecordFromTerminalContext(termCtx, "interactive session", "interactive session")
				sstore.Save(record)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&agentName, "agent", "", "initial agent for the terminal session")
	cmd.Flags().StringVar(&providerName, "provider", "", "initial chat provider")
	cmd.Flags().StringVar(&modelName, "model", "", "initial model name")
	cmd.Flags().BoolVar(&advancedMode, "advanced", false, "start in advanced mode with full panels")
	cmd.Flags().StringVar(&headlessTask, "task", "", "run a task non-interactively and exit")
	cmd.Flags().StringVar(&outputFormat, "output", "text", "output format: text, json")
	cmd.Flags().BoolVar(&skipSession, "no-resume", false, "skip session resume detection")
	cmd.Flags().BoolVar(&forceSession, "force-resume", false, "resume last session without prompt")

	return cmd
}

func tryResumeSession(sessionsDir, projectPath string, termCtx *pipeline.TerminalContext, force bool) bool {
	store, err := pipeline.NewSessionStore(sessionsDir)
	if err != nil {
		return false
	}

	record, err := store.LoadLatest(projectPath)
	if err != nil || record == nil {
		return false
	}

	if record.State == "completed" || record.State == "failed" {
		return false
	}

	fmt.Fprintf(os.Stdout, "\n  Session found:\n")
	fmt.Fprintf(os.Stdout, "    Project: %s\n", filepath.Base(projectPath))
	fmt.Fprintf(os.Stdout, "    Last task: %s\n", truncateStr(record.LastTask, 60))
	fmt.Fprintf(os.Stdout, "    Progress: %.0f%%\n", record.Progress*100)
	fmt.Fprintf(os.Stdout, "    State: %s\n", record.State)
	fmt.Fprintf(os.Stdout, "    Duration: %s\n", time.Since(record.StartedAt).Round(time.Second))

	if len(record.Blockers) > 0 {
		fmt.Fprintf(os.Stdout, "    Unresolved: %s\n", strings.Join(record.Blockers, ", "))
	}

	if force {
		fmt.Fprint(os.Stdout, "\n  Resuming session...\n\n")
		pipeline.RestoreTerminalContext(termCtx, record)
		return true
	}

	fmt.Fprint(os.Stdout, "\n  Continue? [Y/n] ")
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.ToLower(strings.TrimSpace(response))

	if response == "" || response == "y" || response == "yes" {
		fmt.Fprint(os.Stdout, "\n  Resuming session...\n\n")
		pipeline.RestoreTerminalContext(termCtx, record)
		return true
	}

	fmt.Fprint(os.Stdout, "  Starting fresh session.\n\n")
	return false
}

// ─── Headless execution ──────────────────────────────────────────────────

type headlessOutput struct {
	Task      string       `json:"task"`
	Status    string       `json:"status"`
	Plan      headlessPlan `json:"plan"`
	Execution headlessExec `json:"execution"`
	Result    string       `json:"result"`
	Cost      headlessCost `json:"cost"`
}

type headlessPlan struct {
	Tasks            int `json:"tasks"`
	EstimatedMinutes int `json:"estimated_minutes"`
}

type headlessExec struct {
	TasksCompleted int    `json:"tasks_completed"`
	TasksFailed    int    `json:"tasks_failed"`
	AgentUsed      string `json:"agent_used"`
	ModelUsed      string `json:"model_used"`
	Duration       string `json:"duration"`
}

type headlessCost struct {
	Tokens       int     `json:"tokens"`
	EstimatedUSD float64 `json:"estimated_usd"`
}

func (svc *terminalServices) runHeadless(ctx context.Context, task, format string) error {
	startTime := time.Now()

	// ── Security gate ──────────────────────────────────────────────
	secGate := pipeline.NewSecurityGate()
	conf, err := secGate.Evaluate(task)
	if err != nil {
		writeHeadlessJSON(format, task, "blocked", err.Error())
		return fmt.Errorf("task blocked by security gate: %w", err)
	}
	if conf != nil && conf.AutoBlock {
		writeHeadlessJSON(format, task, "blocked", conf.Reason)
		return fmt.Errorf("task blocked: %s", conf.Reason)
	}

	// ── GeneralContext: acquire knowledge + plan ──────────────────
	svc.genCtx.TransitionState(pipeline.StatePlanning, "task: "+truncateStr(task, 80))

	if items, err := svc.genCtx.AcquireKnowledge(task); err == nil && len(items) > 0 {
		svc.genCtx.RecordKnowledgeUsed(make([]string, len(items)))
		svc.benchmark.RecordKnowledgeUse(len(items))
		fmt.Fprintf(os.Stderr, "  Knowledge: %d items found\n", len(items))
	}

	plan, planErr := svc.genCtx.PlanExecution(task)
	if planErr != nil {
		writeHeadlessJSON(format, task, "failed", planErr.Error())
		return planErr
	}

	planID := plan.ID
	if planID == "" {
		planID = "headless-" + time.Now().Format("20060102-150405")
		plan.ID = planID
	}
	svc.benchmark.RecordPlan(plan)

	fmt.Fprintf(os.Stderr, "  Planning: %d tasks, ~%d min\n", len(plan.Tasks), plan.EstimatedMinutes)

	// ── Model routing ─────────────────────────────────────────────
	if len(plan.Tasks) > 0 {
		choice := svc.modelRouter.Select(plan.Tasks[0], svc.genCtx)
		if choice != nil && choice.Model != "" {
			svc.termCtx.ActiveModel = choice.Model
			fmt.Fprintf(os.Stderr, "  Model: %s/%s (%s)\n", choice.Provider, choice.Model, choice.Reason)
		}
		_ = svc.termCtx.ActiveModel // recorded via benchmark.RecordTaskCompletion
	}

	// ── Agent routing ─────────────────────────────────────────────
	var previousAgent string
	for _, t := range plan.Tasks {
		agent, err := svc.genCtx.SelectAgent(t)
		if err == nil && agent != "" {
			t.Agent = agent
			fmt.Fprintf(os.Stderr, "  Agent: %s → %s\n", t.ID, agent)
			if previousAgent != "" && previousAgent != agent && svc.handoffCoord != nil {
				artifact := svc.handoffCoord.PrepareHandoff(previousAgent, agent, t, svc.genCtx)
				if handoffErr := svc.handoffCoord.ExecuteHandoff(ctx, artifact); handoffErr != nil {
					log.Debug().Err(handoffErr).Msg("terminal: handoff execution failed")
				} else {
					svc.genCtx.RegisterAgentSwitch(previousAgent, agent,
						fmt.Sprintf("handoff from %s to %s for task %s", previousAgent, agent, t.ID))
				}
			}
			previousAgent = agent
		}
	}
	if len(plan.Tasks) > 0 && plan.Tasks[0].Agent != "" {
		svc.termCtx.ActiveAgent = plan.Tasks[0].Agent
	}
	_ = svc.termCtx.ActiveAgent // recorded via benchmark.RecordTaskCompletion

	// ── Setup execution ───────────────────────────────────────────
	svc.stepRunner.SetPlanID(planID)
	svc.stepRunner.SetPlugins(nil)

	svc.termCtx.SessionState = "executing"
	if svc.termCtx.SessionContext == nil {
		svc.termCtx.SessionContext = pipeline.NewSessionContext(plan)
	}
	svc.termCtx.SessionContext.Plan = plan
	svc.genCtx.TransitionState(pipeline.StateExecuting, "executing "+planID)

	// ── Execute with SelfHealer recovery ──────────────────────────
	perfStart := time.Now()
	fmt.Fprintf(os.Stderr, "  Executing...\n")

	var result *pipeline.RunPlanResult
	var executeErr error

	for attempt := 0; attempt <= 3; attempt++ {
		result, executeErr = svc.stepRunner.RunPlanParallel(ctx, plan,
			func(stepName string, status pipeline.TaskStatus, done, total int) {
				symbol := "."
				switch status {
				case pipeline.TaskCompleted:
					symbol = "+"
				case pipeline.TaskFailed:
					symbol = "x"
				case pipeline.TaskRunning:
					symbol = ">"
				}
				fmt.Fprintf(os.Stderr, "    [%d/%d] %s %s (%s)\n", done, total, symbol, stepName, status)
			})

		if executeErr == nil && result != nil && result.TasksFailed == 0 {
			break
		}

		if attempt < 3 {
			fmt.Fprintf(os.Stderr, "  Self-healing attempt %d/3...\n", attempt+1)
			healed := false
			for _, t := range plan.Tasks {
				if t.Status == pipeline.TaskFailed {
					taskErr := fmt.Errorf("task %s failed", t.ID)
					if t.Result != nil && t.Result.Error != "" {
						taskErr = fmt.Errorf("task %s failed: %s", t.ID, t.Result.Error)
					}
					healAttempt, healErr := svc.selfHealer.Heal(ctx, t, taskErr)
					recovered := healErr == nil && healAttempt != nil && healAttempt.Success
					svc.benchmark.RecordError(taskErr, recovered)
					if recovered {
						t.Status = pipeline.TaskRunning
						fmt.Fprintf(os.Stderr, "    Healed: %s → %s\n", t.ID, healAttempt.Action)
						healed = true
					} else {
						fmt.Fprintf(os.Stderr, "    Failed to heal: %s (category: %s)\n", t.ID, healAttempt.Category)
					}
				}
			}
			if !healed {
				fmt.Fprintf(os.Stderr, "  No tasks could be healed in attempt %d\n", attempt+1)
			}
		}
	}

	execDuration := time.Since(perfStart)
	svc.perfTracker.RecordPhase(pipeline.PhaseGeneration, execDuration)

	// ── Review pipeline ───────────────────────────────────────────
	reviewStart := time.Now()
	var fileChanges []pipeline.FileChange
	for _, t := range plan.Tasks {
		for _, f := range t.OutputFiles {
			fileChanges = append(fileChanges, pipeline.FileChange{Path: f, Action: "modified"})
		}
	}
	if len(fileChanges) > 0 {
		reviewReport, reviewErr := svc.reviewPipeline.Review(ctx, plan.Tasks[0], fileChanges)
		if reviewErr == nil && reviewReport != nil {
			fmt.Fprintf(os.Stderr, "  Review: %.1f/10, %d issues (%d critical)\n",
				reviewReport.Score, reviewReport.TotalIssues, reviewReport.CriticalIssues)
		}
	}
	svc.perfTracker.RecordPhase(pipeline.PhaseVerification, time.Since(reviewStart))

	// ── Build & Test Verification ─────────────────────────────────
	verifyStart := time.Now()
	buildPass, buildOutput, buildErr := svc.verificationRunner.BuildVerify()
	if buildErr != nil {
		fmt.Fprintf(os.Stderr, "  Build: ERROR — %v\n", buildErr)
	} else if buildPass {
		fmt.Fprintf(os.Stderr, "  Build: PASS\n")
	} else {
		fmt.Fprintf(os.Stderr, "  Build: FAIL\n%s\n", truncateStr(buildOutput, 200))
	}
	svc.benchmark.RecordVerification(buildPass)

	testPass, testOutput, testErr := svc.verificationRunner.TestVerify()
	if testErr != nil {
		fmt.Fprintf(os.Stderr, "  Tests: ERROR — %v\n", testErr)
	} else if testPass {
		fmt.Fprintf(os.Stderr, "  Tests: PASS\n")
	} else {
		fmt.Fprintf(os.Stderr, "  Tests: FAIL\n%s\n", truncateStr(testOutput, 200))
	}
	svc.benchmark.RecordVerification(testPass)
	svc.perfTracker.RecordPhase(pipeline.PhaseVerification, time.Since(verifyStart))

	// ── Definition of Done ────────────────────────────────────────
	dod := pipeline.StandardDoD()
	dod.SetWorkDir(svc.termCtx.ProjectPath)
	dodReport := dod.Validate(ctx, plan)

	totalDuration := time.Since(startTime)
	svc.perfTracker.RecordPhase(pipeline.PhasePlanning, totalDuration)

	// ── Final state ───────────────────────────────────────────────
	svc.termCtx.SessionState = "completed"
	if result == nil {
		result = &pipeline.RunPlanResult{}
	}
	if result.TasksFailed > 0 || !dodReport.AllPassed {
		svc.termCtx.SessionState = "failed"
	}
	svc.genCtx.TransitionState(svc.termCtx.SessionState,
		fmt.Sprintf("%d/%d tasks", result.TasksDone, result.TasksTotal))

	// ── Cost tracking ─────────────────────────────────────────────
	promptTokens := int64(svc.termCtx.CostEstimate.TotalInputTokens)
	compTokens := int64(svc.termCtx.CostEstimate.TotalOutputTokens)
	svc.costTracker.TrackTask(svc.termCtx.SessionID, planID, svc.termCtx.ActiveModel,
		promptTokens, compTokens, totalDuration)

	// ── Handoff chain trace ────────────────────────────────────────
	if svc.handoffCoord != nil {
		trace := svc.handoffCoord.ChainTrace(planID)
		if trace != "" {
			fmt.Fprintf(os.Stderr, "  %s\n", trace)
		}
	}

	// ── Benchmark update ──────────────────────────────────────────
	svc.benchmark.RecordTaskCompletion(plan.Tasks[0], result.TasksFailed == 0,
		svc.termCtx.ActiveAgent, svc.termCtx.ActiveModel)
	svc.benchmark.RecordVerification(dodReport.AllPassed)
	svc.benchmark.RecordDelivery(svc.termCtx.SessionState == "completed")
	svc.benchmark.CalculateScores()

	// ── Output ────────────────────────────────────────────────────
	agentUsed := svc.termCtx.ActiveAgent
	if agentUsed == "" {
		agentUsed = "cosca-backend"
	}
	totalTokens := promptTokens + compTokens

	summary := fmt.Sprintf("Plan %s: %d/%d tasks completed", planID, result.TasksDone, result.TasksTotal)
	if result.TasksFailed > 0 {
		summary = fmt.Sprintf("Plan %s: %d completed, %d failed", planID, result.TasksDone, result.TasksFailed)
	}
	fmt.Fprintf(os.Stderr, "  Done: %s (took %s)\n", summary, totalDuration.Round(time.Second))

	finalResult := summary
	if len(plan.Tasks) > 0 && plan.Tasks[0].Result != nil && plan.Tasks[0].Result.Output != "" {
		finalResult = plan.Tasks[0].Result.Output
	}

	if format == "json" {
		writeHeadlessJSON(format, task, svc.termCtx.SessionState, finalResult)
		// Full JSON with all fields
		output := headlessOutput{
			Task:   task,
			Status: svc.termCtx.SessionState,
			Plan: headlessPlan{
				Tasks:            len(plan.Tasks),
				EstimatedMinutes: plan.EstimatedMinutes,
			},
			Execution: headlessExec{
				TasksCompleted: result.TasksDone,
				TasksFailed:    result.TasksFailed,
				AgentUsed:      agentUsed,
				ModelUsed:      svc.termCtx.ActiveModel,
				Duration:       totalDuration.Round(time.Second).String(),
			},
			Result: finalResult,
			Cost: headlessCost{
				Tokens:       int(totalTokens),
				EstimatedUSD: svc.termCtx.CostEstimate.EstimatedCostUSD,
			},
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(output)
	} else {
		fmt.Fprintf(os.Stdout, "Status: %s\n", svc.termCtx.SessionState)
		fmt.Fprintf(os.Stdout, "Plan: %d tasks, ~%d min\n", len(plan.Tasks), plan.EstimatedMinutes)
		fmt.Fprintf(os.Stdout, "Execution: %d completed, %d failed\n", result.TasksDone, result.TasksFailed)
		fmt.Fprintf(os.Stdout, "Agent: %s | Model: %s\n", agentUsed, svc.termCtx.ActiveModel)
		fmt.Fprintf(os.Stdout, "Duration: %s\n", totalDuration.Round(time.Second))
		fmt.Fprintf(os.Stdout, "Result: %s\n", finalResult)
		fmt.Fprintf(os.Stdout, "Cost: %d tokens (~$%.4f)\n", totalTokens, svc.termCtx.CostEstimate.EstimatedCostUSD)
		fmt.Fprintf(os.Stdout, "Build: %v | Tests: %v\n", buildPass, testPass)
	}

	// ── Workflow summary ──────────────────────────────────────────
	wfList := svc.workflowMgr.List()
	if len(wfList) > 0 {
		fmt.Fprintf(os.Stderr, "  Workflows available: %d\n", len(wfList))
		for _, wf := range wfList {
			fmt.Fprintf(os.Stderr, "    - %s (v%s, %d steps)\n", wf.Name, wf.Version, wf.Steps)
		}
	}

	if sstore, err := pipeline.NewSessionStore(filepath.Join(svc.termCtx.ProjectPath, ".cosca", "sessions")); err == nil {
		record := pipeline.RecordFromTerminalContext(svc.termCtx, task, task)
		sstore.Save(record)
	}

	if result.TasksFailed > 0 || !dodReport.AllPassed {
		return fmt.Errorf("headless: pipeline did not pass all DoD checks")
	}

	return nil
}

func writeHeadlessJSON(format, task, status, msg string) {
	if format != "json" {
		return
	}
	output := headlessOutput{Task: task, Status: status, Result: msg}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(output)
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// buildTerminalSandboxGate creates the per-command execution sandbox for the
// terminal. The terminal runs OUTSIDE the jail (OpenCode mode); this gate is
// the security replacement: every LLM-issued command executes inside a bwrap
// sandbox confined to the workspace (SandboxWorkspace), with network off.
func buildTerminalSandboxGate(dir string) *sandbox.Gate {
	return sandbox.NewGate(dir, chat.SandboxWorkspace)
}
