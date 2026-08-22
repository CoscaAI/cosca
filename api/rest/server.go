// Package rest provides the REST API server for the Cosca platform.
package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/api/auth"
	"github.com/CoscaAI/cosca/api/middleware"
	"github.com/CoscaAI/cosca/api/rest/handler"
	"github.com/CoscaAI/cosca/api/stream"
	"github.com/CoscaAI/cosca/internal/agents"
	auditpkg "github.com/CoscaAI/cosca/internal/audit"
	authpkg "github.com/CoscaAI/cosca/internal/auth"
	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/department"
	"github.com/CoscaAI/cosca/internal/grpcclient"
	"github.com/CoscaAI/cosca/internal/kernel"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/orchestration"
	"github.com/CoscaAI/cosca/internal/pipeline"
	pluginspkg "github.com/CoscaAI/cosca/internal/plugins"
	"github.com/CoscaAI/cosca/internal/providers"
	"github.com/CoscaAI/cosca/internal/runtime"
	secretspkg "github.com/CoscaAI/cosca/internal/secrets"
	"github.com/CoscaAI/cosca/internal/skills"
	"github.com/CoscaAI/cosca/internal/trace"
	"github.com/CoscaAI/cosca/internal/workflows"
)

// PipelineServices bundles pipeline components from bootstrap so they can
// be shared between /v1/run (pipeline path) and /v1/pipeline/run endpoints,
// avoiding duplicate creation of orchestrator, planner, step runner, and
// recovery loop.
type PipelineServices struct {
	Enabled bool

	Runner       pipeline.Runner
	Orchestrator orchestration.Orchestrator

	Planner      *pipeline.Planner
	StepRunner   *pipeline.StepRunner
	RecoveryLoop *pipeline.RecoveryLoop
	PostTaskHook *pipeline.PostTaskHook
	CMITracker   *pipeline.CMITracker

	WorkflowHistory *pipeline.WorkflowHistory
	CheckpointStore *pipeline.CheckpointStore
	PluginRegistry  *pipeline.PluginRegistry
}

// Server is the REST API server.
type Server struct {
	mux         *http.ServeMux
	config      Config
	middlewares []func(http.Handler) http.Handler

	// The underlying HTTP server (set when Serve() is called).
	// Uses atomic.Pointer to avoid race between Serve() writing and Shutdown() reading.
	httpServer atomic.Pointer[http.Server]

	// Engine references for health/ready checks.
	knowledgeEngine *knowledge.Engine
	memoryEngine    *memory.MemoryEngine
	runtimeInstance *runtime.Runtime

	// Manager references for the REST API.
	agentsManager    *agents.Manager
	skillsManager    *skills.Manager
	providersManager *providers.Manager
	workflowsManager *workflows.Manager
	pluginsManager   *pluginspkg.Manager

	// Auth references.
	authStore   *authpkg.UserStore
	apiKeyStore *authpkg.APIKeyStore
	jwtSecret   []byte

	// tokenStore tracks active refresh tokens server-side so they can be
	// revoked on logout and rotated with reuse detection on refresh. It is
	// optional (nil-safe): when nil, refresh tokens remain JWT-only.
	tokenStore *authpkg.TokenStore

	// Audit and secrets subsystems.
	auditStore   *auditpkg.Store
	secretsVault *secretspkg.Vault

	// Trace flight recorder (append-only event ledger). Optional (nil-safe):
	// when nil, the trace routes respond 503 and the rest of the server is
	// untouched.
	traceStore *trace.Store

	// Department conversation ledger (append-only Dept→Dept audit trail).
	// Optional (nil-safe): when nil, the department routes report a 503 and the
	// rest of the server is untouched.
	deptStore *department.ConversationStore

	// Kernel kill-switch (emergency stop). Always non-nil after New() —
	// when the caller passes nil, an internal manager is created so the
	// routes never disappear.
	emergencyManager *kernel.EmergencyManager

	// WebSocket hub for real-time event streaming.
	wsHub *stream.Hub

	// WebSocket allowed origin host patterns.
	wsAllowedOrigins []string

	// Pipeline services from bootstrap (nil when pipeline is disabled).
	pipelineServices *PipelineServices
}

