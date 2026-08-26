package cli

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/CoscaAI/cosca/internal/auth"
	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/discovery"
	"github.com/CoscaAI/cosca/internal/indexer"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/modlink"
	"github.com/CoscaAI/cosca/internal/search"
	_ "modernc.org/sqlite"
)

// NewKnowledgeCommand creates the `cosca knowledge` command and its subcommands.
func NewKnowledgeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "knowledge",
		Short: "Manage the Cosca knowledge base",
		Long: `Manage and interact with the Cosca knowledge base.

The knowledge base stores indexed information about your project including
code structure, documentation, decisions, patterns, and relationships.

Subcommands provide search, graph visualization, statistics, and maintenance.
`,
		Example: `  cosca knowledge search <query>    Search the knowledge base
  cosca knowledge graph              Show the knowledge graph
  cosca knowledge stats              Show knowledge statistics
  cosca knowledge rebuild            Rebuild the knowledge base
  cosca knowledge verify             Verify index integrity
  cosca knowledge law list           List knowledge laws (CKL)
  cosca knowledge discovery list     List knowledge discoveries (Hall of Fame)
  cosca knowledge status             Show epistemic status of laws (CKL)
  cosca knowledge claim              Classify claims (FACT/EVIDENCE/INFERENCE/...)
  cosca knowledge evidence add       Add evidence with provenance (P0-P5)
  cosca knowledge revalidate         Revalidate knowledge against local source (hash)
  cosca knowledge add <lib|repo>     Register a Knowledge Package (manifest) — KNOWLEDGE ≠ DEPENDENCY
  cosca knowledge packages list      List registered Knowledge Packages
  cosca knowledge packages show <id> Show a Knowledge Package manifest
  cosca knowledge packages status    Show a Knowledge Package status summary
  cosca knowledge diff <pkg> <from> <to>  What changed between package versions
  cosca knowledge match [dir]       Match project dependencies against known packages
  cosca knowledge resolve <task>    Tarefa → Knowledge Resolver → lacuna de conhecimento?
  cosca knowledge index-entities   Vetoriza os nós do grafo para busca semântica`,
	}

	cmd.AddCommand(
		NewKnowledgeSearchCommand(),
		NewKnowledgeGraphCommand(),
		NewKnowledgeRebuildCommand(),
		NewKnowledgeVerifyCommand(),
		NewKnowledgeGCCommand(),
		NewKnowledgePromoteCommand(),
		NewKnowledgeStatsCommand(),
		NewKnowledgeBenchmarkCommand(),
		NewKnowledgeVacuumCommand(),
		NewKnowledgeExplainCommand(),
		NewKnowledgeRelationsCommand(),
		NewKnowledgeCompileCommand(),
		NewKnowledgeIndexEntitiesCommand(),
		NewKnowledgeLawCommand(),
		NewKnowledgeDiscoveryCommand(),
		NewKnowledgeStatusCommand(),
		NewKnowledgeClaimCommand(),
		NewKnowledgeEvidenceCommand(),
		NewKnowledgeRevalidateCommand(),
		NewKnowledgeAddCommand(),
		NewKnowledgeAcquireCommand(),
		NewKnowledgeIndexCommand(),
		NewKnowledgePackagesCommand(),
		NewKnowledgeDiffCommand(),
		NewKnowledgeMatchCommand(),
		NewKnowledgeResolveCommand(),
		NewKnowledgeReadinessCommand(),
		NewKnowledgeIngestCommand(),
		NewKnowledgeClassifyCommand(),
	)

	return cmd
}

