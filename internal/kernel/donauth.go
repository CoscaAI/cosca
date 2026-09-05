// Package kernel — Don authentication (war phrase).
//
// ⚠️ LEGADO — a war phrase NÃO é mais chamada pelo portão privilegiado.
//
// A assinatura da family chain agora é machine-bound via DPAPI + nonce de
// consentimento-ao-conteúdo (ver internal/cli/memory_identity.go). O portão
// privilegiado deixou de usar a war phrase como 2º fator; este pacote é
// mantido APENAS para compatibilidade com o subsistema `cosca don` e outros
// consumidores legados (cosca CLI / kernel.DonAuth / HashDonPhrase). Não
// alterar a semântica sem coordenação com a squad que migra os testes.
//
// Histórico: proteção contra impersonação — ordens de alto risco eram
// confirmadas pela war phrase do Don. Apenas o hash bcrypt da frase era
// armazenado (nunca a frase em si), e toda tentativa de verificação — sucesso
// ou falha — era registrada para o Don auditar quem tentou falar em seu nome.
package kernel

import (
	"errors"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// DonAuth verifies the Don's war phrase and records every attempt.
type DonAuth struct {
	mu        sync.RWMutex
	hash      []byte // bcrypt hash of the war phrase (never the phrase itself)
	enabled   bool
	attempts  []DonAttempt
	maxLog    int
	lockUntil time.Time // brute-force protection: lock after consecutive failures
	lockAfter int       // consecutive failures that trigger the lock
	failures  int
}

// DonAttempt is a single recorded verification attempt.
type DonAttempt struct {
	OK        bool      `json:"ok"`
	At        time.Time `json:"at"`
	Reason    string    `json:"reason,omitempty"`
	Source    string    `json:"source,omitempty"` // e.g. "session:abc123", "rest:10.0.0.7"
	Challenge string    `json:"challenge"`        // what operation the phrase protected
}

// NewDonAuth creates a DonAuth in disabled state (no war phrase configured).
// Use SetPhrase to arm it.
func NewDonAuth() *DonAuth {
	return &DonAuth{
		enabled:   false,
		maxLog:    100,
		lockAfter: 5,
	}
}

// ErrDonLocked is returned when too many failed attempts have temporarily
// locked authentication.
var ErrDonLocked = errors.New("don auth locked: too many failed attempts")

// ErrDonNotArmed is returned when no war phrase has been configured.
var ErrDonNotArmed = errors.New("don auth not armed: no war phrase configured")

// ErrDonPhraseMismatch is returned when the provided phrase is wrong.
var ErrDonPhraseMismatch = errors.New("don auth failed: phrase mismatch")

// SetPhrase arms authentication with the given war phrase. Only the bcrypt
// hash is kept. Returns an error if hashing fails.
func (d *DonAuth) SetPhrase(phrase string) error {
	if strings.TrimSpace(phrase) == "" {
		return errors.New("war phrase must not be empty")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(phrase), 12)
	if err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.hash = hash
	d.enabled = true
	d.failures = 0
	d.lockUntil = time.Time{}
	return nil
}

// SetPhraseHash arms authentication from a pre-computed bcrypt hash
// (e.g. loaded from config). Useful so the phrase itself never touches disk.
func (d *DonAuth) SetPhraseHash(hash []byte) error {
	if len(hash) == 0 {
		return errors.New("phrase hash must not be empty")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.hash = hash
	d.enabled = true
	d.failures = 0
	d.lockUntil = time.Time{}
	return nil
}

// Disable disarms authentication (only the Don may do this).
func (d *DonAuth) Disable() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.enabled = false
	d.hash = nil
	d.failures = 0
	d.lockUntil = time.Time{}
}

// Enabled reports whether a war phrase is configured.
func (d *DonAuth) Enabled() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.enabled
}

// Verify checks the phrase for the given challenge (operation being
// protected) and source (who is asking). It records the attempt and enforces
// a temporary lock after repeated failures.
func (d *DonAuth) Verify(phrase, challenge, source string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.enabled {
		return d.log(ErrDonNotArmed, false, challenge, source)
	}

	now := time.Now()
	if now.Before(d.lockUntil) {
		return d.log(ErrDonLocked, false, challenge, source)
	}

	if d.failures >= d.lockAfter {
		d.lockUntil = now.Add(time.Minute)
		d.failures = 0
		return d.log(ErrDonLocked, false, challenge, source)
	}

	if err := bcrypt.CompareHashAndPassword(d.hash, []byte(phrase)); err != nil {
		d.failures++
		return d.log(ErrDonPhraseMismatch, false, challenge, source)
	}

	d.failures = 0
	return d.log(nil, true, challenge, source)
}

// log records an attempt and returns err (nil on success).
func (d *DonAuth) log(err error, ok bool, challenge, source string) error {
	d.attempts = append(d.attempts, DonAttempt{
		OK:        ok,
		At:        time.Now().UTC(),
		Reason:    reasonOf(err),
		Source:    source,
		Challenge: challenge,
	})
	if len(d.attempts) > d.maxLog {
		d.attempts = d.attempts[len(d.attempts)-d.maxLog:]
	}
	return err
}

func reasonOf(err error) string {
	if err == nil {
		return "ok"
	}
	return err.Error()
}

// Attempts returns a copy of the recorded verification attempts (most recent
// last). Used by the audit trail so the Don can see every impersonation try.
func (d *DonAuth) Attempts() []DonAttempt {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]DonAttempt, len(d.attempts))
	copy(out, d.attempts)
	return out
}

// Reset clears the attempt log and the failure counter.
func (d *DonAuth) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.attempts = nil
	d.failures = 0
	d.lockUntil = time.Time{}
}

// DonState is the serializable brute-force state, persisted between process
// executions so the lockout survives CLI restarts.
type DonState struct {
	Failures  int       `json:"failures"`
	LockUntil time.Time `json:"lock_until"`
}

// Snapshot returns the current brute-force state. The caller persists it
// (e.g. to disk) so a subsequent process can Restore it and the lockout
// survives restarts.
func (d *DonAuth) Snapshot() DonState {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return DonState{Failures: d.failures, LockUntil: d.lockUntil}
}

// Restore applies a previously persisted brute-force state. A lock that has
// already expired is cleared; remaining failure count carries over.
func (d *DonAuth) Restore(s DonState) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.failures = s.Failures
	d.lockUntil = s.LockUntil
	// Only clear when a lock was actually active and has since expired.
	// A zero LockUntil means no lock was ever armed — failures carry over.
	if !d.lockUntil.IsZero() && time.Now().After(d.lockUntil) {
		d.lockUntil = time.Time{}
		d.failures = 0
	}
}

// HashDonPhrase hashes the Don's war phrase with bcrypt (cost 12, the house
// standard from internal/auth). Only the hash is ever persisted — never the
// phrase itself.
func HashDonPhrase(phrase string) ([]byte, error) {
	if strings.TrimSpace(phrase) == "" {
		return nil, errors.New("war phrase must not be empty")
	}
	return bcrypt.GenerateFromPassword([]byte(phrase), 12)
}
