package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/api/grpcserver"
	"github.com/CoscaAI/cosca/internal/bootstrap"
	"github.com/CoscaAI/cosca/internal/config"
)

// runtimeGRPCPort is the default gRPC port for the standalone runtime daemon
// (`cosca runtime start`). The REST server (`cosca serve`) keeps its own
// gRPC port (default 14122); the standalone runtime uses 14123 so both can
// run side by side.
const runtimeGRPCPort = 14123

// NewRuntimeCommand creates the `cosca runtime` command and its subcommands.
func NewRuntimeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runtime",
		Short: "Manage the Cosca runtime",
		Long: `Manage the Cosca runtime daemon.

The runtime handles indexing, search, context building, and other background tasks.
`,
		Example: `  cosca runtime status         Show runtime status
  cosca runtime start          Start the runtime
  cosca runtime stop           Stop the runtime
  cosca runtime restart        Restart the runtime
  cosca runtime logs           View runtime logs
  cosca runtime info           Show runtime information`,
	}

	cmd.AddCommand(
		NewRuntimeStartCommand(),
		NewRuntimeStopCommand(),
		NewRuntimeRestartCommand(),
		NewRuntimeStatusCommand(),
		NewRuntimeLogsCommand(),
		NewRuntimeInfoCommand(),
	)

	return cmd
}

// runtimePIDFile returns the daemon PID file path for the given data dir.
// It MUST match the PIDPath the daemon writes in bootstrap.Compose
// (fallback: <dataDir>/cosca.pid).
func runtimePIDFile(dataDir string) string {
	return filepath.Join(dataDir, "cosca.pid")
}

// readDaemonPID reads and parses the daemon PID file. An error is returned
// when the file is missing, empty or malformed.
func readDaemonPID(pidFile string) (int, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse pid file %s: %w", pidFile, err)
	}
	return pid, nil
}

