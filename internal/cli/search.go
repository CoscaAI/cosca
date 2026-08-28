package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/codegraph"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/oracle"
	"github.com/CoscaAI/cosca/internal/search"
)

var searchLimit int
var searchOffset int
var searchTTL string

// NewSearchCommand creates the `cosca search` command.
func NewSearchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search [type] <query>",
		Short: "Search the Cosca knowledge base",
		Long: `Search across all Cosca indexed content.

You can search across all content or narrow by type:

Types: workflow, prompt, skill, agent, template, architecture, provider, component, hook, route, api

Subcommands:
  layered <query>   Busca em camadas (FTS5 → BM25 → vector → LLM) com parada antecipada
  cache              Fingerprint cache de busca (stats, clear, config --ttl)

Flags:
  --ttl <d>          Habilita o cache de conhecimento por fingerprint com o
                     TTL dado (ex.: 10m, 1h) — uso único, opt-in.
`,
		Example: `  cosca search "database schema"           Search all content
  cosca search workflow "code review"      Search workflows only
  cosca search --json "error handling"     JSON output
  cosca search layered "backup"            Layered search (cost ladder, early stop)
  cosca search --ttl 10m "backup"          Search with fingerprint cache (10m TTL)
  cosca search cache stats                 Search cache stats (hits, misses, TTL)
  cosca search cache clear                 Clear the search cache`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()

			searchType := ""
			query := args[0]
			if len(args) > 1 {
				searchType = args[0]
				query = args[1]
			}

			var results []KnowledgeSearchResult

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			// One-shot fingerprint cache: --ttl opta pelo SearchWithPolicy. Sem
			// --ttl, o caminho legado (adapter) é mantido — zero mudança.
			if searchTTL != "" {
				ttl, err := time.ParseDuration(searchTTL)
				if err != nil {
					return fmt.Errorf("invalid --ttl %q: %w", searchTTL, err)
				}
				policy := knowledge.DefaultCachePolicy()
				policy.TTL = ttl

				ke, err := newCLIKnowledgeEngine(dir)
				if err != nil {
					return fmt.Errorf("knowledge engine not available: %w", err)
				}
				defer func() { _ = ke.Close() }()

				params := search.DefaultSearchParams()
				params.Query = query
				params.Limit = searchLimit
				params.Offset = searchOffset

				res, err := ke.SearchWithPolicy(ctx, params, &policy)
				if err != nil {
					return fmt.Errorf("search failed: %w", err)
				}
				results = adapterResults(res)
			} else {
				ke := NewKnowledgeEngine(dir)
				if ke == nil {
					return fmt.Errorf("knowledge engine not available")
				}

				var err error
				results, err = ke.Search(query, searchLimit, searchOffset)
				if err != nil {
					return fmt.Errorf("search failed: %w", err)
				}
			}

			if useJSON {
				return printJSON(cmd, results)
			}

			formatter.Header(fmt.Sprintf("Search Results for %q", query))
			if searchType != "" {
				formatter.KeyValue("Type", searchType)
			}
			formatter.KeyValue("Results", fmt.Sprintf("%d", len(results)))
			formatter.Println("")

			// ── SEARCH_PROTOCOL §6-§7: classificar por relevância
			// semântica e NÃO apresentar NOISE. TEXT MATCH ≠ SEMANTIC
			// RELEVANCE (§5) — o motor ranqueia por score; o classificador
			// decide o que é DIRECT/RELATED/CONTEXTUAL vs lixo.
			rawHits := make([]oracle.RawHit, 0, len(results))
			for i, r := range results {
				rawHits = append(rawHits, oracle.RawHit{
					Rank:    i + 1,
					Title:   r.Title,
					Type:    r.Type,
					Score:   r.Score,
					Snippet: r.Snippet,
				})
			}
			classified := oracle.ClassifyHits(oracle.IntentExplore, query, rawHits)

			shown := 0
			noise := 0
			for _, r := range classified {
				if !r.Presentable {
					noise++
					continue // §7: NOISE não se apresenta
				}
				shown++
				formatter.Printf("%d. %s\n", r.Rank, r.Title)
				formatter.KeyValue("   Type", r.Type)
				formatter.KeyValue("   Score", fmt.Sprintf("%.2f", r.Score))
				formatter.KeyValue("   Relevance", string(r.Relevance))
				if r.Snippet != "" {
					formatter.KeyValue("   Snippet", r.Snippet)
				}
				formatter.Println("")
			}
			if noise > 0 {
				formatter.Warning(fmt.Sprintf("%d resultado(s) descartado(s) por baixa relevância semântica (NOISE)", noise))
			}
			if shown == 0 {
				formatter.Warning("Nenhum resultado com relevância semântica suficiente (SEARCH_PROTOCOL §15: ausência também é resultado).")
			}

			return nil
		},
	}

	cmd.Flags().IntVarP(&searchLimit, "limit", "l", 10, "Maximum number of results")
	cmd.Flags().IntVarP(&searchOffset, "offset", "o", 0, "Result offset")
	cmd.Flags().StringVar(&searchTTL, "ttl", "", "fingerprint cache TTL (e.g. 10m, 1h) — enables the fingerprint knowledge cache for this search")

	cmd.AddCommand(NewSearchLayeredCommand())
	cmd.AddCommand(NewSearchCodeCommand())
	cmd.AddCommand(NewSearchCacheCommand())

	return cmd
}

