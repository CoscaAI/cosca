package pipeline

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestWorkflowHistoryAppendLoadSequence(t *testing.T) {
	h, err := NewWorkflowHistory(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	// Append without IDs — auto-assigned.
	ev1 := StepEvent{Type: StepEventStarted, PlanID: "p1", TaskID: "t1"}
	ev2 := StepEvent{Type: StepEventCompleted, PlanID: "p1", TaskID: "t1"}
	if err := h.Append("p1", ev1); err != nil {
		t.Fatal(err)
	}
	if err := h.Append("p1", ev2); err != nil {
		t.Fatal(err)
	}

	events, err := h.Load("p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if events[0].Sequence != 1 || events[1].Sequence != 2 {
		t.Fatalf("sequences = %d,%d want 1,2", events[0].Sequence, events[1].Sequence)
	}
	if events[0].ID == "" || events[1].ID == "" {
		t.Fatal("IDs must be auto-assigned")
	}
	if events[0].Hash == "" {
		t.Fatal("hash must be assigned")
	}
	// Hash chains: hash[i] = sha256(hash[i-1] + id[i]).
	if events[1].Hash == events[0].Hash {
		t.Fatal("hashes must differ")
	}
	// Timestamps auto-set.
	if events[0].Timestamp.IsZero() {
		t.Fatal("timestamp must be auto-set")
	}

	last, err := h.LastSequence("p1")
	if err != nil || last != 2 {
		t.Fatalf("LastSequence = %d, %v", last, err)
	}
}

func TestWorkflowHistoryLoadMissingReturnsNil(t *testing.T) {
	h, _ := NewWorkflowHistory(t.TempDir())
	events, err := h.Load("ghost")
	if err != nil {
		t.Fatalf("Load(missing) error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("missing plan events = %d", len(events))
	}
}

func TestWorkflowHistoryListPlans(t *testing.T) {
	h, _ := NewWorkflowHistory(t.TempDir())
	_ = h.Append("plan-x", StepEvent{Type: PlanCreated, PlanID: "plan-x"})
	_ = h.Append("plan-y", StepEvent{Type: PlanCreated, PlanID: "plan-y"})

	plans, err := h.ListPlans()
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 2 {
		t.Fatalf("plans = %v, want 2", plans)
	}
}

func TestWorkflowHistoryVerifyIntegrity(t *testing.T) {
	h, _ := NewWorkflowHistory(t.TempDir())

	// Single event trivially valid.
	_ = h.Append("p", StepEvent{Type: PlanCreated, PlanID: "p"})
	n, err := h.VerifyIntegrity("p")
	if err != nil || n != 1 {
		t.Fatalf("single event: n=%d err=%v", n, err)
	}

	// Two events form a valid chain.
	_ = h.Append("p", StepEvent{Type: StepEventStarted, PlanID: "p", TaskID: "t1"})
	_ = h.Append("p", StepEvent{Type: StepEventCompleted, PlanID: "p", TaskID: "t1"})
	n, err = h.VerifyIntegrity("p")
	if err != nil || n != 3 {
		t.Fatalf("chain: n=%d err=%v", n, err)
	}

	// Corrupt an event's hash → broken chain.
	events, _ := h.Load("p")
	events[1].Hash = strings.Repeat("0", 64)
	h2, _ := NewWorkflowHistory(t.TempDir())
	_ = h2.Append("p", StepEvent{Type: PlanCreated, PlanID: "p"})
	_ = h2.Append("p", StepEvent{Type: StepEventStarted, PlanID: "p", TaskID: "t1"})
	// Rewrite second event's hash directly in the file.
	path := h2.filePath("p")
	lines := readLines(path)
	lines[1] = strings.Replace(lines[1], `"hash":"`, `"hash":"0000000000000000000000000000000000000000000000000000000000000000`, 1)
	// Rebuild file with corrupted hash on the SECOND event (index 1).
	writeLines(t, path, lines)

	_, err = h2.VerifyIntegrity("p")
	if err == nil {
		t.Fatal("corrupted chain must fail verification")
	}
	if !strings.Contains(err.Error(), "hash chain broken") {
		t.Fatalf("error = %v", err)
	}
}

func TestNewEventIDFormat(t *testing.T) {
	re := regexp.MustCompile(`^EVT-\d{8}-[0-9A-Fa-f]{8}$`)
	for i := 0; i < 5; i++ {
		if !re.MatchString(NewEventID()) {
			t.Fatal("NewEventID invalid format")
		}
	}
}

func TestReplayPlan(t *testing.T) {
	plan := &Plan{
		ID:     "p1",
		Intent: "do things",
		Tasks: []*TaskNode{
			{ID: "t1", Description: "first", DependsOn: []string{}},
			{ID: "t2", Description: "second", DependsOn: []string{"t1"}},
		},
	}
	now := time.Now().UTC()
	events := []StepEvent{
		{Type: PlanCreated, PlanID: "p1", Timestamp: now},
		{Type: StepEventStarted, PlanID: "p1", TaskID: "t1", Timestamp: now},
		{Type: StepEventCompleted, PlanID: "p1", TaskID: "t1", Agent: "cosca-backend", Output: "ok", Duration: 50, Timestamp: now},
		{Type: StepEventFailed, PlanID: "p1", TaskID: "t2", Output: "boom", Timestamp: now},
		{Type: PlanFailed, PlanID: "p1", Timestamp: now},
	}

	replayed := ReplayPlan(plan, events)
	if replayed.Status != "failed" {
		t.Fatalf("plan status = %q", replayed.Status)
	}
	if len(replayed.Tasks) != 2 {
		t.Fatalf("tasks = %d", len(replayed.Tasks))
	}
	t1 := replayed.Tasks[0]
	if t1.Status != TaskCompleted || t1.Result == nil || !t1.Result.Success || t1.Result.Output != "ok" {
		t.Fatalf("t1 replay: %+v", t1)
	}
	t2 := replayed.Tasks[1]
	if t2.Status != TaskFailed || t2.Result == nil || t2.Result.Success || t2.Result.Error != "boom" {
		t.Fatalf("t2 replay: %+v", t2)
	}
	// Original plan untouched.
	if plan.Tasks[0].Status != "" && plan.Tasks[0].Status != TaskPending {
		t.Fatal("replay must not mutate the input plan")
	}
}

func TestReplayPlanSkippedAndRetrying(t *testing.T) {
	plan := &Plan{ID: "p", Tasks: []*TaskNode{{ID: "t1"}, {ID: "t2"}}}
	events := []StepEvent{
		{Type: StepEventSkipped, PlanID: "p", TaskID: "t1"},
		{Type: StepEventRetrying, PlanID: "p", TaskID: "t2"},
	}
	replayed := ReplayPlan(plan, events)
	if replayed.Tasks[0].Status != TaskSkipped {
		t.Fatalf("skipped status = %q", replayed.Tasks[0].Status)
	}
	if replayed.Tasks[1].Status != TaskPending {
		t.Fatalf("retrying status = %q", replayed.Tasks[1].Status)
	}
}

func TestSplitLines(t *testing.T) {
	got := splitLines("a\nb\nc")
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("splitLines = %v", got)
	}
	if got := splitLines(""); len(got) != 0 {
		t.Fatalf("empty = %v", got)
	}
	if got := splitLines("single"); len(got) != 1 || got[0] != "single" {
		t.Fatalf("single = %v", got)
	}
}

// readLines returns the non-empty lines of a file (test helper).
func readLines(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return splitLines(string(data))
}

// writeLines persists lines to a file (test helper).
func writeLines(t *testing.T, path string, lines []string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}