// NewKnowledgeSearchCommand creates the `cosca knowledge search` subcommand.
func NewKnowledgeSearchCommand() *cobra.Command {
	var limit int
	var offset int
	var global bool
	var scopeModule string
	var mode string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the knowledge base",
		Long: `Search the Cosca knowledge base for information matching the query.

Com --global/-g, busca no knowledge.db global (~/.config/cosca/knowledge.db),
compartilhado entre TODOS os projetos. Sem a flag, busca no .cosca/knowledge.db
local e também faz grep nos documentos globais adquiridos.

Modo modular (--mode=modular ou search.mode=modular na config): o roteador
determinístico (modlink) decide o espaço de busca. Uma consulta sem rota
(NO_ROUTE) devolve 0 resultados + o sinal NO_ROUTE — nunca um full-scan
silencioso. --scope=<module> restringe AINDA MAIS (nunca amplia): o módulo deve
existir no registry, senão é erro; se a query for incompatível com o escopo
forçado, o resultado é 0.`,
		Example: `  cosca knowledge search "database schema"
  cosca knowledge search --limit 20 "error handling patterns"
  cosca knowledge search --json "architecture decisions"
  cosca knowledge search --global "cobra"
  cosca knowledge search --mode=modular "decisão arquitetural do banco"
  cosca knowledge search --mode=modular --scope=adr "decisão arquitetural"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			if global {
				return runGlobalSearch(cmd, formatter, useJSON, args[0], limit, offset)
			}

			dir, _ := os.Getwd()

			// Load project config for embedding provider settings (unified with search layered)
			// AND the search mode (search.mode: legacy | modular).
			var (
				embeddingProvider string
				embeddingBaseURL  string
				embeddingModel    string
				embeddingDigest   string
				embeddingAPIKey   string
				embeddingDim      int
				cfgSearchMode     string
			)
			if c, cfgErr := config.Load(); cfgErr == nil {
				embeddingProvider = c.Embedding.Provider
				embeddingBaseURL = c.Embedding.BaseURL
				embeddingModel = c.Embedding.Model
				embeddingDigest = c.Embedding.Digest
				embeddingAPIKey = c.Embedding.APIKey
				embeddingDim = c.Embedding.Dimensions
				cfgSearchMode = c.Search.Mode
			}

			// Modo efetivo: flag --mode > config -> default LEGACY (retrocompatível).
			effectiveMode := mode
			if effectiveMode == "" {
				effectiveMode = cfgSearchMode
			}
			if effectiveMode != search.ModeModular {
				effectiveMode = search.ModeLegacy
			}

			// ── Modo MODULAR (FASE 1 routing/scope) ─────────────────────────
			// O roteador determina o espaço; a busca confina. O caminho RÁPIDO do
			// daemon 24/7 é intencionalmente ignorado aqui (FASE 1 roteia no
			// caminho LOCAL — mais simples de tornar determinístico e honesto
			// quanto ao escopo; o daemon pode adotar o mesmo roteamento depois).
			if effectiveMode == search.ModeModular {
				return runModularKnowledgeSearch(
					cmd, formatter, useJSON, dir, args[0], limit, offset, scopeModule,
					embeddingProvider, embeddingBaseURL, embeddingModel, embeddingDigest,
					embeddingAPIKey, embeddingDim,
				)
			}

			// ── Modo LEGACY (comportamento atual, sem roteamento) ───────────
			// Caminho rápido: o daemon 24/7 já tem o knowledge engine vivo (Init
			// frio custa ~6s por invocação CLI). Busca via REST quando o daemon
			// responde; senão cai no Engine local (mesma pipeline).
			if daemonHits, daemonOK := daemonKnowledgeSearch(args[0], limit+offset); daemonOK {
				results := make([]KnowledgeSearchResult, 0, len(daemonHits))
				for _, r := range daemonHits {
					results = append(results, KnowledgeSearchResult{
						Title:   r.Title,
						Type:    string(r.Type),
						Score:   r.Score,
						Snippet: r.Snippet,
					})
				}
				globalResults := searchGlobalKnowledge(args[0], limit)
				results = mergeSearchResults(results, globalResults, limit+offset)
				return printKnowledgeResults(cmd, formatter, useJSON, results, args[0], globalResults)
			}

			ke, kErr := knowledge.New(knowledge.Config{
				DBPath:              knowledgeDBPath(dir),
				RootDir:             dir,
				AutoMigrate:         true,
				EmbeddingProvider:   embeddingProvider,
				EmbeddingBaseURL:    embeddingBaseURL,
				EmbeddingModel:      embeddingModel,
				EmbeddingDigest:     embeddingDigest,
				EmbeddingAPIKey:     embeddingAPIKey,
				EmbeddingDimensions: embeddingDim,
			})
			if kErr != nil {
				return fmt.Errorf("knowledge engine not available: %w", kErr)
			}
			if iErr := ke.Init(); iErr != nil {
				return fmt.Errorf("init knowledge engine: %w", iErr)
			}
			defer ke.Close()

			// Use layered search with cosine similarity (same pipeline as `cosca search layered`)
			lsCfg := search.DefaultLayeredConfig()
			ls := search.NewLayeredSearch(ke, lsCfg).SetRanker(ke.Ranker()).SetGraph(ke.Graph())
			res, sErr := ls.Search(cmd.Context(), args[0])
			if sErr != nil {
				return fmt.Errorf("search failed: %w", sErr)
			}

			results := make([]KnowledgeSearchResult, 0, len(res.Results))
			for _, r := range res.Results {
				results = append(results, KnowledgeSearchResult{
					Title:   r.Title,
					Type:    string(r.Type),
					Score:   r.Score,
					Snippet: r.Snippet,
				})
			}
			globalResults := searchGlobalKnowledge(args[0], limit)
			results = mergeSearchResults(results, globalResults, limit+offset)
			return printKnowledgeResults(cmd, formatter, useJSON, results, args[0], globalResults)
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "l", 10, "Maximum number of results")
	cmd.Flags().IntVarP(&offset, "offset", "o", 0, "Result offset")
	cmd.Flags().BoolVarP(&global, "global", "g", false, "Search the global knowledge base (~/.config/cosca/)")
	cmd.Flags().StringVar(&scopeModule, "scope", "", "Módulo do registry (modlink) para confinar AINDA MAIS a busca (restrição adicional, nunca ampliação)")
	cmd.Flags().StringVar(&mode, "mode", "", "Modo de busca: legacy (default, sem roteamento) | modular (roteamento obrigatório; NoRoute → 0 + NO_ROUTE)")
	return cmd
}

// runModularKnowledgeSearch executa `cosca knowledge search` no modo MODULAR:
// o roteador determinístico decide o espaço, a busca confina a ele, e uma
// consulta sem rota (NoRoute) devolve 0 resultados + o sinal NO_ROUTE — nunca
// full-scan silencioso.
//
// --scope=M é uma restrição ADICIONAL, não um substituto do roteador: M precisa
// constar no registry (senão é erro), e é aceito apenas se a query roteada
// incluir M; query incompatível com o escopo forçado → 0 resultados.
func runModularKnowledgeSearch(
	cmd *cobra.Command, formatter *OutputFormatter, useJSON bool,
	dir, query string, limit, offset int, forcedScope string,
	embeddingProvider, embeddingBaseURL, embeddingModel, embeddingDigest, embeddingAPIKey string,
	embeddingDim int,
) error {
	routes := modlink.DefaultRoutes()
	if err := modlink.ValidateRoutes(routes); err != nil {
		return fmt.Errorf("route registry inválido: %w", err)
	}
	resolver := modlink.NewResolver(routes)

	var forcedModule string
	if forcedScope != "" {
		byModule := modlink.RoutesByModule(routes)
		route, ok := byModule[forcedScope]
		if !ok {
			return fmt.Errorf("--scope=%q não está no registry de rotas (módulos conhecidos): %s",
				forcedScope, strings.Join(sortedRouteModules(byModule), ", "))
		}
		forcedModule = route.Module
	}

	ke, kErr := knowledge.New(knowledge.Config{
		DBPath:              knowledgeDBPath(dir),
		RootDir:             dir,
		AutoMigrate:         true,
		EmbeddingProvider:   embeddingProvider,
		EmbeddingBaseURL:    embeddingBaseURL,
		EmbeddingModel:      embeddingModel,
		EmbeddingDigest:     embeddingDigest,
		EmbeddingAPIKey:     embeddingAPIKey,
		EmbeddingDimensions: embeddingDim,
	})
	if kErr != nil {
		return fmt.Errorf("knowledge engine not available: %w", kErr)
	}
	if iErr := ke.Init(); iErr != nil {
		return fmt.Errorf("init knowledge engine: %w", iErr)
	}
	defer ke.Close()

	params := search.DefaultSearchParams()
	params.Query = query
	params.Limit = limit
	params.Offset = offset

	scoped, scope := search.ApplyForcedScope(resolver, query, forcedModule, params)
	if scope.NoRoute {
		// Route desconhecida (ou escopo forçado incompatível) → sinal explícito,
		// NUNCA full-scan. O agente não é bloqueado; só o conhecimento semântico
		// fica vazio + o sinal NO_ROUTE.
		if useJSON {
			return printJSON(cmd, map[string]interface{}{
				"query":    query,
				"results":  []KnowledgeSearchResult{},
				"total":    0,
				"no_route": true,
			})
		}
		formatter.Header("Knowledge Search Results")
		formatter.KeyValue("Scope", "modular")
		formatter.KeyValue("Query", query)
		formatter.KeyValue("Results", "0")
		formatter.Warning("NO_ROUTE — nenhum espaço semântico confiável para esta consulta (sem full-scan)")
		return nil
	}

	res, sErr := ke.Search(cmd.Context(), scoped)
	if sErr != nil {
		return fmt.Errorf("search failed: %w", sErr)
	}

	results := make([]KnowledgeSearchResult, 0, len(res.Results))
	for _, r := range res.Results {
		results = append(results, KnowledgeSearchResult{
			Title:   r.Title,
			Type:    string(r.Type),
			Score:   r.Score,
			Snippet: r.Snippet,
		})
	}
	// In modo modular NÃO mesclamos o grep global de documentos adquiridos
	// (searchGlobalKnowledge): o espaço semântico é estritamente o roteado — a
	// mescla reintroduziria resultados fora do escopo ("nunca silenciosamente amplo").
	globalResults := []KnowledgeSearchResult(nil)
	merged := mergeSearchResults(results, globalResults, limit+offset)
	return printKnowledgeResults(cmd, formatter, useJSON, merged, query, globalResults)
}

// sortedRouteModules devolve os módulos conhecidos do registry, ordenados, para
// mensagens de erro legíveis.
func sortedRouteModules(byModule map[string]modlink.Route) []string {
	modules := make([]string, 0, len(byModule))
	for m := range byModule {
		modules = append(modules, m)
	}
	sort.Strings(modules)
	return modules
}

// runGlobalSearch searches the global knowledge.db via FTS5.

// coscaGlobalDir: caminho canônico do diretório global do Cosca
// (~/.config/cosca/ no host REAL). Dentro da jaula bwrap o HOME=/,
// então UserHomeDir() aponta para o workspace — o padrão da casa é
// tentar o caminho real do host primeiro, depois UserHomeDir() (L338).
func coscaGlobalDir() (string, error) {
	real := "/home/cosca/.config/cosca"
	if fi, err := os.Stat(real); err == nil && fi.IsDir() {
		return real, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".config", "cosca"), nil
}

// coscaGlobalDB: caminho do knowledge.db global (host real primeiro).
func coscaGlobalDB() (string, error) {
	dir, err := coscaGlobalDir()
	if err != nil {
		return "", err
	}
	real := filepath.Join("/home/cosca/.config/cosca", "knowledge.db")
	if _, err := os.Stat(real); err == nil {
		return real, nil
	}
	return filepath.Join(dir, "knowledge.db"), nil
}

func runGlobalSearch(cmd *cobra.Command, formatter *OutputFormatter, useJSON bool, query string, limit, offset int) error {
	kbDir, err := coscaGlobalDir()
	if err != nil {
		return err
	}

	ke, kErr := openKnowledgeEngine(kbDir, true, "")
	if kErr != nil {
		return fmt.Errorf("global knowledge engine not available: %w", kErr)
	}

	if iErr := ke.Init(); iErr != nil {
		return fmt.Errorf("global knowledge engine init failed: %w", iErr)
	}
	defer ke.Close()

	params := search.DefaultSearchParams()
	params.Query = query
	params.Limit = limit
	params.Offset = offset
	results, sErr := ke.Search(cmd.Context(), params)
	if sErr != nil {
		return fmt.Errorf("global search failed: %w", sErr)
	}

	var out []KnowledgeSearchResult
	if results != nil {
		for _, r := range results.Results {
			out = append(out, KnowledgeSearchResult{
				Title:   r.Title,
				Type:    string(r.Type),
				Score:   r.Score,
				Snippet: r.Snippet,
			})
		}
	}

	if useJSON {
		return printJSON(cmd, out)
	}

	formatter.Header("Knowledge Search Results")
	formatter.KeyValue("Scope", "global")
	formatter.KeyValue("Query", query)
	formatter.KeyValue("Results", fmt.Sprintf("%d", len(out)))
	formatter.Println("")

	for i, r := range out {
		formatter.Printf("%d. %s\n", i+1, r.Title)
		formatter.KeyValue("   Type", r.Type)
		formatter.KeyValue("   Score", fmt.Sprintf("%.2f", r.Score))
		if r.Snippet != "" {
			formatter.KeyValue("   Snippet", r.Snippet)
		}
		formatter.Println("")
	}

	return nil
}

// NewKnowledgeGraphCommand creates the `cosca knowledge graph` subcommand.
func NewKnowledgeGraphCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Show the knowledge graph",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			g := NewGraph(".")
			if g == nil {
				return fmt.Errorf("graph engine not available")
			}

			stats, _ := g.Stats()

			if useJSON {
				return printJSON(cmd, stats)
			}

			formatter.Header("Knowledge Graph")
			formatter.KeyValue("Scope", g.Scope())
			formatter.KeyValue("Nodes", fmt.Sprintf("%d", stats.TotalNodes))
			formatter.KeyValue("Edges", fmt.Sprintf("%d", stats.TotalEdges))

			return nil
		},
	}

	return cmd
}

// NewKnowledgeRebuildCommand creates the `cosca knowledge rebuild` subcommand.
func NewKnowledgeRebuildCommand() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "rebuild",
		Short: "Rebuild the knowledge base",
		Long: `Reconstrói o Knowledge Base (FTS5 + embeddings) a partir dos documentos
adquiridos em knowledge/acquired/.

Com --global/-g, reconstrói o knowledge.db global (~/.config/cosca/knowledge.db),
compartilhado entre TODOS os projetos. Sem a flag, reconstrói o .cosca/knowledge.db local.`,
		Example: `  cosca knowledge rebuild
  cosca knowledge rebuild --global`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			var kbDir string
			var scope string
			var err error

			if global {
				kbDir, err = coscaGlobalDir()
				if err != nil {
					return err
				}
				scope = "global"
			} else {
				dir, _ := os.Getwd()
				kbDir = filepath.Join(dir, ".cosca")
				scope = "local"
			}

			spinner := formatter.Spinner(fmt.Sprintf("Rebuilding %s knowledge base", scope))
			spinner.Start()

			ke, kErr := openKnowledgeEngine(kbDir, global, "")
			if kErr != nil {
				spinner.Fail("Knowledge engine not available")
				return fmt.Errorf("knowledge engine not available: %w", kErr)
			}

			if iErr := ke.Init(); iErr != nil {
				spinner.Fail("Knowledge engine init failed")
				return fmt.Errorf("knowledge engine init failed: %w", iErr)
			}
			defer ke.Close()

			if err := ke.Rebuild(cmd.Context()); err != nil {
				spinner.Fail(fmt.Sprintf("Rebuild failed: %v", err))
				return fmt.Errorf("rebuild failed: %w", err)
			}

			acquiredDir := filepath.Join(kbDir, "knowledge", "acquired")
			if _, sErr := os.Stat(acquiredDir); os.IsNotExist(sErr) {
				if mErr := os.MkdirAll(acquiredDir, 0o755); mErr != nil {
					spinner.Fail(fmt.Sprintf("Rebuild failed: %v", mErr))
					return fmt.Errorf("creating acquired dir: %w", mErr)
				}
				formatter.Warning("no acquired knowledge yet — rebuild will index an empty base; use 'cosca knowledge acquire <id> --allow-remote --compile' first")
			}
			if iErr := ke.IndexDirectory(cmd.Context(), acquiredDir); iErr != nil {
				spinner.Fail(fmt.Sprintf("Rebuild failed: %v", iErr))
				return fmt.Errorf("indexing acquired knowledge: %w", iErr)
			}

			spinner.Stop(fmt.Sprintf("%s knowledge base rebuilt", scope))

			if useJSON {
				return printJSON(cmd, map[string]string{"status": "ok", "scope": scope})
			}

			formatter.Success(fmt.Sprintf("%s knowledge base rebuilt successfully", scope))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "Rebuild the global knowledge base (~/.config/cosca/)")

	return cmd
}

// NewKnowledgeIndexCommand creates the `cosca knowledge index <dir>` subcommand.
func NewKnowledgeIndexCommand() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "index <dir>",
		Short: "Indexa os documentos de um diretório no Knowledge Base (FTS5 + embeddings)",
		Long: `Indexa os documentos de um diretório no Knowledge Base (FTS5 + embeddings).

Com --global/-g, indexa no knowledge.db global (~/.config/cosca/knowledge.db),
compartilhado entre TODOS os projetos. Sem a flag, indexa no .cosca/knowledge.db local.`,
		Example: `  cosca knowledge index docs
  cosca knowledge index internal/embed/cosca
  cosca knowledge index docs --global`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			var kbDir string
			var scope string
			var err error

			if global {
				kbDir, err = coscaGlobalDir()
				if err != nil {
					return err
				}
				scope = "global"
			} else {
				dir, _ := os.Getwd()
				kbDir = filepath.Join(dir, ".cosca")
				scope = "local"
			}

			// The indexer enforces path containment against the engine RootDir
			// (C1 — see internal/indexer validatePathWithin). Root the engine at
			// the explicitly-named target directory so `index <dir>` can ingest
			// project docs while the containment invariant (indexed paths stay
			// within the declared root) is preserved for every other command.
			indexRoot, aErr := filepath.Abs(args[0])
			if aErr != nil {
				return fmt.Errorf("resolve directory %q: %w", args[0], aErr)
			}

			spinner := formatter.Spinner(fmt.Sprintf("Indexando %s no knowledge base %s", args[0], scope))
			spinner.Start()

			ke, kErr := openKnowledgeEngine(kbDir, global, indexRoot)
			if kErr != nil {
				spinner.Fail("Knowledge engine not available")
				return fmt.Errorf("knowledge engine not available: %w", kErr)
			}

			if iErr := ke.Init(); iErr != nil {
				spinner.Fail("Knowledge engine init failed")
				return fmt.Errorf("knowledge engine init failed: %w", iErr)
			}
			defer ke.Close()

			if iErr := ke.IndexDirectory(cmd.Context(), indexRoot); iErr != nil {
				spinner.Fail(fmt.Sprintf("Indexing failed: %v", iErr))
				return fmt.Errorf("indexing failed: %w", iErr)
			}

			spinner.Stop("Índice atualizado")

			if useJSON {
				return printJSON(cmd, map[string]string{"status": "ok", "scope": scope, "dir": args[0]})
			}

			formatter.Success(fmt.Sprintf("Índice atualizado no knowledge base %s", scope))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "Indexar no knowledge.db global (~/.config/cosca/)")

	return cmd
}

// NewKnowledgeIndexEntitiesCommand creates the `cosca knowledge index-entities`
// subcommand — vetoriza os NÓS do grafo (entities) para busca semântica.
//
// Antes deste comando, a tabela vectors só tinha linhas ligadas via
// document_id/chunk_id: o grafo não tinha vetores próprios e a busca semântica
// não encontrava skills/ADRs/patterns/bugs por similaridade. O pipeline grava
// vetores com entity_id populado ("ent-<entity_id>", INSERT OR REPLACE — é
// idempotente e re-indexa sem duplicar).
func NewKnowledgeIndexEntitiesCommand() *cobra.Command {
	var global bool
	var limit int

	cmd := &cobra.Command{
		Use:   "index-entities",
		Short: "Vetoriza os nós do grafo (entities) para busca semântica",
		Long: `Vetoriza os NÓS do grafo (entities) para busca semântica.

O pipeline gera embeddings do conteúdo representativo de cada nó do grafo
(name + path + metadados como description/decision) e grava na tabela vectors
com entity_id populado. Com isso, "cosca knowledge search" passa a encontrar
skills, ADRs, patterns e bugs por similaridade semântica — não só por substring
de nome.

Nós do tipo chunk sem arestas no grafo são ignorados, e chunks que já têm vetor
(embutidos pelo pipeline de documentos) também — o vetor de entidade só
duplicaria a cobertura. Sem provedor de embeddings o comando degrada com graça
(FTS5 continua cobrindo a busca).

Com --global/-g, opera no knowledge.db global (~/.config/cosca/knowledge.db),
compartilhado entre TODOS os projetos. Sem a flag, no .cosca/knowledge.db local.
Com --limit N, vetoriza no máximo N nós (útil com provedor de embeddings lento).`,
		Example: `  cosca knowledge index-entities
  cosca knowledge index-entities --limit 50
  cosca knowledge index-entities --global
  cosca knowledge index-entities --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			var kbDir string
			var scope string
			var err error

			if global {
				kbDir, err = coscaGlobalDir()
				if err != nil {
					return err
				}
				scope = "global"
			} else {
				dir, _ := os.Getwd()
				kbDir = filepath.Join(dir, ".cosca")
				scope = "local"
			}

			spinner := formatter.Spinner(fmt.Sprintf("Vetorizando nós do grafo no knowledge base %s", scope))
			spinner.Start()

			ke, kErr := openKnowledgeEngine(kbDir, global, "")
			if kErr != nil {
				spinner.Fail("Knowledge engine not available")
				return fmt.Errorf("knowledge engine not available: %w", kErr)
			}

			if iErr := ke.Init(); iErr != nil {
				spinner.Fail("Knowledge engine init failed")
				return fmt.Errorf("knowledge engine init failed: %w", iErr)
			}
			defer ke.Close()

			stats, iErr := ke.IndexEntityVectors(cmd.Context(), limit)
			if iErr != nil {
				spinner.Fail(fmt.Sprintf("Falha ao vetorizar entidades: %v", iErr))
				return fmt.Errorf("index entity vectors: %w", iErr)
			}

			spinner.Stop(fmt.Sprintf("Vetorização concluída: %d vetores de entidades gravados", stats.Indexed))

			if useJSON {
				return printJSON(cmd, stats)
			}

			formatter.Header("Vetorização de entidades do grafo")
			formatter.KeyValue("Escopo", scope)
			formatter.KeyValue("Nós examinados", fmt.Sprintf("%d", stats.Total))
			formatter.KeyValue("Ignorados", fmt.Sprintf("%d", stats.Skipped))
			formatter.KeyValue("Vetores gravados (entity_id)", fmt.Sprintf("%d", stats.Indexed))
			formatter.KeyValue("Sem embedding (FTS cobre)", fmt.Sprintf("%d", stats.Missing))
			formatter.KeyValue("Duração", fmt.Sprintf("%dms", stats.Duration.Milliseconds()))

			if stats.Indexed == 0 && stats.Missing > 0 {
				formatter.Warning("Nenhum vetor gravado — sem provedor de embeddings ativo. A busca FTS5 continua cobrindo.")
			} else if stats.Indexed == 0 {
				formatter.Warning("Nenhum vetor gravado — grafo vazio ou todos os nós ignorados. Rode 'cosca knowledge compile'/'cosca knowledge index' primeiro.")
			} else {
				formatter.Success("Busca semântica agora encontra os nós do grafo por similaridade.")
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "Indexar no knowledge.db global (~/.config/cosca/)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Limite de nós a vetorizar (0 = todos)")

	return cmd
}

