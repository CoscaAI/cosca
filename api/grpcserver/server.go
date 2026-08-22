package grpcserver

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog"

	cospb "github.com/CoscaAI/cosca/api/grpc/pb"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/runtime"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

const defaultGracefulStopTimeout = 10 * time.Second

var gracefulStopTimeout = defaultGracefulStopTimeout

// Config holds configuration for the gRPC server.
type Config struct {
	// Host is the address to listen on (default "127.0.0.1" — loopback
	// only). Pass "0.0.0.0" to expose gRPC on the network; if you do,
	// ALWAYS provide TLS credentials so JWT tokens and payloads are not
	// transmitted in plaintext.
	Host string
	// Port is the TCP port to listen on (default 14122).
	Port int
	// Reflection enables gRPC server reflection for development tooling
	// (e.g. grpcurl). Disabled by default in production.
	Reflection bool
	// JWTSecret is the HMAC key used for JWT token validation. When empty
	// or nil, authentication is skipped (dev mode).
	JWTSecret []byte
	// TLSCertFile is the PEM-encoded TLS certificate file. When both
	// TLSCertFile and TLSKeyFile are set, the server is started with TLS
	// credentials (recommended when Host is not loopback). Empty = no TLS.
	TLSCertFile string
	// TLSKeyFile is the PEM-encoded TLS private key file. Must be provided
	// together with TLSCertFile. Empty = no TLS.
	TLSKeyFile string
}

// DefaultConfig returns a Config with safe production defaults.
func DefaultConfig() Config {
	return Config{
		Host:       "127.0.0.1",
		Port:       14122,
		Reflection: false,
	}
}

// GRPCServer wraps a gRPC server with lifecycle management and registers
// all Cosca services (knowledge, memory, runtime) on it.
type GRPCServer struct {
	server   *grpc.Server
	config   Config
	listener net.Listener
	logger   zerolog.Logger

	gracefulStopOnce sync.Once

	// Service instances for registration and later shutdown.
	runtimeSrv   *RuntimeServiceServer
	knowledgeSrv *KnowledgeServiceServer
	memorySrv    *MemoryServiceServer
}

// New creates a new GRPCServer, registers all available services and
// interceptors, and returns it ready to Serve(). ke, mem, and rt may be
// nil — the corresponding service will return codes.FailedPrecondition
// until the engine is wired in a future phase.
func New(ke *knowledge.Engine, mem *memory.MemoryEngine, rt *runtime.Runtime, cfg Config, logger zerolog.Logger, extraInterceptors ...grpc.UnaryServerInterceptor) *GRPCServer {
	// Build the interceptor chain: recovery first (outermost), then auth
	// (when a JWT secret is configured), then logging.
	unaryInterceptors := []grpc.UnaryServerInterceptor{
		RecoveryInterceptor(),
	}
	if len(cfg.JWTSecret) > 0 {
		unaryInterceptors = append(unaryInterceptors, AuthInterceptor(cfg.JWTSecret))
	}
	unaryInterceptors = append(unaryInterceptors,
		LoggingInterceptor(logger),
	)
	unaryInterceptors = append(unaryInterceptors, extraInterceptors...)

	serverOpts := []grpc.ServerOption{grpc.ChainUnaryInterceptor(unaryInterceptors...)}

	// TLS: when both a certificate and a key are provided, serve encrypted.
	// Without them the server stays plaintext (loopback-only by default),
	// which is acceptable for local dev tooling but must not be exposed on
	// the network.
	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		creds, err := credentials.NewServerTLSFromFile(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			logger.Error().Err(err).
				Str("cert_file", cfg.TLSCertFile).
				Str("key_file", cfg.TLSKeyFile).
				Msg("gRPC server failed to load TLS credentials — refusing to start (fail closed, no plaintext fallback)")
			return nil
		}
		serverOpts = append(serverOpts, grpc.Creds(creds))
	}

	srv := grpc.NewServer(serverOpts...)

	gs := &GRPCServer{
		server: srv,
		config: cfg,
		logger: logger.With().Str("component", "grpcserver").Logger(),
	}

	// Register RuntimeService (FASE 1).
	gs.runtimeSrv = NewRuntimeServiceServer(rt)
	cospb.RegisterRuntimeServiceServer(srv, gs.runtimeSrv)
	if cfg.Reflection {
		reflection.Register(srv)
	}

	// Register KnowledgeService (FASE 2).
	gs.knowledgeSrv = NewKnowledgeServiceServer(ke)
	cospb.RegisterKnowledgeServiceServer(srv, gs.knowledgeSrv)

	// Register MemoryService (FASE 3).
	if mem != nil {
		gs.memorySrv = NewMemoryServiceServer(mem)
		cospb.RegisterMemoryServiceServer(srv, gs.memorySrv)
	}

	return gs
}

// Serve starts the gRPC server on the configured address and blocks until
// the server stops (via GracefulStop/Stop) or the listener fails.
func (gs *GRPCServer) Serve() error {
	addr := fmt.Sprintf("%s:%d", gs.config.Host, gs.config.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("gRPC server listen on %s: %w", addr, err)
	}
	gs.listener = lis

	gs.logger.Info().
		Str("addr", addr).
		Bool("reflection", gs.config.Reflection).
		Bool("tls", gs.config.TLSCertFile != "" && gs.config.TLSKeyFile != "").
		Msg("gRPC server starting")
	return gs.server.Serve(lis)
}

// GracefulStop stops the gRPC server gracefully, waiting for pending RPCs up
// to a bounded deadline before forcing the server to stop.
func (gs *GRPCServer) GracefulStop() {
	gs.gracefulStopOnce.Do(func() {
		gs.logger.Info().Msg("gRPC server graceful stop initiated")

		gracefulDone := make(chan struct{})
		go func() {
			gs.server.GracefulStop()
			close(gracefulDone)
		}()

		timer := time.NewTimer(gracefulStopTimeout)
		defer timer.Stop()

		select {
		case <-gracefulDone:
			gs.logger.Info().Msg("gRPC server stopped gracefully")
		case <-timer.C:
			gs.logger.Warn().
				Dur("timeout", gracefulStopTimeout).
				Msg("gRPC graceful stop timed out; forcing server stop")
			gs.server.Stop()
		}
	})
}

// Stop stops the gRPC server immediately, aborting any pending RPCs.
func (gs *GRPCServer) Stop() {
	gs.logger.Info().Msg("gRPC server force stop initiated")
	gs.server.Stop()
	gs.logger.Info().Msg("gRPC server stopped")
}

// Addr returns the address the server is listening on, or an empty string
// if Serve() has not been called yet.
func (gs *GRPCServer) Addr() string {
	if gs.listener != nil {
		return gs.listener.Addr().String()
	}
	return fmt.Sprintf("%s:%d", gs.config.Host, gs.config.Port)
}
