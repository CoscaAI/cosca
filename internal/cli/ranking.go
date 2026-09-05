//
// `cosca ranking` — Ranking multi-sinal explicável (score breakdown por sinal).
//
// Regra do Don: "Não use apenas estrelas para ranking. Eu evitaria 'stars =
// qualidade'. Eu faria vários sinais ... e o ranking seria explicável:
// Repository #1 Score: 87, Relevance 94, Freshness 90, Authority 88, Tests 91,
// Documentation 76, Security 83. Assim o Cosca pode explicar a escolha."
//
// O Ranker combina 5 sinais (BM25, vetor, grafo, frescor, popularidade) com
// pesos fixos. `cosca ranking explain "<query>"` roda a busca + ranking e
// expõe o breakdown do resultado escolhido: score total + contribuição de cada
// sinal + uma justificação em pt-BR. O breakdown usa exatamente os mesmos
// pesos e normalização do Rank (invariante de consistência: o Total do
// breakdown é o score que o Rank usa para ordenar).
//
// Subcomandos:
//   explain <query> [--index N]   Score total + breakdown por sinal + justificação
//

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/ranking"
	"github.com/CoscaAI/cosca/internal/search"
)

// NewRankingCommand cria a árvore de comandos `cosca ranking`.
func NewRankingCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ranking",
		Short: "Ranking multi-sinal explicável — score breakdown por sinal",
		Long: `Ranking multi-sinal explicável.

O ranking do Cosca combina cinco sinais com pesos fixos (BM25, vetor, grafo,
frescor, popularidade) em vez de uma "nota de estrelas" opaca. Cada resultado
pode ser auditado: quanto cada sinal contribuiu para o score final e por que a
escolha venceu.

  cosca ranking explain "<query>" [--index N]   Breakdown do resultado escolhido`,
		Example: `  cosca ranking explain "backup"
  cosca ranking explain "backup" --index 1
  cosca ranking explain --json "backup"`,
	}

	cmd.AddCommand(
		NewRankingExplainCommand(),
	)
	return cmd
}

// NewRankingExplainCommand cria `cosca ranking explain <query>`.
func NewRankingExplainCommand() *cobra.Command {
	var index int

	cmd := &cobra.Command{
		Use:   "explain <query>",
		Short: "Explica o ranking do resultado: score total + breakdown por sinal + justificação",
		Long: `Roda a busca + ranking e imprime o score total do resultado com o
breakdown por sinal (BM25, Vetor, Grafo, Frescor, Popularidade) e a
justificação em pt-BR citando o sinal dominante.

  --index N   escolhe o resultado (0 = topo)
  --json      emite o breakdown estruturado

Invariante: o breakdown usa os mesmos pesos e a mesma normalização do Rank —
o "Score total" é exatamente o score que o ranking usa para ordenar.`,
		Example: `  cosca ranking explain "backup"
  cosca ranking explain "backup" --index 1
  cosca ranking explain --json "backup"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			dir, _ := os.Getwd()

			ke, err := knowledge.New(knowledge.Config{
				DBPath:  filepath.Join(dir, "knowledge.db"),
				RootDir: filepath.Dir(dir),
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

			params := search.DefaultSearchParams()
			params.Query = args[0]
			params.Limit = 50 // candidatos; o ranking acontece abaixo
			results, err := ke.Search(ctx, params)
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}
			if results == nil || len(results.Results) == 0 {
				if useJSON {
					return printJSON(cmd, map[string]interface{}{
						"query":         args[0],
						"ranked":        nil,
						"justification": "Nenhum resultado encontrado para a consulta.",
					})
				}
				formatter.Warning("Nenhum resultado encontrado para a consulta.")
				return nil
			}

			rankables := make([]ranking.Rankable, 0, len(results.Results))
			for _, r := range results.Results {
				rankables = append(rankables, &cliRankable{
					id:      r.ID,
					content: r.Content + " " + r.Title,
					score:   r.Score,
				})
			}

			ranker := ranking.New(ranking.DefaultConfig())
			breakdowns := ranker.ExplainRanked(rankables, args[0])

			if index < 0 || index >= len(breakdowns) {
				return fmt.Errorf("--index %d fora do intervalo (0..%d)", index, len(breakdowns)-1)
			}

			bd := breakdowns[index]
			just := bd.Justification()

			var src *search.SearchResult
			for i := range results.Results {
				if results.Results[i].ID == bd.ItemID {
					src = &results.Results[i]
					break
				}
			}
			if src == nil {
				src = &results.Results[index]
			}

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"query": args[0],
					"index": index,
					"rank":  index + 1,
					"item": map[string]interface{}{
						"id":    src.ID,
						"title": src.Title,
						"type":  string(src.Type),
						"score": src.Score,
					},
					"breakdown":     bd,
					"justification": just,
				})
			}

			formatter.Header(fmt.Sprintf("RANKING EXPLICÁVEL — %q", args[0]))
			formatter.KeyValue("Resultado", fmt.Sprintf("#%d %s", index+1, src.Title))
			formatter.KeyValue("Tipo", string(src.Type))
			formatter.KeyValue("Score total", fmt.Sprintf("%.2f/100", bd.Total*100))
			formatter.KeyValue("BM25", fmt.Sprintf("%.2f/100", bd.BM25*100))
			formatter.KeyValue("Vetor", fmt.Sprintf("%.2f/100", bd.Vector*100))
			formatter.KeyValue("Grafo", fmt.Sprintf("%.2f/100", bd.Graph*100))
			formatter.KeyValue("Frescor", fmt.Sprintf("%.2f/100", bd.Freshness*100))
			formatter.KeyValue("Popularidade", fmt.Sprintf("%.2f/100", bd.Popularity*100))
			formatter.KeyValue("Justificação", just)
			return nil
		},
	}

	cmd.Flags().IntVar(&index, "index", 0, "índice do resultado a explicar (0 = topo)")
	return cmd
}

// cliRankable adapta um search.SearchResult ao ranking.Rankable para que o
// breakdown reproduza a mesma pontuação do engine de busca (mesmos 0s para
// timestamp/referências/distância, como o adaptador interno do search).
type cliRankable struct {
	id      string
	content string
	score   float64
}

func (c *cliRankable) ID() string          { return c.id }
func (c *cliRankable) Content() string     { return c.content }
func (c *cliRankable) Score() float64      { return c.score }
func (c *cliRankable) Timestamp() int64    { return 0 }
func (c *cliRankable) ReferenceCount() int { return 0 }
func (c *cliRankable) GraphDistance() int  { return 0 }
