// Testes do conector Instagram com httptest mockando a Graph API (sem rede real).
package instagram

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

func targetWithMedia() *domain.PostTarget {
	return &domain.PostTarget{
		Platform: domain.PlatformInstagram,
		PlatformSpecificData: map[string]any{
			"mediaUrls": []any{"https://cdn.rizomai.app/img1.jpg"},
		},
	}
}

func TestPublish(t *testing.T) {
	var gotCreatePath, gotPublishPath string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/123456789/media":
			gotCreatePath = r.URL.Path
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["image_url"] != "https://cdn.rizomai.app/img1.jpg" || body["caption"] != "olá" {
				t.Errorf("body media = %v", body)
			}
			_, _ = w.Write([]byte(`{"id":"17895667968018981"}`))
		case "/123456789/media_publish":
			gotPublishPath = r.URL.Path
			_, _ = w.Write([]byte(`{"id":"17918486968058939"}`))
		default:
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
	})

	res, err := c.Publish(context.Background(), "olá", targetWithMedia(),
		types.Credentials{AccessToken: "tok", ExternalID: "123456789"})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if gotCreatePath != "/123456789/media" || gotPublishPath != "/123456789/media_publish" {
		t.Errorf("caminhos: create=%q publish=%q", gotCreatePath, gotPublishPath)
	}
	if res.ExternalID != "17918486968058939" {
		t.Errorf("ExternalID = %q", res.ExternalID)
	}
}

func TestPublishRateLimitedRetryable(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":900,"message":"rate limit"}}`))
	})
	_, err := c.Publish(context.Background(), "x", targetWithMedia(), types.Credentials{AccessToken: "t", ExternalID: "1"})
	pe, ok := err.(*types.Error)
	if !ok || !pe.Retryable {
		t.Fatalf("429 deveria ser retryable, veio %v", err)
	}
}

func TestPublishAntiSpam2207051DoesNotDuplicate(t *testing.T) {
	// media container criado; media_publish falha com 2207051 ("pode ter
	// publicado") → o conector VERIFICA se existe mídia recente antes de falhar.
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/123456789/media":
			if r.Method == http.MethodGet {
				// verificação pós-erro: existe mídia recente → já publicou
				_, _ = w.Write([]byte(`{"data":[{"id":"1791"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"1789"}`))
		case "/123456789/media_publish":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"code":2207051,"message":"Media upload failed"}}`))
		default:
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
	})

	res, err := c.Publish(context.Background(), "x", targetWithMedia(), types.Credentials{AccessToken: "t", ExternalID: "123456789"})
	if err != nil {
		t.Fatalf("com mídia existente após 2207051 o Publish deveria "+
			"considerar sucesso (anti-duplicado), veio %v", err)
	}
	if res == nil || res.ExternalID == "" {
		t.Error("resultado esperado após verificação pós-erro")
	}
}

func TestPublishNoCredentials(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("não deveria chamar a API sem token")
	})
	_, err := c.Publish(context.Background(), "x", targetWithMedia(), types.Credentials{})
	if err == nil || !strings.Contains(err.Error(), "no_credentials") {
		t.Errorf("esperava no_credentials, veio %v", err)
	}
}

func TestPublishNoMedia(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("não deveria chamar a API sem mídia")
	})
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{}, types.Credentials{AccessToken: "t", ExternalID: "1"})
	if err == nil || !strings.Contains(err.Error(), "no_media") {
		t.Errorf("esperava no_media, veio %v", err)
	}
}

func TestPublishNotConfigured(t *testing.T) {
	c := New(Config{})
	_, err := c.Publish(context.Background(), "x", targetWithMedia(), types.Credentials{AccessToken: "t", ExternalID: "1"})
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
