// Testes do conector LinkedIn com httptest mockando a API v2 (sem rede real).
package linkedin

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
	c := New(Config{ClientID: "cli", ClientSecret: "sec", RedirectURI: "http://localhost:8080/v1/connect/linkedin/callback"})
	c.apiBase = srv.URL
	c.tokenURL = srv.URL + "/token"
	c.authBase = srv.URL + "/authorize"
	return c
}

func TestPublishUsesRestLiHeadersAndAuthor(t *testing.T) {
	var gotRestli, gotVersion string
	var gotAuthor string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/ugcPosts" {
			t.Errorf("path = %q", r.URL.Path)
		}
		gotRestli = r.Header.Get("X-RestLi-Protocol-Version")
		gotVersion = r.Header.Get("LinkedIn-Version")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotAuthor, _ = body["author"].(string)
		_, _ = w.Write([]byte(`{"id":"urn:li:share:67890"}`))
	})

	res, err := c.Publish(context.Background(), "olá", &domain.PostTarget{}, types.Credentials{
		AccessToken: "tok", ExternalID: "urn:li:person:123",
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if gotRestli != "2.0.0" {
		t.Errorf("X-RestLi-Protocol-Version = %q (obrigatório 2.0.0 — insumo §6.2)", gotRestli)
	}
	if gotVersion == "" {
		t.Error("LinkedIn-Version ausente (versionamento por header)")
	}
	if gotAuthor != "urn:li:person:123" {
		t.Errorf("author = %q", gotAuthor)
	}
	if res.ExternalID != "urn:li:share:67890" {
		t.Errorf("ExternalID = %q", res.ExternalID)
	}
}

func TestPublishNoAuthor(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("não deveria publicar sem author URN")
	})
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{}, types.Credentials{AccessToken: "t"})
	if err == nil || !strings.Contains(err.Error(), "no_author") {
		t.Errorf("esperava no_author, veio %v", err)
	}
}

func TestValidateAccountFetchesURN(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/userinfo" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("LinkedIn-Version"); got == "" {
			t.Error("userinfo sem LinkedIn-Version")
		}
		_, _ = w.Write([]byte(`{"sub":"urn:li:person:123"}`))
	})
	if err := c.ValidateAccount(context.Background(), types.Credentials{AccessToken: "t"}); err != nil {
		t.Fatalf("ValidateAccount: %v", err)
	}
}

func TestExchangeCode(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_, _ = w.Write([]byte(`{"access_token":"at","refresh_token":"rt","expires_in":5184000,"scope":"w_member_social"}`))
		case "/v2/userinfo":
			_, _ = w.Write([]byte(`{"sub":"urn:li:person:123"}`))
		default:
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
	})

	tok, err := c.ExchangeCode(context.Background(), "code9", "", "https://cb")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if tok.ExternalID != "urn:li:person:123" {
		t.Errorf("ExternalID = %q (esperava URN do userinfo)", tok.ExternalID)
	}
	if tok.AccessToken != "at" {
		t.Errorf("AccessToken = %q", tok.AccessToken)
	}
}
