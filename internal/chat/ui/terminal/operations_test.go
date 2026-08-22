package terminal

import (
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/trace"
)

func TestBuildTreeFromEventsNesting(t *testing.T) {
	events := []trace.Event{
		{TraceID: "T1", Action: "PLAN_CREATED", Result: "running"},
		{TraceID: "T1", Action: "TOOL_STARTED", Result: "running"},
		{TraceID: "T1", Action: "TOOL_RESULT", Result: "success"},
		{TraceID: "T1", Action: "EXECUTION_DONE", Result: "success"},
		{TraceID: "T2", Action: "BUILD_STARTED", Result: "running"},
	}

	root := BuildTreeFromEvents(events)

	if len(root.Children) != 2 {
		t.Fatalf("expected 2 trace roots, got %d", len(root.Children))
	}

	t1 := root.Children[0]
	if t1.ID != "T1" {
		t.Fatalf("first root = %q, want T1", t1.ID)
	}
	// EXECUTION_DONE is a terminal marker — updates root status, no leaf.
	if t1.Status != "done" {
		t.Fatalf("T1 root status = %q, want done", t1.Status)
	}
	if len(t1.Children) != 3 {
		t.Fatalf("T1 root children = %d, want 3", len(t1.Children))
	}
	if t1.Children[0].Label != "PLAN_CREATED" {
		t.Fatalf("T1 child 0 = %q, want PLAN_CREATED", t1.Children[0].Label)
	}

	t2 := root.Children[1]
	if t2.Status != "running" {
		t.Fatalf("T2 root status = %q, want running", t2.Status)
	}
}

func TestBuildTreeFromEventsParentNesting(t *testing.T) {
	events := []trace.Event{
		{TraceID: "T1", Action: "PLAN_CREATED", Result: "running"},
		{TraceID: "T1", ParentEvent: "T1", Action: "TASK_STARTED", Result: "running"},
	}

	root := BuildTreeFromEvents(events)

	t1 := root.Children[0]
	// The second event attaches under the trace root (parent "T1").
	if len(t1.Children) != 2 {
		t.Fatalf("expected 2 children under T1, got %d", len(t1.Children))
	}
	if t1.Children[1].Label != "TASK_STARTED" {
		t.Fatalf("nested child label = %q, want TASK_STARTED", t1.Children[1].Label)
	}
}

func TestOperationsAddEvent(t *testing.T) {
	p := NewOperationsPanel()

	p.AddEvent(trace.Event{TraceID: "T1", Action: "PLAN_CREATED", Result: "running"})
	p.AddEvent(trace.Event{TraceID: "T1", Action: "TOOL_STARTED", Result: "running"})
	p.AddEvent(trace.Event{TraceID: "T1", Action: "TOOL_RESULT", Result: "success"})

	if p.events != 3 {
		t.Fatalf("events = %d, want 3", p.events)
	}
	if p.active != 2 {
		t.Fatalf("active = %d, want 2 (PLAN_CREATED + TOOL_STARTED)", p.active)
	}
	if root := p.tree.root; root == nil || len(root.Children) != 1 {
		t.Fatalf("operations tree should have 1 trace root")
	}
}

func TestOperationsRender(t *testing.T) {
	p := NewOperationsPanel()
	p.AddEvent(trace.Event{TraceID: "T1", Action: "PLAN_CREATED", Result: "running", Details: "planning"})
	p.SetFrame(0)

	out := p.Render(30, 12)
	if !strings.Contains(out, "EXECUÇÃO") {
		t.Fatalf("render missing EXECUÇÃO header:\n%s", out)
	}
	if !strings.Contains(out, "PLAN_CREATED") {
		t.Fatalf("render missing PLAN_CREATED node:\n%s", out)
	}
}
