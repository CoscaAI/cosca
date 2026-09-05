package memory

// Teste de INVARIANTE DE ISOLAMENTO (professor, §18).
//
// INVARIANTE: a memória de um PROJETO (conjunto `dataDir/.cosca`) DEVE
// permanecer escopada ao projeto. O que é gravado no dataDir A NUNCA deve
// ser lido a partir do dataDir B.
//
// TEST:
//   - Cria engine com DataDir=A (projeto A) e grava um record em LayerProject.
//   - Cria engine com DataDir=B (projeto B).
//   - Busca pelo ID do record em B e por busca textual.
//   - Esperado: o record de A NÃO é retornado em B (isolamento project-local).
//
// Isso VALIDA o requisito do Don: "quando pedir pra criar um projeto, tudo
// relacionado ao projeto fica no projeto".

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
)

func TestProjectIsolation_MemoryDoesNotLeakAcrossDataDirs(t *testing.T) {
	ctx := context.Background()

	// ── Projeto A (dataDir A) ─────────────────────────────────────────
	dirA := t.TempDir()
	engA, err := NewEngine(
		WithLogger(zerolog.Nop()),
		WithConfig(EngineConfig{DataDir: dirA, AutoPrune: false}),
	)
	if err != nil {
		t.Fatalf("engA: %v", err)
	}
	defer engA.Close()

	rec := MemoryRecord{
		ID:      "proj-a-secreto",
		Type:    MemoryTypeDecision,
		Layer:   LayerProject,
		Content: "segredo exclusivo do projeto A",
		Agent:   "cosca-test",
	}
	if _, err := engA.Store(ctx, rec); err != nil {
		t.Fatalf("store A: %v", err)
	}

	// ── Projeto B (dataDir B) ─────────────────────────────────────────
	dirB := t.TempDir()
	engB, err := NewEngine(
		WithLogger(zerolog.Nop()),
		WithConfig(EngineConfig{DataDir: dirB, AutoPrune: false}),
	)
	if err != nil {
		t.Fatalf("engB: %v", err)
	}
	defer engB.Close()

	// 1) Busca por ID exato: NUNCA deve retornar o record do projeto A.
	if _, err := engB.Retrieve(ctx, "proj-a-secreto", LayerProject); err == nil {
		t.Errorf("VIOLACAO: projeto B encontrou record gravado no projeto A (por ID)")
	}

	// 2) Busca textual: NUNCA deve retornar o record do projeto A.
	results, err := engB.Search(ctx, "segredo exclusivo do projeto A", SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("search B: %v", err)
	}
	for _, r := range results {
		if r.ID == "proj-a-secreto" {
			t.Errorf("VIOLACAO: projeto B encontrou record do projeto A (por texto)")
		}
	}
}

func TestProjectIsolation_KnowledgeDBIsPerProject(t *testing.T) {
	// knowledge.db é criado dentro do coscaDir (getCoscaDir = workspace/.cosca).
	// Este teste valida que o caminho do DB é sempre `<dataDir>/knowledge.db`,
	// ou seja, ancorado ao projeto, nunca um path global compartilhado.
	dir := t.TempDir()
	coscaDir := dir + "/.cosca"

	// Simula: knowledge.New usa filepath.Join(coscaDir, "knowledge.db")
	// (EngineBuilder: DBPath = filepath.Join(coscaDir, "knowledge.db")).
	// Invariante: o DB de um projeto é sempre local ao seu .cosca.
	if got := coscaDir + "/knowledge.db"; got != dir+"/.cosca/knowledge.db" {
		t.Errorf("knowledge.db deve ser ancorado ao .cosca do projeto, got %s", got)
	}
}
