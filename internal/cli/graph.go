package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/graph"
)

// NewGraphCommand creates the `cosca graph` command and its subcommands.
func NewGraphCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Manage the knowledge graph",
		Long: `Manage and query the Cosca knowledge graph.

The knowledge graph represents entities and their relationships,
enabling intelligent discovery and navigation of project knowledge.

Subcommands:
  show              Display the knowledge graph overview
  query <entity>    Query entity relationships
  stats             Graph statistics
  export            Export the graph in various formats
  evidence <entity> Evidence map of an entity (documented_in, tested_by,
                    affected_by, contradicted_by)
  link              Register a relationship between entities
`,
		Example: `  cosca graph show
  cosca graph query "UserService"
  cosca graph query --depth 3 "Database"
  cosca graph stats
  cosca graph export --format json
  cosca graph export --format graphml
  cosca graph evidence Bubblewrap
  cosca graph link --from Bubblewrap --to namespaces --type implements --weight 0.8`,
	}

	cmd.AddCommand(
		NewGraphShowCommand(),
		NewGraphQueryCommand(),
		NewGraphStatsCommand(),
		NewGraphExportCommand(),
		NewGraphEvidenceCommand(),
		NewGraphLinkCommand(),
	)

	return cmd
}

// NewGraphShowCommand creates the `cosca graph show` subcommand.
func NewGraphShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Display the knowledge graph overview",
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
			formatter.KeyValue("Nodes", fmt.Sprintf("%d", stats.TotalNodes))
			formatter.KeyValue("Edges", fmt.Sprintf("%d", stats.TotalEdges))

			return nil
		},
	}

	return cmd
}

