// Package rest provides the REST API server for the Cosca platform.
package rest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	"github.com/CoscaAI/cosca/internal/brainweb"
	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/cost"
	"github.com/CoscaAI/cosca/internal/department"
	"github.com/CoscaAI/cosca/internal/grpcclient"
	"github.com/CoscaAI/cosca/internal/kernel"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/modlink"
	"github.com/CoscaAI/cosca/internal/orchestration"
	"github.com/CoscaAI/cosca/internal/perception"
	"github.com/CoscaAI/cosca/internal/perception/bus"
	"github.com/CoscaAI/cosca/internal/pipeline"
	pluginspkg "github.com/CoscaAI/cosca/internal/plugins"
	"github.com/CoscaAI/cosca/internal/providers"
	"github.com/CoscaAI/cosca/internal/runtime"
	secretspkg "github.com/CoscaAI/cosca/internal/secrets"
	"github.com/CoscaAI/cosca/internal/skills"
	"github.com/CoscaAI/cosca/internal/trace"
	"github.com/CoscaAI/cosca/internal/vision"
	"github.com/CoscaAI/cosca/internal/workflows"
	coscapkg "github.com/CoscaAI/cosca/pkg/cosca"
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

	// routeResolver is the deterministic modular router (FASE 1 routing/scope).
	// Nil = legacy (no routing). Configured via SetModularSearch so the
	// /v1/knowledge/search handler can confine the search to a routed scope.
	routeResolver *modlink.Resolver
	// searchMode é o modo de busca ("legacy" | "modular").
	searchMode string

	// perceptionSvc is the continuous Perception Loop service (screen → vision →
	// perceptual WorldState → SSE). Nil-safe: when nil (or disabled), the
	// /v1/perception/* endpoints report a clear 503. Set via SetPerceptionService.
	perceptionSvc *perception.Service

	// busSvc is the Perception Bus — the multimodal (vision+audio) synchroniser
	// (FASE A). Nil-safe: when nil (or disabled), the /v1/perception/bus*
	// endpoints report a clear 503. Set via SetBusService.
	busSvc *bus.Bus
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

	// ActivityLogPath, when set, overrides the activity log source path
	// (.cosca/activity.jsonl) used by the /v1/cognitive/activity feed. If empty,
	// the default `<cwd>/.cosca/activity.jsonl` is used. Injectable so tests
	// can point the observatory at a temporary file.
	ActivityLogPath string

	// DeliberateConfig opts the /v1/run engine into the Kernel-First
	// Deliberation stage (ADR-032). Zero-value (unset) is fail-closed:
	// Enabled=false preserves the legacy path exactly.
	DeliberateConfig orchestration.DeliberateConfig
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

// SetModularSearch configura o roteador determinístico (modlink) e o modo de
// busca no servidor REST (FASE 1 routing/scope). Chamado pelo serve com o
// boot.RouteResolver de bootstrap. Passar nil restaura o comportamento legacy.
func (s *Server) SetModularSearch(resolver *modlink.Resolver, mode string) {
	s.routeResolver = resolver
	s.searchMode = mode
}

// SetPerceptionService injects the continuous Perception Loop service into the
// REST server, enabling the /v1/perception/* endpoints. Pass nil (or leave
// unset) to keep them disabled (503). The service must already be Started by
// the caller; the server only reads its State and subscribes to its stream.
func (s *Server) SetPerceptionService(svc *perception.Service) {
	s.perceptionSvc = svc
}

