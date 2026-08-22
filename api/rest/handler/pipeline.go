// PipelineHandler handles the POST /v1/pipeline/run endpoint — full autonomous
// pipeline execution: Plan → StepRunner → RecoveryLoop → DoD → PostTaskHook → CMI.
//
// Unlike /v1/run which sends a prompt straight to the LLM, /v1/pipeline/run
// decomposes the prompt into a task DAG, executes each task with build/test
// validation, auto-recovers from failures, and records learnings for evolution.
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	apiauth "github.com/CoscaAI/cosca/api/auth"
	"github.com/CoscaAI/cosca/internal/audit"
	"github.com/CoscaAI/cosca/internal/pipeline"
	"github.com/rs/zerolog/log"
)

// PipelineHandler executes autonomous pipeline runs.
type PipelineHandler struct {
	runner       pipeline.Runner
	planner      *pipeline.Planner
	recoveryLoop *pipeline.RecoveryLoop
	hook         *pipeline.PostTaskHook
	cmi          *pipeline.CMITracker
	handoffStore *pipeline.HandoffStore
	auditStore   *audit.Store

	// Durable execution: event sourcing + checkpoint + crash recovery.
	history    *pipeline.WorkflowHistory
	checkpoint *pipeline.CheckpointStore

	// Plugin system: extensible step hooks.
	plugins *pipeline.PluginRegistry

	// Permission system: conditional authorization.
	permissions *pipeline.PermissionEvaluator

	// Default timeout for pipeline execution.
	timeout time.Duration
}

// NewPipelineHandler creates a new PipelineHandler.
func NewPipelineHandler(
	runner pipeline.Runner,
	planner *pipeline.Planner,
	recoveryLoop *pipeline.RecoveryLoop,
	hook *pipeline.PostTaskHook,
	cmi *pipeline.CMITracker,
	handoffStore *pipeline.HandoffStore,
	auditStore *audit.Store,
) *PipelineHandler {
	return &PipelineHandler{
		runner:       runner,
		planner:      planner,
		recoveryLoop: recoveryLoop,
		hook:         hook,
		cmi:          cmi,
		handoffStore: handoffStore,
		auditStore:   auditStore,
		timeout:      10 * time.Minute,
	}
}

// SetHistoryDir enables durable execution with event sourcing.
// History and checkpoint files are stored under the given directory.
func (h *PipelineHandler) SetHistoryDir(dir string) error {
	hist, err := pipeline.NewWorkflowHistory(dir)
	if err != nil {
		return fmt.Errorf("history: %w", err)
	}
	h.history = hist

	cpDir := dir + "/checkpoints"
	cp, err := pipeline.NewCheckpointStore(cpDir)
	if err != nil {
		return fmt.Errorf("checkpoint: %w", err)
	}
	h.checkpoint = cp

	// Initialize plugin registry with built-in plugins.
	h.plugins = pipeline.NewPluginRegistry()
	pipeline.RegisterBuiltinPlugins(h.plugins, cp)

	// Initialize permission evaluator with default policies.
	h.permissions = pipeline.NewPermissionEvaluator(
		pipeline.DefaultPermissionPolicy(),
		pipeline.AgentPermissionPolicy(),
	)
	return nil
}

// SetDurable wires pre-created durable execution components from bootstrap
// into the PipelineHandler. This avoids duplicate creation when the server
// already has WorkflowHistory, CheckpointStore, and PluginRegistry from
// bootstrap composition.
func (h *PipelineHandler) SetDurable(
	history *pipeline.WorkflowHistory,
	checkpoint *pipeline.CheckpointStore,
	plugins *pipeline.PluginRegistry,
) {
	h.history = history
	h.checkpoint = checkpoint
	if plugins != nil {
		h.plugins = plugins
	}
	// Initialize permission evaluator with default policies.
	h.permissions = pipeline.NewPermissionEvaluator(
		pipeline.DefaultPermissionPolicy(),
		pipeline.AgentPermissionPolicy(),
	)
}

// SetTimeout sets the pipeline execution timeout. Defaults to 10 minutes.
func (h *PipelineHandler) SetTimeout(d time.Duration) {
	if d > 0 {
		h.timeout = d
	}
}

