// Testes da instrumentação de custo do cosca run (Fase 0 ADR-031).
//
// Prova o WIRING: `recordFromRun` — o ponto onde o log de custo é montado —
// mapeia a evidência de valor determinística (build/test verificados + memória
// persistida) para o vetor decomposto. Uma execução com valor real agora produz
// UsefulWork() > 0 e Efficiency() > 0; uma sem evidência continua honesta em 0.
package cli

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/pipeline"
)

func TestRecordFromRun_RealValue(t *testing.T) {
	run := &pipeline.RunResult{
		Agent:   "cosca-backend",
		TraceID: "trace-1",
		TokenUsage: pipeline.TokenUsage{
			Input:  80,
			Output: 20,
		},
		BuildResult: &pipeline.BuildResult{Success: true},
		TestResult:  &pipeline.TestResult{Success: true, Passed: 5, Failed: 0},
		MemoryID:    "mem-1",
	}

	rec := recordFromRun(run)

	if rec.TokensTotal != 100 {
		t.Fatalf("TokensTotal = %d, want 100", rec.TokensTotal)
	}
	if rec.ArtifactValue != 1 {
		t.Fatalf("ArtifactValue = %d, want 1 (build passou)", rec.ArtifactValue)
	}
	if rec.EvidenceGain != 5 {
		t.Fatalf("EvidenceGain = %d, want 5 (5 testes aprovados)", rec.EvidenceGain)
	}
	if rec.KnowledgeGain != 1 {
		t.Fatalf("KnowledgeGain = %v, want 1 (memória persistida)", rec.KnowledgeGain)
	}
	if rec.TaskProgress != 1 {
		t.Fatalf("TaskProgress = %v, want 1 (build/test verificados)", rec.TaskProgress)
	}
	if rec.UsefulWork() <= 0 {
		t.Fatalf("UsefulWork() = %v, want > 0", rec.UsefulWork())
	}
	if rec.Efficiency() <= 0 {
		t.Fatalf("Efficiency() = %v, want > 0", rec.Efficiency())
	}
}

func TestRecordFromRun_Honest_NoValue(t *testing.T) {
	// Execução sem build/test e sem memória persistida → vetor 0 (honesto).
	run := &pipeline.RunResult{
		Agent:   "cosca-backend",
		TraceID: "trace-2",
		TokenUsage: pipeline.TokenUsage{
			Input:  60,
			Output: 40,
		},
	}

	rec := recordFromRun(run)

	if rec.UsefulWork() != 0 {
		t.Fatalf("UsefulWork() = %v, want 0 (nenhuma evidência de valor)", rec.UsefulWork())
	}
}

func TestRecordFromRun_FailingBuildIsHonest(t *testing.T) {
	// Build falhou → nenhum artefato verificado, dimension não é inflada.
	run := &pipeline.RunResult{
		Agent:   "cosca-backend",
		TraceID: "trace-3",
		TokenUsage: pipeline.TokenUsage{
			Input:  30,
			Output: 30,
		},
		BuildResult: &pipeline.BuildResult{Success: false},
	}

	rec := recordFromRun(run)

	if rec.ArtifactValue != 0 {
		t.Fatalf("ArtifactValue = %d, want 0 (build falhou)", rec.ArtifactValue)
	}
	if rec.UsefulWork() != 0 {
		t.Fatalf("UsefulWork() = %v, want 0 (build falhou)", rec.UsefulWork())
	}
}