// SetBusService injects the Perception Bus (multimodal synchroniser, FASE A)
// into the REST server, enabling the /v1/perception/bus* endpoints. Pass nil
// (or leave unset) to keep them disabled (503). The bus must already be Started
// by the caller; the server only reads its State and subscribes to its stream.
func (s *Server) SetBusService(svc *bus.Bus) {
	s.busSvc = svc
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
	// FASE 1 routing/scope: injeta o roteador determinístico e o modo no handler
	// de /v1/knowledge/search (modular → confina; NoRoute → vazio + no_route).
	if s.routeResolver != nil {
		kh.SetRouteResolver(s.routeResolver)
		kh.SetSearchMode(s.searchMode)
	}
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
	// ADR-032: opt-in via server config. Zero-value is fail-closed — the
	// /v1/run engine stays on the legacy path unless explicitly enabled.
	runH.SetDeliberateConfig(s.config.DeliberateConfig)

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

	// Shadow / Decision / Deliberation endpoints — os contratos de leitura do
	// painel "Casa Visível" (L1/L2/L3) sobre o Cognitive Shadow ledger
	// (deliberação contrafactual) e o flight recorder causal. Read-only.
	//
	// O store do shadow não é injetado no Server; os handlers caem no
	// shadow.DefaultStore() (diretório `.cosca` do projeto) — nil-safe, lê de
	// .cosca/shadow/records.jsonl. O store do trace (s.traceStore) é injetado e
	// opcional: quando nil, o enriquecimento causal do decision trace é omitido.
	//
	// Metrologia agregada (L1/L2) — acessível a qualquer sessão autenticada
	// (dados sanitizados, sem prompts/args). O Decision Trace completo (L3) pode
	// expor detalhes de eventos do flight recorder de todos os usuários, por
	// isso é editor+.
	shadowH := handler.NewShadowHandler(nil)
	decisionH := handler.NewDecisionHandler(nil, s.traceStore)
	s.mux.HandleFunc("GET /v1/shadow/summary", shadowH.Summary)
	s.mux.HandleFunc("GET /v1/shadow/histogram", shadowH.Histogram)
	s.mux.HandleFunc("GET /v1/shadow/decisions", shadowH.Decisions)
	s.mux.HandleFunc("GET /v1/shadow/decisions/{id}", shadowH.Get)
	s.mux.HandleFunc("GET /v1/decisions", decisionH.List)
	s.mux.HandleFunc("GET /v1/deliberation/stats", decisionH.Stats)
	s.mux.Handle("GET /v1/decisions/{id}", editorOnly(http.HandlerFunc(decisionH.Get)))

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

	// Contratos de leitura do "cérebro" Cosca (consumidos pelo dashboard
	// "Casa Visível" — app separada). A UI do visualizador 3D embutida
	// (go:embed "web/") foi MOVIDA para esse dashboard; o root NÃO serve
	// mais user interface. Estes endpoints mantêm EXATAMENTE os data
	// contracts de antes (Graph/Observatory/Activity/Perception) e são
	// read-only + sanitizados (nada de prompts/args/instructions).
	//
	// GET /v1/organization/graph        → organograma (agents = capos, skills = soldados)
	// GET /v1/cognitive/observatory     → Observatório Cognitivo (constelações + trace causal)
	// GET /v1/cognitive/activity        → ações recentes do cérebro
	// GET /v1/cognitive/perception      → percepção determinística frame-a-frame
	//
	// Leitura autenticada (qualquer sessão válida), não editorOnly — são
	// projeções read-only sem conteúdo sensível de usuário.
	brainH := brainweb.NewHandler(s.agentsManager, s.skillsManager, coscapkg.Version).
		WithActivity(newExecutionActivitySource(s.config.ActivityLogPath)).
		WithObservatory(knowledgeItemsFn(k), traceReplayFn(s.traceStore), cognitiveStatsFn(s, rt)).
		WithPerception(newPerceptionSource()).
		WithCost(coscaCostStore())
	s.mux.HandleFunc("GET /v1/organization/graph", brainH.Graph)
	s.mux.HandleFunc("GET /v1/cognitive/observatory", brainH.Observatory)
	s.mux.HandleFunc("GET /v1/cognitive/activity", brainH.Activity)
	s.mux.HandleFunc("GET /v1/cognitive/perception", brainH.Perception)

	// Perception Loop endpoints (continuous screen → vision → world-state).
	// Read-only, always registered; nil/disabled service → 503 (opt-in). They
	// are NOT in publicPaths, so the auth middleware protects them — a live
	// screen-capture feed is sensitive by nature.
	// NOTE: registered as a lazy closure that reads s.perceptionSvc per request,
	// because SetPerceptionService wires it AFTER New(). Capturing it at New()
	// time would leave the handler stuck with nil (always 503). Same pattern as
	// the perception bus below.
	s.mux.HandleFunc("GET /v1/perception/state", func(w http.ResponseWriter, r *http.Request) {
		handler.NewPerceptionHandler(s.perceptionSvc).State(w, r)
	})
	s.mux.HandleFunc("GET /v1/perception/stream", func(w http.ResponseWriter, r *http.Request) {
		handler.NewPerceptionHandler(s.perceptionSvc).Stream(w, r)
	})

	// Perception Bus (FASE A — multimodal vision+audio synchroniser). Read-only,
	// always registered; nil/disabled service → 503 (opt-in). The handler is
	// constructed lazily per-request via a closure that reads s.busSvc, so it
	// sees the value wired by SetBusService AFTER New() — avoiding the nil
	// capture problem (the perception handler above is registered at New() time
	// and relies on the same late-wiring; for the bus we read it live so it is
	// robust to ordering).
	s.mux.HandleFunc("GET /v1/perception/bus/state", func(w http.ResponseWriter, r *http.Request) {
		handler.NewBusHandler(s.busSvc).State(w, r)
	})
	s.mux.HandleFunc("GET /v1/perception/bus", func(w http.ResponseWriter, r *http.Request) {
		handler.NewBusHandler(s.busSvc).Stream(w, r)
	})
}