// NewKnowledgeVerifyCommand creates the `cosca knowledge verify` subcommand.
func NewKnowledgeVerifyCommand() *cobra.Command {
	var fix bool

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify knowledge base integrity",
		Long: `Verify the knowledge base integrity (SQLite, FTS5, vector store, graph).

With --fix, automatically repair orphan vectors in both directions:
  • chunks without embeddings are re-embedded using the active provider
  • dangling vectors (pointing at deleted chunks/documents) are removed
Re-embedding can take several minutes on large databases.`,
		Example: `  cosca knowledge verify          # Check integrity
  cosca knowledge verify --fix    # Check and repair orphan vectors`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()

			spinner := formatter.Spinner("Verifying knowledge base integrity")
			spinner.Start()

			ke := NewKnowledgeEngine(dir)
			if ke == nil {
				spinner.Fail("Knowledge engine not available")
				return fmt.Errorf("knowledge engine not available")
			}

			result, _ := ke.Verify()

			spinner.Stop(fmt.Sprintf("Verification complete: %d issues found", result.Issues))

			if useJSON {
				return printJSON(cmd, result)
			}

			if result.Issues == 0 {
				formatter.Success("Knowledge base integrity verified — no issues found")
			} else {
				formatter.Warning(fmt.Sprintf("Knowledge base has %d issues:", result.Issues))
				for _, issue := range result.IssueList {
					formatter.Bullet(issue.Message)
				}

				if fix {
					formatter.Println("")
					formatter.Header("Repairing orphan vectors")
					dangling := ke.CleanupDanglingVectors()
					ke.RepairOrphanVectors()

					// Re-verify so the report reflects what was actually fixed
					// (RepairOrphanVectors only re-embeds chunks missing vectors;
					// CleanupDanglingVectors removes vectors pointing at nothing).
					recheck, rErr := ke.Verify()
					if rErr != nil {
						formatter.Warning(fmt.Sprintf("Re-verify falhou (o relatorio pode estar incompleto): %v", rErr))
					}
					formatter.Println("")
					if dangling > 0 {
						formatter.Success(fmt.Sprintf("Removed %d dangling vectors", dangling))
					}
					if recheck.Issues == 0 {
						formatter.Success("Knowledge base integrity verified — no issues found")
					} else {
						formatter.Warning(fmt.Sprintf("Repair completed but %d issues remain:", recheck.Issues))
						for _, issue := range recheck.IssueList {
							formatter.Bullet(issue.Message)
						}
					}
				} else {
					formatter.Println("")
					formatter.Println("\n  Tip: run 'cosca knowledge verify --fix' to repair orphan vectors")
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&fix, "fix", false, "Repair orphan vectors (re-embed chunks without vectors + remove dangling vectors)")

	return cmd
}

// NewKnowledgeGCCommand creates the `cosca knowledge gc` subcommand.
func NewKnowledgeGCCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Remove documents whose tier window expired",
		Long: `Remove documentos cuja janela de tier expirou (L338).

Regra da casa: TUDO entra como médio (7 dias) — o longo (1 ano) só entra por
promoção explícita. Este comando roda o ciclo de vida: apaga os expirados,
limpa os vetores órfãos deixados para trás e mostra o que sobrou por tier.`,
		Example: `  cosca knowledge gc
  cosca knowledge gc --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()

			spinner := formatter.Spinner("Coletando documentos expirados")
			spinner.Start()

			ke := NewKnowledgeEngine(dir)
			if ke == nil {
				spinner.Fail("Knowledge engine not available")
				return fmt.Errorf("knowledge engine not available")
			}

			removed := ke.GCExpired()

			spinner.Stop(fmt.Sprintf("GC concluído: %d documentos expirados removidos", removed))

			if useJSON {
				return printJSON(cmd, map[string]int{"removed": removed})
			}

			formatter.KeyValue("Documentos expirados removidos", fmt.Sprintf("%d", removed))
			if removed == 0 {
				formatter.Success("Nada a coletar — tudo dentro da janela de tier.")
			} else {
				formatter.Success("Ciclo de vida aplicado: expirados fora, vetores órfãos limpos.")
			}

			return nil
		},
	}

	return cmd
}

// NewKnowledgePromoteCommand creates the `cosca knowledge promote` subcommand.
func NewKnowledgePromoteCommand() *cobra.Command {
	var tier string
	var all bool

	cmd := &cobra.Command{
		Use:   "promote <doc-id>",
		Short: "Move a document to another tier (long = 1 year)",
		Long: `Move um documento para outro tier (L338).

Regra da casa: TUDO entra como médio (7 dias). O longo (1 ano) é reservado —
só entra aqui o que o Don decidir: "o longo a gente vai ver o que coloca".

Tiers:
  long    → 1 ano (promoção — o que merece viver)
  medium  → 7 dias (rebaixamento — o padrão)`,
		Example: `  cosca knowledge promote <doc-id> --tier long
  cosca knowledge promote --all --tier long   # promove todos os documentos`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			if tier != "long" && tier != "medium" {
				return fmt.Errorf("tier inválido: %s (use 'long' ou 'medium')", tier)
			}
			if !all && len(args) == 0 {
				return fmt.Errorf("informe o doc-id ou use --all")
			}

			dir, _ := os.Getwd()

			spinner := formatter.Spinner(fmt.Sprintf("Promovendo para %s", tier))
			spinner.Start()

			ke := NewKnowledgeEngine(dir)
			if ke == nil {
				spinner.Fail("Knowledge engine not available")
				return fmt.Errorf("knowledge engine not available")
			}

			var promoted int
			var firstErr error
			if all {
				// --all: promove todos os documentos do tier atual.
				fromTier := "medium"
				if tier == "medium" {
					fromTier = "long"
				}
				ids := ke.ListDocumentsByTier(fromTier)
				for _, id := range ids {
					if err := ke.PromoteDocument(id, tier); err != nil {
						if firstErr == nil {
							firstErr = err
						}
						continue
					}
					promoted++
				}
			} else {
				if err := ke.PromoteDocument(args[0], tier); err != nil {
					spinner.Fail(err.Error())
					return err
				}
				promoted = 1
			}

			spinner.Stop(fmt.Sprintf("%d documento(s) movido(s) para %s", promoted, tier))

			if useJSON {
				return printJSON(cmd, map[string]interface{}{"promoted": promoted, "tier": tier})
			}

			formatter.KeyValue("Documentos movidos", fmt.Sprintf("%d", promoted))
			formatter.KeyValue("Tier destino", tier)
			if tier == "long" {
				formatter.Success("Agora vivem 1 ano — só o Don decide o que mora no longo.")
			} else {
				formatter.Success("De volta ao padrão: 7 dias de janela.")
			}
			if firstErr != nil {
				formatter.Warning(fmt.Sprintf("Alguns falharam: %v", firstErr))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&tier, "tier", "long", "Tier de destino: long (1 ano) ou medium (7 dias)")
	cmd.Flags().BoolVar(&all, "all", false, "Aplicar a todos os documentos do tier oposto")

	return cmd
}
func NewKnowledgeStatsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show knowledge base statistics",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()

			ke := NewKnowledgeEngine(dir)
			if ke == nil {
				return fmt.Errorf("knowledge engine not available")
			}

			stats, _ := ke.Stats()

			if useJSON {
				return printJSON(cmd, stats)
			}

			formatter.Header("Knowledge Base Statistics")
			formatter.KeyValue("Total Entries", fmt.Sprintf("%d", stats.TotalEntries))
			formatter.KeyValue("Database Size", stats.DatabaseSize)

			return nil
		},
	}

	return cmd
}

// NewKnowledgeBenchmarkCommand creates the `cosca knowledge benchmark` subcommand.
func NewKnowledgeBenchmarkCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "Benchmark knowledge base performance",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

			dir, _ := os.Getwd()

			ke := NewKnowledgeEngine(dir)
			if ke == nil {
				return fmt.Errorf("knowledge engine not available")
			}

			formatter.Header("Knowledge Base Benchmark")
			result, _ := ke.Benchmark()
			formatter.KeyValue("Search Latency", result.AvgSearchTime)
			formatter.KeyValue("Index Size", result.IndexSize)
			formatter.KeyValue("Queries/Second", fmt.Sprintf("%d", result.QueriesRun))

			return nil
		},
	}

	return cmd
}

// NewKnowledgeVacuumCommand creates the `cosca knowledge vacuum` subcommand.
func NewKnowledgeVacuumCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vacuum",
		Short: "Clean up stale data",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()

			spinner := formatter.Spinner("Vacuuming knowledge base")
			spinner.Start()

			ke := NewKnowledgeEngine(dir)
			if ke != nil {
				if _, err := ke.Vacuum(); err != nil {
					formatter.Verbose(fmt.Sprintf("Vacuum warning: %v", err))
				}
			}

			spinner.Stop("Vacuum complete")

			if useJSON {
				return printJSON(cmd, map[string]string{"status": "ok"})
			}

			formatter.Success("Knowledge base vacuumed successfully")
			return nil
		},
	}

	return cmd
}

// NewKnowledgeExplainCommand creates the `cosca knowledge explain` subcommand.
func NewKnowledgeExplainCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "explain <result-id>",
		Short: "Explain why a result was returned",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)

			dir, _ := os.Getwd()

			ke := NewKnowledgeEngine(dir)
			if ke == nil {
				return fmt.Errorf("knowledge engine not available")
			}

			explanation, _ := ke.Explain(args[0])

			formatter.Header("Search Result Explanation")
			formatter.KeyValue("Result ID", args[0])
			formatter.KeyValue("Reason", strings.Join(explanation.MatchedTerms, ", "))
			formatter.KeyValue("Confidence", fmt.Sprintf("%.2f", explanation.Score))

			return nil
		},
	}

	return cmd
}

