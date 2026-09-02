// Testes do conector YouTube com httptest mockando a Data API v3 (upload
// resumable em 2 passos — sem rede real).
package youtube

import (
	"context"
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
	c := New(Config{ClientID: "cli", ClientSecret: "sec", RedirectURI: "http://localhost:8080/cb"})
	c.apiBase = srv.URL
	c.tokenURL = srv.URL + "/token"
	c.authBase = srv.URL + "/auth"
	return c
}

func mediaTarget() *domain.PostTarget {
	return &domain.PostTarget{
		Platform: domain.PlatformYouTube,
		PlatformSpecificData: map[string]any{
			"mediaUrls":  []any{"http://media.test/video.mp4"},
			"title":      "Meu vídeo",
			"visibility": "unlisted",
		},
	}
}

func TestPublishResumableUpload(t *testing.T) {
	// Um único servidor cobre: metadata (init), download da mídia e o upload.
	var gotUploadBody string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/upload/youtube/v3/videos":
			// passo 1: metadata → devolve o Location do upload resumable
			w.Header().Set("Location", srv.URL+"/upload-dest")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case r.URL.Path == "/upload-dest":
			// passo 2: recebe os bytes da mídia (streaming) e devolve o vídeo
			b, _ := io.ReadAll(r.Body)
			gotUploadBody = string(b)
			_, _ = w.Write([]byte(`{"id":"video123","status":{"uploadStatus":"uploaded"}}`))
		case r.URL.Path == "/video.mp4":
			// download da mídia (origem do mediaUrls)
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte("fake-video-bytes"))
		default:
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := New(Config{ClientID: "cli", ClientSecret: "sec", RedirectURI: "http://localhost:8080/cb"})
	c.apiBase = srv.URL
	c.tokenURL = srv.URL + "/token"
	c.authBase = srv.URL + "/auth"

	target := mediaTarget()
	target.PlatformSpecificData["mediaUrls"] = []any{srv.URL + "/video.mp4"}

	res, err := c.Publish(context.Background(), "descrição", target, types.Credentials{AccessToken: "tok"})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if gotUploadBody != "fake-video-bytes" {
		t.Errorf("upload body = %q (esperava o streaming da mídia)", gotUploadBody)
	}
	if res.ExternalID != "video123" {
		t.Errorf("ExternalID = %q", res.ExternalID)
	}
	if !strings.Contains(res.PublishedURL, "video123") {
		t.Errorf("PublishedURL = %q", res.PublishedURL)
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
		if r.URL.Path != "/youtube/v3/channels" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Authorization = %q", got)
		}
		_, _ = w.Write([]byte(`{"items":[{"id":"chan1"}]}`))
	})
	if err := c.ValidateAccount(context.Background(), types.Credentials{AccessToken: "tok"}); err != nil {
		t.Fatalf("ValidateAccount: %v", err)
	}
}
