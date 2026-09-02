// Testes do conector Telegram com httptest mockando a Bot API (sem rede real).
package telegram

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
	c := New(Config{ParseMode: "HTML"})
	c.apiBase = srv.URL
	return c
}

func TestPublish(t *testing.T) {
	var gotPath, gotChatID string
	var gotText string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotChatID, _ = body["chat_id"].(string)
		gotText, _ = body["text"].(string)
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":42}}`))
	})

	res, err := c.Publish(context.Background(), "olá", &domain.PostTarget{}, types.Credentials{
		AccessToken: "12345:AAsecret", ExternalID: "-1001234567890",
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if gotPath != "/bot12345:AAsecret/sendMessage" {
		t.Errorf("path = %q (token deve ir no path)", gotPath)
	}
	if gotChatID != "-1001234567890" {
		t.Errorf("chat_id = %q", gotChatID)
	}
	if gotText != "olá" {
		t.Errorf("text = %q", gotText)
	}
	if res.ExternalID != "42" {
		t.Errorf("ExternalID = %q", res.ExternalID)
	}
}

func TestPublishChatNotFound(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"description":"Bad Request: chat not found"}`))
	})
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{}, types.Credentials{AccessToken: "t", ExternalID: "99"})
	pe, ok := err.(*types.Error)
	if !ok {
		t.Fatalf("erro não é *types.Error: %T", err)
	}
	if pe.Code != "chat_not_found" {
		t.Errorf("code = %q (esperava chat_not_found)", pe.Code)
	}
	if pe.Retryable {
		t.Error("chat_not_found não é retryable")
	}
}

func TestPublishNoCredentials(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("não deveria chamar a API sem token")
	})
	_, err := c.Publish(context.Background(), "x", &domain.PostTarget{}, types.Credentials{})
	if err == nil || !strings.Contains(err.Error(), "no_credentials") {
		t.Errorf("esperava no_credentials, veio %v", err)
	}
}

func TestValidateAccountGetMe(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bot12345:AAsecret/getMe" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"id":12345,"username":"RizomaiBot"}}`))
	})
	if err := c.ValidateAccount(context.Background(), types.Credentials{AccessToken: "12345:AAsecret"}); err != nil {
		t.Fatalf("ValidateAccount: %v", err)
	}
}

func TestValidateAccountRejected(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"description":"Unauthorized"}`))
	})
	err := c.ValidateAccount(context.Background(), types.Credentials{AccessToken: "bad"})
	if err == nil {
		t.Fatal("token inválido deveria falhar")
	}
}
