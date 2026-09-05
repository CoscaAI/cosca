// Package e2e — ETAPA 5, PROVA E: OpenCode (MCP stdio) → Kernel.
//
// O professor pede provar que o `cosca-mcp` (JSON-RPC 2.0/stdio) `tools/call`
// chega a uma ferramenta COGNITIVA kernel-first (determinística, zero-LLM).
// A ferramenta `cosca.reason` é a mais direta: dado uma `sequence` (sem trace
// store), ela devolve um ContextPacket com a sequência submetida + detecção de
// divergência determinística — NUNCA um LLM como autoridade. Isto é o MESMO
// padrão de teste do pacote internal/mcpserver (server_test.go, reusado).
//
// NOTA HONESTA (GAP): a superfície MCP atual expõe tools cognitivas de LEITURA
// (recall/context/reason/trace/project/cost/self...) e um cosca.cli restrito a
// allowlist. NÃO existe uma tool `resolve`/`run` que exponha o endpoint
// /v1/run do Kernel (nem o `run` está na allowlist do cosca.cli). Portanto o
// "OpenCode → Kernel /v1/run" descrito no mapa DA FASE 2 ainda NÃO é uma
// capacidade MCP de primeira classe — isso é um limite que registramos aqui,
// não uma implementação duplicada deste teste (ver o relatório ETAPA 5).
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/mcpserver"
)

// runMCPServer sends NDJSON requests to a cosca-mcp stdio server and returns the
// output (reuses internal/mcpserver.NewServer + Serve — no duplicate server).
func runMCPServer(t *testing.T, engine *mcpserver.Engine, lines ...string) string {
	t.Helper()
	in := bytes.NewBufferString(strings.Join(lines, "\n"))
	out := &bytes.Buffer{}
	srv := mcpserver.NewServer(engine, in, out)
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("mcp server Serve: %v", err)
	}
	return out.String()
}

// TestE2E_MCP_ToolsCall_KernelFirstDeterministic proves that tools/call reaches
// the deterministic kernel-first tool cosca.reason (zero-LLM): a submitted
// sequence yields a context packet with the sequence + no LLM as authority.
func TestE2E_MCP_ToolsCall_KernelFirstDeterministic(t *testing.T) {
	// Engine SEM dependências (nil-safe): a tool cosca.reason precisa apenas da
	// `sequence` — nenhum órgão (knowledge/memory/trace) é necessário.
	eng := mcpserver.NewEngine()

	out := runMCPServer(t, eng,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"cosca.reason","arguments":{"sequence":["A","B","C"]}}}`)

	var resp map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("unmarshal mcp response: %v", err)
	}
	if resp["error"] != nil {
		t.Fatalf("cosca.reason returned JSON-RPC error: %s", resp["error"])
	}
	var result struct {
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(resp["result"], &result); err != nil {
		t.Fatalf("unmarshal mcp result: %v", err)
	}
	if result.IsError {
		t.Fatal("cosca.reason should not report isError for a valid sequence")
	}

	// O conteúdo é um ContextPacket (epistemológico, honesto) — decodifica-o.
	var callResult mcpserver.CallResult
	if err := json.Unmarshal(resp["result"], &callResult); err != nil {
		t.Fatalf("unmarshal CallResult: %v", err)
	}
	if len(callResult.Content) == 0 {
		t.Fatal("expected content items in the tool call result")
	}
	var packet mcpserver.ContextPacket
	if err := json.Unmarshal([]byte(callResult.Content[0].Text), &packet); err != nil {
		t.Fatalf("decode context packet: %v", err)
	}
	if packet.Query == "" {
		t.Error("expected a populated packet query")
	}
	if len(packet.Context) == 0 {
		t.Error("expected the submitted sequence as a context item")
	}
}

// TestE2E_MCP_UnknownTool_DefaultDeny proves the MCP layer is default-deny: an
// unknown tool returns a JSON-RPC error (I8), never a fabricated "run" tool.
func TestE2E_MCP_UnknownTool_DefaultDeny(t *testing.T) {
	eng := mcpserver.NewEngine()
	out := runMCPServer(t, eng,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"cosca.run","arguments":{}}}`)
	var resp map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["error"] == nil {
		t.Fatal("unknown tool 'cosca.run' should return a JSON-RPC error (default-deny)")
	}
	// A falha é NAQUELA tool (não fabricada); a message varia entre
	// "não encontrada"/"desconhecida" conforme a versão — buscamos o nome.
	if !strings.Contains(string(resp["error"]), "cosca.run") {
		t.Fatalf("unexpected error message: %s", resp["error"])
	}
}
