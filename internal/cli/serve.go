package cli

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"

	"github.com/CoscaAI/cosca/api/grpcserver"
	"github.com/CoscaAI/cosca/api/middleware"
	"github.com/CoscaAI/cosca/api/rest"
	"github.com/CoscaAI/cosca/internal/agents"
	"github.com/CoscaAI/cosca/internal/audit"
	"github.com/CoscaAI/cosca/internal/auth"
	"github.com/CoscaAI/cosca/internal/bootstrap"
	"github.com/CoscaAI/cosca/internal/capability"
	"github.com/CoscaAI/cosca/internal/chat"
	chatprovider "github.com/CoscaAI/cosca/internal/chat/provider"
	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/department"
	"github.com/CoscaAI/cosca/internal/durable"
	"github.com/CoscaAI/cosca/internal/env"
	"github.com/CoscaAI/cosca/internal/grpcclient"
	"github.com/CoscaAI/cosca/internal/kernel"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/metrics"
	"github.com/CoscaAI/cosca/internal/orchestration"
	"github.com/CoscaAI/cosca/internal/perception"
	"github.com/CoscaAI/cosca/internal/perception/bus"
	"github.com/CoscaAI/cosca/internal/providers"
	"github.com/CoscaAI/cosca/internal/runtime"
	"github.com/CoscaAI/cosca/internal/secrets"
	"github.com/CoscaAI/cosca/internal/skills"
	"github.com/CoscaAI/cosca/internal/sqlite"
	"github.com/CoscaAI/cosca/internal/trace"
	"github.com/CoscaAI/cosca/internal/workflows"
	"github.com/CoscaAI/cosca/internal/worldmodel/audio/tts"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// NewServeCommand creates the `cosca serve` command that starts the REST API
// server with all engine dependencies initialized.
func NewServeCommand() *cobra.Command {
	var (
		host              string
		port              int
		metricsPort       int
		corsOrigins       string
		dataDir           string
		tlsCertFile       string
		tlsKeyFile        string
		grpcPort          int
		grpcReflection    bool
		grpcDisable       bool
		enableWebsocket   bool
		wsAllowedOrigins  string
		provider          string
		apiOnly           bool
		runtimeGRPCAddr   string
		runtimeStandalone bool
		pipelineEnable    bool
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Cosca REST API server",
		Long: `Start the Cosca REST API server with all engine dependencies.

This command initializes the Knowledge Engine, Memory Engine, and Runtime,
then starts an HTTP server exposing the REST API on the configured port.
It also starts a gRPC server, a Prometheus metrics server, and an optional
WebSocket real-time event gateway.

The server supports graceful shutdown on SIGINT and SIGTERM.

REST Endpoints:
  /health                          Liveness probe (K8s)
  /ready                           Readiness probe (K8s)
  /v1/knowledge/search             Search knowledge base
  /v1/knowledge/index              Index documents (editor+)
  /v1/knowledge/stats              Knowledge statistics
  /v1/knowledge/sync               Sync knowledge base (editor+)
  /v1/knowledge/sync/stream        Sync progress (SSE, editor+)
  /v1/memory/store                 Store memory record (editor+)
  /v1/memory/search                Search memory
  /v1/memory/get                   Get memory record
  /v1/memory/delete                Delete memory record (editor+)
  /v1/memory/promote               Promote memory between layers (editor+)
  /v1/memory/stats                 Memory statistics
  /v1/status                       Runtime status
  /v1/health                       Runtime health
  /v1/status/stream                Runtime status stream (SSE)
  /v1/run                          Execute AI orchestration (editor+)
  /v1/run/stream                   Streaming chat completion (SSE, editor+)
  /v1/workflows/{name}/run         Execute workflow (editor+)
  /v1/workflows/{name}/run/stream  Workflow execution progress (SSE, editor+)
  /v1/providers/{name}/test        Test provider connectivity (editor+)
  /v1/providers/active             Set global active provider (admin only)
  /v1/executions                   Execution history — LLM prompts (editor+)
  /v1/traces                       Trace summaries — execution timeline (editor+)
  /v1/traces/{id}                  Full trace timeline (flight recorder, editor+)
  /v1/traces/{id}/replay           Ordered action sequence + divergence (editor+)
  /v1/analytics                    Aggregate query analytics (admin only)
  /v1/kernel/emergency             Kill-switch status (admin only)
  /v1/kernel/emergency/stop        Emergency stop — shut down the daemon (admin only)
  /v1/kernel/emergency/halt        Emergency halt — stop everything (admin only)
  /v1/ws                           WebSocket real-time event gateway

Role hierarchy: admin > editor > viewer. Read-only routes are open to
viewers; routes that write data or execute LLM calls require editor+;
globally sensitive routes require admin.

Security defaults (fail-closed):
  - Listens on 127.0.0.1 (loopback) by default. Pass --host 0.0.0.0 to
    expose the API on the network.
  - CORS is disabled by default: no Access-Control-Allow-Origin header is
    emitted until origins are listed explicitly via --cors-origins
    (or COSCA_CORS_ORIGINS), e.g. --cors-origins "http://localhost:3000".
  - Skill install (POST /v1/skills/{name}/install) accepts LOCAL sources
    inside the skills directory only. Remote http/https sources are denied
    unless COSCA_SKILLS_ALLOW_REMOTE_SOURCES=true is set explicitly.

Public registration (POST /v1/auth/register) is DISABLED by default.
Set COSCA_ENABLE_REGISTRATION=true to allow self-registration of viewer
accounts. Existing user login is never affected by this setting.

gRPC Endpoints (port 14122):
  KnowledgeService: Search, Index, Stats, Sync
  MemoryService: Store, Search, Get, Delete, Promote, Stats
  RuntimeService: Status, Health
`,
		Example: `  cosca serve
  cosca serve --port 14120 --cors-origins "http://localhost:3000"
  cosca serve --host 0.0.0.0 --port 14120 --metrics-port 14121
  cosca serve --tls-cert-file /etc/certs/cosca.pem --tls-key-file /etc/certs/cosca-key.pem`,
		Args: cobra.NoArgs,
		RunE: runServeProvider(&provider, &host, &port, &metricsPort, &corsOrigins, &dataDir, &tlsCertFile, &tlsKeyFile, &grpcPort, &grpcReflection, &grpcDisable, &enableWebsocket, &wsAllowedOrigins, &apiOnly, &runtimeGRPCAddr, &runtimeStandalone, &pipelineEnable),
	}

	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "host address to listen on (default 127.0.0.1 — loopback only; pass --host 0.0.0.0 to expose on the network)")
	cmd.Flags().IntVar(&port, "port", 14120, "port for the REST API server")
	cmd.Flags().IntVar(&metricsPort, "metrics-port", 14121, "port for the Prometheus metrics server")
	cmd.Flags().StringVar(&corsOrigins, "cors-origins", "", "comma-separated list of allowed CORS origins (empty = fail-closed, no cross-origin access)")
	cmd.Flags().StringVar(&dataDir, "data-dir", "", "data directory for engines (default: .cosca in current directory)")
	cmd.Flags().StringVar(&tlsCertFile, "tls-cert-file", "", "TLS certificate file (enables HTTPS when provided with --tls-key-file)")
	cmd.Flags().StringVar(&tlsKeyFile, "tls-key-file", "", "TLS private key file (enables HTTPS when provided with --tls-cert-file)")
	cmd.Flags().IntVar(&grpcPort, "grpc-port", 14122, "port for the gRPC server")
	cmd.Flags().BoolVar(&grpcReflection, "grpc-reflection", false, "enable gRPC server reflection (development only)")
	cmd.Flags().BoolVar(&grpcDisable, "grpc-disable", false, "disable the gRPC server entirely")
	cmd.Flags().BoolVar(&enableWebsocket, "enable-websocket", true, "enable the WebSocket real-time event gateway")
	cmd.Flags().StringVar(&wsAllowedOrigins, "websocket-allowed-origins", "localhost", "comma-separated list of allowed WebSocket origin hosts")
	cmd.Flags().StringVar(&provider, "provider", "", `AI provider for the daemon: "" (default — use config / current behavior) or "none" (deterministic mode — no AI provider; all subsystems boot, LLM endpoints return 503). When the flag is unset, COSCA_PROVIDER=none is honored.`)
	cmd.Flags().BoolVar(&apiOnly, "api-only", false, "run in API-only mode: do NOT start the internal background daemon (no PID file, no sync/backup loop); status/health endpoints consult the standalone runtime daemon via gRPC (see --runtime-grpc-addr)")
	cmd.Flags().StringVar(&runtimeGRPCAddr, "runtime-grpc-addr", grpcclient.DefaultRuntimeAddr, "gRPC address of the standalone runtime daemon (used with --api-only)")
	cmd.Flags().BoolVar(&runtimeStandalone, "runtime-standalone", false, "standalone runtime mode — delegates compute fabric and daemon to external runtime daemon")
	cmd.Flags().BoolVar(&pipelineEnable, "pipeline-enable", true, "enable autonomous pipeline flow for /v1/run (Planner -> StepRunner -> RecoveryLoop); when false, /v1/run uses the legacy direct-to-LLM path")

	return cmd
}