// NewKnowledgeCompileCommand creates the `cosca knowledge compile` subcommand.
func NewKnowledgeCompileCommand() *cobra.Command {
	var category string

	cmd := &cobra.Command{
		Use:   "compile",
		Short: "Compile knowledge from the repository into SQLite",
		Long: `Compile structured knowledge from the Knowledge Repository
(.cosca/framework/knowledge/) into the SQLite database (.cosca/knowledge.db).

The compiler reads Markdown and YAML files from the repository, parses
YAML frontmatter and headings, and indexes them with FTS5 full-text search.

Categories:
  patterns        Pattern documentation (Markdown with YAML frontmatter)
  heuristics      Heuristic rules in YAML format
  architecture    Architecture Decision Records (Markdown)
  failures        Post-mortem and incident reports (Markdown)
  best-practices  Best practice guides (Markdown)
  cognitive       Cognitive frameworks (YAML, Markdown)

Use --category to compile a single category, or omit to compile all.
`,
		Example: `  cosca knowledge compile                      # Compile all categories
  cosca knowledge compile --category heuristics # Compile only heuristics
  cosca knowledge compile --category patterns   # Compile only patterns
  cosca knowledge compile --json                # JSON output`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}

			repoPath := filepath.Join(dir, ".cosca", "fallback", "knowledge")
			dbPath := filepath.Join(dir, ".cosca", "knowledge.db")

			// Open SQLite database
			db, err := sql.Open("sqlite", dbPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer func() {
				if closeErr := db.Close(); closeErr != nil {
					formatter.Verbose(fmt.Sprintf("Warning closing database: %v", closeErr))
				}
			}()

			// Configure basic pragmas for concurrent safety
			if _, err := db.Exec("PRAGMA journal_mode = wal"); err != nil {
				formatter.Verbose(fmt.Sprintf("Warning setting journal mode: %v", err))
			}
			if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
				formatter.Verbose(fmt.Sprintf("Warning setting busy timeout: %v", err))
			}

			compiler := knowledge.NewCompiler(db, repoPath)

			spinner := formatter.Spinner("Compiling knowledge")
			spinner.Start()

			var result *knowledge.CompileResult
			if category != "" {
				result, err = compiler.CompileCategory(
					context.Background(),
					knowledge.KnowledgeCategory(category),
				)
			} else {
				result, err = compiler.Compile(context.Background())
			}

			if err != nil {
				spinner.Fail(fmt.Sprintf("Compile failed: %v", err))
				return fmt.Errorf("compile failed: %w", err)
			}

			spinner.Stop(fmt.Sprintf("Compiled %d entries", result.Total))

			if useJSON {
				return printJSON(cmd, result)
			}

			formatter.Println("")
			formatter.Header("Knowledge Compilation Results")
			formatter.KeyValue("Categories", strings.Join(result.Categories, ", "))
			formatter.KeyValue("Total entries", fmt.Sprintf("%d", result.Total))
			formatter.KeyValue("New", fmt.Sprintf("%d", result.New))
			formatter.KeyValue("Updated", fmt.Sprintf("%d", result.Updated))
			formatter.KeyValue("Skipped", fmt.Sprintf("%d", result.Skipped))
			formatter.KeyValue("Duration", fmt.Sprintf("%dms", result.Duration))

			if len(result.Errors) > 0 {
				formatter.Warning(fmt.Sprintf("%d errors occurred:", len(result.Errors)))
				for _, e := range result.Errors {
					formatter.Bullet(e)
				}
			} else {
				formatter.Success("Knowledge compiled successfully")
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&category, "category", "c", "",
		"Category to compile (patterns, heuristics, architecture, failures, best-practices, cognitive)")

	return cmd
}

// NewKnowledgeRevalidateCommand creates the `cosca knowledge revalidate`
// subcommand — conhecimento envelhecido via hash, revalidação determinística.
//
// Re-hash o arquivo local em ev.Path e compara com o ev.SHA256 registrado na
// evidência. Se diferem → a fonte mudou → item marcado como STALE (estado
// epistemológico CKL). NUNCA apaga nem promove — apenas marca o que precisa
// ser revalidado. `--dry-run` mostra o que mudaria sem gravar nada.
func NewKnowledgeRevalidateCommand() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "revalidate [id]",
		Short: "Revalida o conhecimento contra a fonte local (hash SHA-256)",
		Long: `Descobre quando o conhecimento envelheceu comparando o hash
SHA-256 do arquivo local em Path com o hash registrado na evidência.

Conforme o Don: "O Cosca aprendeu K-442 'API X funciona dessa maneira'. Meses
depois, GitHub → nova release → arquivo alterado → hash diferente. O sistema
detecta: K-442 STATUS: STALE." Sem argumento, revalida todos os itens; com um
ID, apenas aquele item.

O mecanismo é local e determinístico — não busca nada na rede. Se o hash
mudou, o item é marcado como STALE. NUNCA apaga nem promove/demote nível:
apenas diz "esse conhecimento precisa ser revalidado". --dry-run mostra o que
mudaria sem gravar nada.

Persistência: .cosca/knowledge/laws.json (runtime, gitignored).
`,
		Example: `  cosca knowledge revalidate
  cosca knowledge revalidate K-442
  cosca knowledge revalidate --dry-run`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			lawsPath, err := resolveLawsPath()
			if err != nil {
				return err
			}

			// Primeira execução: semear as 5 leis reais (idempotente).
			if _, err := ensureSeededLaws(lawsPath); err != nil {
				return err
			}

			engine, err := loadLawsEngine(lawsPath)
			if err != nil {
				return err
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getwd: %w", err)
			}

			// Persistimos sempre o conjunto completo (clones) para nunca
			// perder itens: apenas o(s) alvo(s) é(são) revalidado(s).
			items := engine.All()
			target := items
			if len(args) == 1 {
				found := false
				for _, it := range items {
					if it.ID == args[0] {
						target = []*knowledge.KnowledgeItem{it}
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("lei %q não encontrada em %s", args[0], lawsPath)
				}
			}

			if len(items) == 0 {
				formatter.Warning("Nenhuma lei registrada ainda — use \"cosca knowledge law add-evidence <id> ...\".")
				return nil
			}

			// Coleta determinística: ordem dos itens, ordem das evidências.
			results := make([]knowledge.AgingResult, 0, len(target))
			stale := 0
			dirty := false
			for _, item := range target {
				itemResults, err := knowledge.RevalidateItem(item, cwd)
				if err != nil {
					if len(args) == 1 {
						return err
					}
					formatter.Warning(fmt.Sprintf("%s: %v", item.ID, err))
					continue
				}
				if knowledge.ApplyAging(item, itemResults) {
					stale++
				}
				for _, res := range itemResults {
					if !res.Skipped {
						dirty = true
					}
					results = append(results, res)
				}
			}

			if useJSON {
				if err := printJSON(cmd, map[string]interface{}{
					"dry_run": dryRun,
					"stale":   stale,
					"results": results,
				}); err != nil {
					return err
				}
			} else {
				formatter.Header(fmt.Sprintf("Revalidação de conhecimento (%d item(ns))", len(target)))
				if dryRun {
					formatter.Warning("DRY-RUN: nada será gravado em disco.")
				}

				rows := make([][]string, 0, len(results))
				for _, res := range results {
					status := "OK"
					if res.Skipped {
						status = "SKIP"
					}
					if res.Changed {
						status = "STALE"
					}
					rows = append(rows, []string{
						res.ItemID,
						res.EvidenceID,
						res.Path,
						abbreviateSHA(res.RecordedHash),
						abbreviateSHA(res.CurrentHash),
						status,
					})
				}
				formatter.Table([]string{"Item", "Evidência", "Caminho", "SHA256 registrado", "SHA256 atual", "Status"}, rows)

				if stale > 0 {
					formatter.Warning("Esse conhecimento precisa ser revalidado.")
				}
			}

			// Gravação: apenas fora do dry-run e quando algo foi verificado.
			if !dryRun && dirty {
				out := knowledge.NewPromotionEngine()
				for _, item := range items {
					if err := out.Register(item); err != nil {
						return err
					}
				}
				if err := out.Save(lawsPath); err != nil {
					return fmt.Errorf("salvar %s: %w", lawsPath, err)
				}
				if !useJSON {
					formatter.Success(fmt.Sprintf("Revalidação gravada em %s", lawsPath))
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "mostra o que mudaria sem gravar nada")
	return cmd
}

// NewKnowledgeRelationsCommand creates the `cosca knowledge relations` subcommand.
func NewKnowledgeRelationsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "relations <entity>",
		Short: "Show entity relationships",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			g := NewGraph(".")
			if g == nil {
				return fmt.Errorf("graph engine not available")
			}

			relations, _ := g.GetRelations(args[0], 2)

			if useJSON {
				return printJSON(cmd, relations)
			}

			formatter.Header(fmt.Sprintf("Relations for %q", args[0]))
			for _, rel := range relations {
				formatter.KeyValue(rel.Type, rel.Target)
			}

			return nil
		},
	}

	return cmd
}

//
// `cosca knowledge add` + `cosca knowledge packages` — Knowledge Packages.
//
// O manifesto é genérico (sem "modo Next.js", "modo Prisma"): a biblioteca é
// apenas outro Knowledge Package que o Cosca CONHECE. A distinção é
// fundamental:
//
//	KNOWLEDGE ≠ DEPENDENCY ≠ EXECUTABLE CODE.
//
// `add` faz a descoberta heurística LOCAL (DetectPackageInfo, sem rede),
// mostra o rascunho e pergunta SÓ o que precisa decisão (versões a
// acompanhar, default ["*"]). NADA é instalado no projeto — o conhecimento
// fica em .cosca/knowledge/packages/<id>.json (runtime, gitignored).
//

// defaultPackageVersions é a família de versões a acompanhar quando o usuário
// não decide nada (input não interativo ou Enter).
var defaultPackageVersions = []string{"*"}

// defaultPackageSources devolve as fontes default do manifesto conforme o
// que foi detectado: repositório presente ⇒ docs oficiais + repositório
// oficial; caso contrário apenas docs oficiais.
func defaultPackageSources(pkg knowledge.KnowledgePackage) []string {
	if pkg.Repository != "" {
		return []string{"official-docs", "official-repository"}
	}
	return []string{"official-docs"}
}

// promptPackageVersions pergunta apenas o que precisa decisão: as famílias
// de versão a acompanhar. Default ["*"].
//
// Quando o input é o stdin do processo (não injetado) E não é um terminal
// (CI, pipes), aplica o default sem travar. Entrada explicitamente fornecida
// (scripts, testes via cmd.SetIn) ou um terminal real é lida.
func promptPackageVersions(cmd *cobra.Command) ([]string, error) {
	in := cmd.InOrStdin()
	if f, ok := in.(*os.File); ok && !term.IsTerminal(int(f.Fd())) {
		return append([]string(nil), defaultPackageVersions...), nil
	}

	if _, err := fmt.Fprint(cmd.OutOrStdout(), "\nVersões a acompanhar (default: *): "); err != nil {
		return nil, err
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("falha ao ler as versões: %w", err)
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return append([]string(nil), defaultPackageVersions...), nil
	}
	var versions []string
	for _, v := range strings.Split(line, ",") {
		if v = strings.TrimSpace(v); v != "" {
			versions = append(versions, v)
		}
	}
	if len(versions) == 0 {
		return append([]string(nil), defaultPackageVersions...), nil
	}
	return versions, nil
}

// NewKnowledgeAddCommand cria `cosca knowledge add <lib|repo>`.
func NewKnowledgeAddCommand() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "add <lib|repo>",
		Short: "Registra um Knowledge Package (manifesto) — conhecer, sem instalar",
		Long: `Registra um Knowledge Package (manifesto genérico de biblioteca) no
Knowledge Repository. Com --global/-g, registra em ~/.config/cosca/knowledge/packages/
(compartilhado entre todos os projetos); sem a flag, registra no .cosca/ local.

A descoberta é heurística e LOCAL (sem rede): deriva o id do nome/repo,
chuta o ecosystem por uma tabela estática best-effort e usa kind "library"
como default. O rascunho é mostrado e o comando pergunta SÓ o que precisa
decisão: as famílias de versão a acompanhar (default ["*"]).

A separação é explícita: KNOWLEDGE ≠ DEPENDENCY ≠ EXECUTABLE CODE. NADA é
instalado no projeto — o Cosca apenas passa a CONHECER a biblioteca. A
aquisição real (fontes, versões, coleta, hash + procedência, quarantine)
fica para o wire com "cosca evidence" (uma etapa posterior).`,
		Example: `  cosca knowledge add prisma
  cosca knowledge add github:pgx/pgx
  cosca knowledge add nestjs --global
  cosca knowledge add nestjs -g`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			// "cosca knowledge add *" — registra TODAS as dependências do projeto.
			if args[0] == "*" {
				return addAllProjectDeps(cmd, formatter, global, useJSON)
			}

			pkg, err := knowledge.DetectPackageInfo(args[0])
			if err != nil {
				return err
			}

			var store *knowledge.PackageStore
			var storeLabel string
			if global {
				store, err = knowledge.NewGlobalPackageStore()
				if err != nil {
					return fmt.Errorf("store global: %w", err)
				}
				storeLabel = fmt.Sprintf("~/.config/cosca/knowledge/packages/%s.json", pkg.ID)
			} else {
				store = knowledge.NewPackageStore(".")
				storeLabel = fmt.Sprintf(".cosca/knowledge/packages/%s.json", pkg.ID)
			}

			if store.Exists(pkg.ID) {
				return fmt.Errorf(
					"Knowledge Package %q já registrado em %s — use \"cosca knowledge packages show %s\"",
					pkg.ID, storeLabel, pkg.ID)
			}

			// Rascunho detectado (descoberta automática).
			scope := "local"
			if global {
				scope = "global"
			}
			formatter.Header(fmt.Sprintf("Knowledge Package detectado — %s (%s)", pkg.ID, scope))
			formatter.KeyValue("Nome", pkg.ID)
			formatter.KeyValue("Kind", pkg.Kind)
			formatter.KeyValue("Ecosystem", pkg.Ecosystem)
			if pkg.Repository != "" {
				formatter.KeyValue("Repository", pkg.Repository)
			}

			// Só o que precisa decisão: versões a acompanhar.
			versions, err := promptPackageVersions(cmd)
			if err != nil {
				return err
			}

			pkg.Versions = versions
			pkg.Sources = defaultPackageSources(pkg)
			pkg.KnowledgeLevel = knowledge.PackageKnowledgeNone
			pkg.Status = knowledge.PackageStatusManifest

			if err := store.Add(pkg); err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, pkg)
			}

			formatter.Success(fmt.Sprintf(
				"Knowledge Package %s registrado como manifesto em %s",
				pkg.ID, storeLabel))
			formatter.KeyValue("Kind", pkg.Kind)
			formatter.KeyValue("Ecosystem", pkg.Ecosystem)
			formatter.KeyValue("Versões", strings.Join(pkg.Versions, ", "))
			formatter.KeyValue("Status", pkg.Status)
			formatter.Warning("Conhecimento registrado. NADA foi instalado no projeto (KNOWLEDGE ≠ DEPENDENCY ≠ CODE).")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "Registrar no repositório global (~/.config/cosca/knowledge/packages/) — compartilhado entre todos os projetos")
	return cmd
}