// NewGraphQueryCommand creates the `cosca graph query` subcommand.
func NewGraphQueryCommand() *cobra.Command {
	var depth int

	cmd := &cobra.Command{
		Use:   "query <entity>",
		Short: "Query entity relationships",
		Long:  `Query the knowledge graph for relationships of a specific entity.`,
		Example: `  cosca graph query "UserService"
  cosca graph query --depth 3 "Database"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			g := NewGraph(".")

			nodes, err := g.Query(args[0], depth)
			if err != nil {
				return fmt.Errorf("graph query failed: %w", err)
			}

			if useJSON {
				return printJSON(cmd, nodes)
			}

			formatter.Header(fmt.Sprintf("Relationships for %q", args[0]))
			for _, n := range nodes {
				formatter.Bullet(n.Source + " --" + n.Relationship + "--> " + n.Target)
			}

			return nil
		},
	}

	cmd.Flags().IntVarP(&depth, "depth", "d", 2, "Maximum traversal depth")
	return cmd
}

// NewGraphStatsCommand creates the `cosca graph stats` subcommand.
func NewGraphStatsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show knowledge graph statistics",
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			g := NewGraph(".")
			stats, _ := g.Stats()

			if useJSON {
				return printJSON(cmd, stats)
			}

			formatter.Header("Graph Statistics")
			formatter.KeyValue("Total Nodes", fmt.Sprintf("%d", stats.TotalNodes))
			formatter.KeyValue("Total Edges", fmt.Sprintf("%d", stats.TotalEdges))
			formatter.KeyValue("Density", fmt.Sprintf("%.4f", stats.Density))
			formatter.KeyValue("Connected Components", fmt.Sprintf("%d", stats.Components))

			return nil
		},
	}

	return cmd
}

// NewGraphExportCommand creates the `cosca graph export` subcommand.
func NewGraphExportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the knowledge graph",
		Long:  `Export the knowledge graph in various formats (json, graphml, dot).`,
		Example: `  cosca graph export --format json
  cosca graph export --format graphml --output graph.graphml
  cosca graph export --format dot --output graph.dot`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

			g := NewGraph(".")

			validFormats := map[string]bool{"json": true, "graphml": true, "dot": true}
			if !validFormats[format] {
				return fmt.Errorf("unsupported format: %s (supported: json, graphml, dot)", format)
			}

			spinner := formatter.Spinner(fmt.Sprintf("Exporting graph as %s", format))
			spinner.Start()

			if outputFile == "" {
				outputFile = fmt.Sprintf("cosca-graph.%s", format)
			}
			// Dentro da jaula o workspace é "/" — um caminho absoluto do host
			// (ex.: /home/cosca/...) não existe na bolha. COSCA_PROJECT_DIR
			// aponta o workspace original: reescrevemos o caminho para o
			// equivalente relativo (ex.: <workspace>/graph.json).
			if filepath.IsAbs(outputFile) {
				proj := os.Getenv("COSCA_PROJECT_DIR")
				if proj != "" {
					if rel, err := filepath.Rel(proj, outputFile); err == nil && !strings.HasPrefix(rel, "..") {
						outputFile = rel
					}
				}
			}
			if err := g.Export(outputFile, format); err != nil {
				spinner.Stop("Export failed")
				return fmt.Errorf("graph export failed: %w", err)
			}

			stats, _ := g.Stats()
			spinner.Stop(fmt.Sprintf("Graph exported: %d nodes, %d edges", stats.TotalNodes, stats.TotalEdges))

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "json", "Export format (json, graphml, dot)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file path")
	return cmd
}

// ── Grafo de evidências ─────────────────────────────────────────────────────
//
// `cosca graph evidence <entity>` e `cosca graph link` operam sobre um grafo
// persistido em <projeto>/.cosca/graph/evidence.json (snapshot JSON via
// graph.Serialize/Deserialize). As relações de evidência — documented_in,
// tested_by, affected_by, contradicted_by — transformam o conhecimento de
// "pasta cheia de documentos" num mapa de evidências navegável.

// evidenceGraphDir é o diretório do grafo de evidências dentro do projeto.
const evidenceGraphDir = ".cosca/graph"

// evidenceGraphFile é o snapshot JSON do grafo de evidências.
const evidenceGraphFile = "evidence.json"

// evidenceGraphPath devolve o caminho absoluto do snapshot do grafo de
// evidências no diretório de trabalho atual.
func evidenceGraphPath() string {
	return filepath.Join(evidenceGraphDir, evidenceGraphFile)
}

// loadEvidenceGraph carrega o snapshot persistido; devolve um grafo vazio
// quando ainda não existe nenhum.
func loadEvidenceGraph() (*graph.Graph, error) {
	data, err := os.ReadFile(evidenceGraphPath())
	if err != nil {
		if os.IsNotExist(err) {
			return graph.New(), nil
		}
		return nil, fmt.Errorf("ler %s: %w", evidenceGraphPath(), err)
	}
	g, err := graph.Deserialize(data)
	if err != nil {
		return nil, fmt.Errorf("deserializar grafo de evidências: %w", err)
	}
	return g, nil
}

// saveEvidenceGraph persiste o snapshot do grafo de evidências.
func saveEvidenceGraph(g *graph.Graph) error {
	data, err := g.Serialize()
	if err != nil {
		return fmt.Errorf("serializar grafo de evidências: %w", err)
	}
	if err := os.MkdirAll(evidenceGraphDir, 0o755); err != nil {
		return fmt.Errorf("criar %s: %w", evidenceGraphDir, err)
	}
	if err := os.WriteFile(evidenceGraphPath(), data, 0o644); err != nil {
		return fmt.Errorf("salvar %s: %w", evidenceGraphPath(), err)
	}
	return nil
}

// isValidRelationship valida o tipo da relação contra os tipos conhecidos.
func isValidRelationship(rel string) bool {
	for _, valid := range graph.ValidRelationshipTypes() {
		if valid == rel {
			return true
		}
	}
	return false
}

// graphEvidenceEdge é uma aresta dirigida no mapa de evidências de uma
// entidade. Incoming indica que a aresta aponta PARA a entidade consultada.
type graphEvidenceEdge struct {
	Type     string  `json:"type"`
	Source   string  `json:"source"`
	Target   string  `json:"target"`
	Weight   float64 `json:"weight"`
	Incoming bool    `json:"incoming,omitempty"`
}

// graphEvidenceResult é a forma JSON de `cosca graph evidence <entity>`.
type graphEvidenceResult struct {
	Entity string              `json:"entity"`
	Edges  []graphEvidenceEdge `json:"edges"`
}

// evidenceEdges coleta todas as arestas (de saída e de entrada) da entidade,
// preservando a ordem de inserção das arestas de saída e ordenando as de
// entrada pelo ID da fonte para manter a saída determinística.
func evidenceEdges(g *graph.Graph, entity string) []graphEvidenceEdge {
	var result []graphEvidenceEdge

	if out, err := g.GetEdges(entity); err == nil {
		for _, e := range out {
			result = append(result, graphEvidenceEdge{
				Type: e.Type, Source: e.Source, Target: e.Target, Weight: e.Weight,
			})
		}
	}

	var incoming []graphEvidenceEdge
	for _, e := range g.GetAllEdges() {
		if e.Target == entity && e.Source != entity {
			incoming = append(incoming, graphEvidenceEdge{
				Type: e.Type, Source: e.Source, Target: e.Target, Weight: e.Weight,
				Incoming: true,
			})
		}
	}
	sort.SliceStable(incoming, func(i, j int) bool {
		return incoming[i].Source < incoming[j].Source
	})
	result = append(result, incoming...)

	return result
}

// NewGraphEvidenceCommand cria `cosca graph evidence <entity>` — o mapa de
// evidências de uma entidade.
func NewGraphEvidenceCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "evidence <entity>",
		Short: "Mapa de evidências de uma entidade — relações e evidências no grafo",
		Long: `Mostra o mapa de evidências de uma entidade: todas as arestas de
saída e de entrada (documented_in, tested_by, affected_by, contradicted_by e
as demais relações), agrupadas por relação. O conhecimento deixa de ser uma
pasta cheia de documentos e vira um mapa de evidências navegável.

O grafo é persistido em .cosca/graph/evidence.json.`,
		Example: `  cosca graph evidence Bubblewrap
  cosca graph evidence Bubblewrap --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			entity := args[0]

			g, err := loadEvidenceGraph()
			if err != nil {
				return err
			}

			edges := evidenceEdges(g, entity)

			if useJSON {
				return printJSON(cmd, graphEvidenceResult{Entity: entity, Edges: edges})
			}

			if len(edges) == 0 {
				formatter.Warning(fmt.Sprintf(
					"Nenhuma evidência registrada para %q — registre com \"cosca graph link --from %s --to <alvo> --type <rel>\".",
					entity, entity))
				return nil
			}

			formatter.Header(fmt.Sprintf("Mapa de evidências — %s", entity))
			for _, e := range edges {
				arrow := "→"
				otherEnd := e.Target
				if e.Incoming {
					arrow = "←"
					otherEnd = e.Source
				}
				line := fmt.Sprintf("%s %s %s", e.Type, arrow, otherEnd)
				if e.Weight != 1.0 {
					line += fmt.Sprintf(" (weight %g)", e.Weight)
				}
				formatter.Bullet(line)
			}

			return nil
		},
	}
}