// runServe returns the RunE function for the serve command, capturing flag
// pointers so the command can be tested. It keeps the historical signature
// (used by existing tests) and delegates to runServeProvider with no explicit
// --provider flag. The COSCA_PROVIDER environment variable is still honored
// at boot.
func runServe(host *string, port, metricsPort *int, corsOrigins, dataDir, tlsCertFile, tlsKeyFile *string, grpcPort *int, grpcReflection, grpcDisable, enableWebsocket *bool, wsAllowedOrigins *string) func(cmd *cobra.Command, args []string) error {
	provider := ""
	apiOnly := false
	runtimeGRPCAddr := grpcclient.DefaultRuntimeAddr
	runtimeStandalone := false
	pipelineEnable := true
	return runServeProvider(&provider, host, port, metricsPort, corsOrigins, dataDir, tlsCertFile, tlsKeyFile, grpcPort, grpcReflection, grpcDisable, enableWebsocket, wsAllowedOrigins, &apiOnly, &runtimeGRPCAddr, &runtimeStandalone, &pipelineEnable)
}

// runServeProvider returns the RunE function for the serve command, capturing
// flag pointers (including --provider) so the command can be tested. When
// --provider=none (or COSCA_PROVIDER=none) is active the daemon boots in
// deterministic mode: every subsystem (knowledge, memory, laws, audit,
// security, runtime) is initialized exactly as in normal mode, but LLM
// dependent wiring is skipped and cognitive endpoints respond 503.
//
// --api-only (FASE 2, DDNA-2026-08-07-001) disables the internal background
// daemon (EnableDaemon=false — no PID file, no sync/backup loop) and makes
// the runtime status/health endpoints consult the standalone runtime daemon
// (`cosca runtime start`) through a gRPC client at --runtime-grpc-addr.
func runServeProvider(provider *string, host *string, port, metricsPort *int, corsOrigins, dataDir, tlsCertFile, tlsKeyFile *string, grpcPort *int, grpcReflection, grpcDisable, enableWebsocket *bool, wsAllowedOrigins *string, apiOnly *bool, runtimeGRPCAddr *string, runtimeStandalone *bool, pipelineEnable *bool) func(cmd *cobra.Command, args []string) error {
	return func(_ *cobra.Command, _ []string) error {
		// 1. Pre-bootstrap: load .env, resolve data dir, create logger
		dir, logger, err := servePreBootstrap(host, port, metricsPort, corsOrigins, dataDir)
		if err != nil {
			return err
		}

		// 2. Compose engines: API-only, deterministic, bootstrap.Compose
		eng, err := serveComposeEngines(dir, logger, provider, apiOnly, runtimeGRPCAddr, runtimeStandalone, pipelineEnable)
		if err != nil {
			return err
		}
		runtimeClient := eng.client
		deterministic := eng.deterministic
		boot := eng.boot
		ke := eng.ke
		mem := eng.mem
		rtInstance := eng.rt

		// 3. Init managers: agents, skills, providers, workflows
		mgr := serveInitManagers(dir, logger, deterministic)

		// 4. Init auth: user store, JWT secret, API key store, token store
		authRes := serveInitAuth(dir, logger)
		defer func() {
			if err := authRes.db.Close(); err != nil {
				logger.Warn().Err(err).Msg("auth db close error")
			}
		}()
		if authRes.tokens != nil {
			defer func() {
				if err := authRes.tokens.Close(); err != nil {
					logger.Warn().Err(err).Msg("token store close error")
				}
			}()
		}

		// 5. Init stores: audit, secrets, trace, department
		stores := serveInitStores(dir, authRes.jwtSecret, logger)
		defer func() {
			if err := stores.audit.Close(); err != nil {
				logger.Warn().Err(err).Msg("audit store close error")
			}
		}()
		defer func() {
			if err := stores.secrets.Close(); err != nil {
				logger.Warn().Err(err).Msg("secrets vault close error")
			}
		}()
		if stores.trace != nil {
			defer func() {
				if err := stores.trace.Close(); err != nil {
					logger.Warn().Err(err).Msg("trace store close error")
				}
			}()
		}
		if stores.department != nil {
			defer func() {
				if err := stores.department.Close(); err != nil {
					logger.Warn().Err(err).Msg("department store close error")
				}
			}()
		}

		// 6. Init chat: provider selection, registry setup
		serveInitChat(deterministic, logger, boot)

		// 7. Start servers: REST, gRPC, metrics
		servers := serveStartServers(
			dir, logger, ke, mem, rtInstance,
			mgr.agents, mgr.skills, mgr.providers, mgr.workflows,
			authRes.store, authRes.apiKey, authRes.jwtSecret, authRes.tokens,
			stores.audit, stores.secrets, stores.trace, stores.department,
			runtimeClient, deterministic, boot, eng.searchMode, eng.deliberateCfg,
			eng.emergency,
			eng.perceptionSvc,
			eng.busSvc,
			eng.episodicSink,
			eng.voiceSpeaker,
			host, port, metricsPort, corsOrigins,
			tlsCertFile, tlsKeyFile,
			grpcPort, grpcReflection, grpcDisable,
			enableWebsocket, wsAllowedOrigins,
			pipelineEnable,
		)

		// 8. Event loop: signal handling, shutdown sequence
		return serveEventLoop(servers, dir, logger, rtInstance, ke, mem, grpcDisable)
	}
}

// ── Result types for serve refactoring ────────────────────────────────────────

