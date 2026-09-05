package grpcserver

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/CoscaAI/cosca/api/auth"
	internalauth "github.com/CoscaAI/cosca/internal/auth"
)

// LoggingInterceptor returns a gRPC unary server interceptor that logs
// every RPC call: method name, duration, and resulting status code.
// It uses zerolog contextualized with "component" = "grpc".
func LoggingInterceptor(logger zerolog.Logger) grpc.UnaryServerInterceptor {
	grpcLogger := logger.With().Str("component", "grpc").Logger()

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		st, _ := status.FromError(err)
		logEvent := grpcLogger.Info().
			Str("method", info.FullMethod).
			Dur("duration", duration).
			Str("code", st.Code().String())

		if err != nil {
			logEvent = grpcLogger.Warn().
				Str("method", info.FullMethod).
				Dur("duration", duration).
				Str("code", st.Code().String()).
				Str("error", st.Message())
		}
		logEvent.Msg("grpc call")

		return resp, err
	}
}

// RecoveryInterceptor returns a gRPC unary server interceptor that recovers
// from panics in the handler chain and converts them into codes.Internal
// errors with a generic message. This prevents the entire server from
// crashing due to an unhandled panic in a single RPC handler.
func RecoveryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				resp = nil
				err = status.Errorf(codes.Internal, "internal server error: panic recovered in %s", info.FullMethod)
			}
		}()
		return handler(ctx, req)
	}
}

// AuthInterceptor returns a gRPC unary server interceptor that authenticates
// requests via JWT bearer tokens. It extracts the "authorization" metadata,
// expects "Bearer <token>" format, validates the token using
// internalauth.ValidateAccessToken (so refresh tokens are rejected), and
// stores the parsed claims in the request context under auth.ContextKeyClaims
// for downstream handlers.
//
// Loopback exemption: when the caller's peer IP is 127.0.0.1 or ::1
// (local inter-process communication), authentication is skipped. The
// security boundary is at the network edge (TLS, firewall, closed port);
// requiring JWT between two processes on the same loopback is security
// theater — a compromised local process can read COSCA_JWT_SECRET from
// the environment anyway. If gRPC is ever exposed on 0.0.0.0, TLS + JWT
// are mandatory.
//
// If secret is nil or empty, the interceptor passes all requests through
// without authentication (dev mode). On authentication failure it returns
// codes.Unauthenticated.
func AuthInterceptor(secret []byte) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Dev mode: no auth required when secret is not configured.
		if len(secret) == 0 {
			return handler(ctx, req)
		}

		// Loopback exemption: local inter-process communication (serve ↔ runtime
		// on the same machine) skips JWT auth. The security boundary is at the
		// network edge — not between processes on 127.0.0.1.
		if isLoopbackPeer(ctx) {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "missing metadata")
		}

		authHeader := md.Get("authorization")
		if len(authHeader) == 0 {
			return nil, status.Errorf(codes.Unauthenticated, "missing authorization header")
		}

		authVal := authHeader[0]
		if !strings.HasPrefix(authVal, "Bearer ") {
			return nil, status.Errorf(codes.Unauthenticated, "invalid authorization format")
		}

		tokenStr := strings.TrimPrefix(authVal, "Bearer ")
		if tokenStr == "" {
			return nil, status.Errorf(codes.Unauthenticated, "empty token")
		}

		claims, err := internalauth.ValidateAccessToken(tokenStr, secret)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
		}

		ctx = context.WithValue(ctx, auth.ContextKeyClaims, claims)
		return handler(ctx, req)
	}
}

// isLoopbackPeer checks whether the gRPC peer address is loopback
// (127.0.0.1 or ::1). Local inter-process calls are exempt from JWT auth.
// If the peer cannot be determined, returns false (auth required — fail closed).
func isLoopbackPeer(ctx context.Context) bool {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return false
	}
	addr := p.Addr.String()
	// Strip port: gRPC peer addresses include the port (e.g. "127.0.0.1:45678").
	if host, _, err := net.SplitHostPort(addr); err == nil {
		addr = host
	}
	ip := net.ParseIP(addr)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}
