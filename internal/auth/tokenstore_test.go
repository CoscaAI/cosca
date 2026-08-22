package auth

import (
	"testing"
	"time"
)

func newTestTokenStore(t *testing.T) *TokenStore {
	t.Helper()
	ts, err := NewTokenStoreInMemory()
	if err != nil {
		t.Fatalf("NewTokenStoreInMemory: %v", err)
	}
	t.Cleanup(func() { _ = ts.Close() })
	return ts
}

// TestStoreRefresh_IsValid verifies a stored, non-revoked, non-expired token is valid.
func TestStoreRefresh_IsValid(t *testing.T) {
	ts := newTestTokenStore(t)
	exp := time.Now().Add(time.Hour)

	if err := ts.StoreRefresh("user-1", "jti-1", exp); err != nil {
		t.Fatalf("StoreRefresh: %v", err)
	}
	if !ts.IsValid("user-1", "jti-1") {
		t.Error("expected jti-1 to be valid")
	}
	if ts.IsValid("user-1", "jti-unknown") {
		t.Error("expected unknown jti to be invalid")
	}
	if ts.IsValid("user-other", "jti-1") {
		t.Error("expected jti-1 to be invalid for another user")
	}
}

// TestStoreRefresh_Expired verifies an expired token is invalid.
func TestStoreRefresh_Expired(t *testing.T) {
	ts := newTestTokenStore(t)
	if err := ts.StoreRefresh("user-1", "jti-exp", time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("StoreRefresh: %v", err)
	}
	if ts.IsValid("user-1", "jti-exp") {
		t.Error("expected expired token to be invalid")
	}
}

// TestRevoke verifies Revoke invalidates all tokens for the user.
func TestRevoke(t *testing.T) {
	ts := newTestTokenStore(t)
	exp := time.Now().Add(time.Hour)
	if err := ts.StoreRefresh("user-1", "jti-1", exp); err != nil {
		t.Fatalf("StoreRefresh: %v", err)
	}
	if err := ts.StoreRefresh("user-1", "jti-2", exp); err != nil {
		t.Fatalf("StoreRefresh: %v", err)
	}
	if err := ts.Revoke("user-1"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if ts.IsValid("user-1", "jti-1") || ts.IsValid("user-1", "jti-2") {
		t.Error("expected all tokens revoked after Revoke")
	}
	// Other users unaffected.
	if err := ts.StoreRefresh("user-2", "jti-3", exp); err != nil {
		t.Fatalf("StoreRefresh user-2: %v", err)
	}
	if !ts.IsValid("user-2", "jti-3") {
		t.Error("expected user-2 token unaffected")
	}
}

// TestRevokeAndRotate_ReuseDetected verifies presenting an already-rotated
// token while another session is active triggers reuse detection.
func TestRevokeAndRotate_ReuseDetected(t *testing.T) {
	ts := newTestTokenStore(t)
	exp := time.Now().Add(time.Hour)

	// Two active sessions for the same user.
	if err := ts.StoreRefresh("user-1", "jti-a", exp); err != nil {
		t.Fatalf("StoreRefresh jti-a: %v", err)
	}
	if err := ts.StoreRefresh("user-1", "jti-b", exp); err != nil {
		t.Fatalf("StoreRefresh jti-b: %v", err)
	}

	// Rotate jti-a legitimately: retire it.
	if err := ts.RevokeAndRotate("user-1", "jti-a", "jti-c"); err != nil {
		t.Fatalf("RevokeAndRotate legitimate: %v", err)
	}

	// Now jti-a is no longer active — presenting it again = reuse (theft).
	if err := ts.RevokeAndRotate("user-1", "jti-a", "jti-d"); err == nil {
		t.Fatal("expected ErrTokenReuse when presenting already-rotated token with active session")
	}

	// Reuse detection revokes ALL sessions for the user.
	if ts.IsValid("user-1", "jti-b") {
		t.Error("expected jti-b revoked after reuse detection")
	}
}
