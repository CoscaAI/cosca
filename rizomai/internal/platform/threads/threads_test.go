// Testes do conector Threads com httptest mockando a API da Meta (sem rede real).
package threads

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
	c.tokenURL = srv.URL + "/access_token"
	c.authBase = srv.URL + "/authorize"
	return c
}

func TestPublishText(t *testing.T) {
	var createPath, pubPath string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/123/threads":
			createPath = r.URL.Path
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["media_type"] != "TEXT" || body["text"] != "olá" {
				t.Errorf("body threads = %v", body)
			}
			_, _ = w.Write([]byte(`{"id":"456"}`))
		case "/456/publish":
			pubPath = r.URL.Path
			_, _ = w.Write([]byte(`{"id":"789"}`))
		default:
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
	})

	res, err := c.Publish(context.Background(), "olá", &domain.PostTarget{}, types.Credentials{AccessToken: "tok", ExternalID: "123"})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if createPath != "/123/threads" || pubPath != "/456/publish" {
		t.Errorf("caminhos: create=%q publish=%q", createPath, pubPath)
	}
	if res.ExternalID != "789" {
		t.Errorf("ExternalID = %q", res.ExternalID)
	}
}

func TestPublishNoAccount(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("não deveria chamar a API sem user id")
	})
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{}, types.Credentials{AccessToken: "t"})
	if err == nil || !strings.Contains(err.Error(), "no_account") {
		t.Errorf("esperava no_account, veio %v", err)
	}
}

func TestPublishRateLimitedRetryable(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limit"}}`))
	})
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{}, types.Credentials{AccessToken: "t", ExternalID: "1"})
	pe, ok := err.(*types.Error)
	if !ok || !pe.Retryable {
		t.Fatalf("429 deveria ser retryable, veio %v", err)
	}
}

func TestPublishNoCredentials(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("não deveria chamar a API sem token")
	})
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{}, types.Credentials{ExternalID: "1"})
	if err == nil || !strings.Contains(err.Error(), "no_credentials") {
		t.Errorf("esperava no_credentials, veio %v", err)
	}
}

func TestPublishNotConfigured(t *testing.T) {
	c := New(Config{})
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{}, types.Credentials{AccessToken: "t", ExternalID: "1"})
	if err == nil || !strings.Contains(err.Error(), "not_configured") {
		t.Errorf("esperava not_configured, veio %v", err)
	}
}

func TestValidateAccount(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/me" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"123","username":"fulano"}`))
	})
	if err := c.ValidateAccount(context.Background(), types.Credentials{AccessToken: "t"}); err != nil {
		t.Fatalf("ValidateAccount: %v", err)
	}
}
