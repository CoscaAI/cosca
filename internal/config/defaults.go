//
// Default configuration values for the Cosca platform.
// These defaults are used when no configuration file exists
// and no environment variables are set.

package config

import "time"

// =============================================================================
// Default Paths
// =============================================================================

const (
	// DefaultCoscaHome is the default Cosca home directory.
	DefaultCoscaHome = "~/.config/cosca"
	// DefaultCoscaProjectDir is the default project-relative Cosca directory.
	DefaultCoscaProjectDir = ".cosca"

	// DefaultRuntimeDir is the default runtime directory name.
	DefaultRuntimeDir = "runtime"
	// DefaultDataDir is the default data directory name.
	DefaultDataDir = "data"
	// DefaultCacheDir is the default cache directory name.
	DefaultCacheDir = "cache"
	// DefaultLogsDir is the default logs directory name.
	DefaultLogsDir = "logs"
	// DefaultTempDir is the default temp directory name.
	DefaultTempDir = "tmp"
	// DefaultPluginsDir is the default plugins directory name.
	DefaultPluginsDir = "plugins"
	// DefaultBackupDir is the default backup directory name.
	DefaultBackupDir = "backups"
)

// =============================================================================
// Default Timeouts
// =============================================================================

const (
	// DefaultCommandTimeout is the default timeout for command execution.
	DefaultCommandTimeout = 30 * time.Second
	// DefaultAgentTimeout is the default timeout for agent execution.
	DefaultAgentTimeout = 5 * time.Minute
	// DefaultSkillTimeout is the default timeout for skill execution.
	DefaultSkillTimeout = 2 * time.Minute
	// DefaultWorkflowTimeout is the default timeout for workflow execution.
	DefaultWorkflowTimeout = 10 * time.Minute
	// DefaultRequestTimeout is the default HTTP request timeout.
	DefaultRequestTimeout = 30 * time.Second
	// DefaultWatchTimeout is the default file watcher debounce timeout.
	DefaultWatchTimeout = 100 * time.Millisecond
	// DefaultGracefulShutdown is the default graceful shutdown timeout.
	DefaultGracefulShutdown = 10 * time.Second
	// DefaultHealthCheckInterval is the default health check interval.
	DefaultHealthCheckInterval = 30 * time.Second

	// DefaultMaxRetries is the default number of retry attempts.
	DefaultMaxRetries = 3
	// DefaultRetryDelay is the default delay between retries.
	DefaultRetryDelay = 1 * time.Second
	// DefaultMaxBackoff is the maximum backoff duration.
	DefaultMaxBackoff = 30 * time.Second
)

// =============================================================================
// Default Cache & Database
// =============================================================================

const (
	// DefaultCacheSize is the default in-memory cache size (entries).
	DefaultCacheSize = 10000
	// DefaultCacheTTL is the default cache TTL.
	DefaultCacheTTL = 5 * time.Minute
	// DefaultMaxOpenDBConn is the default max open database connections.
	DefaultMaxOpenDBConn = 25
	// DefaultMaxIdleDBConn is the default max idle database connections.
	DefaultMaxIdleDBConn = 5
	// DefaultDBConnMaxLifetime is the default max connection lifetime.
	DefaultDBConnMaxLifetime = 30 * time.Minute
	// DefaultDBConnMaxIdleTime is the default max idle connection time.
	DefaultDBConnMaxIdleTime = 5 * time.Minute
	// DefaultSearchResultLimit is the default search result limit.
	DefaultSearchResultLimit = 50
	// DefaultSearchMaxResults is the maximum search results allowed.
	DefaultSearchMaxResults = 1000
)

// =============================================================================
// Default Provider Configuration
// =============================================================================

