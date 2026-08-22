//
// `cosca trace` — Trace ID universal + eventos estruturados (flight recorder).
//
// Regra do Don: "Eu colocaria um Trace ID universal: toda operação importante
// ganha um identificador TRACE-20260802-7F92." Cada evento carrega o contexto
// estruturado (trace_id, parent_event, timestamp, actor, action, input_hash,
// output_hash, code_version, knowledge_version, cognitive_version,
// environment, result) — o suficiente para reconstruir a história e detectar
// o ponto de divergência: "SUSPICIOUS DIVERGENCE — onde esta execução difere
// das anteriores".
//
// O ledger vive em .cosca/trace.db (SQLite) e é APPEND-ONLY: nada aqui
// edita/apaga eventos. O flight recorder nunca reescreve o passado.
//
// Subcomandos:
//   new                              Imprime um novo Trace ID universal
//   event <id> --action --actor ...  Registra um evento estruturado no trace
//   show <id>                        Linha do tempo (flight recorder) do trace
//   causal <id> [--baseline <id>]    Cadeia causa → consequência (grafo causal)
//                                    + pontos de divergência com um trace anterior
//   diff <id-a> <id-b>               Compara as sequências de ações e aponta
//                                    onde <id-b> diverge de <id-a>
//                                    (SUSPICIOUS DIVERGENCE na posição N)
//   latest [--limit N]               Eventos recentes de todos os traces
//

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/trace"
)

// resolveTraceStore abre o ledger de traces do projeto atual
// (<projeto>/.cosca/trace.db).
func resolveTraceStore() (*trace.Store, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getwd: %w", err)
	}
	return trace.NewStore(filepath.Join(dir, ".cosca", "trace.db"))
}

// NewTraceCommand cria a árvore de comandos `cosca trace`.
func NewTraceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trace",
		Short: "Trace ID universal + eventos estruturados (flight recorder) e detecção de divergência",
		Long: `Trace ID universal + eventos estruturados (flight recorder).

Toda operação importante ganha um Trace ID universal (TRACE-20260802-7F92) e
cada evento carrega o contexto estruturado necessário para reconstruir a
história: trace_id, parent_event, timestamp, actor, action, input_hash,
output_hash, code_version, knowledge_version, cognitive_version, environment e
result. Com a história completa dá para detectar o ponto de divergência —
"SUSPICIOUS DIVERGENCE", onde esta execução difere das anteriores.

O ledger vive em .cosca/trace.db (SQLite) e é APPEND-ONLY — nunca edita nem
apaga eventos. O flight recorder não reescreve o passado.

Subcomandos:
  new                              Imprime um novo Trace ID universal
  event <id> --action --actor ...  Registra um evento estruturado no trace
  show <id>                        Linha do tempo (flight recorder) do trace
  causal <id> [--baseline <id>]    Cadeia causa → consequência (grafo causal)
  diff <id-a> <id-b>               Compara as sequências de ações e aponta
                                   onde <id-b> diverge de <id-a>
  latest [--limit N]               Eventos recentes de todos os traces`,
		Example: `  cosca trace new
  cosca trace event TRACE-20260802-7F92 --action TASK_STARTED --actor kernel --details "tarefa #42"
  cosca trace show TRACE-20260802-7F92
  cosca trace causal TRACE-20260802-7F92 --baseline TRACE-20260802-1A2B
  cosca trace diff TRACE-20260802-7F92 TRACE-20260802-1A2B
  cosca trace latest --limit 10`,
	}

	cmd.AddCommand(
		NewTraceNewCommand(),
		NewTraceEventCommand(),
		NewTraceShowCommand(),
		NewTraceCausalCommand(),
		NewTraceDiffCommand(),
		NewTraceLatestCommand(),
	)
	return cmd
}

// NewTraceNewCommand cria `cosca trace new`.
func NewTraceNewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "new",
		Short: "Imprime um novo Trace ID universal (TRACE-YYYYMMDD-XXXX)",
		Long: `Gera e imprime um novo Trace ID universal no formato do Don:
TRACE-YYYYMMDD-XXXX (sufixo de 4 hex chars derivado de crypto/rand). Use-o
para iniciar uma operação importante e registre os eventos com "cosca trace
event <id>".`,
		Example: `  cosca trace new`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			id := trace.NewID()
			if IsJSONOutput(cmd) {
				return printJSON(cmd, map[string]string{"trace_id": id.String()})
			}
			formatter.Success("Novo Trace ID universal criado:")
			formatter.KeyValue("Trace ID", id.String())
			return nil
		},
	}
}

