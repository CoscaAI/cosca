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
	type fileEntry struct {
		path     string
		category string // "agent", "knowledge", "workflow"
		subType  string // "learnings", "patterns", "failures", "general"
	}

	var files []fileEntry

	// 1. Agent memory files (.cosca/memory/agent/cosca-*/)
	agentDir := filepath.Join(projectRoot, ".cosca", "fallback", "memory", "agent")
	if entries, err := os.ReadDir(agentDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			agentPath := filepath.Join(agentDir, entry.Name())
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
				})
			}
		}
	} else {
		log.Warn().Err(err).Msg("agent memory directory not found")
	}

	// 2. Knowledge files (.cosca/framework/knowledge/**/*.md, *.yaml)
	knowledgeDir := filepath.Join(projectRoot, ".cosca", "fallback", "knowledge")
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
			})
			return nil
		})
		if err != nil {
			log.Warn().Err(err).Msg("error walking knowledge directory")
		}
	}

	// 3. Workflow files (.cosca/framework/workflows/*.md)
	workflowDir := filepath.Join(projectRoot, ".cosca", "fallback", "workflows")
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

		if err := engine.IndexDocument(ctx, f.path); err != nil {
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
