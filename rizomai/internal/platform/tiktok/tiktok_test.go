// Testes do conector TikTok com httptest mockando a Content Posting API.
package tiktok

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
	c := New(Config{ClientKey: "ck", ClientSecret: "cs", RedirectURI: "http://localhost:8080/cb"})
	c.apiBase = srv.URL
	c.tokenURL = srv.URL + "/token"
	c.authBase = srv.URL + "/auth"
	return c
}

func mediaTarget() *domain.PostTarget {
	return &domain.PostTarget{
		Platform: domain.PlatformTikTok,
		PlatformSpecificData: map[string]any{
			"mediaUrls": []any{"https://cdn.rizomai.app/vid.mp4"},
		},
	}
}

func TestPublishInitThenPublish(t *testing.T) {
	var initPath, pubPath string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/post/publish/video/init/":
			initPath = r.URL.Path
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if got := r.Header.Get("Authorization"); got != "Bearer tok" {
				t.Errorf("Authorization = %q", got)
			}
			_, _ = w.Write([]byte(`{"data":{"publish_id":"pub123","upload_url":"https://upload.test"}}`))
		case "/v2/post/publish/video/":
			pubPath = r.URL.Path
			_, _ = w.Write([]byte(`{"data":{"publish_id":"pub123","status":"PUBLISH_COMPLETE"}}`))
		default:
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
	})

	res, err := c.Publish(context.Background(), "olá", mediaTarget(), types.Credentials{AccessToken: "tok", ExternalID: "open123"})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if initPath != "/v2/post/publish/video/init/" || pubPath != "/v2/post/publish/video/" {
		t.Errorf("caminhos: init=%q publish=%q", initPath, pubPath)
	}
	if res.ExternalID != "pub123" {
		t.Errorf("ExternalID = %q", res.ExternalID)
	}
}

func TestPublishNoMedia(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("não deveria chamar a API sem vídeo")
	})
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{}, types.Credentials{AccessToken: "t"})
	if err == nil || !strings.Contains(err.Error(), "no_media") {
		t.Errorf("esperava no_media, veio %v", err)
	}
}

func TestPublishRateLimitedRetryable(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":"rate_limit_exceeded"}}`))
	})
	_, err := c.Publish(context.Background(), "x", mediaTarget(), types.Credentials{AccessToken: "t"})
	pe, ok := err.(*types.Error)
	if !ok || !pe.Retryable {
		t.Fatalf("429 deveria ser retryable, veio %v", err)
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
		if r.URL.Path != "/v2/user/info/" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":{"open_id":"open123"}}`))
	})
	if err := c.ValidateAccount(context.Background(), types.Credentials{AccessToken: "t"}); err != nil {
		t.Fatalf("ValidateAccount: %v", err)
	}
}
