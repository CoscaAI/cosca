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
// FASE 0 (campanha "monolithic vs modular"): o baseline foi trocado de
// FICTÍCIO para REAL (ground-truth do benchmark em
// internal/search/vectorevidence_bench_test.go, benchmarkQueries). A flag
// `--arm` escolhe o braço medido:
//   - `--arm=monolithic` (default)  o retriever atual, FULL-SCAN/legacy
//     (idêntico ao comportamento histórico).
//   - `--arm=modular`    o braço ROTEADO (modlink.DefaultRoutes → ApplyScope →
//     RouteCandidateIDs → Search confinado). Uma query sem rota (NoRoute)
//     devolve resultado vazio sinalizado (no_route=true) — nunca full-scan.
//   - `--arm=both`       roda os DOIS, com tabela comparativa ou, em --json, um
//     struct {monolithic, modular, delta}.
//
// Observabilidade (crítica para a campanha): por query, o braço roteado expõe
// `no_route`, `candidate_count` (len(CandidateIDs)), `routed_module(s)` e
// `scanned` (vetores de fato escaneados). Sem isso o modular "venceria" por
// recusa, não por qualidade.
//
// Flags:
//   --strict    com --audit: qualquer falha => exit 1 (bloqueante). Sem
//               --strict, o achado vira débito reportado (exit 0).
//   --summary   imprime 1 linha curta p/ o hook (ex: "recall: OK").
//   --baseline  caminho do qrels-baseline.json (default: qrels-baseline.json).
//   --k         K do recall@K (default: 5).
//   --arm       monolithic | modular | both (default: monolithic).
//   --mode      legacy | modular (default: legacy) — metadado; não altera o
//               braço (o braço é quem decide o roteamento).
//   --audit     com persistência: grava .cosca/evals/recall/<arm>-<commit>-<ts>.json
//               (asset, timestamp, embedding, K, floor, queries, aggregates).
//
// Exemplos:
//   cosca gate recall --check
//   cosca gate recall --audit
//   cosca gate recall --audit --strict --summary
//   cosca gate recall --audit --json
//   cosca gate recall --audit --baseline internal/grounding/testdata/qrels-baseline.json --k 10
//   cosca gate recall --audit --arm modular
//   cosca gate recall --audit --arm both --json
//
// DEPENDÊNCIA: o --audit real exige o serviço de embeddings (Ollama em
// localhost:11434 com nomic-embed-text). Para CI sem embed, valide apenas com
// --check (não depende do engine).
//

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/grounding"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/modlink"
	"github.com/CoscaAI/cosca/internal/search"
	"github.com/CoscaAI/cosca/internal/vector"
)

// recallBaselineFileName é o default do arquivo de gabarito congelado.
const recallBaselineFileName = "qrels-baseline.json"

// braços da campanha (FASE 0) para a flag --arm.
const (
	armMonolithic = "monolithic"
	armModular    = "modular"
	armBoth       = "both"
)

// valida o valor da flag --arm (feita no cobra via validation, mantém o parse
// simples).
func validArm(v string) bool {
	switch v {
	case armMonolithic, armModular, armBoth:
		return true
	}
	return false
}

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
		arm      string
		mode     string
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

FASE 0 (campanha monolithic vs modular): --arm escolhe o braço medido.
  monolithic (default)  retriever FULL-SCAN/legacy (comportamento atual).
  modular               roteador determinístico (modlink.DefaultRoutes) →
                        ApplyScope → RouteCandidateIDs → Search confinado;
                        NoRoute ⇒ resultado vazio sinalizado (no_route=true).
  both                  roda os dois e produz uma tabela comparativa (ou, em
                        --json, {monolithic, modular, delta}).

Piso default: recall@5 >= 0.60, first_relevant_hit >= 0.80.