type serveEngineResult struct {
	client        *grpcclient.RuntimeClient
	deterministic bool
	boot          *bootstrap.Result
	ke            *knowledge.Engine
	mem           *memory.MemoryEngine
	rt            *runtime.Runtime
	// emergency é o kill-switch do kernel (Etapa 3b), criado ANTES do Compose
	// e reusado pelo serveStartServers — o MESMO emergencyMgr protege o
	// orchestration (via bootstrap) e o servidor REST.
	emergency *kernel.EmergencyManager
	// searchMode é o modo de busca da config ("legacy" | "modular") - FASE 1
	// routing/scope. Vazio = legacy (comportamento atual).
	searchMode string
	// deliberateCfg é a configuração da Kernel-First Deliberation (ADR-032)
	// lida da config; repassada ao servidor REST (`/v1/run`). Fail-closed.
	deliberateCfg orchestration.DeliberateConfig
	// perceptionCfg é a configuração do perception loop (da config). Passada ao
	// serveStartServers para montar/iniciar o serviço. Opt-in (Enabled=false).
	perceptionCfg config.PerceptionConfig
	// perceptionSvc é o serviço do Perception Loop (screen → vision → estado →
	// SSE). Nil-safe: quando a config está desabilitada, é nil e os endpoints
	// /v1/perception/* reportam 503.
	perceptionSvc *perception.Service
	// busSvc é o Perception Bus (sincronização multimodal visão+áudio, FASE A).
	// Nil-safe: criado apenas quando perception.audio.enabled (opt-in). Quando
	// nil, /v1/perception/bus* reporta 503.
	busSvc *bus.Bus
	// episodicSink é o gravador de memória episódica multimodal (FASE D).
	// Nil-safe: criado apenas quando perception.episodic.enabled (opt-in). Quando
	// nil, nenhum EpisodicRecord é persistido.
	episodicSink *bus.MemorySink
	// voiceSpeaker é o Speaker TTS nativo (FASE C — "o COSCA fala"). Nil-safe:
	// criado apenas quando perception.audio.tts.provider=sherpa (opt-in). Quando
	// nil, /v1/voice/speaks reporta 503.
	voiceSpeaker *tts.Speaker
}

type serveManagerResult struct {
	agents    *agents.Manager
	skills    *skills.Manager
	providers *providers.Manager
	workflows *workflows.Manager
}

type serveAuthResult struct {
	db        *sql.DB
	store     *auth.UserStore
	jwtSecret []byte
	apiKey    *auth.APIKeyStore
	tokens    *auth.TokenStore
}

type serveStoreResult struct {
	audit      *audit.Store
	secrets    *secrets.Vault
	trace      *trace.Store
	department *department.ConversationStore
}

type serveServerResult struct {
	rest          *rest.Server
	metrics       *http.Server
	grpc          *grpcserver.GRPCServer
	emergencyCh   chan string
	sigCh         chan os.Signal
	errCh         chan error
	serverStart   time.Time
	gitInit       gitState
	perceptionSvc *perception.Service
	busSvc        *bus.Bus
	episodicSink  *bus.MemorySink
	voiceSpeaker  *tts.Speaker
}

// ── 1. preBootstrap ───────────────────────────────────────────────────────────

func servePreBootstrap(host *string, port, metricsPort *int, corsOrigins, dataDir *string) (string, zerolog.Logger, error) {
	env.LoadDefault(false)

	configureCORSFromEnv(corsOrigins)

	dir, dirErr := resolveDataDir(*dataDir)
	if dirErr != nil {
		return "", zerolog.Logger{}, dirErr
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", zerolog.Logger{}, fmt.Errorf("create data directory: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return "", zerolog.Logger{}, fmt.Errorf("restrict data directory permissions: %w", err)
	}

	logger := log.With().
		Str("component", "serve").
		Str("addr", fmt.Sprintf("%s:%d", *host, *port)).
		Logger()

	logger.Info().
		Str("data_dir", dir).
		Str("cors_origins", *corsOrigins).
		Int("metrics_port", *metricsPort).
		Msg("starting cosca serve")

	return dir, logger, nil
}

// ── 2. composeEngines ────────────────────────────────────────────────────────

func serveComposeEngines(dir string, logger zerolog.Logger, provider *string, apiOnly *bool, runtimeGRPCAddr *string, runtimeStandalone *bool, pipelineEnable *bool) (*serveEngineResult, error) {
	var runtimeClient *grpcclient.RuntimeClient
	if *apiOnly {
		logger.Info().
			Str("runtime_grpc_addr", *runtimeGRPCAddr).
			Msg("api-only mode — runtime daemon externo esperado em " + *runtimeGRPCAddr)
		runtimeClient = grpcclient.NewRuntimeClient(*runtimeGRPCAddr)
	}

	deterministic := shouldRunDeterministic(*provider, os.Getenv("COSCA_PROVIDER"))
	if deterministic {
		logger.Info().Msg("modo determinístico — sem provider de IA")
		logger.Info().
			Str("capability_level", capability.CurrentLevel("none", false).String()).
			Msg("cosca capability level")
		fmt.Println("Capacidade cognitiva: indisponível (provider=none)")
		fmt.Println("Operando em modo determinístico.")
		fmt.Println("Disponível: ✓ knowledge ✓ laws ✓ memory ✓ audit ✓ security ✓ runtime ✓ workflows(backup)")
		fmt.Println("Indisponível: ✗ raciocínio aberto ✗ geração de código")
	}

	var (
		embeddingProvider   string
		embeddingBaseURL    string
		embeddingModel      string
		embeddingDigest     string
		embeddingAPIKey     string
		embeddingDimensions int
		embeddingSearchMode string
		watchFrameworkDir   string
		deliberateCfg       orchestration.DeliberateConfig
		perceptionCfg       config.PerceptionConfig
	)
	if c, loadErr := config.Load(); loadErr != nil {
		logger.Warn().Err(loadErr).Msg("failed to load project config — embedding provider will be auto-detected")
	} else {
		embeddingProvider = c.Embedding.Provider
		embeddingBaseURL = c.Embedding.BaseURL
		embeddingAPIKey = c.Embedding.APIKey
		embeddingModel = c.Embedding.Model
		embeddingDigest = c.Embedding.Digest
		embeddingDimensions = c.Embedding.Dimensions
		embeddingSearchMode = c.Search.Mode
		// Perception Loop (opt-in): ler a config para montar o serviço contínuo.
		perceptionCfg = c.Perception
		// Kernel-First Deliberation (ADR-032). Fail-closed (LEI DO COFRE):
		// only explicitly enabled:true turns the stage on — an absent section
		// yields Disabled=false.
		deliberateCfg = orchestration.DeliberateConfigFromConfig(c.Orchestration.Deliberation)

		if c.Watch.Enabled {
			if wd, wdErr := os.Getwd(); wdErr == nil {
				fwDir := filepath.Join(wd, "internal", "embed", "cosca")
				if _, statErr := os.Stat(fwDir); statErr == nil {
					watchFrameworkDir = fwDir
				}
			}
		}
	}

	// Kill-switch do kernel (Etapa 3b): criado ANTES do Compose para fluir ao
	// orchestration (via Config.HaltChecker) e ao servidor REST (via
	// serveStartServers) — o MESMO emergencyMgr protege todos os caminhos LLM
	// do serve (REST /v1/run + pipeline + run).
	emergencyMgr := kernel.NewEmergencyManager()

	boot, bootErr := bootstrap.Compose(bootstrap.Config{
		DataDir:             dir,
		Logger:              logger,
		EnableDaemon:        !*apiOnly,
		RuntimeStandalone:   *apiOnly || *runtimeStandalone,
		EmbeddingProvider:   embeddingProvider,
		EmbeddingBaseURL:    embeddingBaseURL,
		EmbeddingModel:      embeddingModel,
		EmbeddingDigest:     embeddingDigest,
		EmbeddingAPIKey:     embeddingAPIKey,
		EmbeddingDimensions: embeddingDimensions,
		WatchFrameworkDir:   watchFrameworkDir,
		WorkspaceDir:        workspaceDir(),
		DeliberateConfig:    deliberateCfg,
		HaltChecker:         emergencyMgr,
		Pipeline: bootstrap.PipelineConfig{
			Enabled: *pipelineEnable,
		},
	})
	if v := config.GetViper(); v != nil {
		v.Set("runtime.enabled", !*apiOnly)
		v.Set("pipeline.enabled", *pipelineEnable)
	}
	if bootErr != nil {
		logger.Warn().Err(bootErr).Msg("engine composition completed with issues")
	}

	// Perception Loop service (opt-in). Built from config; disabled → nil so
	// the /v1/perception/* endpoints report a clear 503.
	perceptionSvc := buildPerceptionService(perceptionCfg, logger)
	if perceptionSvc != nil && perceptionSvc.Enabled() {
		logger.Info().
			Dur("interval", perceptionSvc.Interval()).
			Str("mode", perceptionSvc.Mode()).
			Str("capture", perceptionCfg.Capture).
			Msg("perception loop enabled")
	}

	// Perception Bus (FASE A, opt-in). Created only when perception.audio.enabled
	// and a live perception loop is wired. When nil, /v1/perception/bus* reports
	// a clear 503. The bus is a pure adapter over the loop — it never duplicates
	// or alters the vision loop.
	busSvc := buildPerceptionBus(perceptionCfg, perceptionSvc, logger)
	if busSvc != nil {
		logger.Info().
			Dur("window", perceptionCfg.Audio.Window).
			Dur("tol", perceptionCfg.Audio.Tolerance).
			Msg("perception bus enabled (multimodal sync)")
	}

	// FASE C: native-Go TTS "speaker" (opt-in via perception.audio.tts.provider).
	// Built here (with the perception config + logger) so the REST server can wire
	// the /v1/voice/speaks endpoint. Nil when not opted-in / not compiled.
	voiceSpeaker, vErr := buildTtsSpeaker(perceptionCfg, logger)
	if vErr != nil {
		logger.Warn().Err(vErr).Msg("voice: TTS speaker build failed — /v1/voice/speaks disabled")
		voiceSpeaker = nil
	}

	// FASE D: memória episódica multimodal (opt-in). Gravador que assina o
	// Perception Bus e persiste EpisodicRecords (representação — nunca frames
	// brutos) na engine de memória. Nil-safe: só criado quando
	// perception.episodic.enabled e há bus + engine de memória.
	episodicSink := buildEpisodicSink(perceptionCfg, busSvc, boot.Memory, logger)

	return &serveEngineResult{
		client:        runtimeClient,
		deterministic: deterministic,
		boot:          boot,
		ke:            boot.Knowledge,
		mem:           boot.Memory,
		rt:            boot.Runtime,
		emergency:     emergencyMgr,
		searchMode:    embeddingSearchMode,
		deliberateCfg: deliberateCfg,
		perceptionCfg: perceptionCfg,
		perceptionSvc: perceptionSvc,
		busSvc:        busSvc,
		episodicSink:  episodicSink,
		voiceSpeaker:  voiceSpeaker,
	}, nil
}

