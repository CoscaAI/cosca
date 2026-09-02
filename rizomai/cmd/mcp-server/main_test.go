// Smoke test do handshake MCP (JSON-RPC sobre stdio) — sem rede real.
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func msg(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func TestInitializeHandshake(t *testing.T) {
	s := newServer("http://api.invalid", "sk_test")
	resp := s.handle(msg(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))

	res, _ := resp["result"].(map[string]any)
	if res == nil {
		t.Fatalf("initialize sem result: %v", resp)
	}
	info, _ := res["serverInfo"].(map[string]any)
	if info["name"] != "rizomai-mcp" {
		t.Errorf("serverInfo = %v", info)
	}
	if res["protocolVersion"] == "" {
		t.Error("protocolVersion ausente")
	}
}

func TestInitializedNotificationNoResponse(t *testing.T) {
	s := newServer("http://api.invalid", "sk_test")
	if resp := s.handle(msg(t, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)); resp != nil {
		t.Errorf("notificação não deveria ter resposta: %v", resp)
	}
}

func TestToolsList(t *testing.T) {
	s := newServer("http://api.invalid", "sk_test")
	resp := s.handle(msg(t, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))

	res, _ := resp["result"].(map[string]any)
	list, _ := res["tools"].([]map[string]any)
	if len(list) != 6 {
		t.Fatalf("esperava 6 tools, veio %d: %v", len(list), list)
	}

	names := map[string]bool{}
	for _, tl := range list {
		names[tl["name"].(string)] = true
	}
	for _, want := range []string{"list_profiles", "create_post", "list_posts", "get_post", "connect_account", "get_usage"} {
		if !names[want] {
			t.Errorf("tool %s ausente", want)
		}
	}
}

func TestToolsCallAPIUnavailable(t *testing.T) {
	s := newServer("http://api.invalid", "sk_test")
	resp := s.handle(msg(t, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_profiles","arguments":{}}}`))

	res, _ := resp["result"].(map[string]any)
	content, _ := res["content"].([]map[string]any)
	isErr, _ := res["isError"].(bool)
	if !isErr {
		t.Errorf("API indisponível deveria marcar isError=true: %v", resp)
	}
	if len(content) == 0 || !strings.Contains(content[0]["text"].(string), "API indisponível") {
		t.Errorf("mensagem de erro clara ausente: %v", content)
	}
}

func TestToolsCallUnknown(t *testing.T) {
	s := newServer("http://api.invalid", "sk_test")
	resp := s.handle(msg(t, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"hack","arguments":{}}}`))
	if _, hasErr := resp["error"]; !hasErr {
		t.Errorf("tool desconhecida deveria retornar erro JSON-RPC: %v", resp)
	}
}

func TestGetPostRequiresID(t *testing.T) {
	s := newServer("http://api.invalid", "sk_test")
	resp := s.handle(msg(t, `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"get_post","arguments":{}}}`))
	res, _ := resp["result"].(map[string]any)
	if isErr, _ := res["isError"].(bool); !isErr {
		t.Errorf("get_post sem id deveria ser erro: %v", resp)
	}
}
