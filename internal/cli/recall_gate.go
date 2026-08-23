//
// `cosca gate recall` — GATE DE RECALL (gabarito congelado + fail-closed).
//
// Implementa o Bloco 2 / C6 do ADR-011: um gabarito congelado
// (qrels-baseline.json) gera o recall do retrieval e aplica um piso
// determinístico. É o análogo do `cosca gate catalog` (precedente de gate
// fail-on-invariant no CI), mas medindo recall de retrieval em vez de
// invariantes de catálogo. Paridade de forma: `--audit` / `--strict` /
// `--summary` / `--json`.
//
// Operações:
//   --check  (default)  VALIDAÇÃO de contrato do baseline: carrega e valida o
//                       qrels-baseline.json (LoadQrels + ValidateQrels). Não
//                       roda recall. Baseline corrompido/ausente é falha.
//   --audit             RODA o gate de recall (retriever real injetado via
//                       knowledge.Engine.Search): computa recall@K,
//                       first_relevant_hit (MRR), expected_top1_hit, nDCG@k e
//                       queries_errored. FAIL-CLOSED:
//                         queries_errored > 0            → fail
//                         recall@K < floor               → fail
//                         first_relevant_hit < floor     → fail
//
// Flags:
//   --strict    com --audit: qualquer falha => exit 1 (bloqueante). Sem
//               --strict, o achado vira débito reportado (exit 0).
//   --summary   imprime 1 linha curta p/ o hook (ex: "recall: OK").
//   --baseline  caminho do qrels-baseline.json (default: qrels-baseline.json).
//   --k         K do recall@K (default: 5).
//
// Exemplos:
//   cosca gate recall --check
//   cosca gate recall --audit
//   cosca gate recall --audit --strict --summary
//   cosca gate recall --audit --json
//   cosca gate recall --audit --baseline .cosca/qrels-baseline.json --k 10
//

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/grounding"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/search"
)

// recallBaselineFileName é o default do arquivo de gabarito congelado.
const recallBaselineFileName = "qrels-baseline.json"

// resolveRecallBaseline resolve o caminho do baseline: usa a flag, senão
// procura qrels-baseline.json no cwd e depois no testdata (paridade com o
// testdata do ADR §3.4).
func resolveRecallBaseline(flagPath string) (string, error) {
	candidates := []string{flagPath}
	if flagPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("getwd: %w", err)
		}
		candidates = []string{
			filepath.Join(cwd, recallBaselineFileName),
			filepath.Join(cwd, "testdata", recallBaselineFileName),
		}
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("baseline %q não encontrado (use --baseline)", recallBaselineFileName)
}