// buildPerceptionService monta o serviço do Perception Loop a partir da config.
// Retorna nil quando a config está desabilitada (opt-in).
func buildPerceptionService(cfg config.PerceptionConfig, logger zerolog.Logger) *perception.Service {
	if !cfg.Enabled {
		return nil
	}
	return perception.NewService(perception.Config{
		Enabled:     cfg.Enabled,
		Mode:        cfg.Mode,
		Interval:    cfg.Interval,
		MaxInterval: cfg.MaxInterval,
		Capture:     cfg.Capture,
		ModelsDir:   cfg.ModelsDir,
		ChangeDetection: perception.ChangeDetectionConfig{
			Enabled:   cfg.ChangeDetection.Enabled,
			Threshold: cfg.ChangeDetection.Threshold,
		},
	}, perception.WithLogger(logger))
}

// buildPerceptionBus monta o Perception Bus (FASE A — sincronização multimodal
// visão+áudio) a partir da config. Retorna nil quando o bus não está opt-in
// (perception.audio.enabled) ou quando não há um perception loop ativo para
// assinar (o bus é um adapter puro sobre o loop). O AudioSource Fase A é um
// no-op; a Fase B liga o ASR (sherpa) no mesmo seam.
func buildPerceptionBus(cfg config.PerceptionConfig, visionSvc *perception.Service, logger zerolog.Logger) *bus.Bus {
	if !cfg.Audio.Enabled {
		return nil
	}
	if visionSvc == nil {
		logger.Warn().Msg("perception bus: audio.enabled but no perception loop wired — bus disabled")
		return nil
	}

	// FASE B: build the native-Go STT audio source (when opt-in). The builder
	// is defined per-build-tag (perception_stt_noop.go / perception_stt_sherpa.go)
	// so the default build never pulls cgo. A nil source falls back to Noop.
	audioSrc, aErr := buildSttAudioSource(cfg, logger)
	if aErr != nil {
		logger.Warn().Err(aErr).Msg("perception bus: STT audio source failed — using no-op source")
		audioSrc = nil
	}
	// FASE D: close the audio loop with a live microphone when opt-in
	// (perception.audio.mic.enabled). The mic source wraps the STT source so the
	// bus receives mic-driven transcriptions (mic → PCM → STT → bus).
	if audioSrc != nil {
		micSrc, mErr := buildMicAudioSource(cfg, audioSrc, logger)
		if mErr != nil {
			logger.Warn().Err(mErr).Msg("perception bus: microphone capture failed — using STT-only source")
		} else if micSrc != nil {
			audioSrc = micSrc
		}
	}
	if audioSrc == nil {
		audioSrc = bus.NoopAudioSource{}
	}

	return bus.NewBus(bus.Config{
		Window:    cfg.Audio.Window,
		MaxObs:    bus.DefaultMaxObs,
		Tolerance: cfg.Audio.Tolerance,
	}, visionSvc, audioSrc, logger.With().Str("component", "perception-bus").Logger())
}

// buildEpisodicSink monta o gravador de memória episódica multimodal (FASE D).
// Retorna nil quando não está opt-in (perception.episodic.enabled) ou quando
// não há bus ativo nem engine de memória disponível. O sink NUNCA bloqueia o
// bus (grava em goroutine de fundo, buffer com descarte).
func buildEpisodicSink(cfg config.PerceptionConfig, busSvc *bus.Bus, memEngine *memory.MemoryEngine, logger zerolog.Logger) *bus.MemorySink {
	if !cfg.Episodic.Enabled {
		return nil
	}
	if busSvc == nil {
		logger.Warn().Msg("perception episodic: enabled but no perception bus wired — episodic memory disabled")
		return nil
	}
	if memEngine == nil {
		logger.Warn().Msg("perception episodic: enabled but no memory engine wired — episodic memory disabled")
		return nil
	}
	// Garante que a camada episódica está registrada na engine (idempotente).
	if err := memEngine.EnsureEpisodicLayer(); err != nil {
		logger.Warn().Err(err).Msg("perception episodic: ensure layer failed — episodic memory disabled")
		return nil
	}
	ttl := cfg.Episodic.TTL
	if ttl <= 0 {
		ttl = config.DefaultEpisodicTTL
	}
	maxRec := cfg.Episodic.MaxRecords
	if maxRec <= 0 {
		maxRec = config.DefaultEpisodicMaxRecords
	}
	memEngine.ConfigureEpisodicRetention(ttl, maxRec)

	return bus.NewMemorySink(busSvc, memEngine, bus.MemorySinkConfig{
		Enabled:     true,
		MaxBuffered: 256,
		MaxSeen:     2048,
	}, logger.With().Str("component", "episodic").Logger())
}