// NewTraceEventCommand cria `cosca trace event <id>`.
func NewTraceEventCommand() *cobra.Command {
	var (
		parentEvent, actor, action string
		inputHash, outputHash      string
		codeVersion, knowledgeVer  string
		cognitiveVer, environment  string
		result, details            string
	)

	cmd := &cobra.Command{
		Use:   "event <id>",
		Short: "Registra um evento estruturado no trace (append-only)",
		Long: `Registra um evento estruturado no ledger (.cosca/trace.db). O trace
é append-only: cada execução adiciona ao passado, nunca reescreve.
Defaults automáticos: timestamp = agora (UTC); environment = GOOS/GOARCH;
code_version = short hash do git quando --code-version não for dado.

  --action   a ação do evento (ex.: PLAN_CREATED, TASK_STARTED, TEST_FAILED)
  --actor    quem agiu (ex.: kernel, agent-x, don)`,
		Example: `  cosca trace event TRACE-20260802-7F92 --action TASK_STARTED --actor kernel --result running --details "tarefa #42"
  cosca trace event TRACE-20260802-7F92 --action TEST_FAILED --actor agent-x --result failed`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)

			id, ok := trace.Parse(args[0])
			if !ok {
				return fmt.Errorf("Trace ID inválido %q (esperava TRACE-YYYYMMDD-XXXX)", args[0])
			}

			if strings.TrimSpace(action) == "" {
				return fmt.Errorf("--action é obrigatório (ex.: TASK_STARTED)")
			}
			if strings.TrimSpace(actor) == "" {
				return fmt.Errorf("--actor é obrigatório (ex.: kernel, agent-x, don)")
			}

			store, err := resolveTraceStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			ev := trace.Event{
				TraceID:          id.String(),
				ParentEvent:      parentEvent,
				Actor:            actor,
				Action:           action,
				InputHash:        inputHash,
				OutputHash:       outputHash,
				CodeVersion:      codeVersion,
				KnowledgeVersion: knowledgeVer,
				CognitiveVersion: cognitiveVer,
				Environment:      environment,
				Result:           result,
				Details:          details,
			}
			if ev.CodeVersion == "" {
				ev.CodeVersion = gitShortHash()
			}

			if err := store.Append(ev); err != nil {
				return err
			}

			if IsJSONOutput(cmd) {
				return printJSON(cmd, ev)
			}
			formatter.Success(fmt.Sprintf("Evento registrado no trace %s", ev.TraceID))
			formatter.KeyValue("Ação", ev.Action)
			formatter.KeyValue("Actor", ev.Actor)
			if ev.Result != "" {
				formatter.KeyValue("Resultado", ev.Result)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&parentEvent, "parent-event", "", "id do evento pai (estrutura de árvore)")
	cmd.Flags().StringVar(&actor, "actor", "", "quem agiu (kernel, agent-x, don) — obrigatório")
	cmd.Flags().StringVar(&action, "action", "", "ação do evento (PLAN_CREATED, TASK_STARTED, TEST_FAILED, ...) — obrigatório")
	cmd.Flags().StringVar(&inputHash, "input-hash", "", "SHA-256 do input")
	cmd.Flags().StringVar(&outputHash, "output-hash", "", "SHA-256 do output")
	cmd.Flags().StringVar(&codeVersion, "code-version", "", "versão do código (git describe/short); padrão: git rev-parse --short HEAD")
	cmd.Flags().StringVar(&knowledgeVer, "knowledge-version", "", "versão do conhecimento (CV-XXXX)")
	cmd.Flags().StringVar(&cognitiveVer, "cognitive-version", "", "versão cognitiva")
	cmd.Flags().StringVar(&environment, "environment", "", "ambiente (ex.: linux/amd64); padrão: GOOS/GOARCH")
	cmd.Flags().StringVar(&result, "result", "", "resultado (success, failed, running, ...)")
	cmd.Flags().StringVar(&details, "details", "", "detalhes livres do evento")
	return cmd
}

