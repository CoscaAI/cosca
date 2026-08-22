package auth

import (
	"testing"
	"time"
)

const testSecret = "this-is-a-32-byte-test-secret-123456"

// TestValidateAccessToken_RejectsRefresh ensures a refresh token is NOT
// accepted as an access token (Onda 2, vulnerability A4).
func TestValidateAccessToken_RejectsRefresh(t *testing.T) {
	refresh, err := GenerateToken(Claims{
		Sub:  "user-1",
		Type: "refresh",
		Exp:  time.Now().Add(time.Hour).Unix(),
	}, []byte(testSecret))
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}
	if _, err := ValidateAccessToken(refresh, []byte(testSecret)); err == nil {
		t.Error("ValidateAccessToken accepted a refresh token — must reject")
	}
}

// TestValidateAccessToken_AcceptsAccess ensures an access token passes.
func TestValidateAccessToken_AcceptsAccess(t *testing.T) {
	access, err := GenerateToken(Claims{
		Sub:  "user-1",
		Type: "access",
		Exp:  time.Now().Add(time.Hour).Unix(),
	}, []byte(testSecret))
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}
	if _, err := ValidateAccessToken(access, []byte(testSecret)); err != nil {
		t.Errorf("ValidateAccessToken rejected valid access token: %v", err)
	}
}

// TestValidateRefreshToken_RejectsAccess ensures an access token is NOT
// accepted as a refresh token.
func TestValidateRefreshToken_RejectsAccess(t *testing.T) {
	access, err := GenerateToken(Claims{
		Sub:  "user-1",
		Type: "access",
		Exp:  time.Now().Add(time.Hour).Unix(),
	}, []byte(testSecret))
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}
	if _, err := ValidateRefreshToken(access, []byte(testSecret)); err == nil {
		t.Error("ValidateRefreshToken accepted an access token — must reject")
	}
}
