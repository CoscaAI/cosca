package auth

import (
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════
// JWT Tests
// ═══════════════════════════════════════════════════════════════════════════

func TestGenerateAndValidateToken(t *testing.T) {
	secret := []byte("test-secret")

	claims := Claims{
		Sub:      "user-123",
		Username: "testuser",
		Role:     "editor",
		Type:     "access",
		Iat:      time.Now().Unix(),
		Exp:      time.Now().Add(24 * time.Hour).Unix(),
	}

	token, err := GenerateToken(claims, secret)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	if token == "" {
		t.Fatal("expected non-empty token")
	}

	parsed, err := ValidateToken(token, secret)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if parsed.Sub != claims.Sub {
		t.Errorf("expected sub %s, got %s", claims.Sub, parsed.Sub)
	}
	if parsed.Username != claims.Username {
		t.Errorf("expected username %s, got %s", claims.Username, parsed.Username)
	}
	if parsed.Role != claims.Role {
		t.Errorf("expected role %s, got %s", claims.Role, parsed.Role)
	}
}

func TestValidateTokenExpired(t *testing.T) {
	secret := []byte("test-secret")

	claims := Claims{
		Sub:      "user-123",
		Username: "testuser",
		Role:     "viewer",
		Iat:      time.Now().Add(-48 * time.Hour).Unix(),
		Exp:      time.Now().Add(-24 * time.Hour).Unix(),
	}

	token, err := GenerateToken(claims, secret)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	_, err = ValidateToken(token, secret)
	if err != ErrTokenExpired {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}
}

func TestValidateTokenWrongSecret(t *testing.T) {
	claims := Claims{
		Sub:      "user-123",
		Username: "testuser",
		Role:     "viewer",
		Iat:      time.Now().Unix(),
		Exp:      time.Now().Add(24 * time.Hour).Unix(),
	}

	token, err := GenerateToken(claims, []byte("secret-a"))
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	_, err = ValidateToken(token, []byte("secret-b"))
	if err != ErrInvalidSignature {
		t.Errorf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestGenerateTokenPair(t *testing.T) {
	secret := []byte("test-secret")
	user := User{
		ID:       "user-1",
		Username: "testuser",
		Role:     "admin",
	}

	pair, err := GenerateTokenPair(user, secret)
	if err != nil {
		t.Fatalf("GenerateTokenPair failed: %v", err)
	}

	if pair.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if pair.RefreshToken == "" {
		t.Error("expected non-empty refresh token")
	}
	if pair.ExpiresIn <= 0 {
		t.Errorf("expected positive expires_in, got %d", pair.ExpiresIn)
	}

	// Validate access token.
	accessClaims, err := ValidateToken(pair.AccessToken, secret)
	if err != nil {
		t.Fatalf("access token validation failed: %v", err)
	}
	if accessClaims.Type != "access" {
		t.Errorf("expected type 'access', got '%s'", accessClaims.Type)
	}

	// Validate refresh token.
	refreshClaims, err := ValidateToken(pair.RefreshToken, secret)
	if err != nil {
		t.Fatalf("refresh token validation failed: %v", err)
	}
	if refreshClaims.Type != "refresh" {
		t.Errorf("expected type 'refresh', got '%s'", refreshClaims.Type)
	}
}

func TestGenerateTokenPairTTL(t *testing.T) {
	// Verify access token TTL is 24h and refresh token TTL is 7d.
	secret := []byte("test-ttl-secret")
	user := User{
		ID:       "ttl-user",
		Username: "ttluser",
		Role:     "viewer",
	}

	pair, err := GenerateTokenPair(user, secret)
	if err != nil {
		t.Fatalf("GenerateTokenPair failed: %v", err)
	}

	if pair.ExpiresIn != int64(AccessTokenTTL.Seconds()) {
		t.Errorf("expected expires_in %d, got %d", int64(AccessTokenTTL.Seconds()), pair.ExpiresIn)
	}

	// Validate access token expiry window.
	accessClaims, err := ValidateToken(pair.AccessToken, secret)
	if err != nil {
		t.Fatalf("access token validation failed: %v", err)
	}

	expectedExp := accessClaims.Iat + int64(AccessTokenTTL.Seconds())
	if accessClaims.Exp != expectedExp {
		t.Errorf("expected access token exp %d, got %d", expectedExp, accessClaims.Exp)
	}

	// Validate refresh token expiry window.
	refreshClaims, err := ValidateToken(pair.RefreshToken, secret)
	if err != nil {
		t.Fatalf("refresh token validation failed: %v", err)
	}
	expectedRefreshExp := refreshClaims.Iat + int64(RefreshTokenTTL.Seconds())
	if refreshClaims.Exp != expectedRefreshExp {
		t.Errorf("expected refresh token exp %d, got %d", expectedRefreshExp, refreshClaims.Exp)
	}
}

// TestValidateTokenNoExpiry ensures that tokens with Exp=0 are rejected.
// This is a security measure — a token without expiration must not be accepted.
func TestValidateTokenNoExpiry(t *testing.T) {
	secret := []byte("test-secret")

	claims := Claims{
		Sub:      "user-no-exp",
		Username: "noexp",
		Role:     "viewer",
		Iat:      time.Now().Unix(),
		Exp:      0, // No expiration — should be rejected.
	}

	token, err := GenerateToken(claims, secret)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	_, err = ValidateToken(token, secret)
	if err != ErrTokenExpired {
		t.Errorf("expected ErrTokenExpired for Exp=0, got %v", err)
	}
}

// TestValidateTokenNegativeExpiry ensures tokens with negative Exp are rejected.
func TestValidateTokenNegativeExpiry(t *testing.T) {
	secret := []byte("test-secret")

	claims := Claims{
		Sub:      "user-neg-exp",
		Username: "negexp",
		Role:     "viewer",
		Iat:      time.Now().Unix(),
		Exp:      -1, // Negative expiration — should be rejected.
	}

	token, err := GenerateToken(claims, secret)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	_, err = ValidateToken(token, secret)
	if err != ErrTokenExpired {
		t.Errorf("expected ErrTokenExpired for negative Exp, got %v", err)
	}
}

func TestValidateTokenMalformed(t *testing.T) {
	secret := []byte("test-secret")

	tests := []struct {
		name  string
		token string
	}{
		{"empty token", ""},
		{"no dots", "justastring"},
		{"one dot", "header.payload"},
		{"four parts", "a.b.c.d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateToken(tt.token, secret)
			if err != ErrInvalidToken {
				t.Errorf("expected ErrInvalidToken, got %v", err)
			}
		})
	}
}

func TestValidateTokenBadBase64(t *testing.T) {
	secret := []byte("test-secret")

	// Token where the signature part is not valid base64.
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyIn0.not!!valid!!base64"

	_, err := ValidateToken(token, secret)
	if err == nil {
		t.Fatal("expected error for bad base64 signature")
	}
	// Should wrap ErrInvalidToken.
	if !strings.Contains(err.Error(), ErrInvalidToken.Error()) {
		t.Errorf("expected error to contain '%s', got '%v'", ErrInvalidToken.Error(), err)
	}
}

func TestValidateTokenInvalidJSON(t *testing.T) {
	// Craft a token with valid base64 but invalid JSON claims.
	// The claims part needs to be valid base64 but invalid JSON.
	//
	// We build a real token with a known secret, then replace the claims
	// payload with base64 encoding of malformed JSON.
	secret := []byte("test-secret")

	// Generate a real header.
	headerJSON, _ := json.Marshal(jwtHeader)
	headerEnc := encodeSegment(headerJSON)

	// Claims part: valid base64 but invalid JSON.
	invalidClaimsEnc := encodeSegment([]byte("{not json"))

	// Compute a real signature for the (header.invalidClaims) input.
	signingInput := headerEnc + "." + invalidClaimsEnc
	sig := encodeSegment(sign([]byte(signingInput), secret))

	token := signingInput + "." + sig

	_, err := ValidateToken(token, secret)
	if err == nil {
		t.Fatal("expected error for invalid JSON claims")
	}
	if !strings.Contains(err.Error(), ErrInvalidToken.Error()) {
		t.Errorf("expected error to contain '%s', got '%v'", ErrInvalidToken.Error(), err)
	}
}

func TestValidateTokenBadBase64Claims(t *testing.T) {
	secret := []byte("test-secret")

	// Valid header, bad base64 claims, real signature.
	headerJSON, _ := json.Marshal(jwtHeader)
	headerEnc := encodeSegment(headerJSON)

	// Not valid base64url.
	badClaimsEnc := "not!!valid!!base64!!"

	signingInput := headerEnc + "." + badClaimsEnc
	sig := encodeSegment(sign([]byte(signingInput), secret))

	token := signingInput + "." + sig

	_, err := ValidateToken(token, secret)
	if err == nil {
		t.Fatal("expected error for bad base64 claims")
	}
}

func TestValidateTokenHeaderNotDecoded(t *testing.T) {
	// ValidateToken does NOT decode or validate the JWT header.
	// The header is only used for signature verification.
	// This is acceptable because HMAC prevents header tampering.
	// But it does mean: if you build a token with a garbage header and
	// a known secret, it will still validate as long as the signature matches.

	secret := []byte("test-secret")
	claims := Claims{
		Sub: "user", Username: "u", Role: "viewer",
		Iat: time.Now().Unix(), Exp: time.Now().Add(time.Hour).Unix(),
	}
	claimsJSON, _ := json.Marshal(claims)
	claimsEnc := encodeSegment(claimsJSON)

	// Use a garbage header that is still a single string (no dots).
	garbageHeader := "not-valid-base64-but-does-not-matter"
	signingInput := garbageHeader + "." + claimsEnc
	sig := encodeSegment(sign([]byte(signingInput), secret))
	token := signingInput + "." + sig

	// The token validates because the signature matches the (garbage header + claims).
	_, err := ValidateToken(token, secret)
	if err != nil {
		t.Errorf("token with garbage header validates (header not decoded): %v", err)
	}

	// BUT if the signature is computed with a DIFFERENT header, it fails.
	signingInputFake := "fake-header" + "." + claimsEnc
	sigFake := encodeSegment(sign([]byte(signingInputFake), secret))
	tokenFake := garbageHeader + "." + claimsEnc + "." + sigFake
	_, err = ValidateToken(tokenFake, secret)
	if err != ErrInvalidSignature {
		t.Errorf("expected ErrInvalidSignature for mismatched header in signature, got %v", err)
	}
}

func TestGenerateTokenDifferentSecrets(t *testing.T) {
	claims := Claims{
		Sub: "user", Username: "u", Role: "viewer",
		Iat: time.Now().Unix(), Exp: time.Now().Add(time.Hour).Unix(),
	}

	t1, err := GenerateToken(claims, []byte("secret-1"))
	if err != nil {
		t.Fatalf("GenerateToken with secret-1 failed: %v", err)
	}
	t2, err := GenerateToken(claims, []byte("secret-2"))
	if err != nil {
		t.Fatalf("GenerateToken with secret-2 failed: %v", err)
	}

	// Different secrets should produce different signatures.
	if t1 == t2 {
		t.Error("tokens with different secrets should not be identical")
	}

	// t2 should not validate with secret-1.
	_, err = ValidateToken(t2, []byte("secret-1"))
	if err != ErrInvalidSignature {
		t.Errorf("expected ErrInvalidSignature validating t2 with secret-1, got %v", err)
	}
}

func TestValidateTokenJustExpired(t *testing.T) {
	secret := []byte("test-secret")

	// Token that expired 1 second ago.
	claims := Claims{
		Sub: "u", Username: "u", Role: "viewer",
		Iat: time.Now().Add(-2 * time.Hour).Unix(),
		Exp: time.Now().Add(-1 * time.Second).Unix(),
	}

	token, err := GenerateToken(claims, secret)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	_, err = ValidateToken(token, secret)
	if err != ErrTokenExpired {
		t.Errorf("expected ErrTokenExpired for just-expired token, got %v", err)
	}
}

func TestValidateTokenJustValid(t *testing.T) {
	secret := []byte("test-secret")

	// Token that expires in 1 minute — should still be valid.
	claims := Claims{
		Sub: "u", Username: "u", Role: "viewer",
		Iat: time.Now().Add(-24 * time.Hour).Unix(),
		Exp: time.Now().Add(1 * time.Minute).Unix(),
	}

	token, err := GenerateToken(claims, secret)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	_, err = ValidateToken(token, secret)
	if err != nil {
		t.Errorf("expected valid token, got error: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// API Key Store Tests
// ═══════════════════════════════════════════════════════════════════════════

func TestGenerateSecret(t *testing.T) {
	// generateSecret returns a hex string of 2*byteLen chars.
	s1, err := generateSecret(32)
	if err != nil {
		t.Fatalf("generateSecret failed: %v", err)
	}
	if len(s1) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(s1))
	}

	// Two calls should produce different secrets (cryptographic randomness).
	s2, err := generateSecret(32)
	if err != nil {
		t.Fatalf("second generateSecret failed: %v", err)
	}
	if s1 == s2 {
		t.Error("two generateSecret calls should produce different values")
	}

	// Test with different byte length.
	s3, err := generateSecret(16)
	if err != nil {
		t.Fatalf("generateSecret(16) failed: %v", err)
	}
	if len(s3) != 32 {
		t.Errorf("expected 32 hex chars for 16 bytes, got %d", len(s3))
	}
}

func TestHashKey(t *testing.T) {
	// hashKey is deterministic.
	h1 := hashKey("cosca_sk_abc123")
	h2 := hashKey("cosca_sk_abc123")
	if h1 != h2 {
		t.Errorf("hashKey should be deterministic: %s != %s", h1, h2)
	}

	// Different inputs produce different hashes.
	h3 := hashKey("cosca_sk_different")
	if h1 == h3 {
		t.Error("different keys should produce different hashes")
	}

	// Verify output is hex-encoded SHA-256 (64 chars).
	if len(h1) != 64 {
		t.Errorf("expected 64 hex chars (SHA-256), got %d", len(h1))
	}
}

func TestNewAPIKeyStoreInMemory(t *testing.T) {
	store := NewAPIKeyStore(APIKeyStoreConfig{})
	if store == nil {
		t.Fatal("expected non-nil store")
	}

	keys := store.List()
	if len(keys) != 0 {
		t.Errorf("expected empty store, got %d keys", len(keys))
	}
}

func TestNewAPIKeyStoreWithDataDir(t *testing.T) {
	dir := t.TempDir()
	store := NewAPIKeyStore(APIKeyStoreConfig{DataDir: dir})
	if store == nil {
		t.Fatal("expected non-nil store")
	}

	keys := store.List()
	if len(keys) != 0 {
		t.Errorf("expected empty store from new data dir, got %d keys", len(keys))
	}
}

func TestAPIKeyGenerate(t *testing.T) {
	store := NewAPIKeyStore(APIKeyStoreConfig{})

	ak, fullKey, err := store.Generate("Test Key", "creator", "admin", 0)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify returned APIKey fields.
	if ak.ID == "" {
		t.Error("expected non-empty ID")
	}
	if ak.Name != "Test Key" {
		t.Errorf("expected name 'Test Key', got '%s'", ak.Name)
	}
	if ak.Role != "admin" {
		t.Errorf("expected role 'admin', got '%s'", ak.Role)
	}
	if ak.Status != "active" {
		t.Errorf("expected status 'active', got '%s'", ak.Status)
	}
	if ak.Prefix == "" {
		t.Error("expected non-empty prefix")
	}
	if !strings.HasPrefix(ak.Prefix, "cosca_") {
		t.Errorf("expected prefix to start with 'cosca_', got '%s'", ak.Prefix)
	}
	if strings.HasSuffix(ak.Prefix, "...") {
		// Prefix should end with "..." since key is >12 chars.
	} else {
		t.Error("expected prefix to end with '...'")
	}
	if ak.Hash == "" {
		t.Error("expected non-empty hash")
	}
	if ak.ExpiresAt != "" {
		t.Errorf("expected empty ExpiresAt for no-expiry key, got '%s'", ak.ExpiresAt)
	}

	// Verify the full key format.
	if !strings.HasPrefix(fullKey, "cosca_sk_") {
		t.Errorf("expected full key to start with 'cosca_sk_', got '%s'", fullKey)
	}
	if len(fullKey) < 20 {
		t.Errorf("expected full key to be reasonably long, got %d chars", len(fullKey))
	}

	// Verify the key validates.
	validated, err := store.Validate(fullKey)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if validated.ID != ak.ID {
		t.Errorf("validated key ID mismatch: %s != %s", validated.ID, ak.ID)
	}
}

func TestAPIKeyGenerateWithExpiry(t *testing.T) {
	store := NewAPIKeyStore(APIKeyStoreConfig{})

	expiresIn := 24 * time.Hour
	ak, fullKey, err := store.Generate("Expiring Key", "creator", "editor", expiresIn)
	if err != nil {
		t.Fatalf("Generate with expiry failed: %v", err)
	}

	if ak.ExpiresAt == "" {
		t.Error("expected non-empty ExpiresAt for expiring key")
	}

	// Parse the expiration and verify it's within expected range.
	exp, err := time.Parse(time.RFC3339, ak.ExpiresAt)
	if err != nil {
		t.Fatalf("failed to parse ExpiresAt: %v", err)
	}

	expectedMin := time.Now().UTC().Add(expiresIn - time.Minute)
	expectedMax := time.Now().UTC().Add(expiresIn + time.Minute)
	if exp.Before(expectedMin) || exp.After(expectedMax) {
		t.Errorf("ExpiresAt %s not within expected range [%s, %s]",
			exp.Format(time.RFC3339),
			expectedMin.Format(time.RFC3339),
			expectedMax.Format(time.RFC3339))
	}

	// The key should validate now (not yet expired).
	_, err = store.Validate(fullKey)
	if err != nil {
		t.Fatalf("non-expired key should validate: %v", err)
	}
}

func TestAPIKeyValidateInvalidPrefix(t *testing.T) {
	store := NewAPIKeyStore(APIKeyStoreConfig{})

	// Keys without the correct prefix should be rejected.
	_, err := store.Validate("random-key-without-prefix")
	if err != ErrAPIKeyInvalid {
		t.Errorf("expected ErrAPIKeyInvalid, got %v", err)
	}

	_, err = store.Validate("sk_cosca_test123")
	if err != ErrAPIKeyInvalid {
		t.Errorf("expected ErrAPIKeyInvalid, got %v", err)
	}
}

func TestAPIKeyValidateNotFound(t *testing.T) {
	store := NewAPIKeyStore(APIKeyStoreConfig{})

	// A well-formed but unknown key should return invalid.
	_, err := store.Validate("cosca_sk_" + strings.Repeat("a", 64))
	if err != ErrAPIKeyInvalid {
		t.Errorf("expected ErrAPIKeyInvalid for unknown key, got %v", err)
	}
}

func TestAPIKeyValidateExpired(t *testing.T) {
	store := NewAPIKeyStore(APIKeyStoreConfig{})

	// We need to manually insert an expired key since Generate always creates
	// fresh keys. We can use the internal map directly.
	secret := "cosca_sk_" + strings.Repeat("e", 64)
	keyHash := hashKey(secret)

	pastExpiry := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	expiredKey := &APIKey{
		ID:        "expired-1",
		Name:      "Expired",
		Prefix:    "cosca_sk_ee...",
		Hash:      keyHash,
		Role:      "viewer",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		ExpiresAt: pastExpiry,
		Status:    "active",
		CreatedBy: "test",
	}

	store.mu.Lock()
	store.keys[expiredKey.ID] = expiredKey
	store.byPrefix[expiredKey.Prefix] = expiredKey
	store.byHash[expiredKey.Hash] = expiredKey
	store.mu.Unlock()

	_, err := store.Validate(secret)
	if err != ErrAPIKeyExpired {
		t.Errorf("expected ErrAPIKeyExpired, got %v", err)
	}
}

func TestAPIKeyValidateRevoked(t *testing.T) {
	store := NewAPIKeyStore(APIKeyStoreConfig{})

	// Create and then revoke a key.
	_, fullKey, err := store.Generate("To Revoke", "creator", "viewer", 0)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Find the key ID from the store.
	keys := store.List()
	if len(keys) == 0 {
		t.Fatal("expected at least 1 key")
	}

	err = store.Revoke(keys[0].ID)
	if err != nil {
		t.Fatalf("Revoke failed: %v", err)
	}

	// The revoked key should fail validation.
	_, err = store.Validate(fullKey)
	if err != ErrAPIKeyRevoked {
		t.Errorf("expected ErrAPIKeyRevoked, got %v", err)
	}
}

func TestAPIKeyValidateUpdatesLastUsed(t *testing.T) {
	store := NewAPIKeyStore(APIKeyStoreConfig{})

	_, fullKey, err := store.Generate("LastUsed Key", "creator", "admin", 0)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Validate the key — this should update LastUsedAt.
	before := time.Now().UTC()
	ak, err := store.Validate(fullKey)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if ak.LastUsedAt == "" {
		t.Error("expected non-empty LastUsedAt after validation")
	}

	lastUsed, err := time.Parse(time.RFC3339, ak.LastUsedAt)
	if err != nil {
		t.Fatalf("failed to parse LastUsedAt: %v", err)
	}

	if lastUsed.Before(before.Add(-time.Second)) {
		t.Errorf("LastUsedAt %s should not be before %s",
			ak.LastUsedAt, before.Format(time.RFC3339))
	}
}

func TestAPIKeyList(t *testing.T) {
	store := NewAPIKeyStore(APIKeyStoreConfig{})

	// Empty store.
	keys := store.List()
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}

	// Generate some keys.
	_, _, _ = store.Generate("Key One", "creator", "admin", 0)
	_, _, _ = store.Generate("Key Two", "creator", "editor", 0)
	_, _, _ = store.Generate("Key Three", "creator", "viewer", 24*time.Hour)

	keys = store.List()
	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}

	// Verify hashes are stripped.
	for _, k := range keys {
		if k.Hash != "" {
			t.Errorf("key %s has exposed hash", k.Name)
		}
		if k.ID == "" {
			t.Error("key has empty ID")
		}
	}
}

func TestAPIKeyGetByID(t *testing.T) {
	store := NewAPIKeyStore(APIKeyStoreConfig{})

	// Not found.
	_, err := store.GetByID("nonexistent")
	if err != ErrAPIKeyNotFound {
		t.Errorf("expected ErrAPIKeyNotFound, got %v", err)
	}

	// Create a key and find it.
	ak, _, err := store.Generate("GetByID Key", "creator", "admin", 0)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	found, err := store.GetByID(ak.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if found.ID != ak.ID {
		t.Errorf("ID mismatch: %s != %s", found.ID, ak.ID)
	}
	if found.Name != "GetByID Key" {
		t.Errorf("name mismatch: %s", found.Name)
	}
	// Hash must be stripped.
	if found.Hash != "" {
		t.Error("GetByID should not expose hash")
	}
}

func TestAPIKeyRevoke(t *testing.T) {
	store := NewAPIKeyStore(APIKeyStoreConfig{})

	// Revoke non-existent key.
	err := store.Revoke("nonexistent")
	if err != ErrAPIKeyNotFound {
		t.Errorf("expected ErrAPIKeyNotFound, got %v", err)
	}

	// Create and revoke.
	ak, fullKey, err := store.Generate("RevokeMe", "creator", "viewer", 0)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	err = store.Revoke(ak.ID)
	if err != nil {
		t.Fatalf("Revoke failed: %v", err)
	}

	// Verify status changed.
	found, err := store.GetByID(ak.ID)
	if err != nil {
		t.Fatalf("revoked key should still be retrievable: %v", err)
	}
	if found.Status != "revoked" {
		t.Errorf("expected status 'revoked', got '%s'", found.Status)
	}

	// Verify validation fails.
	_, err = store.Validate(fullKey)
	if err != ErrAPIKeyRevoked {
		t.Errorf("expected ErrAPIKeyRevoked, got %v", err)
	}
}

func TestAPIKeyListAfterRevoke(t *testing.T) {
	// Revoked keys should still appear in List (audit trail).
	store := NewAPIKeyStore(APIKeyStoreConfig{})

	ak, _, _ := store.Generate("Audit Key", "creator", "admin", 0)
	_ = store.Revoke(ak.ID)

	keys := store.List()
	found := false
	for _, k := range keys {
		if k.ID == ak.ID {
			found = true
			if k.Status != "revoked" {
				t.Errorf("expected status 'revoked' in list, got '%s'", k.Status)
			}
			break
		}
	}
	if !found {
		t.Error("revoked key should appear in List")
	}
}

func TestAPIKeyStorePersistence(t *testing.T) {
	dir := t.TempDir()

	// Create store, generate keys.
	store1 := NewAPIKeyStore(APIKeyStoreConfig{DataDir: dir})
	_, _, err := store1.Generate("Persist Key 1", "creator", "admin", 0)
	if err != nil {
		t.Fatalf("Generate 1 failed: %v", err)
	}
	_, _, err = store1.Generate("Persist Key 2", "creator", "editor", 1*time.Hour)
	if err != nil {
		t.Fatalf("Generate 2 failed: %v", err)
	}

	// Verify the file exists.
	filePath := filepath.Join(dir, "api-keys.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("api-keys.json should exist after generating keys")
	}

	// Create a new store from the same dir — keys should be reloaded.
	store2 := NewAPIKeyStore(APIKeyStoreConfig{DataDir: dir})
	keys := store2.List()
	if len(keys) != 2 {
		t.Errorf("expected 2 keys after reload, got %d", len(keys))
	}

	// Verify the keys are valid (hashes loaded correctly).
	for _, k := range keys {
		if k.ID == "" {
			t.Error("reloaded key has empty ID")
		}
		if k.Name == "" {
			t.Error("reloaded key has empty name")
		}
	}
}

func TestAPIKeyStoreLoadCorruptJSON(t *testing.T) {
	dir := t.TempDir()

	// Write corrupt JSON to api-keys.json.
	filePath := filepath.Join(dir, "api-keys.json")
	if err := os.WriteFile(filePath, []byte("this is not valid json {{{["), 0644); err != nil {
		t.Fatalf("failed to write corrupt file: %v", err)
	}

	// Creating a store should log an error but not crash.
	store := NewAPIKeyStore(APIKeyStoreConfig{DataDir: dir})
	if store == nil {
		t.Fatal("store should not be nil even with corrupt file")
	}

	// Store should be empty since JSON was unparseable.
	keys := store.List()
	if len(keys) != 0 {
		t.Errorf("expected 0 keys from corrupt file, got %d", len(keys))
	}
}

func TestAPIKeyStorePersistenceAtomicWrite(t *testing.T) {
	dir := t.TempDir()

	store := NewAPIKeyStore(APIKeyStoreConfig{DataDir: dir})
	_, _, err := store.Generate("Atomic Key", "creator", "admin", 0)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify no .tmp file remains (clean atomic write).
	tmpPath := filepath.Join(dir, "api-keys.json.tmp")
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temporary file should not exist after successful write")
	}

	// Verify the real file is valid JSON.
	filePath := filepath.Join(dir, "api-keys.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read api-keys.json: %v", err)
	}

	var keys []map[string]interface{}
	if err := json.Unmarshal(data, &keys); err != nil {
		t.Fatalf("api-keys.json is not valid JSON: %v", err)
	}

	if len(keys) != 1 {
		t.Errorf("expected 1 key in file, got %d", len(keys))
	}
}