// NewTraceShowCommand cria `cosca trace show <id>` — a vista flight recorder.
func NewTraceShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Linha do tempo (flight recorder) de um trace — tabela ordenada por tempo",
		Long: `Imprime a linha do tempo completa de um trace: tabela (tempo, actor,
ação, resultado, detalhes) ordenada por timestamp — a vista "flight recorder"
que permite reconstruir a história da operação.

  --json   emite os eventos brutos em JSON (mesmos campos do ledger).`,
		Example: `  cosca trace show TRACE-20260802-7F92
  cosca trace show TRACE-20260802-7F92 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)

			id, ok := trace.Parse(args[0])
			if !ok {
				return fmt.Errorf("Trace ID inválido %q (esperava TRACE-YYYYMMDD-XXXX)", args[0])
			}

			store, err := resolveTraceStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			events, err := store.Get(id.String())
			if err != nil {
				return err
			}
			if IsJSONOutput(cmd) {
				return printJSON(cmd, events)
			}
			if len(events) == 0 {
				formatter.Warning(fmt.Sprintf("Trace %s sem eventos — registre com \"cosca trace event %s --action ... --actor ...\".", id, id))
				return nil
			}

			formatter.Header(fmt.Sprintf("TRACE %s — flight recorder", id))
			formatter.KeyValue("Eventos", fmt.Sprintf("%d", len(events)))
			formatter.KeyValue("Primeiro", time.Unix(events[0].Timestamp, 0).Format(time.RFC3339))
			formatter.KeyValue("Último", time.Unix(events[len(events)-1].Timestamp, 0).Format(time.RFC3339))

			rows := make([][]string, 0, len(events))
			for _, e := range events {
				rows = append(rows, []string{
					time.Unix(e.Timestamp, 0).Format("2006-01-02 15:04:05"),
					e.Actor,
					e.Action,
					e.Result,
					truncateTraceDetails(e.Details, 60),
				})
			}
			formatter.Table([]string{"Tempo", "Actor", "Ação", "Resultado", "Detalhes"}, rows)
			return nil
		},
	}
}

// NewTraceCausalCommand cria `cosca trace causal <id> [--baseline <id>]` — a
// cadeia causa → consequência do trace.
func NewTraceCausalCommand() *cobra.Command {
	var baseline string

	cmd := &cobra.Command{
		Use:   "causal <id>",
		Short: "Cadeia causa → consequência do trace (grafo causal) + pontos de divergência",
		Long: `Imprime a cadeia causa → consequência do trace (grafo causal): cada
evento vira um nó e eventos consecutivos são ligados por "caused" (evento N
causou o evento N+1) — determinístico, sem LLM. A cadeia responde "por que
isto aconteceu?": Knowledge K-81 → Agent-72 decision → daemon.go changed →
test failed → Agent-72 correction → test passed → Reviewer approved.

