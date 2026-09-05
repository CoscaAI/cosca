package kernel

import (
	"errors"
	"golang.org/x/crypto/bcrypt"
	"testing"
	"time"
)

// TestDonAuth_NotArmed verifies that verification fails cleanly when no war
// phrase is configured.
func TestDonAuth_NotArmed(t *testing.T) {
	d := NewDonAuth()
	if d.Enabled() {
		t.Fatal("must start disabled")
	}
	err := d.Verify("qualquer", "jail bypass", "session:test")
	if !errors.Is(err, ErrDonNotArmed) {
		t.Fatalf("expected ErrDonNotArmed, got %v", err)
	}
	att := d.Attempts()
	if len(att) != 1 || att[0].OK {
		t.Fatalf("attempt must be recorded and failed: %+v", att)
	}
}

// TestDonAuth_PhraseRoundTrip verifies correct and incorrect phrases.
func TestDonAuth_PhraseRoundTrip(t *testing.T) {
	d := NewDonAuth()
	if err := d.SetPhrase("sangue e ouro"); err != nil {
		t.Fatal(err)
	}
	if !d.Enabled() {
		t.Fatal("must be enabled after SetPhrase")
	}
	if err := d.Verify("sangue e ouro", "sudo rm", "session:don"); err != nil {
		t.Fatalf("correct phrase must pass: %v", err)
	}
	if err := d.Verify("errada", "sudo rm", "session:impostor"); !errors.Is(err, ErrDonPhraseMismatch) {
		t.Fatalf("wrong phrase must fail with mismatch, got %v", err)
	}
	// The phrase itself must never be stored.
	if string(d.hash) == "sangue e ouro" {
		t.Fatal("phrase must never be stored in plaintext")
	}
}

// TestDonAuth_PhraseHash verifies arming from a pre-computed hash works and
// that tampered hashes are rejected at arm time.
func TestDonAuth_PhraseHash(t *testing.T) {
	d := NewDonAuth()
	// Tampered/empty hash rejected.
	if err := d.SetPhraseHash(nil); err == nil {
		t.Fatal("empty hash must be rejected")
	}
	// Valid flow: hash computed elsewhere (e.g. config) then loaded.
	h := mustBcrypt(t, "fogo no parquinho")
	if err := d.SetPhraseHash(h); err != nil {
		t.Fatal(err)
	}
	if err := d.Verify("fogo no parquinho", "cosca don status", "session:don"); err != nil {
		t.Fatalf("phrase must verify against loaded hash: %v", err)
	}
}

// TestDonAuth_Lockout verifies brute-force protection: after N consecutive
// failures, verification is temporarily locked.
func TestDonAuth_Lockout(t *testing.T) {
	d := NewDonAuth()
	d.SetPhrase("segredo")
	// Fail 5 times → next attempt locks.
	for i := 0; i < 5; i++ {
		d.Verify("errada", "challenge", "session:x")
	}
	if err := d.Verify("errada", "challenge", "session:x"); !errors.Is(err, ErrDonLocked) {
		t.Fatalf("expected lock after repeated failures, got %v", err)
	}
	// Even the correct phrase is refused while locked.
	if err := d.Verify("segredo", "challenge", "session:don"); !errors.Is(err, ErrDonLocked) {
		t.Fatalf("correct phrase must still be locked, got %v", err)
	}
}

// TestDonAuth_AttemptLog verifies the audit trail records success, failure
// and source, and that Reset clears it.
func TestDonAuth_AttemptLog(t *testing.T) {
	d := NewDonAuth()
	d.SetPhrase("pai da familia")
	d.Verify("errada", "jail", "session:impostor")
	d.Verify("pai da familia", "jail", "session:don")

	att := d.Attempts()
	if len(att) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(att))
	}
	if att[0].OK || !att[1].OK {
		t.Fatalf("first must fail, second must pass: %+v", att)
	}
	if att[0].Source != "session:impostor" || att[0].Challenge != "jail" {
		t.Fatalf("attempt metadata missing: %+v", att[0])
	}
	if att[0].At.IsZero() || att[0].At.After(time.Now().Add(time.Minute)) {
		t.Fatalf("attempt timestamp invalid: %+v", att[0])
	}
	d.Reset()
	if len(d.Attempts()) != 0 {
		t.Fatal("Reset must clear the attempt log")
	}
}

