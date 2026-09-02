// IDs com prefixo estável (ADR-005 §1.4): profile_, post_, target_, ...
//
// Sufixo = 16 bytes de crypto/rand em hex (32 chars): entropia suficiente para
// colisão desprezível, legível em logs/URLs e indexável como TEXT no Postgres.
// A geração é feita em Go (não via extensão pgcrypto) porque o contrato exige
// prefixo textual — o valor persistido é o valor exposto na API.
package domain

import (
	"crypto/rand"
	"encoding/hex"
)

const idSuffixBytes = 16

func newID(prefix string) (string, error) {
	b := make([]byte, idSuffixBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b), nil
}

// NewTeamID gera um ID de team (prefixo team_).
func NewTeamID() (string, error) { return newID("team_") }

// NewProfileID gera um ID de profile (prefixo profile_).
func NewProfileID() (string, error) { return newID("profile_") }

// NewAccountID gera um ID de conta conectada (prefixo account_).
func NewAccountID() (string, error) { return newID("account_") }

// NewPostID gera um ID de post (prefixo post_).
func NewPostID() (string, error) { return newID("post_") }

// NewTargetID gera um ID de post_target (prefixo target_).
func NewTargetID() (string, error) { return newID("target_") }

// NewAttemptID gera um ID de publish_attempt (prefixo attempt_).
func NewAttemptID() (string, error) { return newID("attempt_") }

// NewAPIKeyID gera um ID de api_key (prefixo key_).
func NewAPIKeyID() (string, error) { return newID("key_") }

// NewEventID gera um ID de evento de webhook (prefixo evt_ — ADR-009 §1.1).
func NewEventID() (string, error) { return newID("evt_") }

// NewDeliveryID gera um ID de entrega de webhook (prefixo dlv_).
func NewDeliveryID() (string, error) { return newID("dlv_") }

// NewOAuthStateID gera um ID de state OAuth (prefixo oast_).
func NewOAuthStateID() (string, error) { return newID("oast_") }
