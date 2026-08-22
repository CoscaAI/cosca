package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/adapter"
	"github.com/CoscaAI/cosca/internal/agents"
	"github.com/CoscaAI/cosca/internal/chat"
	chatprovider "github.com/CoscaAI/cosca/internal/chat/provider"
	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/diagnostics"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/orchestration"
	"github.com/CoscaAI/cosca/internal/pipeline"
	"github.com/CoscaAI/cosca/internal/skills"
	"github.com/CoscaAI/cosca/internal/trace"
	"github.com/CoscaAI/cosca/internal/workflows"
)

// pipelineWiring is the shared, REAL autonomous-pipeline wiring used by
// `cosca workflow run`, `cosca pipeline run` and `cosca run`. It mirrors the
// composition built in terminal.go: orchestrator → runner → planner →
// stepRunner → recoveryLoop → DoD, plus a workflows.Manager configured so
// Run() executes every step through the real pipeline — never the simulated
// 10ms tick.
type pipelineWiring struct {
	dir        string
	coscaDir   string
	workDir    string
	autoFix    bool
	runner     pipeline.Runner
	planner    *pipeline.Planner
	stepRunner *pipeline.StepRunner
	recovery   *pipeline.RecoveryLoop
	manager    *workflows.Manager

	// Handles de recursos SQLite abertos pelo wiring (trace.db,
	// knowledge.db, memory/index.db). Guardados aqui para que Close()
	// os libere — no Windows um handle aberto impede a remoção do
	// diretório e segura os arquivos após o comando terminar.
	traceStore *trace.Store
	knowledge  *knowledge.Engine
	memEngine  *memory.MemoryEngine
}

// Close libera os recursos SQLite abertos pelo wiring. Deve ser chamado via
// defer pelo comando que construiu o wiring.
func (w *pipelineWiring) Close() error {
	if w.traceStore != nil {
		_ = w.traceStore.Close()
	}
	if w.knowledge != nil {
		_ = w.knowledge.Close()
	}
	if w.memEngine != nil {
		_ = w.memEngine.Close()
	}
	return nil
}

// DoD returns a Definition of Done configured against the project work dir.
func (w *pipelineWiring) DoD() *pipeline.DefinitionOfDone {
	dod := pipeline.StandardDoD()
	dod.SetWorkDir(w.workDir)
	return dod
}