// NewRuntimeStartCommand creates the `cosca runtime start` subcommand.
//
// FASE 1 (DDNA-2026-08-07-001): `runtime start` is now a REAL standalone
// foreground daemon. It composes the full engine stack through the bootstrap
// package (knowledge + memory + runtime + compute + daemon with PID file and
// automatic backup), exposes the gRPC RuntimeService on 127.0.0.1:14123
// (--grpc-port) and blocks until SIGINT/SIGTERM. The reported PID is the
// daemon's own PID (os.Getpid()) — the PID file records the same value.
func NewRuntimeStartCommand() *cobra.Command {
	var grpcPort int

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the runtime daemon (foreground)",
		Long: `Start the runtime daemon in the foreground.

Composes the full engine stack (knowledge, memory, runtime, compute fabric),
starts the background daemon (PID file, watchdog, sync loop, automatic
knowledge backup), and exposes the gRPC RuntimeService on 127.0.0.1:14123.

The process blocks until SIGINT or SIGTERM, then shuts down gracefully:
gRPC → daemon → runtime → memory → knowledge.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runRuntimeStart(cmd, grpcPort)
		},
	}

	cmd.Flags().IntVar(&grpcPort, "grpc-port", runtimeGRPCPort, "gRPC port for the runtime service (loopback)")
	return cmd
}

// runRuntimeStart runs the foreground daemon. Shared by `runtime start` and
// `runtime restart` (stop + start).
func runRuntimeStart(cmd *cobra.Command, grpcPort int) error {
	formatter := GetFormatter(cmd)
	useJSON := IsJSONOutput(cmd)

	// Resolve and harden the data directory (.cosca in the cwd).
	dir, dirErr := resolveDataDir("")
	if dirErr != nil {
		return fmt.Errorf("resolve data directory: %w", dirErr)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return fmt.Errorf("restrict data directory permissions: %w", err)
	}

	logger := log.With().
		Str("component", "runtime").
		Str("data_dir", dir).
		Int("grpc_port", grpcPort).
		Logger()

	// Honor the project config's embedding settings. The config values
	// override the engine's defaults: embedding.provider selects the provider,
	// embedding.base_url points it at a custom endpoint (e.g. a local
	// OpenAI-compatible embeddings server), and api_key/model/dimensions are
	// forwarded as provider overrides. A config load failure is non-fatal and
	// leaves the provider empty so auto-detection applies.
	var (
		embeddingProvider   string
		embeddingBaseURL    string
		embeddingModel      string
		embeddingDigest     string
		embeddingAPIKey     string
		embeddingDimensions int
	)
	if c, loadErr := config.Load(); loadErr != nil {
		logger.Warn().Err(loadErr).Msg("failed to load project config — embedding provider will be auto-detected")
	} else {
		embeddingProvider = c.Embedding.Provider
		embeddingBaseURL = c.Embedding.BaseURL
		embeddingAPIKey = c.Embedding.APIKey
		// Model and dimensions are always forwarded from the project config —
		// the config is the source of truth for the embedding provider.
		// Providers keep their own local defaults (e.g. the openai provider
		// defaults to nomic-embed-text / 768 on the local endpoint) for
		// zero-config behavior, so an explicitly configured value must not be
		// suppressed even when it matches the Cosca defaults.
		embeddingModel = c.Embedding.Model
		embeddingDigest = c.Embedding.Digest
		embeddingDimensions = c.Embedding.Dimensions
	}

	// Compose the engine stack with the daemon always enabled.
	boot, bootErr := bootstrap.Compose(bootstrap.Config{
		DataDir:             dir,
		Logger:              logger,
		EnableDaemon:        true,
		EmbeddingProvider:   embeddingProvider,
		EmbeddingBaseURL:    embeddingBaseURL,
		EmbeddingModel:      embeddingModel,
		EmbeddingDigest:     embeddingDigest,
		EmbeddingAPIKey:     embeddingAPIKey,
		EmbeddingDimensions: embeddingDimensions,
	})
	if bootErr != nil {
		return fmt.Errorf("engine composition failed: %w", bootErr)
	}

	// gRPC RuntimeService on loopback (FASE 1 service: Status/Health).
	grpcCfg := grpcserver.DefaultConfig()
	grpcCfg.Host = "127.0.0.1"
	grpcCfg.Port = grpcPort
	if s := os.Getenv("COSCA_JWT_SECRET"); s != "" {
		grpcCfg.JWTSecret = []byte(s)
	}
	grpcSrv := grpcserver.New(boot.Knowledge, boot.Memory, boot.Runtime, grpcCfg, logger)
	if grpcSrv == nil {
		return fmt.Errorf("gRPC server failed to initialize")
	}

	pidFile := runtimePIDFile(dir)
	addr := fmt.Sprintf("127.0.0.1:%d", grpcPort)

	if useJSON {
		if err := printJSON(cmd, map[string]string{
			"status":   "started",
			"pid":      strconv.Itoa(os.Getpid()),
			"data_dir": dir,
			"grpc":     addr,
			"pid_file": pidFile,
		}); err != nil {
			return err
		}
	} else {
		fmt.Println("Runtime daemon started (foreground)")
		fmt.Printf("PID: %d\n", os.Getpid())
		fmt.Printf("Data dir: %s\n", dir)
		fmt.Printf("gRPC addr: %s\n", addr)
		fmt.Printf("PID file: %s\n", pidFile)
	}

	// Foreground wait: block until SIGINT/SIGTERM or a gRPC server error.
	// cmd.Context() lets the parent command (and tests) cancel the daemon.
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info().Str("addr", addr).Msg("gRPC server listening")
		if err := grpcSrv.Serve(); err != nil {
			errCh <- fmt.Errorf("gRPC server: %w", err)
		}
	}()

	var runErr error
	select {
	case <-ctx.Done():
		logger.Info().Msg("received signal, shutting down")
	case err := <-errCh:
		runErr = err
		logger.Error().Err(err).Msg("gRPC server error, shutting down")
	}

	// ── Graceful shutdown (same order as serve) ────────────────────────
	grpcSrv.GracefulStop()
	logger.Info().Msg("gRPC server stopped")

	if daemon := boot.Runtime.Daemon(); daemon != nil && daemon.IsRunning() {
		if err := daemon.Stop(); err != nil {
			logger.Warn().Err(err).Msg("daemon stop error")
		} else {
			logger.Info().Msg("daemon stopped")
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := boot.Runtime.Stop(shutdownCtx); err != nil {
		logger.Warn().Err(err).Msg("runtime shutdown error")
	} else {
		logger.Info().Msg("runtime stopped")
	}

	if boot.Memory != nil {
		if err := boot.Memory.Close(); err != nil {
			logger.Warn().Err(err).Msg("memory engine close error")
		}
	}
	if boot.Knowledge != nil {
		if err := boot.Knowledge.Close(); err != nil {
			logger.Warn().Err(err).Msg("knowledge engine close error")
		}
	}

	if runErr != nil {
		return fmt.Errorf("runtime daemon failed: %w", runErr)
	}

	if useJSON {
		return printJSON(cmd, map[string]string{"status": "stopped"})
	}
	formatter.Success("Runtime stopped")
	return nil
}

// NewRuntimeStopCommand creates the `cosca runtime stop` subcommand.
//
// FASE 1: stop is real. It reads the daemon PID file (<dir>/cosca.pid),
// sends SIGTERM, waits up to ~10s for exit, then SIGKILL if the process
// persists, and finally cleans any orphan PID file. Missing or dead PID →
// honest "not running" (exit 0, no error). It NEVER reports success without
// actually stopping a process.
func NewRuntimeStopCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the runtime daemon",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return stopRuntimeDaemon(cmd)
		},
	}
	return cmd
}

// stopRuntimeDaemon implements the real PID-file-based stop. Shared by
// `runtime stop` and `runtime restart`.
func stopRuntimeDaemon(cmd *cobra.Command) error {
	formatter := GetFormatter(cmd)
	useJSON := IsJSONOutput(cmd)

	dir, dirErr := resolveDataDir("")
	if dirErr != nil {
		return fmt.Errorf("resolve data directory: %w", dirErr)
	}
	pidFile := runtimePIDFile(dir)

	pid, err := readDaemonPID(pidFile)
	if err != nil || pid <= 0 || !processAlive(pid) {
		// Not running. Clean any orphan PID file (stale entry from a
		// crashed daemon) and report honestly — exit 0, no error.
		if pid > 0 {
			_ = os.Remove(pidFile)
		}
		if useJSON {
			return printJSON(cmd, map[string]string{"status": "stopped", "pid": "none"})
		}
		formatter.Warning("Runtime is not running")
		return nil
	}

	// Graceful stop: SIGTERM first, wait up to ~10s, then SIGKILL.
	// signalProcess é platform-specific (syscall.Kill no Unix; no Windows o
	// sinal é mapeado para terminação forçada — não há SIGTERM nativo).
	if err := signalProcess(pid, syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to signal runtime (pid %d): %w", pid, err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for processAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}

	if processAlive(pid) {
		log.Warn().Int("pid", pid).Msg("runtime did not stop after SIGTERM — sending SIGKILL")
		if err := signalProcess(pid, syscall.SIGKILL); err != nil {
			return fmt.Errorf("failed to force-stop runtime (pid %d): %w", pid, err)
		}
		// Small grace period for the kill to take effect.
		time.Sleep(500 * time.Millisecond)
	}

	// Clean up in case the daemon could not remove its own PID file.
	_ = os.Remove(pidFile)

	if useJSON {
		return printJSON(cmd, map[string]string{"status": "stopped", "pid": strconv.Itoa(pid)})
	}
	formatter.Success("Runtime stopped")
	return nil
}

// NewRuntimeRestartCommand creates the `cosca runtime restart` subcommand.
//
// FASE 1: restart = real stop (PID file) + foreground start.
func NewRuntimeRestartCommand() *cobra.Command {
	var grpcPort int

	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart the runtime daemon",
		Long: `Restart the runtime daemon: stops the running daemon (PID file),
then starts a new foreground daemon (see 'cosca runtime start').`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := stopRuntimeDaemon(cmd); err != nil {
				return err
			}
			return runRuntimeStart(cmd, grpcPort)
		},
	}

	cmd.Flags().IntVar(&grpcPort, "grpc-port", runtimeGRPCPort, "gRPC port for the runtime service (loopback)")
	return cmd
}

// NewRuntimeStatusCommand creates the `cosca runtime status` subcommand.
//
// FASE 1: status reports REALITY — the daemon is "running" only when the PID
// file exists AND the process is alive (processAlive). Never a canned value.
func NewRuntimeStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show runtime status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, dirErr := resolveDataDir("")
			if dirErr != nil {
				return fmt.Errorf("resolve data directory: %w", dirErr)
			}
			pidFile := runtimePIDFile(dir)

			pid, err := readDaemonPID(pidFile)
			running := err == nil && pid > 0 && processAlive(pid)

			if useJSON {
				if running {
					return printJSON(cmd, map[string]string{
						"state":    "running",
						"pid":      strconv.Itoa(pid),
						"pid_file": pidFile,
					})
				}
				return printJSON(cmd, map[string]string{
					"state":    "stopped",
					"pid_file": pidFile,
				})
			}

			formatter.Header("Runtime Status")
			if running {
				formatter.KeyValue("State", "running")
				formatter.KeyValue("PID", strconv.Itoa(pid))
			} else {
				formatter.KeyValue("State", "stopped")
			}
			formatter.KeyValue("PID file", pidFile)

			return nil
		},
	}
	return cmd
}

// NewRuntimeLogsCommand creates the `cosca runtime logs` subcommand.
//
// FASE 1: the daemon runs in the foreground and writes logs to its stdout —
// there is no persistent log file yet. Logs reports this honestly instead of
// returning the old placeholder lines.
func NewRuntimeLogsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View runtime logs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

			dir, dirErr := resolveDataDir("")
			if dirErr != nil {
				return fmt.Errorf("resolve data directory: %w", dirErr)
			}

			formatter.Header("Runtime Logs")
			formatter.Println("The runtime daemon runs in the foreground — logs are written to its stdout.")
			formatter.Println("Data dir: " + dir)
			formatter.Println("Start it with: cosca runtime start")

			return nil
		},
	}
	return cmd
}

// NewRuntimeInfoCommand creates the `cosca runtime info` subcommand.
//
// FASE 1: info reports honest daemon facts — version, state (derived from
// the PID file + process aliveness), PID file path and the gRPC endpoint.
func NewRuntimeInfoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show runtime information",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, dirErr := resolveDataDir("")
			if dirErr != nil {
				return fmt.Errorf("resolve data directory: %w", dirErr)
			}
			pidFile := runtimePIDFile(dir)

			pid, err := readDaemonPID(pidFile)
			running := err == nil && pid > 0 && processAlive(pid)

			state := "stopped"
			if running {
				state = "running"
			}

			if useJSON {
				return printJSON(cmd, map[string]string{
					"version":  Version,
					"state":    state,
					"pid_file": pidFile,
					"grpc":     fmt.Sprintf("127.0.0.1:%d", runtimeGRPCPort),
				})
			}

			formatter.Header("Runtime Information")
			formatter.KeyValue("Version", Version)
			formatter.KeyValue("State", state)
			formatter.KeyValue("PID file", pidFile)
			formatter.KeyValue("gRPC addr", fmt.Sprintf("127.0.0.1:%d", runtimeGRPCPort))

			return nil
		},
	}
	return cmd
}
