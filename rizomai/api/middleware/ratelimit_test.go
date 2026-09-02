package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// teamCtx injeta o team no contexto (teste no mesmo pacote — acessa ctxTeamID).
func teamCtx(ctx context.Context, teamID string) context.Context {
	return context.WithValue(ctx, ctxTeamID, teamID)
}

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter(3) // 3 req/min p/ teste rápido
	now := time.Now()

	for i := 0; i < 3; i++ {
		ok, _, _ := rl.Allow("team_1", now)
		if !ok {
			t.Fatalf("request %d deveria passar", i+1)
		}
	}

	// 4º imediato → negado
	ok, remaining, reset := rl.Allow("team_1", now.Add(time.Second))
	if ok {
		t.Fatal("4º request deveria ser negado")
	}
	if remaining != 0 {
		t.Errorf("remaining = %d, esperado 0", remaining)
	}
	if reset <= now.Unix() {
		t.Errorf("reset epoch deveria estar no futuro, veio %d (now %d)", reset, now.Unix())
	}

	// Após ~60s o refill devolve tokens
	ok, _, _ = rl.Allow("team_1", now.Add(60*time.Second))
	if !ok {
		t.Error("após 60s o bucket deveria ter refill")
	}

	// Chaves distintas são independentes
	ok, _, _ = rl.Allow("team_2", now)
	if !ok {
		t.Error("team_2 deveria ter bucket próprio")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	rl := NewRateLimiter(2)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RateLimit(rl)(inner)

	// Rota sem team (pública) não é limitada.
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rota sem auth deveria passar: %d", rec.Code)
	}

	// Com team: 2 passam, 3º vira 429 com headers.
	for i := 0; i < 2; i++ {
		req = httptest.NewRequest(http.MethodGet, "/v1/posts", nil)
		req = req.WithContext(teamCtx(req.Context(), "team_1"))
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d deveria passar: %d", i+1, rec.Code)
		}
		if got := rec.Header().Get("X-RateLimit-Limit"); got != "2" {
			t.Errorf("X-RateLimit-Limit = %q, esperado 2", got)
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/posts", nil)
	req = req.WithContext(teamCtx(req.Context(), "team_1"))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("3º request deveria ser 429, veio %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After deveria estar presente no 429")
	}
	if rec.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Error("X-RateLimit-Remaining deveria ser 0 no 429")
	}
	if _, err := strconv.Atoi(rec.Header().Get("X-RateLimit-Reset")); err != nil {
		t.Errorf("X-RateLimit-Reset deveria ser epoch numérico: %v", err)
	}
}
