// Command cosca-indexer indexes all Cosca knowledge, memory, and workflow
// files into the SQLite vector store with TF-IDF embeddings.
//
// It uses the local zero-dependency embedding provider so no API keys are needed.
// Usage:
//
//	cd /path/to/project && go run cmd/cosca-indexer/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/indexer"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/providers/local"
	"github.com/CoscaAI/cosca/internal/providers/ollama"
	"github.com/CoscaAI/cosca/internal/search"
)

// fileEntry é um candidato a indexação: path + categoria de fonte + subType de
// registro + agente dono (fontes de agente) + proveniência física (origin).
type fileEntry struct {
	path     string
	category string // "agent", "knowledge", "workflow"
	subType  string // "learnings", "patterns", "failures", "general"
	agent    string // agente dono (fontes de agente)
	origin   string // proveniência física: "opencode" | "fallback" | "embed"
}

func main() {
	// Register embedding providers — Ollama (768-dim nomic-embed-text) preferred, local TF-IDF fallback.
	local.Register()

	// Try to register Ollama; non-fatal if it's not running.
	_ = registerOllamaProvider()

	// Set up logging
	log.Logger = zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: "15:04:05",
	}).Level(zerolog.InfoLevel).With().Timestamp().Logger()

	log.Info().Msg("Cosca Knowledge Indexer starting")

	// Determine project root from CWD
	projectRoot, err := os.Getwd()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get working directory")
	}

	// Verify we're in the cosca project
	if _, err := os.Stat(filepath.Join(projectRoot, ".cosca")); os.IsNotExist(err) {
		log.Fatal().Msg("must be run from the cosca project root (.cosca not found)")
	}

	// Database path
	dbPath := filepath.Join(projectRoot, ".cosca", "knowledge.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatal().Err(err).Msg("failed to create data directory")
	}

	// Initialize the knowledge engine
	cfg := knowledge.Config{
		DBPath:       dbPath,
		RootDir:      projectRoot,
		AutoMigrate:  true,
		WatchEnabled: true,
		IndexerConfig: func() indexer.IndexerConfig {
			idxCfg := indexer.DefaultConfig()
			idxCfg.RequireEmbeddings = true
			return idxCfg
		}(),
	}

	engine, err := knowledge.New(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create knowledge engine")
	}

	if err := engine.Init(); err != nil {
		log.Fatal().Err(err).Msg("failed to initialize knowledge engine")
	}
	defer func() {
		if err := engine.Close(); err != nil {
			log.Error().Err(err).Msg("failed to close knowledge engine")
		}
	}()

	ctx := context.Background()

	// Collect all files to index
	var files []fileEntry

	// 1. Fonte de agente. A fonte VIVA de projeto (.cosca/memory/agent/**)
	// é ingerida como origem de projeto (fonte única curada). A cópia legada em
	// .cosca/fallback/memory/agent é resquício do sync morto (derivada do
	// embed canônico) — NÃO é ingerida para evitar indexação duplicada e
	// conflitante do mesmo agente (o canônico diverge do fallback). O legado
	// .opencode/cosca/memory/agent é redundante e NÃO é ingerido: a fonte
	// curada agora vive em .cosca/.
	// Passa pelo CLASSIFICADOR: só o que for persistente é indexado.
	agentRoots := []struct {
		dir    string
		origin string
	}{
		{filepath.Join(projectRoot, ".cosca", "memory", "agent"), "cosca"},
	}
	for _, ar := range agentRoots {
		entries, err := os.ReadDir(ar.dir)
		if err != nil {
			log.Warn().Err(err).Str("dir", ar.dir).Msg("agent memory directory not found")
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			agentName := entry.Name()
			agentPath := filepath.Join(ar.dir, agentName)
			agentFiles, err := os.ReadDir(agentPath)
			if err != nil {
				log.Warn().Err(err).Str("dir", agentPath).Msg("skipping agent dir")
				continue
			}
			for _, af := range agentFiles {
				if af.IsDir() {
					continue
				}
				ext := strings.ToLower(filepath.Ext(af.Name()))
				if ext != ".md" {
					continue
				}
				subType := "general"
				baseName := strings.TrimSuffix(af.Name(), ext)
				switch baseName {
				case "learnings":
					subType = "learnings"
				case "patterns":
					subType = "patterns"
				case "failures":
					subType = "failures"
				}
				files = append(files, fileEntry{
					path:     filepath.Join(agentPath, af.Name()),
					category: "agent",
					subType:  subType,
					agent:    agentName,
					origin:   ar.origin,
				})
			}
		}
	}

	// 2. Knowledge files (internal/embed/cosca/knowledge/**/*.md, *.yaml).
	// Fonte versionada (o cérebro) — a cópia antiga em .cosca/fallback/knowledge
	// é resquício do sync morto e NÃO deve ser indexada.
	knowledgeDir := filepath.Join(projectRoot, "internal", "embed", "cosca", "knowledge")
	if _, err := os.Stat(knowledgeDir); err == nil {
		err = filepath.Walk(knowledgeDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".md" && ext != ".yaml" && ext != ".yml" {
				return nil
			}
			files = append(files, fileEntry{
				path:     path,
				category: "knowledge",
				subType:  strings.TrimPrefix(filepath.Dir(path), knowledgeDir+string(filepath.Separator)),
				origin:   "embed",
			})
			return nil
		})
		if err != nil {
			log.Warn().Err(err).Msg("error walking knowledge directory")
		}
	}

	// 3. Workflow files (internal/embed/cosca/workflows/*.md) — fonte versionada.
	workflowDir := filepath.Join(projectRoot, "internal", "embed", "cosca", "workflows")
	if entries, err := os.ReadDir(workflowDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext != ".md" {
				continue
			}
			files = append(files, fileEntry{
				path:     filepath.Join(workflowDir, entry.Name()),
				category: "workflow",
				subType:  strings.TrimSuffix(entry.Name(), ext),
				origin:   "embed",
			})
		}
	} else {
		log.Warn().Err(err).Msg("workflows directory not found")
	}

	log.Info().Int("total_files", len(files)).Msg("files to index")

	// Index each file
	start := time.Now()
	var failed int
	var skipped int

	for i, f := range files {
		relPath, _ := filepath.Rel(projectRoot, f.path)
		log.Info().
			Int("progress", i+1).
			Int("total", len(files)).
			Str("file", relPath).
			Str("category", f.category).
			Str("type", f.subType).
			Msg("indexing")

		// Proveniência semântica => metadata_json (scope/origin/kind/agent).
		meta := metaFor(f, projectRoot)

		// Fontes de agente passam pelo CLASSIFICADOR (fail-closed): só o que
		// for persistente é indexado. scope é decidido pelo classificador.
		if f.category == "agent" {
			content, rErr := os.ReadFile(f.path)
			if rErr != nil {
				log.Warn().Err(rErr).Str("file", relPath).Msg("failed to read for classification, skipping")
				skipped++
				continue
			}
			cls := knowledge.ClassifyDoc(f.path, nil, string(content), f.agent)
			if !cls.Persistent {
				log.Info().
					Str("file", relPath).
					Str("reason", cls.Reason).
					Float64("confidence", cls.Confidence).
					Msg("skipped by classifier (not persistent)")
				skipped++
				continue
			}
			meta["scope"] = cls.Scope
			meta["kind"] = cls.Kind
			meta["epistemic"] = string(knowledge.EpistemicFor(cls.Kind)) // FASE 4 — classe epistêmica estrutural
			log.Info().Str("file", relPath).Str("kind", cls.Kind).Str("scope", cls.Scope).Msg("classified persistent")
		}

		// scope=global é reservado ao cérebro embarcado (metaFor já define);
		// fontes de projeto nunca recebem scope=global (classificador).
		if err := engine.IndexDocumentWithMeta(ctx, f.path, meta); err != nil {
			log.Warn().Err(err).Str("file", relPath).Msg("failed to index, skipping")
			failed++
			continue
		}
	}
	if failed > 0 {
		log.Error().Int("failed", failed).Msg("indexing incomplete: required embeddings were not produced for all files")
	}

	elapsed := time.Since(start)
	indexed := len(files) - failed - skipped

	// Get stats
	stats, err := engine.GetStats()
	if err != nil {
		log.Warn().Err(err).Msg("failed to get stats")
	} else {
		log.Info().
			Int("indexed", indexed).
			Int("failed", failed).
			Int("documents", stats.DocumentCount).
			Int("chunks", stats.ChunkCount).
			Int("entities", stats.EntityCount).
			Int("vectors", stats.VectorCount).
			Dur("duration", elapsed).
			Int64("db_size_bytes", stats.DBSize).
			Msg("indexing complete")
	}

	// Run a validation query
	log.Info().Msg("running validation query...")
	validateResults, err := engine.Query(ctx, "embedding vector search indexing")
	if err != nil {
		log.Error().Err(err).Msg("validation query failed")
	} else {
		if validateResults != nil {
			log.Info().
				Int("results", len(validateResults.Results)).
				Dur("query_duration", validateResults.Duration).
				Msg("validation query complete")
			if len(validateResults.Results) > 0 {
				for i, r := range validateResults.Results {
					if i >= 5 {
						break
					}
					log.Info().
						Int("rank", r.Rank).
						Str("type", string(r.Type)).
						Float64("score", r.Score).
						Str("title", r.Title).
						Str("source", r.Source).
						Str("snippet", truncate(r.Snippet, 120)).
						Msgf("result #%d", i+1)
				}
			}
		}
	}

	// Run a semantic search using vector mode only
	log.Info().Msg("running semantic search query (vector only)...")
	params := search.DefaultSearchParams()
	params.Query = "agent memory learning"
	params.EnableFTS = false
	params.EnableVector = true
	params.Limit = 5
	semResults, err := engine.Search(ctx, params)
	if err != nil {
		log.Warn().Err(err).Msg("semantic search failed (expected if vectors empty)")
	} else if semResults != nil && len(semResults.Results) > 0 {
		log.Info().
			Int("results", len(semResults.Results)).
			Dur("duration", semResults.Duration).
			Msg("semantic search returned results")
		for i, r := range semResults.Results {
			if i >= 5 {
				break
			}
			log.Info().
				Int("rank", r.Rank).
				Float64("score", r.Score).
				Str("snippet", truncate(r.Snippet, 120)).
				Msgf("semantic result #%d", i+1)
		}
	} else {
		log.Warn().Msg("semantic search returned no results")
	}

	// Print summary
	fmt.Println()
	fmt.Println("==============================================")
	fmt.Println("  COSCA KNOWLEDGE INDEXER — SUMMARY")
	fmt.Println("==============================================")
	fmt.Printf("  Total files indexed:    %d\n", indexed)
	fmt.Printf("  Failed:                 %d\n", failed)
	fmt.Printf("  Skipped:                %d\n", skipped)
	fmt.Printf("  Duration:               %v\n", elapsed.Round(time.Millisecond))
	if stats != nil {
		fmt.Printf("  Documents:              %d\n", stats.DocumentCount)
		fmt.Printf("  Chunks:                 %d\n", stats.ChunkCount)
		fmt.Printf("  Entities:               %d\n", stats.EntityCount)
		fmt.Printf("  Vectors:                %d\n", stats.VectorCount)
		fmt.Printf("  Missing vectors:        %d\n", stats.IndexStats.MissingVectors)
		fmt.Printf("  DB Size:                %d bytes\n", stats.DBSize)
	}
	fmt.Println("==============================================")
	if failed > 0 {
		os.Exit(1)
	}
}