// ── 3. initManagers ───────────────────────────────────────────────────────────

func serveInitManagers(dir string, logger zerolog.Logger, deterministic bool) *serveManagerResult {
	agentsMgr := agents.NewManager(dir)
	logger.Info().Int("count", len(agentsMgr.List())).Msg("agents manager initialized")

	skillsMgr := skills.NewManager(dir)
	skillsMgr.AllowRemoteSources = os.Getenv("COSCA_SKILLS_ALLOW_REMOTE_SOURCES") == "true"
	if skillsMgr.AllowRemoteSources {
		logger.Warn().Msg("remote skill sources are ENABLED (COSCA_SKILLS_ALLOW_REMOTE_SOURCES=true) — SSRF exposure is opt-in")
	}
	logger.Info().Int("count", len(skillsMgr.List())).Msg("skills manager initialized")

	providersMgr := providers.NewManager()
	logger.Info().Int("count", len(providersMgr.List())).Msg("providers manager initialized")

	var durableLedger *durable.Ledger
	if durableDB, openErr := sqlite.Open(sqlite.DefaultConfig(filepath.Join(dir, "durable.db"))); openErr == nil {
		if l, lErr := durable.NewLedger(durableDB); lErr == nil {
			durableLedger = l
			if recovered, recErr := durableLedger.RecoverStale(context.Background()); recErr == nil && recovered > 0 {
				logger.Info().Int("recovered", recovered).Msg("durable: released stale runs from previous session")
			}
		} else {
			logger.Warn().Err(lErr).Msg("durable ledger unavailable, workflows run without crash recovery")
		}
	}
	workflowOpts := []workflows.ManagerOption{}
	if durableLedger != nil {
		workflowOpts = append(workflowOpts, workflows.WithDurableRun(
			workflows.DurableRunConfig{Enabled: true, WorkerID: "cosca-serve", Lease: 60 * time.Second},
			durableLedger,
		))
	}
	workflowsMgr := workflows.NewManager(dir, workflowOpts...)
	logger.Info().Int("count", len(workflowsMgr.List())).Msg("workflows manager initialized")

	if deterministic {
		workflowsMgr.SetStepRunner(func(_ context.Context, step workflows.Step) (string, error) {
			return "", fmt.Errorf("capacidade cognitiva indisponível — modo determinístico (provider=none): workflow step %q requires an LLM provider", step.Name)
		})
	} else {
		workflowsMgr.SetStepRunner(func(ctx context.Context, step workflows.Step) (string, error) {
			registry := chat.GetRegistry()
			if registry == nil || registry.Name() == "chat-registry" {
				return "", fmt.Errorf("workflow step runner: no chat provider configured")
			}

			agentName := step.Agent
			if agentName == "" {
				agentName = "COSCA KERNEL"
			}
			systemContent := fmt.Sprintf(
				"You are %s. Complete the task described below thoroughly and accurately.",
				agentName,
			)
			if step.Name != "" {
				systemContent = fmt.Sprintf(
					"You are %s, executing the workflow step: %s. Complete the task described below thoroughly and accurately.",
					agentName, step.Name,
				)
			}

			messages := []chat.Message{
				{Role: chat.RoleSystem, Content: systemContent},
				{Role: chat.RoleUser, Content: step.Description},
			}

			opts := chat.DefaultChatOptions()
			resp, err := registry.Chat(ctx, messages, opts)
			if err != nil {
				return "", fmt.Errorf("workflow step %q: chat failed: %w", step.Name, err)
			}

			if len(resp.Choices) == 0 {
				return "", fmt.Errorf("workflow step %q: empty response from provider", step.Name)
			}

			return resp.Choices[0].Message.Content, nil
		})
	}

	return &serveManagerResult{
		agents:    agentsMgr,
		skills:    skillsMgr,
		providers: providersMgr,
		workflows: workflowsMgr,
	}
}

// ── 4. initAuth ──────────────────────────────────────────────────────────────

func serveInitAuth(dir string, logger zerolog.Logger) *serveAuthResult {
	authDB, authDBErr := sql.Open("sqlite", filepath.Join(dir, "knowledge.db"))
	if authDBErr != nil {
		logger.Fatal().Err(authDBErr).Msg("failed to open auth database")
	}
	authDB.SetMaxOpenConns(1)

	authStore := auth.NewUserStore(auth.UserStoreConfig{DB: authDB})
	jwtSecret := []byte(os.Getenv("COSCA_JWT_SECRET"))
	if len(jwtSecret) == 0 {
		logger.Fatal().Msg("COSCA_JWT_SECRET not set — refusing to start. Generate a strong secret: openssl rand -base64 64")
	}
	if len(jwtSecret) < 32 {
		logger.Warn().Msg("COSCA_JWT_SECRET is less than 32 bytes — consider using a stronger secret for production")
	}
	logger.Info().Int("users", len(authStore.List())).Msg("auth system initialized")

	apiKeyStore := auth.NewAPIKeyStore(auth.APIKeyStoreConfig{DataDir: dir})
	logger.Info().Msg("api key store initialized")

	tokenStore, tokenStoreErr := auth.NewTokenStore(filepath.Join(dir, "auth_tokens.db"))
	if tokenStoreErr != nil {
		logger.Warn().Err(tokenStoreErr).Msg("failed to open refresh-token store — refresh tokens will be JWT-only (no revocation)")
		tokenStore = nil
	} else {
		logger.Info().Msg("refresh-token store initialized")
	}

	return &serveAuthResult{
		db:        authDB,
		store:     authStore,
		jwtSecret: jwtSecret,
		apiKey:    apiKeyStore,
		tokens:    tokenStore,
	}
}

// ── 5. initStores ────────────────────────────────────────────────────────────

func serveInitStores(dir string, jwtSecret []byte, logger zerolog.Logger) *serveStoreResult {
	auditStore, err := audit.NewStore(filepath.Join(dir, "audit.db"))
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to initialize audit store")
	}
	logger.Info().Msg("audit store initialized")

	secretsVault, err := secrets.New(filepath.Join(dir, "secrets.db"), jwtSecret)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to initialize secrets vault")
	}
	logger.Info().Msg("secrets vault initialized")

	traceStore, traceStoreErr := trace.NewStore(filepath.Join(dir, "trace.db"))
	if traceStoreErr != nil {
		logger.Warn().Err(traceStoreErr).Msg("failed to open trace store — /v1/traces endpoints disabled")
		traceStore = nil
	} else {
		logger.Info().Msg("trace store initialized")
	}

	deptStore, deptStoreErr := department.NewConversationStore(filepath.Join(dir, "department.db"))
	if deptStoreErr != nil {
		logger.Warn().Err(deptStoreErr).Msg("failed to open department store — /v1/departments endpoints disabled")
		deptStore = nil
	} else {
		logger.Info().Msg("department store initialized")
	}

	return &serveStoreResult{
		audit:      auditStore,
		secrets:    secretsVault,
		trace:      traceStore,
		department: deptStore,
	}
}