func TestAPIKeyValidateInvalidExpiresAt(t *testing.T) {
	store := NewAPIKeyStore(APIKeyStoreConfig{})

	// Insert a key with an invalid ExpiresAt format.
	secret := "cosca_sk_" + strings.Repeat("f", 64)
	keyHash := hashKey(secret)

	badKey := &APIKey{
		ID:        "bad-expiry-1",
		Name:      "Bad Expiry",
		Prefix:    "cosca_sk_ff...",
		Hash:      keyHash,
		Role:      "viewer",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		ExpiresAt: "not-a-valid-date",
		Status:    "active",
		CreatedBy: "test",
	}

	store.mu.Lock()
	store.keys[badKey.ID] = badKey
	store.byPrefix[badKey.Prefix] = badKey
	store.byHash[badKey.Hash] = badKey
	store.mu.Unlock()

	// Fail-closed: an invalid/corrupted ExpiresAt must NOT extend validity.
	// Corruption never grants privilege — the key is treated as expired.
	_, err := store.Validate(secret)
	if err != ErrAPIKeyExpired {
		t.Errorf("expected ErrAPIKeyExpired for key with invalid expires_at, got %v", err)
	}
}

func TestAPIKeyGenerateMultiple(t *testing.T) {
	store := NewAPIKeyStore(APIKeyStoreConfig{})

	// Generate many keys to stress-test prefix collision handling.
	for i := 0; i < 20; i++ {
		_, _, err := store.Generate("Key_"+string(rune('A'+i%26)), "creator", "viewer", 0)
		if err != nil {
			t.Fatalf("Generate %d failed: %v", i, err)
		}
	}

	keys := store.List()
	if len(keys) < 20 {
		t.Errorf("expected at least 20 keys, got %d", len(keys))
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// User Store Tests (extended)
// ═══════════════════════════════════════════════════════════════════════════

func TestUserStoreCreateAndAuthenticate(t *testing.T) {
	store := NewUserStore(UserStoreConfig{})

	// Create a test user.
	user, err := store.Create("testuser", "password123", "editor", "test@example.com")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if user.ID == "" {
		t.Error("expected non-empty user ID")
	}
	if user.Username != "testuser" {
		t.Errorf("expected username 'testuser', got '%s'", user.Username)
	}
	if user.Role != "editor" {
		t.Errorf("expected role 'editor', got '%s'", user.Role)
	}

	// Authenticate with correct password.
	authUser, err := store.Authenticate("testuser", "password123")
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if authUser.ID != user.ID {
		t.Error("authenticated user ID mismatch")
	}

	// Authenticate with wrong password.
	_, err = store.Authenticate("testuser", "wrongpassword")
	if err != ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestUserStoreDuplicate(t *testing.T) {
	store := NewUserStore(UserStoreConfig{})

	_, err := store.Create("duplicate", "pass", "viewer", "")
	if err != nil {
		t.Fatalf("first Create failed: %v", err)
	}

	_, err = store.Create("duplicate", "pass", "viewer", "")
	if err != ErrUserExists {
		t.Errorf("expected ErrUserExists, got %v", err)
	}
}

func TestUserStoreInvalidRole(t *testing.T) {
	store := NewUserStore(UserStoreConfig{})

	_, err := store.Create("badrole", "pass", "superadmin", "")
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
}

func TestUserStoreDefaultAdmin(t *testing.T) {
	// Default admin is only auto-created when COSCA_DEV_MODE=true.
	t.Setenv("COSCA_DEV_MODE", "true")

	var store *UserStore
	output := captureStderr(t, func() {
		store = NewUserStore(UserStoreConfig{})
	})

	// The default admin exists with the admin role.
	user, err := store.GetByUsername("admin")
	if err != nil {
		t.Fatalf("default admin not found: %v", err)
	}
	if user.Role != "admin" {
		t.Errorf("expected admin role, got '%s'", user.Role)
	}

	// The provisioned credential is random (printed once to stderr), never
	// the hardcoded "admin".
	if password := extractPrintedAdminPassword(output); password == "admin" {
		t.Error("dev admin password must not be the hardcoded 'admin'")
	}

	// M5: the auto-provisioned admin must be forced to change its password.
	if !user.MustChangePassword {
		t.Error("expected MustChangePassword=true for the dev-mode admin")
	}

	// The old default credential admin/admin must NOT work anymore.
	if _, err := store.Authenticate("admin", "admin"); err == nil {
		t.Error("dev admin must not authenticate with the hardcoded 'admin' password")
	}
}

// captureStderr runs fn with os.Stderr redirected to a pipe and returns the
// captured output. Only safe in non-parallel tests (the dev-mode bootstrap
// tests do not call t.Parallel).
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	fn()

	_ = w.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	_ = r.Close()
	return string(data)
}

// extractPrintedAdminPassword parses the random dev-admin password printed by
// NewUserStore from a captured stderr dump. Returns "" when not found.
func extractPrintedAdminPassword(output string) string {
	const marker = "random password: "
	idx := strings.Index(output, marker)
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(output[idx+len(marker):])
	if sp := strings.Index(rest, " "); sp >= 0 {
		rest = rest[:sp]
	}
	return rest
}

// TestDevAdmin_RandomPassword verifies the dev-mode admin bootstrap uses a
// cryptographically random password (never a default/hardcoded one) and flags
// the account for an immediate password change (M5).
func TestDevAdmin_RandomPassword(t *testing.T) {
	t.Setenv("COSCA_DEV_MODE", "true")

	var store *UserStore
	output := captureStderr(t, func() {
		store = NewUserStore(UserStoreConfig{})
	})
	password := extractPrintedAdminPassword(output)

	if password == "" {
		t.Fatal("expected a random admin password to be generated and printed once")
	}
	if password == "admin" {
		t.Error("dev admin password must not be the hardcoded 'admin'")
	}

	user, err := store.GetByUsername("admin")
	if err != nil {
		t.Fatalf("dev admin missing: %v", err)
	}
	if !user.MustChangePassword {
		t.Error("dev admin must be flagged for password change (MustChangePassword=true)")
	}

	// The printed credential is the only working one, and the flag survives
	// authentication so the login endpoint still forces the change flow.
	authUser, err := store.Authenticate("admin", password)
	if err != nil {
		t.Fatalf("generated admin password must authenticate: %v", err)
	}
	if !authUser.MustChangePassword {
		t.Error("MustChangePassword must remain true after dev-admin login")
	}
}

func TestUserStoreList(t *testing.T) {
	// Default admin is only auto-created when COSCA_DEV_MODE=true.
	t.Setenv("COSCA_DEV_MODE", "true")

	store := NewUserStore(UserStoreConfig{})

	// Default admin + 2 new users = 3.
	_, _ = store.Create("user1", "pass1", "editor", "")
	_, _ = store.Create("user2", "pass2", "viewer", "")

	users := store.List()
	if len(users) < 3 {
		t.Errorf("expected at least 3 users, got %d", len(users))
	}

	// Verify password hashes are not exposed.
	for _, u := range users {
		if u.PasswordHash != "" {
			t.Errorf("user %s has exposed password hash", u.Username)
		}
	}
}

func TestUserStoreDelete(t *testing.T) {
	store := NewUserStore(UserStoreConfig{})

	user, _ := store.Create("todelete", "pass", "viewer", "")

	err := store.Delete(user.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.GetByID(user.ID)
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound after delete, got %v", err)
	}
}

func TestUserStoreUpdateRole(t *testing.T) {
	store := NewUserStore(UserStoreConfig{})

	user, _ := store.Create("rolechange", "pass", "viewer", "")

	err := store.UpdateRole(user.ID, "editor")
	if err != nil {
		t.Fatalf("UpdateRole failed: %v", err)
	}

	updated, _ := store.GetByID(user.ID)
	if updated.Role != "editor" {
		t.Errorf("expected role 'editor', got '%s'", updated.Role)
	}
}

// ─── New User Store tests ─────────────────────────────────────────────────

func TestUserStoreGetByUsername(t *testing.T) {
	store := NewUserStore(UserStoreConfig{})

	user, err := store.Create("byusername", "pass", "editor", "byusername@example.com")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Found by username.
	found, err := store.GetByUsername("byusername")
	if err != nil {
		t.Fatalf("GetByUsername failed: %v", err)
	}
	if found.ID != user.ID {
		t.Errorf("ID mismatch: %s != %s", found.ID, user.ID)
	}

	// Not found.
	_, err = store.GetByUsername("nonexistentuser")
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserStoreGetByIDNotFound(t *testing.T) {
	store := NewUserStore(UserStoreConfig{})

	_, err := store.GetByID("nonexistent-id")
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserStoreGetByIDReturnsCopy(t *testing.T) {
	store := NewUserStore(UserStoreConfig{})
	user, err := store.Create("copyid", "password123", "editor", "copy@example.com")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	storedHash := user.PasswordHash
	if storedHash == "" {
		t.Fatal("expected a non-empty hash after Create")
	}

	got, err := store.GetByID(user.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	got.PasswordHash = "MUTATED"

	again, err := store.GetByID(user.ID)
	if err != nil {
		t.Fatalf("GetByID after mutation failed: %v", err)
	}
	if again.PasswordHash != storedHash {
		t.Errorf("GetByID returned the internal pointer: store hash changed from %q to %q", storedHash, again.PasswordHash)
	}
}

func TestUserStoreGetByUsernameReturnsCopy(t *testing.T) {
	store := NewUserStore(UserStoreConfig{})
	user, err := store.Create("copyusername", "password123", "viewer", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	storedHash := user.PasswordHash
	if storedHash == "" {
		t.Fatal("expected a non-empty hash after Create")
	}

	got, err := store.GetByUsername(user.Username)
	if err != nil {
		t.Fatalf("GetByUsername failed: %v", err)
	}
	got.PasswordHash = "MUTATED"

	again, err := store.GetByUsername(user.Username)
	if err != nil {
		t.Fatalf("GetByUsername after mutation failed: %v", err)
	}
	if again.PasswordHash != storedHash {
		t.Errorf("GetByUsername returned the internal pointer: store hash changed from %q to %q", storedHash, again.PasswordHash)
	}
}

func TestUserStoreDeleteNotFound(t *testing.T) {
	store := NewUserStore(UserStoreConfig{})

	err := store.Delete("nonexistent-id")
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserStoreUpdateRoleNotFound(t *testing.T) {
	store := NewUserStore(UserStoreConfig{})

	err := store.UpdateRole("nonexistent-id", "editor")
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserStoreUpdateRoleInvalid(t *testing.T) {
	store := NewUserStore(UserStoreConfig{})

	user, _ := store.Create("validuser", "pass", "viewer", "")

	// Invalid role for existing user.
	err := store.UpdateRole(user.ID, "superadmin")
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
	if !strings.Contains(err.Error(), ErrInvalidRole.Error()) {
		t.Errorf("expected error to contain ErrInvalidRole, got %v", err)
	}

	// Also test invalid role on non-existent user.
	err = store.UpdateRole("nonexistent-id", "superadmin")
	if err == nil {
		t.Fatal("expected error for invalid role on non-existent user")
	}
}

func TestUserStoreAuthenticateAccountLock(t *testing.T) {
	store := NewUserStore(UserStoreConfig{})

	_, err := store.Create("lockuser", "password", "viewer", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Fail authentication 4 times (should NOT lock yet).
	for i := 0; i < 4; i++ {
		_, err = store.Authenticate("lockuser", "wrongpassword")
		if err != ErrInvalidCredentials {
			t.Fatalf("attempt %d: expected ErrInvalidCredentials, got %v", i+1, err)
		}
	}

	// After 4 failures, the user should still be able to log in with correct password.
	user, err := store.Authenticate("lockuser", "password")
	if err != nil {
		t.Fatalf("user should not be locked after 4 failures: %v", err)
	}
	if user.FailedAttempts != 0 {
		t.Errorf("FailedAttempts should be reset after successful login, got %d", user.FailedAttempts)
	}

	// Now fail 5 times consecutively to trigger lock.
	for i := 0; i < 5; i++ {
		_, err = store.Authenticate("lockuser", "wrongpassword")
		if err == nil {
			t.Fatal("expected error on failed auth")
		}
	}

	// Now even correct password should fail due to lock.
	_, err = store.Authenticate("lockuser", "password")
	if err == nil {
		t.Fatal("expected account locked error")
	}
	if !strings.Contains(err.Error(), "account locked") {
		t.Errorf("expected 'account locked' in error, got %v", err)
	}

	// Verify LockedUntil is set.
	lockedUser, err := store.GetByUsername("lockuser")
	if err != nil {
		t.Fatalf("GetByUsername failed: %v", err)
	}
	if lockedUser.LockedUntil == "" {
		t.Error("LockedUntil should be set after 5 failures")
	}
	if lockedUser.FailedAttempts < 5 {
		t.Errorf("FailedAttempts should be at least 5, got %d", lockedUser.FailedAttempts)
	}
}

func TestUserStoreAuthenticateLockExpiry(t *testing.T) {
	store := NewUserStore(UserStoreConfig{})

	_, err := store.Create("lockexp", "password", "viewer", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Manually set the user as locked with an expired lock time.
	store.mu.Lock()
	user := store.byUsername["lockexp"]
	user.LockedUntil = time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	user.FailedAttempts = 5
	store.mu.Unlock()

	// Authenticating with correct password should succeed (lock expired).
	authUser, err := store.Authenticate("lockexp", "password")
	if err != nil {
		t.Fatalf("expected successful auth after lock expiry: %v", err)
	}
	if authUser.LockedUntil != "" {
		t.Errorf("LockedUntil should be cleared after lock expiry, got '%s'", authUser.LockedUntil)
	}
	if authUser.FailedAttempts != 0 {
		t.Errorf("FailedAttempts should be 0 after lock expiry, got %d", authUser.FailedAttempts)
	}
}

func TestUserStoreAuthenticateCorruptedLockState(t *testing.T) {
	store := NewUserStore(UserStoreConfig{})

	_, err := store.Create("corruptlock", "password", "viewer", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Corrupt the lock state so time.Parse cannot read it.
	store.mu.Lock()
	user := store.byUsername["corruptlock"]
	user.LockedUntil = "not-a-valid-date"
	user.FailedAttempts = 5
	store.mu.Unlock()

	// Fail-closed: unreadable LockedUntil must be treated as "still locked",
	// and the brute-force counters must NOT be reset.
	_, err = store.Authenticate("corruptlock", "password")
	if err == nil {
		t.Fatal("expected error for unreadable lock state")
	}
	if !strings.Contains(err.Error(), "account locked") {
		t.Errorf("expected 'account locked' in error, got %v", err)
	}

	lockedUser, err := store.GetByUsername("corruptlock")
	if err != nil {
		t.Fatalf("GetByUsername failed: %v", err)
	}
	if lockedUser.LockedUntil == "" {
		t.Error("LockedUntil must NOT be cleared on corrupted lock state")
	}
	if lockedUser.FailedAttempts != 5 {
		t.Errorf("FailedAttempts must NOT be reset on corrupted lock state, got %d", lockedUser.FailedAttempts)
	}
}

func TestUserStoreAuthenticateNonexistentUser(t *testing.T) {
	store := NewUserStore(UserStoreConfig{})

	_, err := store.Authenticate("nonexistent", "password")
	if err != ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials for nonexistent user, got %v", err)
	}
}

func TestUserStoreNonDevMode(t *testing.T) {
	// In non-dev mode, no default admin should be created.
	// We need to clear COSCA_DEV_MODE (it might be set by the environment).
	t.Setenv("COSCA_DEV_MODE", "false")

	store := NewUserStore(UserStoreConfig{})

	// Store should be empty.
	users := store.List()
	if len(users) != 0 {
		t.Errorf("expected 0 users in non-dev mode, got %d", len(users))
	}
}

func TestUserStoreCreateWithEmptyEmail(t *testing.T) {
	store := NewUserStore(UserStoreConfig{})

	user, err := store.Create("noemail", "pass", "viewer", "")
	if err != nil {
		t.Fatalf("Create with empty email failed: %v", err)
	}
	if user.Email != "" {
		t.Errorf("expected empty email, got '%s'", user.Email)
	}
}

func TestUserStoreAuthenticateFailedAttemptsCounter(t *testing.T) {
	store := NewUserStore(UserStoreConfig{})

	_, err := store.Create("counter", "correct", "viewer", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Fail once.
	_, _ = store.Authenticate("counter", "wrong")

	user, err := store.GetByUsername("counter")
	if err != nil {
		t.Fatalf("GetByUsername failed: %v", err)
	}
	if user.FailedAttempts != 1 {
		t.Errorf("expected 1 failed attempt, got %d", user.FailedAttempts)
	}

	// Fail again.
	_, _ = store.Authenticate("counter", "wrong")

	user, err = store.GetByUsername("counter")
	if err != nil {
		t.Fatalf("GetByUsername failed: %v", err)
	}
	if user.FailedAttempts != 2 {
		t.Errorf("expected 2 failed attempts, got %d", user.FailedAttempts)
	}
}

func TestUserStorePersistence(t *testing.T) {
	// UserStoreConfig atualmente so suporta DB *sql.DB.
	// Com DB=nil, o store opera em memoria (dentro do mesmo processo).
	store1 := NewUserStore(UserStoreConfig{})
	user1, err := store1.Create("persistuser", "password", "editor", "persist@example.com")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verifica que o usuario foi criado corretamente.
	found, err := store1.GetByUsername("persistuser")
	if err != nil {
		t.Fatalf("GetByUsername failed: %v", err)
	}
	if found.ID != user1.ID {
		t.Errorf("ID mismatch: %s != %s", found.ID, user1.ID)
	}
	if found.Role != "editor" {
		t.Errorf("role mismatch: %s", found.Role)
	}

	// Autentica com as credenciais persistidas (em memoria).
	_, err = store1.Authenticate("persistuser", "password")
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
}

func TestUserStoreCreateEmptyStore(t *testing.T) {
	// Cria store vazio (sem DB) — deve funcionar em memoria.
	store := NewUserStore(UserStoreConfig{})
	if store == nil {
		t.Fatal("store should not be nil")
	}

	// Lista deve ser vazia.
	users := store.List()
	if len(users) != 0 {
		t.Errorf("expected empty store, got %d users", len(users))
	}
}

func TestUserStoreCreateInMemory(t *testing.T) {
	store := NewUserStore(UserStoreConfig{})
	user, err := store.Create("memuser", "pass", "viewer", "mem@example.com")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if user.ID == "" {
		t.Error("expected non-empty ID")
	}
	if user.Username != "memuser" {
		t.Errorf("expected username 'memuser', got '%s'", user.Username)
	}
	if user.Role != "viewer" {
		t.Errorf("expected role 'viewer', got '%s'", user.Role)
	}

	// Deve estar na lista.
	users := store.List()
	if len(users) != 1 {
		t.Errorf("expected 1 user, got %d", len(users))
	}
}

func TestUserStoreDeleteInMemory(t *testing.T) {
	store := NewUserStore(UserStoreConfig{})
	user, err := store.Create("todelete", "pass", "viewer", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	err = store.Delete(user.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Usuario deve ter sumido.
	_, err = store.GetByUsername("todelete")
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound after delete, got %v", err)
	}
}

func TestUserStoreUpdateRoleInMemory(t *testing.T) {
	store := NewUserStore(UserStoreConfig{})
	user, err := store.Create("rolepersist", "pass", "viewer", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	err = store.UpdateRole(user.ID, "admin")
	if err != nil {
		t.Fatalf("UpdateRole failed: %v", err)
	}

	// Verifica que o papel foi atualizado.
	found, err := store.GetByUsername("rolepersist")
	if err != nil {
		t.Fatalf("GetByUsername failed: %v", err)
	}
	if found.Role != "admin" {
		t.Errorf("expected role 'admin', got '%s'", found.Role)
	}
}

func TestUserStoreListIsCopy(t *testing.T) {
	// Verify that mutating the returned slice doesn't affect internal state.
	store := NewUserStore(UserStoreConfig{})
	_, _ = store.Create("listcopy", "pass", "viewer", "")

	users := store.List()
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}

	// Mutate the returned slice.
	users[0].Username = "hacked"
	users = append(users, User{ID: "injected", Username: "evil"})

	// Re-list — should be unaffected.
	users2 := store.List()
	if len(users2) != 1 {
		t.Errorf("expected still 1 user, got %d", len(users2))
	}
	found, _ := store.GetByUsername("listcopy")
	if found.Username != "listcopy" {
		t.Errorf("username should not have been mutated, got '%s'", found.Username)
	}
}

func TestUserStoreAuthenticateResetsFailedAttempts(t *testing.T) {
	store := NewUserStore(UserStoreConfig{})

	_, err := store.Create("resetuser", "correct", "viewer", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Fail 3 times.
	for i := 0; i < 3; i++ {
		_, _ = store.Authenticate("resetuser", "wrong")
	}

	user, _ := store.GetByUsername("resetuser")
	if user.FailedAttempts != 3 {
		t.Errorf("expected 3 failed attempts, got %d", user.FailedAttempts)
	}

	// Succeed — should reset counter.
	_, err = store.Authenticate("resetuser", "correct")
	if err != nil {
		t.Fatalf("Authenticate with correct password failed: %v", err)
	}

	user, _ = store.GetByUsername("resetuser")
	if user.FailedAttempts != 0 {
		t.Errorf("expected 0 failed attempts after success, got %d", user.FailedAttempts)
	}
	if user.LockedUntil != "" {
		t.Errorf("expected empty LockedUntil after success, got '%s'", user.LockedUntil)
	}
}

func TestHashPassword(t *testing.T) {
	// hashPassword should produce bcrypt hashes.
	hash, err := hashPassword("mypassword")
	if err != nil {
		t.Fatalf("hashPassword failed: %v", err)
	}

	if hash == "" {
		t.Error("expected non-empty hash")
	}
	if hash == "mypassword" {
		t.Error("hash should not be the plaintext password")
	}

	// Hashes should be different for the same password (bcrypt includes salt).
	hash2, _ := hashPassword("mypassword")
	if hash == hash2 {
		t.Error("two hashes of the same password should differ due to salt")
	}

	// Verify hash is bcrypt format (starts with $2a$).
	if !strings.HasPrefix(hash, "$2a$") {
		t.Errorf("expected bcrypt hash format ($2a$...), got '%s'", hash[:10])
	}
}

func TestUserStoreMultipleSessions(t *testing.T) {
	// Verify that concurrent users can be managed without race conditions.
	store := NewUserStore(UserStoreConfig{})

	_, _ = store.Create("user_a", "pass_a", "admin", "")
	_, _ = store.Create("user_b", "pass_b", "editor", "")
	_, _ = store.Create("user_c", "pass_c", "viewer", "")

	users := store.List()
	if len(users) != 3 {
		t.Errorf("expected 3 users, got %d", len(users))
	}

	// Each should authenticate independently.
	for _, tt := range []struct {
		username, password, role string
	}{
		{"user_a", "pass_a", "admin"},
		{"user_b", "pass_b", "editor"},
		{"user_c", "pass_c", "viewer"},
	} {
		au, err := store.Authenticate(tt.username, tt.password)
		if err != nil {
			t.Errorf("auth for %s failed: %v", tt.username, err)
			continue
		}
		if au.Role != tt.role {
			t.Errorf("expected role %s for %s, got %s", tt.role, tt.username, au.Role)
		}
	}
}

// newUserStoreWithDB builds a UserStore backed by a real SQLite database so
// persistUpdate/persistDelete execute real statements. Closing the *sql.DB
// afterwards simulates a durable-store failure (fail-closed checks).
func newUserStoreWithDB(t *testing.T) (*UserStore, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:auth_users_test?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return NewUserStore(UserStoreConfig{DB: db}), db
}

// TestUserStoreDeletePropagatesPersistError verifies that Delete fails loudly
// when the durable DELETE fails — a deleted account must never be able to
// resurrect silently after a restart.
func TestUserStoreDeletePropagatesPersistError(t *testing.T) {
	store, db := newUserStoreWithDB(t)
	user, err := store.Create("delpersist", "pass", "viewer", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	err = store.Delete(user.ID)
	if err == nil {
		t.Fatal("expected Delete to fail when the durable delete fails")
	}
	if !strings.Contains(err.Error(), "persist delete") {
		t.Errorf("expected wrapped persist delete error, got %v", err)
	}
}

// TestUserStoreUpdateRolePropagatesPersistError verifies that a role change is
// durable or fails loudly — a role escalation/demotion must not silently
// revert on the next restart.
func TestUserStoreUpdateRolePropagatesPersistError(t *testing.T) {
	store, db := newUserStoreWithDB(t)
	user, err := store.Create("rolepersist", "pass", "viewer", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	err = store.UpdateRole(user.ID, "admin")
	if err == nil {
		t.Fatal("expected UpdateRole to fail when the durable update fails")
	}
	if !strings.Contains(err.Error(), "persist role") {
		t.Errorf("expected wrapped persist role error, got %v", err)
	}
}

// TestUserStoreAuthenticatePropagatesPersistError verifies that lockout/
// brute-force counter persistence failures surface as errors instead of being
// swallowed — a lockout must not silently vanish on the next restart.
func TestUserStoreAuthenticatePropagatesPersistError(t *testing.T) {
	store, db := newUserStoreWithDB(t)
	if _, err := store.Create("lockpersist", "pass", "viewer", ""); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	_, err := store.Authenticate("lockpersist", "wrong")
	if err == nil {
		t.Fatal("expected Authenticate to fail when persisting the failed-attempt counter fails")
	}
	if !strings.Contains(err.Error(), "persist failed attempts") {
		t.Errorf("expected wrapped persist failed attempts error, got %v", err)
	}

	_, err = store.Authenticate("lockpersist", "pass")
	if err == nil {
		t.Fatal("expected Authenticate to fail when persisting the counter reset fails")
	}
	if !strings.Contains(err.Error(), "persist reset attempts") {
		t.Errorf("expected wrapped persist reset attempts error, got %v", err)
	}
}

func TestUserStoreInvalidRoleInCreate(t *testing.T) {
	store := NewUserStore(UserStoreConfig{})

	// Test all the edge case roles.
	for _, role := range []string{"", "superuser", "Admin", "ADMIN", "moderator", "guest"} {
		_, err := store.Create("user_"+role, "pass", role, "")
		if err == nil {
			t.Errorf("expected error for invalid role '%s'", role)
		}
	}
}