// coscaCostStore devolve o cost.Store do diretório .cosca do projeto (nil-safe:
// se o .cosca não puder ser resolvido o Store ainda é criado; o brainweb lê de
// forma best-effort e projeta neutro quando não há telemetria). Mesmo padrão de
// resolução usado pela fonte de activity log.
func coscaCostStore() *cost.Store {
	coscaDir := filepath.Join(".", ".cosca")
	if cwd, err := os.Getwd(); err == nil {
		coscaDir = filepath.Join(cwd, ".cosca")
	}
	return cost.ForCoscaDir(coscaDir)
}

// perceptionSource implementa brainweb.PerceptionSource: roda a percepção
// determinística frame-a-frame (visão sem VLM externa) sobre um vídeo do
// projeto. Read-only e best-effort — sem vídeo, devolve vazio sem pânico.
type perceptionSource struct{}

func newPerceptionSource() *perceptionSource { return &perceptionSource{} }

// Perceive roda vision.AnalyzeVideo e converte para a projeção mínima do cérebro.
func (s *perceptionSource) Perceive() *brainweb.Perception {
	video := os.Getenv("COSCA_BRAIN_VIDEO")
	if video == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return &brainweb.Perception{GeneratedAt: time.Now().UTC(), Events: []brainweb.PEvent{}}
		}
		video = filepath.Join(cwd, "data", "brain.mp4")
	}
	if _, err := os.Stat(video); err != nil {
		return &brainweb.Perception{GeneratedAt: time.Now().UTC(), Events: []brainweb.PEvent{}}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := vision.AnalyzeVideo(ctx, video, vision.PipelineOptions{FPS: 1})
	if err != nil || res == nil {
		return &brainweb.Perception{GeneratedAt: time.Now().UTC(), Events: []brainweb.PEvent{}}
	}

	p := &brainweb.Perception{
		GeneratedAt: time.Now().UTC(),
		Source:      video,
		Frames:      len(res.Frames),
		Duration:    res.Probe.DurationSeconds(),
		Events:      make([]brainweb.PEvent, 0, len(res.Events)),
	}
	for _, ev := range res.Events {
		p.Events = append(p.Events, brainweb.PEvent{
			Kind:        ev.Kind,
			Frame:       ev.Frame,
			NeedsVLM:    ev.NeedsVLM,
			Explanation: ev.Explanation,
			Epistemic:   string(ev.Observation.Epistemic),
			Level:       string(ev.Observation.Level()),
		})
	}
	return p
}