// NewGraphLinkCommand cria `cosca graph link --from <id> --to <id> --type
// <rel> [--weight 0.9]` — registra uma relação no grafo de evidências.
func NewGraphLinkCommand() *cobra.Command {
	var (
		from   string
		to     string
		rel    string
		weight float64
	)

	cmd := &cobra.Command{
		Use:   "link",
		Short: "Registra uma relação entre entidades no grafo de evidências",
		Long: `Registra uma relação dirigida <from> → <to> no grafo de evidências
(snapshot em .cosca/graph/evidence.json). O tipo da relação é validado contra
os tipos conhecidos; peso é opcional (default 1.0).

Tipos de relação de evidência: documented_in, tested_by, affected_by,
contradicted_by — além de implements, depends_on, references e demais.`,
		Example: `  cosca graph link --from Bubblewrap --to namespaces --type implements --weight 0.8
  cosca graph link --from Bubblewrap --to CVE-2024-2947 --type affected_by
  cosca graph link --from Bubblewrap --to CONFLICT-102 --type contradicted_by`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			if !isValidRelationship(rel) {
				return fmt.Errorf(
					"tipo de relação inválido %q — use um dos tipos: %s",
					rel, strings.Join(graph.ValidRelationshipTypes(), ", "))
			}

			g, err := loadEvidenceGraph()
			if err != nil {
				return err
			}

			edge := &graph.Edge{Source: from, Target: to, Type: rel, Weight: weight}
			if err := g.AddEdge(edge); err != nil {
				return fmt.Errorf("registrar relação: %w", err)
			}
			if err := saveEvidenceGraph(g); err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, edge)
			}

			formatter.Success(fmt.Sprintf("Relação %s → %s (%s) registrada no grafo de evidências.", from, to, rel))
			if weight != 1.0 {
				formatter.KeyValue("Peso", fmt.Sprintf("%g", edge.Weight))
			}
			formatter.KeyValue("Grafo", evidenceGraphPath())
			return nil
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "ID da entidade de origem (ex.: Bubblewrap)")
	cmd.Flags().StringVar(&to, "to", "", "ID da entidade de destino (ex.: namespaces)")
	cmd.Flags().StringVar(&rel, "type", "", "tipo da relação (ex.: implements, affected_by)")
	cmd.Flags().Float64Var(&weight, "weight", 1.0, "peso da relação (default 1.0)")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("to")
	_ = cmd.MarkFlagRequired("type")
	return cmd
}

var format string
var outputFile string