// ── 6. initChat ──────────────────────────────────────────────────────────────

func serveInitChat(deterministic bool, logger zerolog.Logger, boot *bootstrap.Result) {
	if deterministic {
		logger.Info().Msg("chat provider auto-select skipped — modo determinístico (provider=none)")
		return
	}

	chatCfg := chat.DefaultChatRegistryConfig()
	chatCtx, chatCancel := context.WithTimeout(context.Background(), 10*time.Second)

	if err := chatprovider.RegisterChatProviders(chat.GetRegistry(), map[string]interface{}{
		"fabric": boot.Fabric,
	}); err != nil {
		logger.Warn().Err(err).Msg("failed to register chat providers")
	}

	if err := chat.GetRegistry().Select(chatCtx, chatCfg); err != nil {
		logger.Warn().Err(err).Msg("failed to auto-select chat provider — /v1/run may not work")
	} else {
		logger.Info().Str("provider", chat.GetRegistry().Name()).Str("model", chat.GetRegistry().Model()).Msg("chat provider selected for /v1/run")
	}
	chatCancel()

	// Boot-time Ollama detection (side-effect-free). When the configured
	// provider is ollama but the local daemon is not functional, log a clear
	// recommendation. Automatic bootstrap stays explicit (provider bootstrap
	// --install --pull); the deterministic flow is untouched.
	if report := ollamaBootHint(context.Background(), nil); report != nil && !report.OK {
		logger.Warn().
			Str("hint", "cosca provider bootstrap --install --pull").
			Str("reason", report.Error).
			Msg("ollama provider not functional — run 'cosca provider bootstrap --install --pull' to bootstrap it automatically")
	}
}

// ── 7. startServers ──────────────────────────────────────────────────────────

