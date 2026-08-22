package pipeline

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultCondenserConfig(t *testing.T) {
	d := DefaultCondenserConfig()
	if d.MaxEvents != 100 || d.KeepEvents != 20 || d.MinCondenseEvents != 30 {
		t.Fatalf("defaults: %+v", d)
	}
}

func TestNewContextCondenserDefaults(t *testing.T) {
	c := NewContextCondenser(CondenserConfig{})
	if c.config.MaxEvents != 100 || c.config.KeepEvents != 20 || c.config.MinCondenseEvents != 30 {
		t.Fatalf("normalized config: %+v", c.config)
	}
}

func TestShouldCondense(t *testing.T) {
	c := NewContextCondenser(CondenserConfig{MaxEvents: 50, KeepEvents: 10, MinCondenseEvents: 20})
	if c.ShouldCondense(49) {
		t.Fatal("49 < 50 must not condense")
	}
	if !c.ShouldCondense(50) {
		t.Fatal("50 >= 50 must condense")
	}
}

func TestCondenseTooFewEvents(t *testing.T) {
	c := NewContextCondenser(CondenserConfig{MinCondenseEvents: 30})
	_, err := c.Condense("p", make([]StepEvent, 10))
	if err == nil || !strings.Contains(err.Error(), "at least 30") {
		t.Fatalf("error = %v", err)
	}
}

func TestCondenseBuildsSummary(t *testing.T) {
	c := NewContextCondenser(CondenserConfig{MinCondenseEvents: 3})
	now := time.Now()
	events := []StepEvent{
		{Type: StepEventStarted, PlanID: "p", TaskID: "t1", Agent: "cosca-backend", Timestamp: now, Duration: 100},
		{Type: StepEventCompleted, PlanID: "p", TaskID: "t1", Agent: "cosca-backend", Timestamp: now, Duration: 200},
		{Type: StepEventFailed, PlanID: "p", TaskID: "t2", Agent: "cosca-testing", Timestamp: now, Duration: 50},
	}

	ev, err := c.Condense("p", events)
	if err != nil {
		t.Fatalf("Condense: %v", err)
	}
	if ev.Type != CondensationEvent {
		t.Fatalf("type = %v", ev.Type)
	}
	if ev.PlanID != "p" || ev.Input != "3 events condensed" {
		t.Fatalf("event: %+v", ev)
	}
	out := ev.Output
	for _, want := range []string{"1 completed", "1 failed", "1 started", "cosca-backend:2", "cosca-testing:1", "350ms"} {
		if !strings.Contains(out, want) {
			t.Fatalf("summary missing %q: %s", want, out)
		}
	}
}

func TestCondenseCountsPlanEvents(t *testing.T) {
	c := NewContextCondenser(CondenserConfig{MinCondenseEvents: 2})
	now := time.Now()
	events := []StepEvent{
		{Type: PlanCompleted, Timestamp: now},
		{Type: PlanFailed, Timestamp: now},
	}
	ev, err := c.Condense("p", events)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ev.Output, "1 completed") || !strings.Contains(ev.Output, "1 failed") {
		t.Fatalf("plan-level events not counted: %s", ev.Output)
	}
}

func TestCompactNoop(t *testing.T) {
	c := NewContextCondenser(CondenserConfig{KeepEvents: 5, MinCondenseEvents: 3})
	events := make([]StepEvent, 4) // <= KeepEvents
	out, n, err := c.Compact("p", events)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 || len(out) != 4 {
		t.Fatalf("noop compact: n=%d len=%d", n, len(out))
	}

	// Too few to condense (split < MinCondenseEvents).
	events = make([]StepEvent, 7) // split = 7-5 = 2 < 3
	out, n, _ = c.Compact("p", events)
	if n != 0 || len(out) != 7 {
		t.Fatalf("below-min compact: n=%d len=%d", n, len(out))
	}
}

func TestCompactCondensesOldEvents(t *testing.T) {
	c := NewContextCondenser(CondenserConfig{KeepEvents: 2, MinCondenseEvents: 3})
	now := time.Now()
	events := []StepEvent{
		{Type: StepEventStarted, TaskID: "a", Timestamp: now},
		{Type: StepEventStarted, TaskID: "b", Timestamp: now},
		{Type: StepEventStarted, TaskID: "c", Timestamp: now},
		{Type: StepEventCompleted, TaskID: "d", Timestamp: now},
		{Type: StepEventCompleted, TaskID: "e", Timestamp: now},
	} // split = 5-2 = 3 >= 3 → condense 3, keep 2

	out, n, err := c.Compact("p", events)
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if n != 3 {
		t.Fatalf("condensed = %d, want 3", n)
	}
	if len(out) != 3 { // 1 condensation + 2 kept
		t.Fatalf("out len = %d, want 3", len(out))
	}
	if out[0].Type != CondensationEvent {
		t.Fatalf("first must be condensation: %v", out[0].Type)
	}
	if out[1].TaskID != "d" || out[2].TaskID != "e" {
		t.Fatalf("kept events wrong: %+v", out[1:])
	}
}

func TestContextBudget(t *testing.T) {
	b := NewContextBudget(600) // 3 events * 200 = 600
	if b.MaxTokens != 600 || b.TokensPerEvent != 200 {
		t.Fatalf("budget: %+v", b)
	}
	if b.AddEvent(StepEvent{}) {
		t.Fatal("1 event must not trigger condensation")
	}
	if b.AddEvent(StepEvent{}) {
		t.Fatal("2 events must not trigger condensation")
	}
	if !b.AddEvent(StepEvent{}) {
		t.Fatal("3 events must trigger condensation (600 >= 600)")
	}
	if b.Usage() != 100.0 {
		t.Fatalf("Usage = %v, want 100", b.Usage())
	}

	z := NewContextBudget(0)
	if z.Usage() != 0 {
		t.Fatal("zero-max Usage must be 0")
	}
}