// executionActivitySource implementa brainweb.ActivitySource sobre fontes
// REAIS de atividade. Fonte primária: o activity log (activity.jsonl, escrito
// por todo comando cosca — CLI direto ou invocação de agentes/opencode). É o
// feed que alimenta o cérebro com o que realmente aconteceu. Read-only.
// execActivityTTL é a janela do cache do feed de atividade (evita I/O em disco
// a cada request do observatório). Mantido curto: 2s.
const execActivityTTL = 2 * time.Second

// executionActivitySource implementa brainweb.ActivitySource sobre fontes
// REAIS de atividade. Fonte primária: o activity log (activity.jsonl, escrito
// por todo comando cosca — CLI direto ou invocação de agentes/opencode). É o
// feed que alimenta o cérebro com o que realmente aconteceu. Read-only.
//
// activityPath é o caminho do activity.jsonl; quando vazio, cai no default
// `<cwd>/.cosca/activity.jsonl`. Um caminho explícito (injetado via Config e
// nos testes) permite apontar o feed para um arquivo temporário.
type executionActivitySource struct {
	activityPath string

	mu       sync.Mutex
	cache    []brainweb.Activity
	cacheAt  time.Time
	cacheLim int
}

func newExecutionActivitySource(activityPath string) *executionActivitySource {
	return &executionActivitySource{activityPath: activityPath}
}

func (s *executionActivitySource) Recent(limit int) []brainweb.Activity {
	if limit <= 0 {
		limit = 30
	}
	// Cache curto (2s) — evita re-ler o activity log a cada request do feed.
	s.mu.Lock()
	if s.cache != nil && time.Since(s.cacheAt) < execActivityTTL && s.cacheLim >= limit {
		out := s.cache
		if limit < len(out) {
			out = out[:limit]
		}
		s.mu.Unlock()
		return out
	}
	s.mu.Unlock()

	acts := s.load(limit)

	s.mu.Lock()
	s.cache = acts
	s.cacheAt = time.Now()
	s.cacheLim = limit
	s.mu.Unlock()
	return acts
}

// load lê a fonte real de atividade (activity log → fallback executions).
func (s *executionActivitySource) load(limit int) []brainweb.Activity {
	path := s.activityPath
	if path == "" {
		coscaDir := filepath.Join(".", ".cosca")
		if cwd, err := os.Getwd(); err == nil {
			coscaDir = filepath.Join(cwd, ".cosca")
		}
		path = filepath.Join(coscaDir, "activity.jsonl")
	}
	activities := readActivityLog(path, limit)
	if len(activities) > 0 {
		return activities
	}

	// Fallback: executions (quando o activity log não existe).
	store := orchestration.GetExecutionStore()
	if store == nil {
		return []brainweb.Activity{}
	}
	execs, _ := store.List(limit, 0, "", "", "")
	acts := make([]brainweb.Activity, 0, len(execs))
	for _, e := range execs {
		if e == nil {
			continue
		}
		acts = append(acts, brainweb.Activity{
			ID:         e.ID,
			Agent:      e.Agent,
			Status:     e.Status,
			DurationMs: e.DurationMs,
			SkillsUsed: e.SkillsUsed,
			Provider:   e.Provider,
			Model:      e.Model,
			At:         e.CreatedAt.UnixMilli(),
		})
	}
	return acts
}

// activityReadWindow é a janela de bytes lidos no tail-read do activity.jsonl.
const activityReadWindow int64 = 512 * 1024 // 512KB — lê só a cauda do arquivo.