func serveStartServers(
	dir string,
	logger zerolog.Logger,
	ke *knowledge.Engine,
	mem *memory.MemoryEngine,
	rtInstance *runtime.Runtime,
	agentsMgr *agents.Manager,
	skillsMgr *skills.Manager,
	providersMgr *providers.Manager,
	workflowsMgr *workflows.Manager,
	authStore *auth.UserStore,
	apiKeyStore *auth.APIKeyStore,
	jwtSecret []byte,
	tokenStore *auth.TokenStore,
	auditStore *audit.Store,
	secretsVault *secrets.Vault,
	traceStore *trace.Store,
	deptStore *department.ConversationStore,
	runtimeClient *grpcclient.RuntimeClient,
	deterministic bool,
	boot *bootstrap.Result,
	searchMode string,
	deliberateCfg orchestration.DeliberateConfig,
	emergencyMgr *kernel.EmergencyManager,
	perceptionSvc *perception.Service,
	busSvc *bus.Bus,
	episodicSink *bus.MemorySink,
	voiceSpeaker *tts.Speaker,
	host *string,
	port *int,
	metricsPort *int,
	corsOrigins *string,
	tlsCertFile *string,
	tlsKeyFile *string,
	grpcPort *int,
	grpcReflection *bool,
	grpcDisable *bool,
	enableWebsocket *bool,
	wsAllowedOrigins *string,
	pipelineEnable *bool,
) *serveServerResult {
	// O emergencyMgr vem do serveComposeEngines (Etapa 3b) — o MESMO que
	// protege o orchestration via bootstrap. Aqui só configuramos o shutdown
	// hook e o passamos ao servidor REST.
	if emergencyMgr == nil {
		emergencyMgr = kernel.NewEmergencyManager()
	}
	emergencyCh := make(chan string, 1)
	emergencyMgr.SetShutdownFn(func(reason string) {
		select {
		case emergencyCh <- reason:
		default:
		}
	})

	serverCfg := rest.DefaultConfig()
	serverCfg.Host = *host
	serverCfg.Port = *port
	serverCfg.CORSOrigins = *corsOrigins
	serverCfg.RuntimeClient = runtimeClient
	// Kernel-First Deliberation (ADR-032): propagado ao /v1/run. Fail-closed:
	// zero-value (Enabled=false) mantém o caminho legado.
	serverCfg.DeliberateConfig = deliberateCfg

	serverCfg.RegistrationEnabled = os.Getenv("COSCA_ENABLE_REGISTRATION") == "true"
	if serverCfg.RegistrationEnabled {
		logger.Warn().Msg("public registration is ENABLED — anyone can create a viewer account (COSCA_ENABLE_REGISTRATION=true)")
	} else {
		logger.Info().Msg("public registration is disabled — set COSCA_ENABLE_REGISTRATION=true to enable self-registration")
	}

	server := rest.New(ke, mem, rtInstance, agentsMgr, skillsMgr, providersMgr, workflowsMgr, authStore, apiKeyStore, jwtSecret, serverCfg, tokenStore, auditStore, secretsVault, emergencyMgr, nil, *enableWebsocket, parseWsOrigins(*wsAllowedOrigins), traceStore, deptStore, buildPipelineServices(boot, *pipelineEnable))

	// FASE 1 routing/scope: injeta o roteador determinístico (boot.RouteResolver)
	// e o modo de busca no servidor REST (modular → /v1/knowledge/search confina
	// ao escopo roteado; NoRoute → vazio + no_route:true; legacy → atual).
	if boot != nil && boot.RouteResolver != nil {
		server.SetModularSearch(boot.RouteResolver, searchMode)
	}

	// Perception Loop (opt-in): injeta o serviço no servidor (habilita
	// /v1/perception/*) e o inicia. Quando nil (desabilitado), os endpoints
	// reportam 503 — comportamento default inalterado.
	if perceptionSvc != nil {
		server.SetPerceptionService(perceptionSvc)
		perceptionSvc.Start(context.Background())
	}

	// Perception Bus (FASE A, opt-in): injeta o bus no servidor (habilita
	// /v1/perception/bus*) e o inicia. Nil-safe: quando não opt-in, os endpoints
	// reportam 503. O bus assina o perception loop — nunca o modificou.
	if busSvc != nil {
		server.SetBusService(busSvc)
		busSvc.Start(context.Background())
	}

	// FASE D: memória episódica multimodal (opt-in). Inicia o gravador que
	// assina o bus. Nil-safe: quando não opt-in, nenhum EpisodicRecord é
	// persistido. O sink é aditivo e nunca bloqueia o bus.
	if episodicSink != nil {
		episodicSink.Start(context.Background())
	}

	// FASE C: native-Go TTS speaker (opt-in). Enables /v1/voice/speaks. Nil-safe:
	// quando não opt-in (ou build sem a tag tts_sherpa), o endpoint reporta 503.
	if voiceSpeaker != nil {
		server.SetVoiceSpeaker(voiceSpeaker)
	}

	httpMetrics := metrics.NewHTTPMetrics()

	server.Use(middleware.MetricsMiddleware(httpMetrics))
	server.Use(middleware.LoggingMiddleware(logger))

	if deterministic {
		server.Use(deterministicModeMiddleware)
	}

	grpcMetrics := metrics.NewGRPCMetrics()

	metricsAddr := fmt.Sprintf("%s:%d", *host, *metricsPort)
	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("GET /metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(metrics.FormatMetrics(rtInstance, ke, mem, httpMetrics, grpcMetrics)))
	})
	metricsMux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"healthy":true}`))
	})

	metricsSecret := os.Getenv("COSCA_METRICS_SECRET")
	if metricsSecret == "" {
		if os.Getenv("COSCA_DEV_MODE") == "true" {
			logger.Warn().Msg("COSCA_METRICS_SECRET not set and COSCA_DEV_MODE=true — metrics endpoint has NO authentication (dev only)")
		} else {
			var err error
			metricsSecret, err = randomHexSecret(16)
			if err != nil {
				logger.Fatal().Err(err).Msg("failed to generate metrics secret")
			}
			logger.Warn().Str("metrics_secret", metricsSecret).
				Msg("COSCA_METRICS_SECRET not set — generated a random secret for the metrics endpoint. Set COSCA_METRICS_SECRET to pin the value and silence this warning.")
		}
	}

	metricsHandler := metricsAuthMiddleware(metricsMux, metricsSecret)

	metricsSrv := &http.Server{
		Addr:         metricsAddr,
		Handler:      metricsHandler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	useTLS := *tlsCertFile != "" && *tlsKeyFile != ""
	if *tlsCertFile != "" && *tlsKeyFile == "" {
		logger.Warn().Msg("--tls-cert-file provided without --tls-key-file — starting without TLS")
	} else if *tlsKeyFile != "" && *tlsCertFile == "" {
		logger.Warn().Msg("--tls-key-file provided without --tls-cert-file — starting without TLS")
	}

	var grpcSrv *grpcserver.GRPCServer
	if !*grpcDisable {
		grpcCfg := grpcserver.DefaultConfig()
		grpcCfg.Host = *host
		grpcCfg.Port = *grpcPort
		// FASE 3.5: mirror the project search mode onto the daemon so, in modular
		// mode, the gRPC server applies the same routing/scope as the local path.
		if c, loadErr := config.Load(); loadErr == nil {
			grpcCfg.SearchMode = c.Search.Mode
		}
		grpcCfg.Reflection = *grpcReflection
		grpcCfg.JWTSecret = jwtSecret
		if useTLS {
			grpcCfg.TLSCertFile = *tlsCertFile
			grpcCfg.TLSKeyFile = *tlsKeyFile
		}

		grpcMetricsInterceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
			start := time.Now()
			resp, err := handler(ctx, req)
			st, _ := status.FromError(err)
			grpcMetrics.Record(info.FullMethod, st.Code().String(), time.Since(start))
			return resp, err
		}

		grpcSrv = grpcserver.New(ke, mem, rtInstance, grpcCfg, logger, grpcMetricsInterceptor)
		if grpcSrv == nil {
			logger.Error().Msg("gRPC server failed to initialize (invalid TLS credentials) — gRPC disabled")
		} else {
			logger.Info().Str("grpc_addr", fmt.Sprintf("%s:%d", grpcCfg.Host, grpcCfg.Port)).Bool("grpc_tls", useTLS).Msg("gRPC server configured")
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	var startWg sync.WaitGroup
	errCh := make(chan error, 3)

	startWg.Add(1)
	go func() {
		startWg.Done()
		scheme := "http"
		if useTLS {
			scheme = "https"
		}
		logger.Info().Str("addr", server.Addr()).Str("scheme", scheme).Msg("REST API server listening")
		if useTLS {
			if err := server.ServeTLS(*tlsCertFile, *tlsKeyFile); err != nil && err != http.ErrServerClosed {
				errCh <- fmt.Errorf("REST server: %w", err)
			}
		} else {
			if err := server.Serve(); err != nil && err != http.ErrServerClosed {
				errCh <- fmt.Errorf("REST server: %w", err)
			}
		}
	}()

	if !*grpcDisable && grpcSrv != nil {
		startWg.Add(1)
		go func() {
			startWg.Done()
			logger.Info().Str("addr", grpcSrv.Addr()).Msg("gRPC server listening")
			if err := grpcSrv.Serve(); err != nil {
				errCh <- fmt.Errorf("gRPC server: %w", err)
			}
		}()
	}

	startWg.Add(1)
	go func() {
		startWg.Done()
		logger.Info().Str("addr", metricsAddr).Msg("metrics server listening")
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("metrics server: %w", err)
		}
	}()

	serverStart := time.Now()

	startWg.Wait()

	gitInit := captureGitState(dir)

	return &serveServerResult{
		rest:          server,
		metrics:       metricsSrv,
		grpc:          grpcSrv,
		emergencyCh:   emergencyCh,
		sigCh:         sigCh,
		errCh:         errCh,
		serverStart:   serverStart,
		gitInit:       gitInit,
		perceptionSvc: perceptionSvc,
		busSvc:        busSvc,
		episodicSink:  episodicSink,
		voiceSpeaker:  voiceSpeaker,
	}
}

// ── 8. eventLoop ─────────────────────────────────────────────────────────────

func serveEventLoop(srv *serveServerResult, dir string, logger zerolog.Logger, rtInstance *runtime.Runtime, ke *knowledge.Engine, mem *memory.MemoryEngine, grpcDisable *bool) error {
	var shutdownReason string
	select {
	case sig := <-srv.sigCh:
		shutdownReason = sig.String()
		logger.Info().Str("signal", sig.String()).Msg("received signal, shutting down")
	case reason := <-srv.emergencyCh:
		shutdownReason = "emergency:" + reason
		logger.Warn().Str("reason", reason).Msg("EMERGENCY STOP — kill switch triggered, shutting down")
	case err := <-srv.errCh:
		shutdownReason = "server_error"
		logger.Error().Err(err).Msg("server error, shutting down")
	}

	recordSessionOnShutdown(dir, srv.serverStart, shutdownReason, srv.gitInit, &logger)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if !*grpcDisable && srv.grpc != nil {
		srv.grpc.GracefulStop()
		logger.Info().Msg("gRPC server stopped")
	}

	if err := srv.rest.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("REST server shutdown error")
	} else {
		logger.Info().Msg("REST server stopped")
	}

	if err := srv.metrics.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("metrics server shutdown error")
	} else {
		logger.Info().Msg("metrics server stopped")
	}

	if daemon := rtInstance.Daemon(); daemon != nil && daemon.IsRunning() {
		if err := daemon.Stop(); err != nil {
			logger.Warn().Err(err).Msg("daemon stop error")
		} else {
			logger.Info().Msg("daemon stopped")
		}
	}

	// Perception Loop (opt-in): para o serviço antes do rest do shutdown.
	if srv.perceptionSvc != nil {
		srv.perceptionSvc.Stop()
		logger.Info().Msg("perception loop stopped")
	}

	// Perception Bus (FASE A, opt-in): para na mesma sequência do perception.
	if srv.busSvc != nil {
		srv.busSvc.Stop()
		logger.Info().Msg("perception bus stopped")
	}

	// FASE D: memória episódica multimodal (opt-in): para o gravador.
	if srv.episodicSink != nil {
		srv.episodicSink.Stop()
		logger.Info().Msg("episodic memory sink stopped")
	}

	// FASE C: fecha o Speaker TTS (libera o modelo sherpa/ONNX).
	if srv.voiceSpeaker != nil {
		_ = srv.voiceSpeaker.Close()
		logger.Info().Msg("voice speaker closed")
	}

	if err := rtInstance.Stop(shutdownCtx); err != nil {
		logger.Warn().Err(err).Msg("runtime shutdown error")
	} else {
		logger.Info().Msg("runtime stopped")
	}

	if mem != nil {
		if err := mem.Close(); err != nil {
			logger.Warn().Err(err).Msg("memory engine close error")
		}
	}

	if ke != nil {
		if err := ke.Close(); err != nil {
			logger.Warn().Err(err).Msg("knowledge engine close error")
		}
	}

	logger.Info().Msg("shutdown complete")
	return nil
}

// ── Existing helpers (unchanged) ─────────────────────────────────────────────

// shouldRunDeterministic decides whether the serve daemon runs in
// deterministic mode (no AI provider, provider=none).
//
// Precedence: an explicit --provider flag wins over the COSCA_PROVIDER
// environment variable. Deterministic mode is active only when the winning
// value is "none"; a real provider (flag or env) keeps the current behavior.
func shouldRunDeterministic(flagValue, envValue string) bool {
	flag := strings.ToLower(strings.TrimSpace(flagValue))
	if flag != "" {
		return flag == "none"
	}
	env := strings.ToLower(strings.TrimSpace(envValue))
	return env == "none"
}

// deterministicModeMiddleware guards LLM-dependent REST endpoints when the
// daemon boots with provider=none (deterministic mode). Every endpoint that
// would need a cognitive capability responds 503 with a clear message instead
// of attempting an LLM call. Deterministic endpoints (status, health,
// knowledge, memory, laws, audit, workflows listing) pass through untouched.
func deterministicModeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isLLMRestPath(r.Method, r.URL.Path) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"capacidade cognitiva indisponível — modo determinístico (provider=none)"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isLLMRestPath reports whether the REST route requires an AI provider. Only
// the execution endpoints (run + workflow run) consume LLM tokens.
func isLLMRestPath(method, path string) bool {
	if method != http.MethodPost {
		return false
	}
	if path == "/v1/run" || path == "/v1/run/stream" {
		return true
	}
	if strings.HasPrefix(path, "/v1/workflows/") && (strings.HasSuffix(path, "/run") || strings.HasSuffix(path, "/run/stream")) {
		return true
	}
	return false
}

// metricsAuthMiddleware returns an HTTP middleware that checks for a valid
// Bearer token in the Authorization header. The token must match the
// configured metrics secret, compared in constant time (M9) to avoid timing
// side-channels. Returns 401 on mismatch or missing header. When secret is
// empty (dev mode with no COSCA_METRICS_SECRET) authentication is skipped.
func metricsAuthMiddleware(next http.Handler, secret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if secret == "" {
			next.ServeHTTP(w, r)
			return
		}
		authHeader := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if len(authHeader) <= len(prefix) || authHeader[:len(prefix)] != prefix {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"missing or invalid authorization header"}`))
			return
		}
		token := authHeader[len(prefix):]
		if !metricsTokenMatches(token, secret) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid metrics secret"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// metricsTokenMatches compares a presented token against the expected secret
