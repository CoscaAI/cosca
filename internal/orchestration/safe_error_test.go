package orchestration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/chat/executor"
	"github.com/CoscaAI/cosca/internal/chat/tool"
)

func TestSafeErrorDoesNotExposeSensitiveText(t *testing.T) {
	secret := "api-key=super-secret-prompt-value"
	err := errors.New(secret)

	info := safeError("provider_failed", err)
	if info.Code != "provider_failed" || info.Hash == "" || info.Length != len(secret) {
		t.Fatalf("unexpected safe diagnostics: %+v", info)
	}
	if got := safeErrorMessage(info.Code); strings.Contains(got, secret) {
		t.Fatalf("safe message exposed secret: %q", got)
	}

	stage := NewStageError("executor", err, time.Second)
	if strings.Contains(stage.Error, secret) || strings.Contains(stage.ErrorCode, secret) {
		t.Fatalf("stage result exposed secret: %+v", stage)
	}

	// O executor canônico (via adapter) mascara erros de parsing de
	// argumentos — nunca expõe o texto bruto.
	adapter := newExecutorAdapter(executor.New(tool.NewRegistry(), nil, t.TempDir()))
	results, execErr := adapter.ExecuteAll(context.Background(), []chat.ToolCall{
		{ID: "t1", Function: chat.FunctionCall{Name: "read", Arguments: "{not-json"}},
	})
	if execErr != nil {
		t.Fatalf("tool execution returned unexpected error: %v", execErr)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 masked result, got %d", len(results))
	}
	if strings.Contains(results[0].Error, "not-json") || strings.Contains(results[0].Error, "{") {
		t.Fatalf("tool result exposed raw args: %+v", results[0])
	}
	if results[0].ErrorCode == "" {
		t.Fatal("expected error code on masked result")
	}
}

func TestPipelineFailureDoesNotExposeProcessorError(t *testing.T) {
	secret := "/workspace/private/prompt=super-secret"
	p := NewPipeline(nil)
	p.RegisterProcessor("failing", func(context.Context, string, map[string]interface{}) (string, error) {
		return "", errors.New(secret)
	})

	_, err := p.ExecuteStep(context.Background(), NewPipelineContext("request", "input"), PipelineStep{
		Name: "private-step", Skill: "failing",
	})
	if err == nil {
		t.Fatal("expected pipeline failure")
	}
	if strings.Contains(err.Error(), secret) || !strings.Contains(err.Error(), "pipeline step") {
		t.Fatalf("unexpected public pipeline error: %v", err)
	}
}
