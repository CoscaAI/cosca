// Testes do conector Pinterest com httptest mockando a API v5 (sem rede real).
package pinterest

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
		Platform: domain.PlatformPinterest,
		PlatformSpecificData: map[string]any{
			"boardId":   "123456789",
			"title":     "Pin legal",
			"mediaUrls": []any{"https://cdn.rizomai.app/img.jpg"},
		},
	}
}

func TestPublishPin(t *testing.T) {
	var gotBoard string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pins" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotBoard, _ = body["board_id"].(string)
		_, _ = w.Write([]byte(`{"id":"pin987654","link":"https://pinterest.com/pin/987654/"}`))
	})

	res, err := c.Publish(context.Background(), "descrição", mediaTarget(), types.Credentials{AccessToken: "tok"})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if gotBoard != "123456789" {
		t.Errorf("board_id = %q", gotBoard)
	}
	if res.ExternalID != "pin987654" {
		t.Errorf("ExternalID = %q", res.ExternalID)
	}
}

func TestPublishNoBoard(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("não deveria chamar a API sem boardId")
	})
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{}, types.Credentials{AccessToken: "t"})
	if err == nil || !strings.Contains(err.Error(), "no_board") {
		t.Errorf("esperava no_board, veio %v", err)
	}
}

func TestPublishNoMedia(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("não deveria chamar a API sem imagem")
	})
	target := &domain.PostTarget{Platform: domain.PlatformPinterest, PlatformSpecificData: map[string]any{"boardId": "1"}}
	_, err := c.Publish(context.Background(), "x", target, types.Credentials{AccessToken: "t"})
	if err == nil || !strings.Contains(err.Error(), "no_media") {
		t.Errorf("esperava no_media, veio %v", err)
	}
}

func TestPublishRateLimitedRetryable(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message":"Rate limit exceeded"}`))
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
		if r.URL.Path != "/user_account" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"username":"fulano","id":"123"}`))
	})
	if err := c.ValidateAccount(context.Background(), types.Credentials{AccessToken: "t"}); err != nil {
		t.Fatalf("ValidateAccount: %v", err)
	}
}