// NewKnowledgeAcquireCommand cria `cosca knowledge acquire <id>`.
func NewKnowledgeAcquireCommand() *cobra.Command {
	var allowRemote bool
	var compile bool
	var embed bool
	var global bool
	var force bool

	cmd := &cobra.Command{
		Use:   "acquire <id|* >",
		Short: "Adquire a documentação oficial de um Knowledge Package e indexa no Knowledge Base",
		Long: `Faz o download da documentação oficial de um Knowledge Package registrado
e a indexa no Knowledge Base, permitindo buscas offline via "cosca knowledge search".

Com --global/-g, indexa no knowledge.db global (~/.config/cosca/knowledge.db),
compartilhado entre TODOS os projetos. Sem a flag, indexa no .cosca/knowledge.db local.

Use "*" para adquirir TODAS as dependências do projeto de uma vez:
  cosca knowledge acquire "*" --global --allow-remote --compile

Pipeline: manifest → fetch (SSRF-safe) → hash SHA-256 → salvar em markdown →
compilar no FTS5 → atualizar manifesto.

Após a aquisição, buscas como "cosca knowledge search nestjs controller"
retornam resultados LOCAIS (ou globais, com --global) — zero internet, zero LLM.

--allow-remote é obrigatório para buscar da internet (fail-closed).
Com --compile, roda o Knowledge Compiler para indexar o conteúdo no FTS5.`,
		Example: `  cosca knowledge acquire nestjs --allow-remote
  cosca knowledge acquire nestjs --allow-remote --compile
  cosca knowledge acquire nestjs --global --allow-remote --compile
  cosca knowledge acquire "*" --global --allow-remote --compile`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			// "cosca knowledge acquire *" — adquire TODAS as dependências do projeto.
			if args[0] == "*" {
				return acquireAllProjectDeps(cmd, formatter, global, allowRemote, force, useJSON)
			}

			pkgID := args[0]

			// Resolve store and kbDir based on scope.
			var targetStore *knowledge.PackageStore
			var kbDir string
			var scope string
			var err error

			if global {
				kbDir, err = coscaGlobalDir()
				if err != nil {
					return err
				}
				store, err := knowledge.NewGlobalPackageStore()
				if err != nil {
					return err
				}
				targetStore = store
				scope = "global"
			} else {
				dir, _ := os.Getwd()
				kbDir = filepath.Join(dir, ".cosca")
				targetStore = knowledge.NewPackageStore(".")
				scope = "local"
			}

			// Verify the package exists in the right store.
			_, err = targetStore.Get(pkgID)
			if err != nil {
				hint := "local"
				if global {
					hint = "global"
				}
				return fmt.Errorf("package %q não encontrado no repositório %s. Registre primeiro com: cosca knowledge add %s%s",
					pkgID, hint, pkgID, map[bool]string{true: " --global", false: ""}[global])
			}

			formatter.Verbose(fmt.Sprintf("Acquiring documentation for %s (%s)...", pkgID, scope))

			alreadyAcquired := false
			result, err := knowledge.AcquirePackage(cmd.Context(), targetStore, pkgID, kbDir, allowRemote, force)
			if err != nil {
				return err
			}
			alreadyAcquired = result.Status == "already_acquired"

			if useJSON {
				return printJSON(cmd, result)
			}

			if alreadyAcquired && !compile && !embed {
				formatter.Success(fmt.Sprintf("%s — documentação já adquirida (%s)", pkgID, scope))
				formatter.KeyValue("Dica", "use --compile --embed para reindexar")
				return nil
			}

			if !alreadyAcquired {
				formatter.Success(fmt.Sprintf("%s — documentação adquirida (%s)", pkgID, scope))
				formatter.KeyValue("Source", result.SourceURL)
				formatter.KeyValue("Artifact", result.ArtifactID)
				formatter.KeyValue("SHA-256", result.SHA256[:16]+"...")
				formatter.KeyValue("Size", fmt.Sprintf("%d KiB", result.SizeBytes>>10))
				if result.MarkdownPath != "" {
					formatter.KeyValue("Knowledge file", result.MarkdownPath)
				}
				for _, w := range result.Warnings {
					formatter.Warning(w)
				}
			} else {
				formatter.Success(fmt.Sprintf("%s — reindexando (%s)", pkgID, scope))
				for _, w := range result.Warnings {
					formatter.Warning(w)
				}
			}

			if compile || embed {
				acquiredDir := filepath.Join(kbDir, "knowledge", "acquired")
				what := "FTS5"
				if embed {
					what = "FTS5 + embeddings"
				}
				formatter.Verbose(fmt.Sprintf("Indexing acquired knowledge (%s)...", what))
				ke, keErr := openKnowledgeEngine(kbDir, global, "")
				if keErr != nil {
					formatter.Warning(fmt.Sprintf("engine: %v — rode 'cosca knowledge rebuild' manualmente", keErr))
				} else if err := ke.Init(); err != nil {
					formatter.Warning(fmt.Sprintf("engine init: %v — rode 'cosca knowledge rebuild' manualmente", err))
				} else if err := ke.IndexDirectory(cmd.Context(), acquiredDir); err != nil {
					formatter.Warning(fmt.Sprintf("index: %v — rode 'cosca knowledge rebuild' manualmente", err))
				} else {
					ke.Close()
					formatter.Success(fmt.Sprintf("Indexado (%s) — busca local ativada", what))
				}
			} else {
				formatter.KeyValue("Próximo passo", "cosca knowledge acquire "+pkgID+" --compile")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&allowRemote, "allow-remote", false, "Permitir fetch da internet (fail-closed: obrigatório)")
	cmd.Flags().BoolVar(&compile, "compile", false, "Compilar no Knowledge Base FTS5 após adquirir")
	cmd.Flags().BoolVar(&embed, "embed", false, "Gerar embeddings (busca semântica/vetorial) após compilar")
	cmd.Flags().BoolVarP(&global, "global", "g", false, "Indexar no knowledge.db global (~/.config/cosca/) — compartilhado entre todos os projetos")
	cmd.Flags().BoolVar(&force, "force", false, "Forçar aquisição mesmo se o repositório falhar na verificação de segurança (ex: muitos issues abertos)")
	return cmd
}

// openCompiler abre o Knowledge Compiler para o projeto corrente.
func openCompiler(kbDir string) (*knowledge.Compiler, error) {
	dbPath := filepath.Join(kbDir, "knowledge.db")
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	repoPath := filepath.Join(kbDir, "knowledge")
	return knowledge.NewCompiler(db, repoPath), nil
}

// openKnowledgeEngine cria um Knowledge Engine com o DB path correto.
// Para global, o db está em kbDir/knowledge.db diretamente (sem .cosca/).
//
// rootOverride, quando não vazio, define o RootDir do engine (o sandbox de
// contenção de paths do indexer). Vazio = kbDir, comportamento padrão.
func openKnowledgeEngine(kbDir string, global bool, rootOverride string) (*knowledge.Engine, error) {
	dbPath := filepath.Join(kbDir, "knowledge.db")
	rootDir := kbDir
	if rootOverride != "" {
		rootDir = rootOverride
	}
	var (
		embeddingProvider   string
		embeddingBaseURL    string
		embeddingModel      string
		embeddingDigest     string
		embeddingAPIKey     string
		embeddingDimensions int
		embeddingBatchSize  int
	)
	if c, err := config.Load(); err == nil {
		embeddingProvider = c.Embedding.Provider
		embeddingBaseURL = c.Embedding.BaseURL
		embeddingModel = c.Embedding.Model
		embeddingDigest = c.Embedding.Digest
		embeddingAPIKey = c.Embedding.APIKey
		embeddingDimensions = c.Embedding.Dimensions
		embeddingBatchSize = c.Embedding.BatchSize
	}
	return knowledge.New(knowledge.Config{
		DBPath:              dbPath,
		RootDir:             rootDir,
		AutoMigrate:         true,
		EmbeddingProvider:   embeddingProvider,
		EmbeddingBaseURL:    embeddingBaseURL,
		EmbeddingModel:      embeddingModel,
		EmbeddingDigest:     embeddingDigest,
		EmbeddingAPIKey:     embeddingAPIKey,
		EmbeddingDimensions: embeddingDimensions,
		IndexerConfig: indexer.IndexerConfig{
			EmbedBatchSize: embeddingBatchSize,
		},
	})
}

// NewKnowledgePackagesCommand cria o grupo `cosca knowledge packages`.
func NewKnowledgePackagesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "packages",
		Short: "Gerencia os Knowledge Packages registrados (manifestos)",
		Long: `Gerencia os Knowledge Packages do Knowledge Repository
(.cosca/knowledge/packages/<id>.json, runtime, gitignored).

Subcomandos:
  list                     Tabela de todos os manifestos (ID | Kind | Ecosystem | Versões | Status)
  show <id>                Manifesto completo de um pacote
  status <id>              Resumo do status de um pacote`,
		Example: `  cosca knowledge packages list
  cosca knowledge packages show prisma
  cosca knowledge packages status prisma`,
	}

	cmd.AddCommand(
		NewKnowledgePackagesListCommand(),
		NewKnowledgePackagesShowCommand(),
		NewKnowledgePackagesStatusCommand(),
	)
	return cmd
}

// NewKnowledgePackagesListCommand cria `cosca knowledge packages list`.
func NewKnowledgePackagesListCommand() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lista os Knowledge Packages registrados",
		Long: `Lista todos os manifestos de Knowledge Package numa tabela:
ID | Kind | Ecosystem | Versões | Status. Com --global/-g, lista
os packages do repositório global (~/.config/cosca/).`,
		Example: `  cosca knowledge packages list
  cosca knowledge packages list --global
  cosca knowledge packages list -g --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			var pkgs []knowledge.KnowledgePackage
			var err error
			var scope string

			if global {
				store, gErr := knowledge.NewGlobalPackageStore()
				if gErr != nil {
					return gErr
				}
				pkgs, err = store.List()
				scope = "globais"
			} else {
				local := knowledge.NewPackageStore(".")
				gstore, _ := knowledge.NewGlobalPackageStore()
				pkgs, err = knowledge.FederatedList(local, gstore)
				scope = "locais + globais"
			}

			if err != nil {
				return err
			}
			if useJSON {
				return printJSON(cmd, pkgs)
			}
			if len(pkgs) == 0 {
				formatter.Warning("Nenhum Knowledge Package registrado — use \"cosca knowledge add <lib|repo>\".")
				return nil
			}
			formatter.Header(fmt.Sprintf("Knowledge Packages registrados — %s (%d)", scope, len(pkgs)))
			rows := make([][]string, 0, len(pkgs))
			for _, p := range pkgs {
				rows = append(rows, []string{
					p.ID,
					p.Kind,
					p.Ecosystem,
					strings.Join(p.Versions, ", "),
					p.Status,
				})
			}
			formatter.Table([]string{"ID", "Kind", "Ecosystem", "Versões", "Status"}, rows)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "Listar apenas os packages globais (~/.config/cosca/)")
	return cmd
}

// NewKnowledgePackagesShowCommand cria `cosca knowledge packages show <id>`.
func NewKnowledgePackagesShowCommand() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Mostra o manifesto completo de um Knowledge Package",
		Example: `  cosca knowledge packages show prisma
  cosca knowledge packages show nestjs -g
  cosca knowledge packages show prisma --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			var pkg *knowledge.KnowledgePackage
			var err error

			if global {
				store, gErr := knowledge.NewGlobalPackageStore()
				if gErr != nil {
					return gErr
				}
				pkg, err = store.Get(args[0])
			} else {
				pkg, err = knowledge.FederatedGet(
					knowledge.NewPackageStore("."),
					mustGlobalStore(),
					args[0])
			}

			if err != nil {
				return err
			}
			if useJSON {
				return printJSON(cmd, pkg)
			}

			formatter.Header(fmt.Sprintf("Knowledge Package %s", pkg.ID))
			formatter.KeyValue("ID", pkg.ID)
			formatter.KeyValue("Kind", pkg.Kind)
			formatter.KeyValue("Ecosystem", pkg.Ecosystem)
			formatter.KeyValue("Versões", strings.Join(pkg.Versions, ", "))
			formatter.KeyValue("Fontes", strings.Join(pkg.Sources, ", "))
			formatter.KeyValue("Nível de conhecimento", pkg.KnowledgeLevel)
			if pkg.Repository != "" {
				formatter.KeyValue("Repository", pkg.Repository)
			}
			if pkg.License != "" {
				formatter.KeyValue("License", pkg.License)
			}
			if len(pkg.DependsOn) > 0 {
				formatter.KeyValue("Depende de", strings.Join(pkg.DependsOn, ", "))
			}
			if len(pkg.ArtifactIDs) > 0 {
				formatter.KeyValue("Artefatos", strings.Join(pkg.ArtifactIDs, ", "))
			}
			if !pkg.AcquiredAt.IsZero() {
				formatter.KeyValue("Adquirido em", pkg.AcquiredAt.Format(time.RFC3339))
			}
			formatter.KeyValue("Status", pkg.Status)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "Buscar apenas no repositório global")
	return cmd
}