// TestDonAuth_Disable verifies the Don can disarm and that armed state
// returns to disabled cleanly.
func TestDonAuth_Disable(t *testing.T) {
	d := NewDonAuth()
	d.SetPhrase("pode desligar")
	if !d.Enabled() {
		t.Fatal("must be enabled")
	}
	d.Disable()
	if d.Enabled() {
		t.Fatal("must be disabled after Disable")
	}
	if err := d.Verify("pode desligar", "x", "session:don"); !errors.Is(err, ErrDonNotArmed) {
		t.Fatalf("disabled auth must report not armed, got %v", err)
	}
}

// TestDonAuth_EmptyPhrase verifies an empty war phrase is rejected.
func TestDonAuth_EmptyPhrase(t *testing.T) {
	d := NewDonAuth()
	if err := d.SetPhrase("   "); err == nil {
		t.Fatal("empty/blank phrase must be rejected")
	}
}

// TestDonAuth_SnapshotRestore verifies brute-force state survives a process
// restart: failures carry over, and the accumulated failures from a previous
// process trigger the lock — blocking even the correct phrase.
func TestDonAuth_SnapshotRestore(t *testing.T) {
	// Process 1: fail 4 times (below the lock threshold of 5).
	d1 := NewDonAuth()
	d1.SetPhrase("segredo")
	for i := 0; i < 4; i++ {
		d1.Verify("errada", "challenge", "session:x")
	}
	snap := d1.Snapshot()
	if snap.Failures != 4 {
		t.Fatalf("expected 4 failures in snapshot, got %d", snap.Failures)
	}

	// Process 2: restore. The 5th failure increments to the threshold, and
	// the next attempt (even with the correct phrase) must be locked.
	d2 := NewDonAuth()
	d2.SetPhrase("segredo")
	d2.Restore(snap)
	if err := d2.Verify("errada", "challenge", "session:x"); !errors.Is(err, ErrDonPhraseMismatch) {
		t.Fatalf("5th failure must report mismatch, got %v", err)
	}
	if snap2 := d2.Snapshot(); snap2.Failures != 5 {
		t.Fatalf("failures must reach threshold 5, got %d", snap2.Failures)
	}
	if err := d2.Verify("segredo", "challenge", "session:don"); !errors.Is(err, ErrDonLocked) {
		t.Fatalf("correct phrase must be refused once threshold is reached, got %v", err)
	}
}

// TestDonAuth_RestoreExpiredLock verifies an expired lock is cleared on
// Restore and does not keep the auth trapped.
func TestDonAuth_RestoreExpiredLock(t *testing.T) {
	d := NewDonAuth()
	d.SetPhrase("segredo")
	d.Restore(DonState{Failures: 5, LockUntil: time.Now().Add(-time.Minute)})
	if err := d.Verify("segredo", "challenge", "session:don"); err != nil {
		t.Fatalf("expired lock must be cleared on restore, got %v", err)
	}
}

// TestDonAuth_SnapshotClearedAfterSuccess verifies a successful verification
// resets failures so the next snapshot is clean.
func TestDonAuth_SnapshotClearedAfterSuccess(t *testing.T) {
	d := NewDonAuth()
	d.SetPhrase("segredo")
	d.Verify("errada", "challenge", "session:x")
	d.Verify("segredo", "challenge", "session:don")
	snap := d.Snapshot()
	if snap.Failures != 0 {
		t.Fatalf("failures must reset after success, got %d", snap.Failures)
	}
}

// mustBcrypt hashes a phrase for tests using the same cost as the runtime.
func mustBcrypt(t *testing.T, phrase string) []byte {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(phrase), 12)
	if err != nil {
		t.Fatal(err)
	}
	return h
}