// registerOllamaProvider registers the Ollama embedding provider with the
// global registry. It does not connect to the server at registration time;
// the actual connection happens lazily during embedding generation.
func registerOllamaProvider() error {
	ollama.Register()
	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// metaFor monta a proveniência semântica de uma fonte embarcada (global) ou
// de agente (project, sobre-escrita pelo classificador no loop de indexação).
func metaFor(f fileEntry, projectRoot string) map[string]any {
	meta := map[string]any{
		"scope":  scopeFor(f),
		"origin": f.origin,
		"kind":   "knowledge",
	}
	if f.category == "workflow" {
		meta["kind"] = "workflow"
	}
	if f.agent != "" {
		meta["agent"] = f.agent
	}
	if p := filepath.Base(projectRoot); p != "" && p != "." && p != string(filepath.Separator) {
		meta["project"] = p
	}
	return meta
}

// scopeFor: scope=global é reservado ao cérebro embarcado (knowledge/workflow
// de internal/embed/cosca/**). Fontes de agente são scope=project (o valor
// final é confirmado pelo classificador no loop).
// scopeFor decide o escopo pela ORIGEM física da fonte (não só pela categoria).
//   - "cosca" (.cosca/memory/agent/**) → project (conhecimento VIVO
//     do projeto — a fonte única curada que o agente produz durante o trabalho).
//   - "fallback" (.cosca/fallback/**) e "embed" (internal/embed/cosca/**) →
//     global (cérebro embarcado: uma é a cópia materializada, a outra a fonte;
//     ambas são conteúdo de framework, não conhecimento único do projeto).
//
// Para fontes de agente o loop de indexação SOBRESCREVE com cls.Scope (do
// classificador), que segue a mesma regra (regra de ouro: origem de projeto
// nunca vira global).
func scopeFor(f fileEntry) string {
	switch f.origin {
	case "cosca":
		return knowledge.ScopeProject
	case "fallback", "embed":
		return knowledge.ScopeGlobal
	default:
		return knowledge.ScopeProject
	}
}
