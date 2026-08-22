package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/nodegraph"
)

// NewNodeGraphCommand creates the `cosca ngraph` command group — Node Graph
// (§21 do manifesto Creative/Scientific/Media) e cache por assinatura (§23),
// Fase 1 etapa 1.6.
//
// NOTA: `cosca graph` pertence ao knowledge graph do kernel. O node graph do
// ecossistema criativo usa `cosca ngraph` (node graph).
func NewNodeGraphCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ngraph",
		Short: "Node Graph — workflows serializáveis com cache por assinatura (§21)",
		Long: `Node Graph — o grafo de nós do ecossistema criativo (§21).

Grafo = dados puros serializáveis em JSON. Execução em ordem topológica, por
demanda, com cache por assinatura de inputs (§23): só re-executa o que mudou.

Exemplo do manifesto §21:
  IMAGE → SEGMENT → REMOVE_BG → UPSCALE → EXPORT
  VIDEO → EXTRACT_AUDIO → WHISPER → TRANSLATE → SUBTITLE → RENDER

Subcommands:
  validate <file>  Validate a workflow JSON file (DAG, inputs, ciclos)
  sig <file>       Print the signature of every node (cache keys)
  run <file>       Execute the workflow (JSON in → JSON out)`,
		Example: `  cosca ngraph validate workflow.json
  cosca ngraph sig workflow.json
  cosca ngraph run workflow.json --cache`,
	}
	cmd.AddCommand(
		NewNodeGraphValidateCommand(),
		NewNodeGraphSigCommand(),
		NewNodeGraphRunCommand(),
		NewNodeGraphDemoProceduralCommand(),
		NewNodeGraphDemoSceneCommand(),
	)
	return cmd
}

// loadNodeGraphFile lê e valida um arquivo de workflow JSON.
func loadNodeGraphFile(path string) (*nodegraph.Graph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return nodegraph.Unmarshal(data)
}

// NewNodeGraphValidateCommand creates `cosca ngraph validate <file>`.
func NewNodeGraphValidateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "validate <file>",
		Short:   "Validate a workflow JSON file",
		Example: `  cosca ngraph validate workflow.json`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)

			g, err := loadNodeGraphFile(args[0])
			if err != nil {
				return err
			}
			order, err := g.TopoOrder()
			if err != nil {
				return err
			}
			formatter.Success(fmt.Sprintf("Graph %q valid: %d nodes, DAG ok", g.Name, len(order)))
			formatter.Println("")
			formatter.Println("  Execution order:")
			for i, n := range order {
				formatter.Bullet(fmt.Sprintf("%d. %s (%s)", i+1, n.ID, n.Type))
			}
			return nil
		},
	}
	return cmd
}

// NewNodeGraphSigCommand creates `cosca ngraph sig <file>`.
func NewNodeGraphSigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sig <file>",
		Short: "Print the signature (cache key) of every node",
		Long: `Print the recursive signature of every node. The signature is the
cache key: hash of (type + params + signatures of inputs). When any ancestor
changes, the signature changes — the cache invalidates the node.`,
		Example: `  cosca ngraph sig workflow.json`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			g, err := loadNodeGraphFile(args[0])
			if err != nil {
				return err
			}

			type sigRow struct {
				Node      string `json:"node"`
				Type      string `json:"type"`
				Signature string `json:"signature"`
			}
			rows := make([]sigRow, 0, g.Len())
			for _, n := range g.Nodes {
				sig, err := g.Signature(n.ID)
				if err != nil {
					return err
				}
				rows = append(rows, sigRow{Node: n.ID, Type: string(n.Type), Signature: sig})
			}
			if useJSON {
				return printJSON(cmd, rows)
			}

			formatter.Header(fmt.Sprintf("Node signatures — %s", g.Name))
			tableRows := make([][]string, 0, len(rows))
			for _, r := range rows {
				tableRows = append(tableRows, []string{r.Node, r.Type, r.Signature[:16]})
			}
			formatter.Table([]string{"Node", "Type", "Signature"}, tableRows)
			return nil
		},
	}
	return cmd
}

// NewNodeGraphRunCommand creates `cosca ngraph run <file>`.
func NewNodeGraphRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <file>",
		Short: "Execute the workflow (JSON in → JSON out)",
		Long: `Execute a workflow JSON file. Each node receives the outputs of its
inputs. Nós procedurais (famílias Noise/Math/Pattern do Procedural Kernel
System) e de mídia RODAM de verdade via procgen.Registry()/media.Executor;
tipos desconhecidos passam direto (type:id) — executores de tasks/models/gpu
plugam nas próximas etapas.

Run with --cache: a second run with identical inputs is served entirely from
the cache (proves §23 — only changed nodes re-execute).`,
		Example: `  cosca ngraph run workflow.json
  cosca ngraph run workflow.json --cache
  cosca ngraph run workflow.json --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)
			useCache := cmd.Flags().Lookup("cache").Value.String() == "true"

			g, err := loadNodeGraphFile(args[0])
			if err != nil {
				return err
			}

			exec := newDispatchExecutor(nil)

			var opts *nodegraph.RunOptions
			var cache *nodegraph.Cache
			if useCache {
				cache = nodegraph.NewCache()
				opts = &nodegraph.RunOptions{Cache: cache}
			}

			stats, err := g.Run(context.Background(), exec, opts)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, map[string]any{
					"graph":       g.Name,
					"executed":    stats.Executed,
					"cached_hits": stats.CachedHits,
					"results":     stats.Results,
				})
			}

			formatter.Success(fmt.Sprintf("Graph %q executed", g.Name))
			formatter.KeyValue("Nodes Executed", fmt.Sprint(stats.Executed))
			formatter.KeyValue("Cache Hits", fmt.Sprint(stats.CachedHits))
			formatter.Println("")
			formatter.Println("  Results:")
			for _, n := range g.Nodes {
				formatter.Bullet(fmt.Sprintf("%s → %v", n.ID, stats.Results[n.ID]))
			}
			return nil
		},
	}
	cmd.Flags().Bool("cache", false, "Run with an in-memory signature cache (§23)")
	return cmd
}
