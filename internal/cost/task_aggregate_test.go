package cost

import (
	"testing"
	"time"
)

// TestAggregateTasks_GroupByTaskID prova a unidade CAUSAL: execuções do
// mesmo task_id (write_file + build) são agrupadas em UM TaskRecord com
// Executions[] aninhadas — em vez de records separados sem vínculo.
func TestAggregateTasks_GroupByTaskID(t *testing.T) {
	now := time.Now()
	records := []Record{
		{
			TaskID: "task-123", AgentID: "frontend",
			InputTokens: 1000, OutputTokens: 500, TokensTotal: 1500,
			ArtifactValue: 1, TaskProgress: 1, At: now,
		},
		{
			TaskID: "task-123", AgentID: "frontend",
			InputTokens: 300, OutputTokens: 200, TokensTotal: 500,
			EvidenceGain: 1, TaskProgress: 1, At: now.Add(2 * time.Second),
		},
		{
			TaskID: "task-456", AgentID: "backend",
			InputTokens: 100, OutputTokens: 100, TokensTotal: 200,
			At: now,
		},
	}

	tasks := AggregateTasks(records)
	if len(tasks) != 2 {
		t.Fatalf("esperava 2 tasks, got %d", len(tasks))
	}

	// task-123 deve ter 2 execuções aninhadas (write + build).
	var task123 *TaskRecord
	for i := range tasks {
		if tasks[i].TaskID == "task-123" {
			task123 = &tasks[i]
			break
		}
	}
	if task123 == nil {
		t.Fatal("task-123 nao encontrada")
	}
	if task123.Runs != 2 {
		t.Fatalf("task-123 esperava 2 runs, got %d", task123.Runs)
	}
	if task123.TokensTotal != 2000 {
		t.Fatalf("task-123 esperava 2000 tokens, got %d", task123.TokensTotal)
	}
	if task123.ArtifactValue != 1 || task123.EvidenceGain != 1 {
		t.Fatalf("task-123 esperava artifact=1/evidence=1, got art=%d ev=%d", task123.ArtifactValue, task123.EvidenceGain)
	}
	if len(task123.Executions) != 2 {
		t.Fatalf("task-123 esperava 2 executions, got %d", len(task123.Executions))
	}
	if task123.UsefulWork() < 2 {
		t.Fatalf("task-123 useful work deveria ser >=2, got %.2f", task123.UsefulWork())
	}

	// task-456 (sem evidência) deve ter useful work 0.
	var task456 *TaskRecord
	for i := range tasks {
		if tasks[i].TaskID == "task-456" {
			task456 = &tasks[i]
			break
		}
	}
	if task456 == nil {
		t.Fatal("task-456 nao encontrada")
	}
	if task456.UsefulWork() != 0 {
		t.Fatalf("task-456 util work deveria ser 0, got %.2f", task456.UsefulWork())
	}
}

// TestRecordToExecution_BackwardCompatible assegura que a conversão preserva
// os campos de valor já registrados.
func TestRecordToExecution_BackwardCompatible(t *testing.T) {
	r := Record{
		TaskID: "abc", InputTokens: 10, OutputTokens: 20, TokensTotal: 30,
		ArtifactValue: 1, EvidenceGain: 1, DurationMs: 500,
	}
	e := recordToExecution(r)
	if e.TokensTotal != 30 {
		t.Fatalf("esperava tokens=30, got %d", e.TokensTotal)
	}
	if e.ArtifactValue != 1 || e.EvidenceGain != 1 {
		t.Fatalf("perdeu evidencia na conversao: art=%d ev=%d", e.ArtifactValue, e.EvidenceGain)
	}
	if e.Status != "success" {
		t.Fatalf("esperava success, got %q", e.Status)
	}
}

// TestExecutionStatus_Derive assegura a derivação de status por evidência.
func TestExecutionStatus_Derive(t *testing.T) {
	if executionStatus(Record{}) != "failed" {
		t.Fatal("record sem evidencia deveria ser failed")
	}
	if executionStatus(Record{KnowledgeGain: 1}) != "success" {
		t.Fatal("record com knowledge deveria ser success")
	}
}