// buildPipelineWiring composes the full autonomous pipeline for a project
// directory. The workflows.Manager returned inside the wiring is created with
// a real PipelineExecutor AND a real StepRunner, so every step of a workflow
// is executed through the orchestration engine (LLM/agents) instead of being
// simulated. dir is the project root; when empty os.Getwd() is used.
func buildPipelineWiring(dir string) (*pipelineWiring, error) {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("resolve working directory: %w", err)
		}
	}
	coscaDir := filepath.Join(dir, ".cosca")

	wiring := &pipelineWiring{dir: dir, coscaDir: coscaDir, workDir: dir}

	projectCfg, loadErr := config.Load()
	if loadErr != nil {
		log.Debug().Err(loadErr).Msg("pipeline wiring: project config unavailable")
	}
	loadChatEnv(projectCfg)

	agentMgr := agents.NewManager(coscaDir)
	skillMgr := skills.NewManager(coscaDir)
	agentResolver := adapter.NewAgentResolverAdapter(agentMgr)
	skillResolver := adapter.NewSkillResolverAdapter(skillMgr)

	var knowledgeSearcher orchestration.KnowledgeSearcher
	knowledgeCfg := knowledge.DefaultConfig()
	knowledgeCfg.DBPath = filepath.Join(coscaDir, "knowledge.db")
	knowledgeCfg.RootDir = dir
	if ke, kerr := knowledge.New(knowledgeCfg); kerr == nil {
		if initErr := ke.Init(); initErr != nil {
			log.Debug().Err(initErr).Msg("pipeline wiring: knowledge init failed")
		} else {
			knowledgeSearcher = adapter.NewKnowledgeAdapter(ke)
			wiring.knowledge = ke
		}
	}

	var memRetriever orchestration.MemoryRetriever
	var memStorer orchestration.MemoryStorer
	if me, merr := memory.NewEngine(memory.WithConfig(memory.EngineConfig{
		DataDir:            coscaDir,
		DefaultTTL:         24 * time.Hour,
		MaxRecordsPerLayer: 1000,
		AutoPrune:          true,
		PruneInterval:      30 * time.Minute,
	})); merr == nil {
		memAdapter := adapter.NewMemoryAdapter(me)
		memRetriever = memAdapter
		memStorer = memAdapter
		wiring.memEngine = me
	} else {
		log.Debug().Err(merr).Msg("pipeline wiring: memory engine unavailable")
	}

	registry := chat.GetRegistry()
	chatCfg := chat.DefaultChatRegistryConfig()
	if err := chatprovider.RegisterChatProviders(registry, nil); err != nil {
		log.Debug().Err(err).Msg("pipeline wiring: chat provider registration failed")
	}
	selCtx, selCancel := context.WithTimeout(context.Background(), 15*time.Second)
	selErr := registry.Select(selCtx, chatCfg)
	selCancel()
	if selErr != nil {
		// Degrade gracefully: execution attempts will surface real errors
		// instead of a fabricated success.
		log.Debug().Err(selErr).Msg("pipeline wiring: no chat provider selected (degraded)")
	}

	orchConfig := orchestration.DefaultOrchestratorConfig()
	if memRetriever != nil {
		orchConfig.EnableMAG = true
		orchConfig.MAGConfig = orchestration.DefaultMAGConfig()
	}
	orchConfig.WorkspaceDir = dir

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

	planner := pipeline.NewPlanner(knowledgeSearcher, agentResolver)

	classifier := diagnostics.NewClassifier()
	recovery := pipeline.NewRecoveryLoop(classifier)
	recovery.SetWorkDir(dir)
	if projectCfg != nil && projectCfg.Pipeline.AutoFix {
		recovery.AutoFix = true
	}

	historyDir := filepath.Join(coscaDir, "history")
	history, histErr := pipeline.NewWorkflowHistory(historyDir)
	var checkpoint *pipeline.CheckpointStore
	if histErr != nil {
		log.Debug().Err(histErr).Msg("pipeline wiring: workflow history unavailable")
	} else {
		checkpoint, _ = pipeline.NewCheckpointStore(filepath.Join(historyDir, "checkpoints"))
	}

	var stepRunner *pipeline.StepRunner
	if history != nil && checkpoint != nil {
		stepRunner = pipeline.NewDurableStepRunner(runner, history, checkpoint, recovery)
	} else {
		stepRunner = pipeline.NewStepRunner(runner)
	}

	// Durable execution: append-only JSONL event log under
	// .cosca/durable-events/. Opt-in at the wiring level (a bare StepRunner
	// keeps its nil-log legacy behaviour). On a crash, completed steps are
	// replayed from the log and execution resumes from the first incomplete
	// task — never losing progress.
	if err := stepRunner.EnableDurableLog(filepath.Join(coscaDir, "durable-events")); err != nil {
		log.Debug().Err(err).Msg("pipeline wiring: durable event log unavailable")
	}
	// Universal flight recorder: mirror durable events so the desktop's
	// execution tree shows the durable state.
	if ts, tsErr := trace.NewStore(filepath.Join(coscaDir, "trace.db")); tsErr == nil {
		stepRunner.SetTraceStore(ts)
		wiring.traceStore = ts
	} else {
		log.Debug().Err(tsErr).Msg("pipeline wiring: trace store unavailable")
	}

	// workflows.Manager configured for REAL execution: a PipelineExecutor
	// delegates each step to the orchestration engine, and a StepRunner
	// provides the same real execution as a fallback path.
	executor := &workflowPipelineExecutor{runner: runner}
	mgr := workflows.NewManager(coscaDir,
		workflows.WithPipeline(executor),
		workflows.WithExecutionStore(orchestration.GetExecutionStore()),
	)
	mgr.SetStepRunner(buildWorkflowStepRunner(runner))

	wiring.autoFix = recovery.AutoFix
	wiring.runner = runner
	wiring.planner = planner
	wiring.stepRunner = stepRunner
	wiring.recovery = recovery
	wiring.manager = mgr

	return wiring, nil
}

