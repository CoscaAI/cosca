// Testes do conector Facebook com httptest mockando a Graph API (sem rede real).
package facebook

import (
	"context"
	"encoding/json"
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
	c := New(Config{ClientID: "cli", ClientSecret: "sec", RedirectURI: "http://localhost:8080/cb"})
	c.apiBase = srv.URL
	c.tokenURL = srv.URL + "/oauth"
	c.authBase = srv.URL + "/dialog"
	return c
}

func TestPublishToPageFeed(t *testing.T) {
	var gotMessage string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/12345678901234/feed" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotMessage, _ = body["message"].(string)
		_, _ = w.Write([]byte(`{"id":"12345678901234_9876543210"}`))
	})

	res, err := c.Publish(context.Background(), "olá mundo", &domain.PostTarget{},
		types.Credentials{AccessToken: "tok", ExternalID: "12345678901234"})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if gotMessage != "olá mundo" {
		t.Errorf("message = %q", gotMessage)
	}
	if res.ExternalID != "12345678901234_9876543210" {
		t.Errorf("ExternalID = %q", res.ExternalID)
	}
}

func TestPublishNoPage(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("não deveria publicar sem pageId")
	})
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{}, types.Credentials{AccessToken: "t"})
	if err == nil || !strings.Contains(err.Error(), "no_page") {
		t.Errorf("esperava no_page, veio %v", err)
	}
}

func TestPublishRateLimitedRetryable(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"code":2,"message":"temporarily unavailable"}}`))
	})
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{}, types.Credentials{AccessToken: "t", ExternalID: "123"})
	pe, ok := err.(*types.Error)
	if !ok || !pe.Retryable {
		t.Fatalf("503 deveria ser retryable, veio %v", err)
	}
}

func TestPublishNoCredentials(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("não deveria chamar a API sem token")
	})
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{}, types.Credentials{ExternalID: "123"})
	if err == nil || !strings.Contains(err.Error(), "no_credentials") {
		t.Errorf("esperava no_credentials, veio %v", err)
	}
}

func TestPublishNotConfigured(t *testing.T) {
	c := New(Config{})
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{}, types.Credentials{AccessToken: "t", ExternalID: "123"})
	if err == nil || !strings.Contains(err.Error(), "not_configured") {
		t.Errorf("esperava not_configured, veio %v", err)
	}
}

func TestValidateAccount(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/me" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"123"}`))
	})
	if err := c.ValidateAccount(context.Background(), types.Credentials{AccessToken: "t"}); err != nil {
		t.Fatalf("ValidateAccount: %v", err)
	}
}

func TestValidateAccountInvalidToken(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":190,"message":"Invalid OAuth access token"}}`))
	})
	err := c.ValidateAccount(context.Background(), types.Credentials{AccessToken: "bad"})
	if err == nil {
		t.Fatal("esperava erro")
	}
	pe, _ := err.(*types.Error)
	if pe == nil || pe.Code != "invalid_token" {
		t.Errorf("esperava invalid_token, veio %v", err)
	}
}
