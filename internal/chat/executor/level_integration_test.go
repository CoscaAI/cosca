// Package executor — integração do sistema de NÍVEIS de capacidade (decisão do
// Don 2026-08-25): o LevelGate bloqueia ações fora da soberania do nível atual,
// ANTES do policy, independente do LLM. Enforcement por código, não por promessa.
package executor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/level"
)

func TestLevelGateBloqueiaEdicaoDoCerebroNoOperacional(t *testing.T) {
	mt := &mockTool{name: "edit", executeFunc: func(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
		return &chat.ToolResult{Output: "editado"}, nil
	}}
	reg := newMockRegistry()
	reg.tools["edit"] = mt
	ex := New(reg, nil, "/tmp")
	ex.SetLevelGate(level.NewGate(level.L2Operacional)) // L2 NÃO edita o próprio cérebro

	// Editar o cérebro (internal/embed/cosca) no L2 → DENY antes de executar.
	res, _ := ex.Execute(context.Background(), ToolCall{ID: "1", Name: "edit",
		Input: map[string]interface{}{"path": "internal/embed/cosca/KERNEL.md"}})
	if res.Status != StatusError {
		t.Fatalf("editar cerebro no L2: status=%s, esperado ERROR (level DENY)", res.Status)
	}
	if res.Error == "" || res.Error == "editado" {
		t.Fatalf("editar cerebro no L2: erro=%q, esperado mensagem de level", res.Error)
	}
}

func TestLevelGateBloqueiaEdicaoDoCerebroNoInicial(t *testing.T) {
	mt := &mockTool{name: "write", executeFunc: func(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
		return &chat.ToolResult{Output: "escrito"}, nil
	}}
	reg := newMockRegistry()
	reg.tools["write"] = mt
	ex := New(reg, nil, "/tmp")
	ex.SetLevelGate(level.NewGate(level.L1Inicial)) // L1: primeiro despertar, só leitura

	res, _ := ex.Execute(context.Background(), ToolCall{ID: "1", Name: "write",
		Input: map[string]interface{}{"path": "internal/embed/cosca/blocks/x.md"}})
	if res.Status != StatusError {
		t.Fatalf("L1 editar cerebro: status=%s, esperado ERROR", res.Status)
	}
}

func TestLevelGatePermiteOperarMaquinaNoOperacional(t *testing.T) {
	mt := &mockTool{name: "bash", executeFunc: func(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
		return &chat.ToolResult{Output: "ok"}, nil
	}}
	reg := newMockRegistry()
	reg.tools["bash"] = mt
	ex := New(reg, nil, "/tmp")
	ex.SetLevelGate(level.NewGate(level.L2Operacional))

	res, _ := ex.Execute(context.Background(), ToolCall{ID: "1", Name: "bash",
		Input: map[string]interface{}{"command": "cosca doctor"}})
	if res.Status == StatusError && res.Error != "" {
		t.Fatalf("L2 operar maquina deveria ser permitido, erro=%s", res.Error)
	}
}

func TestLevelGateSemGatePermiteQualquerCoisa(t *testing.T) {
	// Sem level gate configurado → comportamento histórico (nada bloqueado pelo nível).
	mt := &mockTool{name: "edit", executeFunc: func(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
		return &chat.ToolResult{Output: "editado"}, nil
	}}
	reg := newMockRegistry()
	reg.tools["edit"] = mt
	ws := t.TempDir()
	ex := New(reg, nil, ws)
	// NÃO chama SetLevelGate → sem gate de nível.

	target := ws + "/file.go"
	res, _ := ex.Execute(context.Background(), ToolCall{ID: "1", Name: "edit",
		Input: map[string]interface{}{"path": target}})
	if res.Status == StatusError {
		t.Fatalf("sem level gate deveria permitir (historico), erro=%s", res.Error)
	}
}