// NewKnowledgePackagesStatusCommand cria `cosca knowledge packages status <id>`.
func NewKnowledgePackagesStatusCommand() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "status <id>",
		Short: "Resumo do status de um Knowledge Package",
		Long: `Resumo do status de um Knowledge Package: status do ciclo de vida
(manifest → acquiring → quarantined → validated), nível de conhecimento
(none/partial/validated), versões acompanhadas, fontes e artefatos A-XXXX.
Com --global/-g, busca no repositório global.`,
		Example: `  cosca knowledge packages status prisma
  cosca knowledge packages status nestjs -g
  cosca knowledge packages status prisma --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			var pkg *knowledge.KnowledgePackage
			var err error

			if global {
				store, gErr := knowledge.NewGlobalPackageStore()
				if gErr != nil {
					return gErr
				}
				pkg, err = store.Get(args[0])
			} else {
				pkg, err = knowledge.FederatedGet(
					knowledge.NewPackageStore("."),
					mustGlobalStore(),
					args[0])
			}

			if err != nil {
				return err
			}

			acquired := "nunca"
			if !pkg.AcquiredAt.IsZero() {
				acquired = pkg.AcquiredAt.Format(time.RFC3339)
			}

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"id":              pkg.ID,
					"kind":            pkg.Kind,
					"ecosystem":       pkg.Ecosystem,
					"status":          pkg.Status,
					"knowledge_level": pkg.KnowledgeLevel,
					"versions":        pkg.Versions,
					"sources":         pkg.Sources,
					"repository":      pkg.Repository,
					"artifact_count":  len(pkg.ArtifactIDs),
					"acquired_at":     acquired,
				})
			}

			formatter.Header(fmt.Sprintf("Status — Knowledge Package %s", pkg.ID))
			formatter.KeyValue("Status", pkg.Status)
			formatter.KeyValue("Nível de conhecimento", pkg.KnowledgeLevel)
			formatter.KeyValue("Ecosystem", pkg.Ecosystem)
			formatter.KeyValue("Versões", strings.Join(pkg.Versions, ", "))
			formatter.KeyValue("Fontes", strings.Join(pkg.Sources, ", "))
			if pkg.Repository != "" {
				formatter.KeyValue("Repository", pkg.Repository)
			}
			formatter.KeyValue("Artefatos", fmt.Sprintf("%d (A-XXXX)", len(pkg.ArtifactIDs)))
			formatter.KeyValue("Adquirido em", acquired)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "Buscar apenas no repositório global")
	return cmd
}

// NewKnowledgeDiffCommand cria `cosca knowledge diff <pkg> <from> <to>`.
func NewKnowledgeDiffCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "diff <pkg> <from> <to>",
		Short: "O que mudou entre versões de um Knowledge Package (Knowledge Diff)",
		Long: `O que mudou no conhecimento que o Cosca já tinha entre duas versões
de um Knowledge Package.

Cruza o VersionDiffs do manifesto (a fonte determinística do diff, registrada
por um humano ou pelo pipeline de aquisição) com os KnowledgeItems do CKL
(cujas evidências referenciam o pacote) e responde: o que mudou, o risco e o
impacto no projeto. Determinístico, sem LLM — o Cosca NUNCA inventa um diff.

Conforme o Don: "Prisma 6.0 → 6.1: NEW + recurso X, CHANGED ~ comportamento
Z, DEPRECATED - API A, REMOVED - API B, RISK ⚠️ 3 padrões existentes podem
ser afetados, AFFECTED KNOWLEDGE K-182 K-219 K-441. E dá para cruzar com o
projeto: 'Chef, a atualização altera uma API utilizada em 4 pontos do
projeto. Não atualizei nada. Preparei a análise.'"

Persistência: .cosca/knowledge/packages/<id>.json (manifesto, runtime) e
.cosca/knowledge/laws.json (KnowledgeItems do CKL, runtime).`,
		Example: `  cosca knowledge diff prisma 6.0 6.1
  cosca knowledge diff prisma 6.0 6.1 --json`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			pkg, err := knowledge.NewPackageStore(".").Get(args[0])
			if err != nil {
				return err
			}

			d, err := pkg.Diff(args[1], args[2])
			if err != nil {
				return err
			}

			// KnowledgeItems do CKL (laws.json runtime). Leitura pura: arquivo
			// ausente ⇒ nenhum item, não é erro.
			lawsPath, err := resolveLawsPath()
			if err != nil {
				return err
			}
			engine, err := loadLawsEngine(lawsPath)
			if err != nil {
				return err
			}
			all := engine.All()
			items := make([]knowledge.KnowledgeItem, 0, len(all))
			deps := append([]string(nil), pkg.DependsOn...)
			for _, it := range all {
				items = append(items, *it)
				// Dependências do projeto: best-effort, "ou itens relacionados".
				deps = append(deps, it.Title)
				for _, ev := range it.Evidence {
					deps = append(deps, ev.Description, ev.Source)
				}
			}

			assessed := pkg.AssessRisk(d, items)
			assessed.ProjectImpact = pkg.ProjectImpact(&assessed, deps)

			if useJSON {
				return printJSON(cmd, assessed)
			}

			formatter.Header(fmt.Sprintf("Knowledge Diff — %s", assessed.PackageID))
			formatter.KeyValue("Versão", fmt.Sprintf("%s → %s", assessed.FromVersion, assessed.ToVersion))
			formatter.Println("")

			if len(assessed.Entries) == 0 {
				formatter.Warning(assessed.Summary)
				formatter.KeyValue("Risk", assessed.RiskLevel)
				formatter.KeyValue("Impacto no projeto", fmt.Sprintf("%d pontos", assessed.ProjectImpact))
				return nil
			}

			sections := []struct {
				label string
				kind  string
				icon  string
			}{
				{"NEW", knowledge.DiffKindNew, "+"},
				{"CHANGED", knowledge.DiffKindChanged, "~"},
				{"DEPRECATED", knowledge.DiffKindDeprecated, "-"},
				{"REMOVED", knowledge.DiffKindRemoved, "-"},
			}
			for _, sec := range sections {
				shown := false
				for _, e := range assessed.Entries {
					if e.Kind != sec.kind {
						continue
					}
					if !shown {
						formatter.Header(sec.label)
						shown = true
					}
					line := fmt.Sprintf("  %s %s", sec.icon, e.Symbol)
					if e.Detail != "" {
						line += " — " + e.Detail
					}
					formatter.Println(line)
				}
			}

			formatter.Println("")
			riskLine := fmt.Sprintf("RISK: %s", assessed.RiskLevel)
			if assessed.RiskLevel == knowledge.RiskMedium || assessed.RiskLevel == knowledge.RiskHigh {
				riskLine += " ⚠️ " + assessed.Summary
			}
			formatter.Warning(riskLine)

			if len(assessed.AffectedKnowledge) > 0 {
				formatter.KeyValue("AFFECTED KNOWLEDGE", strings.Join(assessed.AffectedKnowledge, ", "))
			}
			formatter.KeyValue("Impacto no projeto", fmt.Sprintf("%d pontos", assessed.ProjectImpact))

			// Aviso pt-BR para o Don quando o risco é médio ou maior.
			if assessed.RiskLevel == knowledge.RiskMedium || assessed.RiskLevel == knowledge.RiskHigh {
				formatter.Warning(fmt.Sprintf(
					"Chef, a atualização altera uma API utilizada em %d pontos. Não atualizei nada. Preparei a análise.",
					assessed.ProjectImpact))
			}

			return nil
		},
	}
}

// matchStatusLabel devolve o rótulo pt-BR com ícone do status do match.
func matchStatusLabel(s knowledge.MatchStatus) string {
	switch s {
	case knowledge.MatchVerified:
		return "✓ verificado"
	case knowledge.MatchStale:
		return "⚠ stale"
	case knowledge.MatchPartial:
		return "◐ parcial"
	case knowledge.MatchMissing:
		return "✗ sem conhecimento"
	}
	return string(s)
}

// NewKnowledgeMatchCommand cria `cosca knowledge match [dir]` — Projeto →
// Dependency Detection → Knowledge Matching.
//
// Detecta as dependências do projeto (package.json/go.mod/Cargo.toml/etc.,
// read-only via discovery.DetectProject), carrega a PackageStore do projeto
// (.cosca/knowledge/packages) e cruza cada dependência com o conhecimento:
// "Tenho conhecimento verificado?" → SIM → usa / STALE → revalida / NÃO →
// oferece aquisição. NUNCA instala nada.
func NewKnowledgeMatchCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "match [dir]",
		Short: "Casa as dependências do projeto com o conhecimento conhecido (Projeto → Dependency Detection → Knowledge Matching)",
		Long: `Detecta as dependências do projeto (package.json, go.mod, Cargo.toml,
etc.) e cruza cada uma com os Knowledge Packages registrados em
.cosca/knowledge/packages/.

Conforme o Don: "Projeto → Dependency Detection → Knowledge Matching →
'Tenho conhecimento verificado?' → SIM → usa / STALE → revalida / NÃO →
oferece aquisição. Aí adicionar Prisma, Zod, Gin, Fiber, Tokio etc. deixa de
ser desenvolver uma feature para cada biblioteca."

A detecção de dependências é READ-ONLY (discovery) e o matching é
determinístico e local (sem LLM): cada dependência é normalizada e procurada
na PackageStore. NADA é instalado — apenas diz o que o Cosca sabe (e sabe que
NÃO sabe) sobre cada dependência.

Sem [dir], usa o diretório corrente. --json emite a lista de DependencyMatch.`,
		Example: `  cosca knowledge match
  cosca knowledge match ./meu-projeto
  cosca knowledge match --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}

			// Projeto → Dependency Detection (read-only, sem rede).
			info, err := discovery.DetectProject(cmd.Context(), dir, zerolog.Nop())
			if err != nil {
				return fmt.Errorf("detectar projeto %q: %w", dir, err)
			}

			// Knowledge Matching contra a PackageStore do projeto + global.
			globalStore, _ := knowledge.NewGlobalPackageStore()
			matches, err := knowledge.MatchProject(info.Dependencies, knowledge.NewPackageStore(dir), globalStore)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, matches)
			}

			formatter.Header(fmt.Sprintf("Knowledge Matching — %s", info.Name))
			if len(matches) == 0 {
				formatter.Warning("Nenhuma dependência detectada em " + dir + ".")
				return nil
			}

			rows := make([][]string, 0, len(matches))
			verified := 0
			missing := 0
			for _, m := range matches {
				if m.Status == knowledge.MatchVerified {
					verified++
				}
				if m.Status == knowledge.MatchMissing {
					missing++
				}
				rows = append(rows, []string{
					m.Dependency,
					m.PackageID,
					matchStatusLabel(m.Status),
					m.Suggest(),
				})
			}

			formatter.Table([]string{"Dependência", "Pacote", "Status", "Ação sugerida"}, rows)
			formatter.Println("")
			formatter.Println(fmt.Sprintf(
				"%d dependências · %d com conhecimento verificado · %d sem conhecimento",
				len(matches), verified, missing))
			return nil
		},
	}
}

