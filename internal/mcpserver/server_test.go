// server_test.go — Testes do PROTOCOLO MCP do COSCA (JSON-RPC 2.0 stdio).
//
// Cobre os 5 casos da ADR-028:
//   (a) initialize responde protocolVersion/serverInfo
//   (b) tools/list lista as 7 tools cognitivas
//   (c) tools/call cosca.recall com query válida → packet bem-formado
//   (d) tool inexistente → erro JSON-RPC
//   (e) servidor sem engine → erro JSON-RPC limpo (nil-safe)
package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// runServer envia um NDJSON de requisições para o servidor e devolve a saída.
func runServer(t *testing.T, engine *Engine, lines ...string) string {
	t.Helper()
	in := bytes.NewBufferString(strings.Join(lines, "\n"))
	out := &bytes.Buffer{}
	srv := NewServer(engine, in, out)
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	return out.String()
}

func TestInitialize_Handshake(t *testing.T) {
	out := runServer(t, NewEngine(),
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`)
	var resp map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("unmarshal resp: %v", err)
	}
	var result struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name string `json:"name"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(resp["result"], &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.ProtocolVersion != "2024-11-05" {
		t.Fatalf("protocolVersion = %q, esperava 2024-11-05", result.ProtocolVersion)
	}
	if result.ServerInfo.Name != "cosca-mcp" {
		t.Fatalf("serverInfo.name = %q", result.ServerInfo.Name)
	}
}

func TestInitialize_EchoesClientVersion(t *testing.T) {
	// O servidor EC OA a versão que o cliente pediu (não impõe a sua).
	out := runServer(t, NewEngine(),
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`)
	var resp map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var result struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := json.Unmarshal(resp["result"], &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.ProtocolVersion != "2025-06-18" {
		t.Fatalf("protocolVersion = %q, esperava 2025-06-18 (eco)", result.ProtocolVersion)
	}
}

func TestToolsList_SevenCognitiveTools(t *testing.T) {	out := runServer(t, NewEngine(),
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	var resp map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var result toolsListResult
	if err := json.Unmarshal(resp["result"], &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Tools) != 11 {
		t.Fatalf("tools/list = %d, esperava 11", len(result.Tools))
	}
	if result.Tools[0].Name != ToolRecall {
		t.Fatalf("tools[0].name = %q, esperava compact field", result.Tools[0].Name)
	}
}

func TestToolCall_RecallPacket(t *testing.T) {
	eng := NewEngine(WithKnowledge(newTestKnowledgeEngine(t)))
	out := runServer(t, eng,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"cosca.recall","arguments":{"query":"provenance","limit":5}}}`)
	var resp map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["error"] != nil {
		t.Fatalf("recall retornou erro: %s", resp["error"])
	}
	var result toolCallResult
	if err := json.Unmarshal(resp["result"], &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.IsError {
		t.Fatalf("recall isError: %v", result.Content)
	}
	var packet ContextPacket
	if err := json.Unmarshal([]byte(result.Content[0].Text), &packet); err != nil {
		t.Fatalf("decodificar packet: %v", err)
	}
	if packet.Query != "provenance" {
		t.Fatalf("packet.query = %q", packet.Query)
	}
	if len(packet.Context) == 0 {
		t.Fatal("esperava context items")
	}
}

func TestToolCall_UnknownToolError(t *testing.T) {
	out := runServer(t, NewEngine(),
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"cosca.nope","arguments":{}}}`)
	var resp map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["error"] == nil {
		t.Fatal("tool desconhecida deveria retornar erro")
	}
}

func TestToolCall_NoEngineNilSafe(t *testing.T) {
	// Engine com dependências nulas: o servidor sobe, mas ferramentas que
	// precisam de órgão retornam erro limpo (nunca pânico).
	out := runServer(t, NewEngine(),
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"cosca.recall","arguments":{"query":"x"}}}`)
	var resp map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["error"] == nil {
		t.Fatal("recall sem knowledge engine deveria retornar erro (nil-safe)")
	}
	if !strings.Contains(string(resp["error"]), "indisponível") {
		t.Fatalf("mensagem de erro inesperada: %s", resp["error"])
	}
}