Com --baseline <id> (um trace anterior), também aponta os índices onde esta
execução diverge do histórico (heurística DetectDivergence — o marcador
"SUSPICIOUS DIVERGENCE" do Don). Extensão ao final é normal e não é marcada.

  --json       emite o grafo causal (nós + arestas) em JSON, com divergência
  --baseline   Trace ID anterior para comparar e apontar a divergência`,
		Example: `  cosca trace causal TRACE-20260802-7F92
  cosca trace causal TRACE-20260802-7F92 --baseline TRACE-20260802-1A2B
  cosca trace causal TRACE-20260802-7F92 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)

			id, ok := trace.Parse(args[0])
			if !ok {
				return fmt.Errorf("Trace ID inválido %q (esperava TRACE-YYYYMMDD-XXXX)", args[0])
			}

			store, err := resolveTraceStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			events, err := store.Get(id.String())
			if err != nil {
				return err
			}
			if len(events) == 0 {
				formatter.Warning(fmt.Sprintf("Trace %s sem eventos — registre com \"cosca trace event %s --action ... --actor ...\".", id, id))
				return nil
			}

			cg := trace.BuildCausalGraph(events)

			var (
				baselineID     string
				baselineEvents []trace.Event
				divergence     []int
			)
			if strings.TrimSpace(baseline) != "" {
				bid, okB := trace.Parse(baseline)
				if !okB {
					return fmt.Errorf("Trace ID inválido para --baseline %q (esperava TRACE-YYYYMMDD-XXXX)", baseline)
				}
				if bid == id {
					return fmt.Errorf("--baseline não pode ser o próprio trace %s", id)
				}
				be, err := store.Get(bid.String())
				if err != nil {
					return err
				}
				if len(be) == 0 {
					return fmt.Errorf("trace anterior %s sem eventos — registre eventos antes de comparar", bid)
				}
				baselineID = bid.String()
				baselineEvents = be
				divergence = cg.DivergencePoints(baselineEvents)
			}

			if IsJSONOutput(cmd) {
				return printJSON(cmd, traceCausalResult{
					CausalGraph: cg,
					Baseline:    baselineID,
					Divergence:  divergence,
					Suspicious:  len(divergence) > 0,
				})
			}

			formatter.Header(fmt.Sprintf("CAUSAL GRAPH %s — causa → consequência", id))
			formatter.KeyValue("Eventos", fmt.Sprintf("%d", len(cg.Nodes)))
			formatter.KeyValue("Arestas caused", fmt.Sprintf("%d", len(cg.Edges)))
			if baselineID != "" {
				formatter.KeyValue("Baseline (anterior)", baselineID)
			}

			if len(divergence) > 0 {
				formatter.Error("SUSPICIOUS DIVERGENCE — esta execução difere das anteriores:")
				curSeq := trace.SequenceFromEvents(events)
				for _, pos := range divergence {
					expected := actionAtPosition(trace.SequenceFromEvents(baselineEvents), pos)
					got := actionAtPosition(curSeq, pos)
					formatter.Bullet(fmt.Sprintf("posição %d: anterior=%q ≠ atual=%q", pos+1, expected, got))
				}
			} else if baselineID != "" {
				formatter.Success("Sem divergência — esta execução coincide com o trace anterior (ou é extensão normal ao final).")
			}

			for i, n := range cg.Nodes {
				line := fmt.Sprintf("%d. %s", i+1, n.Action)
				if n.Actor != "" {
					line += fmt.Sprintf(" (%s)", n.Actor)
				}
				if n.Result != "" {
					line += fmt.Sprintf(" → %s", n.Result)
				}
				formatter.Println(line)
				if i < len(cg.Nodes)-1 {
					formatter.Println("   ↓ caused")
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&baseline, "baseline", "", "Trace ID anterior para comparar e apontar a divergência (opcional)")
	return cmd
}

// traceCausalResult é a forma JSON de `cosca trace causal <id>`: o grafo
// causal (herdado de CausalGraph) mais os pontos de divergência com o
// baseline, quando informado.
type traceCausalResult struct {
	trace.CausalGraph
	Baseline   string `json:"baseline,omitempty"`
	Divergence []int  `json:"divergence"`
	Suspicious bool   `json:"suspicious"`
}

