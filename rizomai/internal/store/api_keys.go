// Repositório de API keys (ADR-006).
//
// Armazenamento: apenas o HASH da chave — SHA-256(pepper || key).
// Justificativa: a chave tem alta entropia (sk_live_ + 16 bytes de crypto/rand),
// então SHA-256 com pepper é suficiente (mesmo padrão de Stripe); o pepper via
// env protege contra rainbow-tables caso o banco vaze. Sem custo computacional
// por request (irrelevante: ~60 req/min/chave). scrypt/bcrypt seriam overkill
// para credenciais de alta entropia (são para senhas).
package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/rizomai/rizomai/internal/domain"
)

const (
	// APIKeyPrefix é o prefixo público das chaves (formato sk_live_...).
	APIKeyPrefix = "sk_live_"
	// apiKeyEntropyBytes é a entropia do sufixo (16 bytes → 32 chars hex).
	apiKeyEntropyBytes = 16
)

// HashAPIKey computa o hash armazenado: SHA-256(pepper + 0x00 + key).
func HashAPIKey(pepper, key string) string {
	h := sha256.New()
	h.Write([]byte(pepper))
	h.Write([]byte{0})
	h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil))
}

// CreateAPIKey gera uma chave sk_live_<hex>, persiste SOMENTE o hash e devolve
// a chave pura UMA única vez (o chamador deve exibi-la ao operador — não há
// rota pública; uso em seeding/scripts — ADR-006).
func (s *Store) CreateAPIKey(ctx context.Context, teamID, name, pepper string) (plainKey string, key *domain.APIKey, err error) {
	id, err := domain.NewAPIKeyID()
	if err != nil {
		return "", nil, err
	}

	secret := make([]byte, apiKeyEntropyBytes)
	if _, err := rand.Read(secret); err != nil {
		return "", nil, err
	}
	plainKey = APIKeyPrefix + hex.EncodeToString(secret)

	now := time.Now().UTC()
	key = &domain.APIKey{
		ID:        id,
		TeamID:    teamID,
		Name:      name,
		KeyHash:   HashAPIKey(pepper, plainKey),
		KeyPrefix: plainKey[:len(APIKeyPrefix)+4], // sk_live_<4 primeiros hex> p/ identificação visual
		CreatedAt: now,
	}

	if _, err := s.db.Exec(ctx,
		`INSERT INTO api_keys (id, team_id, name, key_hash, key_prefix) VALUES ($1, $2, $3, $4, $5)`,
		key.ID, key.TeamID, key.Name, key.KeyHash, key.KeyPrefix,
	); err != nil {
		return "", nil, err
	}
	return plainKey, key, nil
}

// LookupAPIKey busca uma chave ativa (não revogada) por hash e atualiza
// last_used_at (best-effort). Usada pelo middleware de auth.
func (s *Store) LookupAPIKey(ctx context.Context, keyHash string) (*domain.APIKey, error) {
	var k domain.APIKey
	err := s.db.QueryRow(ctx,
		`SELECT id, team_id, name, key_hash, key_prefix, last_used_at, revoked_at, created_at
		   FROM api_keys
		  WHERE key_hash = $1 AND revoked_at IS NULL`,
		keyHash,
	).Scan(&k.ID, &k.TeamID, &k.Name, &k.KeyHash, &k.KeyPrefix,
		&k.LastUsedAt, &k.RevokedAt, &k.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	_, _ = s.db.Exec(ctx,
		`UPDATE api_keys SET last_used_at = now() WHERE id = $1`, k.ID)
	return &k, nil
}