// NewGateRecallCommand cria a subárvore `cosca gate recall`.
func NewGateRecallCommand() *cobra.Command {
	var (
		audit    bool
		strict   bool
		summary  bool
		baseline string
		k        int
	)

	cmd := &cobra.Command{
		Use:   "recall",
		Short: "GATE DE RECALL — gabarito congelado + piso fail-closed de não-regressão de retrieval",
		Long: `GATE DE RECALL do ADR-011 (Bloco 2 / C6): mede o recall do motor de
busca contra um gabarito congelado (qrels-baseline.json).

  --check  (default)  Valida o contrato do baseline (carrega + valida). Não roda
                      recall. Baseline corrompido/ausente é falha de contrato.
  --audit             Roda o gate de recall (retriever real via search engine):
                      recall@K, first_relevant_hit (MRR), expected_top1_hit,
                      nDCG@k e queries_errored.
                      FAIL-CLOSED: queries_errored>0 ⇒ fail; recall@K<floor ⇒
                      fail; first_relevant_hit<floor ⇒ fail.

Piso default: recall@5 >= 0.60, first_relevant_hit >= 0.80.

Flags:
  --strict    com --audit: falha => exit 1 (bloqueante).
  --summary   1 linha curta p/ o hook (ex: "recall: OK").
  --baseline  caminho do qrels-baseline.json.
  --k         K do recall@K (default 5).`,
		Example: `  cosca gate recall --check
  cosca gate recall --audit
  cosca gate recall --audit --strict --summary
  cosca gate recall --audit --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			path, err := resolveRecallBaseline(baseline)
			if err != nil {
				return err
			}

			if !audit {
				return runRecallCheck(cmd, formatter, useJSON, summary, path)
			}

			qrels, err := grounding.LoadQrels(path)
			if err != nil {
				return err
			}

			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			ke, err := newCLIKnowledgeEngine(dir)
			if err != nil {
				return fmt.Errorf("knowledge engine not available: %w", err)
			}
			defer func() { _ = ke.Close() }()

			retriever := buildKnowledgeRetriever(ke)
			floor := grounding.DefaultFloor()
			if k > 0 {
				floor.K = k
			}
			return runRecallAudit(cmd, formatter, useJSON, strict, summary, retriever, qrels, floor)
		},
	}

	cmd.Flags().BoolVar(&audit, "audit", false, "rodar o gate de recall (em vez de só validar o baseline)")
	cmd.Flags().BoolVar(&strict, "strict", false, "com --audit: tornar falhas bloqueantes (exit 1)")
	cmd.Flags().BoolVar(&summary, "summary", false, "imprimir 1 linha curta de resumo (p/ hook)")
	cmd.Flags().StringVar(&baseline, "baseline", "", "caminho do qrels-baseline.json")
	cmd.Flags().IntVar(&k, "k", 5, "K do recall@K (default 5)")
	return cmd
}

// buildKnowledgeRetriever cria um Retriever a partir de um knowledge.Engine:
// cada query gera o top-k via engine.Search e mapeia os resultados para
// grounding.Result (ChunkID = result.ID). O engine é um ponteiro vivo — o
// chamador é responsável pelo Close.
func buildKnowledgeRetriever(ke *knowledge.Engine) grounding.Retriever {
	return func(ctx context.Context, query string, k int) ([]grounding.Result, error) {
		params := search.DefaultSearchParams()
		params.Query = query
		params.Limit = k
		res, err := ke.Search(ctx, params)
		if err != nil {
			return nil, err
		}
		out := make([]grounding.Result, 0, len(res.Results))
		for _, r := range res.Results {
			if r.ID == "" {
				continue
			}
			out = append(out, grounding.Result{ChunkID: r.ID, Score: r.Score})
		}
		return out, nil
	}
}

// runRecallCheck valida o contrato do baseline (LoadQrels já valida). É o
// --check default. Falha de contrato é sempre exit 1 (um baseline quebrado não
// pode silenciosamente passar o gate).
func runRecallCheck(cmd *cobra.Command, formatter *OutputFormatter, useJSON, summary bool, path string) error {
	qrels, err := grounding.LoadQrels(path)
	if err != nil {
		return err
	}

	if summary {
		fmt.Fprintln(cmd.OutOrStdout(), fmt.Sprintf("recall: baseline OK (%d queries)", len(qrels)))
		return nil
	}
	if useJSON {
		return printJSON(cmd, map[string]interface{}{
			"check":    "baseline",
			"file":     path,
			"queries":  len(qrels),
			"valid":    true,
		})
	}

	formatter.Header("GATE RECALL — contrato do baseline")
	formatter.Success("Baseline válido ✓")
	formatter.KeyValue("Arquivo", path)
	formatter.KeyValue("Queries do gabarito", fmt.Sprintf("%d", len(qrels)))
	formatter.KeyValue("Próximo passo", "cosca gate recall --audit")
	return nil
}

// runRecallAudit executa o RecallGate e aplica o fail-closed. Retorna
// ExitCodeError{1} quando --strict e o gate falhou. É a função pura/abanável:
// recebe um Retriever e o baseline, sem tocar no engine.
func runRecallAudit(cmd *cobra.Command, formatter *OutputFormatter, useJSON, strict, summary bool, retriever grounding.Retriever, baseline []grounding.Qrels, floor grounding.Floor) error {
	gate := grounding.NewRecallGate(floor)
	rep, err := gate.Run(cmd.Context(), retriever, baseline)
	if err != nil {
		return err
	}

	if summary {
		fmt.Fprintln(cmd.OutOrStdout(), recallSummaryLine(rep))
		if strict && !rep.Passed {
			return recallExit(false)
		}
		return nil
	}
	if useJSON {
		if err := printJSON(cmd, rep); err != nil {
			return err
		}
	} else {
		printRecallReport(formatter, rep)
	}

	if strict && !rep.Passed {
		return recallExit(false)
	}
	return nil
}

// recallExit traduz o veredito em ExitCodeError{1} (paridade catalogExit).
func recallExit(pass bool) error {
	if pass {
		return nil
	}
	return ExitCodeError{Code: 1}
}

// recallSummaryLine é a 1 linha curta do resumo para o hook.
func recallSummaryLine(rep *grounding.RecallReport) string {
	if rep.Passed {
		return fmt.Sprintf("recall: OK (recall@%d %.2f, first %.2f)", rep.K, rep.RecallAtK, rep.FirstRelevantHit)
	}
	return fmt.Sprintf("recall: FAIL (recall@%d %.2f, first %.2f, errors %d)", rep.K, rep.RecallAtK, rep.FirstRelevantHit, rep.QueriesErrored)
}

// printRecallReport renderiza o relatório humano do gate de recall.
func printRecallReport(formatter *OutputFormatter, rep *grounding.RecallReport) {
	formatter.Header(fmt.Sprintf("GATE RECALL — %d querie(s), K=%d", rep.TotalQueries, rep.K))
	if rep.Passed {
		formatter.Success("RECALL OK ✓")
	} else {
		formatter.Error("RECALL FAILEOU ✗")
	}

	formatter.KeyValue("recall@"+fmt.Sprintf("%d", rep.K), fmt.Sprintf("%.4f", rep.RecallAtK))
	formatter.KeyValue("first_relevant_hit (MRR)", fmt.Sprintf("%.4f", rep.FirstRelevantHit))
	formatter.KeyValue("expected_top1_hit", fmt.Sprintf("%.4f", rep.ExpectedTop1Hit))
	formatter.KeyValue("nDCG@"+fmt.Sprintf("%d", rep.K), fmt.Sprintf("%.4f", rep.NDCGAtK))
	formatter.KeyValue("queries_errored", fmt.Sprintf("%d", rep.QueriesErrored))
	formatter.KeyValue("piso recall@K", fmt.Sprintf("%.2f", rep.Floor.RecallAtK))
	formatter.KeyValue("piso first_relevant", fmt.Sprintf("%.2f", rep.Floor.FirstRelevant))

	if rep.Passed {
		return
	}
	formatter.Header("Motivos de falha (fail-closed)")
	for _, r := range rep.FailReasons {
		formatter.Bullet(r)
	}
	formatter.Header("Como tratar")
	formatter.Bullet("É débito de retrieval: o recall regrediu ou uma query erro.")
	formatter.Bullet("Investigue o ranking/reranker/embedding antes de relaxar o piso.")
	formatter.Bullet("Para travar em CI/qualidade, use: cosca gate recall --audit --strict")
}