// NewSearchCodeCommand cria `cosca search code <query>` — busca semântica
// determinística de código (ADR-019), zero LLM/zero rede (I1), dentro do
// umbrella de busca do conhecimento.
func NewSearchCodeCommand() *cobra.Command {
	var dir string
	var limit int
	var dim int
	var saveIndex string
	var useIndex string
	cmd := &cobra.Command{
		Use:   "code <query>",
		Short: "Busca semântica determinística de código (para encontrar arquivos)",
		Long: `Busca os arquivos de código mais similares à consulta, via embedding
determinístico + fusão de sinais (unigram + bigram + MinHash). Zero LLM, zero
rede (I1) — determinístico. Usa um índice RAM-first (F3) e publica atomicamente
(opcional: --save-index). Pode reusar um índice já publicado (--use-index) para
buscar sub-ms sem reescrever.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)
			// F3: reusa um índice publicado (sub-ms, sem reescrever); senão constrói.
			ix, err := codegraph.LoadIndex(useIndex)
			if err != nil {
				return fmt.Errorf("load index %q: %w", useIndex, err)
			}
			if ix == nil {
				ix, err = codegraph.BuildIndex(dir, dim)
				if err != nil {
					return fmt.Errorf("build code index: %w", err)
				}
			}
			hits := ix.SearchSimilar(args[0], limit)
			// F3: publica atômico (fail-closed I2) — nunca deixa índice parcial.
			if saveIndex != "" {
				if err := ix.Save(saveIndex); err != nil {
					formatter.Warning("falha ao salvar índice: " + err.Error())
				}
			}
			if useJSON {
				return printJSON(cmd, map[string]interface{}{"type": "code", "query": args[0], "results": hits})
			}
			formatter.Header("Busca de código (semântica, determinística)")
			formatter.KeyValue("Consulta", args[0])
			formatter.KeyValue("Diretório", dir)
			if useIndex != "" && ix != nil {
				formatter.KeyValue("Índice", "reusado (publicado)")
			}
			formatter.KeyValue("Arquivos indexados", fmt.Sprintf("%d", len(ix.Signals)))
			formatter.KeyValue("Resultados", fmt.Sprintf("%d", len(hits)))
			rows := make([][]string, 0, len(hits))
			for _, h := range hits {
				rows = append(rows, []string{h.Path, fmt.Sprintf("%.3f", h.Score)})
			}
			if len(rows) > 0 {
				formatter.Table([]string{"Arquivo", "Score"}, rows)
			} else {
				formatter.Warning("Nenhum arquivo de código correspondente (fail-open).")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", ".", "diretório raiz do codebase a indexar")
	cmd.Flags().IntVar(&limit, "limit", 8, "número máximo de resultados")
	cmd.Flags().IntVar(&dim, "dim", 512, "dimensão do embedding")
	cmd.Flags().StringVar(&saveIndex, "save-index", "", "publicar o índice atômico neste caminho (fail-closed I2)")
	cmd.Flags().StringVar(&useIndex, "use-index", "", "reusar um índice já publicado (sub-ms, sem reescrever)")
	return cmd
}

// newCLIKnowledgeEngine builds and initializes a knowledge engine rooted at
// dir (DB at <dir>/knowledge.db) with a functional multi-level cache (Memory +
// SQLite). Used by the fingerprint-cache commands and the --ttl search path.
func newCLIKnowledgeEngine(dir string) (*knowledge.Engine, error) {
	// Honor the project config's embedding settings (mirrors serve.go, runtime.go
	// and knowledge search): without a provider the vector phase fails with
	// "embed query: no embedding provider selected", collapsing recall to 0.
	// A config load failure is non-fatal and leaves the provider empty so
	// auto-detection applies.
	var (
		embeddingProvider   string
		embeddingBaseURL    string
		embeddingModel      string
		embeddingDigest     string
		embeddingAPIKey     string
		embeddingDimensions int
	)
	if c, err := config.Load(); err == nil {
		embeddingProvider = c.Embedding.Provider
		embeddingBaseURL = c.Embedding.BaseURL
		embeddingModel = c.Embedding.Model
		embeddingDigest = c.Embedding.Digest
		embeddingAPIKey = c.Embedding.APIKey
		embeddingDimensions = c.Embedding.Dimensions
	}

	ke, err := knowledge.New(knowledge.Config{
		DBPath:              knowledgeDBPath(dir),
		RootDir:             filepath.Dir(dir),
		AutoMigrate:         true,
		CacheConfig:         knowledge.DefaultConfig().CacheConfig,
		EmbeddingProvider:   embeddingProvider,
		EmbeddingBaseURL:    embeddingBaseURL,
		EmbeddingModel:      embeddingModel,
		EmbeddingDigest:     embeddingDigest,
		EmbeddingAPIKey:     embeddingAPIKey,
		EmbeddingDimensions: embeddingDimensions,
	})
	if err != nil {
		return nil, err
	}
	if err := ke.Init(); err != nil {
		return nil, err
	}
	return ke, nil
}

// adapterResults converts search.SearchResults to the CLI's flat result type.
func adapterResults(res *search.SearchResults) []KnowledgeSearchResult {
	var out []KnowledgeSearchResult
	if res != nil {
		for _, r := range res.Results {
			out = append(out, KnowledgeSearchResult{
				Title:   r.Title,
				Type:    string(r.Type),
				Score:   r.Score,
				Snippet: r.Snippet,
			})
		}
	}
	return out
}

// NewSearchLayeredCommand creates `cosca search layered <query>`.
//
// Busca em camadas com custo progressivo e parada antecipada:
//
//	L1 FTS5    → para se hits >= MinFTSCount e o topo é on-topic (0 tokens IA)
//	L2 BM25    → para se o melhor candidato re-rankeado confirma
//	L3 vector  → refinamento por embeddings (pula com graça se sem provedor)
//	L4 LLM     → apenas com --allow-llm (placeholder — nenhum LLM é chamado aqui)
//
// A decisão de camada é determinística e explicável; --json expõe o mesmo
// resultado estruturado (layer, stopped_early, reason, stats).
func NewSearchLayeredCommand() *cobra.Command {
	var allowLLM bool

	cmd := &cobra.Command{
		Use:   "layered <query>",
		Short: "Busca em camadas — FTS5 → BM25 → vector → LLM (custo progressivo, parada antecipada)",
		Long: `Busca em camadas com custo progressivo e parada antecipada.

A consulta sobe de camada somente quando a camada anterior não responde com
confiança determinística — nunca gasta IA sem necessidade:

  L1 FTS5    → para se hits suficientes e on-topic (0 tokens de IA)
  L2 BM25    → para se o melhor candidato re-rankeado confirma
  L3 vector  → refinamento por embeddings (pula com graça se não há provedor)
  L4 LLM     → raciocínio OPTO-IN: exige --allow-llm (default desabilitado)

Exemplo de custo (conversa do Don): "FTS5 → 200 → BM25 → 30 → vector → 8 →
LLM → sintetiza". Ou simplesmente: "FTS5 → resposta encontrada", sem uma única
chamada de IA. L4 é um placeholder — esta feature marca a decisão e nunca
invoca um LLM por si só; o provedor é plugado depois.`,
		Example: `  cosca search layered "backup"
  cosca search layered "backup" --allow-llm
  cosca search layered --json "backup"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			dir, _ := os.Getwd()

			// Load project config for embedding provider settings
			var (
				embeddingProvider string
				embeddingBaseURL  string
				embeddingModel    string
				embeddingDigest   string
				embeddingAPIKey   string
				embeddingDim      int
			)
			if c, cfgErr := config.Load(); cfgErr == nil {
				embeddingProvider = c.Embedding.Provider
				embeddingBaseURL = c.Embedding.BaseURL
				embeddingModel = c.Embedding.Model
				embeddingDigest = c.Embedding.Digest
				embeddingAPIKey = c.Embedding.APIKey
				embeddingDim = c.Embedding.Dimensions
			}

			ke, err := knowledge.New(knowledge.Config{
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
			if err != nil {
				return fmt.Errorf("knowledge engine not available: %w", err)
			}
			if err := ke.Init(); err != nil {
				return fmt.Errorf("init knowledge engine: %w", err)
			}
			defer func() {
				if err := ke.Close(); err != nil {
					formatter.Verbose(fmt.Sprintf("Warning closing knowledge engine: %v", err))
				}
			}()

			cfg := search.DefaultLayeredConfig()
			cfg.AllowLLM = allowLLM
			ls := search.NewLayeredSearch(ke, cfg).SetRanker(ke.Ranker()).SetGraph(ke.Graph())

			res, err := ls.Search(ctx, args[0])
			if err != nil {
				return fmt.Errorf("layered search failed: %w", err)
			}

			if useJSON {
				return printJSON(cmd, res)
			}

			formatter.Header(fmt.Sprintf("Busca em camadas: %q", args[0]))
			formatter.KeyValue("Camada", res.Layer.String())
			formatter.KeyValue("Parou sem IA", earlyStopText(res))
			formatter.KeyValue("Motivo", res.Reason)
			formatter.KeyValue("FTS5 hits", fmt.Sprintf("%d", res.Stats.FTS5Hits))
			formatter.KeyValue("BM25 hits", fmt.Sprintf("%d", res.Stats.BM25Hits))
			formatter.KeyValue("Vector hits", fmt.Sprintf("%d", res.Stats.VectorHits))
			formatter.KeyValue("LLM chamado", fmt.Sprintf("%v", res.Stats.LLMCalled))
			formatter.Println("")

			if len(res.Results) == 0 {
				formatter.Warning("Nenhum resultado encontrado nas camadas percorridas.")
				return nil
			}

			formatter.Header("Melhores candidatos")
			for i, r := range res.Results {
				formatter.Printf("%d. %s\n", i+1, r.Title)
				formatter.KeyValue("   Tipo", string(r.Type))
				formatter.KeyValue("   Score", fmt.Sprintf("%.4f", r.Score))
				if r.Snippet != "" {
					formatter.KeyValue("   Trecho", r.Snippet)
				}
				formatter.Println("")
			}

			if res.StoppedEarly {
				formatter.Success("Parou sem gastar IA — custo mínimo.")
			} else {
				formatter.Warning("Precisou de raciocínio (L4) — custo de IA envolvido.")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&allowLLM, "allow-llm", false, "habilita L4 (raciocínio/IA) — opt-in, default desabilitado")
	return cmd
}

// earlyStopText humaniza o status de parada antecipada para o output de texto.
func earlyStopText(res *search.LayeredResult) string {
	if res.StoppedEarly {
		return "✓ parou sem IA"
	}
	return "✗ precisou de raciocínio"
}

// ── `cosca search cache` — fingerprint cache de busca ───────────────────────

// NewSearchCacheCommand cria a árvore `cosca search cache`.
func NewSearchCacheCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Fingerprint cache de busca — stats, clear, config --ttl",
		Long: `Fingerprint cache de busca do conhecimento externo.

O cache de busca chaveia resultados pela fingerprint canônica da query
(QueryFingerprint) com TTL configurável: N queries equivalentes compartilham
UMA busca externa em vez de N. Subcomandos:

  stats               Hits, misses, entries e o TTL configurado
  clear               Limpa o cache de busca
  config --ttl <d>    Persiste o TTL do cache (ex.: 10m, 1h) em .cosca/config.yaml

O TTL padrão é 5m (DefaultCachePolicy). O ` + "`--ttl`" + ` de uso único do
"cosca search" opta pelo cache por fingerprint sem persistir nada.`,
		Example: `  cosca search cache stats
  cosca search cache clear
  cosca search cache config --ttl 10m`,
	}

	cmd.AddCommand(
		NewSearchCacheStatsCommand(),
		NewSearchCacheClearCommand(),
		NewSearchCacheConfigCommand(),
	)
	return cmd
}

// NewSearchCacheStatsCommand cria `cosca search cache stats`.
func NewSearchCacheStatsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show search cache statistics (hits, misses, entries, TTL)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()

			ke, err := newCLIKnowledgeEngine(dir)
			if err != nil {
				return fmt.Errorf("knowledge engine not available: %w", err)
			}
			defer func() { _ = ke.Close() }()

			stats := ke.CacheStats()
			if cfg, cfgErr := loadConfig(); cfgErr == nil && cfg.Cache.TTL > 0 {
				stats.TTL = cfg.Cache.TTL
			}

			if useJSON {
				return printJSON(cmd, stats)
			}

			formatter.Header("Search Cache Statistics")
			formatter.KeyValue("Hits", fmt.Sprintf("%d", stats.Hits))
			formatter.KeyValue("Misses", fmt.Sprintf("%d", stats.Misses))
			formatter.KeyValue("Entries", fmt.Sprintf("%d", stats.Entries))
			formatter.KeyValue("TTL", stats.TTL.String())
			formatter.KeyValue("Enabled", fmt.Sprintf("%t", stats.Enabled))
			return nil
		},
	}
	return cmd
}