const (
	// DefaultProvider is the default LLM provider.
	DefaultProvider = "ollama"
	// DefaultProviderModel is the default model to use.
	DefaultProviderModel = "qwen2.5-coder:14b-128k"
	// DefaultProviderContextWindow is the default context window (tokens).
	DefaultProviderContextWindow = 131072
	// DefaultProviderMaxTokens is the default max tokens for provider calls.
	DefaultProviderMaxTokens = 16384
	// DefaultProviderTemperature is the default temperature for provider calls.
	DefaultProviderTemperature = 0.2
	// DefaultEmbeddingModel is the default embedding model.
	DefaultEmbeddingModel = "nomic-embed-text"
	// DefaultEmbeddingDimensions is the default embedding dimensions.
	DefaultEmbeddingDimensions = 768

	// DefaultMaxConcurrentOps is the default max concurrent operations.
	DefaultMaxConcurrentOps = 10
	// DefaultRateLimitPerMin is the default rate limit per minute.
	DefaultRateLimitPerMin = 60
)

// =============================================================================
// Default Editor
// =============================================================================

const (
	// DefaultEditor is the default text editor.
	DefaultEditor = "vim"
	// DefaultEditorTheme is the default editor theme.
	DefaultEditorTheme = "default"
	// DefaultEditorFontSize is the default editor font size.
	DefaultEditorFontSize = 12
)

// =============================================================================
// Default Performance & Resource Targets
// =============================================================================

const (
	// DefaultMaxMemoryMB is the default max memory usage in MB.
	DefaultMaxMemoryMB = 512
	// DefaultMaxOpenFiles is the default max open file descriptors.
	DefaultMaxOpenFiles = 1024
	// DefaultLogMaxSizeMB is the default max log file size in MB.
	DefaultLogMaxSizeMB = 100
	// DefaultLogMaxBackups is the default max number of log backups.
	DefaultLogMaxBackups = 5
	// DefaultLogMaxAgeDays is the default max log age in days.
	DefaultLogMaxAgeDays = 30
	// DefaultDBPageSize is the default SQLite page size.
	DefaultDBPageSize = 4096
	// DefaultDBCacheSizeKB is the default SQLite cache size in KB.
	DefaultDBCacheSizeKB = 65536 // 64 MB
	// DefaultBatchSize is the default batch processing size.
	DefaultBatchSize = 100
	// DefaultVectorDimensions is the default vector embedding dimensions.
	DefaultVectorDimensions = 1536
	// DefaultIndexInterval is the default indexing interval.
	DefaultIndexInterval = 5 * time.Second
)

// =============================================================================
// Default Feature Flags
// =============================================================================

const (
	// DefaultEnableMetrics enables metrics collection by default.
	DefaultEnableMetrics = false
	// DefaultEnableTelemetry enables telemetry by default.
	DefaultEnableTelemetry = true
	// DefaultEnableAutoUpdate enables auto-update checks by default.
	DefaultEnableAutoUpdate = true
	// DefaultEnableVectorSearch enables vector search by default.
	// true desde 2026-08-22 (ordem do Don): o gargalo histórico foi resolvido
	// pelo fast path int8 AVX2 (docs/reports/performance-int8-fastpath-2026-08-17.md,
	// 52.66 Mvec/s no limite físico da máquina), a doc de configuração já
	// recomendava vector_search: true e o motor de busca (internal/search)
	// sempre assumiu default true. O Ollama local (nomic-embed-text) cobre a
	// dependência de embeddings. Sem conteúdo indexado a flag é inofensiva —
	// o vetor store vazio simplesmente não contribui resultados.
	DefaultEnableVectorSearch = true
	// DefaultEnableGraphSearch enables graph search by default.
	DefaultEnableGraphSearch = false
	// DefaultEnableWatch enables file watching by default.
	DefaultEnableWatch = true
	// DefaultEnablePluginSystem enables the plugin system by default.
	DefaultEnablePluginSystem = false
	// DefaultEnableVision enables the automatic vision recognition hook in the
	// agent loop by default. Opt-in (false): agents that do not want vision keep
	// their exact prior behaviour — no regression. Set vision.enabled: true (or
	// COSCA_VISION__ENABLED=true) to turn on auto-recognition.
	DefaultEnableVision = false
	// DefaultChangeDetection is the default change-detection gate on the
	// Perception Loop. Enabled by default (the gocv motion-detect learning:
	// only run vision when the screen changes).
	DefaultChangeDetection = true
	// DefaultChangeDetectionThreshold is the default normalised mean-absolute-
	// delta above which a capture is considered a real change (in the 0.02–0.05
	// window from the motion-detect learnings).
	DefaultChangeDetectionThreshold = 0.02

	// DefaultPerceptionAudioWindow is the default temporal window (5s) the
	// Perception Bus uses to remember recent observations for multimodal sync.
	DefaultPerceptionAudioWindow = 5 * time.Second
	// DefaultPerceptionAudioTolerance is the default overlap tolerance (300ms)
	// used to bind an audio segment to the vision frames around it.
	DefaultPerceptionAudioTolerance = 300 * time.Millisecond
	// DefaultPerceptionAudioSampleRate is the default audio sample rate (Hz).
	DefaultPerceptionAudioSampleRate = 16000
	// DefaultPerceptionAudioChunkMS is the default audio chunk length (ms).
	DefaultPerceptionAudioChunkMS = 100

	// DefaultEpisodicTTL is the retention of multimodal episodic memory
	// (FASE D): 30 days. Nothing grows forever — expired records are pruned.
	DefaultEpisodicTTL = 30 * 24 * time.Hour
	// DefaultEpisodicMaxRecords is the soft cap of episodic records. Above it
	// the oldest are trimmed. Bounded memory, never unbounded.
	DefaultEpisodicMaxRecords = 5000
)