Flags:
  --strict    com --audit: falha => exit 1 (bloqueante).
  --summary   1 linha curta p/ o hook (ex: "recall: OK").
  --baseline  caminho do qrels-baseline.json.
  --k         K do recall@K (default 5).
  --arm       monolithic | modular | both (default monolithic).
  --mode      legacy | modular (default legacy) — metadado da corrida.

  --audit também grava uma corrida versionada em .cosca/evals/recall/
  <arm>-<commit>-<timestamp>.json (asset, timestamp, embedding model:dim, K,
  floor, queries com recall@K/MRR/nDCG/no_route/candidates_used/scanned, e
  aggregates). O aditamento informa aos agentes e ao relatório da campanha.`,
		Example: `  cosca gate recall --check
  cosca gate recall --audit
  cosca gate recall --audit --strict --summary
  cosca gate recall --audit --json
  cosca gate recall --audit --arm modular
  cosca gate recall --audit --arm both --json
  cosca gate recall --audit --baseline internal/grounding/testdata/qrels-baseline.json --k 10`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			if !validArm(arm) {
				return fmt.Errorf("--arm inválido: %q (use monolithic|modular|both)", arm)
			}
			if mode != "" && mode != search.ModeLegacy && mode != search.ModeModular {
				return fmt.Errorf("--mode inválido: %q (use legacy|modular)", mode)
			}

			path, err := resolveRecallBaseline(baseline)
			if err != nil {
				return err
			}

			if !audit {
				return runRecallCheck(cmd, formatter, useJSON, summary, path)
			}

			file, err := grounding.LoadQrelsFile(path)
			if err != nil {
				return err
			}
			qrels := file.Queries

			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			ke, err := newCLIKnowledgeEngine(dir)
			if err != nil {
				return fmt.Errorf("knowledge engine not available: %w", err)
			}
			defer func() { _ = ke.Close() }()

			floor := grounding.DefaultFloor()
			if k > 0 {
				floor.K = k
			}
			return runRecallAuditRun(cmd, formatter, useJSON, strict, summary, ke, qrels, floor, arm, mode, file.Embedding)
		},
	}

	cmd.Flags().BoolVar(&audit, "audit", false, "rodar o gate de recall (em vez de só validar o baseline)")
	cmd.Flags().BoolVar(&strict, "strict", false, "com --audit: tornar falhas bloqueantes (exit 1)")
	cmd.Flags().BoolVar(&summary, "summary", false, "imprimir 1 linha curta de resumo (p/ hook)")
	cmd.Flags().StringVar(&baseline, "baseline", "", "caminho do qrels-baseline.json")
	cmd.Flags().IntVar(&k, "k", 5, "K do recall@K (default 5)")
	cmd.Flags().StringVar(&arm, "arm", armMonolithic, "braço da campanha: monolithic | modular | both")
	cmd.Flags().StringVar(&mode, "mode", search.ModeLegacy, "metadado do modo: legacy | modular")
	return cmd
}

// buildKnowledgeRetriever cria um Retriever (braço MONOLITHIC / legacy) a partir
// de um knowledge.Engine: cada query gera o top-k via engine.Search com os params
// default (FULL-SCAN, sem roteamento) e mapeia os resultados para
// grounding.Result (ChunkID = result.ID). Captura, por query, o número de
// vetores efetivamente escaneados (MetricsSink) para a observabilidade "scanned".
// O engine é um ponteiro vivo — o chamador é responsável pelo Close.
func buildKnowledgeRetriever(ke *knowledge.Engine) grounding.Retriever {
	return func(ctx context.Context, query string, k int) ([]grounding.Result, error) {
		params := search.DefaultSearchParams()
		params.Query = query
		params.Limit = k
		scanned := 0
		ke.SetMetricsSink(func(m vector.SearchMetrics) { scanned = m.ScannedVectors })
		res, err := ke.Search(ctx, params)
		if err != nil {
			return nil, err
		}
		out := make([]grounding.Result, 0, len(res.Results))
		for _, r := range res.Results {
			if r.ID == "" {
				continue
			}
			out = append(out, grounding.Result{ChunkID: r.ID, Score: r.Score, Scanned: scanned})
		}
		return out, nil
	}
}

// buildRoutedKnowledgeRetriever cria um Retriever para o braço MODULAR (FASE 0):
//
//	query → modlink.DefaultRoutes → ApplyScope (SearchScope)
//	     → NoRoute? ⇒ resultado vazio sinalizado (no_route=true) — nunca full-scan
//	     → RouteCandidateIDs(scope) ⇒ params.CandidateIDs
//	     → engine.Search(params) confinado ao espaço roteado
//
// O resolver é passado pelo chamador (produção: NewResolver(DefaultRoutes())). O
// braço registra por query: no_route, routed_module(s), candidate_count (len dos
// candidatos permitidos) e scanned — a observabilidade que a campanha exige para
// não deixar o modular 'vencer' por recusa silenciosa.
func buildRoutedKnowledgeRetriever(ke *knowledge.Engine, resolver *modlink.Resolver) grounding.Retriever {
	return func(ctx context.Context, query string, k int) ([]grounding.Result, error) {
		params := search.DefaultSearchParams()
		params.Query = query
		params.Limit = k
		scanned := 0
		ke.SetMetricsSink(func(m vector.SearchMetrics) { scanned = m.ScannedVectors })

		scoped, scope := search.ApplyScope(resolver, query, params)
		if scope.NoRoute || len(scope.Modules) == 0 {
			// Recusa honesta: 0 resultados, sinalizado. Nunca full-scan.
			return []grounding.Result{{
				NoRoute:       true,
				RoutedModules: append([]string(nil), scope.Modules...),
				Scanned:       scanned,
			}}, nil
		}

		var cands []string
		if c, cErr := ke.RouteCandidateIDs(scope); cErr == nil && len(c) > 0 {
			cands = c
			scoped.CandidateIDs = c
		}

		res, sErr := ke.Search(ctx, scoped)
		if sErr != nil {
			return nil, sErr
		}
		out := make([]grounding.Result, 0, len(res.Results))
		for _, r := range res.Results {
			if r.ID == "" {
				continue
			}
			out = append(out, grounding.Result{
				ChunkID:        r.ID,
				Score:          r.Score,
				RoutedModules:  append([]string(nil), scope.Modules...),
				CandidateCount: len(cands),
				Scanned:        scanned,
			})
		}
		if len(out) == 0 {
			// Roteado mas sem resultados: carrega a observabilidade num sentinel
			// (ChunkID vazio, omitido da lista de recuperados — recall zero).
			return []grounding.Result{{
				RoutedModules:  append([]string(nil), scope.Modules...),
				CandidateCount: len(cands),
				Scanned:        scanned,
			}}, nil
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
			"check":   "baseline",
			"file":    path,
			"queries": len(qrels),
			"valid":   true,
		})
	}

	formatter.Header("GATE RECALL — contrato do baseline")
	formatter.Success("Baseline válido ✓")
	formatter.KeyValue("Arquivo", path)
	formatter.KeyValue("Queries do gabarito", fmt.Sprintf("%d", len(qrels)))
	formatter.KeyValue("Próximo passo", "cosca gate recall --audit")
	return nil
}

// runRecallAuditReport executa o RecallGate, imprime o relatório (humano ou
// JSON), aplica o fail-closed do --strict e devolve o relatório para a
// observabilidade/persistência. É o núcleo de um braço único; a variante
// retrocompatível runRecallAudit descarta o relatório.
func runRecallAuditReport(cmd *cobra.Command, formatter *OutputFormatter, useJSON, strict, summary bool, retriever grounding.Retriever, baseline []grounding.Qrels, floor grounding.Floor) (*grounding.RecallReport, error) {
	gate := grounding.NewRecallGate(floor)
	rep, err := gate.Run(cmd.Context(), retriever, baseline)
	if err != nil {
		return rep, err
	}

	if summary {
		fmt.Fprintln(cmd.OutOrStdout(), recallSummaryLine(rep))
		if strict && !rep.Passed {
			return rep, recallExit(false)
		}
		return rep, nil
	}
	if useJSON {
		if err := printJSON(cmd, rep); err != nil {
			return rep, err
		}
	} else {
		printRecallReport(formatter, rep)
	}

	if strict && !rep.Passed {
		return rep, recallExit(false)
	}
	return rep, nil
}

// runRecallAudit executa o RecallGate e aplica o fail-closed. Retorna
// ExitCodeError{1} quando --strict e o gate falhou. É a função pura/abanável:
// recebe um Retriever e o baseline, sem tocar no engine. Mantida para
// retrocompatibilidade (testes) — delega ao núcleo runRecallAuditReport.
func runRecallAudit(cmd *cobra.Command, formatter *OutputFormatter, useJSON, strict, summary bool, retriever grounding.Retriever, baseline []grounding.Qrels, floor grounding.Floor) error {
	_, err := runRecallAuditReport(cmd, formatter, useJSON, strict, summary, retriever, baseline, floor)
	return err
}

// runRecallAuditRun orquestra a campanha: escolhe o(s) braço(s) pela flag --arm,
// roda o(s) gate(s) e persiste a corrida versionada em .cosca/evals/recall/.
func runRecallAuditRun(cmd *cobra.Command, formatter *OutputFormatter, useJSON, strict, summary bool, ke *knowledge.Engine, qrels []grounding.Qrels, floor grounding.Floor, arm, mode string, embed grounding.EmbeddingSig) error {
	switch arm {
	case armModular:
		resolver := modlink.NewResolver(modlink.DefaultRoutes())
		retriever := buildRoutedKnowledgeRetriever(ke, resolver)
		rep, err := runRecallAuditReport(cmd, formatter, useJSON, strict, summary, retriever, qrels, floor)
		if err != nil {
			return err
		}
		if _, perr := persistRecallAudit(arm, mode, embed, floor, map[string]*grounding.RecallReport{arm: rep}); perr != nil {
			return fmt.Errorf("persist recall audit: %w", perr)
		}
		return nil
	case armBoth:
		return runRecallAuditBoth(cmd, formatter, useJSON, strict, summary, ke, qrels, floor, mode, embed)
	default: // armMonolithic
		retriever := buildKnowledgeRetriever(ke)
		rep, err := runRecallAuditReport(cmd, formatter, useJSON, strict, summary, retriever, qrels, floor)
		if err != nil {
			return err
		}
		if _, perr := persistRecallAudit(arm, mode, embed, floor, map[string]*grounding.RecallReport{arm: rep}); perr != nil {
			return fmt.Errorf("persist recall audit: %w", perr)
		}
		return nil
	}
}

// runRecallAuditBoth mede os dois braços com o MESMO qrels e produz um relatório
// comparativo: em --json, um struct {monolithic, modular, delta}; em texto, uma
// tabela por query + agregado. Persiste a corrida versionada.
func runRecallAuditBoth(cmd *cobra.Command, formatter *OutputFormatter, useJSON, strict, summary bool, ke *knowledge.Engine, qrels []grounding.Qrels, floor grounding.Floor, mode string, embed grounding.EmbeddingSig) error {
	resolver := modlink.NewResolver(modlink.DefaultRoutes())
	mono := buildKnowledgeRetriever(ke)
	mod := buildRoutedKnowledgeRetriever(ke, resolver)
	gate := grounding.NewRecallGate(floor)

	mRep, err := gate.Run(cmd.Context(), mono, qrels)
	if err != nil {
		return err
	}
	dRep, err := gate.Run(cmd.Context(), mod, qrels)
	if err != nil {
		return err
	}

	if _, perr := persistRecallAudit(armBoth, mode, embed, floor, map[string]*grounding.RecallReport{
		armMonolithic: mRep,
		armModular:    dRep,
	}); perr != nil {
		return fmt.Errorf("persist recall audit: %w", perr)
	}

	if summary {
		fmt.Fprintf(cmd.OutOrStdout(), "both: mono [%s] | modular [%s]\n", recallSummaryLine(mRep), recallSummaryLine(dRep))
	} else if useJSON {
		if err := printJSON(cmd, buildRecallBothJSON(mRep, dRep, mode)); err != nil {
			return err
		}
	} else {
		printRecallBoth(formatter, mRep, dRep)
	}

	if strict && (!mRep.Passed || !dRep.Passed) {
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
	formatter.Header(fmt.Sprintf("GATE RECALL — arm=%s %d querie(s), K=%d", reportArmLabel(rep), rep.TotalQueries, rep.K))
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
	formatter.KeyValue("no_route_queries", fmt.Sprintf("%d", countNoRoute(rep)))
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

// countNoRoute conta quantas queries do relatório foram recusadas por no-route
// (observabilidade FASE 0).
func countNoRoute(rep *grounding.RecallReport) int {
	n := 0
	for _, q := range rep.Queries {
		if q.NoRoute {
			n++
		}
	}
	return n
}

// reportArmLabel devolve um rótulo curto do relatório (não temos o arm no
// relatório; usamos um placeholder estável para a vitrine).
func reportArmLabel(rep *grounding.RecallReport) string {
	if countNoRoute(rep) == len(rep.Queries) && len(rep.Queries) > 0 {
		return "modular(no-route)"
	}
	if countNoRoute(rep) > 0 {
		return "modular"
	}
	return "monolithic"
}

// printRecallBoth renderiza a tabela comparativa por query + o agregado.
func printRecallBoth(formatter *OutputFormatter, mono, mod *grounding.RecallReport) {
	formatter.Header(fmt.Sprintf("GATE RECALL — both | %d querie(s), K=%d", mono.TotalQueries, mono.K))

	rows := make([][]string, 0, len(mono.Queries)+1)
	for i := range mono.Queries {
		q := mono.Queries[i]
		m := mod.Queries[i]
		noRoute := ""
		if m.NoRoute {
			noRoute = "NO_ROUTE"
		}
		rows = append(rows, []string{
			truncate(q.Query, 44),
			fmt.Sprintf("%.2f / %.2f", q.Recall, m.Recall),
			fmt.Sprintf("%.2f / %.2f", q.FirstRelevant, m.FirstRelevant),
			fmt.Sprintf("%.2f / %.2f", q.NDCG, m.NDCG),
			noRoute,
		})
	}
	formatter.Table([]string{"Query", "recall@K mono/mod", "MRR mono/mod", "nDCG mono/mod", "no-route"}, rows)

	formatter.Header("Agregado")
	rowsA := [][]string{
		{"recall@K", fmt.Sprintf("%.4f", mono.RecallAtK), fmt.Sprintf("%.4f", mod.RecallAtK), fmt.Sprintf("%+.4f", mod.RecallAtK-mono.RecallAtK)},
		{"first_relevant_hit (MRR)", fmt.Sprintf("%.4f", mono.FirstRelevantHit), fmt.Sprintf("%.4f", mod.FirstRelevantHit), fmt.Sprintf("%+.4f", mod.FirstRelevantHit-mono.FirstRelevantHit)},
		{"nDCG@K", fmt.Sprintf("%.4f", mono.NDCGAtK), fmt.Sprintf("%.4f", mod.NDCGAtK), fmt.Sprintf("%+.4f", mod.NDCGAtK-mono.NDCGAtK)},
		{"expected_top1_hit", fmt.Sprintf("%.4f", mono.ExpectedTop1Hit), fmt.Sprintf("%.4f", mod.ExpectedTop1Hit), fmt.Sprintf("%+.4f", mod.ExpectedTop1Hit-mono.ExpectedTop1Hit)},
		{"queries_errored", fmt.Sprintf("%d", mono.QueriesErrored), fmt.Sprintf("%d", mod.QueriesErrored), fmt.Sprintf("%+d", mod.QueriesErrored-mono.QueriesErrored)},
		{"no_route_queries", fmt.Sprintf("%d", countNoRoute(mono)), fmt.Sprintf("%d", countNoRoute(mod)), fmt.Sprintf("%+d", countNoRoute(mod)-countNoRoute(mono))},
	}
	formatter.Table([]string{"Métrica", "monolithic", "modular", "Δ (mod−mono)"}, rowsA)
	formatter.Bullet("Δ > 0 ⇒ o braço modular ganhou; Δ < 0 ⇒ perdeu. NoRoute ⇒ recusa honesta (0 recall), nunca full-scan.")
}

// buildRecallBothJSON monta o struct JSON do modo both.
func buildRecallBothJSON(mono, mod *grounding.RecallReport, mode string) map[string]interface{} {
	return map[string]interface{}{
		"arm":        armBoth,
		"mode":       mode,
		"k":          mono.K,
		"monolithic": mono,
		"modular":    mod,
		"delta": map[string]interface{}{
			"recall_at_k":             roundDiff(mod.RecallAtK - mono.RecallAtK),
			"first_relevant_hit":      roundDiff(mod.FirstRelevantHit - mono.FirstRelevantHit),
			"ndcg_at_k":               roundDiff(mod.NDCGAtK - mono.NDCGAtK),
			"expected_top1_hit":       roundDiff(mod.ExpectedTop1Hit - mono.ExpectedTop1Hit),
			"queries_errored":         mod.QueriesErrored - mono.QueriesErrored,
			"no_route_monolithic":     countNoRoute(mono),
			"no_route_modular":        countNoRoute(mod),
			"scanned_mean_monolithic": meanScanned(mono),
			"scanned_mean_modular":    meanScanned(mod),
		},
	}
}

// roundDiff arredonda uma diferença para 4 casas (padrão das métricas).
func roundDiff(v float64) float64 {
	return float64(int(v*10000+0.5)) / 10000
}

// meanScanned calcula a média de vetores escaneados sobre as queries do relatório.
func meanScanned(rep *grounding.RecallReport) float64 {
	if len(rep.Queries) == 0 {
		return 0
	}
	sum := 0
	for _, q := range rep.Queries {
		sum += q.Scanned
	}
	return roundDiff(float64(sum) / float64(len(rep.Queries)))
}

// ── Persistência versionada por corrida (FASE 0) ────────────────────────────

// recallPersist é o esquema do arquivo .cosca/evals/recall/<arm>-<commit>-<ts>.json.
type recallPersist struct {
	Asset          string                      `json:"asset"`
	Timestamp      string                      `json:"timestamp"`
	Arm            string                      `json:"arm"`
	Mode           string                      `json:"mode"`
	EmbeddingModel string                      `json:"embedding_model"`
	EmbeddingDim   string                      `json:"embedding_dim"`
	K              int                         `json:"k"`
	Floor          recallPersistFloor          `json:"floor"`
	Queries        []recallPersistQuery        `json:"queries"`
	Aggregates     map[string]recallPersistAgg `json:"aggregates"`
}

// recallPersistFloor é o piso aplicado (layout JSON-friendly).
type recallPersistFloor struct {
	K             int     `json:"k"`
	RecallAtK     float64 `json:"recall_at_k"`
	FirstRelevant float64 `json:"first_relevant"`
}

// recallPersistQuery é a observabilidade por query de um braço.
type recallPersistQuery struct {
	Query          string   `json:"query"`
	Arm            string   `json:"arm"`
	RecallAtK      float64  `json:"recall_at_k"`
	MRR            float64  `json:"mrr"`
	NDCG           float64  `json:"ndcg"`
	ExpectedTop1   float64  `json:"expected_top1"`
	NoRoute        bool     `json:"no_route"`
	CandidatesUsed int      `json:"candidates_used"`
	Scanned        int      `json:"scanned"`
	RoutedModules  []string `json:"routed_modules,omitempty"`
	Errored        bool     `json:"errored"`
	Err            string   `json:"err,omitempty"`
}

// recallPersistAgg é o agregado de um braço.
type recallPersistAgg struct {
	RecallAtK        float64 `json:"recall_at_k"`
	FirstRelevantHit float64 `json:"first_relevant_hit"`
	ExpectedTop1Hit  float64 `json:"expected_top1_hit"`
	NDCGAtK          float64 `json:"ndcg_at_k"`
	QueriesErrored   int     `json:"queries_errored"`
	NoRouteQueries   int     `json:"no_route_queries"`
	TotalQueries     int     `json:"total_queries"`
	ScannedMean      float64 `json:"scanned_mean"`
	Passed           bool    `json:"passed"`
}

// persistRecallAudit grava a corrida versionada em .cosca/evals/recall/. Devolve
// o caminho gravado ou um erro (não-bloqueante para o gate). Para `arms` encha
// com um ou dois relatórios (chave = nome do braço).
func persistRecallAudit(arm, mode string, embed grounding.EmbeddingSig, floor grounding.Floor, arms map[string]*grounding.RecallReport) (string, error) {
	asset := gitHead()
	ts := time.Now().UTC().Format(time.RFC3339)

	p := recallPersist{
		Asset:     asset,
		Timestamp: ts,
		Arm:       arm,
		Mode:      mode,
		K:         floor.K,
		Floor: recallPersistFloor{
			K:             floor.K,
			RecallAtK:     floor.RecallAtK,
			FirstRelevant: floor.FirstRelevant,
		},
	}

	// Assinatura de embedding: usa a do baseline se presente; senão o corpus
	// (nomic-embed-text:768) como fallback honesto.
	if embed.Model != "" {
		p.EmbeddingModel = embed.Model
		p.EmbeddingDim = embed.Dim
	} else {
		p.EmbeddingModel = "nomic-embed-text"
		p.EmbeddingDim = "768"
	}

	// Ordem determinística dos braços (para o arquivo ser estável).
	names := make([]string, 0, len(arms))
	for name := range arms {
		names = append(names, name)
	}
	sort.Strings(names)

	p.Aggregates = make(map[string]recallPersistAgg, len(names))
	for _, name := range names {
		rep := arms[name]
		p.Aggregates[name] = buildPersistAgg(rep)
		for _, q := range rep.Queries {
			p.Queries = append(p.Queries, recallPersistQuery{
				Query:          q.Query,
				Arm:            name,
				RecallAtK:      q.Recall,
				MRR:            q.FirstRelevant,
				NDCG:           q.NDCG,
				ExpectedTop1:   q.ExpectedTop1,
				NoRoute:        q.NoRoute,
				CandidatesUsed: q.CandidateCount,
				Scanned:        q.Scanned,
				RoutedModules:  q.RoutedModules,
				Errored:        q.Errored,
				Err:            q.Err,
			})
		}
	}
	// Ordena as queries por braço e depois pela ordem original (query).
	sort.SliceStable(p.Queries, func(i, j int) bool { return p.Queries[i].Arm < p.Queries[j].Arm })

	outDir := filepath.Join(".cosca", "evals", "recall")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("create eval dir: %w", err)
	}
	stamp := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s-%s.json", arm, shortCommit(asset), stamp)
	path := filepath.Join(outDir, filename)

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// buildPersistAgg converte um RecallReport no agregado de persistência.
func buildPersistAgg(rep *grounding.RecallReport) recallPersistAgg {
	return recallPersistAgg{
		RecallAtK:        rep.RecallAtK,
		FirstRelevantHit: rep.FirstRelevantHit,
		ExpectedTop1Hit:  rep.ExpectedTop1Hit,
		NDCGAtK:          rep.NDCGAtK,
		QueriesErrored:   rep.QueriesErrored,
		NoRouteQueries:   countNoRoute(rep),
		TotalQueries:     rep.TotalQueries,
		ScannedMean:      meanScanned(rep),
		Passed:           rep.Passed,
	}
}

// gitHead devolve o commit HEAD (git rev-parse HEAD) ou "unknown" quando não for
// possível (fora de repo, binário ausente, etc.).
func gitHead() string {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// shortCommit encurta um commit para 8 caracteres (ou o próprio quando menor).
func shortCommit(c string) string {
	if len(c) > 8 {
		return c[:8]
	}
	return c
}
