// Package middleware contém os middlewares do gateway (ADR-004):
// auth de API key (ADR-006), rate-limit e logging.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/rizomai/rizomai/api/respond"
	"github.com/rizomai/rizomai/internal/domain"
	"github.com/rizomai/rizomai/internal/store"
)

// ctxKey evita colisão de chaves de contexto.
type ctxKey int

const (
	ctxTeamID ctxKey = iota
	ctxKeyID
)

// KeyVerifier abstrai o lookup de API key (interface permite mock nos testes
// sem banco real).
type KeyVerifier interface {
	LookupAPIKey(ctx context.Context, keyHash string) (*domain.APIKey, error)
}

// Auth valida `Authorization: Bearer sk_...` (ADR-006) e injeta team/key no
// contexto. Erros seguem o envelope da spec: 401 {code: UNAUTHORIZED}.
func Auth(keys KeyVerifier, pepper string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authz := r.Header.Get("Authorization")
			if authz == "" {
				unauthorized(w, "header Authorization ausente — use 'Authorization: Bearer sk_...'")
				return
			}

			scheme, key, ok := strings.Cut(authz, " ")
			if !ok || !strings.EqualFold(scheme, "Bearer") || !strings.HasPrefix(key, store.APIKeyPrefix) {
				unauthorized(w, "credencial inválida — formato esperado: Bearer sk_...")
				return
			}

			k, err := keys.LookupAPIKey(r.Context(), store.HashAPIKey(pepper, key))
			if err != nil || k == nil {
				unauthorized(w, "API key inválida ou revogada")
				return
			}

			ctx := context.WithValue(r.Context(), ctxTeamID, k.TeamID)
			ctx = context.WithValue(ctx, ctxKeyID, k.ID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func unauthorized(w http.ResponseWriter, msg string) {
	respond.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", msg, nil)
}

// TeamIDFromContext devolve o team autenticado ("" se ausente).
func TeamIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxTeamID).(string)
	return v
}

// KeyIDFromContext devolve o id da API key autenticada ("" se ausente).
func KeyIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyID).(string)
	return v
}