// NewSearchCacheClearCommand cria `cosca search cache clear`.
func NewSearchCacheClearCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear the search fingerprint cache",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()

			ke, err := newCLIKnowledgeEngine(dir)
			if err != nil {
				return fmt.Errorf("knowledge engine not available: %w", err)
			}
			defer func() { _ = ke.Close() }()

			if err := ke.ClearCache(); err != nil {
				return fmt.Errorf("failed to clear search cache: %w", err)
			}

			if useJSON {
				return printJSON(cmd, map[string]interface{}{"cleared": true})
			}
			formatter.Success("Search cache cleared")
			return nil
		},
	}
	return cmd
}

// NewSearchCacheConfigCommand cria `cosca search cache config`.
//
// O TTL é persistido no mesmo lugar onde vive o cache config:
// .cosca/config.yaml (chave cache.ttl, campo Cache.TTL em time.Duration).
func NewSearchCacheConfigCommand() *cobra.Command {
	var ttl string

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show or set the search cache TTL (persisted in .cosca/config.yaml)",
		Long: `Show or set the search cache TTL.

  cosca search cache config            Mostra o TTL atual
  cosca search cache config --ttl 10m  Persiste um novo TTL (10m, 1h, 1h30m)

O valor é gravado em .cosca/config.yaml (cache.ttl). O TTL padrão é 5m.`,
		Example: `  cosca search cache config
  cosca search cache config --ttl 10m`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			cfgPath := configPath()
			cfg, err := config.LoadFromFile(cfgPath)
			if err != nil {
				cfg = config.DefaultConfig()
				cfg.Paths.Project = getConfigDir()
				// ProjectDir não existe dentro da jaula — usa config global.
				if _, statErr := os.Stat(getConfigDir()); os.IsNotExist(statErr) || os.IsPermission(statErr) {
					cfgPath = filepath.Join(config.UserHomeDir(), ".config", "cosca", "config.yaml")
				}
			}

			if ttl == "" {
				if useJSON {
					return printJSON(cmd, map[string]interface{}{
						"cache.ttl": cfg.Cache.TTL,
						"ttl":       cfg.Cache.TTL.String(),
					})
				}
				formatter.KeyValue("cache.ttl", cfg.Cache.TTL.String())
				return nil
			}

			d, err := time.ParseDuration(ttl)
			if err != nil {
				return fmt.Errorf("invalid --ttl %q: %w", ttl, err)
			}
			if d <= 0 {
				return fmt.Errorf("--ttl must be positive (got %q)", ttl)
			}

			cfg.Cache.TTL = d
			if v := config.GetViper(); v != nil {
				v.Set("cache.ttl", int64(d.Seconds()))
			}
			if err := cfg.Save(cfgPath); err != nil {
				return fmt.Errorf("failed to write config: %w", err)
			}

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"cache.ttl": d,
					"ttl":       d.String(),
				})
			}
			formatter.Success(fmt.Sprintf("cache.ttl set to %s", d.String()))
			return nil
		},
	}

	cmd.Flags().StringVar(&ttl, "ttl", "", "new search cache TTL (e.g. 10m, 1h)")
	return cmd
}