// pipelineRequest is the JSON body for POST /v1/pipeline/run.
type pipelineRequest struct {
	Prompt string `json:"prompt"`
	Agent  string `json:"agent,omitempty"`
}

// pipelineResponse is the JSON response for POST /v1/pipeline/run.
type pipelineResponse struct {
	PlanID         string                `json:"plan_id"`
	Intent         string                `json:"intent"`
	IntentType     string                `json:"intent_type"`
	TasksTotal     int                   `json:"tasks_total"`
	TasksDone      int                   `json:"tasks_done"`
	TasksFailed    int                   `json:"tasks_failed"`
	DoDReport      *pipeline.DoDReport   `json:"dod_report"`
	CMISnapshot    *pipeline.CMISnapshot `json:"cmi_snapshot"`
	SessionSummary string                `json:"session_summary"`
	Tasks          []pipelineTaskResult  `json:"tasks,omitempty"`
	DurationMs     int64                 `json:"duration_ms"`
	Autonomy       float64               `json:"autonomy"` // 0.0–1.0
}

type pipelineTaskResult struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Agent       string `json:"agent"`
	Status      string `json:"status"`
	Output      string `json:"output,omitempty"`
	Error       string `json:"error,omitempty"`
	RetriesUsed int    `json:"retries_used,omitempty"`
	DurationMs  int64  `json:"duration_ms,omitempty"`
}