// workflowPipelineExecutor adapts the real pipeline Runner to the
// workflows.PipelineExecutor contract. Each workflow step is converted into a
// pipeline TaskNode and executed through the real orchestration engine (LLM,
// build, test) via StepRunner.RunStep. Outputs are stored under step_N_output
// so the workflow manager can count completed steps.
type workflowPipelineExecutor struct {
	runner pipeline.Runner
}

var _ workflows.PipelineExecutor = (*workflowPipelineExecutor)(nil)

func (w *workflowPipelineExecutor) Execute(ctx context.Context, pc orchestration.PipelineContext, def orchestration.PipelineDefinition) (orchestration.PipelineContext, error) {
	sr := pipeline.NewStepRunner(w.runner)
	for i, step := range def.Steps {
		input := pc.Prompt
		if step.Input != "" {
			if v, ok := pc.Data.Extra[step.Input]; ok {
				if s, isStr := v.(string); isStr && s != "" {
					input = s
				}
			}
		}
		task := &pipeline.TaskNode{
			ID:          step.Name,
			Description: input,
			Agent:       step.Skill,
		}
		result, err := sr.RunStep(ctx, step.Name, task)
		if err != nil {
			return pc, fmt.Errorf("workflow step %q: %w", step.Name, err)
		}
		if result == nil || !result.Success {
			errText := "step failed"
			if result != nil && result.Error != "" {
				errText = result.Error
			}
			return pc, fmt.Errorf("workflow step %q failed: %s", step.Name, errText)
		}
		pc = pc.WithContextData(fmt.Sprintf("step_%d_output", i), result.Output)
	}
	return pc, nil
}

// buildWorkflowStepRunner returns a workflows.StepRunner that executes each
// workflow step through the real pipeline (StepRunner.RunStep → orchestrator).
// It is the fallback execution path used when the Manager has no pipeline
// executor, so Run() never falls back to the 10ms simulation.
func buildWorkflowStepRunner(runner pipeline.Runner) workflows.StepRunner {
	return func(ctx context.Context, step workflows.Step) (string, error) {
		sr := pipeline.NewStepRunner(runner)
		task := &pipeline.TaskNode{
			ID:          step.Name,
			Description: step.Description,
			Agent:       step.Agent,
		}
		result, err := sr.RunStep(ctx, step.Name, task)
		if err != nil {
			return "", err
		}
		if result == nil || !result.Success {
			errText := "step failed"
			if result != nil && result.Error != "" {
				errText = result.Error
			}
			return "", fmt.Errorf("step %q failed: %s", step.Name, errText)
		}
		return result.Output, nil
	}
}

// workflowToPlan converts a workflow's steps into a pipeline Plan so the CLI
// run paths can execute them with full per-task evidence and enforce DoD.
func workflowToPlan(wf *workflows.Workflow) *pipeline.Plan {
	tasks := make([]*pipeline.TaskNode, 0, len(wf.StepList))
	for i, s := range wf.StepList {
		name := s.Name
		if name == "" {
			name = fmt.Sprintf("step-%d", i+1)
		}
		tasks = append(tasks, &pipeline.TaskNode{
			ID:          name,
			Description: s.Description,
			Agent:       s.Agent,
			Status:      pipeline.TaskPending,
		})
	}
	plan := &pipeline.Plan{
		ID:        "WF-" + wf.Name,
		Intent:    wf.Description,
		Objective: wf.Description,
		Tasks:     tasks,
	}
	if len(tasks) > 0 {
		plan.EstimatedMinutes = len(tasks) * 3
		plan.RiskLevel = "medium"
	}
	return plan
}
