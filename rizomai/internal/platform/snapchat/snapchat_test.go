// Testes do conector Snapchat com httptest mockando a Story API (sem rede real).
package snapchat

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
	c.tokenURL = srv.URL + "/token"
	c.authBase = srv.URL + "/auth"
	return c
}

func mediaTarget() *domain.PostTarget {
	return &domain.PostTarget{
		Platform: domain.PlatformSnapchat,
		PlatformSpecificData: map[string]any{
			"mediaUrls": []any{"https://cdn.rizomai.app/story.mp4"},
		},
	}
}

func TestPublishStory(t *testing.T) {
	var gotBody map[string]any
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/stories" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":"story123"}`))
	})

	res, err := c.Publish(context.Background(), "olá", mediaTarget(), types.Credentials{AccessToken: "tok"})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	media, _ := gotBody["media"].(map[string]any)
	if media["url"] != "https://cdn.rizomai.app/story.mp4" {
		t.Errorf("media = %v", gotBody["media"])
	}
	if res.ExternalID != "story123" {
		t.Errorf("ExternalID = %q", res.ExternalID)
	}
}

func TestPublishNoMedia(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("não deveria chamar a API sem mídia")
	})
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{}, types.Credentials{AccessToken: "t"})
	if err == nil || !strings.Contains(err.Error(), "no_media") {
		t.Errorf("esperava no_media, veio %v", err)
	}
}

func TestPublishRateLimitedRetryable(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	_, err := c.Publish(context.Background(), "x", mediaTarget(), types.Credentials{AccessToken: "t"})
	pe, ok := err.(*types.Error)
	if !ok || !pe.Retryable {
		t.Fatalf("502 deveria ser retryable, veio %v", err)
	}
}

func TestPublishNoCredentials(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("não deveria chamar a API sem token")
	})
	_, err := c.Publish(context.Background(), "x", mediaTarget(), types.Credentials{})
	if err == nil || !strings.Contains(err.Error(), "no_credentials") {
		t.Errorf("esperava no_credentials, veio %v", err)
	}
}

func TestPublishNotConfigured(t *testing.T) {
	c := New(Config{})
	_, err := c.Publish(context.Background(), "x", mediaTarget(), types.Credentials{AccessToken: "t"})
	if err == nil || !strings.Contains(err.Error(), "not_configured") {
		t.Errorf("esperava not_configured, veio %v", err)
	}
}

func TestValidateAccount(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/me" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"123"}`))
	})
	if err := c.ValidateAccount(context.Background(), types.Credentials{AccessToken: "t"}); err != nil {
		t.Fatalf("ValidateAccount: %v", err)
	}
}