// resolveProjectDeps devolve as dependências do projeto com a versão quando o
// manifesto permite ("prisma" → "prisma@6.1"), lendo package.json/go.mod
// (read-only). Determinístico e sem rede. Best-effort: sem versão detectável,
// devolve o nome cru como discovery.DetectProject o reportou.
func resolveProjectDeps(dir string, names []string) []string {
	versions := map[string]string{}
	if data, err := os.ReadFile(filepath.Join(dir, "package.json")); err == nil {
		var pkg struct {
			Dependencies    map[string]string `json:"dependencies"`
			DevDependencies map[string]string `json:"devDependencies"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			for k, v := range pkg.Dependencies {
				versions[strings.ToLower(k)] = v
			}
			for k, v := range pkg.DevDependencies {
				versions[strings.ToLower(k)] = v
			}
		}
	}
	if data, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 2 && strings.Contains(fields[0], "/") {
				versions[strings.ToLower(fields[0])] = fields[1]
			}
		}
	}

	deps := make([]string, 0, len(names))
	for _, n := range names {
		if v, ok := versions[strings.ToLower(n)]; ok && v != "" {
			deps = append(deps, n+"@"+v)
		} else {
			deps = append(deps, n)
		}
	}
	return deps
}

// NewKnowledgeResolveCommand cria `cosca knowledge resolve <task> [--dir .]` —
// Tarefa → Knowledge Resolver → Gap Detection (aquisição adaptativa).
//
// Detecta as dependências do projeto (discovery, read-only), cruza com a
// PackageStore (MatchProject) e decide "tenho conhecimento suficiente?":
//
//	SIM → "Conhecimento suficiente — continua"
//	NÃO → tabela de lacunas (pacote, versão, API, razão, fontes sugeridas) +
//	      próxima ação ("adquirir: cosca knowledge add prisma")
//
// Determinístico e local (sem rede, sem LLM — o wire de aquisição vem depois).
func NewKnowledgeResolveCommand() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "resolve <task>",
		Short: "Tarefa → Knowledge Resolver → 'tenho conhecimento suficiente?' (Gap Detection)",
		Long: `Resolve uma tarefa contra o conhecimento conhecido e responde
"tenho conhecimento suficiente?" antes de continuar.

Conforme o Don: "Tarefa → Knowledge Resolver → 'Tenho conhecimento suficiente?'
→ SIM → continua / NÃO → Gap Detection → Knowledge Acquisition → Sources →
Evidence → Validation → Knowledge → continua a tarefa. O agente recebe
'Implemente X usando Prisma 6.1'. O resolver verifica: Projeto: Prisma 6.1,
Conhecimento: Prisma 6.0 ✓, Prisma 6.1 ✗, API X ? → Knowledge gap detected:
Missing Prisma 6.1 / API X."

As dependências vêm de discovery.DetectProject (read-only) com a versão lida de
package.json/go.mod; o matching e a decisão são determinísticos e locais (sem
rede, sem LLM). Com lacunas, a tabela mostra pacote, versão, API, razão (pt-BR)
e as fontes sugeridas (official-docs → official-repository → release-notes) +
a próxima ação de aquisição.

Sem --dir, usa o diretório corrente. --json emite o ResolveResult completo.`,
		Example: `  cosca knowledge resolve "create transaction"
  cosca knowledge resolve "create transaction" --dir ./meu-projeto
  cosca knowledge resolve "create transaction" --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			// Projeto → Dependency Detection (read-only, sem rede).
			info, err := discovery.DetectProject(cmd.Context(), dir, zerolog.Nop())
			if err != nil {
				return fmt.Errorf("detectar projeto %q: %w", dir, err)
			}

			// Knowledge Resolver: deps com versão + PackageStore do projeto + global.
			deps := resolveProjectDeps(dir, info.Dependencies)
			globalStore, _ := knowledge.NewGlobalPackageStore()
			result, err := knowledge.ResolveTask(args[0], deps, knowledge.NewPackageStore(dir), globalStore)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, result)
			}

			formatter.Header(fmt.Sprintf("Knowledge Resolver — %q", args[0]))
			if result.Sufficient {
				formatter.Success("Conhecimento suficiente — continua")
				if msg := result.VoiceSummary(); msg != "" {
					formatter.KeyValue("🔊", msg)
				}
				return nil
			}

			rows := make([][]string, 0, len(result.Gaps))
			for _, g := range result.Gaps {
				rows = append(rows, []string{
					g.PackageID,
					g.Version,
					g.API,
					g.Reason,
					strings.Join(g.Sources, ", "),
				})
			}
			formatter.Table([]string{"Pacote", "Versão", "API", "Razão", "Fontes sugeridas"}, rows)
			formatter.KeyValue("Próxima ação", result.NextAction)
			if msg := result.VoiceSummary(); msg != "" {
				formatter.KeyValue("🔊", msg)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", ".", "diretório do projeto")
	return cmd
}

// mustGlobalStore retorna a store global ou nil (erro é silencioso — a
// store global é um bônus, nunca um bloqueio).
func mustGlobalStore() *knowledge.PackageStore {
	store, err := knowledge.NewGlobalPackageStore()
	if err != nil {
		return nil
	}
	return store
}

// searchGlobalKnowledge busca nos arquivos de conhecimento globais
// (~/.config/cosca/knowledge/acquired/). Leitura direta dos markdowns,
// sem dependência de schema SQLite.
func searchGlobalKnowledge(query string, limit int) []KnowledgeSearchResult {
	// UserHomeDir pode retornar o projeto dentro da jail.
	// Tenta caminhos reais primeiro, depois UserHomeDir.
	acquiredDir := filepath.Join("/home/cosca/.config/cosca", "knowledge", "acquired")
	if _, err := os.Stat(acquiredDir); os.IsNotExist(err) {
		if dir, dErr := coscaGlobalDir(); dErr == nil {
			acquiredDir = filepath.Join(dir, "knowledge", "acquired")
		}
	}
	if _, err := os.Stat(acquiredDir); os.IsNotExist(err) {
		return nil
	}

	queryLower := strings.ToLower(query)
	var results []KnowledgeSearchResult

	filepath.Walk(acquiredDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		content := string(data)
		if !strings.Contains(strings.ToLower(content), queryLower) {
			return nil
		}

		// Extract title from markdown heading.
		title := filepath.Base(filepath.Dir(path)) // package name from dir
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "# ") {
				title = strings.TrimPrefix(line, "# ")
				break
			}
		}

		// Find the query position and extract snippet.
		idx := strings.Index(strings.ToLower(content), queryLower)
		start := idx - 60
		if start < 0 {
			start = 0
		}
		end := idx + len(query) + 240
		if end > len(content) {
			end = len(content)
		}
		snippet := content[start:end]
		if start > 0 {
			snippet = "..." + snippet
		}
		if end < len(content) {
			snippet = snippet + "..."
		}

		results = append(results, KnowledgeSearchResult{
			Title:   title,
			Type:    "acquired",
			Score:   0.7,
			Snippet: snippet,
		})
		return nil
	})

	// Sort by score descending (all have same score, but keep consistent).
	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

// mergeSearchResults mescla resultados locais e globais, removendo duplicatas
// por título e preservando o maior score.
func mergeSearchResults(local, global []KnowledgeSearchResult, maxResults int) []KnowledgeSearchResult {
	seen := make(map[string]float64)
	var merged []KnowledgeSearchResult

	for _, r := range local {
		seen[r.Title] = r.Score
		merged = append(merged, r)
	}
	for _, r := range global {
		if existing, ok := seen[r.Title]; ok {
			if r.Score > existing {
				// Replace: remove old, add new with higher score.
				for i, m := range merged {
					if m.Title == r.Title {
						merged[i] = r
						break
					}
				}
				seen[r.Title] = r.Score
			}
		} else {
			seen[r.Title] = r.Score
			merged = append(merged, r)
		}
	}

	// Sort by score descending.
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})

	if len(merged) > maxResults {
		merged = merged[:maxResults]
	}
	return merged
}

// addAllProjectDeps registra todas as dependências do projeto como
// Knowledge Packages. Usa discovery.DetectProject para detectar deps,
// depois registra as que ainda não existem. Non-interactive — usa
// defaults (versions: ["*"]).
func addAllProjectDeps(cmd *cobra.Command, formatter *OutputFormatter, global bool, useJSON bool) error {
	dir, _ := os.Getwd()

	info, err := discovery.DetectProject(cmd.Context(), dir, zerolog.Nop())
	if err != nil {
		return fmt.Errorf("detectar projeto: %w", err)
	}

	if len(info.Dependencies) == 0 {
		formatter.Warning("Nenhuma dependência detectada no projeto.")
		return nil
	}

	var store *knowledge.PackageStore
	var scope string
	if global {
		store, err = knowledge.NewGlobalPackageStore()
		if err != nil {
			return err
		}
		scope = "global"
	} else {
		store = knowledge.NewPackageStore(".")
		scope = "local"
	}

	added := 0
	skipped := 0
	for _, dep := range info.Dependencies {
		pkg, pErr := knowledge.DetectPackageInfo(dep)
		if pErr != nil {
			continue
		}
		if store.Exists(pkg.ID) {
			skipped++
			continue
		}
		pkg.Versions = []string{"*"}
		pkg.Sources = defaultPackageSources(pkg)
		pkg.KnowledgeLevel = knowledge.PackageKnowledgeNone
		pkg.Status = knowledge.PackageStatusManifest
		if aErr := store.Add(pkg); aErr != nil {
			formatter.Warning(fmt.Sprintf("%s: %v", pkg.ID, aErr))
		} else {
			added++
		}
	}

	if useJSON {
		return printJSON(cmd, map[string]int{"added": added, "skipped": skipped, "total": len(info.Dependencies)})
	}

	formatter.Success(fmt.Sprintf("Projeto: %s", info.Name))
	formatter.KeyValue("Dependências detectadas", fmt.Sprintf("%d", len(info.Dependencies)))
	formatter.KeyValue(fmt.Sprintf("Registradas (%s)", scope), fmt.Sprintf("%d", added))
	formatter.KeyValue("Já existiam", fmt.Sprintf("%d", skipped))
	return nil
}

// acquireAllProjectDeps adquire todas as dependências do projeto detectadas
// via discovery.DetectProject. Para cada dependência, registra o manifesto
// (se necessário) com repo auto-resolvido e depois faz a aquisição.
func acquireAllProjectDeps(cmd *cobra.Command, formatter *OutputFormatter, global, allowRemote, force, useJSON bool) error {
	if !allowRemote {
		return fmt.Errorf("--allow-remote é obrigatório para aquisição em lote")
	}

	info, err := discovery.DetectProject(cmd.Context(), ".", zerolog.Nop())
	if err != nil {
		return fmt.Errorf("detecção do projeto: %w", err)
	}

	if len(info.Dependencies) == 0 {
		formatter.Warning("Nenhuma dependência detectada no projeto.")
		return nil
	}

	var store *knowledge.PackageStore
	var kbDir string
	var scope string

	if global {
		home, hErr := os.UserHomeDir()
		if hErr != nil {
			return fmt.Errorf("home dir: %w", hErr)
		}
		kbDir = filepath.Join(home, ".config", "cosca")
		store, err = knowledge.NewGlobalPackageStore()
		if err != nil {
			return err
		}
		scope = "global"
	} else {
		dir, _ := os.Getwd()
		kbDir = filepath.Join(dir, ".cosca")
		store = knowledge.NewPackageStore(".")
		scope = "local"
	}

	formatter.Header(fmt.Sprintf("Acquiring all dependencies — %s (%s)", info.Name, scope))
	formatter.KeyValue("Dependências detectadas", fmt.Sprintf("%d", len(info.Dependencies)))

	acquired := 0
	skipped := 0
	failed := 0

	for _, dep := range info.Dependencies {
		pkg, pErr := knowledge.DetectPackageInfo(dep)
		if pErr != nil {
			failed++
			continue
		}

		// Skip if already acquired or validated
		if existing, gErr := store.Get(pkg.ID); gErr == nil && (existing.Status == knowledge.PackageStatusAcquired || existing.Status == knowledge.PackageStatusValidated) {
			skipped++
			continue
		}

		// Auto-resolve repo if missing or clearly wrong (npm scope pattern)
		if pkg.Repository == "" || !strings.Contains(pkg.Repository, "/") || strings.HasPrefix(pkg.Repository, "@") {
			pkg.Repository = autoResolveRepo(pkg.ID, dep, pkg.Ecosystem)
		}

		// Register in JSON store if not present, or update repo if missing
		if !store.Exists(pkg.ID) {
			pkg.Versions = []string{"*"}
			pkg.Sources = defaultPackageSources(pkg)
			pkg.KnowledgeLevel = knowledge.PackageKnowledgeNone
			pkg.Status = knowledge.PackageStatusManifest
			if aErr := store.Add(pkg); aErr != nil {
				formatter.Warning(fmt.Sprintf("%s: registro falhou — %v", pkg.ID, aErr))
				failed++
				continue
			}
		} else if existing, gErr := store.Get(pkg.ID); gErr == nil && (existing.Repository == "" || !strings.Contains(existing.Repository, "/") || strings.HasPrefix(existing.Repository, "@")) {
			// Update repo in existing manifest
			existing.Repository = pkg.Repository
			store.Add(*existing)
		}

		// Acquire
		formatter.Verbose(fmt.Sprintf("  %s → %s", pkg.ID, pkg.Repository))
		result, aErr := knowledge.AcquirePackage(cmd.Context(), store, pkg.ID, kbDir, allowRemote, force)
		if aErr != nil {
			formatter.Warning(fmt.Sprintf("%s: %v", pkg.ID, aErr))
			failed++
		} else {
			acquired++
			_ = result
		}
		// Throttle: GitHub secondary rate limit (900 pts/min). 800ms = ~75 req/min.
		time.Sleep(800 * time.Millisecond)
	}

	if useJSON {
		return printJSON(cmd, map[string]int{
			"acquired": acquired, "skipped": skipped, "failed": failed,
			"total": len(info.Dependencies),
		})
	}

	formatter.Success(fmt.Sprintf("Adquiridos: %d | Pulados: %d | Falhas: %d | Total: %d",
		acquired, skipped, failed, len(info.Dependencies)))
	return nil
}