// NewTraceDiffCommand cria `cosca trace diff <id-a> <id-b>`.
func NewTraceDiffCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "diff <id-a> <id-b>",
		Short: "Compara as sequências de ações e aponta a divergência (SUSPICIOUS DIVERGENCE)",
		Long: `Compara a sequência de ações de <id-a> (referência/anterior) com a de
<id-b> (execução atual) usando a heurística determinística DetectDivergence
(sem LLM) e imprime os índices onde <id-b> diverge — o marcador
"SUSPICIOUS DIVERGENCE" do Don.

Regra: extensão ao final (prefix-extension) é normal e NÃO é marcada; uma
ação que difere do que as execuções anteriores fizeram NA MESMA posição (ou
uma ação inesperada no meio da sequência) é divergência.`,
		Example: `  cosca trace diff TRACE-20260802-7F92 TRACE-20260802-1A2B`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)

			idA, okA := trace.Parse(args[0])
			idB, okB := trace.Parse(args[1])
			if !okA || !okB {
				return fmt.Errorf("Trace IDs inválidos (esperava TRACE-YYYYMMDD-XXXX): %q, %q", args[0], args[1])
			}

			store, err := resolveTraceStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			eventsA, err := store.Get(idA.String())
			if err != nil {
				return err
			}
			eventsB, err := store.Get(idB.String())
			if err != nil {
				return err
			}
			if len(eventsA) == 0 || len(eventsB) == 0 {
				return fmt.Errorf("ambos os traces precisam ter eventos para comparação (%s: %d, %s: %d)",
					idA, len(eventsA), idB, len(eventsB))
			}

			seqA := trace.SequenceFromEvents(eventsA)
			seqB := trace.SequenceFromEvents(eventsB)
			divergence := trace.DetectDivergence([]trace.Sequence{seqA}, seqB)

			if IsJSONOutput(cmd) {
				return printJSON(cmd, map[string]interface{}{
					"baseline":   idA.String(),
					"current":    idB.String(),
					"sequence_a": seqA.Actions,
					"sequence_b": seqB.Actions,
					"divergence": divergence,
					"suspicious": len(divergence) > 0,
				})
			}

			formatter.Header("DIFF TRACE — detecção de divergência")
			formatter.KeyValue("Referência (anterior)", idA.String())
			formatter.KeyValue("Execução atual", idB.String())
			formatter.KeyValue("Sequência A", strings.Join(seqA.Actions, " → "))
			formatter.KeyValue("Sequência B", strings.Join(seqB.Actions, " → "))

			if len(divergence) == 0 {
				formatter.Success("Sem divergência — a sequência B coincide com A (ou é uma extensão normal ao final).")
				return nil
			}

			formatter.Error("SUSPICIOUS DIVERGENCE — esta execução difere das anteriores:")
			for _, pos := range divergence {
				got := "—"
				if pos < len(seqB.Actions) {
					got = seqB.Actions[pos]
				}
				expected := actionAtPosition(seqA, pos)
				formatter.Bullet(fmt.Sprintf("posição %d: A=%q (esperado) ≠ B=%q (atual)", pos+1, expected, got))
			}
			return nil
		},
	}
}

// NewTraceLatestCommand cria `cosca trace latest [--limit N]`.
func NewTraceLatestCommand() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "latest",
		Short: "Eventos recentes de todos os traces (flight recorder global)",
		Long: `Lista os eventos mais recentes de todos os traces, do mais novo para o
mais antigo, em uma tabela (tempo, trace, actor, ação, resultado, detalhes).
Rastro append-only — apenas leitura.

  --limit N   número máximo de eventos (padrão 20)`,
		Example: `  cosca trace latest
  cosca trace latest --limit 50
  cosca trace latest --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

			store, err := resolveTraceStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			events, err := store.Latest(limit)
			if err != nil {
				return err
			}
			if IsJSONOutput(cmd) {
				return printJSON(cmd, events)
			}
			if len(events) == 0 {
				formatter.Warning("Nenhum evento registrado ainda — rode \"cosca trace new\" e \"cosca trace event ...\".")
				return nil
			}

			rows := make([][]string, 0, len(events))
			for _, e := range events {
				rows = append(rows, []string{
					time.Unix(e.Timestamp, 0).Format("2006-01-02 15:04:05"),
					e.TraceID,
					e.Actor,
					e.Action,
					e.Result,
					truncateTraceDetails(e.Details, 40),
				})
			}
			formatter.Header(fmt.Sprintf("Eventos recentes — %d evento(s)", len(events)))
			formatter.Table([]string{"Tempo", "Trace ID", "Actor", "Ação", "Resultado", "Detalhes"}, rows)
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "número máximo de eventos a listar")
	return cmd
}

// actionAtPosition devolve a ação na posição dada da sequência, ou "—".
func actionAtPosition(seq trace.Sequence, pos int) string {
	if pos < len(seq.Actions) {
		return seq.Actions[pos]
	}
	return "—"
}

// truncateTraceDetails reduz o detalhe para o tamanho dado (com "…") — usado
// nas tabelas para não estourar a largura.
func truncateTraceDetails(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return strings.TrimSpace(s[:max-1]) + "…"
}

// gitShortHash devolve o short hash do commit atual (git rev-parse --short
// HEAD). Best-effort: "" quando o git falha ou não há repo.
func gitShortHash() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
