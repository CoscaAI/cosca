// Testes do conector X com httptest mockando a API v2 (sem rede real).
package x

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rizomai/rizomai/internal/domain"
	"github.com/rizomai/rizomai/internal/platform/types"
)

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := New(Config{ClientID: "cli_id", ClientSecret: "cli_secret", RedirectURI: "http://localhost:8080/v1/connect/x/callback"})
	c.apiBase = srv.URL
	c.tokenURL = srv.URL + "/token"
	c.authBase = srv.URL + "/authorize"
	return c
}

func TestPublish(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2/tweets" || r.Method != http.MethodPost {
			t.Errorf("esperava POST /2/tweets, veio %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-abc" {
			t.Errorf("Authorization = %q", got)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["text"] != "olá mundo" {
			t.Errorf("text = %v", body["text"])
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"id":"1860000001","text":"olá mundo"}}`))
	})

	res, err := c.Publish(context.Background(), "olá mundo", &domain.PostTarget{Platform: domain.PlatformX}, types.Credentials{AccessToken: "token-abc"})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if res.ExternalID != "1860000001" {
		t.Errorf("ExternalID = %q", res.ExternalID)
	}
	if !strings.Contains(res.PublishedURL, "1860000001") {
		t.Errorf("PublishedURL = %q", res.PublishedURL)
	}
}

func TestPublishRateLimitedIsRetryable(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"detail":"Rate limit exceeded"}`))
	})

	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{}, types.Credentials{AccessToken: "t"})
	if err == nil {
		t.Fatal("esperava erro")
	}
	pe, ok := err.(*types.Error)
	if !ok {
		t.Fatalf("erro não é *types.Error: %T", err)
	}
	if !pe.Retryable {
		t.Error("429 deveria ser Retryable (ADR-003)")
	}
}

func TestPublishNoCredentials(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("não deveria fazer request sem token")
	})
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{}, types.Credentials{})
	if err == nil || !strings.Contains(err.Error(), "no_credentials") {
		t.Errorf("esperava no_credentials, veio %v", err)
	}
}

func TestPublishNotConfigured(t *testing.T) {
	c := New(Config{}) // sem credenciais de app
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{}, types.Credentials{AccessToken: "t"})
	if err == nil || !strings.Contains(err.Error(), "not_configured") {
		t.Errorf("esperava not_configured, veio %v", err)
	}
}

func TestValidateAccount(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2/users/me" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":{"id":"123","username":"fulano"}}`))
	})
	if err := c.ValidateAccount(context.Background(), types.Credentials{AccessToken: "t"}); err != nil {
		t.Fatalf("ValidateAccount: %v", err)
	}
}

func TestValidateAccountInvalidToken(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"title":"Unauthorized","detail":"invalid token"}`))
	})
	err := c.ValidateAccount(context.Background(), types.Credentials{AccessToken: "t"})
	if err == nil {
		t.Fatal("esperava erro")
	}
	pe, _ := err.(*types.Error)
	if pe == nil || pe.Code != "invalid_token" {
		t.Errorf("esperava invalid_token, veio %v", err)
	}
}

func TestExchangeCode(t *testing.T) {
	var gotForm, gotAuth string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotForm = string(b)
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"access_token":"at","refresh_token":"rt","expires_in":7200,"scope":"tweet.read"}`))
	})

	tok, err := c.ExchangeCode(context.Background(), "code123", "myverifier", "http://localhost:8080/cb")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if tok.AccessToken != "at" || tok.RefreshToken != "rt" {
		t.Errorf("token = %+v", tok)
	}
	if !strings.Contains(gotForm, "grant_type=authorization_code") || !strings.Contains(gotForm, "code_verifier=myverifier") {
		t.Errorf("form = %q", gotForm)
	}
	if !strings.HasPrefix(gotAuth, "Basic ") {
		t.Errorf("Authorization = %q (esperava Basic)", gotAuth)
	}
}

func TestAuthURLPKCE(t *testing.T) {
	c := New(Config{ClientID: "cli", ClientSecret: "sec", RedirectURI: "https://cb"})
	state := "statetest"
	verifier := "verifier1234567890verifier1234567890"
	u, err := c.AuthURL(state, verifier)
	if err != nil {
		t.Fatal(err)
	}
	// verifier embutido no state (insumo §6.1)
	if !strings.Contains(u, "state="+state+"-cv_"+verifier) {
		t.Errorf("state não embute verifier: %s", u)
	}
	// code_challenge = S256 do verifier
	want := pkceChallenge(verifier)
	if !strings.Contains(u, "code_challenge="+want) {
		t.Errorf("code_challenge ausente/errado: %s", u)
	}
	if !strings.Contains(u, "code_challenge_method=S256") {
		t.Errorf("method S256 ausente: %s", u)
	}
}