// autoResolveRepo tenta resolver o GitHub repository (org/repo) para um pacote.
// Usa o ID normalizado primeiro, depois o nome bruto da dependência como fallback.
func autoResolveRepo(id, rawDep, ecosystem string) string {
	// Try normalized ID first
	if r := lookupRepo(id); r != "" {
		return r
	}
	// Fallback: raw dependency name
	if rawDep != id {
		if r := lookupRepo(rawDep); r != "" {
			return r
		}
	}
	// Go packages: infer from module path
	if ecosystem == "go" {
		if strings.HasPrefix(id, "github.com/") {
			parts := strings.SplitN(strings.TrimPrefix(id, "github.com/"), "/", 3)
			if len(parts) >= 2 {
				return parts[0] + "/" + parts[1]
			}
		}
	}
	return ""
}

func lookupRepo(id string) string {
	// Known npm → GitHub mappings
	known := map[string]string{
		"@prisma/client": "prisma/prisma",
		"@nestjs/common": "nestjs/nest", "@nestjs/core": "nestjs/nest",
		"@nestjs/config": "nestjs/nest", "@nestjs/jwt": "nestjs/jwt",
		"@nestjs/passport": "nestjs/passport", "@nestjs/swagger": "nestjs/swagger",
		"@nestjs/throttler": "nestjs/throttler", "@nestjs/schedule": "nestjs/nest",
		"@nestjs/bull": "nestjs/bull", "@nestjs/cache-manager": "nestjs/cache-manager",
		"@nestjs/platform-express": "nestjs/nest",
		"@nestjs/schematics":       "nestjs/schematics",
		"@nestjs/cli":              "nestjs/nest-cli",
		"@nestjs/testing":          "nestjs/nest",
		"@eslint/eslintrc":         "eslint/eslint", "@eslint/js": "eslint/eslint",
		"@types/express":         "DefinitelyTyped/DefinitelyTyped",
		"@types/bcryptjs":        "DefinitelyTyped/DefinitelyTyped",
		"@types/jest":            "DefinitelyTyped/DefinitelyTyped",
		"@types/multer":          "DefinitelyTyped/DefinitelyTyped",
		"@types/node":            "DefinitelyTyped/DefinitelyTyped",
		"@types/passport-jwt":    "DefinitelyTyped/DefinitelyTyped",
		"@types/passport-local":  "DefinitelyTyped/DefinitelyTyped",
		"@types/supertest":       "DefinitelyTyped/DefinitelyTyped",
		"@types/uuid":            "DefinitelyTyped/DefinitelyTyped",
		"bcryptjs":               "dcodeIO/bcrypt.js",
		"bull":                   "OptimalBits/bull",
		"cache-manager":          "jaredwray/cache-manager",
		"class-transformer":      "typestack/class-transformer",
		"class-validator":        "typestack/class-validator",
		"compression":            "expressjs/compression",
		"eslint":                 "eslint/eslint",
		"eslint-config-prettier": "prettier/eslint-config-prettier",
		"eslint-plugin-prettier": "prettier/eslint-plugin-prettier",
		"express":                "expressjs/express",
		"globals":                "sindresorhus/globals",
		"helmet":                 "helmetjs/helmet",
		"jest":                   "jestjs/jest",
		"multer":                 "expressjs/multer",
		"nest-winston":           "gremo/nest-winston",
		"passport":               "jaredhanson/passport",
		"passport-jwt":           "mikenicholson/passport-jwt",
		"passport-local":         "jaredhanson/passport-local",
		"prettier":               "prettier/prettier",
		"prisma":                 "prisma/prisma",
		"reflect-metadata":       "rbuckton/reflect-metadata",
		"rxjs":                   "ReactiveX/rxjs",
		"source-map-support":     "evanw/node-source-map-support",
		"supertest":              "ladjs/supertest",
		"swagger-ui-express":     "scottie1984/swagger-ui-express",
		"ts-jest":                "kulshekhar/ts-jest",
		"ts-loader":              "TypeStrong/ts-loader",
		"ts-node":                "TypeStrong/ts-node",
		"tsconfig-paths":         "dividab/tsconfig-paths",
		"typescript":             "microsoft/TypeScript",
		"typescript-eslint":      "typescript-eslint/typescript-eslint",
		"uuid":                   "uuidjs/uuid",
		"winston":                "winstonjs/winston",
	}

	if r, ok := known[id]; ok {
		return r
	}

	return ""
}

// ── Knowledge Readiness Gate ──────────────────────────────────────────────────

// NewKnowledgeReadinessCommand creates `cosca knowledge readiness`.
func NewKnowledgeReadinessCommand() *cobra.Command {
	var stack string
	var detect bool

	cmd := &cobra.Command{
		Use:   "readiness",
		Short: "Verifica se o Cosca conhece as ferramentas do projeto antes de iniciar",
		Long: `Knowledge Readiness Gate — antes dos agentes começarem a trabalhar,
verifica se cada ferramenta no stack tem conhecimento adequado
(package + documentação + vetores).

Estados por ferramenta:
  ✅ ready     — KnowledgePackage validado + docs + vetores
  ⚠️ partial   — Manifesto existe mas incompleto (sem docs, vetores, ou stale)
  ❌ unknown   — Nenhum KnowledgePackage registrado

Com --detect, detecta automaticamente as dependências do projeto
(package.json, go.mod, Cargo.toml) e verifica todas de uma vez.
Use isso no meio do projeto quando novas ferramentas aparecerem.`,
		Example: `  cosca knowledge readiness --stack "prisma,fastapi,react"
  cosca knowledge readiness --detect
  cosca knowledge readiness --detect --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getwd: %w", err)
			}

			var tools []string

			if detect {
				// Auto-detect dependencies from the project.
				info, detectErr := discovery.DetectProject(cmd.Context(), dir, zerolog.Nop())
				if detectErr != nil {
					return fmt.Errorf("detect project: %w", detectErr)
				}
				tools = info.Dependencies
				if len(tools) == 0 {
					formatter.Warning("Nenhuma dependência detectada no projeto.")
					return nil
				}
				formatter.Println(fmt.Sprintf("Detectadas %d dependências do projeto.", len(tools)))
			} else {
				if stack == "" {
					return fmt.Errorf("--stack é obrigatório (ou use --detect para auto-detectar)")
				}
				tools = splitAndTrim(stack)
			}

			if len(tools) == 0 {
				return fmt.Errorf("nenhuma ferramenta para verificar")
			}

			// Resolve stores.
			localStore := mustPackageStore(dir)
			globalStore := mustGlobalStore()

			report, err := knowledge.CheckReadiness(dir, tools, localStore, globalStore)
			if err != nil {
				return fmt.Errorf("readiness check: %w", err)
			}

			if useJSON {
				return printJSON(cmd, report)
			}

			// Pretty-print the report.
			formatter.Header("Knowledge Readiness Report")
			if detect {
				formatter.KeyValue("Projeto", dir)
			}
			formatter.KeyValue("Stack", fmt.Sprintf("%d ferramentas", len(report.Stack)))
			formatter.Println("")

			for _, item := range report.Items {
				icon := "❌"
				switch item.Status {
				case knowledge.ReadinessReady:
					icon = "✅"
				case knowledge.ReadinessPartial:
					icon = "⚠️"
				}
				eco := item.Ecosystem
				if eco == "" || eco == "unknown" {
					eco = "?"
				}

				formatter.Println(fmt.Sprintf("  %s %-25s [%s]  %s", icon, item.Name, eco, item.Detail))
				if item.Action != "" {
					formatter.Println(fmt.Sprintf("     → %s", item.Action))
				}
			}

			formatter.Println("")
			formatter.Header("Resumo")
			formatter.Println(fmt.Sprintf("  ✅ %d ready  |  ⚠️ %d partial  |  ❌ %d unknown",
				report.Ready, report.Partial, report.Unknown))

			if report.Sufficient {
				formatter.Success("Todas as ferramentas estão prontas — pode iniciar.")
			} else {
				formatter.Warning(fmt.Sprintf("Resolva os %d gaps antes de iniciar:", len(report.Gaps)))
				for _, g := range report.Gaps {
					formatter.Bullet(g)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&stack, "stack", "", "Stack de ferramentas (separadas por vírgula)")
	cmd.Flags().BoolVar(&detect, "detect", false, "Auto-detectar dependências do projeto (package.json/go.mod/etc.)")

	return cmd
}

// mustPackageStore retorna a store local ou nil.
func mustPackageStore(dir string) *knowledge.PackageStore {
	return knowledge.NewPackageStore(filepath.Join(dir, ".cosca", "knowledge", "packages"))
}

// daemonKnowledgeSearch tenta a busca via daemon 24/7 — o engine já está vivo
// lá (Init frio custa ~6s por invocação CLI). Autentica com um JWT curto
// assinado com o COSCA_JWT_SECRET do serve.env (o CLI é o Don — arquivo 0600).
// Retorna (resultados, ok); ok=false se o daemon não respondeu ou não há auth
// (o chamador cai no Engine local).
func daemonKnowledgeSearch(query string, limit int) ([]daemonSearchHit, bool) {
	secret := daemonJWTSecret()
	if secret == "" {
		return nil, false
	}
	now := time.Now().Unix()
	token, err := auth.GenerateToken(auth.Claims{
		Sub:      "cli",
		Username: "cli",
		Role:     "admin",
		Type:     "access",
		Iat:      now,
		Exp:      now + 60, // curto — só para a chamada
	}, []byte(secret))
	if err != nil {
		return nil, false
	}

	body, _ := json.Marshal(map[string]interface{}{"query": query, "limit": limit})
	client := &http.Client{Timeout: 2 * time.Second}
	req, rErr := http.NewRequest(http.MethodPost, "http://127.0.0.1:14120/v1/knowledge/search", strings.NewReader(string(body)))
	if rErr != nil {
		return nil, false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	var out daemonSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, false
	}
	return out.Results, true
}

// daemonJWTSecret lê o COSCA_JWT_SECRET do serve.env do usuário (0600 — só o
// Don). Vazio se o arquivo não existir ou a chave não estiver presente.
func daemonJWTSecret() string {
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".config", "cosca", "serve.env"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "COSCA_JWT_SECRET=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "COSCA_JWT_SECRET="))
		}
	}
	return ""
}

// daemonSearchResponse espelha o formato de SearchResponse do handler REST
// (sem importar api/rest/handler — evita acoplamento de pacote).
type daemonSearchResponse struct {
	Results []daemonSearchHit `json:"results"`
}

type daemonSearchHit struct {
	Title   string  `json:"title"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
	Type    string  `json:"type"`
}

// printKnowledgeResults formata resultados de busca (comum aos caminhos
// daemon e Engine local): federação global + JSON ou formatter.
func printKnowledgeResults(cmd *cobra.Command, formatter *OutputFormatter, useJSON bool, results []KnowledgeSearchResult, query string, globalResults []KnowledgeSearchResult) error {
	if useJSON {
		return printJSON(cmd, results)
	}

	formatter.Header("Knowledge Search Results")
	scope := "local"
	if len(globalResults) > 0 {
		scope = "local + global"
	}
	formatter.KeyValue("Scope", scope)
	formatter.KeyValue("Query", query)
	formatter.KeyValue("Results", fmt.Sprintf("%d", len(results)))
	formatter.Println("")

	for i, r := range results {
		formatter.Printf("%d. %s\n", i+1, r.Title)
		formatter.KeyValue("   Type", r.Type)
		formatter.KeyValue("   Score", fmt.Sprintf("%.2f", r.Score))
		if r.Snippet != "" {
			formatter.KeyValue("   Snippet", r.Snippet)
		}
		formatter.Println("")
	}
	return nil
}