// ExecutePipeline handles POST /v1/pipeline/run.
func (h *PipelineHandler) ExecutePipeline(w http.ResponseWriter, r *http.Request) {
	var req pipelineRequest
	limitBody(w, r, bodyLimitLarge)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}

	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	if len(req.Prompt) > maxPromptLength {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("prompt too long: max %d characters", maxPromptLength))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	// ── 0. Permission check ────────────────────────────────────────────
	if h.permissions != nil {
		user := "anonymous"
		role := "viewer"

		if claims, ok := apiauth.ClaimsFromContext(r.Context()); ok && claims != nil {
			if claims.Sub != "" {
				user = claims.Sub
			}
			if claims.Role != "" {
				role = claims.Role
			}
		}

		pc := &pipeline.PermissionContext{
			User:   user,
			Role:   role,
			Action: pipeline.ActionPlanExecute,
		}
		if allowed, reason := h.permissions.Authorize(ctx, pc); !allowed {
			writeError(w, http.StatusForbidden, fmt.Sprintf("permission denied: %s", reason))
			return
		}
	}

	startTime := time.Now()

	// ── 1. Plan ────────────────────────────────────────────────────────
	plan := h.planner.Plan(req.Prompt)
	sessionCtx := pipeline.NewSessionContext(plan)

	// ── 2. Execute tasks with durable execution ────────────────────────
	var stepRunner *pipeline.StepRunner
	if h.history != nil {
		stepRunner = pipeline.NewDurableStepRunner(h.runner, h.history, h.checkpoint, h.recoveryLoop)
	} else {
		stepRunner = pipeline.NewStepRunner(h.runner)
	}
	stepRunner.SetPlanID(plan.ID)
	if h.plugins != nil {
		stepRunner.SetPlugins(h.plugins)
	}

	planResult, runErr := stepRunner.RunPlanParallel(ctx, plan, nil)

	var taskResults []pipelineTaskResult
	knowledgeGained := 0
	errorsDetected := 0
	knowledgeReused := 0

	if runErr != nil {
		// RunPlan itself failed (not individual tasks)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("pipeline execution failed: %v", runErr))
		return
	}

	for _, task := range plan.Tasks {
		tr := pipelineTaskResult{
			ID:          task.ID,
			Description: task.Description,
			Agent:       task.Agent,
			Status:      string(task.Status),
		}

		if task.Result != nil {
			tr.Output = task.Result.Output
			tr.Error = task.Result.Error
			tr.DurationMs = task.Result.DurationMs
		}

		switch task.Status {
		case pipeline.TaskCompleted:
			knowledgeGained++
		case pipeline.TaskFailed:
			errorsDetected++
		}

		// ── Handoff artifact ──────────────────────────────────────
		if h.handoffStore != nil {
			artifact := &pipeline.HandoffArtifact{
				FromAgent: task.Agent,
				Objective: task.Description,
			}
			_ = h.handoffStore.Save(artifact)
			sessionCtx.RecordHandoff(artifact)
		}

		// ── PostTaskHook (stage 7-8 auto-evolution) ───────────────
		if h.hook != nil {
			outcome := string(task.Status)
			_ = h.hook.RecordTask(ctx, pipeline.EvolutionRecord{
				AgentName: task.Agent,
				TaskType:  plan.IntentType,
				Outcome:   outcome,
				Technique: "pipeline-step",
				Level:     3,
				Learned:   []string{fmt.Sprintf("task %s: %s", task.ID, outcome)},
			})
			_ = h.hook.UpdateCapability(ctx, task.Agent, outcome)
		}

		taskResults = append(taskResults, tr)
	}

	_ = planResult // used for events/history in audit

	// ── 3. Definition of Done ──────────────────────────────────────────
	dod := pipeline.StandardDoD()
	dodReport := dod.Validate(ctx, plan)

	// ── 4. CMI Update ──────────────────────────────────────────────────
	if h.cmi != nil {
		h.cmi.RecordTask("success", knowledgeGained, errorsDetected, knowledgeReused)
	}
	cmiSnapshot := &pipeline.CMISnapshot{}
	if h.cmi != nil {
		snap := h.cmi.Snapshot()
		cmiSnapshot = &snap
	}

	// ── 5. Response ────────────────────────────────────────────────────
	completed, _ := plan.Progress()
	failed := 0
	for _, t := range plan.Tasks {
		if t.Status == pipeline.TaskFailed {
			failed++
		}
	}

	duration := time.Since(startTime)

	resp := pipelineResponse{
		PlanID:         plan.ID,
		Intent:         plan.Intent,
		IntentType:     plan.IntentType,
		TasksTotal:     len(plan.Tasks),
		TasksDone:      completed,
		TasksFailed:    failed,
		DoDReport:      dodReport,
		CMISnapshot:    cmiSnapshot,
		SessionSummary: sessionCtx.Summary(),
		Tasks:          taskResults,
		DurationMs:     duration.Milliseconds(),
	}

	LogEvent(h.auditStore, r, "pipeline.run", "pipeline",
		audit.DetailsJSON(map[string]string{
			"plan_id":      plan.ID,
			"intent_type":  plan.IntentType,
			"tasks_total":  fmt.Sprintf("%d", len(plan.Tasks)),
			"tasks_done":   fmt.Sprintf("%d", completed),
			"tasks_failed": fmt.Sprintf("%d", failed),
		}), "success")

	// ── 6. Analytics ───────────────────────────────────────────────────
	analytics := &pipeline.RunAnalytics{
		PlanID:              plan.ID,
		Intent:              plan.Intent,
		IntentType:          plan.IntentType,
		TasksTotal:          len(plan.Tasks),
		TasksDone:           completed,
		TasksFailed:         failed,
		RecoveriesAttempted: errorsDetected,
		RecoveriesSucceeded: errorsDetected, // proxied — refined in analytics store
		LLMCalls:            len(plan.Tasks),
		DurationMs:          duration.Milliseconds(),
	}
	analytics.ComputeDerived()
	resp.Autonomy = analytics.AutonomyScore

	// Persist analytics into the event history so downstream consumers
	// (cosca qgate / cosca status autonomy dashboard) reflect this run.
	if h.history != nil {
		store := pipeline.NewAnalyticsStore(h.history)
		if saveErr := store.Save(analytics); saveErr != nil {
			log.Warn().Err(saveErr).Str("plan_id", plan.ID).Msg("pipeline: failed to persist analytics")
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// ExecutePipelineStream handles POST /v1/pipeline/run/stream — SSE streaming.
func (h *PipelineHandler) ExecutePipelineStream(w http.ResponseWriter, r *http.Request) {
	var req pipelineRequest
	limitBody(w, r, bodyLimitLarge)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDecodeError(w, err)
		return
	}
	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	// Setup SSE
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	startTime := time.Now()

	// Plan
	plan := h.planner.Plan(req.Prompt)
	sendSSE(w, flusher, "plan", map[string]interface{}{
		"plan_id":  plan.ID,
		"intent":   plan.Intent,
		"tasks":    len(plan.Tasks),
	})

	// Setup StepRunner
	var stepRunner *pipeline.StepRunner
	if h.history != nil {
		stepRunner = pipeline.NewDurableStepRunner(h.runner, h.history, h.checkpoint, h.recoveryLoop)
	} else {
		stepRunner = pipeline.NewStepRunner(h.runner)
	}
	stepRunner.SetPlanID(plan.ID)
	if h.plugins != nil {
		stepRunner.SetPlugins(h.plugins)
	}

	// Execute with progress streaming (parallel execution, respecting DAG)
	runResult, runErr := stepRunner.RunPlanParallel(ctx, plan, func(stepName string, status pipeline.TaskStatus, done, total int) {
		sendSSE(w, flusher, "step", map[string]interface{}{
			"step":   stepName,
			"status": string(status),
			"done":   done,
			"total":  total,
		})
	})

	dur := time.Since(startTime)

	if runErr != nil {
		sendSSE(w, flusher, "error", map[string]interface{}{"error": runErr.Error()})
		return
	}

	// Analytics
	analytics := &pipeline.RunAnalytics{
		PlanID:     plan.ID,
		TasksTotal: len(plan.Tasks),
		DurationMs: dur.Milliseconds(),
	}
	if runResult != nil {
		analytics.TasksDone = runResult.TasksDone
		analytics.TasksFailed = runResult.TasksFailed
		analytics.RecoveriesSucceeded = runResult.Recovered
		analytics.LLMCalls = runResult.TasksDone
	}
	analytics.ComputeDerived()

	// Persist analytics into the event history so downstream consumers
	// (cosca qgate / cosca status autonomy dashboard) reflect this run.
	if h.history != nil {
		store := pipeline.NewAnalyticsStore(h.history)
		if saveErr := store.Save(analytics); saveErr != nil {
			log.Warn().Err(saveErr).Str("plan_id", plan.ID).Msg("pipeline: failed to persist analytics")
		}
	}

	sendSSE(w, flusher, "done", map[string]interface{}{
		"plan_id":  plan.ID,
		"duration": dur.Round(time.Millisecond).String(),
		"autonomy": analytics.AutonomyScore,
	})
}

func sendSSE(w http.ResponseWriter, flusher http.Flusher, event string, data interface{}) {
	jsonData, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, jsonData)
	flusher.Flush()
}

