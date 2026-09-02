package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rizomai/rizomai/internal/domain"
	"github.com/rizomai/rizomai/internal/store"
)

// mockKeys é um KeyVerifier falso (sem banco).
type mockKeys struct {
	byHash map[string]*domain.APIKey
}

func (m *mockKeys) LookupAPIKey(ctx context.Context, hash string) (*domain.APIKey, error) {
	if k, ok := m.byHash[hash]; ok {
		return k, nil
	}
	return nil, store.ErrNotFound
}

func TestAuth(t *testing.T) {
	const pepper = "test-pepper"
	plainKey := store.APIKeyPrefix + "1234abcd"
	validHash := store.HashAPIKey(pepper, plainKey)

	mk := &mockKeys{byHash: map[string]*domain.APIKey{
		validHash: {ID: "key_1", TeamID: "team_1"},
	}}

	var gotTeamID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTeamID = TeamIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	handler := Auth(mk, pepper)(inner)

	cases := []struct {
		name       string
		authHeader string
		wantStatus int
		wantTeam   string
	}{
		{"sem header", "", http.StatusUnauthorized, ""},
		{"scheme não-bearer", "Basic dXNlcjpwYXNz", http.StatusUnauthorized, ""},
		{"sem prefixo sk_", "Bearer tokenqualquer", http.StatusUnauthorized, ""},
		{"chave desconhecida", "Bearer sk_live_zzzz", http.StatusUnauthorized, ""},
		{"chave válida", "Bearer " + plainKey, http.StatusOK, "team_1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotTeamID = ""
			req := httptest.NewRequest(http.MethodGet, "/v1/posts", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, esperado %d (body: %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if gotTeamID != tc.wantTeam {
				t.Errorf("team no contexto = %q, esperado %q", gotTeamID, tc.wantTeam)
			}

			// 401 deve seguir o envelope da spec {code, error, details}
			if tc.wantStatus == http.StatusUnauthorized {
				var env struct {
					Code string `json:"code"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
					t.Fatalf("resposta 401 não é JSON válido: %v", err)
				}
				if env.Code != "UNAUTHORIZED" {
					t.Errorf("code = %q, esperado UNAUTHORIZED", env.Code)
				}
			}
		})
	}
}
