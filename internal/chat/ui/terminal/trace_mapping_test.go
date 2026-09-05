package terminal

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/pipeline"
)

func TestMapPipelineEventRunTypes(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{string(pipeline.EventContent), ActionContent},
		{string(pipeline.EventToolStart), ActionToolStarted},
		{string(pipeline.EventToolResult), ActionToolResult},
		{string(pipeline.EventBuildStart), ActionBuildStarted},
		{string(pipeline.EventBuildEnd), ActionBuildEnd},
		{string(pipeline.EventTestStart), ActionTestStarted},
		{string(pipeline.EventTestEnd), ActionTestEnd},
		{string(pipeline.EventError), ActionExecutionFailed},
		{string(pipeline.EventDone), ActionExecutionDone},
	}

	for _, c := range cases {
		ev := MapPipelineEvent(c.in, "kernel", "detail")
		if ev.Action != c.want {
			t.Errorf("MapPipelineEvent(%q).Action = %q, want %q", c.in, ev.Action, c.want)
		}
	}
}

func TestMapPipelineEventHighLevel(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"PlanCreated", ActionPlanCreated},
		{"StepStarted", ActionTaskStarted},
		{"ToolStarted", ActionToolStarted},
		{"BuildStarted", ActionBuildStarted},
		{"TestStarted", ActionTestStarted},
		{"Done", ActionExecutionDone},
		{"Error", ActionExecutionFailed},
	}

	for _, c := range cases {
		ev := MapPipelineEvent(c.in, "agent-x", "task detail")
		if ev.Action != c.want {
			t.Errorf("MapPipelineEvent(%q).Action = %q, want %q", c.in, ev.Action, c.want)
		}
	}
}

func TestMapPipelineEventResultAndActor(t *testing.T) {
	ev := MapPipelineEvent(string(pipeline.EventDone), "", "summary")
	if ev.Result != "success" {
		t.Fatalf("done event Result = %q, want success", ev.Result)
	}
	if ev.Actor != "kernel" {
		t.Fatalf("empty actor should default to kernel, got %q", ev.Actor)
	}
	if ev.Details != "summary" {
		t.Fatalf("details = %q, want summary", ev.Details)
	}
	if ev.TraceID == "" {
		t.Fatalf("mapped event must carry a TraceID")
	}

	fe := MapPipelineEvent(string(pipeline.EventError), "agent-x", "boom")
	if fe.Result != "failed" {
		t.Fatalf("error event Result = %q, want failed", fe.Result)
	}
	if fe.Actor != "agent-x" {
		t.Fatalf("actor = %q, want agent-x", fe.Actor)
	}
}

func TestMapPipelineEventUnknownFallback(t *testing.T) {
	ev := MapPipelineEvent("some_unknown", "kernel", "")
	if ev.Action != "SOME_UNKNOWN" {
		t.Fatalf("unknown type action = %q, want SOME_UNKNOWN", ev.Action)
	}

	ev = MapPipelineEvent("custom_failed", "kernel", "")
	if ev.Result != "failed" {
		t.Fatalf("custom_failed result = %q, want failed", ev.Result)
	}
}