// ── History & Replay ──────────────────────────────────────────────────────

// GetHistory handles GET /v1/pipeline/{plan_id}/history.
// Returns the full immutable event log for a pipeline execution.
func (h *PipelineHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("plan_id")
	if planID == "" {
		writeError(w, http.StatusBadRequest, "plan_id is required")
		return
	}

	if h.history == nil {
		writeError(w, http.StatusServiceUnavailable, "event history not available (durable execution disabled)")
		return
	}

	events, err := h.history.Load(planID)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("history not found for plan %q: %v", planID, err))
		return
	}

	if events == nil {
		events = []pipeline.StepEvent{}
	}

	// Verify integrity
	verified, hashErr := h.history.VerifyIntegrity(planID)
	integrity := map[string]interface{}{
		"verified":  hashErr == nil,
		"count":     verified,
		"total":     len(events),
	}
	if hashErr != nil {
		integrity["error"] = hashErr.Error()
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"plan_id":   planID,
		"events":    events,
		"integrity": integrity,
	})
}

// ReconcileStalePlans scans the history directory for incomplete plans and
// runs reconciliation on each. Returns the total number of tasks fixed.
// Designed to be called on boot for auto-recovery.
func (h *PipelineHandler) ReconcileStalePlans(ctx context.Context) (int, error) {
	if h.history == nil {
		return 0, fmt.Errorf("event history not available")
	}

	// Create a lightweight StepRunner just for reconciliation
	runner := pipeline.NewDurableStepRunner(h.runner, h.history, h.checkpoint, h.recoveryLoop)

	reconciler := pipeline.NewReconciler(runner)
	return reconciler.ReconcileStalePlans(ctx)
}

