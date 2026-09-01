// Package cli — Fases 5+6 do COSCA Environment Provisioner: Knowledge +
// Index build. Reusa diretamente o que o COSCA já sabe fazer (db build,
// cobertura vetorial, backfill) — o "trabalho pesado já está feito".
package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/installer"
	"github.com/CoscaAI/cosca/internal/knowledge"
)

// knowledgeCheck valida o database + corpus (Fase 5).
type knowledgeCheck struct{}

func (k *knowledgeCheck) ID() string   { return "knowledge.database" }
func (k *knowledgeCheck) Name() string { return "Knowledge / Database" }

func (k *knowledgeCheck) Detect() (installer.Result, []string) {
	dir, err := resolveDataDir("")
	if err != nil {
		return installer.ResultFail, []string{"data dir não resolvido: " + err.Error()}
	}
	kb := filepath.Join(dir, "knowledge.db")
	if _, err := os.Stat(kb); err != nil {
		return installer.ResultRebuildRequired, []string{"knowledge.db não encontrado em " + kb}
	}
	eng, err := openKnowledgeForCheck(dir)
	if err != nil {
		return installer.ResultFail, []string{"knowledge engine não abriu: " + err.Error()}
	}
	defer eng.Close()
	chunks, _ := eng.VectorCoverageCounts()
	if chunks == 0 {
		return installer.ResultRebuildRequired, []string{"database vazio — corpus precisa ser indexado"}
	}
	return installer.ResultPass, []string{
		"knowledge.db presente em " + kb,
		fmt.Sprintf("%d chunks no corpus", chunks),
	}
}

func (k *knowledgeCheck) Install() error {
	// O corpus é indexado na Fase 6 (index build) — a base já existe via
	// cosca init. Se falta, o provisioner constrói o conhecimento do zero.
	return nil
}

func (k *knowledgeCheck) Validate() (installer.Result, []string) {
	dir, _ := resolveDataDir("")
	eng, err := openKnowledgeForCheck(dir)
	if err != nil {
		return installer.ResultFail, []string{"knowledge engine: " + err.Error()}
	}
	defer eng.Close()
	chunks, vectors := eng.VectorCoverageCounts()
	return installer.ResultPass, []string{
		fmt.Sprintf("database legível: %d chunks, %d vetores", chunks, vectors),
	}
}

// indexBuildCheck reconstrói/verifica o índice vetorial (Fase 6). Detecta a
// cobertura; se degradada (< 80%), REBUILD_REQUIRED → Install roda o backfill.
type indexBuildCheck struct{}

func (i *indexBuildCheck) ID() string   { return "knowledge.vector_index" }
func (i *indexBuildCheck) Name() string { return "Index build" }

func (i *indexBuildCheck) Detect() (installer.Result, []string) {
	dir, _ := resolveDataDir("")
	eng, err := openKnowledgeForCheck(dir)
	if err != nil {
		return installer.ResultFail, []string{"knowledge engine: " + err.Error()}
	}
	defer eng.Close()
	chunks, vectors := eng.VectorCoverageCounts()
	if chunks == 0 {
		return installer.ResultRebuildRequired, []string{"sem chunks para indexar"}
	}
	coverage := float64(vectors) / float64(chunks)
	if coverage < 0.8 {
		return installer.ResultRebuildRequired, []string{
			fmt.Sprintf("cobertura vetorial %.1f%% (%d/%d) — rebuild necessário", coverage*100, vectors, chunks),
		}
	}
	return installer.ResultPass, []string{
		fmt.Sprintf("índice vetorial saudável: %.1f%% cobertura (%d/%d)", coverage*100, vectors, chunks),
	}
}

func (i *indexBuildCheck) Install() error {
	// Reconstrói o índice: backfill dos vetores + módulos físicos do split.
	dir, err := resolveDataDir("")
	if err != nil {
		return err
	}
	eng, err := openKnowledgeForCheck(dir)
	if err != nil {
		return err
	}
	defer eng.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if _, err := eng.BackfillVectors(ctx, false); err != nil {
		return fmt.Errorf("backfill de vetores: %w", err)
	}

	// Módulos físicos do split (ADR-013) — reconstruíveis via db build.
	kb := filepath.Join(dir, "knowledge.db")
	if _, err := os.Stat(kb); err == nil {
		if _, err := dbBuildModules(nil, kb, dir); err != nil {
			return fmt.Errorf("db build (split): %w", err)
		}
	}
	return nil
}

func (i *indexBuildCheck) Validate() (installer.Result, []string) {
	dir, _ := resolveDataDir("")
	eng, err := openKnowledgeForCheck(dir)
	if err != nil {
		return installer.ResultFail, []string{"knowledge engine: " + err.Error()}
	}
	defer eng.Close()
	chunks, vectors := eng.VectorCoverageCounts()
	if chunks > 0 && float64(vectors)/float64(chunks) >= 0.8 {
		return installer.ResultPass, []string{
			fmt.Sprintf("índice reconstruído: %d vetores / %d chunks (%.1f%%)",
				vectors, chunks, float64(vectors)/float64(chunks)*100),
		}
	}
	return installer.ResultFail, []string{
		fmt.Sprintf("índice ainda degradado: %d vetores / %d chunks", vectors, chunks),
	}
}

// openKnowledgeForCheck abre o knowledge engine com a config do projeto.
func openKnowledgeForCheck(dir string) (*knowledge.Engine, error) {
	var (
		embeddingProvider string
		embeddingModel    string
		embeddingBaseURL  string
	)
	if c, err := config.Load(); err == nil {
		embeddingProvider = c.Embedding.Provider
		embeddingModel = c.Embedding.Model
		embeddingBaseURL = c.Embedding.BaseURL
	}
	eng, err := knowledge.New(knowledge.Config{
		DBPath:            filepath.Join(dir, "knowledge.db"),
		RootDir:           ".",
		AutoMigrate:       true,
		EmbeddingProvider: embeddingProvider,
		EmbeddingModel:    embeddingModel,
		EmbeddingBaseURL:  embeddingBaseURL,
	})
	if err != nil {
		return nil, err
	}
	if err := eng.Init(); err != nil {
		_ = eng.Close()
		return nil, err
	}
	return eng, nil
}
