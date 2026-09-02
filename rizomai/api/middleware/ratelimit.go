// Rate limiting: token bucket em memória por tenant (ADR-010: rate limit por
// key/tenant). Default 60 req/min. Headers X-RateLimit-* declarados na spec
// (openapi/rizomai.yaml) e 429 RATE_LIMITED com Retry-After.
//
// Limitação conhecida (documentada): buckets vivem na memória do processo —
// em multi-réplica o limite é por instância, não global. Aceitável no MVP
// (1 réplica); gate para Redis/cache centralizado na Fase 3+.
package middleware

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/rizomai/rizomai/api/respond"
)

// RateLimiter é um token bucket por chave.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	capacity float64       // tokens máximos (burst)
	refill   float64       // tokens por segundo
	ttl      time.Duration // inatividade para descartar o bucket
}

type bucket struct {
	tokens  float64
	updated time.Time
}

// NewRateLimiter cria um limitador de reqPerMin por chave.
func NewRateLimiter(reqPerMin int) *RateLimiter {
	if reqPerMin <= 0 {
		reqPerMin = 60
	}
	cap := float64(reqPerMin)
	return &RateLimiter{
		buckets:  map[string]*bucket{},
		capacity: cap,
		refill:   cap / 60.0,
		ttl:      10 * time.Minute,
	}
}

// Allow consome 1 token da chave e reporta remaining e o epoch de reset.
func (rl *RateLimiter) Allow(key string, now time.Time) (ok bool, remaining int, resetEpoch int64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, exists := rl.buckets[key]
	if !exists || now.Sub(b.updated) > rl.ttl {
		b = &bucket{tokens: rl.capacity, updated: now}
		rl.buckets[key] = b
	}

	elapsed := now.Sub(b.updated).Seconds()
	b.tokens = math.Min(rl.capacity, b.tokens+elapsed*rl.refill)
	b.updated = now

	if b.tokens >= 1 {
		b.tokens--
		remaining = int(math.Floor(b.tokens))
		toFull := (rl.capacity - b.tokens) / rl.refill
		resetEpoch = now.Add(time.Duration(math.Ceil(toFull)) * time.Second).Unix()
		return true, remaining, resetEpoch
	}

	// Negado: tempo até ter 1 token de novo (Retry-After).
	toOne := (1 - b.tokens) / rl.refill
	resetEpoch = now.Add(time.Duration(math.Ceil(toOne)) * time.Second).Unix()
	return false, 0, resetEpoch
}

// Limit é o limite por janela (para o header X-RateLimit-Limit).
func (rl *RateLimiter) Limit() int { return int(rl.capacity) }

// RateLimit é o middleware: aplica por team autenticado. Rotas sem auth
// (ex.: healthz) passam direto.
func RateLimit(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			teamID := TeamIDFromContext(r.Context())
			if teamID == "" {
				next.ServeHTTP(w, r)
				return
			}

			now := time.Now()
			ok, remaining, resetEpoch := rl.Allow(teamID, now)

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.Limit()))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetEpoch, 10))

			if !ok {
				retryAfter := resetEpoch - now.Unix()
				if retryAfter < 1 {
					retryAfter = 1
				}
				w.Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))
				respond.Error(w, http.StatusTooManyRequests, "RATE_LIMITED",
					"Rate limit excedido — tente novamente mais tarde",
					map[string]any{"retryAfterSeconds": retryAfter})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
