package orchestration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
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

	result, executeErr := (&ToolExecutor{}).Execute(context.Background(), chat.ToolCall{Function: chat.FunctionCall{Arguments: "{not-json"}})
	if executeErr != nil {
		t.Fatalf("tool execution returned unexpected error: %v", executeErr)
	}
	if result == nil || strings.Contains(result.Error, secret) || strings.Contains(result.Error, "not-json") {
		t.Fatalf("tool result exposed raw error: %+v", result)
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