// DefaultPerceptionSTTModelType is the default streaming STT model architecture.
const DefaultPerceptionSTTModelType = "transducer"

// DefaultSTTConfig returns the sensible-default native-Go STT config (FASE B).
// Provider is "" (disabled) so existing Fase A behaviour is bit-for-bit
// unchanged until the user opts into `perception.audio.stt.provider: sherpa`.
func DefaultSTTConfig() STTConfig {
	return STTConfig{
		Provider:       "", // opt-in: no STT until explicitly set to "sherpa"
		SampleRate:     16000,
		NumThreads:      2,
		Device:          "cpu",
		DecodingMethod:  "greedy_search",
		EnableEndpoint:  true,
		ModelType:       DefaultPerceptionSTTModelType,
	}
}

// DefaultMicConfig returns the sensible-default microphone capture config
// (FASE D). Enabled is false (opt-in) so existing behaviour is bit-for-bit
// unchanged until the user opts into `perception.audio.mic.enabled: true`
// (plus a sherpa STT provider). Device 0 = system default capture device.
func DefaultMicConfig() MicConfig {
	return MicConfig{
		Enabled:    false, // opt-in: no mic capture until explicitly enabled
		Device:     0,     // WAVE_MAPPER default capture device
		SampleRate: 16000,
		Channels:   1,
		ChunkMS:    100,
	}
}

// DefaultPerceptionTTSModelType is the default offline TTS model architecture.
const DefaultPerceptionTTSModelType = "vits"

// DefaultTTSConfig returns the sensible-default native-Go TTS config (FASE C).
// Provider is "" (disabled) so the speaking loop stays silent (no-op) until the
// user opts into `perception.audio.tts.provider: sherpa`.
func DefaultTTSConfig() TTSConfig {
	return TTSConfig{
		Provider:   "", // opt-in: no TTS until explicitly set to "sherpa"
		SampleRate: 16000,
		NumThreads: 2,
		Device:     "cpu",
		ModelType:  DefaultPerceptionTTSModelType,
		Speed:      1.0,
		Sid:        0,
	}
}

// =============================================================================
// Default Network
// =============================================================================

const (
	// DefaultAPIPort is the default API server port.
	DefaultAPIPort = 8370
	// DefaultMetricsPort is the default metrics server port.
	DefaultMetricsPort = 8371
	// DefaultRPCPort is the default gRPC server port.
	DefaultRPCPort = 8372
	// DefaultHost is the default bind host.
	DefaultHost = "127.0.0.1"
)