// Config configures the REST API server.
type Config struct {
	Host           string
	Port           int
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	MaxHeaderBytes int
	CORSOrigins    string // comma-separated list of allowed CORS origins

	// RegistrationEnabled opts into public self-registration
	// (POST /v1/auth/register). Defaults to false — registration is
	// disabled unless the operator explicitly enables it via
	// COSCA_ENABLE_REGISTRATION=true.
	RegistrationEnabled bool

	// RuntimeClient, when set, makes the runtime handler consult the
	// standalone runtime daemon via gRPC instead of the in-process runtime
	// engine (API-only mode — `cosca serve --api-only`, FASE 2
	// DDNA-2026-08-07-001). Nil keeps the historical in-process behavior.
	RuntimeClient *grpcclient.RuntimeClient
}

// DefaultConfig returns a default REST API server configuration.
func DefaultConfig() Config {
	return Config{
		// Security hardening: bind to loopback only. The server is NOT
		// exposed on the network unless the operator explicitly passes
		// --host 0.0.0.0 (or an equivalent config value).
		Host:           "127.0.0.1",
		Port:           14120,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   60 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
		// CORSOrigins defaults to "" (empty) — fail-closed: no cross-origin
		// requests are allowed until the operator explicitly lists origins
		// via --cors-origins / COSCA_CORS_ORIGINS.
	}
}

// New creates a new REST API server with the given engines, managers, and
// auth dependencies. knowledgeEngine, memoryEngine, runtimeInstance, and
// all managers can be nil if not available. authStore must be non-nil.
// apiKeyStore may be nil if API key support is not needed.
// tokenStore may be nil — when nil, refresh tokens are JWT-only (no
// server-side revocation / rotation reuse detection).
// auditStore and secretsVault are optional and may be nil.
// traceStore is optional (nil-safe): when nil the trace routes report a
// clear error and the rest of the server is unaffected.
// deptStore is optional (nil-safe): when nil the department routes report a
// clear error and the rest of the server is unaffected.
// emergencyMgr is optional — when nil, an internal EmergencyManager is
// created so the kill-switch routes always exist (the serve command passes
// its wired manager so the emergency can reach the shutdown path).
// pluginsMgr may be nil if plugin system is not available.
// enableWebsocket controls whether the WebSocket Hub is created.
func New(
	knowledgeEngine *knowledge.Engine,
	memoryEngine *memory.MemoryEngine,
	runtimeInstance *runtime.Runtime,
	agentsMgr *agents.Manager,
	skillsMgr *skills.Manager,
	providersMgr *providers.Manager,
	workflowsMgr *workflows.Manager,
	authStore *authpkg.UserStore,
	apiKeyStore *authpkg.APIKeyStore,
	jwtSecret []byte,
	cfg Config,
	tokenStore *authpkg.TokenStore,
	auditStore *auditpkg.Store,
	secretsVault *secretspkg.Vault,
	emergencyMgr *kernel.EmergencyManager,
	pluginsMgr *pluginspkg.Manager,
	enableWebsocket bool,
	wsAllowedOrigins []string,
	traceStore *trace.Store,
	deptStore *department.ConversationStore,
	pipelineSvc *PipelineServices,
) *Server {
	// Create the WebSocket hub if enabled. It starts a background goroutine
	// immediately but idles until the first connection arrives.
	var wsHub *stream.Hub
	if enableWebsocket {
		wsHub = stream.NewHub(zerolog.Nop())
	}

	// The kill switch must always exist once the API is up — even when the
	// caller did not wire a manager, the endpoints respond (and report the
	// state) so the Don can always verify the daemon's emergency posture.
	if emergencyMgr == nil {
		emergencyMgr = kernel.NewEmergencyManager()
	}

	s := &Server{
		mux:              http.NewServeMux(),
		config:           cfg,
		knowledgeEngine:  knowledgeEngine,
		memoryEngine:     memoryEngine,
		runtimeInstance:  runtimeInstance,
		agentsManager:    agentsMgr,
		skillsManager:    skillsMgr,
		providersManager: providersMgr,
		workflowsManager: workflowsMgr,
		pluginsManager:   pluginsMgr,
		authStore:        authStore,
		apiKeyStore:      apiKeyStore,
		jwtSecret:        jwtSecret,
		tokenStore:       tokenStore,
		auditStore:       auditStore,
		secretsVault:     secretsVault,
		traceStore:       traceStore,
		deptStore:        deptStore,
		emergencyManager: emergencyMgr,
		wsHub:            wsHub,
		wsAllowedOrigins: wsAllowedOrigins,
		pipelineServices: pipelineSvc,
	}
	s.registerRoutes(knowledgeEngine, memoryEngine, runtimeInstance, pipelineSvc)
	return s
}

