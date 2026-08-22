// Package auth provides JWT authentication and user management for the Cosca REST API.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Claims represents the JWT claims embedded in access and refresh tokens.
type Claims struct {
	Sub      string `json:"sub"`                 // User ID
	TenantID string `json:"tenant_id,omitempty"` // Tenant boundary, when configured
	Username string `json:"username"`            // Username
	Role     string `json:"role"`                // admin, editor, viewer
	Type     string `json:"type,omitempty"`      // "access" or "refresh"
	JTI      string `json:"jti,omitempty"`       // JWT ID — unique per token (revocation tracking)
	Iat      int64  `json:"iat"`                 // Issued at (unix seconds)
	Exp      int64  `json:"exp"`                 // Expiration (unix seconds)
}

// TokenPair contains an access token and a refresh token returned from a
// successful login or refresh operation.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

const (
	// AccessTokenTTL is the lifetime of an access token.
	AccessTokenTTL = 15 * time.Minute

	// RefreshTokenTTL is the lifetime of a refresh token.
	RefreshTokenTTL = 7 * 24 * time.Hour
)

// Common auth errors returned by token and user operations.
var (
	ErrTokenExpired     = errors.New("token has expired")
	ErrInvalidSignature = errors.New("invalid token signature")
	ErrInvalidToken     = errors.New("invalid token format")
	ErrTokenType        = errors.New("token type mismatch")
)

// header is the JWT header used for every token.
type header struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

var jwtHeader = header{Alg: "HS256", Typ: "JWT"}

// encodeSegment base64url-encodes a byte slice without padding.
func encodeSegment(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

// decodeSegment base64url-decodes a string.
func decodeSegment(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

// GenerateToken creates a signed JWT with the given claims and secret.
// Returns the complete token string: header.payload.signature.
//
// If claims.JTI is empty, a random UUID is generated and used as the JWT ID
// so every issued token is unique and individually revocable.
func GenerateToken(claims Claims, secret []byte) (string, error) {
	if claims.JTI == "" {
		claims.JTI = uuid.New().String()
	}

	// Marshal and encode the header.
	headerJSON, err := json.Marshal(jwtHeader)
	if err != nil {
		return "", fmt.Errorf("marshal header: %w", err)
	}
	headerEnc := encodeSegment(headerJSON)

	// Marshal and encode the claims.
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}
	claimsEnc := encodeSegment(claimsJSON)

	// Build the signing input: header.payload.
	signingInput := headerEnc + "." + claimsEnc

	// Compute HMAC-SHA256 signature.
	signature := sign([]byte(signingInput), secret)
	signatureEnc := encodeSegment(signature)

	return signingInput + "." + signatureEnc, nil
}

// ValidateToken parses and validates a JWT token string.
// It verifies the signature and expiration. On success, the parsed
// Claims are returned.
//
// DEPRECATED for authentication use: this function does NOT check the token
// Type claim, so a refresh token would be accepted anywhere an access token
// is expected (see A4 — refresh tokens used as access tokens). Use
// ValidateAccessToken or ValidateRefreshToken instead. It is kept for
// internal/compatibility use and for callers that manage tokens of any type.
func ValidateToken(tokenStr string, secret []byte) (*Claims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	headerEnc := parts[0]
	claimsEnc := parts[1]
	signatureEnc := parts[2]

	// Decode the signature.
	expectedSig, err := decodeSegment(signatureEnc)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	// Verify signature.
	signingInput := headerEnc + "." + claimsEnc
	computedSig := sign([]byte(signingInput), secret)
	if !hmac.Equal(expectedSig, computedSig) {
		return nil, ErrInvalidSignature
	}

	// Decode and unmarshal claims.
	claimsJSON, err := decodeSegment(claimsEnc)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	var claims Claims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	// Check expiration.
	// Reject tokens with no expiration (Exp=0) as a security measure.
	now := time.Now().Unix()
	if claims.Exp <= 0 || claims.Exp < now {
		return nil, ErrTokenExpired
	}

	return &claims, nil
}

// ValidateAccessToken parses and validates a JWT as an ACCESS token: it
// verifies the signature, the expiration, AND that the Type claim is exactly
// "access". Refresh tokens are rejected — a leaked refresh token can never be
// used to call protected endpoints.
func ValidateAccessToken(tokenStr string, secret []byte) (*Claims, error) {
	return validateTokenOfType(tokenStr, secret, "access")
}

// ValidateRefreshToken parses and validates a JWT as a REFRESH token: it
// verifies the signature, the expiration, AND that the Type claim is exactly
// "refresh". Access tokens are rejected on the refresh endpoint.
func ValidateRefreshToken(tokenStr string, secret []byte) (*Claims, error) {
	return validateTokenOfType(tokenStr, secret, "refresh")
}

// validateTokenOfType validates a token and enforces the required Type claim.
func validateTokenOfType(tokenStr string, secret []byte, want string) (*Claims, error) {
	claims, err := ValidateToken(tokenStr, secret)
	if err != nil {
		return nil, err
	}
	if claims.Type != want {
		return nil, fmt.Errorf("%w: got %q, want %q", ErrTokenType, claims.Type, want)
	}
	return claims, nil
}

// GenerateTokenPair creates an access token (24h) and a refresh token (7d)
// for the given user.
func GenerateTokenPair(user User, secret []byte) (*TokenPair, error) {
	now := time.Now().Unix()

	// Access token claims.
	accessClaims := Claims{
		Sub:      user.ID,
		Username: user.Username,
		Role:     user.Role,
		Type:     "access",
		Iat:      now,
		Exp:      now + int64(AccessTokenTTL.Seconds()),
	}

	accessToken, err := GenerateToken(accessClaims, secret)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	// Refresh token claims.
	refreshClaims := Claims{
		Sub:      user.ID,
		Username: user.Username,
		Role:     user.Role,
		Type:     "refresh",
		Iat:      now,
		Exp:      now + int64(RefreshTokenTTL.Seconds()),
	}

	refreshToken, err := GenerateToken(refreshClaims, secret)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(AccessTokenTTL.Seconds()),
	}, nil
}

// sign computes HMAC-SHA256(message, secret).
func sign(message, secret []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write(message)
	return mac.Sum(nil)
}
