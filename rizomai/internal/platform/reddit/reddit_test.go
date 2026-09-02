// Testes do conector Reddit com httptest mockando a API v1 (sem rede real).
package reddit

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
	c := New(Config{ClientID: "cid", ClientSecret: "csec", Username: "user", Password: "pass", UserAgent: "rizomai-test/1.0"})
	c.apiBase = srv.URL
	c.authURL = srv.URL + "/api/v1/access_token"
	return c
}

func targetWithSubreddit() *domain.PostTarget {
	return &domain.PostTarget{
		Platform: domain.PlatformReddit,
		PlatformSpecificData: map[string]any{
			"subreddit": "tecnologia",
			"title":     "Post de teste",
		},
	}
}

func TestPublishSelfPost(t *testing.T) {
	var gotTokenForm, gotUA string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/access_token":
			b, _ := io.ReadAll(r.Body)
			gotTokenForm = string(b)
			_, _ = w.Write([]byte(`{"access_token":"reddit-token"}`))
		case "/api/submit":
			gotUA = r.Header.Get("User-Agent")
			if got := r.Header.Get("Authorization"); got != "Bearer reddit-token" {
				t.Errorf("Authorization = %q", got)
			}
			_, _ = w.Write([]byte(`{"json":{"errors":[],"data":{"id":"abc123"}}}`))
		default:
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
	})

	res, err := c.Publish(context.Background(), "conteúdo do post", targetWithSubreddit(), types.Credentials{})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if !strings.Contains(gotTokenForm, "grant_type=password") || !strings.Contains(gotTokenForm, "username=user") {
		t.Errorf("token form = %q", gotTokenForm)
	}
	if gotUA == "" {
		t.Error("User-Agent descritivo obrigatório ausente (insumo §4.4)")
	}
	if !strings.Contains(gotUA, "rizomai") {
		t.Errorf("User-Agent deveria ser descritivo: %q", gotUA)
	}
	if res.ExternalID != "abc123" {
		t.Errorf("ExternalID = %q", res.ExternalID)
	}
}

func TestPublishRateLimitErrorIsRetryable(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/access_token":
			_, _ = w.Write([]byte(`{"access_token":"t"}`))
		case "/api/submit":
			_, _ = w.Write([]byte(`{"json":{"errors":[["ratelimit","looks like you've been doing that a lot"]]}}`))
		default:
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
	})
	_, err := c.Publish(context.Background(), "x", targetWithSubreddit(), types.Credentials{})
	pe, ok := err.(*types.Error)
	if !ok || !pe.Retryable {
		t.Fatalf("ratelimit deveria ser retryable, veio %v", err)
	}
	if pe.Code != "ratelimit" {
		t.Errorf("code = %q", pe.Code)
	}
}

func TestPublishNoSubreddit(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("não deveria chamar a API sem subreddit")
	})
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{}, types.Credentials{})
	if err == nil || !strings.Contains(err.Error(), "no_subreddit") {
		t.Errorf("esperava no_subreddit, veio %v", err)
	}
}

func TestPublishNotConfigured(t *testing.T) {
	c := New(Config{})
	_, err := c.Publish(context.Background(), "x", targetWithSubreddit(), types.Credentials{})
	if err == nil || !strings.Contains(err.Error(), "not_configured") {
		t.Errorf("esperava not_configured, veio %v", err)
	}
}

func TestValidateAccount(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/access_token":
			_, _ = w.Write([]byte(`{"access_token":"t"}`))
		case "/api/v1/me":
			_, _ = w.Write([]byte(`{"name":"user","id":"x"}`))
		default:
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
	})
	if err := c.ValidateAccount(context.Background(), types.Credentials{}); err != nil {
		t.Fatalf("ValidateAccount: %v", err)
	}
}