// Use adds middleware to the server's handler chain.
// Middleware is applied in the order added (outermost first).
func (s *Server) Use(mw func(http.Handler) http.Handler) {
	s.middlewares = append(s.middlewares, mw)
}

// publicPaths lists the paths that do not require authentication or CSRF.
var publicPaths = []string{
	"/health",
	"/ready",
	"/v1/auth/login",
	"/v1/auth/register",
	"/v1/auth/refresh",
	"/v1/auth/logout",
	"/v1/csrf-token",
	"/v1/ws",
}

// registerRoutes registers all API routes.
func (s *Server) registerRoutes(k *knowledge.Engine, m *memory.MemoryEngine, rt *runtime.Runtime, pipelineSvc *PipelineServices) {
	kh := handler.NewKnowledgeHandler(k, s.auditStore)
	mh := handler.NewMemoryHandler(m, s.auditStore)

	// Single Owner Model (Fase 3): when a runtime daemon is configured,
	// delegate knowledge and memory operations via gRPC instead of using
	// the local engines (which become fallbacks only).
	if s.config.RuntimeClient != nil {
		knowledgeClient := grpcclient.NewKnowledgeClient(s.config.RuntimeClient.Addr())
		kh.SetKnowledgeClient(knowledgeClient)
		memoryClient := grpcclient.NewMemoryClient(s.config.RuntimeClient.Addr())
		mh.SetMemoryClient(memoryClient)
	}

	rh := handler.NewRuntimeHandlerWithClient(rt, s.config.RuntimeClient)
	ah := handler.NewAgentsHandler(s.agentsManager)
	sh := handler.NewSkillsHandler(s.skillsManager, s.auditStore)
	ph := handler.NewProvidersHandler(s.providersManager)
	wh := handler.NewWorkflowsHandler(s.workflowsManager, s.auditStore)

	// Auth handlers. The refresh-token store (optional, nil-safe) enables
	// server-side revocation on logout and rotation reuse detection.
	authH := handler.NewAuthHandler(s.authStore, s.jwtSecret, s.auditStore, s.tokenStore)
	// Public self-registration is opt-in (security): disabled unless the
	// operator set COSCA_ENABLE_REGISTRATION=true (wired by serve.go).
	authH.RegistrationEnabled = s.config.RegistrationEnabled
	usersH := handler.NewUsersHandler(s.authStore, s.auditStore)

	// Role-gated route helpers. The role hierarchy is admin > editor >
	// viewer (see api/auth/rbac.go). Admin bypasses every check.
	adminOnly := auth.RequireRole(auth.RoleAdmin)
	editorOnly := auth.RequireRole(auth.RoleEditor)

	// Knowledge endpoints
	s.mux.HandleFunc("POST /v1/knowledge/search", kh.Search)
	// Indexing and sync mutate the knowledge base — editor+ only.
	s.mux.Handle("POST /v1/knowledge/index", editorOnly(http.HandlerFunc(kh.Index)))
	s.mux.HandleFunc("GET /v1/knowledge/stats", kh.Stats)
	s.mux.HandleFunc("GET /v1/knowledge/epistemology", kh.Epistemology)
	s.mux.HandleFunc("GET /v1/knowledge/epistemology/{status}", kh.EpistemologyByStatus)
	s.mux.Handle("POST /v1/knowledge/sync", editorOnly(http.HandlerFunc(kh.Sync)))
	s.mux.Handle("POST /v1/knowledge/sync/stream", editorOnly(http.HandlerFunc(kh.SyncStream)))
	s.mux.HandleFunc("GET /v1/symbols/search", kh.SymbolsSearch)

	// Memory endpoints
	// Store/delete/promote mutate memory — editor+ only. Search/get are reads.
	s.mux.Handle("POST /v1/memory/store", editorOnly(http.HandlerFunc(mh.Store)))
	s.mux.Handle("GET /v1/memory/search", handler.RequireMemoryScope(http.HandlerFunc(mh.Search)))
	s.mux.Handle("GET /v1/memory/get", handler.RequireMemoryScope(http.HandlerFunc(mh.Get)))
	// Legacy SDK alias retained while clients migrate to /memory/get.
	s.mux.Handle("GET /v1/memory/{id}", handler.RequireMemoryScope(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		q.Set("id", r.PathValue("id"))
		r.URL.RawQuery = q.Encode()
		mh.Get(w, r)
	})))
	s.mux.Handle("DELETE /v1/memory/delete", editorOnly(http.HandlerFunc(mh.Delete)))
	s.mux.Handle("POST /v1/memory/promote", editorOnly(http.HandlerFunc(mh.Promote)))
	s.mux.Handle("GET /v1/memory/stats", handler.RequireMemoryScope(http.HandlerFunc(mh.Stats)))

	// Runtime endpoints
	s.mux.HandleFunc("GET /v1/status", rh.Status)
	s.mux.HandleFunc("GET /v1/health", rh.Health)
	s.mux.HandleFunc("GET /v1/status/stream", rh.StatusStream)

	// AgentBridge endpoints (control plane for agent sessions + bridge)
	abh := handler.NewAgentBridgeHandler(nil, nil, nil)
	s.mux.HandleFunc("GET /v1/agentbridge/status", abh.Status)
	s.mux.HandleFunc("GET /v1/agentbridge/sessions", abh.Sessions)
	s.mux.HandleFunc("GET /v1/agentbridge/sessions/{id}/events", abh.SessionEvents)
	s.mux.HandleFunc("GET /v1/agentbridge/sessions/{id}/events/stream", abh.SessionEventsStream)
	// Action endpoints mutate state — editor+ only.
	s.mux.Handle("POST /v1/agentbridge/sessions", editorOnly(http.HandlerFunc(abh.CreateSession)))
	s.mux.Handle("POST /v1/agentbridge/sessions/{id}/detach", editorOnly(http.HandlerFunc(abh.DetachSession)))
	s.mux.Handle("POST /v1/agentbridge/sessions/{id}/resume", editorOnly(http.HandlerFunc(abh.ResumeSession)))
	s.mux.Handle("POST /v1/agentbridge/sessions/{id}/stop", editorOnly(http.HandlerFunc(abh.StopSession)))
	s.mux.Handle("DELETE /v1/agentbridge/sessions/{id}", editorOnly(http.HandlerFunc(abh.DestroySession)))
	s.mux.Handle("POST /v1/agentbridge/sessions/{id}/events", editorOnly(http.HandlerFunc(abh.EmitEvent)))

	// Wire WebSocket Hub for real-time broadcasts.
	rh.SetHub(s.wsHub)
	kh.SetHub(s.wsHub)
	wh.SetHub(s.wsHub)

	// Agents endpoints
	s.mux.HandleFunc("GET /v1/agents", ah.List)
	s.mux.HandleFunc("GET /v1/agents/search", ah.Search)
	s.mux.HandleFunc("GET /v1/agents/{name}", ah.Get)

	// Skills endpoints
	s.mux.HandleFunc("GET /v1/skills", sh.List)
	s.mux.HandleFunc("GET /v1/skills/search", sh.Search)
	s.mux.HandleFunc("GET /v1/skills/{name}", sh.Get)

	// Providers endpoints
	s.mux.HandleFunc("GET /v1/providers", ph.List)
	s.mux.HandleFunc("GET /v1/providers/{name}", ph.Get)
	// Provider connectivity tests can trigger external calls — editor+ only.
	s.mux.Handle("POST /v1/providers/{name}/test", editorOnly(http.HandlerFunc(ph.Test)))
	// Switching the global active provider affects every LLM request — admin only.
	s.mux.Handle("PUT /v1/providers/active", adminOnly(http.HandlerFunc(ph.SetActive)))

	// Workflows endpoints
	s.mux.HandleFunc("GET /v1/workflows", wh.List)
	s.mux.HandleFunc("GET /v1/workflows/search", wh.Search)
	s.mux.HandleFunc("GET /v1/workflows/{name}", wh.Get)
	// Workflow execution consumes LLM tokens — editor+ only.
	s.mux.Handle("POST /v1/workflows/{name}/run", editorOnly(http.HandlerFunc(wh.Run)))
	s.mux.Handle("POST /v1/workflows/{name}/run/stream", editorOnly(http.HandlerFunc(wh.RunStream)))

	// Auth endpoints (public — login, register, and refresh do not require a token).
	s.mux.HandleFunc("POST /v1/auth/login", authH.Login)
	s.mux.HandleFunc("POST /v1/auth/register", authH.Register)
	s.mux.HandleFunc("POST /v1/auth/refresh", authH.Refresh)
	s.mux.HandleFunc("POST /v1/auth/logout", authH.Logout)
	// GET /v1/auth/me requires a valid token (checked by Middleware).
	s.mux.HandleFunc("GET /v1/auth/me", authH.Me)
	// POST /v1/auth/change-password requires a valid token (NOT a public
	// path — the auth middleware protects it, and the CSRF middleware
	// protects it from cross-site password changes).
	s.mux.HandleFunc("POST /v1/auth/change-password", authH.ChangePassword)
	// CSRF token endpoint — returns a token and sets a cookie for
	// double-submit CSRF protection.
	s.mux.HandleFunc("GET /v1/csrf-token", authH.CSRF)

	// User management endpoints (admin only).
	s.mux.Handle("GET /v1/users", adminOnly(http.HandlerFunc(usersH.List)))
	s.mux.Handle("POST /v1/users", adminOnly(http.HandlerFunc(usersH.Create)))
	s.mux.Handle("DELETE /v1/users/{id}", adminOnly(http.HandlerFunc(usersH.Delete)))
	s.mux.Handle("PUT /v1/users/{id}/role", adminOnly(http.HandlerFunc(usersH.UpdateRole)))

	// Skill install requires admin role.
	s.mux.Handle("POST /v1/skills/{name}/install", adminOnly(http.HandlerFunc(sh.Install)))

	// Run endpoints — execute AI orchestration prompts.
	// Execution burns LLM tokens — editor+ only.
	// Uses the global chat registry (singleton, populated by provider init).
	// The orchestration engine is wired with the same memory/knowledge/skills
	// components as `cosca run` so /v1/run injects memory + knowledge.
	runH := handler.NewRunHandler(s.agentsManager, chat.GetRegistry(), s.auditStore)
	runH.SetHub(s.wsHub)
	runH.SetSkillsManager(s.skillsManager)

	// Knowledge: prefer gRPC to runtime daemon (Single Owner Model).
	// Falls back to local knowledge engine when the daemon is not reachable.
	var knowledgeSearcher orchestration.KnowledgeSearcher
	if s.config.RuntimeClient != nil {
		knowledgeClient := grpcclient.NewKnowledgeClient(s.config.RuntimeClient.Addr())
		knowledgeSearcher = grpcclient.NewKnowledgeSearcherAdapter(knowledgeClient)
		runH.SetKnowledgeSearcher(knowledgeSearcher)

		// Memory: same pattern — gRPC to runtime, fallback to local.
		memoryClient := grpcclient.NewMemoryClient(s.config.RuntimeClient.Addr())
		runH.SetMemoryRetriever(grpcclient.NewMemoryRetrieverAdapter(memoryClient))
		runH.SetMemoryStorer(grpcclient.NewMemoryStorerAdapter(memoryClient))
	} else {
		runH.SetKnowledgeEngine(s.knowledgeEngine)
		runH.SetMemoryEngine(s.memoryEngine)
	}

	// When pipeline is enabled, wire pipeline components into /v1/run.
	// Pipeline mode routes through Planner → StepRunner → RecoveryLoop.
	// Provider overrides (req.Provider != "") fall back to the legacy
	// direct-to-LLM path.
	if pipelineSvc != nil && pipelineSvc.Enabled {
		runH.SetPipelineRunner(pipelineSvc.Runner)
		runH.SetPipelinePlanner(pipelineSvc.Planner)
		runH.SetPipelineComponents(
			pipelineSvc.StepRunner,
			pipelineSvc.RecoveryLoop,
			pipelineSvc.PostTaskHook,
			pipelineSvc.CMITracker,
			pipelineSvc.WorkflowHistory,
			pipelineSvc.CheckpointStore,
		)
	}

	s.mux.Handle("POST /v1/run", editorOnly(http.HandlerFunc(runH.Execute)))
	s.mux.Handle("POST /v1/run/stream", editorOnly(http.HandlerFunc(runH.Stream)))

	// Pipeline endpoint — autonomous task decomposition, execution,
	// build/test validation, recovery loop, and auto-evolution.
	// Reuses shared pipeline components from bootstrap when available;
	// otherwise creates them from scratch (backward compat).
	{
		var pipelineH *handler.PipelineHandler

		if pipelineSvc != nil && pipelineSvc.Enabled {
			// Reuse bootstrap pipeline components — no duplicate creation.
			handoffStore, _ := pipeline.NewHandoffStore(".cosca/handoffs/")

			pipelineH = handler.NewPipelineHandler(
				pipelineSvc.Runner, pipelineSvc.Planner, pipelineSvc.RecoveryLoop,
				pipelineSvc.PostTaskHook, pipelineSvc.CMITracker, handoffStore, s.auditStore,
			)
			// Wire durable execution from bootstrap if available.
			if pipelineSvc.WorkflowHistory != nil && pipelineSvc.CheckpointStore != nil {
				pipelineH.SetDurable(
					pipelineSvc.WorkflowHistory,
					pipelineSvc.CheckpointStore,
					pipelineSvc.PluginRegistry,
				)
				// On boot, reconcile any plans left incomplete from prior crash.
				go func() {
					time.Sleep(2 * time.Second)
					fixed, err := pipelineH.ReconcileStalePlans(context.Background())
					if err != nil {
						zlog.Warn().Err(err).Msg("pipeline: boot reconciliation failed")
					} else if fixed > 0 {
						zlog.Info().Int("fixed", fixed).Msg("pipeline: boot reconciliation completed")
					}
				}()
			}
		}

		if pipelineH != nil {
			s.mux.Handle("POST /v1/pipeline/run", editorOnly(http.HandlerFunc(pipelineH.ExecutePipeline)))
			s.mux.Handle("POST /v1/pipeline/run/stream", editorOnly(http.HandlerFunc(pipelineH.ExecutePipelineStream)))
			// Event history & replay — read-only, editor+ (contains LLM output).
			s.mux.Handle("GET /v1/pipeline/{plan_id}/history", editorOnly(http.HandlerFunc(pipelineH.GetHistory)))
			s.mux.Handle("GET /v1/pipeline/{plan_id}/replay", editorOnly(http.HandlerFunc(pipelineH.ReplayPlan)))
			s.mux.Handle("POST /v1/pipeline/{plan_id}/reconcile", editorOnly(http.HandlerFunc(pipelineH.ReconcilePlan)))
			s.mux.Handle("GET /v1/pipeline/{plan_id}/analytics", editorOnly(http.HandlerFunc(pipelineH.GetAnalytics)))
			s.mux.Handle("GET /v1/pipeline/analytics", editorOnly(http.HandlerFunc(pipelineH.GetAggregateAnalytics)))
		}
	}

	// Execution history endpoints — contain LLM prompts of all users,
	// so they are restricted to editor+.
	execH := handler.NewExecutionsHandler(orchestration.GetExecutionStore())
	s.mux.Handle("GET /v1/executions", editorOnly(http.HandlerFunc(execH.List)))
	s.mux.Handle("GET /v1/executions/{id}", editorOnly(http.HandlerFunc(execH.Get)))

	// Trace endpoints — execution timeline + replay over the append-only
	// flight recorder (.cosca/trace.db). Read-only; editor+ (the timeline
	// carries operation context across all users). Nil-safe: when the store
	// is unavailable the routes report a clear error.
	tracesH := handler.NewTracesHandler(s.traceStore)
	s.mux.Handle("GET /v1/traces", editorOnly(http.HandlerFunc(tracesH.List)))
	s.mux.Handle("GET /v1/traces/{id}", editorOnly(http.HandlerFunc(tracesH.Get)))
	s.mux.Handle("GET /v1/traces/{id}/replay", editorOnly(http.HandlerFunc(tracesH.Replay)))
	s.mux.Handle("GET /v1/traces/{id}/causal", editorOnly(http.HandlerFunc(tracesH.Causal)))

	// Department endpoints — inter-department conversations (Dept→Dept) over
	// the append-only audit ledger (.cosca/department.db). List/Thread are
	// read-only projections for the Control Center; Ask appends a question —
	// a write, gated editor+. Nil-safe: when the store is unavailable the
	// routes report a clear error.
	departmentsH := handler.NewDepartmentsHandler(s.deptStore)
	s.mux.HandleFunc("GET /v1/departments", departmentsH.List)
	s.mux.HandleFunc("GET /v1/departments/threads/{thread}", departmentsH.Thread)
	s.mux.Handle("POST /v1/departments/ask", editorOnly(http.HandlerFunc(departmentsH.Ask)))

	// Analytics endpoints — aggregate queries across all users, admin only.
	analyticsH := handler.NewAnalyticsHandler(s.auditStore)
	s.mux.Handle("GET /v1/analytics", adminOnly(http.HandlerFunc(analyticsH.GetAnalytics)))

	// Aggregated platform statistics endpoint.
	statsH := handler.NewStatsHandler(s.agentsManager, s.skillsManager, s.providersManager, s.workflowsManager, s.runtimeInstance)
	s.mux.HandleFunc("GET /v1/stats", statsH.Get)

	// System telemetry endpoints — read-only hardware + status for the Command
	// Center. The probe is stateless and best-effort (never 500s).
	sysH := handler.NewSystemHandler()
	s.mux.HandleFunc("GET /v1/system/hardware", sysH.Hardware)
	s.mux.HandleFunc("GET /v1/system/status", sysH.Status)

	// Plugin endpoints.
	pluginsH := handler.NewPluginsHandler(s.pluginsManager)
	s.mux.HandleFunc("GET /v1/plugins", pluginsH.List)

	// API Key endpoints (admin only).
	apiKeysH := handler.NewAPIKeysHandler(s.apiKeyStore, s.auditStore)
	s.mux.Handle("GET /v1/api-keys", adminOnly(http.HandlerFunc(apiKeysH.List)))
	s.mux.Handle("POST /v1/api-keys", adminOnly(http.HandlerFunc(apiKeysH.Create)))
	s.mux.Handle("DELETE /v1/api-keys/{id}", adminOnly(http.HandlerFunc(apiKeysH.Revoke)))

	// Audit log endpoints (admin only).
	if s.auditStore != nil {
		auditH := handler.NewAuditHandler(s.auditStore)
		s.mux.Handle("GET /v1/audit/logs", adminOnly(http.HandlerFunc(auditH.List)))
		s.mux.Handle("GET /v1/audit/logs/{id}", adminOnly(http.HandlerFunc(auditH.Get)))
		s.mux.Handle("POST /v1/audit/prune", adminOnly(http.HandlerFunc(auditH.Prune)))
	}

	// Secrets vault endpoints (admin only).
	if s.secretsVault != nil {
		secretsH := handler.NewSecretsHandler(s.secretsVault, s.auditStore)
		s.mux.Handle("GET /v1/secrets", adminOnly(http.HandlerFunc(secretsH.List)))
		s.mux.Handle("POST /v1/secrets", adminOnly(http.HandlerFunc(secretsH.CreateOrUpdate)))
		s.mux.Handle("GET /v1/secrets/{key}", adminOnly(http.HandlerFunc(secretsH.Get)))
		s.mux.Handle("DELETE /v1/secrets/{key}", adminOnly(http.HandlerFunc(secretsH.Delete)))
	}

	// Kernel kill-switch endpoints (admin only) — the emergency mechanism
	// that lets the Don stop the daemon remotely. These are always
	// registered; the manager is guaranteed non-nil by New().
	emergencyH := handler.NewEmergencyHandler(s.emergencyManager, s.auditStore)
	s.mux.Handle("GET /v1/kernel/emergency", adminOnly(http.HandlerFunc(emergencyH.Status)))
	s.mux.Handle("POST /v1/kernel/emergency/stop", adminOnly(http.HandlerFunc(emergencyH.Stop)))
	s.mux.Handle("POST /v1/kernel/emergency/halt", adminOnly(http.HandlerFunc(emergencyH.Halt)))

	// Root-level health check endpoints (Kubernetes probes)
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /ready", s.handleReady)

	// WebSocket real-time event gateway.
	wsH := handler.NewWebSocketHandler(s.wsHub, s.jwtSecret, zlog.Logger, s.wsAllowedOrigins)
	s.mux.Handle("GET /v1/ws", wsH)
}

