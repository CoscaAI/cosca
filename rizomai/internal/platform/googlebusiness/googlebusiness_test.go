// Testes do conector Google Business com httptest mockando a My Business API.
package googlebusiness

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

func TestPublishLocalPostWithCTA(t *testing.T) {
	var gotSummary, gotCTA string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v4/accounts/act1/locations/loc1/localPosts" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotSummary, _ = body["summary"].(string)
		cta, _ := body["callToAction"].(map[string]any)
		gotCTA, _ = cta["actionType"].(string)
		_, _ = w.Write([]byte(`{"name":"accounts/act1/locations/loc1/localPosts/12345"}`))
	})

	target := &domain.PostTarget{
		Platform: domain.PlatformGoogleBusiness,
		PlatformSpecificData: map[string]any{
			"callToAction": map[string]any{"actionType": "LEARN_MORE", "url": "https://site.com"},
		},
	}
	res, err := c.Publish(context.Background(), "Promoção!", target,
		types.Credentials{AccessToken: "tok", ExternalID: "accounts/act1/locations/loc1"})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if gotSummary != "Promoção!" {
		t.Errorf("summary = %q", gotSummary)
	}
	if gotCTA != "LEARN_MORE" {
		t.Errorf("callToAction = %q", gotCTA)
	}
	if !strings.Contains(res.ExternalID, "12345") {
		t.Errorf("ExternalID = %q", res.ExternalID)
	}
}

func TestPublishNoLocation(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("não deveria chamar a API sem location")
	})
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{}, types.Credentials{AccessToken: "t"})
	if err == nil || !strings.Contains(err.Error(), "no_location") {
		t.Errorf("esperava no_location, veio %v", err)
	}
}

func TestPublishRateLimitedRetryable(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":429,"message":"rate"}}`))
	})
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{},
		types.Credentials{AccessToken: "t", ExternalID: "accounts/a/locations/l"})
	pe, ok := err.(*types.Error)
	if !ok || !pe.Retryable {
		t.Fatalf("429 deveria ser retryable, veio %v", err)
	}
}

func TestPublishNoCredentials(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("não deveria chamar a API sem token")
	})
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{}, types.Credentials{ExternalID: "a/l"})
	if err == nil || !strings.Contains(err.Error(), "no_credentials") {
		t.Errorf("esperava no_credentials, veio %v", err)
	}
}

func TestPublishNotConfigured(t *testing.T) {
	c := New(Config{})
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{},
		types.Credentials{AccessToken: "t", ExternalID: "a/l"})
	if err == nil || !strings.Contains(err.Error(), "not_configured") {
		t.Errorf("esperava not_configured, veio %v", err)
	}
}

func TestValidateAccount(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v4/accounts" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"accounts":[{"name":"accounts/act1"}]}`))
	})
	if err := c.ValidateAccount(context.Background(), types.Credentials{AccessToken: "t"}); err != nil {
		t.Fatalf("ValidateAccount: %v", err)
	}
}
