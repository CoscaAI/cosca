package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSessionStoreSaveLoad(t *testing.T) {
	s, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	rec := &SessionRecord{
		ID:          "sess-1",
		ProjectPath: "/repo",
		Goal:        "ship it",
		State:       "running",
		Blockers:    []string{"waiting on credentials"},
	}
	if err := s.Save(rec); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if rec.LastActiveAt.IsZero() {
		t.Fatal("LastActiveAt must be set on Save")
	}

	loaded, err := s.Load("sess-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ID != "sess-1" || loaded.Goal != "ship it" || len(loaded.Blockers) != 1 {
		t.Fatalf("roundtrip: %+v", loaded)
	}
}

func TestSessionStoreLoadMissingAndDelete(t *testing.T) {
	s, _ := NewSessionStore(t.TempDir())
	if _, err := s.Load("ghost"); err == nil {
		t.Fatal("Load(missing) expected error")
	}
	// Delete of a non-existent session is idempotent.
	if err := s.Delete("ghost"); err != nil {
		t.Fatalf("Delete(missing): %v", err)
	}

	rec := &SessionRecord{ID: "sess-2", ProjectPath: "/p"}
	_ = s.Save(rec)
	if err := s.Delete("sess-2"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Load("sess-2"); err == nil {
		t.Fatal("session should be deleted")
	}
}

func TestSessionStoreListSortsByLastActive(t *testing.T) {
	s, _ := NewSessionStore(t.TempDir())

	// Save always stamps LastActiveAt=now, so to control ordering we write the
	// JSON files directly with explicit timestamps.
	now := time.Now()
	write := func(rec *SessionRecord) {
		data, _ := json.MarshalIndent(rec, "", "  ")
		if err := os.WriteFile(filepath.Join(s.dir, rec.ID+".session.json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(&SessionRecord{ID: "old", ProjectPath: "/repo", LastActiveAt: now.Add(-2 * time.Hour)})
	write(&SessionRecord{ID: "new", ProjectPath: "/repo", LastActiveAt: now.Add(-time.Minute)})
	write(&SessionRecord{ID: "other", ProjectPath: "/elsewhere", LastActiveAt: now.Add(-3 * time.Hour)})

	all, err := s.List("")
	if err != nil || len(all) != 3 {
		t.Fatalf("List('') = %d, %v", len(all), err)
	}
	// Sorted descending by LastActiveAt: new first.
	if all[0].ID != "new" {
		t.Fatalf("first = %q, want new", all[0].ID)
	}

	filtered, err := s.List("/repo")
	if err != nil || len(filtered) != 2 {
		t.Fatalf("List(/repo) = %d, %v", len(filtered), err)
	}

	// Empty dir → nil, no error.
	empty, _ := NewSessionStore(t.TempDir())
	if recs, _ := empty.List(""); recs != nil {
		t.Fatalf("empty list = %v", recs)
	}
	if _, err := empty.LoadLatest("/x"); err == nil {
		t.Fatal("LoadLatest on empty store expected error")
	}
}

func TestRecordFromTerminalContext(t *testing.T) {
	plan := &Plan{ID: "sess-x", Intent: "intent", EstimatedMinutes: 30, Tasks: []*TaskNode{
		{ID: "t1", Status: TaskCompleted},
		{ID: "t2", Status: TaskPending},
	}}
	sc := NewSessionContext(plan)
	sc.Metrics.StartedAt = time.Now().Add(-time.Minute)
	tc := &TerminalContext{
		SessionContext: sc,
		ProjectPath:    "/repo",
		SessionState:   "running",
		ActiveAgent:    "cosca-backend",
		ActiveModel:    "deepseek-v4",
	}

	rec := RecordFromTerminalContext(tc, "the goal", "last task")
	if rec.ID != "sess-x" || rec.ProjectPath != "/repo" {
		t.Fatalf("record: %+v", rec)
	}
	if rec.Progress != 0.5 {
		t.Fatalf("progress = %v, want 0.5", rec.Progress)
	}
	if rec.Plan == nil || rec.Plan.TasksCompleted != 1 || rec.Plan.TasksTotal != 2 {
		t.Fatalf("plan snapshot: %+v", rec.Plan)
	}
	if rec.Agent != "cosca-backend" || rec.Model != "deepseek-v4" {
		t.Fatalf("agent/model: %+v", rec)
	}
}

func TestRestoreTerminalContext(t *testing.T) {
	record := &SessionRecord{
		ID:          "sess-y",
		ProjectPath: "/p",
		State:       "blocked",
		Agent:       "cosca-ceo",
		Model:       "gpt-4o",
		Plan: &PlanSnapshot{
			ID:           "plan-z",
			Intent:       "do it",
			TasksTotal:   2,
			TaskStatuses: map[string]string{"t1": "completed", "t2": "failed"},
		},
	}

	tc := &TerminalContext{}
	RestoreTerminalContext(tc, record)

	if tc.LastSessionID != "sess-y" || tc.ProjectPath != "/p" || tc.SessionState != "blocked" {
		t.Fatalf("restored fields: %+v", tc)
	}
	if tc.ActiveAgent != "cosca-ceo" || tc.ActiveModel != "gpt-4o" {
		t.Fatalf("agent/model restored: %+v", tc)
	}
	if tc.SessionContext == nil || tc.SessionContext.Plan == nil {
		t.Fatal("session context/plan must be restored")
	}
	if tc.SessionContext.Plan.ID != "plan-z" {
		t.Fatalf("plan ID = %q", tc.SessionContext.Plan.ID)
	}
	// Tasks restored with statuses.
	byID := map[string]TaskStatus{}
	for _, task := range tc.SessionContext.Plan.Tasks {
		byID[task.ID] = task.Status
	}
	if byID["t1"] != TaskCompleted || byID["t2"] != TaskFailed {
		t.Fatalf("task statuses: %v", byID)
	}
	_ = strings.ToUpper
}
