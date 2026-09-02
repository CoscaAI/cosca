// Testes do conector Bluesky com httptest mockando o AT Protocol (sem rede real).
package bluesky

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
	c := New(Config{})
	c.apiBase = srv.URL
	return c
}

func TestPublish(t *testing.T) {
	var gotSessionAuth, gotRecordBody string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.createSession":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["identifier"] != "fulano.bsky.social" || body["password"] != "app-pass" {
				t.Errorf("session body = %v", body)
			}
			_, _ = w.Write([]byte(`{"accessJwt":"jwt1","did":"did:plc:abc123"}`))
		case "/xrpc/com.atproto.repo.createRecord":
			gotSessionAuth = r.Header.Get("Authorization")
			var record map[string]any
			_ = json.NewDecoder(r.Body).Decode(&record)
			gotRecordBody = record["collection"].(string)
			rec, _ := record["record"].(map[string]any)
			if rec["text"] != "olá mundo" {
				t.Errorf("record.text = %v", rec["text"])
			}
			_, _ = w.Write([]byte(`{"uri":"at://did:plc:abc123/app.bsky.feed.post/3k5x","cid":"bafy"}`))
		default:
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
	})

	res, err := c.Publish(context.Background(), "olá mundo", &domain.PostTarget{},
		types.Credentials{AccessToken: "app-pass", ExternalID: "fulano.bsky.social"})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if gotSessionAuth != "Bearer jwt1" {
		t.Errorf("Authorization do createRecord = %q", gotSessionAuth)
	}
	if gotRecordBody != "app.bsky.feed.post" {
		t.Errorf("collection = %q", gotRecordBody)
	}
	if !strings.Contains(res.PublishedURL, "fulano.bsky.social") {
		t.Errorf("PublishedURL = %q", res.PublishedURL)
	}
}

func TestPublishNoHandle(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("não deveria chamar a API sem handle")
	})
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{}, types.Credentials{AccessToken: "p"})
	if err == nil || !strings.Contains(err.Error(), "no_credentials") {
		t.Errorf("esperava no_credentials, veio %v", err)
	}
}

func TestPublishInvalidCredentials(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"AuthenticationRequired","message":"invalid token"}`))
	})
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{}, types.Credentials{AccessToken: "bad", ExternalID: "fulano.bsky.social"})
	if err == nil {
		t.Fatal("credenciais inválidas deveriam falhar")
	}
}

func TestValidateAccount(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/com.atproto.server.createSession" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"accessJwt":"jwt","did":"did:plc:x"}`))
	})
	err := c.ValidateAccount(context.Background(), types.Credentials{AccessToken: "p", ExternalID: "fulano.bsky.social"})
	if err != nil {
		t.Fatalf("ValidateAccount: %v", err)
	}
}
