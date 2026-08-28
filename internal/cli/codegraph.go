package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/codegraph"
)

// NewCodeGraphCommand cria a árvore `cosca codegraph` — grafo de código + busca
// semântica determinística (ADR-019). ZERO LLM, ZERO rede (I1).
func NewCodeGraphCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "codegraph",
		Short: "Code graph + busca semântica determinística (ADR-019)",
		Long: `Grafo de código e busca semântica determinística (embedding int8 + fusão
de sinais, zero LLM — ADR-019).

Subcomandos:
  build    Constrói o grafo de imports/símbolos de um diretório
  search   Busca semanticamente os arquivos mais similares a uma consulta`,
		Example: `  cosca codegraph search "http handler" --dir . --limit 5
  cosca codegraph build --dir .`,
	}
	cmd.AddCommand(NewCodeGraphBuildCommand(), NewCodeGraphSearchCommand(), NewCodeGraphStatsCommand())
	return cmd
}

// NewCodeGraphBuildCommand cria `cosca codegraph build`.
func NewCodeGraphBuildCommand() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Constrói o grafo de imports/símbolos de um diretório",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)
			g, err := codegraph.BuildGraph(dir)
			if err != nil {
				return fmt.Errorf("build graph: %w", err)
			}
			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"dir":   dir,
					"nodes": g.GetNodeCount(),
					"edges": g.GetEdgeCount(),
				})
			}
			formatter.Header("Code Graph")
			formatter.KeyValue("Diretório", dir)
			formatter.KeyValue("Nós (arquivos/módulos)", fmt.Sprintf("%d", g.GetNodeCount()))
			formatter.KeyValue("Arestas (imports)", fmt.Sprintf("%d", g.GetEdgeCount()))
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", ".", "diretório raiz a indexar")
	return cmd
}

// NewCodeGraphSearchCommand cria `cosca codegraph search <query>`.
func NewCodeGraphSearchCommand() *cobra.Command {
	var dir string
	var limit int
	var dim int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Busca semântica determinística dos arquivos mais similares",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)
			g, err := codegraph.BuildGraph(dir)
			if err != nil {
				return fmt.Errorf("build graph: %w", err)
			}
			hits, err := codegraph.SearchSimilar(g, dir, args[0], limit, dim)
			if err != nil {
				return fmt.Errorf("search: %w", err)
			}
			if useJSON {
				return printJSON(cmd, map[string]interface{}{"query": args[0], "results": hits})
			}
			formatter.Header("Busca semântica de código (determinística)")
			formatter.KeyValue("Consulta", args[0])
			formatter.KeyValue("Resultados", fmt.Sprintf("%d", len(hits)))
			rows := make([][]string, 0, len(hits))
			for _, h := range hits {
				rows = append(rows, []string{h.Path, fmt.Sprintf("%.3f", h.Score), h.Lang})
			}
			formatter.Table([]string{"Arquivo", "Score", "Lang"}, rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", ".", "diretório raiz a indexar")
	cmd.Flags().IntVar(&limit, "limit", 5, "número máximo de resultados")
	cmd.Flags().IntVar(&dim, "dim", 512, "dimensão do embedding")
	return cmd
}

// NewCodeGraphStatsCommand cria `cosca codegraph stats` — a COBERTURA do índice,
// separada dos fatos (F4, I3/I4): "não registrado ≠ não existe".
func NewCodeGraphStatsCommand() *cobra.Command {
	var dir string
	var dim int
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Cobertura do índice (arquivos indexados vs pulados), separada dos fatos",
		Long: `Mostra a COBERTURA do índice — quantos arquivos-fonte existiam, quantos
foram indexados, quantos pulados, por linguagem. Isto é métrica SOBRE o grafo
(nunca misturada aos fatos). Best-effort (heurístico v1): "não registrado ≠
inexistente" (I3/I4).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)
			ix, err := codegraph.BuildIndex(dir, dim)
			if err != nil {
				return fmt.Errorf("build index: %w", err)
			}
			c := ix.Coverage
			if useJSON {
				return printJSON(cmd, map[string]interface{}{"dir": dir, "coverage": c})
			}
			formatter.Header("Cobertura do índice (separada dos fatos)")
			formatter.KeyValue("Diretório", dir)
			formatter.KeyValue("Arquivos-fonte (totais)", fmt.Sprintf("%d", c.TotalSourceFiles))
			formatter.KeyValue("Indexados", fmt.Sprintf("%d", c.IndexedFiles))
			formatter.KeyValue("Pulados (omissão registrada)", fmt.Sprintf("%d", c.SkippedFiles))
			formatter.KeyValue("Cobre tudo", fmt.Sprintf("%v", c.CoversAll))
			rows := make([][]string, 0, len(c.Langs))
			for lang, n := range c.Langs {
				rows = append(rows, []string{lang, fmt.Sprintf("%d", n)})
			}
			formatter.Table([]string{"Linguagem", "Arquivos"}, rows)
			formatter.Warning("Best-effort (I4): \"não registrado ≠ inexistente\" — o grafo não afirma completude.")
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", ".", "diretório raiz a indexar")
	cmd.Flags().IntVar(&dim, "dim", 512, "dimensão do embedding")
	return cmd
}