// handleHealth handles GET /health — simple liveness probe for Kubernetes.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]bool{"healthy": true}); err != nil {
		log.Printf("health endpoint: encode error: %v", err)
	}
}

// handleReady handles GET /ready — readiness probe with subsystem checks.
func (s *Server) handleReady(w http.ResponseWriter, _ *http.Request) {
	ready := true
	subsystems := make(map[string]string)

	// Check knowledge engine availability.
	if s.knowledgeEngine != nil {
		subsystems["knowledge"] = "available"
	} else {
		subsystems["knowledge"] = "unavailable"
		ready = false
	}

	// Check memory engine availability.
	if s.memoryEngine != nil {
		subsystems["memory"] = "available"
	} else {
		subsystems["memory"] = "unavailable"
		ready = false
	}

	// Check runtime availability.
	if s.runtimeInstance != nil {
		health := s.runtimeInstance.Health()
		subsystems["runtime"] = string(health)
		if health != runtime.StatusHealthy {
			ready = false
		}
	} else {
		subsystems["runtime"] = "unavailable"
		ready = false // runtime indisponível NÃO é "pronto" — readiness deve refletir
	}

	w.Header().Set("Content-Type", "application/json")
	if ready {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"ready":      ready,
		"subsystems": subsystems,
	}); err != nil {
		log.Printf("ready endpoint: encode error: %v", err)
	}
}