// ReconcilePlan handles POST /v1/pipeline/{plan_id}/reconcile.
// Manually triggers reconciliation on a specific plan.
func (h *PipelineHandler) ReconcilePlan(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("plan_id")
	if planID == "" {
		writeError(w, http.StatusBadRequest, "plan_id is required")
		return
	}

	if h.history == nil {
		writeError(w, http.StatusServiceUnavailable, "event history not available")
		return
	}

	events, err := h.history.Load(planID)
	if err != nil || len(events) == 0 {
		writeError(w, http.StatusNotFound, fmt.Sprintf("no history for plan %q", planID))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	// Build plan from replay
	plan := &pipeline.Plan{ID: planID}
	replayed := pipeline.ReplayPlan(plan, events)
	reconPlan := &pipeline.Plan{ID: planID, Tasks: replayed.Tasks, Status: replayed.Status}

	runner := pipeline.NewDurableStepRunner(h.runner, h.history, h.checkpoint, h.recoveryLoop)
	runner.SetPlanID(planID)

	reconciler := pipeline.NewReconciler(runner)
	result, recErr := reconciler.Reconcile(ctx, reconPlan)

	if recErr != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"plan_id": planID,
			"status":  "partial",
			"error":   recErr.Error(),
			"fixed":   result.TasksFixed,
			"failed":  result.TasksFailed,
			"cycles":  result.Cycles,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"plan_id":   planID,
		"status":    "reconciled",
		"converged": result.Converged,
		"fixed":     result.TasksFixed,
		"failed":    result.TasksFailed,
		"cycles":    result.Cycles,
	})
}

// GetAnalytics handles GET /v1/pipeline/{plan_id}/analytics.
func (h *PipelineHandler) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("plan_id")
	if planID == "" {
		writeError(w, http.StatusBadRequest, "plan_id is required")
		return
	}
	if h.history == nil {
		writeError(w, http.StatusServiceUnavailable, "event history not available")
		return
	}

	store := pipeline.NewAnalyticsStore(h.history)
	analytics, err := store.Load(planID)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("analytics not found for plan %q: %v", planID, err))
		return
	}

	writeJSON(w, http.StatusOK, analytics)
}

// GetAggregateAnalytics handles GET /v1/pipeline/analytics.
func (h *PipelineHandler) GetAggregateAnalytics(w http.ResponseWriter, r *http.Request) {
	if h.history == nil {
		writeError(w, http.StatusServiceUnavailable, "event history not available")
		return
	}

	plans, err := h.history.ListPlans()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("list plans: %v", err))
		return
	}

	store := pipeline.NewAnalyticsStore(h.history)
	agg := &pipeline.AggregateAnalytics{}

	for _, planID := range plans {
		analytics, loadErr := store.Load(planID)
		if loadErr != nil {
			continue
		}
		agg.Add(analytics)
	}

	writeJSON(w, http.StatusOK, agg)
}

// ReplayPlan handles GET /v1/pipeline/{plan_id}/replay.
// Reconstructs the plan state from its event history.
func (h *PipelineHandler) ReplayPlan(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("plan_id")
	if planID == "" {
		writeError(w, http.StatusBadRequest, "plan_id is required")
		return
	}

	if h.history == nil {
		writeError(w, http.StatusServiceUnavailable, "event history not available (durable execution disabled)")
		return
	}

	events, err := h.history.Load(planID)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("history not found for plan %q: %v", planID, err))
		return
	}

	if len(events) == 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"plan_id": planID,
			"status":  "no_history",
			"message": "No events recorded for this plan",
		})
		return
	}

	// Reconstruct plan from events
	plan := &pipeline.Plan{ID: planID}
	replayed := pipeline.ReplayPlan(plan, events)

	// Build task summary
	tasks := make([]map[string]interface{}, 0, len(replayed.Tasks))
	for _, t := range replayed.Tasks {
		taskMap := map[string]interface{}{
			"id":          t.ID,
			"description": t.Description,
			"agent":       t.Agent,
			"status":      string(t.Status),
		}
		if t.Result != nil {
			taskMap["success"] = t.Result.Success
			if t.Result.Error != "" {
				taskMap["error"] = t.Result.Error
			}
			taskMap["duration_ms"] = t.Result.DurationMs
		}
		tasks = append(tasks, taskMap)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"plan_id": planID,
		"status":  replayed.Status,
		"tasks":   tasks,
		"events":  len(events),
	})
}
