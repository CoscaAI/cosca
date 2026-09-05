package task

import (
	"path/filepath"
	"testing"
)

// newTestStore abre um store in-memory (não materializa arquivo).
func newTestStore(t *testing.T) *SQLStore {
	t.Helper()
	s, err := NewSQLStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func sampleState() *TaskState {
	return &TaskState{
		TaskID:         "T-1",
		Objective:      TaskObjective{ID: "O-1", Objective: "Fazer X"},
		Status:         StatusActive,
		CurrentStep:    2,
		Agent:          "cosca-backend",
		PolicyVersion:  "v3",
		Checkpoints:    []string{"c1", "c2"},
		PendingActions: []string{"p1"},
		Observations:   []string{"obs"},
		Errors:         []string{"e1"},
		Artifacts:      map[string]string{"k": "v"},
	}
}

func TestSQLStore_SaveLoadRoundTrip(t *testing.T) {
	s := newTestStore(t)
	st := sampleState()

	if err := s.Save(st); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := s.Load("T-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got == nil {
		t.Fatal("Load retornou nil para tarefa salva")
	}
	if got.TaskID != "T-1" || got.Status != StatusActive || got.CurrentStep != 2 {
		t.Fatalf("round-trip divergiu: %+v", got)
	}
	if len(got.Checkpoints) != 2 || got.Artifacts["k"] != "v" {
		t.Fatalf("slices/mapa não preservados: %+v", got)
	}
}

func TestSQLStore_LoadMissingReturnsNil(t *testing.T) {
	s := newTestStore(t)
	got, err := s.Load("T-inexistente")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != nil {
		t.Fatalf("esperava nil, got %+v", got)
	}
}

func TestSQLStore_SaveUpsertIdempotent(t *testing.T) {
	s := newTestStore(t)

	if err := s.Save(sampleState()); err != nil {
		t.Fatalf("Save #1: %v", err)
	}
	st2 := sampleState()
	st2.CurrentStep = 5
	st2.Status = StatusComplete
	if err := s.Save(st2); err != nil {
		t.Fatalf("Save #2 (upsert): %v", err)
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("upsert criou duplicata: %d tarefas", len(list))
	}
	if list[0].CurrentStep != 5 || list[0].Status != StatusComplete {
		t.Fatalf("upsert não atualizou: %+v", list[0])
	}
}

func TestSQLStore_ListReturnsAll(t *testing.T) {
	s := newTestStore(t)
	for _, id := range []TaskID{"T-1", "T-2", "T-3"} {
		st := sampleState()
		st.TaskID = id
		if err := s.Save(st); err != nil {
			t.Fatalf("Save %s: %v", id, err)
		}
	}
	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("esperava 3 tarefas, got %d", len(list))
	}
}

func TestSQLStore_Delete(t *testing.T) {
	s := newTestStore(t)
	if err := s.Save(sampleState()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := s.Delete("T-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, _ := s.Load("T-1")
	if got != nil {
		t.Fatalf("tarefa ainda existe após Delete: %+v", got)
	}
}

func TestSQLStore_LoadReturnsCopy(t *testing.T) {
	s := newTestStore(t)
	if err := s.Save(sampleState()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, _ := s.Load("T-1")
	// Mutar a cópia NÃO pode afetar o store.
	got.Checkpoints = append(got.Checkpoints, "injetado")
	got.Artifacts["k"] = "mutado"

	again, _ := s.Load("T-1")
	if len(again.Checkpoints) != 2 {
		t.Fatalf("mutação vazou para o store: %+v", again)
	}
	if again.Artifacts["k"] != "v" {
		t.Fatalf("mutação do mapa vazou: %+v", again)
	}
}

func TestSQLStore_FileBacked(t *testing.T) {
	// Round-trip com arquivo real (não in-memory) — cobre o path de privfile.
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.db")

	s, err := NewSQLStore(path)
	if err != nil {
		t.Fatalf("NewSQLStore (file): %v", err)
	}
	if err := s.Save(sampleState()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	_ = s.Close()

	s2, err := NewSQLStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	got, err := s2.Load("T-1")
	if err != nil || got == nil {
		t.Fatalf("persistência em arquivo falhou: got=%v err=%v", got, err)
	}
}