// in constant time, mitigating timing side-channel attacks.
func metricsTokenMatches(presented, expected string) bool {
	return subtle.ConstantTimeCompare([]byte(presented), []byte(expected)) == 1
}

// randomHexSecret generates a cryptographically random hex string of
// byteLen bytes (2*byteLen hex characters).
func randomHexSecret(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random secret: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// gitState captures key git metrics at serve startup for delta computation
// at shutdown.
type gitState struct {
	commitCount  int
	trackedFiles int
}

// captureGitState reads the initial git state from the project directory.
// Returns zero values when not in a git repository.
func captureGitState(dataDir string) gitState {
	projectDir := filepath.Dir(dataDir)
	var gs gitState

	out, err := runGitCmd(projectDir, "rev-list", "--count", "HEAD")
	if err == nil {
		gs.commitCount, _ = strconv.Atoi(strings.TrimSpace(out))
	}

	out, err = runGitCmd(projectDir, "ls-files")
	if err == nil {
		gs.trackedFiles = len(strings.Split(strings.TrimSpace(out), "\n"))
		if gs.trackedFiles == 1 && out == "" {
			gs.trackedFiles = 0
		}
	}

	return gs
}

// runGitCmd executes a git command in the given directory and returns stdout.
func runGitCmd(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// recordSessionOnShutdown writes a session auto-record to
// <dir>/memory/session/ when the serve process shuts down gracefully.
// Errors are logged but never cause shutdown to fail.
func recordSessionOnShutdown(coscaDir string, startedAt time.Time, reason string, init gitState, logger *zerolog.Logger) {
	uptime := int64(time.Since(startedAt).Seconds())
	summary := memory.SessionSummary{
		Date:           time.Now().Format("2006-01-02"),
		UptimeSeconds:  uptime,
		ShutdownReason: reason,
	}

	projectDir := filepath.Dir(coscaDir)
	if final := captureGitState(coscaDir); final.commitCount > 0 || final.trackedFiles > 0 {
		summary.Commits = final.commitCount - init.commitCount
		if summary.Commits < 0 {
			summary.Commits = 0
		}
		summary.FilesChanged = final.trackedFiles - init.trackedFiles
		if summary.FilesChanged < 0 {
			summary.FilesChanged = 0
		}
		_ = projectDir
	}

	if err := memory.RecordSession(coscaDir, summary); err != nil {
		logger.Warn().Err(err).Msg("session auto-record failed")
	} else {
		logger.Info().
			Int64("uptime_sec", uptime).
			Str("reason", reason).
			Int("commits", summary.Commits).
			Int("files_changed", summary.FilesChanged).
			Msg("session recorded")
	}
}

// parseWsOrigins parses a comma-separated list of WebSocket allowed origin
// host patterns. Returns nil for empty input (same-origin only).
func parseWsOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var origins []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			origins = append(origins, p)
		}
	}
	return origins
}

// configureCORSFromEnv overrides the CORS origins flag from the
// COSCA_CORS_ORIGINS environment variable if the flag is still set to its
// default value ("" — fail-closed, no origins allowed).
func configureCORSFromEnv(corsOrigins *string) {
	if env := os.Getenv("COSCA_CORS_ORIGINS"); env != "" && *corsOrigins == "" {
		*corsOrigins = env
	}
}

// workspaceDir returns the project root for tool execution. Inside the jail
// (COSCA_JAILED=1) the workspace is mounted at "/", so "/" is the project
// root (COSCA_PROJECT_DIR is the host path and does not exist in the bubble).
func workspaceDir() string {
	if os.Getenv("COSCA_JAILED") == "1" {
		return "/"
	}
	if d := os.Getenv("COSCA_PROJECT_DIR"); d != "" {
		return d
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

// resolveDataDir resolves the data directory path. If the provided directory
// is empty, it defaults to .cosca in the current working directory.
func resolveDataDir(dataDir string) (string, error) {
	if dataDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
		return filepath.Join(cwd, ".cosca"), nil
	}
	return dataDir, nil
}

// buildPipelineServices extracts pipeline components from the bootstrap
// result into a rest.PipelineServices struct. Returns nil when pipeline
// is disabled or no components are available.
func buildPipelineServices(boot *bootstrap.Result, enabled bool) *rest.PipelineServices {
	if !enabled || boot == nil || boot.Planner == nil {
		return nil
	}
	return &rest.PipelineServices{
		Enabled:         true,
		Runner:          boot.Runner,
		Orchestrator:    boot.Orchestrator,
		Planner:         boot.Planner,
		StepRunner:      boot.StepRunner,
		RecoveryLoop:    boot.RecoveryLoop,
		PostTaskHook:    boot.PostTaskHook,
		CMITracker:      boot.CMITracker,
		WorkflowHistory: boot.WorkflowHistory,
		CheckpointStore: boot.CheckpointStore,
		PluginRegistry:  boot.PluginRegistry,
	}
}