// buildHandler constructs the final http.Handler by wrapping the mux with
// all registered middlewares (outermost first).
//
// Effective chain (outside → in):
//
//	CORS → SecurityHeaders → Auth → CSRF → RateLimiter → Logging → Metrics → mux
//
// CORS is the outermost layer so that OPTIONS preflight requests are
// handled (204 No Content with CORS headers) before any auth, CSRF,
// rate-limit, or security-headers middleware can reject them. This is
// required because the browser sends preflight OPTIONS without any
// authentication context (no cookies, no tokens).
func (s *Server) buildHandler() http.Handler {
	handler := http.Handler(s.mux)

	// Apply custom middlewares in reverse order (first added = closest to mux).
	for i := len(s.middlewares) - 1; i >= 0; i-- {
		handler = s.middlewares[i](handler)
	}

	// Apply rate limiting — checked before auth for all paths.
	// Login gets a stricter limit (5 req/min), others get 100 req/min.
	handler = middleware.RateLimitMiddleware()(handler)

	// Apply CSRF middleware after rate limiting, before auth.
	// This ensures CSRF is checked for authenticated requests only
	// (auth passes through public paths, then CSRF also skips them).
	handler = middleware.CSRFMiddleware(publicPaths)(handler)

	// Apply auth middleware after rate limiting and CSRF. It is ALWAYS
	// applied — fail-closed. With an empty jwtSecret the middleware rejects
	// protected routes (see auth.Middleware), so a misconfigured server can
	// never silently expose the API without authentication.
	handler = auth.Middleware(s.jwtSecret, publicPaths, s.apiKeyStore)(handler)

	// Security headers are set on every response regardless of auth status.
	handler = middleware.SecurityHeadersMiddleware()(handler)

	// CORS must be the outermost layer so OPTIONS preflight requests are
	// handled before any auth or security middleware can reject them.
	handler = middleware.CORSMiddleware(s.config.CORSOrigins)(handler)

	// Recovery MUST be the absolute outermost layer so it catches panics
	// from ALL downstream handlers and middleware (including CORS).
	handler = middleware.RecoveryMiddleware(handler)

	return handler
}