// readActivityLog lê as últimas N linhas do activity.jsonl (append-only).
// Retorna a projeção mínima (agent/status/action/at) para o observatório.
// Apenas o rótulo seguro da ação é exposto (constante "COMMAND_EXECUTED");
// prompts/args sensíveis NUNCA são mapeados para o payload público.
func readActivityLog(path string, limit int) []brainweb.Activity {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	type rawActivity struct {
		At     int64  `json:"at"`
		Actor  string `json:"actor"`
		Action string `json:"action"`
		Agent  string `json:"agent"`
		Prompt string `json:"prompt"` // nome do comando (sanitizado — nunca args)
		Status string `json:"status"`
	}
	// Tail-read: lê apenas a cauda (últimos ~512KB) do arquivo append-only,
	// evitando varrer o arquivo inteiro em cada request de /v1/cognitive/activity.
	info, err := f.Stat()
	if err != nil {
		return nil
	}
	size := info.Size()
	readStart := int64(0)
	if size > activityReadWindow {
		readStart = size - activityReadWindow
	}
	var all []rawActivity
	sc := bufio.NewScanner(io.NewSectionReader(f, readStart, size-readStart))
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		// Se começamos no meio do arquivo, a primeira "linha" é um fragmento
		// parcial — descarta para não corromper o JSON.
		if lineNo == 1 && readStart > 0 {
			continue
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var ra rawActivity
		if err := json.Unmarshal([]byte(line), &ra); err != nil {
			continue
		}
		all = append(all, ra)
	}

	// Devolve as últimas N (mais recentes por at desc).
	startIdx := 0
	if len(all) > limit {
		startIdx = len(all) - limit
	}
	out := make([]brainweb.Activity, 0, len(all)-startIdx)
	for i := len(all) - 1; i >= startIdx; i-- {
		ra := all[i]
		roll := ra.Agent
		if roll == "" {
			roll = ra.Actor
		}
		out = append(out, brainweb.Activity{
			ID:          fmt.Sprintf("%d-%s", ra.At, roll),
			Agent:       roll,
			Status:      ra.Status,
			Action:      ra.Action, // rótulo seguro ("COMMAND_EXECUTED"), nunca args
			Description: ra.Prompt, // nome do comando/ação (seguro — nunca args sensíveis)
			At:          ra.At,
		})
	}
	return out
}

// knowledgeItemsFn devolve os itens de conhecimento do CKL a partir do engine
// real (read-only). Nil-safe: engine indisponível → lista vazia (a UI mostra
// "não sei" em vez de erro).
func knowledgeItemsFn(engine *knowledge.Engine) func() []knowledge.KnowledgeItem {
	return func() []knowledge.KnowledgeItem {
		path := knowledge.ProjectLawsPath()
		items, err := knowledge.ListKnowledgeItems(path)
		if err != nil {
			return []knowledge.KnowledgeItem{}
		}
		return items
	}
}

// traceReplayFn devolve a cadeia causal da execução mais recente a partir do
// trace store real (read-only). Nil-safe: sem store ou sem trace → vazio.
func traceReplayFn(store *trace.Store) func() ([]trace.CausalNode, []trace.CausalEdge, string) {
	return func() ([]trace.CausalNode, []trace.CausalEdge, string) {
		if store == nil {
			return nil, nil, ""
		}
		latest, err := store.Latest(1)
		if err != nil || len(latest) == 0 {
			return nil, nil, ""
		}
		traceID := latest[0].TraceID
		events, err := store.Get(traceID)
		if err != nil || len(events) == 0 {
			return nil, nil, ""
		}
		cg := trace.BuildCausalGraph(events)
		return cg.Nodes, cg.Edges, cg.TraceID
	}
}

// cognitiveStatsFn devolve o snapshot cognitivo a partir de managers reais.
func cognitiveStatsFn(s *Server, rt *runtime.Runtime) func() brainweb.CognitiveSnapshot {
	return func() brainweb.CognitiveSnapshot {
		cs := brainweb.CognitiveSnapshot{Epistemology: map[string]int{}}
		if s.agentsManager != nil {
			cs.Agents = len(s.agentsManager.List())
		}
		if s.skillsManager != nil {
			cs.Skills = len(s.skillsManager.List())
		}
		if rt != nil {
			cs.UptimeSeconds = int64(rt.State().Get().Uptime.Seconds())
		}
		// Epistemologia real do CKL (se disponível).
		items, err := knowledge.ListKnowledgeItems(knowledge.ProjectLawsPath())
		if err == nil {
			for _, it := range items {
				st := string(it.Status)
				if st == "" {
					st = "UNKNOWN"
				}
				cs.Epistemology[st]++
			}
		}
		return cs
	}
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
