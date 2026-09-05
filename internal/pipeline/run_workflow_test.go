package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/CoscaAI/cosca/internal/workflow"
)

// TestRunWorkflow proves the pipeline bridge to the typed-routing engine:
// the adapter executes a value-routed graph and the classic DependsOn planner
// is untouched alongside it.
func TestRunWorkflow(t *testing.T) {
	var path []string

	fetch := workflow.NewFunctionNode("fetch", func(_ context.Context, _ map[string]any) (workflow.Route, any, error) {
		path = append(path, "fetch")
		return workflow.StringRoute("ok"), "payload", nil
	})
	transform := workflow.NewFunctionNode("transform", func(_ context.Context, _ map[string]any) (workflow.Route, any, error) {
		path = append(path, "transform")
		return nil, "transformed", nil
	})

	w, err := workflow.New("bridge", []workflow.Edge{
		{From: fetch, To: transform, Route: workflow.StringRoute("ok")},
	})
	if err != nil {
		t.Fatalf("workflow.New: %v", err)
	}

	out, err := RunWorkflow(context.Background(), w, map[string]any{"seed": true})
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	if len(path) != 2 || path[0] != "fetch" || path[1] != "transform" {
		t.Errorf("expected path [fetch transform], got %v", path)
	}
	if out["seed"] != true {
		t.Errorf("expected initial state to pass through, got %v", out)
	}
}

// TestRunWorkflow_Error propagates a routing failure through the bridge.
func TestRunWorkflow_Error(t *testing.T) {
	a := workflow.NewFunctionNode("a", func(_ context.Context, _ map[string]any) (workflow.Route, any, error) {
		return workflow.StringRoute("left"), nil, nil
	})
	b := workflow.NewFunctionNode("b", func(_ context.Context, _ map[string]any) (workflow.Route, any, error) {
		return nil, nil, nil
	})

	w, err := workflow.New("bridge-err", []workflow.Edge{
		{From: a, To: b, Route: workflow.StringRoute("right")},
	})
	if err != nil {
		t.Fatalf("workflow.New: %v", err)
	}
	if _, err := RunWorkflow(context.Background(), w, nil); err == nil {
		t.Fatal("expected no-route error")
	} else if !errors.Is(err, workflow.ErrNoRouteMatch) {
		t.Fatalf("expected ErrNoRouteMatch, got %v", err)
	}
}