// Serve starts the HTTP server and blocks until it stops.
// Use Shutdown() for graceful shutdown.
func (s *Server) Serve() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	srv := &http.Server{
		Addr:           addr,
		Handler:        s.buildHandler(),
		ReadTimeout:    s.config.ReadTimeout,
		WriteTimeout:   s.config.WriteTimeout,
		IdleTimeout:    s.config.IdleTimeout,
		MaxHeaderBytes: s.config.MaxHeaderBytes,
	}
	s.httpServer.Store(srv)
	return srv.ListenAndServe()
}

// ServeTLS starts the HTTPS server with the given certificate and key files.
// It blocks until the server stops. Use Shutdown() for graceful shutdown.
func (s *Server) ServeTLS(certFile, keyFile string) error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	srv := &http.Server{
		Addr:           addr,
		Handler:        s.buildHandler(),
		ReadTimeout:    s.config.ReadTimeout,
		WriteTimeout:   s.config.WriteTimeout,
		IdleTimeout:    s.config.IdleTimeout,
		MaxHeaderBytes: s.config.MaxHeaderBytes,
	}
	s.httpServer.Store(srv)
	return srv.ListenAndServeTLS(certFile, keyFile)
}

// Shutdown gracefully shuts down the HTTP server, waiting for active
// connections to complete or the context to expire.
// WebSocket connections are drained before the HTTP server is stopped.
func (s *Server) Shutdown(ctx context.Context) error {
	// Shutdown WebSocket hub first — sends close frames to all active
	// connections and waits for them to drain.
	if s.wsHub != nil {
		if err := s.wsHub.Shutdown(ctx); err != nil {
			log.Printf("websocket hub shutdown error: %v", err)
		}
	}

	if srv := s.httpServer.Load(); srv != nil {
		return srv.Shutdown(ctx)
	}
	return nil
}

// Mux returns the underlying http.ServeMux for advanced usage.
func (s *Server) Mux() *http.ServeMux {
	return s.mux
}

// Handler returns the full handler chain (mux wrapped with middleware).
// This is useful when you need to start the server manually.
func (s *Server) Handler() http.Handler {
	return s.buildHandler()
}

// Addr returns the configured listen address.
func (s *Server) Addr() string {
	return fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
}
